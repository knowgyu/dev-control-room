package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/assurance"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/masking"
)

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
		{name: "prompt required", profileID: "codex", status: assurance.ProviderStatus{Provider: "codex", State: assurance.ProviderReady, CommandFound: true, LaunchTrusted: true, ProfileReady: true, ResolvedCommand: []string{`C:\Program Files\nodejs\node.exe`, `C:\Users\fixture\node_modules\@openai\codex\bin\codex.js`}}, runner: func(context.Context, assurance.RunRequest, *masking.Masker) assurance.RunResult {
			return assurance.RunResult{State: domain.AssuranceStateSucceeded, Structured: map[string]any{"unexpected": true}}
		}, wantCode: "provider.prompt_required"},
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
			invocation, err := service.RunAgentInvocation(ctx, AgentInvocationInput{SessionID: session.Metadata.ID, Provider: "codex", ProfileID: testCase.profileID, RequestedModel: "fixture-model"})
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

func TestQualityRunUsesTypedRunnerAndPersistsBoundedReport(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Quality", Path: tempGitRepository(t, "quality")})
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
	if run.Spec.State != domain.AssuranceStateSucceeded || len(run.Spec.ArtifactIDs) != 1 || run.Spec.Command.Executable != "git" {
		t.Fatalf("quality run = %#v", run)
	}
	artifacts, err := service.AssuranceArtifacts(context.Background())
	if err != nil || len(artifacts) != 1 {
		t.Fatalf("artifacts = %#v, %v", artifacts, err)
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
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Techniques", Path: tempGitRepository(t, "techniques")})
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
	techniques := []string{domain.QualityTechniqueStaticSecurity, domain.QualityTechniqueMutation, domain.QualityTechniqueProperty, domain.QualityTechniqueFuzz, domain.QualityTechniqueTargetedE2E}
	ids := make([]string, 0, len(techniques))
	for _, technique := range techniques {
		run, runErr := service.RunQuality(context.Background(), QualityRunInput{CampaignID: campaign.Metadata.ID, Technique: technique, Provider: "fake"})
		if runErr != nil {
			t.Fatal(technique, runErr)
		}
		if len(run.Spec.ArtifactIDs) != 1 {
			t.Fatalf("%s artifacts = %#v", technique, run.Spec.ArtifactIDs)
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
