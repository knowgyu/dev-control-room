package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/assurance"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/environment"
	"github.com/knowgyu/dev-control-room/internal/qualitysetup"
	"github.com/knowgyu/dev-control-room/internal/store"
)

const (
	inspectionGenerationSource   = domain.InspectionGenerationDeterministic
	inspectionGenerationReason   = "ai_proposal_not_configured"
	inspectionResultArtifactType = "inspection_result"
	inspectionResultArtifactMIME = "application/json"
)

var exactQualityVersionPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+){1,3}(?:[-+][0-9A-Za-z.-]+)?$`)

type InspectionPlanGenerateInput struct {
	ProjectID    string               `json:"projectId"`
	RepositoryID string               `json:"repositoryId"`
	WorktreeID   string               `json:"worktreeId"`
	AI           *InspectionAIRequest `json:"ai,omitempty"`
}

type InspectionAIRequest struct {
	Enabled        bool   `json:"enabled"`
	Provider       string `json:"provider"`
	ProfileID      string `json:"profileId"`
	RequestedModel string `json:"requestedModel"`
}

type InspectionPlanReviewInput struct {
	ExpectedRevision int    `json:"expectedRevision"`
	Decision         string `json:"decision"`
}

type InspectionPlanRunInput struct {
	ExpectedRevision int `json:"expectedRevision"`
}

type InspectionPlanGeneration struct {
	Source         string   `json:"source"`
	AIProposal     bool     `json:"aiProposal"`
	Reason         string   `json:"reason"`
	Provider       string   `json:"provider,omitempty"`
	Model          string   `json:"model,omitempty"`
	ProposalDigest string   `json:"proposalDigest,omitempty"`
	Decisions      []string `json:"decisions,omitempty"`
}

type InspectionPlanView struct {
	domain.InspectionPlan
	Generation InspectionPlanGeneration `json:"generation"`
}

type InspectionRunView struct {
	Plan           domain.InspectionPlan         `json:"plan"`
	Results        []domain.InspectionResult     `json:"results"`
	Score          domain.RepositoryQualityScore `json:"score"`
	ResultArtifact string                        `json:"resultArtifactId,omitempty"`
}

type InspectionRunQueryInput struct {
	ProjectID    string
	RepositoryID string
	WorktreeID   string
}

type QualityToolInstallPlanInput struct {
	ProjectID     string                           `json:"projectId"`
	RepositoryID  string                           `json:"repositoryId"`
	WorktreeID    string                           `json:"worktreeId"`
	ComponentID   string                           `json:"componentId"`
	Kind          assurance.QualityToolInstallKind `json:"kind"`
	Version       string                           `json:"version"`
	AffectedFiles []string                         `json:"affectedFiles"`
	AllowGlobal   bool                             `json:"allowGlobal,omitempty"`
}

type QualityToolInstallPreview struct {
	Available bool                                `json:"available"`
	Reason    string                              `json:"reason,omitempty"`
	Action    *assurance.QualityToolInstallAction `json:"action,omitempty"`
}

type QualityScoreComparisonInput struct {
	BeforeID string `json:"beforeId"`
	AfterID  string `json:"afterId"`
}

type QualityImprovementProposalGenerateInput struct {
	PlanID           string `json:"planId"`
	ResultArtifactID string `json:"resultArtifactId"`
}

type QualityImprovementProposalReviewInput struct {
	ExpectedRevision int    `json:"expectedRevision"`
	Decision         string `json:"decision"`
}

type QualityImprovementProposalApplyInput struct {
	ExpectedRevision int `json:"expectedRevision"`
}

type QualityImprovementApplyView struct {
	Proposal domain.QualityImprovementProposal `json:"proposal"`
	Plan     InspectionPlanView                `json:"plan"`
}

type inspectionEvidenceReader func(
	context.Context,
	domain.InspectionPlan,
) (qualitysetup.Report, domain.InspectionFreshness, string, error)

func (a *App) InspectionPlans(ctx context.Context) ([]InspectionPlanView, error) {
	items, err := a.store.ListInspectionPlans(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]InspectionPlanView, 0, len(items))
	for _, item := range items {
		result = append(result, inspectionPlanView(item))
	}
	return result, nil
}

func (a *App) InspectionPlan(ctx context.Context, id string) (InspectionPlanView, error) {
	item, err := a.store.GetInspectionPlan(ctx, id)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return InspectionPlanView{}, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		return InspectionPlanView{}, contract.NotFound("inspection plan not found")
	}
	if err != nil {
		return InspectionPlanView{}, err
	}
	return inspectionPlanView(item), nil
}

func (a *App) LatestInspectionRun(ctx context.Context, input InspectionRunQueryInput) (InspectionRunView, error) {
	input.ProjectID, input.RepositoryID, input.WorktreeID = strings.TrimSpace(input.ProjectID), strings.TrimSpace(input.RepositoryID), strings.TrimSpace(input.WorktreeID)
	if input.ProjectID == "" || input.RepositoryID == "" || input.WorktreeID == "" {
		return InspectionRunView{}, contract.InvalidInput("project, repository, and worktree IDs are required")
	}
	artifacts, err := a.store.ListArtifactsBySourceType(ctx, inspectionResultArtifactType)
	if err != nil {
		return InspectionRunView{}, err
	}
	for _, artifact := range artifacts {
		run, runErr := a.loadInspectionRunArtifact(ctx, artifact.Metadata.ID)
		if runErr != nil {
			continue
		}
		plan, planErr := a.store.GetInspectionPlan(ctx, run.PlanID)
		if planErr != nil || plan.Spec.ProjectID != input.ProjectID || plan.Spec.RepositoryID != input.RepositoryID || plan.Spec.WorktreeID != input.WorktreeID {
			continue
		}
		score, scoreErr := a.store.GetRepositoryQualityScore(ctx, run.ScoreID)
		if scoreErr != nil {
			continue
		}
		return InspectionRunView{Plan: plan, Results: run.Results, Score: score, ResultArtifact: artifact.Metadata.ID}, nil
	}
	return InspectionRunView{}, contract.NotFound("inspection result artifact not found for the selected worktree")
}

func (a *App) RepositoryQualityScores(ctx context.Context) ([]domain.RepositoryQualityScore, error) {
	return a.store.ListRepositoryQualityScores(ctx)
}

func (a *App) RepositoryQualityScore(ctx context.Context, id string) (domain.RepositoryQualityScore, error) {
	item, err := a.store.GetRepositoryQualityScore(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RepositoryQualityScore{}, contract.NotFound("quality score not found")
	}
	return item, err
}

func (a *App) CompareRepositoryQualityScores(ctx context.Context, input QualityScoreComparisonInput) (domain.QualityComparison, error) {
	input.BeforeID = strings.TrimSpace(input.BeforeID)
	input.AfterID = strings.TrimSpace(input.AfterID)
	if input.BeforeID == "" || input.AfterID == "" {
		return domain.QualityComparison{}, contract.InvalidInput("before and after score IDs are required")
	}
	before, err := a.RepositoryQualityScore(ctx, input.BeforeID)
	if err != nil {
		return domain.QualityComparison{}, err
	}
	after, err := a.RepositoryQualityScore(ctx, input.AfterID)
	if err != nil {
		return domain.QualityComparison{}, err
	}
	if before.Spec.ProjectID != after.Spec.ProjectID || before.Spec.RepositoryID != after.Spec.RepositoryID || before.Spec.WorktreeID != after.Spec.WorktreeID {
		return domain.QualityComparison{Status: domain.QualityComparisonInconclusive, Reason: "scope_mismatch"}, nil
	}
	return CompareQualityScores(before, after), nil
}

func (a *App) GenerateInspectionPlan(ctx context.Context, input InspectionPlanGenerateInput) (InspectionPlanView, error) {
	input = normalizeInspectionPlanGenerateInput(input)
	if input.ProjectID == "" || input.RepositoryID == "" || input.WorktreeID == "" {
		return InspectionPlanView{}, contract.InvalidInput("project, repository, and worktree IDs are required")
	}
	current, changed, err := a.discoveryWorktree(ctx, input.ProjectID, input.RepositoryID, input.WorktreeID)
	if err != nil {
		return InspectionPlanView{}, contract.Unavailable("selected worktree could not be revalidated")
	}
	if changed {
		return InspectionPlanView{}, contract.Conflict("selected worktree changed; refresh and try again")
	}
	setup, err := a.QualitySetup(ctx, input.ProjectID, input.RepositoryID, input.WorktreeID, nil)
	if err != nil {
		return InspectionPlanView{}, err
	}
	components, candidates := inspectionPlanParts(setup)
	checks := runnableInspectionChecks(setup, current.Path, candidates)
	if len(checks) == 0 {
		return InspectionPlanView{}, contract.Unavailable("no applicable inspection checks were detected; configure a supported tool and refresh")
	}
	generation := InspectionPlanGeneration{Source: inspectionGenerationSource, AIProposal: false, Reason: inspectionGenerationReason}
	var generationProvider, generationModel, proposalDigest string
	var proposalDecisions []string
	if input.AI != nil && input.AI.Enabled {
		aiProposal, aiErr := a.generateInspectionAIProposal(ctx, *input.AI, current.Path, setup, components, checks)
		if aiErr == nil {
			checks, aiApplyErr := applyInspectionAIDecisions(checks, checks, aiProposal.Decisions)
			if aiApplyErr == nil && len(checks) > 0 {
				generation = InspectionPlanGeneration{
					Source:         domain.InspectionGenerationAI,
					AIProposal:     true,
					Reason:         "ai_bounded_proposal_applied",
					Provider:       aiProposal.Provider,
					Model:          aiProposal.Model,
					ProposalDigest: aiProposal.Digest,
					Decisions:      append([]string{}, aiProposal.Decisions...),
				}
				generationProvider, generationModel, proposalDigest, proposalDecisions = aiProposal.Provider, aiProposal.Model, aiProposal.Digest, append([]string{}, aiProposal.Decisions...)
			} else if aiApplyErr != nil {
				generation.Reason = "ai_output_not_applicable"
			}
		} else {
			generation.Reason = aiErr.Error()
		}
	}
	configDigest := inspectionConfigDigestForSetup(setup, components)
	toolDigest := inspectionToolDigestForSetupRoot(checks, current.Path, setup)
	now := time.Now().UTC()
	plan := domain.InspectionPlan{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.InspectionPlanKind},
		Metadata: domain.ObjectMeta{ID: assuranceID("inspection-plan", input.ProjectID, input.RepositoryID, input.WorktreeID, current.Head, setup.Digest, proposalDigest), Name: "inspection plan"},
		Spec: domain.InspectionPlanSpec{
			ProjectID: input.ProjectID, RepositoryID: input.RepositoryID, WorktreeID: input.WorktreeID,
			Branch: current.Branch, Head: current.Head,
			BaselineDigest: digestText("inspection-baseline-v1", input.ProjectID, input.RepositoryID, input.WorktreeID, current.Head),
			ConfigDigest:   configDigest, EvidenceDigest: "sha256:" + setup.Digest,
			ToolDigest: toolDigest, Components: components, Checks: checks,
			GenerationSource: generation.Source, GenerationProvider: generationProvider, GenerationModel: generationModel,
			AIProposalDigest: proposalDigest, AIProposalDecisions: proposalDecisions,
			State: domain.InspectionPlanStateProposed, Revision: 1, Version: domain.InspectionPlanVersion,
			CreatedAt: now, UpdatedAt: now,
		},
	}
	digest, err := plan.Digest()
	if err != nil {
		return InspectionPlanView{}, err
	}
	plan.Spec.Digest = digest
	if existing, getErr := a.store.GetInspectionPlan(ctx, plan.Metadata.ID); getErr == nil {
		if existing.Spec.State != domain.InspectionPlanStateRejected {
			return inspectionPlanView(existing), nil
		}
		// A rejected plan is intentionally immutable. An explicit regeneration
		// gets a deterministic nonce derived from the rejected generation, so a
		// refresh does not create duplicate plans while still allowing recovery.
		for nonce := existing.Spec.Revision + 1; nonce < existing.Spec.Revision+1000; nonce++ {
			candidateID := assuranceID(plan.Metadata.ID, "regeneration", nonce)
			if _, candidateErr := a.store.GetInspectionPlan(ctx, candidateID); errors.Is(candidateErr, sql.ErrNoRows) {
				plan.Metadata.ID = candidateID
				plan.Spec.CreatedAt, plan.Spec.UpdatedAt = now, now
				plan.Spec.Digest, err = plan.Digest()
				if err != nil {
					return InspectionPlanView{}, err
				}
				break
			}
		}
	}
	if err := a.store.SaveInspectionPlan(ctx, plan); err != nil {
		return InspectionPlanView{}, err
	}
	return inspectionPlanViewWithGeneration(plan, generation), nil
}

func (a *App) QualityImprovementProposals(ctx context.Context) ([]domain.QualityImprovementProposal, error) {
	return a.store.ListQualityImprovementProposals(ctx)
}

func (a *App) QualityImprovementProposal(ctx context.Context, id string) (domain.QualityImprovementProposal, error) {
	item, err := a.store.GetQualityImprovementProposal(ctx, strings.TrimSpace(id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.QualityImprovementProposal{}, contract.NotFound("quality improvement proposal not found")
	}
	return item, err
}

func (a *App) GenerateQualityImprovementProposal(ctx context.Context, input QualityImprovementProposalGenerateInput) (domain.QualityImprovementProposal, error) {
	input.PlanID = strings.TrimSpace(input.PlanID)
	input.ResultArtifactID = strings.TrimSpace(input.ResultArtifactID)
	if input.PlanID == "" || input.ResultArtifactID == "" {
		return domain.QualityImprovementProposal{}, contract.InvalidInput("plan and result artifact IDs are required")
	}
	plan, err := a.store.GetInspectionPlan(ctx, input.PlanID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.QualityImprovementProposal{}, contract.NotFound("inspection plan not found")
	}
	if err != nil {
		return domain.QualityImprovementProposal{}, err
	}
	run, err := a.loadInspectionRunArtifact(ctx, input.ResultArtifactID)
	if err != nil {
		return domain.QualityImprovementProposal{}, err
	}
	if run.PlanID != plan.Metadata.ID {
		return domain.QualityImprovementProposal{}, contract.Conflict("result artifact does not belong to the inspection plan")
	}
	score, err := a.RepositoryQualityScore(ctx, run.ScoreID)
	if err != nil {
		return domain.QualityImprovementProposal{}, err
	}
	if score.Spec.ProjectID != plan.Spec.ProjectID || score.Spec.RepositoryID != plan.Spec.RepositoryID || score.Spec.WorktreeID != plan.Spec.WorktreeID || score.Spec.Head != plan.Spec.Head {
		return domain.QualityImprovementProposal{}, contract.Conflict("result score does not belong to the inspection plan")
	}
	setup, freshness, root, err := a.currentInspectionEvidence(ctx, plan)
	if err != nil {
		return domain.QualityImprovementProposal{}, contract.Unavailable("inspection evidence could not be revalidated")
	}
	changes, rationaleCode := deterministicQualityImprovements(plan, setup, root, run.Results)
	if len(changes) == 0 {
		return domain.QualityImprovementProposal{}, contract.Conflict("inspection results contain no actionable improvement")
	}
	now := time.Now().UTC()
	item := domain.QualityImprovementProposal{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.QualityImprovementProposalKind},
		Metadata: domain.ObjectMeta{ID: assuranceID("quality-improvement", plan.Metadata.ID, run.ScoreID, freshness.Head, changes), Name: "quality improvement proposal"},
		Spec: domain.QualityImprovementProposalSpec{
			ProjectID: plan.Spec.ProjectID, RepositoryID: plan.Spec.RepositoryID, WorktreeID: plan.Spec.WorktreeID,
			PlanID: plan.Metadata.ID, BaseScoreID: run.ScoreID, BaseScoreDigest: digestText(score),
			BasePlanRevision: plan.Spec.Revision, BasePlanDigest: plan.Spec.Digest, Head: freshness.Head,
			ConfigDigest: freshness.ConfigDigest, EvidenceDigest: freshness.EvidenceDigest, ToolDigest: freshness.ToolDigest,
			Changes: changes, RationaleDigest: digestText("quality-improvement-rationale-v1", rationaleCode, changes), RationaleCode: rationaleCode,
			Source: domain.InspectionGenerationDeterministic, State: domain.QualityImprovementStateProposed, Revision: 1,
			CreatedAt: now, UpdatedAt: now,
		},
	}
	if err := a.store.SaveQualityImprovementProposal(ctx, item); err != nil {
		return domain.QualityImprovementProposal{}, err
	}
	return item, nil
}

func (a *App) ReviewQualityImprovementProposal(ctx context.Context, id string, input QualityImprovementProposalReviewInput) (domain.QualityImprovementProposal, error) {
	item, err := a.QualityImprovementProposal(ctx, id)
	if err != nil {
		return domain.QualityImprovementProposal{}, err
	}
	if input.ExpectedRevision != item.Spec.Revision {
		return domain.QualityImprovementProposal{}, contract.Conflict("quality improvement proposal revision is stale")
	}
	state, err := qualityImprovementReviewState(item.Spec.State, strings.ToLower(strings.TrimSpace(input.Decision)))
	if err != nil {
		return domain.QualityImprovementProposal{}, contract.InvalidInput(err.Error())
	}
	now := time.Now().UTC()
	item.Spec.State, item.Spec.Revision, item.Spec.UpdatedAt = state, item.Spec.Revision+1, now
	item.Spec.ReviewedAt = &now
	if err := a.store.UpdateQualityImprovementProposalRevisionCAS(ctx, domain.QualityImprovementProposalKind, item.Metadata.ID, input.ExpectedRevision, item); err != nil {
		if errors.Is(err, store.ErrQualityImprovementRevisionStale) {
			return domain.QualityImprovementProposal{}, contract.Conflict("quality improvement proposal revision is stale")
		}
		return domain.QualityImprovementProposal{}, err
	}
	return item, nil
}

func (a *App) ApplyQualityImprovementProposal(ctx context.Context, id string, input QualityImprovementProposalApplyInput) (QualityImprovementApplyView, error) {
	return a.applyQualityImprovementProposal(ctx, id, input, a.currentInspectionEvidence)
}

func (a *App) applyQualityImprovementProposal(
	ctx context.Context,
	id string,
	input QualityImprovementProposalApplyInput,
	readEvidence inspectionEvidenceReader,
) (QualityImprovementApplyView, error) {
	item, err := a.QualityImprovementProposal(ctx, id)
	if err != nil {
		return QualityImprovementApplyView{}, err
	}
	if input.ExpectedRevision != item.Spec.Revision {
		return QualityImprovementApplyView{}, contract.Conflict("quality improvement proposal revision is stale")
	}
	if item.Spec.State != domain.QualityImprovementStateApproved {
		return QualityImprovementApplyView{}, contract.Conflict("quality improvement proposal must be approved before it can be applied")
	}
	plan, err := a.store.GetInspectionPlan(ctx, item.Spec.PlanID)
	if errors.Is(err, sql.ErrNoRows) {
		return QualityImprovementApplyView{}, contract.NotFound("inspection plan not found")
	}
	if err != nil {
		return QualityImprovementApplyView{}, err
	}
	if plan.Spec.Revision != item.Spec.BasePlanRevision || plan.Spec.Digest != item.Spec.BasePlanDigest {
		return a.staleQualityImprovement(ctx, item, "inspection plan changed")
	}
	setup, freshness, evidenceRoot, err := readEvidence(ctx, plan)
	if err != nil {
		return QualityImprovementApplyView{}, contract.Unavailable("inspection evidence could not be revalidated")
	}
	if staleReasons := plan.StaleReasons(freshness); len(staleReasons) > 0 || freshness.Head != item.Spec.Head || freshness.ConfigDigest != item.Spec.ConfigDigest || freshness.EvidenceDigest != item.Spec.EvidenceDigest || freshness.ToolDigest != item.Spec.ToolDigest {
		return a.staleQualityImprovement(ctx, item, "inspection evidence, configuration, tools, or HEAD changed")
	}
	nextChecks, err := applyQualityImprovementChanges(plan, item.Spec.Changes)
	if err != nil {
		return QualityImprovementApplyView{}, contract.Conflict(err.Error())
	}
	now := time.Now().UTC()
	nextPlan := plan
	nextPlan.Spec.Head = freshness.Head
	nextPlan.Spec.Checks = nextChecks
	nextPlan.Spec.ConfigDigest = freshness.ConfigDigest
	nextPlan.Spec.EvidenceDigest = freshness.EvidenceDigest
	nextPlan.Spec.ToolDigest = inspectionToolDigestForSetupRoot(nextChecks, evidenceRoot, setup)
	nextPlan.Spec.State = domain.InspectionPlanStateProposed
	nextPlan.Spec.Revision++
	nextPlan.Spec.UpdatedAt = now
	nextPlan.Spec.Digest, err = nextPlan.Digest()
	if err != nil {
		return QualityImprovementApplyView{}, err
	}
	item.Spec.State, item.Spec.Revision, item.Spec.UpdatedAt = domain.QualityImprovementStateApplied, item.Spec.Revision+1, now
	item.Spec.AppliedAt = &now
	if err := a.store.ApplyQualityImprovementProposalCAS(ctx, plan.Metadata.ID, plan.Spec.Revision, nextPlan, item.Metadata.ID, input.ExpectedRevision, item); err != nil {
		if errors.Is(err, store.ErrInspectionPlanRevisionStale) {
			return QualityImprovementApplyView{}, contract.Conflict("inspection plan revision is stale")
		}
		if errors.Is(err, store.ErrQualityImprovementRevisionStale) {
			return QualityImprovementApplyView{}, contract.Conflict("quality improvement proposal revision is stale")
		}
		return QualityImprovementApplyView{}, err
	}
	return QualityImprovementApplyView{Proposal: item, Plan: inspectionPlanView(nextPlan)}, nil
}

func (a *App) ReviewInspectionPlan(ctx context.Context, id string, input InspectionPlanReviewInput) (InspectionPlanView, error) {
	plan, err := a.store.GetInspectionPlan(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return InspectionPlanView{}, contract.NotFound("inspection plan not found")
	}
	if err != nil {
		return InspectionPlanView{}, err
	}
	if input.ExpectedRevision != plan.Spec.Revision {
		return InspectionPlanView{}, contract.Conflict("inspection plan revision is stale")
	}
	next, err := inspectionReviewState(plan.Spec.State, strings.ToLower(strings.TrimSpace(input.Decision)))
	if err != nil {
		return InspectionPlanView{}, contract.InvalidInput(err.Error())
	}
	plan.Spec.State = next
	plan.Spec.Revision++
	plan.Spec.UpdatedAt = time.Now().UTC()
	plan.Spec.Digest, err = plan.Digest()
	if err != nil {
		return InspectionPlanView{}, err
	}
	if err := a.store.UpdateInspectionPlanRevisionCAS(ctx, domain.InspectionPlanKind, id, input.ExpectedRevision, plan); err != nil {
		if errors.Is(err, store.ErrInspectionPlanRevisionStale) {
			return InspectionPlanView{}, contract.Conflict("inspection plan revision is stale")
		}
		return InspectionPlanView{}, err
	}
	return inspectionPlanView(plan), nil
}

func (a *App) RunInspectionPlan(ctx context.Context, id string, input InspectionPlanRunInput) (InspectionRunView, error) {
	plan, err := a.store.GetInspectionPlan(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return InspectionRunView{}, contract.NotFound("inspection plan not found")
	}
	if err != nil {
		return InspectionRunView{}, err
	}
	if input.ExpectedRevision != plan.Spec.Revision {
		return InspectionRunView{}, contract.Conflict("inspection plan revision is stale")
	}
	if plan.Spec.State != domain.InspectionPlanStateApproved {
		return InspectionRunView{}, contract.Conflict("inspection plan must be approved before it can run")
	}
	if err := validateReviewedInspectionChecks(plan.Spec.Checks); err != nil {
		return InspectionRunView{}, contract.InvalidInput(err.Error())
	}
	setup, freshness, root, err := a.currentInspectionEvidence(ctx, plan)
	if err != nil {
		return InspectionRunView{}, err
	}
	if plan.IsStale(freshness) {
		stale := plan
		stale.Spec.State = domain.InspectionPlanStateStale
		stale.Spec.Revision++
		stale.Spec.UpdatedAt = time.Now().UTC()
		if err := a.store.UpdateInspectionPlanRevisionCAS(ctx, domain.InspectionPlanKind, id, plan.Spec.Revision, stale); err != nil {
			if errors.Is(err, store.ErrInspectionPlanRevisionStale) {
				return InspectionRunView{}, contract.Conflict("inspection plan revision is stale")
			}
			return InspectionRunView{}, err
		}
		return InspectionRunView{}, contract.Conflict("inspection plan is stale; generate a new plan")
	}
	capability := assurance.CheckWindows11Capability(ctx)
	if !capability.Available {
		return InspectionRunView{}, contract.Unavailable("inspection execution requires native Windows 11")
	}
	results := make([]domain.InspectionResult, 0, len(plan.Spec.Checks))
	for _, check := range plan.Spec.Checks {
		result := a.runInspectionCheck(ctx, capability, setup, root, plan, check)
		results = append(results, result)
	}
	score, err := CalculateRepositoryQualityScore(RepositoryQualityScoreInput{Plan: plan, Results: results, Current: &freshness})
	if err != nil {
		return InspectionRunView{}, err
	}
	score.Metadata.ID = assuranceID("quality-score", plan.Metadata.ID, time.Now().UTC())
	if err := a.store.SaveRepositoryQualityScore(ctx, score); err != nil {
		return InspectionRunView{}, err
	}
	runID := assuranceID("inspection-run", plan.Metadata.ID, score.Metadata.ID)
	artifactContent, err := json.Marshal(struct {
		PlanID  string                    `json:"planId"`
		Results []domain.InspectionResult `json:"results"`
		ScoreID string                    `json:"scoreId"`
	}{PlanID: plan.Metadata.ID, Results: results, ScoreID: score.Metadata.ID})
	if err != nil {
		return InspectionRunView{}, err
	}
	artifact, err := a.SaveAssuranceArtifact(ctx, ArtifactInput{SourceType: inspectionResultArtifactType, SourceID: runID, Name: runID + ".json", MIME: inspectionResultArtifactMIME, Content: artifactContent})
	if err != nil {
		return InspectionRunView{}, err
	}
	return InspectionRunView{Plan: plan, Results: results, Score: score, ResultArtifact: artifact.Metadata.ID}, nil
}

func (a *App) PlanQualityToolInstall(ctx context.Context, input QualityToolInstallPlanInput) (QualityToolInstallPreview, error) {
	input.ProjectID = strings.TrimSpace(input.ProjectID)
	input.RepositoryID = strings.TrimSpace(input.RepositoryID)
	input.WorktreeID = strings.TrimSpace(input.WorktreeID)
	if input.ProjectID == "" || input.RepositoryID == "" || input.WorktreeID == "" {
		return QualityToolInstallPreview{}, contract.InvalidInput("project, repository, and worktree IDs are required")
	}
	current, changed, err := a.discoveryWorktree(ctx, input.ProjectID, input.RepositoryID, input.WorktreeID)
	if err != nil {
		return QualityToolInstallPreview{}, contract.Unavailable("selected worktree could not be revalidated")
	}
	if changed {
		return QualityToolInstallPreview{}, contract.Conflict("selected worktree changed; refresh and try again")
	}
	interpreterPath, environmentScope := "", "project"
	if input.Kind == assurance.QualityToolInstallRuff || input.Kind == assurance.QualityToolInstallPytest {
		interpreterPath, environmentScope = resolveQualityPython(current.Path, current.Path, qualityLookPath)
	}
	request := assurance.QualityToolInstallRequest{
		Kind: input.Kind, WorktreeRoot: current.Path, ComponentRoot: current.Path,
		InterpreterPath: interpreterPath,
		NodePath:        qualityLookPath("node.exe", "node"), NPMPath: qualityLookPath("npm.exe", "npm.cmd", "npm"),
		EnvironmentScope: environmentScope, AllowGlobal: input.AllowGlobal,
		Version: strings.TrimSpace(input.Version), AffectedFiles: input.AffectedFiles,
	}
	action, err := assurance.BuildQualityToolInstallAction(request, assurance.CheckWindows11Capability(ctx))
	if err != nil {
		return QualityToolInstallPreview{Available: false, Reason: qualityToolInstallPreviewReason(err, environmentScope)}, nil
	}
	return QualityToolInstallPreview{Available: true, Action: &action}, nil
}

func (a *App) currentInspectionEvidence(ctx context.Context, plan domain.InspectionPlan) (qualitysetup.Report, domain.InspectionFreshness, string, error) {
	worktree, err := a.Worktree(ctx, plan.Spec.ProjectID, plan.Spec.RepositoryID, plan.Spec.WorktreeID)
	if err != nil {
		return qualitysetup.Report{}, domain.InspectionFreshness{}, "", err
	}
	setup, err := a.QualitySetup(ctx, plan.Spec.ProjectID, plan.Spec.RepositoryID, plan.Spec.WorktreeID, nil)
	if err != nil {
		return qualitysetup.Report{}, domain.InspectionFreshness{}, "", err
	}
	components, _ := inspectionPlanParts(setup)
	if len(components) == 0 {
		components = []domain.InspectionComponent{{ID: "repository"}}
	}
	freshness := inspectionFreshnessForSetup(setup, plan, components)
	freshness.ToolDigest = inspectionToolDigestForSetupRoot(plan.Spec.Checks, worktree.Spec.CanonicalPath, setup)
	return setup, freshness, worktree.Spec.CanonicalPath, nil
}

func inspectionFreshnessForSetup(
	setup qualitysetup.Report,
	plan domain.InspectionPlan,
	components []domain.InspectionComponent,
) domain.InspectionFreshness {
	return domain.InspectionFreshness{
		Head:           setup.Head,
		ConfigDigest:   inspectionConfigDigestForSetup(setup, components),
		EvidenceDigest: "sha256:" + setup.Digest,
		// Setup checks are candidates. The reviewed plan checks are the actual
		// inspection set whose runner availability must be revalidated.
		ToolDigest: inspectionToolDigestForCurrent(plan.Spec.Checks),
	}
}

func (a *App) runInspectionCheck(ctx context.Context, capability assurance.Windows11Capability, setup qualitysetup.Report, root string, plan domain.InspectionPlan, check domain.InspectionCheck) domain.InspectionResult {
	result := domain.InspectionResult{ComponentID: check.ComponentID, CheckID: check.ID, Outcome: domain.InspectionOutcomeInconclusive, Findings: []domain.InspectionFinding{}}
	componentRoot := qualityComponentRoot(root, planRootForComponent(setup, check.ComponentID))
	if !capability.Available {
		result.Outcome = domain.InspectionOutcomeInconclusive
		return result
	}
	switch check.ID {
	case domain.InspectionCheckRuff, domain.InspectionCheckPytest:
		path, scope := resolveQualityPython(root, componentRoot, qualityLookPath)
		if scope != "project" || path == "" {
			result.Outcome = domain.InspectionOutcomeRunnerUnavailable
			return result
		}
		runner := assurance.PythonQualityRunner{Capability: func(context.Context) assurance.Windows11Capability { return capability }}
		request := assurance.QualityAdapterRequest{WorktreeRoot: root, ComponentRoot: componentRoot, ConfigDigest: plan.Spec.ConfigDigest, InterpreterPath: path, Masker: a.masker}
		if check.ID == domain.InspectionCheckRuff {
			return inspectionResultFromPython(runner.RunRuff(ctx, request), check)
		}
		return inspectionResultFromPython(runner.RunPytest(ctx, request), check)
	case domain.InspectionCheckESLint, domain.InspectionCheckVitest:
		runner := assurance.NodeQualityRunner{Capability: func(context.Context) assurance.Windows11Capability { return capability }}
		request := assurance.QualityAdapterRequest{WorktreeRoot: root, ComponentRoot: componentRoot, ConfigDigest: plan.Spec.ConfigDigest, NodePath: qualityLookPath("node.exe", "node"), NPMPath: qualityLookPath("npm.exe", "npm.cmd", "npm"), Masker: a.masker}
		if check.ID == domain.InspectionCheckESLint {
			return inspectionResultFromNode(runner.RunESLint(ctx, request), check)
		}
		return inspectionResultFromNode(runner.RunVitest(ctx, request), check)
	case domain.InspectionCheckGoTest, domain.InspectionCheckGoTestRace, domain.InspectionCheckGoVet, domain.InspectionCheckGoModVerify, domain.InspectionCheckGoBuild:
		return a.runGoInspectionCheck(ctx, componentRoot, check, result)
	default:
		result.Outcome = domain.InspectionOutcomeRunnerUnavailable
		return result
	}
}

func (a *App) runGoInspectionCheck(ctx context.Context, root string, check domain.InspectionCheck, result domain.InspectionResult) domain.InspectionResult {
	selection, err := assurance.NewQualityRunnerRegistry().Select(assurance.QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueStaticSecurity, WorktreeRoot: root})
	if err != nil || selection.State != assurance.QualityRunnerSelectionAvailable {
		result.Outcome = domain.InspectionOutcomeRunnerUnavailable
		return result
	}
	command := selection.Command
	switch check.ID {
	case domain.InspectionCheckGoTest:
		command.Arguments = []string{"test", "-mod=readonly", "./..."}
	case domain.InspectionCheckGoTestRace:
		command.Arguments = []string{"test", "-mod=readonly", "-race", "./..."}
	case domain.InspectionCheckGoVet:
		// The registry-owned vet command is already selected and validated.
	case domain.InspectionCheckGoModVerify:
		command.Arguments = []string{"mod", "verify"}
	case domain.InspectionCheckGoBuild:
		command.Arguments = []string{"build", "-mod=readonly", "./..."}
	default:
		result.Outcome = domain.InspectionOutcomeRunnerUnavailable
		return result
	}
	process, processErr := (environment.ProcessRunner{OutputLimit: 128 << 10}).RunInDirectory(ctx, command.Executable, command.Arguments, qualityRunnerEnvironment(), root, 2*time.Minute)
	if processErr != nil && process.ExitCode == 0 {
		result.Outcome = domain.InspectionOutcomeToolError
		return result
	}
	if process.ExitCode != 0 {
		if check.ID == domain.InspectionCheckGoVet {
			result.Outcome = domain.InspectionOutcomeFindings
			result.Findings = []domain.InspectionFinding{{Severity: domain.SeverityHigh, Count: 1}}
		} else {
			result.Outcome = domain.InspectionOutcomeTestsFailed
		}
		return result
	}
	result.Outcome = domain.InspectionOutcomeClean
	return result
}

func inspectionResultFromPython(value assurance.QualityAdapterResult, check domain.InspectionCheck) domain.InspectionResult {
	return inspectionResultFromAdapter(value.Outcome, value.Findings, check)
}

func inspectionResultFromNode(value assurance.QualityAdapterResult, check domain.InspectionCheck) domain.InspectionResult {
	return inspectionResultFromAdapter(value.Outcome, value.Findings, check)
}

func inspectionResultFromAdapter(outcome assurance.QualityOutcome, findings []assurance.QualityFinding, check domain.InspectionCheck) domain.InspectionResult {
	result := domain.InspectionResult{ComponentID: check.ComponentID, CheckID: check.ID, Outcome: domain.InspectionOutcomeInconclusive, Findings: []domain.InspectionFinding{}}
	for _, finding := range findings {
		severity := domain.SeverityAttention
		if finding.Severity == "error" {
			severity = domain.SeverityHigh
		}
		result.Findings = append(result.Findings, domain.InspectionFinding{Severity: severity, Count: 1})
	}
	switch outcome {
	case assurance.QualityOutcomeClean:
		result.Outcome = domain.InspectionOutcomeClean
	case assurance.QualityOutcomeFindings:
		result.Outcome = domain.InspectionOutcomeFindings
	case assurance.QualityOutcomeTestsFailed:
		result.Outcome = domain.InspectionOutcomeTestsFailed
	case assurance.QualityOutcomeToolError:
		result.Outcome = domain.InspectionOutcomeToolError
	default:
		result.Outcome = domain.InspectionOutcomeInconclusive
	}
	return result
}

func inspectionPlanParts(report qualitysetup.Report) ([]domain.InspectionComponent, []domain.InspectionCheck) {
	components := make([]domain.InspectionComponent, 0, len(report.Components))
	checks := []domain.InspectionCheck{}
	for _, component := range report.Components {
		components = append(components, domain.InspectionComponent{ID: component.ID})
		for _, item := range component.Checks {
			checkID := inspectionCheckID(item.ID)
			if checkID == "" {
				continue
			}
			checks = append(checks, inspectionCheck(checkID, component.ID))
		}
		if containsQualityLanguage(component.Languages, "go") {
			checks = append(checks, inspectionCheck(domain.InspectionCheckGoVet, component.ID))
		}
	}
	sort.Slice(components, func(i, j int) bool { return components[i].ID < components[j].ID })
	sort.Slice(checks, func(i, j int) bool { return inspectionCheckKey(checks[i]) < inspectionCheckKey(checks[j]) })
	return components, uniqueInspectionChecks(checks)
}

func runnableInspectionChecks(report qualitysetup.Report, root string, candidates []domain.InspectionCheck) []domain.InspectionCheck {
	result := make([]domain.InspectionCheck, 0, len(candidates))
	for _, check := range candidates {
		component, ok := qualitySetupComponent(report, check.ComponentID)
		if !ok {
			continue
		}
		componentRoot := qualityComponentRoot(root, component.Path)
		switch check.ID {
		case domain.InspectionCheckRuff, domain.InspectionCheckPytest:
			interpreter, scope := resolveQualityPython(root, componentRoot, qualityLookPath)
			if scope != "project" || interpreter == "" || !qualityPythonPackageInstalled(interpreter, check.ID) {
				continue
			}
		case domain.InspectionCheckESLint, domain.InspectionCheckVitest:
			nodePath := qualityLookPath("node.exe", "node")
			if nodePath == "" {
				continue
			}
			request := assurance.QualityAdapterRequest{WorktreeRoot: root, ComponentRoot: componentRoot, NodePath: nodePath}
			var err error
			if check.ID == domain.InspectionCheckESLint {
				_, err = assurance.NewNodeQualityRunner().SelectESLint(request)
			} else {
				_, err = assurance.NewNodeQualityRunner().SelectVitest(request)
			}
			if err != nil {
				continue
			}
		case domain.InspectionCheckGoTest, domain.InspectionCheckGoTestRace, domain.InspectionCheckGoVet, domain.InspectionCheckGoModVerify, domain.InspectionCheckGoBuild:
			if !qualityExecutableVerified(qualityLookPath("go.exe", "go"), "go.exe", "go") || !regularNonSymlink(filepath.Join(componentRoot, "go.mod")) {
				continue
			}
		default:
			continue
		}
		result = append(result, check)
	}
	return uniqueInspectionChecks(result)
}

func qualitySetupComponent(report qualitysetup.Report, id string) (qualitysetup.Component, bool) {
	for _, component := range report.Components {
		if component.ID == id {
			return component, true
		}
	}
	return qualitysetup.Component{}, false
}

func qualityPythonPackageInstalled(interpreterPath, checkID string) bool {
	packageName := "ruff"
	if checkID == domain.InspectionCheckPytest {
		packageName = "pytest"
	}
	for _, directory := range pythonPackageDirectories(interpreterPath) {
		if !regularDirectory(directory) {
			continue
		}
		entries, err := os.ReadDir(directory)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := strings.ToLower(entry.Name())
			if strings.HasPrefix(name, packageName+"-") || name == packageName || strings.HasPrefix(name, packageName+".") {
				return true
			}
		}
	}
	return false
}

func regularNonSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink == 0 && info.Mode().IsRegular()
}

func regularDirectory(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink == 0 && info.IsDir()
}

func qualityExecutableVerified(path string, names ...string) bool {
	if path == "" || !filepath.IsAbs(path) || !regularNonSymlink(path) {
		return false
	}
	base := strings.ToLower(filepath.Base(path))
	for _, name := range names {
		name = strings.ToLower(filepath.Base(name))
		if base == name || base == strings.TrimSuffix(name, ".exe") || base == strings.TrimSuffix(name, ".cmd") {
			return true
		}
	}
	return false
}

type inspectionAIProposal struct {
	Provider  string
	Model     string
	Decisions []string
	Digest    string
}

type inspectionAIEvidence struct {
	DetectedLanguages []string                `json:"detectedLanguages"`
	Components        []inspectionAIComponent `json:"components"`
	Warnings          []string                `json:"warnings"`
	EvidenceDigest    string                  `json:"evidenceDigest"`
	ConfigDigest      string                  `json:"configDigest"`
	ToolDigest        string                  `json:"toolDigest"`
}

type inspectionAIComponent struct {
	ID     string              `json:"id"`
	Path   string              `json:"path"`
	Checks []inspectionAICheck `json:"checks"`
}

type inspectionAICheck struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (a *App) generateInspectionAIProposal(
	ctx context.Context,
	request InspectionAIRequest,
	root string,
	setup qualitysetup.Report,
	components []domain.InspectionComponent,
	checks []domain.InspectionCheck,
) (inspectionAIProposal, error) {
	provider := strings.ToLower(strings.TrimSpace(request.Provider))
	if provider == "" {
		provider = "codex"
	}
	if provider != "codex" {
		return inspectionAIProposal{}, errors.New("ai_provider_not_supported")
	}
	profileID := strings.TrimSpace(request.ProfileID)
	if profileID == "" {
		profileID = "codex"
	}
	profile, err := a.AgentProfile(ctx, profileID)
	if err != nil {
		return inspectionAIProposal{}, errors.New("ai_profile_unavailable")
	}
	if err := trustedCodexProfile(profile); err != nil {
		return inspectionAIProposal{}, errors.New("ai_profile_untrusted")
	}
	execution := assurance.CodexExecutionFromContext(ctx)
	status := execution.Resolver()
	if status.Provider != "" && !strings.EqualFold(status.Provider, "codex") {
		return inspectionAIProposal{}, errors.New("ai_provider_unavailable")
	}
	if status.State != assurance.ProviderReady || !status.CommandFound || !status.LaunchTrusted || !status.ProfileReady {
		return inspectionAIProposal{}, errors.New("ai_provider_unavailable")
	}
	prompt, err := inspectionAIPrompt(a.masker, setup, components, checks)
	if err != nil {
		return inspectionAIProposal{}, err
	}
	result := a.runCodexInvocation(ctx, profileID, strings.TrimSpace(request.RequestedModel), root, prompt)
	if result.State != domain.AssuranceStateSucceeded || result.RawTranscript || result.Structured == nil {
		return inspectionAIProposal{}, errors.New("ai_execution_failed")
	}
	decisions, err := parseInspectionAIOutput(result.Structured)
	if err != nil {
		return inspectionAIProposal{}, err
	}
	return inspectionAIProposal{
		Provider:  provider,
		Model:     strings.TrimSpace(request.RequestedModel),
		Decisions: decisions,
		Digest:    digestText("inspection-ai-proposal-v1", decisions),
	}, nil
}

func inspectionAIPrompt(masker interface{ Mask(string) string }, setup qualitysetup.Report, components []domain.InspectionComponent, checks []domain.InspectionCheck) (string, error) {
	evidence := inspectionAIEvidence{
		DetectedLanguages: boundedInspectionStrings(setup.DetectedLanguages, 16),
		Components:        inspectionAIComponents(setup),
		Warnings:          boundedInspectionStrings(setup.Warnings, 16),
		EvidenceDigest:    "sha256:" + setup.Digest,
		ConfigDigest:      inspectionConfigDigestForSetup(setup, components),
		ToolDigest:        inspectionToolDigestForCurrent(checks),
	}
	data, err := json.Marshal(evidence)
	if err != nil {
		return "", errors.New("ai_evidence_malformed")
	}
	prompt := "Return only the fixed schema result. Findings must be zero or more exact enum decisions include:<known-check-id> or exclude:<known-check-id>. Use only known check IDs; never return commands, argv, paths, packages, versions, source, or secrets. Evidence: " + string(data)
	if masker != nil {
		prompt = masker.Mask(prompt)
	}
	if len([]byte(prompt)) > assurance.CodexPromptMaxBytes {
		return "", errors.New("ai_evidence_too_large")
	}
	return prompt, nil
}

func parseInspectionAIOutput(value map[string]any) ([]string, error) {
	if value == nil || len(value) != 3 {
		return nil, errors.New("ai_output_malformed")
	}
	for _, key := range []string{"summary", "findings", "nextAction"} {
		if _, ok := value[key]; !ok {
			return nil, errors.New("ai_output_malformed")
		}
	}
	if summary, ok := value["summary"].(string); !ok || strings.TrimSpace(summary) == "" || len([]byte(summary)) > 2000 {
		return nil, errors.New("ai_output_malformed")
	}
	if nextAction, ok := value["nextAction"].(string); !ok || strings.TrimSpace(nextAction) == "" || len([]byte(nextAction)) > 2000 {
		return nil, errors.New("ai_output_malformed")
	}
	findings, ok := value["findings"].([]any)
	if !ok || len(findings) > 20 {
		return nil, errors.New("ai_output_malformed")
	}
	decisions := make([]string, 0, len(findings))
	seen := make(map[string]struct{}, len(findings))
	for _, raw := range findings {
		decision, ok := raw.(string)
		if !ok || decision != strings.TrimSpace(decision) {
			return nil, errors.New("ai_output_malformed")
		}
		kind, checkID, ok := strings.Cut(decision, ":")
		if !ok || (kind != "include" && kind != "exclude") || !domain.IsKnownInspectionCheckID(checkID) {
			return nil, errors.New("ai_output_malformed")
		}
		if _, exists := seen[decision]; exists {
			return nil, errors.New("ai_output_malformed")
		}
		seen[decision] = struct{}{}
		decisions = append(decisions, decision)
	}
	return decisions, nil
}

func applyInspectionAIDecisions(checks, candidates []domain.InspectionCheck, decisions []string) ([]domain.InspectionCheck, error) {
	result := append([]domain.InspectionCheck{}, checks...)
	for _, decision := range decisions {
		kind, checkID, _ := strings.Cut(decision, ":")
		switch kind {
		case "include":
			matched := false
			for _, candidate := range candidates {
				if candidate.ID != checkID {
					continue
				}
				matched = true
				result = append(result, candidate)
			}
			if !matched {
				return nil, errors.New("ai_output_not_applicable")
			}
		case "exclude":
			filtered := result[:0]
			for _, check := range result {
				if check.ID != checkID {
					filtered = append(filtered, check)
				}
			}
			result = filtered
		}
	}
	result = uniqueInspectionChecks(result)
	sort.Slice(result, func(i, j int) bool { return inspectionCheckKey(result[i]) < inspectionCheckKey(result[j]) })
	if len(result) == 0 {
		return nil, errors.New("ai_output_not_applicable")
	}
	return result, nil
}

func inspectionAIComponents(report qualitysetup.Report) []inspectionAIComponent {
	result := make([]inspectionAIComponent, 0, len(report.Components))
	for _, component := range report.Components {
		checks := make([]inspectionAICheck, 0, len(component.Checks))
		for _, check := range component.Checks {
			if !domain.IsKnownInspectionCheckID(inspectionCheckID(check.ID)) {
				continue
			}
			checks = append(checks, inspectionAICheck{ID: inspectionCheckID(check.ID), Status: check.Status})
		}
		result = append(result, inspectionAIComponent{ID: component.ID, Path: relativeInspectionPath(component.Path), Checks: checks})
	}
	if len(result) > 32 {
		result = result[:32]
	}
	return result
}

func relativeInspectionPath(value string) string {
	value = filepath.ToSlash(strings.TrimSpace(value))
	if value == "" || filepath.IsAbs(value) || value == ".." || strings.HasPrefix(value, "../") {
		return "."
	}
	return value
}

func boundedInspectionStrings(values []string, max int) []string {
	result := make([]string, 0, inspectionMinInt(len(values), max))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if len([]byte(value)) > 160 {
			value = string([]byte(value)[:160])
		}
		result = append(result, value)
		if len(result) == max {
			break
		}
	}
	return result
}

func inspectionMinInt(first, second int) int {
	if first < second {
		return first
	}
	return second
}

func deterministicQualityImprovements(plan domain.InspectionPlan, setup qualitysetup.Report, root string, results []domain.InspectionResult) ([]domain.QualityImprovementChange, string) {
	planChecks := make(map[string]domain.InspectionCheck, len(plan.Spec.Checks))
	for _, check := range plan.Spec.Checks {
		planChecks[check.ComponentID+"\x00"+check.ID] = check
	}
	_, candidates := inspectionPlanParts(setup)
	candidates = runnableInspectionChecks(setup, root, candidates)
	changes := []domain.QualityImprovementChange{}
	rationale := ""
	for _, result := range results {
		if !validInspectionResult(result) {
			continue
		}
		key := result.ComponentID + "\x00" + result.CheckID
		switch result.Outcome {
		case domain.InspectionOutcomeRunnerUnavailable:
			if _, exists := planChecks[key]; exists && len(plan.Spec.Checks)-len(changes) > 1 {
				changes = append(changes, domain.QualityImprovementChange{Action: domain.QualityImprovementActionRemoveCheck, ComponentID: result.ComponentID, CheckID: result.CheckID, Reason: domain.QualityImprovementReasonRunnerUnavailable})
				rationale = domain.QualityImprovementReasonRunnerUnavailable
			}
		case domain.InspectionOutcomeInconclusive:
			if _, exists := planChecks[key]; exists && len(plan.Spec.Checks)-len(changes) > 1 {
				changes = append(changes, domain.QualityImprovementChange{Action: domain.QualityImprovementActionRemoveCheck, ComponentID: result.ComponentID, CheckID: result.CheckID, Reason: domain.QualityImprovementReasonInconclusive})
				rationale = domain.QualityImprovementReasonInconclusive
			}
		case domain.InspectionOutcomeFindings, domain.InspectionOutcomeTestsFailed:
			for _, candidate := range candidates {
				candidateKey := candidate.ComponentID + "\x00" + candidate.ID
				if candidate.ComponentID != result.ComponentID {
					continue
				}
				if _, exists := planChecks[candidateKey]; exists {
					continue
				}
				changes = append(changes, domain.QualityImprovementChange{Action: domain.QualityImprovementActionAddCheck, ComponentID: candidate.ComponentID, CheckID: candidate.ID, Reason: domain.QualityImprovementReasonFindingObserved})
				rationale = domain.QualityImprovementReasonFindingObserved
				break
			}
		}
	}
	return uniqueQualityImprovementChanges(changes), rationale
}

func uniqueQualityImprovementChanges(changes []domain.QualityImprovementChange) []domain.QualityImprovementChange {
	result := make([]domain.QualityImprovementChange, 0, len(changes))
	seen := make(map[string]struct{}, len(changes))
	for _, change := range changes {
		key := change.Action + "\x00" + change.ComponentID + "\x00" + change.CheckID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, change)
	}
	return result
}

func applyQualityImprovementChanges(plan domain.InspectionPlan, changes []domain.QualityImprovementChange) ([]domain.InspectionCheck, error) {
	checks := append([]domain.InspectionCheck{}, plan.Spec.Checks...)
	for _, change := range changes {
		if !domain.IsKnownInspectionCheckID(change.CheckID) {
			return nil, errors.New("quality improvement contains an unknown check")
		}
		switch change.Action {
		case domain.QualityImprovementActionAddCheck:
			knownComponent := false
			for _, component := range plan.Spec.Components {
				knownComponent = knownComponent || component.ID == change.ComponentID
			}
			if !knownComponent {
				return nil, errors.New("quality improvement references an unknown component")
			}
			for _, check := range checks {
				if check.ComponentID == change.ComponentID && check.ID == change.CheckID {
					return nil, errors.New("quality improvement adds an existing check")
				}
			}
			checks = append(checks, inspectionCheck(change.CheckID, change.ComponentID))
		case domain.QualityImprovementActionRemoveCheck, domain.QualityImprovementActionMarkRunnerUnavailable:
			filtered := checks[:0]
			removed := false
			for _, check := range checks {
				if check.ComponentID == change.ComponentID && check.ID == change.CheckID {
					removed = true
					continue
				}
				filtered = append(filtered, check)
			}
			if !removed {
				return nil, errors.New("quality improvement removes a missing check")
			}
			checks = filtered
		default:
			return nil, errors.New("quality improvement action is invalid")
		}
	}
	if len(checks) == 0 {
		return nil, errors.New("quality improvement cannot remove every inspection check")
	}
	sort.Slice(checks, func(i, j int) bool { return inspectionCheckKey(checks[i]) < inspectionCheckKey(checks[j]) })
	return uniqueInspectionChecks(checks), nil
}

func (a *App) staleQualityImprovement(ctx context.Context, item domain.QualityImprovementProposal, reason string) (QualityImprovementApplyView, error) {
	now := time.Now().UTC()
	expectedRevision := item.Spec.Revision
	item.Spec.State, item.Spec.Revision, item.Spec.UpdatedAt = domain.QualityImprovementStateStale, expectedRevision+1, now
	if err := a.store.UpdateQualityImprovementProposalRevisionCAS(ctx, domain.QualityImprovementProposalKind, item.Metadata.ID, expectedRevision, item); err != nil && !errors.Is(err, store.ErrQualityImprovementRevisionStale) {
		return QualityImprovementApplyView{}, err
	}
	return QualityImprovementApplyView{}, contract.Conflict(reason)
}

func qualityImprovementReviewState(current, decision string) (string, error) {
	switch decision {
	case "review", "reviewed":
		if current != domain.QualityImprovementStateProposed {
			return "", errors.New("only proposed quality improvements can be reviewed")
		}
		return domain.QualityImprovementStateReviewed, nil
	case "approve", "approved":
		if current != domain.QualityImprovementStateProposed && current != domain.QualityImprovementStateReviewed {
			return "", errors.New("only proposed or reviewed quality improvements can be approved")
		}
		return domain.QualityImprovementStateApproved, nil
	case "reject", "rejected":
		if current != domain.QualityImprovementStateProposed && current != domain.QualityImprovementStateReviewed {
			return "", errors.New("only proposed or reviewed quality improvements can be rejected")
		}
		return domain.QualityImprovementStateRejected, nil
	default:
		return "", errors.New("quality improvement decision must be review, approve, or reject")
	}
}

type inspectionRunArtifact struct {
	PlanID  string                    `json:"planId"`
	Results []domain.InspectionResult `json:"results"`
	ScoreID string                    `json:"scoreId"`
}

func (a *App) loadInspectionRunArtifact(ctx context.Context, id string) (inspectionRunArtifact, error) {
	var artifact domain.Artifact
	if err := a.store.GetAssurance(ctx, domain.ArtifactKind, id, &artifact); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return inspectionRunArtifact{}, contract.NotFound("inspection result artifact not found")
		}
		return inspectionRunArtifact{}, err
	}
	if artifact.Spec.SourceType != inspectionResultArtifactType || artifact.Spec.MIME != inspectionResultArtifactMIME || artifact.Spec.Path == "" || artifact.Spec.Size < 0 || artifact.Spec.Size > 256<<10 {
		return inspectionRunArtifact{}, contract.InvalidInput("invalid inspection result artifact")
	}
	home, err := filepath.Abs(a.home)
	if err != nil {
		return inspectionRunArtifact{}, contract.Unavailable("inspection result artifact is unavailable")
	}
	path, err := filepath.Abs(artifact.Spec.Path)
	if err != nil {
		return inspectionRunArtifact{}, contract.Unavailable("inspection result artifact is unavailable")
	}
	relative, err := filepath.Rel(home, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return inspectionRunArtifact{}, contract.InvalidInput("inspection result artifact path is outside assurance storage")
	}
	data, err := os.ReadFile(path)
	if err != nil || int64(len(data)) != artifact.Spec.Size || len(data) > 256<<10 || artifactHash(data) != artifact.Spec.SHA256 {
		return inspectionRunArtifact{}, contract.Unavailable("inspection result artifact is unavailable")
	}
	var run inspectionRunArtifact
	if err := json.Unmarshal(data, &run); err != nil || run.PlanID == "" || run.ScoreID == "" || len(run.Results) == 0 {
		return inspectionRunArtifact{}, contract.InvalidInput("inspection result artifact is malformed")
	}
	return run, nil
}

func inspectionCheckID(value string) string {
	switch value {
	case "ruff":
		return domain.InspectionCheckRuff
	case "pytest":
		return domain.InspectionCheckPytest
	case "eslint":
		return domain.InspectionCheckESLint
	case "vitest":
		return domain.InspectionCheckVitest
	case "go-test":
		return domain.InspectionCheckGoTest
	default:
		return ""
	}
}

func inspectionCheck(id, componentID string) domain.InspectionCheck {
	adapter, parser := id, id+".result.v1"
	switch id {
	case domain.InspectionCheckRuff:
		adapter, parser = assurance.QualityAdapterRuffID, assurance.QualityParserRuffJSON
	case domain.InspectionCheckPytest:
		adapter, parser = assurance.QualityAdapterPytestID, assurance.QualityParserPytest
	case domain.InspectionCheckESLint:
		adapter, parser = assurance.QualityAdapterESLintID, assurance.QualityParserESLintJSON
	case domain.InspectionCheckVitest:
		adapter, parser = assurance.QualityAdapterVitestID, assurance.QualityParserVitestJSON
	}
	return domain.InspectionCheck{ID: id, ComponentID: componentID, AdapterID: adapter, ParserID: parser}
}

func validateReviewedInspectionChecks(checks []domain.InspectionCheck) error {
	for _, check := range checks {
		expected := inspectionCheck(check.ID, check.ComponentID)
		if check.AdapterID != expected.AdapterID || check.ParserID != expected.ParserID {
			return errors.New("inspection plan contains an unreviewed check definition")
		}
	}
	return nil
}

func uniqueInspectionChecks(items []domain.InspectionCheck) []domain.InspectionCheck {
	result := make([]domain.InspectionCheck, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		key := inspectionCheckKey(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func inspectionPlanView(plan domain.InspectionPlan) InspectionPlanView {
	generation := InspectionPlanGeneration{Source: inspectionGenerationSource, AIProposal: false, Reason: inspectionGenerationReason}
	if plan.Spec.GenerationSource == domain.InspectionGenerationAI {
		generation = InspectionPlanGeneration{
			Source: plan.Spec.GenerationSource, AIProposal: true, Reason: "ai_bounded_proposal_applied",
			Provider: plan.Spec.GenerationProvider, Model: plan.Spec.GenerationModel,
			ProposalDigest: plan.Spec.AIProposalDigest, Decisions: append([]string{}, plan.Spec.AIProposalDecisions...),
		}
	}
	return inspectionPlanViewWithGeneration(plan, generation)
}

func inspectionPlanViewWithGeneration(plan domain.InspectionPlan, generation InspectionPlanGeneration) InspectionPlanView {
	return InspectionPlanView{InspectionPlan: plan, Generation: generation}
}

func normalizeInspectionPlanGenerateInput(input InspectionPlanGenerateInput) InspectionPlanGenerateInput {
	input.ProjectID, input.RepositoryID, input.WorktreeID = strings.TrimSpace(input.ProjectID), strings.TrimSpace(input.RepositoryID), strings.TrimSpace(input.WorktreeID)
	return input
}

func inspectionReviewState(current, decision string) (string, error) {
	switch decision {
	case "review", "reviewed":
		if current != domain.InspectionPlanStateProposed {
			return "", errors.New("only proposed inspection plans can be reviewed")
		}
		return domain.InspectionPlanStateReviewed, nil
	case "approve", "approved":
		if current != domain.InspectionPlanStateProposed && current != domain.InspectionPlanStateReviewed {
			return "", errors.New("only proposed or reviewed inspection plans can be approved")
		}
		return domain.InspectionPlanStateApproved, nil
	case "reject", "rejected":
		if current != domain.InspectionPlanStateProposed && current != domain.InspectionPlanStateReviewed {
			return "", errors.New("only proposed or reviewed inspection plans can be rejected")
		}
		return domain.InspectionPlanStateRejected, nil
	default:
		return "", errors.New("inspection plan decision must be review, approve, or reject")
	}
}

func inspectionConfigDigest(components []domain.InspectionComponent, checks []domain.InspectionCheck) string {
	return digestText("inspection-config-v1", components, checks)
}

func inspectionToolDigest(checks []domain.InspectionCheck) string {
	return digestText("inspection-tools-v1", checks)
}

func inspectionConfigDigestForSetup(report qualitysetup.Report, components []domain.InspectionComponent) string {
	type configComponent struct {
		ID             string   `json:"id"`
		Path           string   `json:"path"`
		Languages      []string `json:"languages"`
		Frameworks     []string `json:"frameworks"`
		PackageManager string   `json:"packageManager"`
		Checks         []string `json:"checks"`
	}
	config := make([]configComponent, 0, len(report.Components))
	for _, component := range report.Components {
		checkStates := make([]string, 0, len(component.Checks))
		for _, check := range component.Checks {
			checkID := inspectionCheckID(check.ID)
			if checkID != "" {
				checkStates = append(checkStates, checkID+":"+check.Status)
			}
		}
		sort.Strings(checkStates)
		config = append(config, configComponent{ID: component.ID, Path: relativeInspectionPath(component.Path), Languages: append([]string{}, component.Languages...), Frameworks: append([]string{}, component.Frameworks...), PackageManager: component.PackageManager, Checks: checkStates})
	}
	sort.Slice(config, func(i, j int) bool { return config[i].ID < config[j].ID })
	return digestText("inspection-config-v2", report.DetectedLanguages, report.SelectedLanguages, report.Warnings, components, config)
}

func inspectionToolDigestForCurrent(checks []domain.InspectionCheck) string {
	return inspectionToolDigestForRoot(checks, "")
}

func inspectionToolDigestForRoot(checks []domain.InspectionCheck, root string) string {
	return inspectionToolDigestForSetupRoot(checks, root, qualitysetup.Report{})
}

func inspectionToolDigestForSetupRoot(checks []domain.InspectionCheck, root string, report qualitysetup.Report) string {
	type tool struct {
		CheckID         string `json:"checkId"`
		Runner          string `json:"runner"`
		Executable      string `json:"executable,omitempty"`
		ExecutableState string `json:"executableState"`
		Package         string `json:"package,omitempty"`
		PackageState    string `json:"packageState"`
		PackageVersion  string `json:"packageVersion,omitempty"`
		ResolvedVersion string `json:"resolvedVersion,omitempty"`
	}
	tools := make([]tool, 0, len(checks))
	for _, check := range checks {
		toolRoot := root
		if component, ok := qualitySetupComponent(report, check.ComponentID); ok {
			toolRoot = qualityComponentRoot(root, component.Path)
		}
		state := inspectionToolState(toolRoot, check)
		tools = append(tools, tool{CheckID: check.ID, Runner: state.Runner, Executable: state.Executable, ExecutableState: state.ExecutableState, Package: state.Package, PackageState: state.PackageState, PackageVersion: state.PackageVersion, ResolvedVersion: state.ResolvedVersion})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].CheckID < tools[j].CheckID })
	return digestText("inspection-tools-v3", tools)
}

func inspectionRunnerForCheck(checkID string) (string, bool) {
	state := inspectionToolState("", domain.InspectionCheck{ID: checkID})
	return state.Runner, state.ExecutableState == "available" && state.PackageState != "unavailable"
}

type inspectionToolResolution struct {
	Runner          string
	Executable      string
	ExecutableState string
	Package         string
	PackageState    string
	PackageVersion  string
	ResolvedVersion string
}

func inspectionToolState(root string, check domain.InspectionCheck) inspectionToolResolution {
	state := inspectionToolResolution{ExecutableState: "unavailable", PackageState: "not_applicable"}
	switch check.ID {
	case domain.InspectionCheckRuff, domain.InspectionCheckPytest:
		state.Runner = "python"
		state.Package = "ruff"
		if check.ID == domain.InspectionCheckPytest {
			state.Package = "pytest"
		}
		path := qualityLookPath("python.exe", "python")
		if root != "" {
			path, _ = resolveQualityPython(root, root, qualityLookPath)
		}
		state.Executable = path
		if path == "" {
			state.PackageState = "unavailable"
			return state
		}
		if !qualityExecutableVerified(path, "python.exe", "python") {
			state.ExecutableState, state.PackageState = "unavailable", "unavailable"
			return state
		}
		state.ExecutableState = "inconclusive"
		if root != "" && qualityPythonPackageInstalled(path, check.ID) {
			state.PackageVersion = pythonPackageVersion(path, state.Package)
			if state.PackageVersion == "" {
				state.PackageState = "inconclusive"
			} else {
				state.PackageState, state.ResolvedVersion = "available", state.PackageVersion
			}
		} else {
			state.PackageState = "inconclusive"
		}
		return state
	case domain.InspectionCheckESLint, domain.InspectionCheckVitest:
		state.Runner = "node"
		state.Package = "eslint"
		if check.ID == domain.InspectionCheckVitest {
			state.Package = "vitest"
		}
		path := qualityLookPath("node.exe", "node")
		state.Executable = path
		if path == "" {
			state.PackageState = "unavailable"
			return state
		}
		if !qualityExecutableVerified(path, "node.exe", "node") {
			state.ExecutableState, state.PackageState = "unavailable", "unavailable"
			return state
		}
		state.ExecutableState = "inconclusive"
		if root != "" {
			manifest := filepath.Join(root, "node_modules", state.Package, "package.json")
			var value struct {
				Version string `json:"version"`
			}
			if data, err := os.ReadFile(manifest); err == nil && json.Unmarshal(data, &value) == nil {
				state.PackageVersion = strings.TrimSpace(value.Version)
				if state.PackageVersion != "" {
					state.PackageState, state.ResolvedVersion = "available", state.PackageVersion
				} else {
					state.PackageState = "inconclusive"
				}
			} else {
				state.PackageState = "unavailable"
			}
		} else {
			state.PackageState = "inconclusive"
		}
		return state
	case domain.InspectionCheckGoTest, domain.InspectionCheckGoTestRace, domain.InspectionCheckGoVet, domain.InspectionCheckGoModVerify, domain.InspectionCheckGoBuild, domain.InspectionCheckGoCoverage, domain.InspectionCheckGoCoveragePercent, domain.InspectionCheckGoMutation, domain.InspectionCheckGoProperty, domain.InspectionCheckGoFuzz, domain.InspectionCheckGoE2E, domain.InspectionCheckGoTestCoverage:
		state.Runner, state.Package = "go", "go toolchain"
		path := qualityLookPath("go.exe", "go")
		state.Executable = path
		if path == "" {
			state.PackageState = "unavailable"
			return state
		}
		if !qualityExecutableVerified(path, "go.exe", "go") {
			state.ExecutableState, state.PackageState = "unavailable", "unavailable"
			return state
		}
		state.ExecutableState = "inconclusive"
		return state
	default:
		return state
	}
}

func pythonPackageVersion(interpreterPath, packageName string) string {
	for _, directory := range pythonPackageDirectories(interpreterPath) {
		entries, err := os.ReadDir(directory)
		if err != nil {
			continue
		}
		prefix := strings.ToLower(packageName) + "-"
		for _, entry := range entries {
			name := strings.ToLower(entry.Name())
			if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".dist-info") {
				continue
			}
			version := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".dist-info")
			if exactQualityVersionPattern.MatchString(version) {
				return version
			}
		}
	}
	return ""
}

func pythonPackageDirectories(interpreterPath string) []string {
	interpreterPath = filepath.Clean(interpreterPath)
	environmentRoot := filepath.Dir(interpreterPath)
	if strings.EqualFold(filepath.Base(environmentRoot), "scripts") {
		environmentRoot = filepath.Dir(environmentRoot)
	}
	return []string{
		filepath.Join(environmentRoot, "Lib", "site-packages"),
		filepath.Join(environmentRoot, "lib", "site-packages"),
		filepath.Join(environmentRoot, "lib"),
	}
}

func containsQualityLanguage(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func planRootForComponent(report qualitysetup.Report, componentID string) string {
	for _, component := range report.Components {
		if component.ID == componentID {
			return filepath.Clean(component.Path)
		}
	}
	return "."
}

func qualityComponentRoot(root, componentPath string) string {
	componentPath = filepath.Clean(componentPath)
	if filepath.IsAbs(componentPath) {
		return componentPath
	}
	return filepath.Join(root, componentPath)
}

func qualityLookPath(names ...string) string {
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}
