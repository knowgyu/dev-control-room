package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/assurance"
	"github.com/knowgyu/dev-control-room/internal/contract"
)

func TestQualityToolsHTTPUsesReadOnlyEnvelope(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	request := httptest.NewRequest(http.MethodGet, "/api/quality/tools", nil)
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("quality tools response = %d: %s", recorder.Code, recorder.Body.String())
	}

	var envelope contract.Envelope[assurance.QualityToolsReadModel]
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Schema != contract.EnvelopeSchema || !envelope.OK || envelope.Data == nil {
		t.Fatalf("quality tools envelope = %#v", envelope)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("quality tools Cache-Control = %q, want no-store", recorder.Header().Get("Cache-Control"))
	}
	if envelope.Data.CheckedAt.IsZero() || len(envelope.Data.Tools) != 6 || len(envelope.Data.Capabilities) != 11 {
		t.Fatalf("quality tools data = %#v", envelope.Data)
	}
	wantToolIDs := []string{
		assurance.QualityRunnerGoToolID,
		assurance.QualityRunnerMutationToolID,
		assurance.QualityToolGovulncheckID,
		assurance.QualityToolStaticcheckID,
		assurance.QualityToolGosecID,
		assurance.QualityToolGitleaksID,
	}
	wantCapabilityIDs := []string{
		"go_test", "go_vet", "go_coverage", "mutation", "property", "fuzz", "targeted_e2e",
		"govulncheck", "staticcheck", "gosec", "gitleaks",
	}
	for index, wantID := range wantToolIDs {
		if envelope.Data.Tools[index].ID != wantID {
			t.Fatalf("tool ID at index %d = %q, want %q", index, envelope.Data.Tools[index].ID, wantID)
		}
	}
	for index, wantID := range wantCapabilityIDs {
		if envelope.Data.Capabilities[index].ID != wantID {
			t.Fatalf("capability ID at index %d = %q, want %q", index, envelope.Data.Capabilities[index].ID, wantID)
		}
	}
	for _, tool := range envelope.Data.Tools {
		if tool.ID == "" || tool.CheckedAt.IsZero() || tool.Reason == "" || tool.InstallGuidance == "" ||
			tool.DiscoveryState == "" || tool.ExecutionReadiness != assurance.QualityExecutionReadinessNotEvaluated ||
			tool.ReasonCode == "" || len(tool.ReasonCodes) == 0 || strings.Contains(string(tool.State), "available") {
			t.Fatalf("invalid tool status = %#v", tool)
		}
	}
	for _, capability := range envelope.Data.Capabilities {
		if capability.ID == "" ||
			capability.ToolID == "" ||
			capability.CheckedAt.IsZero() ||
			capability.Reason == "" ||
			capability.InstallGuidance == "" ||
			capability.DiscoveryState == "" ||
			capability.ExecutionReadiness != assurance.QualityExecutionReadinessNotEvaluated ||
			capability.ReasonCode == "" || len(capability.ReasonCodes) == 0 ||
			strings.Contains(string(capability.State), "available") {
			t.Fatalf("invalid capability status = %#v", capability)
		}
	}
	goTest, ok := findQualityCapability(envelope.Data.Capabilities, "go_test")
	if !ok || goTest.RunnerID != "" || goTest.State == assurance.QualityCapabilityStatePresentUnverified {
		t.Fatalf("generic go_test capability = %#v", goTest)
	}
}

func findQualityCapability(items []assurance.QualityCapabilityStatus, id string) (assurance.QualityCapabilityStatus, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return assurance.QualityCapabilityStatus{}, false
}
