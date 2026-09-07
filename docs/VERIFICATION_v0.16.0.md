# v0.16.0 verification record

Status: **PREPUBLICATION** — clean candidate measurement and scoped acceptance passed; package, CI, and publication remain pending.
Publication: **PENDING**.
Binary/package: **PENDING**.
Date of available evidence: 2026-09-07.

This is the candidate verification record before publication; pending items are
not release claims.

## Candidate and scope

The implementation scope is `v0.15.2..e216acb93f16adcb703303c91d040d9bbb0651ad`.
The range hardens the existing quality-pilot Assurance continuity and atomic
writes, preserves evidence across repeated restart/finalization failures,
strengthens measurement manifest boundaries, and improves pilot UI
responsiveness. The current source version is `0.16.0`; the retained
clean candidate is `8e9e6e24c9ee3b3896f13abd7a93202ecc45ed38`.

The release target is Windows amd64 only. Windows arm64 is verification-only;
Linux and arm64 release packages are not produced.

## Clean candidate dogfood

The authoritative manifest and report are
[`dogfood-measurement.json`](../artifacts/dogfood-v0.16.0/dogfood-measurement.json)
and [`dogfood-measurement-report.md`](../artifacts/dogfood-v0.16.0/dogfood-measurement-report.md).

| Field | Result |
| --- | --- |
| Commit / head | `8e9e6e24c9ee3b3896f13abd7a93202ecc45ed38` |
| Dirty state / platform | `clean` / Windows x64 |
| Run ID / required status | `dogfood-e125339045844519a9e99a79e21142d5` / `pass` |
| Required failures | none |
| Go statement coverage | `58.7%` |
| Health probe | 5/5, p50 `0.434 ms`, p95 `17.284 ms` |
| State probe | 5/5, p50 `2.881 ms`, p95 `5.090 ms` |
| Contract validator / import / dashboard | exit `0` / `201` / run ID matches |

## Scoped native and browser acceptance

- `verify-phase2-journeys.ps1`: **PASS**, 373 assertions; isolated temporary
  fixtures, CLI/MCP/first-use/recovery, real-repository registration and
  read-only scan. Log: [`journeys-v0.16.0.log`](../artifacts/journeys-v0.16.0.log).
- `verify-native-resilience.ps1`: **PASS**, 15 assertions; no real provider,
  production action, or user-data mutation.
- Current browser check: seven routes, one `h1` per route, main focus, no
  overflow at 1244px and 485px CSS widths, guide slide 2 survives reload, and
  console errors `0`. The 390px override rendered at 485px actual width; no
  390px claim is made. Real-repository registration/read-only scan passed.
- The populated measurement dashboard displayed the exact run ID
  `dogfood-e125339045844519a9e99a79e21142d5`, required-gate `PASS`, 58.7%, and
  both HTTP probes at 5/5.

## Pre-version-bump source evidence

The exact retained record is
[`verification-summary.json`](../artifacts/verification-resume-20260907/verification-summary.json).
It reports `PASS` for all 10 native Full steps, but its `sourceCommit` is
`2a954048019ff0048c71034281772c4951d322f0`. The run used a dirty worktree
with the reviewed changes on top of that commit, so it is not clean `e216acb`
or clean v0.16.0 release evidence.

Environment: native Windows, PowerShell `7.6.5`, Go `1.26.7 windows/amd64`,
Node `v24.15.0`, GCC `16.2.0`, and Git `2.53.0.windows.1`.

Passed steps:

```text
gofmt-check
go-test
ui-syntax
go-test-race
go-vet
go-mod-verify
go-build
windows-amd64-build
windows-arm64-build
git-diff-check
```

The script worker also passed Pester `6/6` in `8.56s`, PowerShell parsing, and
the diff check for `scripts/measure-dogfood.ps1` and
`scripts/measurement-contract.tests.ps1`.

## Evidence boundaries

- The historical clean v0.15.2 run at `3dec04e` recorded 12 measurements,
  required status `pass`, and 58.4% statement coverage. It is not a v0.16.0
  result.
- The later dirty `dogfood-p2-final-v2` result is not a clean baseline and does
  not establish a coverage or quality improvement. Coverage and same-commit
  latency are not causal quality evidence.
- Go 1.23 actual runtime execution and the Go 1.23 build-tag test were not run.
- Mutation campaign: **not run**.
- Causal quality score/improvement: **not run/unproven**.

## Final evidence pending from main

| Evidence | Status |
| --- | --- |
| v0.16.0 Windows amd64 binary, ZIP, archive smoke, and SHA-256 | **PENDING** |
| CI result for the final candidate | **PENDING** |
| Tag, publication, and remote asset/hash verification | **PENDING** |

When completed, the release output must contain only:

```text
dev-control-room_0.16.0_windows_amd64.zip
SHA256SUMS
```
