package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/masking"
)

func TestTypedRepositoriesPreserveScopedIdentityAndHistory(t *testing.T) {
	db := openTestDatabase(t, "typed-repositories")
	persistence, err := New(db, masking.New(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	projectA := domain.NewProject("project-a", "Project A", []domain.Repository{domain.NewRepository("backend", "Backend", "/tmp/fixture-a")})
	projectB := domain.NewProject("project-b", "Project B", []domain.Repository{domain.NewRepository("backend", "Backend", "/tmp/fixture-b")})
	if err := persistence.SaveProject(ctx, projectA); err != nil {
		t.Fatal(err)
	}
	if err := persistence.SaveProject(ctx, projectB); err != nil {
		t.Fatal(err)
	}
	for _, projectID := range []string{"project-a", "project-b"} {
		repository, err := persistence.GetRepository(ctx, projectID, "backend")
		if err != nil || repository.Spec.Path == "" {
			t.Fatalf("repository identity was not project-scoped: %s %#v %v", projectID, repository, err)
		}
	}
	now := time.Now().UTC()
	observation := domain.Observation{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ObservationKind}, Metadata: domain.ObjectMeta{ID: "observation-1", Name: "fixture"}, Spec: domain.ObservationSpec{ProjectID: "project-a", RepositoryID: "backend", Collector: "test", ObservationType: "fixture", Fingerprint: "sha256:observation", CollectedAt: now, Evidence: map[string]any{"safe": true}}}
	if err := persistence.SaveObservation(ctx, observation); err != nil {
		t.Fatal(err)
	}
	finding := domain.Finding{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.FindingKind}, Metadata: domain.ObjectMeta{ID: "finding-1", Name: "fixture finding"}, Spec: domain.FindingSpec{ProjectID: "project-a", RepositoryID: "backend", FindingType: domain.FindingDirty, Fingerprint: "sha256:finding", Severity: domain.SeverityAttention, Confidence: domain.ConfidenceConfirmed, Summary: "fixture", RecommendedNext: "review", FirstObserved: now, LastObserved: now, State: domain.FindingOpen}}
	if err := persistence.SaveFinding(ctx, finding); err != nil {
		t.Fatal(err)
	}
	completed := now.Add(time.Second)
	run := domain.ScanRun{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ScanRunKind}, Metadata: domain.ObjectMeta{ID: "scan-1", Name: "fixture scan"}, Spec: domain.ScanRunSpec{ProjectID: "project-a", Trigger: "manual", Status: domain.ScanSucceeded, StartedAt: now, CompletedAt: &completed}}
	if err := persistence.SaveScanRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	event := domain.Event{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.EventKind}, Metadata: domain.ObjectMeta{ID: "event-1", Name: "fixture event"}, Spec: domain.EventSpec{EventType: "fixture", ProjectID: "project-a", RepositoryID: "backend", Summary: "fixture", OccurredAt: now}}
	if err := persistence.SaveEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	failure := domain.FailureFingerprint{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.FailureFingerprintKind}, Metadata: domain.ObjectMeta{ID: "failure-1", Name: "fixture failure"}, Spec: domain.FailureFingerprintSpec{Fingerprint: "sha256:failure", Category: "fixture", FirstSeen: now, LastSeen: now, OccurrenceCount: 1}}
	if err := persistence.SaveFailureFingerprint(ctx, failure); err != nil {
		t.Fatal(err)
	}
	if _, err := persistence.LatestScanRun(ctx, "project-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := persistence.GetFailureFingerprint(ctx, "sha256:failure"); err != nil {
		t.Fatal(err)
	}
	if err := persistence.DeleteProject(ctx, "project-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := persistence.GetRepository(ctx, "project-a", "backend"); err == nil || err != sql.ErrNoRows {
		t.Fatalf("project deletion did not cascade repository: %v", err)
	}
}

func TestSaveProjectRemovesOnlyRepositoriesOmittedFromAggregate(t *testing.T) {
	db := openTestDatabase(t, "project-repository-reconciliation")
	persistence, err := New(db, masking.New(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	project := domain.NewProject("project-a", "Project A", []domain.Repository{
		domain.NewRepository("keep", "Keep", "/tmp/keep"),
		domain.NewRepository("remove", "Remove", "/tmp/remove"),
	})
	if err := persistence.SaveProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	project.Spec.Repositories = project.Spec.Repositories[:1]
	if err := persistence.SaveProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	if _, err := persistence.GetRepository(ctx, project.Metadata.ID, "keep"); err != nil {
		t.Fatalf("retained repository was removed: %v", err)
	}
	if _, err := persistence.GetRepository(ctx, project.Metadata.ID, "remove"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("omitted repository survived aggregate save: %v", err)
	}
}
