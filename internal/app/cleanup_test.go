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

func TestStartupAssuranceJanitorRemovesCrashOrphansAndPreservesReferences(t *testing.T) {
	if !assuranceCleanupRootSupported() {
		t.Skip("assurance cleanup requires directory-handle support")
	}

	home := t.TempDir()
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if service != nil {
			_ = service.Close()
		}
	}()
	repository := tempGitRepository(t, "assurance-janitor")
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Assurance janitor", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateAssuranceSession(context.Background(), AssuranceSessionInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary"})
	if err != nil {
		t.Fatal(err)
	}
	referencedArtifact, err := service.SaveAssuranceArtifact(context.Background(), ArtifactInput{SourceType: "janitor_test", SourceID: "referenced", Name: "referenced.txt", MIME: "text/plain", Content: []byte("keep this artifact")})
	if err != nil {
		t.Fatal(err)
	}
	referencedProposal, err := service.CreateAssuranceProposal(context.Background(), AssuranceProposalInput{SessionID: session.Metadata.ID, Purpose: "keep this proposal", Patch: "diff --git a/keep.go b/keep.go\n+keep\n"})
	if err != nil {
		t.Fatal(err)
	}
	artifactDirectory := filepath.Join(home, "artifacts", "assurance")
	for name, content := range map[string]string{
		"orphan-after-rename.json": "orphan",
		".artifact-write-crash":    "temporary write",
		".artifact-restore-crash":  "temporary restore",
	} {
		if err := os.WriteFile(filepath.Join(artifactDirectory, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	orphanProposalDirectory := filepath.Join(home, "assurance", "proposals", "orphan-after-mkdir")
	if err := os.MkdirAll(orphanProposalDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orphanProposalDirectory, "crash-marker"), []byte("orphan"), 0o600); err != nil {
		t.Fatal(err)
	}
	outsideDirectory := t.TempDir()
	outsideFile := filepath.Join(outsideDirectory, "must-survive.txt")
	if err := os.WriteFile(outsideFile, []byte("outside target"), 0o600); err != nil {
		t.Fatal(err)
	}
	orphanArtifactLink := filepath.Join(artifactDirectory, "orphan-link")
	orphanProposalLink := filepath.Join(home, "assurance", "proposals", "orphan-link")
	artifactLinkCreated := os.Symlink(outsideFile, orphanArtifactLink) == nil
	proposalLinkCreated := os.Symlink(outsideDirectory, orphanProposalLink) == nil

	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	service = nil
	restarted, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()

	for _, name := range []string{"orphan-after-rename.json", ".artifact-write-crash", ".artifact-restore-crash"} {
		if _, err := os.Lstat(filepath.Join(artifactDirectory, name)); !os.IsNotExist(err) {
			t.Fatalf("orphan artifact %q still exists: %v", name, err)
		}
	}
	if artifactLinkCreated {
		if _, err := os.Lstat(orphanArtifactLink); !os.IsNotExist(err) {
			t.Fatalf("orphan artifact symlink still exists: %v", err)
		}
	}
	if proposalLinkCreated {
		if _, err := os.Lstat(orphanProposalLink); !os.IsNotExist(err) {
			t.Fatalf("orphan proposal symlink still exists: %v", err)
		}
	}
	if _, err := os.Stat(orphanProposalDirectory); !os.IsNotExist(err) {
		t.Fatalf("orphan proposal directory still exists: %v", err)
	}
	if data, err := os.ReadFile(outsideFile); err != nil || string(data) != "outside target" {
		t.Fatalf("symlink target was changed: %q, %v", data, err)
	}
	if data, err := os.ReadFile(referencedArtifact.Spec.Path); err != nil || string(data) != "keep this artifact" {
		t.Fatalf("referenced artifact was removed: %q, %v", data, err)
	}
	artifacts, err := restarted.AssuranceArtifacts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("artifact records after janitor = %#v", artifacts)
	}
	if _, err := restarted.AssuranceSession(context.Background(), session.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	proposals, err := restarted.AssuranceProposals(context.Background(), session.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(proposals) != 1 || proposals[0].Metadata.ID != referencedProposal.Metadata.ID {
		t.Fatalf("proposal records after janitor = %#v", proposals)
	}
	if _, err := os.Stat(referencedProposal.Spec.IsolationPath); err != nil {
		t.Fatalf("referenced proposal directory was removed: %v", err)
	}
}

func TestAssuranceJanitorRejectsSymlinkedManagedDirectory(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "must-survive.txt")
	if err := os.WriteFile(outsideFile, []byte("outside target"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, "artifacts"), 0o700); err != nil {
		t.Fatal(err)
	}
	managedLink := filepath.Join(home, "artifacts", "assurance")
	if err := os.Symlink(outside, managedLink); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}
	err := cleanupOrphanedAssuranceArtifacts(home, newAssurancePathReferences(0))
	if err == nil || !strings.Contains(err.Error(), "not a local directory") {
		t.Fatalf("symlinked managed directory error = %v", err)
	}
	if data, err := os.ReadFile(outsideFile); err != nil || string(data) != "outside target" {
		t.Fatalf("symlinked managed directory changed outside target: %q, %v", data, err)
	}
}

func TestAssuranceJanitorFailsClosedWhenManagedDirectoryIsReplacedAfterValidation(t *testing.T) {
	if !assuranceCleanupRootSupported() {
		t.Skip("directory-handle cleanup requires Go 1.24 or newer")
	}
	home := t.TempDir()
	directory := filepath.Join(home, "artifacts", "assurance")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(directory, "orphan.json")
	if err := os.WriteFile(orphan, []byte("orphan"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "must-survive.txt")
	if err := os.WriteFile(outsideFile, []byte("outside target"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := openVerifiedAssuranceCleanupRoot(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("close cleanup root: %v", err)
		}
	})
	if checked, found, err := managedAssuranceDirectory(home, "artifacts", "assurance"); err != nil || !found || checked != directory {
		t.Fatalf("managed directory validation = %q, %v, found %v", checked, err, found)
	}
	if err := os.Rename(directory, filepath.Join(home, "artifacts", "assurance-original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, directory); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}

	err = removeManagedAssuranceEntry(root, home, orphan)
	if err == nil {
		t.Fatal("replacement race unexpectedly removed through an external symlink")
	}
	if data, err := os.ReadFile(outsideFile); err != nil || string(data) != "outside target" {
		t.Fatalf("replacement race changed outside target: %q, %v", data, err)
	}
	if _, err := os.Lstat(directory); err != nil {
		t.Fatalf("replacement symlink was unexpectedly removed: %v", err)
	}
	if _, err := os.Stat(orphan); err == nil {
		t.Fatal("replacement race resolved the orphan through the outside directory")
	}
}

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
