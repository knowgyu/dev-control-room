package action

import (
	"context"
	"errors"
	"path/filepath"
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
	expires := now.Add(20 * time.Minute)
	approval, err := broker.GrantHumanApproval(ctx, plan.Metadata.ID, "approval", "reviewed", &expires)
	if err != nil {
		t.Fatal(err)
	}
	if approval.Spec.ApprovedBy == nil || approval.Spec.ApprovedBy.Kind != domain.ActorHuman || approval.Spec.DecidedAt != *now {
		t.Fatalf("approval was not trusted human decision: %#v", approval.Spec)
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

func TestBrokerExpiryAndHolderBoundRenewal(t *testing.T) {
	broker, _, now := actionFixture(t)
	ctx := context.Background()
	plan, err := broker.Plan(ctx, PlanRequest{ID: "plan", Name: "Production", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "abc"}, RequestedBy: domain.Actor{Kind: domain.ActorHuman, ID: "local-user"}})
	if err != nil {
		t.Fatal(err)
	}
	expires := now.Add(20 * time.Minute)
	if _, err := broker.GrantHumanApproval(ctx, plan.Metadata.ID, "approval", "reviewed", &expires); err != nil {
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
	expires := now.Add(time.Hour)
	if _, err := broker.GrantHumanApproval(ctx, plan.Metadata.ID, "approval-a", "reviewed", &expires); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.GrantHumanApproval(ctx, plan.Metadata.ID, "approval-b", "reviewed", &expires); err != nil {
		t.Fatal(err)
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
	approvalEvent := broker.event(plan64, "approval_granted", domain.Actor{Kind: domain.ActorHuman, ID: "local-user"}, *now, "approval-atomic")
	if err := persistence.SaveActionEvent(ctx, approvalEvent); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.GrantHumanApproval(ctx, plan64.Metadata.ID, "approval-atomic", "reviewed", &expires); err == nil {
		t.Fatal("audit conflict committed approval")
	}
	approvals, err := persistence.ListApprovals(ctx, plan64.Metadata.ID)
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
	expires := now.Add(time.Hour)
	if _, err := broker.GrantHumanApproval(ctx, plan.Metadata.ID, "approval", "reviewed", &expires); err != nil {
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
	worktree := domain.Worktree{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.WorktreeKind}, Metadata: domain.ObjectMeta{ID: "primary", Name: "Primary"}, Spec: domain.WorktreeSpec{ProjectID: "project", RepositoryID: "repo", CanonicalPath: "C:/fixture", PathFingerprint: "sha256:path", Trust: domain.WorktreeTrustVerifiedReadOnly, Primary: true, LastObserved: now}}
	if err := persistence.ReplaceWorktrees(ctx, "project", "repo", []domain.Worktree{worktree}, true); err != nil {
		t.Fatal(err)
	}
	broker, err := New(persistence, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	return broker, persistence, &now
}

func ptr(value time.Time) *time.Time { return &value }
