package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/assurance"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/discovery"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/environment"
	"github.com/knowgyu/dev-control-room/internal/masking"
)

const agentInvocationLease = 2 * time.Hour

type retryInvocationLock struct {
	available  chan struct{}
	references int
}

func (a *App) AssuranceSessions(ctx context.Context) ([]domain.AssuranceSession, error) {
	return a.store.ListAssuranceSessions(ctx)
}

func (a *App) AssuranceSession(ctx context.Context, id string) (domain.AssuranceSession, error) {
	var item domain.AssuranceSession
	if err := a.store.GetAssurance(ctx, domain.AssuranceSessionKind, id, &item); err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, os.ErrNotExist) {
			return item, contract.NotFound("assurance session not found")
		}
		return item, err
	}
	return item, nil
}

func (a *App) AssuranceQuestions(ctx context.Context, sessionID string) ([]domain.AssuranceQuestion, error) {
	return a.store.ListAssuranceQuestions(ctx, sessionID)
}

func (a *App) AssuranceSpecs(ctx context.Context, sessionID string) ([]domain.AssuranceSpec, error) {
	return a.store.ListAssuranceSpecs(ctx, sessionID)
}

func (a *App) AssuranceProposals(ctx context.Context, sessionID string) ([]domain.AssuranceProposal, error) {
	return a.store.ListAssuranceProposals(ctx, sessionID)
}

func (a *App) QualityCampaigns(ctx context.Context) ([]domain.QualityCampaign, error) {
	return a.store.ListQualityCampaigns(ctx)
}

func (a *App) QualityRuns(ctx context.Context) ([]domain.QualityRun, error) {
	return a.store.ListQualityRuns(ctx)
}

func (a *App) AgentInvocations(ctx context.Context) ([]domain.AgentInvocation, error) {
	return a.store.ListAgentInvocations(ctx)
}

func (a *App) PRCIBaselines(ctx context.Context) ([]domain.PRCIBaseline, error) {
	items, err := a.store.ListBaselines(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for index := range items {
		item := &items[index]
		if now.After(item.Spec.FreshUntil) {
			item.Spec.State, item.Spec.StaleReason = "stale", "baseline freshness expired"
			continue
		}
		worktree, worktreeErr := a.Worktree(ctx, item.Spec.ProjectID, item.Spec.RepositoryID, item.Spec.WorktreeID)
		if worktreeErr != nil || worktree.Spec.Head != item.Spec.Head {
			item.Spec.State, item.Spec.StaleReason = "stale", "target Worktree HEAD changed"
		}
	}
	return items, nil
}

func (a *App) AssuranceArtifacts(ctx context.Context) ([]domain.Artifact, error) {
	return a.store.ListArtifacts(ctx)
}

func (a *App) AssuranceEffects(ctx context.Context) ([]domain.Effect, error) {
	return a.store.ListEffects(ctx)
}

func (a *App) PricingSnapshots(ctx context.Context) ([]domain.ProviderPricingSnapshot, error) {
	return a.store.ListPricingSnapshots(ctx)
}

func (a *App) AssuranceDashboard(ctx context.Context, providerFilter, modelFilter string) (AssuranceDashboard, error) {
	invocations, err := a.AgentInvocations(ctx)
	if err != nil {
		return AssuranceDashboard{}, err
	}
	effects, err := a.AssuranceEffects(ctx)
	if err != nil {
		return AssuranceDashboard{}, err
	}
	pricing, err := a.PricingSnapshots(ctx)
	if err != nil {
		return AssuranceDashboard{}, err
	}
	providerFilter, modelFilter = strings.TrimSpace(providerFilter), strings.TrimSpace(modelFilter)
	filtered := make([]domain.AgentInvocation, 0, len(invocations))
	var total int64
	complete := true
	var cost float64
	costKnown := true
	for _, invocation := range invocations {
		if providerFilter != "" && invocation.Spec.Provider != providerFilter {
			continue
		}
		if modelFilter != "" && invocation.Spec.RequestedModel != modelFilter {
			continue
		}
		filtered = append(filtered, invocation)
		if invocation.Spec.Usage.TotalTokens == nil {
			complete = false
		} else {
			total += *invocation.Spec.Usage.TotalTokens
		}
		if invocation.Spec.Usage.InputTokens == nil || invocation.Spec.Usage.OutputTokens == nil {
			costKnown = false
			continue
		}
		snapshot, found := pricingFor(pricing, invocation.Spec.Provider, invocation.Spec.RequestedModel, invocation.Spec.StartedAt)
		if !found {
			costKnown = false
			continue
		}
		cost += float64(*invocation.Spec.Usage.InputTokens)*snapshot.Spec.InputPerMillion/1_000_000 + float64(*invocation.Spec.Usage.OutputTokens)*snapshot.Spec.OutputPerMillion/1_000_000
	}
	var estimated *float64
	costState := "unknown"
	if costKnown && len(filtered) > 0 {
		estimated = &cost
		costState = "estimated"
	}
	// Keep the legacy dashboard usable when the optional impact surface is
	// unavailable. The dedicated impact endpoint reports that failure to the UI
	// while the operational invocation/effect lists remain visible.
	impact, impactErr := a.AssuranceImpact(ctx, AssuranceImpactQuery{Provider: providerFilter, Model: modelFilter, Days: defaultImpactPeriodDays})
	if impactErr != nil {
		impact = AssuranceImpactDashboard{}
	}
	return AssuranceDashboard{GeneratedAt: time.Now().UTC(), ProviderFilter: providerFilter, ModelFilter: modelFilter, Effects: effects, Invocations: filtered, TotalTokens: total, UsageComplete: complete, EstimatedCost: estimated, CostLabel: "estimated public API list-price equivalent", CostState: costState, Impact: impact, Traceability: impact.Traceability}, nil
}

func pricingFor(items []domain.ProviderPricingSnapshot, provider, model string, at time.Time) (domain.ProviderPricingSnapshot, bool) {
	var selected domain.ProviderPricingSnapshot
	found := false
	for _, item := range items {
		if item.Spec.Provider != provider || item.Spec.Model != model || item.Spec.EffectiveAt.After(at) {
			continue
		}
		if !found || item.Spec.EffectiveAt.After(selected.Spec.EffectiveAt) {
			selected, found = item, true
		}
	}
	return selected, found
}

func (a *App) ProviderStatuses(ctx context.Context) ([]ProviderStatus, error) {
	_ = ctx
	codex := assurance.CodexResolver{}
	items := []ProviderStatus{providerStatus(codex.Resolve())}
	for _, provider := range []string{"claude", "gemini"} {
		items = append(items, ProviderStatus{Provider: provider, State: string(assurance.ProviderNotConfigured), ReasonCode: "provider.optional_not_configured", Detail: "선택한 경우에만 설정합니다."})
	}
	return items, nil
}

func providerStatus(status assurance.ProviderStatus) ProviderStatus {
	return ProviderStatus{Provider: status.Provider, State: string(status.State), CommandFound: status.CommandFound, LaunchTrusted: status.LaunchTrusted, ProfileReady: status.ProfileReady, ResolvedCommand: append([]string(nil), status.ResolvedCommand...), Version: status.Version, ReasonCode: status.ReasonCode, Detail: status.Detail}
}

func (a *App) CreateAssuranceSession(ctx context.Context, input AssuranceSessionInput) (domain.AssuranceSession, error) {
	worktree, err := a.Worktree(ctx, input.ProjectID, input.RepositoryID, input.WorktreeID)
	if err != nil {
		return domain.AssuranceSession{}, err
	}
	for _, existing := range mustAssuranceSessions(a.store.ListAssuranceSessions(ctx)) {
		if existing.Spec.ProjectID == input.ProjectID && existing.Spec.RepositoryID == input.RepositoryID && existing.Spec.WorktreeID == input.WorktreeID && isActiveAssuranceState(existing.Spec.State) {
			return domain.AssuranceSession{}, contract.Conflict("an active assurance session already exists for this Worktree")
		}
	}
	now := time.Now().UTC()
	id := assuranceID("session", input.ProjectID, input.RepositoryID, input.WorktreeID, now)
	item := domain.AssuranceSession{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AssuranceSessionKind}, Metadata: domain.ObjectMeta{ID: id, Name: "Assurance session"}, Spec: domain.AssuranceSessionSpec{ProjectID: input.ProjectID, RepositoryID: input.RepositoryID, WorktreeID: input.WorktreeID, Head: worktree.Spec.Head, State: domain.AssuranceStateDraft, Provider: strings.TrimSpace(input.Provider), RequestedModel: strings.TrimSpace(input.RequestedModel), CreatedAt: now, UpdatedAt: now, ResumeBrief: domain.ResumeBrief{SessionID: id, CurrentHead: worktree.Spec.Head, NextSafeAction: "CI baseline을 먼저 확인합니다."}}}
	if err := a.store.SaveAssuranceSession(ctx, item); err != nil {
		return domain.AssuranceSession{}, err
	}
	return item, nil
}

