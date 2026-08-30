package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
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
		`<html lang="ko">`, "본문으로 건너뛰기", "개선", "프로젝트", "작업", "검증", "진단", "활동",
		"지금 개선할 것", "프로젝트별 상태", "최근 실행", "검증 근거",
		"시작하기", "Jenkins 대상 그룹", "사용법", "저장소를 연결하면 시작합니다.",
		"폴더 선택", "저장소 찾기",
		"발견 결과", "Agent Profile",
		"등록 정보만 제거하며 저장소 파일은 삭제하지 않습니다.",
		"assurance-demo-board", "예시 화면 보기",
		`data-view="home"`, `data-view="projects" hidden`, `aria-label="주 탐색"`, `id="home-assurance" class="ledger" aria-live="polite"`,
		`class="decision-strip home-setup"`, `class="evidence-flow work-flow"`,
		`href="/ui/app.css?v=0.14.0"`, `src="/ui/app.js?v=0.14.0"`, `meta name="control-room-token"`,
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
		"/api/external-work-groups", "/api/releases/", "/api/cleanup/", "Worktree 확인", "승인 요청",
		"const guideSlides = [", "#guide?slide=", "data-guide-next", "renderGuide", "isAssuranceDemoRoute", "renderAssuranceDemo", "#assurance?demo=1",
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
	for _, value := range []string{".app-shell", ".primary-nav", ".skip-link", ":focus-visible", "prefers-reduced-motion", "font-variant-numeric: tabular-nums", "--space-7: 32px", "--control-height: 40px", ".button.primary:disabled", ".flow-step", "align-items: start;", ".assurance-empty", "grid-template-columns: minmax(76px, auto) minmax(0, 1fr) auto", ".project-card .ledger-row__context { display: none; }", "grid-template-columns: minmax(116px, 140px) minmax(0, 1fr) auto", ".diagnostic-findings .finding", "#environment > .list-item", "#environment > .list-item > p { margin: 0; }", ".provider-card { border-left: 0; }", ".demo-banner", ".demo-board", ".demo-kpis"} {
		if !strings.Contains(css, value) {
			t.Errorf("embedded UI CSS missing %q", value)
		}
	}
}

