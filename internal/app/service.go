package app

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/knowgyu/dev-control-room/internal/action"
	"github.com/knowgyu/dev-control-room/internal/assurance"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/environment"
	"github.com/knowgyu/dev-control-room/internal/scheduler"
)

// QueryService and CommandService are the application boundary. CLI and HTTP
// adapters depend on these interfaces instead of reaching into configuration,
// collectors, or storage directly.
type QueryService interface {
	Health(context.Context) Health
	Snapshot(context.Context) (Snapshot, error)
	Projects(context.Context) ([]domain.Project, error)
	Project(context.Context, string) (domain.Project, error)
	Repositories(context.Context, string) ([]domain.Repository, error)
	Repository(context.Context, string, string) (domain.Repository, error)
	Worktrees(context.Context, string, string) ([]domain.Worktree, error)
	Worktree(context.Context, string, string, string) (domain.Worktree, error)
	CleanupCandidates(context.Context, string) ([]domain.CleanupCandidate, error)
	CleanupPlan(context.Context, string) (CleanupPlan, error)
	CleanupResult(context.Context, string) (CleanupResult, error)
	Guidance(context.Context, string, string, string) (GuidanceReport, error)
	Proposals(context.Context, string, string, string) ([]domain.Proposal, error)
	Proposal(context.Context, string) (domain.Proposal, error)
	Checksets(context.Context, string, string) ([]domain.Checkset, error)
	Checkset(context.Context, string) (domain.Checkset, error)
	CheckRuns(context.Context, string) ([]domain.CheckRun, error)
	Findings(context.Context, string, string) ([]domain.Finding, error)
	Finding(context.Context, string) (domain.Finding, error)
	Events(context.Context, int) ([]domain.Event, error)
	EnvironmentHealth(context.Context, bool) (environment.Health, error)
	AgentProfiles(context.Context) ([]domain.AgentProfile, error)
	AgentProfile(context.Context, string) (domain.AgentProfile, error)
	Integrations(context.Context) ([]IntegrationConfig, error)
	CheckIntegration(context.Context, string) (IntegrationHealth, error)
	Runbooks(context.Context) ([]PowerShellRunbookConfig, error)
	ExternalWorkGroups(context.Context) ([]ExternalWorkGroupConfig, error)
	ExternalWorkPlan(context.Context, string) (ExternalWorkGroupPlan, error)
	ExternalWorkResult(context.Context, string) (ExternalWorkGroupResult, error)
	ReleasePlan(context.Context, string) (ReleasePlan, error)
	ReleaseResult(context.Context, string) (ReleaseResult, error)
	GitHubLatestRun(context.Context, string) (GitHubLatestRun, error)
	JenkinsLatestBuild(context.Context, string) (JenkinsLatestBuild, error)
	KubernetesStatus(context.Context, string) (KubernetesStatus, error)
	KubernetesLogs(context.Context, string) (KubernetesLogs, error)
	ActionStatus(context.Context, string) (ActionApprovalStatus, error)
	ActionPlans(context.Context) ([]domain.ActionPlan, error)
	ActionRuns(context.Context, string) ([]domain.ActionRun, error)
	RepositorySyncPlan(context.Context, string) (RepositorySyncPlan, error)
	FailureFingerprints(context.Context, int) ([]domain.FailureFingerprint, error)
	Safeguards(context.Context, int) ([]domain.SafeguardRule, error)
	Safeguard(context.Context, string) (domain.SafeguardRule, error)
	AssuranceSessions(context.Context) ([]domain.AssuranceSession, error)
	AssuranceSession(context.Context, string) (domain.AssuranceSession, error)
	AssuranceQuestions(context.Context, string) ([]domain.AssuranceQuestion, error)
	AssuranceSpecs(context.Context, string) ([]domain.AssuranceSpec, error)
	AssuranceProposals(context.Context, string) ([]domain.AssuranceProposal, error)
	QualityObjectives(context.Context) ([]domain.QualityObjective, error)
	QualityObjective(context.Context, string) (domain.QualityObjective, error)
	QualityHome(context.Context) (QualityHome, error)
	QualityTools(context.Context) (assurance.QualityToolsReadModel, error)
	QualityCampaigns(context.Context) ([]domain.QualityCampaign, error)
	QualityRuns(context.Context) ([]domain.QualityRun, error)
	AgentInvocations(context.Context) ([]domain.AgentInvocation, error)
	PRCIBaselines(context.Context) ([]domain.PRCIBaseline, error)
	AssuranceArtifacts(context.Context) ([]domain.Artifact, error)
	AssuranceEffects(context.Context) ([]domain.Effect, error)
	UnattendedApprovalScopes(context.Context) ([]domain.UnattendedApprovalScope, error)
	UnattendedApprovalScope(context.Context, string) (domain.UnattendedApprovalScope, error)
	CheckUnattendedApprovalScope(context.Context, string) (domain.UnattendedApprovalMatch, error)
	ProviderStatuses(context.Context) ([]ProviderStatus, error)
	PricingSnapshots(context.Context) ([]domain.ProviderPricingSnapshot, error)
	AssuranceDashboard(context.Context, string, string) (AssuranceDashboard, error)
	AssuranceImpact(context.Context, AssuranceImpactQuery) (AssuranceImpactDashboard, error)
	AssuranceTrace(context.Context, string) (AssuranceTrace, error)
	AssuranceArtifactStorage(context.Context) (ArtifactStorageSummary, error)
}

