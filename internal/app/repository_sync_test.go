package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/action"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestRepositorySyncPlanSupportsManyRepositoriesAndExplainsSkips(t *testing.T) {
	first := tempGitRepository(t, "first")
	second := tempGitRepository(t, "second")
	third := tempGitRepository(t, "third")
	configureTrackedRemote(t, first, "first-remote")
	configureTrackedRemote(t, second, "second-remote")
	if err := writeFixtureFile(second, "local-change.txt"); err != nil {
		t.Fatal(err)
	}

	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Many Repositories", Path: first})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		id, name, path string
	}{{"dirty", "Dirty", second}, {"no-upstream", "No upstream", third}} {
		if _, err := service.AddRepository(context.Background(), AddRepositoryInput{ProjectID: project.Metadata.ID, ID: item.id, Name: item.name, Path: item.path}); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}

	plan, err := service.RepositorySyncPlan(context.Background(), project.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Plans) != 1 || plan.Plans[0].Spec.RepositoryID != "repo-1" {
		t.Fatalf("sync plans = %#v", plan.Plans)
	}
	if plan.Plans[0].Spec.ActionType != repositorySyncAction || len(plan.Plans[0].Spec.Execution.Arguments) != 3 {
		t.Fatalf("sync action contract = %#v", plan.Plans[0].Spec)
	}
	codes := make(map[string]string, len(plan.Skipped))
	for _, item := range plan.Skipped {
		codes[item.RepositoryID] = item.Code
	}
	if codes["dirty"] != "local_changes" || codes["no-upstream"] != "missing_upstream" {
		t.Fatalf("sync skip reasons = %#v", codes)
	}
}

func TestRepositorySyncHTTPExecutesOnlyPersistedPlanIDs(t *testing.T) {
	repository := tempGitRepository(t, "tracked")
	configureTrackedRemote(t, repository, "tracked-remote")
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "HTTP Sync", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/projects/"+project.Metadata.ID+"/repository-sync/plan", nil)
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("sync plan HTTP = %d: %s", recorder.Code, recorder.Body.String())
	}
	var envelope contract.Envelope[RepositorySyncPlan]
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil || envelope.Data == nil || len(envelope.Data.Plans) != 1 {
		t.Fatalf("sync plan envelope = %#v, %v", envelope, err)
	}

	fake := &fakeActionProcessRunner{}
	service.broker, err = action.NewWithRunner(service.store, nil, fake)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"planIds": []string{envelope.Data.Plans[0].Metadata.ID}, "requestId": "http-sync-1"})
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/projects/"+project.Metadata.ID+"/repository-sync/execute", bytes.NewReader(body))
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	recorder = httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("sync execute HTTP = %d: %s", recorder.Code, recorder.Body.String())
	}
	var resultEnvelope contract.Envelope[RepositorySyncResult]
	if err := json.Unmarshal(recorder.Body.Bytes(), &resultEnvelope); err != nil || resultEnvelope.Data == nil || len(resultEnvelope.Data.Outcomes) != 1 || resultEnvelope.Data.Outcomes[0].Run == nil || resultEnvelope.Data.Outcomes[0].Run.Spec.Status != domain.ActionRunSucceeded || fake.calls != 1 {
		t.Fatalf("sync execute result = %#v, fake = %#v, err = %v", resultEnvelope, fake, err)
	}

	badBody, _ := json.Marshal(map[string]any{"planIds": []string{"not-a-persisted-sync-plan"}, "requestId": "http-sync-2"})
	request = httptest.NewRequest(http.MethodPost, "/api/projects/"+project.Metadata.ID+"/repository-sync/execute", bytes.NewReader(badBody))
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	recorder = httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unknown sync plan = %d: %s", recorder.Code, recorder.Body.String())
	}
}

func configureTrackedRemote(t *testing.T, repository, name string) {
	t.Helper()
	remote := filepath.Join(t.TempDir(), name+".git")
	if err := os.MkdirAll(remote, 0o700); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, remote, "init", "--bare")
	gitFixture(t, repository, "remote", "add", "origin", remote)
	gitFixture(t, repository, "push", "-u", "origin", "main")
}

func writeFixtureFile(repository, name string) error {
	return os.WriteFile(filepath.Join(repository, name), []byte("fixture\n"), 0o600)
}
