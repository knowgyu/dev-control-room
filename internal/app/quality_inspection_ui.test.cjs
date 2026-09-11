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

const uiSource = [
  sourceBlock("  const escapeHTML =", "  const findingTone ="),
  sourceBlock("  const qualityInspectionPlanStateLabels =", "  function renderQualityWorkSurface"),
  sourceBlock("  let loadingQualityInspection = false;", "  async function loadAssuranceMeasurementData"),
].join("\n");

const target = {
  value: "project|repository|worktree",
  projectID: "project",
  repositoryID: "repository",
  worktreeID: "worktree",
  repositoryName: "dev-control-room",
  repositoryPath: "C:\\src\\dev-control-room",
  branch: "main",
  head: "head-2",
  label: "Dev Control Room · main",
};
const plan = (state = "proposed") => ({
  metadata: { id: "plan-1", name: "deterministic inspection plan" },
  generation: { source: "deterministic", aiProposal: false, reason: "ai_proposal_not_configured" },
  spec: {
    projectId: target.projectID,
    repositoryId: target.repositoryID,
    worktreeId: target.worktreeID,
    branch: target.branch,
    head: target.head,
    state,
    revision: state === "proposed" ? 1 : 2,
    updatedAt: "2026-09-10T01:00:00Z",
    configDigest: "sha256:config",
    evidenceDigest: "sha256:evidence",
    toolDigest: "sha256:tools",
    digest: "sha256:plan",
    checks: [{ id: "quality.go.test", componentId: "repository" }],
  },
});
const improvement = (state = "proposed", revision = state === "proposed" ? 1 : 2) => ({
  metadata: { id: "proposal-1", name: "quality improvement proposal" },
  spec: {
    projectId: target.projectID,
    repositoryId: target.repositoryID,
    worktreeId: target.worktreeID,
    planId: "plan-1",
    baseScoreId: "score-3",
    basePlanRevision: 2,
    head: target.head,
    state,
    revision,
    rationaleCode: "finding_observed",
    changes: [{ action: "add_check", componentId: "repository", checkId: "quality.go.test_race", reason: "finding_observed" }],
    updatedAt: "2026-09-10T03:00:00Z",
  },
});
const installActionPlan = {
  metadata: { id: "install-plan-1", name: "Install Ruff" },
  spec: {
    actionType: "quality.tool.install.python",
    approvalRequired: true,
    execution: { executable: "python.exe", arguments: ["-m", "pip", "install", "ruff==0.6.9"] },
  },
};
const score = (id, overall, status = "fresh", createdAt = "2026-09-10T02:00:00Z") => ({
  metadata: { id },
  spec: {
    projectId: target.projectID,
    repositoryId: target.repositoryID,
    worktreeId: target.worktreeID,
    overall,
    confidence: 90,
    status,
    scoreVersion: "repository-quality-score-v1",
    head: target.head,
    createdAt,
    updatedAt: createdAt,
    components: [{ checkId: "quality.go.test", value: overall, confidence: 90, outcome: "clean", findingCount: 0 }],
  },
});

