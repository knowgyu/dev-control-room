# v0.16.0 verification record

Status: **PENDING** — pre-version-bump regression evidence only.
Publication: **PENDING**.
Binary/package: **PENDING**.
Date of available evidence: 2026-09-07.

## Candidate and scope

The intended v0.16.0 scope is `v0.15.2..e216acb93f16adcb703303c91d040d9bbb0651ad`.
The range hardens the existing quality-pilot Assurance continuity and atomic
writes, preserves evidence across repeated restart/finalization failures,
strengthens measurement manifest boundaries, and improves pilot UI
responsiveness. The current source version is `0.16.0`; the retained
pre-version-bump record does not verify the current versioned binary.

The release target is Windows amd64 only. Windows arm64 is verification-only;
Linux and arm64 release packages are not produced.

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
| Clean dogfood at the final source SHA | **PENDING** |
| v0.16.0 Windows amd64 binary, ZIP, archive smoke, and SHA-256 | **PENDING** |
| Native browser/UI acceptance | **PENDING** |
| Tag, publication, and remote asset/hash verification | **PENDING** |

When completed, the release output must contain only:

```text
dev-control-room_0.16.0_windows_amd64.zip
SHA256SUMS
```
