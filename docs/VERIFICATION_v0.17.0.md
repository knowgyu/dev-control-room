# v0.17.0 verification record

Status: **PREPARATION — NOT RELEASED**
Publication: **PENDING — not performed**
Binary/package: **PENDING — package execution intentionally not run**
Prepared: 2026-09-10

This record describes the planned M1 scope and the verification state of this
release-preparation edit. It contains no release or test-pass claim for the
final candidate. Main must replace the pending fields with fresh evidence from
the exact clean source candidate before publishing anything.

## Scope and non-claims

The intended scope is project-oriented, read-only setup inspection for a
registered selected checkout: automatic/manual display language filtering,
mixed Python/FastAPI, Vue, JavaScript/TypeScript, and Go component evidence,
bounded setup previews, and the prior UX and repository-discovery fixes.

M1 does **not** include Python/FastAPI or Vue analyzer execution, normalized
findings, automatic installation, AI setup or source upload, arbitrary shell
execution, or a persisted approved executable plan. Existing Go root-only
actual checks remain the prior supported execution path.

## Candidate identity

| Field | Status | Required follow-up |
| --- | --- | --- |
| Final source SHA | **PENDING** | Main selects and records the exact clean candidate SHA. |
| Branch/worktree | **PENDING** | Confirm the intended branch and clean/approved release worktree. |
| Latest verified release | `v0.16.0` | Use as the baseline; do not treat it as v0.17.0 evidence. |
| Release target | `Windows amd64 ZIP + SHA256SUMS` | Verify only after source and Full gate acceptance. |

The shared worktree was already dirty before this preparation, and concurrent
implementation/UX changes remain outside this documentation/version scope.

## Preparation-only static checks

These checks cover the owned version, MCP-test, script, and documentation edits
only. They do not establish M1 acceptance or release readiness.

| Check | Result |
| --- | --- |
| `gofmt -l internal/version/version.go internal/mcp/server_test.go` | **PASS** — no files reported. |
| `go test -count=1 ./internal/mcp -run '^TestServeExposesOnlyTypedToolsAndStableResults$'` | **PASS** — focused test completed successfully. |
| PowerShell AST parse of `scripts/package.ps1` | **PASS**. |
| `git diff --check` | **PASS** — no whitespace errors. |
| v0.17.0 version/document path contract | **PASS**. |
| New release-document trailing-whitespace check | **PASS**. |

## Verification gates

| Gate | Status | Evidence required from main |
| --- | --- | --- |
| Focused M1 tests | **PENDING** | Detector, bounded discovery, service/API, and relevant UI contract results. |
| Native Windows Fast verifier | **PENDING** | Fresh `verify.ps1 -Mode Fast` result for the selected source. |
| Native Windows Full verifier | **PENDING** | Fresh `verify.ps1 -Mode Full` result, including amd64/arm64 builds. |
| Browser/native acceptance | **PENDING** | Mixed fixture, automatic/manual filter, reload/reset semantics, prior UX, and no-install/no-execution observations. |
| Package/archive/hash smoke | **PENDING — not run** | Run `package.ps1` only after source and Full gate acceptance; inspect ZIP and SHA-256. |
| CI evidence | **PENDING** | Record the successor CI result for the final candidate. |
| Tag/release/remote assets | **PENDING — not run** | Separate authorization and evidence required; this task performed none. |

## Ownership boundary

The UI cachebuster and the `internal/app/web_checkset_test.go` version
assertion are intentionally not changed in this preparation; they remain
Banach-owned integration work. This record therefore does not claim complete
UI-version alignment until that work and its tests are integrated and checked.

## Required M1 observations

Main should record fresh evidence for:

- automatic detection of a mixed backend/frontend fixture with FastAPI and Vue
  evidence;
- manual language filtering that narrows recommendations while retaining all
  components and evidence;
- unchanged HEAD/evidence preserving the display preference, and changed
  evidence resetting it to automatic detection;
- maximum depth 3, 2,000 directory entries, 64 eligible files, and 128 KiB per
  file, with partial/warning state for incomplete inspection;
- recognized configuration versus setup-needed, unsupported, and ambiguous
  states without inferring installation, tests, coverage, or success;
- no automatic install, AI/provider launch, configuration write, registry
  access, source upload, or Python/Vue analyzer execution during detection;
- the existing Go root-only actual checks remaining available through their
  prior execution path.

## Evidence fields to complete

```text
source SHA: PENDING
environment: PENDING — OS, PowerShell, Go, Node, gcc; native Windows versus WSL
commands: PENDING — focused, Fast, Full, browser/native, CI, package/hash
results: PENDING — record each command/flow separately; do not collapse gaps into PASS
artifacts: PENDING — logs, screenshots, manifests, binary and ZIP hashes
gaps: PENDING — all unrun or human-only acceptance items
side effects: package/tag/push/release not run by this preparation
next action: main selects a clean candidate and fills this record with fresh evidence
```

No package execution, commit, push, tag, release publication, or external
integration mutation was performed for v0.17.0 preparation.
