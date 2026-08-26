// Package action contains the policy and execution boundary for reviewed
// Actions. It only starts server-owned typed commands after Broker admission.
package action

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/environment"
	"github.com/knowgyu/dev-control-room/internal/masking"
	"github.com/knowgyu/dev-control-room/internal/store"
)

var (
	ErrPolicyDenied             = errors.New("action policy denies this plan")
	ErrApprovalRequired         = errors.New("action requires a current human approval")
	ErrLockConflict             = errors.New("action target is locked or expired")
	ErrIdempotencyConflict      = errors.New("action idempotency key was already used")
	ErrUnknownAction            = errors.New("action type is not reviewed")
	ErrWorktreeUntrusted        = errors.New("action worktree is not trusted for execution")
	ErrExecutionContextStale    = errors.New("action worktree execution context is stale")
	ErrHumanDecisionUnavailable = errors.New("native human decision prompt is unavailable")
	ErrHumanDecisionInProgress  = errors.New("a human decision ceremony is already active")
	ErrApprovalScopeMismatch    = errors.New("unattended approval scope does not match the action plan")
	ErrActionExecution          = errors.New("action execution failed")
	ErrActionPrecheck           = errors.New("action precheck failed")
	ErrActionPostcheck          = errors.New("action postcheck failed")
)

const (
	leaseDuration    = 5 * time.Minute
	approvalLifetime = 15 * time.Minute
)

// HumanDecisionPrompt is the broker's only approval authority. Its request is
// constructed from a persisted plan; adapters never provide an actor, digest,
// expiry, or decision.
type HumanDecisionPrompt interface {
	Decide(context.Context, HumanDecisionRequest) (HumanDecision, error)
}

type HumanDecision string

const (
	HumanDecisionGrant  HumanDecision = "granted"
	HumanDecisionReject HumanDecision = "rejected"
	HumanDecisionCancel HumanDecision = "cancelled"
)

type HumanDecisionRequest struct {
	Plan       string
	Digest     string
	Worktree   string
	Executable string
	ExpiresAt  time.Time
}

type HumanDecisionResult struct {
	Decision HumanDecision `json:"decision"`
}

type Broker struct {
	store          *store.Store
	now            func() time.Time
	prompt         HumanDecisionPrompt
	runner         ProcessRunner
	masker         *masking.Masker
	ceremonyMu     sync.Mutex
	ceremonyActive bool
}

func New(persistence *store.Store, now func() time.Time, prompts ...HumanDecisionPrompt) (*Broker, error) {
	return newBroker(persistence, now, defaultProcessRunner{}, prompts...)
}

func NewWithRunner(persistence *store.Store, now func() time.Time, runner ProcessRunner, prompts ...HumanDecisionPrompt) (*Broker, error) {
	if runner == nil {
		runner = defaultProcessRunner{}
	}
	return newBroker(persistence, now, runner, prompts...)
}

func newBroker(persistence *store.Store, now func() time.Time, runner ProcessRunner, prompts ...HumanDecisionPrompt) (*Broker, error) {
	if persistence == nil {
		return nil, errors.New("action persistence is required")
	}
	if now == nil {
		now = time.Now
	}
	prompt := newHumanDecisionPrompt()
	if len(prompts) > 0 && prompts[0] != nil {
		prompt = prompts[0]
	}
	return &Broker{store: persistence, now: now, prompt: prompt, runner: runner, masker: masking.New(nil, []string{"TOKEN", "PASSWORD", "SECRET", "API_KEY", "AUTHORIZATION"})}, nil
}

// ProcessResult is the bounded result returned by one typed Action command.
type ProcessResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type ProcessRunner interface {
	Run(context.Context, string, []string, []string, string, time.Duration, int) (ProcessResult, error)
}

type defaultProcessRunner struct{}

func (defaultProcessRunner) Run(ctx context.Context, executable string, args []string, env []string, directory string, timeout time.Duration, outputLimit int) (ProcessResult, error) {
	result, err := (environment.ProcessRunner{OutputLimit: outputLimit}).RunInDirectory(ctx, executable, args, env, directory, timeout)
	return ProcessResult{Stdout: result.Stdout, Stderr: result.Stderr, ExitCode: result.ExitCode}, err
}

