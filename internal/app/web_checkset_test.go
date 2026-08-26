package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestEmbeddedUIExposesKoreanMultiViewControlRoom(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	html := embeddedUIAsset(t, service, "/", "text/html")
	for _, value := range []string{
		`<html lang="ko">`, "본문으로 건너뛰기", "홈", "프로젝트", "작업", "진단", "기록",
		"지금 확인할 항목", "프로젝트별 상태", "최근 실행 결과",
		"등록 → 관찰 → 검토 → 실행", "처음 사용하는 순서", "외부 작업과 릴리스", "Jenkins 대상 그룹",
		"폴더 선택", "저장소 찾기",
		"발견 및 제안 검토", "Agent Profile 관리",
		"등록 정보만 제거하며 저장소 파일은 삭제하지 않습니다.",
		`data-view="home"`, `data-view="projects" hidden`, `aria-label="주 탐색"`,
		`href="/ui/app.css"`, `src="/ui/app.js"`, `meta name="control-room-token"`,
	} {
		if !strings.Contains(html, value) {
			t.Errorf("embedded UI HTML missing %q", value)
		}
	}
	if strings.Contains(html, "__MUTATION_TOKEN__") {
		t.Error("embedded UI contains an unresolved mutation token placeholder")
	}

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	for _, value := range []string{
		"Pre-PR 점검", "Checkset 만들기", "적용", "실행", "결과 보기",
		"/api/checksets", "/apply", "/run", "/runs", "X-Control-Room-Token",
		"/api/projects/discover", "/api/folder-picker",
		"Agent Profile", "/api/actions/plans", "저장소 새로고침 계획", "실행 대상으로 표시",
		"정리 후보", "/api/cleanup/candidates", "지침 점검", "/api/handoffs/preview",
		"반복된 실패", "/api/safeguards/rules", "검토 전 모의 적용",
		"data-safeguard=\"shadow\"", "유효한 방지", "오탐", "모의 적용으로 되돌리기", "평가 비용",
		"활성화 조건", "활성 승인",
		"등록 해제", "method: \"DELETE\"", "/repositories/",
		"unregisterInput.value !== unregisterTarget?.name", "project.repos.length <= 1",
		"마지막 저장소는 개별 해제할 수 없습니다", "원본 저장소 파일은 변경하지 않았습니다",
		"프로젝트 이름 변경", "새 저장소 등록", "프로젝트 내보내기", "/api/projects/import",
		"data-finding=\"acknowledge\"", "confidenceLabels", "/acknowledge",
		"data-proposal=\"apply\"", "data-proposal=\"reject\"", "/worktrees/${encode(target[2])}/discover",
		"renderCheckRun", "사전 점검", "사후 점검", "실행 종료 코드",
		"renderActionEvents", "감사 이벤트", "작업 폴더", "프로젝트 저장소 최신화 결과",
		"Agent Profile 추가", "data-profile=\"edit\"", "/api/agent-profiles",
		"근거 변경됨", "기존 점검 다시 찾기", "열림 및 확인함", `data-unregister="profile"`,
		"surfaceErrors", `role="alert"`, "data-retry", "loadRouteData(currentRoute(), true)",
		"/api/external-work-groups", "/api/releases/", "/api/cleanup/", "Worktree 신뢰", "전용 실행",
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("embedded UI JavaScript missing %q", value)
		}
	}
	if strings.Contains(javascript, "alert(") {
		t.Error("embedded UI uses blocking browser alerts instead of an accessible status message")
	}
	if strings.Contains(javascript, ".catch(() => [])") {
		t.Error("embedded UI hides failed API requests as empty collections")
	}

	css := embeddedUIAsset(t, service, "/ui/app.css", "text/css")
	for _, value := range []string{".app-shell", ".side-nav", ".skip-link", ":focus-visible", "prefers-reduced-motion"} {
		if !strings.Contains(css, value) {
			t.Errorf("embedded UI CSS missing %q", value)
		}
	}
}

func TestEmbeddedUIAssuranceDashboardContract(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	html := embeddedUIAsset(t, service, "/", "text/html")
	for _, value := range []string{
		`data-route="assurance"`, `data-view="assurance" hidden`, "검증 대시보드", "검증의 가치",
		"Provider와 모델", `id="assurance-provider-filter"`, `id="assurance-model-filter"`,
		`id="assurance-runs"`, `id="assurance-invocations"`, `id="assurance-effects"`, `id="assurance-artifacts"`,
		"대시보드 보기", "원문 미수집", "보관 상태",
	} {
		if !strings.Contains(html, value) {
			t.Errorf("embedded Assurance dashboard HTML missing %q", value)
		}
	}

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	for _, value := range []string{
		"assuranceDashboardPath", "renderAssuranceDashboard", "renderAssuranceBenefits", "refreshAssuranceFilter",
		"/api/assurance/dashboard", "/api/assurance/runs", "/api/assurance/invocations",
		"/api/assurance/artifacts", "/api/assurance/effects", "assurance-provider-filter", "assurance-model-filter",
		"Agent 실행", "효과 기록", "재실행", "근거 연결", "비용 경계",
		"rawTranscript", "usageComplete", "estimatedCost", "configDigest", "artifactIds", "evidenceIds",
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("embedded Assurance dashboard JavaScript missing %q", value)
		}
	}
	if strings.Contains(javascript, "JSON.stringify(spec.structured)") || strings.Contains(javascript, "spec.transcript") {
		t.Error("embedded Assurance dashboard must not expose raw transcript or structured provider output")
	}

	css := embeddedUIAsset(t, service, "/ui/app.css", "text/css")
	for _, value := range []string{".assurance-hero", ".assurance-benefits", ".assurance-filters", ".assurance-metrics", ".assurance-record"} {
		if !strings.Contains(css, value) {
			t.Errorf("embedded Assurance dashboard CSS missing %q", value)
		}
	}
}