type CommandService interface {
	QueueScan(context.Context, string) error
	RunScan(context.Context, string) error
	AddProject(context.Context, AddProjectInput) (domain.Project, error)
	AddProjectTree(context.Context, AddProjectTreeInput) (domain.Project, error)
	DiscoverRepositories(context.Context, string) ([]RepositoryCandidate, error)
	PickDirectory(context.Context) (string, error)
	UpdateProject(context.Context, string, UpdateProjectInput) (domain.Project, error)
	RemoveProject(context.Context, string) error
	AddRepository(context.Context, AddRepositoryInput) (domain.Repository, error)
	UpdateRepository(context.Context, string, string, UpdateRepositoryInput) (domain.Repository, error)
	RemoveRepository(context.Context, string, string) error
	AcknowledgeFinding(context.Context, string) error
	Discover(context.Context, string, string, string) (domain.Discovery, error)
	ApplyProposal(context.Context, string) (domain.Proposal, error)
	RejectProposal(context.Context, string) (domain.Proposal, error)
	CreateCheckset(context.Context, CreateChecksetInput) (domain.Checkset, error)
	ApplyCheckset(context.Context, string) (domain.Checkset, error)
	RunCheckset(context.Context, string) (domain.CheckRun, error)
	ExportProject(context.Context, string) ([]byte, error)
	ImportProject(context.Context, []byte) (domain.Project, error)
	AddAgentProfile(context.Context, AddAgentProfileInput) (domain.AgentProfile, error)
	UpdateAgentProfile(context.Context, string, UpdateAgentProfileInput) (domain.AgentProfile, error)
	RemoveAgentProfile(context.Context, string) error
	AddIntegration(context.Context, AddIntegrationInput) (IntegrationConfig, error)
	UpdateIntegration(context.Context, string, UpdateIntegrationInput) (IntegrationConfig, error)
	RemoveIntegration(context.Context, string) error
	AddPowerShellRunbook(context.Context, AddPowerShellRunbookInput) (PowerShellRunbookConfig, error)
	UpdatePowerShellRunbook(context.Context, string, UpdatePowerShellRunbookInput) (PowerShellRunbookConfig, error)
	RemovePowerShellRunbook(context.Context, string) error
	AddExternalWorkGroup(context.Context, ExternalWorkGroupConfig) (ExternalWorkGroupConfig, error)
	UpdateExternalWorkGroup(context.Context, string, ExternalWorkGroupConfig) (ExternalWorkGroupConfig, error)
	RemoveExternalWorkGroup(context.Context, string) error
	PlanExternalWork(context.Context, ExternalWorkPlanInput) (ExternalWorkGroupPlan, error)
	ExecuteExternalWork(context.Context, string, string, string) (ExternalWorkGroupResult, error)
	PlanCleanup(context.Context, CleanupPlanInput) (CleanupPlan, error)
	ExecuteCleanup(context.Context, string, string, string) (CleanupResult, error)
	PlanRelease(context.Context, ReleasePlanInput) (ReleasePlan, error)
	ExecuteRelease(context.Context, string, string, string) (ReleaseResult, error)
	Schedule(context.Context, scheduler.Operation) (scheduler.Result, error)
	PlanAction(context.Context, ActionPlanInput) (domain.ActionPlan, error)
	PlanPowerShellRunbook(context.Context, PowerShellRunbookPlanInput) (domain.ActionPlan, error)
	StartHumanApprovalCeremony(context.Context, string) (action.HumanDecisionResult, error)
	AdmitAction(context.Context, string, string, string) (action.Admission, error)
	ExecuteAction(context.Context, string, string, string) (domain.ActionRun, error)
	ExecuteRepositorySync(context.Context, ExecuteRepositorySyncInput) (RepositorySyncResult, error)
	TrustActionWorktree(context.Context, string) (domain.WorktreeExecutionTrust, error)
	PrepareHandoff(context.Context, HandoffInput) (HandoffPreview, error)
	LaunchHandoff(context.Context, HandoffLaunchInput) (HandoffLaunch, error)
	ReviewSafeguard(context.Context, string, string) (domain.SafeguardRule, error)
	FeedbackSafeguard(context.Context, string, domain.SafeguardFeedback) (domain.SafeguardRule, error)
	ActivateSafeguard(context.Context, string) (domain.SafeguardRule, error)
	RollbackSafeguard(context.Context, string) (domain.SafeguardRule, error)
	RetireSafeguard(context.Context, string) (domain.SafeguardRule, error)
	RenewAction(context.Context, action.Admission) (action.Admission, error)
	ReleaseAction(context.Context, action.Admission) error
	CreateAssuranceSession(context.Context, AssuranceSessionInput) (domain.AssuranceSession, error)
	AnswerAssuranceQuestion(context.Context, string, string, string) (domain.AssuranceSession, error)
	CreateAssuranceQuestion(context.Context, AssuranceQuestionInput) (domain.AssuranceQuestion, error)
	CreateAssuranceSpec(context.Context, AssuranceSpecInput) (domain.AssuranceSpec, error)
	CreateAssuranceProposal(context.Context, AssuranceProposalInput) (domain.AssuranceProposal, error)
	ReviewAssuranceProposal(context.Context, string, string) (domain.AssuranceProposal, error)
	CreateQualityObjective(context.Context, QualityObjectiveInput) (domain.QualityObjective, error)
	DecideQualityObjective(context.Context, string, QualityObjectiveDecisionInput) (domain.QualityObjective, error)
	RevalidateQualityObjective(context.Context, string, QualityObjectiveRevalidationInput) (domain.QualityObjective, error)
	ConfirmQualityObjective(context.Context, string, QualityObjectiveConfirmationInput) (domain.QualityObjective, error)
	CreatePRCIBaseline(context.Context, BaselineInput) (domain.PRCIBaseline, error)
	CreateQualityCampaign(context.Context, QualityCampaignInput) (domain.QualityCampaign, error)
	RunQuality(context.Context, QualityRunInput) (domain.QualityRun, error)
	RunAgentInvocation(context.Context, AgentInvocationInput) (domain.AgentInvocation, error)
	RetryAgentInvocation(context.Context, string, string) (domain.AgentInvocation, error)
	SaveAssuranceArtifact(context.Context, ArtifactInput) (domain.Artifact, error)
	CreateEffect(context.Context, EffectInput) (domain.Effect, error)
	CreateUnattendedApprovalScope(context.Context, UnattendedApprovalScopeInput) (domain.UnattendedApprovalScope, error)
	ApproveUnattendedApprovalScope(context.Context, string) (domain.UnattendedApprovalScope, error)
	RevokeUnattendedApprovalScope(context.Context, string) (domain.UnattendedApprovalScope, error)
	SavePricingSnapshot(context.Context, domain.ProviderPricingSnapshot) (domain.ProviderPricingSnapshot, error)
	ExportAssuranceArtifacts(context.Context, []string, string) (ArtifactExportResult, error)
	SetAssuranceArtifactRetention(context.Context, string, string) (domain.Artifact, error)
	RestoreAssuranceArtifact(context.Context, string) (domain.Artifact, error)
	DeleteAssuranceArtifact(context.Context, string, string) (domain.Artifact, error)
	ExportAssuranceReport(context.Context, AssuranceReportQuery) (AssuranceReportExport, error)
}