// PlanRequest deliberately excludes risk and policy. Those fields are derived
// from the server-owned reviewed action definition.
type PlanRequest struct {
	ID, Name, ProjectID, RepositoryID, WorktreeID, ActionType string
	Inputs                                                    map[string]string
	RequestedBy                                               domain.Actor
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

func (b *Broker) Plan(ctx context.Context, request PlanRequest) (domain.ActionPlan, error) {
	definition, ok := domain.ActionDefinitionFor(request.ActionType)
	if !ok {
		return domain.ActionPlan{}, ErrUnknownAction
	}
	worktree, err := b.store.GetWorktree(ctx, request.ProjectID, request.RepositoryID, request.WorktreeID)
	if err != nil {
		return domain.ActionPlan{}, err
	}
	executionContext, err := domain.ExecutionContextForWorktree(worktree)
	if err != nil {
		return domain.ActionPlan{}, err
	}
	execution, err := definition.ExecutionFor(request.Inputs)
	if err != nil {
		return domain.ActionPlan{}, err
	}
	now := b.now().UTC()
	plan := domain.ActionPlan{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ActionPlanKind}, Metadata: domain.ObjectMeta{ID: request.ID, Name: request.Name}, Spec: domain.ActionPlanSpec{ProjectID: request.ProjectID, RepositoryID: request.RepositoryID, WorktreeID: request.WorktreeID, ActionType: definition.ActionType, Risk: definition.Risk, Inputs: request.Inputs, Execution: execution, ExecutionContext: executionContext, Prechecks: definition.Prechecks, Postchecks: definition.Postchecks, PolicyDecision: definition.PolicyDecision, ApprovalRequired: definition.ApprovalRequired, RequestedBy: request.RequestedBy, RequestedAt: now}}
	if request.ApprovalScopeID != "" {
		scope, scopeErr := b.store.GetUnattendedApprovalScope(ctx, request.ApprovalScopeID)
		if scopeErr != nil {
			return domain.ActionPlan{}, scopeErr
		}
		scopeDigest, digestErr := scope.Digest()
		if digestErr != nil {
			return domain.ActionPlan{}, fmt.Errorf("digest unattended approval scope: %w", digestErr)
		}
		match := scope.Match(domain.UnattendedApprovalRequest{ScopeID: scope.Metadata.ID, ScopeDigest: scopeDigest, ProjectID: request.ProjectID, RepositoryID: request.RepositoryID, WorktreeID: request.WorktreeID, ProviderProfile: request.ProviderProfile, ActionType: definition.ActionType, Risk: definition.Risk, Techniques: request.Techniques, ToolSetup: request.ToolSetup, ToolVersion: request.ToolVersion, ToolConfigDigest: request.ToolConfigDigest, ArgumentSchemaDigest: request.ArgumentSchemaDigest, WritablePaths: request.WritablePaths, NetworkPolicy: request.NetworkPolicy, DiskBytes: request.DiskLimitBytes, Deadline: request.ScopeDeadline, Prohibited: request.ProhibitedOperations}, now)
		plan.Spec.ApprovalScopeID = scope.Metadata.ID
		plan.Spec.ApprovalScopeDigest = scopeDigest
		plan.Spec.ProviderProfile = request.ProviderProfile
		plan.Spec.Techniques = append([]string(nil), request.Techniques...)
		plan.Spec.ToolSetup = append([]string(nil), request.ToolSetup...)
		plan.Spec.ToolVersion = request.ToolVersion
		plan.Spec.ToolConfigDigest = request.ToolConfigDigest
		plan.Spec.ArgumentSchemaDigest = request.ArgumentSchemaDigest
		plan.Spec.WritablePaths = append([]string(nil), request.WritablePaths...)
		plan.Spec.NetworkPolicy = request.NetworkPolicy
		plan.Spec.DiskLimitBytes = request.DiskLimitBytes
		plan.Spec.ScopeDeadline = request.ScopeDeadline
		plan.Spec.ProhibitedOperations = append([]string(nil), request.ProhibitedOperations...)
		plan.Spec.ScopeMatch = match.Matched
		plan.Spec.ScopeMatchReasons = append([]string(nil), match.Reasons...)
		plan.Spec.ScopeCheckedAt = match.CheckedAt
	}
	if err := b.store.SaveActionPlan(ctx, plan); err != nil {
		return domain.ActionPlan{}, err
	}
	persisted, err := b.store.GetActionPlan(ctx, plan.Metadata.ID)
	if err != nil {
		return domain.ActionPlan{}, err
	}
	if err := b.audit(ctx, persisted, "planned", persisted.Spec.RequestedBy, now, persisted.Metadata.ID); err != nil {
		return domain.ActionPlan{}, err
	}
	if persisted.Spec.ApprovalScopeID != "" && !persisted.Spec.ScopeMatch {
		return persisted, ErrApprovalScopeMismatch
	}
	return persisted, nil
}

