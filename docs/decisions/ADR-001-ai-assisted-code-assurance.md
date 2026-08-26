# ADR-001: AI-assisted Code Assurance uses local CLI agents and evidence-first runs

Status: accepted
Date: 2026-08-26

## Context

Dev Control Room already has deterministic Checksets, an Action Broker, Agent
Profiles, interactive Agent Handoff, local artifacts/evidence conventions, and
a multi-repository fixture. The next product need is targeted mutation,
property, fuzz, static/security, contract, and end-to-end assurance, including
AI help to discover intent and author tests. The operator uses approved
Enterprise CLI tools and needs durable continuity after long breaks.

## Decision

- Retain Checksets as fast reviewed PR/baseline checks; introduce separate,
  on-demand Quality Runs and Campaigns for heavier assurance work.
- Invoke only locally installed, already authenticated Codex, Claude, and
  Gemini CLIs in v1. Codex non-interactive execution is the first real adapter;
  Claude/Gemini start with detection/profile support and fake adapters. No
  direct provider API integration, key management, or hosted orchestration is
  in v1 scope.
- Treat local Assurance Specs, structured questions/answers, run state, and
  Resume Briefs as durable memory. Provider session resume is optional.
- Persist structured results, selected evidence, usage, patches, and artifacts
  by default, rather than raw provider transcripts.
- Run deterministic tools for mutation/property/fuzz/static execution. AI plans,
  asks questions, drafts tests/configuration, triages results, and optionally
  critiques; it cannot adopt a patch or grant approval.
- Surface provider/model selection and usage. Price calculations use immutable
  official-price snapshots and are labelled estimated API list-price equivalent,
  never actual Enterprise entitlement cost.
- Preserve evidence locally with quota, retention, pin, archive/export, hash
  verification, deletion-risk warnings, and evidence-backed effect claims.
- Limit v1 runnable techniques to static/security, mutation, property-based,
  fuzz, and targeted E2E. Track contract/integration and
  differential/metamorphic adapters as
  [follow-up GitHub issue #2](https://github.com/knowgyu/dev-control-room/issues/2).

## Consequences

The product gains controlled agent automation without depending on a provider
conversation transcript or API account. It must implement provider capability
detection, fake-process tests, file/data-boundary filtering, structured output
schemas, artifact lifecycle controls, and clear cost labelling. Existing
deferred Hardener/QA/CRAP wording is superseded; existing Agent Handoff and
Action Broker contracts remain.

## Links

- Detailed plan: docs/AI_CODE_ASSURANCE_PLAN.md
- Existing foundation: docs/AI_INTEGRATION.md
- Current status index: docs/ROADMAP.md

## v0.9.0 implementation status

The decision is implemented as a bounded local foundation: structured sessions,
fake-provider E2E, real local Codex acceptance, registered native Go runners,
bounded artifacts, and the evidence dashboard are available. v0.9.0 adds
period-based effect metrics, previous-period comparisons, safe trace/report
export, archive manifests, hash-checked restore, retention pinning, and quota
visibility. It does not make the decision's full lifecycle promise complete.
Unattended Approval Scope persistence/revalidation is [#9](https://github.com/knowgyu/dev-control-room/issues/9),
isolated patch materialization and adoption ceremony remain [#10](https://github.com/knowgyu/dev-control-room/issues/10),
mutation adapter coverage remains [#5](https://github.com/knowgyu/dev-control-room/issues/5),
and causal effect attribution remains [#12](https://github.com/knowgyu/dev-control-room/issues/12).
Native provider resilience, authority, mutation, and accessibility gaps remain
in [#3](https://github.com/knowgyu/dev-control-room/issues/3)–[#5](https://github.com/knowgyu/dev-control-room/issues/5)
and [#7](https://github.com/knowgyu/dev-control-room/issues/7).
