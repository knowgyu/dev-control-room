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

func TestEmbeddedUIQualityToolsDiagnosticsSurfaceUsesLiveData(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	html := embeddedUIAsset(t, service, "/", "text/html")
	viewStart := strings.Index(html, `<section class="view" data-view="diagnostics"`)
	if viewStart < 0 {
		t.Fatal("embedded UI is missing the diagnostics view")
	}
	viewEndOffset := strings.Index(html[viewStart+1:], `<section class="view"`)
	if viewEndOffset < 0 {
		t.Fatal("embedded UI diagnostics view has no end boundary")
	}
	diagnostics := html[viewStart : viewStart+1+viewEndOffset]
	if strings.Contains(html[:viewStart], `id="quality-tools"`) || strings.Contains(html[viewStart+1+viewEndOffset:], `id="quality-tools"`) {
		t.Fatal("quality tools surface escaped the diagnostics view")
	}
	for _, value := range []string{
		`id="quality-tools-section"`,
		`id="quality-tools"`,
		`aria-live="polite"`,
		`aria-busy="true"`,
		"품질 도구",
		"로컬 도구 탐색 결과만 표시합니다. 실행·신뢰 여부는 여기서 판단하지 않습니다.",
	} {
		if !strings.Contains(diagnostics, value) {
			t.Errorf("diagnostics view missing %q", value)
		}
	}

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	styles := embeddedUIAsset(t, service, "/ui/app.css", "text/css")
	for _, value := range []string{
		`request("/api/quality/tools")`,
		"data.tools.map(item => renderQualityToolsItem(item, \"도구\"))",
		"data.capabilities.map(item => renderQualityToolsItem(item, \"기능\"))",
		"reason",
		"installGuidance",
		"available",
		"present_unverified",
		"needs_target",
		"missing",
		"installed_not_registered",
		"untrusted",
		"탐색됨",
		"후보 발견 · 미검증",
		"탐색되지 않음",
		"신뢰 확인 안 됨",
		`qualityToolsStateLabels[value] || "상태 미상"`,
		"탐색 설명",
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("quality tools UI missing live field or state %q", value)
		}
	}
	if strings.Contains(javascript, `qualityToolsStateLabels[value] || value`) {
		t.Fatal("quality tools unknown state leaks the raw backend value")
	}
	for _, value := range []string{
		`qualityTools.status === "loading"`,
		`class="quality-tools-loading"`,
		`qualityTools.status === "error"`,
		`class="quality-tools-error" role="alert"`,
		`!hasTools && !hasCapabilities`,
		`class="quality-tools-empty"`,
		"data-quality-tools-retry",
		"qualityToolsErrorMessage(error)",
		"서버 버전과 연결 상태를 확인한 뒤 다시 확인하세요.",
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("quality tools state handling missing %q", value)
		}
	}
	for _, value := range []string{
		".quality-tools-entry",
		".quality-tools-state",
		".quality-tools-state--present_unverified",
		".quality-tools-state--unknown",
		"@media (max-width: 720px)",
	} {
		if !strings.Contains(styles, value) {
			t.Errorf("quality tools styles missing %q", value)
		}
	}
	if strings.Contains(javascript, "quality-tools-entry ledger-row") || strings.Contains(javascript, "quality-tools-entry provider-card") {
		t.Fatal("quality tools UI reused a forbidden row layout")
	}
}

func TestQualityToolsUIContractHidesUnknownStatusValue(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	unknownStatus := "backend_internal_state"
	if strings.Contains(javascript, unknownStatus) {
		t.Fatalf("test fixture status unexpectedly appears in UI source: %q", unknownStatus)
	}
	if !strings.Contains(javascript, `qualityToolsStateLabels[value] || "상태 미상"`) {
		t.Fatal("quality tools UI has no fixed fallback for unknown status")
	}
	if strings.Contains(javascript, `qualityToolsStateLabels[value] || value`) {
		t.Fatal("quality tools UI exposes unknown backend status values")
	}
}

func TestQualityToolsUIContractReadsPopulatedLiveResponse(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/quality/tools", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("quality tools response = %d: %s", recorder.Code, recorder.Body.String())
	}
	var envelope contract.Envelope[assurance.QualityToolsReadModel]
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.OK || envelope.Data == nil || len(envelope.Data.Tools) == 0 || len(envelope.Data.Capabilities) == 0 {
		t.Fatalf("quality tools live response = %#v", envelope)
	}
	if !qualityToolsResponseHasState(envelope.Data, assurance.QualityToolStatePresentUnverified) {
		t.Fatalf("quality tools live response does not exercise %q: %#v", assurance.QualityToolStatePresentUnverified, envelope.Data)
	}
}

func qualityToolsResponseHasState(data *assurance.QualityToolsReadModel, want assurance.QualityToolState) bool {
	for _, item := range data.Tools {
		if item.State == want {
			return true
		}
	}
	for _, item := range data.Capabilities {
		if item.State == assurance.QualityCapabilityState(want) {
			return true
		}
	}
	return false
}
