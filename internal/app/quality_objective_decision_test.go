package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestQualityObjectiveDecisionHTTPSuccessPaths(t *testing.T) {
	tests := []struct {
		name        string
		disposition string
		action      string
		reason      string
		wantState   string
	}{
		{name: "pursue", disposition: domain.QualityObjectiveDispositionPursue, action: "add regression tests", wantState: domain.QualityObjectiveStateReady},
		{name: "defer", disposition: domain.QualityObjectiveDispositionDefer, reason: "waiting for the next review window", wantState: domain.QualityObjectiveStateBlocked},
		{name: "dismiss", disposition: domain.QualityObjectiveDispositionDismiss, reason: "duplicate of an existing objective", wantState: domain.QualityObjectiveStateRejected},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			service, objective := newQualityObjectiveDecisionFixture(t)
			input := QualityObjectiveDecisionInput{
				ExpectedRevision: objective.Spec.Revision,
				Disposition:      testCase.disposition,
				Action:           testCase.action,
				Reason:           testCase.reason,
				Actor:            "knowgyu",
			}

			recorder := postQualityObjectiveDecision(t, service, objective.Metadata.ID, input, service.mutationToken, "http://127.0.0.1:38471")
			if recorder.Code != http.StatusOK {
				t.Fatalf("decision response = %d: %s", recorder.Code, recorder.Body.String())
			}

			var envelope contract.Envelope[domain.QualityObjective]
			if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil {
				t.Fatal(err)
			}
			if !envelope.OK || envelope.Error != nil || envelope.Data == nil || envelope.Schema != contract.EnvelopeSchema {
				t.Fatalf("decision envelope = %#v", envelope)
			}
			if envelope.Data.Metadata.ID != objective.Metadata.ID || envelope.Data.Spec.State != testCase.wantState || envelope.Data.Spec.Revision != objective.Spec.Revision+1 {
				t.Fatalf("updated objective = %#v", envelope.Data)
			}
			if envelope.Data.Spec.Decision == nil || envelope.Data.Spec.Decision.Disposition != testCase.disposition || envelope.Data.Spec.Decision.Actor != "knowgyu" {
				t.Fatalf("decision record = %#v", envelope.Data.Spec.Decision)
			}

			loaded, err := service.QualityObjective(context.Background(), objective.Metadata.ID)
			if err != nil {
				t.Fatal(err)
			}
			if loaded.Spec.Revision != objective.Spec.Revision+1 || loaded.Spec.State != testCase.wantState {
				t.Fatalf("persisted objective = %#v", loaded)
			}
		})
	}
}

