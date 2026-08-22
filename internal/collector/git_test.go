package collector

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeRemoteRemovesCredentialsAndDetectsProvider(t *testing.T) {
	tests := []struct {
		name, raw, wantURL, wantHost, wantProvider string
	}{
		{"https", "https://alice:secret@example.com/org/repo.git?x=1", "https://example.com/org/repo", "example.com", ProviderUnknown},
		{"ssh", "ssh://git@github.com/org/repo.git", "https://github.com/org/repo", "github.com", ProviderGitHub},
		{"scp", "git@gitlab.com:org/repo.git", "https://gitlab.com/org/repo", "gitlab.com", ProviderGitLab},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			url, host, provider, capabilities := NormalizeRemote(test.raw)
			if url != test.wantURL || host != test.wantHost || provider != test.wantProvider {
				t.Fatalf("NormalizeRemote() = %q, %q, %q; want %q, %q, %q", url, host, provider, test.wantURL, test.wantHost, test.wantProvider)
			}
			if strings.Contains(url, "secret") || len(capabilities) != 1 || capabilities[0] != CapabilityRemoteRead {
				t.Fatalf("remote credentials or capability contract leaked: %q %#v", url, capabilities)
			}
		})
	}
}

func TestGitCollectorUsesTemporaryRepositoryAndFindsDirtyDetachedState(t *testing.T) {
	repository := t.TempDir()
	gitTest(t, repository, "init", "--initial-branch=main")
	gitTest(t, repository, "config", "user.email", "test@example.invalid")
	gitTest(t, repository, "config", "user.name", "Dev Room Test")
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repository, "add", "README.md")
	gitTest(t, repository, "commit", "-m", "fixture")
	gitTest(t, repository, "remote", "add", "origin", "git@github.com:fixture/repo.git")

	collector := NewGitCollector(nil)
	state, err := collector.Collect(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	if state.Dirty || state.Detached || len(state.Remotes) != 1 || state.Remotes[0].Provider != ProviderGitHub {
		t.Fatalf("unexpected clean state: %#v", state)
	}
	if err := os.WriteFile(filepath.Join(repository, "dirty.txt"), []byte("uncommitted\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err = collector.Collect(context.Background(), repository)
	if err != nil || !state.Dirty || !state.UnsafeCleanup {
		t.Fatalf("dirty state was not deterministic: %#v, %v", state, err)
	}
	gitTest(t, repository, "checkout", "--detach", "HEAD")
	state, err = collector.Collect(context.Background(), repository)
	if err != nil || !state.Detached || state.Branch != "" {
		t.Fatalf("detached state was not detected: %#v, %v", state, err)
	}
}

func TestProcessRunnerRejectsGenericExecutable(t *testing.T) {
	_, err := (ProcessRunner{}).Run(context.Background(), "sh", nil, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "only the git executable") {
		t.Fatalf("generic process execution was not rejected: %v", err)
	}
}

func TestGitCollectorRejectsRegisteredSubdirectoryBeforeReadingRepositoryStatus(t *testing.T) {
	repository := t.TempDir()
	gitTest(t, repository, "init", "--initial-branch=main")
	subdirectory := filepath.Join(repository, "nested")
	if err := os.Mkdir(subdirectory, 0o700); err != nil {
		t.Fatal(err)
	}

	state, err := NewGitCollector(nil).Collect(context.Background(), subdirectory)
	if err == nil || state.Error != "registered path is not the Git worktree root" {
		t.Fatalf("registered subdirectory crossed the repository boundary: %#v, %v", state, err)
	}
}

func gitTest(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, output)
	}
}

func TestWorktreeDetailsReadsNULPorcelainAndRealLinkedState(t *testing.T) {
	repository := t.TempDir()
	gitTest(t, repository, "init", "--initial-branch=main")
	gitTest(t, repository, "config", "user.email", "test@example.invalid")
	gitTest(t, repository, "config", "user.name", "Dev Room Test")
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repository, "add", "README.md")
	gitTest(t, repository, "commit", "-m", "fixture")
	linked := filepath.Join(t.TempDir(), "linked with space\nand newline")
	gitTest(t, repository, "worktree", "add", "-b", "linked", linked)
	if err := os.WriteFile(filepath.Join(linked, "untracked"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	collector := NewGitCollector(nil)
	state, err := collector.Collect(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	items, complete := collector.WorktreeDetails(context.Background(), repository, state.Worktrees)
	if !complete || len(items) != 2 {
		t.Fatalf("worktree enumeration = %#v complete=%v", items, complete)
	}
	var primary, linkedState *Worktree
	for i := range items {
		if items[i].Primary {
			primary = &items[i]
		} else {
			linkedState = &items[i]
		}
	}
	if primary == nil || primary.ID != "primary" || primary.Trust != "verified_read_only" {
		t.Fatalf("primary identity = %#v", primary)
	}
	if linkedState == nil || !linkedState.Untracked || !linkedState.Dirty || linkedState.ID == "" || linkedState.AssociationFingerprint == "" {
		t.Fatalf("linked state = %#v", linkedState)
	}
	linkedID := linkedState.ID
	moved := filepath.Join(t.TempDir(), "moved")
	gitTest(t, repository, "worktree", "move", linked, moved)
	state, err = collector.Collect(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	items, complete = collector.WorktreeDetails(context.Background(), repository, state.Worktrees)
	if !complete {
		t.Fatal("moved worktree did not enumerate")
	}
	for _, item := range items {
		if !item.Primary && item.ID != linkedID {
			t.Fatalf("linked id changed after move: %s want %s", item.ID, linkedID)
		}
	}
}

func TestParseWorktreesPreservesNewlinePathAndFlags(t *testing.T) {
	items := parseWorktrees("worktree /tmp/a space\nand newline\x00HEAD abc\x00detached\x00locked reason\x00prunable gone\x00unknown field\x00\x00bare\x00")
	if len(items) != 1 || items[0].Path != "/tmp/a space\nand newline" || !items[0].Locked || !items[0].Prunable {
		t.Fatalf("NUL porcelain parsing lost boundaries: %#v", items)
	}
}

type failedWorktreeListRunner struct{}

func (failedWorktreeListRunner) Run(_ context.Context, _ string, args []string, _ string) (CommandResult, error) {
	if len(args) > 1 && args[0] == "worktree" {
		return CommandResult{ExitCode: 1}, nil
	}
	return CommandResult{}, nil
}
func TestWorktreeListNonzeroWithoutRunnerErrorIsIncomplete(t *testing.T) {
	if _, err := NewGitCollector(failedWorktreeListRunner{}).worktrees(context.Background(), t.TempDir()); err == nil {
		t.Fatal("nonzero worktree list exit was accepted")
	}
}
