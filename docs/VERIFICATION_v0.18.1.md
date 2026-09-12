# v0.18.1 verification record

Status: **RELEASE READY**
Updated: 2026-09-13

This patch verifies the repository-quality and component-aware installation
hardening added after v0.18.0. The release target is Windows amd64 only.

## Candidate identity

| Field | Result |
| --- | --- |
| Reviewed implementation source | `v0.18.1` tag target |
| Version | `0.18.1` |
| Target | Windows 11 amd64 |
| Release assets | One ZIP plus `SHA256SUMS` |
| Linux/macOS/arm64 assets | Not produced |

## Automated verification

| Check | Status |
| --- | --- |
| `pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full` | **PASS** |
| `pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Fast` | **PASS** |
| `go vet ./...` | **PASS** |
| focused Action Broker race tests | **PASS** |
| quality inspection UI Node tests | **PASS — 11 tests** |
| Windows process-tree timeout regression | **PASS — 10/10 reruns** |
| Windows storage process serialization | **PASS — normal 20/20, race 10/10** |
| Windows amd64 package/archive/version/checksum smoke | **PASS** |

Packaged executable version: `0.18.1`

The tests cover actual `result + *exec.ExitError` behavior, unexpected pytest
exit codes, failed-test mapping, runnable-only improvement proposals, artifact
tampering, returned plan digests, target-switch response races, mixed
FastAPI/Vue component installation, repeated plans, audit recovery, legacy
plan rejection, lock release, Windows path identity normalization, and safe
pre-execution storage-lock retries.

## Manual and external boundaries

- No real package installation is performed by the release gate.
- Jenkins, production systems, Scheduler mutation, and destructive cleanup are
  not contacted.
- Publication and downloaded-asset verification are performed after the tag is
  pushed and the GitHub release is published.
