//go:build windows

package proc

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

// Toolhelp32 module APIs are not wrapped by package syscall, so load them directly.
var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procModule32Frst = kernel32.NewProc("Module32FirstW")
	procModule32Next = kernel32.NewProc("Module32NextW")
)

const (
	th32csSnapModule   = 0x00000008
	th32csSnapModule32 = 0x00000010
	errorBadLength     = syscall.Errno(24)
)

// moduleEntry32 mirrors MODULEENTRY32W.
type moduleEntry32 struct {
	Size         uint32
	ModuleID     uint32
	ProcessID    uint32
	GlblcntUsage uint32
	ProccntUsage uint32
	ModBaseAddr  uintptr
	ModBaseSize  uint32
	HModule      syscall.Handle
	Module       [256]uint16
	ExePath      [syscall.MAX_PATH]uint16
}

// Processes snapshots the process list with CreateToolhelp32Snapshot.
//
// Unlike the original binary, which only called Process32Next and so silently
// skipped the first entry, this starts with Process32First.
func Processes() ([]Process, error) {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot: %w", err)
	}
	defer syscall.CloseHandle(snap)

	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))

	var out []Process
	for err = syscall.Process32First(snap, &e); err == nil; err = syscall.Process32Next(snap, &e) {
		out = append(out, Process{
			PID:  e.ProcessID,
			PPID: e.ParentProcessID,
			Name: syscall.UTF16ToString(e.ExeFile[:]),
		})
	}
	if !errors.Is(err, syscall.ERROR_NO_MORE_FILES) {
		return nil, fmt.Errorf("Process32: %w", err)
	}
	return out, nil
}

// Modules snapshots the modules loaded in pid (both 64- and 32-bit).
// Needs PROCESS_QUERY_INFORMATION | PROCESS_VM_READ on the target, so protected
// or elevated processes return ERROR_ACCESS_DENIED.
func Modules(pid uint32) ([]Module, error) {
	var snap syscall.Handle
	var err error
	// ERROR_BAD_LENGTH means the target was loading/unloading modules; retry.
	for range 5 {
		snap, err = syscall.CreateToolhelp32Snapshot(th32csSnapModule|th32csSnapModule32, pid)
		if !errors.Is(err, errorBadLength) {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("pid %d: CreateToolhelp32Snapshot: %w", pid, err)
	}
	defer syscall.CloseHandle(snap)

	var e moduleEntry32
	e.Size = uint32(unsafe.Sizeof(e))

	var out []Module
	for err = module32(procModule32Frst, snap, &e); err == nil; err = module32(procModule32Next, snap, &e) {
		out = append(out, Module{
			Name: syscall.UTF16ToString(e.Module[:]),
			Path: syscall.UTF16ToString(e.ExePath[:]),
			Base: e.ModBaseAddr,
			Size: uint64(e.ModBaseSize),
		})
	}
	if !errors.Is(err, syscall.ERROR_NO_MORE_FILES) {
		return nil, fmt.Errorf("pid %d: Module32: %w", pid, err)
	}
	return out, nil
}

func module32(p *syscall.LazyProc, snap syscall.Handle, e *moduleEntry32) error {
	r, _, err := p.Call(uintptr(snap), uintptr(unsafe.Pointer(e)))
	if r != 0 {
		return nil
	}
	return err
}