func (a *App) AnswerAssuranceQuestion(ctx context.Context, sessionID, questionID, answer string) (domain.AssuranceSession, error) {
	session, err := a.AssuranceSession(ctx, sessionID)
	if err != nil {
		return domain.AssuranceSession{}, err
	}
	questions, err := a.AssuranceQuestions(ctx, sessionID)
	if err != nil {
		return domain.AssuranceSession{}, err
	}
	var answered *domain.AssuranceQuestion
	for index := range questions {
		if questions[index].Metadata.ID == questionID {
			answered = &questions[index]
			break
		}
	}
	if answered == nil || strings.TrimSpace(answer) == "" {
		return domain.AssuranceSession{}, contract.InvalidInput("answer requires a known question and non-empty answer")
	}
	now := time.Now().UTC()
	answered.Spec.Answer = strings.TrimSpace(answer)
	answered.Spec.AnsweredAt = &now
	if err := a.store.UpdateAssuranceQuestion(ctx, *answered); err != nil {
		return domain.AssuranceSession{}, err
	}
	session.Spec.State = domain.AssuranceStateReady
	session.Spec.UpdatedAt = now
	session.Spec.ResumeBrief.WaitingQuestion = ""
	session.Spec.ResumeBrief.NextSafeAction = "답변을 반영해 Assurance Spec을 검토합니다."
	if err := a.updateAssuranceSession(ctx, session); err != nil {
		return domain.AssuranceSession{}, err
	}
	return session, nil
}

func (a *App) CreateAssuranceQuestion(ctx context.Context, input AssuranceQuestionInput) (domain.AssuranceQuestion, error) {
	session, err := a.AssuranceSession(ctx, input.SessionID)
	if err != nil {
		return domain.AssuranceQuestion{}, err
	}
	if strings.TrimSpace(input.Prompt) == "" {
		return domain.AssuranceQuestion{}, contract.InvalidInput("assurance question prompt is required")
	}
	now := time.Now().UTC()
	item := domain.AssuranceQuestion{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AssuranceQuestionKind}, Metadata: domain.ObjectMeta{ID: assuranceID("question", session.Metadata.ID, input.Prompt), Name: "Assurance question"}, Spec: domain.AssuranceQuestionSpec{SessionID: session.Metadata.ID, Prompt: strings.TrimSpace(input.Prompt), Required: input.Required, AskedAt: now}}
	if err := a.store.SaveAssuranceQuestion(ctx, item); err != nil {
		return domain.AssuranceQuestion{}, err
	}
	session.Spec.State = domain.AssuranceStateAwaitingAnswer
	session.Spec.UpdatedAt = now
	session.Spec.QuestionIDs = append(session.Spec.QuestionIDs, item.Metadata.ID)
	session.Spec.ResumeBrief.WaitingQuestion = item.Metadata.ID
	session.Spec.ResumeBrief.NextSafeAction = "질문에 답한 뒤 Assurance Spec을 검토합니다."
	_ = a.updateAssuranceSession(ctx, session)
	return item, nil
}

func (a *App) CreateAssuranceSpec(ctx context.Context, input AssuranceSpecInput) (domain.AssuranceSpec, error) {
	session, err := a.AssuranceSession(ctx, input.SessionID)
	if err != nil {
		return domain.AssuranceSpec{}, err
	}
	if strings.TrimSpace(input.Intent) == "" {
		return domain.AssuranceSpec{}, contract.InvalidInput("assurance intent is required")
	}
	specs, err := a.AssuranceSpecs(ctx, session.Metadata.ID)
	if err != nil {
		return domain.AssuranceSpec{}, err
	}
	now := time.Now().UTC()
	revision := len(specs) + 1
	item := domain.AssuranceSpec{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AssuranceSpecKind}, Metadata: domain.ObjectMeta{ID: assuranceID("spec", session.Metadata.ID, revision), Name: "Assurance Spec"}, Spec: domain.AssuranceSpecSpec{SessionID: session.Metadata.ID, Revision: revision, Intent: strings.TrimSpace(input.Intent), Questions: append([]string(nil), input.Questions...), Properties: append([]string(nil), input.Properties...), Targets: append([]string(nil), input.Targets...), ToolSetup: append([]string(nil), input.ToolSetup...), CreatedAt: now, Source: strings.TrimSpace(input.Source), State: "draft"}}
	if item.Spec.Source == "" {
		item.Spec.Source = "human_review"
	}
	canonical := item
	canonical.Spec.Digest = ""
	digest, _ := canonical.Digest()
	item.Spec.Digest = digest
	if err := a.store.SaveAssuranceSpec(ctx, item); err != nil {
		return domain.AssuranceSpec{}, err
	}
	session.Spec.CurrentSpecID = item.Metadata.ID
	session.Spec.UpdatedAt = now
	session.Spec.State = domain.AssuranceStateReady
	session.Spec.ResumeBrief.NextSafeAction = "Spec와 Quality Run 목적을 검토합니다."
	_ = a.updateAssuranceSession(ctx, session)
	return item, nil
}

func (a *App) CreateAssuranceProposal(ctx context.Context, input AssuranceProposalInput) (domain.AssuranceProposal, error) {
	session, err := a.AssuranceSession(ctx, input.SessionID)
	if err != nil {
		return domain.AssuranceProposal{}, err
	}
	worktree, err := a.Worktree(ctx, session.Spec.ProjectID, session.Spec.RepositoryID, session.Spec.WorktreeID)
	if err != nil {
		return domain.AssuranceProposal{}, err
	}
	if strings.TrimSpace(input.Patch) == "" || strings.TrimSpace(input.Purpose) == "" {
		return domain.AssuranceProposal{}, contract.InvalidInput("proposal purpose and patch are required")
	}
	now := time.Now().UTC()
	id := assuranceID("proposal", session.Metadata.ID, now)
	isolation := filepath.Join(a.home, "assurance", "proposals", id)
	if err := os.MkdirAll(isolation, 0o700); err != nil {
		return domain.AssuranceProposal{}, err
	}
	patchData := []byte(a.masker.Mask(input.Patch))
	patchSum := sha256.Sum256(patchData)
	artifact, err := a.SaveAssuranceArtifact(ctx, ArtifactInput{SourceType: "assurance_proposal", SourceID: id, Name: id + ".patch", MIME: "text/x-diff", Content: patchData})
	if err != nil {
		return domain.AssuranceProposal{}, err
	}
	criticSummary, confidence, state := "검토가 필요합니다.", "unknown", "critic_advisory"
	if strings.Contains(strings.ToLower(input.Patch), "git push") || strings.Contains(strings.ToLower(input.Patch), "git commit") {
		criticSummary, confidence = "commit/push 동작이 포함되어 있어 자동 채택할 수 없습니다.", "high"
	}
	item := domain.AssuranceProposal{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AssuranceProposalKind}, Metadata: domain.ObjectMeta{ID: id, Name: "Assurance proposal"}, Spec: domain.AssuranceProposalSpec{SessionID: session.Metadata.ID, ProjectID: session.Spec.ProjectID, RepositoryID: session.Spec.RepositoryID, WorktreeID: session.Spec.WorktreeID, BaseHead: worktree.Spec.Head, IsolationPath: isolation, PatchArtifactID: artifact.Metadata.ID, PatchDigest: hex.EncodeToString(patchSum[:]), Purpose: strings.TrimSpace(input.Purpose), State: state, CriticSummary: criticSummary, CriticConfidence: confidence, CreatedAt: now}}
	if err := a.store.SaveAssuranceProposal(ctx, item); err != nil {
		return domain.AssuranceProposal{}, err
	}
	session.Spec.ResumeBrief.ProposedPatch = id
	session.Spec.UpdatedAt = now
	session.Spec.ResumeBrief.NextSafeAction = "patch를 검토하고 명시적으로 채택하거나 거절합니다."
	_ = a.updateAssuranceSession(ctx, session)
	return item, nil
}

