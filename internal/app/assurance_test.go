package app

import (
	"context"
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
