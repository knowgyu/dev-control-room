package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestUnattendedApprovalScopePersistsWithRevisionAndTargetIndex(t *testing.T) {
	db := openTestDatabase(t, "approval-scope-persistence")
	persistence, err := New(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 27, 1, 0, 0, 0, time.UTC)
	scope := storeApprovalScope(t, now)
	ctx := context.Background()
	project := domain.NewProject("project-a", "Project A", []domain.Repository{domain.NewRepository("repo-a", "Repo A", scope.Spec.WritablePaths[0])})
	if err := persistence.SaveProject(ctx, project); err != nil {
		t.Fatal(err)
	}
	worktree := domain.Worktree{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.WorktreeKind}, Metadata: domain.ObjectMeta{ID: "primary", Name: "Primary"}, Spec: domain.WorktreeSpec{ProjectID: "project-a", RepositoryID: "repo-a", CanonicalPath: scope.Spec.WritablePaths[0], PathFingerprint: storeApprovalDigest, Trust: domain.WorktreeTrustVerifiedReadOnly, Primary: true, Head: "head", LastObserved: now}}
	if err := persistence.ReplaceWorktrees(ctx, "project-a", "repo-a", []domain.Worktree{worktree}, true); err != nil {
		t.Fatal(err)
	}
	if err := persistence.SaveUnattendedApprovalScope(ctx, scope); err != nil {
		t.Fatal(err)
	}
	var indexName string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'idx_unattended_scope_target'`).Scan(&indexName); err != nil {
		t.Fatalf("approval scope target index is missing: %v", err)
	}
	if indexName != "idx_unattended_scope_target" {
		t.Fatalf("unexpected index name %q", indexName)
	}
	got, err := persistence.GetUnattendedApprovalScope(ctx, scope.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Spec.Revision != 1 || got.Spec.State != domain.UnattendedScopeDraft || got.Spec.ProjectID != scope.Spec.ProjectID {
		t.Fatalf("persisted scope = %#v", got)
	}
	items, err := persistence.ListUnattendedApprovalScopes(ctx)
	if err != nil || len(items) != 1 || items[0].Metadata.ID != scope.Metadata.ID {
		t.Fatalf("listed scopes = %#v, %v", items, err)
	}

	approvedAt := now.Add(time.Second)
	updated := got
	updated.Spec.State = domain.UnattendedScopeApproved
	updated.Spec.ApprovedBy = "local-user"
	updated.Spec.ApprovedAt = &approvedAt
	updated.Spec.Revision = 2
	updated.Spec.UpdatedAt = approvedAt
	if err := persistence.UpdateUnattendedApprovalScope(ctx, updated); err != nil {
		t.Fatal(err)
	}
	got, err = persistence.GetUnattendedApprovalScope(ctx, scope.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Spec.State != domain.UnattendedScopeApproved || got.Spec.Revision != 2 || got.Spec.ApprovedBy != "local-user" {
		t.Fatalf("updated scope = %#v", got)
	}
	if err := persistence.UpdateUnattendedApprovalScope(ctx, updated); err == nil {
		t.Fatal("stale approval scope revision was accepted")
	}

	mutated := scope
	mutated.Spec.ProviderProfile = "other-profile"
	if err := persistence.SaveUnattendedApprovalScope(ctx, mutated); err == nil {
		t.Fatal("mutable duplicate approval scope was accepted")
	}
	if _, err := persistence.GetUnattendedApprovalScope(ctx, "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing approval scope = %v", err)
	}
}

func storeApprovalScope(t *testing.T, now time.Time) domain.UnattendedApprovalScope {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo")
	return domain.UnattendedApprovalScope{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.UnattendedApprovalScopeKind},
		Metadata: domain.ObjectMeta{ID: "scope-store", Name: "Stored scope"},
		Spec: domain.UnattendedApprovalSpec{
			ProjectID:            "project-a",
			RepositoryID:         "repo-a",
			WorktreeID:           "primary",
			ProviderProfile:      "codex",
			ActionTypes:          []string{"repository.refresh"},
			RiskClasses:          []domain.ActionRisk{domain.RiskSafeLocal},
			Techniques:           []string{domain.QualityTechniqueStaticSecurity},
			ToolVersion:          "go1.26.7",
			ToolConfigDigest:     storeApprovalDigest,
			ArgumentSchemaDigest: storeApprovalDigest,
			WritablePaths:        []string{root},
			NetworkPolicy:        domain.NetworkPolicyAllowlist,
			DiskLimitBytes:       1 << 20,
			Deadline:             now.Add(time.Hour),
			Prohibited:           []string{"ci_edit", "commit", "delete", "pull_request", "push", "remote_dispatch", "scope_expansion"},
			State:                domain.UnattendedScopeDraft,
			Revision:             1,
			CreatedAt:            now,
			UpdatedAt:            now,
		},
	}
}

const storeApprovalDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