func (a *App) ReviewAssuranceProposal(ctx context.Context, proposalID, decision string) (domain.AssuranceProposal, error) {
	proposals, err := a.AssuranceProposals(ctx, "")
	if err != nil {
		return domain.AssuranceProposal{}, err
	}
	var item domain.AssuranceProposal
	for _, candidate := range proposals {
		if candidate.Metadata.ID == proposalID {
			item = candidate
			break
		}
	}
	if item.Metadata.ID == "" {
		return item, contract.NotFound("assurance proposal not found")
	}
	if decision != "adopt" && decision != "reject" {
		return item, contract.InvalidInput("proposal decision must be adopt or reject")
	}
	now := time.Now().UTC()
	if decision == "adopt" {
		item.Spec.State = "adopted"
	} else {
		item.Spec.State = "rejected"
	}
	item.Spec.ReviewedAt = &now
	if err := a.store.UpdateAssuranceRevision(ctx, domain.AssuranceProposalKind, item.Metadata.ID, 2, item.Spec.State, now, item); err != nil {
		return domain.AssuranceProposal{}, err
	}
	return item, nil
}

func (a *App) CreatePRCIBaseline(ctx context.Context, input BaselineInput) (domain.PRCIBaseline, error) {
	worktree, err := a.Worktree(ctx, input.ProjectID, input.RepositoryID, input.WorktreeID)
	if err != nil {
		return domain.PRCIBaseline{}, err
	}
	entries, digest, sources, err := discoverBaseline(worktree.Spec.CanonicalPath)
	if err != nil {
		return domain.PRCIBaseline{}, contract.CodedError{Code: contract.ErrorUnavailable, Message: "PR CI baseline discovery is unavailable"}
	}
	now := time.Now().UTC()
	entries, digest, sources = a.enrichPRCIBaseline(ctx, input, worktree, entries, digest, sources, now)
	item := domain.PRCIBaseline{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.PRCIBaselineKind}, Metadata: domain.ObjectMeta{ID: assuranceID("baseline", input.ProjectID, input.RepositoryID, input.WorktreeID, now), Name: "PR CI baseline"}, Spec: domain.PRCIBaselineSpec{ProjectID: input.ProjectID, RepositoryID: input.RepositoryID, WorktreeID: input.WorktreeID, TargetBranch: strings.TrimSpace(input.TargetBranch), Head: worktree.Spec.Head, SourceDigest: digest, CapturedAt: now, FreshUntil: now.Add(24 * time.Hour), State: "fresh", Entries: entries, Sources: sources}}
	if err := a.store.SaveBaseline(ctx, item); err != nil {
		return domain.PRCIBaseline{}, err
	}
	return item, nil
}

func (a *App) CreateQualityCampaign(ctx context.Context, input QualityCampaignInput) (domain.QualityCampaign, error) {
	worktree, err := a.Worktree(ctx, input.ProjectID, input.RepositoryID, input.WorktreeID)
	if err != nil {
		return domain.QualityCampaign{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return domain.QualityCampaign{}, contract.InvalidInput("quality campaign name is required")
	}
	now := time.Now().UTC()
	item := domain.QualityCampaign{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.QualityCampaignKind}, Metadata: domain.ObjectMeta{ID: assuranceID("campaign", input.ProjectID, input.RepositoryID, input.WorktreeID, now), Name: name}, Spec: domain.QualityCampaignSpec{ProjectID: input.ProjectID, RepositoryID: input.RepositoryID, WorktreeID: input.WorktreeID, Name: name, State: domain.AssuranceStateDraft, SessionID: strings.TrimSpace(input.SessionID), CreatedAt: now, UpdatedAt: now}}
	_ = worktree
	if err := a.store.SaveQualityCampaign(ctx, item); err != nil {
		return domain.QualityCampaign{}, err
	}
	return item, nil
}

func (a *App) RunQuality(ctx context.Context, input QualityRunInput) (domain.QualityRun, error) {
	campaigns, err := a.QualityCampaigns(ctx)
	if err != nil {
		return domain.QualityRun{}, err
	}
	var campaign domain.QualityCampaign
	for _, item := range campaigns {
		if item.Metadata.ID == input.CampaignID {
			campaign = item
			break
		}
	}
	if campaign.Metadata.ID == "" {
		return domain.QualityRun{}, contract.NotFound("quality campaign not found")
	}
	worktree, err := a.Worktree(ctx, campaign.Spec.ProjectID, campaign.Spec.RepositoryID, campaign.Spec.WorktreeID)
	if err != nil {
		return domain.QualityRun{}, err
	}
	if !validTechnique(input.Technique) {
		return domain.QualityRun{}, contract.InvalidInput("quality technique is not enabled in v1")
	}
	executionRoot, revalidationErr := a.revalidateAssuranceWorktree(ctx, worktree)
	var selection assurance.QualityRunnerSelection
	var selectionErr error
	if revalidationErr == nil {
		selection, selectionErr = assurance.NewQualityRunnerRegistry().Select(assurance.QualityRunnerSelectionRequest{
			TechniqueID:  input.Technique,
			WorktreeRoot: executionRoot,
		})
	} else {
		selectionErr = revalidationErr
	}
	now := time.Now().UTC()
	runnerID := "quality.registry"
	traceID := assuranceID("trace", campaign.Metadata.ID, input.Technique, now.String())
	configDigest := digestText(input.Technique)
	selectionState := string(assurance.QualityRunnerSelectionUnavailable)
	var unavailable *assurance.QualityRunnerUnavailableReason
	if selectionErr == nil {
		runnerID = selection.Definition.RunnerID
		configDigest = selection.Metadata.ConfigDigest
		selectionState = string(selection.State)
		unavailable = selection.Unavailable
	}
	run := domain.QualityRun{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.QualityRunKind}, Metadata: domain.ObjectMeta{ID: assuranceID("run", campaign.Metadata.ID, input.Technique, now.String()), Name: "Quality Run"}, Spec: domain.QualityRunSpec{CampaignID: campaign.Metadata.ID, ProjectID: campaign.Spec.ProjectID, RepositoryID: campaign.Spec.RepositoryID, WorktreeID: campaign.Spec.WorktreeID, Branch: worktree.Spec.Branch, Head: worktree.Spec.Head, Technique: input.Technique, Runner: runnerID, ConfigDigest: configDigest, InputDigest: digestText(worktree.Spec.Head, input.Technique, configDigest), TraceID: traceID, State: domain.AssuranceStateQueued, StartedAt: now, Evidence: map[string]any{"provider": strings.TrimSpace(input.Provider), "model": strings.TrimSpace(input.Model), "selectionState": selectionState, "configDigest": configDigest}}}
	if revalidationErr != nil {
		run.Spec.Evidence["unavailable"] = &assurance.QualityRunnerUnavailableReason{Code: "worktree.revalidation_failed", Detail: "selected Worktree could not be revalidated"}
	} else if selectionErr == nil {
		run.Spec.Evidence["runnerMetadata"] = selection.Metadata
		if unavailable != nil {
			run.Spec.Evidence["unavailable"] = unavailable
		}
	} else {
		run.Spec.Evidence["unavailable"] = &assurance.QualityRunnerUnavailableReason{Code: "runner.selection_unavailable", Detail: "registered Quality Runner selection could not be completed"}
	}
	if err := a.store.SaveQualityRun(ctx, run); err != nil {
		return domain.QualityRun{}, err
	}
	run.Spec.State = domain.AssuranceStateRunning
	if err := a.store.UpdateAssuranceRevision(ctx, domain.QualityRunKind, run.Metadata.ID, 2, run.Spec.State, now, run); err != nil {
		return domain.QualityRun{}, err
	}
	var processResult environment.Result
	var processErr error
	executed := false
	if revalidationErr != nil {
		run.Spec.State = domain.AssuranceStateFailed
		run.Spec.Summary = "선택한 Worktree를 다시 확인할 수 없습니다."
		run.Spec.StaleReason = "worktree.revalidation_failed"
	} else if selectionErr != nil {
		run.Spec.State = domain.AssuranceStateFailed
		run.Spec.Summary = "Quality Runner를 사용할 수 없습니다."
		run.Spec.StaleReason = "runner.selection_unavailable"
	} else if selection.State != assurance.QualityRunnerSelectionAvailable {
		run.Spec.State = domain.AssuranceStateFailed
		run.Spec.Summary = "선택한 Quality Runner를 사용할 수 없습니다."
		if unavailable != nil {
			run.Spec.StaleReason = unavailable.Code + ": " + unavailable.Detail
		} else {
			run.Spec.StaleReason = "runner.unavailable"
		}
	} else {
		run.Spec.Command = qualityCheckCommand(selection.Command, selection.Definition.Timeout)
		executed = true
		processResult, processErr = (environment.ProcessRunner{OutputLimit: 128 << 10}).RunInDirectory(ctx, selection.Command.Executable, selection.Command.Arguments, qualityRunnerEnvironment(), selection.WorktreeRoot, selection.Definition.Timeout)
		if processErr != nil {
			run.Spec.State = domain.AssuranceStateFailed
			run.Spec.Summary = "선택한 Quality Runner가 실패했습니다."
			run.Spec.StaleReason = "runner.execution_failed"
		} else {
			run.Spec.State = domain.AssuranceStateSucceeded
			run.Spec.Summary = "선택한 Quality Runner가 완료되었습니다."
		}
	}
	completed := time.Now().UTC()
	run.Spec.CompletedAt = &completed
	if executed {
		run.Spec.ExitCode = processResult.ExitCode
	}
	run.Spec.Evidence["result"] = qualityRunResultEvidence(a.masker, run.Spec.State, processResult, processErr, executed)
	report, marshalErr := json.Marshal(qualityRunArtifact(run, executed))
	if marshalErr != nil {
		return domain.QualityRun{}, marshalErr
	}
	run.Spec.OutputDigest = digestText(report)
	artifact, artifactErr := a.SaveAssuranceArtifact(ctx, ArtifactInput{SourceType: "quality_run", SourceID: run.Metadata.ID, Name: run.Metadata.ID + ".json", MIME: "application/json", Content: report, TraceID: traceID})
	if artifactErr != nil {
		return domain.QualityRun{}, artifactErr
	}
	run.Spec.ArtifactIDs = []string{artifact.Metadata.ID}
	if err := a.store.UpdateAssuranceRevision(ctx, domain.QualityRunKind, run.Metadata.ID, 3, run.Spec.State, completed, run); err != nil {
		return domain.QualityRun{}, err
	}
	campaign.Spec.RunIDs = append(campaign.Spec.RunIDs, run.Metadata.ID)
	campaign.Spec.UpdatedAt = completed
	if revision, revisionErr := a.store.AssuranceRevision(ctx, domain.QualityCampaignKind, campaign.Metadata.ID); revisionErr == nil {
		_ = a.store.UpdateAssuranceRevision(ctx, domain.QualityCampaignKind, campaign.Metadata.ID, revision+1, campaign.Spec.State, completed, campaign)
	}
	if selectionErr != nil || (selectionErr == nil && selection.State != assurance.QualityRunnerSelectionAvailable) {
		return run, contract.Unavailable("selected Quality Runner is unavailable")
	}
	if processErr != nil {
		return run, contract.CodedError{Code: contract.ErrorExecutionFailed, Message: "quality run did not succeed"}
	}
	return run, nil
}

