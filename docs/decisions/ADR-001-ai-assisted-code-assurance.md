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
