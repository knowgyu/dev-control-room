package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/assurance"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/environment"
)

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
	return AssuranceDashboard{GeneratedAt: time.Now().UTC(), ProviderFilter: providerFilter, ModelFilter: modelFilter, Effects: effects, Invocations: filtered, TotalTokens: total, UsageComplete: complete, EstimatedCost: estimated, CostLabel: "estimated public API list-price equivalent", CostState: costState}, nil
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
	now := time.Now().UTC()
	run := domain.QualityRun{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.QualityRunKind}, Metadata: domain.ObjectMeta{ID: assuranceID("run", campaign.Metadata.ID, input.Technique, now.String()), Name: "Quality Run"}, Spec: domain.QualityRunSpec{CampaignID: campaign.Metadata.ID, ProjectID: campaign.Spec.ProjectID, RepositoryID: campaign.Spec.RepositoryID, WorktreeID: campaign.Spec.WorktreeID, Head: worktree.Spec.Head, Technique: input.Technique, Runner: "typed-git-diff-check", Command: domain.CheckCommand{Executable: "git", Arguments: []string{"diff", "--check"}, TimeoutSeconds: 60}, ConfigDigest: digestText(input.Technique, input.Provider, input.Model), State: domain.AssuranceStateQueued, StartedAt: now, Evidence: map[string]any{"provider": input.Provider, "model": input.Model}}}
	if err := a.store.SaveQualityRun(ctx, run); err != nil {
		return domain.QualityRun{}, err
	}
	run.Spec.State = domain.AssuranceStateRunning
	if err := a.store.UpdateAssuranceRevision(ctx, domain.QualityRunKind, run.Metadata.ID, 2, run.Spec.State, now, run); err != nil {
		return domain.QualityRun{}, err
	}
	techniqueReport, techniqueErr := assurance.RunFixtureTechnique(ctx, input.Technique, worktree.Spec.CanonicalPath)
	if techniqueErr != nil {
		run.Spec.State = domain.AssuranceStateFailed
		run.Spec.Summary = "quality technique adapter가 실행되지 않았습니다."
		run.Spec.StaleReason = "technique adapter failed"
	}
	result, processErr := (environment.ProcessRunner{OutputLimit: 128 << 10}).RunInDirectory(ctx, "git", []string{"diff", "--check"}, environment.AllowlistedEnvironment(nil), worktree.Spec.CanonicalPath, time.Minute)
	completed := time.Now().UTC()
	run.Spec.CompletedAt = &completed
	run.Spec.ExitCode = result.ExitCode
	run.Spec.State = domain.AssuranceStateSucceeded
	run.Spec.Summary = "정적 diff 점검이 완료되었습니다."
	run.Spec.Evidence["techniqueReport"] = techniqueReport
	if techniqueErr != nil {
		run.Spec.State = domain.AssuranceStateFailed
		run.Spec.Summary = "quality technique adapter가 실행되지 않았습니다."
	}
	if processErr != nil {
		run.Spec.State = domain.AssuranceStateFailed
		run.Spec.Summary = "정적 diff 점검이 실패했습니다."
		run.Spec.StaleReason = "typed runner failed"
	}
	report, _ := json.Marshal(map[string]any{"runId": run.Metadata.ID, "technique": input.Technique, "state": run.Spec.State, "exitCode": run.Spec.ExitCode, "stderr": a.masker.Mask(result.Stderr), "techniqueReport": techniqueReport})
	artifact, artifactErr := a.SaveAssuranceArtifact(ctx, ArtifactInput{SourceType: "quality_run", SourceID: run.Metadata.ID, Name: run.Metadata.ID + ".json", MIME: "application/json", Content: report})
	if artifactErr == nil {
		run.Spec.ArtifactIDs = []string{artifact.Metadata.ID}
	}
	if err := a.store.UpdateAssuranceRevision(ctx, domain.QualityRunKind, run.Metadata.ID, 3, run.Spec.State, completed, run); err != nil {
		return domain.QualityRun{}, err
	}
	if processErr != nil {
		return run, contract.CodedError{Code: contract.ErrorExecutionFailed, Message: "quality run did not succeed"}
	}
	return run, nil
}

