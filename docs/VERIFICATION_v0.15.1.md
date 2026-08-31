# v0.15.1 verification record

Status: PASS for the local source candidate and Windows amd64 package.
Publication: not performed.
Date: 2026-08-31.

## Candidate and scope

The release-preparation tree starts from commit
`f193287a5749757b0e0e0cc7d37c14bdf153c3d5` (`feat: add reproducible dogfood
measurements`) and carries the v0.15.1 version bump, release records, and
package metadata. The underlying slice adds the versioned measurement
foundation, deterministic bounded summaries, the native PowerShell dogfood
runner, and the static manifest contract check. Existing approval, Action
Broker, artifact, secret, provider, and MCP execution boundaries are unchanged.

The release target is Windows amd64 only. Windows arm64 is a verification-only
cross-build; Linux and arm64 release packages are not produced.

## Source verification

Run from the native Windows checkout:

```powershell
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full -ArtifactDirectory artifacts\verification-v0.15.1-final
pwsh -NoProfile -File .\scripts\verify-measurement-contract.ps1 -ManifestPath artifacts\dogfood-committed\dogfood-measurement.json
```

The Full gate was run from that release-preparation tree and passed formatting,
`go test -count=1 ./...`, embedded UI JavaScript syntax, `go test -count=1
-race ./...`, `go vet ./...`, `go mod verify`, normal build, Windows amd64 and
arm64 cross-builds, and
`git diff --check`. It recorded 10 passing steps in
`artifacts/verification-v0.15.1-final/verification-summary.json`:

| Step | Duration (seconds) |
| --- | ---: |
| `gofmt-check` | 0.061 |
| `go-test` | 208.933 |
| `ui-syntax` | 0.054 |
| `go-test-race` | 251.493 |
| `go-vet` | 0.766 |
| `go-mod-verify` | 0.963 |
| `go-build` | 1.892 |
| `windows-amd64-build` | 3.050 |
| `windows-arm64-build` | 1.722 |
| `git-diff-check` | 0.089 |

The native tool versions recorded by the verifier were Git
`2.53.0.windows.1`, Go `1.26.7 windows/amd64`, Node `24.15.0`, PowerShell
`7.6.4`, and GCC `16.2.0`. The verification-only Windows build hashes were:

```text
E46E1BF2B9F61A36059E712862F2F5B324A464BF9784A4B0289C7FE6D57B590B  dev-control-room-windows-amd64.exe
1E89085FC14E59B29070E19888C28F66D012437C39AF2AD322DE801BE1B55BB5  dev-control-room-windows-arm64.exe
```

The measurement contract validator passed the clean dogfood manifest. That run
recorded 12 measurements, required status `pass`, total statement coverage
`57.9%`, and `unknown`/`unavailable` provenance with zero samples for both
optional server metrics because no loopback probe was selected.

The dogfood command used to produce that evidence was:

```powershell
pwsh -NoProfile -File .\scripts\measure-dogfood.ps1 -OutputDirectory artifacts\dogfood-committed
```

It captured commit/head
`f193287a5749757b0e0e0cc7d37c14bdf153c3d5` with `dirtyState: clean`.

## Package verification

The Windows amd64 package command was:

```powershell
pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.15.1 -OutputDirectory artifacts\release-v0.15.1
```

The expected release output is exactly:

```text
dev-control-room_0.15.1_windows_amd64.zip
SHA256SUMS
```

The package script performs a `windows/amd64` `CGO_ENABLED=0` trimpath build,
requires both v0.15.1 documents, and writes only the Windows amd64 ZIP and its
SHA-256 list. The generated archive and checksum are ignored artifacts and are
not committed. No Linux or arm64 package is created.

The checksum was recomputed from the ZIP and matched `SHA256SUMS`. The archive
contains 9 entries, including the v0.15.1 release and verification documents,
and contains no Linux or arm64 entry. After extraction, the Windows amd64
executable returned version `0.15.1` with exit code 0. The final checksum is
recorded in this source document after the ZIP is generated; the packaged copy
of this document intentionally does not contain a self-referential hash.

The final ignored ZIP is `13,303,612` bytes and its SHA-256 is:

```text
eca40fdd568d3e5272d8eac2d6e32653293756f7d942e9c0f485eaa548901586  dev-control-room_0.15.1_windows_amd64.zip
```

## Evidence boundaries

This record establishes local native Windows source and package verification;
it is not a provider-authoritative CI result, mutation result, adoption result,
release publication, or causal productivity claim. The measurement runner does
not invent baselines or deltas. A one-sample command duration has a p95 equal
to that sample and is not evidence of a stable tail; repeated comparable
latency samples are required for a meaningful p95 comparison.
