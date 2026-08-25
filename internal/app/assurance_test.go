package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/domain"
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
