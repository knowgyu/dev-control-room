package action

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/store"
)

func TestBrokerDerivesProtectedPolicyAndPersistsAudit(t *testing.T) {
	broker, persistence, now := actionFixture(t)
	ctx := context.Background()
	plan, err := broker.Plan(ctx, PlanRequest{ID: "plan", Name: "Production", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "abc"}, RequestedBy: domain.Actor{Kind: domain.ActorAgent, ID: "agent"}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Spec.Risk != domain.RiskHighImpact || plan.Spec.PolicyDecision != domain.PolicyApprovalRequired || !plan.Spec.ApprovalRequired {
		t.Fatalf("forged low-risk production plan survived: %#v", plan.Spec)
	}
	if _, err := broker.Plan(ctx, PlanRequest{ID: "unknown", Name: "Unknown", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "abc", "risk": "safe_local"}, RequestedBy: domain.Actor{Kind: domain.ActorAgent, ID: "agent"}}); err == nil {
		t.Fatalf("forged production input = %v", err)
	}
	if _, err := broker.Plan(ctx, PlanRequest{ID: "unreviewed", Name: "Unreviewed", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "release.production.fast", Inputs: map[string]string{"commit": "abc"}, RequestedBy: domain.Actor{Kind: domain.ActorAgent, ID: "agent"}}); !errors.Is(err, ErrUnknownAction) {
		t.Fatalf("unknown action = %v", err)
	}
	result, err := broker.StartHumanApprovalCeremony(ctx, plan.Metadata.ID)
	if err != nil || result.Decision != HumanDecisionGrant {
		t.Fatalf("ceremony = %#v, %v", result, err)
	}
	approvals, err := persistence.ListApprovals(ctx, plan.Metadata.ID)
	if err != nil || len(approvals) != 1 || approvals[0].Spec.ApprovedBy == nil || approvals[0].Spec.ApprovedBy.Kind != domain.ActorHuman || approvals[0].Spec.DecidedAt != *now {
		t.Fatalf("approval was not trusted human decision: %#v, %v", approvals, err)
	}
	events, err := persistence.ListActionEvents(ctx, plan.Metadata.ID)
	if err != nil || len(events) != 2 {
		t.Fatalf("audit events = %#v, %v", events, err)
	}
	byType := map[string]domain.ActionEvent{}
	for _, event := range events {
		byType[event.Spec.EventType] = event
	}
	if byType["planned"].Spec.Actor.Kind != domain.ActorAgent || byType["approval_granted"].Spec.Actor.Kind != domain.ActorHuman {
		t.Fatalf("audit actor records = %#v", events)
	}
	if err := persistence.SaveActionEvent(ctx, byType["planned"]); !errors.Is(err, store.ErrActionEventImmutable) {
		t.Fatalf("audit event was mutable: %v", err)
	}
}

func TestBrokerRejectsAgentApprovalAndRequiresExactWorktree(t *testing.T) {
	broker, persistence, now := actionFixture(t)
	ctx := context.Background()
	if _, err := broker.Plan(ctx, PlanRequest{ID: "missing-worktree", Name: "Missing", ProjectID: "project", RepositoryID: "repo", ActionType: "release.production", Inputs: map[string]string{"commit": "abc"}, RequestedBy: domain.Actor{Kind: domain.ActorAgent, ID: "agent"}}); err == nil {
		t.Fatal("action plan without exact worktree was accepted")
	}
	plan, err := broker.Plan(ctx, PlanRequest{ID: "plan", Name: "Production", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "abc"}, RequestedBy: domain.Actor{Kind: domain.ActorAgent, ID: "agent"}})
	if err != nil {
		t.Fatal(err)
	}
	digest, _ := plan.Digest()
	agent := domain.Actor{Kind: domain.ActorAgent, ID: "agent"}
	forged := domain.Approval{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ApprovalKind}, Metadata: domain.ObjectMeta{ID: "forged", Name: "Forged"}, Spec: domain.ApprovalSpec{ActionPlanID: plan.Metadata.ID, ActionPlanDigest: digest, Status: domain.ApprovalGranted, RequestedBy: agent, ApprovedBy: &agent, ExpiresAt: ptr(now.Add(time.Minute)), DecidedAt: *now}}
	if err := persistence.SaveApprovalAt(ctx, forged, *now); err == nil {
		t.Fatal("agent approval persisted")
	}
}