// StartHumanApprovalCeremony is deliberately the only public approval path.
// The modal receives server-derived, input-free plan metadata and its decision
// is the sole source of a granted approval.
func (b *Broker) StartHumanApprovalCeremony(ctx context.Context, planID string) (HumanDecisionResult, error) {
	b.ceremonyMu.Lock()
	if b.ceremonyActive {
		b.ceremonyMu.Unlock()
		return HumanDecisionResult{}, ErrHumanDecisionInProgress
	}
	b.ceremonyActive = true
	b.ceremonyMu.Unlock()
	defer func() {
		b.ceremonyMu.Lock()
		b.ceremonyActive = false
		b.ceremonyMu.Unlock()
	}()

	plan, err := b.store.GetActionPlan(ctx, planID)
	if err != nil {
		return HumanDecisionResult{}, err
	}
	now := b.now().UTC()
	digest, err := plan.Digest()
	if err != nil {
		return HumanDecisionResult{}, fmt.Errorf("digest action plan: %w", err)
	}
	expiresAt := now.Add(approvalLifetime)
	decision, err := b.prompt.Decide(ctx, HumanDecisionRequest{Plan: plan.Spec.ActionType, Digest: digest, Worktree: plan.Spec.WorktreeID, Executable: plan.Spec.Execution.Executable, ExpiresAt: expiresAt})
	if err != nil {
		return HumanDecisionResult{}, fmt.Errorf("native human decision: %w", err)
	}
	if decision == HumanDecisionCancel {
		return HumanDecisionResult{Decision: decision}, nil
	}
	if decision != HumanDecisionGrant && decision != HumanDecisionReject {
		return HumanDecisionResult{}, ErrHumanDecisionUnavailable
	}
	human := domain.Actor{Kind: domain.ActorHuman, ID: "local-user"}
	approvals, err := b.store.ListApprovals(ctx, planID)
	if err != nil {
		return HumanDecisionResult{}, err
	}
	approvalID := decisionID(plan.Metadata.ID, decision, now, len(approvals))
	approval := domain.Approval{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ApprovalKind}, Metadata: domain.ObjectMeta{ID: approvalID, Name: "Human action approval"}, Spec: domain.ApprovalSpec{ActionPlanID: planID, ActionPlanDigest: digest, Status: domain.ApprovalGranted, RequestedBy: plan.Spec.RequestedBy, ApprovedBy: &human, ExpiresAt: &expiresAt, DecidedAt: now}}
	eventType := "approval_granted"
	if decision == HumanDecisionReject {
		approval.Spec.Status = domain.ApprovalRejected
		approval.Spec.Reason = "rejected in native confirmation"
		approval.Spec.ExpiresAt = nil
		eventType = "approval_rejected"
	}
	if err := b.store.SaveApprovalAndActionEvent(ctx, approval, b.event(plan, eventType, human, now, approvalID), now); err != nil {
		return HumanDecisionResult{}, err
	}
	return HumanDecisionResult{Decision: decision}, nil
}

func decisionID(planID string, decision HumanDecision, now time.Time, ordinal int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%d", planID, decision, now.UTC().Format(time.RFC3339Nano), ordinal)))
	return "approval-" + hex.EncodeToString(sum[:])[:55]
}

type Admission struct {
	Plan domain.ActionPlan
	Lock store.ActionLock
}

type AdmissionStatus string

const (
	AdmissionEligible         AdmissionStatus = "eligible"
	AdmissionPolicyDenied     AdmissionStatus = "policy_denied"
	AdmissionApprovalRequired AdmissionStatus = "approval_required"
)

