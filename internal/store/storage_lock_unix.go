//go:build !windows

package store

import (
	"errors"
	"os"
	"syscall"
)

type platformFileLock struct{}

func tryPlatformFileLock(file *os.File, _ *platformFileLock) error {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return errStorageLockUnavailable
	}
	return err
}

func unlockPlatformFile(file *os.File, _ *platformFileLock) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