function harness({ targets = [target], requests = [], selections = {}, actionPlan = installActionPlan, approvalDecision = "granted", latestRun = null } = {}) {
  const elements = new Map([
    ["quality-inspection-target", { innerHTML: "" }],
    ["quality-inspection-content", { innerHTML: "", setAttribute() {} }],
  ]);
  const state = {
    selectedTargetValue: targets[0]?.value || "",
    service: { status: "ready" },
    qualityInspection: {
      status: "ready",
      error: "",
      plans: [],
      scores: [],
      selectedPlanID: "",
      selectedPlan: null,
      lastRun: null,
      comparison: { status: "idle", data: null, error: "", beforeID: "", afterID: "" },
      aiEnabled: false,
      proposals: [], proposalStatus: "idle", proposalError: "", selectedProposalID: "", selectedProposal: null,
      proposalMutation: { kind: "", status: "idle", error: "" },
      toolPreview: { status: "idle", data: null, error: "", input: null },
      toolActionPlan: { status: "idle", data: null, error: "", approvalStatus: "idle", executionStatus: "idle" },
      mutation: { kind: "", status: "idle", error: "" },
    },
  };
  const calls = requests;
  let proposalResponse = improvement();
  const context = vm.createContext({
    state,
    URLSearchParams,
    FormData: function TestFormData(form) {
      return { entries: () => Object.entries(form?.values || {})[Symbol.iterator]() };
    },
    document: {
      getElementById: id => elements.get(id),
      querySelector: selector => selections[selector] || null,
    },
    targetOptions: () => targets,
    selectedTarget: () => targets.find(item => item.value === state.selectedTargetValue) || targets[0] || null,
    rememberTarget: value => { state.selectedTargetValue = value; },
    renderAssuranceDashboard: () => {},
    serviceBlocksMutation: () => false,
    mutationHeaders: () => ({ "Content-Type": "application/json", "X-Control-Room-Token": "test-token" }),
    showNotice: () => {},
    encode: encodeURIComponent,
    formatDate: value => value || "",
    formatCount: value => String(value ?? 0),
    toneClass: tone => `state-${tone}`,
    stateText: (text, tone) => `<span class="state-${tone}">${text}</span>`,
    request: async (url, options = {}) => {
      calls.push({ url, options });
      if (url === "/api/quality/inspection-plans") return [plan()];
      if (url === "/api/quality/scores") return [score("score-2", 86)];
      if (url.startsWith("/api/quality/inspection-runs/latest?")) {
        if (latestRun) return latestRun;
        const error = new Error("not found"); error.status = 404; throw error;
      }
      if (url === "/api/quality/inspection-plans/plan-1") return plan();
      if (url === "/api/quality/inspection-plans/generate") return plan();
      if (url === "/api/quality/inspection-plans/plan-1/review") return plan("approved");
      if (url === "/api/quality/inspection-plans/plan-1/run") {
        return { plan: plan("approved"), resultArtifactId: "artifact-1", results: [{ checkId: "quality.go.test", outcome: "runner_unavailable" }], score: score("score-3", 0, "inconclusive") };
      }
      if (url === "/api/quality/improvement-proposals") return [proposalResponse];
      if (url === "/api/quality/improvement-proposals/proposal-1") return proposalResponse;
      if (url === "/api/quality/improvement-proposals/generate") return proposalResponse;
      if (url === "/api/quality/improvement-proposals/proposal-1/review") {
        proposalResponse = improvement("approved");
        return proposalResponse;
      }
      if (url === "/api/quality/improvement-proposals/proposal-1/apply") {
        proposalResponse = improvement("applied", 3);
        return { proposal: proposalResponse, plan: plan("proposed") };
      }
      if (url === "/api/quality/scores/score-before") return score("score-before", 70);
      if (url === "/api/quality/scores/score-3") return score("score-3", 0, "inconclusive");
      if (url === "/api/quality/comparisons?beforeId=score-before&afterId=score-after") return { status: "comparable", beforeOverall: 70, afterOverall: 86, delta: 16 };
      if (url === "/api/quality/scores/score-after") return score("score-after", 86);
      if (url === "/api/quality/tool-installs/plan") return { available: true, action: { package: "ruff", version: "0.6.9", command: { executable: "python.exe", arguments: ["-m", "pip", "install", "ruff==0.6.9"] }, affectedFiles: ["pyproject.toml"] } };
      if (url === "/api/quality/tool-installs/action-plan") {
        if (!actionPlan) throw new Error("설치 전용 Action Plan endpoint 없음");
        return actionPlan;
      }
      if (url === "/ui/actions/plans/install-plan-1/approval") return { decision: approvalDecision };
      if (url === "/api/actions/plans/install-plan-1/execute") return { metadata: { id: "run-install-1" }, spec: { status: "succeeded" } };
      throw new Error(`unexpected request: ${url}`);
    },
    escapeHTML: undefined,
  });
  vm.runInContext(`${uiSource}\nthis.ui = { renderQualityInspectionWorkflow, loadQualityInspectionData, generateQualityInspectionPlan, reviewQualityInspectionPlan, runQualityInspectionPlan, compareQualityInspectionScores, requestQualityToolInstallPreview, generateQualityImprovementProposal, reviewQualityImprovementProposal, applyQualityImprovementProposal, requestQualityToolInstallActionPlan, approveQualityToolInstallActionPlan, executeQualityToolInstallActionPlan };`, context);
  return { ui: context.ui, state, elements, calls };
}