type ApplicationService interface {
	QueryService
	CommandService
}

type Health struct {
	OK            bool   `json:"ok"`
	Service       string `json:"service"`
	NetworkMode   string `json:"network_mode"`
	Telemetry     bool   `json:"telemetry"`
	Contract      string `json:"contract"`
	ConfigVersion int    `json:"config_version"`
}

type AddProjectInput struct {
	Name string
	Path string
}

type AddProjectTreeInput struct {
	Name  string
	Root  string
	Paths []string
}

type RepositoryCandidate struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type AddRepositoryInput struct {
	ProjectID string
	ID        string
	Name      string
	Path      string
}

type UpdateProjectInput struct {
	Name string
}

type UpdateRepositoryInput struct {
	Name string
	Path string
}

type CreateChecksetInput struct {
	ID         string
	Name       string
	ProposalID string
	Steps      []domain.CheckStep
}

// ActionPlanInput deliberately excludes policy and approval fields. The
// broker derives those values from its reviewed server-owned definition.
type ActionPlanInput struct {
	ID, Name, ProjectID, RepositoryID, WorktreeID, ActionType string
	Inputs                                                    map[string]string
	ApprovalScopeID                                           string
	ProviderProfile                                           string
	Techniques                                                []string
	ToolSetup                                                 []string
	ToolVersion                                               string
	ToolConfigDigest                                          string
	ArgumentSchemaDigest                                      string
	WritablePaths                                             []string
	NetworkPolicy                                             string
	DiskLimitBytes                                            int64
	ScopeDeadline                                             time.Time
	ProhibitedOperations                                      []string
}