func qualityCheckCommand(command assurance.TypedCommand, timeout time.Duration) domain.CheckCommand {
	return domain.CheckCommand{Executable: command.Executable, Arguments: append([]string(nil), command.Arguments...), TimeoutSeconds: int(timeout / time.Second)}
}

func qualityRunnerEnvironment() []string {
	env := environment.AllowlistedEnvironment([]string{"GOCACHE"})
	filtered := make([]string, 0, len(env)+5)
	cacheSet := false
	for _, value := range env {
		name, _, _ := strings.Cut(value, "=")
		switch strings.ToUpper(name) {
		case "GOPROXY", "GOSUMDB", "GOTOOLCHAIN", "GOWORK":
			continue
		case "GOCACHE":
			cacheSet = true
		}
		filtered = append(filtered, value)
	}
	if !cacheSet {
		filtered = append(filtered, "GOCACHE="+filepath.Join(os.TempDir(), "dev-control-room-go-cache"))
	}
	return append(filtered, "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off")
}

const qualityRunOutputLimit = 16 << 10

func boundedQualityOutput(masker *masking.Masker, value string) string {
	if masker != nil {
		value = masker.Mask(value)
	}
	value = strings.ToValidUTF8(value, "�")
	if len(value) > qualityRunOutputLimit {
		return value[:qualityRunOutputLimit] + "…[truncated]"
	}
	return value
}

func qualityRunResultEvidence(masker *masking.Masker, state string, result environment.Result, processErr error, executed bool) map[string]any {
	evidence := map[string]any{"state": state, "executed": executed}
	if executed {
		evidence["exitCode"] = result.ExitCode
		evidence["stdout"] = boundedQualityOutput(masker, result.Stdout)
		evidence["stderr"] = boundedQualityOutput(masker, result.Stderr)
		if processErr != nil {
			evidence["error"] = "runner execution failed"
		}
	} else {
		evidence["exitCode"] = nil
		evidence["stdout"] = ""
		evidence["stderr"] = ""
	}
	return evidence
}

func qualityRunArtifact(run domain.QualityRun, executed bool) map[string]any {
	report := map[string]any{
		"runId":          run.Metadata.ID,
		"technique":      run.Spec.Technique,
		"runner":         run.Spec.Runner,
		"selectionState": run.Spec.Evidence["selectionState"],
		"configDigest":   run.Spec.ConfigDigest,
		"state":          run.Spec.State,
		"result":         run.Spec.Evidence["result"],
	}
	if metadata, ok := run.Spec.Evidence["runnerMetadata"]; ok {
		report["runnerMetadata"] = metadata
	}
	if unavailable, ok := run.Spec.Evidence["unavailable"]; ok {
		report["unavailable"] = unavailable
	}
	if executed {
		report["command"] = run.Spec.Command
	}
	return report
}

func (a *App) RunAgentInvocation(ctx context.Context, input AgentInvocationInput) (domain.AgentInvocation, error) {
	return a.runAgentInvocation(ctx, input, "", "")
}

// RetryAgentInvocation is an explicit, user-directed new attempt after the
// service marked an invocation interrupted. The original prompt is
// intentionally not persisted, so the caller must provide it again. A retry
// has a deterministic idempotency key and parent link; repeating the request
// returns the existing child instead of launching another provider process.
func (a *App) RetryAgentInvocation(ctx context.Context, invocationID, prompt string) (domain.AgentInvocation, error) {
	invocationID = strings.TrimSpace(invocationID)
	if invocationID == "" {
		return domain.AgentInvocation{}, contract.InvalidInput("interrupted invocation id is required")
	}
	if strings.TrimSpace(prompt) == "" {
		return domain.AgentInvocation{}, contract.InvalidInput("retry prompt is required because the original prompt is not stored")
	}
	normalizedPrompt, err := assurance.ValidateCodexPrompt(prompt)
	if err != nil {
		return domain.AgentInvocation{}, contract.InvalidInput("retry prompt must be one line and at most 2000 UTF-8 bytes")
	}
	releaseRetry, lockErr := a.acquireRetryInvocation(ctx, invocationID)
	if lockErr != nil {
		return domain.AgentInvocation{}, lockErr
	}
	defer releaseRetry()

	var original domain.AgentInvocation
	if err := a.store.GetAssurance(ctx, domain.AgentInvocationKind, invocationID, &original); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.AgentInvocation{}, contract.NotFound("agent invocation not found")
		}
		return domain.AgentInvocation{}, err
	}
	if original.Spec.State != domain.AssuranceStateInterrupted {
		return domain.AgentInvocation{}, contract.Conflict("only an interrupted invocation can be retried")
	}
	retry, providerErr := a.runAgentInvocation(ctx, AgentInvocationInput{
		SessionID: original.Spec.SessionID, Provider: original.Spec.Provider,
		ProfileID: original.Spec.ProfileID, RequestedModel: original.Spec.RequestedModel,
		Prompt: normalizedPrompt,
	}, original.Metadata.ID, "retry:"+original.Metadata.ID)
	if providerErr != nil {
		return retry, providerErr
	}

	if sessionErr := a.reconcileRetrySession(ctx, original, retry); sessionErr != nil {
		// The child invocation is durable even though its Resume Brief could
		// not be reconciled. Return both values so the caller can surface this
		// as a partial retry rather than confusing it with provider execution.
		return retry, sessionErr
	}
	return retry, nil
}

func (a *App) reconcileRetrySession(ctx context.Context, original, retry domain.AgentInvocation) error {
	return reconcileRetrySession(ctx, original, retry,
		func(ctx context.Context, sessionID string) (domain.AssuranceSession, error) {
			var session domain.AssuranceSession
			err := a.store.GetAssurance(ctx, domain.AssuranceSessionKind, sessionID, &session)
			return session, err
		},
		a.updateAssuranceSession,
	)
}

