// Command procsearch finds running processes and loaded modules by name.
//
//	procsearch ps   [-match exact|fold|contains] [-first] [NAME]
//	procsearch mods -pid PID [-match exact|fold|contains] [-first] [NAME]
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"procsearch/internal/proc"
	"procsearch/internal/search"
)

const usage = `usage:
  procsearch ps   [-match MODE] [-first] [NAME]          list/find processes (default MODE: exact)
  procsearch mods -pid PID [-match MODE] [-first] [NAME] list/find modules in a process (default MODE: fold)

MODE is exact, fold (case-insensitive) or contains (case-insensitive substring).
-first prints only the best match: a PID for ps, a hex base address for mods.
Exit status: 0 found, 1 not found, 2 usage or system error.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	var err error
	switch args[0] {
	case "ps":
		err = runPs(args[1:], stdout)
	case "mods", "modules":
		err = runMods(args[1:], stdout)
	case "-h", "-help", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}

	switch {
	case err == nil:
		return 0
	case errors.Is(err, search.ErrNotFound):
		fmt.Fprintln(stderr, "procsearch:", err)
		return 1
	default:
		fmt.Fprintln(stderr, "procsearch:", err)
		return 2
	}
}

type common struct {
	mode  search.Mode
	first bool
	name  string
}

func parseCommon(fs *flag.FlagSet, args []string, defaultMode search.Mode) (common, error) {
	match := fs.String("match", defaultMode.String(), "match mode: exact, fold or contains")
	first := fs.Bool("first", false, "print only the best match")
	if err := fs.Parse(args); err != nil {
		return common{}, err
	}
	mode, err := search.ParseMode(*match)
	if err != nil {
		return common{}, err
	}
	if fs.NArg() > 1 {
		return common{}, fmt.Errorf("expected at most one NAME, got %d", fs.NArg())
	}
	c := common{mode: mode, first: *first, name: fs.Arg(0)}
	if c.first && c.name == "" {
		return common{}, errors.New("-first needs a NAME")
	}
	return c, nil
}

func runPs(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("ps", flag.ContinueOnError)
	c, err := parseCommon(fs, args, search.Exact)
	if err != nil {
		return err
	}

	if c.first {
		pid, err := proc.FindProcessID(c.name, c.mode)
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, pid)
		return nil
	}

	procs, err := proc.FindProcesses(c.name, c.mode)
	if err != nil {
		return err
	}
	if len(procs) == 0 {
		return fmt.Errorf("process %q: %w", c.name, search.ErrNotFound)
	}
	tw := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PID\tPPID\tNAME")
	for _, p := range procs {
		fmt.Fprintf(tw, "%d\t%d\t%s\n", p.PID, p.PPID, p.Name)
	}
	return tw.Flush()
}

func runMods(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("mods", flag.ContinueOnError)
	pid := fs.Uint("pid", 0, "process ID to inspect (required)")
	c, err := parseCommon(fs, args, search.Fold)
	if err != nil {
		return err
	}
	if *pid == 0 || *pid > 1<<32-1 {
		return errors.New("-pid is required")
	}

	if c.first {
		base, err := proc.FindModuleBase(uint32(*pid), c.name, c.mode)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%#x\n", base)
		return nil
	}

	mods, err := proc.FindModules(uint32(*pid), c.name, c.mode)
	if err != nil {
		return err
	}
	if len(mods) == 0 {
		return fmt.Errorf("module %q in pid %d: %w", c.name, *pid, search.ErrNotFound)
	}
	tw := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "BASE\tSIZE\tNAME\tPATH")
	for _, m := range mods {
		fmt.Fprintf(tw, "%#x\t%#x\t%s\t%s\n", m.Base, m.Size, m.Name, m.Path)
	}
	return tw.Flush()
}
