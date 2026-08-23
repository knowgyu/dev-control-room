package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverReturnsOnlyBoundedDeterministicEntryPoints(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"scripts":{"test":"go test ./...","lint":"golangci-lint run"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	workflow := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(workflow, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workflow, "ci.yml"), []byte("run: outside-step\njobs:\n  test:\n    run: also-outside-step\n    steps:\n      - run: npm test\n      - name: |\n          - run: ignored-name-scalar-text\n        env:\n          NOTE: |\n            - run: ignored-env-scalar-text\n      - run: npm test # comment\n      - run: |\n          npm run lint\n          - run: ignored-scalar-text\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("items = %#v", items)
	}
	commands := map[string]bool{}
	for _, item := range items {
		commands[item.Command] = true
		if item.SourceDigest == "" || item.SourcePath == "" {
			t.Fatalf("proposal lacks source evidence: %#v", item)
		}
	}
	for _, command := range []string{"npm run lint", "npm run test", "npm test"} {
		if !commands[command] {
			t.Fatalf("missing %q from %#v", command, items)
		}
	}
}

func TestDiscoverRejectsSymlinkSourcesOutsideSelectedWorktree(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "package.json"), []byte(`{"scripts":{"outside":"echo outside"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Symlink(filepath.Join(outside, "package.json"), filepath.Join(root, "package.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := Discover(root); err == nil {
		t.Fatal("package symlink outside selected worktree was accepted")
	}

	root = t.TempDir()
	workflow := filepath.Join(outside, "workflows")
	if err := os.MkdirAll(workflow, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workflow, "ci.yml"), []byte("- run: npm test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".github"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(workflow, filepath.Join(root, ".github", "workflows")); err != nil {
		t.Fatal(err)
	}
	if _, err := Discover(root); err == nil {
		t.Fatal("workflow symlink outside selected worktree was accepted")
	}
}

func TestDiscoverSkipsMalformedPackageManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err := Discover(root)
	if err != nil || len(items) != 0 {
		t.Fatalf("malformed manifest = %#v, %v", items, err)
	}
}
