package action

import (
	"context"
	"database/sql"
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
