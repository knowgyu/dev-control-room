// Package action contains the policy-only Action Broker core. It records and
// admits reviewed actions; it never starts a process or mutates a target.
package action

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/store"
)

var (
	ErrPolicyDenied        = errors.New("action policy denies this plan")
	ErrApprovalRequired    = errors.New("action requires a current human approval")
	ErrLockConflict        = errors.New("action target is locked or expired")
	ErrIdempotencyConflict = errors.New("action idempotency key was already used")
	ErrUnknownAction       = errors.New("action type is not reviewed")
)

const leaseDuration = 5 * time.Minute

type Broker struct {
	store *store.Store
	now   func() time.Time
}

func New(persistence *store.Store, now func() time.Time) (*Broker, error) {
	if persistence == nil {
		return nil, errors.New("action persistence is required")
	}
	if now == nil {
		now = time.Now
	}
	return &Broker{store: persistence, now: now}, nil
}

// PlanRequest deliberately excludes risk and policy. Those fields are derived
// from the server-owned reviewed action definition.
type PlanRequest struct {
	ID, Name, ProjectID, RepositoryID, WorktreeID, ActionType string
	Inputs                                                    map[string]string
	RequestedBy                                               domain.Actor
}

func (b *Broker) Plan(ctx context.Context, request PlanRequest) (domain.ActionPlan, error) {
	definition, ok := domain.ActionDefinitionFor(request.ActionType)
	if !ok {
		return domain.ActionPlan{}, ErrUnknownAction
	}
	now := b.now().UTC()
	plan := domain.ActionPlan{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ActionPlanKind}, Metadata: domain.ObjectMeta{ID: request.ID, Name: request.Name}, Spec: domain.ActionPlanSpec{ProjectID: request.ProjectID, RepositoryID: request.RepositoryID, WorktreeID: request.WorktreeID, ActionType: definition.ActionType, Risk: definition.Risk, Inputs: request.Inputs, PolicyDecision: definition.PolicyDecision, ApprovalRequired: definition.ApprovalRequired, RequestedBy: request.RequestedBy, RequestedAt: now}}
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
	return persisted, nil
}

// GrantHumanApproval constructs the approving actor itself, so request data
// cannot designate an agent as human.
func (b *Broker) GrantHumanApproval(ctx context.Context, planID, approvalID, reason string, expiresAt *time.Time) (domain.Approval, error) {
	plan, err := b.store.GetActionPlan(ctx, planID)
	if err != nil {
		return domain.Approval{}, err
	}
	now := b.now().UTC()
	digest, err := plan.Digest()
	if err != nil {
		return domain.Approval{}, fmt.Errorf("digest action plan: %w", err)
	}
	human := domain.Actor{Kind: domain.ActorHuman, ID: "local-user"}
	approval := domain.Approval{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ApprovalKind}, Metadata: domain.ObjectMeta{ID: approvalID, Name: "Human action approval"}, Spec: domain.ApprovalSpec{ActionPlanID: planID, ActionPlanDigest: digest, Status: domain.ApprovalGranted, RequestedBy: plan.Spec.RequestedBy, ApprovedBy: &human, Reason: reason, ExpiresAt: expiresAt, DecidedAt: now}}
	if err := b.store.SaveApprovalAndActionEvent(ctx, approval, b.event(plan, "approval_granted", human, now, approvalID), now); err != nil {
		return domain.Approval{}, err
	}
	return approval, nil
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