func TestQualityObjectiveDecisionHTTPResumesBlockedObjective(t *testing.T) {
	service, objective := newQualityObjectiveDecisionFixture(t)

	deferRequest := QualityObjectiveDecisionInput{
		ExpectedRevision: objective.Spec.Revision,
		Disposition:      domain.QualityObjectiveDispositionDefer,
		Reason:           "defer until the dependency is ready",
		Actor:            "knowgyu",
	}
	deferred := postQualityObjectiveDecision(t, service, objective.Metadata.ID, deferRequest, service.mutationToken, "")
	if deferred.Code != http.StatusOK {
		t.Fatalf("defer response = %d: %s", deferred.Code, deferred.Body.String())
	}

	pursueRequest := QualityObjectiveDecisionInput{
		ExpectedRevision: objective.Spec.Revision + 1,
		Disposition:      domain.QualityObjectiveDispositionPursue,
		Action:           "resume with a focused test change",
		Actor:            "knowgyu",
	}
	resumed := postQualityObjectiveDecision(t, service, objective.Metadata.ID, pursueRequest, service.mutationToken, "")
	if resumed.Code != http.StatusOK {
		t.Fatalf("resume response = %d: %s", resumed.Code, resumed.Body.String())
	}

	var envelope contract.Envelope[domain.QualityObjective]
	if err := json.NewDecoder(resumed.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data == nil || envelope.Data.Spec.State != domain.QualityObjectiveStateReady || envelope.Data.Spec.Revision != objective.Spec.Revision+2 {
		t.Fatalf("resumed objective envelope = %#v", envelope)
	}
}

func TestQualityObjectiveDecisionHTTPRejectsUnauthorizedAndInvalidPayloadsWithoutMutation(t *testing.T) {
	tests := []struct {
		name       string
		rawBody    string
		input      *QualityObjectiveDecisionInput
		token      string
		origin     string
		wantStatus int
		wantCode   contract.ErrorCode
	}{
		{name: "missing token", input: &QualityObjectiveDecisionInput{ExpectedRevision: 1, Disposition: domain.QualityObjectiveDispositionPursue, Action: "add tests", Actor: "knowgyu"}, wantStatus: http.StatusForbidden, wantCode: contract.ErrorForbidden},
		{name: "wrong origin", input: &QualityObjectiveDecisionInput{ExpectedRevision: 1, Disposition: domain.QualityObjectiveDispositionPursue, Action: "add tests", Actor: "knowgyu"}, token: "use-fixture-token", origin: "https://example.invalid", wantStatus: http.StatusForbidden, wantCode: contract.ErrorForbidden},
		{name: "malformed json", rawBody: `{"expectedRevision":1,"disposition":"pursue"`, token: "use-fixture-token", wantStatus: http.StatusBadRequest, wantCode: contract.ErrorInvalidInput},
		{name: "missing expected revision", input: &QualityObjectiveDecisionInput{Disposition: domain.QualityObjectiveDispositionPursue, Action: "add tests", Actor: "knowgyu"}, token: "use-fixture-token", wantStatus: http.StatusBadRequest, wantCode: contract.ErrorInvalidInput},
		{name: "invalid disposition", input: &QualityObjectiveDecisionInput{ExpectedRevision: 1, Disposition: "review", Action: "add tests", Actor: "knowgyu"}, token: "use-fixture-token", wantStatus: http.StatusBadRequest, wantCode: contract.ErrorInvalidInput},
		{name: "pursue action required", input: &QualityObjectiveDecisionInput{ExpectedRevision: 1, Disposition: domain.QualityObjectiveDispositionPursue, Actor: "knowgyu"}, token: "use-fixture-token", wantStatus: http.StatusBadRequest, wantCode: contract.ErrorInvalidInput},
		{name: "minimum percent out of range", input: &QualityObjectiveDecisionInput{ExpectedRevision: 1, Disposition: domain.QualityObjectiveDispositionPursue, Action: "add tests", Actor: "knowgyu", MinimumPercent: 101}, token: "use-fixture-token", wantStatus: http.StatusBadRequest, wantCode: contract.ErrorInvalidInput},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			service, objective := newQualityObjectiveDecisionFixture(t)
			rawBody := []byte(testCase.rawBody)
			if testCase.input != nil {
				var err error
				rawBody, err = json.Marshal(*testCase.input)
				if err != nil {
					t.Fatal(err)
				}
			}
			token := testCase.token
			if token == "use-fixture-token" {
				token = service.mutationToken
			}
			recorder := postQualityObjectiveDecisionBody(t, service, objective.Metadata.ID, rawBody, token, testCase.origin)
			if recorder.Code != testCase.wantStatus {
				t.Fatalf("response = %d, want %d: %s", recorder.Code, testCase.wantStatus, recorder.Body.String())
			}
			var envelope contract.Envelope[map[string]any]
			if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.OK || envelope.Error == nil || envelope.Error.Code != testCase.wantCode || envelope.Schema != contract.EnvelopeSchema {
				t.Fatalf("error envelope = %#v", envelope)
			}

			loaded, err := service.QualityObjective(context.Background(), objective.Metadata.ID)
			if err != nil {
				t.Fatal(err)
			}
			if loaded.Spec.Revision != objective.Spec.Revision || loaded.Spec.State != domain.QualityObjectiveStateDraft || loaded.Spec.Decision != nil {
				t.Fatalf("invalid request partially mutated objective = %#v", loaded)
			}
		})
	}
}