// Status is a persisted, read-only snapshot for adapters. Eligible means the
// current policy and approval evidence permit a later Admit call; it neither
// acquires a lock nor grants any authority itself.
type Status struct {
	Plan      domain.ActionPlan
	Approvals []domain.Approval
	Events    []domain.ActionEvent
	Admission AdmissionStatus
}

func (b *Broker) Status(ctx context.Context, planID string) (Status, error) {
	plan, err := b.store.GetActionPlan(ctx, planID)
	if err != nil {
		return Status{}, err
	}
	approvals, err := b.store.ListApprovals(ctx, planID)
	if err != nil {
		return Status{}, err
	}
	events, err := b.store.ListActionEvents(ctx, planID)
	if err != nil {
		return Status{}, err
	}
	status := AdmissionEligible
	if plan.Spec.PolicyDecision == domain.PolicyDenied {
		status = AdmissionPolicyDenied
	} else if plan.Spec.ApprovalRequired {
		status = AdmissionApprovalRequired
		now := b.now().UTC()
		for _, approval := range approvals {
			if approval.Spec.Status == domain.ApprovalGranted && approval.ValidateForAt(plan, now) == nil {
				status = AdmissionEligible
				break
			}
		}
	}
	return Status{Plan: plan, Approvals: approvals, Events: events, Admission: status}, nil
}

func (b *Broker) Admit(ctx context.Context, planID, holder, idempotencyKey string) (Admission, error) {
	plan, err := b.store.GetActionPlan(ctx, planID)
	if err != nil {
		return Admission{}, err
	}
	if plan.Spec.PolicyDecision == domain.PolicyDenied {
		return Admission{}, ErrPolicyDenied
	}
	if err := b.validateApprovalScope(ctx, plan); err != nil {
		return Admission{}, err
	}
	now := b.now().UTC()
	if plan.Spec.ApprovalRequired {
		approvals, err := b.store.ListApprovals(ctx, planID)
		if err != nil {
			return Admission{}, err
		}
		approved := false
		for _, approval := range approvals {
			if approval.Spec.Status == domain.ApprovalGranted && approval.ValidateForAt(plan, now) == nil {
				approved = true
				break
			}
		}
		if !approved {
			return Admission{}, ErrApprovalRequired
		}
	}
	if err := b.validateExecutionContext(ctx, plan); err != nil {
		return Admission{}, err
	}
	digest, err := plan.Digest()
	if err != nil {
		return Admission{}, fmt.Errorf("digest action plan: %w", err)
	}
	lock := store.ActionLock{Scope: scope(plan), ActionPlanID: plan.Metadata.ID, ActionPlanDigest: digest, Holder: holder, ExpiresAt: now.Add(leaseDuration)}
	if err := b.store.AcquireActionLock(ctx, lock, now); err != nil {
		if errors.Is(err, store.ErrActionLockHeld) {
			return Admission{}, ErrLockConflict
		}
		return Admission{}, err
	}
	if err := b.store.ClaimActionIdempotency(ctx, idempotencyKey, plan.Metadata.ID, digest, now); err != nil {
		_ = b.store.ReleaseActionLock(ctx, lock)
		if errors.Is(err, store.ErrActionIdempotencyClaimed) {
			return Admission{}, ErrIdempotencyConflict
		}
		return Admission{}, err
	}
	if err := b.audit(ctx, plan, "admitted", domain.Actor{Kind: domain.ActorSystem, ID: holder}, now, idempotencyKey); err != nil {
		_ = b.store.ReleaseActionLock(ctx, lock)
		_ = b.store.ReleaseActionIdempotency(ctx, idempotencyKey, plan.Metadata.ID, digest)
		return Admission{}, err
	}
	return Admission{Plan: plan, Lock: lock}, nil
}

func (b *Broker) validateExecutionContext(ctx context.Context, plan domain.ActionPlan) error {
	worktree, err := b.store.GetWorktree(ctx, plan.Spec.ProjectID, plan.Spec.RepositoryID, plan.Spec.WorktreeID)
	if err != nil {
		return err
	}
	if worktree.Spec.Trust != domain.WorktreeTrustVerifiedReadOnly {
		return ErrWorktreeUntrusted
	}
	current, err := domain.ExecutionContextForWorktree(worktree)
	if err != nil {
		return ErrExecutionContextStale
	}
	if current != plan.Spec.ExecutionContext {
		return ErrExecutionContextStale
	}
	trust, err := b.store.GetWorktreeExecutionTrust(ctx, current.ProjectID, current.RepositoryID, current.WorktreeID)
	if err != nil {
		return ErrWorktreeUntrusted
	}
	if trust.Context != current {
		return ErrWorktreeUntrusted
	}
	return nil
}

