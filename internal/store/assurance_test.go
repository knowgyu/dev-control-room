package store

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/masking"
)

func TestAssuranceLifecyclePersistsAdditiveObjectsAndRejectsDuplicateActiveSession(t *testing.T) {
	db := openTestDatabase(t, "assurance-lifecycle")
	persistence, err := New(db, masking.New(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	project := domain.NewProject("assurance", "Assurance", []domain.Repository{domain.NewRepository("repo", "Repo", t.TempDir())})
	if err := persistence.SaveProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := persistence.ReplaceWorktrees(ctx, "assurance", "repo", []domain.Worktree{{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.WorktreeKind}, Metadata: domain.ObjectMeta{ID: "primary", Name: "primary"}, Spec: domain.WorktreeSpec{ProjectID: "assurance", RepositoryID: "repo", CanonicalPath: project.Spec.Repositories[0].Spec.Path, PathFingerprint: "sha256:path", Trust: domain.WorktreeTrustVerifiedReadOnly, Primary: true, Head: "head", LastObserved: now}}}, true); err != nil {
		t.Fatal(err)
	}
	session := domain.AssuranceSession{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AssuranceSessionKind}, Metadata: domain.ObjectMeta{ID: "session-a", Name: "Session"}, Spec: domain.AssuranceSessionSpec{ProjectID: "assurance", RepositoryID: "repo", WorktreeID: "primary", Head: "head", State: domain.AssuranceStateDraft, CreatedAt: now, UpdatedAt: now}}
	if err := persistence.SaveAssuranceSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	if err := persistence.SaveAssuranceSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	items, err := persistence.ListAssuranceSessions(ctx)
	if err != nil || len(items) != 1 {
		t.Fatalf("sessions = %#v, %v", items, err)
	}
	if err := persistence.UpdateAssuranceRevision(ctx, domain.AssuranceSessionKind, session.Metadata.ID, 2, domain.AssuranceStateReady, now.Add(time.Second), session); err != nil {
		t.Fatal(err)
	}
	if err := persistence.UpdateAssuranceRevision(ctx, domain.AssuranceSessionKind, session.Metadata.ID, 2, domain.AssuranceStateDraft, now.Add(2*time.Second), session); err == nil {
		t.Fatal("stale assurance revision accepted")
	}
}

