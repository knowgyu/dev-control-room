# Milestone D verification

Status: accepted for authoring and review boundary
Updated: 2026-08-26

Assurance Q&A, revisioned Specs, isolated patch artifacts, and critic advice
are now durable. A proposal can be explicitly adopted or rejected, but the
service has no proposal path that commits, pushes, opens a PR, edits CI, or
grants an Action approval. The critic is advisory and is stored with a
confidence label.

```text
scope: WSL; isolated generic Git fixture
commands:
  go test ./internal/app ./internal/store -run 'TestAssuranceAuthoring|TestFakeProvider|TestQualityRun' -count=1  PASS
```

gaps: native patch review UI, real provider critic, and native Worktree process
acceptance remain separate. The adopt test changes only the persisted proposal
state; it does not touch the fixture repository.

