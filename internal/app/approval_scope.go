package app

import (
	"context"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

func (a *App) UnattendedApprovalScopes(ctx context.Context) ([]domain.UnattendedApprovalScope, error) {
	return a.store.ListUnattendedApprovalScopes(ctx)
}

func (a *App) UnattendedApprovalScope(ctx context.Context, id string) (domain.UnattendedApprovalScope, error) {
	if strings.TrimSpace(id) == "" {
		return domain.UnattendedApprovalScope{}, contract.InvalidInput("approval scope id is required")
	}
	return a.store.GetUnattendedApprovalScope(ctx, id)
}

func (a *App) CheckUnattendedApprovalScope(ctx context.Context, actionPlanID string) (domain.UnattendedApprovalMatch, error) {
	if strings.TrimSpace(actionPlanID) == "" {
		return domain.UnattendedApprovalMatch{}, contract.InvalidInput("action plan id is required")
	}
	plan, err := a.store.GetActionPlan(ctx, actionPlanID)
	if err != nil {
		return domain.UnattendedApprovalMatch{}, err
	}
	if plan.Spec.ApprovalScopeID == "" {
		return domain.UnattendedApprovalMatch{}, contract.InvalidInput("action plan has no unattended approval scope")
	}
	scope, err := a.store.GetUnattendedApprovalScope(ctx, plan.Spec.ApprovalScopeID)
	if err != nil {
		return domain.UnattendedApprovalMatch{}, err
	}
	match := scope.Match(plan.UnattendedApprovalRequest(), time.Now().UTC())
	if match.ScopeDigest != plan.Spec.ApprovalScopeDigest || !plan.Spec.ScopeMatch {
		match.Matched = false
		match.Reasons = appendUniqueStrings(match.Reasons, "persisted_plan_match_failed")
	}
	return match, nil
}

func (a *App) CreateUnattendedApprovalScope(ctx context.Context, input UnattendedApprovalScopeInput) (domain.UnattendedApprovalScope, error) {
	if err := a.validateApprovalScopeTarget(ctx, input.ProjectID, input.RepositoryID, input.WorktreeID); err != nil {
		return domain.UnattendedApprovalScope{}, err
	}
	now := time.Now().UTC()
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = assuranceID("approval-scope", input.ProjectID, input.RepositoryID, input.WorktreeID, input.ProviderProfile, input.Deadline)
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "Unattended approval scope"
	}
	scope := domain.UnattendedApprovalScope{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.UnattendedApprovalScopeKind},
		Metadata: domain.ObjectMeta{ID: id, Name: name},
		Spec: domain.UnattendedApprovalSpec{
			ProjectID:            strings.TrimSpace(input.ProjectID),
			RepositoryID:         strings.TrimSpace(input.RepositoryID),
			WorktreeID:           strings.TrimSpace(input.WorktreeID),
			ProviderProfile:      strings.TrimSpace(input.ProviderProfile),
			ActionTypes:          append([]string(nil), input.ActionTypes...),
			RiskClasses:          append([]domain.ActionRisk(nil), input.RiskClasses...),
			Techniques:           append([]string(nil), input.Techniques...),
			ToolSetup:            append([]string(nil), input.ToolSetup...),
			ToolVersion:          strings.TrimSpace(input.ToolVersion),
			ToolConfigDigest:     strings.TrimSpace(input.ToolConfigDigest),
			ArgumentSchemaDigest: strings.TrimSpace(input.ArgumentSchemaDigest),
			WritablePaths:        append([]string(nil), input.WritablePaths...),
			NetworkPolicy:        strings.TrimSpace(input.NetworkPolicy),
			DiskLimitBytes:       input.DiskLimitBytes,
			Deadline:             input.Deadline.UTC(),
			Prohibited:           append([]string(nil), input.Prohibited...),
			State:                domain.UnattendedScopeDraft,
			Revision:             1,
			CreatedAt:            now,
			UpdatedAt:            now,
		},
	}
	if err := a.store.SaveUnattendedApprovalScope(ctx, scope); err != nil {
		return domain.UnattendedApprovalScope{}, err
	}
	return scope, nil
}

func (a *App) ApproveUnattendedApprovalScope(ctx context.Context, id string) (domain.UnattendedApprovalScope, error) {
	scope, err := a.store.GetUnattendedApprovalScope(ctx, id)
	if err != nil {
		return domain.UnattendedApprovalScope{}, err
	}
	now := time.Now().UTC()
	if err := scope.ValidateForApprovalAt(now); err != nil {
		return domain.UnattendedApprovalScope{}, contract.InvalidInput(err.Error())
	}
	if err := a.validateApprovalScopeTarget(ctx, scope.Spec.ProjectID, scope.Spec.RepositoryID, scope.Spec.WorktreeID); err != nil {
		return domain.UnattendedApprovalScope{}, err
	}
	approvedAt := now
	scope.Spec.State = domain.UnattendedScopeApproved
	scope.Spec.ApprovedBy = "local-user"
	scope.Spec.ApprovedAt = &approvedAt
	scope.Spec.Revision++
	scope.Spec.UpdatedAt = now
	if err := a.store.UpdateUnattendedApprovalScope(ctx, scope); err != nil {
		return domain.UnattendedApprovalScope{}, err
	}
	return scope, nil
}

func (a *App) RevokeUnattendedApprovalScope(ctx context.Context, id string) (domain.UnattendedApprovalScope, error) {
	scope, err := a.store.GetUnattendedApprovalScope(ctx, id)
	if err != nil {
		return domain.UnattendedApprovalScope{}, err
	}
	if scope.Spec.State != domain.UnattendedScopeApproved {
		return domain.UnattendedApprovalScope{}, contract.InvalidInput("only an approved unattended scope can be revoked")
	}
	now := time.Now().UTC()
	scope.Spec.State = domain.UnattendedScopeRevoked
	scope.Spec.Revision++
	scope.Spec.UpdatedAt = now
	if err := a.store.UpdateUnattendedApprovalScope(ctx, scope); err != nil {
		return domain.UnattendedApprovalScope{}, err
	}
	return scope, nil
}

func (a *App) validateApprovalScopeTarget(ctx context.Context, projectID, repositoryID, worktreeID string) error {
	worktree, err := a.store.GetWorktree(ctx, projectID, repositoryID, worktreeID)
	if err != nil {
		return err
	}
	if _, err := domain.ExecutionContextForWorktree(worktree); err != nil {
		return contract.InvalidInput("approval scope requires a current observed Worktree")
	}
	return nil
}