test("assurance renderer explains deterministic plan review, score status, and no-target state", () => {
  const h = harness();
  h.state.qualityInspection.plans = [plan()];
  h.state.qualityInspection.scores = [score("score-1", 82, "stale")];
  h.ui.renderQualityInspectionWorkflow();
  const html = h.elements.get("quality-inspection-content").innerHTML;
  assert.match(h.elements.get("quality-inspection-target").innerHTML, /quality-inspection-target/);
  assert.match(html, /AI 제안은 연결되지 않았습니다/);
  assert.match(html, /검토 후 승인/);
  assert.match(html, /오래됨/);
  assert.match(html, /확신도/);

  const empty = harness({ targets: [] });
  empty.ui.renderQualityInspectionWorkflow();
  assert.match(empty.elements.get("quality-inspection-target").innerHTML, /선택된 Worktree가 없습니다/);
  assert.match(empty.elements.get("quality-inspection-content").innerHTML, /검사 흐름을 시작할 수 없습니다/);
});

test("assurance request flow uses list/get/generate/review/run/scores routes and mutation token", async () => {
  const h = harness();
  await h.ui.loadQualityInspectionData(true);
  assert.deepEqual(h.calls.slice(0, 4).map(item => item.url), [
    "/api/quality/inspection-plans",
    "/api/quality/scores",
    "/api/quality/inspection-runs/latest?projectId=project&repositoryId=repository&worktreeId=worktree",
    "/api/quality/inspection-plans/plan-1",
  ]);
  assert.ok(h.calls.some(item => item.url === "/api/quality/improvement-proposals"));
  assert.ok(h.calls.some(item => item.url === "/api/quality/improvement-proposals/proposal-1"));

  const generated = await h.ui.generateQualityInspectionPlan();
  assert.equal(generated.metadata.id, "plan-1");
  const generateCall = h.calls.find(item => item.url === "/api/quality/inspection-plans/generate");
  assert.equal(generateCall.options.method, "POST");
  assert.equal(generateCall.options.headers["X-Control-Room-Token"], "test-token");
  assert.deepEqual(JSON.parse(generateCall.options.body), {
    projectId: "project",
    repositoryId: "repository",
    worktreeId: "worktree",
  });

  h.state.qualityInspection.selectedPlan = plan();
  await h.ui.reviewQualityInspectionPlan("approve", { dataset: { id: "plan-1" } });
  const reviewCall = h.calls.find(item => item.url.endsWith("/review"));
  assert.deepEqual(JSON.parse(reviewCall.options.body), { expectedRevision: 1, decision: "approve" });

  h.state.qualityInspection.selectedPlan = plan("approved");
  await h.ui.runQualityInspectionPlan({ dataset: { id: "plan-1" } });
  assert.ok(h.calls.some(item => item.url === "/api/quality/inspection-plans/plan-1/run"));
  assert.ok(h.calls.some(item => item.url === "/api/quality/scores/score-3"));
  assert.equal(h.state.qualityInspection.lastRun.results[0].outcome, "runner_unavailable");
});