func TestBrokerReusesActionPlanWhenOnlyRequestedAtChanges(t *testing.T) {
	broker, _, now := actionFixture(t)
	ctx := context.Background()
	request := PlanRequest{ID: "repeatable-plan", Name: "Production", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "abc"}, RequestedBy: domain.Actor{Kind: domain.ActorAgent, ID: "agent"}}
	first, err := broker.Plan(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Minute)
	second, err := broker.Plan(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if second.Metadata.ID != first.Metadata.ID || !second.Spec.RequestedAt.Equal(first.Spec.RequestedAt) {
		t.Fatalf("repeated plan was not reused: first=%#v second=%#v", first, second)
	}
}

func TestBrokerRepairsMissingPlannedAuditOnEquivalentRetry(t *testing.T) {
	broker, persistence, now := actionFixture(t)
	ctx := context.Background()
	request := PlanRequest{ID: "recoverable-plan", Name: "Production", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "abc"}, RequestedBy: domain.Actor{Kind: domain.ActorAgent, ID: "agent"}}
	if _, err := persistence.DB().Exec(`
CREATE TRIGGER fail_planned_audit_once
BEFORE INSERT ON action_events
WHEN NEW.event_type = 'planned'
BEGIN
SELECT RAISE(ABORT, 'injected planned audit failure');
END`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = persistence.DB().Exec(`DROP TRIGGER IF EXISTS fail_planned_audit_once`)
	})

	first, err := broker.Plan(ctx, request)
	if err == nil {
		t.Fatal("initial plan unexpectedly succeeded despite planned audit failure")
	}
	if first.Metadata.ID != "" {
		t.Fatalf("failed plan returned persisted data: %#v", first)
	}
	persisted, err := persistence.GetActionPlan(ctx, request.ID)
	if err != nil {
		t.Fatalf("persisted plan after audit failure: %v", err)
	}
	events, err := persistence.ListActionEvents(ctx, request.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("planned audit was written despite injected failure: %#v", events)
	}

	if _, err := persistence.DB().Exec(`DROP TRIGGER fail_planned_audit_once`); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Minute)
	retried, err := broker.Plan(ctx, request)
	if err != nil {
		t.Fatalf("equivalent retry did not repair the planned audit: %v", err)
	}
	if !reflect.DeepEqual(retried, persisted) {
		t.Fatalf("equivalent retry changed the immutable plan: first=%#v retry=%#v", persisted, retried)
	}
	events, err = persistence.ListActionEvents(ctx, request.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Spec.EventType != "planned" || !events[0].Spec.OccurredAt.Equal(persisted.Spec.RequestedAt) {
		t.Fatalf("repaired planned audit = %#v", events)
	}

	if _, err := broker.Plan(ctx, request); err != nil {
		t.Fatalf("idempotent retry after repair failed: %v", err)
	}
	events, err = persistence.ListActionEvents(ctx, request.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("equivalent retries created duplicate planned audits: %#v", events)
	}
}

func TestBrokerRejectsConflictingPlannedAuditOnEquivalentRetry(t *testing.T) {
	broker, persistence, _ := actionFixture(t)
	ctx := context.Background()
	request := PlanRequest{ID: "conflicting-plan", Name: "Production", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "abc"}, RequestedBy: domain.Actor{Kind: domain.ActorAgent, ID: "agent"}}
	if _, err := persistence.DB().Exec(`
CREATE TRIGGER fail_planned_audit
BEFORE INSERT ON action_events
WHEN NEW.event_type = 'planned'
BEGIN
SELECT RAISE(ABORT, 'injected planned audit failure');
END`); err != nil {
		t.Fatal(err)
	}
	first, err := broker.Plan(ctx, request)
	if err == nil {
		t.Fatal("initial plan unexpectedly succeeded despite planned audit failure")
	}
	if first.Metadata.ID != "" {
		t.Fatalf("failed plan returned persisted data: %#v", first)
	}
	if _, err := persistence.DB().Exec(`DROP TRIGGER fail_planned_audit`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = persistence.DB().Exec(`DROP TRIGGER IF EXISTS fail_planned_audit`)
	})

	persisted, err := persistence.GetActionPlan(ctx, request.ID)
	if err != nil {
		t.Fatal(err)
	}
	conflict := broker.event(persisted, "planned", persisted.Spec.RequestedBy, persisted.Spec.RequestedAt.Add(time.Minute), persisted.Metadata.ID)
	if err := persistence.SaveActionEvent(ctx, conflict); err != nil {
		t.Fatal(err)
	}

	if _, err := broker.Plan(ctx, request); !errors.Is(err, store.ErrActionEventImmutable) {
		t.Fatalf("conflicting planned audit was accepted: %v", err)
	}
	events, err := persistence.ListActionEvents(ctx, request.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || !events[0].Spec.OccurredAt.Equal(conflict.Spec.OccurredAt) {
		t.Fatalf("conflicting planned audit was overwritten or duplicated: %#v", events)
	}
}

