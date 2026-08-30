package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/assurance"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/environment"
	"github.com/knowgyu/dev-control-room/internal/masking"
)

func tempGoGitRepository(t *testing.T, name string) string {
	t.Helper()
	directory := tempGitRepository(t, name)
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module fixture.example/quality\n\ngo 1.23\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, directory, "add", "go.mod", "main.go")
	gitFixture(t, directory, "commit", "-m", "add minimal Go fixture")
	return directory
}

func tempGoCoverageGitRepository(t *testing.T, name string) string {
	t.Helper()
	directory := qualityCoverageTempDir(t)
	gitFixture(t, directory, "init", "--initial-branch=main")
	gitFixture(t, directory, "config", "user.email", "test@example.invalid")
	gitFixture(t, directory, "config", "user.name", "Dev Room Test")
	if err := os.WriteFile(filepath.Join(directory, "README.md"), []byte(name+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod": "module fixture.example/coverage\n\ngo 1.23\n",
		"covered.go": `package fixture

func Covered() int { return 42 }
`,
		"covered_test.go": `package fixture

import "testing"

func TestCovered(t *testing.T) {
	if Covered() != 42 {
		t.Fatal("unexpected fixture value")
	}
}
`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gitFixture(t, directory, "add", "README.md", "go.mod", "covered.go", "covered_test.go")
	gitFixture(t, directory, "commit", "-m", "add coverage fixture")
	return directory
}

func TestFakeProviderE2EProducesResumeEvidenceWithoutTranscript(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Assurance", Path: tempGitRepository(t, "assurance")})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateAssuranceSession(context.Background(), AssuranceSessionInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Provider: "fake", RequestedModel: "fixture-model"})
	if err != nil {
		t.Fatal(err)
	}
	invocation, err := service.RunAgentInvocation(context.Background(), AgentInvocationInput{SessionID: session.Metadata.ID, Provider: "fake", ProfileID: "fake", RequestedModel: "fixture-model", Scenario: "success"})
	if err != nil {
		t.Fatal(err)
	}
	if invocation.Spec.State != domain.AssuranceStateSucceeded || invocation.Spec.RawTranscript || len(invocation.Spec.ArtifactIDs) != 1 || invocation.Spec.Usage.TotalTokens == nil {
		t.Fatalf("unexpected invocation: %#v", invocation)
	}
	updated, err := service.AssuranceSession(context.Background(), session.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Spec.ResumeBrief.NextSafeAction == "" || len(updated.Spec.ResumeBrief.Completed) != 1 {
		t.Fatalf("resume brief = %#v", updated.Spec.ResumeBrief)
	}
}

func TestCodexInvocationRequiresExplicitPromptBeforeTrustedRunner(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	repository := tempGitRepository(t, "codex-typed")
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Codex typed", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateAssuranceSession(context.Background(), AssuranceSessionInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Provider: "codex", RequestedModel: "fixture-model"})
	if err != nil {
		t.Fatal(err)
	}
	runnerCalls := 0
	ctx := assurance.WithCodexExecution(context.Background(), assurance.CodexExecution{
		Resolver: func() assurance.ProviderStatus {
			return assurance.ProviderStatus{Provider: "codex", State: assurance.ProviderReady, CommandFound: true, LaunchTrusted: true, ProfileReady: true, ResolvedCommand: []string{`C:\Program Files\nodejs\node.exe`, `C:\Users\fixture\node_modules\@openai\codex\bin\codex.js`}}
		},
		Runner: func(_ context.Context, _ assurance.RunRequest, _ *masking.Masker) assurance.RunResult {
			runnerCalls++
			return assurance.RunResult{State: domain.AssuranceStateSucceeded, Structured: map[string]any{"unexpected": true}, RawTranscript: true}
		},
	})
	invocation, err := service.RunAgentInvocation(ctx, AgentInvocationInput{SessionID: session.Metadata.ID, Provider: "codex", ProfileID: "codex", RequestedModel: "fixture-model"})
	if err != nil {
		t.Fatal(err)
	}
	if runnerCalls != 0 || invocation.Spec.State != domain.AssuranceStateFailed || invocation.Spec.FailureCode != "provider.prompt_required" || invocation.Spec.RawTranscript {
		t.Fatalf("calls=%d invocation=%#v", runnerCalls, invocation)
	}
}

func TestCodexInvocationUsesFixedArgvAndDoesNotPersistPrompt(t *testing.T) {
	home := t.TempDir()
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Codex bounded", Path: tempGitRepository(t, "codex-bounded")})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateAssuranceSession(context.Background(), AssuranceSessionInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Provider: "codex", RequestedModel: "fixture-model"})
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := service.Worktree(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil {
		t.Fatal(err)
	}
	const prompt = "inspect private fixture"
	var captured assurance.RunRequest
	ctx := assurance.WithCodexExecution(context.Background(), assurance.CodexExecution{
		Resolver: func() assurance.ProviderStatus {
			return assurance.ProviderStatus{Provider: "codex", State: assurance.ProviderReady, CommandFound: true, LaunchTrusted: true, ProfileReady: true, ResolvedCommand: []string{`C:\Program Files\nodejs\node.exe`, `C:\Users\fixture\node_modules\@openai\codex\bin\codex.js`}}
		},
		Runner: func(_ context.Context, request assurance.RunRequest, _ *masking.Masker) assurance.RunResult {
			captured = request
			return assurance.RunResult{State: domain.AssuranceStateSucceeded, Structured: map[string]any{
				"summary":    "completed: " + prompt,
				"findings":   []any{"finding mentions " + prompt},
				"nextAction": "review " + prompt,
			}, Summary: "completed: " + prompt, RawTranscript: true}
		},
	})
	invocation, err := service.RunAgentInvocation(ctx, AgentInvocationInput{SessionID: session.Metadata.ID, Provider: "codex", ProfileID: "codex", RequestedModel: "fixture-model", Prompt: "  " + prompt + "  "})
	if err != nil {
		t.Fatal(err)
	}
	if invocation.Spec.State != domain.AssuranceStateSucceeded || invocation.Spec.RawTranscript {
		t.Fatalf("invocation = %#v", invocation)
	}
	want := []string{`C:\Users\fixture\node_modules\@openai\codex\bin\codex.js`, "exec", "--json", "--sandbox", "read-only", "--ephemeral", "--ignore-user-config", "--ignore-rules", "--cd", worktree.Spec.CanonicalPath, "--output-schema", filepath.Join(home, "runtime", "codex", "output-schema.json"), "--model", "fixture-model", "--", prompt}
	if captured.Worktree != worktree.Spec.CanonicalPath || len(captured.Command.Arguments) != len(want) {
		t.Fatalf("captured request = %#v, want argv %#v", captured, want)
	}
	for index := range want {
		if captured.Command.Arguments[index] != want[index] {
			t.Fatalf("captured argv = %#v, want %#v", captured.Command.Arguments, want)
		}
	}
	schema, err := os.ReadFile(filepath.Join(home, "runtime", "codex", "output-schema.json"))
	if err != nil || string(schema) != string(assurance.CodexOutputSchema()) {
		t.Fatalf("schema = %q, err=%v", schema, err)
	}
	persisted, _ := json.Marshal(invocation)
	if strings.Contains(string(persisted), prompt) {
		t.Fatalf("prompt persisted in invocation: %s", persisted)
	}
	artifacts, err := service.AssuranceArtifacts(context.Background())
	if err != nil || len(artifacts) != 1 {
		t.Fatalf("artifacts = %#v, err=%v", artifacts, err)
	}
	artifact, err := os.ReadFile(artifacts[0].Spec.Path)
	if err != nil || strings.Contains(string(artifact), prompt) {
		t.Fatalf("prompt persisted in artifact: %q, err=%v", artifact, err)
	}
}

func TestCodexInvocationFailsClosedForProfileLauncherAndUnsupportedOutput(t *testing.T) {
	testCases := []struct {
		name       string
		profileID  string
		status     assurance.ProviderStatus
		runner     assurance.TypedRunner
		wantCode   string
		wantCalled bool
	}{
		{name: "missing profile", profileID: "missing", status: assurance.ProviderStatus{Provider: "codex", State: assurance.ProviderReady, CommandFound: true, LaunchTrusted: true, ProfileReady: true, ResolvedCommand: []string{`C:\Program Files\nodejs\node.exe`, `C:\Users\fixture\node_modules\@openai\codex\bin\codex.js`}}, wantCode: "provider.profile_required"},
		{name: "untrusted profile", profileID: "claude", status: assurance.ProviderStatus{Provider: "codex", State: assurance.ProviderReady, CommandFound: true, LaunchTrusted: true, ProfileReady: true, ResolvedCommand: []string{`C:\Program Files\nodejs\node.exe`, `C:\Users\fixture\node_modules\@openai\codex\bin\codex.js`}}, wantCode: "provider.profile_untrusted"},
		{name: "untrusted launcher", profileID: "codex", status: assurance.ProviderStatus{Provider: "codex", State: assurance.ProviderDetected, CommandFound: true, ReasonCode: "provider.untrusted_launcher"}, wantCode: "provider.untrusted_launcher"},
		{name: "missing launcher", profileID: "codex", status: assurance.ProviderStatus{Provider: "codex", State: assurance.ProviderNotConfigured, ReasonCode: "provider.not_found"}, wantCode: "provider.not_found"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service, err := New(t.TempDir(), "127.0.0.1:38471")
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Codex fail closed", Path: tempGitRepository(t, "codex-fail-"+testCase.name)})
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
			runnerCalls := 0
			runner := testCase.runner
			if runner == nil {
				runner = func(context.Context, assurance.RunRequest, *masking.Masker) assurance.RunResult {
					runnerCalls++
					return assurance.RunResult{State: domain.AssuranceStateSucceeded, Structured: map[string]any{"unexpected": true}}
				}
			} else {
				original := runner
				runner = func(ctx context.Context, request assurance.RunRequest, masker *masking.Masker) assurance.RunResult {
					runnerCalls++
					return original(ctx, request, masker)
				}
			}
			ctx := assurance.WithCodexExecution(context.Background(), assurance.CodexExecution{Resolver: func() assurance.ProviderStatus { return testCase.status }, Runner: runner})
			invocation, err := service.RunAgentInvocation(ctx, AgentInvocationInput{SessionID: session.Metadata.ID, Provider: "codex", ProfileID: testCase.profileID, RequestedModel: "fixture-model", Prompt: "inspect this fixture"})
			if err != nil {
				t.Fatal(err)
			}
			if invocation.Spec.State != domain.AssuranceStateFailed || invocation.Spec.FailureCode != testCase.wantCode || runnerCalls != boolToInt(testCase.wantCalled) {
				t.Fatalf("invocation=%#v runnerCalls=%d", invocation, runnerCalls)
			}
		})
	}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func TestFakeProviderFailureMatrixKeepsSessionRecoverable(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Assurance Failure", Path: tempGitRepository(t, "assurance-failure")})
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
	invocation, err := service.RunAgentInvocation(context.Background(), AgentInvocationInput{SessionID: session.Metadata.ID, Provider: "claude", ProfileID: "claude", Scenario: "approval_prompt"})
	if err != nil {
		t.Fatal(err)
	}
	if invocation.Spec.State != domain.AssuranceStateFailed || invocation.Spec.FailureCode != "provider.interactive_prompt" {
		t.Fatalf("failure = %#v", invocation)
	}
	updated, err := service.AssuranceSession(context.Background(), session.Metadata.ID)
	if err != nil || updated.Spec.ResumeBrief.NextSafeAction == "" || len(updated.Spec.ResumeBrief.FailedEvidence) != 1 {
		t.Fatalf("recoverable session = %#v, %v", updated, err)
	}
}

func TestStartupRecoveryMarksActiveInvocationInterruptedWithoutRelaunch(t *testing.T) {
	home := t.TempDir()
	repository := tempGitRepository(t, "startup-recovery")
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Startup recovery", Path: repository})
	if err != nil {
		_ = service.Close()
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		_ = service.Close()
		t.Fatal(err)
	}
	session, err := service.CreateAssuranceSession(context.Background(), AssuranceSessionInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Provider: "fake", RequestedModel: "fixture-model"})
	if err != nil {
		_ = service.Close()
		t.Fatal(err)
	}
	worktree, err := service.Worktree(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil {
		_ = service.Close()
		t.Fatal(err)
	}
	started := time.Now().UTC().Add(-time.Minute)
	lease := started.Add(2 * time.Hour)
	invocation := domain.AgentInvocation{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AgentInvocationKind},
		Metadata: domain.ObjectMeta{ID: "interrupted-invocation", Name: "Interrupted invocation"},
		Spec: domain.AgentInvocationSpec{
			SessionID: session.Metadata.ID, ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary",
			Branch: worktree.Spec.Branch, Head: worktree.Spec.Head, Provider: "fake", ProfileID: "fake", RequestedModel: "fixture-model",
			SelectionSource: "user", State: domain.AssuranceStateRunning, IdempotencyKey: "interrupted-invocation", StartedAt: started,
			LeaseExpiresAt: &lease, TraceID: "trace-interrupted-invocation", RawTranscript: false,
		},
	}
	if err := service.store.SaveAgentInvocation(context.Background(), invocation); err != nil {
		_ = service.Close()
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}

	restarted, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	invocations, err := restarted.AgentInvocations(context.Background())
	if err != nil || len(invocations) != 1 {
		t.Fatalf("recovered invocations = %#v, err=%v", invocations, err)
	}
	recovered := invocations[0]
	if recovered.Metadata.ID != invocation.Metadata.ID || recovered.Spec.State != domain.AssuranceStateInterrupted || recovered.Spec.FailureCode != "provider.interrupted" || recovered.Spec.LeaseExpiresAt != nil {
		t.Fatalf("recovered invocation = %#v", recovered)
	}
	updated, err := restarted.AssuranceSession(context.Background(), session.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Spec.State != domain.AssuranceStateInterrupted || !hasAssuranceValue(updated.Spec.ResumeBrief.Pending, invocation.Metadata.ID) || !hasAssuranceValue(updated.Spec.ResumeBrief.FailedEvidence, "provider.interrupted") || updated.Spec.ResumeBrief.NextSafeAction == "" {
		t.Fatalf("recovery brief = %#v", updated.Spec.ResumeBrief)
	}
	revision, err := restarted.store.AssuranceRevision(context.Background(), domain.AgentInvocationKind, invocation.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if again, err := reopened.store.AssuranceRevision(context.Background(), domain.AgentInvocationKind, invocation.Metadata.ID); err != nil || again != revision {
		t.Fatalf("recovery was not idempotent: revision=%d want=%d err=%v", again, revision, err)
	}
}

func hasAssuranceValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestRetryInterruptedInvocationCreatesIdempotentChildWithoutPromptPersistence(t *testing.T) {
	home := t.TempDir()
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Retry", Path: tempGitRepository(t, "retry")})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateAssuranceSession(context.Background(), AssuranceSessionInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Provider: "fake", RequestedModel: "fixture-model"})
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := service.Worktree(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	original := domain.AgentInvocation{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AgentInvocationKind},
		Metadata: domain.ObjectMeta{ID: "interrupted-for-retry", Name: "Interrupted invocation"},
		Spec: domain.AgentInvocationSpec{
			SessionID: session.Metadata.ID, ProjectID: project.Metadata.ID, RepositoryID: "repo-1",
			WorktreeID: "primary", Branch: worktree.Spec.Branch, Head: worktree.Spec.Head,
			Provider: "fake", ProfileID: "fake", RequestedModel: "fixture-model",
			SelectionSource: "user", State: domain.AssuranceStateInterrupted,
			IdempotencyKey: "interrupted-for-retry", InputDigest: digestText("original prompt"),
			TraceID: "trace-interrupted-for-retry", StartedAt: now.Add(-time.Minute), RawTranscript: false,
			FailureCode: "provider.interrupted",
		},
	}
	if err := service.store.SaveAgentInvocation(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	session.Spec.State = domain.AssuranceStateInterrupted
	session.Spec.UpdatedAt = now
	session.Spec.ResumeBrief.Pending = []string{original.Metadata.ID}
	session.Spec.ResumeBrief.NextSafeAction = "중단된 실행의 상태와 재시도 범위를 검토합니다."
	if err := service.updateAssuranceSession(context.Background(), session); err != nil {
		t.Fatal(err)
	}

	retry, err := service.RetryAgentInvocation(context.Background(), original.Metadata.ID, "retry prompt")
	if err != nil {
		t.Fatal(err)
	}
	if retry.Metadata.ID == original.Metadata.ID || retry.Spec.ParentID != original.Metadata.ID ||
		retry.Spec.State != domain.AssuranceStateSucceeded || retry.Spec.InputDigest != digestText("retry prompt") ||
		retry.Spec.IdempotencyKey == original.Spec.IdempotencyKey {
		t.Fatalf("retry invocation = %#v", retry)
	}
	persisted, err := json.Marshal(retry)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(persisted), "retry prompt") || strings.Contains(string(persisted), "original prompt") {
		t.Fatalf("retry prompt crossed the persistence boundary: %s", persisted)
	}
	invocations, err := service.AgentInvocations(context.Background())
	if err != nil || len(invocations) != 2 {
		t.Fatalf("invocations after retry = %#v, err=%v", invocations, err)
	}
	updated, err := service.AssuranceSession(context.Background(), session.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Spec.State != domain.AssuranceStateReady || hasAssuranceValue(updated.Spec.ResumeBrief.Pending, original.Metadata.ID) {
		t.Fatalf("retry did not resolve the interrupted pending item: %#v", updated.Spec.ResumeBrief)
	}

	again, err := service.RetryAgentInvocation(context.Background(), original.Metadata.ID, "retry prompt")
	if err != nil {
		t.Fatal(err)
	}
	if again.Metadata.ID != retry.Metadata.ID {
		t.Fatalf("idempotent retry changed child: first=%s again=%s", retry.Metadata.ID, again.Metadata.ID)
	}
	invocations, err = service.AgentInvocations(context.Background())
	if err != nil || len(invocations) != 2 {
		t.Fatalf("idempotent retry created another invocation: %#v, err=%v", invocations, err)
	}
	if _, err := service.RetryAgentInvocation(context.Background(), original.Metadata.ID, "different prompt"); err == nil || !strings.Contains(err.Error(), "idempotency key") {
		t.Fatalf("different retry prompt was not rejected: %v", err)
	}
	if _, err := service.RetryAgentInvocation(context.Background(), retry.Metadata.ID, "another retry"); err == nil || !strings.Contains(err.Error(), "only an interrupted invocation") {
		t.Fatalf("succeeded child was retryable: %v", err)
	}
	if _, err := service.RetryAgentInvocation(context.Background(), original.Metadata.ID, "retry\nwith newline"); err == nil || !strings.Contains(err.Error(), "one line") {
		t.Fatalf("newline retry prompt was accepted: %v", err)
	}
}

func TestReconcileRetrySessionPreservesActualSessionFailure(t *testing.T) {
	original := domain.AgentInvocation{Metadata: domain.ObjectMeta{ID: "interrupted"}, Spec: domain.AgentInvocationSpec{SessionID: "session-1"}}
	retry := domain.AgentInvocation{Metadata: domain.ObjectMeta{ID: "retry-child"}, Spec: domain.AgentInvocationSpec{State: domain.AssuranceStateSucceeded}}
	now := time.Now().UTC()
	baseSession := domain.AssuranceSession{Metadata: domain.ObjectMeta{ID: "session-1"}, Spec: domain.AssuranceSessionSpec{State: domain.AssuranceStateInterrupted, UpdatedAt: now, ResumeBrief: domain.ResumeBrief{Pending: []string{original.Metadata.ID}}}}
	sessionErr := errors.New("session read failed")
	updateErr := errors.New("session update failed")
	tests := []struct {
		name       string
		readErr    error
		updateErr  error
		wantCalled bool
	}{
		{name: "session read", readErr: sessionErr},
		{name: "session update", updateErr: updateErr, wantCalled: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var updated domain.AssuranceSession
			called := false
			gotErr := reconcileRetrySession(
				context.Background(), original, retry,
				func(_ context.Context, sessionID string) (domain.AssuranceSession, error) {
					if sessionID != original.Spec.SessionID {
						t.Fatalf("session id = %q, want %q", sessionID, original.Spec.SessionID)
					}
					if test.readErr != nil {
						return domain.AssuranceSession{}, test.readErr
					}
					return baseSession, nil
				},
				func(_ context.Context, session domain.AssuranceSession) error {
					called = true
					updated = session
					return test.updateErr
				},
			)
			wantErr := test.readErr
			if wantErr == nil {
				wantErr = test.updateErr
			}
			if gotErr != wantErr {
				t.Fatalf("reconcile error = %v, want exact error %v", gotErr, wantErr)
			}
			if called != test.wantCalled {
				t.Fatalf("update called = %v, want %v", called, test.wantCalled)
			}
			if test.wantCalled && (updated.Spec.State != domain.AssuranceStateReady || hasAssuranceValue(updated.Spec.ResumeBrief.Pending, original.Metadata.ID)) {
				t.Fatalf("retry session transition was not prepared before update failure: %#v", updated)
			}
		})
	}
}

func TestConcurrentRetriesDoNotOverwriteSessionOrCreateChildren(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Concurrent retry", Path: tempGitRepository(t, "concurrent-retry")})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateAssuranceSession(context.Background(), AssuranceSessionInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Provider: "fake", RequestedModel: "fixture-model"})
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := service.Worktree(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	original := domain.AgentInvocation{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AgentInvocationKind},
		Metadata: domain.ObjectMeta{ID: "concurrent-interrupted", Name: "Interrupted invocation"},
		Spec: domain.AgentInvocationSpec{
			SessionID: session.Metadata.ID, ProjectID: project.Metadata.ID, RepositoryID: "repo-1",
			WorktreeID: "primary", Branch: worktree.Spec.Branch, Head: worktree.Spec.Head,
			Provider: "fake", ProfileID: "fake", RequestedModel: "fixture-model",
			SelectionSource: "user", State: domain.AssuranceStateInterrupted,
			IdempotencyKey: "concurrent-interrupted", InputDigest: digestText("original prompt"),
			TraceID: "trace-concurrent-interrupted", StartedAt: now.Add(-time.Minute), RawTranscript: false,
			FailureCode: "provider.interrupted",
		},
	}
	if err := service.store.SaveAgentInvocation(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	session.Spec.State = domain.AssuranceStateInterrupted
	session.Spec.UpdatedAt = now
	session.Spec.ResumeBrief.Pending = []string{original.Metadata.ID}
	session.Spec.ResumeBrief.NextSafeAction = "중단된 실행의 상태와 재시도 범위를 검토합니다."
	if err := service.updateAssuranceSession(context.Background(), session); err != nil {
		t.Fatal(err)
	}

	type outcome struct {
		item domain.AgentInvocation
		err  error
	}
	start := make(chan struct{})
	results := make(chan outcome, 2)
	for range 2 {
		go func() {
			<-start
			item, retryErr := service.RetryAgentInvocation(context.Background(), original.Metadata.ID, "retry prompt")
			results <- outcome{item: item, err: retryErr}
		}()
	}
	close(start)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("concurrent retries failed: first=%v second=%v", first.err, second.err)
	}
	if first.item.Metadata.ID == "" || first.item.Metadata.ID != second.item.Metadata.ID || first.item.Spec.State != domain.AssuranceStateSucceeded || second.item.Spec.State != domain.AssuranceStateSucceeded {
		t.Fatalf("concurrent retry children = %#v and %#v", first.item, second.item)
	}
	invocations, err := service.AgentInvocations(context.Background())
	if err != nil || len(invocations) != 2 {
		t.Fatalf("concurrent retries created duplicate children: %#v, err=%v", invocations, err)
	}
	updated, err := service.AssuranceSession(context.Background(), session.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Spec.State != domain.AssuranceStateReady || hasAssuranceValue(updated.Spec.ResumeBrief.Pending, original.Metadata.ID) || hasAssuranceValue(updated.Spec.ResumeBrief.Pending, first.item.Metadata.ID) || len(updated.Spec.ResumeBrief.Completed) != 1 || updated.Spec.ResumeBrief.Completed[0] != first.item.Metadata.ID {
		t.Fatalf("concurrent retry session state was overwritten: %#v", updated.Spec.ResumeBrief)
	}
}

func TestAgentInvocationFailsClosedWhenEvidenceArtifactCannotPersist(t *testing.T) {
	home := t.TempDir()
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Artifact failure", Path: tempGitRepository(t, "artifact-failure")})
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
	if err := os.WriteFile(filepath.Join(home, "artifacts"), []byte("block artifact directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	invocation, invokeErr := service.RunAgentInvocation(context.Background(), AgentInvocationInput{SessionID: session.Metadata.ID, Provider: "fake", ProfileID: "fake"})
	var coded contract.CodedError
	if !errors.As(invokeErr, &coded) || coded.Code != contract.ErrorExecutionFailed {
		t.Fatalf("artifact persistence error = %v", invokeErr)
	}
	if invocation.Spec.State != domain.AssuranceStateFailed || invocation.Spec.FailureCode != "artifact.persistence_failed" || invocation.Spec.Structured != nil || len(invocation.Spec.ArtifactIDs) != 0 {
		t.Fatalf("artifact persistence invocation = %#v", invocation)
	}
	persistedItems, err := service.AgentInvocations(context.Background())
	if err != nil || len(persistedItems) != 1 || persistedItems[0].Spec.State != domain.AssuranceStateFailed || persistedItems[0].Spec.FailureCode != "artifact.persistence_failed" {
		t.Fatalf("persisted artifact failure = %#v, %v", persistedItems, err)
	}
}

func TestBaselineDiscoversRequiredObservedLocalEquivalentUnknownAndTurnsStale(t *testing.T) {
	repository := tempGitRepository(t, "baseline")
	if err := os.MkdirAll(filepath.Join(repository, ".github", "workflows"), 0o700); err != nil {
		t.Fatal(err)
	}
	packageData, _ := json.Marshal(map[string]any{"scripts": map[string]string{"test": "go test ./..."}})
	if err := os.WriteFile(filepath.Join(repository, "package.json"), packageData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, ".github", "required-checks.txt"), []byte("build-windows\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, ".github", "workflows", "ci.yml"), []byte("on: pull_request\njobs:\n  test:\n    steps:\n      - run: go test ./...\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Baseline", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	baseline, err := service.CreatePRCIBaseline(context.Background(), BaselineInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", TargetBranch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	classes := map[string]bool{}
	for _, entry := range baseline.Spec.Entries {
		classes[entry.Classification] = true
	}
	for _, classification := range []string{domain.BaselineRequired, domain.BaselineObserved, domain.BaselineLocalEquivalent, domain.BaselineUnknown} {
		if !classes[classification] {
			t.Fatalf("baseline missing %s: %#v", classification, baseline.Spec.Entries)
		}
	}
	if err := os.WriteFile(filepath.Join(repository, "next.txt"), []byte("next\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, repository, "add", ".")
	gitFixture(t, repository, "commit", "-m", "advance fixture")
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	items, err := service.PRCIBaselines(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("baselines = %#v, %v", items, err)
	}
	if items[0].Spec.State != "stale" || !strings.Contains(items[0].Spec.StaleReason, "HEAD") {
		t.Fatalf("baseline freshness = %#v", items[0])
	}
}

func TestBaselineSkipsMultilineWorkflowRunScalars(t *testing.T) {
	repository := tempGitRepository(t, "baseline-multiline")
	workflowPath := filepath.Join(repository, ".github", "workflows", "ci.yml")
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o700); err != nil {
		t.Fatal(err)
	}
	workflow := "jobs:\n  test:\n    steps:\n      - run: |\n          go test ./...\n      - run: >-\n          go vet ./...\n      - run: go test -count=1 ./...\n"
	if err := os.WriteFile(workflowPath, []byte(workflow), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, _, _, err := discoverBaseline(repository)
	if err != nil {
		t.Fatal(err)
	}
	observed := make([]string, 0)
	for _, entry := range entries {
		if entry.Classification == domain.BaselineObserved {
			observed = append(observed, entry.Command)
		}
	}
	if len(observed) != 1 || observed[0] != "go test -count=1 ./..." {
		t.Fatalf("observed one-line workflow commands = %#v", observed)
	}
}

func TestQualityRunUsesRegisteredGoVetRunnerAndPersistsBoundedReport(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Quality", Path: tempGoGitRepository(t, "quality")})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	campaign, err := service.CreateQualityCampaign(context.Background(), QualityCampaignInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Name: "static fixture"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := service.RunQuality(context.Background(), QualityRunInput{CampaignID: campaign.Metadata.ID, Technique: domain.QualityTechniqueStaticSecurity, Provider: "fake", Model: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if run.Spec.State != domain.AssuranceStateSucceeded || len(run.Spec.ArtifactIDs) != 1 || !strings.HasSuffix(strings.ToLower(run.Spec.Command.Executable), "go.exe") {
		t.Fatalf("quality run = %#v", run)
	}
	if run.Spec.Runner != assurance.QualityRunnerGoVetID || !reflectQualityGoVetArgs(run.Spec.Command.Arguments) || !strings.HasPrefix(run.Spec.ConfigDigest, "sha256:") || run.Spec.ExitCode != 0 {
		t.Fatalf("registered runner = %#v", run)
	}
	if run.Spec.Evidence["selectionState"] != string(assurance.QualityRunnerSelectionAvailable) {
		t.Fatalf("selection evidence = %#v", run.Spec.Evidence)
	}
	artifacts, err := service.AssuranceArtifacts(context.Background())
	if err != nil || len(artifacts) != 1 {
		t.Fatalf("artifacts = %#v, %v", artifacts, err)
	}
	artifact, err := os.ReadFile(artifacts[0].Spec.Path)
	if err != nil {
		t.Fatal(err)
	}
	artifactText := string(artifact)
	for _, required := range []string{assurance.QualityRunnerGoVetID, "runnerMetadata", "configDigest", "exitCode", "result"} {
		if !strings.Contains(artifactText, required) {
			t.Fatalf("artifact missing %q: %s", required, artifactText)
		}
	}
	if strings.Contains(artifactText, "git diff") || strings.Contains(artifactText, "techniqueReport") {
		t.Fatalf("artifact retained the old fake runner: %s", artifactText)
	}
	if revision, err := service.store.AssuranceRevision(context.Background(), domain.QualityRunKind, run.Metadata.ID); err != nil || revision != 3 {
		t.Fatalf("quality run revision = %d, err=%v", revision, err)
	}
}

func newQualityCoverageFixture(t *testing.T) (*App, domain.QualityCampaign, string) {
	t.Helper()
	service, err := New(qualityCoverageTempDir(t), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Errorf("close service: %v", err)
		}
	})
	path := tempGoCoverageGitRepository(t, "quality-coverage")
	project := domain.NewProject("quality-coverage", "Quality coverage", []domain.Repository{domain.NewRepository("repo-1", "Quality coverage", path)})
	if err := service.store.SaveProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := service.store.ReplaceWorktrees(context.Background(), project.Metadata.ID, "repo-1", []domain.Worktree{{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.WorktreeKind},
		Metadata: domain.ObjectMeta{ID: "primary", Name: "primary"},
		Spec: domain.WorktreeSpec{
			ProjectID: project.Metadata.ID, RepositoryID: "repo-1", CanonicalPath: path, PathFingerprint: worktreePathFingerprint(path),
			AssociationFingerprint: "sha256:coverage-fixture", Trust: domain.WorktreeTrustVerifiedReadOnly, Primary: true,
			Head: "head", Branch: "main", LastObserved: now,
		},
	}}, true); err != nil {
		t.Fatal(err)
	}
	campaign, err := service.CreateQualityCampaign(context.Background(), QualityCampaignInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Name: "coverage fixture"})
	if err != nil {
		t.Fatal(err)
	}
	return service, campaign, path
}

func qualityCoveragePersistence(service *App, path string) qualityRunPersistence {
	persistence := service.qualityRunPersistence()
	persistence.revalidateWorktree = func(context.Context, domain.Worktree) (string, error) {
		return path, nil
	}
	return persistence
}

func newStoredQualityCampaignFixture(t *testing.T) (*App, domain.QualityCampaign) {
	t.Helper()
	service, err := New(qualityCoverageTempDir(t), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Errorf("close service: %v", err)
		}
	})
	path := qualityCoverageTempDir(t)
	project := domain.NewProject("campaign-project", "Campaign project", []domain.Repository{domain.NewRepository("repository", "Repository", path)})
	if err := service.store.SaveProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := service.store.ReplaceWorktrees(context.Background(), project.Metadata.ID, "repository", []domain.Worktree{{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.WorktreeKind},
		Metadata: domain.ObjectMeta{ID: "primary", Name: "primary"},
		Spec: domain.WorktreeSpec{
			ProjectID: project.Metadata.ID, RepositoryID: "repository", CanonicalPath: path, PathFingerprint: "sha256:path",
			Trust: domain.WorktreeTrustVerifiedReadOnly, Primary: true, Head: "head", LastObserved: now,
		},
	}}, true); err != nil {
		t.Fatal(err)
	}
	campaign := domain.QualityCampaign{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.QualityCampaignKind},
		Metadata: domain.ObjectMeta{ID: "campaign-concurrent", Name: "Concurrent campaign"},
		Spec: domain.QualityCampaignSpec{
			ProjectID: project.Metadata.ID, RepositoryID: "repository", WorktreeID: "primary", Name: "Concurrent campaign",
			State: domain.AssuranceStateDraft, CreatedAt: now, UpdatedAt: now,
		},
	}
	if err := service.store.SaveQualityCampaign(context.Background(), campaign); err != nil {
		t.Fatal(err)
	}
	return service, campaign
}

func TestQualityRunPersistsNormalizedGoCoverageAndLinkedArtifacts(t *testing.T) {
	service, campaign, path := newQualityCoverageFixture(t)
	run, err := service.runQualityWithPersistence(context.Background(), QualityRunInput{CampaignID: campaign.Metadata.ID, Technique: domain.QualityTechniqueGoTestCoverage}, qualityCoveragePersistence(service, path))
	if err != nil {
		t.Fatal(err)
	}
	if run.Spec.State != domain.AssuranceStateSucceeded || run.Spec.Outcome != domain.QualityRunOutcomeCoverageCollected || run.Spec.Coverage == nil || len(run.Spec.ArtifactIDs) != 2 {
		t.Fatalf("coverage run = %#v", run)
	}
	if run.Spec.Coverage.Mode != "set" || run.Spec.Coverage.FileCount != 1 || run.Spec.Coverage.TotalStatements == 0 || run.Spec.Coverage.CoveredStatements == 0 || run.Spec.Coverage.Percent <= 0 || run.Spec.Coverage.ProfileArtifactID == "" {
		t.Fatalf("coverage summary = %#v", run.Spec.Coverage)
	}
	artifacts, err := service.AssuranceArtifacts(context.Background())
	if err != nil || len(artifacts) != 2 {
		t.Fatalf("coverage artifacts = %#v, %v", artifacts, err)
	}
	var profile, report domain.Artifact
	for _, artifact := range artifacts {
		switch artifact.Spec.MIME {
		case "text/plain":
			profile = artifact
		case "application/json":
			report = artifact
		}
	}
	if profile.Metadata.ID == "" || report.Metadata.ID == "" || profile.Metadata.ID != run.Spec.Coverage.ProfileArtifactID || !containsText(run.Spec.ArtifactIDs, profile.Metadata.ID) || !containsText(run.Spec.ArtifactIDs, report.Metadata.ID) {
		t.Fatalf("coverage artifact links = profile=%#v report=%#v run=%#v", profile, report, run)
	}
	reportData, err := os.ReadFile(report.Spec.Path)
	if err != nil {
		t.Fatal(err)
	}
	var reportObject map[string]any
	if err := json.Unmarshal(reportData, &reportObject); err != nil {
		t.Fatal(err)
	}
	coverage, ok := reportObject["coverage"].(map[string]any)
	if !ok || coverage["profileArtifactId"] != profile.Metadata.ID {
		t.Fatalf("normalized report coverage = %#v", reportObject["coverage"])
	}
	campaigns, err := service.QualityCampaigns(context.Background())
	if err != nil || len(campaigns) != 1 || !containsText(campaigns[0].Spec.RunIDs, run.Metadata.ID) {
		t.Fatalf("campaign run ids = %#v, %v", campaigns, err)
	}
}

func TestQualityRunReportPersistenceFailureCompensatesCoverageArtifactAndClosesRun(t *testing.T) {
	service, campaign, path := newQualityCoverageFixture(t)
	reportErr := errors.New("report persistence fixture failure")
	persistence := qualityCoveragePersistence(service, path)
	baseSaveArtifact := persistence.saveArtifact
	persistence.saveArtifact = func(ctx context.Context, input ArtifactInput) (domain.Artifact, error) {
		if strings.HasSuffix(input.Name, ".json") {
			return domain.Artifact{}, reportErr
		}
		return baseSaveArtifact(ctx, input)
	}

	run, err := service.runQualityWithPersistence(context.Background(), QualityRunInput{CampaignID: campaign.Metadata.ID, Technique: domain.QualityTechniqueGoTestCoverage}, persistence)
	if !errors.Is(err, reportErr) || !strings.Contains(err.Error(), "persist quality run report artifact") {
		t.Fatalf("report persistence error = %v", err)
	}
	if run.Spec.State != domain.AssuranceStateFailed || run.Spec.Outcome != domain.QualityRunOutcomeInconclusive || run.Spec.StaleReason != "quality.persistence_failed" || len(run.Spec.ArtifactIDs) != 0 {
		t.Fatalf("closed report-failure run = %#v", run)
	}
	storedRuns, err := service.QualityRuns(context.Background())
	if err != nil || len(storedRuns) != 1 || storedRuns[0].Spec.State != domain.AssuranceStateFailed || storedRuns[0].Spec.Outcome != domain.QualityRunOutcomeInconclusive {
		t.Fatalf("stored report-failure run = %#v, %v", storedRuns, err)
	}
	if revision, err := service.store.AssuranceRevision(context.Background(), domain.QualityRunKind, run.Metadata.ID); err != nil || revision != 3 {
		t.Fatalf("report-failure run revision = %d, err=%v", revision, err)
	}
	artifacts, err := service.AssuranceArtifacts(context.Background())
	if err != nil || len(artifacts) != 1 || artifacts[0].Spec.Retention != domain.ArtifactRetentionDeleted {
		t.Fatalf("compensated report-failure artifacts = %#v, %v", artifacts, err)
	}
	if _, err := os.Stat(artifacts[0].Spec.Path); !os.IsNotExist(err) {
		t.Fatalf("compensated profile path = %v", err)
	}
}

func TestQualityRunFinalRevisionFailureCompensatesPersistedArtifactsAndClosesRun(t *testing.T) {
	service, campaign, path := newQualityCoverageFixture(t)
	finalErr := errors.New("final revision fixture failure")
	persistence := qualityCoveragePersistence(service, path)
	baseUpdateRevision := persistence.updateAssuranceRevision
	persistence.updateAssuranceRevision = func(ctx context.Context, kind, id string, revision int, state string, updatedAt time.Time, value any) error {
		if kind == domain.QualityRunKind && revision == 3 {
			if err := baseUpdateRevision(ctx, kind, id, revision, state, updatedAt, value); err != nil {
				return err
			}
			return finalErr
		}
		return baseUpdateRevision(ctx, kind, id, revision, state, updatedAt, value)
	}

	run, err := service.runQualityWithPersistence(context.Background(), QualityRunInput{CampaignID: campaign.Metadata.ID, Technique: domain.QualityTechniqueGoTestCoverage}, persistence)
	if !errors.Is(err, finalErr) || !strings.Contains(err.Error(), "persist quality run final revision") {
		t.Fatalf("final revision persistence error = %v", err)
	}
	if run.Spec.State != domain.AssuranceStateFailed || run.Spec.Outcome != domain.QualityRunOutcomeInconclusive || run.Spec.StaleReason != "quality.persistence_failed" || len(run.Spec.ArtifactIDs) != 0 {
		t.Fatalf("closed final-failure run = %#v", run)
	}
	if revision, err := service.store.AssuranceRevision(context.Background(), domain.QualityRunKind, run.Metadata.ID); err != nil || revision != 4 {
		t.Fatalf("final-failure run revision = %d, err=%v", revision, err)
	}
	artifacts, err := service.AssuranceArtifacts(context.Background())
	if err != nil || len(artifacts) != 2 {
		t.Fatalf("compensated final-failure artifacts = %#v, %v", artifacts, err)
	}
	for _, artifact := range artifacts {
		if artifact.Spec.Retention != domain.ArtifactRetentionDeleted {
			t.Fatalf("final-failure artifact retention = %#v", artifact)
		}
		if _, err := os.Stat(artifact.Spec.Path); !os.IsNotExist(err) {
			t.Fatalf("final-failure artifact path = %v", err)
		}
	}
}

func TestQualityCampaignRunIDsUseBoundedRevisionCASRetry(t *testing.T) {
	service, campaign := newStoredQualityCampaignFixture(t)
	release := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(2)
	results := make(chan error, 2)
	for _, runID := range []string{"run-concurrent-a", "run-concurrent-b"} {
		runID := runID
		firstUpdate := true
		go func() {
			err := service.appendQualityRunToCampaignWithHook(context.Background(), campaign.Metadata.ID, runID, time.Now().UTC(), func() {
				if firstUpdate {
					firstUpdate = false
					ready.Done()
				}
				<-release
			})
			results <- err
		}()
	}
	ready.Wait()
	close(release)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("concurrent campaign update: %v", err)
		}
	}
	campaigns, err := service.QualityCampaigns(context.Background())
	if err != nil || len(campaigns) != 1 || len(campaigns[0].Spec.RunIDs) != 2 || !containsText(campaigns[0].Spec.RunIDs, "run-concurrent-a") || !containsText(campaigns[0].Spec.RunIDs, "run-concurrent-b") {
		t.Fatalf("concurrent campaign run ids = %#v, %v", campaigns, err)
	}
	if revision, err := service.store.AssuranceRevision(context.Background(), domain.QualityCampaignKind, campaign.Metadata.ID); err != nil || revision != 3 {
		t.Fatalf("concurrent campaign revision = %d, err=%v", revision, err)
	}
}

