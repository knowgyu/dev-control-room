package action

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestBrokerPersistsExactApprovalScopeMatchAndRevalidatesExecution(t *testing.T) {
	broker, persistence, now := actionFixture(t)
	ctx := context.Background()
	scope := actionApprovalScope(*now)
	if err := persistence.SaveUnattendedApprovalScope(ctx, scope); err != nil {
		t.Fatal(err)
	}

	plan, err := broker.Plan(ctx, actionApprovalPlanRequest(scope.Metadata.ID, "plan-scoped", *now))
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Spec.ScopeMatch || len(plan.Spec.ScopeMatchReasons) != 1 || plan.Spec.ScopeMatchReasons[0] != "exact_match" {
		t.Fatalf("scope match evidence = %#v", plan.Spec)
	}
	if plan.Spec.ApprovalScopeDigest == "" || plan.Spec.ScopeCheckedAt.IsZero() {
		t.Fatalf("scope evidence is incomplete: %#v", plan.Spec)
	}

	admission, err := broker.Admit(ctx, plan.Metadata.ID, "scope-holder", "scope-request")
	if err != nil {
		t.Fatal(err)
	}
	approvedAt := *now
	scope.Spec.State = domain.UnattendedScopeRevoked
	scope.Spec.Revision = 3
	scope.Spec.UpdatedAt = approvedAt.Add(time.Second)
	if err := persistence.UpdateUnattendedApprovalScope(ctx, scope); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.Execute(ctx, admission); !errors.Is(err, ErrApprovalScopeMismatch) {
		t.Fatalf("revoked scope execution = %v", err)
	}
	var lockCount int
	if err := persistence.DB().QueryRow(`SELECT COUNT(*) FROM action_locks WHERE action_plan_id = ?`, plan.Metadata.ID).Scan(&lockCount); err != nil {
		t.Fatal(err)
	}
	if lockCount != 0 {
		t.Fatalf("execution rejection left %d action locks", lockCount)
	}
}

func TestBrokerPersistsApprovalScopeMismatchAsAuditablePlanEvidence(t *testing.T) {
	broker, persistence, now := actionFixture(t)
	ctx := context.Background()
	scope := actionApprovalScope(*now)
	if err := persistence.SaveUnattendedApprovalScope(ctx, scope); err != nil {
		t.Fatal(err)
	}
	request := actionApprovalPlanRequest(scope.Metadata.ID, "plan-mismatch", *now)
	request.ProviderProfile = "other-profile"
	plan, err := broker.Plan(ctx, request)
	if !errors.Is(err, ErrApprovalScopeMismatch) {
		t.Fatalf("provider mismatch error = %v", err)
	}
	if plan.Metadata.ID != request.ID || plan.Spec.ScopeMatch || !hasApprovalScopeString(plan.Spec.ScopeMatchReasons, "provider_profile_mismatch") {
		t.Fatalf("mismatch evidence = %#v", plan)
	}
	persisted, err := persistence.GetActionPlan(ctx, plan.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Spec.ScopeMatch || !hasApprovalScopeString(persisted.Spec.ScopeMatchReasons, "provider_profile_mismatch") {
		t.Fatalf("persisted mismatch evidence = %#v", persisted.Spec)
	}
}

func actionApprovalScope(now time.Time) domain.UnattendedApprovalScope {
	approvedAt := now.Add(-time.Second)
	return domain.UnattendedApprovalScope{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.UnattendedApprovalScopeKind},
		Metadata: domain.ObjectMeta{ID: "scope-action", Name: "Action scope"},
		Spec: domain.UnattendedApprovalSpec{
			ProjectID:            "project",
			RepositoryID:         "repo",
			WorktreeID:           "primary",
			ProviderProfile:      "codex",
			ActionTypes:          []string{"repository.refresh"},
			RiskClasses:          []domain.ActionRisk{domain.RiskSafeLocal},
			Techniques:           []string{domain.QualityTechniqueStaticSecurity},
			ToolVersion:          "go1.26.7",
			ToolConfigDigest:     actionApprovalDigest,
			ArgumentSchemaDigest: actionApprovalDigest,
			WritablePaths:        []string{"C:/fixture"},
			NetworkPolicy:        domain.NetworkPolicyAllowlist,
			DiskLimitBytes:       1 << 20,
			Deadline:             now.Add(time.Hour),
			Prohibited:           []string{"ci_edit", "commit", "delete", "pull_request", "push", "remote_dispatch", "scope_expansion"},
			State:                domain.UnattendedScopeApproved,
			ApprovedBy:           "local-user",
			ApprovedAt:           &approvedAt,
			Revision:             2,
			CreatedAt:            now.Add(-2 * time.Second),
			UpdatedAt:            approvedAt,
		},
	}
}

func actionApprovalPlanRequest(scopeID, planID string, now time.Time) PlanRequest {
	return PlanRequest{
		ID:                   planID,
		Name:                 "Scoped refresh",
		ProjectID:            "project",
		RepositoryID:         "repo",
		WorktreeID:           "primary",
		ActionType:           "repository.refresh",
		RequestedBy:          domain.Actor{Kind: domain.ActorSystem, ID: "adapter"},
		ApprovalScopeID:      scopeID,
		ProviderProfile:      "codex",
		Techniques:           []string{domain.QualityTechniqueStaticSecurity},
		ToolVersion:          "go1.26.7",
		ToolConfigDigest:     actionApprovalDigest,
		ArgumentSchemaDigest: actionApprovalDigest,
		WritablePaths:        []string{"C:/fixture/internal"},
		NetworkPolicy:        domain.NetworkPolicyAllowlist,
		DiskLimitBytes:       1024,
		ScopeDeadline:        now.Add(30 * time.Minute),
		ProhibitedOperations: []string{"ci_edit", "commit", "delete", "pull_request", "push", "remote_dispatch", "scope_expansion"},
	}
}

const actionApprovalDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

func hasApprovalScopeString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