type ExternalWorkPlanInput struct {
	GroupID      string `json:"groupId"`
	ProjectID    string `json:"projectId"`
	RepositoryID string `json:"repositoryId"`
	WorktreeID   string `json:"worktreeId"`
}

type CleanupPlanInput struct {
	CandidateID  string `json:"candidateId"`
	ProjectID    string `json:"projectId"`
	RepositoryID string `json:"repositoryId"`
	WorktreeID   string `json:"worktreeId"`
}

func (a *App) PlanAction(ctx context.Context, input ActionPlanInput) (domain.ActionPlan, error) {
	plan, err := a.broker.Plan(ctx, action.PlanRequest{ID: input.ID, Name: input.Name, ProjectID: input.ProjectID, RepositoryID: input.RepositoryID, WorktreeID: input.WorktreeID, ActionType: input.ActionType, Inputs: input.Inputs, RequestedBy: domain.Actor{Kind: domain.ActorSystem, ID: "adapter"}, ApprovalScopeID: input.ApprovalScopeID, ProviderProfile: input.ProviderProfile, Techniques: input.Techniques, ToolSetup: input.ToolSetup, ToolVersion: input.ToolVersion, ToolConfigDigest: input.ToolConfigDigest, ArgumentSchemaDigest: input.ArgumentSchemaDigest, WritablePaths: input.WritablePaths, NetworkPolicy: input.NetworkPolicy, DiskLimitBytes: input.DiskLimitBytes, ScopeDeadline: input.ScopeDeadline, ProhibitedOperations: input.ProhibitedOperations})
	return plan, classifyActionError(err)
}

type ActionApprovalStatus struct {
	Plan      domain.ActionPlan      `json:"plan"`
	Approvals []domain.Approval      `json:"approvals"`
	Events    []domain.ActionEvent   `json:"events"`
	Admission action.AdmissionStatus `json:"admission"`
}

func (a *App) ActionStatus(ctx context.Context, planID string) (ActionApprovalStatus, error) {
	status, err := a.broker.Status(ctx, planID)
	if err != nil {
		return ActionApprovalStatus{}, classifyActionError(err)
	}
	return ActionApprovalStatus{Plan: status.Plan, Approvals: status.Approvals, Events: status.Events, Admission: status.Admission}, nil
}

func (a *App) ActionPlans(ctx context.Context) ([]domain.ActionPlan, error) {
	items, err := a.store.ListActionPlans(ctx)
	if err != nil {
		return nil, classifyActionError(err)
	}
	return items, nil
}

func (a *App) ActionRuns(ctx context.Context, planID string) ([]domain.ActionRun, error) {
	items, err := a.store.ListActionRuns(ctx, planID)
	if err != nil {
		return nil, classifyActionError(err)
	}
	return items, nil
}

func (a *App) StartHumanApprovalCeremony(ctx context.Context, planID string) (action.HumanDecisionResult, error) {
	result, err := a.broker.StartHumanApprovalCeremony(ctx, planID)
	return result, classifyActionError(err)
}

