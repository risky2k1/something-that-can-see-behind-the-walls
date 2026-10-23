// Package search implements the name-matching part of process/module lookup:
// walk a snapshot of items, compare each name against a target, return the hit.
package search

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound is returned (wrapped) when no item in a snapshot matches.
var ErrNotFound = errors.New("not found")

// Mode selects how a candidate name is compared with the target.
type Mode int

const (
	// Exact is a byte-for-byte comparison (the original binary used this for processes).
	Exact Mode = iota
	// Fold is a Unicode case-insensitive comparison (the original used strings.EqualFold for modules).
	Fold
	// Contains is a case-insensitive substring match, for interactive searching.
	Contains
)

func (m Mode) String() string {
	switch m {
	case Exact:
		return "exact"
	case Fold:
		return "fold"
	case Contains:
		return "contains"
	default:
		return fmt.Sprintf("Mode(%d)", int(m))
	}
}

// ParseMode converts a flag value into a Mode.
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "exact":
		return Exact, nil
	case "fold", "i", "insensitive":
		return Fold, nil
	case "contains", "sub", "substring":
		return Contains, nil
	default:
		return 0, fmt.Errorf("unknown match mode %q (want exact, fold or contains)", s)
	}
}

// Match reports whether candidate matches target under m.
func (m Mode) Match(candidate, target string) bool {
	switch m {
	case Exact:
		return candidate == target
	case Fold:
		return strings.EqualFold(candidate, target)
	case Contains:
		return strings.Contains(strings.ToLower(candidate), strings.ToLower(target))
	default:
		return false
	}
}

// All returns every item whose name matches target, in snapshot order.
// An empty target matches everything.
func All[T any](items []T, name func(T) string, target string, mode Mode) []T {
	if target == "" {
		return items
	}
	var out []T
	for _, it := range items {
		if mode.Match(name(it), target) {
			out = append(out, it)
		}
	}
	return out
}

// Best returns the single best match for target.
//
// The original binary returned the first hit in snapshot order. That is kept as the
// tie-breaker, but an exact-case hit is preferred over a looser one, so "Foo.dll"
// wins over "FOO.DLL" when searching for "Foo.dll" in Fold or Contains mode.
func Best[T any](items []T, name func(T) string, target string, mode Mode) (T, bool) {
	var zero T
	if target == "" {
		return zero, false
	}
	for _, it := range items {
		if name(it) == target {
			return it, true
		}
	}
	if mode == Exact {
		return zero, false
	}
	for _, it := range items {
		if mode.Match(name(it), target) {
			return it, true
		}
	}
	return zero, false
}