func TestEmbeddedUIKeyboardRouteFocusContract(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	html := embeddedUIAsset(t, service, "/", "text/html")
	for _, value := range []string{
		`<a class="skip-link" href="#main-content">`,
		`<h1 id="view-title">`,
		`<main id="main-content" tabindex="0" aria-labelledby="view-title">`,
		`data-home-established-only hidden`,
	} {
		if !strings.Contains(html, value) {
			t.Errorf("embedded UI keyboard contract missing %q", value)
		}
	}

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	for _, value := range []string{
		"let routeFocusTimer = 0",
		`if (candidate === "main-content") return activeRoute`,
		`document.querySelector(".skip-link").addEventListener("click"`,
		"event.preventDefault()",
		"window.clearTimeout(routeFocusTimer)",
		"routeFocusTimer = window.setTimeout(() => {",
		`document.getElementById("main-content").focus({ preventScroll: true })`,
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("embedded UI route focus contract missing %q", value)
		}
	}
}

func TestEmbeddedUIProviderCapabilityGroupingContract(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	html := embeddedUIAsset(t, service, "/", "text/html")
	for _, value := range []string{
		`<div id="environment" class="loading" aria-live="polite">`,
		"선택 Provider는 필요할 때만 설정합니다. 미설정 상태는 전체 환경 경고가 아닙니다.",
		`<div id="provider-statuses" class="loading" tabindex="-1" aria-live="polite">`,
	} {
		if !strings.Contains(html, value) {
			t.Errorf("embedded UI provider grouping HTML missing %q", value)
		}
	}

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	for _, value := range []string{
		"const optionalProviderIDs = new Set",
		"const requiredEnvironmentReady = environment =>",
		"const providerStateSummaries =",
		`detected: "실행 확인 필요"`,
		`unavailable: "사용할 수 없음"`,
		"const providerSummary = state =>",
		"Provider 상태는 아래 카드에서 한 번에 확인합니다.",
		"new Map((state.providerStatuses || []).map",
		"data-provider-capability=",
		"진단 세부 정보",
		"data-provider-recovery",
		`data-focus-target="provider-statuses"`,
		"진단 열기",
		"const providerIDs = new Set",
		"return !providerFinding",
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("embedded UI provider grouping JavaScript missing %q", value)
		}
	}
	if strings.Contains(javascript, "state.environment.available") {
		t.Error("optional Provider state must not determine the global environment summary")
	}
	if strings.Contains(javascript, "const detail = item.detail") {
		t.Error("Provider detail must not be the default card summary")
	}
}

func TestEmbeddedUIFirstUseAndFindingTargetContract(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	html := embeddedUIAsset(t, service, "/", "text/html")
	for _, value := range []string{
		`id="home-onboarding"`, `id="home-metrics"`, `id="home-providers"`, `id="home-assurance"`,
		`data-home-established-only hidden`, "처음 사용하는 순서", "프로젝트 등록",
	} {
		if !strings.Contains(html, value) {
			t.Errorf("embedded UI first-use HTML missing %q", value)
		}
	}

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	for _, value := range []string{
		`document.querySelectorAll("[data-home-established-only]")`,
		"function findingRoute(item)",
		"new URLSearchParams({ finding: findingID })",
		"data-finding-id=",
		"pendingFindingID",
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("embedded UI first-use/finding JavaScript missing %q", value)
		}
	}
	if strings.Contains(javascript, `href="#projects">확인 항목 열기`) {
		t.Error("finding CTA must retain its target instead of linking to a generic project list")
	}
}

func embeddedUIAsset(t *testing.T, service *App, path, contentType string) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s = %d", path, recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, contentType) {
		t.Fatalf("GET %s content type = %q, want prefix %q", path, got, contentType)
	}
	return recorder.Body.String()
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

func TestEmbeddedUIUnregisterRoutesPreserveRepositoryFiles(t *testing.T) {
	first := tempGitRepository(t, "ui-unregister-first")
	second := tempGitRepository(t, "ui-unregister-second")
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })

	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "UI Unregister", Path: first})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddRepository(context.Background(), AddRepositoryInput{ProjectID: project.Metadata.ID, ID: "second", Name: "Second", Path: second}); err != nil {
		t.Fatal(err)
	}
	repositoryPath := "/api/projects/" + project.Metadata.ID + "/repositories/second"
	assertUIError(t, service, http.MethodDelete, repositoryPath, nil, "", "", http.StatusForbidden, contract.ErrorForbidden)
	removedRepository := callUICheckset[map[string]bool](t, service, http.MethodDelete, repositoryPath, nil)
	if !removedRepository["removed"] {
		t.Fatalf("repository unregister response = %#v", removedRepository)
	}
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("repository files changed after unregister: %v", err)
	}

	removedProject := callUICheckset[map[string]bool](t, service, http.MethodDelete, "/api/projects/"+project.Metadata.ID, nil)
	if !removedProject["removed"] {
		t.Fatalf("project unregister response = %#v", removedProject)
	}
	if _, err := os.Stat(first); err != nil {
		t.Fatalf("project repository files changed after unregister: %v", err)
	}
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
