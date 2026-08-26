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
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

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
	if !envelope.OK || envelope.Data == nil || (*envelope.Data)["milestone"] != "post-mvp" || (*envelope.Data)["version"] != version {
		t.Fatalf("unexpected version envelope: %#v", envelope)
	}
}

func TestCLIHelpDescribesFirstUseAndJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--help"}, &stdout, &stderr); code != int(contract.ExitSuccess) {
		t.Fatalf("help exit code = %d, stderr=%s", code, stderr.String())
	}
	for _, want := range []string{"첫 사용", "project add", "env doctor", "--json"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help omitted %q: %s", want, stdout.String())
		}
	}
	stdout.Reset()
	if code := run([]string{"help", "--json"}, &stdout, &stderr); code != int(contract.ExitSuccess) {
		t.Fatalf("JSON help exit code = %d, stderr=%s", code, stderr.String())
	}
	var envelope contract.Envelope[map[string]any]
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || !envelope.OK || envelope.Data == nil {
		t.Fatalf("invalid JSON help envelope: %s (%v)", stdout.String(), err)
	}
}

func TestAssuranceProviderCLIUsesStableEnvelope(t *testing.T) {
	home := t.TempDir()
	service, err := app.New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"assurance", "provider", "--json", "--home", home}, &stdout, &stderr); code != int(contract.ExitSuccess) {
		t.Fatalf("provider exit code = %d, stderr=%s", code, stderr.String())
	}
	var envelope contract.Envelope[[]app.ProviderStatus]
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || !envelope.OK || envelope.Data == nil || len(*envelope.Data) < 3 {
		t.Fatalf("invalid provider envelope: %s (%v)", stdout.String(), err)
	}
}

func TestAssuranceCommandContextPropagatesCancellationAndCleansUp(t *testing.T) {
	t.Run("parent cancellation", func(t *testing.T) {
		parent, cancel := context.WithCancel(context.Background())
		ctx, stop := newAssuranceCommandContext(parent)
		t.Cleanup(stop)
		cancel()
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
			t.Fatal("assurance command context did not propagate cancellation")
		}
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatalf("context error = %v, want cancellation", ctx.Err())
		}
	})

	t.Run("stop cleanup", func(t *testing.T) {
		ctx, stop := newAssuranceCommandContext(context.Background())
		stop()
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
			t.Fatal("assurance command context stop did not cancel context")
		}
	})
}

func TestAssuranceLifecycleCLIUsesNamedFlagsAndStableEnvelopes(t *testing.T) {
	home, projectID := setupAssuranceCLIFixture(t)
	base := []string{"--home", home, "--json"}

	sessionArgs := append([]string{"assurance", "session", "create", "--project", projectID, "--repository", "repo-1", "--worktree", "primary", "--provider", "fake", "--model", "fixture"}, base...)
	session := runAssuranceJSON[domain.AssuranceSession](t, sessionArgs...)
	if session.Spec.ProjectID != projectID || session.Spec.RepositoryID != "repo-1" || session.Spec.WorktreeID != "primary" || session.Spec.Provider != "fake" {
		t.Fatalf("unexpected assurance session: %#v", session)
	}

	baselineArgs := append([]string{"assurance", "baseline", "create", "--project", projectID, "--repository", "repo-1", "--worktree", "primary", "--target-branch", "main"}, base...)
	baseline := runAssuranceJSON[domain.PRCIBaseline](t, baselineArgs...)
	if baseline.Spec.ProjectID != projectID || baseline.Spec.TargetBranch != "main" || baseline.Spec.State != "fresh" {
		t.Fatalf("unexpected PR CI baseline: %#v", baseline)
	}

	campaignArgs := append([]string{"assurance", "campaign", "create", "--project", projectID, "--repository", "repo-1", "--worktree", "primary", "--name", "fixture campaign", "--session", session.Metadata.ID}, base...)
	campaign := runAssuranceJSON[domain.QualityCampaign](t, campaignArgs...)
	if campaign.Spec.ProjectID != projectID || campaign.Spec.Name != "fixture campaign" || campaign.Spec.SessionID != session.Metadata.ID {
		t.Fatalf("unexpected quality campaign: %#v", campaign)
	}

	runArgs := append([]string{"assurance", "run", "--campaign", campaign.Metadata.ID, "--technique", domain.QualityTechniqueStaticSecurity, "--provider", "fake", "--model", "fixture"}, base...)
	qualityRun := runAssuranceJSON[domain.QualityRun](t, runArgs...)
	if qualityRun.Spec.CampaignID != campaign.Metadata.ID || qualityRun.Spec.Technique != domain.QualityTechniqueStaticSecurity || qualityRun.Spec.State != domain.AssuranceStateSucceeded || len(qualityRun.Spec.ArtifactIDs) != 1 {
		t.Fatalf("unexpected quality run: %#v", qualityRun)
	}

	invocationArgs := append([]string{"assurance", "invocation", "run", "--session", session.Metadata.ID, "--provider", "fake", "--profile", "fake", "--model", "fixture", "--scenario", "success"}, base...)
	invocation := runAssuranceJSON[domain.AgentInvocation](t, invocationArgs...)
	if invocation.Spec.SessionID != session.Metadata.ID || invocation.Spec.Provider != "fake" || invocation.Spec.State != domain.AssuranceStateSucceeded || invocation.Spec.Usage.TotalTokens == nil {
		t.Fatalf("unexpected agent invocation: %#v", invocation)
	}

	inspectArgs := append([]string{"assurance", "invocation", "inspect", "--id", invocation.Metadata.ID}, base...)
	inspected := runAssuranceJSON[domain.AgentInvocation](t, inspectArgs...)
	if !reflect.DeepEqual(inspected, invocation) {
		t.Fatalf("inspect result differs from run result: inspected=%#v invocation=%#v", inspected, invocation)
	}

	var stdout, stderr bytes.Buffer
	if code := run(append([]string{"assurance", "sessions"}, base...), &stdout, &stderr); code != int(contract.ExitSuccess) || !strings.Contains(stdout.String(), session.Metadata.ID) {
		t.Fatalf("existing sessions query changed: code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestAssuranceCLIRejectsUnsafeOrAmbiguousInputs(t *testing.T) {
	home := t.TempDir()
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing required session flag", args: []string{"assurance", "session", "create", "--repository", "repo-1", "--worktree", "primary", "--home", home}, want: "--project is required"},
		{name: "positional quality run", args: []string{"assurance", "run", "campaign-id", "--home", home}, want: "assurance run accepts named flags only"},
		{name: "unknown technique", args: []string{"assurance", "run", "--campaign", "campaign-id", "--technique", "arbitrary", "--home", home}, want: "--technique must be one of"},
		{name: "unknown provider", args: []string{"assurance", "invocation", "run", "--session", "session-id", "--provider", "arbitrary", "--home", home}, want: "--provider must be one of"},
		{name: "codex requires bounded prompt", args: []string{"assurance", "invocation", "run", "--session", "session-id", "--provider", "codex", "--home", home}, want: "provider.prompt_required"},
		{name: "unknown scenario", args: []string{"assurance", "invocation", "run", "--session", "session-id", "--provider", "fake", "--scenario", "arbitrary", "--home", home}, want: "--scenario is not a supported fixture scenario"},
		{name: "arbitrary command flag", args: []string{"assurance", "run", "--campaign", "campaign-id", "--technique", domain.QualityTechniqueStaticSecurity, "--command", "remove-all", "--home", home}, want: "flag provided but not defined"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(test.args, &stdout, &stderr); code != int(contract.ExitInvalidInput) {
				t.Fatalf("exit code = %d, want invalid input (%d), stdout=%s stderr=%s", code, contract.ExitInvalidInput, stdout.String(), stderr.String())
			}
			var envelope contract.Envelope[map[string]any]
			if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil || envelope.Schema != contract.EnvelopeSchema || envelope.OK || envelope.Error == nil || !strings.Contains(envelope.Error.Message, test.want) {
				t.Fatalf("unexpected error envelope: %s (%v)", stderr.String(), err)
			}
		})
	}
}

