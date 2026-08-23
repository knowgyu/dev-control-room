package collector

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
)

const (
	maxRepositoryDiscoveryDepth   = 8
	maxRepositoryDiscoveryEntries = 10000
)

// DiscoverGitRoots finds Git checkout roots below a user-selected directory.
// It only reads directory metadata, never follows symlinks, and stops at the
// first Git root so nested vendor/worktree internals are not re-registered.
func DiscoverGitRoots(root string) ([]string, error) {
	canonical, err := canonicalDirectory(root)
	if err != nil {
		return nil, err
	}
	var roots []string
	entries := 0
	var visit func(string, int) error
	visit = func(directory string, depth int) error {
		if depth > maxRepositoryDiscoveryDepth {
			return nil
		}
		items, err := os.ReadDir(directory)
		if err != nil {
			return nil
		}
		entries += len(items)
		if entries > maxRepositoryDiscoveryEntries {
			return errors.New("repository discovery reached its bounded entry limit")
		}
		for _, item := range items {
			if item.Name() == ".git" {
				roots = append(roots, directory)
				return nil
			}
		}
		if depth == maxRepositoryDiscoveryDepth {
			return nil
		}
		for _, item := range items {
			if !item.IsDir() || item.Type()&os.ModeSymlink != 0 || skippedDiscoveryDirectory(item.Name()) {
				continue
			}
			if err := visit(filepath.Join(directory, item.Name()), depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(canonical, 0); err != nil {
		return nil, err
	}
	sort.Strings(roots)
	return roots, nil
}

func skippedDiscoveryDirectory(name string) bool {
	switch name {
	case ".git", ".worktrees", "node_modules", "vendor", "bin", "obj", "build", "dist", ".venv", ".tox":
		return true
	default:
		return false
	}
}
