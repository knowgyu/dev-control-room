# Milestone G verification

Status: Phase 1 automated dogfood accepted; native real-provider acceptance
pending and explicitly recorded
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

gaps: authenticated real Codex/Claude/Gemini CLI, native Windows non-TTY and
process-tree acceptance, and a human-reviewed effect report were not available
from this source workspace. Those are native/manual gates, not inferred from
fake or WSL success. The exact procedure is recorded in
docs/NATIVE_WINDOWS_SMOKE.md and the verification playbook.

