//go:build windows

package store

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

type platformFileLock struct {
	overlapped windows.Overlapped
}

func tryPlatformFileLock(file *os.File, state *platformFileLock) error {
	err := windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		&state.overlapped,
	)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
		return errStorageLockUnavailable
	}
	return err
}

func unlockPlatformFile(file *os.File, state *platformFileLock) error {
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &state.overlapped)
}