func TestBrokerDoesNotDuplicateLegacyPlannedAuditOnEquivalentRetry(t *testing.T) {
	broker, persistence, _ := actionFixture(t)
	ctx := context.Background()
	request := PlanRequest{ID: "legacy-planned-event-plan", Name: "Production", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "abc"}, RequestedBy: domain.Actor{Kind: domain.ActorAgent, ID: "agent"}}
	if _, err := persistence.DB().Exec(`
CREATE TRIGGER fail_planned_audit
BEFORE INSERT ON action_events
WHEN NEW.event_type = 'planned'
BEGIN
SELECT RAISE(ABORT, 'injected planned audit failure');
END`); err != nil {
		t.Fatal(err)
	}
	first, err := broker.Plan(ctx, request)
	if err == nil {
		t.Fatal("initial plan unexpectedly succeeded despite planned audit failure")
	}
	if first.Metadata.ID != "" {
		t.Fatalf("failed plan returned persisted data: %#v", first)
	}
	if _, err := persistence.DB().Exec(`DROP TRIGGER fail_planned_audit`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = persistence.DB().Exec(`DROP TRIGGER IF EXISTS fail_planned_audit`)
	})

	persisted, err := persistence.GetActionPlan(ctx, request.ID)
	if err != nil {
		t.Fatal(err)
	}
	legacy := broker.event(persisted, "planned", persisted.Spec.RequestedBy, persisted.Spec.RequestedAt, persisted.Metadata.ID)
	legacy.Metadata.ID = "legacy-planned-event"
	if err := persistence.SaveActionEvent(ctx, legacy); err != nil {
		t.Fatal(err)
	}

	if _, err := broker.Plan(ctx, request); !errors.Is(err, store.ErrActionEventImmutable) {
		t.Fatalf("legacy planned audit was duplicated: %v", err)
	}
	events, err := persistence.ListActionEvents(ctx, request.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Metadata.ID != legacy.Metadata.ID {
		t.Fatalf("legacy planned audit was overwritten or duplicated: %#v", events)
	}
}