test("assurance comparison and tool-install preview stay evidence-bound", async () => {
  const h = harness({
    selections: {
      "[data-quality-inspection-before]": { value: "score-before" },
      "[data-quality-inspection-after]": { value: "score-after" },
    },
  });
  await h.ui.compareQualityInspectionScores();
  assert.equal(h.state.qualityInspection.comparison.data.status, "comparable");
  assert.ok(h.calls.some(item => item.url === "/api/quality/comparisons?beforeId=score-before&afterId=score-after"));

  await h.ui.requestQualityToolInstallPreview({ values: { kind: "python.ruff", version: "0.6.9", affectedFiles: "pyproject.toml" } });
  const previewCall = h.calls.find(item => item.url === "/api/quality/tool-installs/plan");
  assert.equal(previewCall.options.headers["X-Control-Room-Token"], "test-token");
  assert.deepEqual(JSON.parse(previewCall.options.body), {
    projectId: "project",
    repositoryId: "repository",
    worktreeId: "worktree",
    kind: "python.ruff",
    version: "0.6.9",
    affectedFiles: ["pyproject.toml"],
    allowGlobal: false,
  });
  assert.equal(h.state.qualityInspection.toolPreview.data.available, true);
});

test("explicit AI generation sends the bounded request and truthful fallback copy", async () => {
  const h = harness({ selections: { "[data-quality-inspection-ai]": { checked: true } } });
  await h.ui.generateQualityInspectionPlan();
  const call = h.calls.find(item => item.url === "/api/quality/inspection-plans/generate");
  assert.deepEqual(JSON.parse(call.options.body), {
    projectId: "project",
    repositoryId: "repository",
    worktreeId: "worktree",
    ai: { enabled: true, provider: "codex", profileId: "codex", requestedModel: "" },
  });
  h.state.qualityInspection.plans = [plan()];
  h.state.qualityInspection.aiEnabled = true;
  h.ui.renderQualityInspectionWorkflow();
  assert.match(h.elements.get("quality-inspection-content").innerHTML, /결정적 fallback/);
  assert.doesNotMatch(h.elements.get("quality-inspection-content").innerHTML, /AI가 점수나 실행을 결정합니다/);
});

test("improvement proposal uses the latest result artifact and applies enum changes back to a proposed plan", async () => {
  const h = harness();
  h.state.qualityInspection.selectedPlan = plan("approved");
  h.state.qualityInspection.lastRun = { plan: plan("approved"), resultArtifactId: "artifact-1", score: score("score-3", 0, "inconclusive"), results: [] };
  const generated = await h.ui.generateQualityImprovementProposal();
  assert.equal(generated.metadata.id, "proposal-1");
  const generateCall = h.calls.find(item => item.url === "/api/quality/improvement-proposals/generate");
  assert.deepEqual(JSON.parse(generateCall.options.body), { planId: "plan-1", resultArtifactId: "artifact-1" });

  h.state.qualityInspection.selectedProposal = improvement();
  await h.ui.reviewQualityImprovementProposal("approve", { dataset: { id: "proposal-1" } });
  const reviewCall = h.calls.find(item => item.url.endsWith("/improvement-proposals/proposal-1/review"));
  assert.deepEqual(JSON.parse(reviewCall.options.body), { expectedRevision: 1, decision: "approve" });

  h.state.qualityInspection.selectedProposal = improvement("approved");
  await h.ui.applyQualityImprovementProposal({ dataset: { id: "proposal-1" } });
  const applyCall = h.calls.find(item => item.url.endsWith("/improvement-proposals/proposal-1/apply"));
  assert.deepEqual(JSON.parse(applyCall.options.body), { expectedRevision: 2 });
  assert.equal(h.state.qualityInspection.selectedPlan.spec.state, "proposed");
  h.ui.renderQualityInspectionWorkflow();
  assert.match(h.elements.get("quality-inspection-content").innerHTML, /새 검사 계획은 다시.*제안됨/);
  assert.match(h.elements.get("quality-inspection-content").innerHTML, /rationaleCode/);
  assert.match(h.elements.get("quality-inspection-content").innerHTML, /검사 추가/);
});