func TestEmbeddedUIInformationArchitectureContract(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	html := embeddedUIAsset(t, service, "/", "text/html")
	css := embeddedUIAsset(t, service, "/ui/app.css", "text/css")
	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")

	if strings.Count(html, "page-context") > 1 || strings.Contains(html, `id="view-title"`) {
		t.Error("embedded UI must not duplicate page context or keep the retired view-title heading")
	}
	primaryNav := regexp.MustCompile(`(?is)<nav\b[^>]*\bclass="[^"]*\bprimary-nav\b[^"]*"[^>]*>`)
	if !primaryNav.MatchString(html) {
		t.Error("embedded UI is missing the primary-nav")
	}
	for route, text := range map[string]string{
		"home": "개선", "projects": "프로젝트", "work": "작업",
		"assurance": "검증", "diagnostics": "진단", "activity": "활동", "guide": "사용법",
	} {
		pattern := regexp.MustCompile(`(?is)<a\b[^>]*href="#` + route + `"[^>]*data-route="` + route + `"[^>]*>` + regexp.QuoteMeta(text) + `</a>`)
		if !pattern.MatchString(html) {
			t.Errorf("primary navigation route %q must use label %q", route, text)
		}
	}
	if strings.Contains(strings.ToLower(html+css), "side-nav") || strings.Contains(strings.ToLower(html+css), "sidebar") {
		t.Error("embedded UI must not keep the retired side-nav")
	}
	mainTag := regexp.MustCompile(`(?is)<main\b[^>]*>`).FindString(html)
	if !strings.Contains(mainTag, `id="main-content"`) || !strings.Contains(mainTag, `aria-label="개선"`) {
		t.Error("embedded UI main must have the initial 개선 aria-label")
	}

	routes := map[string]bool{
		"home": false, "projects": false, "work": false,
		"assurance": false, "diagnostics": false, "activity": false, "guide": false,
	}
	view := regexp.MustCompile(`(?is)<section\b[^>]*\bdata-view="([^"]+)"[^>]*>`)
	h1 := regexp.MustCompile(`(?is)<h1\b[^>]*>(.*?)</h1>`)
	matches := view.FindAllStringSubmatchIndex(html, -1)
	if len(matches) != len(routes) {
		t.Fatalf("embedded UI has %d data-view sections, want %d", len(matches), len(routes))
	}
	for index, match := range matches {
		route := html[match[2]:match[3]]
		if _, ok := routes[route]; !ok {
			t.Errorf("embedded UI has unexpected data-view %q", route)
			continue
		}
		if routes[route] {
			t.Errorf("embedded UI repeats data-view %q", route)
		}
		routes[route] = true

		start := match[0]
		end := len(html)
		if index+1 < len(matches) {
			end = matches[index+1][0]
		} else if mainEnd := strings.Index(html[start:], "</main>"); mainEnd >= 0 {
			end = start + mainEnd
		}
		headings := h1.FindAllStringSubmatch(html[start:end], -1)
		if len(headings) != 1 {
			t.Errorf("data-view %q has %d h1 headings, want exactly one", route, len(headings))
			continue
		}
		if strings.TrimSpace(headings[0][1]) == "" {
			t.Errorf("data-view %q has an empty h1 heading", route)
		}
	}
	for route, found := range routes {
		if !found {
			t.Errorf("embedded UI is missing data-view %q", route)
		}
	}
	if count := strings.Count(html, `class="page-heading`); count != len(routes) {
		t.Errorf("embedded UI has %d page headings, want %d", count, len(routes))
	}
	if !strings.Contains(html, `class="section-heading`) {
		t.Error("embedded UI must expose a shared section heading contract")
	}

	for _, value := range []string{
		`const routeTitles = {`,
		`const main = document.getElementById("main-content");`,
		`main.setAttribute("aria-label", routeTitles[active]);`,
		"document.title = `${routeTitles[active]} · Dev Control Room`;",
		`document.querySelectorAll("[data-route]")`,
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("embedded UI route contract missing %q", value)
		}
	}
	if strings.Contains(javascript, "view-title") {
		t.Error("embedded UI route handling must not access view-title")
	}
	for _, value := range []string{`class="decision-strip`, `class="ledger`, `class="evidence-flow`, `class="ledger-row`} {
		if !strings.Contains(html+javascript, value) {
			t.Errorf("embedded UI v0.13 ledger structure missing %q", value)
		}
	}
	for _, value := range []string{
		".page-heading", ".section-heading", ".decision-strip", ".ledger", ".ledger-row",
		"@font-face", `font-family: "Pretendard Variable"`, `url("/ui/PretendardVariable.woff2")`, "font-display: swap",
		":focus-visible", "prefers-reduced-motion", "overscroll-behavior: contain",
	} {
		if !strings.Contains(css, value) {
			t.Errorf("embedded UI v0.13 presentation contract missing %q", value)
		}
	}
	if regexp.MustCompile(`(?i)transition\s*:\s*all\b`).MatchString(css) {
		t.Error("embedded UI must not use transition: all")
	}

	root := regexp.MustCompile(`:root\s*\{`)
	if count := len(root.FindAllString(css, -1)); count != 1 {
		t.Errorf("embedded UI CSS has %d :root blocks, want exactly one", count)
	}

	for _, value := range []string{
		`const editorFieldName = id => id.replace(/^edit-/, "");`,
		`name="${escapeHTML(options.name || editorFieldName(id))}"`,
		`autocomplete="off" value="${escapeHTML(value)}"`,
		`name="${escapeHTML(editorFieldName(id))}" autocomplete="off" rows="3"`,
		`id="edit-integration-kind" name="kind" autocomplete="off"`,
		`id="edit-launch-mode" name="launchMode" autocomplete="off"`,
		`id="edit-data-boundary" name="dataBoundary" autocomplete="off"`,
		`value === "unavailable" ? "unknown"`,
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("embedded UI dynamic editor contract missing %q", value)
		}
	}
	for _, value := range []string{
		`<caption>`, `<th scope="col">`,
		`id="expected-revision" name="expectedRevision" autocomplete="off"`,
		`id="handoff-model" name="model" autocomplete="off"`,
		`data-runbook-param="${escapeHTML(parameter)}" data-runbook-id="${escapeHTML(item.id)}" name="${escapeHTML(parameter)}" autocomplete="off"`,
		`id="unregister-confirmation" name="confirmation" autocomplete="off"`,
	} {
		if !strings.Contains(javascript+html, value) {
			t.Errorf("embedded UI field/table contract missing %q", value)
		}
	}
	if strings.Contains(javascript, `<strong>${escapeHTML(localize(ordered[0].spec.summary))}</strong>`) {
		t.Error("Home next action must link to the finding without repeating its summary")
	}
}