func TestQualityRunProcessOutcomeDistinguishesFailureTimeoutAndInconclusive(t *testing.T) {
	tests := []struct {
		name        string
		result      environment.Result
		processErr  error
		wantState   string
		wantOutcome string
		wantReason  string
	}{
		{name: "tests failed", result: environment.Result{ExitCode: 1}, processErr: errors.New("exit status 1"), wantState: domain.AssuranceStateFailed, wantOutcome: domain.QualityRunOutcomeTestsFailed, wantReason: "runner.tests_failed"},
		{name: "timeout", processErr: context.DeadlineExceeded, wantState: domain.AssuranceStateTimedOut, wantOutcome: domain.QualityRunOutcomeInconclusive, wantReason: "runner.timeout"},
		{name: "bounded output", processErr: errors.New("process output exceeded bounded limit"), wantState: domain.AssuranceStateFailed, wantOutcome: domain.QualityRunOutcomeInconclusive, wantReason: "runner.inconclusive"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state, outcome, _, reason := qualityRunProcessOutcome(test.result, test.processErr)
			if state != test.wantState || outcome != test.wantOutcome || reason != test.wantReason {
				t.Fatalf("outcome = %q/%q/%q", state, outcome, reason)
			}
		})
	}
}

func TestQualityRunPersistsActualRunnerFailureAndReturnsExecutionError(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	root := tempGoGitRepository(t, "quality-failure")
	if err := os.WriteFile(filepath.Join(root, "broken.go"), []byte("package main\nfunc broken( {\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Quality failure", Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	campaign, err := service.CreateQualityCampaign(context.Background(), QualityCampaignInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Name: "static failure"})
	if err != nil {
		t.Fatal(err)
	}
	run, runErr := service.RunQuality(context.Background(), QualityRunInput{CampaignID: campaign.Metadata.ID, Technique: domain.QualityTechniqueStaticSecurity})
	var coded contract.CodedError
	if !errors.As(runErr, &coded) || coded.Code != contract.ErrorExecutionFailed {
		t.Fatalf("run error = %v", runErr)
	}
	if run.Spec.State != domain.AssuranceStateFailed || run.Spec.ExitCode == 0 || !reflectQualityGoVetArgs(run.Spec.Command.Arguments) || len(run.Spec.ArtifactIDs) != 1 {
		t.Fatalf("failed quality run = %#v", run)
	}
	if result, ok := run.Spec.Evidence["result"].(map[string]any); !ok || result["executed"] != true || result["exitCode"].(int) == 0 {
		t.Fatalf("failure result evidence = %#v", run.Spec.Evidence["result"])
	}
	if revision, err := service.store.AssuranceRevision(context.Background(), domain.QualityRunKind, run.Metadata.ID); err != nil || revision != 3 {
		t.Fatalf("failed quality run revision = %d, err=%v", revision, err)
	}
}

func TestQualityRunPersistsUnavailableWithoutProcessCommand(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Quality unavailable", Path: tempGitRepository(t, "quality-unavailable")})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	campaign, err := service.CreateQualityCampaign(context.Background(), QualityCampaignInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Name: "static unavailable"})
	if err != nil {
		t.Fatal(err)
	}
	run, runErr := service.RunQuality(context.Background(), QualityRunInput{CampaignID: campaign.Metadata.ID, Technique: domain.QualityTechniqueStaticSecurity})
	var coded contract.CodedError
	if !errors.As(runErr, &coded) || coded.Code != contract.ErrorUnavailable {
		t.Fatalf("run error = %v", runErr)
	}
	if run.Spec.State != domain.AssuranceStateFailed || run.Spec.Runner != assurance.QualityRunnerGoVetID || run.Spec.Command.Executable != "" || len(run.Spec.Command.Arguments) != 0 || len(run.Spec.ArtifactIDs) != 1 {
		t.Fatalf("unavailable quality run = %#v", run)
	}
	if reason, ok := run.Spec.Evidence["unavailable"].(*assurance.QualityRunnerUnavailableReason); !ok || reason.Code != assurance.QualityRunnerReasonGoModMissing {
		t.Fatalf("unavailable evidence = %#v", run.Spec.Evidence)
	}
	artifact, err := os.ReadFile(func() string {
		items, listErr := service.AssuranceArtifacts(context.Background())
		if listErr != nil || len(items) != 1 {
			t.Fatalf("artifacts = %#v, err=%v", items, listErr)
		}
		return items[0].Spec.Path
	}())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(artifact), "\"command\"") || strings.Contains(string(artifact), "git diff") || strings.Contains(string(artifact), "techniqueReport") {
		t.Fatalf("unavailable artifact contains a process command or fake report: %s", artifact)
	}
}

