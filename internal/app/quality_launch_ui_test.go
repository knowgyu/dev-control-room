package app

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestEmbeddedUIWorkCodeCheckSurfaceUsesPlainTargetFlow(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	html := embeddedUIAsset(t, service, "/", "text/html")
	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	ui := html + javascript
	for _, value := range []string{
		`id="quality-work-surface"`,
		`id="quality-target"`,
		`id="quality-techniques"`,
		`id="quality-run-results"`,
		`id="advanced-work"`,
		"코드 검사",
		"Go 정적 검사 (go vet)",
		"테스트·커버리지",
		"저장소 상태 새로 고침",
	} {
		if !strings.Contains(ui, value) {
			t.Errorf("work code-check surface missing %q", value)
		}
	}
	for _, retired := range []string{`id="quality-launch"`, `id="quality-launch-form"`, "Campaign 만들고 Quality Run", "Checkset이 아니라 이 Campaign"} {
		if strings.Contains(html, retired) {
			t.Errorf("work surface still exposes retired quality-launch copy %q", retired)
		}
	}
}

func TestEmbeddedUIWorkCodeCheckUsesAssuranceRunAndReusesCampaign(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	for _, value := range []string{
		"const primaryQualityTechniques = [",
		`id: "static_security"`,
		`label: "Go 정적 검사 (go vet)"`,
		`command: "go vet -mod=readonly ./..."`,
		`id: "go_test_coverage"`,
		`command: "go test -mod=readonly -count=1 -covermode=set -coverprofile=<자동 생성> ./..."`,
		"async function ensureQualityCampaign(target)",
		`request("/api/assurance/campaigns", {`,
		`request("/api/assurance/runs", {`,
		"campaignId: campaign.metadata?.id",
		"body: JSON.stringify({ campaignId: campaign.metadata?.id, technique: techniqueID })",
		"selected-target",
		"#assurance?run=",
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("work code-check JavaScript missing %q", value)
		}
	}
	if got := strings.Count(javascript, `request("/api/assurance/runs", {`); got != 1 {
		t.Fatalf("work code-check surface submits %d assurance run requests, want one", got)
	}
	if strings.Contains(javascript, "qualityLaunchTechniqueLabels") || strings.Contains(javascript, "renderQualityLaunch") {
		t.Fatal("work code-check surface still contains the retired launch flow")
	}
}

func TestEmbeddedUIWorkCodeCheckBindsPendingAndRecoveryToTargetAndTechnique(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	renderStart := strings.Index(javascript, "function renderQualityWorkSurface")
	renderEnd := strings.Index(javascript[renderStart:], "\n  function renderAssuranceBenefits")
	if renderStart < 0 || renderEnd < 0 {
		t.Fatal("embedded UI is missing the bounded Work code-check renderer")
	}
	renderer := javascript[renderStart : renderStart+renderEnd]
	for _, value := range []string{
		"state.qualityRunPending === technique.id && state.qualityRunTargetValue === target.value",
		"Boolean(state.qualityRunPending)",
		"state.qualityRunErrorTargetValue === target.value",
		"disabled = offline || Boolean(state.qualityRunPending)",
	} {
		if !strings.Contains(renderer, value) {
			t.Errorf("Work renderer missing target/technique binding %q", value)
		}
	}
	runnerStart := strings.Index(javascript, "async function runQualityTechnique")
	runnerEnd := strings.Index(javascript[runnerStart:], "\n\n  let loadingQualityHome")
	if runnerStart < 0 || runnerEnd < 0 {
		t.Fatal("embedded UI is missing the bounded Work code-check runner")
	}
	runner := javascript[runnerStart : runnerStart+runnerEnd]
	previous := strings.Index(runner, "const previousTechniqueIDs")
	postTry := strings.Index(runner, "try {")
	if previous < 0 || postTry < 0 || previous > postTry {
		t.Fatal("same-technique result snapshot must be captured before the POST try block")
	}
	if !strings.Contains(runner, "await refreshQualityResultData();") || !strings.Contains(runner, "Only GET is retried here; an unknown POST outcome is never submitted twice.") {
		t.Fatal("failed or unavailable runs must refresh persisted results without retrying the POST")
	}
}