func (a *App) AdmitAction(ctx context.Context, planID, holder, idempotencyKey string) (action.Admission, error) {
	admission, err := a.broker.Admit(ctx, planID, holder, idempotencyKey)
	return admission, classifyActionError(err)
}

func (a *App) ExecuteAction(ctx context.Context, planID, holder, idempotencyKey string) (domain.ActionRun, error) {
	plan, planErr := a.store.GetActionPlan(ctx, planID)
	if planErr != nil {
		return domain.ActionRun{}, classifyActionError(planErr)
	}
	if plan.Spec.ActionType == "external.jenkins.group" {
		return domain.ActionRun{}, contract.InvalidInput("external Jenkins plans require the external-work execute route")
	}
	if plan.Spec.ActionType == "cleanup.destructive" {
		return domain.ActionRun{}, contract.InvalidInput("cleanup plans require the cleanup execute route")
	}
	if plan.Spec.ActionType == "release.jenkins.stage" || plan.Spec.ActionType == "release.jenkins.production" || plan.Spec.ActionType == "release.jenkins.group" {
		return domain.ActionRun{}, contract.InvalidInput("release plans require the release execute route")
	}
	admission, err := a.broker.Admit(ctx, planID, holder, idempotencyKey)
	if err != nil {
		return domain.ActionRun{}, classifyActionError(err)
	}
	revalidate := func(ctx context.Context) error {
		current, changed, err := a.discoveryWorktree(ctx, admission.Plan.Spec.ProjectID, admission.Plan.Spec.RepositoryID, admission.Plan.Spec.WorktreeID)
		if err != nil {
			return err
		}
		if changed {
			return errors.New("action Worktree evidence changed")
		}
		if admission.Plan.Spec.ActionType == repositorySyncAction {
			return validateRepositorySyncState(current, admission.Plan.Spec.ExecutionContext)
		}
		return nil
	}
	run, err := a.broker.ExecuteWithRevalidation(ctx, admission, revalidate)
	if run.Metadata.ID != "" && shouldRecordActionRunFailure(run.Spec.Status) {
		recordErr := a.recordFailureOccurrence(context.WithoutCancel(ctx), failureOccurrence{
			Category: "action", SourceType: admission.Plan.Spec.ActionType, Status: string(run.Spec.Status), ExitCode: run.Spec.ExitCode,
			ProjectID: run.Spec.ProjectID, RepositoryID: run.Spec.RepositoryID, WorktreeID: run.Spec.WorktreeID, EvidenceRef: run.Metadata.ID,
		})
		err = errors.Join(err, recordErr)
	}
	return run, classifyActionError(err)
}

func shouldRecordActionRunFailure(status domain.ActionRunStatus) bool {
	switch status {
	case domain.ActionRunPrecheckFailed, domain.ActionRunFailed, domain.ActionRunTimedOut, domain.ActionRunPostcheckFailed, domain.ActionRunUnavailable:
		return true
	default:
		return false
	}
}

func (a *App) TrustActionWorktree(ctx context.Context, planID string) (domain.WorktreeExecutionTrust, error) {
	plan, err := a.store.GetActionPlan(ctx, planID)
	if err != nil {
		return domain.WorktreeExecutionTrust{}, classifyActionError(err)
	}
	trust, err := a.broker.TrustWorktreeForExecution(ctx, plan.Spec.ProjectID, plan.Spec.RepositoryID, plan.Spec.WorktreeID, time.Now().UTC())
	if err != nil {
		return domain.WorktreeExecutionTrust{}, classifyActionError(err)
	}
	return trust, nil
}

func (a *App) RenewAction(ctx context.Context, admission action.Admission) (action.Admission, error) {
	renewed, err := a.broker.Renew(ctx, admission)
	return renewed, classifyActionError(err)
}

func (a *App) ReleaseAction(ctx context.Context, admission action.Admission) error {
	return classifyActionError(a.broker.Release(ctx, admission))
}

