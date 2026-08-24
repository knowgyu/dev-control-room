package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
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
	if _, err := persistence.RecordFailureFingerprint(ctx, failure); err != nil {
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

func TestSafeguardRuleRoundTripPreservesLifecycleAndMetrics(t *testing.T) {
	db := openTestDatabase(t, "safeguard-rule-round-trip")
	persistence, err := New(db, masking.New(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	project := domain.NewProject("project-a", "Project A", []domain.Repository{domain.NewRepository("repo-a", "Repo A", "/tmp/repo-a")})
	if err := persistence.SaveProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	rule := domain.SafeguardRule{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.SafeguardRuleKind},
		Metadata: domain.ObjectMeta{ID: "safeguard-1", Name: "fixture safeguard"},
		Spec: domain.SafeguardRuleSpec{
			Fingerprint: "sha256:" + strings.Repeat("a", 64), Category: "checkset",
			ProjectID: "project-a", RepositoryID: "repo-a",
			State: domain.SafeguardShadow, Revision: 1, Owner: "local-user", OccurrenceCount: 3,
			FirstSeen: now.Add(-time.Hour), LastSeen: now, CreatedAt: now, UpdatedAt: now,
			Metrics: domain.SafeguardMetrics{Evaluations: 2, Hits: 1, Misses: 1, EvaluationCostUnits: 2},
		},
	}
	created, err := persistence.CreateSafeguardRule(context.Background(), rule)
	if err != nil || !created {
		t.Fatalf("create safeguard = %t, %v", created, err)
	}
	stored, err := persistence.GetSafeguardRule(context.Background(), rule.Metadata.ID)
	if err != nil || stored.Spec.Owner != "local-user" || stored.Spec.Metrics.Hits != 1 {
		t.Fatalf("stored safeguard = %#v, %v", stored, err)
	}
	updated := stored
	updated.Spec.Revision++
	updated.Spec.UpdatedAt = now.Add(time.Second)
	updated.Spec.Metrics.Evaluations++
	updated.Spec.Metrics.Misses++
	updated.Spec.Metrics.EvaluationCostUnits++
	changed, err := persistence.UpdateSafeguardRule(context.Background(), updated, stored.Spec.Revision)
	if err != nil || !changed {
		t.Fatalf("update safeguard = %t, %v", changed, err)
	}
	stale := stored
	stale.Spec.Revision++
	stale.Spec.UpdatedAt = now.Add(2 * time.Second)
	changed, err = persistence.UpdateSafeguardRule(context.Background(), stale, stored.Spec.Revision)
	if err != nil || changed {
		t.Fatalf("stale safeguard update = %t, %v", changed, err)
	}
	items, err := persistence.ListSafeguardRules(context.Background(), 10)
	if err != nil || len(items) != 1 || items[0].Metadata.ID != rule.Metadata.ID {
		t.Fatalf("safeguard list = %#v, %v", items, err)
	}
	if err := persistence.DeleteProject(context.Background(), project.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := persistence.GetSafeguardRule(context.Background(), rule.Metadata.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("project deletion left stale safeguard: %v", err)
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

func TestReplaceWorktreesRetainsMembershipAfterIncompleteEnumeration(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, "worktree-retention")
	s, err := New(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	project := domain.NewProject("project", "Project", []domain.Repository{domain.NewRepository("repo", "Repo", "/fixture")})
	if err := s.SaveProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	item := domain.Worktree{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.WorktreeKind}, Metadata: domain.ObjectMeta{ID: "primary", Name: "fixture"}, Spec: domain.WorktreeSpec{ProjectID: "project", RepositoryID: "repo", CanonicalPath: "/fixture", Trust: domain.WorktreeTrustVerifiedReadOnly, Primary: true, LastObserved: time.Now().UTC()}}
	if err := s.ReplaceWorktrees(ctx, "project", "repo", []domain.Worktree{item}, true); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceWorktrees(ctx, "project", "repo", nil, false); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListWorktrees(ctx, "project", "repo")
	if err != nil || len(items) != 1 || items[0].Spec.TombstonedAt != nil {
		t.Fatalf("incomplete enumeration removed membership: %#v %v", items, err)
	}
	if err := s.ReplaceWorktrees(ctx, "project", "repo", nil, true); err != nil {
		t.Fatal(err)
	}
	items, err = s.ListWorktrees(ctx, "project", "repo")
	if err != nil || items[0].Spec.TombstonedAt == nil {
		t.Fatalf("complete enumeration did not tombstone membership: %#v %v", items, err)
	}
}

func TestReplaceWorktreesPersistsUnverifiedAdvertisedMembership(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, "unverified-worktree")
	s, err := New(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveProject(ctx, domain.NewProject("project", "Project", []domain.Repository{domain.NewRepository("repo", "Repo", "/fixture")})); err != nil {
		t.Fatal(err)
	}
	item := domain.Worktree{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.WorktreeKind}, Metadata: domain.ObjectMeta{ID: "unverified-abc", Name: "unverified-abc"}, Spec: domain.WorktreeSpec{ProjectID: "project", RepositoryID: "repo", CanonicalPath: "sha256:abc", AssociationFingerprint: "unverified:abc", Trust: domain.WorktreeTrustUnverified, LastObserved: time.Now().UTC(), Prunable: true, Error: "worktree path is unavailable"}}
	if err := s.ReplaceWorktrees(ctx, "project", "repo", []domain.Worktree{item}, true); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListWorktrees(ctx, "project", "repo")
	if err != nil || len(items) != 1 || items[0].Spec.Trust != domain.WorktreeTrustUnverified {
		t.Fatalf("unverified membership was not retained: %#v %v", items, err)
	}
}

func TestReplaceWorktreesKeepsDurableIDAcrossTrustTransitions(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, "worktree-transition")
	s, _ := New(db, nil)
	if err := s.SaveProject(ctx, domain.NewProject("p", "P", []domain.Repository{domain.NewRepository("r", "R", "/fixture")})); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	verified := domain.Worktree{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.WorktreeKind}, Metadata: domain.ObjectMeta{ID: "linked-stable", Name: "linked-stable"}, Spec: domain.WorktreeSpec{ProjectID: "p", RepositoryID: "r", CanonicalPath: "/masked", PathFingerprint: "sha256:path", AssociationFingerprint: "sha256:association", Trust: domain.WorktreeTrustVerifiedReadOnly, LastObserved: now}}
	if err := s.ReplaceWorktrees(ctx, "p", "r", []domain.Worktree{verified}, true); err != nil {
		t.Fatal(err)
	}
	unverified := verified
	unverified.Metadata.ID = "unverified-temp"
	unverified.Spec.AssociationFingerprint = ""
	unverified.Spec.Trust = domain.WorktreeTrustUnverified
	unverified.Spec.Error = "unavailable"
	unverified.Spec.LastObserved = now.Add(time.Second)
	if err := s.ReplaceWorktrees(ctx, "p", "r", []domain.Worktree{unverified}, true); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListWorktrees(ctx, "p", "r")
	if err != nil || len(items) != 1 || items[0].Metadata.ID != "linked-stable" || items[0].Spec.Trust != domain.WorktreeTrustUnverified {
		t.Fatalf("transition lost durable identity: %#v %v", items, err)
	}
}

func TestWorktreeReconciliationDoesNotMergeDifferentVerifiedAssociations(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, "verified-association")
	s, _ := New(db, nil)
	_ = s.SaveProject(ctx, domain.NewProject("p", "P", []domain.Repository{domain.NewRepository("r", "R", "/fixture")}))
	now := time.Now().UTC()
	makeItem := func(id, assoc string) domain.Worktree {
		return domain.Worktree{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.WorktreeKind}, Metadata: domain.ObjectMeta{ID: id, Name: id}, Spec: domain.WorktreeSpec{ProjectID: "p", RepositoryID: "r", CanonicalPath: "/masked", PathFingerprint: "sha256:path", AssociationFingerprint: assoc, Trust: domain.WorktreeTrustVerifiedReadOnly, LastObserved: now}}
	}
	if err := s.ReplaceWorktrees(ctx, "p", "r", []domain.Worktree{makeItem("linked-a", "sha256:a")}, false); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceWorktrees(ctx, "p", "r", []domain.Worktree{makeItem("linked-b", "sha256:b")}, false); err != nil {
		t.Fatal(err)
	}
	items, _ := s.ListWorktrees(ctx, "p", "r")
	if len(items) != 2 {
		t.Fatalf("different verified associations merged: %#v", items)
	}
}

