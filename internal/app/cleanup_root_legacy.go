//go:build !go1.24

package app

import (
	"errors"
	"os"
	"path/filepath"
)

var errAssuranceCleanupRootUnavailable = errors.New("assurance cleanup requires directory-handle support")

type legacyAssuranceCleanupRoot struct {
	home string
}

func openAssuranceCleanupRoot(home string) (assuranceCleanupRoot, error) {
	return legacyAssuranceCleanupRoot{home: home}, nil
}

func assuranceCleanupRootSupported() bool {
	return false
}

func (r legacyAssuranceCleanupRoot) path(name string) string {
	return filepath.Join(r.home, filepath.FromSlash(name))
}

func (r legacyAssuranceCleanupRoot) Lstat(name string) (os.FileInfo, error) {
	return os.Lstat(r.path(name))
}

func (r legacyAssuranceCleanupRoot) Stat(name string) (os.FileInfo, error) {
	return os.Stat(r.path(name))
}

func (r legacyAssuranceCleanupRoot) Open(name string) (*os.File, error) {
	return os.Open(r.path(name))
}

func (r legacyAssuranceCleanupRoot) Remove(string) error {
	return errAssuranceCleanupRootUnavailable
}

func (r legacyAssuranceCleanupRoot) RemoveAll(string) error {
	return errAssuranceCleanupRootUnavailable
}

func (r legacyAssuranceCleanupRoot) Close() error {
	return nil
}