func TestEmbeddedUIWorkCodeCheckDoesNotRetryUnknownPOSTOutcome(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	runnerStart := strings.Index(javascript, "async function runQualityTechnique")
	runnerEnd := strings.Index(javascript[runnerStart:], "\n\n  let loadingQualityHome")
	if runnerStart < 0 || runnerEnd < 0 {
		t.Fatal("embedded UI is missing the Work code-check runner")
	}
	runner := javascript[runnerStart : runnerStart+runnerEnd]
	if got := strings.Count(runner, `request("/api/assurance/runs", {`); got != 1 {
		t.Fatalf("unknown POST outcome path submits %d assurance run requests, want one", got)
	}
	if !strings.Contains(runner, "state.qualityRunErrorTargetValue = target.value") {
		t.Fatal("run errors must remain scoped to the target that was selected")
	}
}

func TestEmbeddedUIQualitySetupUsesReadOnlyTargetContract(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	html := embeddedUIAsset(t, service, "/", "text/html")
	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	for _, value := range []string{
		`id="quality-setup"`,
		`data-quality-language-mode`,
		`data-quality-language`,
		"언어와 구성 요소를 확인하세요.",
		"감지 결과가 다르면 언어를 직접 선택하세요.",
		"선택하지 않은 구성도 목록에 남습니다.",
		"도구를 설치하거나 실행하지 않습니다.",
		"구성 확인",
		"현재 버전은 준비 방법까지 안내합니다. Python·Vue 검사 실행은 아직 지원하지 않습니다.",
	} {
		if !strings.Contains(html+javascript, value) {
			t.Errorf("M1 quality setup UI missing %q", value)
		}
	}
	for _, value := range []string{
		"const qualitySetupPath = (target, preference)",
		"projectId: target.projectID",
		"repositoryId: target.repositoryID",
		"worktreeId: target.worktreeID",
		`query.set("languages", qualitySetupList(preference.languages).join(","))`,
		"let qualitySetupRequestSequence = 0",
		"loadQualitySetupForSelectedTarget(force)",
		"state.selectedTargetValue !== target.value",
		"existing_configuration",
		"setup_needed",
		"unsupported",
		"ambiguous",
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("M1 quality setup contract missing %q", value)
		}
	}
	if got := strings.Count(javascript, `request(currentRequestKey)`); got != 1 {
		t.Fatalf("quality setup should issue one read-only request site, got %d", got)
	}
	if strings.Contains(javascript, `request("/api/quality/setup", {`) {
		t.Fatal("quality setup must not mutate or install from the preview flow")
	}
}

func TestEmbeddedUIQualitySetupKeepsEvidenceAndGatesGoExecution(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	for _, value := range []string{
		"targetValue: target?.value || \"\"",
		"head: String(data?.head || target?.head || \"\")",
		"digest: String(data?.digest || target?.digest || \"\")",
		"components.map(component => renderQualitySetupComponent(component",
		".filter(check => qualitySetupCheckIsRelevant(check, component, preference))",
		"const goRunnable = Boolean(setupData && qualitySetupHasRunnableGo(setupData, setupPreference));",
		"(path === \".\" || path === \"\")",
		"techniqueContainer.innerHTML = goRunnable",
		"if (resultsSection) resultsSection.hidden = !runs.length && !warning",
		"HEAD 또는 구성 근거가 바뀌어 언어 선택을 자동 감지로 초기화했습니다.",
		"구성 확인됨은 실행 완료를 뜻하지 않습니다.",
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("M1 evidence/Go gate contract missing %q", value)
		}
	}
	renderStart := strings.Index(javascript, "const renderQualitySetup =")
	renderEnd := strings.Index(javascript[renderStart:], "\n  const qualityRunInlineResult")
	if renderStart < 0 || renderEnd < 0 {
		t.Fatal("embedded UI is missing the bounded setup renderer")
	}
	setupRenderer := javascript[renderStart : renderStart+renderEnd]
	if strings.Contains(setupRenderer, `data-quality-run=`) || strings.Contains(setupRenderer, `method: "POST"`) {
		t.Fatal("setup evidence renderer must not expose execution or installation controls")
	}
	if strings.Contains(javascript, "data-quality-ai") || strings.Contains(javascript, "quality-setup-ai") {
		t.Fatal("M1 setup must not add a pretend AI control")
	}
	if strings.Contains(javascript, "이 대상에 표시할 Go 실행은 없습니다") {
		t.Fatal("non-Go targets must not receive an irrelevant Go empty state")
	}
}

func TestEmbeddedUIQualitySetupBehavior(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for the UI behavior tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, node, "--test", "quality_setup_ui.test.cjs").CombinedOutput()
	if err != nil {
		t.Fatalf("quality setup UI behavior: %v\n%s", err, output)
	}
}