func TestEmbeddedUIAssuranceEvidenceLedgerContract(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	html := embeddedUIAsset(t, service, "/", "text/html")
	for _, value := range []string{
		`data-route="assurance"`, `data-view="assurance" hidden`, "검증 기준과 효과 집계",
		`id="assurance-filter-title"`, `id="assurance-provider-filter"`, `id="assurance-model-filter"`,
		`id="assurance-runs"`, `id="assurance-invocations"`, `id="assurance-effects"`, `id="assurance-artifacts"`,
		"효과 근거", "근거 보관", "원문은 수집하지 않음", "보관 용량",
		"assurance-demo-board", "assurance-demo-banner", "예시 데이터", "실제 기록으로 돌아가기",
		`class="filter-bar"`, `class="assurance-ledger"`, `class="assurance-records"`,
	} {
		if !strings.Contains(html, value) {
			t.Errorf("embedded Assurance evidence-ledger HTML missing %q", value)
		}
	}

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	for _, value := range []string{
		"assuranceDashboardPath", "renderAssuranceDashboard", "renderAssuranceBenefits", "refreshAssuranceFilter",
		"applyAssuranceRouteState", "syncAssuranceRouteState", `history.replaceState(null, "", serialized ? `,
		"/api/assurance/dashboard", "/api/assurance/runs", "/api/assurance/invocations",
		"/api/assurance/artifacts", "/api/assurance/effects", "assurance-provider-filter", "assurance-model-filter",
		"Agent 실행", "효과 기록", "재실행", "근거 연결", "비용 경계", "회귀 방지", "예상 시간 절감", "time_saved_estimated", "근거 흐름",
		"const hasActivity = trend.some", "추세를 만들 표본이 없습니다.",
		"rawTranscript", "usageComplete", "estimatedCost", "configDigest", "artifactIds", "evidenceIds",
		"function renderHomeAssurance(dashboard, runs)", "home-assurance-proof", "효과 추적 보기", "원본·artifact·재검증 연결",
		"invocation-retry", "data-assurance-retry", "/retry", "원래 prompt는 저장하지 않습니다.", "중단 실행을 재시도했습니다.",
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("embedded Assurance evidence-ledger JavaScript missing %q", value)
		}
	}
	if strings.Contains(javascript, "JSON.stringify(spec.structured)") || strings.Contains(javascript, "spec.transcript") {
		t.Error("embedded Assurance dashboard must not expose raw transcript or structured provider output")
	}

	css := embeddedUIAsset(t, service, "/ui/app.css", "text/css")
	for _, value := range []string{".filter-bar", ".assurance-ledger", ".assurance-records", ".assurance-record", ".trace-inspector", ".artifact-storage", ".demo-board", ".demo-panel", ".demo-trend-row"} {
		if !strings.Contains(css, value) {
			t.Errorf("embedded Assurance evidence-ledger CSS missing %q", value)
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
		`data-home-established-only hidden`,
	} {
		if !strings.Contains(html, value) {
			t.Errorf("embedded UI keyboard contract missing %q", value)
		}
	}
	mainTag := regexp.MustCompile(`(?is)<main\b[^>]*>`).FindString(html)
	if !strings.Contains(mainTag, `id="main-content"`) || !strings.Contains(mainTag, `aria-label="개선"`) {
		t.Error("embedded UI main must retain the initial 개선 aria-label")
	}

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	for _, value := range []string{
		"let routeFocusTimer = 0",
		"let hasMountedRoute = false",
		`if (candidate === "main-content") return activeRoute`,
		`document.querySelector(".skip-link").addEventListener("click"`,
		"event.preventDefault()",
		"window.clearTimeout(routeFocusTimer)",
		"routeFocusTimer = window.setTimeout(() => {",
		"function focusElementByID(id)",
		`focusElementByID("main-content")`,
		"if (focusPendingFinding()) return;",
		"const shouldMoveRouteFocus = hasMountedRoute",
		"if (shouldMoveRouteFocus || focusTarget)",
		`history.replaceState(null, "", "#home")`,
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("embedded UI route focus contract missing %q", value)
		}
	}
}

