# Design

## Source of truth

- Status: Active
- Last refreshed: 2026-08-24
- Primary product surfaces: embedded loopback browser UI on native Windows 11
- Evidence reviewed: `docs/PRODUCT.md`, `docs/ARCHITECTURE.md`,
  `docs/IMPLEMENTATION_PLAN.md`, `docs/HANDOFF.md`, `internal/app/web.go`, and
  GitHub issue #1

## Brand

- Personality: calm, precise, local-first engineering control room
- Trust signals: explicit scope, visible evidence, clear risk language, and
  honest unavailable states
- Avoid: generic admin-dashboard decoration, raw-log-first layouts, excessive
  animation, unexplained English labels, and destructive wording for registry
  changes

## Product goals

- Goals: show what needs attention, keep project context visible, and make
  deterministic checks and Actions easy to find and review
- Non-goals: chat UI, agent orchestration, generic shell access, and a complete
  replacement for GitHub, Jenkins, or an IDE
- Success signals: the user can register or edit a project, review and
  acknowledge a Finding, complete `discover -> proposal -> review -> apply ->
  run`, inspect bounded Check/Action evidence, prepare a masked Handoff, and
  unregister an item without scanning one long page

## Personas and jobs

- Primary personas: one developer maintaining several local projects on Windows
- User jobs: review current Findings, inspect a Project or Worktree, run a
  reviewed check or Action, diagnose the environment, and review activity
- Key contexts of use: a morning overview, pre-PR work, local diagnosis, and
  Windows acceptance testing

## Information architecture

- Primary navigation: 홈, 프로젝트, 작업, 진단, 기록
- Core routes/screens: hash-based screens within the embedded UI; Project detail
  is selected from the 프로젝트 screen without adding server-side routes
- Content hierarchy:
  - 홈: 지금 확인할 항목, 프로젝트별 상태, 최근 실행 결과
  - 프로젝트: 등록/가져오기/내보내기, Project cards, selected Project
    details, Repository CRUD, Worktrees, filterable evidence-backed Findings
  - 작업: exact-Worktree discovery, Proposal evidence review, Pre-PR Checkset
    execution/results, and Action planning/approval/execution/results
  - 진단: 개발 환경, Agent Profile CRUD, structured Guidance/Handoff review,
    정리 후보, 반복된 실패
  - 기록: complete activity history

## Design principles

- Findings and next actions come before raw evidence.
- Keep the selected Project and Worktree explicit wherever scope matters.
- Use Korean for navigation and explanations while retaining established terms
  such as Worktree, Pre-PR, Action, MCP, HEAD, and Agent Profile.
- Registration changes use `등록 해제`; they never imply repository files are
  deleted.
- Tradeoff: use lightweight client-side views instead of a frontend framework
  so the local binary remains small and auditable.

## Visual language

- Color: Control Navy `#101826`, Slate `#1B2638`, Paper `#F5F7FA`, Signal Blue
  `#2F6FEB`, Amber `#B7791F`, and Critical Red `#C2414D`
- Typography: `Segoe UI` for interface text, `Malgun Gothic` for Korean fallback,
  and the system monospace face for paths, identifiers, and evidence
- Spacing/layout rhythm: 4 px base, 16-24 px content rhythm, compact data rows
- Shape/radius/elevation: restrained 8-12 px radii, borders before shadows
- Motion: short view transitions only when motion is allowed; no ambient motion
- Imagery/iconography: text and small inline symbols only; status color is never
  the sole carrier of meaning

## Components

- Existing components to reuse: native buttons, forms, `details`, tables, and
  the current application-service HTTP APIs
- New/changed components: application shell, navigation links, view headers,
  Project cards, status chips, evidence disclosure panels, structured run
  results, empty states, toast, and native `dialog`
- Variants and states: default, hover, focus-visible, selected, disabled,
  loading, empty, warning, error, and success
- Token/component ownership: CSS custom properties and semantic classes in the
  embedded UI stylesheet; no separate design-system package

## Accessibility

- Target standard: practical WCAG 2.2 AA baseline
- Keyboard/focus behavior: a skip link bypasses primary navigation; route
  changes move reading focus to `main`; all actions are keyboard reachable;
  focus is visible; dialogs restore focus through native behavior
- Contrast/readability: text and status labels meet readable contrast; status
  meaning is written as text
- Screen-reader semantics: landmarks, headings, current navigation state,
  labelled forms, live status messages, and descriptive confirmation text
- Reduced motion and sensory considerations: respect `prefers-reduced-motion`
  and avoid flashing or color-only cues

## Responsive behavior

- Supported breakpoints/devices: desktop Windows browsers first; usable down to
  a narrow mobile-sized viewport for remote inspection
- Layout adaptations: fixed side navigation becomes a compact top navigation;
  cards and toolbars collapse to one column; wide tables scroll horizontally
- Touch/hover differences: controls keep a minimum practical touch target and
  do not require hover to reveal an action

## Interaction states

- Loading: name the data being loaded in Korean
- Empty: explain what is missing and provide the next relevant action
- Error: show the safe server message and the retryable action; do not expose
  raw filesystem, SQL, command, or secret-bearing details
- Success: confirm the completed action using the same verb as its button
- Disabled: explain prerequisites near the control
- Offline/slow network, if applicable: preserve the current view and show that
  the local service could not be reached

## Content voice

- Tone: concise, direct, and calm
- Terminology: `확인할 항목`, `개발 환경`, `지침 점검`, `정리 후보`,
  `반복된 실패`, and `등록 해제`
- Microcopy rules: buttons use active verbs; empty states direct the next step;
  technical terms stay in English when a Korean translation would be awkward

## Implementation constraints

- Framework/styling system: Go `embed`, plain HTML, CSS, and browser JavaScript
- Design-token constraints: one small CSS custom-property set; no new package
- Performance constraints: one embedded document and two static assets; bounded
  API refreshes only for data needed by the active product surfaces
- Compatibility constraints: native Windows 11 and current embedded browser;
  server behavior, CSRF, same-origin checks, masking, and Action Broker policy
  are unchanged
- Test/screenshot expectations: handler tests assert Korean navigation, static
  asset delivery, safe unregister copy, and existing API hooks; native Windows
  visual and interaction acceptance remains a separate recorded smoke test

## Open questions

- [ ] Decide whether a future release needs user-selectable English UI; owner:
  product; impact: localization architecture only, not this Korean-first pass
- [ ] Revisit richer icons or screenshots only after native Windows usability
  feedback identifies a concrete navigation problem
