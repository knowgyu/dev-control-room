package app

import (
	"strings"
	"testing"
)

func TestEmbeddedUIQualityHomePopulatedQueueRendersOneSemanticState(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	itemStart := strings.Index(javascript, "function renderQualityQueueItem")
	if itemStart < 0 {
		t.Fatal("embedded UI is missing the quality queue renderer")
	}
	itemEnd := strings.Index(javascript[itemStart:], "\n  function renderQualityHome")
	if itemEnd < 0 {
		t.Fatal("embedded UI quality queue renderer has no end boundary")
	}
	itemRenderer := javascript[itemStart : itemStart+itemEnd]
	if got := strings.Count(itemRenderer, "stateText(qualityQueueStateLabel(stateValue), tone)"); got != 1 {
		t.Fatalf("quality queue item renders semantic state %d times, want once", got)
	}
	if strings.Contains(itemRenderer, `quality-queue-item__heading`) && strings.Contains(itemRenderer, `class="chip`) {
		t.Fatal("quality queue item still renders a duplicate visible state chip")
	}

	renderStart := strings.Index(javascript, "function renderQualityHome")
	if renderStart < 0 {
		t.Fatal("embedded UI is missing the QualityHome renderer")
	}
	renderEnd := strings.Index(javascript[renderStart:], "\n  function renderHome")
	if renderEnd < 0 {
		t.Fatal("embedded UI QualityHome renderer has no end boundary")
	}
	renderer := javascript[renderStart : renderStart+renderEnd]
	for _, value := range []string{
		"quality-queue-list",
		"queue.map(renderQualityQueueItem)",
		"quality-queue-next-state",
		`qualityHome.status === "ready"`,
	} {
		if !strings.Contains(renderer, value) {
			t.Errorf("populated QualityHome renderer missing %q", value)
		}
	}
}

func TestEmbeddedUIQualityHomeEndpointErrorShowsRetryableAlert(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	loadStart := strings.Index(javascript, "async function loadQualityHome")
	if loadStart < 0 {
		t.Fatal("embedded UI is missing the QualityHome loader")
	}
	loadEnd := strings.Index(javascript[loadStart:], "\n\n  async function loadAssuranceTrace")
	if loadEnd < 0 {
		t.Fatal("embedded UI QualityHome loader has no end boundary")
	}
	loader := javascript[loadStart : loadStart+loadEnd]
	for _, value := range []string{
		`request("/api/quality/home")`,
		`status: "error"`,
		"normalizeQualityHomeError(error)",
		"renderQualityHome()",
	} {
		if !strings.Contains(loader, value) {
			t.Errorf("QualityHome endpoint-error loader missing %q", value)
		}
	}

	renderStart := strings.Index(javascript, "function renderQualityHome")
	if renderStart < 0 {
		t.Fatal("embedded UI is missing the QualityHome renderer")
	}
	renderEnd := strings.Index(javascript[renderStart:], "\n  function renderHome")
	if renderEnd < 0 {
		t.Fatal("embedded UI QualityHome renderer has no end boundary")
	}
	renderer := javascript[renderStart : renderStart+renderEnd]
	for _, value := range []string{
		`if (failed)`,
		`class="quality-queue-error" role="alert"`,
		"data-quality-retry",
		"qualityHome.error",
		"다른 프로젝트·실행·활동 데이터는 계속 확인할 수 있습니다.",
	} {
		if !strings.Contains(renderer, value) {
			t.Errorf("endpoint-error UI missing %q", value)
		}
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