func TestAssuranceRunnersRevalidateWorktreeImmediatelyBeforeExecution(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Revalidate assurance", Path: tempGoGitRepository(t, "revalidate-assurance")})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	session, err := service.CreateAssuranceSession(context.Background(), AssuranceSessionInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Provider: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	campaign, err := service.CreateQualityCampaign(context.Background(), QualityCampaignInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Name: "revalidate"})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := service.Worktree(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil {
		t.Fatal(err)
	}
	stored.Spec.Trust = domain.WorktreeTrustUnverified
	stored.Spec.LastObserved = time.Now().UTC()
	if err := service.store.ReplaceWorktrees(context.Background(), project.Metadata.ID, "repo-1", []domain.Worktree{stored}, true); err != nil {
		t.Fatal(err)
	}

	run, runErr := service.RunQuality(context.Background(), QualityRunInput{CampaignID: campaign.Metadata.ID, Technique: domain.QualityTechniqueStaticSecurity})
	var unavailable contract.CodedError
	if !errors.As(runErr, &unavailable) || unavailable.Code != contract.ErrorUnavailable {
		t.Fatalf("quality revalidation error = %v", runErr)
	}
	if run.Spec.State != domain.AssuranceStateFailed || run.Spec.StaleReason != "worktree.revalidation_failed" || run.Spec.Command.Executable != "" {
		t.Fatalf("quality revalidation run = %#v", run)
	}
	if reason, ok := run.Spec.Evidence["unavailable"].(*assurance.QualityRunnerUnavailableReason); !ok || reason.Code != "worktree.revalidation_failed" {
		t.Fatalf("quality revalidation evidence = %#v", run.Spec.Evidence)
	}

	runnerCalls := 0
	ctx := assurance.WithCodexExecution(context.Background(), assurance.CodexExecution{
		Resolver: func() assurance.ProviderStatus {
			return assurance.ProviderStatus{Provider: "codex", State: assurance.ProviderReady, CommandFound: true, LaunchTrusted: true, ProfileReady: true, ResolvedCommand: []string{`C:\\Program Files\\nodejs\\node.exe`, `C:\\Users\\fixture\\node_modules\\@openai\\codex\\bin\\codex.js`}}
		},
		Runner: func(context.Context, assurance.RunRequest, *masking.Masker) assurance.RunResult {
			runnerCalls++
			return assurance.RunResult{State: domain.AssuranceStateSucceeded}
		},
	})
	invocation, err := service.RunAgentInvocation(ctx, AgentInvocationInput{SessionID: session.Metadata.ID, Provider: "codex", ProfileID: "codex", Prompt: "inspect fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if runnerCalls != 0 || invocation.Spec.State != domain.AssuranceStateFailed || invocation.Spec.FailureCode != "worktree.revalidation_failed" {
		t.Fatalf("invocation revalidation = %#v, runner calls=%d", invocation, runnerCalls)
	}
}