func TestQualityObjectivePersistsWithAssuranceRevisionCAS(t *testing.T) {
	db := openTestDatabase(t, "quality-objective")
	persistence, err := New(db, masking.New(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	path := t.TempDir()
	project := domain.NewProject("quality", "Quality", []domain.Repository{domain.NewRepository("repo", "Repo", path)})
	if err := persistence.SaveProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := persistence.ReplaceWorktrees(ctx, "quality", "repo", []domain.Worktree{{
		TypeMeta: TypeMetaForTest(domain.WorktreeKind),
		Metadata: domain.ObjectMeta{ID: "primary", Name: "primary"},
		Spec: domain.WorktreeSpec{
			ProjectID: "quality", RepositoryID: "repo", CanonicalPath: path,
			PathFingerprint: "sha256:path", AssociationFingerprint: "sha256:association",
			Trust: domain.WorktreeTrustVerifiedReadOnly, Primary: true, Head: "head", LastObserved: now,
		},
	}}, true); err != nil {
		t.Fatal(err)
	}
	objective := domain.QualityObjective{
		TypeMeta: TypeMetaForTest(domain.QualityObjectiveKind),
		Metadata: domain.ObjectMeta{ID: "objective-1", Name: "Improve quality"},
		Spec: domain.QualityObjectiveSpec{
			ProjectID: "quality", RepositoryID: "repo", WorktreeID: "primary", Head: "head",
			Owner: "owner", Title: "Improve quality", State: domain.QualityObjectiveStateDraft,
			Revision: 1, CreatedAt: now, UpdatedAt: now,
		},
	}
	if err := persistence.SaveQualityObjective(ctx, objective); err != nil {
		t.Fatal(err)
	}
	if err := persistence.SaveQualityObjective(ctx, objective); err != nil {
		t.Fatal(err)
	}
	items, err := persistence.ListQualityObjectives(ctx)
	if err != nil || len(items) != 1 {
		t.Fatalf("objectives = %#v, err = %v", items, err)
	}
	loaded, err := persistence.GetQualityObjective(ctx, objective.Metadata.ID)
	if err != nil || loaded.Spec.Head != "head" {
		t.Fatalf("loaded objective = %#v, err = %v", loaded, err)
	}
	objective.Spec.Revision = 2
	objective.Spec.State = domain.QualityObjectiveStateBaselinePending
	objective.Spec.UpdatedAt = now.Add(time.Second)
	if err := persistence.UpdateQualityObjectiveRevisionCAS(ctx, domain.QualityObjectiveKind, objective.Metadata.ID, 1, objective); err != nil {
		t.Fatal(err)
	}
	if err := persistence.UpdateQualityObjectiveRevisionCAS(ctx, domain.QualityObjectiveKind, objective.Metadata.ID, 1, objective); err == nil {
		t.Fatal("stale objective revision accepted")
	}
}

func TestUpdateAssuranceRevisionRejectsQualityObjectiveAndPreservesSnapshot(t *testing.T) {
	persistence, objective := newQualityObjectiveStoreFixture(t, "quality-objective-generic-update")
	updated := objective
	updated.Spec.Revision = 2
	updated.Spec.State = domain.QualityObjectiveStateBaselinePending
	updated.Spec.Title = "must not overwrite"
	updated.Spec.UpdatedAt = objective.Spec.UpdatedAt.Add(time.Second)

	err := persistence.UpdateAssuranceRevision(context.Background(), domain.QualityObjectiveKind, objective.Metadata.ID, updated.Spec.Revision, updated.Spec.State, updated.Spec.UpdatedAt, updated)
	if !errors.Is(err, ErrQualityObjectiveRequiresCAS) {
		t.Fatalf("generic objective update error = %v, want ErrQualityObjectiveRequiresCAS", err)
	}
	got, err := persistence.GetQualityObjective(context.Background(), objective.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Spec.Revision != objective.Spec.Revision || got.Spec.State != objective.Spec.State || got.Spec.Title != objective.Spec.Title {
		t.Fatalf("generic update changed objective = %#v, want original %#v", got, objective)
	}
}

func TestGetAssuranceWithRevisionReadsCampaignSnapshot(t *testing.T) {
	db := openTestDatabase(t, "quality-campaign-snapshot")
	persistence, err := New(db, masking.New(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	path := t.TempDir()
	project := domain.NewProject("project", "Project", []domain.Repository{domain.NewRepository("repository", "Repository", path)})
	if err := persistence.SaveProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := persistence.ReplaceWorktrees(context.Background(), "project", "repository", []domain.Worktree{{
		TypeMeta: TypeMetaForTest(domain.WorktreeKind),
		Metadata: domain.ObjectMeta{ID: "worktree", Name: "worktree"},
		Spec: domain.WorktreeSpec{
			ProjectID: "project", RepositoryID: "repository", CanonicalPath: path, PathFingerprint: "sha256:path",
			Trust: domain.WorktreeTrustVerifiedReadOnly, Primary: true, Head: "head", LastObserved: now,
		},
	}}, true); err != nil {
		t.Fatal(err)
	}
	campaign := domain.QualityCampaign{
		TypeMeta: TypeMetaForTest(domain.QualityCampaignKind),
		Metadata: domain.ObjectMeta{ID: "campaign-1", Name: "Coverage"},
		Spec: domain.QualityCampaignSpec{
			ProjectID: "project", RepositoryID: "repository", WorktreeID: "primary", Name: "Coverage",
			State: domain.AssuranceStateDraft, CreatedAt: now, UpdatedAt: now,
		},
	}
	if err := persistence.SaveQualityCampaign(context.Background(), campaign); err != nil {
		t.Fatal(err)
	}
	var loaded domain.QualityCampaign
	revision, err := persistence.GetAssuranceWithRevision(context.Background(), domain.QualityCampaignKind, campaign.Metadata.ID, &loaded)
	if err != nil || revision != 1 || loaded.Metadata.ID != campaign.Metadata.ID || loaded.Spec.Name != campaign.Spec.Name {
		t.Fatalf("campaign snapshot = revision %d, %#v, err=%v", revision, loaded, err)
	}
}

func TypeMetaForTest(kind string) domain.TypeMeta {
	return domain.TypeMeta{APIVersion: domain.APIVersion, Kind: kind}
}

func TestUpdateQualityObjectiveRevisionCASSucceedsExactlyOnce(t *testing.T) {
	persistence, objective := newQualityObjectiveStoreFixture(t, "quality-objective-cas-success")
	updated := objective
	updated.Spec.Title = "Updated quality objective"
	updated.Spec.State = domain.QualityObjectiveStateBaselinePending
	updated.Spec.Revision = 2
	updated.Spec.UpdatedAt = objective.Spec.UpdatedAt.Add(time.Second)

	if err := persistence.UpdateQualityObjectiveRevisionCAS(context.Background(), domain.QualityObjectiveKind, objective.Metadata.ID, 1, updated); err != nil {
		t.Fatal(err)
	}
	got, err := persistence.GetQualityObjective(context.Background(), objective.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Spec.Revision != 2 || got.Spec.State != updated.Spec.State || got.Spec.Title != updated.Spec.Title {
		t.Fatalf("updated objective = %#v, want %#v", got, updated)
	}
	if revision, err := persistence.AssuranceRevision(context.Background(), domain.QualityObjectiveKind, objective.Metadata.ID); err != nil || revision != 2 {
		t.Fatalf("stored revision = %d, err = %v", revision, err)
	}
}

func TestUpdateQualityObjectiveRevisionCASRejectsStaleRevisionWithoutOverwrite(t *testing.T) {
	persistence, objective := newQualityObjectiveStoreFixture(t, "quality-objective-cas-stale")
	winner := objective
	winner.Spec.Title = "Winner"
	winner.Spec.State = domain.QualityObjectiveStateBaselinePending
	winner.Spec.Revision = 2
	winner.Spec.UpdatedAt = objective.Spec.UpdatedAt.Add(time.Second)
	if err := persistence.UpdateQualityObjectiveRevisionCAS(context.Background(), domain.QualityObjectiveKind, objective.Metadata.ID, 1, winner); err != nil {
		t.Fatal(err)
	}

	stale := objective
	stale.Spec.Title = "Stale update"
	stale.Spec.State = domain.QualityObjectiveStateBlocked
	stale.Spec.Revision = 2
	stale.Spec.UpdatedAt = objective.Spec.UpdatedAt.Add(2 * time.Second)
	err := persistence.UpdateQualityObjectiveRevisionCAS(context.Background(), domain.QualityObjectiveKind, objective.Metadata.ID, 1, stale)
	if !errors.Is(err, ErrQualityObjectiveRevisionStale) {
		t.Fatalf("stale update error = %v, want ErrQualityObjectiveRevisionStale", err)
	}
	got, err := persistence.GetQualityObjective(context.Background(), objective.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Spec.Revision != winner.Spec.Revision || got.Spec.Title != winner.Spec.Title || got.Spec.State != winner.Spec.State {
		t.Fatalf("stale update overwrote objective = %#v, winner = %#v", got, winner)
	}
}

func TestUpdateQualityObjectiveRevisionCASDistinguishesMissingID(t *testing.T) {
	persistence, objective := newQualityObjectiveStoreFixture(t, "quality-objective-cas-missing")
	missing := objective
	missing.Metadata.ID = "missing-objective"
	missing.Spec.Revision = 2
	err := persistence.UpdateQualityObjectiveRevisionCAS(context.Background(), domain.QualityObjectiveKind, missing.Metadata.ID, 1, missing)
	if !errors.Is(err, ErrQualityObjectiveNotFound) {
		t.Fatalf("missing objective error = %v, want ErrQualityObjectiveNotFound", err)
	}
}

func TestUpdateQualityObjectiveRevisionCASRejectsWrongKindWithoutOverwrite(t *testing.T) {
	persistence, objective := newQualityObjectiveStoreFixture(t, "quality-objective-cas-kind")
	updated := objective
	updated.Spec.Title = "Wrong kind update"
	updated.Spec.Revision = 2
	updated.Spec.UpdatedAt = objective.Spec.UpdatedAt.Add(time.Second)
	err := persistence.UpdateQualityObjectiveRevisionCAS(context.Background(), domain.AssuranceSessionKind, objective.Metadata.ID, 1, updated)
	if !errors.Is(err, ErrQualityObjectiveKindMismatch) {
		t.Fatalf("wrong kind error = %v, want ErrQualityObjectiveKindMismatch", err)
	}
	got, err := persistence.GetQualityObjective(context.Background(), objective.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Spec.Revision != objective.Spec.Revision || got.Spec.Title != objective.Spec.Title {
		t.Fatalf("wrong kind update overwrote objective = %#v", got)
	}
}

func TestUpdateQualityObjectiveRevisionCASConcurrentAttemptsHaveOneWinner(t *testing.T) {
	persistence, objective := newQualityObjectiveStoreFixture(t, "quality-objective-cas-concurrent")
	first := objective
	first.Spec.Title = "Concurrent first"
	first.Spec.Description = "first complete JSON snapshot"
	first.Spec.State = domain.QualityObjectiveStateBaselinePending
	first.Spec.Revision = 2
	first.Spec.UpdatedAt = objective.Spec.UpdatedAt.Add(time.Second)
	second := objective
	second.Spec.Title = "Concurrent second"
	second.Spec.Description = "second complete JSON snapshot"
	second.Spec.State = domain.QualityObjectiveStateBlocked
	second.Spec.Revision = 2
	second.Spec.UpdatedAt = objective.Spec.UpdatedAt.Add(2 * time.Second)

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, candidate := range []domain.QualityObjective{first, second} {
		wg.Add(1)
		go func(item domain.QualityObjective) {
			defer wg.Done()
			<-start
			results <- persistence.UpdateQualityObjectiveRevisionCAS(context.Background(), domain.QualityObjectiveKind, objective.Metadata.ID, 1, item)
		}(candidate)
	}
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	stale := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrQualityObjectiveRevisionStale):
			stale++
		default:
			t.Fatalf("concurrent update error = %v", err)
		}
	}
	if successes != 1 || stale != 1 {
		t.Fatalf("concurrent results = successes %d, stale %d, want one each", successes, stale)
	}

	got, err := persistence.GetQualityObjective(context.Background(), objective.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	firstStored := got.Spec.Title == first.Spec.Title && got.Spec.Description == first.Spec.Description && got.Spec.State == first.Spec.State
	secondStored := got.Spec.Title == second.Spec.Title && got.Spec.Description == second.Spec.Description && got.Spec.State == second.Spec.State
	if got.Spec.Revision != 2 || (!firstStored && !secondStored) {
		t.Fatalf("concurrent final objective = %#v, want one complete winner at revision 2", got)
	}
}

func newQualityObjectiveStoreFixture(t *testing.T, name string) (*Store, domain.QualityObjective) {
	t.Helper()
	db := openTestDatabase(t, name)
	persistence, err := New(db, masking.New(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	path := t.TempDir()
	now := time.Now().UTC().Truncate(time.Microsecond)
	project := domain.NewProject("quality", "Quality", []domain.Repository{domain.NewRepository("repo", "Repo", path)})
	if err := persistence.SaveProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err := persistence.ReplaceWorktrees(context.Background(), "quality", "repo", []domain.Worktree{{
		TypeMeta: TypeMetaForTest(domain.WorktreeKind),
		Metadata: domain.ObjectMeta{ID: "primary", Name: "primary"},
		Spec: domain.WorktreeSpec{
			ProjectID: "quality", RepositoryID: "repo", CanonicalPath: path,
			PathFingerprint: "sha256:path", AssociationFingerprint: "sha256:association",
			Trust: domain.WorktreeTrustVerifiedReadOnly, Primary: true, Head: "head", LastObserved: now,
		},
	}}, true); err != nil {
		t.Fatal(err)
	}
	objective := domain.QualityObjective{
		TypeMeta: TypeMetaForTest(domain.QualityObjectiveKind),
		Metadata: domain.ObjectMeta{ID: "objective-1", Name: "Improve quality"},
		Spec: domain.QualityObjectiveSpec{
			ProjectID: "quality", RepositoryID: "repo", WorktreeID: "primary", Head: "head",
			Owner: "owner", Title: "Improve quality", State: domain.QualityObjectiveStateDraft,
			Revision: 1, CreatedAt: now, UpdatedAt: now,
		},
	}
	if err := persistence.SaveQualityObjective(context.Background(), objective); err != nil {
		t.Fatal(err)
	}
	return persistence, objective
}
