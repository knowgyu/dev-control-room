package app

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/knowgyu/dev-control-room/internal/action"
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
	Schedule(context.Context, scheduler.Operation) (scheduler.Result, error)
	PlanAction(context.Context, ActionPlanInput) (domain.ActionPlan, error)
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
}

func (a *App) PlanAction(ctx context.Context, input ActionPlanInput) (domain.ActionPlan, error) {
	plan, err := a.broker.Plan(ctx, action.PlanRequest{ID: input.ID, Name: input.Name, ProjectID: input.ProjectID, RepositoryID: input.RepositoryID, WorktreeID: input.WorktreeID, ActionType: input.ActionType, Inputs: input.Inputs, RequestedBy: domain.Actor{Kind: domain.ActorSystem, ID: "adapter"}})
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
