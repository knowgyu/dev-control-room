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
    workItems: [],
    actionDetails: [],
    repositorySyncPlan: null,
    repositorySyncResult: null,
    cleanup: [],
    safeguards: [],
    profiles: [],
    integrations: [],
    runbooks: [],
    integrationHealth: {},
    githubLatestRuns: {},
    jenkinsLatestBuilds: {},
    kubernetesStatuses: {},
    kubernetesLogs: {},
    activeProjectID: "",
    checkRuns: new Map(),
    expandedChecks: new Set(),
    expandedActions: new Set(),
    guidanceResult: null,
    guidanceMode: "",
    findingFilters: { severity: "", state: "active" },
    surfaceErrors: { checksets: "", actions: "", cleanup: "", safeguards: "", profiles: "", integrations: "", runbooks: "" },
    loaded: { work: false, diagnostics: false },
    loading: { work: false, diagnostics: false },
  };
  const routeTitles = {
    home: "홈",
    projects: "프로젝트",
    work: "작업",
    diagnostics: "진단",
    activity: "기록",
  };
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
    ["Check out an intentional branch before making changes.", "변경하기 전에 의도한 브랜치를 checkout하세요."],
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
  const localize = value => {
    const text = String(value ?? "");
    if (messageTranslations.has(text)) return messageTranslations.get(text);
    const drift = text.match(/^Repository differs from upstream \((\d+) ahead, (\d+) behind\)$/);
    if (drift) return `upstream과 차이가 있습니다. ahead ${drift[1]}, behind ${drift[2]}`;
    const scans = text.match(/^(manual|scheduled) scan completed for (\d+) project\(s\)$/);
    if (scans) return `${scans[1] === "manual" ? "수동" : "예약"} 점검을 완료했습니다. 프로젝트 ${scans[2]}개`;
    const projectAdded = text.match(/^Project added with (\d+) repositories$/);
    if (projectAdded) return `프로젝트와 저장소 ${projectAdded[1]}개를 등록했습니다.`;
    return text;
  };
  const formatDate = value => value ? new Intl.DateTimeFormat("ko-KR", {
    dateStyle: "short",
    timeStyle: "short",
  }).format(new Date(value)) : "아직 없음";
  const projectRepositories = () => (state.snapshot.projects || []).flatMap(project =>
    (project.repos || []).map(repository => ({ ...repository, projectID: project.id, projectName: project.name })));
  const openFindings = () => state.findings.filter(item => ["open", "acknowledged"].includes(item.spec.state));
  const registryProject = projectID => state.registryProjects.find(project => project.metadata.id === projectID);
  const targetOptions = () => projectRepositories().flatMap(repository => (repository.worktrees || []).map(worktree => ({
    value: `${repository.projectID}|${repository.id}|${worktree.metadata.id}`,
    label: `${repository.projectName} / ${repository.id} / ${worktree.metadata.id}`,
  })));
  const currentRoute = () => routeTitles[location.hash.slice(1).split("/")[0]] ? location.hash.slice(1).split("/")[0] : "home";

  async function request(path, options = {}) {
    const response = await fetch(path, options);
    const body = await response.json();
    if (!response.ok || !body.ok) throw new Error(body.error?.message || `요청에 실패했습니다. (${response.status})`);
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
    document.querySelectorAll("[data-view]").forEach(view => { view.hidden = view.dataset.view !== active; });
    document.querySelectorAll("[data-route]").forEach(link => {
      if (link.dataset.route === active) link.setAttribute("aria-current", "page");
      else link.removeAttribute("aria-current");
    });
    document.getElementById("view-title").textContent = routeTitles[active];
    document.title = `${routeTitles[active]} · Dev Control Room`;
    window.scrollTo({ top: 0 });
    document.getElementById("main-content").focus({ preventScroll: true });
    if (initialized) void loadRouteData(active, false);
  }

  function surfaceError(message, route) {
    return `<div class="list-item state-bad" role="alert"><strong>데이터를 불러오지 못했습니다.</strong><p>${escapeHTML(message)}</p><button class="button small" type="button" data-retry="${escapeHTML(route)}">다시 시도</button></div>`;
  }

  function findingCard(item) {
    const severity = String(item.spec.severity || "info");
    const scope = [item.spec.projectId, item.spec.repositoryId].filter(Boolean).join(" / ");
    return `<article class="finding ${escapeHTML(severity)}">
      <div class="finding-header"><h3>${escapeHTML(localize(item.spec.summary))}</h3><span class="chip ${severity === "info" ? "" : severity === "attention" ? "warn" : "bad"}">${escapeHTML(severityLabels[severity] || severity)}</span></div>
      <p class="next">다음 단계: ${escapeHTML(localize(item.spec.recommendedNextAction))}</p>
      <details><summary>근거와 상태 보기</summary><dl class="detail-grid">
        <div><dt>범위</dt><dd>${escapeHTML(scope || "전체")}</dd></div>
        <div><dt>상태</dt><dd>${escapeHTML(label(item.spec.state))}</dd></div>
        <div><dt>확신도</dt><dd>${escapeHTML(confidenceLabels[item.spec.confidence] || item.spec.confidence || "알 수 없음")}</dd></div>
        <div><dt>처음 확인</dt><dd>${escapeHTML(formatDate(item.spec.firstObserved))}</dd></div>
        <div><dt>최근 확인</dt><dd>${escapeHTML(formatDate(item.spec.lastObserved))}</dd></div>
        <div class="wide"><dt>근거 참조</dt><dd>${escapeHTML((item.spec.evidenceRefs || []).join(", ") || "없음")}</dd></div>
      </dl>${item.spec.state === "open" ? `<div class="item-actions"><button class="button small" type="button" data-finding="acknowledge" data-id="${escapeHTML(item.metadata.id)}">확인함으로 표시</button></div>` : ""}</details>
    </article>`;
  }

  function renderHome() {
    const projects = state.snapshot.projects || [];
    const repositories = projectRepositories();
    const findings = openFindings();
    document.getElementById("m-projects").textContent = projects.length;
    document.getElementById("m-repos").textContent = repositories.length;
    document.getElementById("m-findings").textContent = findings.length;
    document.getElementById("m-scan").textContent = formatDate(state.snapshot.generated_at);
    document.getElementById("m-environment").textContent = state.environment.available ? "정상" : "확인 필요";

    const ordered = findings.slice().sort((left, right) => {
      const rank = { critical: 4, high: 3, attention: 2, info: 1 };
      return (rank[right.spec.severity] || 0) - (rank[left.spec.severity] || 0);
    });
    document.getElementById("home-findings").innerHTML = ordered.length
      ? `<div class="finding-list">${ordered.slice(0, 5).map(findingCard).join("")}</div>`
      : '<div class="empty-state"><strong>지금 확인할 항목이 없습니다.</strong><span>새 상태를 확인하려면 ‘지금 점검’을 실행하세요.</span></div>';

    document.getElementById("home-projects").innerHTML = projects.length
      ? `<div class="item-list">${projects.map(project => {
        const projectFindings = findings.filter(item => item.spec.projectId === project.id);
        const high = projectFindings.filter(item => ["high", "critical"].includes(item.spec.severity)).length;
        return `<button class="project-card" type="button" data-open-project="${escapeHTML(project.id)}"><strong>${escapeHTML(project.name)}</strong><span>${escapeHTML(project.repos.length)}개 저장소 · 확인 항목 ${projectFindings.length}개${high ? ` · 높음 ${high}개` : ""}</span></button>`;
      }).join("")}</div>`
      : '<div class="empty-state"><strong>등록된 프로젝트가 없습니다.</strong><span>프로젝트 화면에서 첫 저장소를 등록하세요.</span></div>';

    const recentRuns = state.events.filter(item => /(action|check|run|scan)/i.test(item.spec.type)).slice().reverse();
    document.getElementById("home-runs").innerHTML = recentRuns.length
      ? `<div class="item-list">${recentRuns.slice(0, 5).map(item => `<article class="list-item"><div class="list-item-header"><h3>${escapeHTML(localize(item.spec.summary))}</h3><span class="chip">기록</span></div><p class="meta">${escapeHTML(formatDate(item.spec.occurredAt))}</p></article>`).join("")}</div>`
      : '<div class="empty-state"><strong>아직 실행 결과가 없습니다.</strong><span>작업 화면에서 검토된 점검이나 Action을 실행할 수 있습니다.</span></div>';
  }

  function projectStatus(repository) {
    if (repository.error || repository.unsafe_cleanup) return { className: "bad", text: "확인 필요" };
    if (repository.dirty || repository.behind) return { className: "warn", text: "주의" };
    return { className: "ok", text: "정상" };
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
        return `<button class="project-card ${project.id === state.activeProjectID ? "selected" : ""}" type="button" data-project="${escapeHTML(project.id)}"><strong>${escapeHTML(project.name)}</strong><span>${escapeHTML(project.id)}</span><span class="project-counts"><span class="chip">저장소 ${project.repos.length}</span><span class="chip ${count ? "warn" : "ok"}">확인 항목 ${count}</span></span></button>`;
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
        return `<article class="repository-card"><div class="repository-header"><div><h3>${escapeHTML(repositoryName)}</h3><p class="meta">${escapeHTML(repository.id)}</p><div class="repository-path"><code>${escapeHTML(repository.path)}</code></div></div><div class="repository-actions"><span class="chip ${status.className}">${status.text}</span><button class="button small" type="button" data-repository-action="edit" data-project="${escapeHTML(project.id)}" data-repository="${escapeHTML(repository.id)}">정보 변경</button><button class="button small" type="button" data-unregister="repository" data-project="${escapeHTML(project.id)}" data-repository="${escapeHTML(repository.id)}" data-name="${escapeHTML(repositoryName)}" ${onlyRepository ? "disabled" : ""}>저장소 등록 해제</button></div></div>
          <div class="repository-summary"><span class="chip">브랜치 ${escapeHTML(repository.branch || "detached")}</span><span class="chip">ahead ${escapeHTML(repository.ahead || 0)}</span><span class="chip">behind ${escapeHTML(repository.behind || 0)}</span><span class="chip">Worktree ${(repository.worktrees || []).length}</span></div>
          ${onlyRepository ? '<p class="meta">마지막 저장소는 개별 해제할 수 없습니다. 프로젝트 등록 해제를 사용하세요.</p>' : ""}
          <details><summary>Worktree 상세 보기</summary><div class="worktree-list">${(repository.worktrees || []).length ? repository.worktrees.map(worktree => `<div class="worktree"><strong>${escapeHTML(worktree.metadata.id)} · ${worktree.spec.primary ? "기본" : "연결됨"}</strong><code>${escapeHTML(worktree.spec.canonicalPath)}</code><div class="meta">${escapeHTML(worktree.spec.branch || "detached")} · HEAD ${escapeHTML(worktree.spec.head || "확인 불가")} · ${worktree.spec.dirty ? "변경 있음" : "변경 없음"} · ${worktree.spec.untracked ? "추적하지 않은 파일 있음" : "추적되지 않은 파일 없음"} · upstream ${escapeHTML(worktree.spec.upstream || "없음")} ${escapeHTML(worktree.spec.ahead || 0)}/${escapeHTML(worktree.spec.behind || 0)} · ${worktree.spec.locked ? "잠김" : "잠기지 않음"} · ${worktree.spec.prunable ? "정리 가능 표시" : "유지됨"} · ${escapeHTML(label(worktree.spec.trust))} · ${escapeHTML(worktree.spec.tombstonedAt ? "관찰 종료" : worktree.spec.error || "현재")}</div></div>`).join("") : '<div class="empty-state"><span>관찰된 Worktree가 없습니다.</span></div>'}</div></details>
        </article>`;
      }).join("")}</div>
      <div class="panel-heading section-heading"><div><p class="eyebrow">확인할 항목</p><h2>이 프로젝트의 확인할 항목</h2></div><div class="toolbar"><select id="finding-severity" aria-label="심각도 필터"><option value="">모든 심각도</option>${Object.entries(severityLabels).map(([value, text]) => `<option value="${value}" ${state.findingFilters.severity === value ? "selected" : ""}>${text}</option>`).join("")}</select><select id="finding-state" aria-label="상태 필터"><option value="active" ${state.findingFilters.state === "active" ? "selected" : ""}>열림 및 확인함</option><option value="">모든 상태</option>${["open", "acknowledged", "resolved", "suppressed", "expired"].map(value => `<option value="${value}" ${state.findingFilters.state === value ? "selected" : ""}>${escapeHTML(label(value))}</option>`).join("")}</select></div></div>
      ${visibleFindings.length ? `<div class="finding-list">${visibleFindings.map(findingCard).join("")}</div>` : '<div class="empty-state"><strong>조건에 맞는 확인 항목이 없습니다.</strong><span>필터를 바꾸거나 새 점검을 실행하세요.</span></div>'}`;
  }

  function renderCheckRun(run) {
    return `<article class="run-detail"><div class="list-item-header"><strong>${escapeHTML(label(run.spec.status))}</strong><span class="meta">${escapeHTML(formatDate(run.spec.completedAt || run.spec.startedAt))}</span></div><dl class="detail-grid"><div><dt>Worktree</dt><dd>${escapeHTML(run.spec.worktreeId)}</dd></div><div><dt>HEAD</dt><dd><code>${escapeHTML(run.spec.head)}</code></dd></div></dl><div class="step-results">${(run.spec.steps || []).map(step => `<details ${step.status === "failed" ? "open" : ""}><summary>${escapeHTML(step.stepId)} · ${escapeHTML(label(step.status))}</summary><dl class="detail-grid"><div><dt>실행 종료 코드</dt><dd>${escapeHTML(step.exitCode ?? (step.status === "passed" ? 0 : "없음"))}</dd></div></dl>${step.stdout ? `<div><strong class="result-label">표준 출력</strong><pre class="command-output">${escapeHTML(step.stdout)}</pre></div>` : ""}${step.stderr ? `<div><strong class="result-label">오류 출력</strong><pre class="command-output error-output">${escapeHTML(step.stderr)}</pre></div>` : ""}${!step.stdout && !step.stderr ? '<p class="meta">기록된 출력이 없습니다.</p>' : ""}</details>`).join("")}</div></article>`;
  }

  function checksetCard(checkset, repository) {
    const runs = state.checkRuns.get(checkset.metadata.id) || [];
    const expanded = state.expandedChecks.has(checkset.metadata.id);
    return `<article class="list-item"><div class="list-item-header"><div><h3>${escapeHTML(checkset.metadata.name)}</h3><p class="meta">${escapeHTML(repository.projectName)} / ${escapeHTML(repository.id)} / ${escapeHTML(checkset.spec.worktreeId)}</p></div><span class="chip">${escapeHTML(label(checkset.spec.state))}</span></div><details><summary>점검 단계와 근거</summary><dl class="detail-grid"><div><dt>HEAD</dt><dd><code>${escapeHTML(checkset.spec.head)}</code></dd></div><div><dt>제안</dt><dd>${escapeHTML(checkset.spec.proposalId)}</dd></div></dl>${(checkset.spec.steps || []).map(step => `<div class="command-card"><strong>${escapeHTML(step.name)}</strong><code>${escapeHTML([step.command.executable, ...(step.command.arguments || [])].join(" "))}</code><span class="meta">제한 시간 ${escapeHTML(step.command.timeoutSeconds)}초</span></div>`).join("")}</details><div class="item-actions"><button class="button small" type="button" data-checkset="apply" data-id="${escapeHTML(checkset.metadata.id)}" ${checkset.spec.state === "draft" ? "" : "disabled"}>적용</button><button class="button primary small" type="button" data-checkset="run" data-id="${escapeHTML(checkset.metadata.id)}" ${checkset.spec.state === "applied" ? "" : "disabled"}>실행</button><button class="button small" type="button" data-checkset="results" data-id="${escapeHTML(checkset.metadata.id)}">${expanded ? "결과 닫기" : "결과 보기"}</button></div>${expanded ? `<div class="result-box">${runs.length ? runs.slice().reverse().map(renderCheckRun).join("") : "아직 실행 결과가 없습니다."}</div>` : runs.length ? `<p class="meta">최근 결과 ${escapeHTML(label(runs[runs.length - 1].spec.status))}</p>` : ""}</article>`;
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

  function renderWork() {
    const targets = targetOptions();
    const proposalHTML = state.workItems.flatMap(item => (item.proposals || []).map(proposal => proposalCard(proposal, item))).join("");
    document.getElementById("proposal-ui").innerHTML = `${state.surfaceErrors.checksets ? surfaceError(state.surfaceErrors.checksets, "work") : ""}<div class="toolbar"><select id="discovery-target" aria-label="발견 대상 Worktree">${targets.length ? targets.map(target => `<option value="${escapeHTML(target.value)}">${escapeHTML(target.label)}</option>`).join("") : '<option value="">관찰된 Worktree 없음</option>'}</select><button id="discover-worktree" class="button primary" type="button" ${targets.length ? "" : "disabled"}>기존 점검 찾기</button></div>${proposalHTML ? `<div class="item-list">${proposalHTML}</div>` : '<div class="empty-state"><strong>검토할 제안이 없습니다.</strong><span>Worktree를 선택해 기존 점검 명령을 찾아보세요.</span></div>'}`;

    const checksetHTML = state.workItems.flatMap(item => (item.checksets || []).map(checkset => checksetCard(checkset, item.repository))).join("");
    const checksetContent = checksetHTML
      ? `<div class="item-list">${checksetHTML}</div>`
      : '<div class="empty-state"><strong>실행할 Pre-PR 점검이 없습니다.</strong><span>제안을 적용한 뒤 Checkset으로 만드세요.</span></div>';
    document.getElementById("checksets").innerHTML = checksetContent;

    const plans = state.actionDetails.map(detail => {
      const admission = detail.status.admission;
      const latest = detail.runs?.[0];
      const resultVisible = state.expandedActions.has(detail.plan.metadata.id);
      const actionButtons = admission === "approval_required"
        ? `<button class="button small" data-action="approve" data-id="${escapeHTML(detail.plan.metadata.id)}">승인 요청</button>`
        : admission === "eligible"
          ? `<button class="button primary small" data-action="execute" data-id="${escapeHTML(detail.plan.metadata.id)}">실행</button>`
          : "";
      return `<article class="list-item"><div class="list-item-header"><div><h3>${escapeHTML(detail.plan.metadata.name)}</h3><p class="meta">${escapeHTML(detail.plan.spec.projectId)} / ${escapeHTML(detail.plan.spec.repositoryId)} / ${escapeHTML(detail.plan.spec.worktreeId)}</p></div><span class="chip ${admission === "eligible" ? "ok" : admission === "approval_required" ? "warn" : "bad"}">${escapeHTML(label(admission))}</span></div><details><summary>계획과 승인 근거</summary><dl class="detail-grid"><div><dt>Action</dt><dd><code>${escapeHTML(detail.plan.spec.actionType)}</code></dd></div><div><dt>위험 등급</dt><dd>${escapeHTML(label(detail.plan.spec.risk))}</dd></div><div><dt>정책 판단</dt><dd>${escapeHTML(label(detail.plan.spec.policyDecision))}</dd></div><div><dt>요청 시각</dt><dd>${escapeHTML(formatDate(detail.plan.spec.requestedAt))}</dd></div><div class="wide"><dt>실행 명령</dt><dd><code>${escapeHTML([detail.plan.spec.execution?.executable, ...(detail.plan.spec.execution?.arguments || [])].join(" "))}</code></dd></div><div class="wide"><dt>승인 기록</dt><dd>${detail.status.approvals?.length ? detail.status.approvals.map(approval => `${escapeHTML(label(approval.spec.status))} · ${escapeHTML(formatDate(approval.spec.decidedAt))}`).join("<br>") : "없음"}</dd></div><div class="wide"><dt>감사 이벤트</dt><dd>${detail.status.events?.length ? detail.status.events.map(item => `${escapeHTML(item.spec.eventType)} · ${escapeHTML(formatDate(item.spec.occurredAt))}`).join("<br>") : "없음"}</dd></div></dl></details><div class="item-actions"><button class="button small" type="button" data-action="trust" data-id="${escapeHTML(detail.plan.metadata.id)}">실행 대상으로 표시</button>${actionButtons}<button class="button small" type="button" data-action="runs" data-id="${escapeHTML(detail.plan.metadata.id)}">${resultVisible ? "결과 닫기" : "결과 보기"}</button></div>${resultVisible ? `<div class="result-box">${detail.runs.length ? detail.runs.map(run => renderActionRun(run, detail.status.events)).join("") : "아직 실행 결과가 없습니다."}</div>` : latest ? `<p class="meta">최근 결과 ${escapeHTML(label(latest.spec.status))}</p>` : ""}</article>`;
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

  function renderDiagnostics() {
    const environment = state.environment;
    document.getElementById("environment").innerHTML = `<div class="list-item ${environment.available ? "state-ok" : "state-warn"}"><strong>${environment.available ? "설정된 기능을 모두 사용할 수 있습니다." : "일부 기능을 사용할 수 없습니다."}</strong></div>${(environment.findings || []).length ? `<div class="finding-list" style="margin-top:10px">${environment.findings.map(item => `<article class="finding ${escapeHTML(item.severity)}"><div class="finding-header"><h3>${escapeHTML(environmentSource(item.type))} · ${escapeHTML(item.target || item.type)}</h3><span class="chip ${item.severity === "high" ? "bad" : "warn"}">${escapeHTML(severityLabels[item.severity] || item.severity)}</span></div><p>${escapeHTML(localize(item.summary))}</p><p class="next">다음 단계: ${escapeHTML(localize(item.recommendedNextAction))}</p></article>`).join("")}</div>` : ""}`;

    const targets = targetOptions();
    document.getElementById("guidance-ui").innerHTML = `${state.surfaceErrors.profiles ? surfaceError(state.surfaceErrors.profiles, "diagnostics") : ""}<div class="toolbar"><select id="guidance-target" aria-label="지침 점검 대상">${targets.length ? targets.map(target => `<option value="${escapeHTML(target.value)}">${escapeHTML(target.label)}</option>`).join("") : '<option value="">관찰된 Worktree 없음</option>'}</select><select id="handoff-profile" aria-label="Agent Profile">${state.profiles.length ? state.profiles.map(profile => `<option value="${escapeHTML(profile.metadata.id)}">${escapeHTML(profile.metadata.name)}</option>`).join("") : '<option value="">Agent Profile 없음</option>'}</select><input id="handoff-model" aria-label="선택 모델" placeholder="모델 선택 사항"><button id="guidance-check" class="button" type="button" ${targets.length ? "" : "disabled"}>지침 점검 실행</button><button id="handoff-preview" class="button" type="button" ${targets.length && state.profiles.length ? "" : "disabled"}>Handoff 미리 보기</button></div>${renderGuidanceResult()}`;

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

    const runbookContent = state.runbooks.length
      ? `<div class="item-list">${state.runbooks.map(item => `<article class="list-item"><div class="list-item-header"><div><h3>${escapeHTML(item.name)}</h3><p class="meta">${escapeHTML(item.id)} · 승인 필요</p></div><span class="chip warn">PowerShell</span></div><dl class="detail-grid"><div class="wide"><dt>스크립트</dt><dd><code>${escapeHTML(item.scriptPath)}</code></dd></div><div><dt>제한 시간</dt><dd>${escapeHTML(item.timeoutSeconds)}초</dd></div><div class="wide"><dt>허용 환경 변수</dt><dd>${escapeHTML((item.environmentAllowlist || []).join(", ") || "없음")}</dd></div></dl>${(item.parameters || []).length ? `<div class="runbook-parameters">${item.parameters.map(parameter => `<label><span>${escapeHTML(parameter)}</span><input data-runbook-param="${escapeHTML(parameter)}" data-runbook-id="${escapeHTML(item.id)}" placeholder="선택 값"></label>`).join("")}</div>` : '<p class="meta">입력할 parameter가 없습니다.</p>'}<div class="item-actions"><select data-runbook-target="${escapeHTML(item.id)}" aria-label="runbook 실행 Worktree">${targets.length ? targets.map(target => `<option value="${escapeHTML(target.value)}">${escapeHTML(target.label)}</option>`).join("") : '<option value="">관찰된 Worktree 없음</option>'}</select><button class="button primary small" type="button" data-runbook="plan" data-id="${escapeHTML(item.id)}" ${targets.length ? "" : "disabled"}>실행 계획 만들기</button><button class="button small" type="button" data-runbook="edit" data-id="${escapeHTML(item.id)}">정보 변경</button><button class="button small" type="button" data-unregister="runbook" data-runbook-id="${escapeHTML(item.id)}" data-name="${escapeHTML(item.name)}">제거</button></div></article>`).join("")}</div>`
      : '<div class="empty-state"><strong>등록된 PowerShell runbook이 없습니다.</strong><span>검토한 .ps1 파일을 등록하면 선택한 Worktree에서 실행 계획을 만들 수 있습니다.</span></div>';
    document.getElementById("runbook-list").innerHTML = (state.surfaceErrors.runbooks ? surfaceError(state.surfaceErrors.runbooks, "diagnostics") : "") + runbookContent;

    const cleanupContent = state.cleanup.length
      ? `<div class="item-list">${state.cleanup.map(item => `<article class="list-item"><div class="list-item-header"><h3>${escapeHTML(item.spec.repositoryId)} / ${escapeHTML(item.spec.worktreeId)}</h3><span class="chip warn">${escapeHTML(label(item.spec.decision))}</span></div><p class="meta"><code>${escapeHTML(item.spec.canonicalPath)}</code></p><p>${(item.spec.reasons || []).map(reason => escapeHTML(localize(reason))).join("<br>")}</p></article>`).join("")}</div>`
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
      ? `<div class="table-wrap"><table><thead><tr><th>시각</th><th>유형</th><th>내용</th><th>범위</th></tr></thead><tbody>${state.events.slice().reverse().map(item => `<tr><td>${escapeHTML(formatDate(item.spec.occurredAt))}</td><td><code>${escapeHTML(item.spec.type)}</code></td><td>${escapeHTML(localize(item.spec.summary))}</td><td>${escapeHTML([item.spec.projectId, item.spec.repositoryId].filter(Boolean).join(" / ") || "전체")}</td></tr>`).join("")}</tbody></table></div>`
      : '<div class="empty-state"><strong>아직 활동 기록이 없습니다.</strong><span>점검이나 등록 변경을 실행하면 감사 기록이 남습니다.</span></div>';
  }

  function renderAll() {
    renderHome();
    renderProjects();
    renderWork();
    renderDiagnostics();
    renderActivity();
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
    ]);
    state.loading.work = false;
    state.loaded.work = true;
    renderWork();
  }

  async function loadDiagnosticsData(force) {
    if (state.loading.diagnostics || (state.loaded.diagnostics && !force)) return;
    state.loading.diagnostics = true;
    await Promise.all([
      loadSurface("cleanup", () => request("/api/cleanup/candidates"), items => { state.cleanup = items; }),
      loadSurface("safeguards", () => request("/api/safeguards/rules"), items => { state.safeguards = items; }),
      loadSurface("profiles", () => request("/api/agent-profiles"), items => { state.profiles = items; }),
      loadSurface("integrations", () => request("/api/integrations"), items => { state.integrations = items; }),
      loadSurface("runbooks", () => request("/api/runbooks"), items => { state.runbooks = items; }),
    ]);
    state.loading.diagnostics = false;
    state.loaded.diagnostics = true;
    renderDiagnostics();
  }

  async function loadRouteData(route, force) {
    if (route === "work") await loadWorkData(force);
    if (route === "diagnostics") await loadDiagnosticsData(force);
  }

  let refreshing = false;
  let initialized = false;
  async function refreshAll() {
    if (refreshing) return;
    refreshing = true;
    try {
      const [snapshot, registryProjects, findings, events, environment] = await Promise.all([
        request("/api/state"),
        request("/api/projects"),
        request("/api/findings"),
        request("/api/events"),
        request("/api/environment"),
      ]);
      state.snapshot = snapshot;
      state.registryProjects = registryProjects || [];
      state.findings = findings || [];
      state.events = events || [];
      state.environment = environment;
      initialized = true;
      await loadRouteData(currentRoute(), true);
      renderAll();
    } catch (error) {
      showNotice(`로컬 서비스에서 상태를 불러오지 못했습니다. ${error.message}`, true);
    } finally {
      refreshing = false;
    }
  }

  const unregisterDialog = document.getElementById("unregister-dialog");
  const unregisterInput = document.getElementById("unregister-confirmation");
  const editorDialog = document.getElementById("editor-dialog");
  const editorFields = document.getElementById("editor-fields");
  let unregisterTarget = null;
  let editorTarget = null;

  const editorInput = (id, labelText, value = "", options = {}) => `<label class="${options.wide ? "wide" : ""}"><span>${escapeHTML(labelText)}</span><input id="${id}" ${options.type ? `type="${options.type}"` : ""} ${options.required === false ? "" : "required"} ${options.readonly ? "readonly" : ""} value="${escapeHTML(value)}"></label>`;
  const editorTextarea = (id, labelText, value = "") => `<label class="wide"><span>${escapeHTML(labelText)}</span><textarea id="${id}" rows="3">${escapeHTML(value)}</textarea></label>`;

  function openEditor(kind, context = {}) {
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
      editorFields.innerHTML = `${integration ? "" : editorInput("edit-id", "연동 ID")}${editorInput("edit-name", "표시 이름", integration?.name || "")}<label><span>종류</span><select id="edit-integration-kind"><option value="github" ${integration?.kind === "github" ? "selected" : ""}>GitHub</option><option value="jenkins" ${integration?.kind === "jenkins" ? "selected" : ""}>Jenkins</option><option value="kubernetes" ${integration?.kind === "kubernetes" ? "selected" : ""}>Kubernetes</option></select></label>${editorInput("edit-endpoint", "API 주소", integration?.endpoint || "", { wide: true })}${editorInput("edit-credential", "Credential reference", integration?.credentialRef || "", { required: false })}${editorTextarea("edit-values", "대상 값 · key=value 한 줄에 하나", Object.entries(integration?.values || {}).map(([key, value]) => `${key}=${value}`).join("\n"))}`;
    } else {
      const profile = context.profile;
      title.textContent = profile ? "Agent Profile 변경" : "Agent Profile 추가";
      description.textContent = "명령과 인자는 분리해 저장하고, 허용한 환경 변수 이름만 전달합니다.";
      editorFields.innerHTML = `${profile ? "" : editorInput("edit-id", "Profile ID")}${editorInput("edit-name", "표시 이름", profile?.metadata.name || "")}${editorInput("edit-command", "실행 명령", profile?.spec.command || "")}${editorTextarea("edit-version-probe", "버전 확인 인자 · 한 줄에 하나", (profile?.spec.versionProbe || []).join("\n"))}${editorInput("edit-timeout", "제한 시간(초)", profile?.spec.timeoutSeconds || 10, { type: "number" })}${editorInput("edit-model-template", "모델 인자 템플릿", profile?.spec.modelArgumentTemplate || "", { required: false })}${editorTextarea("edit-environment", "허용 환경 변수 · 한 줄에 하나", (profile?.spec.environmentAllowlist || []).join("\n"))}<label><span>실행 방식</span><select id="edit-launch-mode"><option value="direct" ${profile?.spec.launchMode === "direct" ? "selected" : ""}>직접 실행</option><option value="powershell_profile" ${profile?.spec.launchMode === "powershell_profile" ? "selected" : ""}>PowerShell profile</option></select></label><label><span>데이터 경계</span><select id="edit-data-boundary"><option value="enterprise" ${profile?.spec.dataBoundary === "enterprise" ? "selected" : ""}>기업 경계</option><option value="local" ${profile?.spec.dataBoundary === "local" ? "selected" : ""}>로컬 전용</option></select></label>`;
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
    unregisterTarget = {
      kind: button.dataset.unregister,
      projectID: button.dataset.project,
      repositoryID: button.dataset.repository || "",
      profileID: button.dataset.profileId || "",
      integrationID: button.dataset.integrationId || "",
      runbookID: button.dataset.runbookId || "",
      name: button.dataset.name,
    };
    const isProject = unregisterTarget.kind === "project";
    const isProfile = unregisterTarget.kind === "profile";
    const isIntegration = unregisterTarget.kind === "integration";
    const isRunbook = unregisterTarget.kind === "runbook";
    document.getElementById("unregister-title").textContent = isProfile ? "Agent Profile 제거" : isIntegration ? "연동 설정 제거" : isRunbook ? "PowerShell runbook 제거" : isProject ? "프로젝트 등록 해제" : "저장소 등록 해제";
    document.getElementById("unregister-description").textContent = isProfile
      ? `“${unregisterTarget.name}” Agent Profile의 저장된 실행 설정을 제거합니다.`
      : isProject
        ? `“${unregisterTarget.name}” 프로젝트의 등록과 모든 저장소 관찰 기록을 해제합니다.`
      : isIntegration ? `“${unregisterTarget.name}” 연동 설정을 제거합니다.` : isRunbook ? `“${unregisterTarget.name}” PowerShell runbook 설정을 제거합니다.` : `“${unregisterTarget.name}” 저장소의 등록과 관찰 기록을 해제합니다.`;
    document.getElementById("unregister-safety").textContent = isProfile
      ? "Profile 설정만 제거하며 Agent 프로그램이나 작업 파일은 삭제하지 않습니다."
      : isIntegration ? "저장소나 외부 시스템은 변경하지 않고 로컬 설정만 제거합니다."
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
    if (button.dataset.retry) {
      button.disabled = true;
      await loadRouteData(button.dataset.retry, true);
      return;
    }
    if (button.id === "show-register") {
      document.getElementById("register-panel").hidden = false;
      document.getElementById("name").focus();
      return;
    }
    if (button.id === "hide-register") {
      document.getElementById("register-panel").hidden = true;
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

  document.getElementById("scan").addEventListener("click", async event => {
    const button = event.currentTarget;
    button.disabled = true;
    button.textContent = "점검 중…";
    try {
      await request("/api/scan", { method: "POST", headers: mutationHeaders() });
      showNotice("프로젝트 점검을 완료했습니다.");
      await refreshAll();
    } catch (error) {
      showNotice(error.message, true);
    } finally {
      button.disabled = false;
      button.textContent = "지금 점검";
    }
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

  document.getElementById("pick-folder").addEventListener("click", async () => {
    try {
      const result = await request("/api/folder-picker", { method: "POST", headers: mutationHeaders() });
      if (result.path) {
        pathInput.value = result.path;
        await discoverRepositories();
      }
    } catch (error) {
      showNotice(error.message, true);
    }
  });
  document.getElementById("find-repositories").addEventListener("click", discoverRepositories);
  document.getElementById("import-project").addEventListener("click", () => document.getElementById("import-project-file").click());
  document.getElementById("import-project-file").addEventListener("change", async event => {
    const file = event.target.files?.[0];
    if (!file) return;
    try {
      const project = await request("/api/projects/import", { method: "POST", headers: mutationHeaders(), body: await file.text() });
      state.activeProjectID = project.metadata.id;
      showNotice("프로젝트 설정을 가져왔습니다. 비밀 값은 포함되지 않습니다.");
      await refreshAll();
    } catch (error) {
      showNotice(error.message, true);
    } finally {
      event.target.value = "";
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
      event.target.reset();
      candidates.dataset.discovered = "false";
      candidates.textContent = "폴더를 선택하면 아래의 Git 저장소를 읽기 전용으로 찾습니다.";
      document.getElementById("register-panel").hidden = true;
      showNotice("프로젝트를 등록했습니다.");
      await refreshAll();
    } catch (error) {
      showNotice(error.message, true);
    } finally {
      submit.disabled = false;
    }
  });

  document.addEventListener("change", event => {
    if (event.target.id === "finding-severity") {
      state.findingFilters.severity = event.target.value;
      renderProjects();
    }
    if (event.target.id === "finding-state") {
      state.findingFilters.state = event.target.value;
      renderProjects();
    }
  });

  document.addEventListener("input", event => {
    if (!event.target.matches("[data-safeguard-owner]")) return;
    const submit = event.target.closest(".list-item").querySelector("[data-owner-submit]");
    submit.disabled = event.target.value.trim() === "";
  });

  document.getElementById("editor-cancel").addEventListener("click", () => editorDialog.close());
  document.getElementById("editor-form").addEventListener("submit", async event => {
    event.preventDefault();
    const submit = document.getElementById("editor-submit");
    submit.disabled = true;
    try {
      const message = await submitEditor();
      editorDialog.close();
      showNotice(message);
      await refreshAll();
    } catch (error) {
      showNotice(error.message, true);
    } finally {
      submit.disabled = false;
    }
  });

  unregisterInput.addEventListener("input", () => {
    document.getElementById("unregister-submit").disabled = unregisterInput.value !== unregisterTarget?.name;
  });
  document.getElementById("unregister-cancel").addEventListener("click", () => unregisterDialog.close());
  document.getElementById("unregister-form").addEventListener("submit", async event => {
    event.preventDefault();
    if (!unregisterTarget || unregisterInput.value !== unregisterTarget.name) return;
    const path = unregisterTarget.kind === "project"
      ? `/api/projects/${encode(unregisterTarget.projectID)}`
      : unregisterTarget.kind === "profile"
        ? `/api/agent-profiles/${encode(unregisterTarget.profileID)}`
        : unregisterTarget.kind === "integration"
          ? `/api/integrations/${encode(unregisterTarget.integrationID)}`
          : unregisterTarget.kind === "runbook"
            ? `/api/runbooks/${encode(unregisterTarget.runbookID)}`
        : `/api/projects/${encode(unregisterTarget.projectID)}/repositories/${encode(unregisterTarget.repositoryID)}`;
    const submit = document.getElementById("unregister-submit");
    submit.disabled = true;
    try {
      await request(path, { method: "DELETE", headers: mutationHeaders() });
      if (unregisterTarget.kind === "project") state.activeProjectID = "";
      unregisterDialog.close();
      showNotice(unregisterTarget.kind === "profile" ? "Agent Profile을 제거했습니다." : unregisterTarget.kind === "integration" ? "연동 설정을 제거했습니다." : unregisterTarget.kind === "runbook" ? "PowerShell runbook을 제거했습니다." : "등록을 해제했습니다. 원본 저장소 파일은 변경하지 않았습니다.");
      await refreshAll();
      document.getElementById(unregisterTarget.kind === "profile" ? "add-profile" : unregisterTarget.kind === "integration" ? "add-integration" : unregisterTarget.kind === "runbook" ? "add-runbook" : "project-list-panel").focus();
    } catch (error) {
      showNotice(error.message, true);
      submit.disabled = false;
    }
  });

  window.addEventListener("hashchange", setRoute);
  if (!location.hash) location.hash = "home";
  setRoute();
  refreshAll();
  window.setInterval(refreshAll, 30000);
})();
