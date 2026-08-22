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
