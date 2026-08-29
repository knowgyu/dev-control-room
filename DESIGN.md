# Dev Control Room design system

Status: active
Refreshed: 2026-08-30

This document is the source of truth for the embedded loopback UI. It describes
the small visual and interaction system implemented in
`internal/app/ui/index.html` and `internal/app/ui/app.css`. It does not create a
runtime dependency or require a frontend framework.

## Product job

Dev Control Room is a Windows local-first control room for one developer. Every
screen should make the following order clear:

1. current state;
2. consequence or evidence quality;
3. the next safe action.

The UI is an operational surface, not a chat, raw-log viewer, marketing
dashboard, or replacement for an IDE, GitHub, Jenkins, or a deployment tool.

## Information architecture

The shell uses one compact top navigation with six routes. A persistent sidebar
is intentionally not part of the product shape.

| Route | Visible page `h1` | Main job |
| --- | --- | --- |
| `home` | 첫 프로젝트 등록 / 오늘의 개발 상태 | first-use registration or the current project state and one useful next action |
| `projects` | 프로젝트 | registered scope, repositories, and Worktrees |
| `work` | 작업 | discovery, reviewed checks, and approved Actions |
| `assurance` | 검증 | repeatability, effects, usage, and linked evidence |
| `diagnostics` | 진단 | environment and optional execution capabilities |
| `activity` | 활동 기록 | immutable operational history |

The navigation markup uses `.primary-nav`. The old `.side-nav` and global
`.page-context` patterns are retired. Each route owns its single visible `h1`;
section headings use `h2` and detail headings use `h3`. `main` keeps
`id="main-content"` and `tabindex="-1"`, starts with `aria-label="홈"`, and the
route adapter updates that label when the hash route changes.

## Visual direction

The visual language is a quiet operations desk: restrained, readable, and
evidence-led. Borders and spacing establish hierarchy; decoration must not
compete with state or action.

### Tokens

`internal/app/ui/app.css` has exactly one `:root` token block. Components use
these semantic tokens rather than introducing local hex values for routine
states.

| Token | Value | Use |
| --- | --- | --- |
| `--canvas` | `#F4F6F6` | page background |
| `--surface` | `#FFFFFF` | panels and controls |
| `--surface-muted` | `#F7F9F9` | secondary evidence surfaces |
| `--ink` | `#202B2F` | primary text |
| `--muted` | `#687579` | supporting text and metadata |
| `--rule` | `#D7E0E0` | borders and dividers |
| `--accent` | `#146C70` | navigation, primary actions, and evidence rail |
| `--accent-soft` | `#E5F1F0` | selected and hover surfaces |
| `--success` / `--success-soft` | `#2D6B52` / `#E8F3ED` | successful or ready state |
| `--warning` / `--warning-soft` | `#8A5B24` / `#FBF1E3` | attention or approval state |
| `--danger` / `--danger-soft` | `#A3424B` / `#F9E9EB` | blocked, failed, or destructive state |
| `--neutral` / `--neutral-soft` / `--neutral-rule` | `#687579` / `#EDF1F1` / `#B9C8CA` | unavailable or not-yet-measured state |

The only signature accent is a 3px status/evidence rail on the left or top of
the object it qualifies. Status always includes text; color is never the only
carrier of meaning. There are no gradients, decorative background shapes, or
large panel shadows. Pills are limited to compact status chips.

### Typography

The UI font stack is:

```text
"Pretendard Variable", Pretendard, "Segoe UI Variable", "Segoe UI", "Malgun Gothic", sans-serif
```

Paths, IDs, commands, hashes, and other machine-oriented values use
`"Cascadia Mono", "Consolas", monospace`.

Korean headings use near-normal letter spacing, `text-wrap: pretty`, and
`word-break: keep-all`. Counts and comparison values use tabular numerals.

Pretendard is deliberately not bundled, loaded from a CDN, or added as a
dependency in this release. It is therefore preferred only when it is already
installed on the Windows machine. The fallback stack is the compatibility
contract, and Korean glyph metrics can vary slightly between machines where
Pretendard is absent. A future bundled-font change would require a separate
asset/license review and is not implied by this UI slice.

## Component contract

These are the only shared primitives. They are CSS/HTML conventions, not a
new component framework.

- `AppShell`: `.app-shell`, `.workspace`, and the sticky top bar.
- `PrimaryNav`: `.primary-nav`, with an `aria-current="page"` link and a 3px
  active rail.
