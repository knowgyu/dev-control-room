package collector

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
)

const (
	maxRepositoryDiscoveryDepth   = 8
	maxRepositoryDiscoveryEntries = 10000
	repositoryDiscoveryReadBatch  = 128
)

const (
	repositoryDiscoveryEntryLimitWarning = "repository discovery stopped at its bounded entry limit; narrow the selected folder to continue"
	repositoryDiscoveryDepthLimitWarning = "repository discovery stopped at its bounded depth limit; narrow the selected folder to continue"
	repositoryDiscoveryReadWarning       = "repository discovery could not read every directory; narrow the selected folder to continue"
	repositoryDiscoveryPathWarning       = "repository discovery skipped a directory whose path changed during traversal"
)

// GitRootDiscoveryResult contains the roots found by a bounded traversal.
// Partial results are safe to display but must not be treated as a complete
// registration set.
type GitRootDiscoveryResult struct {
	Roots    []string
	Partial  bool
	Warnings []string
}

// DiscoverGitRoots finds Git checkout roots below a user-selected directory.
// It only reads directory metadata, never follows symlinks, and stops at the
// first Git root so nested vendor/worktree internals are not re-registered.
func DiscoverGitRoots(root string) ([]string, error) {
	result, err := DiscoverGitRootsDetailed(context.Background(), root)
	if err != nil {
		return nil, err
	}
	if result.Partial {
		return nil, errors.New(repositoryDiscoveryIncompleteError(result.Warnings))
	}
	return result.Roots, nil
}

// DiscoverGitRootsDetailed performs bounded, read-only repository discovery.
// It preserves roots found before a safety bound or an unreadable child stops
// traversal and reports that condition in Partial and Warnings.
func DiscoverGitRootsDetailed(ctx context.Context, root string) (GitRootDiscoveryResult, error) {
	return discoverGitRoots(ctx, root, maxRepositoryDiscoveryDepth, maxRepositoryDiscoveryEntries)
}

func discoverGitRoots(ctx context.Context, root string, maxDepth, maxEntries int) (GitRootDiscoveryResult, error) {
	result := GitRootDiscoveryResult{
		Roots:    []string{},
		Warnings: []string{},
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	canonical, err := canonicalDirectory(root)
	if err != nil {
		return result, err
	}

	state := repositoryDiscoveryState{
		result:     &result,
		maxEntries: maxEntries,
		warnings:   make(map[string]struct{}),
	}
	var visit func(string, int) error
	visit = func(directory string, depth int) error {
		if err := ctx.Err(); err != nil {
			result.Partial = true
			return err
		}
		if depth > maxDepth {
			state.warn(repositoryDiscoveryDepthLimitWarning)
			return nil
		}
		if !state.isSafeDirectory(directory) {
			state.warn(repositoryDiscoveryPathWarning)
			return nil
		}
		if hasGitMarker(directory) {
			result.Roots = append(result.Roots, directory)
			return nil
		}
		directoryFile, readErr := os.Open(directory)
		if readErr != nil {
			if depth == 0 {
				return errors.New("repository discovery root could not be read")
			}
			state.warn(repositoryDiscoveryReadWarning)
			return nil
		}
		items, truncated, readErr := readRepositoryDiscoveryEntries(ctx, directoryFile, state.maxEntries-state.entries)
		if err := ctx.Err(); err != nil {
			result.Partial = true
			return err
		}
		if readErr != nil {
			if depth == 0 {
				return errors.New("repository discovery root could not be read")
			}
			state.warn(repositoryDiscoveryReadWarning)
			return nil
		}
		if truncated {
			state.warn(repositoryDiscoveryEntryLimitWarning)
		}
		for _, item := range items {
			if err := ctx.Err(); err != nil {
				result.Partial = true
				return err
			}
			if item.Name() == ".git" {
				result.Roots = append(result.Roots, directory)
				return nil
			}
			if state.entries >= state.maxEntries {
				state.entryLimitReached = true
				state.warn(repositoryDiscoveryEntryLimitWarning)
				return nil
			}
			state.entries++
			if !item.IsDir() || item.Type()&os.ModeSymlink != 0 || skippedDiscoveryDirectory(item.Name()) {
				continue
			}
			if err := visit(filepath.Join(directory, item.Name()), depth+1); err != nil {
				return err
			}
			if state.entryLimitReached {
				return nil
			}
		}
		if truncated {
			state.entryLimitReached = true
		}
		return nil
	}
	if err := visit(canonical, 0); err != nil {
		sort.Strings(result.Roots)
		return result, err
	}
	sort.Strings(result.Roots)
	return result, nil
}

type repositoryDiscoveryDirectory interface {
	ReadDir(int) ([]os.DirEntry, error)
	Close() error
}

// readRepositoryDiscoveryEntries owns and closes directory. A single lookahead
// entry distinguishes an exact budget from a truncated directory. Only the
// bounded inspected prefix is sorted; uninspected entries are never claimed.
func readRepositoryDiscoveryEntries(
	ctx context.Context,
	directory repositoryDiscoveryDirectory,
	limit int,
) (items []os.DirEntry, truncated bool, resultErr error) {
	defer func() {
		resultErr = errors.Join(resultErr, directory.Close())
	}()
	items = []os.DirEntry{}
	for {
		if err := ctx.Err(); err != nil {
			return items, false, err
		}
		// ReadDir(0) would read the whole directory, so always request at least
		// one entry, including when probing an exhausted budget for EOF.
		batchSize := min(repositoryDiscoveryReadBatch, max(1, limit-len(items)+1))
		batch, err := directory.ReadDir(batchSize)
		if contextErr := ctx.Err(); contextErr != nil {
			return items, false, contextErr
		}
		items = append(items, batch...)
		if err != nil && !errors.Is(err, io.EOF) {
			return items, false, err
		}
		if len(items) > limit {
			items = items[:max(0, limit)]
			truncated = true
			break
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name() < items[j].Name() })
	return items, truncated, nil
}

type repositoryDiscoveryState struct {
	result            *GitRootDiscoveryResult
	entries           int
	maxEntries        int
	entryLimitReached bool
	warnings          map[string]struct{}
}

func (s *repositoryDiscoveryState) isSafeDirectory(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0
}

func (s *repositoryDiscoveryState) warn(warning string) {
	if _, exists := s.warnings[warning]; exists {
		return
	}
	s.warnings[warning] = struct{}{}
	s.result.Partial = true
	s.result.Warnings = append(s.result.Warnings, warning)
}

func hasGitMarker(directory string) bool {
	_, err := os.Lstat(filepath.Join(directory, ".git"))
	return err == nil
}

func repositoryDiscoveryIncompleteError(warnings []string) string {
	if len(warnings) > 0 {
		return warnings[0]
	}
	return "repository discovery was incomplete"
}

func skippedDiscoveryDirectory(name string) bool {
	switch name {
	case ".git", ".worktrees", "node_modules", "vendor", "bin", "obj", "build", "dist", ".venv", ".tox":
		return true
	default:
		return false
	}
}