func classifyActionError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, action.ErrApprovalRequired), errors.Is(err, action.ErrPolicyDenied):
		return contract.CodedError{Code: contract.ErrorPolicyDenied, Message: "action requires a current human approval"}
	case errors.Is(err, action.ErrApprovalScopeMismatch):
		return contract.CodedError{Code: contract.ErrorPolicyDenied, Message: "unattended approval scope does not match the action plan"}
	case errors.Is(err, action.ErrUnknownAction):
		return contract.InvalidInput("action type is not reviewed")
	case errors.Is(err, action.ErrLockConflict), errors.Is(err, action.ErrIdempotencyConflict):
		return contract.Conflict("action admission conflicts with an active request")
	case errors.Is(err, action.ErrHumanDecisionInProgress):
		return contract.Conflict("a human approval ceremony is already active")
	case errors.Is(err, action.ErrHumanDecisionUnavailable):
		return contract.CodedError{Code: contract.ErrorUnavailable, Message: "native human approval is unavailable"}
	case errors.Is(err, action.ErrActionPrecheck), errors.Is(err, action.ErrActionPostcheck), errors.Is(err, action.ErrActionExecution):
		return contract.CodedError{Code: contract.ErrorExecutionFailed, Message: "action execution did not succeed"}
	case errors.Is(err, action.ErrExecutionContextStale), errors.Is(err, action.ErrWorktreeUntrusted):
		return contract.CodedError{Code: contract.ErrorUnavailable, Message: "action Worktree evidence is no longer current"}
	case errors.Is(err, sql.ErrNoRows):
		return contract.NotFound("action plan not found")
	default:
		return err
	}
}

type AddAgentProfileInput struct {
	ID                    string
	Name                  string
	Command               string
	VersionProbe          []string
	TimeoutSeconds        int
	ModelArgumentTemplate string
	EnvironmentAllowlist  []string
	LaunchMode            domain.AgentLaunchMode
	DataBoundary          domain.AgentDataBoundary
}

type UpdateAgentProfileInput struct {
	Name                  string
	Command               string
	VersionProbe          []string
	TimeoutSeconds        int
	ModelArgumentTemplate *string
	EnvironmentAllowlist  []string
	LaunchMode            domain.AgentLaunchMode
	DataBoundary          domain.AgentDataBoundary
}

type AssuranceSessionInput struct {
	ProjectID      string `json:"projectId"`
	RepositoryID   string `json:"repositoryId"`
	WorktreeID     string `json:"worktreeId"`
	Provider       string `json:"provider"`
	RequestedModel string `json:"requestedModel"`
}

type BaselineInput struct {
	ProjectID    string `json:"projectId"`
	RepositoryID string `json:"repositoryId"`
	WorktreeID   string `json:"worktreeId"`
	TargetBranch string `json:"targetBranch"`
}

type QualityCampaignInput struct {
	ProjectID    string `json:"projectId"`
	RepositoryID string `json:"repositoryId"`
	WorktreeID   string `json:"worktreeId"`
	Name         string `json:"name"`
	SessionID    string `json:"sessionId"`
}

type QualityObjectiveInput struct {
	ProjectID     string                         `json:"projectId"`
	RepositoryID  string                         `json:"repositoryId"`
	WorktreeID    string                         `json:"worktreeId"`
	Owner         string                         `json:"owner"`
	Title         string                         `json:"title"`
	Description   string                         `json:"description"`
	FindingIDs    []string                       `json:"findingIds"`
	SessionID     string                         `json:"sessionId"`
	BaselineID    string                         `json:"baselineId"`
	CampaignID    string                         `json:"campaignId"`
	RunIDs        []string                       `json:"runIds"`
	ProposalIDs   []string                       `json:"proposalIds"`
	PrimarySignal *domain.QualityObjectiveSignal `json:"primarySignal,omitempty"`
}

type QualityObjectiveDecisionInput struct {
	ExpectedRevision int     `json:"expectedRevision"`
	Disposition      string  `json:"disposition"`
	Action           string  `json:"action"`
	Reason           string  `json:"reason"`
	Actor            string  `json:"actor"`
	MinimumPercent   float64 `json:"minimumPercent,omitempty"`
}

type QualityObjectiveRevalidationInput struct {
	ExpectedRevision int    `json:"expectedRevision"`
	FindingID        string `json:"findingId,omitempty"`
	QualityRunID     string `json:"qualityRunId,omitempty"`
}

type QualityObjectiveConfirmationInput struct {
	ExpectedRevision int `json:"expectedRevision"`
}

