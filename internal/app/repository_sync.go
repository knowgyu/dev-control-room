package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/knowgyu/dev-control-room/internal/collector"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

const repositorySyncAction = "repository.sync"

// RepositorySyncPlan is a reviewable batch of per-repository Action plans.
// Skipped targets are retained so a user can fix the reason and plan again.
type RepositorySyncPlan struct {
	ProjectID string               `json:"projectId"`
	Plans     []domain.ActionPlan  `json:"plans"`
	Skipped   []RepositorySyncSkip `json:"skipped"`
	CreatedAt time.Time            `json:"createdAt"`
}

type RepositorySyncSkip struct {
	RepositoryID   string `json:"repositoryId"`
	RepositoryName string `json:"repositoryName"`
	WorktreeID     string `json:"worktreeId,omitempty"`
	Code           string `json:"code"`
	Reason         string `json:"reason"`
}

type ExecuteRepositorySyncInput struct {
	ProjectID string   `json:"projectId"`
	PlanIDs   []string `json:"planIds"`
	RequestID string   `json:"requestId"`
}

type RepositorySyncResult struct {
	ProjectID string                  `json:"projectId"`
	Outcomes  []RepositorySyncOutcome `json:"outcomes"`
}

type RepositorySyncOutcome struct {
	PlanID       string            `json:"planId"`
	RepositoryID string            `json:"repositoryId"`
	WorktreeID   string            `json:"worktreeId"`
	Run          *domain.ActionRun `json:"run,omitempty"`
	Error        string            `json:"error,omitempty"`
}

func (a *App) RepositorySyncPlan(ctx context.Context, projectID string) (RepositorySyncPlan, error) {
	project, err := a.Project(ctx, projectID)
	if err != nil {
		return RepositorySyncPlan{}, err
	}
	createdAt := time.Now().UTC()
	result := RepositorySyncPlan{ProjectID: projectID, Plans: make([]domain.ActionPlan, 0, len(project.Spec.Repositories)), Skipped: make([]RepositorySyncSkip, 0), CreatedAt: createdAt}
	for _, repository := range project.Spec.Repositories {
		worktrees, listErr := a.store.ListWorktrees(ctx, projectID, repository.Metadata.ID)
		if listErr != nil {
			return RepositorySyncPlan{}, listErr
		}
		worktree, reason := repositorySyncWorktree(worktrees)
		if reason != nil {
			result.Skipped = append(result.Skipped, RepositorySyncSkip{RepositoryID: repository.Metadata.ID, RepositoryName: repository.Metadata.Name, WorktreeID: reason.worktreeID, Code: reason.code, Reason: reason.message})
			continue
		}
		plan, planErr := a.PlanAction(ctx, ActionPlanInput{ID: repositorySyncPlanID(projectID, repository.Metadata.ID, worktree.Metadata.ID, createdAt), Name: repository.Metadata.Name + " 저장소 최신화", ProjectID: projectID, RepositoryID: repository.Metadata.ID, WorktreeID: worktree.Metadata.ID, ActionType: repositorySyncAction})
		if planErr != nil {
			result.Skipped = append(result.Skipped, RepositorySyncSkip{RepositoryID: repository.Metadata.ID, RepositoryName: repository.Metadata.Name, WorktreeID: worktree.Metadata.ID, Code: "action_plan_failed", Reason: "검토된 최신화 계획을 만들 수 없습니다."})
			continue
		}
		result.Plans = append(result.Plans, plan)
	}
	return result, nil
}

func (a *App) ExecuteRepositorySync(ctx context.Context, input ExecuteRepositorySyncInput) (RepositorySyncResult, error) {
	if strings.TrimSpace(input.ProjectID) == "" || strings.TrimSpace(input.RequestID) == "" || len(input.PlanIDs) == 0 || len(input.PlanIDs) > 32 {
		return RepositorySyncResult{}, contract.InvalidInput("projectId, requestId, and one to 32 planIds are required")
	}
	if _, err := a.Project(ctx, input.ProjectID); err != nil {
		return RepositorySyncResult{}, err
	}
	seen := make(map[string]struct{}, len(input.PlanIDs))
	plans := make([]domain.ActionPlan, len(input.PlanIDs))
	for index, planID := range input.PlanIDs {
		if strings.TrimSpace(planID) == "" {
			return RepositorySyncResult{}, contract.InvalidInput("planIds cannot contain empty values")
		}
		if _, ok := seen[planID]; ok {
			return RepositorySyncResult{}, contract.InvalidInput("planIds must be unique")
		}
		seen[planID] = struct{}{}
		plan, err := a.store.GetActionPlan(ctx, planID)
		if errors.Is(err, sql.ErrNoRows) {
			return RepositorySyncResult{}, contract.NotFound("repository sync plan not found")
		}
		if err != nil {
			return RepositorySyncResult{}, err
		}
		if plan.Spec.ProjectID != input.ProjectID || plan.Spec.ActionType != repositorySyncAction {
			return RepositorySyncResult{}, contract.InvalidInput("planIds must reference repository sync plans in the selected project")
		}
		plans[index] = plan
	}

	result := RepositorySyncResult{ProjectID: input.ProjectID, Outcomes: make([]RepositorySyncOutcome, len(plans))}
	sem := make(chan struct{}, 2) // ponytail: two local Git processes; raise only after measuring Windows pressure.
	var wait sync.WaitGroup
	for index, plan := range plans {
		index, plan := index, plan
		wait.Add(1)
		go func() {
			defer wait.Done()
			outcome := RepositorySyncOutcome{PlanID: plan.Metadata.ID, RepositoryID: plan.Spec.RepositoryID, WorktreeID: plan.Spec.WorktreeID}
			sem <- struct{}{}
			defer func() { <-sem }()
			if _, err := a.TrustActionWorktree(ctx, plan.Metadata.ID); err != nil {
				outcome.Error = "실행 대상 Worktree를 확인하지 못했습니다."
				result.Outcomes[index] = outcome
				return
			}
			run, err := a.ExecuteAction(ctx, plan.Metadata.ID, "repository-sync", input.RequestID+"-"+plan.Metadata.ID)
			if run.Metadata.ID != "" {
				outcome.Run = &run
			}
			if err != nil {
				outcome.Error = "저장소 최신화가 완료되지 않았습니다."
			}
			result.Outcomes[index] = outcome
		}()
	}
	wait.Wait()
	for _, outcome := range result.Outcomes {
		if outcome.Run != nil && outcome.Run.Spec.Status == domain.ActionRunSucceeded {
			_ = a.RunScan(ctx, "repository-sync")
			break
		}
	}
	return result, nil
}