func TestPrimaryIncomingIdentityIsNeverReconciled(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, "primary-reconcile")
	s, _ := New(db, nil)
	_ = s.SaveProject(ctx, domain.NewProject("p", "P", []domain.Repository{domain.NewRepository("r", "R", "/fixture")}))
	now := time.Now().UTC()
	primary := domain.Worktree{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.WorktreeKind}, Metadata: domain.ObjectMeta{ID: "primary", Name: "primary"}, Spec: domain.WorktreeSpec{ProjectID: "p", RepositoryID: "r", CanonicalPath: "/masked", PathFingerprint: "sha256:new", Trust: domain.WorktreeTrustVerifiedReadOnly, Primary: true, LastObserved: now}}
	if err := s.ReplaceWorktrees(ctx, "p", "r", []domain.Worktree{primary}, true); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetWorktree(ctx, "p", "r", "primary")
	if !got.Spec.Primary {
		t.Fatal("primary was reconciled away")
	}
}

func TestWorktreeIdentitySurvivesTrustRecoveryAssociationChangeAndRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	persistence, err := New(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	project := domain.NewProject("project", "Project", []domain.Repository{domain.NewRepository("repo", "Repo", "/fixture")})
	if err := persistence.SaveProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	unverified := domain.Worktree{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.WorktreeKind}, Metadata: domain.ObjectMeta{ID: "unverified-temporary", Name: "unverified-temporary"}, Spec: domain.WorktreeSpec{ProjectID: "project", RepositoryID: "repo", CanonicalPath: "/masked", PathFingerprint: "sha256:same-path", AssociationFingerprint: "unverified:sha256:same-path", Trust: domain.WorktreeTrustUnverified, LastObserved: at}}
	if err := persistence.ReplaceWorktrees(ctx, "project", "repo", []domain.Worktree{unverified}, true); err != nil {
		t.Fatal(err)
	}
	if err := persistence.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	persistence, err = New(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer persistence.Close()
	verified := unverified
	verified.Metadata.ID = "linked-first-association"
	verified.Spec.AssociationFingerprint = "sha256:first-association"
	verified.Spec.Trust = domain.WorktreeTrustVerifiedReadOnly
	verified.Spec.LastObserved = at.Add(time.Second)
	if err := persistence.ReplaceWorktrees(ctx, "project", "repo", []domain.Worktree{verified}, true); err != nil {
		t.Fatal(err)
	}
	items, err := persistence.ListWorktrees(ctx, "project", "repo")
	if err != nil || len(items) != 2 || items[0].Metadata.ID != "linked-first-association" || items[0].Spec.TombstonedAt != nil || items[1].Metadata.ID != "unverified-temporary" || items[1].Spec.TombstonedAt == nil {
		t.Fatalf("verification did not supersede the provisional identity: %#v %v", items, err)
	}
	verified.Metadata.ID = "linked-second-association"
	verified.Spec.AssociationFingerprint = "sha256:second-association"
	verified.Spec.LastObserved = at.Add(2 * time.Second)
	if err := persistence.ReplaceWorktrees(ctx, "project", "repo", []domain.Worktree{verified}, true); err != nil {
		t.Fatal(err)
	}
	items, err = persistence.ListWorktrees(ctx, "project", "repo")
	states := map[string]*time.Time{}
	for i := range items {
		states[items[i].Metadata.ID] = items[i].Spec.TombstonedAt
	}
	if err != nil || len(items) != 3 || states["linked-second-association"] != nil || states["linked-first-association"] == nil || states["unverified-temporary"] == nil {
		t.Fatalf("different verified association was not a new identity: %#v %v", items, err)
	}
}

