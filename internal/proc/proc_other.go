//go:build !linux && !windows

package proc

import (
	"errors"
	"runtime"
)

var errUnsupported = errors.New("process snapshots are not implemented on " + runtime.GOOS)

func Processes() ([]Process, error)        { return nil, errUnsupported }
func Modules(pid uint32) ([]Module, error) { return nil, errUnsupported }
