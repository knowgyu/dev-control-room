package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitHubMergedCommitLookupReturnsEvidenceWithoutBodyLeak(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/sample-owner/sample-repository/commits/abc123/pulls" {
			t.Errorf("GitHub lookup path = %s", request.URL.Path)
		}
		_, _ = response.Write([]byte(`[{"number":12,"merged_at":"2026-08-25T00:00:00Z"}]`))
	}))
	defer server.Close()
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.AddIntegration(context.Background(), AddIntegrationInput{ID: "github", Name: "Fixture GitHub", Kind: IntegrationGitHub, Endpoint: server.URL, Values: map[string]string{"owner": "sample-owner", "repository": "sample-repository"}}); err != nil {
		t.Fatal(err)
	}
	merged, evidence, err := service.githubCommitMerged(context.Background(), "https://github.com/sample-owner/sample-repository.git", "abc123")
	if err != nil || !merged || !strings.Contains(evidence, "#12") || bytes.Contains([]byte(evidence), []byte("merged_at")) {
		t.Fatalf("merged evidence = %t, %q, %v", merged, evidence, err)
	}
}

func TestCleanupCandidatesRemainBlockedAndExplainWorktreeState(t *testing.T) {
	home := t.TempDir()
	repository := tempGitRepository(t, "cleanup")
	linked := t.TempDir() + "/linked"
	gitFixture(t, repository, "worktree", "add", "-b", "cleanup-linked", linked)
	if err := os.WriteFile(filepath.Join(linked, "untracked.txt"), []byte("pending"), 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Cleanup", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	items, err := service.CleanupCandidates(context.Background(), project.Metadata.ID)
	if err != nil || len(items) != 2 {
		t.Fatalf("cleanup candidates = %#v, %v", items, err)
	}
	var primary, linkedCandidate bool
	for _, item := range items {
		if item.Spec.Decision != "blocked" || len(item.Spec.Reasons) == 0 {
			t.Fatalf("unsafe cleanup decision: %#v", item)
		}
		if item.Spec.WorktreeID == "primary" {
			primary = strings.Contains(strings.Join(item.Spec.Reasons, " "), "primary")
		} else {
			linkedCandidate = strings.Contains(strings.Join(item.Spec.Reasons, " "), "untracked")
		}
	}
	if !primary || !linkedCandidate {
		t.Fatalf("missing blocking reasons: %#v", items)
	}
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/cleanup/candidates?project_id="+project.Metadata.ID, nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"blocked"`) {
		t.Fatalf("cleanup HTTP surface = %d %s", recorder.Code, recorder.Body.String())
	}
}
