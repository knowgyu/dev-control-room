# v0.18.1 verification record

Status: **RELEASE CANDIDATE**
Updated: 2026-09-11

This patch verifies the repository-quality and component-aware installation
hardening added after v0.18.0. The release target is Windows amd64 only.

## Candidate identity

| Field | Result |
| --- | --- |
| Reviewed implementation source SHA | `9acf522fbb3d01e11f9aabcd71fd837f4b2fbc5c` |
| Version | `0.18.1` |
| Target | Windows 11 amd64 |
| Release assets | One ZIP plus `SHA256SUMS` |
| Linux/macOS/arm64 assets | Not produced |

## Automated verification

| Check | Status |
| --- | --- |
| `pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Fast` | **PASS** |
| `go vet ./...` | **PASS** |
| focused Action Broker race tests | **PASS** |
| quality inspection UI Node tests | **PASS — 11 tests** |
| Windows process-tree timeout regression | **PASS — 10/10 reruns** |
| Windows amd64 package/archive/version/checksum smoke | **PENDING** |

The tests cover actual `result + *exec.ExitError` behavior, unexpected pytest
exit codes, failed-test mapping, runnable-only improvement proposals, artifact
tampering, returned plan digests, target-switch response races, mixed
FastAPI/Vue component installation, repeated plans, audit recovery, legacy
plan rejection, and lock release.

## Manual and external boundaries

- No real package installation is performed by the release gate.
- Jenkins, production systems, Scheduler mutation, and destructive cleanup are
  not contacted.
- Publication and downloaded-asset verification are recorded after packaging.