func (a *App) RunAgentInvocation(ctx context.Context, input AgentInvocationInput) (domain.AgentInvocation, error) {
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
	id := assuranceID("invocation", session.Metadata.ID, provider, now)
	invocation := domain.AgentInvocation{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AgentInvocationKind}, Metadata: domain.ObjectMeta{ID: id, Name: "Agent invocation"}, Spec: domain.AgentInvocationSpec{SessionID: session.Metadata.ID, WorktreeID: worktree.Metadata.ID, Head: worktree.Spec.Head, Provider: provider, ProfileID: strings.TrimSpace(input.ProfileID), RequestedModel: strings.TrimSpace(input.RequestedModel), SelectionSource: "user", State: domain.AssuranceStateQueued, IdempotencyKey: id, StartedAt: now, RawTranscript: false}}
	if invocation.Spec.ProfileID == "" {
		invocation.Spec.ProfileID = provider
	}
	if err := a.store.SaveAgentInvocation(ctx, invocation); err != nil {
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
	var result assurance.RunResult
	switch provider {
	case "fake", "claude", "gemini":
		result = (assurance.FakeAdapter{Provider: provider, Scenario: scenario}).Run(ctx, assurance.RunRequest{Provider: provider, Model: input.RequestedModel, Worktree: worktree.Spec.CanonicalPath})
	case "codex":
		result = assurance.RunResult{State: domain.AssuranceStateFailed, FailureCode: "provider.native_acceptance_required"}
	default:
		result = assurance.RunResult{State: domain.AssuranceStateFailed, FailureCode: "provider.unknown"}
	}
	completed := time.Now().UTC()
	invocation.Spec.State = result.State
	invocation.Spec.CompletedAt = &completed
	invocation.Spec.Structured = result.Structured
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
		invocation.Spec.Structured = map[string]any{"summary": result.Summary, "result": result.Structured}
	}
	if report, marshalErr := json.Marshal(map[string]any{"provider": provider, "state": result.State, "failureCode": result.FailureCode, "structured": result.Structured, "rawTranscript": false}); marshalErr == nil {
		if artifact, artifactErr := a.SaveAssuranceArtifact(ctx, ArtifactInput{SourceType: "agent_invocation", SourceID: id, Name: id + ".json", MIME: "application/json", Content: report}); artifactErr == nil {
			invocation.Spec.ArtifactIDs = []string{artifact.Metadata.ID}
		}
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
	return invocation, nil
}