func reconcileRetrySession(
	ctx context.Context,
	original, retry domain.AgentInvocation,
	readSession func(context.Context, string) (domain.AssuranceSession, error),
	updateSession func(context.Context, domain.AssuranceSession) error,
) error {
	session, sessionErr := readSession(ctx, original.Spec.SessionID)
	if sessionErr != nil {
		// Keep the actual read failure. RetryAgentInvocation returns the already
		// persisted child alongside this error to make partial success explicit.
		return sessionErr
	}
	session.Spec.ResumeBrief.Pending = removeAssuranceValue(session.Spec.ResumeBrief.Pending, original.Metadata.ID)
	if retry.Spec.State == domain.AssuranceStateSucceeded {
		session.Spec.State = domain.AssuranceStateReady
		session.Spec.ResumeBrief.NextSafeAction = "새 실행의 구조화된 결과와 제안을 검토합니다."
	} else {
		session.Spec.State = domain.AssuranceStateInterrupted
		session.Spec.ResumeBrief.Pending = appendUniqueStrings(session.Spec.ResumeBrief.Pending, retry.Metadata.ID)
		session.Spec.ResumeBrief.NextSafeAction = "실패한 재시도의 원인과 다음 범위를 검토합니다."
	}
	session.Spec.UpdatedAt = time.Now().UTC()
	if updateErr := updateSession(ctx, session); updateErr != nil {
		// The actual revision/update failure is also part of the partial retry
		// result; do not replace it with the provider execution error.
		return updateErr
	}
	return nil
}

func (a *App) acquireRetryInvocation(ctx context.Context, invocationID string) (func(), error) {
	a.retryLocksMu.Lock()
	if a.retryLocks == nil {
		a.retryLocks = make(map[string]*retryInvocationLock)
	}
	lock := a.retryLocks[invocationID]
	if lock == nil {
		lock = &retryInvocationLock{available: make(chan struct{}, 1)}
		lock.available <- struct{}{}
		a.retryLocks[invocationID] = lock
	}
	lock.references++
	a.retryLocksMu.Unlock()

	select {
	case <-lock.available:
		return func() { a.releaseRetryInvocation(invocationID, lock) }, nil
	case <-ctx.Done():
		a.retryLocksMu.Lock()
		lock.references--
		if lock.references == 0 && a.retryLocks[invocationID] == lock {
			delete(a.retryLocks, invocationID)
		}
		a.retryLocksMu.Unlock()
		return func() {}, ctx.Err()
	}
}

func (a *App) releaseRetryInvocation(invocationID string, lock *retryInvocationLock) {
	lock.available <- struct{}{}
	a.retryLocksMu.Lock()
	lock.references--
	if lock.references == 0 && a.retryLocks[invocationID] == lock {
		delete(a.retryLocks, invocationID)
	}
	a.retryLocksMu.Unlock()
}

func (a *App) runAgentInvocation(ctx context.Context, input AgentInvocationInput, parentID, idempotencyKey string) (domain.AgentInvocation, error) {
	session, err := a.AssuranceSession(ctx, input.SessionID)
	if err != nil {
		return domain.AgentInvocation{}, err
	}
	worktree, err := a.Worktree(ctx, session.Spec.ProjectID, session.Spec.RepositoryID, session.Spec.WorktreeID)
	if err != nil {
		return domain.AgentInvocation{}, err
	}
	provider := strings.TrimSpace(input.Provider)
	if provider == "" {
		return domain.AgentInvocation{}, contract.InvalidInput("provider is required")
	}
	now := time.Now().UTC()
	inputDigest := digestText(strings.TrimSpace(input.Prompt))
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	id := assuranceID("invocation", session.Metadata.ID, provider, now)
	if idempotencyKey != "" {
		id = assuranceID("invocation", session.Metadata.ID, provider, idempotencyKey)
		var existing domain.AgentInvocation
		if err := a.store.GetAssurance(ctx, domain.AgentInvocationKind, id, &existing); err == nil {
			if !sameInvocationRequest(existing, session.Metadata.ID, provider, worktree.Metadata.ID, input.RequestedModel, inputDigest, idempotencyKey) {
				return domain.AgentInvocation{}, contract.Conflict("idempotency key is already bound to a different invocation")
			}
			return existing, nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return domain.AgentInvocation{}, err
		}
	}
	if idempotencyKey == "" {
		idempotencyKey = id
	}
	leaseExpiresAt := now.Add(agentInvocationLease)
	invocation := domain.AgentInvocation{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AgentInvocationKind}, Metadata: domain.ObjectMeta{ID: id, Name: "Agent invocation"}, Spec: domain.AgentInvocationSpec{SessionID: session.Metadata.ID, ParentID: strings.TrimSpace(parentID), ProjectID: session.Spec.ProjectID, RepositoryID: session.Spec.RepositoryID, WorktreeID: worktree.Metadata.ID, Branch: worktree.Spec.Branch, Head: worktree.Spec.Head, Provider: provider, ProfileID: strings.TrimSpace(input.ProfileID), RequestedModel: strings.TrimSpace(input.RequestedModel), SelectionSource: "user", State: domain.AssuranceStateQueued, IdempotencyKey: idempotencyKey, InputDigest: inputDigest, TraceID: assuranceID("trace", id), StartedAt: now, RawTranscript: false}}
	invocation.Spec.LeaseExpiresAt = &leaseExpiresAt
	if invocation.Spec.ProfileID == "" {
		invocation.Spec.ProfileID = provider
	}
	if err := a.store.SaveAgentInvocation(ctx, invocation); err != nil {
		if idempotencyKey != id {
			var existing domain.AgentInvocation
			if readErr := a.store.GetAssurance(ctx, domain.AgentInvocationKind, id, &existing); readErr == nil {
				if !sameInvocationRequest(existing, session.Metadata.ID, provider, worktree.Metadata.ID, input.RequestedModel, inputDigest, idempotencyKey) {
					return domain.AgentInvocation{}, contract.Conflict("idempotency key is already bound to a different invocation")
				}
				return existing, nil
			}
		}
		return domain.AgentInvocation{}, err
	}
	invocation.Spec.State = domain.AssuranceStateRunning
	if err := a.store.UpdateAssuranceRevision(ctx, domain.AgentInvocationKind, id, 2, invocation.Spec.State, now, invocation); err != nil {
		return domain.AgentInvocation{}, err
	}
	scenario := assurance.FakeScenario(strings.TrimSpace(input.Scenario))
	if scenario == "" {
		scenario = assurance.FakeSuccess
	}
	executionRoot, revalidationErr := a.revalidateAssuranceWorktree(ctx, worktree)
	var result assurance.RunResult
	if revalidationErr != nil {
		result = assurance.RunResult{State: domain.AssuranceStateFailed, FailureCode: "worktree.revalidation_failed"}
	} else {
		switch provider {
		case "fake", "claude", "gemini":
			result = (assurance.FakeAdapter{Provider: provider, Scenario: scenario}).Run(ctx, assurance.RunRequest{Provider: provider, Model: input.RequestedModel, Worktree: executionRoot})
		case "codex":
			result = a.runCodexInvocation(ctx, invocation.Spec.ProfileID, input.RequestedModel, executionRoot, input.Prompt)
		default:
			result = assurance.RunResult{State: domain.AssuranceStateFailed, FailureCode: "provider.unknown"}
		}
	}
	completed := time.Now().UTC()
	invocation.Spec.State = result.State
	invocation.Spec.CompletedAt = &completed
	invocation.Spec.LeaseExpiresAt = nil
	if result.Structured != nil {
		masked := a.masker.MaskValue(result.Structured)
		invocation.Spec.Structured, _ = redactInvocationPrompt(masked, input.Prompt).(map[string]any)
	}
	if invocation.Spec.Structured != nil {
		if data, marshalErr := json.Marshal(invocation.Spec.Structured); marshalErr == nil {
			invocation.Spec.OutputDigest = digestText(data)
		}
	}
	invocation.Spec.FailureCode = result.FailureCode
	invocation.Spec.ArtifactIDs = nil
	if result.Usage != nil {
		if value, ok := result.Usage["input"]; ok {
			invocation.Spec.Usage.InputTokens = &value
		}
		if value, ok := result.Usage["output"]; ok {
			invocation.Spec.Usage.OutputTokens = &value
		}
		if total := result.Usage["input"] + result.Usage["output"]; total > 0 {
			invocation.Spec.Usage.TotalTokens = &total
		}
	}
	if result.Summary != "" {
		invocation.Spec.Structured = map[string]any{"summary": redactInvocationPrompt(a.masker.Mask(result.Summary), input.Prompt), "result": invocation.Spec.Structured}
	}
	if invocation.Spec.Structured != nil {
		if data, marshalErr := json.Marshal(invocation.Spec.Structured); marshalErr == nil {
			invocation.Spec.OutputDigest = digestText(data)
		}
	}
	artifactPersisted := false
	report, marshalErr := json.Marshal(map[string]any{"provider": provider, "state": result.State, "failureCode": result.FailureCode, "structured": invocation.Spec.Structured, "rawTranscript": false})
	if marshalErr == nil {
		artifact, artifactErr := a.SaveAssuranceArtifact(ctx, ArtifactInput{SourceType: "agent_invocation", SourceID: id, Name: id + ".json", MIME: "application/json", Content: report, TraceID: invocation.Spec.TraceID})
		if artifactErr == nil {
			invocation.Spec.ArtifactIDs = []string{artifact.Metadata.ID}
			artifactPersisted = true
		}
	}
	if !artifactPersisted {
		// A completed provider result without its durable evidence must never be
		// presented as a completed invocation. Keep the failure code generic so
		// filesystem/store details cannot cross the presentation boundary.
		invocation.Spec.State = domain.AssuranceStateFailed
		invocation.Spec.FailureCode = "artifact.persistence_failed"
		invocation.Spec.Structured = nil
		invocation.Spec.ArtifactIDs = nil
		result.State = domain.AssuranceStateFailed
		result.FailureCode = invocation.Spec.FailureCode
	}
	if err := a.store.UpdateAssuranceRevision(ctx, domain.AgentInvocationKind, id, 3, invocation.Spec.State, completed, invocation); err != nil {
		return domain.AgentInvocation{}, err
	}
	session.Spec.UpdatedAt = completed
	session.Spec.State = domain.AssuranceStateReady
	session.Spec.ResumeBrief.Completed = append(session.Spec.ResumeBrief.Completed, id)
	session.Spec.ResumeBrief.NextSafeAction = "구조화된 결과와 제안을 검토합니다."
	if result.State != domain.AssuranceStateSucceeded {
		session.Spec.ResumeBrief.FailedEvidence = append(session.Spec.ResumeBrief.FailedEvidence, result.FailureCode)
		session.Spec.ResumeBrief.NextSafeAction = "실패 원인과 재시도 범위를 검토합니다."
	}
	_ = a.updateAssuranceSession(ctx, session)
	if !artifactPersisted {
		return invocation, contract.CodedError{Code: contract.ErrorExecutionFailed, Message: "agent invocation evidence could not be persisted"}
	}
	return invocation, nil
}