func TestBrokerRejectsLegacyQualityInstallPlanBeforeApprovalOrExecution(t *testing.T) {
	broker, persistence, now := actionFixture(t)
	ctx := context.Background()
	modern, err := broker.Plan(ctx, PlanRequest{
		ID:           "modern-quality-install",
		Name:         "Quality install",
		ProjectID:    "project",
		RepositoryID: "repo",
		WorktreeID:   "primary",
		ActionType:   domain.QualityToolInstallPythonAction,
		Inputs: map[string]string{
			"package":          "ruff",
			"version":          "1.0.0",
			"componentId":      "component-root",
			"componentRoot":    "C:/fixture",
			"executable":       "C:/fixture/python.exe",
			"environmentScope": "project",
			"affectedFiles":    `["pyproject.toml"]`,
		},
		RequestedBy: domain.Actor{Kind: domain.ActorAgent, ID: "agent"},
		ToolVersion: "ruff@1.0.0", ToolConfigDigest: "sha256:" + strings.Repeat("a", 64),
		WritablePaths: []string{"C:/fixture/pyproject.toml"},
	})
	if err != nil {
		t.Fatal(err)
	}

	legacy := modern
	legacy.Metadata.ID = "legacy-quality-install"
	legacy.Metadata.Name = "Legacy quality install"
	legacy.Spec.Inputs = map[string]string{"package": "ruff", "version": "1.0.0"}
	legacy.Spec.Execution.WorkingDirectory = ""
	legacyObject, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	legacyDigest := "sha256:" + strings.Repeat("0", 64)
	if _, err := persistence.DB().Exec(`
INSERT INTO action_plans(
    id, project_id, repository_id, worktree_id, action_type, risk, policy_decision,
    digest, execution_context_digest, created_at, requester_kind, requester_id,
    requested_at, object_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		legacy.Metadata.ID,
		legacy.Spec.ProjectID,
		legacy.Spec.RepositoryID,
		legacy.Spec.WorktreeID,
		legacy.Spec.ActionType,
		legacy.Spec.Risk,
		legacy.Spec.PolicyDecision,
		legacyDigest,
		"sha256:"+strings.Repeat("0", 64),
		now.UTC().Format(time.RFC3339Nano),
		legacy.Spec.RequestedBy.Kind,
		legacy.Spec.RequestedBy.ID,
		legacy.Spec.RequestedAt.UTC().Format(time.RFC3339Nano),
		legacyObject,
	); err != nil {
		t.Fatal(err)
	}

	if _, err := broker.StartHumanApprovalCeremony(ctx, legacy.Metadata.ID); !errors.Is(err, ErrActionPlanStale) {
		t.Fatalf("legacy plan approval error = %v, want %v", err, ErrActionPlanStale)
	}
	status, err := broker.Status(ctx, legacy.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Admission != AdmissionStale {
		t.Fatalf("legacy plan admission = %q, want %q", status.Admission, AdmissionStale)
	}
	if _, err := broker.Admit(ctx, legacy.Metadata.ID, "runner", "legacy-quality-install"); !errors.Is(err, ErrActionPlanStale) {
		t.Fatalf("legacy plan admission error = %v, want %v", err, ErrActionPlanStale)
	}
}

func TestExecuteLegacyQualityInstallReleasesPreUpgradeAdmissionLock(t *testing.T) {
	broker, persistence, now := actionFixture(t)
	ctx := context.Background()
	modern, err := broker.Plan(ctx, PlanRequest{
		ID:           "modern-quality-install-execute",
		Name:         "Quality install",
		ProjectID:    "project",
		RepositoryID: "repo",
		WorktreeID:   "primary",
		ActionType:   domain.QualityToolInstallPythonAction,
		Inputs: map[string]string{
			"package":          "ruff",
			"version":          "1.0.0",
			"componentId":      "component-root",
			"componentRoot":    "C:/fixture",
			"executable":       "C:/fixture/python.exe",
			"environmentScope": "project",
			"affectedFiles":    `["pyproject.toml"]`,
		},
		RequestedBy: domain.Actor{Kind: domain.ActorAgent, ID: "agent"},
		ToolVersion: "ruff@1.0.0", ToolConfigDigest: "sha256:" + strings.Repeat("a", 64),
		WritablePaths: []string{"C:/fixture/pyproject.toml"},
	})
	if err != nil {
		t.Fatal(err)
	}

	legacy := modern
	legacy.Metadata.ID = "legacy-quality-install-execute"
	legacy.Metadata.Name = "Legacy quality install"
	legacy.Spec.Inputs = map[string]string{"package": "ruff", "version": "1.0.0"}
	legacy.Spec.Execution.WorkingDirectory = ""
	legacyObject, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	legacyDigest := "sha256:" + strings.Repeat("0", 64)
	if _, err := persistence.DB().Exec(`
INSERT INTO action_plans(
    id, project_id, repository_id, worktree_id, action_type, risk, policy_decision,
    digest, execution_context_digest, created_at, requester_kind, requester_id,
    requested_at, object_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		legacy.Metadata.ID,
		legacy.Spec.ProjectID,
		legacy.Spec.RepositoryID,
		legacy.Spec.WorktreeID,
		legacy.Spec.ActionType,
		legacy.Spec.Risk,
		legacy.Spec.PolicyDecision,
		legacyDigest,
		"sha256:"+strings.Repeat("0", 64),
		now.UTC().Format(time.RFC3339Nano),
		legacy.Spec.RequestedBy.Kind,
		legacy.Spec.RequestedBy.ID,
		legacy.Spec.RequestedAt.UTC().Format(time.RFC3339Nano),
		legacyObject,
	); err != nil {
		t.Fatal(err)
	}

	lock := store.ActionLock{
		Scope:            scope(legacy),
		ActionPlanID:     legacy.Metadata.ID,
		ActionPlanDigest: legacyDigest,
		Holder:           "pre-upgrade-runner",
		ExpiresAt:        now.Add(time.Minute),
	}
	if err := persistence.AcquireActionLock(ctx, lock, *now); err != nil {
		t.Fatal(err)
	}

	admission := Admission{Plan: legacy, Lock: lock}
	if _, err := broker.Execute(ctx, admission); !errors.Is(err, ErrActionPlanStale) {
		t.Fatalf("legacy execution error = %v, want %v", err, ErrActionPlanStale)
	}

	replacement := lock
	replacement.Holder = "post-upgrade-runner"
	if err := persistence.AcquireActionLock(ctx, replacement, *now); err != nil {
		t.Fatalf("pre-upgrade admission lock was not released: %v", err)
	}
}

func TestBrokerDeniesUntrustedOrChangedWorktreeBeforeFutureExecution(t *testing.T) {
	broker, persistence, now := actionFixture(t)
	ctx := context.Background()
	plan, err := broker.Plan(ctx, PlanRequest{ID: "plan-trust", Name: "Production", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "abc"}, RequestedBy: domain.Actor{Kind: domain.ActorHuman, ID: "local-user"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broker.StartHumanApprovalCeremony(ctx, plan.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := persistence.DB().Exec(`DELETE FROM worktree_execution_trusts WHERE project_id='project' AND repository_id='repo' AND worktree_id='primary'`); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.Admit(ctx, plan.Metadata.ID, "holder", "request-trust"); !errors.Is(err, ErrWorktreeUntrusted) {
		t.Fatalf("untrusted worktree admission = %v", err)
	}
	if _, err := persistence.TrustWorktreeForExecution(ctx, "project", "repo", "primary", *now); err != nil {
		t.Fatal(err)
	}
	worktree, err := persistence.GetWorktree(ctx, "project", "repo", "primary")
	if err != nil {
		t.Fatal(err)
	}
	worktree.Spec.Head = "def456"
	worktree.Spec.LastObserved = worktree.Spec.LastObserved.Add(time.Minute)
	if err := persistence.ReplaceWorktrees(ctx, "project", "repo", []domain.Worktree{worktree}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.Admit(ctx, plan.Metadata.ID, "holder", "request-stale"); !errors.Is(err, ErrExecutionContextStale) {
		t.Fatalf("changed worktree admission = %v", err)
	}
}

func TestBrokerExpiryAndHolderBoundRenewal(t *testing.T) {
	broker, _, now := actionFixture(t)
	ctx := context.Background()
	plan, err := broker.Plan(ctx, PlanRequest{ID: "plan", Name: "Production", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "abc"}, RequestedBy: domain.Actor{Kind: domain.ActorHuman, ID: "local-user"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broker.StartHumanApprovalCeremony(ctx, plan.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	admission, err := broker.Admit(ctx, plan.Metadata.ID, "holder-a", "request-a")
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(4 * time.Minute)
	renewed, err := broker.Renew(ctx, admission)
	if err != nil || !renewed.Lock.ExpiresAt.After(admission.Lock.ExpiresAt) {
		t.Fatalf("renewal = %#v, %v", renewed, err)
	}
	tampered := renewed
	tampered.Lock.Holder = "holder-b"
	if _, err := broker.Renew(ctx, tampered); !errors.Is(err, ErrLockConflict) {
		t.Fatalf("holder bypass renewed lock: %v", err)
	}
	*now = now.Add(17 * time.Minute)
	if _, err := broker.Admit(ctx, plan.Metadata.ID, "holder-c", "request-c"); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("expired approval admitted: %v", err)
	}
}

func TestAuditIDsAreUniqueAndApprovalAuditIsAtomic(t *testing.T) {
	broker, persistence, now := actionFixture(t)
	ctx := context.Background()
	plan, err := broker.Plan(ctx, PlanRequest{ID: strings.Repeat("p", 48), Name: "Production", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "abc"}, RequestedBy: domain.Actor{Kind: domain.ActorHuman, ID: "local-user"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broker.StartHumanApprovalCeremony(ctx, plan.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.StartHumanApprovalCeremony(ctx, plan.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	approvals, err := persistence.ListApprovals(ctx, plan.Metadata.ID)
	if err != nil || len(approvals) != 2 || approvals[0].Metadata.ID == approvals[1].Metadata.ID {
		t.Fatalf("same-tick approvals = %#v, %v", approvals, err)
	}
	events, err := persistence.ListActionEvents(ctx, plan.Metadata.ID)
	if err != nil || len(events) != 3 {
		t.Fatalf("events = %#v, %v", events, err)
	}
	seen := map[string]bool{}
	for _, event := range events {
		if len(event.Metadata.ID) > 64 || seen[event.Metadata.ID] {
			t.Fatalf("invalid event ID %q", event.Metadata.ID)
		}
		seen[event.Metadata.ID] = true
	}
	plan64, err := broker.Plan(ctx, PlanRequest{ID: strings.Repeat("q", 64), Name: "Production", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "def"}, RequestedBy: domain.Actor{Kind: domain.ActorHuman, ID: "local-user"}})
	if err != nil {
		t.Fatal(err)
	}
	events, err = persistence.ListActionEvents(ctx, plan64.Metadata.ID)
	if err != nil || len(events) != 1 || len(events[0].Metadata.ID) > 64 {
		t.Fatalf("64-char plan audit = %#v, %v", events, err)
	}
	approvalID := decisionID(plan64.Metadata.ID, HumanDecisionGrant, *now, 0)
	approvalEvent := broker.event(plan64, "approval_granted", domain.Actor{Kind: domain.ActorHuman, ID: "local-user"}, *now, approvalID)
	if err := persistence.SaveActionEvent(ctx, approvalEvent); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.StartHumanApprovalCeremony(ctx, plan64.Metadata.ID); err == nil {
		t.Fatal("audit conflict committed approval")
	}
	approvals, err = persistence.ListApprovals(ctx, plan64.Metadata.ID)
	if err != nil || len(approvals) != 0 {
		t.Fatalf("atomic approval rollback = %#v, %v", approvals, err)
	}
}

func TestAdmitAuditFailureReleasesLeaseAndIdempotency(t *testing.T) {
	broker, persistence, now := actionFixture(t)
	ctx := context.Background()
	plan, err := broker.Plan(ctx, PlanRequest{ID: "plan", Name: "Production", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "abc"}, RequestedBy: domain.Actor{Kind: domain.ActorHuman, ID: "local-user"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broker.StartHumanApprovalCeremony(ctx, plan.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	if err := persistence.SaveActionEvent(ctx, broker.event(plan, "admitted", domain.Actor{Kind: domain.ActorSystem, ID: "holder"}, *now, "request")); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.Admit(ctx, plan.Metadata.ID, "holder", "request"); err == nil {
		t.Fatal("admit audit conflict succeeded")
	}
	digest, _ := plan.Digest()
	if err := persistence.ClaimActionIdempotency(ctx, "request", plan.Metadata.ID, digest, *now); err != nil {
		t.Fatalf("idempotency was not released: %v", err)
	}
	lock := store.ActionLock{Scope: scope(plan), ActionPlanID: plan.Metadata.ID, ActionPlanDigest: digest, Holder: "other", ExpiresAt: now.Add(time.Minute)}
	if err := persistence.AcquireActionLock(ctx, lock, *now); err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
}

func TestStatusIsReadOnlyAndReportsNotFound(t *testing.T) {
	broker, persistence, _ := actionFixture(t)
	ctx := context.Background()
	if _, err := broker.Status(ctx, "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing status = %v", err)
	}
	plan, err := broker.Plan(ctx, PlanRequest{ID: "plan", Name: "Production", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "abc"}, RequestedBy: domain.Actor{Kind: domain.ActorAgent, ID: "agent"}})
	if err != nil {
		t.Fatal(err)
	}
	beforeApprovals, _ := persistence.ListApprovals(ctx, plan.Metadata.ID)
	beforeEvents, _ := persistence.ListActionEvents(ctx, plan.Metadata.ID)
	status, err := broker.Status(ctx, plan.Metadata.ID)
	if err != nil || status.Admission != AdmissionApprovalRequired || len(status.Approvals) != len(beforeApprovals) || len(status.Events) != len(beforeEvents) {
		t.Fatalf("read-only status = %#v, %v", status, err)
	}
	afterApprovals, _ := persistence.ListApprovals(ctx, plan.Metadata.ID)
	afterEvents, _ := persistence.ListActionEvents(ctx, plan.Metadata.ID)
	if len(afterApprovals) != len(beforeApprovals) || len(afterEvents) != len(beforeEvents) {
		t.Fatal("status mutated approval or audit state")
	}
}

func actionFixture(t *testing.T) (*Broker, *store.Store, *time.Time) {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	persistence, err := store.New(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := persistence.SaveProject(ctx, domain.NewProject("project", "Project", []domain.Repository{domain.NewRepository("repo", "Repo", "C:/fixture")})); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	worktree := domain.Worktree{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.WorktreeKind}, Metadata: domain.ObjectMeta{ID: "primary", Name: "Primary"}, Spec: domain.WorktreeSpec{ProjectID: "project", RepositoryID: "repo", CanonicalPath: "C:/fixture", PathFingerprint: "sha256:path", Trust: domain.WorktreeTrustVerifiedReadOnly, Primary: true, Head: "abc123", Branch: "main", LastObserved: now}}
	if err := persistence.ReplaceWorktrees(ctx, "project", "repo", []domain.Worktree{worktree}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := persistence.TrustWorktreeForExecution(ctx, "project", "repo", "primary", now); err != nil {
		t.Fatal(err)
	}
	broker, err := New(persistence, func() time.Time { return now }, &fakeHumanDecisionPrompt{decision: HumanDecisionGrant})
	if err != nil {
		t.Fatal(err)
	}
	return broker, persistence, &now
}

func ptr(value time.Time) *time.Time { return &value }

type fakeHumanDecisionPrompt struct {
	decision HumanDecision
	err      error
	request  HumanDecisionRequest
	started  chan struct{}
	release  <-chan struct{}
}

func (p *fakeHumanDecisionPrompt) Decide(_ context.Context, request HumanDecisionRequest) (HumanDecision, error) {
	p.request = request
	if p.started != nil {
		close(p.started)
	}
	if p.release != nil {
		<-p.release
	}
	return p.decision, p.err
}

func TestHumanDecisionCeremonyRejectsCancelAndPromptFailure(t *testing.T) {
	broker, persistence, _ := actionFixture(t)
	ctx := context.Background()
	for _, test := range []struct {
		name     string
		decision HumanDecision
		err      error
		wantErr  bool
	}{
		{name: "reject", decision: HumanDecisionReject},
		{name: "cancel", decision: HumanDecisionCancel},
		{name: "failure", err: errors.New("no desktop"), wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			prompt := &fakeHumanDecisionPrompt{decision: test.decision, err: test.err}
			broker.prompt = prompt
			plan, err := broker.Plan(ctx, PlanRequest{ID: "plan-" + test.name, Name: "Production", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "abc"}, RequestedBy: domain.Actor{Kind: domain.ActorAgent, ID: "agent"}})
			if err != nil {
				t.Fatal(err)
			}
			result, err := broker.StartHumanApprovalCeremony(ctx, plan.Metadata.ID)
			if (err != nil) != test.wantErr {
				t.Fatalf("ceremony = %#v, %v", result, err)
			}
			approvals, listErr := persistence.ListApprovals(ctx, plan.Metadata.ID)
			if listErr != nil {
				t.Fatal(listErr)
			}
			if test.decision == HumanDecisionReject && (len(approvals) != 1 || approvals[0].Spec.Status != domain.ApprovalRejected) {
				t.Fatalf("rejection = %#v", approvals)
			}
			if test.decision != HumanDecisionReject && len(approvals) != 0 {
				t.Fatalf("non-grant persisted approval: %#v", approvals)
			}
			if prompt.request.Digest == "" || prompt.request.Worktree != "primary" || prompt.request.Executable == "" || prompt.request.ExpiresAt.IsZero() {
				t.Fatalf("prompt did not receive server-derived ceremony: %#v", prompt.request)
			}
		})
	}
}

func TestHumanDecisionCeremonyAllowsOnlyOnePrompt(t *testing.T) {
	broker, _, _ := actionFixture(t)
	ctx := context.Background()
	started := make(chan struct{})
	release := make(chan struct{})
	broker.prompt = &fakeHumanDecisionPrompt{decision: HumanDecisionCancel, started: started, release: release}
	plan, err := broker.Plan(ctx, PlanRequest{ID: "plan-single", Name: "Production", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "abc"}, RequestedBy: domain.Actor{Kind: domain.ActorAgent, ID: "agent"}})
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { _, err := broker.StartHumanApprovalCeremony(ctx, plan.Metadata.ID); finished <- err }()
	<-started
	if _, err := broker.StartHumanApprovalCeremony(ctx, plan.Metadata.ID); !errors.Is(err, ErrHumanDecisionInProgress) {
		t.Fatalf("concurrent ceremony = %v", err)
	}
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}
