# v0.18.1 verification record

Status: **RELEASE READY**
Updated: 2026-09-11

This patch verifies the repository-quality and component-aware installation
hardening added after v0.18.0. The release target is Windows amd64 only.

## Candidate identity

| Field | Result |
| --- | --- |
| Reviewed implementation source SHA | `666b6fe4fec42a15044ac27f5e94b7e56009079a` |
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
| Windows amd64 package/archive/version/checksum smoke | **PASS** |

Packaged executable version: `0.18.1`

Package SHA-256:
`1efaf59b9abda5f0a1e65ef0670468c371cb3e36bedf984f3a1e42aa2864b8d1`

The tests cover actual `result + *exec.ExitError` behavior, unexpected pytest
exit codes, failed-test mapping, runnable-only improvement proposals, artifact
tampering, returned plan digests, target-switch response races, mixed
FastAPI/Vue component installation, repeated plans, audit recovery, legacy
plan rejection, and lock release.

## Manual and external boundaries

- No real package installation is performed by the release gate.
- Jenkins, production systems, Scheduler mutation, and destructive cleanup are
  not contacted.
- Publication and downloaded-asset verification are performed after the tag is
  pushed and the GitHub release is published.
