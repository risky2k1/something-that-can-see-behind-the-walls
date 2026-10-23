// Package proc takes snapshots of running processes and of the modules
// (executables and shared libraries) mapped into a process, and finds entries by name.
//
// Snapshots are OS-specific (proc_linux.go, proc_windows.go); lookups are not.
package proc

import (
	"fmt"

	"procsearch/internal/search"
)

// Process is one entry of a process snapshot.
type Process struct {
	PID  uint32
	PPID uint32
	Name string // executable base name, e.g. "bash" or "notepad.exe"
}

// Module is an executable image or shared library mapped into a process.
type Module struct {
	Name string  // base name, e.g. "libc.so.6" or "kernel32.dll"
	Path string  // full path on disk
	Base uintptr // lowest mapped address
	Size uint64  // bytes from Base to the end of the last mapping
}

func processName(p Process) string { return p.Name }
func moduleName(m Module) string   { return m.Name }

// FindProcesses returns all running processes whose name matches.
func FindProcesses(name string, mode search.Mode) ([]Process, error) {
	procs, err := Processes()
	if err != nil {
		return nil, err
	}
	return search.All(procs, processName, name, mode), nil
}

// FindProcessID returns the PID of the best-matching process
// (equivalent of the original main.findProcessId).
func FindProcessID(name string, mode search.Mode) (uint32, error) {
	procs, err := Processes()
	if err != nil {
		return 0, err
	}
	p, ok := search.Best(procs, processName, name, mode)
	if !ok {
		return 0, fmt.Errorf("process %q: %w", name, search.ErrNotFound)
	}
	return p.PID, nil
}

// FindModules returns all modules in pid whose name matches.
func FindModules(pid uint32, name string, mode search.Mode) ([]Module, error) {
	mods, err := Modules(pid)
	if err != nil {
		return nil, err
	}
	return search.All(mods, moduleName, name, mode), nil
}

// FindModuleBase returns the base address of the best-matching module in pid
// (equivalent of the original main.getModuleBaseAddress).
func FindModuleBase(pid uint32, name string, mode search.Mode) (uintptr, error) {
	mods, err := Modules(pid)
	if err != nil {
		return 0, err
	}
	m, ok := search.Best(mods, moduleName, name, mode)
	if !ok {
		return 0, fmt.Errorf("module %q in pid %d: %w", name, pid, search.ErrNotFound)
	}
	return m.Base, nil
}
