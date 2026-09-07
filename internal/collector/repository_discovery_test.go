package collector

import (
	"os"
	"path/filepath"
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
