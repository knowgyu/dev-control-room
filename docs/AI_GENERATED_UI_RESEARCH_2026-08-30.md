# AI-generated UI research and product response

Status: adopted for v0.12
Research window: 2026-06-30 through 2026-08-30
Recorded: 2026-08-30

## Why this exists

This record preserves the research and product judgment behind the v0.12 UI
work. It is not a mood board and it is not a license to copy another product.
Future UI changes should use it with `DESIGN.md` so that a fresh AI session does
not reconstruct the same generic dashboard patterns that this release removes.

## Research finding

The recurring problem in AI-generated interfaces is not one visual style. It is
a workflow failure:

1. a model fills missing product context with the statistical average of SaaS
   dashboards;
2. the first coherent-looking draft is mistaken for a finished decision;
3. components are named, but their usage rules and edge states are missing;
4. later prompts locally improve one screen while silently drifting tokens,
   hierarchy, copy, accessibility, and interaction contracts.

Recent design-system work reaches the same practical conclusion from different
directions. Figma's August workflow guidance emphasizes intent, exact component
and state context, and a repeated design/code review loop. Its July Decagon case
study treats design-system saturation and edge states as prerequisites for
reliable agent work. A July study of 49 professionals reports that design-system
context and AI-assisted workflows improved both speed and implementation
completeness, but did not remove the need for expert review. Independent audits
of generated frontends also continue to find serious keyboard, semantics,
responsive, and reduced-motion failures behind polished screenshots.

The useful response is therefore a controlled loop:

```text
product job -> approved design contract -> one scoped change
            -> code diff -> real browser states -> keyboard/narrow-width check
            -> keep, revise, or remove
```

AI remains useful for breadth, alternatives, and mechanical implementation. It
does not choose the product's hierarchy or decide that a plausible first draft
is good enough.

## Sources reviewed

- Figma, "How to move fast toward the right thing" (2026-08-13):
  https://www.figma.com/blog/how-to-move-fast-toward-the-right-thing/
- Figma, "Workflow Lab: Moving between design and code with agents"
  (2026-08-26):
  https://www.figma.com/blog/workflow-lab-moving-between-design-and-code-with-agents/
- Figma, "How Decagon uses AI for design-system saturation" (2026-07-10):
  https://www.figma.com/blog/how-decagon-uses-ai-for-design-system-saturation/
- "Design-System-Aware Development with AI" (2026-07-14), study of 49
  practitioners: https://arxiv.org/abs/2607.13156
- State of AI in Design Systems, July 2026 snapshot of 20 design systems across
  six platforms: https://github.com/kaelig/state-of-ai-in-design-systems
- Knapsack, "AI Doesn't Read Your Design System the Way You Do" (2026-07-27):
  https://www.knapsack.cloud/blog/ai-doesnt-read-your-design-system-the-way-you-do
- Better Design, "How to stop AI UI drift" (2026-08-03):
  https://better-design.com/guides/how-to-stop-ai-ui-drift
- SoBold, "How our design team actually works with AI" (2026-08-03):
  https://sobold.co.uk/news/how-our-design-team-actually-works-with-ai/
- Design with Claude, independent generated-frontend accessibility audit,
  updated July 2026: https://www.designwithclaude.com/design-research/ai-generated-frontends
- CodeWave, "How to spot AI-generated websites" (2026-07-20):
  https://code-wave.co.za/blog/how-to-spot-ai-generated-websites

The vendor and agency sources are experience reports rather than controlled
research. They are used for recurring workflow observations, not as numerical
proof. The academic paper and repository survey provide the stronger empirical
support; all findings are still translated through this product's constraints.

## Audit of the pre-v0.12 interface

The 2026-08-30 source audit found useful accessibility foundations alongside a
generic visual hierarchy:

- 43 eyebrow labels, 80 panel-class uses, 34 `h2` elements, and 14 status chips;
- 12 literal colors outside the single root token block;
- first-use Home repeated one action through a hero, a four-step workflow, and
  another call-to-action surface;
- many bordered containers had equal visual weight even when their decisions
  were unrelated;
- page name, purpose sentence, card title, and card explanation often restated
  the same idea;
- decorative labels and status chips made the interface look active without
  helping the operator decide.

The interface already had important strengths that v0.12 must preserve:

- one visible `h1` per route and no duplicate brand block;
- a skip link, visible focus treatment, polite async status, reduced-motion
  handling, and locale-aware date/number formatting;
- progressive disclosure for detailed evidence;
- optional unconfigured Providers represented as neutral rather than failed;
- the Action Broker, masking, Worktree trust, loopback-only server, and API
  boundaries independent of presentation.

## Product-specific direction

The product is a **local repository operational ledger**, not a miniature SaaS
admin dashboard. Its visual grammar follows work and evidence:

