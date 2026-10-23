//go:build linux

package proc

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"procsearch/internal/search"
)

func TestParseStat(t *testing.T) {
	comm, ppid, err := parseStat([]byte("4242 (my (odd) proc) S 17 4242 4242 0 -1"))
	if err != nil || comm != "my (odd) proc" || ppid != 17 {
		t.Fatalf("parseStat = %q, %d, %v", comm, ppid, err)
	}
	if _, _, err := parseStat([]byte("garbage")); err == nil {
		t.Fatal("expected error for malformed stat")
	}
}

func fakeProc(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	write := func(rel, data string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("1/stat", "1 (init) S 0 1 1 0 -1")
	write("300/stat", "300 (Game.exe) S 1 300 300 0 -1")
	write("20/stat", "20 (game.exe) S 1 20 20 0 -1")
	write("20/maps", ""+
		"00400000-00401000 r--p 00000000 08:01 11 /opt/game/game.exe\n"+
		"00401000-00480000 r-xp 00001000 08:01 11 /opt/game/game.exe\n"+
		"01000000-01021000 rw-p 00000000 00:00 0  [heap]\n"+
		"7f0000000000-7f0000010000 r--p 00000000 08:01 22 /opt/game/Client.so\n"+
		"7f0000010000-7f0000090000 r-xp 00010000 08:01 22 /opt/game/Client.so\n"+
		"7f00000a0000-7f00000b0000 rw-p 00000000 00:00 0\n")
	write("self/stat", "not a pid dir, must be ignored")

	old := procRoot
	procRoot = root
	t.Cleanup(func() { procRoot = old })
}

func TestProcessesAndFind(t *testing.T) {
	fakeProc(t)

	procs, err := Processes()
	if err != nil {
		t.Fatal(err)
	}
	if len(procs) != 3 || procs[0].PID != 1 || procs[1].PID != 20 || procs[2].PID != 300 {
		t.Fatalf("Processes = %+v", procs)
	}

	pid, err := FindProcessID("Game.exe", search.Exact)
	if err != nil || pid != 300 {
		t.Fatalf("exact = %d, %v", pid, err)
	}
	pid, err = FindProcessID("GAME.EXE", search.Fold)
	if err != nil || pid != 20 {
		t.Fatalf("fold should return first in PID order, got %d, %v", pid, err)
	}
	if _, err := FindProcessID("missing", search.Fold); err == nil || !isNotFound(err) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestModulesAndFindBase(t *testing.T) {
	fakeProc(t)

	mods, err := Modules(20)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 2 {
		t.Fatalf("Modules = %+v", mods)
	}
	if mods[0].Name != "game.exe" || mods[0].Base != 0x400000 || mods[0].Size != 0x80000 {
		t.Errorf("exe module = %+v", mods[0])
	}

	base, err := FindModuleBase(20, "client.so", search.Fold)
	if err != nil || base != 0x7f0000000000 {
		t.Fatalf("FindModuleBase = %#x, %v", base, err)
	}
	if _, err := FindModuleBase(20, "client.so", search.Exact); !isNotFound(err) {
		t.Fatalf("exact lookup should miss, got %v", err)
	}
	if _, err := Modules(999); err == nil {
		t.Fatal("expected error for missing pid")
	}
}

func isNotFound(err error) bool {
	return err != nil && errors.Is(err, search.ErrNotFound)
}
