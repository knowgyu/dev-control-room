package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/assurance"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestTrustedGitHubCLIPathRejectsSymlinkAndRequiresRegularExecutable(t *testing.T) {
	directory := t.TempDir()
	regular := filepath.Join(directory, "gh.exe")
	if err := os.WriteFile(regular, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolved, err := trustedGitHubCLIPathWith(func(name string) (string, error) {
		if name != "gh.exe" {
			t.Fatalf("lookup name = %q", name)
		}
		return regular, nil
	}, os.Lstat)
	if err != nil || resolved != regular {
		t.Fatalf("regular gh resolution = %q, %v", resolved, err)
	}
	if _, err := trustedGitHubCLIPathWith(func(string) (string, error) { return directory, nil }, os.Lstat); err == nil {
		t.Fatal("directory became a trusted gh executable")
	}
	link := filepath.Join(t.TempDir(), "gh.exe")
	if err := os.Symlink(regular, link); err != nil {
		t.Skipf("native symlink creation unavailable: %v", err)
	}
	if _, err := trustedGitHubCLIPathWith(func(string) (string, error) { return link, nil }, os.Lstat); err == nil {
		t.Fatal("symlinked gh executable became trusted")
	}
}

func TestPRCIBaselineUsesAuthoritativeGitHubContextsWithoutPersistingRawProviderData(t *testing.T) {
	repository := tempGitRepository(t, "github-baseline")
	gitFixture(t, repository, "remote", "add", "origin", "https://github.com/sample-owner/sample-repository.git")
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "GitHub baseline", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	service.githubBaselinePath = func() (string, error) { return "C:\\tools\\gh.exe", nil }
	var invocations []assurance.GitHubBaselineInvocation
	service.githubBaselineExecutor = func(_ context.Context, invocation assurance.GitHubBaselineInvocation, directory string) (assurance.GitHubBaselineResponse, error) {
		if directory != repository {
			t.Fatalf("baseline worktree = %q, want %q", directory, repository)
		}
		invocations = append(invocations, invocation)
		switch invocation.Arguments[len(invocation.Arguments)-1] {
		case "repos/sample-owner/sample-repository/branches/main":
			return assurance.GitHubBaselineResponse{StatusCode: 200, Body: []byte(`{"protected":true,"protection":{"enabled":true,"required_status_checks":{"contexts":["ci/build"]}},"node_id":"must-not-persist"}`)}, nil
		case "repos/sample-owner/sample-repository/rules/branches/main":
			return assurance.GitHubBaselineResponse{StatusCode: 200, Body: []byte(`{"rules":[{"type":"required_status_checks","parameters":{"required_status_checks":[{"context":"ci/test"},{"context":"ci/build"}]}}],"url":"must-not-persist"}`)}, nil
		default:
			t.Fatalf("unexpected GitHub baseline argv: %#v", invocation)
			return assurance.GitHubBaselineResponse{}, nil
		}
	}

	baseline, err := service.CreatePRCIBaseline(context.Background(), BaselineInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", TargetBranch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if len(invocations) != 2 {
		t.Fatalf("GitHub baseline invocations = %#v", invocations)
	}
	entries := map[string]domain.BaselineEntry{}
	for _, entry := range baseline.Spec.Entries {
		entries[entry.Name] = entry
	}
	for _, context := range []string{"ci/build", "ci/test"} {
		entry, found := entries[context]
		if !found || entry.Classification != domain.BaselineRequired || !entry.Required || entry.SourcePath != "github.authoritative" {
			t.Fatalf("authoritative required context %q = %#v", context, entry)
		}
	}
	if _, found := entries["provider-enforced PR rules"]; found {
		t.Fatalf("available GitHub evidence must replace unknown provider entry: %#v", baseline.Spec.Entries)
	}
	if !reflect.DeepEqual(baseline.Spec.Sources, []string{"github.branch_protection", "github.branch_rules"}) {
		t.Fatalf("baseline sources = %#v", baseline.Spec.Sources)
	}
	encoded, marshalErr := json.Marshal(baseline)
	if marshalErr != nil || strings.Contains(string(encoded), "must-not-persist") || strings.Contains(string(encoded), "gh.exe") {
		t.Fatalf("baseline persisted raw provider data or command: %s (%v)", encoded, marshalErr)
	}
}

func TestPRCIBaselineKeepsProviderRulesUnknownWhenGitHubLookupFails(t *testing.T) {
	repository := tempGitRepository(t, "github-baseline-unavailable")
	gitFixture(t, repository, "remote", "add", "origin", "https://github.com/sample-owner/sample-repository.git")
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "GitHub unavailable", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	service.githubBaselinePath = func() (string, error) { return "C:\\tools\\gh.exe", nil }
	calls := 0
	service.githubBaselineExecutor = func(context.Context, assurance.GitHubBaselineInvocation, string) (assurance.GitHubBaselineResponse, error) {
		calls++
		return assurance.GitHubBaselineResponse{}, errors.New("provider token must-not-persist")
	}

	baseline, err := service.CreatePRCIBaseline(context.Background(), BaselineInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", TargetBranch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("GitHub lookup calls = %d, want 2", calls)
	}
	unknown := false
	for _, entry := range baseline.Spec.Entries {
		if entry.ID == "provider-rules-unknown" && entry.Classification == domain.BaselineUnknown && entry.Observed {
			unknown = true
		}
		if entry.Required && entry.SourcePath == "github.authoritative" {
			t.Fatalf("unavailable provider source became a required context: %#v", entry)
		}
	}
	if !unknown {
		t.Fatalf("unavailable provider rules were not kept explicit: %#v", baseline.Spec.Entries)
	}
	encoded, marshalErr := json.Marshal(baseline)
	if marshalErr != nil || strings.Contains(string(encoded), "must-not-persist") {
		t.Fatalf("baseline leaked provider error: %s (%v)", encoded, marshalErr)
	}
}