- `PageHeader`: `.page-heading` or the home `.hero`; exactly one visible `h1`.
- `Panel`: `.panel`, a bordered surface for one coherent task or evidence set.
- `Button`: `.button`, with `primary`, `danger`, and `small` variants.
- `StatusChip`: `.chip`, with `ok`, `warn`, `bad`, or neutral `unknown`
  variants. It always has a readable label. Missing evidence is neutral, not a
  failure state.
- `Metric`: `.metric-card` and the existing metric collections; only use a
  metric when it informs a decision or action.
- `ActionGroup`: `.item-actions` and `.toolbar`, grouping controls next to the
  object they change.
- `Disclosure`: native `details/summary` with `.disclosure`; advanced evidence
  and settings start closed unless the user explicitly opens them.
- `EmptyState`: `.empty-state`, explaining what is absent and the next useful
  action.
- `ErrorState`: `.surface-error` or a state-marked result, with a safe message
  and retry/recovery action.
- `Dialog`: native `dialog` with labelled description, safety copy, and actions.

Existing feature classes such as `.finding`, `.repository-card`,
`.assurance-record`, `.trace-node`, and `.command-output` are compositions of
these primitives. They are not additional framework components and should not
gain independent token systems.

## Progressive disclosure

The first viewport prioritizes state, consequence, and action. Revision IDs,
digests, exact argv, artifact manifests, pricing basis, raw diagnostics, and
other evidence details belong in native `details` or a focused secondary view.

The Assurance route starts with its page header and scope filters. Its former
explanatory hero is a compact, closed `.disclosure` so the useful evidence is
visible without promotional copy. The Diagnostics route keeps environment,
Provider, and guidance status visible. Agent Profiles, integrations, Jenkins
groups, and runbooks are grouped under one closed `실행 설정` disclosure;
cleanup and safeguards are grouped under one closed `안전 검토` disclosure.

Do not hide a blocking state or a required approval inside disclosure. Put the
decision and its safe next action in the open surface, then expose supporting
evidence below it.

## Accessibility and responsive behavior

- Keep the skip link as the first keyboard target.
- Use native links for navigation, buttons for actions, labels for controls,
  and native `details`/`dialog` before adding ARIA.
- Preserve visible `:focus-visible` treatment and use `aria-live="polite"` for
  async status areas.
- Keep controls touchable, with at least the practical 40px control height.
- Use text plus semantic state color for success, attention, failure, and
  blocked conditions.
- Keep long paths and identifiers breakable; wide activity tables scroll
  horizontally instead of forcing the page wider.
- Respect `prefers-reduced-motion`; the route transition is optional and has a
  reduced variant.
- At `980px`, navigation may wrap into a second row and dense grids simplify.
  At `720px`, grids become one column, controls stack, and the top navigation
  remains horizontally scrollable. No action depends on hover.

## Copy and state rules

Use concise Korean noun phrases for labels and status. Use a short `합니다`
sentence only when it explains a cause, consequence, or recovery step. Keep
established technical terms such as Worktree, Provider, Action, artifact, and
HEAD when translating them would reduce precision.

Every data surface needs intentional loading, empty, partial-failure,
blocked/approval-required, and completed states. Optional Providers that are
not configured are not global environment failures. Error copy exposes a safe
reason and next action, never raw paths, SQL, credentials, or provider
transcripts.

## SOLID/YAGNI boundary

The embedded UI intentionally remains plain HTML/CSS/JavaScript served from Go
`embed`. Do not add React, Tailwind, Storybook, a design-system package, a
bundler, or a generic component runtime for this product’s six-route local
surface.

Keep domain/API policy in the application service and keep the browser adapter
thin. Share only stable presentation contracts: route metadata, state chips,
loading/error/empty states, panels, actions, disclosures, and dialogs. A new
shared primitive is justified only after the same interaction appears in at
least two independent surfaces. One-off diagnostic or provider markup should
remain local to that feature.

Do not rewrite `app.js` into modules or abstract every `innerHTML` template
without a concrete bug, repeated contract, or measurable maintenance win. Do
not make a generic renderer swallow domain-specific evidence. When a feature
changes, preserve DOM IDs, API calls, masking, Worktree trust, and Action
Broker boundaries, then add the smallest focused contract needed to protect
the change.

## Implementation constraints

- Plain HTML, CSS, and browser JavaScript; one embedded document and the
  existing static assets.
- No CDN, new package, telemetry, hosted service, or font download at runtime.
- One CSS token root, one coherent component source, and two responsive layout
  breakpoints plus the reduced-motion rule.
- Visual changes must not change product behavior, API routes, security
  boundaries, or release targets.
