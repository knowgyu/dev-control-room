# Phase 2 — Product usability and interface plan

Status: implemented and released in v0.6.0; native provider/manual gaps remain recorded
Updated: 2026-08-26
Owner: local operator
Prerequisite: [AI_CODE_ASSURANCE_PLAN.md](AI_CODE_ASSURANCE_PLAN.md) Phase 1
is implemented and accepted at its documented safety and artifact boundaries.

## Outcome

Make Dev Control Room understandable and useful to a person opening it for the
first time, without hiding its operational safety model. The interface must
make three things evident with very little reading:

1. what is currently connected, registered, and safe to use;
2. the one useful next action for the current situation; and
3. what outcome the configured control room can now produce.

This is a product-usability phase, not a cosmetic reskin. It covers
information architecture, interaction flow, Korean UI copy, the environment
doctor, CLI discoverability, and a repeatable user-validation loop.

## Fixed decisions

| Area | Decision |
| --- | --- |
| Sequence | Complete and accept Phase 1 first. Phase 2 may prepare designs and test scripts, but does not absorb unfinished assurance features. |
| Visual reference | Use Notion-like calm hierarchy, restrained chrome, readable density, and progressive disclosure as principles. Do not copy Notion branding, layouts, or UI assets. Cross-check a developer-tool reference such as Linear/Vercel when technical state needs more density. |
| Design source | Before UI implementation, create a repository `DESIGN.md` that records the adopted visual and interaction rules, component states, copy limits, and anti-patterns. It is an agent-readable source of truth, not a screenshot collage. |
| Korean copy | Review all user-visible Korean as a dedicated pass after interaction copy is drafted. Use [fluent-korean](https://github.com/snflkd/fluent-korean) as a writing-quality reference, adapted for ChatGPT/Codex project instructions rather than assuming its Claude plugin can be installed here. Use concise `합니다`체 when a full sentence is useful; otherwise end labels and status copy as clear noun phrases. Prefer plain, consistent product Korean over explanatory or promotional prose. |
| Optional capabilities | Missing Claude, Gemini, Jenkins, Kubernetes, or other optional features are neutral “not configured” states. They must not make the whole environment unhealthy or appear as repeated warnings. |
| MCP | Do not make an MCP server a Phase 2 requirement. The existing CLI is the primary discoverable automation interface. Reconsider MCP later only if a concrete external-client workflow needs structured programmatic access beyond the CLI. |
| Safety | A simpler interface must not relax Action Broker approval, Worktree trust, masking, data boundaries, or the human patch-adoption rule. |

## Current experience problems to resolve

1. The environment doctor renders each raw finding independently. A single
   `codex` resolution failure becomes both `Tool · codex` and `Agent Profile ·
   codex`, which looks like two separate broken installations.
2. Default profiles for Codex, Claude, Gemini, and `claude-local` are created
   even if the operator never intends to use them. Their absence currently
   contributes to a broad “some features unavailable” warning.
3. On this Windows machine, `codex` first resolves to the npm `codex.cmd`
   launcher in `%APPDATA%\\npm`. The direct-execution doctor deliberately
   accepts only an absolute local `.exe`, so it refuses the shim before
   execution. This is an execution-boundary decision, not evidence that Codex
   is uninstalled. The screen does not explain that distinction or offer the
   correct recovery path.
4. The home flow exposes generic workflow prose and many empty panels before
   it establishes first value: register a repository and see its state.
5. Several screens explain internal concepts rather than giving a concise,
   context-specific action and a short reason only when it helps recovery.

## Scope

### A. Information architecture and first-use flow

Create two deliberately different home states.

**First-use state**

- Primary action: choose a folder and discover/register Git repositories.
- Show no more than the immediate next step plus progress toward first value.
- State clearly what registering the folder enables: project, repository, and
  Worktree status; then a path to discovered PR checks.
- Keep AI providers, connectors, external operations, and advanced settings
  out of the critical path. They remain discoverable as optional settings.

**Established state**

- Show a compact operational summary: registered project/repository count,
  observation freshness, important findings, executable capabilities, and the
  one next action.
- Make the system’s benefit visible through results (for Phase 1, Quality Run
  evidence and effect records), not generic claims about automation.
- Preserve the morning overview goal in `docs/PRODUCT.md`; do not replace it
  with a marketing dashboard.

### B. Provider readiness and environment recovery

Replace raw source-level warning cards with one provider/capability card.

| User-facing state | Meaning | Primary action |
| --- | --- | --- |
| Ready | A verified launch method and usable profile exist. | Open or use the capability. |
| Detected — execution needs confirmation | A command or package exists, but the configured safe launcher policy has not accepted it. | Review/verify an offered launcher method. |
| Profile needs setup | The CLI is usable but its model, scope, or execution policy is incomplete. | Open the exact profile field that is missing. |
| Not configured (optional) | The operator has not chosen to use this provider. | Add only if wanted; no warning. |
| Unavailable | A capability explicitly required by the selected task cannot run. | Show a specific recovery action and safe diagnostic detail. |

For Codex, preserve the distinction between command discovery and safe process
execution in the data model, but show it together. Reveal the resolved command,
launch mode, probe category, and sanitized failure detail only in “details”.
Never recommend installation when a launcher was already detected.

The Phase 2 implementation must decide, document, and test the supported
Windows execution method for npm shims and packaged Codex installations. It
may not simply permit arbitrary `.cmd` files. A supported adapter must validate
its interpreter/arguments and perform a bounded harmless probe before marking
the provider Ready.

### C. Interaction and visual system

Before UI edits, use the `awesome-design-md` references to inspect the Notion,
Linear, and one developer-tool design system. The repository describes
`DESIGN.md` as a source for visual theme, typography, components, layout,
depth, and agent instructions. Use that model to create this project’s own
small `DESIGN.md`; do not import a reference wholesale.

The resulting rules should include:

- one primary task per page, an explicit page title, and at most one short
  supporting sentence by default;
- structured state before prose: status, consequence, then action;
- progressive disclosure for diagnostic detail and advanced configuration;
- semantic status color with text labels, never color alone;
- predictable empty, loading, partial-failure, blocked, and completed states;
- responsive keyboard-accessible controls with focus states; and
- no decorative dashboard metrics unless they lead to a decision or action.

### D. Korean UX copy review

Create a small product-copy guide in or beside `DESIGN.md` before string
changes. Its fixed default is concise `합니다`체 for full sentences and clear
noun phrases for labels/statuses where a sentence adds no value.

The review pass inventories every visible Korean string in `internal/app/ui`.
For each changed surface, check:

- Is the sentence necessary for the immediate decision?
- Does the text name the user-visible cause and outcome rather than an
  implementation detail?
- Does the button start with a concrete action, without vague phrases such as
  “do this” or generic “configure” where a target can be named?
- Is it concise, natural Korean rather than an English-shaped explanation?
- Does the error reassure only when there is genuine uncertainty, without
  hiding risk or inventing certainty?

The critic may propose revisions, but final visible copy remains human-reviewed
as part of the Phase 2 acceptance sample.

### E. CLI and future client access

Improve the CLI before considering new protocol surface:

- `dev-control-room --help` and subcommand help must state the first-use path,
  required arguments, examples, and where to find machine-readable JSON.
- `doctor` output must distinguish optional/unconfigured from task-blocking
  unavailable states and expose a concise remediation code.
- Every important UI action must have either an equivalent CLI command or an
  explicit reason it is interactive-only.

MCP remains a future integration decision. Revisit it only with an example such
as “another agent needs to query the active project state and propose a bounded
Action without screen scraping.” Any future MCP tool must preserve the same
policy and approval boundaries as CLI/UI calls.

## Verification loop

Phase 2 is accepted by evidence, not by visual preference alone.

1. **Journey specification.** Write runnable scripts for: first project
   registration; return after a day away; provider readiness recovery; optional
   provider left unconfigured; finding-to-evidence drill-down; Quality Run
   result review; and a blocked/approval-required action.
2. **Clean-state exercise.** Run these flows against a fresh local data
   directory and the sample multi-repository fixture. Record the path taken,
   commands, screenshots, and any unexpected decision point.
3. **Comprehension check.** A person unfamiliar with the application should be
   able to answer, without reading logs: “what is this for?”, “what can I do
   now?”, and “why is this blocked?” If not, fix the flow or wording, not just
   the tooltip.
4. **Automated regression.** Add focused Go/web tests for provider state
   grouping, optional-state severity, first-use/established-state rendering,
   CLI help/JSON output, and the chosen Windows Codex launcher policy.
5. **Visual/accessibility review.** Capture stable screenshots at desktop and
   narrow widths; keyboard-test all new controls; verify focus, labels, status
   text, empty states, and error recovery.
6. **Korean copy sample.** Review representative onboarding, empty, warning,
   and recovery messages against the copy guide. Record accepted examples and
   rejected revisions.

Initial measurable targets, to refine during the journey draft:

- a clean user reaches first repository status without entering optional
  provider or connector configuration;
- an optional missing provider produces no global warning;
- an installed-but-untrusted Codex launcher is explained as detected rather
  than missing, with one safe next action;
- each blocking state has one primary recovery action and sanitized detail;
- no page leads with more than one paragraph of explanatory copy.

## Work sequence

1. **P2.0 Research and constraints** — [implemented] inspect the current flows and recent
   native screenshot; read selected Notion/Linear/design-system references;
   read the fluent-korean guidance; create `DESIGN.md`, copy guide, journey
   scripts, and an ADR if the Windows launcher policy changes.
2. **P2.1 State-model correction** — [implemented] split “found”, “trusted to launch”,
   “profile ready”, “optional”, and “task required” in the backend contract;
   add the concrete Windows launcher acceptance tests.
3. **P2.2 First-use and provider surfaces** — [implemented] implement the two home states
   and grouped provider cards, with concise contextual recovery actions.
4. **P2.3 CLI parity and copy pass** — [implemented] improve help/doctor output, then run the
   Korean UI-string review. Do not allow generic prose to re-enter as a result
   of implementation.
5. **P2.4 Validation and release decision** — [completed] run the complete verification
   loop, compare against the prior screenshot issues, update status documents,
   and create follow-up issues only for clearly bounded deferred work.

## Non-goals and deferred work

- No wholesale frontend-framework migration.
- No imitation of Notion’s brand or copyrighted assets.
- No new hosted web dashboard merely for onboarding.
- No MCP-first architecture or public remote control surface.
- No change to Phase 1 quality technique scope, artifact retention, cost
  ledger, or approval policy.
- Broad team personalization, multi-user onboarding analytics, and A/B testing
  remain out of scope for this local single-user product.

## Next-session assignment prompt

Use the following as a new ChatGPT/Codex task after Phase 1 is accepted:

> Work only on Phase 2 in `C:\\Users\\knowgyu\\workspace_window\\dev-control-room`.
> First read `AGENTS.md`, `docs/HANDOFF.md`,
> `docs/PHASE_2_PRODUCT_USABILITY_PLAN.md`, `docs/PRODUCT.md`, and the current
> `git status`. Treat the Phase 2 plan as the scope contract; do not reopen or
> implement unfinished Phase 1 features. Inspect the existing UI and provider
> doctor before proposing changes. Read the linked fluent-korean and selected
> awesome-design-md references, then create the project `DESIGN.md`, the copy
> guide, and journey-based acceptance checklist before changing UI code. Keep
> Notion as a principle reference, not a visual clone. Preserve local-first
> safety and do not make MCP a prerequisite. For every change, add or update
> automated tests and run the documented clean-state, screenshot, keyboard,
> and Korean-copy verification. Stop to ask if the supported Windows Codex
> launcher policy requires relaxing a security boundary. Make commits only for
> coherent verified slices; do not push unless explicitly asked.

## References consulted

- [fluent-korean README](https://github.com/snflkd/fluent-korean): output-style
  guidance for clear Korean; it explicitly notes that its instruction text can
  be adapted in other AI environments.
- [Awesome DESIGN.md](https://github.com/abhayjnayakk/awesome-design-md): an
  MIT-licensed reference collection and workflow for agent-readable design
  systems, including Notion, Linear, and developer-tool starting points.
