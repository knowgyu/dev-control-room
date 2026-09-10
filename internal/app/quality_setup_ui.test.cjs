const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

// Evaluate the shipped renderer and loader without starting the application's
// global refresh loop. Fixtures below are API responses, never product data.
const source = fs.readFileSync(path.join(__dirname, "ui/app.js"), "utf8");
function sourceBlock(start, end) {
  const from = source.indexOf(start);
  const to = source.indexOf(end, from);
  assert.ok(from >= 0 && to > from, `missing UI boundary: ${start}`);
  return source.slice(from, to);
}
const uiSource = [
  sourceBlock("  const escapeHTML =", "  const findingTone ="),
  sourceBlock("  const qualitySetupLanguageLabels =", "  const rememberTarget ="),
  sourceBlock("  const primaryQualityTechniques =", "  function renderAssuranceBenefits"),
  sourceBlock("  let qualitySetupRequestSequence =", "  let loadingQualityHome ="),
].join("\n");
const target = { value: "p|r|w", projectID: "p", repositoryID: "r", worktreeID: "w", label: "프로젝트 · main", head: "head-1" };
const checks = [
  { id: "ruff", status: "existing_configuration", reason: "configuration observed; runtime readiness is not checked" },
  { id: "pytest", status: "setup_needed", reason: "configuration not found within inspected scope" },
  { id: "go-test", label: "Go checks", status: "unsupported", reason: "Go runtime readiness is not assessed by setup detection" },
];
const mixedReport = (digest = "digest-1") => ({
  head: "head-1", digest, detectedLanguages: ["python", "go", "javascript"], partial: false,
  warnings: ["excluded child directories were not inspected"],
  components: [
    { path: ".", languages: ["python", "go"], evidence: [{ path: "go.mod", kind: "manifest" }], checks: structuredClone(checks) },
    { path: "frontend", languages: ["javascript"], evidence: [{ path: "frontend/package.json", kind: "manifest" }], checks: [{ id: "script:test", status: "existing_configuration" }] },
  ],
});
function harness({ storage = new Map(), storageBlocked = false, respond = () => mixedReport() } = {}) {
  const elements = new Map();
  const resultsSection = { hidden: false };
  for (const id of ["quality-work-surface", "quality-target", "quality-target-meta", "quality-setup", "quality-techniques", "quality-run-results"]) {
    elements.set(id, { innerHTML: "", closest: () => resultsSection });
  }
  const state = {
    selectedTargetValue: target.value, service: { status: "ready" }, snapshot: { projects: [] },
    qualitySetup: { status: "idle", data: null }, qualityCampaigns: [], qualityCampaignsStatus: "ready",
    qualityRunPending: "", qualityRunError: "", runs: [],
  };
  const requests = [];
  const context = vm.createContext({
    state, URLSearchParams,
    window: { localStorage: {
      getItem: key => { if (storageBlocked) throw new Error("storage blocked"); return storage.get(key) || null; },
      setItem: (key, value) => { if (storageBlocked) throw new Error("storage blocked"); storage.set(key, value); },
    } },
    document: { getElementById: id => elements.get(id) },
    targetOptions: () => [target], selectedTarget: () => target,
    rememberTarget: value => { state.selectedTargetValue = value; },
    runsForTarget: () => state.runs,
    assuranceTechniqueLabels: {}, qualityRunStatusText: () => "완료",
    qualityRunUnavailableText: () => "사용할 수 없음", formatDate: value => value || "", formatCount: String,
    request: async url => { requests.push(url); assert.ok(requests.length <= 3, "unbounded setup reload"); return respond(url, requests.length); },
  });
  vm.runInContext(uiSource + `\nthis.ui = {
    loadQualitySetupForTarget, saveQualitySetupPreference, qualitySetupPreferenceForTarget,
    qualitySetupHasRunnableGo, renderQualityWorkSurface, renderQualitySetupCheck,
    renderQualitySetupComponent, qualitySetupCheckIsRelevant
  };`, context);
  return { ui: context.ui, state, elements, requests, resultsSection, storage };
}
const manualPython = { mode: "manual", languages: ["python"] };
const filtered = (report, languages) => ({ ...report, selectedLanguages: languages, components: report.components.map(component => ({
  ...component, checks: component.checks.filter(check => languages.includes("python") ? ["ruff", "pytest"].includes(check.id) : true),
})) });
const setupHTML = h => h.elements.get("quality-setup").innerHTML;

