//go:build linux

package proc

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// procRoot is a variable so tests can point it at a fake /proc tree.
var procRoot = "/proc"

// Processes snapshots /proc. Processes that exit mid-walk are skipped.
func Processes() ([]Process, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", procRoot, err)
	}

	var out []Process
	for _, e := range entries {
		pid, err := strconv.ParseUint(e.Name(), 10, 32)
		if err != nil || !e.IsDir() {
			continue
		}
		p, err := readProcess(uint32(pid))
		if err != nil {
			continue // raced with exit, or a kernel thread we cannot read
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out, nil
}

func readProcess(pid uint32) (Process, error) {
	dir := filepath.Join(procRoot, strconv.FormatUint(uint64(pid), 10))

	stat, err := os.ReadFile(filepath.Join(dir, "stat"))
	if err != nil {
		return Process{}, err
	}
	comm, ppid, err := parseStat(stat)
	if err != nil {
		return Process{}, err
	}

	// comm is truncated to 15 bytes by the kernel; prefer the real executable name when readable.
	name := comm
	if exe, err := os.Readlink(filepath.Join(dir, "exe")); err == nil {
		name = filepath.Base(strings.TrimSuffix(exe, " (deleted)"))
	}
	return Process{PID: pid, PPID: ppid, Name: name}, nil
}

// parseStat extracts comm and ppid from /proc/<pid>/stat: "pid (comm) state ppid ...".
// comm may itself contain spaces and parentheses, so split on the last ')'.
func parseStat(b []byte) (comm string, ppid uint32, err error) {
	open := bytes.IndexByte(b, '(')
	end := bytes.LastIndexByte(b, ')')
	if open < 0 || end < open {
		return "", 0, errors.New("malformed stat")
	}
	fields := strings.Fields(string(b[end+1:]))
	if len(fields) < 2 {
		return "", 0, errors.New("malformed stat")
	}
	v, err := strconv.ParseUint(fields[1], 10, 32)
	if err != nil {
		return "", 0, fmt.Errorf("malformed stat ppid: %w", err)
	}
	return string(b[open+1 : end]), uint32(v), nil
}

// Modules reads /proc/<pid>/maps and folds the mappings of each file into one Module.
// Reading another user's process requires ptrace permission (same user or root).
func Modules(pid uint32) ([]Module, error) {
	path := filepath.Join(procRoot, strconv.FormatUint(uint64(pid), 10), "maps")
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("pid %d: no such process", pid)
		}
		return nil, fmt.Errorf("pid %d: %w", pid, err)
	}
	defer f.Close()

	mods, err := parseMaps(f)
	if err != nil {
		return nil, fmt.Errorf("pid %d: %w", pid, err)
	}
	return mods, nil
}

func parseMaps(f *os.File) ([]Module, error) {
	type span struct{ lo, hi uint64 }
	spans := map[string]*span{}
	var order []string

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		// 7f12a000-7f12b000 r-xp 00000000 08:01 1234   /usr/lib/libc.so.6
		fields := strings.Fields(sc.Text())
		if len(fields) < 6 {
			continue // anonymous mapping
		}
		file := strings.Join(fields[5:], " ")
		if !strings.HasPrefix(file, "/") {
			continue // [heap], [stack], [vdso], ...
		}
		file = strings.TrimSuffix(file, " (deleted)")

		lo, hi, ok := strings.Cut(fields[0], "-")
		if !ok {
			continue
		}
		start, err1 := strconv.ParseUint(lo, 16, 64)
		end, err2 := strconv.ParseUint(hi, 16, 64)
		if err1 != nil || err2 != nil {
			continue
		}

		if s, seen := spans[file]; seen {
			s.lo = min(s.lo, start)
			s.hi = max(s.hi, end)
		} else {
			spans[file] = &span{start, end}
			order = append(order, file)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	out := make([]Module, 0, len(order))
	for _, file := range order {
		s := spans[file]
		out = append(out, Module{
			Name: filepath.Base(file),
			Path: file,
			Base: uintptr(s.lo),
			Size: s.hi - s.lo,
		})
	}
	return out, nil
}
