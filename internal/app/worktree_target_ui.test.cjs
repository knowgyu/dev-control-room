const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const source = fs.readFileSync(path.join(__dirname, "ui/app.js"), "utf8");
function sourceBlock(start, end) {
  const from = source.indexOf(start);
  const to = source.indexOf(end, from);
  assert.ok(from >= 0 && to > from, `missing UI boundary: ${start}`);
  return source.slice(from, to);
}

const targets = [
  { value: "project|repo|main", projectID: "project", repositoryID: "repo", worktreeID: "main", projectName: "프로젝트", repositoryName: "backend", label: "프로젝트 · backend · main" },
  { value: "project|repo|feature", projectID: "project", repositoryID: "repo", worktreeID: "feature", projectName: "프로젝트", repositoryName: "backend", label: "프로젝트 · backend · feature" },
];

const uiSource = [
  sourceBlock("  const escapeHTML =", "  const findingTone ="),
  sourceBlock("  const projectRepositories =", "  const decodeURIComponentSafe ="),
  sourceBlock("  function renderExternalOperations()", "  function environmentSource"),
  sourceBlock("  function renderWork()", "  function environmentSource"),
  sourceBlock("  function renderDiagnostics()", "  function renderSafeguardRule"),
  sourceBlock("  async function loadWorkData(force)", "  async function loadDiagnosticsData"),
].join("\n");
const changeSource = source.slice(
  source.lastIndexOf('  document.addEventListener("change", event => {'),
  source.indexOf('  document.addEventListener("keydown", event => {', source.lastIndexOf('  document.addEventListener("change", event => {')),
);

function optionValue(html, value) {
  const escaped = value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = html.match(new RegExp(`<option value="${escaped}"[^>]*>`));
  assert.ok(match, `missing option ${value}`);
  return match[0];
}

function harness({ selected = targets[1], proposals = [], request } = {}) {
  const elements = new Map();
  for (const id of [
    "quality-work-surface", "quality-target", "quality-target-meta", "quality-setup", "quality-techniques", "quality-run-results",
    "proposal-ui", "checksets", "action-ui", "operations-ui", "environment", "provider-statuses", "guidance-ui", "profile-list", "integration-list", "runbook-list", "cleanup-queue", "safeguards",
  ]) elements.set(id, { innerHTML: "", closest: () => ({ hidden: false }) });
  const selectors = ["discovery-target", "action-target", "external-target", "guidance-target", "runbook-target"]
    .map(id => ({ id, value: selected.value, options: targets.map(target => ({ value: target.value })), matches: selector => selector === "select[data-worktree-target]" }));
  const storage = new Map();
  let changeHandler;
  const state = {
    snapshot: { projects: [{ id: "project", name: "프로젝트", repos: [{ id: "repo", name: "backend", path: "C:\\src\\backend", worktrees: targets.map(target => ({ metadata: { id: target.worktreeID }, spec: { branch: target.worktreeID, head: `head-${target.worktreeID}` } })) }] }] },
    registryProjects: [], findings: [], events: [], environment: { generatedAt: "now", findings: [], tools: [], profiles: [], environment: [] }, providerStatuses: [],
    externalGroups: [], actionDetails: [], operationResults: {}, workItems: [], checkRuns: new Map(), expandedChecks: new Set(), expandedActions: new Set(),
    profiles: [], integrations: [], runbooks: [{ id: "runbook-1", name: "진단", scriptPath: "verify.ps1", timeoutSeconds: 5, parameters: [], environmentAllowlist: [] }], cleanup: [], safeguards: [], surfaceErrors: { checksets: "", actions: "", externalGroups: "", profiles: "", integrations: "", runbooks: "", cleanup: "", safeguards: "" },
    selectedTargetValue: selected.value, discoveryTargetValue: "", loading: { work: false }, loaded: { work: false }, service: { status: "ready" }, repositorySyncPlan: null, repositorySyncResult: null,
  };
  const context = vm.createContext({
    state, URLSearchParams, CSS: { escape: value => value },
    window: { localStorage: { getItem: key => storage.get(key) || null, setItem: (key, value) => storage.set(key, value), removeItem: key => storage.delete(key) } },
    document: {
      getElementById: id => elements.get(id),
      querySelectorAll: selector => selector.includes("worktree-target") ? selectors : [],
      querySelector: () => null,
      addEventListener: (type, handler) => { if (type === "change") changeHandler = handler; },
    },
    targetOptions: () => targets,
    loadSurface: async (key, loader, assign) => { try { assign(await loader()); } catch (error) { state.surfaceErrors[key] = error.message; } },
    renderQualityWorkSurface: () => {}, renderQualityTools: () => {}, renderProviderStatuses: () => {}, renderExternalGroups: () => {}, renderGuidanceResult: () => "",
    requiredEnvironmentReady: () => true, surfaceError: () => "", formatDate: value => value || "", formatCount: String, label: value => value || "알 수 없음", localize: value => value,
    renderRepositorySyncPlan: () => "",
    proposalCard: proposal => `<article data-proposal-id="${proposal.metadata.id}">${proposal.metadata.id}</article>`,
    checksetCard: checkset => `<article data-checkset-id="${checkset.metadata.id}">${checkset.metadata.id}</article>`,
    qualityInspectionScopeMatches: (item, target) => item.spec?.projectId === target?.projectID && item.spec?.repositoryId === target?.repositoryID && item.spec?.worktreeId === target?.worktreeID,
    isSpecialPlan: () => false, renderOperationResult: () => "",
    request: request || (async () => []),
  });
  vm.runInContext(`${uiSource}\n${changeSource}\nthis.ui = { renderWork, renderExternalOperations, renderDiagnostics, loadWorkData, selectedTarget, renderTargetOptions };`, context);
  return { ui: context.ui, state, elements, selectors, storage, get changeHandler() { return changeHandler; } };
}

