package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/action"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestChecksetAndActionFailuresEnterSafeguardLearning(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	repository := tempGitRepository(t, "safeguard-sources")
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Safeguards", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	workflow := filepath.Join(repository, ".github", "workflows", "checks.yml")
	if err := os.MkdirAll(filepath.Dir(workflow), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workflow, []byte("jobs:\n  checks:\n    steps:\n      - run: git diff --check\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "bad.txt"), []byte("clean\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, repository, "add", ".github/workflows/checks.yml", "bad.txt")
	gitFixture(t, repository, "commit", "-m", "add failing check fixture")
	discovery, err := service.Discover(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil || len(discovery.Spec.ProposalIDs) != 1 {
		t.Fatalf("discovery = %#v, %v", discovery, err)
	}
	proposal, err := service.ApplyProposal(context.Background(), discovery.Spec.ProposalIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	checkset, err := service.CreateCheckset(context.Background(), CreateChecksetInput{ID: "failing-check", Name: "Failing check", ProposalID: proposal.Metadata.ID, Steps: []domain.CheckStep{{ID: "diff", Name: "Diff", Command: *proposal.Spec.TypedCommand}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyCheckset(context.Background(), checkset.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "bad.txt"), []byte("trailing space \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		run, err := service.RunCheckset(context.Background(), checkset.Metadata.ID)
		if err != nil || run.Spec.Status != domain.CheckFailed {
			t.Fatalf("failed check run = %#v, %v", run, err)
		}
	}

	service.broker, err = action.NewWithRunner(service.store, nil, failingSafeguardActionRunner{})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanAction(context.Background(), ActionPlanInput{ID: "failing-action", Name: "Failing action", ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", ActionType: "repository.refresh"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.TrustActionWorktree(context.Background(), plan.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	for index := range 3 {
		run, err := service.ExecuteAction(context.Background(), plan.Metadata.ID, "ui", "failure-"+string(rune('a'+index)))
		if contract.Classify(err).Code != contract.ErrorExecutionFailed || run.Spec.Status != domain.ActionRunFailed {
			t.Fatalf("failed action run = %#v, %v", run, err)
		}
	}
	rules, err := service.Safeguards(context.Background(), 10)
	if err != nil || len(rules) != 2 {
		t.Fatalf("checkset and action safeguard rules = %#v, %v", rules, err)
	}
}

type failingSafeguardActionRunner struct{}

func (failingSafeguardActionRunner) Run(context.Context, string, []string, []string, string, time.Duration, int) (action.ProcessResult, error) {
	return action.ProcessResult{ExitCode: 7, Stdout: "TOKEN=secret-output"}, errors.New("exit status 7")
}

func TestRepeatedFailureCreatesPersistentRuleAndEvaluatesExactFingerprint(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	home := service.home
	ctx := context.Background()
	projectID, repositoryID := seedSafeguardScope(t, service)
	occurrence := failureOccurrence{Category: "checkset", SourceType: "pre-pr", Status: "failed", ExitCode: 7, ProjectID: projectID, RepositoryID: repositoryID, EvidenceRef: "check-run-1"}
	for range 3 {
		if err := service.recordFailureOccurrence(ctx, occurrence); err != nil {
			t.Fatal(err)
		}
	}
	rules, err := service.Safeguards(ctx, 10)
	if err != nil || len(rules) != 1 || rules[0].Spec.State != domain.SafeguardProposal {
		t.Fatalf("proposal rule = %#v, %v", rules, err)
	}
	rule, err := service.ReviewSafeguard(ctx, rules[0].Metadata.ID, "local-user")
	if err != nil || rule.Spec.State != domain.SafeguardShadow {
		t.Fatalf("shadow rule = %#v, %v", rule, err)
	}
	mismatch := occurrence
	mismatch.SourceType = "other"
	if err := service.recordFailureOccurrence(ctx, mismatch); err != nil {
		t.Fatal(err)
	}
	if err := service.recordFailureOccurrence(ctx, occurrence); err != nil {
		t.Fatal(err)
	}
	rule, err = service.Safeguard(ctx, rule.Metadata.ID)
	if err != nil || rule.Spec.Metrics.Evaluations != 2 || rule.Spec.Metrics.Hits != 1 || rule.Spec.Metrics.Misses != 1 || rule.Spec.Metrics.EvaluationCostUnits != 2 {
		t.Fatalf("exact evaluation metrics = %#v, %v", rule.Spec.Metrics, err)
	}
	if _, err := service.FeedbackSafeguard(ctx, rule.Metadata.ID, domain.SafeguardFeedbackPositive); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ActivateSafeguard(ctx, rule.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = restarted.Close() })
	persisted, err := restarted.Safeguard(ctx, rule.Metadata.ID)
	if err != nil || persisted.Spec.State != domain.SafeguardActive || persisted.Spec.Metrics.PositiveFeedback != 1 {
		t.Fatalf("persisted safeguard = %#v, %v", persisted, err)
	}
}

func TestSafeguardMutationsAreUIOnlyAndProtected(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	projectID, repositoryID := seedSafeguardScope(t, service)
	occurrence := failureOccurrence{Category: "action", SourceType: "repository.refresh", Status: "failed", ExitCode: 9, ProjectID: projectID, RepositoryID: repositoryID}
	for range 3 {
		if err := service.recordFailureOccurrence(context.Background(), occurrence); err != nil {
			t.Fatal(err)
		}
	}
	rules, err := service.Safeguards(context.Background(), 10)
	if err != nil || len(rules) != 1 {
		t.Fatalf("rules = %#v, %v", rules, err)
	}
	body, _ := json.Marshal(map[string]string{"owner": "local-user"})
	path := "/ui/safeguards/" + rules[0].Metadata.ID + "/shadow"
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unprotected safeguard review = %d: %s", recorder.Code, recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	recorder = httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	var response contract.Envelope[domain.SafeguardRule]
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &response) != nil || response.Data == nil || response.Data.Spec.State != domain.SafeguardShadow {
		t.Fatalf("protected safeguard review = %d: %s", recorder.Code, recorder.Body.String())
	}
	if err := service.recordFailureOccurrence(context.Background(), occurrence); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/ui/safeguards/"+rules[0].Metadata.ID+"/feedback", bytes.NewBufferString(`{"feedback":"positive"}`))
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	recorder = httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("safeguard feedback = %d: %s", recorder.Code, recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "/ui/safeguards/"+rules[0].Metadata.ID+"/activate", nil)
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	recorder = httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	response = contract.Envelope[domain.SafeguardRule]{}
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &response) != nil || response.Data == nil || response.Data.Spec.ActivationApprovedBy != "local-user" {
		t.Fatalf("human safeguard activation = %d: %s", recorder.Code, recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "/api/safeguards/"+rules[0].Metadata.ID+"/activate", nil)
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	recorder = httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound && recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("API exposed safeguard mutation = %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestConcurrentFailureOccurrencesDoNotLoseCounts(t *testing.T) {
	home := t.TempDir()
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	projectID, repositoryID := seedSafeguardScope(t, service)
	second, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	occurrence := failureOccurrence{Category: "checkset", SourceType: "concurrent", Status: "failed", ExitCode: 1, ProjectID: projectID, RepositoryID: repositoryID}
	const count = 20
	errCh := make(chan error, count)
	var group sync.WaitGroup
	services := []*App{service, second}
	for index := range count {
		group.Add(1)
		go func(writer *App) {
			defer group.Done()
			errCh <- writer.recordFailureOccurrence(context.Background(), occurrence)
		}(services[index%len(services)])
	}
	group.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	rules, err := service.Safeguards(context.Background(), 10)
	if err != nil || len(rules) != 1 || rules[0].Spec.OccurrenceCount != count {
		t.Fatalf("concurrent safeguard count = %#v, %v", rules, err)
	}
	if _, err := service.ReviewSafeguard(context.Background(), rules[0].Metadata.ID, "local-user"); err != nil {
		t.Fatal(err)
	}
	errCh = make(chan error, count)
	for index := range count {
		group.Add(1)
		go func(writer *App) {
			defer group.Done()
			errCh <- writer.recordFailureOccurrence(context.Background(), occurrence)
		}(services[index%len(services)])
	}
	group.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	rule, err := service.Safeguard(context.Background(), rules[0].Metadata.ID)
	if err != nil || rule.Spec.OccurrenceCount != count*2 || rule.Spec.Metrics.Evaluations != count || rule.Spec.Metrics.Hits != count {
		t.Fatalf("concurrent safeguard metrics = %#v, %v", rule, err)
	}
}

func TestOnlyVerifiedTerminalRunsEnterFailureLearning(t *testing.T) {
	for status, want := range map[domain.CheckRunStatus]bool{
		domain.CheckPassed: false, domain.CheckSkipped: false, domain.CheckCancelled: false,
		domain.CheckFailed: true, domain.CheckTimedOut: true, domain.CheckUnavailable: true,
	} {
		if got := shouldRecordCheckRunFailure(status); got != want {
			t.Errorf("check status %s record = %t, want %t", status, got, want)
		}
	}
	for status, want := range map[domain.ActionRunStatus]bool{
		domain.ActionRunSucceeded: false, domain.ActionRunCancelled: false, domain.ActionRunRunning: false,
		domain.ActionRunPrecheckFailed: true, domain.ActionRunFailed: true, domain.ActionRunTimedOut: true,
		domain.ActionRunPostcheckFailed: true, domain.ActionRunUnavailable: true,
	} {
		if got := shouldRecordActionRunFailure(status); got != want {
			t.Errorf("action status %s record = %t, want %t", status, got, want)
		}
	}
}

func seedSafeguardScope(t *testing.T, service *App) (string, string) {
	t.Helper()
	project := domain.NewProject("safeguard-project", "Safeguard Project", []domain.Repository{domain.NewRepository("repo", "Repo", t.TempDir())})
	if err := service.store.SaveProject(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	return project.Metadata.ID, project.Spec.Repositories[0].Metadata.ID
}