test("tool install action plan stays server-owned and requires human approval before execute", async () => {
  const h = harness();
  await h.ui.requestQualityToolInstallPreview({ values: { kind: "python.ruff", version: "0.6.9", affectedFiles: "pyproject.toml" } });
  assert.equal(h.state.qualityInspection.toolActionPlan.status, "ready");
  assert.ok(h.calls.some(item => item.url === "/api/quality/tool-installs/action-plan"));
  await h.ui.approveQualityToolInstallActionPlan({ dataset: { id: "install-plan-1" } });
  assert.ok(h.calls.some(item => item.url === "/ui/actions/plans/install-plan-1/approval"));
  await h.ui.executeQualityToolInstallActionPlan({ dataset: { id: "install-plan-1" } });
  const executeCall = h.calls.find(item => item.url === "/api/actions/plans/install-plan-1/execute");
  assert.equal(JSON.parse(executeCall.options.body).holder, "ui");
  h.ui.renderQualityInspectionWorkflow();
  assert.match(h.elements.get("quality-inspection-content").innerHTML, /실제 완료 여부는 Action 결과에서 확인/);
  assert.doesNotMatch(h.elements.get("quality-inspection-content").innerHTML, /설치 완료/);
});

test("rejected and cancelled human approval responses never unlock execute", async () => {
  for (const decision of ["rejected", "cancelled"]) {
    const h = harness({ approvalDecision: decision });
    await h.ui.requestQualityToolInstallPreview({ values: { kind: "python.ruff", version: "0.6.9", affectedFiles: "pyproject.toml" } });
    await h.ui.approveQualityToolInstallActionPlan({ dataset: { id: "install-plan-1" } });
    assert.equal(h.state.qualityInspection.toolActionPlan.approvalStatus, decision);
    await h.ui.executeQualityToolInstallActionPlan({ dataset: { id: "install-plan-1" } });
    assert.equal(h.calls.filter(item => item.url === "/api/actions/plans/install-plan-1/execute").length, 0);
    h.ui.renderQualityInspectionWorkflow();
    const html = h.elements.get("quality-inspection-content").innerHTML;
    assert.match(html, decision === "rejected" ? /사람이 거절/ : /승인이 취소/);
    assert.doesNotMatch(html, /설치 Action 실행/);
  }
});

test("refresh restores the latest inspection result artifact for the selected worktree", async () => {
  const run = { plan: plan("approved"), resultArtifactId: "artifact-refresh", results: [{ checkId: "quality.go.test", outcome: "clean" }], score: score("score-refresh", 91) };
  const h = harness({ latestRun: run });
  await h.ui.loadQualityInspectionData(true);
  assert.equal(h.state.qualityInspection.lastRun.resultArtifactId, "artifact-refresh");
  assert.equal(h.state.qualityInspection.selectedPlanID, "plan-1");
  assert.ok(h.state.qualityInspection.scores.some(item => item.metadata.id === "score-refresh"));
});

test("missing install action plan endpoint leaves the preview honest and never marks installation complete", async () => {
  const h = harness({ actionPlan: null });
  await h.ui.requestQualityToolInstallPreview({ values: { kind: "python.ruff", version: "0.6.9", affectedFiles: "pyproject.toml" } });
  assert.equal(h.state.qualityInspection.toolPreview.data.available, true);
  assert.equal(h.state.qualityInspection.toolActionPlan.status, "error");
  h.ui.renderQualityInspectionWorkflow();
  const html = h.elements.get("quality-inspection-content").innerHTML;
  assert.match(html, /미리보기 전용/);
  assert.match(html, /설치 완료로 표시하지 않습니다/);
  assert.doesNotMatch(html, /설치 Action 실행/);
});
