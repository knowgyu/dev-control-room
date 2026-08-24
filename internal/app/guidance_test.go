package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestGuidanceDoctorIsBoundedAndHandoffIsMaskedPreviewOnly(t *testing.T) {
	home := t.TempDir()
	repository := tempGitRepository(t, "guidance")
	if err := os.WriteFile(filepath.Join(repository, "AGENTS.md"), []byte("# Local rules\n\nRun the verified checkset.\nSee [missing](docs/missing.md).\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Guidance", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	report, err := service.Guidance(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil || len(report.Files) != 1 || len(report.Findings) == 0 {
		t.Fatalf("guidance report = %#v, %v", report, err)
	}
	if !strings.Contains(report.Findings[0].Code, "guidance") {
		t.Fatalf("guidance findings are not typed: %#v", report.Findings)
	}
	preview, err := service.PrepareHandoff(context.Background(), HandoffInput{ProfileID: "codex", ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary"})
	if err != nil {
		t.Fatal(err)
	}
	if preview.TranscriptIncluded || preview.ProfileID != "codex" || len(preview.Scope) == 0 || len(preview.VerificationCommands) == 0 {
		t.Fatalf("unsafe handoff preview: %#v", preview)
	}
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/projects/"+project.Metadata.ID+"/repositories/repo-1/worktrees/primary/guidance", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "guidance.missing_reference") {
		t.Fatalf("guidance HTTP surface = %d %s", recorder.Code, recorder.Body.String())
	}
	now := time.Now().UTC()
	if err := service.store.SaveFailureFingerprint(context.Background(), domain.FailureFingerprint{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.FailureFingerprintKind}, Metadata: domain.ObjectMeta{ID: "failure-fixture", Name: "fixture"}, Spec: domain.FailureFingerprintSpec{Fingerprint: "sha256:fixture", Category: "check.fixture", FirstSeen: now.Add(-time.Hour), LastSeen: now, OccurrenceCount: 3}}); err != nil {
		t.Fatal(err)
	}
	proposals, err := service.SafeguardProposals(context.Background(), 10)
	if err != nil || len(proposals) != 1 || proposals[0].Mode != "shadow" || proposals[0].State != "proposal" {
		t.Fatalf("safeguard proposals = %#v, %v", proposals, err)
	}
}