func (b *Broker) validateApprovalScope(ctx context.Context, plan domain.ActionPlan) error {
	if plan.Spec.ApprovalScopeID == "" {
		return nil
	}
	scope, err := b.store.GetUnattendedApprovalScope(ctx, plan.Spec.ApprovalScopeID)
	if err != nil {
		return err
	}
	match := scope.Match(plan.UnattendedApprovalRequest(), b.now().UTC())
	if !match.Matched || match.ScopeDigest != plan.Spec.ApprovalScopeDigest || !plan.Spec.ScopeMatch {
		return ErrApprovalScopeMismatch
	}
	return nil
}

func (b *Broker) Renew(ctx context.Context, admission Admission) (Admission, error) {
	now := b.now().UTC()
	lock, err := b.store.RenewActionLock(ctx, admission.Lock, now, now.Add(leaseDuration))
	if errors.Is(err, store.ErrActionLockHeld) {
		return Admission{}, ErrLockConflict
	}
	if err != nil {
		return Admission{}, err
	}
	admission.Lock = lock
	return admission, nil
}

func (b *Broker) Release(ctx context.Context, admission Admission) error {
	return b.store.ReleaseActionLock(ctx, admission.Lock)
}

// TrustWorktreeForExecution records the explicit user-facing transition from
// read-only observation to eligibility. It never approves an ActionPlan.
func (b *Broker) TrustWorktreeForExecution(ctx context.Context, projectID, repositoryID, worktreeID string, at time.Time) (domain.WorktreeExecutionTrust, error) {
	worktree, err := b.store.GetWorktree(ctx, projectID, repositoryID, worktreeID)
	if err != nil {
		return domain.WorktreeExecutionTrust{}, err
	}
	if worktree.Spec.Trust != domain.WorktreeTrustVerifiedReadOnly {
		return domain.WorktreeExecutionTrust{}, ErrWorktreeUntrusted
	}
	return b.store.TrustWorktreeForExecution(ctx, projectID, repositoryID, worktreeID, at.UTC())
}

// Execute consumes an already admitted plan. It is the only process execution
// path: the caller cannot provide an executable, arguments, working directory,
// environment, approval, or target outside the persisted admission.
func (b *Broker) Execute(ctx context.Context, admission Admission) (domain.ActionRun, error) {
	return b.ExecuteWithRevalidation(ctx, admission, nil)
}