type syncSkipReason struct {
	worktreeID string
	code       string
	message    string
}

func repositorySyncWorktree(worktrees []domain.Worktree) (domain.Worktree, *syncSkipReason) {
	var primary *domain.Worktree
	for index := range worktrees {
		if worktrees[index].Spec.Primary {
			primary = &worktrees[index]
			break
		}
	}
	if primary == nil {
		return domain.Worktree{}, &syncSkipReason{code: "no_primary_worktree", message: "기본 Worktree가 관찰되지 않았습니다."}
	}
	if reason := repositorySyncEligibility(primary); reason != "" {
		return domain.Worktree{}, &syncSkipReason{worktreeID: primary.Metadata.ID, code: repositorySyncReasonCode(primary), message: reason}
	}
	return *primary, nil
}

func repositorySyncEligibility(worktree *domain.Worktree) string {
	switch {
	case worktree.Spec.TombstonedAt != nil:
		return "Worktree 관찰이 종료되었습니다."
	case worktree.Spec.Trust != domain.WorktreeTrustVerifiedReadOnly:
		return "Worktree가 읽기 전용으로 확인되지 않았습니다."
	case worktree.Spec.Error != "" || worktree.Spec.LastObserved.IsZero():
		return "Worktree 관찰이 완료되지 않았습니다."
	case worktree.Spec.Head == "":
		return "현재 HEAD를 확인할 수 없습니다."
	case worktree.Spec.Detached || worktree.Spec.Branch == "":
		return "detached Worktree에서는 최신화하지 않습니다."
	case worktree.Spec.Dirty || worktree.Spec.Untracked:
		return "커밋하지 않은 변경이나 추적하지 않는 파일이 있습니다."
	case worktree.Spec.Upstream == "":
		return "upstream이 설정되지 않았습니다."
	case worktree.Spec.Ahead > 0:
		return "원격에 아직 보내지 않은 커밋이 있습니다."
	case worktree.Spec.Locked || worktree.Spec.Prunable:
		return "Worktree가 잠겼거나 정리 대상으로 표시되었습니다."
	default:
		return ""
	}
}

func repositorySyncReasonCode(worktree *domain.Worktree) string {
	switch {
	case worktree.Spec.TombstonedAt != nil:
		return "tombstoned"
	case worktree.Spec.Trust != domain.WorktreeTrustVerifiedReadOnly:
		return "unverified_worktree"
	case worktree.Spec.Error != "" || worktree.Spec.LastObserved.IsZero():
		return "incomplete_observation"
	case worktree.Spec.Head == "":
		return "missing_head"
	case worktree.Spec.Detached || worktree.Spec.Branch == "":
		return "detached"
	case worktree.Spec.Dirty || worktree.Spec.Untracked:
		return "local_changes"
	case worktree.Spec.Upstream == "":
		return "missing_upstream"
	case worktree.Spec.Ahead > 0:
		return "local_ahead"
	case worktree.Spec.Locked || worktree.Spec.Prunable:
		return "locked_or_prunable"
	default:
		return "not_eligible"
	}
}

func validateRepositorySyncState(current collector.Worktree, expected domain.WorktreeExecutionContext) error {
	if current.Head == "" || current.Branch != expected.Branch || current.Detached || current.Dirty || current.Untracked || current.Upstream == "" || current.Ahead > 0 || current.Locked || current.Prunable || current.Error != "" {
		return fmt.Errorf("repository sync Worktree state is no longer eligible")
	}
	return nil
}

func repositorySyncPlanID(projectID, repositoryID, worktreeID string, at time.Time) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{projectID, repositoryID, worktreeID, at.UTC().Format(time.RFC3339Nano)}, "\x00")))
	return "sync-" + hex.EncodeToString(sum[:])[:56]
}
