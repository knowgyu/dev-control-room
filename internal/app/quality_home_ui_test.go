package app

import (
	"strings"
	"testing"
)

func TestEmbeddedUIQualityHomeTargetFlowRendersOneSelectedTarget(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	renderStart := strings.Index(javascript, "function renderQualityHome")
	if renderStart < 0 {
		t.Fatal("embedded UI is missing the home target renderer")
	}
	renderEnd := strings.Index(javascript[renderStart:], "\n  function renderHome")
	if renderEnd < 0 {
		t.Fatal("embedded UI home target renderer has no end boundary")
	}
	renderer := javascript[renderStart : renderStart+renderEnd]
	for _, value := range []string{
		"const targets = targetOptions()",
		"targetSelect.disabled = !targets.length || Boolean(state.qualityRunPending)",
		"rememberTarget(target.value)",
		"home-next-state",
		"작업에서 구성 확인",
		"Git 상태 확인 항목입니다. 품질 점검 결과와 섞지 않았습니다.",
	} {
		if !strings.Contains(renderer, value) {
			t.Errorf("home target renderer missing %q", value)
		}
	}
	if strings.Contains(renderer, "quality-queue-list") || strings.Contains(renderer, "queue.map(renderQualityQueueItem)") {
		t.Fatal("home target renderer still uses the retired queue surface")
	}
}

func TestEmbeddedUIServiceRecoveryKeepsStaleDataAndOffersRefresh(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	html := embeddedUIAsset(t, service, "/", "text/html")
	for _, value := range []string{
		"function renderServiceRecovery()",
		`state.service.status === "offline"`,
		`data-service-refresh`,
		"Promise.all(requests.map",
		"state.service = failed.length === 0",
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("service recovery contract missing %q", value)
		}
	}
	if !strings.Contains(html, `id="service-recovery"`) {
		t.Fatal("embedded UI is missing the single service recovery band")
	}
}

func TestEmbeddedUIQualityObjectiveDetailRouteAndReadContract(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	html := embeddedUIAsset(t, service, "/ui/index.html", "text/html")
	for _, value := range []string{
		`objectiveID = params.get("objective") || ""`,
		`state.qualityObjective.selectedID = active === "home" ? decodeURIComponentSafe(target.objectiveID) : ""`,
		"href: referenceID ? `#home?objective=${encode(referenceID)}` : \"#home\"",
		"request(`/api/quality/objectives/${encode(selectedID)}`)",
		`if (detail.status === "not_found")`,
		`id="quality-objective-detail"`,
		`aria-busy="false" hidden`,
	} {
		if !strings.Contains(javascript+html, value) {
			t.Errorf("QualityObjective detail route/read contract missing %q", value)
		}
	}
	if strings.Contains(javascript, `href: projectPath }`) {
		t.Fatal("QualityObjective queue action still redirects to Projects")
	}
}

func TestEmbeddedUIQualityObjectiveMutationContract(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	for _, value := range []string{
		`headers: mutationHeaders()`,
		`let payload = { expectedRevision }`,
		`payload.actor = String(draft.actor || "").trim()`,
		`payload.action = String(draft.action || "").trim()`,
		`payload.reason = String(draft.reason || "").trim()`,
		"path = `/api/quality/objectives/${encode(id)}/decision`",
		"path = `/api/quality/objectives/${encode(id)}/revalidations`",
		`payload.findingId = sourceID`,
		`payload.qualityRunId = sourceID`,
		`body: JSON.stringify({ expectedRevision })`,
		"/api/quality/objectives/${encode(id)}/confirm",
		`다른 화면에서 과제가 변경되었습니다. 최신 상태를 다시 불러온 뒤 확인하세요.`,
		`await Promise.all([loadQualityHome(), loadQualityObjective(id, true)])`,
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("QualityObjective mutation contract missing %q", value)
		}
	}

	mutationStart := strings.Index(javascript, `document.addEventListener("submit"`)
	if mutationStart < 0 {
		t.Fatal("QualityObjective mutation submit handler is missing")
	}
	mutationEnd := strings.Index(javascript[mutationStart:], "\n\n  async function loadAssuranceTrace")
	if mutationEnd < 0 {
		t.Fatal("QualityObjective mutation submit handler has no end boundary")
	}
	mutation := javascript[mutationStart : mutationStart+mutationEnd]
	for _, forged := range []string{"payload.outcome", "payload.head", "payload.configDigest"} {
		if strings.Contains(mutation, forged) {
			t.Fatalf("revalidation mutation forges server-derived field %q", forged)
		}
	}
}

func TestEmbeddedUIQualityObjectiveDetailStateAndConditionalControls(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	for _, value := range []string{
		`const canConfirm = stateValue === "review" && latest?.outcome === "improved"`,
		`if (!spec.decision || ["adopted", "rejected", "stale"].includes(spec.state)) return ""`,
		`const qualityObjectiveStateLabel = value => qualityObjectiveStateLabels[value] || "상태 미상"`,
		`const qualityObjectiveOutcomeLabel = value => qualityObjectiveOutcomeLabels[value] || "결과 미상"`,
		`stateValue === "stale"`,
		`stateValue === "review"`,
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("QualityObjective state contract missing %q", value)
		}
	}
}