func TestQualityObjectiveDecisionHTTPRejectsMissingAndStaleObjectives(t *testing.T) {
	service, objective := newQualityObjectiveDecisionFixture(t)
	input := QualityObjectiveDecisionInput{
		ExpectedRevision: objective.Spec.Revision,
		Disposition:      domain.QualityObjectiveDispositionPursue,
		Action:           "add tests",
		Actor:            "knowgyu",
	}

	missing := postQualityObjectiveDecision(t, service, "objective-missing", input, service.mutationToken, "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing objective response = %d: %s", missing.Code, missing.Body.String())
	}

	first := postQualityObjectiveDecision(t, service, objective.Metadata.ID, input, service.mutationToken, "")
	if first.Code != http.StatusOK {
		t.Fatalf("initial decision response = %d: %s", first.Code, first.Body.String())
	}
	stale := postQualityObjectiveDecision(t, service, objective.Metadata.ID, input, service.mutationToken, "")
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale decision response = %d: %s", stale.Code, stale.Body.String())
	}
	var envelope contract.Envelope[map[string]any]
	if err := json.NewDecoder(stale.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.OK || envelope.Error == nil || envelope.Error.Code != contract.ErrorConflict {
		t.Fatalf("stale error envelope = %#v", envelope)
	}

	loaded, err := service.QualityObjective(context.Background(), objective.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Spec.Revision != objective.Spec.Revision+1 || loaded.Spec.State != domain.QualityObjectiveStateReady || loaded.Spec.Decision == nil {
		t.Fatalf("stale request changed objective = %#v", loaded)
	}
}

func TestQualityObjectiveDecisionAndConfirmationHTTPUseStrictJSONContracts(t *testing.T) {
	decisionBodies := []string{
		`{"expectedRevision":1,"disposition":"pursue","action":"add tests","actor":"knowgyu","state":"ready"}`,
		`{"expectedRevision":1,"disposition":"pursue","action":"add tests","actor":"knowgyu"}{}`,
		`{"expectedRevision":"1","disposition":"pursue","action":"add tests","actor":"knowgyu"}`,
	}
	for index, body := range decisionBodies {
		t.Run("decision strict body "+string(rune('1'+index)), func(t *testing.T) {
			service, objective := newQualityObjectiveDecisionFixture(t)
			response := postQualityObjectiveRaw(t, service, "/api/quality/objectives/"+objective.Metadata.ID+"/decision", body, service.mutationToken, "")
			if response.Code != http.StatusBadRequest {
				t.Fatalf("decision response = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			loaded, err := service.QualityObjective(context.Background(), objective.Metadata.ID)
			if err != nil {
				t.Fatal(err)
			}
			if loaded.Spec.Revision != objective.Spec.Revision || loaded.Spec.Decision != nil {
				t.Fatalf("strict decision request mutated objective = %#v", loaded)
			}
		})
	}

	confirmationBodies := []string{
		`{"expectedRevision":1,"state":"adopted"}`,
		`{"expectedRevision":1}{}`,
		`{"expectedRevision":"1"}`,
	}
	for index, body := range confirmationBodies {
		t.Run("confirmation strict body "+string(rune('1'+index)), func(t *testing.T) {
			service, objective := newQualityObjectiveDecisionFixture(t)
			response := postQualityObjectiveRaw(t, service, "/api/quality/objectives/"+objective.Metadata.ID+"/confirm", body, service.mutationToken, "")
			if response.Code != http.StatusBadRequest {
				t.Fatalf("confirmation response = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			loaded, err := service.QualityObjective(context.Background(), objective.Metadata.ID)
			if err != nil {
				t.Fatal(err)
			}
			if loaded.Spec.Revision != objective.Spec.Revision || loaded.Spec.State != domain.QualityObjectiveStateDraft {
				t.Fatalf("strict confirmation request mutated objective = %#v", loaded)
			}
		})
	}
}

func newQualityObjectiveDecisionFixture(t *testing.T) (*App, domain.QualityObjective) {
	t.Helper()
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	project, err := service.AddProject(context.Background(), AddProjectInput{
		Name: "Quality objective decision",
		Path: tempGoGitRepository(t, "quality-objective-decision"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	objective, err := service.CreateQualityObjective(context.Background(), QualityObjectiveInput{
		ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary",
		Owner: "owner", Title: "Improve decision coverage",
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, objective
}

func postQualityObjectiveDecision(t *testing.T, service *App, id string, input QualityObjectiveDecisionInput, token, origin string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return postQualityObjectiveDecisionBody(t, service, id, body, token, origin)
}

func postQualityObjectiveDecisionBody(t *testing.T, service *App, id string, body []byte, token, origin string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/quality/objectives/"+id+"/decision", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("X-Control-Room-Token", token)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	return recorder
}
