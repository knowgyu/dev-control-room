package collector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverGitRootsFindsNestedRootsAndSkipsInternals(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "apps", "first")
	second := filepath.Join(root, "apps", "second")
	for _, path := range []string{filepath.Join(first, ".git"), filepath.Join(second, ".git"), filepath.Join(root, "node_modules", "ignored", ".git")} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	wantFirst := canonicalTestDirectory(t, first)
	wantSecond := canonicalTestDirectory(t, second)
	got, err := DiscoverGitRoots(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{wantFirst, wantSecond}
	if len(got) != len(want) || got[0] != wantFirst || got[1] != wantSecond {
		t.Fatalf("roots = %#v, want %#v", got, want)
	}
}

func TestDiscoverGitRootsTreatsGitFileAsWorktreeRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: C:/fixture/.git/worktrees/one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	wantRoot := canonicalTestDirectory(t, root)
	got, err := DiscoverGitRoots(root)
	if err != nil || len(got) != 1 || got[0] != wantRoot {
		t.Fatalf("roots = %#v, err = %v", got, err)
	}
}

func TestDiscoverGitRootsDetailedRecognizesDirectRootBeforeEntryLimit(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: C:/fixture/.git/worktrees/one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b", "c"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}

	result, err := discoverGitRoots(context.Background(), root, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	wantRoot := canonicalTestDirectory(t, root)
	if len(result.Roots) != 1 || result.Roots[0] != wantRoot {
		t.Fatalf("roots = %#v, want [%q]", result.Roots, wantRoot)
	}
	if result.Partial || len(result.Warnings) != 0 {
		t.Fatalf("direct root result = %#v, want complete result", result)
	}
}

func TestDiscoverGitRootsDetailedRetainsRootsWhenEntryLimitReached(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "a-first")
	second := filepath.Join(root, "b-second")
	for _, path := range []string{filepath.Join(first, ".git"), filepath.Join(second, ".git"), filepath.Join(root, "c-third"), filepath.Join(root, "d-fourth")} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	result, err := discoverGitRoots(context.Background(), root, maxRepositoryDiscoveryDepth, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{canonicalTestDirectory(t, first), canonicalTestDirectory(t, second)}
	if len(result.Roots) != len(want) || result.Roots[0] != want[0] || result.Roots[1] != want[1] {
		t.Fatalf("roots = %#v, want %#v", result.Roots, want)
	}
	if !result.Partial || !hasRepositoryDiscoveryWarning(result.Warnings, repositoryDiscoveryEntryLimitWarning) {
		t.Fatalf("bounded result = %#v, want partial entry-limit warning", result)
	}
}

func TestDiscoverGitRootsDetailedContinuesAfterDepthLimitedSubtree(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a-deep")
	current := deep
	for depth := 1; depth <= maxRepositoryDiscoveryDepth+1; depth++ {
		if err := os.Mkdir(current, 0o700); err != nil {
			t.Fatal(err)
		}
		current = filepath.Join(current, fmt.Sprintf("level-%02d", depth))
	}
	second := filepath.Join(root, "b-repository")
	if err := os.MkdirAll(filepath.Join(second, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}

	result, err := discoverGitRoots(context.Background(), root, maxRepositoryDiscoveryDepth, maxRepositoryDiscoveryEntries)
	if err != nil {
		t.Fatal(err)
	}
	wantSecond := canonicalTestDirectory(t, second)
	if len(result.Roots) != 1 || result.Roots[0] != wantSecond {
		t.Fatalf("roots = %#v, want [%q]", result.Roots, wantSecond)
	}
	if !result.Partial || !hasRepositoryDiscoveryWarning(result.Warnings, repositoryDiscoveryDepthLimitWarning) {
		t.Fatalf("depth-limited result = %#v, want partial depth warning", result)
	}
}

func TestDiscoverGitRootsDetailedSkipsBoundedDirectories(t *testing.T) {
	root := t.TempDir()
	visible := filepath.Join(root, "visible")
	if err := os.MkdirAll(filepath.Join(visible, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".worktrees", "node_modules", "vendor", "bin", "obj", "build", "dist", ".venv", ".tox"} {
		if err := os.MkdirAll(filepath.Join(root, name, "hidden", ".git"), 0o700); err != nil {
			t.Fatal(err)
		}
	}

	result, err := DiscoverGitRootsDetailed(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	want := canonicalTestDirectory(t, visible)
	if len(result.Roots) != 1 || result.Roots[0] != want {
		t.Fatalf("roots = %#v, want [%q]", result.Roots, want)
	}
	if result.Partial || len(result.Warnings) != 0 {
		t.Fatalf("skipped-directory result = %#v, want complete result", result)
	}
}

func TestDiscoverGitRootsDetailedHonorsCancellation(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := DiscoverGitRootsDetailed(ctx, root)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(result.Roots) != 0 || result.Partial {
		t.Fatalf("cancelled result = %#v, want empty non-partial result", result)
	}
}

func TestDiscoverGitRootsDetailedDoesNotFollowSymlinkDirectories(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("directory symlink is unavailable: %v", err)
	}

	result, err := DiscoverGitRootsDetailed(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Roots) != 0 {
		t.Fatalf("roots = %#v, want no roots through symlink", result.Roots)
	}
}

func TestDiscoverGitRootsDetailedDistinguishesExactEntryBudget(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("file-%d", i)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name    string
		limit   int
		partial bool
	}{
		{name: "exact budget", limit: 5},
		{name: "one unread entry", limit: 4, partial: true},
		{name: "zero budget", limit: 0, partial: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := discoverGitRoots(context.Background(), root, maxRepositoryDiscoveryDepth, tc.limit)
			if err != nil {
				t.Fatal(err)
			}
			if result.Partial != tc.partial || len(result.Roots) != 0 {
				t.Fatalf("entry-boundary result = %#v, want partial %v", result, tc.partial)
			}
			if hasRepositoryDiscoveryWarning(result.Warnings, repositoryDiscoveryEntryLimitWarning) != tc.partial {
				t.Fatalf("entry-boundary warnings = %v", result.Warnings)
			}
		})
	}
}

func TestReadRepositoryDiscoveryEntriesBoundsBatchesAndCloses(t *testing.T) {
	root := t.TempDir()
	const entryCount = repositoryDiscoveryReadBatch*2 + 3
	for i := entryCount - 1; i >= 0; i-- {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("file-%04d", i)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name      string
		limit     int
		truncated bool
	}{
		{name: "multiple batches and EOF", limit: entryCount},
		{name: "one lookahead only", limit: repositoryDiscoveryReadBatch + 1, truncated: true},
		{name: "exhausted budget uses positive read", limit: 0, truncated: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := os.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			directory := &trackedRepositoryDiscoveryDirectory{File: file}
			entries, truncated, err := readRepositoryDiscoveryEntries(context.Background(), directory, tc.limit)
			if err != nil || truncated != tc.truncated || len(entries) != min(tc.limit, entryCount) {
				t.Fatalf("bounded entries = %d, truncated %v, err %v", len(entries), truncated, err)
			}
			if directory.readEntries > tc.limit+1 {
				t.Fatalf("read %d entries for budget %d", directory.readEntries, tc.limit)
			}
			for _, size := range directory.requests {
				if size <= 0 || size > repositoryDiscoveryReadBatch {
					t.Fatalf("unbounded ReadDir request: %d", size)
				}
			}
			for i := 1; i < len(entries); i++ {
				if entries[i-1].Name() > entries[i].Name() {
					t.Fatal("inspected directory entries are not sorted")
				}
			}
			if directory.closes != 1 {
				t.Fatalf("directory closed %d times, want once", directory.closes)
			}
			if file.Fd() != ^uintptr(0) {
				t.Fatal("directory remains open after bounded read")
			}
		})
	}
}