// ExecuteWithRevalidation lets the application layer refresh read-only Git
// evidence immediately before launch and after completion. Broker checks still
// run against the persisted exact Worktree context at both boundaries.
func (b *Broker) ExecuteWithRevalidation(ctx context.Context, admission Admission, revalidate func(context.Context) error) (domain.ActionRun, error) {
	plan, err := b.store.GetActionPlan(ctx, admission.Lock.ActionPlanID)
	if err != nil {
		return domain.ActionRun{}, err
	}
	if admission.Plan.Metadata.ID != plan.Metadata.ID || admission.Lock.ActionPlanID != plan.Metadata.ID || admission.Lock.Scope != scope(plan) || admission.Lock.Holder == "" || !admission.Lock.ExpiresAt.After(b.now().UTC()) {
		return domain.ActionRun{}, ErrLockConflict
	}
	// Once an admission has been acquired, every pre-launch rejection must
	// release its lease. This matters when a scope is revoked or expires after
	// admission but before the execution boundary is reached.
	defer func() { _ = b.Release(context.Background(), admission) }()
	digest, err := plan.Digest()
	if err != nil || digest != admission.Lock.ActionPlanDigest {
		return domain.ActionRun{}, ErrExecutionContextStale
	}
	admissionDigest, admissionErr := admission.Plan.Digest()
	if admissionErr != nil || admissionDigest != digest {
		return domain.ActionRun{}, ErrExecutionContextStale
	}
	if plan.Spec.PolicyDecision == domain.PolicyDenied {
		return domain.ActionRun{}, ErrPolicyDenied
	}
	if err := b.validateApprovalScope(ctx, plan); err != nil {
		return domain.ActionRun{}, err
	}
	if plan.Spec.ApprovalRequired {
		approvals, approvalErr := b.store.ListApprovals(ctx, plan.Metadata.ID)
		if approvalErr != nil {
			return domain.ActionRun{}, approvalErr
		}
		approved := false
		for _, approval := range approvals {
			if approval.Spec.Status == domain.ApprovalGranted && approval.ValidateForAt(plan, b.now().UTC()) == nil {
				approved = true
				break
			}
		}
		if !approved {
			return domain.ActionRun{}, ErrApprovalRequired
		}
	}
	if revalidate != nil {
		if err := revalidate(ctx); err != nil {
			return b.saveRejectedRun(ctx, plan, admission.Lock.Holder, b.now().UTC(), "precheck_failed", err)
		}
	}
	if err := b.validateExecutionContext(ctx, plan); err != nil {
		return b.saveRejectedRun(ctx, plan, admission.Lock.Holder, b.now().UTC(), "precheck_failed", err)
	}
	prechecks := []domain.ActionEvidence{{ID: "worktree-identity", Kind: domain.EvidenceWorktreeIdentity, Passed: true, Detail: "verified execution context"}, {ID: "worktree-head", Kind: domain.EvidenceWorktreeHead, Passed: true, Detail: plan.Spec.ExecutionContext.Head}}
	started := b.now().UTC()
	runID := actionRunID(plan.Metadata.ID, admission.Lock.Holder, started)
	if err := b.audit(ctx, plan, "started", domain.Actor{Kind: domain.ActorSystem, ID: admission.Lock.Holder}, started, runID+"-started"); err != nil {
		return domain.ActionRun{}, err
	}
	command := plan.Spec.Execution
	result, processErr := b.runner.Run(ctx, command.Executable, command.Arguments, environment.AllowlistedEnvironment(command.EnvironmentAllowlist), plan.Spec.ExecutionContext.CanonicalPath, time.Duration(command.TimeoutSeconds)*time.Second, command.MaxOutputBytes)
	completed := b.now().UTC()
	postchecks := []domain.ActionEvidence{{ID: "process-exit", Kind: domain.EvidenceProcessExit, Passed: processErr == nil && result.ExitCode == 0, Detail: strconv.Itoa(result.ExitCode)}}
	status := domain.ActionRunSucceeded
	terminalErr := error(nil)
	if processErr != nil {
		switch {
		case errors.Is(processErr, context.DeadlineExceeded) || strings.Contains(strings.ToLower(processErr.Error()), "timed out"):
			status, terminalErr = domain.ActionRunTimedOut, ErrActionExecution
		case ctx.Err() != nil:
			status, terminalErr = domain.ActionRunCancelled, ErrActionExecution
		case result.ExitCode != 0:
			status, terminalErr = domain.ActionRunFailed, ErrActionExecution
		default:
			status, terminalErr = domain.ActionRunUnavailable, ErrActionExecution
		}
	}
	if status == domain.ActionRunSucceeded {
		if revalidate != nil {
			if err := revalidate(ctx); err != nil {
				postchecks[0].Passed = false
				status, terminalErr = domain.ActionRunPostcheckFailed, ErrActionPostcheck
			}
		}
		// repository.sync is intentionally allowed to advance HEAD through a
		// fast-forward pull. Its application-level revalidation still enforces
		// the exact path, branch, clean state, and upstream contract.
		if plan.Spec.ActionType != "repository.sync" {
			if err := b.validateExecutionContext(ctx, plan); err != nil {
				postchecks[0].Passed = false
				status, terminalErr = domain.ActionRunPostcheckFailed, ErrActionPostcheck
			}
		}
	}
	run := domain.ActionRun{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ActionRunKind}, Metadata: domain.ObjectMeta{ID: runID, Name: plan.Metadata.Name}, Spec: domain.ActionRunSpec{ActionPlanID: plan.Metadata.ID, ActionPlanDigest: digest, ProjectID: plan.Spec.ProjectID, RepositoryID: plan.Spec.RepositoryID, WorktreeID: plan.Spec.WorktreeID, Holder: admission.Lock.Holder, ExecutionContext: plan.Spec.ExecutionContext, StartedAt: started, CompletedAt: completed, Status: status, ExitCode: result.ExitCode, Stdout: b.maskOutput(result.Stdout, command.EnvironmentAllowlist), Stderr: b.maskOutput(result.Stderr, command.EnvironmentAllowlist), Prechecks: prechecks, Postchecks: postchecks}}
	eventType := "completed"
	if status != domain.ActionRunSucceeded {
		eventType = string(status)
	}
	if err := b.store.SaveActionRunAndEvent(ctx, run, b.event(plan, eventType, domain.Actor{Kind: domain.ActorSystem, ID: admission.Lock.Holder}, completed, runID), completed); err != nil {
		return domain.ActionRun{}, err
	}
	return run, terminalErr
}

