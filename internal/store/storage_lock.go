package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	storageLockTimeout       = 5 * time.Second
	storageLockPollInterval  = 25 * time.Millisecond
	storageRetryAttempts     = 4
	storageRetryInitialDelay = 15 * time.Millisecond
)

var (
	errStorageLockUnavailable = errors.New("storage lock is unavailable")
	ErrStorageBusy            = errors.New("storage is busy")
)

type storageLockContextKey struct{}

func contextWithStorageLock(ctx context.Context) context.Context {
	return context.WithValue(ctx, storageLockContextKey{}, true)
}

func hasStorageLock(ctx context.Context) bool {
	return ctx != nil && ctx.Value(storageLockContextKey{}) == true
}

// StorageBusyError indicates that another process held the same home's
// storage lock until the bounded wait expired. Its message intentionally
// contains no database path, SQL text, or underlying operating-system detail.
type StorageBusyError struct {
	operation string
}

func (e *StorageBusyError) Error() string {
	if e == nil || e.operation == "" {
		return "storage is busy; retry later"
	}
	return fmt.Sprintf("storage is busy during %s; retry later", e.operation)
}

func (e *StorageBusyError) Unwrap() error { return ErrStorageBusy }

func IsStorageBusy(err error) bool {
	return errors.Is(err, ErrStorageBusy)
}

type storageLock struct {
	file     *os.File
	platform platformFileLock
	release  sync.Once
}

func databaseLockPath(databasePath string) string {
	return databasePath + ".lock"
}

func acquireStorageLock(ctx context.Context, lockPath string) (*storageLock, error) {
	return acquireStorageLockWithin(ctx, lockPath, storageLockTimeout)
}

func acquireStorageLockWithin(ctx context.Context, lockPath string, timeout time.Duration) (*storageLock, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		timeout = storageLockPollInterval
	}
	deadline := time.Now().Add(timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return nil, fmt.Errorf("open storage lock: storage unavailable")
		}
		_ = os.Chmod(lockPath, 0o600)
		lock := &storageLock{file: file}
		if err := tryPlatformFileLock(file, &lock.platform); err == nil {
			return lock, nil
		} else if !errors.Is(err, errStorageLockUnavailable) {
			_ = file.Close()
			return nil, fmt.Errorf("acquire storage lock: storage unavailable")
		}
		_ = file.Close()

		if !time.Now().Before(deadline) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return nil, &StorageBusyError{operation: "storage access"}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(minDuration(storageLockPollInterval, time.Until(deadline))):
		}
	}
}

func (l *storageLock) Close() error {
	if l == nil {
		return nil
	}
	var err error
	l.release.Do(func() {
		if unlockErr := unlockPlatformFile(l.file, &l.platform); unlockErr != nil {
			err = fmt.Errorf("release storage lock: storage unavailable")
		}
		if closeErr := l.file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close storage lock: storage unavailable")
		}
	})
	return err
}

func storageDatabasePath(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == ":memory:" || strings.HasPrefix(name, "file::memory:") {
		return ""
	}
	if strings.HasPrefix(name, "file:") {
		name = strings.TrimPrefix(name, "file:")
	}
	if query := strings.IndexByte(name, '?'); query >= 0 {
		name = name[:query]
	}
	if name == "" {
		return ""
	}
	if decoded, err := filepath.Abs(name); err == nil {
		return filepath.Clean(decoded)
	}
	return filepath.Clean(name)
}

func storageLockPath(name string) string {
	if databasePath := storageDatabasePath(name); databasePath != "" {
		return databaseLockPath(databasePath)
	}
	return ""
}

func waitForStorageRetry(ctx context.Context, attempt int) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if attempt >= storageRetryAttempts-1 {
		return &StorageBusyError{operation: "storage access"}
	}
	delay := storageRetryInitialDelay * time.Duration(1<<attempt)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func minDuration(left, right time.Duration) time.Duration {
	if right <= 0 || left < right {
		return left
	}
	return right
}

func isSQLiteBusyError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"database is locked",
		"database table is locked",
		"database schema is locked",
		"sqlite_busy",
		"sqlite_locked",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
