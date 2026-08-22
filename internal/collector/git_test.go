package collector

import (
	"context"
	"errors"
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

type recordingGitRunner struct {
	registered string
	foreign    string
	calls      []string
	exit128    bool
}

func (r *recordingGitRunner) Run(_ context.Context, _ string, args []string, directory string) (CommandResult, error) {
	r.calls = append(r.calls, directory+"\x00"+strings.Join(args, " "))
	if r.exit128 && len(args) > 0 && args[0] == "for-each-ref" {
		return CommandResult{ExitCode: 128}, errors.New("fatal git fixture")
	}
	if len(args) >= 2 && args[0] == "rev-parse" {
		switch args[1] {
		case "--git-common-dir", "--git-dir":
			if directory == r.registered {
				return CommandResult{Stdout: filepath.Join(r.registered, ".git") + "\n"}, nil
			}
			return CommandResult{Stdout: filepath.Join(r.foreign, ".git") + "\n"}, nil
		case "--show-toplevel":
			return CommandResult{Stdout: directory + "\n"}, nil
		}
	}
	if len(args) > 0 && args[0] == "symbolic-ref" {
		return CommandResult{Stdout: "main\n"}, nil
	}
	if len(args) > 0 && args[0] == "rev-parse" {
		return CommandResult{Stdout: "abc\n"}, nil
	}
	if len(args) > 0 && args[0] == "status" {
		return CommandResult{}, nil
	}
	if len(args) > 0 && args[0] == "for-each-ref" {
		return CommandResult{}, nil
	}
	return CommandResult{}, nil
}

func TestWorktreeForeignCommonDirectoryDoesNotReadState(t *testing.T) {
	registered, foreign := t.TempDir(), t.TempDir()
	for _, path := range []string{filepath.Join(registered, ".git"), filepath.Join(foreign, ".git")} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	runner := &recordingGitRunner{registered: registered, foreign: foreign}
	items, complete := NewGitCollector(runner).WorktreeDetails(context.Background(), registered, []Worktree{{Path: foreign, Trust: "unverified"}})
	if complete || len(items) != 1 || items[0].Trust != "unverified" || items[0].Error == "" {
		t.Fatalf("foreign worktree was trusted: %#v complete=%v", items, complete)
	}
	for _, call := range runner.calls {
		if strings.HasPrefix(call, foreign+"\x00status ") || strings.HasPrefix(call, foreign+"\x00symbolic-ref ") || strings.HasPrefix(call, foreign+"\x00for-each-ref ") {
			t.Fatalf("foreign path received post-association state read: %q", call)
		}
	}
}

func TestWorktreeExit128MakesDetailsIncomplete(t *testing.T) {
	repository := t.TempDir()
	if err := os.Mkdir(filepath.Join(repository, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &recordingGitRunner{registered: repository, foreign: repository, exit128: true}
	items, complete := NewGitCollector(runner).WorktreeDetails(context.Background(), repository, []Worktree{{Path: repository, Trust: "unverified"}})
	if complete || len(items) != 1 || items[0].Trust != "unverified" || items[0].Error == "" {
		t.Fatalf("exit 128 was accepted: %#v complete=%v", items, complete)
	}
}

func TestRegisteredLinkedWorktreeIsPrimary(t *testing.T) {
	repository := t.TempDir()
	gitTest(t, repository, "init", "--initial-branch=main")
	gitTest(t, repository, "config", "user.email", "test@example.invalid")
	gitTest(t, repository, "config", "user.name", "Dev Room Test")
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, repository, "add", "README.md")
	gitTest(t, repository, "commit", "-m", "fixture")
	linked := filepath.Join(t.TempDir(), "registered-linked")
	gitTest(t, repository, "worktree", "add", "-b", "linked", linked)
	collector := NewGitCollector(nil)
	state, err := collector.Collect(context.Background(), linked)
	if err != nil {
		t.Fatal(err)
	}
	items, complete := collector.WorktreeDetails(context.Background(), linked, state.Worktrees)
	if !complete {
		t.Fatalf("linked collection incomplete: %#v", items)
	}
	var primary int
	for _, item := range items {
		if item.Primary {
			primary++
			if item.ID != "primary" || item.Path != linked {
				t.Fatalf("registered linked checkout is not primary: %#v", item)
			}
		}
	}
	if primary != 1 {
		t.Fatalf("primary count = %d, want 1", primary)
	}
}

type prunableNoReadRunner struct {
	readPath string
	reads    int
}

func (r *prunableNoReadRunner) Run(_ context.Context, _ string, _ []string, directory string) (CommandResult, error) {
	if directory == r.readPath {
		r.reads++
	}
	return CommandResult{}, errors.New("unexpected git read")
}
func TestPrunableWorktreeIsNeverRead(t *testing.T) {
	path := t.TempDir()
	r := &prunableNoReadRunner{readPath: path}
	items, complete := NewGitCollector(r).WorktreeDetails(context.Background(), t.TempDir(), []Worktree{{Path: path, Prunable: true}})
	if complete || r.reads != 0 || len(items) != 1 || items[0].Trust != "unverified" {
		t.Fatalf("prunable path was read or trusted: %#v reads=%d complete=%v", items, r.reads, complete)
	}
}