func TestReadRepositoryDiscoveryEntriesHonorsCancellationAndCloseErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	closeErr := errors.New("fixture close failure")
	for _, tc := range []struct {
		name            string
		cancelAfterRead bool
		closeErr        error
		want            error
	}{
		{name: "cancel between batches", cancelAfterRead: true, want: context.Canceled},
		{name: "close failure", closeErr: closeErr, want: closeErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			file, err := os.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			directory := &trackedRepositoryDiscoveryDirectory{File: file, closeErr: tc.closeErr}
			if tc.cancelAfterRead {
				directory.afterRead = cancel
			}
			_, _, err = readRepositoryDiscoveryEntries(ctx, directory, repositoryDiscoveryReadBatch*2)
			if !errors.Is(err, tc.want) || directory.closes != 1 {
				t.Fatalf("read error = %v, close count = %d", err, directory.closes)
			}
			if tc.cancelAfterRead && len(directory.requests) != 1 {
				t.Fatalf("continued reading after cancellation: %v", directory.requests)
			}
		})
	}
}

type trackedRepositoryDiscoveryDirectory struct {
	*os.File
	requests    []int
	readEntries int
	closes      int
	afterRead   func()
	closeErr    error
}

func (d *trackedRepositoryDiscoveryDirectory) ReadDir(n int) ([]os.DirEntry, error) {
	d.requests = append(d.requests, n)
	entries, err := d.File.ReadDir(n)
	d.readEntries += len(entries)
	if d.afterRead != nil {
		d.afterRead()
	}
	return entries, err
}

func (d *trackedRepositoryDiscoveryDirectory) Close() error {
	d.closes++
	return errors.Join(d.File.Close(), d.closeErr)
}

func hasRepositoryDiscoveryWarning(warnings []string, want string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, want) {
			return true
		}
	}
	return false
}
