package app

import (
	"context"
	"database/sql"
	"errors"

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
	ActionStatus(context.Context, string) (ActionApprovalStatus, error)
}

type CommandService interface {
	QueueScan(context.Context, string) error
	RunScan(context.Context, string) error
	AddProject(context.Context, AddProjectInput) (domain.Project, error)
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
	Schedule(context.Context, scheduler.Operation) (scheduler.Result, error)
	PlanAction(context.Context, ActionPlanInput) (domain.ActionPlan, error)
	StartHumanApprovalCeremony(context.Context, string) (action.HumanDecisionResult, error)
	AdmitAction(context.Context, string, string, string) (action.Admission, error)
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

func (a *App) StartHumanApprovalCeremony(ctx context.Context, planID string) (action.HumanDecisionResult, error) {
	result, err := a.broker.StartHumanApprovalCeremony(ctx, planID)
	return result, classifyActionError(err)
}

func (a *App) AdmitAction(ctx context.Context, planID, holder, idempotencyKey string) (action.Admission, error) {
	admission, err := a.broker.Admit(ctx, planID, holder, idempotencyKey)
	return admission, classifyActionError(err)
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
	case errors.Is(err, sql.ErrNoRows):
		return contract.NotFound("action plan not found")
	default:
		return err
	}
}

type AddAgentProfileInput struct {
	ID                   string
	Name                 string
	Command              string
	VersionProbe         []string
	TimeoutSeconds       int
	EnvironmentAllowlist []string
	LaunchMode           domain.AgentLaunchMode
	DataBoundary         domain.AgentDataBoundary
}

type UpdateAgentProfileInput struct {
	Name                 string
	Command              string
	VersionProbe         []string
	TimeoutSeconds       int
	EnvironmentAllowlist []string
	LaunchMode           domain.AgentLaunchMode
	DataBoundary         domain.AgentDataBoundary
}
