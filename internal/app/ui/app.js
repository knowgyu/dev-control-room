(() => {
  "use strict";

  const token = document.querySelector('meta[name="control-room-token"]').content;
  const mutationHeaders = () => ({
    "Content-Type": "application/json",
    "X-Control-Room-Token": token,
  });
  const state = {
    snapshot: { projects: [] },
    registryProjects: [],
    findings: [],
    events: [],
    environment: { available: false, findings: [] },
    providerStatuses: [],
    assuranceDashboard: { invocations: [], effects: [], usageComplete: true, costState: "unknown" },
    assuranceRuns: [],
    qualityCampaigns: [],
    qualityCampaignsStatus: "loading",
    qualityCampaignsError: "",
    qualitySetup: { status: "idle", targetValue: "", requestKey: "", data: null, error: "", preferenceReset: false },
    assuranceMeasurement: { status: "loading", data: null, error: "" },
    assuranceInvocations: [],
    assuranceArtifacts: [],
    assuranceEffects: [],
    assuranceImpact: null,
    assuranceStorage: null,
    assuranceTrace: null,
    assuranceTraceEffectID: "",
    assuranceTraceOpener: null,
    assuranceFocusConsumed: "",
    assuranceImpactError: "",
    assuranceStorageError: "",
    assuranceTraceError: "",
    qualityHome: { status: "loading", data: null, error: "" },
    qualityTools: { status: "loading", data: null, error: "" },
    qualityInspection: {
      status: "idle", error: "", plans: [], scores: [], selectedPlanID: "", selectedPlan: null, lastRun: null,
      comparison: { status: "idle", data: null, error: "", beforeID: "", afterID: "" },
      aiEnabled: false,
      proposals: [], proposalStatus: "idle", proposalError: "", selectedProposalID: "", selectedProposal: null,
      proposalMutation: { kind: "", status: "idle", error: "" },
      toolPreview: { status: "idle", data: null, error: "", input: null },
      toolActionPlan: { status: "idle", data: null, error: "", approvalStatus: "idle", executionStatus: "idle" },
      mutation: { kind: "", status: "idle", error: "" },
    },
    homeChecks: { status: "loading", items: [], error: "" },
    service: { status: "loading", message: "로컬 서비스에 연결하는 중입니다…", error: "" },
    lastSuccessfulRefreshAt: "",
    assuranceFilters: { days: "30", provider: "", model: "", project: "" },
    assuranceEffectFilter: "all",
    workItems: [],
    actionDetails: [],
    repositorySyncPlan: null,
    repositorySyncResult: null,
    cleanup: [],
    safeguards: [],
    profiles: [],
    integrations: [],
    externalGroups: [],
    runbooks: [],
    integrationHealth: {},
    githubLatestRuns: {},
    jenkinsLatestBuilds: {},
    kubernetesStatuses: {},
    kubernetesLogs: {},
    operationResults: {},
    activeProjectID: "",
    selectedTargetValue: "",
    qualityRunPending: "",
    qualityRunTargetValue: "",
    qualityRunError: "",
    qualityRunErrorTargetValue: "",
    checkRuns: new Map(),
    expandedChecks: new Set(),
    expandedActions: new Set(),
    guidanceResult: null,
    guidanceMode: "",
    findingFilters: { severity: "", state: "active" },
    surfaceErrors: { checksets: "", actions: "", cleanup: "", safeguards: "", profiles: "", integrations: "", externalGroups: "", runbooks: "" },
    loaded: { work: false, diagnostics: false },
    loading: { work: false, diagnostics: false },
    qualityObjective: {
      selectedID: "",
      status: "idle",
      data: null,
      error: "",
      mutation: { kind: "", status: "idle", error: "", draft: {} },
    },
  };
  const routeTitles = {
    home: "개선",
    projects: "프로젝트",
    work: "작업",
    assurance: "검증",
    diagnostics: "진단",
    activity: "활동 기록",
    guide: "사용법",
  };
  const guideSlides = [
    {
      kicker: "01 · 범위",
      status: "파일은 읽기만 함",
      tone: "neutral",
      title: "프로젝트에서 폴더를 고르세요.",
      body: "먼저 관찰할 범위를 등록합니다. 등록 전에 찾은 Git 저장소를 직접 고르며, 이 과정은 원본 파일을 바꾸지 않습니다.",
      place: "프로젝트",
      control: "프로젝트 등록 → 폴더 선택 → 저장소 찾기",
      done: "프로젝트 목록에 저장소와 Worktree가 보임",
      bullets: ["여러 저장소가 나오면 이번에 확인할 저장소만 체크합니다.", "폴더 선택은 Windows 기본 Explorer 대화상자를 사용합니다."],
      action: "프로젝트 등록 열기",
      href: "#projects",
    },
    {
      kicker: "02 · 대상",
      status: "저장소와 브랜치 선택",
      tone: "neutral",
      title: "작업에서 확인할 저장소를 고르세요.",
      body: "등록한 저장소와 브랜치를 확인합니다. 선택한 대상은 개선과 작업 화면을 오가거나 다시 열어도 유지됩니다.",
      place: "개선 · 작업",
      control: "저장소 선택 → 브랜치 확인",
      done: "확인하려는 대상과 경로가 표시됨",
      bullets: ["등록한 대상이 보이지 않으면 저장소 상태를 새로 고치세요.", "환경 문제가 있을 때는 진단에서 기본 도구를 확인합니다."],
      action: "작업 열기",
      href: "#work",
    },
    {
      kicker: "03 · 우선순위",
      status: "현재 상태를 기준으로 선택",
      tone: "neutral",
      title: "저장소 상태와 점검 결과를 구분해 보세요.",
      body: "‘저장소 상태 새로고침’은 Git 상태와 확인 항목을 갱신합니다. 프로젝트 구성 확인과 검사 실행은 작업 화면에서 별도로 선택합니다.",
      place: "개선 · 작업",
      control: "저장소 상태 새로고침 → 작업 → 구성 확인",
      done: "저장소 상태와 구성 확인 결과를 각각 확인함",
      bullets: ["새로고침은 테스트나 품질 점검을 실행하지 않습니다.", "구성 확인은 선택한 저장소의 제한된 근거만 읽습니다."],
      action: "작업 열기",
      href: "#work",
    },
    {
      kicker: "04 · 구성 확인",
      status: "근거를 먼저 확인",
      tone: "attention",
      title: "언어와 구성 요소를 확인하세요.",
      body: "작업에서 저장소를 고르면 언어·프레임워크·패키지 관리자와 확인한 근거를 표시합니다. 필요하면 언어 범위를 직접 좁힐 수 있지만, 선택만으로 언어가 확인되지는 않습니다.",
      place: "작업",
      control: "대상 선택 → 자동 감지 → 언어 선택(필요시) → 근거 보기",
      done: "구성 상태·근거·준비 안내를 확인함",
      bullets: ["저장소별 설정과 실제 실행기를 확인해 적용 가능한 검사 계획을 만듭니다.", "Ruff·pytest·ESLint·Vitest·Go 검사는 도구와 프로젝트 근거가 확인된 경우에만 실행 대상으로 표시합니다.", "AI는 허용된 검사 후보만 제안하고, 점수·명령·승인을 결정하지 않습니다."],
      action: "작업으로 이동",
      href: "#work",
    },
    {
      kicker: "05 · 기록",
      status: "다시 확인할 수 있음",
      tone: "positive",
      title: "실행한 검사의 결과를 확인하세요.",
      body: "지원되는 검사를 직접 실행했다면 작업에서 결과를 바로 확인합니다. 전체 결과는 검증에서, 등록과 실행 기록은 활동에서 다시 볼 수 있습니다.",
      place: "검증 · 활동",
      control: "결과 보기 → 검증의 근거 확인 → 활동 기록 열기",
      done: "무엇을 언제 어떤 기준으로 확인했는지 설명할 수 있음",
      bullets: ["설정 확인만으로 검사 결과가 생기지는 않습니다.", "테스트·커버리지는 저장소의 테스트 코드를 실행하므로 명령을 먼저 확인하세요."],
      action: "검증 기록 보기",
      href: "#assurance",
    },
  ];
  const severityLabels = {
    info: "정보",
    attention: "주의",
    high: "높음",
    critical: "긴급",
  };
  const statusLabels = {
    open: "열림",
    acknowledged: "확인함",
    resolved: "해결됨",
    suppressed: "숨김",
    expired: "만료됨",
    pending: "검토 대기",
    rejected: "거절됨",
    stale: "근거 변경됨",
    draft: "초안",
    applied: "적용됨",
    passed: "통과",
    failed: "실패",
    skipped: "건너뜀",
    unavailable: "사용할 수 없음",
    eligible: "실행 가능",
    approval_required: "승인 필요",
    policy_denied: "정책에 따라 차단됨",
    succeeded: "성공",
    running: "실행 중",
    cancelled: "취소됨",
    timed_out: "시간 초과",
    precheck_failed: "사전 점검 실패",
    postcheck_failed: "사후 점검 실패",
    blocked: "차단됨",
    shadow: "검토 전 모의 적용",
    proposal: "검토 대기",
    active: "활성",
    retired: "사용 종료",
    positive: "유효한 방지",
    false_positive: "오탐",
    granted: "승인됨",
    allowed: "허용됨",
    denied: "차단됨",
    read_only: "읽기 전용",
    safe_local: "로컬 변경",
    external_change: "외부 변경",
    high_impact: "고위험 변경",
    deterministic: "결정적 발견",
    direct: "직접 실행",
    powershell_profile: "PowerShell 프로필",
    enterprise: "기업 경계",
    local: "로컬 전용",
    verified_read_only: "읽기 전용 확인됨",
    unverified: "확인되지 않음",
    completed: "완료",
    reviewable: "검토 가능",
    queued: "대기",
    in_progress: "진행 중",
    requested: "요청됨",
    waiting: "대기",
    awaiting_answer: "답변 대기",
    cancelling: "취소 중",
    interrupted: "중단됨",
  };
  const conclusionLabels = { success: "성공", failure: "실패", cancelled: "취소됨", skipped: "건너뜀", neutral: "중립" };
  const buildResultLabels = { SUCCESS: "성공", FAILURE: "실패", ABORTED: "중단됨", UNSTABLE: "불안정", NOT_BUILT: "빌드 안 됨" };
  const confidenceLabels = {
    confirmed: "확인됨",
    likely: "가능성 높음",
    uncertain: "불확실",
  };
  const messageTranslations = new Map([
    ["Repository has uncommitted changes", "저장소에 커밋하지 않은 변경이 있습니다."],
    ["Review the worktree before any cleanup or automation.", "정리나 자동화를 실행하기 전에 Worktree를 검토하세요."],
    ["Repository HEAD is detached", "저장소 HEAD가 detached 상태입니다."],
    ["Check out an intentional branch before making changes.", "변경하기 전에 의도한 브랜치를 확인하세요."],
    ["Repository has no normalized remote", "저장소에 확인된 remote가 없습니다."],
    ["Configure a remote only after confirming the intended destination.", "대상 주소를 확인한 뒤 remote를 설정하세요."],
    ["Worktree cleanup is unsafe", "현재 Worktree를 안전하게 정리할 수 없습니다."],
    ["Do not remove or reset this worktree until its local state is reviewed.", "로컬 상태를 검토하기 전에는 이 Worktree를 제거하거나 reset하지 마세요."],
    ["Repository observation was incomplete", "저장소 상태를 끝까지 확인하지 못했습니다."],
    ["Check the registered worktree and Git installation before retrying.", "등록된 Worktree와 Git 설치 상태를 확인한 뒤 다시 시도하세요."],
    ["Project has no recent completed scan", "최근에 완료된 프로젝트 점검이 없습니다."],
    ["Run a scan before relying on repository health.", "저장소 상태를 판단하기 전에 점검을 실행하세요."],
    ["environment doctor has not run yet", "아직 개발 환경을 점검하지 않았습니다."],
    ["run env doctor before relying on environment health", "개발 환경 상태를 사용하기 전에 환경 점검을 실행하세요."],
    ["environment variable declaration is duplicated or conflicting", "환경 변수 선언이 중복되었거나 서로 충돌합니다."],
    ["keep one declaration and make its scope explicit", "선언 하나만 남기고 적용 범위를 명확히 하세요."],
    ["declared environment variable is missing", "선언된 환경 변수를 찾을 수 없습니다."],
    ["define the variable in the declared Windows scope", "선언한 Windows 범위에 환경 변수를 설정하세요."],
    ["install or configure the reported capability, then run env doctor again", "해당 도구를 설치하거나 설정한 뒤 환경을 다시 점검하세요."],
    ["repeated verified failure may justify a deterministic safeguard", "반복해서 확인된 실패에 결정적 safeguard가 필요할 수 있습니다."],
    ["review the masked evidence and approve a removable shadow rule", "마스킹된 근거를 검토하고 제거 가능한 shadow rule을 승인하세요."],
    ["Project updated", "프로젝트 정보를 변경했습니다."],
    ["Repository added", "저장소를 등록했습니다."],
    ["Repository updated", "저장소 정보를 변경했습니다."],
    ["Project removed; repository files were not changed", "프로젝트 등록을 해제했습니다. 저장소 파일은 변경하지 않았습니다."],
    ["Repository removed; repository files were not changed", "저장소 등록을 해제했습니다. 저장소 파일은 변경하지 않았습니다."],
  ]);

  const escapeHTML = value => String(value ?? "").replace(/[&<>'"]/g, character => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    "'": "&#39;",
    '"': "&quot;",
  })[character]);
  const encode = value => encodeURIComponent(value);
  const label = value => statusLabels[value] || value || "알 수 없음";
  const toneClass = tone => ({
    positive: "state-ok",
    attention: "state-warning",
    negative: "state-danger",
    neutral: "state-neutral",
  })[tone] || "state-neutral";
  const rowToneClass = tone => ({
    positive: "state-ok",
    attention: "state-warn",
    negative: "state-bad",
    neutral: "state-neutral",
  })[tone] || "state-neutral";
  const stateText = (text, tone = "neutral") => `<span class="state-text ${toneClass(tone)}" data-tone="${escapeHTML(tone)}">${escapeHTML(text)}</span>`;
  const findingTone = severity => ["high", "critical"].includes(severity) ? "negative" : severity === "attention" ? "attention" : "neutral";
  const providerTone = providerState => providerState === "ready" ? "positive" : providerState === "not_configured" ? "neutral" : ["unavailable"].includes(providerState) ? "negative" : "attention";
  const assuranceTone = value => ["succeeded", "passed", "measured", "prevented_regression", "active", "pinned", "complete"].includes(value)
    ? "positive"
    : ["failed", "timed_out", "cancelled", "interrupted", "stale", "deleted", "negative"].includes(value)
      ? "negative"
      : ["unavailable", "unknown"].includes(value) ? "neutral" : "attention";
  const eventTone = item => assuranceTone(item?.spec?.status || item?.spec?.conclusion || "unknown");
  const providerStateLabels = {
    ready: "사용 가능",
    detected: "실행 확인 필요",
    unavailable: "사용할 수 없음",
    auth_required: "인증 필요",
    profile_needs_setup: "프로필 설정 필요",
    not_configured: "미설정",
  };
  const providerLabel = state => providerStateLabels[state] || "확인 필요";
  const providerStateSummaries = {
    ready: "실행 경로를 확인했습니다.",
    detected: "실행 경로 확인이 필요합니다.",
    unavailable: "현재 실행할 수 없습니다.",
    auth_required: "인증 확인이 필요합니다.",
    profile_needs_setup: "프로필 설정이 필요합니다.",
    not_configured: "아직 설정하지 않았습니다.",
  };
  const providerSummary = state => providerStateSummaries[state] || "상태를 확인하세요.";
  const providerDiagnosticTexts = {
    "provider.not_found": "설치 경로를 찾지 못했습니다.",
    "provider.untrusted_launcher": "검증되지 않은 launcher를 찾았습니다.",
    "provider.node_required": "로컬 node.exe가 필요합니다.",
    "provider.node_missing": "로컬 node.exe 파일을 찾지 못했습니다.",
    "provider.node_not_regular": "node.exe 파일을 신뢰할 수 없습니다.",
    "provider.package_metadata_missing": "Codex 패키지 정보를 찾지 못했습니다.",
    "provider.package_not_trusted": "Codex 패키지 정보를 신뢰할 수 없습니다.",
    "provider.package_entry_missing": "Codex 실행 파일을 찾지 못했습니다.",
    "provider.package_entry_not_regular": "Codex 실행 파일을 신뢰할 수 없습니다.",
    "provider.package_entry_unreadable": "Codex 실행 파일을 읽을 수 없습니다.",
  };
  const providerDiagnostic = item => providerDiagnosticTexts[item.reasonCode] || (item.state === "ready" ? "검증된 실행 경로를 사용합니다." : "진단 코드와 실행 경로를 확인합니다.");
  const optionalProviderIDs = new Set(["codex", "claude", "gemini", "claude-local"]);
  const requiredEnvironmentReady = environment => {
    if (!environment?.generatedAt) return false;
    if ((environment.tools || []).some(item => item.required && (item.state !== "available" || item.available !== true))) return false;
    if ((environment.profiles || []).some(item => item.required && (item.state !== "available" || item.available !== true))) return false;
    return !(environment.environment || []).some(item => item.state !== "declared");
  };
  const assuranceTechniqueLabels = {
    static_security: "Go 정적 검사 (go vet)",
    go_test_coverage: "테스트·커버리지",
    mutation: "변이",
    property: "Property",
    fuzz: "Fuzz",
    targeted_e2e: "Targeted E2E",
  };
  const assuranceEffectLabels = {
    measured: "측정됨",
    prevented_regression: "회귀 방지",
    user_estimated: "사용자 추정",
    ai_inference: "AI 추론",
    unavailable: "확인 불가",
    improved: "개선",
    unchanged: "변화 없음",
    regressed: "악화",
    unknown: "확인 불가",
  };
  const assuranceImpactStateLabels = {
    measured: "측정됨",
    prevented_regression: "회귀 방지",
    user_estimated: "사용자 추정",
    ai_inference: "AI 추론",
    unavailable: "확인 불가",
  };
  const assuranceImpactStateClass = value => ["measured", "prevented_regression"].includes(value) ? "ok" : ["user_estimated", "ai_inference"].includes(value) ? "warn" : value === "unavailable" ? "unknown" : "";
  const assuranceEffectClassification = spec => ["measured", "prevented_regression", "user_estimated", "ai_inference", "unavailable"].includes(spec.kind) ? spec.kind : "unavailable";
  const assuranceComparisonLabels = { increase: "증가", decrease: "감소", neutral: "변화 없음", unavailable: "비교 불가" };
  const assuranceUnitLabels = { count: "건", percent: "%", seconds: "초", minutes: "분", hours: "시간", milliseconds: "ms" };
  const assuranceRetentionLabels = {
    active: "활성",
    pinned: "고정",
    archived: "보관됨",
    deleted: "삭제됨",
  };
  const measurementStatusLabels = { pass: "통과", fail: "실패", unknown: "알 수 없음" };
  const measurementProvenanceLabels = { measured: "측정됨", estimated: "추정", inferred: "추론", unavailable: "사용할 수 없음" };
  const measurementComparisonStateLabels = { empty: "기록 없음", comparable: "비교 가능", missing: "이전 실행 없음", unknown: "비교 상태 알 수 없음", incomparable: "비교 불가", unavailable: "비교 불가" };
  const measurementMetricLabels = {
    "quality.gofmt": "gofmt",
    "quality.go.test": "Go test",
    "quality.go.test_race": "Go test · race",
    "quality.go.vet": "Go vet",
    "quality.go.mod_verify": "Go mod verify",
    "quality.go.build": "Go build",
    "quality.ui.syntax": "UI syntax",
    "quality.go.coverage": "Coverage 수집",
    "quality.go.coverage_percent": "Coverage",
    "performance.http.health.latency": "HTTP · /api/health",
    "performance.http.state.latency": "HTTP · /api/state",
    "process.dogfood.run_duration": "전체 runner 시간",
  };
  const numberFormatter = new Intl.NumberFormat("ko-KR");
  const localize = value => {
    const text = String(value ?? "");
    if (messageTranslations.has(text)) return messageTranslations.get(text);
    const drift = text.match(/^Repository differs from upstream \((\d+) ahead, (\d+) behind\)$/);
    if (drift) return `upstream과 차이가 있습니다. ahead ${drift[1]}, behind ${drift[2]}`;
    const scans = text.match(/^(manual|scheduled|schedule|startup) scan completed for (\d+) project\(s\)$/);
    if (scans) return `${({ manual: "수동", scheduled: "예약", schedule: "예약", startup: "시작" })[scans[1]]} 점검을 완료했습니다. 프로젝트 ${scans[2]}개`;
    if (text === "Project added") return "프로젝트를 등록했습니다.";
    const projectAdded = text.match(/^Project added with (\d+) repositories$/);
    if (projectAdded) return `프로젝트와 저장소 ${projectAdded[1]}개를 등록했습니다.`;
    return text;
  };
  const formatDate = value => {
    if (!value) return "아직 없음";
    const date = new Date(value);
    if (Number.isNaN(date.getTime()) || date.getUTCFullYear() < 1971) return "아직 없음";
    return new Intl.DateTimeFormat("ko-KR", { dateStyle: "short", timeStyle: "short" }).format(date);
  };
  const formatCount = value => numberFormatter.format(Number(value) || 0);
  const formatOptionalCount = value => value === null || value === undefined ? "미상" : numberFormatter.format(Number(value) || 0);
  const formatImpactValue = (value, unit = "") => {
    if (value === null || value === undefined || value === "") return "확인 불가";
    const numeric = Number(value);
    const text = Number.isFinite(numeric) ? new Intl.NumberFormat("ko-KR", { maximumFractionDigits: 2 }).format(numeric) : String(value);
    return `${text}${assuranceUnitLabels[unit] || (unit ? ` ${unit}` : "")}`;
  };
  const formatImpactDelta = (comparison = {}, unit = "") => {
    if (comparison.state === "unavailable" || comparison.delta === null || comparison.delta === undefined) return "이전 기간 비교 불가";
    const delta = Number(comparison.delta);
    const deltaText = Number.isFinite(delta) ? `${delta > 0 ? "+" : ""}${formatImpactValue(delta, unit)}` : "변화 기록 없음";
    const percent = comparison.deltaPercent === null || comparison.deltaPercent === undefined ? "" : ` (${Number(comparison.deltaPercent) > 0 ? "+" : ""}${formatImpactValue(comparison.deltaPercent, "percent")})`;
    return `${assuranceComparisonLabels[comparison.state] || "변화"} ${deltaText}${percent}`;
  };
  const safePercentage = (value, maximum) => {
    const numeric = Number(value) || 0;
    const max = Number(maximum) || 1;
    return Math.max(0, Math.min(100, Math.round((numeric / max) * 100)));
  };
  const artifactID = item => item?.metadata?.id || item?.id || item?.spec?.id || "";
  const assuranceScope = spec => [spec.projectId, spec.repositoryId, spec.worktreeId].filter(Boolean).join(" / ") || "전체 범위";
  const assuranceDashboardPath = () => {
    const query = new URLSearchParams();
    if (state.assuranceFilters.provider) query.set("provider", state.assuranceFilters.provider);
    if (state.assuranceFilters.model) query.set("model", state.assuranceFilters.model);
    const suffix = query.toString();
    return `/api/assurance/dashboard${suffix ? `?${suffix}` : ""}`;
  };
  const assuranceImpactQuery = includeFormat => {
    const query = new URLSearchParams({ days: state.assuranceFilters.days || "30" });
    if (state.assuranceFilters.provider) query.set("provider", state.assuranceFilters.provider);
    if (state.assuranceFilters.model) query.set("model", state.assuranceFilters.model);
    if (state.assuranceFilters.project) query.set("project", state.assuranceFilters.project);
    if (includeFormat) query.set("format", includeFormat);
    return query;
  };
  const assuranceImpactPath = () => `/api/assurance/impact?${assuranceImpactQuery().toString()}`;
  const assuranceStoragePath = () => "/api/assurance/artifacts/storage";
  const projectRepositories = () => (state.snapshot.projects || []).flatMap(project =>
    (project.repos || []).map(repository => ({ ...repository, projectID: project.id, projectName: project.name })));
  const openFindings = () => state.findings.filter(item => ["open", "acknowledged"].includes(item.spec.state));
  const registryProject = projectID => state.registryProjects.find(project => project.metadata.id === projectID);
  const targetOptions = () => projectRepositories().flatMap(repository => (repository.worktrees || []).map(worktree => {
    const branch = worktree.spec?.branch || worktree.metadata?.name || "브랜치 미상";
    const primary = worktree.spec?.primary || worktree.spec?.isPrimary || worktree.metadata?.name === "main" || worktree.metadata?.name === "master";
    const repositoryName = repository.name || repository.metadata?.name || repository.id;
    const branchLabel = primary ? `${branch} · 기본` : branch;
    const targetName = [...new Set([repository.projectName, repositoryName].filter(Boolean))].join(" · ");
    return {
      value: `${repository.projectID}|${repository.id}|${worktree.metadata.id}`,
      label: `${targetName} · ${branchLabel}`,
      projectID: repository.projectID,
      projectName: repository.projectName,
      repositoryID: repository.id,
      repositoryName,
      repositoryPath: repository.path || repository.spec?.path || "",
      worktreeID: worktree.metadata.id,
      branch,
      head: worktree.spec?.head || "",
      digest: worktree.spec?.digest || worktree.spec?.configDigest || "",
      primary,
    };
  }));
  const selectedTargetStorageKey = "dev-control-room.selected-target";
  const readStoredTarget = () => {
    try { return window.localStorage.getItem(selectedTargetStorageKey) || ""; } catch (_) { return ""; }
  };
  const qualitySetupLanguageLabels = {
    python: "Python",
    javascript: "JavaScript",
    typescript: "TypeScript",
    go: "Go",
  };
  const qualitySetupLanguages = Object.keys(qualitySetupLanguageLabels);
  const qualitySetupStatusLabels = {
    existing_configuration: "구성 확인됨",
    setup_needed: "준비 필요",
    unsupported: "지원 범위 밖",
    ambiguous: "확인 필요",
  };
  const rememberTarget = value => {
    state.selectedTargetValue = value || "";
    try {
      if (value) window.localStorage.setItem(selectedTargetStorageKey, value);
      else window.localStorage.removeItem(selectedTargetStorageKey);
    } catch (_) { /* selection remains in memory when storage is unavailable */ }
  };
  const selectedTarget = () => targetOptions().find(target => target.value === state.selectedTargetValue) || targetOptions()[0] || null;
  const decodeURIComponentSafe = value => {
    try { return decodeURIComponent(value); } catch (_) { return ""; }
  };
  const routeState = () => {
    const [path, query = ""] = location.hash.slice(1).split("?", 2);
    const [name, projectID = ""] = path.split("/", 2);
    let findingID = "";
    let objectiveID = "";
    let targetValue = "";
    let runID = "";
    try {
      const params = new URLSearchParams(query);
      findingID = params.get("finding") || "";
      objectiveID = params.get("objective") || "";
      targetValue = params.get("target") || "";
      runID = params.get("run") || "";
    } catch (_) {
      findingID = "";
      objectiveID = "";
    }
    return { name, projectID: decodeURIComponentSafe(projectID), findingID, objectiveID, targetValue, runID, query };
  };
  const isAssuranceDemoRoute = () => {
    const target = routeState();
    return target.name === "assurance" && new URLSearchParams(target.query || "").get("demo") === "1";
  };
  const guideSlideIndex = () => {
    const query = new URLSearchParams(routeState().query || "");
    const value = Number.parseInt(query.get("slide") || "1", 10);
    return Number.isFinite(value) ? Math.max(0, Math.min(guideSlides.length - 1, value - 1)) : 0;
  };
  let activeRoute = "home";
  let pendingFindingID = "";
  let pendingProjectFocusID = "";
  let pendingRouteFocus = "";
  let hasMountedRoute = false;
  const currentRoute = () => {
    const candidate = routeState().name;
    if (routeTitles[candidate]) return candidate;
    if (candidate === "main-content") return activeRoute;
    return "home";
  };
  let routeFocusTimer = 0;

  function applyAssuranceRouteState(target) {
    const query = new URLSearchParams(target.query || "");
    const days = query.get("days") || "30";
    const effect = query.get("effect") || "all";
    state.assuranceFilters.days = ["7", "30", "90"].includes(days) ? days : "30";
    state.assuranceFilters.provider = query.get("provider") || "";
    state.assuranceFilters.model = query.get("model") || "";
    state.assuranceFilters.project = query.get("project") || "";
    state.assuranceEffectFilter = ["all", "measured", "prevented_regression", "user_estimated", "ai_inference", "unavailable"].includes(effect)
      ? effect
      : "all";
  }

  function syncAssuranceRouteState() {
    if (activeRoute !== "assurance") return;
    const query = new URLSearchParams();
    if (state.assuranceFilters.days !== "30") query.set("days", state.assuranceFilters.days);
    if (state.assuranceFilters.provider) query.set("provider", state.assuranceFilters.provider);
    if (state.assuranceFilters.model) query.set("model", state.assuranceFilters.model);
    if (state.assuranceFilters.project) query.set("project", state.assuranceFilters.project);
    if (state.assuranceEffectFilter !== "all") query.set("effect", state.assuranceEffectFilter);
    const serialized = query.toString();
    history.replaceState(null, "", serialized ? `#assurance?${serialized}` : "#assurance");
  }

  function syncGuideRouteState(index) {
    if (activeRoute !== "guide") return;
    const value = Math.max(0, Math.min(guideSlides.length - 1, Number(index) || 0));
    history.replaceState(null, "", value === 0 ? "#guide" : `#guide?slide=${value + 1}`);
  }

  function renderGuide() {
    const slide = guideSlides[guideSlideIndex()];
    const index = guideSlideIndex();
    const slideContainer = document.getElementById("guide-slide");
    const indexContainer = document.getElementById("guide-slide-index");
    const kickerContainer = document.getElementById("guide-slide-kicker");
    const statusContainer = document.getElementById("guide-slide-status");
    const dotsContainer = document.getElementById("guide-dots");
    const previous = document.querySelector("[data-guide-prev]");
    const next = document.querySelector("[data-guide-next]");
    if (!slide || !slideContainer || !indexContainer || !kickerContainer || !statusContainer || !dotsContainer) return;
    indexContainer.textContent = `${String(index + 1).padStart(2, "0")} / ${String(guideSlides.length).padStart(2, "0")}`;
    kickerContainer.textContent = slide.kicker;
    statusContainer.className = `state-text ${toneClass(slide.tone)}`;
    statusContainer.textContent = slide.status;
    slideContainer.setAttribute("aria-label", `사용법 ${index + 1}단계`);
    slideContainer.innerHTML = `<h2 id="guide-slide-title">${escapeHTML(slide.title)}</h2><p>${escapeHTML(slide.body)}</p><dl class="guide-action-grid"><div><dt>화면</dt><dd>${escapeHTML(slide.place)}</dd></div><div><dt>누를 것</dt><dd>${escapeHTML(slide.control)}</dd></div><div><dt>완료 기준</dt><dd>${escapeHTML(slide.done)}</dd></div></dl><ul>${slide.bullets.map(item => `<li>${escapeHTML(item)}</li>`).join("")}</ul><a class="button small" href="${escapeHTML(slide.href)}">${escapeHTML(slide.action)}</a>`;
    dotsContainer.innerHTML = guideSlides.map((item, itemIndex) => `<button class="guide-dot ${itemIndex === index ? "selected" : ""}" type="button" data-guide-slide="${itemIndex}" aria-label="${escapeHTML(`${itemIndex + 1}단계: ${item.kicker}`)}" aria-current="${itemIndex === index ? "step" : "false"}">${String(itemIndex + 1).padStart(2, "0")}</button>`).join("");
    if (previous) previous.disabled = index === 0;
    if (next) {
      next.disabled = false;
      next.textContent = index === guideSlides.length - 1 ? "처음으로" : "다음";
    }
  }

  function setGuideSlide(index) {
    const value = Number(index);
    const nextIndex = Number.isFinite(value) ? Math.max(0, Math.min(guideSlides.length - 1, value)) : 0;
    syncGuideRouteState(nextIndex);
    renderGuide();
  }

  async function request(path, options = {}) {
    const response = await fetch(path, options);
    const body = await response.json();
    if (!response.ok || !body.ok) {
      const error = new Error(body.error?.message || `요청에 실패했습니다. (${response.status})`);
      error.status = response.status;
      error.code = body.error?.code || "";
      throw error;
    }
    return body.data;
  }

  function showNotice(message, error = false) {
    const notice = document.getElementById("notice");
    notice.textContent = message;
    notice.classList.toggle("error", error);
    notice.setAttribute("role", error ? "alert" : "status");
    notice.hidden = false;
    window.clearTimeout(showNotice.timer);
    showNotice.timer = window.setTimeout(() => { notice.hidden = true; }, 5000);
  }

  function setRoute() {
    const active = currentRoute();
    const target = routeState();
    const shouldMoveRouteFocus = hasMountedRoute;
    hasMountedRoute = true;
    activeRoute = active;
    if (active === "assurance") applyAssuranceRouteState(target);
    pendingFindingID = active === "projects" ? target.findingID : "";
    state.qualityObjective.selectedID = active === "home" ? decodeURIComponentSafe(target.objectiveID) : "";
    if (target.targetValue) rememberTarget(target.targetValue);
    else if (!state.selectedTargetValue) state.selectedTargetValue = readStoredTarget();
    if (active === "projects" && target.projectID) state.activeProjectID = target.projectID;
    document.querySelectorAll("[data-view]").forEach(view => { view.hidden = view.dataset.view !== active; });
    document.querySelectorAll("[data-route]").forEach(link => {
      if (link.dataset.route === active) link.setAttribute("aria-current", "page");
      else link.removeAttribute("aria-current");
    });
    const main = document.getElementById("main-content");
    if (main) main.setAttribute("aria-label", routeTitles[active]);
    document.title = `${routeTitles[active]} · Dev Control Room`;
    if (active === "guide") renderGuide();
    if (active === "assurance" && initialized) renderAssuranceDashboard();
    if (active === "home") renderQualityObjectiveDetail();
    window.scrollTo({ top: 0 });
    if (active === "projects" && initialized) renderProjects();
    window.clearTimeout(routeFocusTimer);
    const focusTarget = pendingRouteFocus;
    pendingRouteFocus = "";
    if (shouldMoveRouteFocus || focusTarget) {
      routeFocusTimer = window.setTimeout(() => {
        if (focusPendingFinding()) return;
        if (focusTarget && focusElementByID(focusTarget)) return;
        focusElementByID("main-content");
      }, 0);
    }
    if (initialized) void loadRouteData(active, false);
  }

  function focusElementByID(id) {
    const element = id ? document.getElementById(id) : null;
    if (!element) return false;
    if (id === "main-content") element.classList.add("route-focus");
    element.focus({ preventScroll: true });
    return true;
  }

  function focusPendingFinding() {
    if (!pendingFindingID) return false;
    const finding = document.getElementById(`finding-${encode(pendingFindingID)}`);
    if (!finding) return false;
    finding.focus({ preventScroll: true });
    pendingFindingID = "";
    return true;
  }

  function focusPendingProject() {
    if (!pendingProjectFocusID) return false;
    const project = [...document.querySelectorAll("[data-project]")].find(item => item.dataset.project === pendingProjectFocusID);
    if (!project) return false;
    project.focus({ preventScroll: true });
    pendingProjectFocusID = "";
    return true;
  }

  function surfaceError(message, route) {
    return `<div class="list-item state-bad" role="alert"><strong>데이터를 불러오지 못했습니다.</strong><p>${escapeHTML(message)}</p><button class="button small" type="button" data-retry="${escapeHTML(route)}">다시 시도</button></div>`;
  }

  function findingCard(item, idPrefix = "finding") {
    const severity = String(item.spec.severity || "info");
    const tone = findingTone(severity);
    const domainState = String(item.spec.state || "");
    const scope = [item.spec.projectId, item.spec.repositoryId].filter(Boolean).join(" / ");
    return `<article id="${idPrefix}-${escapeHTML(encode(item.metadata.id))}" class="ledger-row finding ${rowToneClass(tone)}" data-tone="${escapeHTML(tone)}" data-severity="${escapeHTML(severity)}" data-state="${escapeHTML(domainState)}" data-finding-id="${escapeHTML(item.metadata.id)}" tabindex="-1">
      <div class="ledger-row__state">${stateText(severityLabels[severity] || severity, tone)}</div>
      <div class="ledger-row__main"><h3>${escapeHTML(localize(item.spec.summary))}</h3>
      <p class="next">다음 단계: ${escapeHTML(localize(item.spec.recommendedNextAction))}</p>
      <details><summary>근거와 상태 보기</summary><dl class="detail-grid">
        <div><dt>범위</dt><dd>${escapeHTML(scope || "전체")}</dd></div>
        <div><dt>상태</dt><dd>${escapeHTML(label(domainState))}</dd></div>
        <div><dt>확신도</dt><dd>${escapeHTML(confidenceLabels[item.spec.confidence] || item.spec.confidence || "알 수 없음")}</dd></div>
        <div><dt>처음 확인</dt><dd>${escapeHTML(formatDate(item.spec.firstObserved))}</dd></div>
        <div><dt>최근 확인</dt><dd>${escapeHTML(formatDate(item.spec.lastObserved))}</dd></div>
        <div class="wide"><dt>근거 참조</dt><dd>${escapeHTML((item.spec.evidenceRefs || []).join(", ") || "없음")}</dd></div>
      </dl>${domainState === "open" ? `<div class="item-actions"><button class="button small" type="button" data-finding="acknowledge" data-id="${escapeHTML(item.metadata.id)}">확인함으로 표시</button></div>` : ""}</details></div>
      <div class="ledger-row__context"><span>상태 ${escapeHTML(label(domainState))}</span><span>${escapeHTML(scope || "전체 범위")}</span></div>
      <div class="ledger-row__action"></div>
    </article>`;
  }

  const qualityHomeObject = value => value && typeof value === "object" && !Array.isArray(value) ? value : {};
  const normalizeQualityHome = value => {
    const data = qualityHomeObject(value);
    return {
      summary: qualityHomeObject(data.summary),
      queue: Array.isArray(data.queue) ? data.queue.filter(item => item && typeof item === "object") : [],
      objectives: Array.isArray(data.objectives) ? data.objectives : [],
    };
  };
  const normalizeQualityTools = value => {
    const data = qualityHomeObject(value);
    return {
      checkedAt: data.checkedAt || "",
      tools: Array.isArray(data.tools) ? data.tools.filter(item => item && typeof item === "object") : [],
      capabilities: Array.isArray(data.capabilities) ? data.capabilities.filter(item => item && typeof item === "object") : [],
    };
  };
  const qualityToolsStateLabels = {
    available: "탐색됨",
    present_unverified: "후보 발견 · 미검증",
    needs_target: "대상 필요",
    missing: "탐색되지 않음",
    installed_not_registered: "설치됐지만 미등록",
    untrusted: "신뢰 확인 안 됨",
  };
  const qualityToolsStateLabel = value => qualityToolsStateLabels[value] || "상태 미상";
  const qualityToolsStateClass = value => Object.prototype.hasOwnProperty.call(qualityToolsStateLabels, value) ? value : "unknown";
  const qualityToolsErrorMessage = error => {
    const raw = String(error?.message || "").trim();
    const technical = error?.name === "SyntaxError" || error?.name === "TypeError" || /JSON|fetch|network|failed to load|요청에 실패했습니다\.\s*\(\d{3}\)/i.test(raw);
    if (raw && !technical) return `품질 도구 탐색 결과를 불러오지 못했습니다. 서버 안내: ${raw} 서버 버전과 연결 상태를 확인한 뒤 다시 확인하세요.`;
    return "품질 도구 탐색 결과를 불러오지 못했습니다. 서버 버전과 연결 상태를 확인한 뒤 다시 확인하세요.";
  };
  const qualityToolsItemMeta = item => [
    item.version ? `버전 ${item.version}` : "",
    item.path ? `경로 ${item.path}` : "",
    item.runnerId ? `실행기 ${item.runnerId}` : "",
    item.toolId ? `도구 ${item.toolId}` : "",
  ].filter(Boolean).join(" · ");
  const renderQualityToolsItem = (item, kind) => {
    const stateValue = String(item?.state || "");
    const stateClass = qualityToolsStateClass(stateValue);
    const name = String(item?.name || item?.id || "이름 없는 품질 항목");
    const reason = String(item?.reason || "상태 설명이 제공되지 않았습니다.");
    const guidance = String(item?.installGuidance || "설치·설정 안내가 제공되지 않았습니다.");
    const meta = qualityToolsItemMeta(item);
    return `<article class="quality-tools-entry quality-tools-entry--${stateClass}">
      <div class="quality-tools-entry__heading"><div><h4>${escapeHTML(name)}</h4><p class="quality-tools-entry__kind">${escapeHTML(kind)}</p></div><span class="quality-tools-state quality-tools-state--${stateClass}" aria-label="상태: ${escapeHTML(qualityToolsStateLabel(stateValue))}">${escapeHTML(qualityToolsStateLabel(stateValue))}</span></div>
      <p class="quality-tools-entry__reason">${escapeHTML(reason)}</p>
      <p class="quality-tools-entry__guidance"><span>다음 확인</span>${escapeHTML(guidance)}</p>
      ${meta ? `<p class="quality-tools-entry__meta">${escapeHTML(meta)}</p>` : ""}
    </article>`;
  };
  function renderQualityTools() {
    const container = document.getElementById("quality-tools");
    if (!container) return;
    const qualityTools = state.qualityTools || { status: "loading", data: null, error: "" };
    const data = normalizeQualityTools(qualityTools.data);
    const loading = qualityTools.status === "loading";
    const failed = qualityTools.status === "error";
    const hasTools = data.tools.length > 0;
    const hasCapabilities = data.capabilities.length > 0;
    container.setAttribute("aria-busy", String(loading));
    if (loading) {
      container.innerHTML = '<div class="quality-tools-loading"><strong>로컬 도구 탐색 결과를 불러오는 중입니다.</strong><span>서버가 제공한 도구·기능 탐색 결과를 읽고 있습니다.</span></div>';
      return;
    }
    if (failed) {
      container.innerHTML = `<div class="quality-tools-error" role="alert"><strong>품질 도구 탐색 결과를 확인하지 못했습니다.</strong><span>${escapeHTML(qualityTools.error || "서버 버전과 연결 상태를 확인한 뒤 다시 확인하세요.")}</span><button class="button small" type="button" data-quality-tools-retry>다시 확인</button></div>`;
      return;
    }
    if (!hasTools && !hasCapabilities) {
      container.innerHTML = '<div class="quality-tools-empty"><strong>탐색 결과에 품질 도구나 기능이 없습니다.</strong><span>이 서버가 제공한 도구·기능 탐색 결과가 비어 있습니다.</span></div>';
      return;
    }
    const checkedAt = data.checkedAt ? `<span>탐색 시각 ${escapeHTML(formatDate(data.checkedAt))}</span>` : "";
    container.innerHTML = `<div class="quality-tools-summary">${checkedAt}<span>도구 ${formatCount(data.tools.length)}개 · 기능 ${formatCount(data.capabilities.length)}개</span></div>${hasTools ? `<section class="quality-tools-group" aria-labelledby="quality-tools-installed-title"><h3 id="quality-tools-installed-title">도구 항목</h3><div class="quality-tools-list">${data.tools.map(item => renderQualityToolsItem(item, "도구")).join("")}</div></section>` : ""}${hasCapabilities ? `<section class="quality-tools-group" aria-labelledby="quality-tools-capabilities-title"><h3 id="quality-tools-capabilities-title">품질 기능 항목</h3><div class="quality-tools-list">${data.capabilities.map(item => renderQualityToolsItem(item, "기능")).join("")}</div></section>` : ""}`;
  }
  const qualitySummaryCount = (summary, key) => {
    const value = Number(summary?.[key]);
    return Number.isFinite(value) && value >= 0 ? formatCount(value) : "—";
  };
  const qualityQueueKindLabels = {
    QualityObjective: "품질 목표",
    QualityRun: "품질 점검",
    PRCIBaseline: "PR/CI 기준선",
    AssuranceProposal: "검증 제안",
    Finding: "확인 항목",
    Proposal: "점검 제안",
  };
  const qualityQueueStateLabels = {
    critic_advisory: "검토 대기",
    proposed: "검토 대기",
  };
  const qualityQueueKindLabel = kind => qualityQueueKindLabels[kind] || kind || "개선 항목";
  const qualityQueueStateLabel = stateValue => qualityQueueStateLabels[stateValue] || label(stateValue);
  const qualityQueueTone = item => {
    const severity = String(item?.severity || "").toLowerCase();
    const itemState = String(item?.state || "").toLowerCase();
    if (["critical", "high"].includes(severity) || ["failed", "timed_out", "interrupted"].includes(itemState)) return "negative";
    if (["attention"].includes(severity) || ["blocked", "stale", "pending", "proposed", "critic_advisory"].includes(itemState)) return "attention";
    return "neutral";
  };
  const qualityQueueScope = item => [item?.projectId, item?.repositoryId, item?.worktreeId].filter(Boolean).join(" / ") || "전체 범위";
  const qualityQueueAction = item => {
    const kind = String(item?.kind || "");
    const stateValue = String(item?.state || "");
    const projectID = String(item?.projectId || "");
    const referenceID = String(item?.referenceId || "");
    const projectPath = projectID ? `#projects/${encode(projectID)}` : "#projects";
    if (kind === "Finding") {
      return { label: "확인 항목 열기", href: projectID && referenceID ? `${projectPath}?finding=${encode(referenceID)}` : projectPath };
    }
    if (kind === "QualityObjective") return { label: "개선 과제 열기", href: referenceID ? `#home?objective=${encode(referenceID)}` : "#home" };
    if (kind === "Proposal") return { label: "점검 제안 검토", href: "#work" };
    if (kind === "QualityRun") return { label: ["queued", "running", "cancelling"].includes(stateValue) ? "진행 상태 보기" : "검증 결과 보기", href: "#assurance" };
    if (kind === "PRCIBaseline") return { label: "기준선 확인", href: "#assurance" };
    if (kind === "AssuranceProposal") return { label: "검증 제안 검토", href: "#assurance" };
    return { label: "관련 기록 보기", href: "#activity" };
  };

  function renderQualityQueueItem(item, index) {
    const tone = qualityQueueTone(item);
    const kind = qualityQueueKindLabel(String(item.kind || ""));
    const stateValue = String(item.state || "");
    const title = String(item.title || "").trim() || "제목이 기록되지 않은 개선 항목";
    const reason = String(item.summary || "").trim() || "개선 사유가 기록되지 않았습니다.";
    const action = qualityQueueAction(item);
    const itemID = String(item.id || item.referenceId || `queue-${index}`);
    return `<article class="quality-queue-item ${rowToneClass(tone)}" data-quality-queue-id="${escapeHTML(itemID)}">
      <div class="quality-queue-item__status">
        <span class="eyebrow">${escapeHTML(kind)}</span>
        ${stateText(qualityQueueStateLabel(stateValue), tone)}
      </div>
      <div class="quality-queue-item__body">
        <div class="quality-queue-item__heading"><h3>${escapeHTML(title)}</h3></div>
        <p class="quality-queue-item__why"><span>이유</span>${escapeHTML(reason)}</p>
        <div class="quality-queue-item__footer"><span class="quality-queue-item__scope">${escapeHTML(qualityQueueScope(item))} · 업데이트 ${escapeHTML(formatDate(item.updatedAt))}</span><a class="button small quality-queue-item__action" href="${escapeHTML(action.href)}">${escapeHTML(action.label)}</a></div>
      </div>
    </article>`;
  }

  const qualityRunStatusText = run => {
    const spec = run?.spec || {};
    if (spec.outcome === "runner_unavailable" || spec.state === "unavailable") return "이 대상에서는 사용할 수 없음";
    if (spec.state === "succeeded") return "완료";
    if (spec.state === "running" || spec.state === "queued") return "실행 중";
    return label(spec.state || "unknown");
  };
  const qualityRunUnavailableText = run => {
    const reason = String(run?.spec?.staleReason || "");
    if (reason.includes("worktree")) return "선택한 저장소 상태를 다시 확인할 수 없습니다.";
    if (reason.includes("runner")) return "이 저장소에서 Go 점검 도구를 사용할 수 없습니다.";
    return "이 저장소에서 현재 점검을 사용할 수 없습니다.";
  };
  const runsForTarget = target => (state.assuranceRuns || [])
    .filter(item => item.spec?.projectId === target?.projectID && item.spec?.repositoryId === target?.repositoryID && item.spec?.worktreeId === target?.worktreeID)
    .sort((left, right) => new Date(right.spec?.startedAt || 0) - new Date(left.spec?.startedAt || 0));

  function renderQualityHome() {
    const targetSelect = document.getElementById("home-target");
    const nextStep = document.getElementById("home-next-step");
    const checkState = document.getElementById("home-check-state");
    const issues = document.getElementById("home-issues");
    if (!targetSelect || !nextStep || !checkState || !issues) return;
    const targets = targetOptions();
    if (!state.selectedTargetValue || !targets.some(target => target.value === state.selectedTargetValue)) {
      state.selectedTargetValue = targets[0]?.value || "";
    }
    targetSelect.innerHTML = targets.length
      ? targets.map(target => `<option value="${escapeHTML(target.value)}">${escapeHTML(target.label)}</option>`).join("")
      : '<option value="">등록된 저장소가 없습니다</option>';
    targetSelect.value = state.selectedTargetValue;
    targetSelect.disabled = !targets.length || Boolean(state.qualityRunPending);
    const target = selectedTarget();
    if (!target) {
      const hasProjects = (state.snapshot.projects || []).length > 0;
      nextStep.innerHTML = hasProjects
        ? '<div class="home-next-state"><strong>저장소 상태를 새로 고치세요.</strong><p>등록된 프로젝트는 있지만 아직 관찰된 Worktree가 없습니다. 저장소 상태를 다시 읽은 뒤 코드 검사 대상을 고릅니다.</p><button class="button primary small" type="button" data-repository-refresh>저장소 상태 새로 고침</button></div>'
        : '<div class="home-next-state"><strong>첫 저장소를 등록하세요.</strong><p>저장소를 등록하면 이곳에서 바로 실행할 점검 대상을 고를 수 있습니다.</p><a class="button primary small" href="#projects">프로젝트 등록</a></div>';
      checkState.innerHTML = "";
      issues.innerHTML = "";
      return;
    }
    rememberTarget(target.value);
    const runs = runsForTarget(target);
    const latest = runs[0];
    const offline = state.service.status === "offline" || state.service.status === "error";
    const actionHref = `#work?target=${encode(target.value)}`;
    const latestLabel = latest ? qualityRunStatusText(latest) : "아직 실행하지 않음";
    nextStep.innerHTML = latest
      ? `<div class="home-next-state"><span class="eyebrow">${escapeHTML(assuranceTechniqueLabels[latest.spec?.technique] || latest.spec?.technique || "품질 점검")}</span><strong>${escapeHTML(latestLabel === "완료" ? "최근 점검 결과를 확인하세요." : "최근 점검 결과를 다시 확인하세요.")}</strong><p>${escapeHTML(latest.spec?.summary || qualityRunUnavailableText(latest))}</p><div class="item-actions"><a class="button primary small" href="${escapeHTML(actionHref)}">작업에서 다시 실행</a><a class="button small" href="#assurance?run=${encode(latest.metadata?.id || "")}">결과 열기</a></div></div>`
      : `<div class="home-next-state"><span class="eyebrow">구성 확인</span><strong>이 저장소의 구성을 확인하세요.</strong><p>언어와 구성 요소를 자동으로 확인하고, 필요한 준비 안내를 대상에 맞춰 표시합니다. 실제 실행 가능 여부와 결과는 별도 단계입니다.</p><a class="button primary small" href="${escapeHTML(actionHref)}">작업에서 구성 확인</a></div>`;
    checkState.innerHTML = `<div class="home-check-summary"><div><span class="eyebrow">선택한 대상</span><strong>${escapeHTML(target.label)}</strong><span class="meta">${escapeHTML(target.branch)} · HEAD ${escapeHTML(target.head || "확인 전")}</span></div><span class="chip ${latest ? (latest.spec?.state === "succeeded" ? "ok" : "warn") : ""}">${escapeHTML(latestLabel)}</span></div>${offline ? '<p class="meta">오프라인 상태입니다. 마지막으로 불러온 결과만 표시하며 새 점검은 실행할 수 없습니다.</p>' : latest ? `<p class="meta">마지막 실행 ${escapeHTML(formatDate(latest.spec?.startedAt))} · 결과는 작업 화면에서 대상별로 이어집니다.</p>` : '<p class="meta">저장된 실행 결과가 없습니다. 아직 0건이라는 뜻이 아니라, 이 대상의 실행 기록을 확인하지 못한 상태일 수 있습니다.</p>'}`;
    const projectFindings = openFindings().filter(item => item.spec?.projectId === target.projectID);
    issues.innerHTML = projectFindings.length
      ? `<div class="home-issue-summary"><strong>저장소 확인 항목 ${escapeHTML(projectFindings.length)}개</strong><span>Git 상태 확인 항목입니다. 품질 점검 결과와 섞지 않았습니다.</span><a class="text-link" href="#projects/${encode(target.projectID)}">프로젝트에서 확인</a></div>`
      : '<div class="empty-state"><strong>저장소 확인 항목이 없습니다.</strong><span>저장소 상태와 품질 점검 결과는 서로 다른 기록으로 표시합니다.</span></div>';
  }

  const qualityObjectiveStateLabels = {
    draft: "초안",
    baseline_pending: "기준 확인 대기",
    ready: "개선 준비",
    running: "점검 중",
    review: "개선 확인 대기",
    adopted: "개선 완료",
    rejected: "제외됨",
    stale: "근거 오래됨",
    blocked: "보류됨",
  };
  const qualityObjectiveOutcomeLabels = {
    improved: "개선 확인",
    not_improved: "아직 개선되지 않음",
    inconclusive: "확인 불가",
  };
  const qualityObjectiveSignalLabels = {
    finding: "확인 항목",
    go_coverage: "Go 커버리지 점검",
  };
  const qualityObjectiveStateLabel = value => qualityObjectiveStateLabels[value] || "상태 미상";
  const qualityObjectiveOutcomeLabel = value => qualityObjectiveOutcomeLabels[value] || "결과 미상";
  const qualityObjectiveSignalLabel = value => qualityObjectiveSignalLabels[value] || "원본 신호 미상";
  const qualityObjectiveTone = value => {
    if (["adopted", "review"].includes(value)) return "positive";
    if (["rejected", "stale"].includes(value)) return "negative";
    if (["blocked", "baseline_pending"].includes(value)) return "attention";
    return "neutral";
  };
  const qualityObjectiveSpec = value => value?.spec && typeof value.spec === "object" ? value.spec : {};
  const qualityObjectiveLatest = spec => (Array.isArray(spec.revalidations) ? spec.revalidations : []).slice().sort((left, right) => {
    const rightTime = new Date(right.checkedAt || 0).getTime();
    const leftTime = new Date(left.checkedAt || 0).getTime();
    if (rightTime !== leftTime) return rightTime - leftTime;
    return Number(right.sequence || 0) - Number(left.sequence || 0);
  })[0] || null;
  const qualityObjectiveDraftValue = (draft, key) => String(draft?.[key] ?? "");
  const qualityObjectiveMutationError = error => error?.status === 409
    ? "다른 화면에서 과제가 변경되었습니다. 최신 상태를 다시 불러온 뒤 확인하세요."
    : error?.message || "요청을 완료하지 못했습니다. 잠시 후 다시 시도하세요.";
  const qualityObjectiveDecisionStates = new Set(["draft", "baseline_pending", "blocked"]);

  function renderQualityObjectiveDecisionForm(spec, draft) {
    if (!qualityObjectiveDecisionStates.has(spec.state)) return "";
    const disposition = qualityObjectiveDraftValue(draft, "disposition");
    const coverage = spec.primarySignal?.kind === "go_coverage";
    return `<section class="quality-objective-form" aria-labelledby="quality-objective-decision-title">
      <div class="section-heading"><div><span class="eyebrow">사람의 결정</span><h3 id="quality-objective-decision-title">이 과제를 어떻게 다룰까요?</h3></div></div>
      <form data-quality-objective-decision class="form-grid">
        <label><span>처리 방침</span><select name="disposition" data-quality-objective-disposition required><option value="">선택하세요</option><option value="pursue" ${disposition === "pursue" ? "selected" : ""}>개선하기</option><option value="defer" ${disposition === "defer" ? "selected" : ""}>보류하기</option><option value="dismiss" ${disposition === "dismiss" ? "selected" : ""}>제외하기</option></select></label>
        <label><span>담당자</span><input name="actor" value="${escapeHTML(qualityObjectiveDraftValue(draft, "actor"))}" autocomplete="off" required placeholder="직접 입력"></label>
        ${disposition === "pursue" ? `<label class="wide"><span>하기로 한 일</span><textarea name="action" rows="3" required placeholder="예: 경계 입력 테스트를 추가합니다.">${escapeHTML(qualityObjectiveDraftValue(draft, "action"))}</textarea></label>` : ""}
        ${["defer", "dismiss"].includes(disposition) ? `<label class="wide"><span>이유</span><textarea name="reason" rows="3" required placeholder="결정 이유를 직접 입력하세요.">${escapeHTML(qualityObjectiveDraftValue(draft, "reason"))}</textarea></label>` : ""}
        ${coverage ? `<label><span>최소 커버리지 (%)</span><input name="minimumPercent" type="number" min="0" max="100" step="0.01" value="${escapeHTML(qualityObjectiveDraftValue(draft, "minimumPercent"))}" placeholder="기준을 직접 입력"></label>` : ""}
        <div class="item-actions wide"><button class="button primary small" type="submit">결정 저장</button></div>
      </form>
    </section>`;
  }

  function renderQualityObjectiveRevalidationForm(spec, draft) {
    if (!spec.decision || ["adopted", "rejected", "stale"].includes(spec.state)) return "";
    const sourceKind = qualityObjectiveDraftValue(draft, "sourceKind") || (spec.primarySignal?.kind === "finding" ? "finding" : "qualityRun");
    const fieldName = sourceKind === "finding" ? "findingId" : "qualityRunId";
    const sourceHint = sourceKind === "finding" ? "후속 Finding ID" : "후속 Quality Run ID";
    return `<section class="quality-objective-form" aria-labelledby="quality-objective-revalidation-title">
      <div class="section-heading"><div><span class="eyebrow">결과 연결</span><h3 id="quality-objective-revalidation-title">후속 점검 결과 연결</h3></div></div>
      <form data-quality-objective-revalidation class="form-grid">
        <fieldset class="quality-objective-source-choice wide"><legend>결과 종류</legend><label><input type="radio" name="sourceKind" value="finding" data-quality-objective-source ${sourceKind === "finding" ? "checked" : ""}> Finding</label><label><input type="radio" name="sourceKind" value="qualityRun" data-quality-objective-source ${sourceKind === "qualityRun" ? "checked" : ""}> Quality Run</label></fieldset>
        <label class="wide"><span>${sourceHint}</span><input name="sourceId" value="${escapeHTML(qualityObjectiveDraftValue(draft, "sourceValue"))}" autocomplete="off" required placeholder="ID를 직접 입력"><small>서버가 저장된 결과를 확인해 개선 여부를 판정합니다.</small></label>
        <div class="item-actions wide"><button class="button small" type="submit">결과 연결</button></div>
      </form>
    </section>`;
  }

  function renderQualityObjectiveDetail() {
    const panel = document.getElementById("quality-objective-detail");
    if (!panel) return;
    const detail = state.qualityObjective || { selectedID: "", status: "idle", data: null, error: "", mutation: { draft: {} } };
    const selectedID = detail.selectedID;
    panel.hidden = !selectedID;
    if (!selectedID) {
      panel.innerHTML = "";
      return;
    }
    const loading = detail.status === "loading" || detail.status === "idle";
    panel.setAttribute("aria-busy", String(loading));
    if (loading) {
      panel.innerHTML = '<div class="quality-objective-loading"><strong>개선 과제 상세를 불러오는 중입니다.</strong><span>원본 신호와 최신 재검증 기록을 확인합니다.</span></div>';
      return;
    }
    if (detail.status === "not_found") {
      panel.innerHTML = '<div class="quality-objective-error" role="alert"><strong>개선 과제를 찾지 못했습니다.</strong><span>큐에서 항목이 제거되었거나 주소가 오래되었습니다.</span><a class="button small" href="#home">큐로 돌아가기</a></div>';
      return;
    }
    if (detail.status === "error" || !detail.data) {
      panel.innerHTML = `<div class="quality-objective-error" role="alert"><strong>개선 과제 상세를 불러오지 못했습니다.</strong><span>${escapeHTML(detail.error || "잠시 후 다시 시도하세요.")}</span><button class="button small" type="button" data-quality-objective-retry>다시 불러오기</button></div>`;
      return;
    }
    const spec = qualityObjectiveSpec(detail.data);
    const stateValue = String(spec.state || "");
    const latest = qualityObjectiveLatest(spec);
    const draft = detail.mutation?.draft || {};
    const mutationError = detail.mutation?.error ? `<p class="quality-objective-mutation-error" role="alert">${escapeHTML(detail.mutation.error)}</p>` : "";
    const signal = spec.primarySignal;
    const signalDetail = signal ? `<dl class="detail-grid"><div><dt>종류</dt><dd>${escapeHTML(qualityObjectiveSignalLabel(signal.kind))}</dd></div><div><dt>원본 ID</dt><dd><code>${escapeHTML(signal.id)}</code></dd></div>${signal.fingerprint ? `<div class="wide"><dt>Fingerprint</dt><dd><code>${escapeHTML(signal.fingerprint)}</code></dd></div>` : ""}${signal.head ? `<div><dt>원본 HEAD</dt><dd><code>${escapeHTML(signal.head)}</code></dd></div>` : ""}${signal.configDigest ? `<div><dt>Config digest</dt><dd><code>${escapeHTML(signal.configDigest)}</code></dd></div>` : ""}<div><dt>관측 시각</dt><dd>${escapeHTML(formatDate(signal.observedAt))}</dd></div></dl>` : '<p class="empty-state">연결된 원본 신호가 없습니다.</p>';
    const decision = spec.decision;
    const decisionDetail = decision ? `<dl class="detail-grid"><div><dt>처리 방침</dt><dd>${escapeHTML(decision.disposition === "pursue" ? "개선하기" : decision.disposition === "defer" ? "보류하기" : decision.disposition === "dismiss" ? "제외하기" : "상태 미상")}</dd></div><div><dt>담당자</dt><dd>${escapeHTML(decision.actor || "기록 없음")}</dd></div>${decision.action ? `<div class="wide"><dt>하기로 한 일</dt><dd>${escapeHTML(decision.action)}</dd></div>` : ""}${decision.reason ? `<div class="wide"><dt>이유</dt><dd>${escapeHTML(decision.reason)}</dd></div>` : ""}${signal?.kind === "go_coverage" && decision.minimumPercent ? `<div><dt>최소 커버리지</dt><dd>${escapeHTML(decision.minimumPercent)}%</dd></div>` : ""}<div><dt>결정 시각</dt><dd>${escapeHTML(formatDate(decision.decidedAt))}</dd></div></dl>` : '<p class="empty-state">아직 처리 방침을 정하지 않았습니다.</p>';
    const revalidations = Array.isArray(spec.revalidations) ? spec.revalidations : [];
    const timeline = revalidations.length ? `<ol class="quality-objective-timeline">${revalidations.map(item => `<li><div><strong>${escapeHTML(qualityObjectiveOutcomeLabel(item.outcome))}</strong><span>${escapeHTML(formatDate(item.checkedAt))}</span></div><p>${escapeHTML(item.sourceKind === "finding" ? "Finding" : item.sourceKind === "go_coverage" ? "Quality Run" : "원본 신호 미상")} · <code>${escapeHTML(item.sourceId || "기록 없음")}</code></p><small>${escapeHTML(item.reasonCode || "판정 이유 없음")}</small></li>`).join("")}</ol>` : '<p class="empty-state">아직 연결한 재검증 결과가 없습니다.</p>';
    const canConfirm = stateValue === "review" && latest?.outcome === "improved";
    const confirm = canConfirm ? '<div class="quality-objective-confirm"><p>최신 결과에서 개선이 확인되었습니다. 현재 Worktree를 다시 확인한 뒤 완료 처리할 수 있습니다.</p><button class="button primary small" type="button" data-quality-objective-confirm>완료 확인</button></div>' : "";
    const staleNote = stateValue === "stale" ? '<p class="quality-objective-notice" role="status">현재 Worktree와 저장된 근거가 달라 다시 확인해야 합니다.</p>' : "";
    panel.innerHTML = `<div class="quality-objective-detail-header"><div><span class="eyebrow">선택한 개선 과제</span><h2 id="quality-objective-detail-title">근거와 다음 단계</h2><p class="meta">${escapeHTML([spec.projectId, spec.repositoryId, spec.worktreeId].filter(Boolean).join(" / ") || "범위 기록 없음")}</p></div><span class="quality-objective-state quality-objective-state--${escapeHTML(qualityObjectiveTone(stateValue))}">${escapeHTML(qualityObjectiveStateLabel(stateValue))}</span></div>${staleNote}${mutationError}<div class="quality-objective-sections"><section><div class="section-heading"><div><span class="eyebrow">시작 신호</span><h3>무엇을 보고 시작했나요?</h3></div></div>${signalDetail}</section><section><div class="section-heading"><div><span class="eyebrow">결정</span><h3>사람이 정한 처리 방향</h3></div></div>${decisionDetail}</section><section><div class="section-heading"><div><span class="eyebrow">재검증 기록</span><h3>최근 결과 ${latest ? `· ${escapeHTML(qualityObjectiveOutcomeLabel(latest.outcome))}` : ""}</h3></div></div>${timeline}</section></div>${renderQualityObjectiveDecisionForm(spec, draft)}${renderQualityObjectiveRevalidationForm(spec, draft)}${confirm}`;
  }

  function renderHome() {
    const projects = state.snapshot.projects || [];
    const findings = openFindings();
    const established = projects.length > 0;
    document.querySelectorAll("[data-home-established-only]").forEach(element => { element.hidden = !established; });
    document.getElementById("home-title").textContent = established ? "이 저장소에서 다음 코드를 검사하세요." : "저장소를 연결하세요.";
    document.getElementById("home-subtitle").textContent = established ? "선택한 대상의 코드 검사와 최근 결과를 한곳에서 이어 봅니다." : "프로젝트를 등록하면 검사할 저장소와 브랜치를 선택할 수 있습니다.";
    document.getElementById("home-onboarding").hidden = established;
    document.getElementById("m-scan").textContent = formatDate(state.snapshot.generated_at);
    renderQualityHome();

    document.getElementById("home-projects").innerHTML = projects.length
      ? `<div class="item-list">${projects.map(project => {
        const projectFindings = findings.filter(item => item.spec.projectId === project.id);
        const high = projectFindings.filter(item => ["high", "critical"].includes(item.spec.severity)).length;
        const tone = high ? "negative" : projectFindings.length ? "attention" : "positive";
        const summary = `${project.repos.length}개 저장소 · 확인 항목 ${projectFindings.length}개${high ? ` · 높음 ${high}개` : ""}`;
        return `<button class="ledger-row project-card ${rowToneClass(tone)}" type="button" data-tone="${tone}" data-finding-count="${escapeHTML(projectFindings.length)}" data-open-project="${escapeHTML(project.id)}"><span class="ledger-row__state">${stateText(projectFindings.length ? "확인 필요" : "정상", tone)}</span><span class="ledger-row__main"><strong>${escapeHTML(project.name)}</strong><span>${escapeHTML(summary)}</span></span><span class="ledger-row__context"><code>${escapeHTML(project.id)}</code></span><span class="ledger-row__action">열기</span></button>`;
      }).join("")}</div>`
      : '<div class="empty-state"><strong>등록된 프로젝트가 없습니다.</strong><span>프로젝트 화면에서 첫 저장소를 등록하세요.</span></div>';

    const recentRuns = state.events.filter(item => /(action|check|run|scan)/i.test(item.spec.type)).slice().reverse();
    document.getElementById("home-runs").innerHTML = recentRuns.length
      ? `<div class="item-list">${recentRuns.slice(0, 5).map(item => { const tone = eventTone(item); return `<article class="ledger-row ${rowToneClass(tone)}" data-tone="${escapeHTML(tone)}" data-state="${escapeHTML(item.spec.status || item.spec.conclusion || "recorded")}"><div class="ledger-row__state">${stateText("기록", tone)}</div><div class="ledger-row__main"><h3>${escapeHTML(localize(item.spec.summary))}</h3></div><div class="ledger-row__context">${escapeHTML(formatDate(item.spec.occurredAt))}</div><div class="ledger-row__action"></div></article>`; }).join("")}</div>`
      : '<div class="empty-state"><strong>아직 실행 결과가 없습니다.</strong><span>작업 화면에서 검토된 점검이나 Action을 실행할 수 있습니다.</span></div>';
    renderProviderStatuses("home-providers", true);
    const assurance = state.assuranceDashboard || {};
    const runs = state.assuranceRuns || [];
    document.getElementById("home-assurance").innerHTML = runs.length || assurance.invocations?.length || assurance.effects?.length
      ? renderHomeAssurance(assurance, runs)
       : '<div class="empty-state"><strong>아직 검증 결과가 없습니다.</strong><span>Quality Run을 실행하면 근거와 효과 기록이 여기에 나타납니다.</span></div>';
  }

  function renderAssuranceFilters() {
    const providerSelect = document.getElementById("assurance-provider-filter");
    const modelSelect = document.getElementById("assurance-model-filter");
    const projectSelect = document.getElementById("assurance-project-filter");
    const daysSelect = document.getElementById("assurance-days-filter");
    const effectSelect = document.getElementById("assurance-effect-filter");
    if (!providerSelect || !modelSelect || !projectSelect || !daysSelect || !effectSelect) return;
    const providers = [...new Set([
      ...(state.providerStatuses || []).map(item => item.provider),
      ...(state.assuranceInvocations || []).map(item => item.spec?.provider),
    ].filter(Boolean))].sort();
    const models = [...new Set((state.assuranceInvocations || []).map(item => item.spec?.requestedModel).filter(Boolean))].sort();
    const projects = (state.snapshot.projects || []).map(project => ({ id: project.id, name: project.name || project.id })).filter(project => project.id);
    const optionMarkup = (value, selected, fallback) => `<option value="${escapeHTML(value)}" ${value === selected ? "selected" : ""}>${escapeHTML(fallback || value)}</option>`;
    providerSelect.innerHTML = optionMarkup("", state.assuranceFilters.provider, "전체 Provider") + providers.map(value => optionMarkup(value, state.assuranceFilters.provider)).join("");
    modelSelect.innerHTML = optionMarkup("", state.assuranceFilters.model, "전체 모델") + models.map(value => optionMarkup(value, state.assuranceFilters.model)).join("");
    projectSelect.innerHTML = optionMarkup("", state.assuranceFilters.project, "전체 프로젝트") + projects.map(project => optionMarkup(project.id, state.assuranceFilters.project, project.name)).join("");
    if (state.assuranceFilters.provider && !providers.includes(state.assuranceFilters.provider)) {
      providerSelect.insertAdjacentHTML("beforeend", optionMarkup(state.assuranceFilters.provider, state.assuranceFilters.provider));
    }
    if (state.assuranceFilters.model && !models.includes(state.assuranceFilters.model)) {
      modelSelect.insertAdjacentHTML("beforeend", optionMarkup(state.assuranceFilters.model, state.assuranceFilters.model));
    }
    if (state.assuranceFilters.project && !projects.some(project => project.id === state.assuranceFilters.project)) {
      projectSelect.insertAdjacentHTML("beforeend", optionMarkup(state.assuranceFilters.project, state.assuranceFilters.project));
    }
    providerSelect.value = state.assuranceFilters.provider;
    modelSelect.value = state.assuranceFilters.model;
    projectSelect.value = state.assuranceFilters.project;
    daysSelect.value = state.assuranceFilters.days;
    effectSelect.value = state.assuranceEffectFilter;
  }

  const primaryQualityTechniques = [
    {
      id: "static_security",
      label: "Go 정적 검사 (go vet)",
      description: "Go 코드의 정적 분석을 실행합니다. 파일을 수정하지 않습니다.",
      command: "go vet -mod=readonly ./...",
    },
    {
      id: "go_test_coverage",
      label: "테스트·커버리지",
      description: "저장소 테스트를 실행하고 커버리지를 수집합니다. 테스트 코드를 실행합니다.",
      command: "go test -mod=readonly -count=1 -covermode=set -coverprofile=<자동 생성> ./...",
    },
  ];
  const campaignForTarget = target => (state.qualityCampaigns || []).find(item =>
    item.spec?.projectId === target?.projectID && item.spec?.repositoryId === target?.repositoryID && item.spec?.worktreeId === target?.worktreeID);
  const qualityTargetTechnicalDetails = target => `<details class="target-detail"><summary>정확한 대상과 기술 정보</summary><dl class="detail-grid"><div><dt>프로젝트 ID</dt><dd><code>${escapeHTML(target?.projectID || "기록 없음")}</code></dd></div><div><dt>저장소 ID</dt><dd><code>${escapeHTML(target?.repositoryID || "기록 없음")}</code></dd></div><div><dt>Worktree ID</dt><dd><code>${escapeHTML(target?.worktreeID || "기록 없음")}</code></dd></div><div class="wide"><dt>경로</dt><dd><code>${escapeHTML(target?.repositoryPath || "기록 없음")}</code></dd></div><div><dt>브랜치</dt><dd>${escapeHTML(target?.branch || "기록 없음")}</dd></div><div><dt>HEAD</dt><dd><code>${escapeHTML(target?.head || "확인 전")}</code></dd></div></dl></details>`;
  const qualitySetupStatusLabel = value => qualitySetupStatusLabels[value] || "상태 미상";
  const qualitySetupStatusTone = value => value === "existing_configuration" ? "positive" : value === "unsupported" ? "neutral" : "attention";
  const qualitySetupLanguageLabel = value => qualitySetupLanguageLabels[value] || value || "언어 미상";
  const qualitySetupList = value => Array.isArray(value) ? value.filter(Boolean).map(item => String(item)) : [];
  const qualitySetupPreviewText = value => {
    if (value === null || value === undefined) return "";
    if (typeof value === "string") return value;
    if (typeof value === "object") return [value.label, value.path, value.command, value.reason].filter(Boolean).join(" · ");
    return String(value);
  };
  const qualitySetupEvidence = (target, data) => ({
    targetValue: target?.value || "",
    head: String(data?.head || target?.head || ""),
    digest: String(data?.digest || target?.digest || ""),
  });
  const qualitySetupEvidenceKnown = (target, data) => {
    const evidence = qualitySetupEvidence(target, data);
    return Boolean(data && (evidence.head || evidence.digest));
  };
  const qualitySetupEvidenceChanged = (target, before, after) => {
    if (!qualitySetupEvidenceKnown(target, before) || !qualitySetupEvidenceKnown(target, after)) return false;
    const previous = qualitySetupEvidence(target, before);
    const next = qualitySetupEvidence(target, after);
    return previous.head !== next.head || previous.digest !== next.digest;
  };
  const qualitySetupPreferenceStorageKey = "dev-control-room.quality-setup-preferences";
  const qualitySetupPreferenceKey = (target, data) => JSON.stringify(qualitySetupEvidence(target, data));
  let qualitySetupPreferences = null;
  const readQualitySetupPreferences = () => {
    if (qualitySetupPreferences) return qualitySetupPreferences;
    try {
      const parsed = JSON.parse(window.localStorage.getItem(qualitySetupPreferenceStorageKey) || "{}");
      qualitySetupPreferences = parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : {};
    } catch (_) {
      qualitySetupPreferences = {};
    }
    return qualitySetupPreferences;
  };
  const qualitySetupPreferenceForTarget = (target, data) => {
    const preferences = readQualitySetupPreferences();
    const evidence = qualitySetupEvidence(target, data);
    const exact = preferences[qualitySetupPreferenceKey(target, data)];
    if (qualitySetupEvidenceKnown(target, data) && exact && exact.targetValue === evidence.targetValue && exact.head === evidence.head && exact.digest === evidence.digest) {
      return { mode: exact.mode === "manual" ? "manual" : "auto", languages: qualitySetupList(exact.languages), reset: false, pending: false };
    }
    // A first request has no server evidence yet. Do not mistake the target's
    // incomplete metadata for a changed repository and erase a saved preference.
    if (!qualitySetupEvidenceKnown(target, data)) return { mode: "auto", languages: [], reset: false, pending: true };
    const previous = Object.values(preferences).find(item => item?.targetValue === evidence.targetValue);
    return { mode: "auto", languages: [], reset: Boolean(previous), pending: false };
  };
  const saveQualitySetupPreference = (target, data, preference) => {
    const preferences = readQualitySetupPreferences();
    const key = qualitySetupPreferenceKey(target, data);
    Object.keys(preferences).forEach(itemKey => {
      if (itemKey !== key && preferences[itemKey]?.targetValue === target.value) delete preferences[itemKey];
    });
    preferences[key] = {
      ...qualitySetupEvidence(target, data),
      mode: preference?.mode === "manual" ? "manual" : "auto",
      languages: qualitySetupList(preference?.languages),
    };
    try {
      window.localStorage.setItem(qualitySetupPreferenceStorageKey, JSON.stringify(preferences));
    } catch (_) { /* preference remains available for this page when storage is unavailable */ }
  };
  const qualitySetupPath = (target, preference) => {
    const query = new URLSearchParams({ projectId: target.projectID, repositoryId: target.repositoryID, worktreeId: target.worktreeID });
    if (preference?.mode === "manual" && qualitySetupList(preference.languages).length) query.set("languages", qualitySetupList(preference.languages).join(","));
    return `/api/quality/setup?${query.toString()}`;
  };
  const qualitySetupDetectedLanguages = data => qualitySetupList(data?.detectedLanguages);
  const qualitySetupHasDetectedGo = data => qualitySetupDetectedLanguages(data).includes("go");
  const qualitySetupComponentLanguages = component => qualitySetupList(component?.languages);
  const qualitySetupCheckLanguages = { ruff: ["python"], pytest: ["python"], eslint: ["javascript", "typescript"], vitest: ["javascript", "typescript"], "go-test": ["go"] };
  const qualitySetupCheckIsRelevant = (check, component, preference) => preference?.mode !== "manual" ||
    (qualitySetupCheckLanguages[check?.id] || (String(check?.id || "").startsWith("script:") ? ["javascript", "typescript"] : qualitySetupComponentLanguages(component))).some(language => qualitySetupList(preference.languages).includes(language));
  const qualitySetupHasRunnableGo = (data, preference) => qualitySetupHasDetectedGo(data) &&
    (preference?.mode !== "manual" || qualitySetupList(preference.languages).includes("go")) &&
    (Array.isArray(data?.components) ? data.components : []).some(component => {
    const path = String(component?.path || "");
    return (path === "." || path === "") && qualitySetupComponentLanguages(component).includes("go");
  });
  const qualitySetupCheckLabels = { ruff: "Python 코드 검사 · Ruff", pytest: "Python 테스트 · pytest", eslint: "코드 검사 · ESLint", vitest: "테스트 · Vitest", "go-test": "Go 구성" };
  const qualitySetupCheckLabel = check => {
    const id = String(check?.id || "");
    if (/^script:[a-zA-Z0-9:_-]+$/.test(id)) return `${id.slice(7)} 스크립트`;
    return qualitySetupCheckLabels[id] || "검사 준비";
  };
  const qualitySetupCheckReason = check => {
    if (check?.id === "go-test") return "Go 구성을 확인했습니다. 현재 Go 실행은 저장소 최상위 폴더만 지원합니다.";
    return ({
      existing_configuration: "기존 설정을 찾았습니다. 실행 환경은 아직 확인하지 않았습니다.",
      setup_needed: "확인한 범위에서 설정을 찾지 못했습니다. 준비 방법을 확인하세요.",
      ambiguous: "설정을 확정하지 못했습니다. 구성 파일과 패키지 관리자 정보를 확인하세요.",
      unsupported: "현재 버전에서 이 검사 준비를 지원하지 않습니다.",
    })[check?.status] || "검사 준비 상태를 확인하지 못했습니다.";
  };
  const qualitySetupWarningText = value => {
    const warning = String(value || "");
    const fixed = {
      "excluded child directories were not inspected": ".git·의존성·빌드 결과 폴더는 기본 확인 범위에서 제외합니다.",
      "directory entry limit reached (2000)": "폴더 항목 2,000개까지 확인해 나머지는 읽지 못했습니다.",
      "maximum inspection depth reached (3)": "하위 폴더 3단계까지만 확인했습니다.",
      "eligible file limit reached (64)": "구성 파일 64개까지 확인해 나머지는 읽지 못했습니다.",
      "inspection canceled": "구성 확인이 중단되었습니다.",
      "inspection canceled or interrupted": "구성 확인이 중단되어 일부만 읽었습니다.",
    };
    if (Object.hasOwn(fixed, warning)) return fixed[warning];
    const prefixes = [
      ["unreadable or unsafe directory: ", "폴더를 읽지 못했거나 경로 상태가 바뀌었습니다"],
      ["directory listing incomplete: ", "폴더 목록을 일부만 읽었습니다"],
      ["skipped symlink: ", "연결된 파일·폴더는 따라가지 않았습니다"],
      ["skipped special file: ", "일반 파일이 아닌 항목은 읽지 않았습니다"],
      ["unreadable or unsafe evidence: ", "구성 파일을 읽지 못했거나 경로 상태가 바뀌었습니다"],
      ["evidence exceeds 128 KiB or changed while reading: ", "파일이 128 KiB를 넘거나 읽는 중 바뀌어 전체를 확인하지 못했습니다"],
      ["conflicting package manager evidence: ", "여러 패키지 관리자 정보가 있어 사용할 도구를 확정하지 못했습니다"],
      ["unsupported or malformed TOML; declaration detection limited: ", "TOML 형식을 전부 해석하지 못해 일부 설정만 확인했습니다"],
      ["unsupported or malformed TOML: ", "TOML 형식을 전부 해석하지 못했습니다"],
      ["unsupported dependency declaration: ", "의존성 선언 일부를 해석하지 못했습니다"],
      ["dynamic project metadata is not resolved: ", "실행해야 알 수 있는 프로젝트 정보는 확인하지 않았습니다"],
      ["unsupported requirements syntax; references are not followed: ", "의존성 선언 일부는 읽지 못했으며 다른 파일 참조는 따라가지 않았습니다"],
      ["unsupported JSONC or malformed JSON: ", "JSON 형식을 전부 해석하지 못했습니다"],
      ["malformed JSON: ", "JSON 형식을 해석하지 못했습니다"],
      ["malformed JSON or unsupported package metadata: ", "패키지 설정 형식을 전부 해석하지 못했습니다"],
      ["unsupported declared package manager: ", "선언된 패키지 관리자는 현재 준비 안내에서 지원하지 않습니다"],
      ["unsupported eslintConfig metadata: ", "ESLint 설정 형식을 해석하지 못했습니다"],
      ["unsupported or malformed setup.cfg; detection limited: ", "setup.cfg 형식을 전부 해석하지 못해 일부 설정만 확인했습니다"],
    ];
    const match = prefixes.find(([prefix]) => warning.startsWith(prefix));
    return match ? `${match[1]} · ${warning.slice(match[0].length)}` : "일부 구성 정보를 확인하지 못했습니다. 구성 파일을 확인한 뒤 다시 불러오세요.";
  };
  const qualitySetupErrorText = error => {
    if (error?.status === 404) return "구성 확인 기능 또는 선택한 저장소를 찾지 못했습니다. 서비스와 대상 등록 상태를 확인하세요.";
    if (error?.status === 400) return "대상이나 언어 선택을 확인한 뒤 다시 불러오세요.";
    return "선택한 저장소의 구성을 읽지 못했습니다. 연결과 저장소 상태를 확인한 뒤 다시 불러오세요.";
  };
  const qualitySetupCheckPreview = check => {
    const command = String(check?.commandPreview || "").trim();
    const setup = qualitySetupList(check?.setupPreview).map(qualitySetupPreviewText).filter(Boolean);
    return `${command ? `<div class="wide"><dt>확인할 명령 미리보기</dt><dd><code>${escapeHTML(command)}</code></dd></div>` : ""}${setup.length ? `<div class="wide"><dt>준비 미리보기</dt><dd><ul>${setup.map(item => `<li>${escapeHTML(item)}</li>`).join("")}</ul></dd></div>` : ""}`;
  };
  const renderQualitySetupCheck = check => {
    const goCheck = check?.id === "go-test";
    const status = String(check?.status || "");
    const preview = qualitySetupCheckPreview(check);
    return `<article class="list-item quality-check-card"><div class="list-item-header"><h4>${escapeHTML(qualitySetupCheckLabel(check))}</h4>${stateText(goCheck ? "구성만 확인" : qualitySetupStatusLabel(status), goCheck ? "neutral" : qualitySetupStatusTone(status))}</div><p>${escapeHTML(qualitySetupCheckReason(check))}</p>${preview ? `<details><summary>준비 방법과 명령 보기</summary><dl class="detail-grid">${preview}</dl></details>` : ""}</article>`;
  };
  const renderQualitySetupComponent = (component, preference, showGoActions = false) => {
    const languages = qualitySetupComponentLanguages(component);
    const frameworks = qualitySetupList(component?.frameworks);
    const evidence = Array.isArray(component?.evidence) ? component.evidence : [];
    const rootGo = showGoActions && ["", "."].includes(component?.path || "") && languages.includes("go");
    const checks = (Array.isArray(component?.checks) ? component.checks : [])
      .filter(check => qualitySetupCheckIsRelevant(check, component, preference))
      .filter(check => !(rootGo && check?.id === "go-test"));
    const summary = [languages.map(qualitySetupLanguageLabel).join(" · "), frameworks.join(" · "), component?.packageManager].filter(Boolean).join(" · ");
    const evidenceKinds = { manifest: "프로젝트 설정", configuration: "도구 설정", lockfile: "의존성 잠금 파일" };
    const name = component?.path && component.path !== "." ? component.path : "저장소 최상위 폴더";
    return `<article class="list-item quality-component-card"><div class="list-item-header"><div><h3>${escapeHTML(name)}</h3><p class="meta">${escapeHTML(summary || "관찰된 구성 정보")}</p></div><span class="state-text state-neutral">관찰됨</span></div>${checks.length ? `<div class="item-list">${checks.map(renderQualitySetupCheck).join("")}</div>` : `<p class="meta">${rootGo ? "Go 검사는 아래 실행 영역에서 확인하세요." : "선택한 언어의 검사 준비 항목은 없습니다. 구성과 근거는 계속 표시합니다."}</p>`}<details><summary>확인한 근거 보기</summary>${evidence.length ? `<ul>${evidence.map(item => `<li><code>${escapeHTML(item?.path || "경로 미상")}</code> · ${escapeHTML(evidenceKinds[item?.kind] || "구성 근거")}</li>`).join("")}</ul>` : '<p class="meta">확인된 근거가 없습니다.</p>'}</details></article>`;
  };
  const renderQualitySetup = (target, setup) => {
    const container = document.getElementById("quality-setup");
    if (!container) return;
    const data = setup?.data;
    const preference = qualitySetupPreferenceForTarget(target, data);
    const mode = preference.reset ? "auto" : preference.mode;
    const detectedLanguages = qualitySetupDetectedLanguages(data);
    const languageChoices = [...new Set([...qualitySetupLanguages, ...detectedLanguages])];
    const checkedLanguages = mode === "manual" ? preference.languages : detectedLanguages;
    const languageOptions = languageChoices.map(language => `<label><input type="checkbox" name="quality-language" data-quality-language="${escapeHTML(language)}" ${checkedLanguages.includes(language) ? "checked" : ""} ${mode === "auto" ? "disabled" : ""}><span>${escapeHTML(qualitySetupLanguageLabel(language))}</span></label>`).join("");
    const loading = setup?.status === "loading";
    const error = setup?.status === "error";
    const partial = Boolean(data?.partial);
    const warnings = [...new Set(qualitySetupList(data?.warnings).map(qualitySetupWarningText))];
    const components = Array.isArray(data?.components) ? data.components : [];
    const showGoActions = setup?.status === "ready" && qualitySetupHasRunnableGo(data, { mode, languages: checkedLanguages });
    const scopeDetails = data ? `<details class="quality-setup-scope"><summary>확인 범위와 참고 사항</summary><p class="meta">선택한 저장소의 알려진 구성 파일만 읽습니다. 제외된 폴더와 읽기 제한은 아래에 표시합니다.</p>${warnings.length ? `<ul>${warnings.map(warning => `<li>${escapeHTML(warning)}</li>`).join("")}</ul>` : '<p class="meta">별도 참고 사항이 없습니다.</p>'}</details>` : "";
    const setupState = loading
      ? '<div class="loading"><strong>저장소 구성을 읽는 중입니다…</strong><span>선택한 저장소의 구성 근거를 읽습니다.</span></div>'
      : error
        ? `<div class="surface-error" role="alert"><strong>구성 확인을 불러오지 못했습니다.</strong><span>${escapeHTML(setup?.error || "연결과 저장소 상태를 확인한 뒤 다시 불러오세요.")}</span><button class="button small" type="button" data-quality-setup-retry>다시 확인</button></div>`
        : !data
          ? '<div class="empty-state"><strong>구성 확인을 시작하세요.</strong><span>자동 감지 결과와 확인한 근거가 이곳에 표시됩니다.</span></div>'
          : `${partial ? '<p class="meta" role="status">일부 범위만 확인했습니다. 제외된 파일이나 제한은 ‘확인 범위와 참고 사항’에서 확인하세요.</p>' : ""}${components.length ? `<div class="item-list">${components.map(component => renderQualitySetupComponent(component, { mode, languages: checkedLanguages }, showGoActions)).join("")}</div>` : '<div class="empty-state"><strong>감지된 구성 요소가 없습니다.</strong><span>선택한 저장소와 구성 파일 위치를 확인하세요. 언어 선택은 표시할 검사 범위만 바꾸며 설정 파일을 만들지 않습니다.</span></div>'}${scopeDetails}`;
    const resetTip = setup?.preferenceReset || preference.reset ? '<p class="quality-setup-reset-tip" role="status">HEAD 또는 구성 근거가 바뀌어 언어 선택을 자동 감지로 초기화했습니다.</p>' : "";
    const status = !data || loading || error ? "" : partial ? stateText("부분 결과", "attention") : components.length ? stateText(`구성 ${components.length}개 확인`, "neutral") : stateText("감지된 구성 없음", "neutral");
    const blocked = loading || !data || ["loading", "offline", "error"].includes(state.service.status);
    container.innerHTML = `<section class="quality-setup-panel" aria-labelledby="quality-setup-title"><header class="section-heading"><div><span class="eyebrow">구성 확인</span><h3 id="quality-setup-title">언어와 구성 요소를 확인하세요.</h3><p class="meta">감지 결과가 다르면 언어를 직접 선택하세요. 선택하지 않은 구성도 목록에 남습니다. 구성 확인됨은 실행 완료를 뜻하지 않습니다.</p></div>${status}</header><fieldset class="quality-language-options" ${blocked ? "disabled" : ""}><legend>표시할 검사 언어</legend><label><input type="radio" name="quality-language-mode" value="auto" data-quality-language-mode ${mode === "auto" ? "checked" : ""}>자동 감지 <span class="meta">관찰된 구성 전체</span></label><label><input type="radio" name="quality-language-mode" value="manual" data-quality-language-mode ${mode === "manual" ? "checked" : ""}>직접 선택 <span class="meta">선택한 언어의 검사 준비만 보기</span></label><div class="quality-language-checkboxes">${languageOptions}</div><button class="button small" type="button" data-quality-setup-apply>${mode === "manual" ? "선택한 검사만 보기" : "구성 다시 확인"}</button></fieldset>${resetTip}${setupState}</section>`;
  };
  const qualityRunInlineResult = item => {
    const spec = item?.spec || {};
    const id = item?.metadata?.id || "";
    const unavailable = spec.outcome === "runner_unavailable" || String(spec.staleReason || "").includes("unavailable");
    const tone = unavailable ? "neutral" : spec.state === "succeeded" ? "positive" : ["failed", "timed_out", "cancelled"].includes(spec.state) ? "negative" : "attention";
    return `<article id="quality-run-result-${escapeHTML(id)}" class="quality-run-result ${rowToneClass(tone)}" tabindex="-1"><div class="quality-run-result__heading"><div><strong>${escapeHTML(assuranceTechniqueLabels[spec.technique] || spec.technique || "코드 검사")}</strong><span class="meta">${escapeHTML(formatDate(spec.startedAt))}</span></div>${stateText(unavailable ? "사용할 수 없음" : qualityRunStatusText(item), tone)}</div><p>${escapeHTML(unavailable ? qualityRunUnavailableText(item) : spec.summary || "결과 요약이 없습니다.")}</p>${spec.coverage ? `<p class="meta">커버리지 ${escapeHTML(formatImpactValue(spec.coverage.percent, "percent"))} · ${escapeHTML(formatCount(spec.coverage.coveredStatements))} / ${escapeHTML(formatCount(spec.coverage.totalStatements))} statements</p>` : ""}<details><summary>실행 명령과 근거</summary><dl class="detail-grid"><div class="wide"><dt>실행 명령</dt><dd><code>${escapeHTML([spec.command?.executable, ...(spec.command?.arguments || [])].filter(Boolean).join(" ") || "기록 없음")}</code></dd></div><div><dt>종료 코드</dt><dd>${escapeHTML(spec.exitCode ?? "기록 없음")}</dd></div><div><dt>HEAD</dt><dd><code>${escapeHTML(spec.head || "기록 없음")}</code></dd></div><div><dt>결과 ID</dt><dd><code>${escapeHTML(id || "기록 없음")}</code></dd></div></dl></details><div class="item-actions"><a class="button small" href="#assurance?run=${encode(id)}">검증 결과에서 보기</a></div></article>`;
  };

  const qualityInspectionPlanStateLabels = {
    proposed: "제안됨",
    reviewed: "검토됨",
    approved: "승인됨",
    stale: "오래됨",
    rejected: "거절됨",
  };
  const qualityImprovementStateLabels = {
    proposed: "제안됨",
    reviewed: "검토됨",
    approved: "승인됨",
    rejected: "거절됨",
    applied: "적용됨",
    stale: "오래됨",
  };
  const qualityImprovementActionLabels = {
    add_check: "검사 추가",
    remove_check: "검사 제외",
    mark_runner_unavailable: "실행기 사용 불가로 표시",
  };
  const qualityImprovementReasonLabels = {
    applicable_check: "적용 가능한 검사",
    finding_observed: "확인 항목에서 제안",
    inconclusive: "판정 보류에서 제안",
    runner_unavailable: "실행기 사용 불가에서 제안",
  };
  const qualityInspectionScoreStatusLabels = {
    fresh: "최신",
    stale: "오래됨",
    inconclusive: "판정 보류",
  };
  const qualityInspectionOutcomeLabels = {
    clean: "문제 없음",
    findings: "확인 항목 있음",
    tests_failed: "검사 실패",
    tool_error: "도구 오류",
    runner_unavailable: "실행기 사용 불가",
    inconclusive: "판정 보류",
  };
  const qualityInspectionCheckLabels = {
    ruff: "Ruff",
    pytest: "pytest",
    eslint: "ESLint",
    vitest: "Vitest",
    "quality.go.test": "Go test",
    "quality.go.test_race": "Go test · race",
    "quality.go.vet": "Go vet",
    "quality.go.mod_verify": "Go mod verify",
    "quality.go.build": "Go build",
    "quality.go.coverage": "Go coverage",
    "quality.go.coverage_percent": "Go coverage %",
    "quality.go.mutation": "변이 검사",
    "quality.go.property": "속성 검사",
    "quality.go.fuzz": "Fuzz 검사",
    "quality.go.e2e": "E2E 검사",
    "quality.go.test_coverage": "테스트 커버리지",
  };
  const qualityInspectionStateLabel = (value, labels, fallback = "상태 미상") => labels[value] || fallback;
  const qualityInspectionPlanSpec = item => item?.spec || {};
  const qualityInspectionScoreSpec = item => item?.spec || {};
  const qualityInspectionPlanTone = value => value === "approved" ? "positive" : ["stale", "rejected"].includes(value) ? "negative" : ["proposed", "reviewed"].includes(value) ? "attention" : "neutral";
  const qualityInspectionScoreTone = value => value === "fresh" ? "positive" : value === "stale" ? "attention" : "negative";
  const qualityInspectionOutcomeTone = value => value === "clean" ? "positive" : ["findings", "tests_failed"].includes(value) ? "attention" : "negative";
  const qualityInspectionScopeMatches = (item, target) => {
    const spec = item?.spec || {};
    return Boolean(target && spec.projectId === target.projectID && spec.repositoryId === target.repositoryID && spec.worktreeId === target.worktreeID);
  };
  const qualityInspectionUpdatedAt = item => Date.parse(item?.spec?.updatedAt || item?.spec?.createdAt || item?.metadata?.createdAt || "") || 0;
  const qualityInspectionPlansForTarget = target => (state.qualityInspection?.plans || [])
    .filter(item => qualityInspectionScopeMatches(item, target))
    .sort((left, right) => qualityInspectionUpdatedAt(right) - qualityInspectionUpdatedAt(left));
  const qualityInspectionScoresForTarget = target => (state.qualityInspection?.scores || [])
    .filter(item => qualityInspectionScopeMatches(item, target))
    .sort((left, right) => qualityInspectionUpdatedAt(right) - qualityInspectionUpdatedAt(left));
  const qualityInspectionPlanChecks = plan => {
    const checks = Array.isArray(plan?.spec?.checks) ? plan.spec.checks : [];
    return checks.map(check => `<li><strong>${escapeHTML(qualityInspectionCheckLabels[check.id] || "검사 항목")}</strong><span><code>${escapeHTML(check.id || "unknown")}</code> · ${escapeHTML(check.componentId || "구성 요소 미상")}</span></li>`).join("");
  };
  const qualityInspectionGenerationNote = plan => {
    const generation = plan?.generation || {};
    if (generation.aiProposal === true) return '<p class="quality-inspection-note">AI 제안이 포함된 계획입니다. 결정적 근거와 제안 내용을 구분해 검토하세요.</p>';
    return `<p class="quality-inspection-note">AI 제안은 연결되지 않았습니다. 현재 계획은 저장소 구성에서 결정적으로 만든 fallback이며, 자동으로 맞다고 가정하지 말고 실행 전에 검토하세요.${generation.reason ? ` <code>${escapeHTML(generation.reason)}</code>` : ""}</p>`;
  };
  const qualityInspectionGenerationNoteForWorkflow = (plan, workflow) => {
    const generation = plan?.generation || {};
    if (generation.aiProposal === true) return '<p class="quality-inspection-note">AI는 허용된 검사 항목 후보만 제안했습니다. 점수와 실행 명령을 결정하지 않으며, 사람의 검토·승인 후에만 실행할 수 있습니다.</p>';
    if (workflow.aiEnabled === true) return '<p class="quality-inspection-note">AI 제안을 요청했지만 연결되지 않아 결정적 fallback으로 만들었습니다. 점수와 실행은 검사 계획과 사람의 승인으로 결정됩니다.</p>';
    return qualityInspectionGenerationNote(plan);
  };
  const qualityInspectionAIControlHTML = workflow => `<label class="quality-inspection-ai-toggle"><input type="checkbox" data-quality-inspection-ai ${workflow.aiEnabled === true ? "checked" : ""}><span><strong>AI로 검사 항목 후보도 받아보기</strong><small>기본 꺼짐 · 후보만 제안하며 점수·실행은 결정하지 않습니다.</small></span></label>`;
  const qualityInspectionPlanActions = (plan, workflow) => {
    const spec = qualityInspectionPlanSpec(plan);
    const id = plan?.metadata?.id || "";
    const pending = workflow.mutation?.status === "submitting";
    if (!id || !spec.revision) return "";
    if (spec.state === "proposed") {
      return `<button class="button small" type="button" data-quality-inspection-review="review" data-id="${escapeHTML(id)}" ${pending ? "disabled" : ""}>검토 완료로 표시</button><button class="button primary small" type="button" data-quality-inspection-review="approve" data-id="${escapeHTML(id)}" ${pending ? "disabled" : ""}>검토 후 승인</button><button class="button small" type="button" data-quality-inspection-review="reject" data-id="${escapeHTML(id)}" ${pending ? "disabled" : ""}>거절</button>`;
    }
    if (spec.state === "reviewed") {
      return `<button class="button primary small" type="button" data-quality-inspection-review="approve" data-id="${escapeHTML(id)}" ${pending ? "disabled" : ""}>승인</button><button class="button small" type="button" data-quality-inspection-review="reject" data-id="${escapeHTML(id)}" ${pending ? "disabled" : ""}>거절</button>`;
    }
    if (spec.state === "approved") {
      return `<button class="button primary small" type="button" data-quality-inspection-run data-id="${escapeHTML(id)}" ${pending ? "disabled" : ""}>검사 실행</button>`;
    }
    return `<button class="button small" type="button" data-quality-inspection="generate" ${pending ? "disabled" : ""}>현재 상태로 새 계획 만들기</button>`;
  };
  const qualityInspectionPlanHTML = (plan, workflow) => {
    if (!plan) {
      const noApplicable = /no applicable inspection checks/i.test(workflow.mutation?.error || workflow.error || "");
      return `<div class="empty-state quality-inspection-empty"><strong>${noApplicable ? "적용 가능한 검사 항목이 없습니다." : "아직 검사 계획이 없습니다."}</strong><span>${noApplicable ? "지원되는 언어·설정과 실제 실행기를 확인한 뒤 새로고침하세요. 실행할 수 없는 Go test fallback은 만들지 않습니다." : "선택한 Worktree의 구성과 확인 가능한 검사 항목을 읽어 계획을 만드세요."}</span><button class="button primary small" type="button" data-quality-inspection="generate" aria-label="선택한 Worktree의 검사 계획 만들기">검사 계획 만들기</button></div>`;
    }
    const spec = qualityInspectionPlanSpec(plan);
    const stateValue = String(spec.state || "");
    const status = qualityInspectionStateLabel(stateValue, qualityInspectionPlanStateLabels);
    const tone = qualityInspectionPlanTone(stateValue);
    return `<article class="quality-inspection-plan"><div class="quality-inspection-card-heading"><div><span class="eyebrow">검사 계획</span><h3>${escapeHTML(plan.metadata?.name || "저장소 검사 계획")}</h3><p class="meta">revision ${escapeHTML(spec.revision || "미상")} · ${escapeHTML(formatDate(spec.updatedAt || spec.createdAt))}</p></div>${stateText(status, tone)}</div>${qualityInspectionGenerationNoteForWorkflow(plan, workflow)}<ul class="quality-inspection-checks" aria-label="검사 계획에 포함된 검사">${qualityInspectionPlanChecks(plan) || "<li><span>검사 항목이 기록되지 않았습니다.</span></li>"}</ul><details><summary>Worktree와 계획 근거 보기</summary><dl class="detail-grid"><div><dt>브랜치</dt><dd>${escapeHTML(spec.branch || "확인 불가")}</dd></div><div><dt>HEAD</dt><dd><code>${escapeHTML(spec.head || "확인 불가")}</code></dd></div><div><dt>구성 digest</dt><dd><code>${escapeHTML(spec.configDigest || "기록 없음")}</code></dd></div><div><dt>근거 digest</dt><dd><code>${escapeHTML(spec.evidenceDigest || "기록 없음")}</code></dd></div><div><dt>도구 digest</dt><dd><code>${escapeHTML(spec.toolDigest || "기록 없음")}</code></dd></div><div><dt>계획 digest</dt><dd><code>${escapeHTML(spec.digest || "기록 없음")}</code></dd></div></dl></details><div class="item-actions quality-inspection-actions">${qualityInspectionPlanActions(plan, workflow)}</div>${workflow.mutation?.status === "error" ? `<p class="quality-inspection-error" role="alert">${escapeHTML(workflow.mutation.error || "계획 요청을 완료하지 못했습니다.")}</p>` : ""}</article>`;
  };
  const qualityInspectionScoreHTML = (score, workflow) => {
    if (!score) return '<div class="empty-state"><strong>아직 실행된 품질 점수가 없습니다.</strong><span>승인된 계획을 실행하면 결정적 점수와 확신도가 기록됩니다.</span></div>';
    const spec = qualityInspectionScoreSpec(score);
    const status = qualityInspectionStateLabel(spec.status, qualityInspectionScoreStatusLabels);
    const tone = qualityInspectionScoreTone(spec.status);
    const components = Array.isArray(spec.components) ? spec.components : [];
    return `<article class="quality-inspection-score"><div class="quality-inspection-card-heading"><div><span class="eyebrow">통합 품질 점수</span><h3><strong class="quality-inspection-score-value">${escapeHTML(spec.overall ?? "—")}</strong><span>/ 100</span></h3><p class="meta">${escapeHTML(formatDate(spec.updatedAt || spec.createdAt))} · score ${escapeHTML(spec.scoreVersion || "버전 미상")}</p></div><span class="quality-inspection-score-status ${toneClass(tone)}">${escapeHTML(status)}</span></div><div class="quality-inspection-score-meta"><span><strong>${escapeHTML(spec.confidence ?? "—")}%</strong> 확신도</span><span>HEAD <code>${escapeHTML(spec.head || "확인 불가")}</code></span></div>${components.length ? `<div class="quality-inspection-component-list">${components.map(component => `<div><span>${escapeHTML(qualityInspectionCheckLabels[component.checkId] || "검사 항목")}</span><strong>${escapeHTML(component.value ?? "—")}</strong><small>${escapeHTML(qualityInspectionOutcomeLabels[component.outcome] || "상태 미상")} · 확신도 ${escapeHTML(component.confidence ?? "—")}% · 확인 항목 ${escapeHTML(component.findingCount ?? 0)}건</small></div>`).join("")}</div>` : ""}${spec.status === "stale" ? '<p class="quality-inspection-note">현재 Worktree 또는 검사 근거가 바뀌어 최신 점수로 볼 수 없습니다. 새 계획을 만들어 다시 확인하세요.</p>' : spec.status === "inconclusive" ? '<p class="quality-inspection-note">실행기나 결과 근거가 충분하지 않아 점수를 확정하지 않았습니다.</p>' : ""}</article>`;
  };
  const qualityInspectionComparisonReason = value => ({
    scope_mismatch: "서로 다른 프로젝트·저장소·Worktree라 비교할 수 없습니다.",
    score_version_mismatch: "점수 버전이 달라 비교할 수 없습니다.",
    score_is_not_fresh: "비교 대상 중 최신 상태가 아닌 점수가 있습니다.",
  }[value] || "비교에 필요한 근거가 충분하지 않습니다.");
  const qualityInspectionComparisonHTML = workflow => {
    const scores = qualityInspectionScoresForTarget(selectedTarget());
    if (scores.length < 2) return '<div class="empty-state"><strong>비교할 점수가 아직 부족합니다.</strong><span>같은 Worktree에서 검사 계획을 두 번 이상 실행하면 전후 점수를 비교할 수 있습니다.</span></div>';
    const comparison = workflow.comparison || {};
    const defaultAfter = scores[0]?.metadata?.id || "";
    const defaultBefore = scores[1]?.metadata?.id || "";
    const beforeID = comparison.beforeID && scores.some(item => item.metadata?.id === comparison.beforeID) ? comparison.beforeID : defaultBefore;
    const afterID = comparison.afterID && scores.some(item => item.metadata?.id === comparison.afterID) ? comparison.afterID : defaultAfter;
    const result = comparison.data;
    const resultMarkup = comparison.status === "loading"
      ? '<p class="quality-inspection-loading" role="status">전후 점수를 비교하는 중입니다…</p>'
      : comparison.status === "error"
        ? `<p class="quality-inspection-error" role="alert">${escapeHTML(comparison.error || "비교 결과를 불러오지 못했습니다.")}</p>`
        : result
          ? `<div class="quality-inspection-comparison-result"><span class="quality-inspection-score-status ${result.status === "comparable" ? "state-positive" : "state-warning"}">${escapeHTML(result.status === "comparable" ? "비교 가능" : "비교 불가")}</span>${result.status === "comparable" ? `<strong>${escapeHTML(result.beforeOverall)} → ${escapeHTML(result.afterOverall)} <em>${Number(result.delta) > 0 ? "+" : ""}${escapeHTML(result.delta)}</em></strong><span>점수 변화 · ${Number(result.delta) > 0 ? "개선" : Number(result.delta) < 0 ? "하락" : "변화 없음"}</span>` : `<span>${escapeHTML(qualityInspectionComparisonReason(result.reason))}</span>`}</div>`
          : "";
    return `<div class="quality-inspection-comparison"><div class="quality-inspection-comparison-controls"><label><span>이전 점수</span><select data-quality-inspection-before>${scores.map(item => `<option value="${escapeHTML(item.metadata?.id || "")}" ${item.metadata?.id === beforeID ? "selected" : ""}>${escapeHTML(formatDate(item.spec?.createdAt || item.spec?.updatedAt))} · ${escapeHTML(item.spec?.overall ?? "—")}점</option>`).join("")}</select></label><label><span>이후 점수</span><select data-quality-inspection-after>${scores.map(item => `<option value="${escapeHTML(item.metadata?.id || "")}" ${item.metadata?.id === afterID ? "selected" : ""}>${escapeHTML(formatDate(item.spec?.createdAt || item.spec?.updatedAt))} · ${escapeHTML(item.spec?.overall ?? "—")}점</option>`).join("")}</select></label><button class="button small" type="button" data-quality-inspection-compare ${beforeID === afterID || comparison.status === "loading" ? "disabled" : ""}>전후 비교</button></div>${resultMarkup}</div>`;
  };
  const qualityInspectionLastRunArtifactID = run => run?.resultArtifactId || run?.resultArtifactID || run?.resultArtifact || "";
  const qualityImprovementScopeMatches = (item, target) => qualityInspectionScopeMatches(item, target);
  const qualityInspectionProposalActions = (proposal, workflow) => {
    const spec = proposal?.spec || {};
    const id = proposal?.metadata?.id || "";
    const pending = workflow.proposalMutation?.status === "submitting";
    if (!id || !spec.revision) return "";
    if (spec.state === "proposed") return `<button class="button small" type="button" data-quality-inspection-proposal-review="review" data-id="${escapeHTML(id)}" ${pending ? "disabled" : ""}>검토 완료로 표시</button><button class="button primary small" type="button" data-quality-inspection-proposal-review="approve" data-id="${escapeHTML(id)}" ${pending ? "disabled" : ""}>검토 후 승인</button><button class="button small" type="button" data-quality-inspection-proposal-review="reject" data-id="${escapeHTML(id)}" ${pending ? "disabled" : ""}>거절</button>`;
    if (spec.state === "reviewed") return `<button class="button primary small" type="button" data-quality-inspection-proposal-review="approve" data-id="${escapeHTML(id)}" ${pending ? "disabled" : ""}>승인</button><button class="button small" type="button" data-quality-inspection-proposal-review="reject" data-id="${escapeHTML(id)}" ${pending ? "disabled" : ""}>거절</button>`;
    if (spec.state === "approved") return `<button class="button primary small" type="button" data-quality-inspection-proposal-apply data-id="${escapeHTML(id)}" ${pending ? "disabled" : ""}>승인한 변경 적용</button>`;
    return "";
  };
  const qualityInspectionProposalHTML = (workflow, run, plan) => {
    const proposal = workflow.selectedProposal;
    const artifactID = qualityInspectionLastRunArtifactID(run);
    const planID = plan?.metadata?.id || run?.plan?.metadata?.id || workflow.selectedPlanID || "";
    const proposalError = workflow.proposalStatus === "error" ? `<p class="quality-inspection-note">개선 제안 API를 사용할 수 없어 이 화면에서는 제안을 읽지 못했습니다. 현재 계획과 검사 결과는 그대로 보존됩니다.</p>` : "";
    if (!proposal) {
      const action = artifactID && planID
        ? `<button class="button primary small" type="button" data-quality-inspection-proposal="generate" ${workflow.proposalMutation?.status === "submitting" ? "disabled" : ""}>최근 실행 결과로 개선 제안 만들기</button>`
        : "";
      return `<div class="empty-state quality-inspection-proposal-empty"><strong>아직 검토할 개선 제안이 없습니다.</strong><span>${artifactID ? "최근 실행 결과에서 검사 집합을 조정할 후보를 만들 수 있습니다." : "승인된 계획을 실행하면 결과 artifact를 바탕으로 enum 검사 집합 제안을 만들 수 있습니다."}</span>${action}${proposalError}</div>`;
    }
    const spec = proposal.spec || {};
    const status = qualityInspectionStateLabel(spec.state, qualityImprovementStateLabels);
    const tone = spec.state === "approved" || spec.state === "applied" ? "positive" : ["rejected", "stale"].includes(spec.state) ? "negative" : "attention";
    const changes = Array.isArray(spec.changes) ? spec.changes : [];
    const changeMarkup = changes.length
      ? `<ul class="quality-inspection-changes" aria-label="검사 집합 변경 제안">${changes.map(change => `<li><div><strong>${escapeHTML(qualityImprovementActionLabels[change.action] || "검사 집합 변경")}</strong><span>${escapeHTML(qualityInspectionCheckLabels[change.checkId] || "검사 항목")}</span></div><small>${escapeHTML(change.componentId || "구성 요소 미상")} · ${escapeHTML(qualityImprovementReasonLabels[change.reason] || "제안 근거 미상")}</small></li>`).join("")}</ul>`
      : '<p class="meta">변경 항목이 기록되지 않았습니다.</p>';
    const appliedNote = spec.state === "applied" ? '<p class="quality-inspection-note">변경을 적용했습니다. 새 검사 계획은 다시 <strong>제안됨</strong> 상태가 되었으므로, 내용을 검토하고 승인한 뒤 다시 실행해야 합니다.</p>' : "";
    return `<article class="quality-inspection-proposal"><div class="quality-inspection-card-heading"><div><span class="eyebrow">개선 제안</span><h3>${escapeHTML(proposal.metadata?.name || "검사 집합 개선 제안")}</h3><p class="meta">revision ${escapeHTML(spec.revision || "미상")} · ${escapeHTML(formatDate(spec.updatedAt || spec.createdAt))}</p></div>${stateText(status, tone)}</div><p class="quality-inspection-note">점수와 실행 명령을 바꾸지 않고, 서버가 허용한 enum 검사 집합 변경만 제안합니다. 사람의 승인 전에는 적용하지 않습니다.</p><div class="quality-inspection-rationale"><span>rationaleCode</span><strong>${escapeHTML(spec.rationaleCode || "기록 없음")}</strong><small>${escapeHTML(qualityImprovementReasonLabels[spec.rationaleCode] || "제안 근거 미상")}</small></div>${changeMarkup}<details><summary>제안의 연결 근거 보기</summary><dl class="detail-grid"><div><dt>대상 계획</dt><dd><code>${escapeHTML(spec.planId || "기록 없음")}</code></dd></div><div><dt>기준 점수</dt><dd><code>${escapeHTML(spec.baseScoreId || "기록 없음")}</code></dd></div><div><dt>기준 계획 revision</dt><dd>${escapeHTML(spec.basePlanRevision || "기록 없음")}</dd></div><div><dt>HEAD</dt><dd><code>${escapeHTML(spec.head || "기록 없음")}</code></dd></div></dl></details><div class="item-actions quality-inspection-actions">${qualityInspectionProposalActions(proposal, workflow)}</div>${appliedNote}${workflow.proposalMutation?.status === "error" ? `<p class="quality-inspection-error" role="alert">${escapeHTML(workflow.proposalMutation.error || "개선 제안 요청을 완료하지 못했습니다.")}</p>` : ""}</article>`;
  };
  const qualityToolInstallActionPlan = data => data?.plan || data?.actionPlan || (data?.metadata?.id ? data : null);
  const qualityToolInstallActionPlanID = data => qualityToolInstallActionPlan(data)?.metadata?.id || data?.planId || data?.actionPlanId || "";
  const qualityToolInstallCommand = data => {
    const plan = qualityToolInstallActionPlan(data);
    const execution = plan?.spec?.execution || data?.execution || data?.action?.command || {};
    return [execution.executable, ...(execution.arguments || [])].filter(Boolean).join(" ");
  };
  const qualityInspectionToolActionPlanHTML = workflow => {
    const actionPlan = workflow.toolActionPlan || {};
    if (actionPlan.status === "loading") return '<div class="quality-inspection-action-plan"><p class="quality-inspection-loading" role="status">설치 승인 계획을 확인하는 중입니다…</p></div>';
    if (actionPlan.status === "error") return `<div class="quality-inspection-action-plan"><span class="state-text state-warning">미리보기 전용</span><p>설치 전용 Action Plan endpoint가 없거나 응답하지 않아 승인·실행 단계로 넘어가지 않았습니다. 도구 설치 완료로 표시하지 않습니다.</p><small>${escapeHTML(actionPlan.error || "Action Plan을 만들지 못했습니다.")}</small></div>`;
    if (actionPlan.status !== "ready" || !actionPlan.data) return "";
    const plan = qualityToolInstallActionPlan(actionPlan.data);
    const id = qualityToolInstallActionPlanID(actionPlan.data);
    if (!plan || !id) return '<div class="quality-inspection-action-plan"><span class="state-text state-warning">미리보기 전용</span><p>설치 계획 응답에 승인 가능한 plan ID가 없어 실행 단계로 넘어가지 않았습니다.</p></div>';
    const spec = plan.spec || {};
    const approvalRequired = spec.approvalRequired ?? actionPlan.data.approvalRequired ?? true;
    const approved = actionPlan.approvalStatus === "approved";
    const rejected = actionPlan.approvalStatus === "rejected";
    const cancelled = actionPlan.approvalStatus === "cancelled";
    const executed = actionPlan.executionStatus === "executed";
    const pending = actionPlan.approvalStatus === "loading" || actionPlan.executionStatus === "loading";
    const approvalText = approved ? "사람 승인됨" : rejected ? "사람이 거절함" : cancelled ? "승인이 취소됨" : approvalRequired ? "사람 승인 필요" : "승인 불필요";
    const controls = executed
      ? '<p class="meta">설치 Action 실행을 요청했습니다. 실제 완료 여부는 Action 결과에서 확인하세요.</p>'
      : approved || !approvalRequired
        ? `<button class="button primary small" type="button" data-quality-tool-install-execute data-id="${escapeHTML(id)}" ${pending ? "disabled" : ""}>설치 Action 실행</button>`
        : `<button class="button primary small" type="button" data-quality-tool-install-approval data-id="${escapeHTML(id)}" ${pending ? "disabled" : ""}>사람 승인 열기</button>`;
    const nextAction = rejected ? "거절되어 실행할 수 없습니다. 필요하면 사람 승인 절차를 다시 열어 결정하세요." : cancelled ? "승인이 취소되어 실행하지 않았습니다. 다시 승인 절차를 시작할 수 있습니다." : "";
    const error = actionPlan.error ? `<p class="quality-inspection-error" role="alert">${escapeHTML(actionPlan.error)}</p>` : "";
    return `<div class="quality-inspection-action-plan"><div class="quality-inspection-action-plan-heading"><div><span class="eyebrow">설치 전용 Action Plan</span><strong>${escapeHTML(plan.metadata?.name || "품질 도구 설치")}</strong><small>plan <code>${escapeHTML(id)}</code></small></div>${stateText(approvalText, approved ? "positive" : rejected || cancelled ? "negative" : "attention")}</div><dl class="detail-grid"><div><dt>고정 command</dt><dd><code>${escapeHTML(qualityToolInstallCommand(actionPlan.data) || "기록 없음")}</code></dd></div><div><dt>승인 필요</dt><dd>${approvalRequired ? "예 · 사람 승인 후 실행" : "아니오"}</dd></div><div><dt>Action type</dt><dd><code>${escapeHTML(spec.actionType || actionPlan.data.actionType || "quality.tool.install")}</code></dd></div><div><dt>설치 범위</dt><dd>${escapeHTML(actionPlan.data.preview?.action?.environmentScope || actionPlan.data.preview?.environmentScope || "서버가 확인한 범위")}</dd></div><div class="wide"><dt>쓰기 범위</dt><dd>${escapeHTML((spec.writablePaths || actionPlan.data.preview?.action?.writablePaths || []).join(", ") || "기록 없음")}</dd></div></dl>${nextAction ? `<p class="quality-inspection-note">${escapeHTML(nextAction)}</p>` : ""}${error}<div class="item-actions">${controls}</div></div>`;
  };
  const qualityInspectionToolPreviewHTML = workflow => {
    const preview = workflow.toolPreview || {};
    const action = preview.data?.action;
    const command = action?.command ? [action.command.executable, ...(action.command.arguments || [])].filter(Boolean).join(" ") : "";
    const output = preview.status === "loading"
      ? '<p class="quality-inspection-loading" role="status">설치 계획 미리보기를 만드는 중입니다…</p>'
      : preview.status === "error"
        ? `<p class="quality-inspection-error" role="alert">${escapeHTML(preview.error || "설치 계획 미리보기를 만들지 못했습니다.")}</p>`
        : preview.status === "ready" && preview.data
          ? preview.data.available && action
            ? `<div class="quality-inspection-preview-result"><span class="state-text state-warning">승인 필요</span><strong>설치 실행은 하지 않았습니다.</strong><p>${escapeHTML(action.package || "품질 도구")} ${escapeHTML(action.version || "버전 미상")}을 선택한 Worktree에 설치하는 명령만 미리 봅니다.</p><code>${escapeHTML(command || "명령 미상")}</code><small>설치 범위: ${escapeHTML(action.environmentScope || "서버 확인 필요")} · 영향 파일: ${escapeHTML((action.affectedFiles || []).join(", ") || "기록 없음")}</small></div>`
            : `<div class="quality-inspection-preview-result"><span class="state-text state-warning">미리보기 불가</span><p>${escapeHTML(preview.data.reason || "현재 환경에서는 승인 가능한 설치 계획을 만들 수 없습니다.")}</p></div>`
          : "";
    return `<details class="quality-inspection-install"><summary><span><strong>품질 도구 설치 미리 보기</strong><small>설치하지 않고 승인 전 명령만 확인</small></span><span class="summary-action">미리 보기</span></summary><div class="quality-inspection-install-body"><p class="meta">정확한 버전과 영향을 받는 파일을 입력하면 서버가 미리보기와 설치 전용 Action Plan을 확인합니다. 이 화면은 endpoint가 제공할 때만 승인·실행으로 이어집니다.</p><form data-quality-tool-install-preview class="quality-inspection-install-form"><label><span>도구</span><select name="kind"><option value="python.ruff">Python · Ruff</option><option value="python.pytest">Python · pytest</option><option value="node.eslint">Node · ESLint</option><option value="node.vitest">Node · Vitest</option></select></label><label><span>정확한 버전</span><input name="version" autocomplete="off" placeholder="예: 0.6.9" required></label><label class="wide"><span>영향 파일</span><input name="affectedFiles" autocomplete="off" placeholder="예: pyproject.toml, package.json" required></label><label class="wide"><input type="checkbox" name="allowGlobal" value="on"><span>프로젝트 venv가 없을 때 전역 설치 허용 — 사람 승인 필수</span></label><button class="button small" type="submit" ${preview.status === "loading" ? "disabled" : ""}>미리보기 확인</button></form>${output}${qualityInspectionToolActionPlanHTML(workflow)}</div></details>`;
  };
  function renderQualityInspectionWorkflow() {
    const targetContainer = document.getElementById("quality-inspection-target");
    const contentContainer = document.getElementById("quality-inspection-content");
    if (!targetContainer || !contentContainer) return;
    const workflow = state.qualityInspection || { status: "idle", plans: [], scores: [], selectedPlanID: "", selectedPlan: null, lastRun: null, comparison: { status: "idle" }, toolPreview: { status: "idle" }, toolActionPlan: { status: "idle" }, mutation: { status: "idle" } };
    const targets = targetOptions();
    if (!state.selectedTargetValue || !targets.some(target => target.value === state.selectedTargetValue)) state.selectedTargetValue = targets[0]?.value || "";
    const target = selectedTarget();
    targetContainer.innerHTML = target
      ? `<div class="quality-inspection-target-controls"><label for="quality-inspection-target"><span>검사할 등록 Worktree</span><select id="quality-inspection-target" aria-label="저장소 품질 검사 Worktree">${targets.map(item => `<option value="${escapeHTML(item.value)}" ${item.value === target.value ? "selected" : ""}>${escapeHTML(item.label)}</option>`).join("")}</select></label><div class="target-detail"><div><span>경로</span><code>${escapeHTML(target.repositoryPath || "경로 확인 불가")}</code></div><div><span>브랜치</span><strong>${escapeHTML(target.branch || "확인 불가")}</strong></div><div><span>HEAD</span><code>${escapeHTML(target.head || "확인 불가")}</code></div></div></div>`
      : '<div class="empty-state"><strong>선택된 Worktree가 없습니다.</strong><span>현재 등록된 프로젝트에서 검사할 저장소와 Worktree를 먼저 관찰하세요.</span><a class="button small" href="#projects">프로젝트에서 등록하기</a></div>';
    contentContainer.setAttribute("aria-busy", String(workflow.status === "loading" || workflow.mutation?.status === "submitting"));
    if (!target) {
      contentContainer.innerHTML = '<div class="quality-inspection-no-target"><strong>검사 흐름을 시작할 수 없습니다.</strong><span>등록된 Worktree가 생기면 계획 생성·검토·실행을 사용할 수 있습니다.</span></div>';
      return;
    }
    if (workflow.status === "loading") {
      contentContainer.innerHTML = '<div class="quality-inspection-loading" role="status"><strong>검사 계획과 점수를 불러오는 중입니다…</strong><span>현재 Worktree에 연결된 기록을 확인합니다.</span></div>';
      return;
    }
    if (workflow.status === "error" && !workflow.plans?.length && !workflow.scores?.length) {
      contentContainer.innerHTML = `<div class="quality-inspection-error" role="alert"><strong>저장소 품질 기록을 불러오지 못했습니다.</strong><span>${escapeHTML(workflow.error || "서버 버전과 연결 상태를 확인한 뒤 다시 시도하세요.")}</span><button class="button small" type="button" data-quality-inspection-refresh>다시 시도</button></div>`;
      return;
    }
    const plans = qualityInspectionPlansForTarget(target);
    const selected = workflow.selectedPlan && qualityInspectionScopeMatches(workflow.selectedPlan, target)
      ? workflow.selectedPlan
      : plans.find(item => item.metadata?.id === workflow.selectedPlanID) || plans[0] || null;
    if (selected && selected.metadata?.id !== workflow.selectedPlanID) workflow.selectedPlanID = selected.metadata.id;
    const scores = qualityInspectionScoresForTarget(target);
    const score = scores[0] || null;
    const run = workflow.lastRun && qualityInspectionScopeMatches(workflow.lastRun.score, target) ? workflow.lastRun : null;
    contentContainer.innerHTML = `<div class="quality-inspection-grid"><section aria-labelledby="quality-inspection-plan-title"><header class="section-heading"><div><h3 id="quality-inspection-plan-title">1. 계획을 만들고 검토합니다</h3><p class="meta">계획의 검사 항목과 HEAD를 읽은 뒤 검토·승인해야 실행할 수 있습니다.</p></div>${plans.length ? `<span class="meta">계획 ${escapeHTML(formatCount(plans.length))}개</span>` : ""}</header>${qualityInspectionAIControlHTML(workflow)}${qualityInspectionPlanHTML(selected, workflow)}</section><section aria-labelledby="quality-inspection-score-title"><header class="section-heading"><div><h3 id="quality-inspection-score-title">2. 결과와 점수를 확인합니다</h3><p class="meta">실행 결과를 통합한 결정적 점수입니다. AI가 점수나 실행을 결정하지 않습니다.</p></div></header>${qualityInspectionScoreHTML(score, workflow)}${run ? `<div class="quality-inspection-run-result"><strong>최근 실행 결과</strong>${run.results?.length ? `<ul>${run.results.map(result => `<li><span>${escapeHTML(qualityInspectionCheckLabels[result.checkId] || "검사 항목")}</span>${stateText(qualityInspectionOutcomeLabels[result.outcome] || "판정 보류", qualityInspectionOutcomeTone(result.outcome))}</li>`).join("")}</ul>` : "<p class=\"meta\">실행 결과 항목이 없습니다.</p>"}${qualityInspectionLastRunArtifactID(run) ? '<button class="button small" type="button" data-quality-inspection-proposal="generate">최근 결과로 개선 제안 만들기</button>' : ""}</div>` : ""}</section></div><section class="quality-inspection-section" aria-labelledby="quality-inspection-proposal-title"><header class="section-heading"><div><h3 id="quality-inspection-proposal-title">3. 개선 제안을 검토하고 적용합니다</h3><p class="meta">최근 실행 결과에서 허용된 enum 검사 집합 변경만 제안합니다. 사람이 승인한 뒤에만 적용됩니다.</p></div></header>${qualityInspectionProposalHTML(workflow, run, selected)}</section><section class="quality-inspection-section" aria-labelledby="quality-inspection-compare-title"><header class="section-heading"><div><h3 id="quality-inspection-compare-title">4. 전후를 비교합니다</h3><p class="meta">같은 프로젝트·저장소·Worktree의 점수만 비교하고, 조건이 맞지 않으면 비교 불가로 표시합니다.</p></div></header>${qualityInspectionComparisonHTML(workflow)}</section>${qualityInspectionToolPreviewHTML(workflow)}${workflow.status === "ready" && workflow.error ? `<p class="quality-inspection-note" role="status">일부 기록만 새로 읽었습니다. ${escapeHTML(workflow.error)}</p>` : ""}`;
  }

  function renderQualityWorkSurface() {
    const section = document.getElementById("quality-work-surface");
    if (!section) return;
    const targetSelect = document.getElementById("quality-target");
    const targetMeta = document.getElementById("quality-target-meta");
    const setupContainer = document.getElementById("quality-setup");
    const techniqueContainer = document.getElementById("quality-techniques");
    const resultsContainer = document.getElementById("quality-run-results");
    if (!targetSelect || !targetMeta || !setupContainer || !techniqueContainer || !resultsContainer) return;
    const targets = targetOptions();
    if (!state.selectedTargetValue || !targets.some(target => target.value === state.selectedTargetValue)) state.selectedTargetValue = targets[0]?.value || "";
    targetSelect.innerHTML = targets.length
      ? targets.map(target => `<option value="${escapeHTML(target.value)}">${escapeHTML(target.label)}</option>`).join("")
      : '<option value="">등록된 저장소가 없습니다</option>';
    targetSelect.value = state.selectedTargetValue;
    targetSelect.disabled = !targets.length || Boolean(state.qualityRunPending);
    const target = selectedTarget();
    const offline = state.service.status === "offline" || state.service.status === "error" || state.service.status === "loading";
    if (!target) {
      const hasProjects = (state.snapshot.projects || []).length > 0;
      targetMeta.innerHTML = hasProjects
        ? '<div class="empty-state"><strong>관찰된 Worktree가 없습니다.</strong><span>등록된 프로젝트의 저장소 상태를 먼저 새로 고치세요.</span><button class="button small" type="button" data-repository-refresh>저장소 상태 새로 고침</button></div>'
        : '<div class="empty-state"><strong>검사할 저장소를 등록하세요.</strong><span>프로젝트를 등록하면 대상별 코드 검사를 시작할 수 있습니다.</span><a class="button small" href="#projects">프로젝트 등록</a></div>';
      setupContainer.innerHTML = "";
      techniqueContainer.innerHTML = "";
      resultsContainer.innerHTML = "";
      const resultsSection = resultsContainer.closest(".quality-run-results");
      if (resultsSection) resultsSection.hidden = true;
      return;
    }
    rememberTarget(target.value);
    const runs = runsForTarget(target);
    const latestByTechnique = new Map();
    runs.forEach(item => { if (!latestByTechnique.has(item.spec?.technique)) latestByTechnique.set(item.spec?.technique, item); });
    const campaign = campaignForTarget(target);
    targetMeta.innerHTML = `<div class="quality-target-card"><div><span class="eyebrow">선택한 저장소</span><h3>${escapeHTML(target.repositoryName || target.projectName || "저장소")}</h3><p class="meta">${escapeHTML(target.branch || "브랜치 미상")}</p></div>${qualityTargetTechnicalDetails(target)}</div>`;
    const setup = state.qualitySetup.targetValue === target.value ? state.qualitySetup : { status: "idle", targetValue: target.value, requestKey: "", data: null, error: "", preferenceReset: false };
    renderQualitySetup(target, setup);
    const setupData = setup.status === "ready" ? setup.data : null;
    const setupPreference = setupData ? qualitySetupPreferenceForTarget(target, setupData) : { mode: "auto", languages: [] };
    const goRunnable = Boolean(setupData && qualitySetupHasRunnableGo(setupData, setupPreference));
    techniqueContainer.innerHTML = goRunnable
      ? `<section class="quality-go-actions" aria-labelledby="quality-go-actions-title"><header class="section-heading"><div><span class="eyebrow">실행 준비</span><h3 id="quality-go-actions-title">발견된 Go 검사</h3><p class="meta">Go가 실제 근거에서 발견된 경우에만 기존 실행 버튼을 표시합니다. 구성 확인과 실행 결과는 서로 다른 사실입니다.</p></div></header>${primaryQualityTechniques.map(technique => {
        const latest = latestByTechnique.get(technique.id);
        const pending = state.qualityRunPending === technique.id && state.qualityRunTargetValue === target.value;
        const disabled = offline || Boolean(state.qualityRunPending) || !campaign && state.qualityCampaignsStatus !== "ready";
        const internal = `<details class="quality-technique-details"><summary>기술 정보</summary><p><code>${escapeHTML(technique.id)}</code> · 서버가 대상 상태를 다시 확인합니다.</p></details>`;
        return `<article class="quality-technique-card"><div class="quality-technique-card__heading"><div><span class="eyebrow">Go 실행</span><h3>${escapeHTML(technique.label)}</h3></div>${latest ? stateText(qualityRunStatusText(latest), latest.spec?.state === "succeeded" ? "positive" : latest.spec?.state === "failed" ? "negative" : "attention") : stateText("아직 실행하지 않음", "neutral")}</div><p>${escapeHTML(technique.description)}</p><code class="quality-command">${escapeHTML(technique.command)}</code>${internal}<button class="button primary" type="button" data-quality-run="${escapeHTML(technique.id)}" ${disabled ? "disabled" : ""}>${pending ? "실행 중…" : "이 검사 실행"}</button></article>`;
      }).join("")}</section>`
      : "";
    const warning = state.qualityRunError && state.qualityRunErrorTargetValue === target.value ? `<p class="quality-run-error" role="status">${escapeHTML(state.qualityRunError)}</p>` : "";
    const resultsSection = resultsContainer.closest(".quality-run-results");
    if (resultsSection) resultsSection.hidden = !runs.length && !warning;
    resultsContainer.innerHTML = runs.length ? `${warning}${runs.slice(0, 6).map(qualityRunInlineResult).join("")}` : warning;
  }

  function renderAssuranceBenefits() {
    const container = document.getElementById("assurance-benefits");
    if (!container) return;
    container.innerHTML = [
      ["재실행", "같은 기준으로 다시 확인합니다."],
      ["근거 연결", "실행·결과·효과를 연결합니다."],
      ["비용 경계", "모르는 사용량은 비용으로 추정하지 않습니다."],
      ["대화 경계", "원문은 화면에 표시하지 않습니다."],
    ].map(([title, text]) => `<article><strong>${escapeHTML(title)}</strong><span>${escapeHTML(text)}</span></article>`).join("");
  }

  function renderAssuranceRun(item) {
    const spec = item.spec || {};
    const evidenceKeys = Object.keys(spec.evidence || {});
    const command = [spec.command?.executable, ...(spec.command?.arguments || [])].filter(Boolean).join(" ");
    const domainState = String(spec.state || "");
    const tone = assuranceTone(domainState);
    const scope = assuranceScope(spec);
    const coverage = spec.coverage
      ? `<p class="assurance-run-coverage"><span>Go coverage</span>${escapeHTML(formatImpactValue(spec.coverage.percent, "percent"))} · ${escapeHTML(formatCount(spec.coverage.coveredStatements))} / ${escapeHTML(formatCount(spec.coverage.totalStatements))} statements</p>`
      : "";
    const coverageDetail = spec.coverage
      ? `<div class="wide"><dt>Go coverage 근거</dt><dd>${escapeHTML(formatImpactValue(spec.coverage.percent, "percent"))} · ${escapeHTML(formatCount(spec.coverage.fileCount))}개 파일 · ${escapeHTML(spec.coverage.mode || "mode 미상")} · profile artifact <code>${escapeHTML(spec.coverage.profileArtifactId || "기록 없음")}</code></dd></div>`
      : "";
    const runID = item.metadata?.id || "";
    return `<article id="assurance-run-${escapeHTML(runID)}" class="ledger-row assurance-record ${rowToneClass(tone)}" data-tone="${escapeHTML(tone)}" data-state="${escapeHTML(domainState)}" tabindex="-1">
      <div class="ledger-row__state">${stateText(label(domainState), tone)}</div>
      <div class="ledger-row__main"><h3>${escapeHTML(assuranceTechniqueLabels[spec.technique] || spec.technique || "Quality Run")}</h3>
      <p>${escapeHTML(spec.summary || "결과 요약이 없습니다.")}</p>${coverage}
      <details><summary>실행 기준과 근거 보기</summary><dl class="detail-grid">
        <div><dt>Runner</dt><dd>${escapeHTML(spec.runner || "알 수 없음")}</dd></div>
        <div><dt>종료 코드</dt><dd>${escapeHTML(spec.exitCode ?? "기록 없음")}</dd></div>
        <div><dt>HEAD</dt><dd><code>${escapeHTML(spec.head || "기록 없음")}</code></dd></div>
        <div><dt>근거 항목</dt><dd>${escapeHTML(evidenceKeys.length ? `${evidenceKeys.length}개` : "없음")}</dd></div>
        <div><dt>artifact</dt><dd>${escapeHTML((spec.artifactIds || []).length ? `${spec.artifactIds.length}개` : "없음")}</dd></div>
        <div><dt>Agent 실행</dt><dd>${escapeHTML((spec.invocationIds || []).length ? `${spec.invocationIds.length}개` : "없음")}</dd></div>
        ${coverageDetail}
        <div class="wide"><dt>검증 명령</dt><dd><code>${escapeHTML(command || "기록 없음")}</code></dd></div>
        <div class="wide"><dt>Config digest</dt><dd><code>${escapeHTML(spec.configDigest || "기록 없음")}</code></dd></div>
      </dl></details></div>
      <div class="ledger-row__context"><span>${escapeHTML(scope)}</span><span>${escapeHTML(formatDate(spec.startedAt))}</span></div>
      <div class="ledger-row__action"></div>
    </article>`;
  }

  function renderAssuranceEffect(item) {
    const spec = item.spec || {};
    const classification = assuranceEffectClassification(spec);
    const outcome = spec.outcome || (["measured", "prevented_regression"].includes(classification) ? classification : "effect");
    const status = spec.reverified
      ? (spec.reverificationRunId && spec.reverifiedCommit ? "재검증 기록" : "재검증 정보 부족")
      : spec.adopted
        ? (spec.adoptedCommit ? "채택 기록" : "채택 정보 부족")
        : classification === "unavailable" ? "확인 불가" : "검토 대기";
    const value = spec.observation?.value ?? spec.value;
    const valueKnown = spec.valueKnown === undefined ? value !== null && value !== undefined : spec.valueKnown;
    const valueText = !valueKnown || value === null || value === undefined ? "" : ` · ${escapeHTML(formatImpactValue(value, spec.observation?.unit || spec.unit || ""))}`;
    const baselineText = spec.baselineValue === null || spec.baselineValue === undefined ? "확인 불가" : formatImpactValue(spec.baselineValue, spec.baselineUnit || spec.unit || "");
    const id = artifactID(item);
    const traceButton = id ? `<button class="button small" type="button" data-assurance-trace="${escapeHTML(id)}">근거 흐름</button>` : "";
    const tone = assuranceTone(classification);
    const scope = assuranceScope(spec);
    return `<article class="ledger-row assurance-record ${rowToneClass(tone)}" data-tone="${escapeHTML(tone)}" data-state="${escapeHTML(classification)}" data-outcome="${escapeHTML(outcome)}"><div class="ledger-row__state">${stateText(assuranceImpactStateLabels[classification] || classification, tone)}</div><div class="ledger-row__main"><h3>${escapeHTML(spec.label || "효과 기록")}</h3><p>${escapeHTML(assuranceEffectLabels[outcome] || outcome || "효과")}${valueText}</p><details><summary>효과 근거 보기</summary><dl class="detail-grid"><div><dt>분류</dt><dd>${escapeHTML(assuranceImpactStateLabels[classification] || classification)}</dd></div><div><dt>지표</dt><dd><code>${escapeHTML(spec.metricKey || "지정 없음")}</code></dd></div><div><dt>결과</dt><dd>${escapeHTML(assuranceEffectLabels[outcome] || outcome || "확인 불가")}</dd></div><div><dt>기준선</dt><dd>${escapeHTML(baselineText)}</dd></div><div><dt>관측값</dt><dd>${escapeHTML(valueKnown ? formatImpactValue(value, spec.unit || "") : "확인 불가")}</dd></div><div><dt>범위</dt><dd>${escapeHTML(scope)}</dd></div><div><dt>생성 시각</dt><dd>${escapeHTML(formatDate(spec.createdAt))}</dd></div><div><dt>채택 시각</dt><dd>${escapeHTML(formatDate(spec.adoptedAt))}</dd></div><div><dt>재검증 시각</dt><dd>${escapeHTML(formatDate(spec.reverifiedAt))}</dd></div><div><dt>원본 Quality Run</dt><dd><code>${escapeHTML(spec.sourceRunId || "연결 없음")}</code></dd></div><div><dt>증거 artifact</dt><dd>${escapeHTML((spec.evidenceIds || []).length ? `${spec.evidenceIds.length}개` : "없음")}</dd></div><div><dt>Trace ID</dt><dd><code>${escapeHTML(spec.traceId || id || "연결 없음")}</code></dd></div><div><dt>채택 커밋</dt><dd><code>${escapeHTML(spec.adoptedCommit || "연결 없음")}</code></dd></div><div><dt>재검증 run</dt><dd><code>${escapeHTML(spec.reverificationRunId || (spec.reverified ? "완료" : "없음"))}</code></dd></div><div><dt>재검증 commit</dt><dd><code>${escapeHTML(spec.reverifiedCommit || "연결 없음")}</code></dd></div><div class="wide"><dt>Fingerprint</dt><dd><code>${escapeHTML(spec.fingerprint || "기록 없음")}</code></dd></div></dl></details></div><div class="ledger-row__context"><span>${escapeHTML(status)}</span><span>${escapeHTML(scope)}</span></div><div class="ledger-row__action">${traceButton}</div></article>`;
  }

  function renderAssuranceInvocation(item) {
    const spec = item.spec || {};
    const usage = spec.usage || {};
    const model = spec.requestedModel || spec.resolvedModel || "모델 미상";
    const domainState = String(spec.state || "");
    const tone = assuranceTone(domainState);
    const id = item.metadata?.id || "";
    const retryable = ["failed", "interrupted"].includes(spec.state);
    const retryForm = id && retryable
      ? `<details class="invocation-retry"><summary>실패/중단 실행 재시도</summary><p class="meta">원래 prompt는 저장하지 않습니다. 새 prompt를 입력합니다.</p><form data-assurance-retry="${escapeHTML(id)}"><label><span>새 prompt</span><input name="prompt" maxlength="2000" autocomplete="off" required></label><div class="item-actions"><button class="button small primary" type="submit">재시도</button></div></form></details>`
      : "";
    const parent = spec.parentId ? `<div><dt>원본 실행</dt><dd><code>${escapeHTML(spec.parentId)}</code></dd></div>` : "";
    return `<article class="ledger-row assurance-record ${rowToneClass(tone)}" data-tone="${escapeHTML(tone)}" data-state="${escapeHTML(domainState)}"><div class="ledger-row__state">${stateText(label(domainState), tone)}</div><div class="ledger-row__main"><h3>${escapeHTML(spec.provider || "Provider 미상")}</h3><p>${escapeHTML(model)} · ${escapeHTML(formatDate(spec.startedAt))}</p><dl class="detail-grid"><div><dt>실행 ID</dt><dd><code>${escapeHTML(id || "기록 없음")}</code></dd></div>${parent}<div><dt>입력 토큰</dt><dd>${escapeHTML(formatOptionalCount(usage.inputTokens))}</dd></div><div><dt>출력 토큰</dt><dd>${escapeHTML(formatOptionalCount(usage.outputTokens))}</dd></div><div><dt>전체 토큰</dt><dd>${escapeHTML(formatOptionalCount(usage.totalTokens))}</dd></div><div><dt>원문 상태</dt><dd>${spec.rawTranscript ? "정책 확인 필요" : "수집하지 않음"}</dd></div>${spec.failureCode ? `<div class="wide"><dt>실패 코드</dt><dd><code>${escapeHTML(spec.failureCode)}</code></dd></div>` : ""}</dl><details><summary>선택 근거 보기</summary><dl class="detail-grid"><div><dt>요청 모델</dt><dd>${escapeHTML(spec.requestedModel || "없음")}</dd></div><div><dt>확정 모델</dt><dd>${escapeHTML(spec.resolvedModel || "없음")}</dd></div><div><dt>선택 출처</dt><dd>${escapeHTML(spec.selectionSource || "알 수 없음")}</dd></div><div><dt>artifact</dt><dd>${escapeHTML((spec.artifactIds || []).length ? `${spec.artifactIds.length}개` : "없음")}</dd></div></dl></details>${retryForm}</div><div class="ledger-row__context"><span>${escapeHTML(model)}</span><span>${escapeHTML(formatOptionalCount(usage.totalTokens))} 토큰</span></div><div class="ledger-row__action"></div></article>`;
  }

  function renderAssuranceArtifact(item) {
    const spec = item.spec || {};
    const retention = assuranceRetentionLabels[spec.retention] || spec.retention || "알 수 없음";
    const tone = assuranceTone(spec.retention);
    const id = artifactID(item);
    const name = item.metadata?.name || spec.name || spec.sourceType || item.name || "근거 artifact";
    const retentionAction = id && spec.retention === "pinned"
      ? `<button class="button small" type="button" data-assurance-artifact="retention" data-id="${escapeHTML(id)}" data-retention="active">고정 해제</button>`
      : id && spec.retention === "active"
        ? `<button class="button small" type="button" data-assurance-artifact="retention" data-id="${escapeHTML(id)}" data-retention="pinned">근거 고정</button>`
        : "";
    const restoreAction = id && spec.retention === "archived"
      ? `<button class="button small" type="button" data-assurance-artifact="restore" data-id="${escapeHTML(id)}">복원</button>`
      : "";
    return `<article class="ledger-row assurance-record ${rowToneClass(tone)}" data-tone="${escapeHTML(tone)}" data-state="${escapeHTML(spec.retention || "unknown")}"><div class="ledger-row__state">${stateText(retention, tone)}</div><div class="ledger-row__main"><h3>${escapeHTML(name)}</h3><p>출처 ${escapeHTML(spec.sourceId || "미상")} · Artifact ID <code>${escapeHTML(id || "기록 없음")}</code></p><details><summary>artifact manifest 보기</summary><dl class="detail-grid"><div><dt>크기</dt><dd>${escapeHTML(formatCount(spec.size))} bytes</dd></div><div><dt>MIME</dt><dd>${escapeHTML(spec.mime || "알 수 없음")}</dd></div><div><dt>보관 상태</dt><dd>${escapeHTML(retention)}</dd></div><div><dt>원본 참조</dt><dd>${escapeHTML(spec.sourceRef || "없음")}</dd></div><div class="wide"><dt>SHA-256</dt><dd><code>${escapeHTML(spec.sha256 || "기록 없음")}</code></dd></div></dl></details></div><div class="ledger-row__context">${escapeHTML(formatDate(spec.createdAt))}</div><div class="ledger-row__action"><div class="item-actions">${retentionAction}${restoreAction}</div></div></article>`;
  }

  function findImpactMetric(metrics, keys, labelText) {
    return metrics.find(metric => keys.some(key => String(metric.key || "").toLowerCase().includes(key)))
      || metrics.find(metric => String(metric.label || "").includes(labelText));
  }

  function impactMetricValue(metric) {
    return metric ? formatImpactValue(metric.value, metric.unit) : "확인 불가";
  }

  function renderHomeAssurance(dashboard, runs) {
    const impact = state.assuranceImpact;
    const metrics = impact?.metrics || [];
    const trace = impact?.traceability || {};
    const recordsTotal = Number(impact?.dataQuality?.recordsTotal) || 0;
    const effectsTotal = Number(trace.effectsTotal) || 0;
    const hasImpactRecords = recordsTotal > 0 || effectsTotal > 0;
    const verified = findImpactMetric(metrics, ["verified_effects"], "검증된 효과");
    const measured = metrics.find(metric => metric.key === "time_saved" && metric.value !== null && metric.value !== undefined);
    const estimated = metrics.find(metric => metric.key === "time_saved_estimated" && metric.value !== null && metric.value !== undefined);
    const duration = measured || estimated;
    const durationState = duration?.state || "unavailable";
    const durationTitle = measured ? "기록된 시간 절감" : estimated ? "예상 시간 절감" : "시간 절감";
    const traceValue = hasImpactRecords && effectsTotal
      ? `${formatCount(trace.completeEffects)} / ${formatCount(effectsTotal)}`
      : "확인 불가";
    const traceState = traceValue === "확인 불가" ? "" : Number(trace.completeEffects) === effectsTotal ? "ok" : "warn";
    const impactPeriod = Number(impact?.periodDays) || 30;
    const summary = `<div class="assurance-summary"><article><span>Quality Run</span><strong>${escapeHTML(formatCount(runs?.length || 0))}</strong></article><article><span>Agent 실행</span><strong>${escapeHTML(formatCount(dashboard?.invocations?.length || 0))}</strong></article><article><span>효과 기록</span><strong>${escapeHTML(formatCount(dashboard?.effects?.length || 0))}</strong></article><article><span>비용 상태</span><strong>${escapeHTML(dashboard?.costState === "estimated" ? "추정" : "미확인")}</strong></article></div>`;
    const proof = `<div class="home-assurance-proof" aria-label="효과 추적 요약"><article class="${escapeHTML(assuranceImpactStateClass(verified?.state || "unavailable"))}"><span>검증된 효과</span><strong>${escapeHTML(hasImpactRecords ? impactMetricValue(verified) : "확인 불가")}</strong><small>원본·artifact·재검증 연결</small></article><article class="${escapeHTML(traceState)}"><span>근거 완결성</span><strong>${escapeHTML(traceValue)}</strong><small>완결된 효과 / 전체 효과</small></article><article class="${escapeHTML(assuranceImpactStateClass(durationState))}"><span>${escapeHTML(durationTitle)}</span><strong>${escapeHTML(hasImpactRecords ? impactMetricValue(duration) : "확인 불가")}</strong><small>${escapeHTML(`최근 ${impactPeriod}일 · 추정값은 측정값과 분리`)}</small></article></div>`;
    return `${summary}${proof}<div class="home-assurance-footer"><p class="meta">검증된 효과는 같은 채택 HEAD에서 성공한 재검증까지 연결된 기록만 셉니다.</p><a class="button small" href="#assurance">효과 추적 보기</a></div>`;
  }

  function renderAssuranceImpactHeadline(impact) {
    const container = document.getElementById("assurance-impact-headline");
    if (!container) return;
    const metrics = impact?.metrics || [];
    const trace = impact?.traceability || {};
    const proofMetric = findImpactMetric(metrics, ["verified", "defect_fixed", "improvement"], "개선");
    const successMetric = findImpactMetric(metrics, ["reverification_rate"], "재검증률");
    const measuredDuration = metrics.find(metric => metric.key === "time_saved" && metric.value !== null && metric.value !== undefined);
    const estimatedDuration = metrics.find(metric => metric.key === "time_saved_estimated" && metric.value !== null && metric.value !== undefined);
    const durationMetric = measuredDuration || estimatedDuration;
    const durationTitle = measuredDuration ? "기록된 시간 절감" : estimatedDuration ? "예상 시간 절감" : "기록된 시간 절감";
    const completeness = trace.effectsTotal ? `${formatImpactValue((Number(trace.completeEffects || 0) / Number(trace.effectsTotal)) * 100, "percent")}` : "확인 불가";
    const cards = [
      ["검증된 개선", proofMetric, "현재 기간"],
      ["재검증률", successMetric, "현재 기간"],
      [durationTitle, durationMetric, "이전 기간 비교"],
      ["근거 완결성", { value: completeness, state: trace.status === "complete" ? "measured" : trace.effectsTotal ? "user_estimated" : "unavailable" }, "trace 연결 상태"],
    ];
    container.innerHTML = cards.map(([title, metric, hint]) => {
      const state = metric?.state || "unavailable";
      const comparison = metric?.comparison || {};
      return `<article class="metric-card impact-headline-card ${assuranceImpactStateClass(state)}"><span>${escapeHTML(title)}</span><strong>${escapeHTML(metric?.value === completeness ? completeness : impactMetricValue(metric))}</strong><small>${escapeHTML(metric?.comparison ? formatImpactDelta(comparison, metric.unit) : hint)}</small></article>`;
    }).join("");
  }

  function renderAssuranceImpactMetrics(impact) {
    const container = document.getElementById("assurance-impact-metrics");
    if (!container) return;
    if (!impact) {
      container.innerHTML = `<div class="empty-state"><strong>효과 지표를 표시할 수 없습니다.</strong><span>Impact API가 복구되면 자동으로 다시 불러옵니다.</span></div>`;
      return;
    }
    const metrics = impact.metrics || [];
    container.innerHTML = metrics.length ? metrics.map(metric => {
      const state = metric.state || "unavailable";
      const comparison = metric.comparison || {};
      const evidenceLabel = state === "user_estimated" ? "기록" : "근거";
      const evidence = metric.evidenceCount === null || metric.evidenceCount === undefined ? `${evidenceLabel} 수 미상` : `${evidenceLabel} ${formatCount(metric.evidenceCount)}개`;
      return `<article class="impact-metric-card ${assuranceImpactStateClass(state)}"><div class="list-item-header"><div><h3>${escapeHTML(metric.label || metric.key || "효과 지표")}</h3><p class="meta"><code>${escapeHTML(metric.key || "metric")}</code></p></div><span class="chip ${assuranceImpactStateClass(state)}">${escapeHTML(assuranceImpactStateLabels[state] || state)}</span></div><strong class="impact-metric-value">${escapeHTML(formatImpactValue(metric.value, metric.unit))}</strong><p class="impact-comparison">${escapeHTML(formatImpactDelta(comparison, metric.unit))}</p><p class="meta">${escapeHTML(metric.sampleCount ? `표본 ${formatCount(metric.sampleCount)}개 · ` : "")} ${escapeHTML(evidence)}</p></article>`;
    }).join("") : '<div class="empty-state"><strong>표시할 효과 지표가 없습니다.</strong><span>Quality Run과 효과 기록이 쌓이면 기간 비교를 시작합니다.</span></div>';
  }

  function renderAssuranceImpactTrend(impact) {
    const container = document.getElementById("assurance-impact-trend");
    if (!container) return;
    const trend = impact?.trend || [];
    const hasActivity = trend.some(item => [item.agentInvocations, item.qualityRuns, item.effects].some(value => Number(value) > 0));
    if (!trend.length || !hasActivity) {
      container.innerHTML = '<div class="empty-state"><strong>추세를 만들 표본이 없습니다.</strong><span>선택한 기간에 실행 기록이 쌓이면 흐름을 표시합니다.</span></div>';
      return;
    }
    const maximum = Math.max(1, ...trend.map(item => Math.max(Number(item.agentInvocations) || 0, Number(item.qualityRuns) || 0, Number(item.effects) || 0)));
    container.innerHTML = trend.map(item => {
      const volume = Math.max(Number(item.agentInvocations) || 0, Number(item.qualityRuns) || 0, Number(item.effects) || 0);
      return `<article class="trend-row"><div class="trend-label"><strong>${escapeHTML(formatDate(item.startAt))}</strong><span>${escapeHTML(formatDate(item.endAt))}</span></div><div class="trend-track" role="img" aria-label="${escapeHTML(`실행 ${item.agentInvocations || 0}건, Quality Run ${item.qualityRuns || 0}건, 효과 ${item.effects || 0}건`) }"><span class="trend-fill" style="width:${safePercentage(volume, maximum)}%"></span></div><p class="meta">Agent ${formatCount(item.agentInvocations)} · 성공 ${formatCount(item.successfulAgents)} · Quality Run ${formatCount(item.qualityRuns)} · 통과 ${formatCount(item.successfulRuns)} · 효과 ${formatCount(item.effects)} · 검증 ${formatCount(item.verifiedEffects)}</p></article>`;
    }).join("");
  }

  function renderAssuranceQuality(impact) {
    const container = document.getElementById("assurance-impact-quality");
    if (!container) return;
    const quality = impact?.dataQuality;
    if (!quality) {
      container.innerHTML = '<div class="empty-state"><strong>품질 정보를 표시할 수 없습니다.</strong><span>Impact API가 복구되면 데이터 경계를 확인합니다.</span></div>';
      return;
    }
    const rows = [["전체 기록", quality.recordsTotal], ["측정 효과", quality.measuredEffects], ["회귀 방지", quality.preventedRegressionEffects], ["사용자 추정", quality.userEstimatedEffects], ["AI 추론", quality.aiInferenceEffects], ["확인 불가", quality.unavailableEffects], ["근거 누락", quality.missingEvidence], ["artifact 누락", quality.missingArtifacts]];
    const baselineLabels = { previous_equal_period: "이전 동일 기간", unavailable: "확인 불가" };
    container.innerHTML = `<div class="quality-list">${rows.map(([name, value]) => `<div><span>${escapeHTML(name)}</span><strong>${escapeHTML(formatCount(value))}</strong></div>`).join("")}</div><p class="meta">비교 기준 ${escapeHTML(baselineLabels[quality.baselineState] || quality.baselineState || "확인 불가")} · 마지막 기록 ${escapeHTML(formatDate(quality.lastEvidenceAt))}</p>`;
  }

  function renderAssuranceTraceability(impact) {
    const container = document.getElementById("assurance-impact-traceability");
    if (!container) return;
    const trace = impact?.traceability;
    if (!trace) {
      container.innerHTML = '<div class="empty-state"><strong>근거 연결 정보를 표시할 수 없습니다.</strong><span>Impact API가 복구되면 trace 완결성을 확인합니다.</span></div>';
      return;
    }
    const status = trace.status || "unavailable";
    container.innerHTML = `<div class="trace-score"><strong>${escapeHTML(trace.effectsTotal ? `${formatCount(trace.completeEffects)} / ${formatCount(trace.effectsTotal)}` : "확인 불가")}</strong><span class="chip ${status === "complete" ? "ok" : status === "partial" ? "warn" : "bad"}">${escapeHTML(status === "complete" ? "완결" : status === "partial" ? "부분 연결" : "확인 필요")}</span></div><div class="quality-list"><div><span>부분 연결</span><strong>${escapeHTML(formatCount(trace.partialEffects))}</strong></div><div><span>미해결</span><strong>${escapeHTML(formatCount(trace.unresolvedEffects))}</strong></div><div><span>연결 artifact</span><strong>${escapeHTML(formatCount(trace.linkedArtifacts))}</strong></div><div><span>누락 artifact</span><strong>${escapeHTML(formatCount(trace.missingArtifacts))}</strong></div></div>`;
  }

  function renderAssuranceArtifactStorage() {
    const container = document.getElementById("assurance-artifact-storage");
    if (!container) return;
    const storage = state.assuranceStorage;
    if (!storage) {
      container.innerHTML = `<div class="empty-state"><strong>보관 용량을 표시할 수 없습니다.</strong><span>${escapeHTML(state.assuranceStorageError || "Storage API가 복구되면 보관 상태를 확인합니다.")}</span></div>`;
      return;
    }
    const storageNumber = (shortKey, longKey) => Number(storage[shortKey] ?? storage[longKey]) || 0;
    const quota = storageNumber("quota", "quotaBytes");
    const used = storageNumber("used", "usedBytes");
    const active = storageNumber("active", "activeBytes");
    const pinned = storageNumber("pinned", "pinnedBytes");
    const archived = storageNumber("archived", "archivedBytes");
    const percent = quota ? Math.min(100, Math.round((used / quota) * 100)) : 0;
    container.innerHTML = `<div class="storage-summary"><div class="storage-meter" role="img" aria-label="보관 용량 ${escapeHTML(formatCount(used))} / ${escapeHTML(formatCount(quota))} bytes"><span style="width:${percent}%"></span></div><strong>${escapeHTML(formatCount(used))} / ${escapeHTML(formatCount(quota))} bytes</strong><span class="meta">활성 ${escapeHTML(formatCount(active))} · 고정 ${escapeHTML(formatCount(pinned))} · 보관 ${escapeHTML(formatCount(archived))} · 누락 ${escapeHTML(formatCount(storage.missingCount))}</span></div>`;
  }

  function renderAssuranceTrace() {
    const panel = document.getElementById("assurance-trace-panel");
    const container = document.getElementById("assurance-trace");
    if (!panel || !container) return;
    panel.hidden = !state.assuranceTraceEffectID;
    if (!state.assuranceTraceEffectID) return;
    if (state.assuranceTraceError) {
      container.innerHTML = `<div class="empty-state state-bad"><strong>trace를 불러오지 못했습니다.</strong><span>${escapeHTML(state.assuranceTraceError)}</span></div>`;
      return;
    }
    const trace = state.assuranceTrace;
    if (!trace) {
      container.innerHTML = '<div class="loading">trace를 불러오는 중입니다.</div>';
      return;
    }
    const effect = trace.effect?.spec || trace.effect || {};
    const nodes = trace.nodes || [];
    const links = trace.links || [];
    const artifacts = trace.artifacts || [];
    container.innerHTML = `<div class="trace-overview"><div><strong>${escapeHTML(effect.label || "효과 기록")}</strong><p class="meta">Effect ID <code>${escapeHTML(state.assuranceTraceEffectID)}</code></p></div><span class="chip ${trace.complete ? "ok" : "warn"}">${trace.complete ? "근거 완결" : "부분 연결"}</span></div><div class="trace-flow"><div><p class="eyebrow">구성 요소</p>${nodes.length ? nodes.map(node => `<article class="trace-node"><strong>${escapeHTML(node.label || node.kind || "기록")}</strong><code>${escapeHTML(node.id || "알 수 없음")}</code><span>${escapeHTML(node.state || node.head || "")}</span></article>`).join("") : '<p class="meta">연결된 노드가 없습니다.</p>'}</div><div><p class="eyebrow">연결 관계</p>${links.length ? `<ul class="trace-links">${links.map(link => `<li><code>${escapeHTML(link.from || link.fromId || "?")}</code> → ${escapeHTML(link.relation || "연결")} → <code>${escapeHTML(link.to || link.toId || "?")}</code></li>`).join("")}</ul>` : '<p class="meta">연결 관계가 없습니다.</p>'}</div></div><div class="trace-evidence"><p class="eyebrow">근거 파일</p>${artifacts.length ? `<div class="item-list">${artifacts.map(artifact => `<article class="trace-artifact"><div class="list-item-header"><div><strong>${escapeHTML(artifact.name || "근거 artifact")}</strong><p class="meta">${escapeHTML(artifact.sourceType || "출처 미상")} · <code>${escapeHTML(artifact.id || "알 수 없음")}</code></p></div><span class="chip ${artifact.present ? "ok" : "bad"}">${artifact.present ? escapeHTML(artifact.retention || "확인됨") : "누락"}</span></div><p class="meta">SHA-256 <code>${escapeHTML(artifact.sha256 || "기록 없음")}</code> · ${escapeHTML(formatCount(artifact.size))} bytes</p></article>`).join("")}</div>` : '<p class="meta">연결된 artifact가 없습니다.</p>'}</div>${(trace.missingRefs || []).length ? `<p class="safety-note">누락된 참조 ${escapeHTML(formatCount(trace.missingRefs.length))}개가 있어 효과를 완결된 근거로 볼 수 없습니다.</p>` : ""}`;
  }

  function renderAssuranceDemo(show) {
    const view = document.querySelector('[data-view="assurance"]');
    const banner = document.getElementById("assurance-demo-banner");
    const board = document.getElementById("assurance-demo-board");
    const measurementPanel = document.getElementById("assurance-measurement-dashboard");
    const empty = document.getElementById("assurance-empty");
    const method = document.querySelector(".assurance-method");
    const impactError = document.getElementById("assurance-impact-error");
    const tracePanel = document.getElementById("assurance-trace-panel");
    if (!banner || !board) return;
    view?.classList.toggle("is-demo", show);
    banner.hidden = !show;
    board.hidden = !show;
    if (measurementPanel) measurementPanel.hidden = show;
    if (!show) {
      method && (method.hidden = false);
      tracePanel && (tracePanel.hidden = !state.assuranceTraceEffectID);
      return;
    }

    document.querySelectorAll("[data-assurance-populated]").forEach(element => { element.hidden = true; });
    if (empty) empty.hidden = true;
    if (method) method.hidden = true;
    if (impactError) impactError.hidden = true;
    if (tracePanel) tracePanel.hidden = true;

    const kpis = [
      ["통과한 검증", "6 / 7", "Quality Run", ""],
      ["재검증률", "86%", "6회 재실행 / 7회", "demo-kpi--accent"],
      ["기록된 시간 절감", "2시간 40분", "측정 1시간 20분 · 추정 1시간 20분", ""],
      ["근거 연결", "92%", "11 / 12 trace", "demo-kpi--warning"],
    ];
    const kpiContainer = document.getElementById("assurance-demo-kpis");
    if (kpiContainer) {
      kpiContainer.innerHTML = kpis.map(([labelText, value, note, tone]) => `<article class="demo-kpi ${tone}"><span>${escapeHTML(labelText)}</span><strong>${escapeHTML(value)}</strong><small>${escapeHTML(note)}</small></article>`).join("");
    }

    const runs = [
      ["ok", "통과", "PR CI 점검", "web-console · main · abc1234", "테스트 18개 · artifact 3개 · 근거 완결", "2026-08-28T14:20:00+09:00", "2026. 8. 28. 14:20"],
      ["ok", "통과", "배포 전 회귀 점검", "billing-api · release/0.13 · 7bd92e1", "회귀 시나리오 12개 · 재검증 연결", "2026-08-26T09:10:00+09:00", "2026. 8. 26. 09:10"],
      ["warn", "주의", "Worktree 정리 전 확인", "web-console · feature/cleanup", "변경 파일 4개 · 사람 확인 필요", "2026-08-24T16:42:00+09:00", "2026. 8. 24. 16:42"],
    ];
    const runContainer = document.getElementById("assurance-demo-runs");
    if (runContainer) {
      runContainer.innerHTML = runs.map(([tone, labelText, title, scope, note, datetime, date]) => `<article class="demo-run"><span class="demo-run__state ${tone}" aria-hidden="true"></span><div><strong>${escapeHTML(title)}</strong><p>${escapeHTML(scope)}</p><p>${escapeHTML(note)}</p></div><div><span class="chip ${tone}">${escapeHTML(labelText)}</span><time datetime="${escapeHTML(datetime)}">${escapeHTML(date)}</time></div></article>`).join("");
    }

    const trend = [
      ["08/04", "92%", "3회 검증", 55],
      ["08/11", "94%", "4회 검증", 68],
      ["08/18", "96%", "5회 검증", 80],
      ["08/25", "97%", "6회 검증", 92],
    ];
    const trendContainer = document.getElementById("assurance-demo-trend");
    if (trendContainer) {
      trendContainer.innerHTML = trend.map(([period, value, note, width]) => `<div class="demo-trend-row"><strong>${escapeHTML(period)}</strong><div class="demo-trend-track" role="img" aria-label="${escapeHTML(`${period} 근거 연결 ${value}`)}"><span class="demo-trend-fill" style="--demo-bar-width:${width}%"></span></div><span>${escapeHTML(value)} · ${escapeHTML(note)}</span></div>`).join("");
    }
  }

  const measurementValue = (value, unit) => value === null || value === undefined ? "확인 불가" : formatImpactValue(value, unit);
  const measurementSignedValue = (value, unit) => {
    if (value === null || value === undefined) return "확인 불가";
    const numeric = Number(value);
    return `${numeric > 0 ? "+" : ""}${formatImpactValue(numeric, unit)}`;
  };
  const measurementTone = status => status === "fail" ? "bad" : status === "pass" ? "ok" : "warn";
  const measurementMetricLabel = name => measurementMetricLabels[name] || name || "이름 없는 측정값";
  const measurementStateLabel = state => measurementComparisonStateLabels[state] || "상태 미상";
  const measurementStatusLabel = status => measurementStatusLabels[status] || "알 수 없음";
  const measurementProvenanceLabel = provenance => measurementProvenanceLabels[provenance] || "출처 미상";

  function renderMeasurementMetric(item, comparison) {
    const tone = measurementTone(item.status);
    const baseline = item.baseline === null || item.baseline === undefined ? "기록 없음" : measurementValue(item.baseline, item.unit);
    const delta = item.delta === null || item.delta === undefined ? "기록 없음" : measurementSignedValue(item.delta, item.unit);
    const requestCount = Number(item.requestCount || 0);
    const successCount = Number(item.successCount || 0);
    const failureCount = Number(item.failureCount || 0);
    const requestDetail = requestCount > 0
      ? `요청 ${formatCount(requestCount)}회 · 성공 ${formatCount(successCount)}회 · 실패 ${formatCount(failureCount)}회`
      : `표본 ${formatCount(item.sampleCount)}개`;
    const mixedProbeNote = successCount > 0 && failureCount > 0 ? " · 성공·실패 혼합으로 결론 불가" : "";
    const failureReasons = (item.failureReasons || []).map(reason => `<code translate="no">${escapeHTML(reason)}</code>`).join("<br>");
    const comparisonText = comparison?.state === "comparable"
      ? `이전 p50 ${measurementValue(comparison.previousP50, item.unit)} · Δ ${measurementSignedValue(comparison.deltaP50, item.unit)}`
      : `이전 비교 ${measurementStateLabel(comparison?.state || "unavailable")}`;
    return `<article class="measurement-metric measurement-metric--${tone}" data-measurement-id="${escapeHTML(item.id)}">
      <header class="measurement-metric-heading"><div><h4>${escapeHTML(measurementMetricLabel(item.name))}</h4><code translate="no">${escapeHTML(item.id)}</code></div><span class="chip ${tone}">${escapeHTML(measurementStatusLabel(item.status))}</span></header>
      <div class="measurement-values"><div><span>p50</span><strong>${escapeHTML(measurementValue(item.p50, item.unit))}</strong></div><div><span>p95</span><strong>${escapeHTML(measurementValue(item.p95, item.unit))}</strong></div></div>
      <p class="meta">${escapeHTML(requestDetail)}${escapeHTML(mixedProbeNote)} · ${escapeHTML(measurementProvenanceLabel(item.provenance))} · ${escapeHTML(item.unit || "단위 미상")}</p>
      <p class="measurement-comparison-note">${escapeHTML(comparisonText)}</p>
      <details><summary>측정 근거 보기</summary><dl class="detail-grid"><div><dt>최솟값 · 최댓값</dt><dd>${escapeHTML(measurementValue(item.min, item.unit))} · ${escapeHTML(measurementValue(item.max, item.unit))}</dd></div><div><dt>요청 결과</dt><dd>${escapeHTML(requestDetail)}</dd></div>${failureReasons ? `<div class="wide"><dt>실패 원인</dt><dd>${failureReasons}</dd></div>` : ""}<div><dt>manifest baseline</dt><dd>${escapeHTML(baseline)}</dd></div><div><dt>manifest delta</dt><dd>${escapeHTML(delta)}</dd></div><div><dt>command ID</dt><dd><code translate="no">${escapeHTML(item.commandId || "기록 없음")}</code></dd></div><div><dt>종료 코드</dt><dd>${escapeHTML(item.exitCode === null || item.exitCode === undefined ? "기록 없음" : item.exitCode)}</dd></div><div><dt>필수 여부</dt><dd>${item.required ? "필수" : "선택"}</dd></div></dl></details>
    </article>`;
  }

  function renderMeasurementGate(latest) {
    const container = document.getElementById("assurance-measurement-gate");
    if (!container) return;
    const failures = latest.requiredFailures || [];
    const status = latest.status || "unknown";
    const tone = measurementTone(status);
    const detail = failures.length
      ? `실패한 필수 검사 ${formatCount(failures.length)}개: ${failures.join(", ")}`
      : status === "pass"
        ? "기록된 필수 검사가 모두 통과했습니다. 선택 측정값은 별도로 확인합니다."
        : "필수 검사 게이트를 판정할 충분한 근거가 없습니다.";
    container.innerHTML = `<header class="measurement-gate-heading"><div><span class="eyebrow">Required gate</span><h3 id="assurance-measurement-gate-title">필수 검사 게이트</h3></div><span class="chip ${tone}">${escapeHTML(measurementStatusLabel(status))}</span></header><p>${escapeHTML(detail)}</p>${failures.length ? `<ul class="measurement-failure-list">${failures.map(id => `<li><code translate="no">${escapeHTML(id)}</code></li>`).join("")}</ul>` : ""}`;
  }

  function renderMeasurementHighlights(latest) {
    const container = document.getElementById("assurance-measurement-highlights");
    if (!container) return;
    const byName = new Map((latest.measurements || []).map(item => [item.name, item]));
    const definitions = [
      ["quality.go.coverage_percent", "Coverage", "p50"],
      ["quality.go.test", "Go test", "p50"],
      ["quality.go.test_race", "Go test · race", "p50"],
      ["quality.go.build", "Go build", "p50"],
      ["quality.go.vet", "Go vet", "p50"],
    ];
    container.innerHTML = definitions.map(([name, title, field]) => {
      const item = byName.get(name);
      const value = item ? measurementValue(item[field], item.unit) : "기록 없음";
      const note = item ? `${measurementStatusLabel(item.status)} · ${measurementProvenanceLabel(item.provenance)}` : "manifest에 없음";
      return `<article class="measurement-highlight"><span>${escapeHTML(title)}</span><strong>${escapeHTML(value)}</strong><small>${escapeHTML(note)}</small></article>`;
    }).join("");
  }

  function renderMeasurementComparisonDashboard(dashboard) {
    const container = document.getElementById("assurance-measurement-comparison");
    if (!container) return;
    const latest = dashboard.latest;
    const previous = dashboard.previousComparable;
    const state = dashboard.comparisonState || "unavailable";
    const comparisonRows = (dashboard.comparisons || []).map(item => {
      const p95 = item.currentP95 !== null || item.previousP95 !== null
        ? `<span class="measurement-comparison-p95">p95 현재 ${escapeHTML(measurementValue(item.currentP95, item.unit))} · 이전 ${escapeHTML(measurementValue(item.previousP95, item.unit))} · Δ ${escapeHTML(measurementSignedValue(item.deltaP95, item.unit))}</span>`
        : "";
      return `<div class="measurement-comparison-row"><div><strong>${escapeHTML(measurementMetricLabel(item.name))}</strong><code translate="no">${escapeHTML(item.id)}</code></div><span>${escapeHTML(item.state === "comparable" ? `p50 현재 ${measurementValue(item.currentP50, item.unit)} · 이전 ${measurementValue(item.previousP50, item.unit)} · Δ ${measurementSignedValue(item.deltaP50, item.unit)}` : `p50 비교 ${measurementStateLabel(item.state)}`)}</span>${p95}</div>`;
    }).join("");
    const previousDetail = previous
      ? `<p class="meta">이전 비교 run <code translate="no">${escapeHTML(previous.runId)}</code> · 종료 ${escapeHTML(formatDate(previous.endedAt))} · commit <code translate="no">${escapeHTML(previous.commit)}</code></p>`
      : `<p class="meta">${escapeHTML(state === "missing" ? "같은 commit·HEAD·구성 digest·플랫폼·도구 버전의 이전 실행이 없습니다." : state === "incomparable" ? "이전 근거가 있지만 변경 상태 또는 재현성 조건이 달라 비교할 수 없습니다." : "commit·구성·도구 버전이 완전히 확인된 이전 실행을 비교할 수 없습니다.")}</p>`;
    container.innerHTML = `<header class="section-heading"><div><h3 id="assurance-measurement-comparison-title">동일 조건 비교</h3><p class="meta">delta는 현재 p50 − 이전 p50이며, 비교 가능한 값만 계산합니다.</p></div><span class="chip ${state === "comparable" ? "ok" : "warn"}">${escapeHTML(measurementStateLabel(state))}</span></header>${latest ? `<p class="meta">현재 run <code translate="no">${escapeHTML(latest.runId)}</code> · 종료 ${escapeHTML(formatDate(latest.endedAt))}</p>` : ""}${previousDetail}${comparisonRows ? `<div class="measurement-comparison-list">${comparisonRows}</div>` : '<div class="empty-state"><span>비교할 측정값이 없습니다.</span></div>'}`;
  }

  function renderMeasurementActions(dashboard) {
    const container = document.getElementById("assurance-measurement-actions");
    if (!container) return;
    const actions = dashboard.nextActions || [];
    container.innerHTML = `<header class="section-heading"><div><h3 id="assurance-measurement-actions-title">근거에서 나온 다음 단계</h3><p class="meta">추정 점수나 원인 주장은 만들지 않습니다.</p></div></header>${actions.length ? `<ol class="measurement-action-list">${actions.map(item => `<li><span class="measurement-action-mark" aria-hidden="true">!</span><div><strong>${escapeHTML(item.label || "확인 필요")}</strong><p>${escapeHTML(item.reason || "추가 근거가 필요합니다.")}</p><code translate="no">${escapeHTML(item.code || "unknown")}</code></div></li>`).join("")}</ol>` : '<div class="empty-state"><strong>추가 조치가 기록되지 않았습니다.</strong><span>현재 manifest에서 결정 가능한 다음 단계가 없습니다.</span></div>'}`;
  }

  function renderMeasurementReproducibility(latest) {
    const container = document.getElementById("assurance-measurement-reproducibility");
    if (!container) return;
    const tools = Object.entries(latest.toolVersions || {}).sort(([left], [right]) => left.localeCompare(right)).map(([name, version]) => `<code translate="no">${escapeHTML(name)}=${escapeHTML(version)}</code>`).join("<br>");
    const toolPaths = Object.entries(latest.toolPaths || {}).sort(([left], [right]) => left.localeCompare(right)).map(([name, path]) => `<code translate="no">${escapeHTML(name)}=${escapeHTML(path)}</code>`).join("<br>");
    container.innerHTML = `<dl class="detail-grid"><div><dt>run ID</dt><dd><code translate="no">${escapeHTML(latest.runId)}</code></dd></div><div><dt>상태</dt><dd>${escapeHTML(measurementStatusLabel(latest.status))}</dd></div><div><dt>commit</dt><dd><code translate="no">${escapeHTML(latest.commit || "알 수 없음")}</code></dd></div><div><dt>HEAD</dt><dd><code translate="no">${escapeHTML(latest.head || "알 수 없음")}</code></dd></div><div><dt>변경 상태</dt><dd>${escapeHTML(latest.dirtyState || "알 수 없음")}</dd></div><div><dt>플랫폼</dt><dd>${escapeHTML([latest.os, latest.arch].filter(Boolean).join(" · ") || "알 수 없음")}</dd></div><div class="wide"><dt>구성 digest</dt><dd><code translate="no">${escapeHTML(latest.configurationDigest || "기록 없음")}</code></dd></div><div class="wide"><dt>도구 버전</dt><dd>${tools || "기록 없음"}</dd></div><div class="wide"><dt>실행 도구 경로</dt><dd>${toolPaths || "기록 없음"}</dd></div><div><dt>시작</dt><dd>${escapeHTML(formatDate(latest.startedAt))}</dd></div><div><dt>종료</dt><dd>${escapeHTML(formatDate(latest.endedAt))}</dd></div><div class="wide"><dt>보고서·근거 식별자</dt><dd>v1 manifest에 별도 기록 없음 · command ID는 각 측정값에 표시</dd></div></dl>`;
  }

  function renderAssuranceMeasurementDashboard() {
    const panel = document.getElementById("assurance-measurement-dashboard");
    const status = document.getElementById("assurance-measurement-status");
    const empty = document.getElementById("assurance-measurement-empty");
    const content = document.getElementById("assurance-measurement-content");
    if (!panel || !status || !empty || !content) return;
    panel.hidden = false;
    const measurementState = state.assuranceMeasurement || { status: "loading", data: null, error: "" };
    const dashboard = measurementState.data || {};
    const latest = dashboard.latest;
    status.hidden = false;
    status.className = "measurement-status";
    if (measurementState.status === "loading") {
      status.textContent = "측정 manifest를 불러오는 중입니다…";
    } else if (measurementState.status === "importing") {
      status.textContent = "측정 manifest를 가져오는 중입니다…";
    } else if (measurementState.status === "error") {
      status.classList.add("measurement-status--error");
      status.textContent = `측정 manifest를 불러오지 못했습니다. ${measurementState.error || "잠시 후 다시 시도하세요."}`;
    } else if (latest) {
      status.classList.add("measurement-status--ready");
      status.textContent = `최근 측정 run ${latest.runId} · ${formatDate(latest.endedAt)}`;
    } else {
      status.classList.add("measurement-status--empty");
      status.textContent = "아직 가져온 측정 manifest가 없습니다.";
    }
    empty.hidden = Boolean(latest) || measurementState.status === "loading" || measurementState.status === "importing" || measurementState.status === "error";
    content.hidden = !latest;
    if (!latest) return;
    renderMeasurementGate(latest);
    renderMeasurementHighlights(latest);
    const comparisons = new Map((dashboard.comparisons || []).map(item => [item.id, item]));
    document.getElementById("assurance-measurement-metrics").innerHTML = (latest.measurements || []).length
      ? latest.measurements.map(item => renderMeasurementMetric(item, comparisons.get(item.id))).join("")
      : '<div class="empty-state"><strong>측정값이 없습니다.</strong><span>manifest의 measurements 배열이 비어 있습니다.</span></div>';
    renderMeasurementComparisonDashboard(dashboard);
    renderMeasurementActions(dashboard);
    renderMeasurementReproducibility(latest);
  }

  function renderAssuranceDashboard() {
    if (isAssuranceDemoRoute()) {
      renderAssuranceDemo(true);
      return;
    }
    renderAssuranceDemo(false);
    renderQualityInspectionWorkflow();
    renderAssuranceMeasurementDashboard();
    const dashboard = state.assuranceDashboard || {};
    const runs = state.assuranceRuns || [];
    const invocations = dashboard.invocations || [];
    const allEffects = state.assuranceEffects?.length ? state.assuranceEffects : dashboard.effects || [];
    const effects = allEffects.filter(item => state.assuranceEffectFilter === "all" || assuranceEffectClassification(item.spec || {}) === state.assuranceEffectFilter);
    const artifacts = state.assuranceArtifacts || [];
    const passed = runs.filter(item => ["succeeded", "passed"].includes(item.spec?.state)).length;
    const estimatedCost = dashboard.costState === "estimated" && dashboard.estimatedCost !== null && dashboard.estimatedCost !== undefined
      ? `${Number(dashboard.estimatedCost).toFixed(4)} · 추정`
      : "미확인";
    const usageValue = dashboard.usageComplete === false && !Number(dashboard.totalTokens)
      ? "미상 · 일부"
      : `${formatCount(dashboard.totalTokens)}${dashboard.usageComplete === false ? " · 일부" : ""}`;
    const metrics = [
      ["Quality Run", formatCount(runs.length)],
      ["통과", formatCount(passed)],
      ["Agent 실행", formatCount(invocations.length)],
      ["효과 기록", formatCount(effects.length)],
      ["사용량", usageValue],
      ["비용 상태", estimatedCost],
    ];
    const metricsContainer = document.getElementById("assurance-metrics");
    if (!metricsContainer) return;
    const impactRecords = Number(state.assuranceImpact?.dataQuality?.recordsTotal) || 0;
    const hasMeasurementEvidence = Boolean(state.assuranceMeasurement?.data?.latest);
    const hasEvidence = Boolean(runs.length || invocations.length || allEffects.length || artifacts.length || impactRecords || hasMeasurementEvidence);
    document.querySelectorAll("[data-assurance-populated]").forEach(element => { element.hidden = !hasEvidence; });
    const empty = document.getElementById("assurance-empty");
    if (empty) empty.hidden = hasEvidence;
    const impactError = document.getElementById("assurance-impact-error");
    if (impactError) {
      impactError.hidden = !state.assuranceImpactError;
      impactError.innerHTML = state.assuranceImpactError
        ? `<strong>효과 데이터를 불러오지 못했습니다.</strong><span>${escapeHTML(state.assuranceImpactError)}</span><button class="button small" type="button" data-assurance-retry>다시 시도</button>`
        : "";
    }
    renderAssuranceImpactHeadline(state.assuranceImpact);
    renderAssuranceImpactMetrics(state.assuranceImpact);
    renderAssuranceImpactTrend(state.assuranceImpact);
    renderAssuranceQuality(state.assuranceImpact);
    renderAssuranceTraceability(state.assuranceImpact);
    renderAssuranceArtifactStorage();
    renderAssuranceTrace();
    metricsContainer.innerHTML = metrics.map(([title, value]) => `<article class="metric-card"><span>${escapeHTML(title)}</span><strong>${escapeHTML(value)}</strong></article>`).join("");
    renderAssuranceFilters();
    renderAssuranceBenefits();
    document.getElementById("assurance-filter-note").textContent = state.assuranceFilters.provider || state.assuranceFilters.model
      ? `Agent 실행 ${formatCount(invocations.length)}건을 표시합니다. 효과 headline은 선택한 기간·범위의 Impact API를 사용합니다.`
      : `최근 ${escapeHTML(state.assuranceFilters.days)}일 효과를 표시합니다. 측정값·추정값·확인 불가를 구분합니다.`;
    document.getElementById("assurance-runs").innerHTML = runs.length ? `<div class="item-list">${runs.map(renderAssuranceRun).join("")}</div>` : '<div class="empty-state"><strong>아직 Quality Run이 없습니다.</strong><span>검증 기준을 실행하면 반복 가능한 결과와 근거가 남습니다.</span></div>';
    document.getElementById("assurance-effects").innerHTML = effects.length ? `<div class="item-list">${effects.map(renderAssuranceEffect).join("")}</div>` : `<div class="empty-state"><strong>${allEffects.length ? "선택한 상태의 효과가 없습니다." : "아직 효과 기록이 없습니다."}</strong><span>검증 결과를 실제 변화와 연결하면 여기에 남습니다.</span></div>`;
    document.getElementById("assurance-invocations").innerHTML = invocations.length ? `<div class="item-list">${invocations.map(renderAssuranceInvocation).join("")}</div>` : '<div class="empty-state"><strong>아직 Agent 실행이 없습니다.</strong><span>Provider 실행 후 모델·사용량·상태를 확인합니다.</span></div>';
    document.getElementById("assurance-artifacts").innerHTML = artifacts.length ? `<div class="item-list">${artifacts.map(renderAssuranceArtifact).join("")}</div>` : '<div class="empty-state"><strong>아직 보관된 근거가 없습니다.</strong><span>검증 결과의 manifest가 생성되면 보관 상태를 확인합니다.</span></div>';
    const focusRunID = activeRoute === "assurance" ? routeState().runID : "";
    const focusRun = focusRunID ? document.getElementById(`assurance-run-${encode(focusRunID)}`) : null;
    if (focusRun && state.assuranceFocusConsumed !== focusRunID) {
      state.assuranceFocusConsumed = focusRunID;
      window.setTimeout(() => { focusRun.scrollIntoView({ block: "start" }); focusRun.focus({ preventScroll: true }); }, 0);
    }
  }

  function renderProviderStatuses(containerID, compact = false) {
    const container = document.getElementById(containerID);
    if (!container) return;
    const providers = [...new Map((state.providerStatuses || []).map(item => [item.provider, item])).values()];
    container.innerHTML = providers.length
      ? `<div class="provider-list">${providers.map(item => {
        const tone = providerTone(item.state);
        const diagnostics = !compact && (item.detail || item.reasonCode || item.resolvedCommand?.length)
          ? `<details><summary>진단 세부 정보</summary><dl class="detail-grid"><div class="wide"><dt>추가 진단</dt><dd>${escapeHTML(providerDiagnostic(item))}</dd></div>${item.reasonCode ? `<div><dt>진단 코드</dt><dd><code>${escapeHTML(item.reasonCode)}</code></dd></div>` : ""}${item.resolvedCommand?.length ? `<div class="wide"><dt>확인된 실행 경로</dt><dd><code>${escapeHTML(item.resolvedCommand.join(" "))}</code></dd></div>` : ""}</dl></details>`
          : "";
        const recovery = item.state !== "ready" ? `<div class="item-actions"><a class="button small" data-provider-recovery data-focus-target="provider-statuses" href="#diagnostics" aria-label="${escapeHTML(item.provider)} 진단">진단</a></div>` : "";
        return `<article class="ledger-row provider-card ${rowToneClass(tone)}" data-provider-capability="${escapeHTML(item.provider)}" data-tone="${escapeHTML(tone)}" data-state="${escapeHTML(item.state || "unknown")}"><div class="ledger-row__state">${stateText(providerLabel(item.state), tone)}</div><div class="ledger-row__main"><h3>${escapeHTML(item.provider)}</h3><p>${escapeHTML(providerSummary(item.state))}</p>${diagnostics}</div><div class="ledger-row__context" aria-hidden="true"></div><div class="ledger-row__action">${recovery}</div></article>`;
      }).join("")}</div>`
      : '<div class="empty-state"><strong>Provider 상태가 없습니다.</strong><span>진단을 실행하면 선택 가능한 Provider를 확인합니다.</span></div>';
  }

  function projectStatus(repository) {
    if (repository.error || repository.unsafe_cleanup) return { tone: "negative", text: "확인 필요" };
    if (repository.dirty || repository.behind) return { tone: "attention", text: "주의" };
    return { tone: "positive", text: "정상" };
  }

  function renderProjects() {
    const projects = state.snapshot.projects || [];
    if (!state.activeProjectID || !projects.some(project => project.id === state.activeProjectID)) {
      state.activeProjectID = projects[0]?.id || "";
    }
    const findings = openFindings();
    document.getElementById("project-list").innerHTML = projects.length
      ? `<div class="project-list">${projects.map(project => {
        const count = findings.filter(item => item.spec.projectId === project.id).length;
        const selected = project.id === state.activeProjectID;
        const tone = count ? "attention" : "positive";
        return `<button class="ledger-row project-card ${rowToneClass(tone)} ${selected ? "selected" : ""}" type="button" data-project="${escapeHTML(project.id)}" data-tone="${escapeHTML(tone)}" data-finding-count="${escapeHTML(count)}" aria-pressed="${selected}"><span class="ledger-row__state">${stateText(count ? "확인 필요" : "정상", tone)}</span><span class="ledger-row__main"><strong>${escapeHTML(project.name)}</strong><span>${escapeHTML(project.repos.length)}개 저장소 · 확인 항목 ${escapeHTML(count)}개</span></span><span class="ledger-row__context"><code>${escapeHTML(project.id)}</code></span><span class="ledger-row__action">열기</span></button>`;
      }).join("")}</div>`
      : '<div class="empty-state"><strong>등록된 프로젝트가 없습니다.</strong><span>‘프로젝트 등록’에서 첫 범위를 추가하세요.</span></div>';

    const project = projects.find(item => item.id === state.activeProjectID);
    const detail = document.getElementById("project-detail");
    if (!project) {
      detail.className = "empty-state";
      detail.innerHTML = '<strong>프로젝트를 선택하세요.</strong><span>저장소와 Worktree 상태, 등록 관리 기능을 확인할 수 있습니다.</span>';
      return;
    }
    detail.className = "";
    const registered = registryProject(project.id);
    const projectName = registered?.metadata.name || project.name;
    const projectFindings = state.findings.filter(item => item.spec.projectId === project.id);
    const visibleFindings = projectFindings.filter(item =>
      (!state.findingFilters.severity || item.spec.severity === state.findingFilters.severity) &&
      (!state.findingFilters.state || (state.findingFilters.state === "active" ? ["open", "acknowledged"].includes(item.spec.state) : item.spec.state === state.findingFilters.state)));
    detail.innerHTML = `<div class="project-detail-header"><div><p class="eyebrow">${escapeHTML(project.id)}</p><h2>${escapeHTML(projectName)}</h2><p class="meta">마지막 관찰 ${escapeHTML(formatDate(project.scanned_at))}</p></div><div class="item-actions"><button class="button small" type="button" data-project-action="edit" data-project="${escapeHTML(project.id)}">프로젝트 이름 변경</button><button class="button small" type="button" data-project-action="add-repository" data-project="${escapeHTML(project.id)}">새 저장소 등록</button><button class="button small" type="button" data-project-action="export" data-project="${escapeHTML(project.id)}">프로젝트 내보내기</button><button class="button danger small" type="button" data-unregister="project" data-project="${escapeHTML(project.id)}" data-name="${escapeHTML(projectName)}">프로젝트 등록 해제</button></div></div>
      <div class="repository-list">${project.repos.map(repository => {
        const status = projectStatus(repository);
        const onlyRepository = project.repos.length <= 1;
        const registeredRepository = registered?.spec.repositories?.find(item => item.metadata.id === repository.id);
        const repositoryName = registeredRepository?.metadata.name || repository.id;
        return `<article class="repository-card ledger-row ${rowToneClass(status.tone)}" data-tone="${escapeHTML(status.tone)}"><div class="ledger-row__state">${stateText(status.text, status.tone)}</div><div class="ledger-row__main"><h3>${escapeHTML(repositoryName)}</h3><p class="repository-path"><code>${escapeHTML(repository.path)}</code></p>${onlyRepository ? '<p class="meta">마지막 저장소는 개별 해제할 수 없습니다. 프로젝트 등록 해제를 사용하세요.</p>' : ""}<details><summary>Worktree 상세 보기</summary><div class="worktree-list">${(repository.worktrees || []).length ? repository.worktrees.map(worktree => `<div class="worktree"><strong>${escapeHTML(worktree.metadata.id)} · ${worktree.spec.primary ? "기본" : "연결됨"}</strong><code>${escapeHTML(worktree.spec.canonicalPath)}</code><div class="meta">${escapeHTML(worktree.spec.branch || "detached")} · HEAD ${escapeHTML(worktree.spec.head || "확인 불가")} · ${worktree.spec.dirty ? "변경 있음" : "변경 없음"} · ${worktree.spec.untracked ? "추적하지 않은 파일 있음" : "추적되지 않은 파일 없음"} · upstream ${escapeHTML(worktree.spec.upstream || "없음")} ${escapeHTML(worktree.spec.ahead || 0)}/${escapeHTML(worktree.spec.behind || 0)} · ${worktree.spec.locked ? "잠김" : "잠기지 않음"} · ${worktree.spec.prunable ? "정리 가능 표시" : "유지됨"} · ${escapeHTML(label(worktree.spec.trust))} · ${escapeHTML(worktree.spec.tombstonedAt ? "관찰 종료" : worktree.spec.error || "현재")}</div></div>`).join("") : '<div class="empty-state"><span>관찰된 Worktree가 없습니다.</span></div>'}</div></details></div><div class="ledger-row__context"><span>브랜치 ${escapeHTML(repository.branch || "detached")}</span><span>ahead ${escapeHTML(repository.ahead || 0)} · behind ${escapeHTML(repository.behind || 0)}</span><span>Worktree ${escapeHTML((repository.worktrees || []).length)}</span></div><div class="ledger-row__action"><div class="repository-actions"><button class="button small" type="button" data-repository-action="edit" data-project="${escapeHTML(project.id)}" data-repository="${escapeHTML(repository.id)}">정보 변경</button><button class="button small" type="button" data-unregister="repository" data-project="${escapeHTML(project.id)}" data-repository="${escapeHTML(repository.id)}" data-name="${escapeHTML(repositoryName)}" ${onlyRepository ? "disabled" : ""}>저장소 등록 해제</button></div></div></article>`;
      }).join("")}</div>
      <div class="panel-heading section-heading"><div><p class="eyebrow">확인할 항목</p><h2>이 프로젝트의 확인할 항목</h2></div><div class="toolbar"><select id="finding-severity" aria-label="심각도 필터"><option value="">모든 심각도</option>${Object.entries(severityLabels).map(([value, text]) => `<option value="${value}" ${state.findingFilters.severity === value ? "selected" : ""}>${text}</option>`).join("")}</select><select id="finding-state" aria-label="상태 필터"><option value="active" ${state.findingFilters.state === "active" ? "selected" : ""}>열림 및 확인함</option><option value="">모든 상태</option>${["open", "acknowledged", "resolved", "suppressed", "expired"].map(value => `<option value="${value}" ${state.findingFilters.state === value ? "selected" : ""}>${escapeHTML(label(value))}</option>`).join("")}</select></div></div>
      ${visibleFindings.length ? `<div class="finding-list">${visibleFindings.map(item => findingCard(item)).join("")}</div>` : '<div class="empty-state"><strong>조건에 맞는 확인 항목이 없습니다.</strong><span>필터를 바꾸거나 새 점검을 실행하세요.</span></div>'}`;
    focusPendingProject();
  }

  function renderCheckRun(run) {
    return `<article class="run-detail"><div class="list-item-header"><strong>${escapeHTML(label(run.spec.status))}</strong><span class="meta">${escapeHTML(formatDate(run.spec.completedAt || run.spec.startedAt))}</span></div><dl class="detail-grid"><div><dt>Worktree</dt><dd>${escapeHTML(run.spec.worktreeId)}</dd></div><div><dt>HEAD</dt><dd><code>${escapeHTML(run.spec.head)}</code></dd></div></dl><div class="step-results">${(run.spec.steps || []).map(step => `<details ${step.status === "failed" ? "open" : ""}><summary>${escapeHTML(step.stepId)} · ${escapeHTML(label(step.status))}</summary><dl class="detail-grid"><div><dt>실행 종료 코드</dt><dd>${escapeHTML(step.exitCode ?? (step.status === "passed" ? 0 : "없음"))}</dd></div></dl>${step.stdout ? `<div><strong class="result-label">표준 출력</strong><pre class="command-output">${escapeHTML(step.stdout)}</pre></div>` : ""}${step.stderr ? `<div><strong class="result-label">오류 출력</strong><pre class="command-output error-output">${escapeHTML(step.stderr)}</pre></div>` : ""}${!step.stdout && !step.stderr ? '<p class="meta">기록된 출력이 없습니다.</p>' : ""}</details>`).join("")}</div></article>`;
  }

  function checksetCard(checkset, repository) {
    const runs = state.checkRuns.get(checkset.metadata.id) || [];
    const expanded = state.expandedChecks.has(checkset.metadata.id);
    const resultsID = `checkset-results-${encode(checkset.metadata.id)}`;
    const resultContent = runs.length ? runs.slice().reverse().map(renderCheckRun).join("") : "아직 실행 결과가 없습니다.";
    return `<article class="list-item"><div class="list-item-header"><div><h3>${escapeHTML(checkset.metadata.name)}</h3><p class="meta">${escapeHTML(repository.projectName)} / ${escapeHTML(repository.id)} / ${escapeHTML(checkset.spec.worktreeId)}</p></div><span class="chip">${escapeHTML(label(checkset.spec.state))}</span></div><details><summary>점검 단계와 근거</summary><dl class="detail-grid"><div><dt>HEAD</dt><dd><code>${escapeHTML(checkset.spec.head)}</code></dd></div><div><dt>제안</dt><dd>${escapeHTML(checkset.spec.proposalId)}</dd></div></dl>${(checkset.spec.steps || []).map(step => `<div class="command-card"><strong>${escapeHTML(step.name)}</strong><code>${escapeHTML([step.command.executable, ...(step.command.arguments || [])].join(" "))}</code><span class="meta">제한 시간 ${escapeHTML(step.command.timeoutSeconds)}초</span></div>`).join("")}</details><div class="item-actions"><button class="button small" type="button" data-checkset="apply" data-id="${escapeHTML(checkset.metadata.id)}" ${checkset.spec.state === "draft" ? "" : "disabled"}>적용</button><button class="button primary small" type="button" data-checkset="run" data-id="${escapeHTML(checkset.metadata.id)}" ${checkset.spec.state === "applied" ? "" : "disabled"}>실행</button><button class="button small" type="button" data-checkset="results" data-id="${escapeHTML(checkset.metadata.id)}" aria-expanded="${expanded}" aria-controls="${resultsID}">${expanded ? "결과 닫기" : "결과 보기"}</button></div><div id="${resultsID}" class="result-box" role="region" aria-label="${escapeHTML(checkset.metadata.name)} 결과" ${expanded ? "" : "hidden"}>${resultContent}</div>${!expanded && runs.length ? `<p class="meta">최근 결과 ${escapeHTML(label(runs[runs.length - 1].spec.status))}</p>` : ""}</article>`;
  }

  function proposalCard(proposal, item) {
    const alreadyCreated = (item.checksets || []).some(checkset => checkset.spec.proposalId === proposal.metadata.id);
    const command = proposal.spec.typedCommand
      ? [proposal.spec.typedCommand.executable, ...(proposal.spec.typedCommand.arguments || [])].join(" ")
      : proposal.spec.command;
    const reviewButtons = proposal.spec.state === "pending"
      ? `<button class="button primary small" type="button" data-proposal="apply" data-id="${escapeHTML(proposal.metadata.id)}">제안 적용</button><button class="button small" type="button" data-proposal="reject" data-id="${escapeHTML(proposal.metadata.id)}">제안 거절</button>`
      : proposal.spec.state === "applied" && proposal.spec.typedCommand && !alreadyCreated
        ? `<button class="button primary small" type="button" data-checkset="create" data-id="${escapeHTML(proposal.metadata.id)}">Checkset 만들기</button>`
        : proposal.spec.state === "stale"
          ? `<button class="button primary small" type="button" data-rediscover data-project="${escapeHTML(proposal.spec.projectId)}" data-repository="${escapeHTML(proposal.spec.repositoryId)}" data-worktree="${escapeHTML(proposal.spec.worktreeId)}">기존 점검 다시 찾기</button>`
          : "";
    return `<article class="list-item"><div class="list-item-header"><div><h3>${escapeHTML(proposal.metadata.name)}</h3><p class="meta">${escapeHTML(item.repository.projectName)} / ${escapeHTML(item.repository.id)} / ${escapeHTML(proposal.spec.worktreeId)}</p></div><span class="chip ${proposal.spec.state === "pending" || proposal.spec.state === "stale" ? "warn" : proposal.spec.state === "applied" ? "ok" : ""}">${escapeHTML(label(proposal.spec.state))}</span></div>${proposal.spec.state === "stale" ? '<p class="safety-note">Worktree 또는 원본이 바뀌었습니다. 다시 발견하세요.</p>' : ""}<p><code>${escapeHTML(command)}</code></p><details ${proposal.spec.state === "pending" ? "open" : ""}><summary>발견 근거 보기</summary><dl class="detail-grid"><div><dt>브랜치</dt><dd>${escapeHTML(proposal.spec.branch || "detached")}</dd></div><div><dt>HEAD</dt><dd><code>${escapeHTML(proposal.spec.head)}</code></dd></div><div><dt>명령 유형</dt><dd>${escapeHTML(proposal.spec.commandKind)}</dd></div><div><dt>추론</dt><dd>${escapeHTML(label(proposal.spec.inference))}</dd></div><div class="wide"><dt>원본 파일</dt><dd><code>${escapeHTML(proposal.spec.sourcePath)}</code></dd></div><div class="wide"><dt>원본 digest</dt><dd><code>${escapeHTML(proposal.spec.sourceDigest)}</code></dd></div><div><dt>발견 시각</dt><dd>${escapeHTML(formatDate(proposal.spec.createdAt))}</dd></div><div><dt>검토 시각</dt><dd>${escapeHTML(formatDate(proposal.spec.reviewedAt))}</dd></div></dl></details>${reviewButtons ? `<div class="item-actions">${reviewButtons}</div>` : ""}</article>`;
  }

  function renderActionEvidence(title, evidence) {
    return `<section class="evidence-group"><strong>${escapeHTML(title)}</strong>${(evidence || []).length ? `<ul>${evidence.map(item => `<li class="${item.passed ? "evidence-pass" : "evidence-fail"}">${escapeHTML(item.id)} · ${item.passed ? "통과" : "실패"}${item.detail ? ` · ${escapeHTML(item.detail)}` : ""}</li>`).join("")}</ul>` : '<p class="meta">기록된 근거가 없습니다.</p>'}</section>`;
  }

  function renderActionEvents(events) {
    return `<section class="evidence-group"><strong>감사 이벤트</strong>${(events || []).length ? `<ul>${events.map(item => `<li>${escapeHTML(item.spec.eventType)} · ${escapeHTML(formatDate(item.spec.occurredAt))}</li>`).join("")}</ul>` : '<p class="meta">기록된 감사 이벤트가 없습니다.</p>'}</section>`;
  }

  function renderActionRun(run, events = []) {
    const context = run.spec.executionContext || {};
    return `<article class="run-detail"><div class="list-item-header"><strong>${escapeHTML(label(run.spec.status))}</strong><span class="meta">${escapeHTML(formatDate(run.spec.completedAt || run.spec.startedAt))}</span></div><dl class="detail-grid"><div><dt>저장소</dt><dd>${escapeHTML(run.spec.repositoryId)}</dd></div><div><dt>Worktree</dt><dd>${escapeHTML(run.spec.worktreeId)}</dd></div><div><dt>작업 폴더</dt><dd><code>${escapeHTML(context.canonicalPath)}</code></dd></div><div><dt>HEAD</dt><dd><code>${escapeHTML(context.head)}</code></dd></div><div><dt>실행 종료 코드</dt><dd>${escapeHTML(run.spec.exitCode ?? (run.spec.status === "succeeded" ? 0 : "없음"))}</dd></div><div><dt>실행자</dt><dd>${escapeHTML(run.spec.holder)}</dd></div></dl>${renderActionEvidence("사전 점검", run.spec.prechecks)}${run.spec.stdout ? `<div><strong class="result-label">표준 출력</strong><pre class="command-output">${escapeHTML(run.spec.stdout)}</pre></div>` : ""}${run.spec.stderr ? `<div><strong class="result-label">오류 출력</strong><pre class="command-output error-output">${escapeHTML(run.spec.stderr)}</pre></div>` : ""}${renderActionEvidence("사후 점검", run.spec.postchecks)}${renderActionEvents(events)}</article>`;
  }

  function renderRepositorySyncPlan() {
    const plan = state.repositorySyncPlan;
    const result = state.repositorySyncResult;
    if (!plan && !result) return "";
    if (!plan && result) {
      const outcomes = (result.outcomes || []).map(item => {
        const run = item.run;
        const status = run?.spec?.status || (item.error ? "failed" : "succeeded");
        const detail = state.actionDetails.find(candidate => candidate.plan.metadata.id === item.planId);
        return `<article class="list-item"><div class="list-item-header"><div><strong>${escapeHTML(item.repositoryId)}</strong><p class="meta">${escapeHTML(item.worktreeId)}</p></div><span class="chip ${status === "succeeded" ? "ok" : "bad"}">${escapeHTML(label(status))}</span></div>${run ? renderActionRun(run, detail?.status?.events) : `<p class="next">${escapeHTML(item.error || "실행 기록이 없습니다.")}</p>`}</article>`;
      }).join("");
      const statuses = (result.outcomes || []).map(item => item.run?.spec?.status || (item.error ? "failed" : "succeeded"));
      const overall = statuses.length && statuses.every(status => status === "succeeded") ? "succeeded" : statuses.some(status => ["failed", "timed_out", "cancelled", "precheck_failed", "postcheck_failed", "unavailable"].includes(status)) ? "failed" : "running";
      return `<section class="review-box"><div class="list-item-header"><div><strong>프로젝트 저장소 최신화 결과</strong><p class="meta">저장소별 실행 결과를 확인하세요.</p></div><span class="chip ${overall === "succeeded" ? "ok" : "bad"}">${escapeHTML(label(overall))}</span></div><div class="item-list">${outcomes || '<div class="empty-state">실행 결과가 없습니다.</div>'}</div></section>`;
    }
    const skipped = (plan.skipped || []).map(item => `<li><strong>${escapeHTML(item.repositoryName || item.repositoryId)}</strong> · ${escapeHTML(item.reason)}</li>`).join("");
    return `<section class="review-box"><div class="list-item-header"><div><strong>프로젝트 저장소 전체 최신화 계획</strong><p class="meta">실행 가능한 저장소 ${escapeHTML((plan.plans || []).length)}개 · 제외 ${escapeHTML((plan.skipped || []).length)}개</p></div><span class="chip ${(plan.plans || []).length ? "warn" : ""}">${(plan.plans || []).length ? "검토 필요" : "실행 대상 없음"}</span></div><p>변경 없는 기본 Worktree만 <code>git pull --ff-only --prune</code>으로 처리합니다. 계획에 없는 저장소는 실행하지 않습니다.</p>${skipped ? `<details open><summary>제외된 저장소 보기</summary><ul>${skipped}</ul></details>` : ""}${(plan.plans || []).length ? `<div class="item-actions"><button class="button primary" type="button" data-repository-sync="execute">계획된 저장소 모두 최신화</button></div>` : ""}</section>`;
  }

  const specialActionTypes = new Set(["external.jenkins.group", "release.jenkins.stage", "release.jenkins.production", "cleanup.destructive"]);
  const isSpecialPlan = detail => specialActionTypes.has(detail.plan.spec.actionType);

  function renderOperationResult(result, actionType) {
    if (!result) return "";
    if (actionType === "cleanup.destructive") {
      return `<div class="result-box ${result.status === "succeeded" ? "state-ok" : "state-bad"}"><strong>정리 ${escapeHTML(label(result.status))}</strong><p><code>${escapeHTML(result.path)}</code>${result.branch ? ` · ${escapeHTML(result.branch)}` : ""}</p>${result.failure ? `<p>${escapeHTML(result.failure)}</p>` : ""}</div>`;
    }
    if (actionType.startsWith("release.")) {
      return `<div class="result-box ${result.status === "succeeded" ? "state-ok" : "state-bad"}"><strong>${escapeHTML(result.environment === "production" ? "Production" : "Stage")} ${escapeHTML(label(result.status))}</strong>${(result.postchecks || []).map(item => `<p>${escapeHTML(item.id)} · ${escapeHTML(label(item.status))} · ${escapeHTML(item.detail || "")}</p>`).join("")}</div>`;
    }
    return `<div class="result-box ${result.status === "succeeded" ? "state-ok" : "state-bad"}"><strong>외부 작업 ${escapeHTML(label(result.status))}</strong>${(result.outcomes || []).map(item => `<p>${escapeHTML(item.targetId)} · ${escapeHTML(label(item.status))}${item.buildNumber ? ` · #${escapeHTML(item.buildNumber)}` : ""}${item.failure ? ` · ${escapeHTML(item.failure)}` : ""}</p>`).join("")}</div>`;
  }

  function renderExternalOperations() {
    const targets = targetOptions();
    const groups = state.externalGroups;
    const plans = state.actionDetails.filter(isSpecialPlan);
    const groupOptions = groups.length ? groups.map(group => `<option value="${escapeHTML(group.id)}">${escapeHTML(group.name)} · 대상 ${escapeHTML(group.targets.length)}개</option>`).join("") : '<option value="">진단에서 대상 그룹을 먼저 등록하세요</option>';
    const targetOptionsHTML = targets.length ? targets.map(target => `<option value="${escapeHTML(target.value)}">${escapeHTML(target.label)}</option>`).join("") : '<option value="">관찰된 Worktree 없음</option>';
    const planCards = plans.length ? `<div class="item-list">${plans.map(detail => {
      const actionType = detail.plan.spec.actionType;
      const admission = detail.status.admission;
      const result = state.operationResults[detail.plan.metadata.id];
      const operation = actionType === "cleanup.destructive" ? "cleanup" : actionType.startsWith("release.") ? "release" : "external";
      const executeLabel = operation === "cleanup" ? "정리 실행" : operation === "release" ? "릴리스 실행" : "외부 작업 실행";
      const button = admission === "approval_required"
        ? `<button class="button small" type="button" data-action="approve" data-id="${escapeHTML(detail.plan.metadata.id)}">승인 요청</button>`
        : admission === "eligible"
          ? `<button class="button primary small" type="button" data-special-action="${operation}" data-id="${escapeHTML(detail.plan.metadata.id)}">${executeLabel}</button>`
          : "";
      return `<article class="list-item"><div class="list-item-header"><div><h3>${escapeHTML(detail.plan.metadata.name)}</h3><p class="meta">${escapeHTML(detail.plan.spec.projectId)} / ${escapeHTML(detail.plan.spec.repositoryId)} / ${escapeHTML(detail.plan.spec.worktreeId)}</p></div><span class="chip ${admission === "eligible" ? "ok" : admission === "approval_required" ? "warn" : "bad"}">${escapeHTML(label(admission))}</span></div><dl class="detail-grid"><div><dt>Action</dt><dd><code>${escapeHTML(actionType)}</code></dd></div><div><dt>요청 시각</dt><dd>${escapeHTML(formatDate(detail.plan.spec.requestedAt))}</dd></div><div class="wide"><dt>승인 기록</dt><dd>${detail.status.approvals?.length ? detail.status.approvals.map(item => `${escapeHTML(label(item.spec.status))} · ${escapeHTML(formatDate(item.spec.decidedAt))}`).join("<br>") : "없음"}</dd></div><div class="wide"><dt>계획 digest</dt><dd><code>${escapeHTML(detail.plan.spec.inputs?.group_digest || detail.plan.spec.inputs?.candidate_digest || "서버가 보관")}</code></dd></div></dl><div class="item-actions"><button class="button small" type="button" data-action="trust" data-id="${escapeHTML(detail.plan.metadata.id)}">실행 대상으로 표시</button>${button}</div>${renderOperationResult(result, actionType)}</article>`;
    }).join("")}</div>` : '<div class="empty-state"><strong>아직 외부 작업·릴리스·정리 계획이 없습니다.</strong><span>위 입력에서 계획을 만든 뒤 이곳에서 근거를 검토하고 승인하세요.</span></div>';
    document.getElementById("operations-ui").innerHTML = `<div class="workflow-note"><strong>실행 순서</strong><span>대상 확인 → 계획 생성 → Worktree 확인 → 승인 → 실행</span></div><div class="toolbar"><select id="external-group" aria-label="외부 작업 대상 그룹">${groupOptions}</select><select id="external-target" aria-label="외부 작업 Worktree">${targetOptionsHTML}</select><input id="expected-revision" name="expectedRevision" autocomplete="off" aria-label="예상 revision" placeholder="예상 revision (선택 사항)"><button id="external-plan" class="button" type="button" ${groups.length && targets.length ? "" : "disabled"}>외부 작업 계획 만들기</button><button id="release-stage-plan" class="button" type="button" ${groups.length && targets.length ? "" : "disabled"}>Stage 계획 만들기</button><button id="release-production-plan" class="button danger" type="button" ${groups.length && targets.length ? "" : "disabled"}>Production 계획 만들기</button></div>${groups.length ? "" : '<p class="safety-note">진단의 실행 설정에서 Jenkins 연동과 대상 그룹을 먼저 등록하세요.</p>'}${planCards}`;
  }

  function renderWork() {
    const targets = targetOptions();
    renderQualityWorkSurface();
    renderExternalOperations();
    const proposalHTML = state.workItems.flatMap(item => (item.proposals || []).map(proposal => proposalCard(proposal, item))).join("");
    document.getElementById("proposal-ui").innerHTML = `${state.surfaceErrors.checksets ? surfaceError(state.surfaceErrors.checksets, "work") : ""}<div class="toolbar"><select id="discovery-target" aria-label="발견 대상 Worktree">${targets.length ? targets.map(target => `<option value="${escapeHTML(target.value)}">${escapeHTML(target.label)}</option>`).join("") : '<option value="">관찰된 Worktree 없음</option>'}</select><button id="discover-worktree" class="button primary" type="button" ${targets.length ? "" : "disabled"}>기존 점검 찾기</button></div>${proposalHTML ? `<div class="item-list">${proposalHTML}</div>` : '<div class="empty-state"><strong>검토할 제안이 없습니다.</strong><span>Worktree를 선택해 기존 점검 명령을 찾아보세요.</span></div>'}`;

    const checksetHTML = state.workItems.flatMap(item => (item.checksets || []).map(checkset => checksetCard(checkset, item.repository))).join("");
    const checksetContent = checksetHTML
      ? `<div class="item-list">${checksetHTML}</div>`
      : '<div class="empty-state"><strong>실행할 Pre-PR 점검이 없습니다.</strong><span>제안을 적용한 뒤 Checkset으로 만드세요.</span></div>';
    document.getElementById("checksets").innerHTML = checksetContent;

    const plans = state.actionDetails.filter(detail => !isSpecialPlan(detail)).map(detail => {
      const admission = detail.status.admission;
      const latest = detail.runs?.[0];
      const resultVisible = state.expandedActions.has(detail.plan.metadata.id);
      const resultsID = `action-results-${encode(detail.plan.metadata.id)}`;
      const resultContent = detail.runs.length ? detail.runs.map(run => renderActionRun(run, detail.status.events)).join("") : "아직 실행 결과가 없습니다.";
      const actionButtons = admission === "approval_required"
        ? `<button class="button small" data-action="approve" data-id="${escapeHTML(detail.plan.metadata.id)}">승인 요청</button>`
        : admission === "eligible"
          ? `<button class="button primary small" data-action="execute" data-id="${escapeHTML(detail.plan.metadata.id)}">실행</button>`
          : "";
      return `<article class="list-item"><div class="list-item-header"><div><h3>${escapeHTML(detail.plan.metadata.name)}</h3><p class="meta">${escapeHTML(detail.plan.spec.projectId)} / ${escapeHTML(detail.plan.spec.repositoryId)} / ${escapeHTML(detail.plan.spec.worktreeId)}</p></div><span class="chip ${admission === "eligible" ? "ok" : admission === "approval_required" ? "warn" : "bad"}">${escapeHTML(label(admission))}</span></div><details><summary>계획과 승인 근거</summary><dl class="detail-grid"><div><dt>Action</dt><dd><code>${escapeHTML(detail.plan.spec.actionType)}</code></dd></div><div><dt>위험 등급</dt><dd>${escapeHTML(label(detail.plan.spec.risk))}</dd></div><div><dt>정책 판단</dt><dd>${escapeHTML(label(detail.plan.spec.policyDecision))}</dd></div><div><dt>요청 시각</dt><dd>${escapeHTML(formatDate(detail.plan.spec.requestedAt))}</dd></div><div class="wide"><dt>실행 명령</dt><dd><code>${escapeHTML([detail.plan.spec.execution?.executable, ...(detail.plan.spec.execution?.arguments || [])].join(" "))}</code></dd></div><div class="wide"><dt>승인 기록</dt><dd>${detail.status.approvals?.length ? detail.status.approvals.map(approval => `${escapeHTML(label(approval.spec.status))} · ${escapeHTML(formatDate(approval.spec.decidedAt))}`).join("<br>") : "없음"}</dd></div><div class="wide"><dt>감사 이벤트</dt><dd>${detail.status.events?.length ? detail.status.events.map(item => `${escapeHTML(item.spec.eventType)} · ${escapeHTML(formatDate(item.spec.occurredAt))}`).join("<br>") : "없음"}</dd></div></dl></details><div class="item-actions"><button class="button small" type="button" data-action="trust" data-id="${escapeHTML(detail.plan.metadata.id)}">실행 대상으로 표시</button>${actionButtons}<button class="button small" type="button" data-action="runs" data-id="${escapeHTML(detail.plan.metadata.id)}" aria-expanded="${resultVisible}" aria-controls="${resultsID}">${resultVisible ? "결과 닫기" : "결과 보기"}</button></div><div id="${resultsID}" class="result-box" role="region" aria-label="${escapeHTML(detail.plan.metadata.name)} 결과" ${resultVisible ? "" : "hidden"}>${resultContent}</div>${!resultVisible && latest ? `<p class="meta">최근 결과 ${escapeHTML(label(latest.spec.status))}</p>` : ""}</article>`;
    }).join("");
    const projects = state.snapshot.projects || [];
    document.getElementById("action-ui").innerHTML = `${state.surfaceErrors.actions ? surfaceError(state.surfaceErrors.actions, "work") : ""}<div class="toolbar"><select id="sync-project" aria-label="전체 최신화 대상 프로젝트">${projects.length ? projects.map(project => `<option value="${escapeHTML(project.id)}">${escapeHTML(project.name)} · ${escapeHTML(project.repos.length)}개 저장소</option>`).join("") : '<option value="">등록된 프로젝트 없음</option>'}</select><button id="repository-sync-plan" class="button primary" type="button" ${projects.length ? "" : "disabled"}>프로젝트 저장소 전체 최신화 계획</button></div>${renderRepositorySyncPlan()}<div class="toolbar"><select id="action-target" aria-label="Action 대상 Worktree">${targets.length ? targets.map(target => `<option value="${escapeHTML(target.value)}">${escapeHTML(target.label)}</option>`).join("") : '<option value="">관찰된 Worktree 없음</option>'}</select><button id="action-plan" class="button" type="button" ${targets.length ? "" : "disabled"}>단일 저장소 새로고침 계획</button></div>${plans ? `<div class="item-list">${plans}</div>` : '<div class="empty-state"><strong>검토된 Action 계획이 없습니다.</strong><span>대상 Worktree를 선택해 저장소 새로고침 계획을 만들 수 있습니다.</span></div>'}`;
  }

  function environmentSource(type) {
    if (type?.startsWith("tool.")) return "Tool";
    if (type?.startsWith("agent_profile.")) return "Agent Profile";
    return "설정";
  }

  function renderGuidanceResult() {
    const result = state.guidanceResult;
    if (!result) return "";
    if (state.guidanceMode === "error") return `<div class="result-box state-bad" role="alert">${escapeHTML(result.message || result)}</div>`;
    if (state.guidanceMode === "guidance") {
      return `<section class="review-box"><div class="list-item-header"><strong>지침 점검 결과</strong><span class="meta">${escapeHTML(formatDate(result.checkedAt))}</span></div><dl class="detail-grid"><div class="wide"><dt>확인한 파일</dt><dd>${(result.files || []).length ? result.files.map(file => `<code>${escapeHTML(file)}</code>`).join("<br>") : "없음"}</dd></div></dl>${(result.findings || []).length ? `<div class="finding-list">${result.findings.map(item => `<article class="finding ${escapeHTML(item.severity)}"><div class="finding-header"><h3>${escapeHTML(localize(item.summary))}</h3><span class="chip">${escapeHTML(severityLabels[item.severity] || item.severity)}</span></div><p class="meta">${escapeHTML(item.file || item.code)}</p><p class="next">다음 단계: ${escapeHTML(localize(item.recommendedNextAction))}</p></article>`).join("")}</div>` : '<div class="empty-state"><strong>기계적으로 확인된 문제는 없습니다.</strong><span>이 결과는 의미상 최신 상태를 보장하지 않습니다.</span></div>'}</section>`;
    }
    const launch = result.launch;
    return `<section class="review-box"><div class="list-item-header"><strong>Agent Handoff 검토</strong><span class="chip ok">마스킹됨</span></div><dl class="detail-grid"><div><dt>Agent Profile</dt><dd>${escapeHTML(result.profileName)} · <code>${escapeHTML(result.profileCommand)}</code></dd></div><div><dt>데이터 경계</dt><dd>${escapeHTML(result.dataBoundary)}</dd></div><div><dt>실행 방식</dt><dd>${escapeHTML(result.launchMode)}</dd></div><div><dt>모델</dt><dd>${escapeHTML(result.model || "Profile 기본값")}</dd></div><div><dt>Worktree 상태</dt><dd>${escapeHTML(result.branch || "detached")} · HEAD <code>${escapeHTML(result.head || "없음")}</code> · ${result.dirty || result.untracked ? "변경 있음" : "깨끗함"}</dd></div><div class="wide"><dt>작업 폴더</dt><dd><code>${escapeHTML(result.workingDirectory)}</code></dd></div><div class="wide"><dt>포함 범위</dt><dd>${(result.scope || []).map(item => escapeHTML(item)).join("<br>") || "없음"}</dd></div><div class="wide"><dt>검증 명령</dt><dd>${(result.verificationCommands || []).map(item => `<code>${escapeHTML(item)}</code>`).join("<br>") || "없음"}</dd></div><div class="wide"><dt>Preview digest</dt><dd><code>${escapeHTML(result.previewDigest)}</code></dd></div></dl><details><summary>실행 인자 확인</summary><pre class="command-output">${escapeHTML((result.arguments || []).join("\n"))}</pre></details><div class="finding-list">${(result.findings || []).map(item => `<article class="finding ${escapeHTML(item.severity)}"><div class="finding-header"><h3>${escapeHTML(localize(item.summary))}</h3><span class="chip">${escapeHTML(severityLabels[item.severity] || item.severity)}</span></div><p class="next">${escapeHTML(localize(item.recommendedNextAction))}</p></article>`).join("") || '<div class="empty-state"><span>Handoff에 포함할 확인 항목이 없습니다.</span></div>'}</div>${launch ? `<div class="result-box state-ok"><strong>Agent를 열었습니다.</strong><p>PID ${escapeHTML(launch.pid)} · ${escapeHTML(formatDate(launch.startedAt))}</p><p class="meta">대화 기록은 수집하지 않습니다.</p></div>` : '<div class="item-actions"><button class="button primary" type="button" data-handoff-launch>이 Agent로 열기</button></div>'}<p class="safety-note">전체 대화 기록: ${result.transcriptIncluded ? "포함됨" : "포함하지 않음"}</p></section>`;
  }

  const externalTargetText = group => (group.targets || []).map(target => {
    const parameters = Object.entries(target.parameters || {}).map(([key, value]) => `${key}=${value}`).join(",");
    return [target.id, target.integrationId, target.completedBuildUrl, parameters, target.fallbackRunbookId || ""].join(" | ");
  }).join("\n");

  function renderExternalGroups() {
    const container = document.getElementById("external-groups");
    const content = state.externalGroups.length
      ? `<div class="item-list">${state.externalGroups.map(group => `<article class="list-item"><div class="list-item-header"><div><h3>${escapeHTML(group.name)}</h3><p class="meta">${escapeHTML(group.id)} · 대상 ${escapeHTML(group.targets.length)}개</p></div><span class="chip warn">Jenkins</span></div><div class="group-targets">${group.targets.map(target => `<div><strong>${escapeHTML(target.id)}</strong><span>${escapeHTML(target.integrationId)}</span><code>${escapeHTML(target.completedBuildUrl)}</code></div>`).join("")}</div><div class="item-actions"><button class="button small" type="button" data-external-group="edit" data-id="${escapeHTML(group.id)}">정보 변경</button><button class="button small" type="button" data-unregister="external-group" data-external-group-id="${escapeHTML(group.id)}" data-name="${escapeHTML(group.name)}">제거</button></div></article>`).join("")}</div>`
      : '<div class="empty-state"><strong>Jenkins 대상 그룹이 없습니다.</strong><span>연동을 먼저 등록한 뒤 완료된 build URL을 하나 이상의 대상에 묶으세요.</span></div>';
    container.innerHTML = (state.surfaceErrors.externalGroups ? surfaceError(state.surfaceErrors.externalGroups, "diagnostics") : "") + content;
  }

  function renderDiagnostics() {
    const environment = state.environment;
    const environmentReady = requiredEnvironmentReady(environment);
    const providerIDs = new Set((state.providerStatuses || []).map(item => item.provider).filter(Boolean));
    const visibleEnvironmentFindings = (environment.findings || []).filter(item => {
      if (!item.type?.startsWith("tool.")) return true;
      const tool = (environment.tools || []).find(candidate => candidate.name === item.target);
      return !tool || tool.required;
    }).filter(item => {
      if (!item.type?.startsWith("agent_profile.")) return true;
      return !optionalProviderIDs.has(item.target);
    }).filter(item => {
      const type = String(item.type || "");
      const target = String(item.target || "");
      const providerFinding = (type.startsWith("tool.") || type.startsWith("agent_profile.")) && providerIDs.has(target);
      return !providerFinding;
    }).filter((item, index, all) => all.findIndex(candidate => candidate.type === item.type && candidate.target === item.target) === index);
    document.getElementById("environment").innerHTML = `<div class="list-item ${environment.generatedAt && environmentReady ? "state-ok" : "state-warn"}"><strong>${!environment.generatedAt ? "아직 환경을 확인하지 않았습니다." : environmentReady ? "필수 도구를 사용할 수 있습니다." : "필수 도구를 확인하세요."}</strong><p class="meta">필수 도구의 상태는 아래에서 확인합니다.</p></div>${visibleEnvironmentFindings.length ? `<div class="finding-list diagnostic-findings">${visibleEnvironmentFindings.map(item => { const tone = findingTone(String(item.severity || "info")); return `<article class="ledger-row finding ${rowToneClass(tone)}" data-tone="${escapeHTML(tone)}" data-severity="${escapeHTML(item.severity || "info")}"><div class="ledger-row__state">${stateText(severityLabels[item.severity] || item.severity, tone)}</div><div class="ledger-row__main"><h3>${escapeHTML(environmentSource(item.type))} · ${escapeHTML(item.target || item.type)}</h3><p>${escapeHTML(localize(item.summary))}</p><p class="next">다음 단계: ${escapeHTML(localize(item.recommendedNextAction))}</p></div><div class="ledger-row__context">환경 확인</div><div class="ledger-row__action"></div></article>`; }).join("")}</div>` : ""}`;
    renderQualityTools();
    renderProviderStatuses("provider-statuses");

    const targets = targetOptions();
    document.getElementById("guidance-ui").innerHTML = `${state.surfaceErrors.profiles ? surfaceError(state.surfaceErrors.profiles, "diagnostics") : ""}<div class="toolbar"><select id="guidance-target" aria-label="지침 점검 대상">${targets.length ? targets.map(target => `<option value="${escapeHTML(target.value)}">${escapeHTML(target.label)}</option>`).join("") : '<option value="">관찰된 Worktree 없음</option>'}</select><select id="handoff-profile" aria-label="Agent Profile">${state.profiles.length ? state.profiles.map(profile => `<option value="${escapeHTML(profile.metadata.id)}">${escapeHTML(profile.metadata.name)}</option>`).join("") : '<option value="">Agent Profile 없음</option>'}</select><input id="handoff-model" name="model" autocomplete="off" aria-label="선택 모델" placeholder="모델 선택 사항"><button id="guidance-check" class="button" type="button" ${targets.length ? "" : "disabled"}>지침 확인</button><button id="handoff-preview" class="button" type="button" ${targets.length && state.profiles.length ? "" : "disabled"}>Agent 전달 내용 보기</button></div>${renderGuidanceResult()}`;

    document.getElementById("profile-list").innerHTML = state.profiles.length ? `<div class="item-list">${state.profiles.map(profile => `<article class="list-item"><div class="list-item-header"><div><h3>${escapeHTML(profile.metadata.name)}</h3><p class="meta">${escapeHTML(profile.metadata.id)}</p></div><span class="chip">${escapeHTML(label(profile.spec.dataBoundary))}</span></div><dl class="detail-grid"><div><dt>실행 명령</dt><dd><code>${escapeHTML(profile.spec.command)}</code></dd></div><div><dt>실행 방식</dt><dd>${escapeHTML(label(profile.spec.launchMode))}</dd></div><div><dt>제한 시간</dt><dd>${escapeHTML(profile.spec.timeoutSeconds)}초</dd></div><div><dt>모델 인자</dt><dd>${escapeHTML(profile.spec.modelArgumentTemplate || "없음")}</dd></div><div class="wide"><dt>허용 환경 변수</dt><dd>${escapeHTML((profile.spec.environmentAllowlist || []).join(", ") || "없음")}</dd></div></dl><div class="item-actions"><button class="button small" type="button" data-profile="edit" data-id="${escapeHTML(profile.metadata.id)}">정보 변경</button><button class="button small" type="button" data-unregister="profile" data-profile-id="${escapeHTML(profile.metadata.id)}" data-name="${escapeHTML(profile.metadata.name)}">제거</button></div></article>`).join("")}</div>` : '<div class="empty-state"><strong>Agent Profile이 없습니다.</strong><span>Handoff 미리 보기를 사용하려면 Profile을 추가하세요.</span></div>';

    const integrationContent = state.integrations.length
      ? `<div class="item-list">${state.integrations.map(item => {
        const health = state.integrationHealth[item.id];
        const healthStatus = health?.status || "not_checked";
        const latest = state.githubLatestRuns[item.id];
        const latestURL = latest?.url && /^https?:\/\//i.test(latest.url) ? latest.url : "";
        const latestMarkup = item.kind === "github" && latest ? `<div class="wide"><dt>최근 workflow</dt><dd>${latest.runId ? `${escapeHTML(label(latest.status))} · ${escapeHTML(conclusionLabels[latest.conclusion] || latest.conclusion || "결과 없음")} · ${escapeHTML(latest.branch || "브랜치 없음")} · 실행 #${escapeHTML(latest.runId)}${latestURL ? ` · <a href="${escapeHTML(latestURL)}" target="_blank" rel="noreferrer">실행 열기</a>` : ""}` : "최근 실행이 없습니다."}</dd></div>` : "";
        const build = state.jenkinsLatestBuilds[item.id];
        const buildURL = build?.url && /^https?:\/\//i.test(build.url) ? build.url : "";
        const buildMarkup = item.kind === "jenkins" && build ? `<div class="wide"><dt>최근 빌드</dt><dd>${build.buildNumber ? `${build.building ? "빌드 중" : escapeHTML(buildResultLabels[build.result] || build.result || "결과 없음")} · ${escapeHTML(build.displayName || `#${build.buildNumber}`)}${buildURL ? ` · <a href="${escapeHTML(buildURL)}" target="_blank" rel="noreferrer">빌드 열기</a>` : ""}` : "최근 빌드가 없습니다."}</dd></div>` : "";
        const kubernetesStatus = state.kubernetesStatuses[item.id];
        const kubernetesLog = state.kubernetesLogs[item.id];
        const podMarkup = item.kind === "kubernetes" && kubernetesStatus ? `<div class="wide"><dt>Pod 상태</dt><dd>${kubernetesStatus.pods?.length ? kubernetesStatus.pods.map(pod => `${escapeHTML(pod.name)} · ${escapeHTML(pod.phase || "상태 없음")} · ${pod.ready ? "준비됨" : "준비되지 않음"} · 재시작 ${escapeHTML(pod.restartCount)}`).join("<br>") : "selector에 맞는 Pod가 없습니다."}</dd></div>` : "";
        const logMarkup = item.kind === "kubernetes" && kubernetesLog ? `<div class="wide"><dt>최근 로그 · ${escapeHTML(kubernetesLog.pod)}</dt><dd><pre class="command-output">${escapeHTML(kubernetesLog.logs || "기록된 로그가 없습니다.")}</pre></dd></div>` : "";
        return `<article class="list-item"><div class="list-item-header"><div><h3>${escapeHTML(item.name)}</h3><p class="meta">${escapeHTML(item.id)} · ${escapeHTML(item.kind)}</p></div><span class="chip ${healthStatus === "passed" ? "ok" : healthStatus === "failed" ? "bad" : "warn"}">${escapeHTML(health ? label(healthStatus) : "미확인")}</span></div><dl class="detail-grid"><div class="wide"><dt>Endpoint</dt><dd><code>${escapeHTML(item.endpoint)}</code></dd></div><div><dt>Credential</dt><dd><code>${escapeHTML(item.credentialRef || "없음")}</code></dd></div><div class="wide"><dt>대상 값</dt><dd>${Object.entries(item.values || {}).map(([key, value]) => `<code>${escapeHTML(key)}=${escapeHTML(value)}</code>`).join("<br>") || "없음"}</dd></div>${health ? `<div class="wide"><dt>마지막 확인</dt><dd>${escapeHTML(health.message)}${health.httpStatus ? ` · HTTP ${escapeHTML(health.httpStatus)}` : ""}</dd></div>` : ""}${latestMarkup}${buildMarkup}${podMarkup}${logMarkup}</dl><div class="item-actions"><button class="button small" type="button" data-integration="check" data-id="${escapeHTML(item.id)}">연결 확인</button>${item.kind === "github" ? `<button class="button small" type="button" data-integration="github-latest" data-id="${escapeHTML(item.id)}">최근 workflow 확인</button>` : ""}${item.kind === "jenkins" ? `<button class="button small" type="button" data-integration="jenkins-latest" data-id="${escapeHTML(item.id)}">최근 빌드 확인</button>` : ""}${item.kind === "kubernetes" ? `<button class="button small" type="button" data-integration="kubernetes-status" data-id="${escapeHTML(item.id)}">Pod 상태 확인</button><button class="button small" type="button" data-integration="kubernetes-logs" data-id="${escapeHTML(item.id)}">최근 로그 확인</button>` : ""}<button class="button small" type="button" data-integration="edit" data-id="${escapeHTML(item.id)}">정보 변경</button><button class="button small" type="button" data-unregister="integration" data-integration-id="${escapeHTML(item.id)}" data-name="${escapeHTML(item.name)}">제거</button></div></article>`;
      }).join("")}</div>`
      : '<div class="empty-state"><strong>등록된 연동이 없습니다.</strong><span>먼저 주소와 credential reference만 등록하세요.</span></div>';
    document.getElementById("integration-list").innerHTML = (state.surfaceErrors.integrations ? surfaceError(state.surfaceErrors.integrations, "diagnostics") : "") + integrationContent;
    renderExternalGroups();

    const runbookContent = state.runbooks.length
      ? `<div class="item-list">${state.runbooks.map(item => `<article class="list-item"><div class="list-item-header"><div><h3>${escapeHTML(item.name)}</h3><p class="meta">${escapeHTML(item.id)} · 승인 필요</p></div><span class="chip warn">PowerShell</span></div><dl class="detail-grid"><div class="wide"><dt>스크립트</dt><dd><code>${escapeHTML(item.scriptPath)}</code></dd></div><div><dt>제한 시간</dt><dd>${escapeHTML(item.timeoutSeconds)}초</dd></div><div class="wide"><dt>허용 환경 변수</dt><dd>${escapeHTML((item.environmentAllowlist || []).join(", ") || "없음")}</dd></div></dl>${(item.parameters || []).length ? `<div class="runbook-parameters">${item.parameters.map(parameter => `<label><span>${escapeHTML(parameter)}</span><input data-runbook-param="${escapeHTML(parameter)}" data-runbook-id="${escapeHTML(item.id)}" name="${escapeHTML(parameter)}" autocomplete="off" placeholder="선택 값"></label>`).join("")}</div>` : '<p class="meta">입력할 parameter가 없습니다.</p>'}<div class="item-actions"><select data-runbook-target="${escapeHTML(item.id)}" aria-label="runbook 실행 Worktree">${targets.length ? targets.map(target => `<option value="${escapeHTML(target.value)}">${escapeHTML(target.label)}</option>`).join("") : '<option value="">관찰된 Worktree 없음</option>'}</select><button class="button primary small" type="button" data-runbook="plan" data-id="${escapeHTML(item.id)}" ${targets.length ? "" : "disabled"}>실행 계획 만들기</button><button class="button small" type="button" data-runbook="edit" data-id="${escapeHTML(item.id)}">정보 변경</button><button class="button small" type="button" data-unregister="runbook" data-runbook-id="${escapeHTML(item.id)}" data-name="${escapeHTML(item.name)}">제거</button></div></article>`).join("")}</div>`
      : '<div class="empty-state"><strong>등록된 PowerShell runbook이 없습니다.</strong><span>검토한 .ps1 파일을 등록하면 선택한 Worktree에서 실행 계획을 만들 수 있습니다.</span></div>';
    document.getElementById("runbook-list").innerHTML = (state.surfaceErrors.runbooks ? surfaceError(state.surfaceErrors.runbooks, "diagnostics") : "") + runbookContent;

    const cleanupContent = state.cleanup.length
      ? `<div class="item-list">${state.cleanup.map(item => `<article class="list-item"><div class="list-item-header"><h3>${escapeHTML(item.spec.repositoryId)} / ${escapeHTML(item.spec.worktreeId)}</h3><span class="chip warn">${escapeHTML(label(item.spec.decision))}</span></div><p class="meta"><code>${escapeHTML(item.spec.canonicalPath)}</code></p><p>${(item.spec.reasons || []).map(reason => escapeHTML(localize(reason))).join("<br>")}</p>${item.spec.decision === "reviewable" ? `<div class="item-actions"><button class="button danger small" type="button" data-cleanup="plan" data-id="${escapeHTML(item.metadata.id)}" data-project="${escapeHTML(item.spec.projectId)}" data-repository="${escapeHTML(item.spec.repositoryId)}" data-worktree="${escapeHTML(item.spec.worktreeId)}">정리 계획 만들기</button></div>` : ""}</article>`).join("")}</div>`
      : '<div class="empty-state"><strong>관찰된 정리 후보가 없습니다.</strong><span>Worktree가 관찰되면 안전 여부를 읽기 전용으로 평가합니다.</span></div>';
    document.getElementById("cleanup-queue").innerHTML = (state.surfaceErrors.cleanup ? surfaceError(state.surfaceErrors.cleanup, "diagnostics") : "") + cleanupContent;

    const safeguardContent = state.safeguards.length
      ? `<div class="item-list">${state.safeguards.map(renderSafeguardRule).join("")}</div>`
      : '<div class="empty-state"><strong>반복된 실패 규칙이 없습니다.</strong><span>같은 형태의 검증된 실패가 3회 반복되면 검토 가능한 규칙이 나타납니다.</span></div>';
    document.getElementById("safeguards").innerHTML = (state.surfaceErrors.safeguards ? surfaceError(state.surfaceErrors.safeguards, "diagnostics") : "") + safeguardContent;
  }

  function renderSafeguardRule(item) {
    const spec = item.spec;
    const metrics = spec.metrics || {};
    const feedbackCount = (metrics.positiveFeedback || 0) + (metrics.falsePositives || 0);
    const canFeedback = ["shadow", "active"].includes(spec.state) && (metrics.hits || 0) > feedbackCount;
    const canActivate = spec.state === "shadow" && (metrics.hits || 0) > 0 && (metrics.positiveFeedback || 0) > 0 && (metrics.falsePositives || 0) === 0;
    const activationHint = spec.state === "shadow" ? `<p class="next">활성화 조건: 정확히 일치 1회 이상, 유효한 방지 1회 이상, 오탐 0회. 현재 ${escapeHTML(metrics.hits || 0)} / ${escapeHTML(metrics.positiveFeedback || 0)} / ${escapeHTML(metrics.falsePositives || 0)}</p>` : "";
    const scope = [spec.projectId, spec.repositoryId, spec.worktreeId].filter(Boolean).join(" / ") || "전체 로컬 범위";
    let controls = "";
    if (spec.state === "proposal") {
      controls = `<input class="owner-input" data-safeguard-owner aria-label="재발 방지 규칙 담당자" placeholder="담당자 이름"><button class="button primary small" type="button" data-safeguard="shadow" data-owner-submit data-id="${escapeHTML(item.metadata.id)}" disabled>모의 적용 시작</button><button class="button small" type="button" data-safeguard="retire" data-id="${escapeHTML(item.metadata.id)}">제안 폐기</button>`;
    } else if (spec.state === "shadow") {
      controls = `<button class="button small" type="button" data-safeguard="positive" data-id="${escapeHTML(item.metadata.id)}" ${canFeedback ? "" : "disabled"}>유효한 방지</button><button class="button small" type="button" data-safeguard="false_positive" data-id="${escapeHTML(item.metadata.id)}" ${canFeedback ? "" : "disabled"}>오탐</button><button class="button primary small" type="button" data-safeguard="activate" data-id="${escapeHTML(item.metadata.id)}" ${canActivate ? "" : "disabled"}>활성화</button><button class="button small" type="button" data-safeguard="retire" data-id="${escapeHTML(item.metadata.id)}">사용 종료</button>`;
    } else if (spec.state === "active") {
      controls = `<button class="button small" type="button" data-safeguard="positive" data-id="${escapeHTML(item.metadata.id)}" ${canFeedback ? "" : "disabled"}>유효한 방지</button><button class="button small" type="button" data-safeguard="false_positive" data-id="${escapeHTML(item.metadata.id)}" ${canFeedback ? "" : "disabled"}>오탐</button><button class="button small" type="button" data-safeguard="rollback" data-id="${escapeHTML(item.metadata.id)}">모의 적용으로 되돌리기</button><button class="button small" type="button" data-safeguard="retire" data-id="${escapeHTML(item.metadata.id)}">사용 종료</button>`;
    }
    const stateClass = spec.state === "active" ? "ok" : spec.state === "retired" ? "" : "warn";
    return `<article class="list-item"><div class="list-item-header"><div><h3>${escapeHTML(spec.category)}</h3><p class="meta">${escapeHTML(scope)} · ${escapeHTML(spec.occurrenceCount)}회 반복</p></div><span class="chip ${stateClass}">${escapeHTML(label(spec.state))}</span></div><p>출력 내용이 아니라 실패 종류·상태·종료 코드가 정확히 같은 경우만 일치로 계산합니다.</p><dl class="detail-grid"><div><dt>담당자</dt><dd>${escapeHTML(spec.owner || "미지정")}</dd></div><div><dt>마지막 발생</dt><dd>${escapeHTML(formatDate(spec.lastSeen))}</dd></div><div><dt>활성 승인</dt><dd>${escapeHTML(spec.activationApprovedBy || "아직 없음")}</dd></div><div><dt>평가</dt><dd>${escapeHTML(metrics.evaluations || 0)}회</dd></div><div><dt>일치 / 불일치</dt><dd>${escapeHTML(metrics.hits || 0)} / ${escapeHTML(metrics.misses || 0)}</dd></div><div><dt>유효한 방지 / 오탐</dt><dd>${escapeHTML(metrics.positiveFeedback || 0)} / ${escapeHTML(metrics.falsePositives || 0)}</dd></div><div><dt>평가 비용</dt><dd>로컬 비교 ${escapeHTML(metrics.evaluationCostUnits || 0)}회</dd></div><div class="wide"><dt>Fingerprint</dt><dd><code>${escapeHTML(spec.fingerprint)}</code></dd></div></dl>${activationHint}${controls ? `<div class="item-actions">${controls}</div>` : ""}</article>`;
  }

  function renderActivity() {
    document.getElementById("events").innerHTML = state.events.length
      ? `<div class="table-wrap" tabindex="0" role="region" aria-label="활동 기록 표. 좌우로 스크롤할 수 있습니다." aria-describedby="activity-table-scroll-hint"><p id="activity-table-scroll-hint" class="table-scroll-hint">좌우로 밀어 표 전체를 확인하세요.</p><table><caption>활동 기록</caption><thead><tr><th scope="col">시각</th><th scope="col">유형</th><th scope="col">내용</th><th scope="col">범위</th></tr></thead><tbody>${state.events.slice().reverse().map(item => `<tr><td>${escapeHTML(formatDate(item.spec.occurredAt))}</td><td><code>${escapeHTML(item.spec.type)}</code></td><td>${escapeHTML(localize(item.spec.summary))}</td><td>${escapeHTML([item.spec.projectId, item.spec.repositoryId].filter(Boolean).join(" / ") || "전체")}</td></tr>`).join("")}</tbody></table></div>`
      : '<div class="empty-state"><strong>아직 활동 기록이 없습니다.</strong><span>점검이나 등록 변경을 실행하면 감사 기록이 남습니다.</span></div>';
  }

  function renderServiceRecovery() {
    const banner = document.getElementById("service-recovery");
    if (!banner) return;
    const service = state.service || { status: "loading", message: "로컬 서비스에 연결하는 중입니다…", error: "" };
    banner.hidden = service.status === "ready";
    banner.className = `service-recovery service-recovery--${escapeHTML(service.status || "loading")}`;
    if (service.status === "loading") {
      banner.innerHTML = '<strong>로컬 서비스에 연결하는 중입니다.</strong><span>저장된 화면은 유지하고 새 요청은 잠시 막습니다.</span>';
      return;
    }
    if (service.status === "offline" || service.status === "error") {
      banner.innerHTML = `<strong>오프라인 — 새 상태와 코드 검사를 요청할 수 없습니다.</strong><span>마지막으로 불러온 결과만 표시합니다. 연결을 확인한 뒤 다시 시도하세요.</span><button class="button small" type="button" data-service-refresh>다시 연결</button>`;
      return;
    }
    banner.innerHTML = `<strong>일부 데이터를 새로 읽지 못했습니다.</strong><span>이 화면의 값은 마지막으로 불러온 기록일 수 있습니다. ${escapeHTML(service.error || "연결을 확인한 뒤 다시 시도하세요.")}</span><button class="button small" type="button" data-service-refresh>다시 연결</button>`;
  }

  function renderAll() {
    renderServiceRecovery();
    renderHome();
    renderGuide();
    renderProjects();
    renderWork();
    renderAssuranceDashboard();
    renderDiagnostics();
    renderActivity();
    focusPendingFinding();
  }

  async function loadSurface(key, loader, assign) {
    try {
      assign(await loader());
      state.surfaceErrors[key] = "";
    } catch (error) {
      state.surfaceErrors[key] = error.message;
    }
  }

  async function loadWorkData(force) {
    if (state.loading.work || (state.loaded.work && !force)) return;
    state.loading.work = true;
    const repositories = projectRepositories();
    await Promise.all([
      loadSurface("checksets", async () => Promise.all(repositories.map(async repository => {
        const [checksets, proposals] = await Promise.all([
          request(`/api/projects/${encode(repository.projectID)}/repositories/${encode(repository.id)}/checksets`),
          request(`/api/projects/${encode(repository.projectID)}/repositories/${encode(repository.id)}/proposals`),
        ]);
        return { repository, checksets, proposals };
      })), items => { state.workItems = items; }),
      loadSurface("actions", async () => {
        const plans = await request("/api/actions/plans");
        return Promise.all(plans.map(async plan => {
          const [status, runs] = await Promise.all([
            request(`/api/actions/plans/${encode(plan.metadata.id)}`),
            request(`/api/actions/plans/${encode(plan.metadata.id)}/runs`),
          ]);
          return { plan, status, runs };
        }));
      }, details => { state.actionDetails = details; }),
      loadSurface("externalGroups", () => request("/api/external-work-groups"), items => { state.externalGroups = items; }),
    ]);
    await Promise.all(state.actionDetails.filter(isSpecialPlan).map(async detail => {
      const id = encode(detail.plan.metadata.id);
      const actionType = detail.plan.spec.actionType;
      const path = actionType === "cleanup.destructive"
        ? `/api/cleanup/plans/${id}/result`
        : actionType.startsWith("release.")
          ? `/api/release-plans/${id}/result`
          : `/api/external-work-plans/${id}/result`;
      try {
        state.operationResults[detail.plan.metadata.id] = await request(path);
      } catch (_) {
        delete state.operationResults[detail.plan.metadata.id];
      }
    }));
    state.loading.work = false;
    state.loaded.work = true;
    renderWork();
  }

  async function loadDiagnosticsData(force) {
    if (state.loading.diagnostics || (state.loaded.diagnostics && !force)) return;
    state.loading.diagnostics = true;
    await Promise.all([
      loadQualityTools(force),
      loadSurface("cleanup", () => request("/api/cleanup/candidates"), items => { state.cleanup = items; }),
      loadSurface("safeguards", () => request("/api/safeguards/rules"), items => { state.safeguards = items; }),
      loadSurface("profiles", () => request("/api/agent-profiles"), items => { state.profiles = items; }),
      loadSurface("integrations", () => request("/api/integrations"), items => { state.integrations = items; }),
      loadSurface("externalGroups", () => request("/api/external-work-groups"), items => { state.externalGroups = items; }),
      loadSurface("runbooks", () => request("/api/runbooks"), items => { state.runbooks = items; }),
    ]);
    state.loading.diagnostics = false;
    state.loaded.diagnostics = true;
    renderDiagnostics();
  }

  async function loadAssuranceProductData() {
    const [impactResult, storageResult] = await Promise.allSettled([
      request(assuranceImpactPath()),
      request(assuranceStoragePath()),
    ]);
    if (impactResult.status === "fulfilled") {
      state.assuranceImpact = impactResult.value || null;
      state.assuranceImpactError = "";
    } else {
      state.assuranceImpact = null;
      state.assuranceImpactError = impactResult.reason?.message || "잠시 후 다시 시도하세요.";
    }
    if (storageResult.status === "fulfilled") {
      state.assuranceStorage = storageResult.value || null;
      state.assuranceStorageError = "";
    } else {
      state.assuranceStorage = null;
      state.assuranceStorageError = storageResult.reason?.message || "잠시 후 다시 시도하세요.";
    }
    renderHome();
    renderAssuranceDashboard();
  }

  let loadingQualityInspection = false;
  let qualityInspectionTargetRequestSequence = 0;
  const qualityInspectionTargetRequestIsCurrent = (targetValue, sequence) =>
    sequence === qualityInspectionTargetRequestSequence && (selectedTarget()?.value || "") === targetValue;
  const ensureQualityInspectionState = () => {
    if (!state.qualityInspection) state.qualityInspection = {};
    const workflow = state.qualityInspection;
    if (!Array.isArray(workflow.plans)) workflow.plans = [];
    if (!Array.isArray(workflow.scores)) workflow.scores = [];
    if (!Array.isArray(workflow.proposals)) workflow.proposals = [];
    if (typeof workflow.aiEnabled !== "boolean") workflow.aiEnabled = false;
    workflow.comparison ||= { status: "idle", data: null, error: "", beforeID: "", afterID: "" };
    workflow.toolPreview ||= { status: "idle", data: null, error: "", input: null };
    workflow.toolActionPlan ||= { status: "idle", data: null, error: "", approvalStatus: "idle", executionStatus: "idle" };
    workflow.mutation ||= { kind: "", status: "idle", error: "" };
    workflow.proposalMutation ||= { kind: "", status: "idle", error: "" };
    workflow.proposalStatus ||= "idle";
    workflow.proposalError ||= "";
    return workflow;
  };
  const qualityInspectionScopeInput = target => target ? ({ projectId: target.projectID, repositoryId: target.repositoryID, worktreeId: target.worktreeID }) : null;
  const qualityInspectionReplacePlan = plan => {
    const workflow = ensureQualityInspectionState();
    const id = plan?.metadata?.id || "";
    if (!id) return;
    workflow.plans = [...workflow.plans.filter(item => item?.metadata?.id !== id), plan];
    workflow.selectedPlanID = id;
    workflow.selectedPlan = plan;
  };
  const qualityInspectionReplaceProposal = proposal => {
    const workflow = ensureQualityInspectionState();
    const id = proposal?.metadata?.id || "";
    if (!id) return;
    workflow.proposals = [...workflow.proposals.filter(item => item?.metadata?.id !== id), proposal];
    workflow.selectedProposalID = id;
    workflow.selectedProposal = proposal;
  };
  async function loadQualityInspectionPlan(id, render = true, isCurrent = null) {
    if (!id) return null;
    const plan = await request(`/api/quality/inspection-plans/${encode(id)}`);
    if (isCurrent && !isCurrent()) return null;
    qualityInspectionReplacePlan(plan);
    if (render) renderAssuranceDashboard();
    return plan;
  }
  async function loadQualityInspectionProposal(id, render = true, isCurrent = null) {
    if (!id) return null;
    const proposal = await request(`/api/quality/improvement-proposals/${encode(id)}`);
    if (isCurrent && !isCurrent()) return null;
    qualityInspectionReplaceProposal(proposal);
    if (render) renderAssuranceDashboard();
    return proposal;
  }
  async function loadQualityInspectionProposals(render = false, targetOverride = null, isCurrent = null) {
    const workflow = ensureQualityInspectionState();
    const target = targetOverride || selectedTarget();
    if (isCurrent && !isCurrent()) return;
    workflow.proposalStatus = "loading";
    workflow.proposalError = "";
    try {
      const items = await request("/api/quality/improvement-proposals");
      if (isCurrent && !isCurrent()) return;
      const proposals = Array.isArray(items) ? items : [];
      const selected = workflow.selectedProposalID
        ? proposals.find(item => item?.metadata?.id === workflow.selectedProposalID)
        : proposals
          .filter(item => qualityImprovementScopeMatches(item, target))
          .sort((left, right) => qualityInspectionUpdatedAt(right) - qualityInspectionUpdatedAt(left))[0];
      let selectedProposal = selected || null;
      if (selected?.metadata?.id) {
        try {
          selectedProposal = await request(`/api/quality/improvement-proposals/${encode(selected.metadata.id)}`);
        } catch (error) {
          if (!isCurrent || isCurrent()) workflow.proposalError = error.message || "선택한 개선 제안 상세를 불러오지 못했습니다.";
        }
      }
      if (isCurrent && !isCurrent()) return;
      workflow.proposals = proposals;
      workflow.selectedProposalID = selectedProposal?.metadata?.id || "";
      workflow.selectedProposal = selectedProposal;
      workflow.proposalStatus = "ready";
    } catch (error) {
      if (isCurrent && !isCurrent()) return;
      workflow.proposalStatus = "error";
      workflow.proposalError = error.message || "개선 제안 목록을 불러오지 못했습니다.";
    }
    if (render) renderAssuranceDashboard();
  }
  async function loadQualityInspectionTargetData(targetValue, render = true) {
    const workflow = ensureQualityInspectionState();
    const sequence = ++qualityInspectionTargetRequestSequence;
    const isCurrent = () => qualityInspectionTargetRequestIsCurrent(targetValue, sequence);
    const target = targetOptions().find(item => item.value === targetValue) || null;
    if (!target) return;
    const latestRunPath = `/api/quality/inspection-runs/latest?projectId=${encode(target.projectID)}&repositoryId=${encode(target.repositoryID)}&worktreeId=${encode(target.worktreeID)}`;
    workflow.status = "ready";
    workflow.error = "";
    try {
      let latestRun = null;
      try {
        latestRun = await request(latestRunPath);
      } catch (error) {
        if (error?.status !== 404) throw error;
      }
      if (!isCurrent()) return;
      workflow.lastRun = latestRun || null;
      if (latestRun?.score?.metadata?.id && !workflow.scores.some(item => item?.metadata?.id === latestRun.score.metadata.id)) {
        workflow.scores = [...workflow.scores, latestRun.score];
      }
      if (latestRun?.plan?.metadata?.id && !workflow.plans.some(item => item?.metadata?.id === latestRun.plan.metadata.id)) {
        workflow.plans = [...workflow.plans, latestRun.plan];
      }
      const plan = latestRun?.plan || qualityInspectionPlansForTarget(target)[0] || null;
      workflow.selectedPlanID = plan?.metadata?.id || "";
      workflow.selectedPlan = plan;
      if (plan?.metadata?.id) {
        try {
          await loadQualityInspectionPlan(plan.metadata.id, false, isCurrent);
        } catch (error) {
          if (isCurrent()) workflow.error = `선택한 계획 상세를 불러오지 못했습니다. ${error.message || "다시 시도하세요."}`;
        }
      }
      await loadQualityInspectionProposals(false, target, isCurrent);
    } catch (error) {
      if (!isCurrent()) return;
      workflow.error = error.message || "선택한 Worktree의 최근 검사 결과를 불러오지 못했습니다.";
    } finally {
      if (isCurrent() && render) renderAssuranceDashboard();
    }
  }
  async function loadQualityInspectionData(force = false) {
    const workflow = ensureQualityInspectionState();
    if (loadingQualityInspection || (!force && workflow.status === "ready")) return;
    const target = selectedTarget();
    const targetValue = target?.value || "";
    const sequence = ++qualityInspectionTargetRequestSequence;
    const isCurrent = () => qualityInspectionTargetRequestIsCurrent(targetValue, sequence);
    loadingQualityInspection = true;
    const previous = { ...workflow };
    workflow.status = "loading";
    workflow.error = "";
    renderAssuranceDashboard();
    try {
		const latestRunPath = target ? `/api/quality/inspection-runs/latest?projectId=${encode(target.projectID)}&repositoryId=${encode(target.repositoryID)}&worktreeId=${encode(target.worktreeID)}` : "";
		const [plansResult, scoresResult, latestRunResult] = await Promise.allSettled([
        request("/api/quality/inspection-plans"),
        request("/api/quality/scores"),
			latestRunPath ? request(latestRunPath) : Promise.resolve(null),
      ]);
      const errors = [];
      if (plansResult.status === "fulfilled") workflow.plans = Array.isArray(plansResult.value) ? plansResult.value : [];
      else errors.push(plansResult.reason?.message || "검사 계획");
      if (scoresResult.status === "fulfilled") workflow.scores = Array.isArray(scoresResult.value) ? scoresResult.value : [];
      else errors.push(scoresResult.reason?.message || "품질 점수");
		if (isCurrent() && latestRunResult.status === "fulfilled" && latestRunResult.value) {
			workflow.lastRun = latestRunResult.value;
			if (latestRunResult.value.score?.metadata?.id && !workflow.scores.some(item => item?.metadata?.id === latestRunResult.value.score.metadata.id)) workflow.scores = [...workflow.scores, latestRunResult.value.score];
			if (latestRunResult.value.plan?.metadata?.id && !workflow.plans.some(item => item?.metadata?.id === latestRunResult.value.plan.metadata.id)) workflow.plans = [...workflow.plans, latestRunResult.value.plan];
		} else if (isCurrent() && latestRunResult.status === "rejected" && latestRunResult.reason?.status !== 404) {
			errors.push(latestRunResult.reason?.message || "최근 검사 결과");
		}
      if (plansResult.status === "rejected" && scoresResult.status === "rejected") {
        workflow.status = "error";
        workflow.error = errors.join(" · ");
        return;
      }
      workflow.status = "ready";
      workflow.error = errors.length ? "계획 또는 점수 중 일부 기록을 새로 읽지 못했습니다." : "";
		if (!isCurrent()) return;
		const plan = workflow.lastRun?.plan?.metadata?.id
			? workflow.plans.find(item => item?.metadata?.id === workflow.lastRun.plan.metadata.id) || workflow.lastRun.plan
			: qualityInspectionPlansForTarget(target)[0] || null;
      workflow.selectedPlanID = plan?.metadata?.id || "";
      workflow.selectedPlan = plan;
      if (plan?.metadata?.id) {
        try {
          await loadQualityInspectionPlan(plan.metadata.id, false, isCurrent);
        } catch (error) {
          workflow.error = workflow.error || `선택한 계획 상세를 불러오지 못했습니다. ${error.message || "다시 시도하세요."}`;
        }
      }
      await loadQualityInspectionProposals(false, target, isCurrent);
    } catch (error) {
      if (!isCurrent()) return;
      workflow.status = "error";
      workflow.error = error.message || "검사 계획과 점수를 불러오지 못했습니다.";
      if (previous.status === "ready") {
        workflow.plans = previous.plans || [];
        workflow.scores = previous.scores || [];
      }
    } finally {
      loadingQualityInspection = false;
      renderAssuranceDashboard();
    }
  }
  const qualityInspectionMutation = (kind, status, error = "") => {
    ensureQualityInspectionState().mutation = { kind, status, error };
    renderAssuranceDashboard();
  };
  async function generateQualityInspectionPlan() {
    const target = selectedTarget();
    if (!target) return null;
    const workflow = ensureQualityInspectionState();
    if (serviceBlocksMutation()) {
      qualityInspectionMutation("generate", "error", "연결이 끊긴 상태에서는 새 검사 계획을 만들 수 없습니다.");
      return null;
    }
    workflow.aiEnabled = document.querySelector("[data-quality-inspection-ai]")?.checked ?? workflow.aiEnabled === true;
    qualityInspectionMutation("generate", "submitting");
    try {
      const body = { ...qualityInspectionScopeInput(target) };
      if (workflow.aiEnabled === true) body.ai = { enabled: true, provider: "codex", profileId: "codex", requestedModel: "" };
      const plan = await request("/api/quality/inspection-plans/generate", { method: "POST", headers: mutationHeaders(), body: JSON.stringify(body) });
      qualityInspectionReplacePlan(plan);
      await loadQualityInspectionPlan(plan?.metadata?.id, false);
      qualityInspectionMutation("", "idle");
      showNotice(plan?.generation?.aiProposal === true ? "AI가 제안한 검사 항목을 포함한 계획을 불러왔습니다. 내용을 검토하세요." : workflow.aiEnabled ? "AI 제안에 연결되지 않아 결정적 fallback 계획을 불러왔습니다. 내용을 검토하세요." : "결정적으로 만든 검사 계획을 불러왔습니다. 내용을 검토하세요.");
      return plan;
    } catch (error) {
      qualityInspectionMutation("generate", "error", error.message || "검사 계획을 만들지 못했습니다.");
      return null;
    } finally {
      renderAssuranceDashboard();
    }
  }
  async function reviewQualityInspectionPlan(decision, button) {
    const workflow = ensureQualityInspectionState();
    const plan = workflow.selectedPlan;
    const id = button?.dataset.id || plan?.metadata?.id || "";
    const spec = qualityInspectionPlanSpec(plan);
    const expectedRevision = Number(spec.revision);
    if (!id || !Number.isInteger(expectedRevision) || expectedRevision < 1) {
      qualityInspectionMutation("review", "error", "최신 검사 계획을 불러온 뒤 다시 시도하세요.");
      return;
    }
    qualityInspectionMutation("review", "submitting");
    try {
      const updated = await request(`/api/quality/inspection-plans/${encode(id)}/review`, {
        method: "POST", headers: mutationHeaders(), body: JSON.stringify({ expectedRevision, decision }),
      });
      qualityInspectionReplacePlan(updated);
      await loadQualityInspectionPlan(id, false);
      qualityInspectionMutation("", "idle");
      showNotice(decision === "reject" ? "검사 계획을 거절했습니다." : decision === "review" ? "검사 계획을 검토 완료로 표시했습니다." : "검사 계획을 승인했습니다.");
    } catch (error) {
      qualityInspectionMutation("review", "error", error.message || "검사 계획 상태를 바꾸지 못했습니다.");
    }
    renderAssuranceDashboard();
  }
  async function runQualityInspectionPlan(button) {
    const workflow = ensureQualityInspectionState();
    const plan = workflow.selectedPlan;
    const id = button?.dataset.id || plan?.metadata?.id || "";
    const expectedRevision = Number(plan?.spec?.revision);
    if (!id || !Number.isInteger(expectedRevision) || expectedRevision < 1) {
      qualityInspectionMutation("run", "error", "최신 승인 계획을 불러온 뒤 다시 시도하세요.");
      return;
    }
    qualityInspectionMutation("run", "submitting");
    try {
      const result = await request(`/api/quality/inspection-plans/${encode(id)}/run`, {
        method: "POST", headers: mutationHeaders(), body: JSON.stringify({ expectedRevision }),
      });
      workflow.lastRun = result || null;
      if (result?.plan) qualityInspectionReplacePlan({ ...result.plan, generation: plan.generation });
      if (result?.score?.metadata?.id) workflow.scores = [...workflow.scores.filter(item => item.metadata?.id !== result.score.metadata.id), result.score];
      if (result?.score?.metadata?.id) {
        try {
          const score = await request(`/api/quality/scores/${encode(result.score.metadata.id)}`);
          workflow.scores = [...workflow.scores.filter(item => item.metadata?.id !== score.metadata?.id), score];
        } catch (_) { /* the run response already contains the score */ }
      }
      await loadQualityInspectionPlan(id, false);
      qualityInspectionMutation("", "idle");
      showNotice("검사 결과와 결정적 품질 점수를 기록했습니다.");
    } catch (error) {
      qualityInspectionMutation("run", "error", error.message || "검사를 실행하지 못했습니다. 실행기 상태를 확인하세요.");
    }
    renderAssuranceDashboard();
  }
  async function compareQualityInspectionScores() {
    const workflow = ensureQualityInspectionState();
    const beforeID = document.querySelector("[data-quality-inspection-before]")?.value || workflow.comparison.beforeID || "";
    const afterID = document.querySelector("[data-quality-inspection-after]")?.value || workflow.comparison.afterID || "";
    workflow.comparison = { status: "loading", data: null, error: "", beforeID, afterID };
    renderAssuranceDashboard();
    if (!beforeID || !afterID || beforeID === afterID) {
      workflow.comparison = { status: "error", data: null, error: "서로 다른 이전·이후 점수를 선택하세요.", beforeID, afterID };
      renderAssuranceDashboard();
      return;
    }
    try {
      const [before, after] = await Promise.all([
        request(`/api/quality/scores/${encode(beforeID)}`),
        request(`/api/quality/scores/${encode(afterID)}`),
      ]);
      const comparison = await request(`/api/quality/comparisons?beforeId=${encode(before?.metadata?.id || beforeID)}&afterId=${encode(after?.metadata?.id || afterID)}`);
      workflow.comparison = { status: "ready", data: comparison, error: "", beforeID, afterID };
    } catch (error) {
      workflow.comparison = { status: "error", data: null, error: error.message || "비교 결과를 불러오지 못했습니다.", beforeID, afterID };
    }
    renderAssuranceDashboard();
  }
  const qualityInspectionProposalMutation = (kind, status, error = "") => {
    ensureQualityInspectionState().proposalMutation = { kind, status, error };
    renderAssuranceDashboard();
  };
  async function generateQualityImprovementProposal() {
    const workflow = ensureQualityInspectionState();
    const run = workflow.lastRun;
    const plan = run?.plan || workflow.selectedPlan;
    const planID = plan?.metadata?.id || workflow.selectedPlanID || "";
    const resultArtifactID = qualityInspectionLastRunArtifactID(run);
    if (!planID || !resultArtifactID) {
      qualityInspectionProposalMutation("generate", "error", "최근 실행 결과 artifact와 검사 계획이 있어야 개선 제안을 만들 수 있습니다.");
      return null;
    }
    if (serviceBlocksMutation()) {
      qualityInspectionProposalMutation("generate", "error", "연결이 끊긴 상태에서는 개선 제안을 만들 수 없습니다.");
      return null;
    }
    qualityInspectionProposalMutation("generate", "submitting");
    try {
      const proposal = await request("/api/quality/improvement-proposals/generate", {
        method: "POST", headers: mutationHeaders(), body: JSON.stringify({ planId: planID, resultArtifactId: resultArtifactID }),
      });
      qualityInspectionReplaceProposal(proposal);
      await loadQualityInspectionProposal(proposal?.metadata?.id, false);
      workflow.proposalStatus = "ready";
      qualityInspectionProposalMutation("", "idle");
      showNotice("최근 실행 결과에서 개선 제안을 만들었습니다. enum 변경을 검토하세요.");
      return proposal;
    } catch (error) {
      qualityInspectionProposalMutation("generate", "error", error.message || "개선 제안을 만들지 못했습니다.");
      return null;
    } finally {
      renderAssuranceDashboard();
    }
  }
  async function reviewQualityImprovementProposal(decision, button) {
    const workflow = ensureQualityInspectionState();
    const proposal = workflow.selectedProposal;
    const id = button?.dataset.id || proposal?.metadata?.id || "";
    const expectedRevision = Number(proposal?.spec?.revision);
    if (!id || !Number.isInteger(expectedRevision) || expectedRevision < 1) {
      qualityInspectionProposalMutation("review", "error", "최신 개선 제안을 불러온 뒤 다시 시도하세요.");
      return;
    }
    qualityInspectionProposalMutation("review", "submitting");
    try {
      const updated = await request(`/api/quality/improvement-proposals/${encode(id)}/review`, {
        method: "POST", headers: mutationHeaders(), body: JSON.stringify({ expectedRevision, decision }),
      });
      qualityInspectionReplaceProposal(updated);
      await loadQualityInspectionProposal(id, false);
      qualityInspectionProposalMutation("", "idle");
      showNotice(decision === "reject" ? "개선 제안을 거절했습니다." : decision === "review" ? "개선 제안을 검토 완료로 표시했습니다." : "개선 제안을 승인했습니다.");
    } catch (error) {
      qualityInspectionProposalMutation("review", "error", error.message || "개선 제안 상태를 바꾸지 못했습니다.");
    }
    renderAssuranceDashboard();
  }
  async function applyQualityImprovementProposal(button) {
    const workflow = ensureQualityInspectionState();
    const proposal = workflow.selectedProposal;
    const id = button?.dataset.id || proposal?.metadata?.id || "";
    const expectedRevision = Number(proposal?.spec?.revision);
    if (!id || !Number.isInteger(expectedRevision) || expectedRevision < 1) {
      qualityInspectionProposalMutation("apply", "error", "최신 승인된 개선 제안을 불러온 뒤 다시 시도하세요.");
      return;
    }
    qualityInspectionProposalMutation("apply", "submitting");
    try {
      const result = await request(`/api/quality/improvement-proposals/${encode(id)}/apply`, {
        method: "POST", headers: mutationHeaders(), body: JSON.stringify({ expectedRevision }),
      });
      const updatedProposal = result?.proposal || result;
      if (updatedProposal?.metadata?.id) qualityInspectionReplaceProposal(updatedProposal);
      if (result?.plan) {
        qualityInspectionReplacePlan(result.plan);
        await loadQualityInspectionPlan(result.plan?.metadata?.id, false);
      }
      await loadQualityInspectionProposal(id, false);
      qualityInspectionProposalMutation("", "idle");
      showNotice("승인한 enum 변경을 적용했습니다. 새 검사 계획은 다시 검토·승인해야 합니다.");
    } catch (error) {
      qualityInspectionProposalMutation("apply", "error", error.message || "개선 제안을 적용하지 못했습니다.");
    }
    renderAssuranceDashboard();
  }
  async function requestQualityToolInstallActionPlan(input) {
    const workflow = ensureQualityInspectionState();
    workflow.toolActionPlan = { status: "loading", data: null, error: "", approvalStatus: "idle", executionStatus: "idle" };
    renderAssuranceDashboard();
    try {
      const data = await request("/api/quality/tool-installs/action-plan", {
        method: "POST", headers: mutationHeaders(), body: JSON.stringify(input),
      });
      workflow.toolActionPlan = { status: "ready", data, error: "", approvalStatus: "idle", executionStatus: "idle" };
      return data;
    } catch (error) {
      workflow.toolActionPlan = { status: "error", data: null, error: error.message || "설치 전용 Action Plan을 만들지 못했습니다.", approvalStatus: "idle", executionStatus: "idle" };
      return null;
    } finally {
      renderAssuranceDashboard();
    }
  }
  async function approveQualityToolInstallActionPlan(button) {
    const workflow = ensureQualityInspectionState();
    const id = button?.dataset.id || qualityToolInstallActionPlanID(workflow.toolActionPlan?.data);
    if (!id) return;
	workflow.toolActionPlan.approvalStatus = "loading";
    workflow.toolActionPlan.error = "";
    renderAssuranceDashboard();
	try {
		const response = await request(`/ui/actions/plans/${encode(id)}/approval`, { method: "POST", headers: mutationHeaders(), body: "" });
		const decision = response?.decision;
		if (decision === "granted") {
			workflow.toolActionPlan.approvalStatus = "approved";
			showNotice("설치 Action Plan의 사람 승인을 기록했습니다. 실행 전 고정 command를 다시 확인하세요.");
		} else if (decision === "rejected" || decision === "cancelled") {
			workflow.toolActionPlan.approvalStatus = decision;
			workflow.toolActionPlan.error = decision === "rejected" ? "사람이 승인을 거절했습니다." : "사람이 승인 절차를 취소했습니다.";
		} else {
			workflow.toolActionPlan.approvalStatus = "idle";
			workflow.toolActionPlan.error = "승인 endpoint가 유효한 decision을 반환하지 않았습니다. 실행하지 않습니다.";
		}
	} catch (error) {
      workflow.toolActionPlan.approvalStatus = "idle";
      workflow.toolActionPlan.error = error.message || "설치 Action Plan을 승인하지 못했습니다.";
    }
    renderAssuranceDashboard();
  }
  async function executeQualityToolInstallActionPlan(button) {
	const workflow = ensureQualityInspectionState();
	const id = button?.dataset.id || qualityToolInstallActionPlanID(workflow.toolActionPlan?.data);
	if (!id) return;
	if (workflow.toolActionPlan?.approvalStatus !== "approved") {
		workflow.toolActionPlan.error = "사람 승인 결과가 granted일 때만 설치 Action을 실행할 수 있습니다.";
		renderAssuranceDashboard();
		return;
	}
    workflow.toolActionPlan.executionStatus = "loading";
    workflow.toolActionPlan.error = "";
    renderAssuranceDashboard();
    try {
      const result = await request(`/api/actions/plans/${encode(id)}/execute`, {
        method: "POST", headers: mutationHeaders(), body: JSON.stringify({ holder: "ui", idempotencyKey: `quality-tool-install-${Date.now()}` }),
      });
      workflow.toolActionPlan.executionStatus = "executed";
      workflow.toolActionPlan.executionResult = result || null;
      showNotice("설치 Action을 실행 요청했습니다. 실제 완료 여부는 Action 결과에서 확인하세요.");
    } catch (error) {
      workflow.toolActionPlan.executionStatus = "idle";
      workflow.toolActionPlan.error = error.message || "설치 Action을 실행하지 못했습니다.";
    }
    renderAssuranceDashboard();
  }
  async function requestQualityToolInstallPreview(form) {
    const workflow = ensureQualityInspectionState();
    const target = selectedTarget();
    const values = Object.fromEntries(new FormData(form).entries());
    const affectedFiles = String(values.affectedFiles || "").split(/[\n,]/).map(value => value.trim()).filter(Boolean);
    if (!target || !String(values.version || "").trim() || !affectedFiles.length) {
      workflow.toolPreview = { status: "error", data: null, error: "Worktree, 정확한 버전, 영향을 받는 파일을 모두 입력하세요." };
      renderAssuranceDashboard();
      return;
    }
	const input = { ...qualityInspectionScopeInput(target), kind: values.kind, version: String(values.version).trim(), affectedFiles, allowGlobal: values.allowGlobal === "on" };
    workflow.toolPreview = { status: "loading", data: null, error: "", input };
    workflow.toolActionPlan = { status: "idle", data: null, error: "", approvalStatus: "idle", executionStatus: "idle" };
    renderAssuranceDashboard();
    try {
      const data = await request("/api/quality/tool-installs/plan", {
        method: "POST", headers: mutationHeaders(), body: JSON.stringify(input),
      });
      workflow.toolPreview = { status: "ready", data, error: "", input };
      if (data?.available && data?.action) await requestQualityToolInstallActionPlan(input);
    } catch (error) {
      workflow.toolPreview = { status: "error", data: null, error: error.message || "설치 계획 미리보기를 만들지 못했습니다.", input };
    }
    renderAssuranceDashboard();
  }

  async function loadAssuranceMeasurementData() {
    const previous = state.assuranceMeasurement?.data || null;
    state.assuranceMeasurement = { status: "loading", data: previous, error: "" };
    renderAssuranceMeasurementDashboard();
    try {
      state.assuranceMeasurement = { status: "ready", data: await request("/api/assurance/measurement-runs/dashboard"), error: "" };
    } catch (error) {
      state.assuranceMeasurement = { status: "error", data: previous, error: error.message || "잠시 후 다시 시도하세요." };
    }
    renderAssuranceDashboard();
  }

  let loadingQualityCampaigns = false;
  async function loadQualityCampaigns(force) {
    if (loadingQualityCampaigns || (!force && state.qualityCampaignsStatus === "ready")) return;
    loadingQualityCampaigns = true;
    state.qualityCampaignsStatus = "loading";
    state.qualityCampaignsError = "";
    renderQualityWorkSurface();
    try {
      const campaigns = await request("/api/assurance/campaigns");
      state.qualityCampaigns = Array.isArray(campaigns) ? campaigns : [];
      state.qualityCampaignsStatus = "ready";
    } catch (error) {
      state.qualityCampaignsStatus = "error";
      state.qualityCampaignsError = error.message || "코드 검사 설정을 불러오지 못했습니다.";
      if (state.service.status === "ready") state.service = { status: "degraded", message: "코드 검사 설정을 새로 읽지 못했습니다.", error: state.qualityCampaignsError };
    } finally {
      loadingQualityCampaigns = false;
      renderQualityWorkSurface();
    }
  }

  async function refreshQualityResultData() {
    const [campaignResult, runsResult] = await Promise.allSettled([
      request("/api/assurance/campaigns"),
      request("/api/assurance/runs"),
    ]);
    if (campaignResult.status === "fulfilled") {
      state.qualityCampaigns = Array.isArray(campaignResult.value) ? campaignResult.value : [];
      state.qualityCampaignsStatus = "ready";
      state.qualityCampaignsError = "";
    } else {
      state.qualityCampaignsStatus = "error";
      state.qualityCampaignsError = campaignResult.reason?.message || "코드 검사 설정을 불러오지 못했습니다.";
      if (state.service.status === "ready") state.service = { status: "degraded", message: "코드 검사 설정을 새로 읽지 못했습니다.", error: state.qualityCampaignsError };
    }
    if (runsResult.status === "fulfilled") state.assuranceRuns = Array.isArray(runsResult.value) ? runsResult.value : [];
    else if (state.service.status === "ready") state.service = { status: "degraded", message: "일부 결과를 새로 읽지 못했습니다.", error: runsResult.reason?.message || "" };
    renderQualityWorkSurface();
    renderHome();
    renderAssuranceDashboard();
    return { campaignResult, runsResult };
  }

  const qualityCampaignRequests = new Map();
  const serviceBlocksMutation = () => ["loading", "offline", "error"].includes(state.service.status);
  async function ensureQualityCampaign(target) {
    const existing = campaignForTarget(target);
    if (existing) return existing;
    const key = target.value;
    if (!qualityCampaignRequests.has(key)) {
      qualityCampaignRequests.set(key, request("/api/assurance/campaigns", {
        method: "POST",
        headers: mutationHeaders(),
        body: JSON.stringify({
          projectId: target.projectID,
          repositoryId: target.repositoryID,
          worktreeId: target.worktreeID,
          name: `${target.projectName} · ${target.repositoryName} 코드 검사`,
        }),
      }).then(campaign => {
        state.qualityCampaigns = [...(state.qualityCampaigns || []), campaign];
        state.qualityCampaignsStatus = "ready";
        return campaign;
      }).finally(() => qualityCampaignRequests.delete(key)));
    }
    return qualityCampaignRequests.get(key);
  }

  async function runQualityTechnique(techniqueID) {
    const technique = primaryQualityTechniques.find(item => item.id === techniqueID);
    const target = selectedTarget();
    if (!technique || !target) return;
    if (state.qualityRunPending) return;
    if (serviceBlocksMutation()) {
      state.qualityRunError = "오프라인 상태에서는 새 코드 검사를 실행할 수 없습니다. 연결을 확인한 뒤 결과를 새로 고치세요.";
      state.qualityRunErrorTargetValue = target.value;
      renderQualityWorkSurface();
      return;
    }
    state.qualityRunPending = techniqueID;
    state.qualityRunTargetValue = target.value;
    state.qualityRunError = "";
    state.qualityRunErrorTargetValue = "";
    renderQualityWorkSurface();
    const previousTechniqueIDs = new Set(runsForTarget(target)
      .filter(item => item.spec?.technique === techniqueID)
      .map(item => item.metadata?.id)
      .filter(Boolean));
    let persistedResult = null;
    try {
      const campaign = await ensureQualityCampaign(target);
      persistedResult = await request("/api/assurance/runs", {
        method: "POST",
        headers: mutationHeaders(),
        body: JSON.stringify({ campaignId: campaign.metadata?.id, technique: techniqueID }),
      });
      await refreshQualityResultData();
      const resultID = persistedResult?.metadata?.id || "";
      state.qualityRunError = "";
      showNotice(`${technique.label} 실행 결과를 작업에 표시했습니다.`);
      if (resultID) {
        const result = document.getElementById(`quality-run-result-${encode(resultID)}`);
        result?.scrollIntoView({ block: "nearest" });
      }
    } catch (error) {
      // RunQuality persists unavailable/failed runs before returning an error.
      // Only GET is retried here; an unknown POST outcome is never submitted twice.
      await refreshQualityResultData();
      const latest = runsForTarget(target).find(item => item.spec?.technique === techniqueID && !previousTechniqueIDs.has(item.metadata?.id));
      if (latest && latest.metadata?.id) {
        state.qualityRunError = `요청 결과: ${qualityRunStatusText(latest)} · ${latest.spec?.summary || qualityRunUnavailableText(latest)}`;
      } else {
        state.qualityRunError = `점검 요청 상태를 확인하지 못했습니다. 결과를 새로 고친 뒤 다시 시도하세요. (${error.message || "연결 오류"})`;
      }
      state.qualityRunErrorTargetValue = target.value;
      showNotice("점검 요청 후 저장된 결과를 새로 확인했습니다.", true);
    } finally {
      state.qualityRunPending = "";
      state.qualityRunTargetValue = "";
      renderQualityWorkSurface();
      renderHome();
    }
  }

  let qualitySetupRequestSequence = 0;
  async function loadQualitySetupForTarget(target, force = false) {
    if (!target) {
      qualitySetupRequestSequence += 1;
      state.qualitySetup = { status: "idle", targetValue: "", requestKey: "", data: null, error: "", preferenceReset: false };
      renderQualityWorkSurface();
      return;
    }
    if (!state.selectedTargetValue) rememberTarget(target.value);
    const previousData = state.qualitySetup.targetValue === target.value ? state.qualitySetup.data : null;
    const preference = qualitySetupPreferenceForTarget(target, previousData);
    const requestKey = qualitySetupPath(target, preference);
    if (!force && state.qualitySetup.status === "ready" && state.qualitySetup.targetValue === target.value && state.qualitySetup.requestKey === requestKey) return;
    const sequence = ++qualitySetupRequestSequence;
    state.qualitySetup = { status: "loading", targetValue: target.value, requestKey, data: previousData, error: "", preferenceReset: preference.reset };
    renderQualityWorkSurface();
    try {
      let currentRequestKey = requestKey;
      let currentPreference = preference;
      let preferenceReset = false;
      let autoRefetched = false;
      let manualRefetched = false;
      let observedData = previousData;
      while (true) {
        const data = await request(currentRequestKey);
        if (sequence !== qualitySetupRequestSequence || state.selectedTargetValue !== target.value) return;
        if (!data || typeof data !== "object") throw new Error("구성 확인 응답이 비어 있습니다.");
        const evidenceChanged = qualitySetupEvidenceChanged(target, observedData, data);
        const nextPreference = qualitySetupPreferenceForTarget(target, data);
        const manualResponseNeedsFreshAutoData = currentPreference.mode === "manual" && !currentPreference.pending && !autoRefetched && (evidenceChanged || nextPreference.reset);
        if (manualResponseNeedsFreshAutoData) {
          autoRefetched = true;
          preferenceReset = true;
          currentPreference = { mode: "auto", languages: [], reset: false, pending: false };
          currentRequestKey = qualitySetupPath(target, currentPreference);
          observedData = data;
          state.qualitySetup = { status: "loading", targetValue: target.value, requestKey: currentRequestKey, data: null, error: "", preferenceReset: true };
          renderQualityWorkSurface();
          continue;
        }
        preferenceReset = preferenceReset || preference.reset || evidenceChanged || nextPreference.reset;
        if (!preferenceReset && !manualRefetched && currentPreference.pending && nextPreference.mode === "manual" && nextPreference.languages.length) {
          manualRefetched = true;
          currentPreference = nextPreference;
          currentRequestKey = qualitySetupPath(target, currentPreference);
          observedData = data;
          continue;
        }
        state.qualitySetup = { status: "ready", targetValue: target.value, requestKey: currentRequestKey, data, error: "", preferenceReset };
        if (preferenceReset) saveQualitySetupPreference(target, data, { mode: "auto", languages: [] });
        break;
      }
    } catch (error) {
      if (sequence !== qualitySetupRequestSequence || state.selectedTargetValue !== target.value) return;
      state.qualitySetup = { status: "error", targetValue: target.value, requestKey, data: null, error: qualitySetupErrorText(error), preferenceReset: false };
    } finally {
      if (sequence === qualitySetupRequestSequence) renderQualityWorkSurface();
    }
  }
  async function loadQualitySetupForSelectedTarget(force = false) {
    const target = selectedTarget();
    if (target && !state.selectedTargetValue) rememberTarget(target.value);
    return loadQualitySetupForTarget(target, force);
  }

  let loadingQualityHome = false;
  const normalizeQualityHomeError = error => {
    const raw = String(error?.message || "").trim();
    const technical = error?.name === "SyntaxError" || error?.name === "TypeError" || /JSON|fetch|network|failed to load|요청에 실패했습니다\.\s*\(\d{3}\)/i.test(raw);
    if (raw && !technical) return `품질 개선 큐를 불러오지 못했습니다. 서버 안내: ${raw} 서버 버전과 연결 상태를 확인한 뒤 다시 불러오세요.`;
    return "품질 개선 큐를 불러오지 못했습니다. 서버 버전과 연결 상태를 확인한 뒤 다시 불러오세요.";
  };
  async function loadQualityHome() {
    if (loadingQualityHome) return;
    loadingQualityHome = true;
    state.qualityHome = { status: "loading", data: null, error: "" };
    renderQualityHome();
    try {
      state.qualityHome = { status: "ready", data: normalizeQualityHome(await request("/api/quality/home")), error: "" };
    } catch (error) {
      state.qualityHome = { status: "error", data: null, error: normalizeQualityHomeError(error) };
    } finally {
      loadingQualityHome = false;
      renderQualityHome();
    }
  }

  let qualityObjectiveRequest = 0;
  async function loadQualityObjective(id, force) {
    const selectedID = String(id || "");
    if (!selectedID) {
      state.qualityObjective = { selectedID: "", status: "idle", data: null, error: "", mutation: { kind: "", status: "idle", error: "", draft: {} } };
      renderQualityObjectiveDetail();
      return;
    }
    if (!force && state.qualityObjective.selectedID === selectedID && state.qualityObjective.status === "ready") return;
    const requestID = ++qualityObjectiveRequest;
    const previousDraft = state.qualityObjective.selectedID === selectedID ? state.qualityObjective.mutation?.draft || {} : {};
    state.qualityObjective = { selectedID, status: "loading", data: null, error: "", mutation: { kind: "", status: "idle", error: "", draft: previousDraft } };
    renderQualityObjectiveDetail();
    try {
      const data = await request(`/api/quality/objectives/${encode(selectedID)}`);
      if (requestID !== qualityObjectiveRequest || state.qualityObjective.selectedID !== selectedID) return;
      state.qualityObjective = { selectedID, status: "ready", data, error: "", mutation: { kind: "", status: "idle", error: "", draft: previousDraft } };
    } catch (error) {
      if (requestID !== qualityObjectiveRequest || state.qualityObjective.selectedID !== selectedID) return;
      state.qualityObjective = { selectedID, status: error.status === 404 ? "not_found" : "error", data: null, error: error.message, mutation: { kind: "", status: "idle", error: "", draft: previousDraft } };
    }
    renderQualityObjectiveDetail();
  }

  let loadingQualityTools = false;
  async function loadQualityTools(force) {
    if (loadingQualityTools || (!force && state.qualityTools.status === "ready")) return;
    loadingQualityTools = true;
    state.qualityTools = { status: "loading", data: null, error: "" };
    renderQualityTools();
    try {
      state.qualityTools = { status: "ready", data: normalizeQualityTools(await request("/api/quality/tools")), error: "" };
    } catch (error) {
      state.qualityTools = { status: "error", data: null, error: qualityToolsErrorMessage(error) };
    } finally {
      loadingQualityTools = false;
      renderQualityTools();
    }
  }

  const qualityObjectiveFormDraft = form => {
    const values = Object.fromEntries(new FormData(form).entries());
    if (values.sourceKind) values.sourceValue = values.sourceId || "";
    delete values.sourceId;
    return values;
  };
  const qualityObjectiveMutationState = (kind, status, error, draft) => {
    state.qualityObjective.mutation = { kind, status, error, draft: draft || {} };
  };

  document.addEventListener("change", event => {
    const input = event.target;
    if (!(input instanceof HTMLElement)) return;
    if (!input.matches("[data-quality-objective-disposition], [data-quality-objective-source]")) return;
    const form = input.closest("form");
    if (!form) return;
    const draft = qualityObjectiveFormDraft(form);
    if (input.matches("[data-quality-objective-source]")) draft.sourceKind = input.value;
    if (input.matches("[data-quality-objective-disposition]")) draft.disposition = input.value;
    state.qualityObjective.mutation.draft = draft;
    renderQualityObjectiveDetail();
  });

  document.addEventListener("submit", async event => {
    const form = event.target;
    if (!(form instanceof HTMLFormElement)) return;
    const isDecision = form.matches("[data-quality-objective-decision]");
    const isRevalidation = form.matches("[data-quality-objective-revalidation]");
    const isToolPreview = form.matches("[data-quality-tool-install-preview]");
    if (!isDecision && !isRevalidation && !isToolPreview) return;
    event.preventDefault();
    if (isToolPreview) {
      await requestQualityToolInstallPreview(form);
      return;
    }
    const detail = state.qualityObjective;
    const id = detail.selectedID;
    const spec = qualityObjectiveSpec(detail.data);
    const draft = qualityObjectiveFormDraft(form);
    const expectedRevision = Number(spec.revision);
    if (!id || !Number.isInteger(expectedRevision) || expectedRevision < 1) {
      qualityObjectiveMutationState("", "error", "최신 과제 상태를 확인한 뒤 다시 시도하세요.", draft);
      renderQualityObjectiveDetail();
      return;
    }
    let path = "";
    let payload = { expectedRevision };
    let kind = "";
    if (isDecision) {
      kind = "decision";
      payload.disposition = String(draft.disposition || "");
      payload.actor = String(draft.actor || "").trim();
      if (payload.disposition === "pursue") payload.action = String(draft.action || "").trim();
      if (["defer", "dismiss"].includes(payload.disposition)) payload.reason = String(draft.reason || "").trim();
      if (spec.primarySignal?.kind === "go_coverage" && String(draft.minimumPercent || "").trim() !== "") payload.minimumPercent = Number(draft.minimumPercent);
      path = `/api/quality/objectives/${encode(id)}/decision`;
    } else {
      kind = "revalidation";
      const sourceKind = String(draft.sourceKind || "");
      const sourceID = String(draft.sourceValue || "").trim();
      if (sourceKind === "finding") payload.findingId = sourceID;
      if (sourceKind === "qualityRun") payload.qualityRunId = sourceID;
      path = `/api/quality/objectives/${encode(id)}/revalidations`;
    }
    qualityObjectiveMutationState(kind, "submitting", "", draft);
    renderQualityObjectiveDetail();
    try {
      await request(path, { method: "POST", headers: mutationHeaders(), body: JSON.stringify(payload) });
      qualityObjectiveMutationState("", "idle", "", {});
      showNotice(kind === "decision" ? "개선 과제의 처리 방침을 저장했습니다." : "후속 점검 결과를 연결했습니다.");
      await Promise.all([loadQualityHome(), loadQualityObjective(id, true)]);
    } catch (error) {
      qualityObjectiveMutationState(kind, "error", qualityObjectiveMutationError(error), draft);
      renderQualityObjectiveDetail();
    }
  });

  async function loadAssuranceTrace(effectID, opener) {
    if (!effectID) return;
    state.assuranceTraceEffectID = effectID;
    state.assuranceTraceOpener = opener;
    state.assuranceTrace = null;
    state.assuranceTraceError = "";
    renderAssuranceTrace();
    const panel = document.getElementById("assurance-trace-panel");
    panel?.scrollIntoView({ behavior: "smooth", block: "start" });
    try {
      state.assuranceTrace = await request(`/api/assurance/traces/${encode(effectID)}`);
    } catch (error) {
      state.assuranceTraceError = error.message || "잠시 후 다시 시도하세요.";
    }
    renderAssuranceTrace();
    panel?.focus({ preventScroll: true });
  }

  async function exportAssuranceImpact(format, button) {
    button.disabled = true;
    try {
      const response = await fetch(`/api/assurance/impact/export?${assuranceImpactQuery(format).toString()}`);
      if (!response.ok) {
        let message = `내보내기에 실패했습니다. (${response.status})`;
        try {
          const body = await response.json();
          message = body.error?.message || message;
        } catch (_) { /* downloadable endpoint may return no JSON on failure */ }
        throw new Error(message);
      }
      const blob = await response.blob();
      const disposition = response.headers.get("Content-Disposition") || "";
      const match = disposition.match(/filename\*?=(?:UTF-8''|\")?([^;\"]+)/i);
      const filename = match?.[1] ? decodeURIComponent(match[1].replace(/\"/g, "")) : `assurance-impact.${format}`;
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = filename;
      link.hidden = true;
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 0);
      showNotice(`${format.toUpperCase()} 보고서를 내보냈습니다.`);
    } catch (error) {
      showNotice(error.message, true);
    } finally {
      button.disabled = false;
    }
  }

  async function loadRouteData(route, force) {
    if (route === "work") {
      await loadWorkData(force);
      await loadQualitySetupForSelectedTarget(force);
    }
    if (route === "diagnostics") await loadDiagnosticsData(force);
    if (route === "assurance") await loadQualityInspectionData(force);
    if (route === "home") await loadQualityObjective(routeState().objectiveID, force);
  }

  let assuranceFiltering = false;
  async function refreshAssuranceFilter() {
    if (assuranceFiltering) return;
    assuranceFiltering = true;
    const providerSelect = document.getElementById("assurance-provider-filter");
    const modelSelect = document.getElementById("assurance-model-filter");
    const projectSelect = document.getElementById("assurance-project-filter");
    const daysSelect = document.getElementById("assurance-days-filter");
    if (providerSelect) providerSelect.disabled = true;
    if (modelSelect) modelSelect.disabled = true;
    if (projectSelect) projectSelect.disabled = true;
    if (daysSelect) daysSelect.disabled = true;
    try {
      state.assuranceDashboard = await request(assuranceDashboardPath());
      await loadAssuranceProductData();
      renderHome();
      renderAssuranceDashboard();
    } catch (error) {
      showNotice(`검증 필터를 적용하지 못했습니다. ${error.message}`, true);
    } finally {
      assuranceFiltering = false;
      if (providerSelect) providerSelect.disabled = false;
      if (modelSelect) modelSelect.disabled = false;
      if (projectSelect) projectSelect.disabled = false;
      if (daysSelect) daysSelect.disabled = false;
    }
  }

  let refreshing = false;
  let initialized = false;
  async function refreshAll() {
    if (refreshing) return;
    refreshing = true;
    try {
      const requests = [
        ["snapshot", request("/api/state")],
        ["registryProjects", request("/api/projects")],
        ["findings", request("/api/findings")],
        ["events", request("/api/events")],
        ["environment", request("/api/environment")],
        ["providerStatuses", request("/api/assurance/providers")],
        ["assuranceDashboard", request(assuranceDashboardPath())],
        ["assuranceRuns", request("/api/assurance/runs")],
        ["assuranceInvocations", request("/api/assurance/invocations")],
        ["assuranceArtifacts", request("/api/assurance/artifacts")],
        ["assuranceEffects", request("/api/assurance/effects")],
      ];
      const results = await Promise.all(requests.map(([, promise]) => promise.then(value => ({ status: "fulfilled", value }), reason => ({ status: "rejected", reason }))));
      const failed = results.filter(result => result.status === "rejected");
      const succeeded = results.length - failed.length;
      requests.forEach(([key], index) => {
        if (results[index].status !== "fulfilled") return;
        const value = results[index].value;
        if (key === "snapshot") state.snapshot = value || { projects: [] };
        if (key === "registryProjects") state.registryProjects = value || [];
        if (key === "findings") state.findings = value || [];
        if (key === "events") state.events = value || [];
        if (key === "environment") state.environment = value || state.environment;
        if (key === "providerStatuses") state.providerStatuses = value || [];
        if (key === "assuranceDashboard") state.assuranceDashboard = value || state.assuranceDashboard;
        if (key === "assuranceRuns") state.assuranceRuns = value || [];
        if (key === "assuranceInvocations") state.assuranceInvocations = value || [];
        if (key === "assuranceArtifacts") state.assuranceArtifacts = value || [];
        if (key === "assuranceEffects") state.assuranceEffects = value || [];
      });
      const firstFailure = failed[0]?.reason;
      state.service = failed.length === 0
        ? { status: "ready", message: "로컬 서비스에 연결되었습니다.", error: "" }
        : succeeded === 0
          ? { status: "offline", message: "로컬 서비스에 연결할 수 없습니다.", error: firstFailure?.message || "연결 실패" }
          : { status: "degraded", message: "일부 데이터를 새로 읽지 못했습니다.", error: firstFailure?.message || "일부 요청 실패" };
      if (succeeded > 0) state.lastSuccessfulRefreshAt = new Date().toISOString();
      initialized = true;
      await loadRouteData(currentRoute(), true);
      renderAll();
      await Promise.all([loadQualityCampaigns(true), loadAssuranceProductData(), loadAssuranceMeasurementData()]);
      renderServiceRecovery();
    } catch (error) {
      state.service = { status: "offline", message: "로컬 서비스에 연결할 수 없습니다.", error: error.message || "연결 실패" };
      renderServiceRecovery();
    } finally {
      refreshing = false;
    }
  }

  let repositoryRefreshPending = false;
  const waitForRepositoryRefresh = delay => new Promise(resolve => window.setTimeout(resolve, delay));
  const hasObservedTarget = projectID => targetOptions().some(target => !projectID || target.projectID === projectID);
  async function queueRepositoryStateRefresh(button, projectID = "") {
    if (repositoryRefreshPending) return false;
    repositoryRefreshPending = true;
    const scanMarkerBeforeRequest = String(state.snapshot.generated_at || "");
    const originalLabel = button?.textContent || "저장소 상태 새로 고침";
    if (button) {
      button.disabled = true;
      button.textContent = "새로 고치는 중…";
    }
    try {
      await request("/api/scan", { method: "POST", headers: mutationHeaders() });
      let observed = false;
      for (const delay of [150, 300, 600, 1000, 1500]) {
        await waitForRepositoryRefresh(delay);
        await refreshAll();
        const scanMarker = String(state.snapshot.generated_at || "");
        observed = hasObservedTarget(projectID) && scanMarker !== "" && scanMarker !== scanMarkerBeforeRequest;
        if (observed) break;
      }
      showNotice(observed
        ? "저장소 상태를 새로 고쳤습니다."
        : "저장소 상태 갱신을 요청했습니다. 아직 완료를 확인하지 못했습니다. 잠시 후 다시 새로 고치세요.", !observed);
      return observed;
    } catch (error) {
      showNotice(error.message, true);
      return false;
    } finally {
      repositoryRefreshPending = false;
      if (button && button.isConnected) {
        button.disabled = false;
        button.textContent = originalLabel;
      }
    }
  }

  const unregisterDialog = document.getElementById("unregister-dialog");
  const unregisterInput = document.getElementById("unregister-confirmation");
  const editorDialog = document.getElementById("editor-dialog");
  const editorFields = document.getElementById("editor-fields");
  let unregisterTarget = null;
  let editorTarget = null;
  let registerOpener = null;
  let unregisterOpener = null;
  let editorOpener = null;

  unregisterInput?.setAttribute("name", "confirmation");

  function restoreDialogFocus(opener) {
    window.setTimeout(() => {
      if (opener?.isConnected && !opener.disabled) {
        opener.focus({ preventScroll: true });
        return;
      }
      focusElementByID("main-content");
    }, 0);
  }

  function setRegisterPanelOpen(open, opener = null) {
    const panel = document.getElementById("register-panel");
    const toggle = document.getElementById("show-register");
    if (opener) registerOpener = opener;
    if (panel) panel.hidden = !open;
    toggle?.setAttribute("aria-expanded", String(open));
  }

  const editorFieldName = id => id.replace(/^edit-/, "");
  const editorInput = (id, labelText, value = "", options = {}) => `<label class="${options.wide ? "wide" : ""}"><span>${escapeHTML(labelText)}</span><input id="${id}" name="${escapeHTML(options.name || editorFieldName(id))}" ${options.type ? `type="${options.type}"` : ""} ${options.required === false ? "" : "required"} ${options.readonly ? "readonly" : ""} autocomplete="off" value="${escapeHTML(value)}"></label>`;
  const editorTextarea = (id, labelText, value = "") => `<label class="wide"><span>${escapeHTML(labelText)}</span><textarea id="${id}" name="${escapeHTML(editorFieldName(id))}" autocomplete="off" rows="3">${escapeHTML(value)}</textarea></label>`;

  function openEditor(kind, context = {}) {
    editorOpener = document.activeElement;
    editorTarget = { kind, ...context };
    const title = document.getElementById("editor-title");
    const description = document.getElementById("editor-description");
    if (kind === "project") {
      title.textContent = "프로젝트 이름 변경";
      description.textContent = "프로젝트 ID와 등록된 저장소는 유지됩니다.";
      editorFields.innerHTML = editorInput("edit-name", "프로젝트 이름", context.project.metadata.name);
    } else if (kind === "add-repository") {
      title.textContent = "새 저장소 등록";
      description.textContent = "기존 프로젝트에 읽기 전용 관찰 대상을 추가합니다.";
      editorFields.innerHTML = editorInput("edit-id", "저장소 ID") + editorInput("edit-name", "저장소 이름") + editorInput("edit-path", "Windows 저장소 경로", "", { wide: true });
    } else if (kind === "repository") {
      title.textContent = "저장소 정보 변경";
      description.textContent = "저장소 ID는 유지되며 이름과 관찰 경로만 변경됩니다.";
      editorFields.innerHTML = editorInput("edit-name", "저장소 이름", context.repository.metadata.name) + editorInput("edit-path", "Windows 저장소 경로", context.repository.spec.path, { wide: true });
    } else if (kind === "runbook") {
      const runbook = context.runbook;
      title.textContent = runbook ? "PowerShell runbook 변경" : "PowerShell runbook 추가";
      description.textContent = "구체적인 .ps1 파일만 등록합니다. 실행 값은 저장하지 않고 계획 생성 시 typed argv로 전달합니다.";
      editorFields.innerHTML = `${runbook ? "" : editorInput("edit-id", "runbook ID")}${editorInput("edit-name", "표시 이름", runbook?.name || "")}${editorInput("edit-script", ".ps1 파일 경로", runbook?.scriptPath || "", { wide: true })}${editorTextarea("edit-parameters", "named parameter 이름 · 한 줄에 하나", (runbook?.parameters || []).join("\n"))}${editorTextarea("edit-runbook-environment", "허용 환경 변수 · 한 줄에 하나", (runbook?.environmentAllowlist || []).join("\n"))}${editorInput("edit-timeout", "제한 시간(초)", runbook?.timeoutSeconds || 300, { type: "number" })}`;
    } else if (kind === "integration") {
      const integration = context.integration;
      title.textContent = integration ? "연동 설정 변경" : "연동 설정 추가";
      description.textContent = "토큰 값은 입력하지 말고 env:이름 또는 credential_manager:이름 형태의 참조만 저장하세요.";
      editorFields.innerHTML = `${integration ? "" : editorInput("edit-id", "연동 ID")}${editorInput("edit-name", "표시 이름", integration?.name || "")}<label><span>종류</span><select id="edit-integration-kind" name="kind" autocomplete="off"><option value="github" ${integration?.kind === "github" ? "selected" : ""}>GitHub</option><option value="jenkins" ${integration?.kind === "jenkins" ? "selected" : ""}>Jenkins</option><option value="kubernetes" ${integration?.kind === "kubernetes" ? "selected" : ""}>Kubernetes</option></select></label>${editorInput("edit-endpoint", "API 주소", integration?.endpoint || "", { wide: true })}${editorInput("edit-credential", "Credential reference", integration?.credentialRef || "", { required: false })}${editorTextarea("edit-values", "대상 값 · key=value 한 줄에 하나", Object.entries(integration?.values || {}).map(([key, value]) => `${key}=${value}`).join("\n"))}`;
    } else if (kind === "external-group") {
      const group = context.group;
      title.textContent = group ? "Jenkins 대상 그룹 변경" : "Jenkins 대상 그룹 추가";
      description.textContent = "대상 한 줄을 ‘대상 ID | Jenkins 연동 ID | 완료된 build URL | key=value,... | fallback runbook ID’로 입력합니다. 비밀 값은 입력하지 않습니다.";
      editorFields.innerHTML = `${group ? "" : editorInput("edit-id", "그룹 ID")}${editorInput("edit-name", "표시 이름", group?.name || "")}${editorTextarea("edit-targets", "Jenkins 대상 · 한 줄에 하나", group ? externalTargetText(group) : "", true)}`;
    } else {
      const profile = context.profile;
      title.textContent = profile ? "Agent Profile 변경" : "Agent Profile 추가";
      description.textContent = "명령과 인자는 분리해 저장하고, 허용한 환경 변수 이름만 전달합니다.";
      editorFields.innerHTML = `${profile ? "" : editorInput("edit-id", "Profile ID")}${editorInput("edit-name", "표시 이름", profile?.metadata.name || "")}${editorInput("edit-command", "실행 명령", profile?.spec.command || "")}${editorTextarea("edit-version-probe", "버전 확인 인자 · 한 줄에 하나", (profile?.spec.versionProbe || []).join("\n"))}${editorInput("edit-timeout", "제한 시간(초)", profile?.spec.timeoutSeconds || 10, { type: "number" })}${editorInput("edit-model-template", "모델 인자 템플릿", profile?.spec.modelArgumentTemplate || "", { required: false, name: "modelArgumentTemplate" })}${editorTextarea("edit-environment", "허용 환경 변수 · 한 줄에 하나", (profile?.spec.environmentAllowlist || []).join("\n"))}<label><span>실행 방식</span><select id="edit-launch-mode" name="launchMode" autocomplete="off"><option value="direct" ${profile?.spec.launchMode === "direct" ? "selected" : ""}>직접 실행</option><option value="powershell_profile" ${profile?.spec.launchMode === "powershell_profile" ? "selected" : ""}>PowerShell profile</option></select></label><label><span>데이터 경계</span><select id="edit-data-boundary" name="dataBoundary" autocomplete="off"><option value="enterprise" ${profile?.spec.dataBoundary === "enterprise" ? "selected" : ""}>기업 경계</option><option value="local" ${profile?.spec.dataBoundary === "local" ? "selected" : ""}>로컬 전용</option></select></label>`;
    }
    editorDialog.showModal();
    editorFields.querySelector("input, select, textarea")?.focus();
  }

  const lineList = value => value.split(/\r?\n/).map(item => item.trim()).filter(Boolean);
  const keyValues = value => Object.fromEntries(lineList(value).map(item => { const [key, ...rest] = item.split("="); return [key.trim(), rest.join("=").trim()]; }).filter(([key, item]) => key && item));

  async function submitEditor() {
    if (editorTarget.kind === "project") {
      await request(`/api/projects/${encode(editorTarget.project.metadata.id)}`, { method: "PUT", headers: mutationHeaders(), body: JSON.stringify({ name: document.getElementById("edit-name").value }) });
      return "프로젝트 이름을 변경했습니다.";
    }
    if (editorTarget.kind === "add-repository") {
      await request(`/api/projects/${encode(editorTarget.projectID)}/repositories`, { method: "POST", headers: mutationHeaders(), body: JSON.stringify({ id: document.getElementById("edit-id").value, name: document.getElementById("edit-name").value, path: document.getElementById("edit-path").value }) });
      return "저장소를 등록했습니다.";
    }
    if (editorTarget.kind === "repository") {
      await request(`/api/projects/${encode(editorTarget.projectID)}/repositories/${encode(editorTarget.repository.metadata.id)}`, { method: "PUT", headers: mutationHeaders(), body: JSON.stringify({ name: document.getElementById("edit-name").value, path: document.getElementById("edit-path").value }) });
      return "저장소 정보를 변경했습니다.";
    }
    if (editorTarget.kind === "runbook") {
      const runbook = editorTarget.runbook;
      const body = { name: document.getElementById("edit-name").value, scriptPath: document.getElementById("edit-script").value, parameters: lineList(document.getElementById("edit-parameters").value), environmentAllowlist: lineList(document.getElementById("edit-runbook-environment").value), timeoutSeconds: Number(document.getElementById("edit-timeout").value) };
      if (!runbook) body.id = document.getElementById("edit-id").value;
      await request(runbook ? `/api/runbooks/${encode(runbook.id)}` : "/api/runbooks", { method: runbook ? "PUT" : "POST", headers: mutationHeaders(), body: JSON.stringify(body) });
      return runbook ? "PowerShell runbook을 변경했습니다." : "PowerShell runbook을 추가했습니다.";
    }
    if (editorTarget.kind === "integration") {
      const integration = editorTarget.integration;
      const body = { name: document.getElementById("edit-name").value, kind: document.getElementById("edit-integration-kind").value, endpoint: document.getElementById("edit-endpoint").value, credentialRef: document.getElementById("edit-credential").value, values: keyValues(document.getElementById("edit-values").value) };
      if (!integration) body.id = document.getElementById("edit-id").value;
      await request(integration ? `/api/integrations/${encode(integration.id)}` : "/api/integrations", { method: integration ? "PUT" : "POST", headers: mutationHeaders(), body: JSON.stringify(body) });
      return integration ? "연동 설정을 변경했습니다." : "연동 설정을 추가했습니다.";
    }
    if (editorTarget.kind === "external-group") {
      const group = editorTarget.group;
      const targets = lineList(document.getElementById("edit-targets").value).map((line, index) => {
        const [id, integrationId, completedBuildUrl, parameterText = "", fallbackRunbookId = ""] = line.split("|").map(item => item.trim());
        return { id: id || `target-${index + 1}`, integrationId, completedBuildUrl, parameters: keyValues(parameterText.replace(/,/g, "\n")), fallbackRunbookId };
      });
      const body = { name: document.getElementById("edit-name").value, targets };
      if (!group) body.id = document.getElementById("edit-id").value;
      await request(group ? `/api/external-work-groups/${encode(group.id)}` : "/api/external-work-groups", { method: group ? "PUT" : "POST", headers: mutationHeaders(), body: JSON.stringify(body) });
      return group ? "Jenkins 대상 그룹을 변경했습니다." : "Jenkins 대상 그룹을 추가했습니다.";
    }
    const profile = editorTarget.profile;
    const body = {
      name: document.getElementById("edit-name").value,
      command: document.getElementById("edit-command").value,
      versionProbe: lineList(document.getElementById("edit-version-probe").value),
      timeoutSeconds: Number(document.getElementById("edit-timeout").value),
      modelArgumentTemplate: document.getElementById("edit-model-template").value,
      environmentAllowlist: lineList(document.getElementById("edit-environment").value),
      launchMode: document.getElementById("edit-launch-mode").value,
      dataBoundary: document.getElementById("edit-data-boundary").value,
    };
    if (!profile) body.id = document.getElementById("edit-id").value;
    await request(profile ? `/api/agent-profiles/${encode(profile.metadata.id)}` : "/api/agent-profiles", { method: profile ? "PUT" : "POST", headers: mutationHeaders(), body: JSON.stringify(body) });
    return profile ? "Agent Profile을 변경했습니다." : "Agent Profile을 추가했습니다.";
  }

  function openUnregister(button) {
    unregisterOpener = button;
    unregisterTarget = {
      kind: button.dataset.unregister,
      projectID: button.dataset.project,
      repositoryID: button.dataset.repository || "",
      profileID: button.dataset.profileId || "",
      integrationID: button.dataset.integrationId || "",
      externalGroupID: button.dataset.externalGroupId || "",
      runbookID: button.dataset.runbookId || "",
      name: button.dataset.name,
    };
    const isProject = unregisterTarget.kind === "project";
    const isProfile = unregisterTarget.kind === "profile";
    const isIntegration = unregisterTarget.kind === "integration";
    const isExternalGroup = unregisterTarget.kind === "external-group";
    const isRunbook = unregisterTarget.kind === "runbook";
    document.getElementById("unregister-title").textContent = isProfile ? "Agent Profile 제거" : isIntegration ? "연동 설정 제거" : isExternalGroup ? "Jenkins 대상 그룹 제거" : isRunbook ? "PowerShell runbook 제거" : isProject ? "프로젝트 등록 해제" : "저장소 등록 해제";
    document.getElementById("unregister-description").textContent = isProfile
      ? `“${unregisterTarget.name}” Agent Profile의 저장된 실행 설정을 제거합니다.`
      : isProject
        ? `“${unregisterTarget.name}” 프로젝트의 등록과 모든 저장소 관찰 기록을 해제합니다.`
      : isIntegration ? `“${unregisterTarget.name}” 연동 설정을 제거합니다.` : isExternalGroup ? `“${unregisterTarget.name}” Jenkins 대상 그룹 설정을 제거합니다.` : isRunbook ? `“${unregisterTarget.name}” PowerShell runbook 설정을 제거합니다.` : `“${unregisterTarget.name}” 저장소의 등록과 관찰 기록을 해제합니다.`;
    document.getElementById("unregister-safety").textContent = isProfile
      ? "Profile 설정만 제거하며 Agent 프로그램이나 작업 파일은 삭제하지 않습니다."
      : isIntegration ? "저장소나 외부 시스템은 변경하지 않고 로컬 설정만 제거합니다."
      : isExternalGroup ? "Jenkins나 credential는 변경하지 않고 로컬 그룹 설정만 제거합니다."
      : isRunbook ? "runbook 설정만 제거하며 .ps1 파일은 삭제하지 않습니다."
      : "등록 정보만 제거하며 저장소 파일은 삭제하지 않습니다.";
    document.getElementById("unregister-label").textContent = `확인 문구: “${unregisterTarget.name}”`;
    unregisterInput.value = "";
    document.getElementById("unregister-submit").disabled = true;
    unregisterDialog.showModal();
    unregisterInput.focus();
  }

  document.addEventListener("click", async event => {
    const button = event.target.closest("button");
    if (!button) return;
    if (button.dataset.guidePrev !== undefined) {
      setGuideSlide(guideSlideIndex() - 1);
      return;
    }
    if (button.dataset.guideNext !== undefined) {
      setGuideSlide(guideSlideIndex() + (guideSlideIndex() === guideSlides.length - 1 ? -guideSlides.length + 1 : 1));
      return;
    }
    if (button.dataset.guideSlide !== undefined) {
      setGuideSlide(Number(button.dataset.guideSlide));
      return;
    }
    if (button.dataset.guideStart !== undefined) {
      if (currentRoute() !== "guide") {
        location.hash = "guide";
        return;
      }
      setGuideSlide(0);
      return;
    }
    if (button.dataset.qualityRetry !== undefined) {
      button.disabled = true;
      await loadQualityHome();
      button.disabled = false;
      return;
    }
    if (button.dataset.qualityToolsRetry !== undefined || button.id === "quality-tools-refresh") {
      button.disabled = true;
      try {
        await loadQualityTools(true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.dataset.qualitySetupRetry !== undefined) {
      button.disabled = true;
      await loadQualitySetupForSelectedTarget(true);
      return;
    }
    if (button.dataset.qualitySetupApply !== undefined) {
      const target = selectedTarget();
      if (!target) return;
      const mode = document.querySelector('input[data-quality-language-mode]:checked')?.value === "manual" ? "manual" : "auto";
      const languages = [...document.querySelectorAll("#quality-setup [data-quality-language]:checked")].map(input => input.dataset.qualityLanguage).filter(Boolean);
      if (mode === "manual" && !languages.length) {
        showNotice("직접 선택에서는 언어를 하나 이상 고르세요.", true);
        return;
      }
      saveQualitySetupPreference(target, state.qualitySetup.data, { mode, languages });
      button.disabled = true;
      await loadQualitySetupForTarget(target, true);
      return;
    }
    if (button.id === "quality-inspection-refresh") {
      button.disabled = true;
      await loadQualityInspectionData(true);
      button.disabled = false;
      return;
    }
    if (button.dataset.qualityInspection === "generate") {
      await generateQualityInspectionPlan();
      return;
    }
    if (button.dataset.qualityInspectionReview !== undefined) {
      await reviewQualityInspectionPlan(button.dataset.qualityInspectionReview, button);
      return;
    }
    if (button.dataset.qualityInspectionRun !== undefined) {
      await runQualityInspectionPlan(button);
      return;
    }
    if (button.dataset.qualityInspectionProposal === "generate") {
      await generateQualityImprovementProposal();
      return;
    }
    if (button.dataset.qualityInspectionProposalReview !== undefined) {
      await reviewQualityImprovementProposal(button.dataset.qualityInspectionProposalReview, button);
      return;
    }
    if (button.dataset.qualityInspectionProposalApply !== undefined) {
      await applyQualityImprovementProposal(button);
      return;
    }
    if (button.dataset.qualityToolInstallApproval !== undefined) {
      await approveQualityToolInstallActionPlan(button);
      return;
    }
    if (button.dataset.qualityToolInstallExecute !== undefined) {
      await executeQualityToolInstallActionPlan(button);
      return;
    }
    if (button.dataset.qualityInspectionCompare !== undefined) {
      await compareQualityInspectionScores();
      return;
    }
    if (button.dataset.qualityRun !== undefined) {
      await runQualityTechnique(button.dataset.qualityRun);
      return;
    }
    if (button.dataset.serviceRefresh !== undefined) {
      button.disabled = true;
      await refreshAll();
      button.disabled = false;
      return;
    }
    if (button.dataset.qualityObjectiveRetry !== undefined) {
      button.disabled = true;
      await loadQualityObjective(state.qualityObjective.selectedID, true);
      return;
    }
    if (button.dataset.qualityObjectiveConfirm !== undefined) {
      const detail = state.qualityObjective;
      const id = detail.selectedID;
      const spec = qualityObjectiveSpec(detail.data);
      const expectedRevision = Number(spec.revision);
      if (!id || !Number.isInteger(expectedRevision) || expectedRevision < 1) return;
      button.disabled = true;
      qualityObjectiveMutationState("confirm", "submitting", "", {});
      renderQualityObjectiveDetail();
      try {
        await request(`/api/quality/objectives/${encode(id)}/confirm`, {
          method: "POST",
          headers: mutationHeaders(),
          body: JSON.stringify({ expectedRevision }),
        });
        qualityObjectiveMutationState("", "idle", "", {});
        showNotice("개선 과제를 완료로 확인했습니다.");
        await Promise.all([loadQualityHome(), loadQualityObjective(id, true)]);
      } catch (error) {
        qualityObjectiveMutationState("confirm", "error", qualityObjectiveMutationError(error), {});
        renderQualityObjectiveDetail();
      }
      return;
    }
    if (button.dataset.retry) {
      button.disabled = true;
      await loadRouteData(button.dataset.retry, true);
      return;
    }
    if (button.dataset.homeScan !== undefined) {
      document.getElementById("scan").click();
      return;
    }
    if (button.dataset.assuranceRetry !== undefined) {
      button.disabled = true;
      await loadAssuranceProductData();
      button.disabled = false;
      return;
    }
    if (button.dataset.assuranceExport) {
      await exportAssuranceImpact(button.dataset.assuranceExport, button);
      return;
    }
    if (button.dataset.assuranceTrace !== undefined) {
      await loadAssuranceTrace(button.dataset.assuranceTrace, button);
      return;
    }
    if (button.dataset.assuranceTraceClose !== undefined) {
      state.assuranceTraceEffectID = "";
      state.assuranceTrace = null;
      state.assuranceTraceError = "";
      renderAssuranceTrace();
      state.assuranceTraceOpener?.focus({ preventScroll: true });
      state.assuranceTraceOpener = null;
      return;
    }
    if (button.dataset.assuranceArtifact) {
      const id = button.dataset.id;
      button.disabled = true;
      try {
        if (button.dataset.assuranceArtifact === "restore") {
          await request(`/api/assurance/artifacts/${encode(id)}/restore`, { method: "POST", headers: mutationHeaders(), body: "" });
          showNotice("artifact를 복원했습니다.");
        } else {
          await request(`/api/assurance/artifacts/${encode(id)}/retention`, { method: "POST", headers: mutationHeaders(), body: JSON.stringify({ retention: button.dataset.retention }) });
          showNotice(button.dataset.retention === "pinned" ? "artifact를 고정했습니다." : "artifact 고정을 해제했습니다.");
        }
        await loadAssuranceProductData();
        state.assuranceArtifacts = await request("/api/assurance/artifacts");
        renderAssuranceDashboard();
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.id === "show-register") {
      setRegisterPanelOpen(true, button);
      document.getElementById("name").focus();
      return;
    }
    if (button.id === "hide-register") {
      setRegisterPanelOpen(false);
      restoreDialogFocus(registerOpener);
      registerOpener = null;
      return;
    }
    if (button.dataset.openProject !== undefined) {
      state.activeProjectID = button.dataset.openProject;
      location.hash = "projects";
      renderProjects();
      return;
    }
    if (button.dataset.rediscover !== undefined) {
      button.disabled = true;
      try {
        await request(`/api/projects/${encode(button.dataset.project)}/repositories/${encode(button.dataset.repository)}/worktrees/${encode(button.dataset.worktree)}/discover`, { method: "POST", headers: mutationHeaders() });
        showNotice("현재 Worktree에서 기존 점검을 다시 찾았습니다.");
        await loadWorkData(true);
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.dataset.projectAction) {
      const project = registryProject(button.dataset.project);
      if (!project) return;
      if (button.dataset.projectAction === "edit") {
        openEditor("project", { project });
      } else if (button.dataset.projectAction === "add-repository") {
        openEditor("add-repository", { projectID: project.metadata.id });
      } else {
        button.disabled = true;
        try {
          const exported = await request(`/api/projects/${encode(project.metadata.id)}/export`);
          const blob = new Blob([JSON.stringify(exported, null, 2)], { type: "application/json" });
          const url = URL.createObjectURL(blob);
          const link = document.createElement("a");
          link.href = url;
          link.download = `${project.metadata.id}.devroom.json`;
          link.click();
          URL.revokeObjectURL(url);
          showNotice("프로젝트 설정을 내보냈습니다. 비밀 값은 포함되지 않습니다.");
        } catch (error) {
          showNotice(error.message, true);
        } finally {
          button.disabled = false;
        }
      }
      return;
    }
    if (button.dataset.repositoryAction === "edit") {
      const project = registryProject(button.dataset.project);
      const repository = project?.spec.repositories?.find(item => item.metadata.id === button.dataset.repository);
      if (repository) openEditor("repository", { projectID: project.metadata.id, repository });
      return;
    }
    if (button.dataset.project !== undefined && !button.dataset.unregister) {
      state.activeProjectID = button.dataset.project;
      pendingProjectFocusID = button.dataset.project;
      renderProjects();
      return;
    }
    if (button.dataset.unregister) {
      openUnregister(button);
      return;
    }
    if (button.dataset.finding === "acknowledge") {
      button.disabled = true;
      try {
        await request(`/api/findings/${encode(button.dataset.id)}/acknowledge`, { method: "POST", headers: mutationHeaders() });
        showNotice("확인 항목을 확인함으로 표시했습니다.");
        await refreshAll();
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.id === "discover-worktree") {
      const target = document.getElementById("discovery-target").value.split("|");
      button.disabled = true;
      try {
        const result = await request(`/api/projects/${encode(target[0])}/repositories/${encode(target[1])}/worktrees/${encode(target[2])}/discover`, { method: "POST", headers: mutationHeaders() });
        showNotice(`기존 점검 명령 ${result.spec?.proposalIds?.length || 0}개를 찾았습니다.`);
        await loadWorkData(true);
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.dataset.proposal) {
      button.disabled = true;
      try {
        await request(`/api/proposals/${encode(button.dataset.id)}/${button.dataset.proposal}`, { method: "POST", headers: mutationHeaders() });
        showNotice(button.dataset.proposal === "apply" ? "제안을 적용했습니다." : "제안을 거절했습니다.");
        await loadWorkData(true);
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.dataset.checkset) {
      const action = button.dataset.checkset;
      const id = button.dataset.id;
      button.disabled = true;
      try {
        if (action === "create") {
          const proposal = await request(`/api/proposals/${encode(id)}`);
          await request("/api/checksets", {
            method: "POST",
            headers: mutationHeaders(),
            body: JSON.stringify({ id: `checks-${id}`, name: proposal.metadata.name, proposalId: id, steps: [{ id: "check", name: proposal.metadata.name, command: proposal.spec.typedCommand }] }),
          });
          showNotice("Checkset을 만들었습니다.");
        } else if (action === "apply") {
          await request(`/api/checksets/${encode(id)}/apply`, { method: "POST", headers: mutationHeaders() });
          showNotice("Checkset을 적용했습니다.");
        } else if (action === "run") {
          await request(`/api/checksets/${encode(id)}/run`, { method: "POST", headers: mutationHeaders() });
          showNotice("Pre-PR 점검을 실행했습니다.");
        } else {
          state.checkRuns.set(id, await request(`/api/checksets/${encode(id)}/runs`));
          state.expandedChecks.has(id) ? state.expandedChecks.delete(id) : state.expandedChecks.add(id);
          renderWork();
          return;
        }
        await refreshAll();
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.id === "add-profile") {
      openEditor("profile", {});
      return;
    }
    if (button.id === "add-integration") {
      openEditor("integration", {});
      return;
    }
    if (button.id === "add-external-group") {
      openEditor("external-group", {});
      return;
    }
    if (button.dataset.externalGroup === "edit") {
      const group = state.externalGroups.find(item => item.id === button.dataset.id);
      if (group) openEditor("external-group", { group });
      return;
    }
    if (button.id === "add-runbook") {
      openEditor("runbook", {});
      return;
    }
    if (button.dataset.runbook === "edit") {
      const runbook = state.runbooks.find(item => item.id === button.dataset.id);
      if (runbook) openEditor("runbook", { runbook });
      return;
    }
    if (button.dataset.runbook === "plan") {
      const runbookID = button.dataset.id;
      const target = document.querySelector(`[data-runbook-target="${CSS.escape(runbookID)}"]`)?.value || "";
      const [projectID, repositoryID, worktreeID] = target.split("|");
      const parameters = Object.fromEntries([...document.querySelectorAll(`[data-runbook-param][data-runbook-id="${CSS.escape(runbookID)}"]`)].filter(input => input.value.trim()).map(input => [input.dataset.runbookParam, input.value]));
      if (!projectID || !repositoryID || !worktreeID) {
        showNotice("실행할 Worktree를 선택하세요.", true);
        return;
      }
      button.disabled = true;
      try {
        await request(`/api/runbooks/${encode(runbookID)}/plan`, { method: "POST", headers: mutationHeaders(), body: JSON.stringify({ projectId: projectID, repositoryId: repositoryID, worktreeId: worktreeID, parameters }) });
        showNotice("PowerShell runbook 실행 계획을 만들었습니다. 작업 화면에서 승인 후 실행하세요.");
        state.loaded.work = false;
        location.hash = "work";
        await loadWorkData(true);
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.dataset.cleanup === "plan") {
      button.disabled = true;
      try {
        await request(`/api/cleanup/${encode(button.dataset.id)}/plan`, {
          method: "POST",
          headers: mutationHeaders(),
          body: JSON.stringify({ projectId: button.dataset.project, repositoryId: button.dataset.repository, worktreeId: button.dataset.worktree }),
        });
        showNotice("정리 계획을 만들었습니다. 작업 화면에서 안전 근거와 승인을 확인하세요.");
        state.loaded.work = false;
        location.hash = "work";
        await loadWorkData(true);
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.dataset.integration === "check") {
      button.disabled = true;
      try {
        state.integrationHealth[button.dataset.id] = await request(`/api/integrations/${encode(button.dataset.id)}/check`, { method: "POST", headers: mutationHeaders(), body: "" });
        renderDiagnostics();
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.dataset.integration === "github-latest") {
      button.disabled = true;
      try {
        state.githubLatestRuns[button.dataset.id] = await request(`/api/integrations/${encode(button.dataset.id)}/github/latest-run`, { method: "POST", headers: mutationHeaders(), body: "" });
        renderDiagnostics();
        showNotice("최근 GitHub workflow 실행을 확인했습니다.");
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.dataset.integration === "jenkins-latest") {
      button.disabled = true;
      try {
        state.jenkinsLatestBuilds[button.dataset.id] = await request(`/api/integrations/${encode(button.dataset.id)}/jenkins/latest-build`, { method: "POST", headers: mutationHeaders(), body: "" });
        renderDiagnostics();
        showNotice("최근 Jenkins 빌드를 확인했습니다.");
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.dataset.integration === "kubernetes-status") {
      button.disabled = true;
      try {
        state.kubernetesStatuses[button.dataset.id] = await request(`/api/integrations/${encode(button.dataset.id)}/kubernetes/status`, { method: "POST", headers: mutationHeaders(), body: "" });
        renderDiagnostics();
        showNotice("Kubernetes Pod 상태를 확인했습니다.");
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.dataset.integration === "kubernetes-logs") {
      button.disabled = true;
      try {
        state.kubernetesLogs[button.dataset.id] = await request(`/api/integrations/${encode(button.dataset.id)}/kubernetes/logs`, { method: "POST", headers: mutationHeaders(), body: "" });
        renderDiagnostics();
        showNotice("현재 Pod의 최근 로그를 확인했습니다.");
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.dataset.integration === "edit") {
      const integration = state.integrations.find(item => item.id === button.dataset.id);
      if (integration) openEditor("integration", { integration });
      return;
    }
    if (button.dataset.profile === "edit") {
      const profile = state.profiles.find(item => item.metadata.id === button.dataset.id);
      if (profile) openEditor("profile", { profile });
      return;
    }
    if (button.id === "repository-sync-plan") {
      const projectID = document.getElementById("sync-project").value;
      button.disabled = true;
      try {
        state.repositorySyncResult = null;
        state.repositorySyncPlan = await request(`/api/projects/${encode(projectID)}/repository-sync/plan`, { method: "POST", headers: mutationHeaders(), body: "" });
        showNotice("프로젝트 저장소 최신화 계획을 만들었습니다. 제외된 저장소의 사유도 확인하세요.");
        await refreshAll();
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (["external-plan", "release-stage-plan", "release-production-plan"].includes(button.id)) {
      const target = document.getElementById("external-target").value.split("|");
      const groupID = document.getElementById("external-group").value;
      const isRelease = button.id !== "external-plan";
      const environment = button.id === "release-production-plan" ? "production" : "stage";
      button.disabled = true;
      try {
        const path = isRelease ? `/api/releases/${encode(groupID)}/plan` : `/api/external-work-groups/${encode(groupID)}/plan`;
        const body = { projectId: target[0], repositoryId: target[1], worktreeId: target[2] };
        if (isRelease) {
          body.environment = environment;
          body.expectedRevision = document.getElementById("expected-revision").value.trim();
        }
        await request(path, { method: "POST", headers: mutationHeaders(), body: JSON.stringify(body) });
        showNotice(isRelease ? `${environment === "production" ? "Production" : "Stage"} 릴리스 계획을 만들었습니다.` : "외부 작업 계획을 만들었습니다.");
        await loadWorkData(true);
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.dataset.specialAction) {
      const detail = state.actionDetails.find(item => item.plan.metadata.id === button.dataset.id);
      if (!detail) return;
      const actionType = detail.plan.spec.actionType;
      const path = button.dataset.specialAction === "cleanup"
        ? `/api/cleanup/plans/${encode(button.dataset.id)}/execute`
        : button.dataset.specialAction === "release"
          ? `/api/release-plans/${encode(button.dataset.id)}/execute`
          : `/api/external-work-plans/${encode(button.dataset.id)}/execute`;
      button.disabled = true;
      try {
        const result = await request(path, { method: "POST", headers: mutationHeaders(), body: JSON.stringify({ holder: "ui", idempotencyKey: `ui-${Date.now()}` }) });
        state.operationResults[button.dataset.id] = result;
        showNotice(`${actionType.startsWith("release.") ? "릴리스" : actionType === "cleanup.destructive" ? "정리" : "외부 작업"} 결과를 기록했습니다.`, result.status !== "succeeded");
        await loadWorkData(true);
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.dataset.repositorySync === "execute") {
      const plan = state.repositorySyncPlan;
      if (!plan?.plans?.length) return;
      button.disabled = true;
      try {
        const result = await request(`/api/projects/${encode(plan.projectId)}/repository-sync/execute`, {
          method: "POST",
          headers: mutationHeaders(),
          body: JSON.stringify({ planIds: plan.plans.map(item => item.metadata.id), requestId: `ui-sync-${Date.now()}` }),
        });
        const failed = (result.outcomes || []).filter(item => item.error).length;
        showNotice(failed ? `${result.outcomes.length - failed}개 저장소를 최신화했고 ${failed}개는 완료되지 않았습니다.` : "프로젝트 저장소를 모두 최신화했습니다.", failed > 0);
        state.repositorySyncPlan = null;
        state.repositorySyncResult = result;
        await refreshAll();
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.id === "action-plan") {
      const target = document.getElementById("action-target").value.split("|");
      button.disabled = true;
      try {
        await request("/api/actions/plans", {
          method: "POST",
          headers: mutationHeaders(),
          body: JSON.stringify({ id: `plan-${Date.now()}`, name: "저장소 새로고침", projectId: target[0], repositoryId: target[1], worktreeId: target[2], actionType: "repository.refresh" }),
        });
        showNotice("저장소 새로고침 계획을 만들었습니다.");
        await refreshAll();
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.dataset.action) {
      const action = button.dataset.action;
      const id = button.dataset.id;
      if (action === "runs") {
        state.expandedActions.has(id) ? state.expandedActions.delete(id) : state.expandedActions.add(id);
        renderWork();
        return;
      }
      button.disabled = true;
      try {
        if (action === "trust") {
          await request(`/ui/actions/plans/${encode(id)}/worktree-trust`, { method: "POST", headers: mutationHeaders(), body: "" });
          showNotice("이 Worktree를 실행 대상으로 표시했습니다.");
        } else if (action === "approve") {
          await request(`/ui/actions/plans/${encode(id)}/approval`, { method: "POST", headers: mutationHeaders(), body: "" });
          showNotice("Action 승인을 기록했습니다.");
        } else if (action === "execute") {
          await request(`/api/actions/plans/${encode(id)}/execute`, {
            method: "POST",
            headers: mutationHeaders(),
            body: JSON.stringify({ holder: "ui", idempotencyKey: `ui-${Date.now()}` }),
          });
          showNotice("Action을 실행했습니다.");
        }
        await refreshAll();
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (button.dataset.safeguard) {
      const action = button.dataset.safeguard;
      const id = button.dataset.id;
      button.disabled = true;
      try {
        let path = `/ui/safeguards/${encode(id)}/${action}`;
        let body = "";
        if (action === "shadow") {
          const owner = button.closest(".list-item").querySelector("[data-safeguard-owner]").value.trim();
          body = JSON.stringify({ owner });
        } else if (["positive", "false_positive"].includes(action)) {
          path = `/ui/safeguards/${encode(id)}/feedback`;
          body = JSON.stringify({ feedback: action });
        }
        await request(path, { method: "POST", headers: mutationHeaders(), body });
        showNotice(action === "shadow" ? "재발 방지 규칙의 모의 적용을 시작했습니다." : action === "activate" ? "재발 방지 규칙을 활성화했습니다." : action === "rollback" ? "재발 방지 규칙을 모의 적용으로 되돌렸습니다." : action === "retire" ? "재발 방지 규칙 사용을 종료했습니다." : "재발 방지 규칙 평가를 기록했습니다.");
        await loadDiagnosticsData(true);
      } catch (error) {
        showNotice(error.message, true);
      } finally {
        button.disabled = false;
      }
      return;
    }
    if (["guidance-check", "handoff-preview"].includes(button.id)) {
      const target = document.getElementById("guidance-target").value.split("|");
      button.disabled = true;
      try {
        const result = button.id === "guidance-check"
          ? await request(`/api/projects/${encode(target[0])}/repositories/${encode(target[1])}/worktrees/${encode(target[2])}/guidance`)
          : await request("/api/handoffs/preview", {
            method: "POST",
            headers: mutationHeaders(),
            body: JSON.stringify({ profileId: document.getElementById("handoff-profile").value, projectId: target[0], repositoryId: target[1], worktreeId: target[2], model: document.getElementById("handoff-model").value }),
          });
        state.guidanceMode = button.id === "guidance-check" ? "guidance" : "handoff";
        state.guidanceResult = result;
        renderDiagnostics();
      } catch (error) {
        state.guidanceMode = "error";
        state.guidanceResult = error;
        renderDiagnostics();
      } finally {
        button.disabled = false;
      }
    }
    if (button.dataset.handoffLaunch !== undefined) {
      const result = state.guidanceResult;
      if (!result?.previewDigest) return;
      button.disabled = true;
      try {
        const launch = await request("/api/handoffs/launch", {
          method: "POST",
          headers: mutationHeaders(),
          body: JSON.stringify({ profileId: result.profileId, projectId: result.projectId, repositoryId: result.repositoryId, worktreeId: result.worktreeId, model: result.model, previewDigest: result.previewDigest }),
        });
        state.guidanceResult = { ...result, launch };
        renderDiagnostics();
        showNotice("Agent를 새 창에서 열었습니다. 대화 기록은 수집하지 않습니다.");
      } catch (error) {
        showNotice(error.message, true);
        button.disabled = false;
      }
      return;
    }
  });

  document.querySelector(".skip-link").addEventListener("click", event => {
    event.preventDefault();
    focusElementByID("main-content");
  });

  document.addEventListener("keydown", event => {
    const region = event.target?.closest?.(".table-wrap[tabindex]");
    if (!region || !["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
    const maxScroll = region.scrollWidth - region.clientWidth;
    if (maxScroll <= 0) return;
    const step = Math.max(120, Math.floor(region.clientWidth * 0.8));
    const nextScroll = event.key === "ArrowLeft"
      ? Math.max(0, region.scrollLeft - step)
      : event.key === "ArrowRight"
        ? Math.min(maxScroll, region.scrollLeft + step)
        : event.key === "Home" ? 0 : maxScroll;
    if (nextScroll === region.scrollLeft) return;
    region.scrollLeft = nextScroll;
    event.preventDefault();
  });

  document.addEventListener("keydown", event => {
    if (currentRoute() !== "guide" || event.defaultPrevented) return;
    const tagName = document.activeElement?.tagName || "";
    if (["INPUT", "SELECT", "TEXTAREA"].includes(tagName)) return;
    if (event.key === "ArrowLeft") {
      event.preventDefault();
      setGuideSlide(guideSlideIndex() - 1);
    } else if (event.key === "ArrowRight") {
      event.preventDefault();
      setGuideSlide(guideSlideIndex() + (guideSlideIndex() === guideSlides.length - 1 ? -guideSlides.length + 1 : 1));
    }
  });

  document.addEventListener("click", event => {
    const recoveryLink = event.target.closest("a[data-provider-recovery]");
    if (!recoveryLink) return;
    const focusTarget = recoveryLink.dataset.focusTarget || "";
    pendingRouteFocus = focusTarget;
    if (currentRoute() === "diagnostics" && location.hash === "#diagnostics") {
      event.preventDefault();
      window.clearTimeout(routeFocusTimer);
      routeFocusTimer = window.setTimeout(() => {
        if (!focusElementByID(focusTarget)) focusElementByID("main-content");
      }, 0);
    }
  });

  document.getElementById("scan").addEventListener("click", async event => {
    await queueRepositoryStateRefresh(event.currentTarget);
  });

  document.addEventListener("click", event => {
    const button = event.target.closest?.("[data-repository-refresh]");
    if (!button) return;
    event.preventDefault();
    void queueRepositoryStateRefresh(button);
  });

  document.getElementById("env-doctor").addEventListener("click", async event => {
    const button = event.currentTarget;
    button.disabled = true;
    try {
      await request("/api/environment/doctor", { method: "POST", headers: mutationHeaders() });
      showNotice("개발 환경을 다시 점검했습니다.");
      await refreshAll();
    } catch (error) {
      showNotice(error.message, true);
    } finally {
      button.disabled = false;
    }
  });

  document.getElementById("provider-refresh").addEventListener("click", async event => {
    const button = event.currentTarget;
    button.disabled = true;
    try {
      await request("/api/environment/doctor", { method: "POST", headers: mutationHeaders() });
      showNotice("Provider 상태를 다시 점검했습니다.");
      await refreshAll();
    } catch (error) {
      showNotice(error.message, true);
    } finally {
      button.disabled = false;
    }
  });

  function setAssuranceDemoRoute(show) {
    history.replaceState(null, "", show ? "#assurance?demo=1" : "#assurance");
    setRoute();
    renderAssuranceDashboard();
  }

  document.getElementById("assurance-demo")?.addEventListener("click", () => setAssuranceDemoRoute(true));
  document.getElementById("assurance-demo-exit")?.addEventListener("click", () => setAssuranceDemoRoute(false));

  document.getElementById("assurance-refresh").addEventListener("click", async event => {
    const button = event.currentTarget;
    button.disabled = true;
    try {
      await refreshAll();
      showNotice("검증 대시보드를 새로 고쳤습니다.");
    } finally {
      button.disabled = false;
    }
  });

  document.getElementById("assurance-filter-reset").addEventListener("click", () => {
    state.assuranceFilters.days = "30";
    state.assuranceFilters.provider = "";
    state.assuranceFilters.model = "";
    state.assuranceFilters.project = "";
    state.assuranceEffectFilter = "all";
    syncAssuranceRouteState();
    void refreshAssuranceFilter();
  });

  const candidates = document.getElementById("repository-candidates");
  const pathInput = document.getElementById("path");
  async function discoverRepositories() {
    if (!pathInput.value.trim()) {
      candidates.dataset.discovered = "false";
      candidates.textContent = "먼저 폴더를 선택하세요.";
      return;
    }
    candidates.textContent = "Git 저장소를 찾는 중입니다.";
    try {
      const items = await request("/api/projects/discover", {
        method: "POST",
        headers: mutationHeaders(),
        body: JSON.stringify({ path: pathInput.value }),
      });
      candidates.dataset.discovered = "true";
      candidates.innerHTML = items.length
        ? `<strong>${items.length}개 저장소를 찾았습니다.</strong>${items.map(item => `<label><input type="checkbox" data-repository-path value="${escapeHTML(item.path)}" checked><span>${escapeHTML(item.name)}<br><code>${escapeHTML(item.path)}</code></span></label>`).join("")}`
        : "이 폴더 아래에서 Git 저장소를 찾지 못했습니다.";
    } catch (error) {
      candidates.dataset.discovered = "false";
      candidates.textContent = error.message;
    }
  }

  async function openFolderPicker(opener) {
    const button = opener || document.getElementById("pick-folder");
    if (button) button.disabled = true;
    try {
      const result = await request("/api/folder-picker", { method: "POST", headers: mutationHeaders() });
      const selectedPath = String(result?.path || "").trim();
      if (!selectedPath) return;
      const registerButton = document.getElementById("show-register");
      if (currentRoute() !== "projects") {
        registerOpener = registerButton;
        location.hash = "projects";
        setRoute();
      }
      setRegisterPanelOpen(true, registerOpener || registerButton);
      pathInput.value = selectedPath;
      await discoverRepositories();
      pathInput.focus({ preventScroll: true });
    } catch (error) {
      showNotice(error.message, true);
    } finally {
      if (button) button.disabled = false;
    }
  }
  document.getElementById("pick-folder")?.addEventListener("click", event => { void openFolderPicker(event.currentTarget); });
  document.getElementById("home-pick-folder")?.addEventListener("click", event => { void openFolderPicker(event.currentTarget); });
  document.getElementById("find-repositories").addEventListener("click", discoverRepositories);
  const projectImportFile = document.getElementById("import-project-file");
  const projectImportStatus = document.getElementById("project-import-status");
  const setProjectImportStatus = message => {
    if (projectImportStatus) projectImportStatus.textContent = message;
  };
  document.getElementById("import-project").addEventListener("click", () => {
    setProjectImportStatus("프로젝트 설정 파일 선택 대기 중입니다.");
    projectImportFile?.click();
  });
  projectImportFile?.addEventListener("cancel", () => {
    setProjectImportStatus("프로젝트 설정 파일을 선택하지 않았습니다.");
  });
  projectImportFile?.addEventListener("change", async event => {
    const file = event.target.files?.[0];
    if (!file) {
      setProjectImportStatus("프로젝트 설정 파일을 선택하지 않았습니다.");
      return;
    }
    setProjectImportStatus("프로젝트 설정 파일을 읽는 중입니다…");
    try {
      const project = await request("/api/projects/import", { method: "POST", headers: mutationHeaders(), body: await file.text() });
      state.activeProjectID = project.metadata.id;
      pendingProjectFocusID = project.metadata.id;
      setProjectImportStatus("프로젝트 설정 파일을 가져왔습니다.");
      showNotice("프로젝트 설정을 가져왔습니다. 비밀 값은 포함되지 않습니다.");
      await refreshAll();
    } catch (error) {
      setProjectImportStatus("프로젝트 설정 파일을 가져오지 못했습니다.");
      showNotice(error.message, true);
    } finally {
      event.target.value = "";
    }
  });
  document.getElementById("assurance-measurement-import")?.addEventListener("click", () => document.getElementById("assurance-measurement-file")?.click());
  document.getElementById("assurance-measurement-file")?.addEventListener("change", async event => {
    const input = event.target;
    const file = input.files?.[0];
    if (!file) return;
    const maximumBytes = 512 * 1024;
    if (file.size > maximumBytes) {
      state.assuranceMeasurement = { status: "error", data: state.assuranceMeasurement?.data || null, error: "manifest가 512 KiB 제한을 초과했습니다." };
      renderAssuranceMeasurementDashboard();
      showNotice("측정 manifest가 너무 큽니다. 512 KiB 이하의 파일을 선택하세요.", true);
      input.value = "";
      return;
    }
    state.assuranceMeasurement = { status: "importing", data: state.assuranceMeasurement?.data || null, error: "" };
    renderAssuranceMeasurementDashboard();
    try {
      await request("/api/assurance/measurement-runs/import", { method: "POST", headers: mutationHeaders(), body: await file.text() });
      showNotice("측정 manifest를 가져왔습니다.");
      await loadAssuranceMeasurementData();
    } catch (error) {
      state.assuranceMeasurement = { status: "error", data: state.assuranceMeasurement?.data || null, error: error.message || "측정 manifest를 가져오지 못했습니다." };
      renderAssuranceMeasurementDashboard();
      showNotice(error.message, true);
    } finally {
      input.value = "";
    }
  });
  document.getElementById("add-form").addEventListener("submit", async event => {
    event.preventDefault();
    const selected = [...candidates.querySelectorAll("input[data-repository-path]:checked")].map(input => input.value);
    if (candidates.dataset.discovered === "true" && !selected.length) {
      showNotice("등록할 저장소를 하나 이상 선택하세요.", true);
      return;
    }
    const submit = event.submitter || event.currentTarget.querySelector('button[type="submit"]');
    submit.disabled = true;
    try {
      const input = { name: document.getElementById("name").value, path: pathInput.value };
      if (selected.length) input.paths = selected;
      const project = await request("/api/projects", {
        method: "POST",
        headers: mutationHeaders(),
        body: JSON.stringify(input),
      });
      state.activeProjectID = project.metadata.id;
      pendingProjectFocusID = project.metadata.id;
      event.target.reset();
      candidates.dataset.discovered = "false";
      candidates.textContent = "폴더를 선택하면 아래의 Git 저장소를 읽기 전용으로 찾습니다.";
      setRegisterPanelOpen(false);
      registerOpener = null;
      showNotice("프로젝트를 등록했습니다. 저장소 상태를 새로 고치는 중입니다…");
      await queueRepositoryStateRefresh(null, project.metadata.id);
    } catch (error) {
      showNotice(error.message, true);
    } finally {
      submit.disabled = false;
    }
  });

  document.addEventListener("submit", async event => {
    const form = event.target.closest?.("form[data-assurance-retry]");
    if (!form) return;
    event.preventDefault();
    const input = form.querySelector('input[name="prompt"]');
    const submit = event.submitter || form.querySelector('button[type="submit"]');
    const prompt = input?.value.trim() || "";
    if (!prompt) {
      showNotice("새 prompt를 입력하세요.", true);
      input?.focus();
      return;
    }
    if (submit) submit.disabled = true;
    try {
      await request(`/api/assurance/invocations/${encode(form.dataset.assuranceRetry)}/retry`, {
        method: "POST",
        headers: mutationHeaders(),
        body: JSON.stringify({ prompt }),
      });
      showNotice("실패/중단 실행을 재시도했습니다.");
      await refreshAll();
    } catch (error) {
      showNotice(error.message, true);
    } finally {
      if (submit) submit.disabled = false;
    }
  });

  document.addEventListener("change", event => {
    if (["home-target", "quality-target"].includes(event.target.id)) {
      rememberTarget(event.target.value);
      state.qualityRunError = "";
      if (event.target.id === "quality-target") {
        qualitySetupRequestSequence += 1;
        state.qualitySetup = { status: "idle", targetValue: event.target.value, requestKey: "", data: null, error: "", preferenceReset: false };
      }
      if (event.target.id === "quality-target" && activeRoute === "work") history.replaceState(null, "", `#work?target=${encode(event.target.value)}`);
      renderHome();
      renderQualityWorkSurface();
      if (event.target.id === "quality-target") void loadQualitySetupForTarget(selectedTarget(), true);
    }
    if (event.target.id === "quality-inspection-target") {
      rememberTarget(event.target.value);
      const workflow = ensureQualityInspectionState();
      workflow.selectedPlanID = "";
      workflow.selectedPlan = null;
      workflow.lastRun = null;
      workflow.comparison = { status: "idle", data: null, error: "", beforeID: "", afterID: "" };
      workflow.selectedProposalID = "";
      workflow.selectedProposal = null;
      workflow.proposals = [];
      workflow.proposalStatus = "idle";
      workflow.proposalError = "";
      workflow.toolPreview = { status: "idle", data: null, error: "", input: null };
      workflow.toolActionPlan = { status: "idle", data: null, error: "", approvalStatus: "idle", executionStatus: "idle" };
      renderAssuranceDashboard();
      if (activeRoute === "assurance") void loadQualityInspectionTargetData(event.target.value);
    }
    if (event.target.matches("input[data-quality-inspection-ai]")) {
      ensureQualityInspectionState().aiEnabled = event.target.checked === true;
      renderAssuranceDashboard();
      document.querySelector("input[data-quality-inspection-ai]")?.focus({ preventScroll: true });
    }
    if (event.target.matches("input[data-quality-language-mode]")) {
      const target = selectedTarget();
      if (!target) return;
      const preference = qualitySetupPreferenceForTarget(target, state.qualitySetup.data);
      saveQualitySetupPreference(target, state.qualitySetup.data, { mode: event.target.value === "manual" ? "manual" : "auto", languages: event.target.value === "manual" ? preference.languages : [] });
      state.qualitySetup.preferenceReset = false;
      const mode = event.target.value;
      renderQualityWorkSurface();
      document.querySelector(`#quality-setup [data-quality-language-mode][value="${CSS.escape(mode)}"]`)?.focus({ preventScroll: true });
    }
    if (event.target.matches("input[data-quality-language]")) {
      const target = selectedTarget();
      if (!target) return;
      const languages = [...document.querySelectorAll("#quality-setup [data-quality-language]:checked")].map(input => input.dataset.qualityLanguage).filter(Boolean);
      saveQualitySetupPreference(target, state.qualitySetup.data, { mode: "manual", languages });
      state.qualitySetup.preferenceReset = false;
      const language = event.target.dataset.qualityLanguage;
      renderQualityWorkSurface();
      document.querySelector(`#quality-setup [data-quality-language="${CSS.escape(language)}"]`)?.focus({ preventScroll: true });
    }
    if (event.target.id === "finding-severity") {
      state.findingFilters.severity = event.target.value;
      renderProjects();
      document.getElementById("finding-severity")?.focus({ preventScroll: true });
    }
    if (event.target.id === "finding-state") {
      state.findingFilters.state = event.target.value;
      renderProjects();
      document.getElementById("finding-state")?.focus({ preventScroll: true });
    }
    if (["assurance-days-filter", "assurance-provider-filter", "assurance-model-filter", "assurance-project-filter"].includes(event.target.id)) {
      state.assuranceFilters.days = document.getElementById("assurance-days-filter").value;
      state.assuranceFilters.provider = document.getElementById("assurance-provider-filter").value;
      state.assuranceFilters.model = document.getElementById("assurance-model-filter").value;
      state.assuranceFilters.project = document.getElementById("assurance-project-filter").value;
      syncAssuranceRouteState();
      void refreshAssuranceFilter();
    }
    if (event.target.id === "assurance-effect-filter") {
      state.assuranceEffectFilter = event.target.value;
      syncAssuranceRouteState();
      renderAssuranceDashboard();
    }
  });

  document.addEventListener("keydown", event => {
    if (event.target?.id !== "assurance-filter-reset" || !["Enter", " "].includes(event.key)) return;
    event.preventDefault();
    event.target.click();
  });

  document.addEventListener("input", event => {
    if (!event.target.matches("[data-safeguard-owner]")) return;
    const submit = event.target.closest(".list-item").querySelector("[data-owner-submit]");
    submit.disabled = event.target.value.trim() === "";
  });

  document.getElementById("editor-cancel").addEventListener("click", () => {
    editorDialog.close();
    restoreDialogFocus(editorOpener);
  });
  editorDialog.addEventListener("cancel", event => {
    event.preventDefault();
    editorDialog.close();
    restoreDialogFocus(editorOpener);
  });
  document.getElementById("editor-form").addEventListener("submit", async event => {
    event.preventDefault();
    const submit = document.getElementById("editor-submit");
    submit.disabled = true;
    try {
      const message = await submitEditor();
      editorDialog.close();
      showNotice(message);
      await refreshAll();
      restoreDialogFocus(editorOpener);
    } catch (error) {
      showNotice(error.message, true);
    } finally {
      submit.disabled = false;
    }
  });

  unregisterInput.addEventListener("input", () => {
    document.getElementById("unregister-submit").disabled = unregisterInput.value !== unregisterTarget?.name;
  });
  document.getElementById("unregister-cancel").addEventListener("click", () => {
    unregisterDialog.close();
    restoreDialogFocus(unregisterOpener);
  });
  unregisterDialog.addEventListener("cancel", event => {
    event.preventDefault();
    unregisterDialog.close();
    restoreDialogFocus(unregisterOpener);
  });
  document.getElementById("unregister-form").addEventListener("submit", async event => {
    event.preventDefault();
    if (!unregisterTarget || unregisterInput.value !== unregisterTarget.name) return;
    const path = unregisterTarget.kind === "project"
      ? `/api/projects/${encode(unregisterTarget.projectID)}`
      : unregisterTarget.kind === "profile"
          ? `/api/agent-profiles/${encode(unregisterTarget.profileID)}`
          : unregisterTarget.kind === "integration"
            ? `/api/integrations/${encode(unregisterTarget.integrationID)}`
            : unregisterTarget.kind === "external-group"
              ? `/api/external-work-groups/${encode(unregisterTarget.externalGroupID)}`
            : unregisterTarget.kind === "runbook"
            ? `/api/runbooks/${encode(unregisterTarget.runbookID)}`
        : `/api/projects/${encode(unregisterTarget.projectID)}/repositories/${encode(unregisterTarget.repositoryID)}`;
    const submit = document.getElementById("unregister-submit");
    submit.disabled = true;
    try {
      await request(path, { method: "DELETE", headers: mutationHeaders() });
      if (unregisterTarget.kind === "project") state.activeProjectID = "";
      unregisterDialog.close();
      showNotice(unregisterTarget.kind === "profile" ? "Agent Profile을 제거했습니다." : unregisterTarget.kind === "integration" ? "연동 설정을 제거했습니다." : unregisterTarget.kind === "external-group" ? "Jenkins 대상 그룹을 제거했습니다." : unregisterTarget.kind === "runbook" ? "PowerShell runbook을 제거했습니다." : "등록을 해제했습니다. 원본 저장소 파일은 변경하지 않았습니다.");
      await refreshAll();
      restoreDialogFocus(unregisterOpener);
    } catch (error) {
      showNotice(error.message, true);
      submit.disabled = false;
    }
  });

  window.addEventListener("hashchange", setRoute);
  if (!location.hash) history.replaceState(null, "", "#home");
  setRoute();
  refreshAll();
  window.setInterval(refreshAll, 30000);
})();
