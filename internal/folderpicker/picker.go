package folderpicker

import "errors"

var (
	ErrUnavailable = errors.New("native folder picker is unavailable")
	ErrCancelled   = errors.New("folder selection was cancelled")
)

// Pick opens the local operating-system folder picker. It never reads or
// returns a path unless the user explicitly confirms the selection.
func Pick() (string, error) {
	return pick()
}
