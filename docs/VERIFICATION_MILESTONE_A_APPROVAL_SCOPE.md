# Milestone A — Unattended Approval Scope verification

Status: verified for the durable approval-scope vertical slice
Updated: 2026-08-27

Candidate source commit: `4a2de02ee3969be88af9206edfd051736ce4b43f`

## Claim

An unattended action is eligible only when the current exact Project,
Repository, Worktree, Action/risk, Provider/tool contract, writable paths,
network policy, disk quota, deadline, and prohibited-operation set match the
human-approved Scope. The decision remains local, revisioned, auditable, and
fail closed.

## Implemented surface

- Schema migration 15 adds the target/state index for
  `UnattendedApprovalScope` objects.
- Draft, approve, revoke, list, get, and plan-check service/API operations are
  available under `/api/assurance/approval-scopes`.
- Approval binds only a current observed Worktree and records a service-owned
  `local-user` approval actor.
- ActionPlan stores the Scope digest, exact contract, match result, reasons, and
  check time. Broker checks the Scope during Plan, Admit, and Execute.
- Stale Scope revision updates fail with a CAS condition. A pre-launch Scope
  rejection releases the already acquired Action lease.
- Required prohibitions cannot be removed, path roots cannot be granted, and
  path requests must remain inside an approved writable root.

## Focused verification

Run from the repository root:

```powershell
gofmt -w internal/domain/assurance.go internal/domain/approval_scope_test.go internal/store/approval_scope_test.go internal/action/broker.go internal/action/approval_scope_test.go internal/app/approval_scope_test.go internal/app/web.go
go test -count=1 ./internal/domain ./internal/store ./internal/action ./internal/app -run 'TestUnattendedApprovalScope|TestBrokerPersistsExactApprovalScope|TestBrokerPersistsApprovalScopeMismatch' -v
```

Result: PASS on the local Windows workspace. The focused run covered domain
contract rejection and exact matching, SQLite migration/index and revision CAS,
ActionPlan audit evidence, revoke-after-admit execution rejection with lease
cleanup, protected HTTP approve/revoke routes, and plan-check reporting.

## Full verification

The repeatable full runner was executed after the candidate commit:

```powershell
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full -ArtifactDirectory artifacts\verification-milestone-a
```

Result: PASS. The runner recorded `go test -count=1 ./...`, UI JavaScript
syntax, `go test -count=1 -race ./...`, `go vet ./...`, `go mod verify`,
`go build ./...`, Windows amd64/arm64 cross-builds, and `git diff --check`.
Environment: Windows, PowerShell 7.6.4, Go 1.26.7 windows/amd64, Node
24.15.0, GCC 16.2.0. Full run duration was about 5m37s.

Build artifact hashes:

| Artifact | SHA-256 |
| --- | --- |
| `dev-control-room-windows-amd64.exe` | `0E10D40A2BDFE4E2AC63A8719704FD25C914567AC6D43181D2972EEEF42916E9` |
| `dev-control-room-windows-arm64.exe` | `223FC79FF84527E457491FEF197A37900F7ECD3E5690F12D9652323A3869B963` |

Machine-readable summary and the full log remain at
`artifacts/verification-milestone-a/verification-summary.json` and
`artifacts/verification-milestone-a/verification.log` in the local workspace.

## Evidence chain

```text
approved Scope revision + digest
        ↓ exact Plan match
persisted Plan match/reasons/check time
        ↓ fresh Admit/Execute match
lease + bounded execution boundary
```

No raw transcript, credential, or unbounded command is introduced. Scope
matching is not an effect claim; it is authority evidence for later
traceability.

## Gaps kept explicit

- Full package/race/vet/build/UI and clean-state journey gates are recorded in
  the release verification after the candidate commit, not inferred from this
  focused run.
- Native Provider resilience, provider-authoritative CI, mutation execution,
  patch materialization/adoption, causal effect attribution, and full manual
  keyboard/second-device acceptance remain partial in Milestones B–G.
- The scope API is available; the product UI still needs a deliberate surface
  for authoring and reviewing scopes rather than exposing every contract field
  on the first screen.

## Resume Brief

Question: none for this slice.
Why blocked: not blocked; the next safe work is the first remaining partial
Phase 1 slice, followed by the planned Phase 2 UI refinement.
Safe work completed: durable Scope lifecycle and Broker revalidation.
Next safe command: `git status --short --branch; go test -count=1 ./...`.