func TestEmbeddedUIDialogFocusAndDescriptionContract(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	html := embeddedUIAsset(t, service, "/", "text/html")
	for _, value := range []string{
		`id="unregister-dialog" aria-labelledby="unregister-title" aria-describedby="unregister-description unregister-safety"`,
		`id="editor-dialog" aria-labelledby="editor-title" aria-describedby="editor-description"`,
	} {
		if !strings.Contains(html, value) {
			t.Errorf("embedded UI dialog contract missing %q", value)
		}
	}

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	for _, value := range []string{
		"let registerOpener = null", "let unregisterOpener = null", "let editorOpener = null", "function restoreDialogFocus(opener)",
		"editorOpener = document.activeElement", "unregisterOpener = button", "restoreDialogFocus(editorOpener)", "restoreDialogFocus(unregisterOpener)", "restoreDialogFocus(registerOpener)",
		"editorDialog.addEventListener(\"cancel\"", "unregisterDialog.addEventListener(\"cancel\"", "event.preventDefault();",
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("embedded UI dialog JavaScript missing %q", value)
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
		`<h2 id="environment-title">필수 도구</h2>`,
		`<div id="environment" class="ledger" aria-live="polite">`,
		"필요한 도구만 연결해 사용합니다.",
		`<div id="provider-statuses" class="ledger" tabindex="-1" aria-live="polite">`,
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
		"필수 도구의 상태는 아래에서 확인합니다.",
		"new Map((state.providerStatuses || []).map",
		"data-provider-capability=",
		"진단 세부 정보",
		"data-provider-recovery",
		`data-focus-target="provider-statuses"`,
		"aria-label=\"${escapeHTML(item.provider)} 진단\"",
		">진단</a>",
		`currentRoute() === "diagnostics" && location.hash === "#diagnostics"`,
		"focusElementByID(focusTarget)",
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
		`data-home-established-only hidden`, `id="home-next-action-section"`, "시작하기", "프로젝트 등록",
	} {
		if !strings.Contains(html, value) {
			t.Errorf("embedded UI first-use HTML missing %q", value)
		}
	}

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	for _, value := range []string{
		`document.querySelectorAll("[data-home-established-only]")`,
		`const [path, query = ""] = location.hash.slice(1).split("?", 2);`,
		"data-finding-id=",
		"pendingFindingID",
		"visibleFindings.map(item => findingCard(item))",
		"pendingProjectFocusID",
		"function focusPendingProject()",
		"if (active === \"projects\" && initialized) renderProjects();",
		`aria-pressed="${selected}"`,
		"startup: \"시작\"",
		"aria-expanded=\"${expanded}\"",
		"aria-expanded=\"${resultVisible}\"",
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("embedded UI first-use/finding JavaScript missing %q", value)
		}
	}
	for _, contract := range []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{
			name:    "finding query is read from the hash route",
			pattern: regexp.MustCompile(`(?s)const routeState = \(\) => \{.*?location\.hash\.slice\(1\)\.split\("\?",\s*2\).*?new URLSearchParams\(query\).*?\.get\("finding"\)`),
		},
		{
			name:    "finding deep-link moves focus to the matching card",
			pattern: regexp.MustCompile(`(?s)function focusPendingFinding\(\).*?document\.getElementById\(\x60finding-\$\{encode\(pendingFindingID\)\}\x60\).*?\.focus\(\{\s*preventScroll:\s*true\s*\}\).*?pendingFindingID = ""`),
		},
		{
			name:    "finding action preserves project and finding query parameters",
			pattern: regexp.MustCompile(`(?s)if \(kind === "Finding"\).*?href:.*?\?finding=.*?encode\(referenceID\)`),
		},
	} {
		if !contract.pattern.MatchString(javascript) {
			t.Errorf("embedded UI %s contract missing", contract.name)
		}
	}
	if strings.Contains(javascript, `href="#projects">확인 항목 열기`) {
		t.Error("finding CTA must retain its target instead of linking to a generic project list")
	}
}

func TestEmbeddedUIPretendardFontAssetContract(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	recorder := embeddedUIResponse(t, service, "/ui/PretendardVariable.woff2")
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /ui/PretendardVariable.woff2 = %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "font/woff2" {
		t.Fatalf("font Content-Type = %q, want font/woff2", got)
	}
	body := recorder.Body.Bytes()
	if len(body) < 1024 {
		t.Fatalf("font asset is only %d bytes; expected a meaningful embedded WOFF2 asset", len(body))
	}
	if len(body) < 4 || !bytes.Equal(body[:4], []byte("wOF2")) {
		t.Fatalf("font asset does not have the WOFF2 signature")
	}
}

func embeddedUIResponse(t *testing.T, service *App, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func embeddedUIAsset(t *testing.T, service *App, path, contentType string) string {
	t.Helper()
	recorder := embeddedUIResponse(t, service, path)
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

func TestEmbeddedUIAssuranceInvocationRetryRouteRequiresMutationToken(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	body := []byte("{\"prompt\":\"retry\"}")
	assertUIError(t, service, http.MethodPost, "/api/assurance/invocations/interrupted/retry", body, "", "", http.StatusForbidden, contract.ErrorForbidden)
	assertUIError(t, service, http.MethodPost, "/api/assurance/invocations/interrupted/retry", body, service.mutationToken, "http://example.invalid", http.StatusForbidden, contract.ErrorForbidden)
	assertUIError(t, service, http.MethodPost, "/api/assurance/invocations/missing/retry", body, service.mutationToken, "http://127.0.0.1:38471", http.StatusNotFound, contract.ErrorNotFound)
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