func reflectQualityGoVetArgs(args []string) bool {
	return len(args) == 3 && args[0] == "vet" && args[1] == "-mod=readonly" && args[2] == "./..."
}

func TestQualityRunnerEnvironmentBlocksModuleNetworkAndWorkspaceInheritance(t *testing.T) {
	values := map[string]string{}
	for _, entry := range qualityRunnerEnvironment() {
		name, value, found := strings.Cut(entry, "=")
		if found {
			values[strings.ToUpper(name)] = value
		}
	}
	for name, want := range map[string]string{"GOPROXY": "off", "GOSUMDB": "off", "GOTOOLCHAIN": "local", "GOWORK": "off"} {
		if values[name] != want {
			t.Fatalf("quality environment %s = %q, want %q", name, values[name], want)
		}
	}
	if values["GOCACHE"] == "" {
		t.Fatalf("quality runner has no isolated Go cache: %#v", values)
	}
}

func TestAssuranceAuthoringPersistsQuestionsSpecAndAdvisoryPatchWithoutAdoptionSideEffects(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Authoring", Path: tempGitRepository(t, "authoring")})
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
	question, err := service.CreateAssuranceQuestion(context.Background(), AssuranceQuestionInput{SessionID: session.Metadata.ID, Prompt: "어떤 입력을 보호해야 합니까?", Required: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AnswerAssuranceQuestion(context.Background(), session.Metadata.ID, question.Metadata.ID, "빈 입력을 거부합니다."); err != nil {
		t.Fatal(err)
	}
	spec, err := service.CreateAssuranceSpec(context.Background(), AssuranceSpecInput{SessionID: session.Metadata.ID, Intent: "입력 검증을 보강합니다.", Properties: []string{"빈 입력은 거부"}, Source: "human_review"})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Spec.Digest == "" || spec.Spec.Revision != 1 {
		t.Fatalf("spec = %#v", spec)
	}
	proposal, err := service.CreateAssuranceProposal(context.Background(), AssuranceProposalInput{SessionID: session.Metadata.ID, Purpose: "검증 테스트 제안", Patch: "diff --git a/test.go b/test.go\n+// git push must remain human-only\n"})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Spec.State != "critic_advisory" || proposal.Spec.CriticSummary == "" || proposal.Spec.IsolationPath == "" || proposal.Spec.PatchArtifactID == "" {
		t.Fatalf("proposal = %#v", proposal)
	}
	reviewed, err := service.ReviewAssuranceProposal(context.Background(), proposal.Metadata.ID, "adopt")
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.Spec.State != "adopted" || reviewed.Spec.ReviewedAt == nil {
		t.Fatalf("reviewed proposal = %#v", reviewed)
	}
}

func TestAllV1TechniqueAdaptersCreateArtifactsAndArchiveDeleteWithWarning(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := exec.LookPath("go.exe"); err != nil {
		t.Fatalf("native go.exe is required for this app integration test: %v", err)
	}
	root := tempGoGitRepository(t, "techniques")
	if err := os.WriteFile(filepath.Join(root, "quality_targets_test.go"), []byte(`package main

import "testing"

func TestPropertyRoundTrip(t *testing.T) {
	if got := "fixture"; got != "fixture" {
		t.Fatalf("round trip = %q", got)
	}
}

func FuzzInput(f *testing.F) {
	f.Add("fixture")
	f.Fuzz(func(t *testing.T, value string) {
		if value != string([]byte(value)) {
			t.Fatal("string normalization changed a value")
		}
	})
}

func TestE2EHealth(t *testing.T) {}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, root, "add", "quality_targets_test.go")
	gitFixture(t, root, "commit", "-m", "add quality runner targets")
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Techniques", Path: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	campaign, err := service.CreateQualityCampaign(context.Background(), QualityCampaignInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Name: "all techniques"})
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, 5)
	staticRun, runErr := service.RunQuality(context.Background(), QualityRunInput{CampaignID: campaign.Metadata.ID, Technique: domain.QualityTechniqueStaticSecurity, Provider: "fake"})
	if runErr != nil || staticRun.Spec.State != domain.AssuranceStateSucceeded || len(staticRun.Spec.ArtifactIDs) != 1 {
		t.Fatalf("static run = %#v, err=%v", staticRun, runErr)
	}
	ids = append(ids, staticRun.Spec.ArtifactIDs[0])
	mutationRun, runErr := service.RunQuality(context.Background(), QualityRunInput{CampaignID: campaign.Metadata.ID, Technique: domain.QualityTechniqueMutation, Provider: "fake"})
	var mutationError contract.CodedError
	if !errors.As(runErr, &mutationError) || mutationError.Code != contract.ErrorUnavailable {
		t.Fatalf("mutation error = %v", runErr)
	}
	if mutationRun.Spec.State != domain.AssuranceStateFailed || mutationRun.Spec.Runner != assurance.QualityRunnerGoMutationID || mutationRun.Spec.Command.Executable != "" || len(mutationRun.Spec.Command.Arguments) != 0 || len(mutationRun.Spec.ArtifactIDs) != 1 {
		t.Fatalf("mutation run = %#v", mutationRun)
	}
	if reason, ok := mutationRun.Spec.Evidence["unavailable"].(*assurance.QualityRunnerUnavailableReason); !ok || reason.Code != assurance.QualityRunnerReasonMutationMissing {
		t.Fatalf("mutation unavailable evidence = %#v", mutationRun.Spec.Evidence)
	}
	ids = append(ids, mutationRun.Spec.ArtifactIDs[0])

	expectedRunners := map[string]string{
		domain.QualityTechniqueProperty:    assurance.QualityRunnerGoPropertyID,
		domain.QualityTechniqueFuzz:        assurance.QualityRunnerGoFuzzID,
		domain.QualityTechniqueTargetedE2E: assurance.QualityRunnerGoE2EID,
	}
	for technique, runner := range expectedRunners {
		run, runErr := service.RunQuality(context.Background(), QualityRunInput{CampaignID: campaign.Metadata.ID, Technique: technique, Provider: "fake"})
		if runErr != nil {
			t.Fatalf("%s error = %v", technique, runErr)
		}
		if run.Spec.State != domain.AssuranceStateSucceeded || run.Spec.Runner != runner || run.Spec.Command.Executable == "" || len(run.Spec.Command.Arguments) == 0 || len(run.Spec.ArtifactIDs) != 1 {
			t.Fatalf("%s run = %#v", technique, run)
		}
		if run.Spec.Evidence["selectionState"] != string(assurance.QualityRunnerSelectionAvailable) {
			t.Fatalf("%s selection evidence = %#v", technique, run.Spec.Evidence)
		}
		ids = append(ids, run.Spec.ArtifactIDs[0])
	}
	if _, err := service.DeleteAssuranceArtifact(context.Background(), ids[0], "wrong"); err == nil {
		t.Fatal("artifact deletion without warning accepted")
	}
	exportRoot := filepath.Join(t.TempDir(), "archive")
	exported, err := service.ExportAssuranceArtifacts(context.Background(), ids[:2], exportRoot)
	if err != nil || !exported.Verified {
		t.Fatalf("export = %#v, %v", exported, err)
	}
	items, err := service.AssuranceArtifacts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	archived := 0
	for _, item := range items {
		if item.Spec.Retention == domain.ArtifactRetentionArchived {
			archived++
		}
	}
	if archived != 2 {
		t.Fatalf("archived = %d, %#v", archived, items)
	}
	deleted, err := service.DeleteAssuranceArtifact(context.Background(), ids[2], "DELETE")
	if err != nil || deleted.Spec.Retention != domain.ArtifactRetentionDeleted {
		t.Fatalf("deleted = %#v, %v", deleted, err)
	}
}

func TestEffectsAndUsageDashboardKeepLabelsUnknownFieldsAndHistoricalPricing(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Effects", Path: tempGitRepository(t, "effects")})
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
	invocation, err := service.RunAgentInvocation(context.Background(), AgentInvocationInput{SessionID: session.Metadata.ID, Provider: "fake", ProfileID: "fake", RequestedModel: "fixture-model"})
	if err != nil {
		t.Fatal(err)
	}
	now := invocation.Spec.StartedAt.Add(-time.Minute)
	snapshot := domain.ProviderPricingSnapshot{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ProviderPricingSnapshotKind}, Metadata: domain.ObjectMeta{ID: "price-fixture", Name: "Fixture pricing"}, Spec: domain.ProviderPricingSpec{Provider: "fake", Model: "fixture-model", OfficialURL: "https://example.invalid/pricing", Currency: "USD", InputPerMillion: 1, OutputPerMillion: 2, EffectiveAt: now, RetrievedAt: now, Status: "manual_review"}}
	if _, err := service.SavePricingSnapshot(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	dashboard, err := service.AssuranceDashboard(context.Background(), "fake", "fixture-model")
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.EstimatedCost == nil || dashboard.CostState != "estimated" || dashboard.CostLabel != "estimated public API list-price equivalent" {
		t.Fatalf("dashboard = %#v", dashboard)
	}
	missing, err := service.RunAgentInvocation(context.Background(), AgentInvocationInput{SessionID: session.Metadata.ID, Provider: "fake", ProfileID: "fake", RequestedModel: "unknown-model", Scenario: "missing_usage"})
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := service.AssuranceDashboard(context.Background(), "fake", "unknown-model")
	if err != nil {
		t.Fatal(err)
	}
	if unknown.EstimatedCost != nil || unknown.CostState != "unknown" || unknown.UsageComplete {
		t.Fatalf("unknown usage = %#v; invocation=%#v", unknown, missing)
	}
	effect, err := service.CreateEffect(context.Background(), EffectInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Fingerprint: "sha256:effect-fixture", Kind: domain.EffectMeasured, SourceRunID: invocation.Metadata.ID, Adopted: true, Reverified: true, Label: "measured"})
	if err != nil {
		t.Fatal(err)
	}
	if effect.Spec.Kind != domain.EffectMeasured {
		t.Fatalf("effect = %#v", effect)
	}
	if _, err := service.CreateEffect(context.Background(), EffectInput{ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", Fingerprint: "sha256:effect-fixture", Kind: domain.EffectMeasured, Label: "duplicate"}); err == nil {
		t.Fatal("duplicate measured effect accepted")
	}
	if _, err := service.SavePricingSnapshot(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.Spec.OutputPerMillion = 99
	if _, err := service.SavePricingSnapshot(context.Background(), snapshot); err == nil {
		t.Fatal("historical pricing snapshot mutated")
	}
}
