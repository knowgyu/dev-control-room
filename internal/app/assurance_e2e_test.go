package app

import (
	"context"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestAssuranceDogfoodThreeRepositoryFixture(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	frontend := tempGoGitRepository(t, "frontend")
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Dev Control Room Fixture", Path: frontend})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ name, path string }{{"backend", tempGoGitRepository(t, "backend")}, {"database", tempGoGitRepository(t, "database")}} {
		if _, err := service.AddRepository(context.Background(), AddRepositoryInput{ProjectID: project.Metadata.ID, ID: item.name, Name: item.name, Path: item.path}); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.RunScan(context.Background(), "dogfood"); err != nil {
		t.Fatal(err)
	}
	repositories, err := service.Repositories(context.Background(), project.Metadata.ID)
	if err != nil || len(repositories) != 3 {
		t.Fatalf("repositories = %#v, %v", repositories, err)
	}
	for _, repository := range repositories {
		session, sessionErr := service.CreateAssuranceSession(context.Background(), AssuranceSessionInput{ProjectID: project.Metadata.ID, RepositoryID: repository.Metadata.ID, WorktreeID: "primary", Provider: "fake", RequestedModel: "fixture-model"})
		if sessionErr != nil {
			t.Fatal(sessionErr)
		}
		if _, err := service.CreatePRCIBaseline(context.Background(), BaselineInput{ProjectID: project.Metadata.ID, RepositoryID: repository.Metadata.ID, WorktreeID: "primary", TargetBranch: "main"}); err != nil {
			t.Fatal(err)
		}
		if _, err := service.RunAgentInvocation(context.Background(), AgentInvocationInput{SessionID: session.Metadata.ID, Provider: "fake", ProfileID: "fake", RequestedModel: "fixture-model"}); err != nil {
			t.Fatal(err)
		}
		campaign, err := service.CreateQualityCampaign(context.Background(), QualityCampaignInput{ProjectID: project.Metadata.ID, RepositoryID: repository.Metadata.ID, WorktreeID: "primary", Name: repository.Metadata.Name + " assurance", SessionID: session.Metadata.ID})
		if err != nil {
			t.Fatal(err)
		}
		run, err := service.RunQuality(context.Background(), QualityRunInput{CampaignID: campaign.Metadata.ID, Technique: domain.QualityTechniqueStaticSecurity, Provider: "fake", Model: "fixture-model"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.CreateEffect(context.Background(), EffectInput{ProjectID: project.Metadata.ID, RepositoryID: repository.Metadata.ID, WorktreeID: "primary", Fingerprint: "sha256:" + repository.Metadata.ID + "-effect", Kind: domain.EffectMeasured, SourceRunID: run.Metadata.ID, EvidenceIDs: run.Spec.ArtifactIDs, Adopted: true, Reverified: true, Label: "measured"}); err != nil {
			t.Fatal(err)
		}
	}
	dashboard, err := service.AssuranceDashboard(context.Background(), "fake", "fixture-model")
	if err != nil {
		t.Fatal(err)
	}
	if len(dashboard.Invocations) != 3 || len(dashboard.Effects) != 3 {
		t.Fatalf("dogfood dashboard = %#v", dashboard)
	}
}