test("reload restores manual Python, re-queries once, and retains mixed observed components", async () => {
  const saved = harness();
  saved.ui.saveQualitySetupPreference(target, mixedReport(), manualPython);
  const h = harness({ storage: saved.storage, respond: url => url.includes("languages=") ? filtered(mixedReport(), ["python"]) : mixedReport() });
  await h.ui.loadQualitySetupForTarget(target);
  assert.equal(h.requests.length, 2);
  assert.equal(new URL(h.requests[0], "http://localhost").searchParams.has("languages"), false);
  assert.equal(new URL(h.requests[1], "http://localhost").searchParams.get("languages"), "python");
  assert.equal(h.state.qualitySetup.preferenceReset, false);
  assert.equal(h.ui.qualitySetupPreferenceForTarget(target, h.state.qualitySetup.data).mode, "manual");
  assert.match(setupHTML(h), /Python 코드 검사/);
  assert.match(setupHTML(h), /frontend\/package.json/);
  assert.doesNotMatch(setupHTML(h), /<h4>Go 구성|<h4>test 스크립트/);
  assert.equal(h.elements.get("quality-techniques").innerHTML, "");
  assert.equal(h.resultsSection.hidden, true);
});

test("same-root component filters each check before applying another request", () => {
  const h = harness();
  const component = mixedReport().components[0];
  const rendered = h.ui.renderQualitySetupComponent(component, manualPython);
  assert.match(rendered, /Python 코드 검사/);
  assert.doesNotMatch(rendered, /<h4>Go 구성/);
  assert.equal(h.ui.qualitySetupHasRunnableGo(mixedReport(), manualPython), false);
  assert.equal(h.ui.qualitySetupHasRunnableGo(mixedReport(), { mode: "manual", languages: ["go"] }), true);
  assert.equal(h.ui.qualitySetupHasRunnableGo({ detectedLanguages: [], components: [] }, { mode: "manual", languages: ["go"] }), false);
  assert.equal(h.ui.qualitySetupHasRunnableGo({ detectedLanguages: ["go"], components: [{ path: "backend", languages: ["go"] }] }, { mode: "auto" }), false);
});

test("changed config during a manual response re-fetches unfiltered data and resets preference", async () => {
  const h = harness({ respond: (url, call) => call === 1 ? filtered(mixedReport("digest-2"), ["python"]) : mixedReport("digest-2") });
  h.ui.saveQualitySetupPreference(target, mixedReport(), manualPython);
  h.state.qualitySetup = { status: "ready", targetValue: target.value, data: mixedReport() };
  await h.ui.loadQualitySetupForTarget(target, true);
  assert.equal(h.requests.length, 2);
  assert.ok(h.requests[0].includes("languages=python"));
  assert.ok(!h.requests[1].includes("languages="));
  assert.equal(h.state.qualitySetup.preferenceReset, true);
  assert.equal(h.ui.qualitySetupPreferenceForTarget(target, h.state.qualitySetup.data).mode, "auto");
  assert.match(setupHTML(h), /test 스크립트/);
  assert.match(h.elements.get("quality-techniques").innerHTML, /data-quality-run=/);
});

test("reload with changed evidence does not restore the old manual filter", async () => {
  const saved = harness();
  saved.ui.saveQualitySetupPreference(target, mixedReport(), manualPython);
  const h = harness({ storage: saved.storage, respond: () => mixedReport("digest-new") });
  await h.ui.loadQualitySetupForTarget(target);
  assert.equal(h.requests.length, 1);
  assert.equal(h.state.qualitySetup.preferenceReset, true);
  assert.equal(h.ui.qualitySetupPreferenceForTarget(target, h.state.qualitySetup.data).mode, "auto");
});

test("blocked localStorage still preserves and changes the in-page language preference", () => {
  const h = harness({ storageBlocked: true });
  h.ui.saveQualitySetupPreference(target, mixedReport(), manualPython);
  assert.equal(h.ui.qualitySetupPreferenceForTarget(target, mixedReport()).mode, "manual");
  assert.equal(h.ui.qualitySetupHasRunnableGo(mixedReport(), h.ui.qualitySetupPreferenceForTarget(target, mixedReport())), false);
  h.ui.saveQualitySetupPreference(target, mixedReport(), { mode: "auto", languages: [] });
  assert.equal(h.ui.qualitySetupPreferenceForTarget(target, mixedReport()).mode, "auto");
});