```text
관찰 -> 근거 -> 승인
```

This sequence is the signature interaction. It appears as ordered ledger rows,
compact state language, and a visible link from a detected condition to its
evidence and safe next action. Teal is not the identity by itself; the evidence
flow is.

### Home

- First use has one direct folder-selection action and a compact readiness
  summary. It does not explain the same registration task three times.
- Established use is exception-first: blocked and attention items are open,
  healthy state is compressed, and the latest execution/evidence is nearby.
- A large marketing hero, numbered onboarding cards, and duplicate bottom CTA
  are not part of the product shape.

### Other routes

- Projects is an inventory ledger. Repositories and Worktrees are rows with
  local actions, not a gallery of equal cards.
- Work is a decision queue. A finding leads to evidence and then an Action
  request; decorative metrics do not precede the queue.
- Assurance is an evidence register. Repeatability, effect, usage, and linked
  artifacts share a comparable row grammar.
- Diagnostics is an environment checklist. Required local capability is
  separated from optional execution capability without treating absence as a
  global failure.
- Activity is a chronological ledger optimized for scanning exact subject,
  outcome, time, and expandable evidence.

## Design-system response

The design system stays deliberately small. It consists of semantic tokens and
stable presentation contracts, not a new framework:

- shell and primary navigation;
- route header and optional one-line context;
- ledger, ledger row, and evidence sequence;
- inline state text for routine status, with chips reserved for compact states
  that must survive dense layouts;
- buttons and adjacent action groups;
- native disclosure and dialog;
- loading, empty, error, approval-required, and completed state patterns.

Panels are used only when a boundary has behavior, ownership, or a distinct
background—not merely to decorate spacing. A shared primitive is added only
after the same interaction exists in two independent product surfaces. Domain
evidence stays domain-specific.

## Typography decision

v0.12 bundles Pretendard Variable locally so Korean metrics do not depend on an
operator's installed fonts or a network request. Machine values continue to use
the native Windows monospace stack unless a separately reviewed font materially
improves scanning. Google Sans Text/Flex is not used for body copy because its
Hangul fallback would create mixed metrics. Google Sans Code remains a possible
future technical-face option, not a requirement for this release.

Bundling must include the exact upstream version, license, source URL, and a
static browser check that the asset is embedded and served locally. There is no
runtime CDN or analytics request.

## Guardrails for future AI-assisted changes

Before implementation:

- name the operator, current state, decision, and safe next action;
- identify which existing primitive owns the interaction;
- list the loading, empty, partial, blocked, and completed states;
- state what existing repetition will be removed.

Before merge:

- inspect the diff for new literal colors, one-off spacing, duplicate copy,
  nested panels, decorative chips, and broad `transition: all` rules;
- inspect every affected route in a real browser at desktop and narrow widths;
- complete keyboard focus, hash navigation, async state, and console checks;
- run the static UI checkset and the repository's proportional verification
  gate;
- update `DESIGN.md` only when a stable contract changed.

The acceptance question is not "does this look polished?" It is "can the local
operator see what changed, why it matters, and what safe action is available
without reading the same message twice?"

## v0.13 follow-up: repair the product surface

The first implementation exposed a second class of problem: some surfaces were
technically valid but looked unfinished because the page had little orientation,
the first-use state occupied a large empty area, and a warning token was used as
a broad background. The follow-up keeps the small design system but makes the
product job explicit at the point of entry.

Decisions applied:

- The palette moves from warm paper neutrals to a cool blue-gray workbench. The
  warning color remains available for approval state, but no longer paints the
  entire external-operation surface.
- Home first use now states the three actions—connect, verify, execute—and has a
  direct link to a four-step `사용법` deck. The deck is an in-app route with
  `#guide?slide=N` URL state, so it is shareable and survives refresh without
  adding a second documentation runtime.
- Windows folder selection now uses the Explorer-backed `IFileOpenDialog` COM
  API with `FOS_PICKFOLDERS`, replacing the legacy `SHBrowseForFolderW` dialog.
  The app still receives a path only after the user confirms it.
- The MCP surface remains narrow. `jenkins.plan` is review-only,
  `jenkins.trigger` calls the existing Action broker and requires a human
  approval already recorded in the app, and `jenkins.latest` is read-only. No
  generic shell or file reader is added.
- Primary Korean copy uses short, direct `합니다` sentences only for cause,
  consequence, and recovery. Technical identifiers remain in detail views, not
  in the first explanation of a page.

The validation loop now treats the guide route, native picker implementation,
MCP tool list, and visible browser geometry as contracts. The release checklist
is: static UI contract → unit/integration tests → Windows build → real browser
route/URL/focus/overflow checks → leave the verified local server running. This
is a response to the recurring failure mode where an implementation stopped after
compilation while the visible product still felt broken.