func sameInvocationRequest(item domain.AgentInvocation, sessionID, provider, worktreeID, model, inputDigest, idempotencyKey string) bool {
	return item.Spec.SessionID == sessionID && item.Spec.Provider == provider &&
		item.Spec.WorktreeID == worktreeID && item.Spec.RequestedModel == strings.TrimSpace(model) &&
		item.Spec.InputDigest == inputDigest && item.Spec.IdempotencyKey == idempotencyKey
}

func removeAssuranceValue(values []string, target string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

// recoverInterruptedInvocations makes a restart an explicit boundary. The
// previous process is not assumed to be alive or safe to resume, and no
// provider is relaunched automatically. An active invocation becomes an
// interrupted record with a durable Resume Brief for a later user-directed
// retry.
func (a *App) recoverInterruptedInvocations(ctx context.Context) error {
	invocations, err := a.store.ListAgentInvocations(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, invocation := range invocations {
		if !recoverableInvocationState(invocation.Spec.State) {
			continue
		}
		invocation.Spec.State = domain.AssuranceStateInterrupted
		invocation.Spec.FailureCode = "provider.interrupted"
		invocation.Spec.LeaseExpiresAt = nil
		revision, err := a.store.AssuranceRevision(ctx, domain.AgentInvocationKind, invocation.Metadata.ID)
		if err != nil {
			return err
		}
		if err := a.store.UpdateAssuranceRevision(ctx, domain.AgentInvocationKind, invocation.Metadata.ID, revision+1, invocation.Spec.State, now, invocation); err != nil {
			var current domain.AgentInvocation
			if readErr := a.store.GetAssurance(ctx, domain.AgentInvocationKind, invocation.Metadata.ID, &current); readErr != nil {
				return err
			}
			if current.Spec.State != domain.AssuranceStateInterrupted || current.Spec.FailureCode != "provider.interrupted" {
				return err
			}
		}

		var session domain.AssuranceSession
		if err := a.store.GetAssurance(ctx, domain.AssuranceSessionKind, invocation.Spec.SessionID, &session); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return err
		}
		session.Spec.State = domain.AssuranceStateInterrupted
		session.Spec.UpdatedAt = now
		session.Spec.ResumeBrief.CurrentHead = invocation.Spec.Head
		session.Spec.ResumeBrief.Pending = appendUniqueStrings(session.Spec.ResumeBrief.Pending, invocation.Metadata.ID)
		session.Spec.ResumeBrief.FailedEvidence = appendUniqueStrings(session.Spec.ResumeBrief.FailedEvidence, invocation.Spec.FailureCode)
		session.Spec.ResumeBrief.NextSafeAction = "중단된 실행의 상태와 재시도 범위를 검토합니다."
		if err := a.updateAssuranceSession(ctx, session); err != nil {
			var current domain.AssuranceSession
			if readErr := a.store.GetAssurance(ctx, domain.AssuranceSessionKind, session.Metadata.ID, &current); readErr != nil {
				return err
			}
			if current.Spec.State != domain.AssuranceStateInterrupted || !containsText(current.Spec.ResumeBrief.Pending, invocation.Metadata.ID) {
				return err
			}
		}
	}
	return nil
}

func recoverableInvocationState(state string) bool {
	return state == domain.AssuranceStateQueued || state == domain.AssuranceStateRunning || state == domain.AssuranceStateCancelling
}

// revalidateAssuranceWorktree replays the persisted Git association proof at
// the last possible point before an assurance runner receives a directory.
// It deliberately returns the freshly collected canonical path, never the
// previously persisted path, so a changed association cannot be spawned.
func (a *App) revalidateAssuranceWorktree(ctx context.Context, stored domain.Worktree) (string, error) {
	if stored.Metadata.ID == "" || stored.Spec.ProjectID == "" || stored.Spec.RepositoryID == "" ||
		stored.Spec.Trust != domain.WorktreeTrustVerifiedReadOnly || stored.Spec.TombstonedAt != nil ||
		stored.Spec.Prunable || stored.Spec.CanonicalPath == "" || stored.Spec.PathFingerprint == "" ||
		stored.Spec.AssociationFingerprint == "" || stored.Spec.Head == "" {
		return "", errors.New("stored assurance Worktree is not executable")
	}
	current, changed, err := a.discoveryWorktree(ctx, stored.Spec.ProjectID, stored.Spec.RepositoryID, stored.Metadata.ID)
	if err != nil || changed {
		return "", errors.New("assurance Worktree revalidation failed")
	}
	if current.ID != stored.Metadata.ID || current.Path == "" || current.Path != stored.Spec.CanonicalPath ||
		current.Head != stored.Spec.Head || current.Trust != string(domain.WorktreeTrustVerifiedReadOnly) ||
		current.Prunable || current.AssociationFingerprint != stored.Spec.AssociationFingerprint ||
		worktreePathFingerprint(current.Path) != stored.Spec.PathFingerprint {
		return "", errors.New("assurance Worktree identity changed")
	}
	return current.Path, nil
}

func (a *App) runCodexInvocation(ctx context.Context, profileID, model, worktree, prompt string) assurance.RunResult {
	if strings.TrimSpace(prompt) == "" {
		return assurance.RunResult{State: domain.AssuranceStateFailed, FailureCode: "provider.prompt_required"}
	}
	if _, err := assurance.ValidateCodexPrompt(prompt); err != nil {
		return assurance.RunResult{State: domain.AssuranceStateFailed, FailureCode: "provider.prompt_invalid"}
	}
	profile, err := a.AgentProfile(ctx, profileID)
	if err != nil {
		return assurance.RunResult{State: domain.AssuranceStateFailed, FailureCode: "provider.profile_required"}
	}
	if err := trustedCodexProfile(profile); err != nil {
		return assurance.RunResult{State: domain.AssuranceStateFailed, FailureCode: "provider.profile_untrusted"}
	}
	execution := assurance.CodexExecutionFromContext(ctx)
	status := execution.Resolver()
	if status.Provider == "" {
		status.Provider = "codex"
	}
	schemaPath, err := assurance.WriteCodexOutputSchema(a.home)
	if err != nil {
		return assurance.RunResult{State: domain.AssuranceStateFailed, FailureCode: "provider.schema_unavailable"}
	}
	if err := assurance.VerifyCodexOutputSchema(schemaPath); err != nil {
		return assurance.RunResult{State: domain.AssuranceStateFailed, FailureCode: "provider.schema_unavailable"}
	}
	command, err := assurance.BuildCodexInvocationCommand(status, model, assurance.CodexInvocationOptions{Worktree: worktree, SchemaPath: schemaPath, Prompt: prompt})
	if err != nil {
		failureCode := status.ReasonCode
		if failureCode == "" {
			failureCode = "provider.invalid_command"
		}
		return assurance.RunResult{State: domain.AssuranceStateFailed, FailureCode: failureCode}
	}
	timeout := time.Duration(profile.Spec.TimeoutSeconds) * time.Second
	return assurance.CodexExecutionFromContext(ctx).Runner(ctx, assurance.RunRequest{Provider: "codex", Model: strings.TrimSpace(model), Command: command, Worktree: worktree, Timeout: timeout}, a.masker)
}

func redactInvocationPrompt(value any, prompt string) any {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return value
	}
	switch typed := value.(type) {
	case string:
		return strings.ReplaceAll(typed, prompt, masking.Replacement)
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for key, item := range typed {
			redacted[strings.ReplaceAll(key, prompt, masking.Replacement)] = redactInvocationPrompt(item, prompt)
		}
		return redacted
	case []any:
		redacted := make([]any, len(typed))
		for index, item := range typed {
			redacted[index] = redactInvocationPrompt(item, prompt)
		}
		return redacted
	default:
		return value
	}
}