test("normal exclusions stay in collapsed details and an empty report has no positive result", async () => {
  const h = harness({ respond: () => ({ head: "head-1", digest: "empty", components: [], detectedLanguages: [], partial: false, warnings: ["excluded child directories were not inspected"] }) });
  await h.ui.loadQualitySetupForTarget(target);
  const html = setupHTML(h);
  assert.match(html, /감지된 구성 없음/);
  assert.doesNotMatch(html, /data-tone="positive"|surface-error|role="alert"/);
  assert.match(html, /<details class="quality-setup-scope"><summary>확인 범위와 참고 사항/);
  assert.ok(html.indexOf(".git") > html.indexOf('class="quality-setup-scope"'));
  assert.doesNotMatch(html, /excluded child directories|언어를 직접 선택해 다시 확인/);
  assert.match(html, /설정 파일을 만들지 않습니다/);
});

test("root Go uses the actual action panel while nested Go remains setup-only", async () => {
  const root = harness();
  await root.ui.loadQualitySetupForTarget(target);
  assert.match(root.elements.get("quality-techniques").innerHTML, /Go 정적 검사 \(go vet\)/);
  assert.doesNotMatch(setupHTML(root), /<h4>Go 구성|Go runtime readiness|지원 범위 밖/);
  const nested = root.ui.renderQualitySetupComponent({ path: "backend", languages: ["go"], checks: [checks[2]] }, { mode: "auto" });
  assert.match(nested, /구성만 확인|저장소 최상위 폴더만 지원/);
  assert.doesNotMatch(nested, /runtime readiness|data-quality-run=/);
});

test("script labels do not invent a test tool; unknown details never leak into primary copy", async () => {
  const h = harness({ respond: () => ({ ...mixedReport(), partial: true, warnings: ["unfamiliar internal detail secret-value", 'unreadable or unsafe evidence: <img src=x onerror=alert(1)>'] }) });
  await h.ui.loadQualitySetupForTarget(target);
  const script = h.ui.renderQualitySetupCheck({ id: "script:test", label: "Vitest", status: "existing_configuration", reason: "unknown runner secret-value" });
  assert.match(script, /<h4>test 스크립트/);
  assert.doesNotMatch(script, /Vitest|unknown runner|secret-value/);
  const html = setupHTML(h);
  assert.doesNotMatch(html, /unfamiliar internal|secret-value|<img src=/);
  assert.match(html, /&lt;img src=x onerror=alert\(1\)&gt;/);
  const preview = h.ui.renderQualitySetupCheck({ id: "ruff", status: "ambiguous", reason: "internal detail", commandPreview: '<script>bad()</script>', setupPreview: ['<img src=x>'] });
  assert.match(preview, /&lt;script&gt;/);
  assert.doesNotMatch(preview, /<script>|<img|internal detail|data-quality-run=/);
});

test("late response cannot overwrite a newly selected target", async () => {
  let resolve;
  const h = harness({ respond: () => new Promise(done => { resolve = done; }) });
  const pending = h.ui.loadQualitySetupForTarget(target);
  h.state.selectedTargetValue = "other|repo|worktree";
  resolve(mixedReport());
  await pending;
  assert.notEqual(h.state.qualitySetup.status, "ready");
  assert.equal(h.state.qualitySetup.data, null);
});

test("historical runs remain visible for a target without applicable Go actions", async () => {
  const h = harness({ respond: () => ({ head: "head-1", digest: "python", detectedLanguages: ["python"], components: [] }) });
  h.state.runs = [{ metadata: { id: "saved-run" }, spec: { state: "succeeded", summary: "이전에 저장된 결과", startedAt: "2026-09-09" } }];
  await h.ui.loadQualitySetupForTarget(target);
  assert.equal(h.elements.get("quality-techniques").innerHTML, "");
  assert.equal(h.resultsSection.hidden, false);
  assert.match(h.elements.get("quality-run-results").innerHTML, /saved-run|이전에 저장된 결과/);
  h.state.service.status = "offline";
  h.ui.renderQualityWorkSurface();
  assert.match(setupHTML(h), /<fieldset class="quality-language-options" disabled>/);
  assert.match(h.elements.get("quality-run-results").innerHTML, /이전에 저장된 결과/);
});