type QualityRunInput struct {
	CampaignID string `json:"campaignId"`
	Technique  string `json:"technique"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
}

type AgentInvocationInput struct {
	SessionID      string `json:"sessionId"`
	Provider       string `json:"provider"`
	ProfileID      string `json:"profileId"`
	RequestedModel string `json:"requestedModel"`
	// Prompt is transient execution input. It is never copied to an
	// AgentInvocation, artifact, log, result, or UI-facing object.
	Prompt   string `json:"prompt,omitempty"`
	Scenario string `json:"scenario,omitempty"`
}

type AssuranceQuestionInput struct {
	SessionID string `json:"sessionId"`
	Prompt    string `json:"prompt"`
	Required  bool   `json:"required"`
}

type AssuranceSpecInput struct {
	SessionID  string   `json:"sessionId"`
	Intent     string   `json:"intent"`
	Questions  []string `json:"questions"`
	Properties []string `json:"properties"`
	Targets    []string `json:"targets"`
	ToolSetup  []string `json:"toolSetup"`
	Source     string   `json:"source"`
}

type AssuranceProposalInput struct {
	SessionID string `json:"sessionId"`
	Purpose   string `json:"purpose"`
	Patch     string `json:"patch"`
}

type ArtifactInput struct {
	SourceType string `json:"sourceType"`
	SourceID   string `json:"sourceId"`
	Name       string `json:"name"`
	MIME       string `json:"mime"`
	Content    []byte `json:"-"`
	TraceID    string `json:"traceId,omitempty"`
}

type EffectInput struct {
	ProjectID           string     `json:"projectId"`
	RepositoryID        string     `json:"repositoryId"`
	WorktreeID          string     `json:"worktreeId"`
	Fingerprint         string     `json:"fingerprint"`
	Kind                string     `json:"kind"`
	SourceRunID         string     `json:"sourceRunId"`
	EvidenceIDs         []string   `json:"evidenceIds"`
	Adopted             bool       `json:"adopted"`
	Reverified          bool       `json:"reverified"`
	Label               string     `json:"label"`
	Value               float64    `json:"value"`
	Unit                string     `json:"unit"`
	MetricKey           string     `json:"metricKey"`
	BaselineID          string     `json:"baselineId"`
	SourceFindingID     string     `json:"sourceFindingId"`
	TraceIDs            []string   `json:"traceIds"`
	TraceID             string     `json:"traceId"`
	ValueKnown          bool       `json:"valueKnown"`
	BaselineValue       *float64   `json:"baselineValue"`
	BaselineUnit        string     `json:"baselineUnit"`
	Outcome             string     `json:"outcome"`
	Note                string     `json:"note"`
	AdoptedAt           *time.Time `json:"adoptedAt"`
	ReverifiedAt        *time.Time `json:"reverifiedAt"`
	AdoptedCommit       string     `json:"adoptedCommit"`
	ReverificationRunID string     `json:"reverificationRunId"`
	ReverifiedCommit    string     `json:"reverifiedCommit"`
	PeriodStart         *time.Time `json:"periodStart"`
	PeriodEnd           *time.Time `json:"periodEnd"`
	RecordedBy          string     `json:"recordedBy"`
	Reason              string     `json:"reason"`
}

type UnattendedApprovalScopeInput struct {
	ID                   string              `json:"id,omitempty"`
	Name                 string              `json:"name"`
	ProjectID            string              `json:"projectId"`
	RepositoryID         string              `json:"repositoryId"`
	WorktreeID           string              `json:"worktreeId"`
	ProviderProfile      string              `json:"providerProfile"`
	ActionTypes          []string            `json:"actionTypes"`
	RiskClasses          []domain.ActionRisk `json:"riskClasses"`
	Techniques           []string            `json:"techniques"`
	ToolSetup            []string            `json:"toolSetup"`
	ToolVersion          string              `json:"toolVersion"`
	ToolConfigDigest     string              `json:"toolConfigDigest"`
	ArgumentSchemaDigest string              `json:"argumentSchemaDigest"`
	WritablePaths        []string            `json:"writablePaths"`
	NetworkPolicy        string              `json:"networkPolicy"`
	DiskLimitBytes       int64               `json:"diskLimitBytes"`
	Deadline             time.Time           `json:"deadline"`
	Prohibited           []string            `json:"prohibited"`
}

type ProviderStatus struct {
	Provider        string   `json:"provider"`
	State           string   `json:"state"`
	CommandFound    bool     `json:"commandFound"`
	LaunchTrusted   bool     `json:"launchTrusted"`
	ProfileReady    bool     `json:"profileReady"`
	ResolvedCommand []string `json:"resolvedCommand,omitempty"`
	Version         string   `json:"version,omitempty"`
	ReasonCode      string   `json:"reasonCode,omitempty"`
	Detail          string   `json:"detail,omitempty"`
}

type ArtifactExportResult struct {
	Destination string   `json:"destination"`
	ArtifactIDs []string `json:"artifactIds"`
	Manifest    string   `json:"manifest"`
	ManifestSHA string   `json:"manifestSha256"`
	Verified    bool     `json:"verified"`
}

type AssuranceDashboard struct {
	GeneratedAt    time.Time                    `json:"generatedAt"`
	ProviderFilter string                       `json:"providerFilter,omitempty"`
	ModelFilter    string                       `json:"modelFilter,omitempty"`
	Effects        []domain.Effect              `json:"effects"`
	Invocations    []domain.AgentInvocation     `json:"invocations"`
	TotalTokens    int64                        `json:"totalTokens"`
	UsageComplete  bool                         `json:"usageComplete"`
	EstimatedCost  *float64                     `json:"estimatedCost,omitempty"`
	CostLabel      string                       `json:"costLabel"`
	CostState      string                       `json:"costState"`
	Impact         AssuranceImpactDashboard     `json:"impact"`
	Traceability   AssuranceTraceabilitySummary `json:"traceability"`
}

type GuidanceFinding struct {
	Severity              string `json:"severity"`
	Code                  string `json:"code"`
	File                  string `json:"file,omitempty"`
	Summary               string `json:"summary"`
	RecommendedNextAction string `json:"recommendedNextAction"`
}

type GuidanceReport struct {
	ProjectID    string            `json:"projectId"`
	RepositoryID string            `json:"repositoryId"`
	WorktreeID   string            `json:"worktreeId"`
	CheckedAt    time.Time         `json:"checkedAt"`
	Files        []string          `json:"files"`
	Findings     []GuidanceFinding `json:"findings"`
}

type HandoffInput struct {
	ProfileID    string `json:"profileId"`
	ProjectID    string `json:"projectId"`
	RepositoryID string `json:"repositoryId"`
	WorktreeID   string `json:"worktreeId"`
	Model        string `json:"model,omitempty"`
}

type HandoffLaunchInput struct {
	HandoffInput
	PreviewDigest string `json:"previewDigest"`
}

type HandoffFinding struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
	Next     string `json:"recommendedNextAction"`
}

type HandoffPreview struct {
	ProfileID             string           `json:"profileId"`
	ProfileName           string           `json:"profileName"`
	ProfileCommand        string           `json:"profileCommand"`
	Model                 string           `json:"model,omitempty"`
	ModelArgumentTemplate string           `json:"modelArgumentTemplate,omitempty"`
	LaunchMode            string           `json:"launchMode"`
	DataBoundary          string           `json:"dataBoundary"`
	ProjectID             string           `json:"projectId"`
	RepositoryID          string           `json:"repositoryId"`
	WorktreeID            string           `json:"worktreeId"`
	WorkingDirectory      string           `json:"workingDirectory"`
	Scope                 []string         `json:"scope"`
	Findings              []HandoffFinding `json:"findings"`
	VerificationCommands  []string         `json:"verificationCommands"`
	TranscriptIncluded    bool             `json:"transcriptIncluded"`
	Head                  string           `json:"head,omitempty"`
	Branch                string           `json:"branch,omitempty"`
	Dirty                 bool             `json:"dirty"`
	Untracked             bool             `json:"untracked"`
	PreviewDigest         string           `json:"previewDigest"`
	Arguments             []string         `json:"arguments"`
}

type HandoffLaunch struct {
	ProfileID          string    `json:"profileId"`
	ProfileName        string    `json:"profileName"`
	Model              string    `json:"model,omitempty"`
	LaunchMode         string    `json:"launchMode"`
	DataBoundary       string    `json:"dataBoundary"`
	ProjectID          string    `json:"projectId"`
	RepositoryID       string    `json:"repositoryId"`
	WorktreeID         string    `json:"worktreeId"`
	WorkingDirectory   string    `json:"workingDirectory"`
	PreviewDigest      string    `json:"previewDigest"`
	PID                int       `json:"pid"`
	StartedAt          time.Time `json:"startedAt"`
	TranscriptIncluded bool      `json:"transcriptIncluded"`
}
