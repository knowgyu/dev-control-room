package app

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/collector"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/masking"
)

func TestSQLiteProjectRepositoryRestartAndManualPeriodicParity(t *testing.T) {
	home := t.TempDir()
	repositoryOne := tempGitRepository(t, "one")
	repositoryTwo := tempGitRepository(t, "two")
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Fixture Project", Path: repositoryOne})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddRepository(context.Background(), AddRepositoryInput{ProjectID: project.Metadata.ID, ID: "repo-two", Name: "Second", Path: repositoryTwo}); err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	firstFindings, err := service.Findings(context.Background(), project.Metadata.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(firstFindings) == 0 {
		t.Fatal("fixture repositories should produce missing_remote findings")
	}
	if _, err := service.UpdateProject(context.Background(), project.Metadata.ID, UpdateProjectInput{Name: "Renamed Fixture"}); err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}

	restarted, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	projects, err := restarted.Projects(context.Background())
	if err != nil || len(projects) != 1 || projects[0].Metadata.Name != "Renamed Fixture" || len(projects[0].Spec.Repositories) != 2 {
		t.Fatalf("project/repository state did not survive restart: %#v, %v", projects, err)
	}
	persistedFindings, err := restarted.Findings(context.Background(), project.Metadata.ID, "")
	if err != nil || len(persistedFindings) != len(firstFindings) {
		t.Fatalf("findings did not survive restart: %d vs %d, %v", len(persistedFindings), len(firstFindings), err)
	}
	if err := restarted.RunScan(context.Background(), "schedule"); err != nil {
		t.Fatal(err)
	}
	var observationCount int
	if err := restarted.store.DB().QueryRow(`SELECT count(*) FROM observations WHERE project_id = ?`, project.Metadata.ID).Scan(&observationCount); err != nil {
		t.Fatal(err)
	}
	if observationCount != 4 {
		t.Fatalf("repeated scans did not retain observation history: got %d, want 4", observationCount)
	}
	periodicFindings, err := restarted.Findings(context.Background(), project.Metadata.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if findingIdentity(firstFindings) != findingIdentity(periodicFindings) {
		t.Fatalf("manual and periodic scans produced different finding identities:\nmanual=%s\nperiodic=%s", findingIdentity(firstFindings), findingIdentity(periodicFindings))
	}
	events, err := restarted.Events(context.Background(), 100)
	if err != nil || len(events) < 4 {
		t.Fatalf("durable event repository was not populated: %d, %v", len(events), err)
	}
	if _, err := os.Stat(filepath.Join(home, "state.db")); err != nil {
		t.Fatal(err)
	}
}

func TestFailedCollectorCompletesFailedScanRunAndCreatesFindings(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Not Git", Path: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); contract.Classify(err).Code != contract.ErrorUnavailable {
		t.Fatalf("collector failure did not produce unavailable result: %v", err)
	}
	run, err := service.store.LatestScanRun(context.Background(), project.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Spec.Status != domain.ScanFailed || run.Spec.CompletedAt == nil {
		t.Fatalf("failed scan run was left incomplete: %#v", run.Spec)
	}
	findings, err := service.Findings(context.Background(), project.Metadata.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	types := make(map[string]bool)
	for _, finding := range findings {
		types[finding.Spec.FindingType] = true
	}
	if !types[domain.FindingCollectorError] || !types[domain.FindingStaleScan] {
		t.Fatalf("failed scan findings are incomplete: %#v", types)
	}
}

func TestUnregisteredPathIsNeverPassedToCollector(t *testing.T) {
	home := t.TempDir()
	registered := t.TempDir()
	unregistered := filepath.Join(t.TempDir(), "must-not-be-used")
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Registered", Path: registered})
	if err != nil {
		t.Fatal(err)
	}
	recorder := &recordingRunner{}
	service.collector = collector.NewGitCollector(recorder)
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	if len(recorder.directories) == 0 {
		t.Fatal("collector was not called for the registered repository")
	}
	for _, directory := range recorder.directories {
		if directory != project.Spec.Repositories[0].Spec.Path || strings.Contains(directory, unregistered) {
			t.Fatalf("collector accessed an unregistered path: %q", directory)
		}
	}
}

func TestSecretCanaryIsMaskedInSQLiteAndOutput(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	service.masker = masking.New([]string{"secret-canary"}, nil)
	event := domain.Event{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.EventKind}, Metadata: domain.ObjectMeta{ID: "secret-event", Name: "fixture"}, Spec: domain.EventSpec{EventType: "fixture.output", Summary: "fixture", Data: map[string]any{"output": "secret-canary"}, OccurredAt: nowUTC()}}
	if err := service.recordEvent(event); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := service.store.DB().QueryRow(`SELECT object_json FROM events WHERE id = ?`, "secret-event").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "secret-canary") {
		t.Fatalf("secret canary persisted in SQLite: %s", raw)
	}
	events, err := service.Events(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(events)
	if strings.Contains(string(encoded), "secret-canary") || !strings.Contains(string(encoded), masking.Replacement) {
		t.Fatalf("secret canary leaked or replacement missing from output: %s", encoded)
	}
}