func (b *Broker) saveRejectedRun(ctx context.Context, plan domain.ActionPlan, holder string, at time.Time, status string, _ error) (domain.ActionRun, error) {
	digest, err := plan.Digest()
	if err != nil {
		return domain.ActionRun{}, err
	}
	runID := actionRunID(plan.Metadata.ID, holder, at)
	runStatus := domain.ActionRunStatus(status)
	run := domain.ActionRun{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ActionRunKind}, Metadata: domain.ObjectMeta{ID: runID, Name: plan.Metadata.Name}, Spec: domain.ActionRunSpec{ActionPlanID: plan.Metadata.ID, ActionPlanDigest: digest, ProjectID: plan.Spec.ProjectID, RepositoryID: plan.Spec.RepositoryID, WorktreeID: plan.Spec.WorktreeID, Holder: holder, ExecutionContext: plan.Spec.ExecutionContext, StartedAt: at, CompletedAt: at, Status: runStatus, Prechecks: []domain.ActionEvidence{{ID: "worktree-identity", Kind: domain.EvidenceWorktreeIdentity, Passed: false, Detail: "current Worktree evidence did not satisfy the execution contract"}, {ID: "worktree-head", Kind: domain.EvidenceWorktreeHead, Passed: false, Detail: "current HEAD evidence did not satisfy the execution contract"}}, Postchecks: []domain.ActionEvidence{{ID: "process-exit", Kind: domain.EvidenceProcessExit, Passed: false, Detail: "process was not started"}}}}
	if err := b.store.SaveActionRunAndEvent(ctx, run, b.event(plan, status, domain.Actor{Kind: domain.ActorSystem, ID: holder}, at, runID), at); err != nil {
		return domain.ActionRun{}, err
	}
	return run, ErrActionPrecheck
}

func (b *Broker) maskOutput(value string, names []string) string {
	if b.masker != nil {
		value = b.masker.Mask(value)
	}
	secrets := make([]string, 0, len(names))
	for _, name := range names {
		if secret, ok := os.LookupEnv(name); ok {
			secrets = append(secrets, secret)
		}
	}
	return masking.New(secrets, nil).Mask(value)
}

func actionRunID(planID, holder string, at time.Time) string {
	sum := sha256.Sum256([]byte(planID + "\x00" + holder + "\x00" + at.UTC().Format(time.RFC3339Nano)))
	return "action-run-" + hex.EncodeToString(sum[:])[:48]
}

func (b *Broker) audit(ctx context.Context, plan domain.ActionPlan, eventType string, actor domain.Actor, at time.Time, nonce string) error {
	return b.store.SaveActionEvent(ctx, b.event(plan, eventType, actor, at, nonce))
}

func (b *Broker) event(plan domain.ActionPlan, eventType string, actor domain.Actor, at time.Time, nonce string) domain.ActionEvent {
	digest, _ := plan.Digest()
	sum := sha256.Sum256([]byte(plan.Metadata.ID + "\x00" + eventType + "\x00" + actor.ID + "\x00" + nonce))
	id := "action-" + hex.EncodeToString(sum[:])[:57]
	return domain.ActionEvent{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ActionEventKind}, Metadata: domain.ObjectMeta{ID: id, Name: eventType}, Spec: domain.ActionEventSpec{ActionPlanID: plan.Metadata.ID, ActionPlanDigest: digest, EventType: eventType, Actor: actor, OccurredAt: at}}
}

func scope(plan domain.ActionPlan) string {
	return plan.Spec.ProjectID + "/" + plan.Spec.RepositoryID + "/" + plan.Spec.WorktreeID
}
