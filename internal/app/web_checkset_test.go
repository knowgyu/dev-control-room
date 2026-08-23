package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestEmbeddedUIExposesChecksetReviewFlow(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("UI response = %d", recorder.Code)
	}
	for _, value := range []string{
		"Pre-PR checksets", "Create Checkset", "Apply", "Run", "Review results",
		"/api/checksets", "/apply", "/run", "/runs", "X-Control-Room-Token", "alert(e.message)",
		"Choose folder", "Find repositories below", "/api/projects/discover", "/api/folder-picker",
	} {
		if !strings.Contains(recorder.Body.String(), value) {
			t.Errorf("embedded UI missing %q", value)
		}
	}
}

func TestEmbeddedUIChecksetProtectedHandlerFlow(t *testing.T) {
	service, proposal := checksetFixture(t)
	appliedProposal := callUICheckset[domain.Proposal](t, service, http.MethodPost, "/api/proposals/"+proposal.Metadata.ID+"/apply", nil)
	if appliedProposal.Spec.State != domain.ProposalApplied {
		t.Fatalf("proposal state = %q", appliedProposal.Spec.State)
	}
	input := CreateChecksetInput{ID: "checks-ui", Name: "UI checks", ProposalID: proposal.Metadata.ID, Steps: []domain.CheckStep{{ID: "check", Name: proposal.Metadata.Name, Command: *proposal.Spec.TypedCommand}}}
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	assertUIError(t, service, http.MethodPost, "/api/checksets", body, "", "", http.StatusForbidden, contract.ErrorForbidden)
	assertUIError(t, service, http.MethodPost, "/api/checksets", body, service.mutationToken, "http://example.invalid", http.StatusForbidden, contract.ErrorForbidden)

	created := callUICheckset[domain.Checkset](t, service, http.MethodPost, "/api/checksets", body)
	if created.Spec.State != domain.ChecksetDraft || created.Spec.WorktreeID != proposal.Spec.WorktreeID || created.Spec.Head != proposal.Spec.Head {
		t.Fatalf("created checkset binding = %#v", created.Spec)
	}
	applied := callUICheckset[domain.Checkset](t, service, http.MethodPost, "/api/checksets/"+created.Metadata.ID+"/apply", nil)
	if applied.Spec.State != domain.ChecksetApplied {
		t.Fatalf("applied state = %q", applied.Spec.State)
	}
	run := callUICheckset[domain.CheckRun](t, service, http.MethodPost, "/api/checksets/"+created.Metadata.ID+"/run", nil)
	if run.Spec.Status != domain.CheckPassed || run.Spec.WorktreeID != proposal.Spec.WorktreeID || run.Spec.Head != proposal.Spec.Head {
		t.Fatalf("run binding = %#v", run.Spec)
	}
	runs := callUICheckset[[]domain.CheckRun](t, service, http.MethodGet, "/api/checksets/"+created.Metadata.ID+"/runs", nil)
	if len(runs) != 1 || runs[0].Metadata.ID != run.Metadata.ID {
		t.Fatalf("review results = %#v", runs)
	}
	assertUIError(t, service, http.MethodGet, "/api/checksets/missing/runs", nil, "", "", http.StatusNotFound, contract.ErrorNotFound)
}

func callUICheckset[T any](t *testing.T, service *App, method, path string, body []byte) T {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	if method != http.MethodGet {
		request.Header.Set("X-Control-Room-Token", service.mutationToken)
		request.Header.Set("Origin", "http://127.0.0.1:38471")
	}
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code < http.StatusOK || recorder.Code >= http.StatusMultipleChoices {
		t.Fatalf("%s %s = %d %s", method, path, recorder.Code, recorder.Body.String())
	}
	var envelope contract.Envelope[T]
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil || !envelope.OK || envelope.Data == nil {
		t.Fatalf("%s %s response = %#v, %v", method, path, envelope, err)
	}
	return *envelope.Data
}

func assertUIError(t *testing.T, service *App, method, path string, body []byte, token, origin string, wantStatus int, wantCode contract.ErrorCode) {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	if token != "" {
		request.Header.Set("X-Control-Room-Token", token)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	var envelope contract.Envelope[map[string]any]
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil || recorder.Code != wantStatus || envelope.OK || envelope.Error == nil || envelope.Error.Code != wantCode {
		t.Fatalf("%s %s error = status %d, %#v, %v", method, path, recorder.Code, envelope, err)
	}
}
