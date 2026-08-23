package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestActionAdaptersCannotGrantApprovalOrAdmitProtectedPlan(t *testing.T) {
	service, project := actionAdapterFixture(t)
	defer service.Close()
	input := ActionPlanInput{ID: "plan-agent", Name: "Production", ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "abc123"}}
	wrongScope := input
	wrongScope.ID, wrongScope.WorktreeID = "plan-wrong-worktree", "other"
	if _, err := service.PlanAction(context.Background(), wrongScope); err == nil {
		t.Fatal("plan accepted a worktree outside the registered exact scope")
	}
	plan, err := service.PlanAction(context.Background(), input)
	if err != nil || plan.Spec.RequestedBy != (domain.Actor{Kind: domain.ActorSystem, ID: "adapter"}) {
		t.Fatalf("plan = %#v, %v", plan, err)
	}
	status, err := service.ActionStatus(context.Background(), plan.Metadata.ID)
	if err != nil || status.Admission != "approval_required" || len(status.Approvals) != 0 {
		t.Fatalf("status before approval = %#v, %v", status, err)
	}
	if _, err := service.AdmitAction(context.Background(), plan.Metadata.ID, "runner-1", "request-1"); contract.Classify(err).Code != contract.ErrorPolicyDenied {
		t.Fatalf("unapproved plan admitted: %v", err)
	}
}

func TestActionHTTPRequiresMutationTokenAndCannotGrantApproval(t *testing.T) {
	service, project := actionAdapterFixture(t)
	defer service.Close()
	input := ActionPlanInput{ID: "plan-http", Name: "Production", ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary", ActionType: "release.production", Inputs: map[string]string{"commit": "abc123"}}
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	body = append(bytes.TrimSuffix(body, []byte("}")), []byte(`,"requestedByAgentID":"forged-agent"}`)...)
	request := httptest.NewRequest(http.MethodPost, "/api/actions/plans", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unprotected plan = %d: %s", recorder.Code, recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "/api/actions/plans", bytes.NewReader(body))
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	recorder = httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("plan HTTP = %d: %s", recorder.Code, recorder.Body.String())
	}
	admitBody := []byte(`{"holder":"runner-1","idempotencyKey":"request-1"}`)
	request = httptest.NewRequest(http.MethodPost, "/api/actions/plans/plan-http/admit", bytes.NewReader(admitBody))
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	recorder = httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	var failure contract.Envelope[map[string]any]
	if recorder.Code != http.StatusForbidden || json.Unmarshal(recorder.Body.Bytes(), &failure) != nil || failure.Error == nil || failure.Error.Code != contract.ErrorPolicyDenied {
		t.Fatalf("unapproved HTTP admit = %d: %s", recorder.Code, recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "/api/actions/plans/plan-http/approve", bytes.NewReader([]byte(`{"id":"approval-http","approvedBy":{"kind":"human","id":"forged"}}`)))
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	recorder = httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound && recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("HTTP adapter exposed approval grant: %d %s", recorder.Code, recorder.Body.String())
	}
	wantStatus, err := service.ActionStatus(context.Background(), "plan-http")
	if err != nil {
		t.Fatal(err)
	}
	if wantStatus.Plan.Spec.RequestedBy != (domain.Actor{Kind: domain.ActorSystem, ID: "adapter"}) {
		t.Fatalf("HTTP accepted forged actor metadata: %#v", wantStatus.Plan.Spec.RequestedBy)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/actions/plans/plan-http", nil)
	recorder = httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	var statusEnvelope contract.Envelope[ActionApprovalStatus]
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &statusEnvelope) != nil || statusEnvelope.Schema != contract.EnvelopeSchema || statusEnvelope.Data == nil || !reflect.DeepEqual(*statusEnvelope.Data, wantStatus) {
		t.Fatalf("HTTP approval status = %d: %s", recorder.Code, recorder.Body.String())
	}
}

func actionAdapterFixture(t *testing.T) (*App, domain.Project) {
	t.Helper()
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Actions", Path: tempGitRepository(t, "actions")})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	return service, project
}