func TestProbeFailedAssociatedWorktreeUsesAssociationNotPath(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, "associated-unverified")
	s, _ := New(db, nil)
	_ = s.SaveProject(ctx, domain.NewProject("p", "P", []domain.Repository{domain.NewRepository("r", "R", "/fixture")}))
	now := time.Now().UTC()
	item := func(id, assoc string, trust string) domain.Worktree {
		return domain.Worktree{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.WorktreeKind}, Metadata: domain.ObjectMeta{ID: id, Name: id}, Spec: domain.WorktreeSpec{ProjectID: "p", RepositoryID: "r", CanonicalPath: "/masked", PathFingerprint: "sha256:path", AssociationFingerprint: assoc, Trust: trust, LastObserved: now}}
	}
	if err := s.ReplaceWorktrees(ctx, "p", "r", []domain.Worktree{item("linked-a", "sha256:a", domain.WorktreeTrustVerifiedReadOnly)}, true); err != nil {
		t.Fatal(err)
	}
	failed := item("linked-b", "sha256:b", domain.WorktreeTrustUnverified)
	failed.Spec.Error = "state unavailable"
	if err := s.ReplaceWorktrees(ctx, "p", "r", []domain.Worktree{failed}, true); err != nil {
		t.Fatal(err)
	}
	items, _ := s.ListWorktrees(ctx, "p", "r")
	if len(items) != 2 {
		t.Fatalf("different association merged by path: %#v", items)
	}
	failed.Metadata.ID = "temporary"
	failed.Spec.AssociationFingerprint = "sha256:b"
	failed.Spec.LastObserved = now.Add(time.Second)
	if err := s.ReplaceWorktrees(ctx, "p", "r", []domain.Worktree{failed}, true); err != nil {
		t.Fatal(err)
	}
	items, _ = s.ListWorktrees(ctx, "p", "r")
	if len(items) != 2 {
		t.Fatalf("same associated probe failure changed identity: %#v", items)
	}
}
