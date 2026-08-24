package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/app"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestWriteCLIErrorDoesNotExposeInternalDetails(t *testing.T) {
	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	code := writeCLIError(errors.New("open C:\\private\\secret-canary: SQL details"))
	_ = writer.Close()
	os.Stderr = original
	data, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	if code != int(contract.ExitInternal) || strings.Contains(string(data), "secret-canary") || strings.Contains(string(data), "SQL details") || !strings.Contains(string(data), `"message":"internal error"`) {
		t.Fatalf("unexpected CLI error output: code=%d output=%s", code, data)
	}
}

func TestCheckRunExitCodes(t *testing.T) {
	for status, want := range map[domain.CheckRunStatus]int{domain.CheckPassed: 0, domain.CheckFailed: 3, domain.CheckUnavailable: 6, domain.CheckTimedOut: 6, domain.CheckCancelled: 5} {
		if got := checkRunExitCode(status); got != want {
			t.Fatalf("%s exit = %d, want %d", status, got, want)
		}
	}
}

func TestVersionJSONReportsCurrentMilestone(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runVersionTo([]string{"--json"}, &stdout, &stderr); code != int(contract.ExitSuccess) {
		t.Fatalf("version exit code = %d, stderr=%s", code, stderr.String())
	}
	var envelope contract.Envelope[map[string]string]
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.OK || envelope.Data == nil || (*envelope.Data)["milestone"] != "3" || (*envelope.Data)["version"] != version {
		t.Fatalf("unexpected version envelope: %#v", envelope)
	}
}

func TestProjectListJSONUsesStableEnvelope(t *testing.T) {
	home := t.TempDir()
	service, err := app.New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"project", "list", "--home", home, "--json"}, &stdout, &stderr); code != int(contract.ExitSuccess) {
		t.Fatalf("project list exit code = %d, stderr=%s", code, stderr.String())
	}
	var envelope contract.Envelope[[]map[string]any]
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Schema != contract.EnvelopeSchema || !envelope.OK || envelope.Data == nil || len(*envelope.Data) != 0 {
		t.Fatalf("unexpected CLI envelope: %#v", envelope)
	}
	want, err := os.ReadFile("testdata/project_list_empty.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != strings.TrimSpace(string(want)) {
		t.Fatalf("CLI JSON golden mismatch: got %s want %s", stdout.String(), want)
	}
}

func TestCleanupListJSONUsesStableEnvelope(t *testing.T) {
	home := t.TempDir()
	service, err := app.New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"cleanup", "list", "--home", home, "--json"}, &stdout, &stderr); code != int(contract.ExitSuccess) {
		t.Fatalf("cleanup list exit code = %d, stderr=%s", code, stderr.String())
	}
	var envelope contract.Envelope[[]domain.CleanupCandidate]
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || !envelope.OK || envelope.Data == nil || len(*envelope.Data) != 0 {
		t.Fatalf("unexpected cleanup envelope: %s (%v)", stdout.String(), err)
	}
}