func trustedCodexProfile(profile domain.AgentProfile) error {
	if profile.Metadata.ID != "codex" || profile.Spec.LaunchMode != domain.AgentLaunchDirect || !strings.EqualFold(strings.TrimSpace(profile.Spec.Command), "codex") {
		return errors.New("Codex requires the reviewed codex direct profile")
	}
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("Codex profile is invalid: %w", err)
	}
	return nil
}

func (a *App) SaveAssuranceArtifact(ctx context.Context, input ArtifactInput) (domain.Artifact, error) {
	if strings.TrimSpace(input.SourceType) == "" || strings.TrimSpace(input.SourceID) == "" || len(input.Content) == 0 {
		return domain.Artifact{}, contract.InvalidInput("artifact source and content are required")
	}
	masked := []byte(a.masker.Mask(string(input.Content)))
	if storage, err := a.AssuranceArtifactStorage(ctx); err == nil && storage.UsedBytes+int64(len(masked)) > storage.QuotaBytes {
		return domain.Artifact{}, contract.Conflict("artifact storage quota exceeded; pin or export evidence before retrying")
	}
	sum := sha256.Sum256(masked)
	id := assuranceID("artifact", input.SourceType, input.SourceID, input.Name, time.Now().UTC())
	directory := filepath.Join(a.home, "artifacts", "assurance")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return domain.Artifact{}, err
	}
	name := filepath.Base(strings.TrimSpace(input.Name))
	if name == "." || name == "" {
		name = id + ".dat"
	}
	path := filepath.Join(directory, id+"-"+name)
	temporary, err := os.CreateTemp(directory, ".artifact-write-*")
	if err != nil {
		return domain.Artifact{}, err
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return domain.Artifact{}, err
	}
	if _, err := temporary.Write(masked); err != nil {
		_ = temporary.Close()
		return domain.Artifact{}, err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return domain.Artifact{}, err
	}
	if err := temporary.Close(); err != nil {
		return domain.Artifact{}, err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return domain.Artifact{}, err
	}
	committed = true
	now := time.Now().UTC()
	item := domain.Artifact{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ArtifactKind}, Metadata: domain.ObjectMeta{ID: id, Name: name}, Spec: domain.ArtifactSpec{ManifestVersion: "devroom/artifact/v1", StorageKey: id + "/" + name, SourceType: input.SourceType, SourceID: input.SourceID, Path: path, MIME: input.MIME, Size: int64(len(masked)), SHA256: hex.EncodeToString(sum[:]), Retention: domain.ArtifactRetentionActive, CreatedAt: now, SourceRef: input.SourceID, TraceID: strings.TrimSpace(input.TraceID), MaskingPolicyDigest: digestText("masking-v1"), RedactionState: "masked"}}
	if err := a.store.SaveArtifact(ctx, item); err != nil {
		_ = os.Remove(path)
		return domain.Artifact{}, err
	}
	return item, nil
}

func (a *App) CreateEffect(ctx context.Context, input EffectInput) (domain.Effect, error) {
	now := time.Now().UTC()
	fingerprint := strings.TrimSpace(input.Fingerprint)
	if fingerprint == "" {
		fingerprint = digestText(input.SourceRunID, input.Kind, strings.Join(input.EvidenceIDs, ","))
	}
	traceIDs := append([]string(nil), input.TraceIDs...)
	if strings.TrimSpace(input.SourceRunID) != "" {
		traceIDs = appendUniqueStrings(traceIDs, input.SourceRunID)
	}
	if strings.TrimSpace(input.SourceFindingID) != "" {
		traceIDs = appendUniqueStrings(traceIDs, input.SourceFindingID)
	}
	if strings.TrimSpace(input.ReverificationRunID) != "" {
		traceIDs = appendUniqueStrings(traceIDs, input.ReverificationRunID)
	}
	for _, evidenceID := range input.EvidenceIDs {
		traceIDs = appendUniqueStrings(traceIDs, evidenceID)
	}
	valueKnown := input.ValueKnown || input.Value != 0
	item := domain.Effect{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.EffectKind}, Metadata: domain.ObjectMeta{ID: assuranceID("effect", fingerprint), Name: "Assurance effect"}, Spec: domain.EffectSpec{Fingerprint: fingerprint, ProjectID: input.ProjectID, RepositoryID: input.RepositoryID, WorktreeID: input.WorktreeID, Kind: input.Kind, MetricKey: input.MetricKey, BaselineID: input.BaselineID, SourceRunID: input.SourceRunID, SourceFindingID: input.SourceFindingID, EvidenceIDs: append([]string(nil), input.EvidenceIDs...), TraceIDs: traceIDs, TraceID: strings.TrimSpace(input.TraceID), Adopted: input.Adopted, Reverified: input.Reverified, AdoptedAt: input.AdoptedAt, ReverifiedAt: input.ReverifiedAt, AdoptedCommit: strings.TrimSpace(input.AdoptedCommit), ReverificationRunID: strings.TrimSpace(input.ReverificationRunID), ReverifiedCommit: strings.TrimSpace(input.ReverifiedCommit), Label: input.Label, Value: input.Value, ValueKnown: valueKnown, Unit: input.Unit, BaselineValue: input.BaselineValue, BaselineUnit: input.BaselineUnit, PeriodStart: input.PeriodStart, PeriodEnd: input.PeriodEnd, Outcome: input.Outcome, RecordedBy: strings.TrimSpace(input.RecordedBy), Reason: strings.TrimSpace(input.Reason), Note: input.Note, CreatedAt: now, UpdatedAt: now}}
	if item.Spec.TraceID == "" {
		item.Spec.TraceID = assuranceID("trace", fingerprint)
	}
	if input.Adopted {
		if item.Spec.AdoptedAt == nil {
			item.Spec.AdoptedAt = &now
		}
	}
	if input.Reverified {
		if item.Spec.ReverifiedAt == nil {
			item.Spec.ReverifiedAt = &now
		}
	}
	if err := a.store.SaveEffect(ctx, item); err != nil {
		return domain.Effect{}, err
	}
	return item, nil
}

