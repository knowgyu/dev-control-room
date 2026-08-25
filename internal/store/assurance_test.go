package store

import (
	"context"
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