func TestEnvironmentDoctorJSONUsesStableEnvelopeAndHidesSecret(t *testing.T) {
	const canary = "secret-canary-value"
	if err := os.Setenv("DEVROOM_CLI_SECRET", canary); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("DEVROOM_CLI_SECRET") })
	home := t.TempDir()
	service, err := app.New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"env", "doctor", "--home", home, "--json"}, &stdout, &stderr)
	if code != int(contract.ExitUnavailable) {
		t.Fatalf("env doctor exit code = %d, stderr=%s", code, stderr.String())
	}
	var envelope contract.Envelope[map[string]any]
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Schema != contract.EnvelopeSchema || !envelope.OK || envelope.Data == nil || strings.Contains(stdout.String(), canary) || strings.Contains(stderr.String(), canary) {
		t.Fatalf("invalid or secret-bearing environment envelope: stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}

func TestProjectWorktreeListJSONUsesReadOnlyEnvelope(t *testing.T) {
	home := t.TempDir()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repository := filepath.Dir(filepath.Dir(workingDirectory))
	service, err := app.New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	project, err := service.AddProject(context.Background(), app.AddProjectInput{Name: "worktrees", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	want, err := service.Worktrees(context.Background(), project.Metadata.ID, "repo-1")
	if err != nil || len(want) != 1 {
		t.Fatalf("service worktrees = %#v, %v", want, err)
	}
	snapshot, err := service.Snapshot(context.Background())
	if err != nil || !reflect.DeepEqual(snapshot.Projects[0].Repos[0].Worktrees, want) {
		t.Fatalf("snapshot worktrees differ: %#v %v", snapshot, err)
	}
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/projects/"+project.Metadata.ID+"/repositories/repo-1/worktrees", nil))
	var httpEnvelope contract.Envelope[[]domain.Worktree]
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &httpEnvelope) != nil || httpEnvelope.Data == nil || !reflect.DeepEqual(*httpEnvelope.Data, want) {
		t.Fatalf("HTTP worktrees differ: %d %s", recorder.Code, recorder.Body.String())
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"project", "worktree", "list", project.Metadata.ID, "repo-1", "--home", home, "--json"}, &stdout, &stderr); code != int(contract.ExitSuccess) {
		t.Fatalf("worktree CLI failed: %d %s", code, stderr.String())
	}
	var envelope contract.Envelope[[]domain.Worktree]
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || !envelope.OK || envelope.Data == nil || !reflect.DeepEqual(*envelope.Data, want) {
		t.Fatalf("worktree CLI envelope = %s (%v)", stdout.String(), err)
	}
	stdout.Reset()
	if code := run([]string{"project", "worktree", "show", project.Metadata.ID, "repo-1", want[0].Metadata.ID, "--home", home, "--json"}, &stdout, &stderr); code != int(contract.ExitSuccess) {
		t.Fatalf("worktree show failed: %d %s", code, stderr.String())
	}
	var show contract.Envelope[domain.Worktree]
	if err := json.Unmarshal(stdout.Bytes(), &show); err != nil || !show.OK || show.Data == nil || !reflect.DeepEqual(*show.Data, want[0]) {
		t.Fatalf("CLI show differs: %s (%v)", stdout.String(), err)
	}
}

func TestActionCLIUsesBrokerApprovalBoundary(t *testing.T) {
	home := t.TempDir()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repository := filepath.Dir(filepath.Dir(workingDirectory))
	service, err := app.New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	project, err := service.AddProject(context.Background(), app.AddProjectInput{Name: "Action CLI", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	planArgs := []string{"action", "plan", "--home", home, "--json", "--id", "plan-cli", "--name", "Production", "--project", project.Metadata.ID, "--repository", "repo-1", "--worktree", "primary", "--type", "release.production", "--input", "commit=abc123"}
	if code := run(planArgs, &stdout, &stderr); code != int(contract.ExitSuccess) {
		t.Fatalf("plan failed: %d %s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(append(planArgs, "--agent-id", "forged-agent"), &stdout, &stderr); code != int(contract.ExitInvalidInput) {
		t.Fatalf("plan accepted caller actor metadata: %d %s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"action", "admit", "plan-cli", "--home", home, "--json", "--holder", "runner-1", "--idempotency-key", "request-1"}, &stdout, &stderr); code != int(contract.ExitPolicyDenied) {
		t.Fatalf("unapproved admission exit = %d: %s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"action", "approve", "plan-cli", "--home", home, "--json", "--id", "approval-cli"}, &stdout, &stderr); code != int(contract.ExitInvalidInput) {
		t.Fatalf("approval surface remained available: %d %s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"action", "status", "plan-cli", "--home", home, "--json"}, &stdout, &stderr); code != int(contract.ExitSuccess) || !strings.Contains(stdout.String(), `"admission":"approval_required"`) {
		t.Fatalf("status = %d: %s %s", code, stdout.String(), stderr.String())
	}
	var cliStatus contract.Envelope[app.ActionApprovalStatus]
	if err := json.Unmarshal(stdout.Bytes(), &cliStatus); err != nil || cliStatus.Schema != contract.EnvelopeSchema || cliStatus.Data == nil {
		t.Fatalf("CLI status envelope = %s (%v)", stdout.String(), err)
	}
	if (*cliStatus.Data).Plan.Spec.RequestedBy != (domain.Actor{Kind: domain.ActorSystem, ID: "adapter"}) {
		t.Fatalf("CLI accepted forged actor metadata: %#v", (*cliStatus.Data).Plan.Spec.RequestedBy)
	}
	service, err = app.New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/actions/plans/plan-cli", nil))
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	var httpStatus contract.Envelope[app.ActionApprovalStatus]
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &httpStatus) != nil || httpStatus.Schema != cliStatus.Schema || httpStatus.Data == nil || !reflect.DeepEqual(*httpStatus.Data, *cliStatus.Data) {
		t.Fatalf("CLI/HTTP status envelopes differ: cli=%s http=%s", stdout.String(), recorder.Body.String())
	}
}