func TestProjectAndRepositoryDeleteKeepRemovalEventDurable(t *testing.T) {
	home := t.TempDir()
	first := tempGitRepository(t, "first")
	second := tempGitRepository(t, "second")
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Delete Fixture", Path: first})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddRepository(context.Background(), AddRepositoryInput{ProjectID: project.Metadata.ID, ID: "second", Name: "Second", Path: second}); err != nil {
		t.Fatal(err)
	}
	if err := service.RemoveRepository(context.Background(), project.Metadata.ID, "second"); err != nil {
		t.Fatal(err)
	}
	if err := service.RemoveProject(context.Background(), project.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	events, err := service.Events(context.Background(), 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Spec.EventType == "project.removed" || event.Spec.EventType == "repository.removed" {
			return
		}
	}
	t.Fatal("removal events were not durable after FK-scoped deletes")
}

type recordingRunner struct {
	directories []string
}

func (r *recordingRunner) Run(_ context.Context, executable string, args []string, directory string) (collector.CommandResult, error) {
	if executable != "git" {
		return collector.CommandResult{}, os.ErrPermission
	}
	r.directories = append(r.directories, directory)
	if len(args) == 2 && args[0] == "rev-parse" && args[1] == "--is-inside-work-tree" {
		return collector.CommandResult{Stdout: "true\n"}, nil
	}
	if len(args) == 2 && args[0] == "rev-parse" && args[1] == "--show-toplevel" {
		return collector.CommandResult{Stdout: directory + "\n"}, nil
	}
	if len(args) > 0 && args[0] == "symbolic-ref" {
		return collector.CommandResult{Stdout: "main\n"}, nil
	}
	if len(args) > 0 && args[0] == "worktree" {
		return collector.CommandResult{Stdout: "worktree " + directory + "\nHEAD abc\nbranch refs/heads/main\n"}, nil
	}
	if len(args) > 0 && args[0] == "rev-parse" && len(args) > 2 {
		return collector.CommandResult{ExitCode: 1}, nil
	}
	return collector.CommandResult{}, nil
}

func findingIdentity(findings []domain.Finding) string {
	parts := make([]string, 0, len(findings))
	for _, finding := range findings {
		parts = append(parts, finding.Spec.FindingType+":"+finding.Spec.Fingerprint+":"+string(finding.Spec.State))
	}
	return strings.Join(parts, "|")
}

func tempGitRepository(t *testing.T, name string) string {
	t.Helper()
	directory := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, directory, "init", "--initial-branch=main")
	gitFixture(t, directory, "config", "user.email", "test@example.invalid")
	gitFixture(t, directory, "config", "user.name", "Dev Room Test")
	if err := os.WriteFile(filepath.Join(directory, "README.md"), []byte(name+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, directory, "add", "README.md")
	gitFixture(t, directory, "commit", "-m", "fixture")
	return directory
}

func gitFixture(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, output)
	}
}

func nowUTC() time.Time { return time.Now().UTC() }
