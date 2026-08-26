package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/assurance"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
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
