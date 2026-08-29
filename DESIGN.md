# Dev Control Room design system

Status: active
Target: v0.12
Refreshed: 2026-08-30

This is the source of truth for the embedded loopback UI. The research and
product reasoning behind this contract is preserved in
`docs/AI_GENERATED_UI_RESEARCH_2026-08-30.md`. Changes must satisfy both files.
The system is implemented in plain embedded HTML, CSS, and JavaScript; it is not
a package or a frontend framework.

## Product job

Dev Control Room is a **local repository operational ledger** for one developer
on Windows. It helps the operator answer, in order:

1. what changed or needs attention;
2. what evidence supports that observation;
3. what safe action is available and whether approval is required.

The signature sequence is:

```text
관찰 -> 근거 -> 승인
```

It is expressed through ordered rows and linked evidence, not through a slogan,
illustration, or accent color. The UI is not a chat, marketing dashboard,
generic admin template, raw-log viewer, IDE, or deployment console.

## Information architecture

The shell uses one compact top navigation with six routes. A persistent sidebar
would duplicate a shallow route hierarchy, so it is not part of the product.

| Route | Visible page `h1` | Main job |
| --- | --- | --- |
| `home` | 첫 저장소 연결 / 오늘의 상태 | one first-use action or an exception-first operating summary |
| `projects` | 프로젝트 | registered scope, repositories, and Worktrees as inventory |
| `work` | 작업 | observations, evidence, plans, approvals, and Actions |
| `assurance` | 검증 | repeatability, effect, usage, and linked evidence |
| `diagnostics` | 진단 | required environment and optional execution capability |
| `activity` | 활동 기록 | chronological operational history |

The visible label for `home` is `상태`, which describes its job more clearly.
Each route owns one visible `h1`. The page name is not repeated as an eyebrow,
panel title, or promotional sentence. `main` keeps `id="main-content"` and
`tabindex="-1"`; the route adapter updates its accessible label and moves focus
after hash navigation.

## Page composition

The default page is flat. Spacing and rules establish hierarchy before a box is
introduced.

```text
compact top bar  [brand] [routes]                    [local state] [action]
────────────────────────────────────────────────────────────────────────────
page title                                      optional route-level action
one useful context line

section title                                                   small action
────────────────────────────────────────────────────────────────────────────
status / subject             evidence or consequence              next action
status / subject             evidence or consequence              next action
```

A bordered surface is justified only when it contains editable controls or an
operation lifecycle, isolates an error/approval/safety consequence, or is a
native disclosure/dialog or selected detail inspector. Do not nest boxes around
a heading, list, and every row. Do not add a box merely to fill whitespace.

### Home states

First use contains one heading, one short reason, one `폴더 선택` action, and a
compact readiness line. It has no onboarding hero, numbered tutorial strip, or
duplicate bottom call to action.

Established use opens with exceptions and the next safe action. Healthy counts
are compressed into a single summary line. Recent execution and assurance
evidence follow as ledger rows. Setup guidance is linked, not permanently
expanded.

### Route rules

- Projects uses a list/detail inventory. Repository and Worktree state are rows,
  not an equal-card gallery.
- Work orders content by the operator's decision path: observation, evidence,
  plan, approval, execution. Approval-required state stays visible.
- Assurance uses one compact filter bar and comparable evidence rows. Unknown
  measurement is never displayed as zero.
- Diagnostics separates required local capability from optional Providers.
  Optional absence is neutral. Advanced setup and cleanup remain disclosures.
- Activity is a chronological ledger with subject, outcome, time, and expandable
  evidence. Wide machine values may scroll inside their own region.

## Visual direction

The visual language is a maintained field notebook: warm neutral canvas,
paper-like surfaces, dense but breathable rows, strong type, and precise rules.
The deliberate risk is reducing decorative containers enough that the product
reads more like an instrument than a dashboard.

There are no gradients, glass effects, oversized radius, decorative background
shapes, emoji icons, floating blobs, or large shadows. A state rail is used only
where the state changes the operator's decision.

### Tokens

`internal/app/ui/app.css` has exactly one `:root` block. Routine components use
semantic tokens; literal colors outside that block require a documented reason.

| Token | Value | Use |
| --- | --- | --- |
| `--canvas` | `#F5F4F0` | warm application background |
| `--surface` | `#FFFFFF` | forms, dialogs, and focused detail |
| `--surface-muted` | `#ECEBE6` | selected rows and quiet grouped evidence |
| `--ink` | `#1F2628` | primary text |
| `--muted` | `#687173` | supporting text and metadata |
| `--rule` | `#D5D5CE` | section and row dividers |
| `--rule-strong` | `#AEB4B2` | selected/focused structural edge |
| `--accent` | `#285F58` | primary action and current route |
| `--accent-soft` | `#E1ECE8` | selected or hover surface |
| `--success` / `--success-soft` | `#2F6A4F` / `#E5F0E9` | ready/completed |
| `--warning` / `--warning-soft` | `#865B24` / `#F6ECD9` | attention/approval |
| `--danger` / `--danger-soft` | `#9B4149` / `#F5E5E6` | blocked/failed/destructive |
| `--neutral` / `--neutral-soft` | `#687173` / `#E9ECEB` | absent/not measured |