test("shared Worktree selection renders selected options on every page surface", () => {
  const h = harness();
  h.ui.renderWork();
  assert.match(optionValue(h.elements.get("proposal-ui").innerHTML, targets[1].value), /selected/);
  assert.match(optionValue(h.elements.get("action-ui").innerHTML, targets[1].value), /selected/);
  assert.match(optionValue(h.elements.get("operations-ui").innerHTML, targets[1].value), /selected/);
  h.ui.renderDiagnostics();
  assert.match(optionValue(h.elements.get("guidance-ui").innerHTML, targets[1].value), /selected/);
  assert.match(optionValue(h.elements.get("runbook-list").innerHTML, targets[1].value), /selected/);
  assert.doesNotMatch(optionValue(h.elements.get("proposal-ui").innerHTML, targets[0].value), /selected/);
});

test("changing any shared Worktree selector persists and synchronizes all selectors", () => {
  const h = harness();
  for (const selector of h.selectors) {
    selector.value = targets[0].value;
    h.changeHandler({ target: selector });
    assert.equal(h.state.selectedTargetValue, targets[0].value);
    assert.equal(h.storage.get("dev-control-room.selected-target"), targets[0].value);
    for (const synced of h.selectors) assert.equal(synced.value, targets[0].value);
  }
});

test("discovery keeps its requested Worktree after reload and names an empty result", async () => {
  const requested = targets[1];
  const h = harness({ request: async url => {
    if (url.endsWith("/checksets")) return [];
    if (url.endsWith("/proposals")) return [{ metadata: { id: "main-proposal", name: "main check" }, spec: { projectId: "project", repositoryId: "repo", worktreeId: targets[0].worktreeID } }];
    if (url === "/api/actions/plans" || url === "/api/external-work-groups") return [];
    throw new Error(`unexpected request: ${url}`);
  }});
  h.state.discoveryTargetValue = requested.value;
  await h.ui.loadWorkData(true);
  assert.equal(h.state.selectedTargetValue, requested.value);
  const html = h.elements.get("proposal-ui").innerHTML;
  assert.match(html, /요청한 Worktree에서 기존 점검을 찾지 못했습니다/);
  assert.match(html, new RegExp(requested.label));
  assert.doesNotMatch(html, /main-proposal/);
});
