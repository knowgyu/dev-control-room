package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/environment"
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
	failures, err := service.FailureFingerprints(context.Background(), 10)
	if err != nil || len(failures) != 0 {
		t.Fatalf("handoff preview created failure learning = %#v, %v", failures, err)
	}
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/projects/"+project.Metadata.ID+"/repositories/repo-1/worktrees/primary/guidance", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "guidance.missing_reference") {
		t.Fatalf("guidance HTTP surface = %d %s", recorder.Code, recorder.Body.String())
	}
}

type recordingHandoffLauncher struct {
	executable string
	arguments  []string
	env        []string
	directory  string
	calls      int
}

func (l *recordingHandoffLauncher) Launch(_ context.Context, executable string, arguments []string, env []string, directory string) (environment.LaunchResult, error) {
	l.executable = executable
	l.arguments = append([]string(nil), arguments...)
	l.env = append([]string(nil), env...)
	l.directory = directory
	l.calls++
	return environment.LaunchResult{PID: 4242}, nil
}

func TestHandoffLaunchRequiresCurrentPreviewAndUsesExactArgv(t *testing.T) {
	home := t.TempDir()
	repository := tempGitRepository(t, "handoff-launch")
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Handoff", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	profile, err := service.AddAgentProfile(context.Background(), AddAgentProfileInput{ID: "fixture-agent", Name: "Fixture Agent", Command: "fixture-agent", ModelArgumentTemplate: "--model {model}", LaunchMode: domain.AgentLaunchDirect, DataBoundary: domain.AgentBoundaryLocal})
	if err != nil {
		t.Fatal(err)
	}
	launcher := &recordingHandoffLauncher{}
	service.launcher = launcher
	input := HandoffInput{ProfileID: profile.Metadata.ID, ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Model: "fixture-model"}
	preview, err := service.PrepareHandoff(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if preview.PreviewDigest == "" || len(preview.Arguments) != 3 || preview.Arguments[0] != "--model" || preview.Arguments[1] != "fixture-model" || preview.Arguments[2] == "" {
		t.Fatalf("handoff preview did not expose exact argv contract: %#v", preview)
	}
	body, err := json.Marshal(HandoffLaunchInput{HandoffInput: input, PreviewDigest: preview.PreviewDigest})
	if err != nil {
		t.Fatal(err)
	}
	unauthorized := httptest.NewRecorder()
	service.Handler().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "/api/handoffs/launch", bytes.NewReader(body)))
	if unauthorized.Code != http.StatusForbidden {
		t.Fatalf("unprotected handoff launch was accepted: %d %s", unauthorized.Code, unauthorized.Body.String())
	}
	launch, err := service.LaunchHandoff(context.Background(), HandoffLaunchInput{HandoffInput: input, PreviewDigest: preview.PreviewDigest})
	if err != nil {
		t.Fatal(err)
	}
	if launch.PID != 4242 || launch.TranscriptIncluded || launcher.calls != 1 || launcher.executable != "fixture-agent" || launcher.directory != repository {
		t.Fatalf("handoff launch contract = %#v, launcher = %#v", launch, launcher)
	}
	if len(launcher.arguments) != len(preview.Arguments) || launcher.arguments[0] != preview.Arguments[0] || launcher.arguments[1] != preview.Arguments[1] || launcher.arguments[2] != preview.Arguments[2] {
		t.Fatalf("launcher argv differs from preview: %#v / %#v", launcher.arguments, preview.Arguments)
	}
	if _, err := service.LaunchHandoff(context.Background(), HandoffLaunchInput{HandoffInput: input, PreviewDigest: "stale"}); err == nil {
		t.Fatal("stale handoff preview was launched")
	}
	events, err := service.Events(context.Background(), 20)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if event.Spec.EventType == "handoff.launched" {
			found = true
		}
	}
	if !found {
		t.Fatal("handoff launch was not audited")
	}
	if launch.StartedAt.Before(time.Unix(0, 0)) {
		t.Fatalf("invalid launch timestamp: %#v", launch)
	}
}
