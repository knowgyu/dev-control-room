# v0.15.2 verification record

Status: PASS for the local source candidate and Windows amd64 package.
Publication: not performed.
Date: 2026-08-31.

## Candidate and scope

The v0.15.2 release-preparation tree starts from commit
`3dec04eeffb74bb82b5896a4317b131bff7e124b` (`fix: harden measurement import
boundaries`) and carries the v0.15.2 version bump, release records, and
updated cache/test expectations. It builds on the v0.15.1 reproducible
measurement foundation and adds the actual measurement manifest
import/list/get/dashboard workflow, storage masking identity preservation,
token-aware absolute-path rejection, and legacy Assurance empty-state
rerendering.

The release target is Windows amd64 only. Windows arm64 is a verification-only
cross-build; Linux and arm64 release packages are not produced.

## Dogfood and contract verification

Before the release-prep edits, the clean `3dec04e` tree was measured with:

```powershell
pwsh -NoProfile -File .\scripts\measure-dogfood.ps1 -OutputDirectory artifacts\dogfood-v0.15.2
pwsh -NoProfile -File .\scripts\verify-measurement-contract.ps1 -ManifestPath artifacts\dogfood-v0.15.2\dogfood-measurement.json
```

The runner and contract validator passed. The manifest records:

| Field | Value |
| --- | --- |
| Run ID | `dogfood-e878541d582343d091bffea3872f382d` |
| Commit / head | `3dec04eeffb74bb82b5896a4317b131bff7e124b` |
| Dirty state | `clean` |
| Platform | `windows/x64` |
| Measurements | `12` |
| Overall status | `pass` |
| Required failures | `none` |
| Go total statement coverage | `58.4%` |
| Optional server probes | `unknown` / `unavailable`, 0 samples |

## Source verification

Run from the native Windows checkout:

```powershell
go test -count=1 ./internal/measurement ./internal/store ./internal/app
go test -count=1 -race ./internal/measurement ./internal/store
node --check internal/app/ui/app.js
go vet ./...
git diff --check
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full -ArtifactDirectory artifacts\verification-v0.15.2
```

The focused measurement/store/app tests, focused race tests, UI syntax check,
`go vet ./...`, and `git diff --check` passed before the final full gate. The
final Full gate passed formatting, `go test -count=1 ./...`, embedded UI
JavaScript syntax, `go test -count=1 -race ./...`, `go vet ./...`,
`go mod verify`, normal build, Windows amd64 and arm64 cross-builds, and
`git diff --check`. It recorded 10 passing steps in the verifier summary at
`artifacts/verification-v0.15.2/verification-summary.json`:

| Step | Duration (seconds) |
| --- | ---: |
| `gofmt-check` | 0.046 |
| `go-test` | 161.882 |
| `ui-syntax` | 0.048 |
| `go-test-race` | 207.139 |
| `go-vet` | 0.730 |
| `go-mod-verify` | 0.952 |
| `go-build` | 1.872 |
| `windows-amd64-build` | 2.978 |
| `windows-arm64-build` | 1.603 |
| `git-diff-check` | 0.097 |

The verification-only Windows build hashes were:

```text
7DBF8FD26933579437D0323EDDEC8F32D8010E8463202194F1F9BAA2C2465D26  dev-control-room-windows-amd64.exe
7135923F35C16DF7918921F24AD70026CA99FDE56747FFFA4CC9B1FE4D4021EE  dev-control-room-windows-arm64.exe
```

## Package verification

The Windows amd64 package command was:

```powershell
pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.15.2 -OutputDirectory artifacts\release-v0.15.2
```

The expected release output is exactly:

```text
dev-control-room_0.15.2_windows_amd64.zip
SHA256SUMS
```

The package script performs a `windows/amd64` `CGO_ENABLED=0` trimpath build,
requires both v0.15.2 documents, and writes only the Windows amd64 ZIP and its
SHA-256 list. The archive smoke checks passed: the ZIP has 9 entries, the
executable returns version `0.15.2` with exit code 0, the recomputed ZIP hash
matches `SHA256SUMS`, and the archive has 0 Linux or arm64 entries. The entries
are `dev-control-room.exe`, `README.md`, `LICENSE`, `THIRD_PARTY_POLICY.md`,
`licenses/Pretendard-OFL-1.1.txt`, `docs/NATIVE_WINDOWS_SMOKE.md`,
`docs/RELEASE_NOTES_v0.15.2.md`, `docs/VERIFICATION_PLAYBOOK.md`, and
`docs/VERIFICATION_v0.15.2.md`.

The final ZIP hash is recorded below after package creation. The verification
document is copied into the archive by the package script, so the packaged
copy cannot contain the hash of the archive that contains it. The source
verification document therefore records the final hash after the ZIP is
created, while the packaged copy intentionally remains the pre-hash version.

Final package size and SHA-256:

```text
13,367,854 bytes
474f2f3bce3f17f8f5f27092afe2534fa36cb88173de38dfa61d56ca34be3994  dev-control-room_0.15.2_windows_amd64.zip
```

## Evidence boundaries

This record establishes local native Windows source and package verification;
it is not a provider-authoritative CI result, mutation result, adoption result,
release publication, or causal productivity claim. The measurement runner does
not invent baselines or deltas. A one-sample command duration has a p95 equal
to that sample and is not evidence of a stable tail; repeated comparable
latency samples are required for a meaningful p95 comparison.