func (a *App) SaveAssuranceArtifact(ctx context.Context, input ArtifactInput) (domain.Artifact, error) {
	if strings.TrimSpace(input.SourceType) == "" || strings.TrimSpace(input.SourceID) == "" || len(input.Content) == 0 {
		return domain.Artifact{}, contract.InvalidInput("artifact source and content are required")
	}
	masked := []byte(a.masker.Mask(string(input.Content)))
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
	if err := os.WriteFile(path, masked, 0o600); err != nil {
		return domain.Artifact{}, err
	}
	now := time.Now().UTC()
	item := domain.Artifact{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ArtifactKind}, Metadata: domain.ObjectMeta{ID: id, Name: name}, Spec: domain.ArtifactSpec{SourceType: input.SourceType, SourceID: input.SourceID, Path: path, MIME: input.MIME, Size: int64(len(masked)), SHA256: hex.EncodeToString(sum[:]), Retention: domain.ArtifactRetentionActive, CreatedAt: now, SourceRef: input.SourceID}}
	if err := a.store.SaveArtifact(ctx, item); err != nil {
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
	item := domain.Effect{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.EffectKind}, Metadata: domain.ObjectMeta{ID: assuranceID("effect", fingerprint), Name: "Assurance effect"}, Spec: domain.EffectSpec{Fingerprint: fingerprint, ProjectID: input.ProjectID, RepositoryID: input.RepositoryID, WorktreeID: input.WorktreeID, Kind: input.Kind, SourceRunID: input.SourceRunID, EvidenceIDs: append([]string(nil), input.EvidenceIDs...), Adopted: input.Adopted, Reverified: input.Reverified, Label: input.Label, Value: input.Value, Unit: input.Unit, CreatedAt: now, UpdatedAt: now}}
	if err := a.store.SaveEffect(ctx, item); err != nil {
		return domain.Effect{}, err
	}
	return item, nil
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
	if filepath.IsAbs(destination) == false || strings.TrimSpace(destination) == "" {
		return ArtifactExportResult{}, contract.InvalidInput("artifact export destination must be an absolute local path")
	}
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
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return ArtifactExportResult{}, err
	}
	result := ArtifactExportResult{Destination: destination, ArtifactIDs: []string{}}
	for _, item := range artifacts {
		if !wanted[item.Metadata.ID] {
			continue
		}
		data, err := os.ReadFile(item.Spec.Path)
		if err != nil {
			return ArtifactExportResult{}, err
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != item.Spec.SHA256 {
			return ArtifactExportResult{}, errors.New("artifact hash verification failed")
		}
		if err := os.WriteFile(filepath.Join(staging, filepath.Base(item.Spec.Path)), data, 0o600); err != nil {
			return ArtifactExportResult{}, err
		}
		result.ArtifactIDs = append(result.ArtifactIDs, item.Metadata.ID)
	}
	if len(result.ArtifactIDs) != len(wanted) {
		return ArtifactExportResult{}, contract.NotFound("one or more artifacts were not found")
	}
	if err := os.Rename(staging, destination); err != nil {
		return ArtifactExportResult{}, err
	}
	for _, item := range artifacts {
		if !wanted[item.Metadata.ID] {
			continue
		}
		now := time.Now().UTC()
		item.Spec.Retention = domain.ArtifactRetentionArchived
		item.Spec.ArchivedAt = &now
		item.Spec.ArchivePath = destination
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
	_ = filepath.WalkDir(workflowRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".yml") || strings.EqualFold(filepath.Ext(path), ".yaml") {
			files = append(files, path)
		}
		return nil
	})
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
		text := string(data)
		if filepath.Base(path) == "package.json" {
			var manifest struct {
				Scripts map[string]string `json:"scripts"`
			}
			if json.Unmarshal(data, &manifest) == nil {
				for name, command := range manifest.Scripts {
					entries = append(entries, domain.BaselineEntry{ID: "package-" + name, Name: "package script " + name, Classification: domain.BaselineLocalEquivalent, SourcePath: rel, Command: command, Observed: true})
				}
			}
		}
		for index, line := range strings.Split(text, "\n") {
			trimmed := strings.TrimSpace(line)
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			if strings.HasPrefix(trimmed, "run:") {
				command := strings.TrimSpace(strings.TrimPrefix(trimmed, "run:"))
				entries = append(entries, domain.BaselineEntry{ID: fmt.Sprintf("workflow-%d", index), Name: "GitHub Actions run", Classification: domain.BaselineObserved, SourcePath: rel, Command: command, Observed: true})
			}
		}
	}
	if len(entries) == 0 {
		entries = append(entries, domain.BaselineEntry{ID: "unknown", Name: "PR check baseline", Classification: domain.BaselineUnknown})
	} else {
		// Provider-enforced rules and current check history are not inferred from
		// local files. Keep that unresolved part explicit instead of presenting a
		// local equivalent as the required PR contract.
		entries = append(entries, domain.BaselineEntry{ID: "provider-rules-unknown", Name: "provider-enforced PR rules", Classification: domain.BaselineUnknown})
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
