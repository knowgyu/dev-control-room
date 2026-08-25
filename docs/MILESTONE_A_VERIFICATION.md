# Milestone A verification

Status: accepted for the current source checkpoint
Updated: 2026-08-26

## Scope

Milestone A adds the assurance lifecycle without changing Checkset or Action
Broker semantics. Assurance records are additive, digest-bound, revisioned, and
stored behind the existing masking boundary. Active-session uniqueness and stale
revision rejection are enforced by SQLite and the typed Store methods.

## Evidence

```text
scope: WSL source workspace; local SQLite lifecycle tests
commands:
  go test ./internal/domain ./internal/store                 PASS
  go test ./internal/assurance ./internal/store ./cmd/dev-control-room -run 'Test(Codex|FakeProvider|AssuranceLifecycle|ProjectWorktreeListJSON)' -count=1  PASS
  gofmt -w internal/domain/assurance.go internal/store/assurance.go internal/store/migrations.go internal/app/service.go internal/app/assurance.go internal/app/web.go  PASS
```

The focused Store test proves additive migration, one active session record,
idempotent same-object persistence, and rejection of a stale revision. The
native Windows toolchain and interactive acceptance are still separate gates;
they are not claimed by this record.

gaps: native Windows runtime, Scheduler, real provider CLI, and GitHub/Jenkins
were not exercised in this WSL milestone check. No external or destructive
operation was performed.

