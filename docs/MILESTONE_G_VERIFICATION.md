# Milestone G verification

Status: partial; fake E2E, real local Codex, and clean-state journey are verified, full accessibility/manual acceptance remains active work
Updated: 2026-08-26

The isolated three-repository fixture exercises baseline discovery, fake-agent
continuity, Quality Run evidence, artifacts, effects, and dashboard aggregation
for frontend/backend/database-shaped repositories. It uses no company paths,
credentials, cloud provider, production action, commit, push, or PR mutation.

```text
scope: WSL; three fresh local Git repositories; fake provider E2E
commands:
  go test ./internal/app -run TestAssuranceDogfoodThreeRepositoryFixture -count=1  PASS
```

## v0.8.0 status update

The candidate adds a 239-assertion fresh-home journey, 46-assertion real local
Codex acceptance, and Browser evidence for first-use/return/narrow views,
registration, Provider recovery, finding focus, and Enter activation. This
does not replace full Tab/Space traversal, native dialog Esc delivery,
second-device/assistive-technology review, human-reviewed effect reporting, or
the Codex resilience cases. Accessibility/manual acceptance remains
[#7](https://github.com/knowgyu/dev-control-room/issues/7); provider resilience
remains [#3](https://github.com/knowgyu/dev-control-room/issues/3).