func TestAssuranceCLIHelpListsLifecycleCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"help", "assurance", "--json"}, &stdout, &stderr); code != int(contract.ExitSuccess) {
		t.Fatalf("assurance help exit code = %d, stderr=%s", code, stderr.String())
	}
	var envelope contract.Envelope[map[string]any]
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || !envelope.OK || envelope.Data == nil {
		t.Fatalf("invalid assurance help envelope: %s (%v)", stdout.String(), err)
	}
	data, ok := (*envelope.Data)["commands"].([]any)
	if !ok {
		t.Fatalf("assurance help commands have unexpected type: %#v", (*envelope.Data)["commands"])
	}
	for _, want := range []string{"session create", "baseline create", "campaign create", "run", "invocation show", "invocation run"} {
		found := false
		for _, item := range data {
			if item == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("assurance help omitted %q: %s", want, stdout.String())
		}
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"assurance", "--help", "--json"}, &stdout, &stderr); code != int(contract.ExitSuccess) || !strings.Contains(stdout.String(), `"command":"assurance"`) {
		t.Fatalf("direct assurance JSON help failed: code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func runAssuranceJSON[T any](t *testing.T, args ...string) T {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := run(args, &stdout, &stderr); code != int(contract.ExitSuccess) {
		t.Fatalf("CLI command failed: code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var envelope contract.Envelope[T]
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON envelope: %v: %s", err, stdout.String())
	}
	if envelope.Schema != contract.EnvelopeSchema || !envelope.OK || envelope.Data == nil {
		t.Fatalf("unexpected JSON envelope: %#v", envelope)
	}
	return *envelope.Data
}

func setupAssuranceCLIFixture(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	repository := filepath.Join(t.TempDir(), "assurance-cli-fixture")
	if err := os.MkdirAll(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "init", repository).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("assurance fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", repository, "-c", "user.email=fixture@example.invalid", "-c", "user.name=fixture"}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	git("add", "README.md")
	git("commit", "-m", "assurance fixture")

	service, err := app.New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	project, err := service.AddProject(context.Background(), app.AddProjectInput{Name: "Assurance CLI", Path: repository})
	if err != nil {
		_ = service.Close()
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		_ = service.Close()
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	return home, project.Metadata.ID
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

func TestSafeguardCLIIsReadOnly(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"safeguard", "activate", "safeguard-1"}, &stdout, &stderr); code != int(contract.ExitInvalidInput) {
		t.Fatalf("safeguard mutation exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "safeguard requires list") {
		t.Fatalf("safeguard mutation error = %s", stderr.String())
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
	repository := filepath.Join(t.TempDir(), "fixture-repository")
	if err := os.MkdirAll(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "init", repository).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		command := exec.Command("git", append([]string{"-C", repository, "-c", "user.email=fixture@example.invalid", "-c", "user.name=fixture"}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	git("add", "README.md")
	git("commit", "-m", "fixture")
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
