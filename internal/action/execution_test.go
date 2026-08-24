package action

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

type fakeProcessRunner struct {
	result     ProcessResult
	err        error
	called     int
	directory  string
	executable string
	args       []string
}

func (r *fakeProcessRunner) Run(_ context.Context, executable string, args []string, _ []string, directory string, _ time.Duration, _ int) (ProcessResult, error) {
	r.called++
	r.executable = executable
	r.args = append([]string(nil), args...)
	r.directory = directory
	return r.result, r.err
}

func TestBrokerExecuteUsesOnlyThePersistedTypedContract(t *testing.T) {
	broker, persistence, _ := actionFixture(t)
	fake := &fakeProcessRunner{result: ProcessResult{ExitCode: 0, Stdout: "ok"}}
	broker.runner = fake
	ctx := context.Background()
	plan, err := broker.Plan(ctx, PlanRequest{ID: "refresh", Name: "Refresh", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "repository.refresh", RequestedBy: domain.Actor{Kind: domain.ActorAgent, ID: "agent"}})
	if err != nil {
		t.Fatal(err)
	}
	admission, err := broker.Admit(ctx, plan.Metadata.ID, "runner", "refresh-request")
	if err != nil {
		t.Fatal(err)
	}
	run, err := broker.Execute(ctx, admission)
	if err != nil || run.Spec.Status != domain.ActionRunSucceeded {
		t.Fatalf("run = %#v, err = %v", run, err)
	}
	if fake.called != 1 || fake.executable != "git" || len(fake.args) != 2 || fake.args[0] != "fetch" || fake.directory != "C:/fixture" {
		t.Fatalf("typed process contract = %#v", fake)
	}
	if len(run.Spec.Prechecks) != 2 || !run.Spec.Prechecks[0].Passed || len(run.Spec.Postchecks) != 1 || !run.Spec.Postchecks[0].Passed {
		t.Fatalf("evidence = %#v", run.Spec)
	}
	stored, err := persistence.GetActionRun(ctx, run.Metadata.ID)
	if err != nil || stored.Spec.Stdout != "ok" {
		t.Fatalf("stored run = %#v, %v", stored, err)
	}
	events, err := persistence.ListActionEvents(ctx, plan.Metadata.ID)
	if err != nil || len(events) != 4 {
		t.Fatalf("execution audit = %#v, %v", events, err)
	}
}

func TestBrokerExecuteFailsClosedBeforeStartingWhenWorktreeChanges(t *testing.T) {
	broker, persistence, now := actionFixture(t)
	fake := &fakeProcessRunner{result: ProcessResult{ExitCode: 0}}
	broker.runner = fake
	ctx := context.Background()
	plan, err := broker.Plan(ctx, PlanRequest{ID: "stale", Name: "Refresh", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "repository.refresh", RequestedBy: domain.Actor{Kind: domain.ActorAgent, ID: "agent"}})
	if err != nil {
		t.Fatal(err)
	}
	admission, err := broker.Admit(ctx, plan.Metadata.ID, "runner", "stale-request")
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := persistence.GetWorktree(ctx, "project", "repo", "primary")
	if err != nil {
		t.Fatal(err)
	}
	worktree.Spec.Head = "changed"
	worktree.Spec.LastObserved = now.Add(time.Minute)
	if err := persistence.ReplaceWorktrees(ctx, "project", "repo", []domain.Worktree{worktree}, true); err != nil {
		t.Fatal(err)
	}
	run, err := broker.Execute(ctx, admission)
	if !errors.Is(err, ErrActionPrecheck) || run.Spec.Status != domain.ActionRunPrecheckFailed || fake.called != 0 {
		t.Fatalf("stale execution = %#v, err = %v, calls = %d", run, err, fake.called)
	}
}

func TestBrokerExecuteRecordsFailureAndMasksOutput(t *testing.T) {
	broker, persistence, _ := actionFixture(t)
	fake := &fakeProcessRunner{result: ProcessResult{ExitCode: 7, Stdout: "TOKEN=secret-value"}, err: errors.New("exit status 7")}
	broker.runner = fake
	ctx := context.Background()
	plan, err := broker.Plan(ctx, PlanRequest{ID: "failed", Name: "Refresh", ProjectID: "project", RepositoryID: "repo", WorktreeID: "primary", ActionType: "repository.refresh", RequestedBy: domain.Actor{Kind: domain.ActorAgent, ID: "agent"}})
	if err != nil {
		t.Fatal(err)
	}
	admission, err := broker.Admit(ctx, plan.Metadata.ID, "runner", "failed-request")
	if err != nil {
		t.Fatal(err)
	}
	run, err := broker.Execute(ctx, admission)
	if !errors.Is(err, ErrActionExecution) || run.Spec.Status != domain.ActionRunFailed {
		t.Fatalf("failed execution = %#v, err = %v", run, err)
	}
	stored, err := persistence.GetActionRun(ctx, run.Metadata.ID)
	if err != nil || stored.Spec.Stdout == "TOKEN=secret-value" {
		t.Fatalf("unmasked action output = %#v, %v", stored, err)
	}
}