State always includes readable text. Color is never the sole carrier.

### Typography

The bundled UI face is:

```text
"Pretendard Variable", Pretendard, "Segoe UI Variable", "Segoe UI",
"Malgun Gothic", sans-serif
```

`PretendardVariable.woff2` v1.3.9 is embedded in the executable and served from
the loopback origin with `font-display: swap`; the exact asset and license are
recorded in `THIRD_PARTY_POLICY.md`. No font is requested from a CDN.

Paths, IDs, commands, hashes, and machine values use `"Cascadia Mono",
"Cascadia Code", Consolas, monospace`. Google Sans Text/Flex is not used because
mixed Hangul fallback would undermine metric consistency. A second bundled code
font is YAGNI until machine-value scanning shows a concrete problem.

Korean headings use near-normal letter spacing, `text-wrap: balance` or
`pretty`, and `word-break: keep-all`. Body text remains left aligned. Counts and
comparison values use tabular numerals.

## Shared presentation contracts

These conventions are the complete shared system:

- `AppShell`: `.app-shell`, `.workspace`, `.topbar`, and `.primary-nav`.
- `PageHeader`: `.page-heading`; one `h1`, one optional context line, and one
  optional route-level action group.
- `SectionHeader`: `.section-heading`; one `h2` plus an optional adjacent action.
- `Ledger`: `.ledger`; a collection of comparable operational records.
- `LedgerRow`: `.ledger-row`; state/subject, evidence/consequence, and an
  adjacent action. It may use a semantic state rail.
- `EvidenceFlow`: `.evidence-flow`; an ordered `관찰`, `근거`, `승인` sequence
  only when those three phases are present in the data.
- `Button`: `.button` with primary, danger, quiet, and small variants.
- `StateText`: `.state-text` for routine status. It includes text and may include
  a small dot or rail.
- `StatusChip`: `.chip` only when a compact state label must remain scannable in
  a dense row. It is not decoration or section metadata.
- `ActionGroup`: `.item-actions` or `.toolbar`, adjacent to the object changed.
- `Disclosure`: native `details/summary` for advanced evidence and settings.
- `FormSurface`: `.form-surface` or the retained `.panel` for editable controls
  with a clear ownership boundary.
- `EmptyState`, `ErrorState`, and `Dialog`: intentional state surfaces with a
  cause and safe recovery action.

Feature classes such as `.finding`, `.repository-card`, `.assurance-record`,
`.trace-node`, and `.command-output` compose these contracts. They do not gain
independent token systems. A shared primitive is justified only when the same
interaction exists in at least two independent routes.

## Copy and progressive disclosure

Use concise Korean nouns for labels and short `합니다` sentences only for a
cause, consequence, or recovery step. Keep exact terms such as Worktree,
Provider, Action, artifact, and HEAD when translation would reduce precision.
Do not use an eyebrow to restate the following heading. Do not label routine
sections with marketing phrases such as “한눈에”, “스마트”, or “강력한”.

The first viewport shows state, consequence, and action. Revision IDs, digests,
exact argv, artifact manifests, pricing basis, raw diagnostics, and historical
detail belong in native disclosure or a focused inspector. A blocking state or
required approval is never hidden.

Every data surface owns loading, empty, partial-failure, approval-required, and
completed behavior. Error copy exposes a safe reason and recovery step, never
raw credentials, provider transcripts, or unmasked sensitive paths.

## Accessibility and responsive behavior

- Keep the skip link as the first keyboard target.
- Use links for navigation, buttons for actions, labels for controls, and native
  disclosure/dialog before ARIA.
- Preserve visible `:focus-visible`; async status uses `aria-live="polite"`.
- Controls have a practical minimum 40px height and no action depends on hover.
- Long paths and IDs break safely; wide ledgers scroll within their region.
- Respect `prefers-reduced-motion`; do not use `transition: all`.
- Activity tables use a caption and column-scoped headers.
- At `980px`, dense three-column rows simplify and navigation may wrap.
- At `720px`, ledger columns become one vertical reading order, actions remain
  adjacent to their subject, and navigation scrolls horizontally.

## SOLID/YAGNI boundary

The browser adapter stays thin over the application service. Domain/API policy,
masking, Worktree trust, and Action Broker rules do not move into presentation
helpers. Render helpers may share escaping, formatting, state language, ledger
rows, and intentional async states; they must not become a generic renderer that
hides domain evidence.

Do not add React, Tailwind, Storybook, a bundler, a design-system package, a
client state library, or a generic component runtime for this six-route local
surface. Do not split `app.js` into modules or abstract every template without a
repeated contract, concrete bug, or measurable maintenance gain.

When a feature changes, preserve DOM IDs, API calls, masking, route state,
Worktree trust, and Action Broker boundaries. Add the smallest contract and test
that protect observable behavior.

## AI-assisted change gate

For every material UI change:

1. identify the operator's decision and the repeated UI being removed;
2. implement one scoped design-contract change;
3. inspect the code diff for token/copy/component drift;
4. inspect real loading, empty, populated, error, and approval states;
5. inspect desktop and narrow widths plus keyboard focus and console output;
6. run the static UI checkset and proportional repository verification.

Do not accept a result merely because its first screenshot looks polished.