func appendUniqueStrings(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func containsText(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func (a *App) SavePricingSnapshot(ctx context.Context, item domain.ProviderPricingSnapshot) (domain.ProviderPricingSnapshot, error) {
	if item.Metadata.ID == "" {
		item.Metadata.ID = assuranceID("pricing", item.Spec.Provider, item.Spec.Model, item.Spec.EffectiveAt)
	}
	if item.TypeMeta.Kind == "" {
		item.TypeMeta = domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ProviderPricingSnapshotKind}
	}
	if err := a.store.SavePricingSnapshot(ctx, item); err != nil {
		return domain.ProviderPricingSnapshot{}, err
	}
	return item, nil
}

func (a *App) ExportAssuranceArtifacts(ctx context.Context, ids []string, destination string) (ArtifactExportResult, error) {
	if strings.TrimSpace(destination) == "" || !filepath.IsAbs(destination) {
		return ArtifactExportResult{}, contract.InvalidInput("artifact export destination must be an absolute local path")
	}
	destination = filepath.Clean(destination)
	artifacts, err := a.AssuranceArtifacts(ctx)
	if err != nil {
		return ArtifactExportResult{}, err
	}
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	if len(wanted) == 0 {
		return ArtifactExportResult{}, contract.InvalidInput("artifact export requires at least one artifact")
	}
	staging := destination + ".staging"
	if _, err := os.Stat(destination); err == nil {
		return ArtifactExportResult{}, contract.Conflict("artifact export destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return ArtifactExportResult{}, err
	}
	if _, err := os.Stat(staging); err == nil {
		return ArtifactExportResult{}, contract.Conflict("artifact export staging destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return ArtifactExportResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return ArtifactExportResult{}, err
	}
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return ArtifactExportResult{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(staging)
		}
	}()
	result := ArtifactExportResult{Destination: destination, ArtifactIDs: []string{}}
	selected := make([]domain.Artifact, 0, len(wanted))
	manifest := AssuranceArchiveManifest{Schema: "devroom/assurance-archive/v1", CreatedAt: time.Now().UTC(), ArtifactID: []AssuranceArchiveManifestItem{}}
	for _, item := range artifacts {
		if !wanted[item.Metadata.ID] {
			continue
		}
		data, err := readRegularFile(item.Spec.Path)
		if err != nil {
			return ArtifactExportResult{}, err
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != item.Spec.SHA256 {
			return ArtifactExportResult{}, errors.New("artifact hash verification failed")
		}
		filename := filepath.Base(item.Spec.Path)
		if filename == "." || filename == "" || filename == ".." || strings.ContainsAny(filename, `/\\`) {
			return ArtifactExportResult{}, errors.New("artifact filename is invalid")
		}
		if err := os.WriteFile(filepath.Join(staging, filename), data, 0o600); err != nil {
			return ArtifactExportResult{}, err
		}
		result.ArtifactIDs = append(result.ArtifactIDs, item.Metadata.ID)
		selected = append(selected, item)
		manifest.ArtifactID = append(manifest.ArtifactID, AssuranceArchiveManifestItem{ArtifactID: item.Metadata.ID, Filename: filename, SHA256: item.Spec.SHA256, Size: item.Spec.Size, MIME: item.Spec.MIME, SourceType: item.Spec.SourceType, SourceID: item.Spec.SourceID})
	}
	if len(result.ArtifactIDs) != len(wanted) {
		return ArtifactExportResult{}, contract.NotFound("one or more artifacts were not found")
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return ArtifactExportResult{}, err
	}
	manifestData = append(manifestData, '\n')
	if err := os.WriteFile(filepath.Join(staging, archiveManifestName), manifestData, 0o600); err != nil {
		return ArtifactExportResult{}, err
	}
	if err := os.Rename(staging, destination); err != nil {
		return ArtifactExportResult{}, err
	}
	committed = true
	result.Manifest = archiveManifestName
	result.ManifestSHA = artifactHash(manifestData)
	archiveID := assuranceID("archive", destination, result.ManifestSHA)
	for _, item := range selected {
		now := time.Now().UTC()
		if item.Spec.Retention != domain.ArtifactRetentionPinned {
			item.Spec.Retention = domain.ArtifactRetentionArchived
		}
		item.Spec.ArchivedAt = &now
		item.Spec.ArchivePath = destination
		item.Spec.ArchiveManifest = archiveManifestName
		item.Spec.ArchiveSHA256 = result.ManifestSHA
		item.Spec.ArchiveID = archiveID
		item.Spec.ArchiveVerifiedAt = &now
		if err := a.store.UpdateAssuranceArtifact(ctx, item); err != nil {
			return ArtifactExportResult{}, err
		}
	}
	result.Verified = true
	return result, nil
}

func (a *App) DeleteAssuranceArtifact(ctx context.Context, id, confirmation string) (domain.Artifact, error) {
	if confirmation != "DELETE" {
		return domain.Artifact{}, contract.InvalidInput("artifact deletion requires confirmation DELETE")
	}
	items, err := a.AssuranceArtifacts(ctx)
	if err != nil {
		return domain.Artifact{}, err
	}
	for _, item := range items {
		if item.Metadata.ID != id {
			continue
		}
		if item.Spec.Retention == domain.ArtifactRetentionDeleted {
			return item, nil
		}
		if item.Spec.Retention == domain.ArtifactRetentionPinned {
			return domain.Artifact{}, contract.Conflict("pinned artifact must be unpinned before deletion")
		}
		if err := os.Remove(item.Spec.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return domain.Artifact{}, err
		}
		now := time.Now().UTC()
		item.Spec.Retention = domain.ArtifactRetentionDeleted
		item.Spec.DeletedAt = &now
		if err := a.store.UpdateAssuranceArtifact(ctx, item); err != nil {
			return domain.Artifact{}, err
		}
		return item, nil
	}
	return domain.Artifact{}, contract.NotFound("assurance artifact not found")
}

func discoverBaseline(root string) ([]domain.BaselineEntry, string, []string, error) {
	if root == "" {
		return nil, "", nil, errors.New("root is empty")
	}
	files := []string{}
	if _, err := os.Stat(filepath.Join(root, "package.json")); err == nil {
		files = append(files, filepath.Join(root, "package.json"))
	}
	workflowRoot := filepath.Join(root, ".github", "workflows")
	for _, pattern := range []string{"*.yml", "*.yaml"} {
		matches, err := filepath.Glob(filepath.Join(workflowRoot, pattern))
		if err != nil {
			return nil, "", nil, err
		}
		files = append(files, matches...)
	}
	sort.Strings(files)
	hash := sha256.New()
	entries := []domain.BaselineEntry{}
	requiredPath := filepath.Join(root, ".github", "required-checks.txt")
	if data, err := os.ReadFile(requiredPath); err == nil {
		_, _ = hash.Write(data)
		for _, line := range strings.Split(string(data), "\n") {
			name := strings.TrimSpace(line)
			if name != "" {
				entries = append(entries, domain.BaselineEntry{ID: "required-" + normalizeAppID(name), Name: name, Classification: domain.BaselineRequired, SourcePath: filepath.ToSlash(filepath.Join(".github", "required-checks.txt")), Required: true})
			}
		}
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", nil, err
		}
		_, _ = hash.Write([]byte(filepath.ToSlash(strings.TrimPrefix(path, root))))
		_, _ = hash.Write(data)
		rel, _ := filepath.Rel(root, path)
		if filepath.Base(path) == "package.json" {
			var manifest struct {
				Scripts map[string]string `json:"scripts"`
			}
			if json.Unmarshal(data, &manifest) == nil {
				names := make([]string, 0, len(manifest.Scripts))
				for name := range manifest.Scripts {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					command := manifest.Scripts[name]
					entries = append(entries, domain.BaselineEntry{ID: "package-" + name, Name: "package script " + name, Classification: domain.BaselineLocalEquivalent, SourcePath: rel, Command: command, Observed: true})
				}
			}
		}
	}
	workflowCandidates, err := discovery.Discover(root)
	if err != nil {
		return nil, "", nil, err
	}
	for _, candidate := range workflowCandidates {
		if candidate.CommandKind != "github_actions_run" {
			continue
		}
		entries = append(entries, domain.BaselineEntry{ID: normalizeAppID("workflow-" + digestText(candidate.SourcePath, candidate.Command)[7:23]), Name: "GitHub Actions run", Classification: domain.BaselineObserved, SourcePath: filepath.FromSlash(candidate.SourcePath), Command: candidate.Command, Observed: true})
	}
	sum := hash.Sum(nil)
	return entries, "sha256:" + hex.EncodeToString(sum), files, nil
}

func validTechnique(value string) bool {
	switch value {
	case domain.QualityTechniqueStaticSecurity, domain.QualityTechniqueMutation, domain.QualityTechniqueProperty, domain.QualityTechniqueFuzz, domain.QualityTechniqueTargetedE2E:
		return true
	}
	return false
}
func isActiveAssuranceState(value string) bool {
	return value != domain.AssuranceStateSucceeded && value != domain.AssuranceStateFailed && value != domain.AssuranceStateCancelled && value != domain.AssuranceStateExpired && value != domain.AssuranceStateStale
}
func assuranceID(prefix string, values ...any) string {
	return normalizeAppID(prefix + "-" + digestText(values...)[7:23])
}
func digestText(values ...any) string {
	data, _ := json.Marshal(values)
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func mustAssuranceSessions(items []domain.AssuranceSession, err error) []domain.AssuranceSession {
	if err != nil {
		return nil
	}
	return items
}

func (a *App) updateAssuranceSession(ctx context.Context, session domain.AssuranceSession) error {
	revision, err := a.store.AssuranceRevision(ctx, domain.AssuranceSessionKind, session.Metadata.ID)
	if err != nil {
		return err
	}
	return a.store.UpdateAssuranceRevision(ctx, domain.AssuranceSessionKind, session.Metadata.ID, revision+1, session.Spec.State, session.Spec.UpdatedAt, session)
}
