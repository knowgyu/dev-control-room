package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
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

func TestApprovedCleanupRemovesOnlyExactCleanLinkedWorktreeAndBranch(t *testing.T) {
	home := t.TempDir()
	repository := tempGitRepository(t, "cleanup-safe")
	linked := filepath.Join(t.TempDir(), "linked")
	gitFixture(t, repository, "worktree", "add", "-b", "cleanup-linked", linked)
	gitFixture(t, repository, "remote", "add", "origin", "https://github.com/sample-owner/sample-repository.git")
	head := exec.Command("git", "rev-parse", "HEAD")
	head.Dir = linked
	commit, err := head.Output()
	if err != nil {
		t.Fatal(err)
	}
	gitFixture(t, repository, "update-ref", "refs/remotes/origin/cleanup-linked", strings.TrimSpace(string(commit)))
	gitFixture(t, linked, "branch", "--set-upstream-to=origin/cleanup-linked", "cleanup-linked")
	github := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !strings.Contains(request.URL.Path, "/repos/sample-owner/sample-repository/commits/") || !strings.HasSuffix(request.URL.Path, "/pulls") {
			t.Errorf("GitHub merge path = %s", request.URL.Path)
		}
		_, _ = response.Write([]byte(`[{"number":7,"merged_at":"2026-08-25T00:00:00Z"}]`))
	}))
	defer github.Close()
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Cleanup", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddIntegration(context.Background(), AddIntegrationInput{ID: "github", Name: "Fixture GitHub", Kind: IntegrationGitHub, Endpoint: github.URL, Values: map[string]string{"owner": "sample-owner", "repository": "sample-repository"}}); err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	candidates, err := service.CleanupCandidates(context.Background(), project.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	var candidate domain.CleanupCandidate
	for _, item := range candidates {
		if item.Spec.WorktreeID != "primary" {
			candidate = item
		}
	}
	if candidate.Metadata.ID == "" || candidate.Spec.Decision != domain.CleanupReviewable {
		t.Fatalf("safe cleanup candidate = %#v", candidate)
	}
	plan, err := service.PlanCleanup(context.Background(), CleanupPlanInput{CandidateID: candidate.Metadata.ID, ProjectID: project.Metadata.ID, RepositoryID: candidate.Spec.RepositoryID, WorktreeID: candidate.Spec.WorktreeID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.TrustActionWorktree(context.Background(), plan.ActionPlan.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	digest, _ := plan.ActionPlan.Digest()
	now := time.Now().UTC()
	expires := now.Add(time.Hour)
	approval := domain.Approval{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ApprovalKind}, Metadata: domain.ObjectMeta{ID: "cleanup-approval", Name: "Fixture approval"}, Spec: domain.ApprovalSpec{ActionPlanID: plan.ActionPlan.Metadata.ID, ActionPlanDigest: digest, Status: domain.ApprovalGranted, RequestedBy: plan.ActionPlan.Spec.RequestedBy, ApprovedBy: &domain.Actor{Kind: domain.ActorHuman, ID: "local-user"}, ExpiresAt: &expires, DecidedAt: now}}
	if err := service.store.SaveApprovalAndActionEvent(context.Background(), approval, externalActionEvent(plan.ActionPlan, "approval_granted", "local-user", now, "approval"), now); err != nil {
		t.Fatal(err)
	}
	result, err := service.ExecuteCleanup(context.Background(), plan.ActionPlan.Metadata.ID, "fixture", "cleanup-run-1")
	if err != nil || result.Status != "succeeded" {
		t.Fatalf("cleanup result = %#v, err = %v", result, err)
	}
	if _, err := os.Stat(linked); !os.IsNotExist(err) {
		t.Fatalf("linked Worktree still exists: %v", err)
	}
	branch := exec.Command("git", "branch", "--list", "cleanup-linked")
	branch.Dir = repository
	if output, err := branch.Output(); err != nil || strings.TrimSpace(string(output)) != "" {
		t.Fatalf("cleanup branch remains: %q, %v", output, err)
	}
	if _, err := service.ExecuteCleanup(context.Background(), plan.ActionPlan.Metadata.ID, "fixture", "cleanup-run-2"); err == nil {
		t.Fatal("repeated cleanup unexpectedly succeeded")
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
