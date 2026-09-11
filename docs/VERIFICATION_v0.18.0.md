# v0.18.0 verification record

Status: **RELEASE CANDIDATE**
Updated: 2026-09-11

This record covers the repository-quality inspection and tool-install boundary
added after v0.17.0. The release target is Windows amd64 only.

## Candidate identity

| Field | Result |
| --- | --- |
| Implementation source SHA | `a27b1cf0f39ad12847ff0df7c065d7c0de819405` |
| Version | `0.18.0` |
| Target | Windows 11 amd64 |
| Release assets | One ZIP plus `SHA256SUMS` |
| Linux/arm64 assets | Not produced |

## Automated verification

| Check | Status |
| --- | --- |
| `go test ./... -count=1` | **PASS after UI expectation refresh** |
| `go vet ./...` | **PASS** |
| focused `go test -race` for CAS, inspection, approval, and install paths | **PASS** |
| `node --check internal/app/ui/app.js` | **PASS** |
| `node --test internal/app/quality_inspection_ui.test.cjs` | **PASS — 9 tests** |
| `git diff --check` | **PASS** |
| Windows amd64 `go build ./cmd/dev-control-room` | **PASS** |

The focused tests cover stale plan/proposal CAS behavior, rejected-plan
regeneration, latest artifact recovery, project tool resolution, installation
scope binding, approval rejection/cancellation, and UI request routing.

## Manual and external boundaries

- A real package install was **not** executed during verification.
- Jenkins, production systems, GitHub mutations, and destructive cleanup were
  **not** contacted.
- Package/archive/hash smoke is performed by the release packaging step after
  the source commit is created.
