(() => {
  "use strict";

  const token = document.querySelector('meta[name="control-room-token"]').content;
  const mutationHeaders = () => ({
    "Content-Type": "application/json",
    "X-Control-Room-Token": token,
  });
  const state = {
    snapshot: { projects: [] },
    findings: [],
    events: [],
    environment: { available: false, findings: [] },
    workItems: [],
    actionDetails: [],
    cleanup: [],
    safeguards: [],
    profiles: [],
    activeProjectID: "",
    checkRuns: new Map(),
    expandedActions: new Set(),
    guidanceResult: "",
    surfaceErrors: { checksets: "", actions: "", cleanup: "", safeguards: "", profiles: "" },
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
    shadow: "shadow mode",
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
      <p class="meta">${escapeHTML(scope)} · ${escapeHTML(label(item.spec.state))} · 근거 ${escapeHTML((item.spec.evidenceRefs || []).join(", ") || "없음")}</p>
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
    const projectFindings = findings.filter(item => item.spec.projectId === project.id);
    detail.innerHTML = `<div class="project-detail-header"><div><p class="eyebrow">${escapeHTML(project.id)}</p><h2>${escapeHTML(project.name)}</h2><p class="meta">마지막 관찰 ${escapeHTML(formatDate(project.scanned_at))}</p></div><button class="button danger small" type="button" data-unregister="project" data-project="${escapeHTML(project.id)}" data-name="${escapeHTML(project.name)}">프로젝트 등록 해제</button></div>
      <div class="repository-list">${project.repos.map(repository => {
        const status = projectStatus(repository);
        const onlyRepository = project.repos.length <= 1;
        return `<article class="repository-card"><div class="repository-header"><div><h3>${escapeHTML(repository.id)}</h3><div class="repository-path"><code>${escapeHTML(repository.path)}</code></div></div><div class="repository-actions"><span class="chip ${status.className}">${status.text}</span><button class="button small" type="button" data-unregister="repository" data-project="${escapeHTML(project.id)}" data-repository="${escapeHTML(repository.id)}" data-name="${escapeHTML(repository.id)}" ${onlyRepository ? "disabled" : ""}>저장소 등록 해제</button></div></div>
          <div class="repository-summary"><span class="chip">브랜치 ${escapeHTML(repository.branch || "detached")}</span><span class="chip">ahead ${escapeHTML(repository.ahead || 0)}</span><span class="chip">behind ${escapeHTML(repository.behind || 0)}</span><span class="chip">Worktree ${(repository.worktrees || []).length}</span></div>
          ${onlyRepository ? '<p class="meta">마지막 저장소는 개별 해제할 수 없습니다. 프로젝트 등록 해제를 사용하세요.</p>' : ""}
          <details><summary>Worktree 상세 보기</summary><div class="worktree-list">${(repository.worktrees || []).length ? repository.worktrees.map(worktree => `<div class="worktree"><strong>${escapeHTML(worktree.metadata.id)} · ${worktree.spec.primary ? "기본" : "연결됨"}</strong><code>${escapeHTML(worktree.spec.canonicalPath)}</code><div class="meta">${escapeHTML(worktree.spec.branch || "detached")} · HEAD ${escapeHTML(worktree.spec.head || "확인 불가")} · ${worktree.spec.dirty ? "변경 있음" : "clean"} · ${worktree.spec.untracked ? "untracked 있음" : "tracked"} · upstream ${escapeHTML(worktree.spec.upstream || "없음")} ${escapeHTML(worktree.spec.ahead || 0)}/${escapeHTML(worktree.spec.behind || 0)} · ${worktree.spec.locked ? "잠김" : "잠기지 않음"} · ${worktree.spec.prunable ? "prunable" : "유지됨"} · ${escapeHTML(worktree.spec.trust)} · ${escapeHTML(worktree.spec.tombstonedAt ? "관찰 종료" : worktree.spec.error || "현재")}</div></div>`).join("") : '<div class="empty-state"><span>관찰된 Worktree가 없습니다.</span></div>'}</div></details>
        </article>`;
      }).join("")}</div>
      <div class="panel-heading" style="margin-top:24px"><div><p class="eyebrow">확인할 항목</p><h2>이 프로젝트의 확인할 항목</h2></div></div>
      ${projectFindings.length ? `<div class="finding-list">${projectFindings.map(findingCard).join("")}</div>` : '<div class="empty-state"><strong>열린 확인 항목이 없습니다.</strong><span>현재 관찰 기준으로 별도 조치가 필요하지 않습니다.</span></div>'}`;
  }

  function checksetCard(checkset, repository) {
    const runs = state.checkRuns.get(checkset.metadata.id) || [];
    return `<article class="list-item"><div class="list-item-header"><div><h3>${escapeHTML(checkset.metadata.name)}</h3><p class="meta">${escapeHTML(repository.projectName)} / ${escapeHTML(repository.id)} / ${escapeHTML(checkset.spec.worktreeId)}</p></div><span class="chip">${escapeHTML(label(checkset.spec.state))}</span></div><p>${(checkset.spec.steps || []).map(step => escapeHTML(step.name)).join(", ")}</p><div class="item-actions"><button class="button small" data-checkset="apply" data-id="${escapeHTML(checkset.metadata.id)}" ${checkset.spec.state === "draft" ? "" : "disabled"}>적용</button><button class="button primary small" data-checkset="run" data-id="${escapeHTML(checkset.metadata.id)}" ${checkset.spec.state === "applied" ? "" : "disabled"}>실행</button><button class="button small" data-checkset="results" data-id="${escapeHTML(checkset.metadata.id)}">결과 보기</button></div>${runs.length ? `<div class="result-box">${runs.slice().reverse().map(run => `${escapeHTML(label(run.spec.status))} · ${escapeHTML(formatDate(run.spec.completedAt || run.spec.startedAt))} · ${(run.spec.steps || []).map(step => `${escapeHTML(step.stepId)}: ${escapeHTML(label(step.status))}`).join(", ")}`).join("<br>")}</div>` : ""}</article>`;
  }

  function renderWork() {
    const checksetHTML = state.workItems.flatMap(item => [
      ...(item.checksets || []).map(checkset => checksetCard(checkset, item.repository)),
      ...(item.proposals || []).filter(proposal => proposal.spec.state === "applied" && proposal.spec.typedCommand).map(proposal => `<article class="list-item"><div class="list-item-header"><div><h3>${escapeHTML(proposal.metadata.name)}</h3><p class="meta">${escapeHTML(item.repository.projectName)} / ${escapeHTML(item.repository.id)} / ${escapeHTML(proposal.spec.worktreeId)}</p></div><span class="chip ok">검토됨</span></div><p>적용된 발견 결과를 실행 가능한 Checkset으로 만듭니다.</p><div class="item-actions"><button class="button primary small" data-checkset="create" data-id="${escapeHTML(proposal.metadata.id)}">Checkset 만들기</button></div></article>`),
    ]).join("");
    const checksetContent = checksetHTML
      ? `<div class="item-list">${checksetHTML}</div>`
      : '<div class="empty-state"><strong>실행할 Pre-PR 점검이 없습니다.</strong><span>검토하고 적용한 discovery proposal이 필요합니다.</span></div>';
    document.getElementById("checksets").innerHTML = (state.surfaceErrors.checksets ? surfaceError(state.surfaceErrors.checksets, "work") : "") + checksetContent;

    const targets = projectRepositories().flatMap(repository => (repository.worktrees || []).map(worktree => ({
      value: `${repository.projectID}|${repository.id}|${worktree.metadata.id}`,
      label: `${repository.projectName} / ${repository.id} / ${worktree.metadata.id}`,
    })));
    const plans = state.actionDetails.map(detail => {
      const admission = detail.status.admission;
      const latest = detail.runs?.[0];
      const resultVisible = state.expandedActions.has(detail.plan.metadata.id);
      const actionButtons = admission === "approval_required"
        ? `<button class="button small" data-action="approve" data-id="${escapeHTML(detail.plan.metadata.id)}">승인 요청</button>`
        : admission === "eligible"
          ? `<button class="button primary small" data-action="execute" data-id="${escapeHTML(detail.plan.metadata.id)}">실행</button>`
          : "";
      return `<article class="list-item"><div class="list-item-header"><div><h3>${escapeHTML(detail.plan.metadata.name)}</h3><p class="meta">${escapeHTML(detail.plan.spec.projectId)} / ${escapeHTML(detail.plan.spec.repositoryId)} / ${escapeHTML(detail.plan.spec.worktreeId)}</p></div><span class="chip ${admission === "eligible" ? "ok" : admission === "approval_required" ? "warn" : "bad"}">${escapeHTML(label(admission))}</span></div><div class="item-actions"><button class="button small" data-action="trust" data-id="${escapeHTML(detail.plan.metadata.id)}">실행 대상으로 표시</button>${actionButtons}<button class="button small" data-action="runs" data-id="${escapeHTML(detail.plan.metadata.id)}">결과 보기</button></div>${resultVisible ? `<div class="result-box">${detail.runs.length ? detail.runs.map(run => `${escapeHTML(label(run.spec.status))} · ${escapeHTML(run.spec.stderr || run.spec.stdout || "출력 없음")}`).join("<br>") : "아직 실행 결과가 없습니다."}</div>` : latest ? `<p class="meta">최근 결과 ${escapeHTML(label(latest.spec.status))}</p>` : ""}</article>`;
    }).join("");
    document.getElementById("action-ui").innerHTML = `${state.surfaceErrors.actions ? surfaceError(state.surfaceErrors.actions, "work") : ""}<div class="toolbar"><select id="action-target" aria-label="Action 대상 Worktree">${targets.length ? targets.map(target => `<option value="${escapeHTML(target.value)}">${escapeHTML(target.label)}</option>`).join("") : '<option value="">관찰된 Worktree 없음</option>'}</select><button id="action-plan" class="button primary" type="button" ${targets.length ? "" : "disabled"}>저장소 새로고침 계획</button></div>${plans ? `<div class="item-list">${plans}</div>` : '<div class="empty-state"><strong>검토된 Action 계획이 없습니다.</strong><span>대상 Worktree를 선택해 저장소 새로고침 계획을 만들 수 있습니다.</span></div>'}`;
  }

  function environmentSource(type) {
    if (type?.startsWith("tool.")) return "Tool";
    if (type?.startsWith("agent_profile.")) return "Agent Profile";
    return "설정";
  }

  function renderDiagnostics() {
    const environment = state.environment;
    document.getElementById("environment").innerHTML = `<div class="list-item ${environment.available ? "state-ok" : "state-warn"}"><strong>${environment.available ? "설정된 기능을 모두 사용할 수 있습니다." : "일부 기능을 사용할 수 없습니다."}</strong></div>${(environment.findings || []).length ? `<div class="finding-list" style="margin-top:10px">${environment.findings.map(item => `<article class="finding ${escapeHTML(item.severity)}"><div class="finding-header"><h3>${escapeHTML(environmentSource(item.type))} · ${escapeHTML(item.target || item.type)}</h3><span class="chip ${item.severity === "high" ? "bad" : "warn"}">${escapeHTML(severityLabels[item.severity] || item.severity)}</span></div><p>${escapeHTML(localize(item.summary))}</p><p class="next">다음 단계: ${escapeHTML(localize(item.recommendedNextAction))}</p></article>`).join("")}</div>` : ""}`;

    const targets = projectRepositories().flatMap(repository => (repository.worktrees || []).map(worktree => ({
      value: `${repository.projectID}|${repository.id}|${worktree.metadata.id}`,
      label: `${repository.projectName} / ${repository.id} / ${worktree.metadata.id}`,
    })));
    document.getElementById("guidance-ui").innerHTML = `${state.surfaceErrors.profiles ? surfaceError(state.surfaceErrors.profiles, "diagnostics") : ""}<div class="toolbar"><select id="guidance-target" aria-label="지침 점검 대상">${targets.length ? targets.map(target => `<option value="${escapeHTML(target.value)}">${escapeHTML(target.label)}</option>`).join("") : '<option value="">관찰된 Worktree 없음</option>'}</select><select id="handoff-profile" aria-label="Agent Profile">${state.profiles.length ? state.profiles.map(profile => `<option value="${escapeHTML(profile.metadata.id)}">${escapeHTML(profile.metadata.name)}</option>`).join("") : '<option value="">Agent Profile 없음</option>'}</select><button id="guidance-check" class="button" type="button" ${targets.length ? "" : "disabled"}>지침 점검 실행</button><button id="handoff-preview" class="button" type="button" ${targets.length && state.profiles.length ? "" : "disabled"}>Handoff 미리 보기</button></div><pre id="guidance-result" class="result-box" ${state.guidanceResult ? "" : "hidden"}>${escapeHTML(state.guidanceResult)}</pre>`;

    const cleanupContent = state.cleanup.length
      ? `<div class="item-list">${state.cleanup.map(item => `<article class="list-item"><div class="list-item-header"><h3>${escapeHTML(item.spec.repositoryId)} / ${escapeHTML(item.spec.worktreeId)}</h3><span class="chip warn">${escapeHTML(label(item.spec.decision))}</span></div><p class="meta"><code>${escapeHTML(item.spec.canonicalPath)}</code></p><p>${(item.spec.reasons || []).map(reason => escapeHTML(localize(reason))).join("<br>")}</p></article>`).join("")}</div>`
      : '<div class="empty-state"><strong>관찰된 정리 후보가 없습니다.</strong><span>Worktree가 관찰되면 안전 여부를 읽기 전용으로 평가합니다.</span></div>';
    document.getElementById("cleanup-queue").innerHTML = (state.surfaceErrors.cleanup ? surfaceError(state.surfaceErrors.cleanup, "diagnostics") : "") + cleanupContent;

    const safeguardContent = state.safeguards.length
      ? `<div class="item-list">${state.safeguards.map(item => `<article class="list-item"><div class="list-item-header"><h3>${escapeHTML(item.category)}</h3><span class="chip warn">${escapeHTML(label(item.mode))}</span></div><p>${escapeHTML(localize(item.summary))}</p><p class="next">${escapeHTML(item.occurrenceCount)}회 발생 · ${escapeHTML(localize(item.recommendedNextAction))}</p></article>`).join("")}</div>`
      : '<div class="empty-state"><strong>반복된 실패 제안이 없습니다.</strong><span>검증된 실패가 반복되면 검토 가능한 safeguard 제안이 나타납니다.</span></div>';
    document.getElementById("safeguards").innerHTML = (state.surfaceErrors.safeguards ? surfaceError(state.surfaceErrors.safeguards, "diagnostics") : "") + safeguardContent;
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
      loadSurface("safeguards", () => request("/api/safeguards/proposals"), items => { state.safeguards = items; }),
      loadSurface("profiles", () => request("/api/agent-profiles"), items => { state.profiles = items; }),
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
      const [snapshot, findings, events, environment] = await Promise.all([
        request("/api/state"),
        request("/api/findings"),
        request("/api/events"),
        request("/api/environment"),
      ]);
      state.snapshot = snapshot;
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
  let unregisterTarget = null;

  function openUnregister(button) {
    unregisterTarget = {
      kind: button.dataset.unregister,
      projectID: button.dataset.project,
      repositoryID: button.dataset.repository || "",
      name: button.dataset.name,
    };
    const isProject = unregisterTarget.kind === "project";
    document.getElementById("unregister-title").textContent = isProject ? "프로젝트 등록 해제" : "저장소 등록 해제";
    document.getElementById("unregister-description").textContent = isProject
      ? `“${unregisterTarget.name}” 프로젝트의 등록과 모든 저장소 관찰 기록을 해제합니다.`
      : `“${unregisterTarget.name}” 저장소의 등록과 관찰 기록을 해제합니다.`;
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
    if (button.dataset.project !== undefined && !button.dataset.unregister) {
      state.activeProjectID = button.dataset.project;
      renderProjects();
      return;
    }
    if (button.dataset.unregister) {
      openUnregister(button);
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
    if (["guidance-check", "handoff-preview"].includes(button.id)) {
      const target = document.getElementById("guidance-target").value.split("|");
      button.disabled = true;
      try {
        const result = button.id === "guidance-check"
          ? await request(`/api/projects/${encode(target[0])}/repositories/${encode(target[1])}/worktrees/${encode(target[2])}/guidance`)
          : await request("/api/handoffs/preview", {
            method: "POST",
            headers: mutationHeaders(),
            body: JSON.stringify({ profileId: document.getElementById("handoff-profile").value, projectId: target[0], repositoryId: target[1], worktreeId: target[2] }),
          });
        state.guidanceResult = JSON.stringify(result, null, 2);
        renderDiagnostics();
      } catch (error) {
        state.guidanceResult = error.message;
        renderDiagnostics();
      } finally {
        button.disabled = false;
      }
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

  unregisterInput.addEventListener("input", () => {
    document.getElementById("unregister-submit").disabled = unregisterInput.value !== unregisterTarget?.name;
  });
  document.getElementById("unregister-cancel").addEventListener("click", () => unregisterDialog.close());
  document.getElementById("unregister-form").addEventListener("submit", async event => {
    event.preventDefault();
    if (!unregisterTarget || unregisterInput.value !== unregisterTarget.name) return;
    const path = unregisterTarget.kind === "project"
      ? `/api/projects/${encode(unregisterTarget.projectID)}`
      : `/api/projects/${encode(unregisterTarget.projectID)}/repositories/${encode(unregisterTarget.repositoryID)}`;
    const submit = document.getElementById("unregister-submit");
    submit.disabled = true;
    try {
      await request(path, { method: "DELETE", headers: mutationHeaders() });
      if (unregisterTarget.kind === "project") state.activeProjectID = "";
      unregisterDialog.close();
      showNotice("등록을 해제했습니다. 원본 저장소 파일은 변경하지 않았습니다.");
      await refreshAll();
      document.getElementById("project-list-panel").focus();
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
