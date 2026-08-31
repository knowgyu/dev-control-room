//go:build go1.24

package app

import "os"

func openAssuranceCleanupRoot(home string) (assuranceCleanupRoot, error) {
	return os.OpenRoot(home)
}

func assuranceCleanupRootSupported() bool {
	return true
}
