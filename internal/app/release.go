package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
)

func (a *App) PlanRelease(ctx context.Context, input ReleasePlanInput) (ReleasePlan, error) {
	if input.Environment != "stage" && input.Environment != "production" {
		return ReleasePlan{}, contract.InvalidInput("release environment must be stage or production")
	}
	group, err := a.externalGroup(input.GroupID)
	if err != nil {
		return ReleasePlan{}, err
	}
	digest, err := a.externalGroupDigest(group)
	if err != nil {
		return ReleasePlan{}, err
	}
	targets, err := a.externalTargetPlans(group)
	if err != nil {
		return ReleasePlan{}, contract.InvalidInput(err.Error())
	}
	expected := strings.TrimSpace(input.ExpectedRevision)
	if expected == "" {
		expected = "not-configured"
	}
	planIDSum := sha256.Sum256([]byte(group.ID + "\x00" + input.Environment + "\x00" + time.Now().UTC().Format(time.RFC3339Nano)))
	actionType := "release.jenkins.stage"
	if input.Environment == "production" {
		actionType = "release.jenkins.production"
	}
	plan, err := a.PlanAction(ctx, ActionPlanInput{ID: "release-plan-" + hex.EncodeToString(planIDSum[:])[:48], Name: group.Name + " " + input.Environment + " release", ProjectID: input.ProjectID, RepositoryID: input.RepositoryID, WorktreeID: input.WorktreeID, ActionType: actionType, Inputs: map[string]string{"group_id": group.ID, "group_digest": digest, "environment": input.Environment, "expected_revision": expected}})
	if err != nil {
		return ReleasePlan{}, err
	}
	return ReleasePlan{ActionPlan: plan, Group: group, Environment: input.Environment, ExpectedRevision: strings.TrimSpace(input.ExpectedRevision), Digest: digest, Targets: targets, Postchecks: []string{"successful-build", "expected-revision"}, CreatedAt: plan.Spec.RequestedAt}, nil
}

func (a *App) ReleasePlan(ctx context.Context, planID string) (ReleasePlan, error) {
	plan, err := a.store.GetActionPlan(ctx, planID)
	if err != nil {
		return ReleasePlan{}, classifyActionError(err)
	}
	if plan.Spec.ActionType != "release.jenkins.stage" && plan.Spec.ActionType != "release.jenkins.production" {
		return ReleasePlan{}, contract.InvalidInput("action plan is not a Jenkins release plan")
	}
	group, err := a.externalGroup(plan.Spec.Inputs["group_id"])
	if err != nil {
		return ReleasePlan{}, err
	}
	digest, err := a.externalGroupDigest(group)
	if err != nil {
		return ReleasePlan{}, err
	}
	if digest != plan.Spec.Inputs["group_digest"] {
		return ReleasePlan{}, contract.Conflict("release group changed; the plan is stale")
	}
	targets, err := a.externalTargetPlans(group)
	if err != nil {
		return ReleasePlan{}, err
	}
	expected := plan.Spec.Inputs["expected_revision"]
	if expected == "not-configured" {
		expected = ""
	}
	return ReleasePlan{ActionPlan: plan, Group: group, Environment: plan.Spec.Inputs["environment"], ExpectedRevision: expected, Digest: digest, Targets: targets, Postchecks: []string{"successful-build", "expected-revision"}, CreatedAt: plan.Spec.RequestedAt}, nil
}

func (a *App) ReleaseResult(_ context.Context, planID string) (ReleaseResult, error) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	for _, result := range a.config.ReleaseResults {
		if result.PlanID == planID {
			return result, nil
		}
	}
	return ReleaseResult{}, contract.NotFound("release result not found")
}

func (a *App) ExecuteRelease(ctx context.Context, planID, holder, idempotencyKey string) (ReleaseResult, error) {
	plan, err := a.ReleasePlan(ctx, planID)
	if err != nil {
		return ReleaseResult{}, err
	}
	admission, err := a.broker.Admit(ctx, planID, holder, idempotencyKey)
	if err != nil {
		return ReleaseResult{}, classifyActionError(err)
	}
	if _, changed, checkErr := a.discoveryWorktree(ctx, plan.ActionPlan.Spec.ProjectID, plan.ActionPlan.Spec.RepositoryID, plan.ActionPlan.Spec.WorktreeID); checkErr != nil || changed {
		_ = a.broker.Release(context.Background(), admission)
		return ReleaseResult{}, contract.Conflict("release Worktree evidence changed; the plan is stale")
	}
	started := time.Now().UTC()
	_ = a.store.SaveActionEvent(ctx, externalActionEvent(plan.ActionPlan, "release_started", holder, started, "started"))
	external := runJenkinsGroup(ctx, a, ExternalWorkGroupPlan{ActionPlan: plan.ActionPlan, Group: plan.Group, Digest: plan.Digest, Targets: plan.Targets, CreatedAt: plan.CreatedAt}, started)
	postchecks := []ReleasePostcheckEvidence{{ID: "successful-build", Status: "passed", Detail: "every required Jenkins target completed successfully"}}
	status := "succeeded"
	if external.Status != "succeeded" {
		status = "failed"
		postchecks[0] = ReleasePostcheckEvidence{ID: "successful-build", Status: "failed", Detail: "one or more required Jenkins targets failed"}
	}
	if plan.ExpectedRevision != "" {
		postchecks = append(postchecks, ReleasePostcheckEvidence{ID: "expected-revision", Status: "failed", Detail: "the configured Jenkins contract did not provide verified revision evidence"})
		if status == "succeeded" {
			status = "postcheck_failed"
		}
	} else {
		postchecks = append(postchecks, ReleasePostcheckEvidence{ID: "expected-revision", Status: "not_configured", Detail: "no expected revision was requested"})
	}
	result := ReleaseResult{PlanID: planID, Environment: plan.Environment, Status: status, External: external, Postchecks: postchecks, CompletedAt: time.Now().UTC()}
	_ = a.store.SaveActionEvent(ctx, externalActionEvent(plan.ActionPlan, "release_"+status, holder, result.CompletedAt, status))
	_ = a.broker.Release(context.Background(), admission)
	if saveErr := a.saveReleaseResult(result); saveErr != nil {
		return result, saveErr
	}
	if status != "succeeded" {
		return result, contract.CodedError{Code: contract.ErrorExecutionFailed, Message: "release postcondition did not pass"}
	}
	return result, nil
}

func (a *App) saveReleaseResult(result ReleaseResult) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	items := append([]ReleaseResult(nil), a.config.ReleaseResults...)
	updated := false
	for index := range items {
		if items[index].PlanID == result.PlanID {
			items[index] = result
			updated = true
			break
		}
	}
	if !updated {
		items = append(items, result)
	}
	if len(items) > 128 {
		items = items[len(items)-128:]
	}
	previous := a.config
	a.config.ReleaseResults = items
	if err := saveConfig(a.home, a.config); err != nil {
		a.config = previous
		return err
	}
	return nil
}
