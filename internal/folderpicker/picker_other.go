//go:build !windows

package folderpicker

func pick() (string, error) {
	return "", ErrUnavailable
}
