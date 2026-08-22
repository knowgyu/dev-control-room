# Third-party dependency policy

Dev Control Room is intended for personal use in a company environment, so the
dependency surface must remain small and auditable.

## Allowed by default

- MIT
- ISC
- BSD-2-Clause
- BSD-3-Clause
- Apache-2.0 after explicit review
- Public-domain components such as SQLite

## Not allowed by default

- GPL, AGPL, LGPL, SSPL, BUSL, Elastic License, or other copyleft/source-
  available dependencies
- SDKs that send telemetry by default
- hosted services required for local operation
- dependencies that bundle credentials or upload logs

Every added dependency must record its purpose, exact version, license, network
behavior, and replacement/removal path in this file before merge.

The pre-Milestone-0 prototype used only the Go standard library and browser APIs.

## Milestone 0 reviewed dependency

### `modernc.org/sqlite` `v1.38.2`

- Purpose: CGo-free SQLite driver used by the forward-only migration harness and
  the local Project, Repository, Observation, Finding, Event, ScanRun, and
  FailureFingerprint repositories.
- License: BSD-3-Clause for the Go driver; SQLite itself is public domain.
- Network behavior: none at runtime. The driver is an in-process database
  implementation and does not contact a hosted service or send telemetry.
- Binary/build behavior: pure Go, so the application build does not require a
  C compiler or a separately installed SQLite DLL. The reviewed module's Go
  1.23-compatible release supports Windows 386, amd64, and arm64.
- Version choice: `v1.38.2` is the newest reviewed release compatible with this
  repository's `go 1.23` module line; newer releases require a newer Go toolchain.
- Removal path: keep the `internal/store` API behind `database/sql`; replacing
  the blank driver import and the single module requirement with another
  reviewed `database/sql` driver removes the dependency without changing the
  domain contracts or migration callers.

The module's transitive dependencies are resolved by Go modules and must remain
in the generated `go.sum`. The complete selected module graph for the pinned
driver was inventoried on 2026-08-22:

| Module | Exact version | License | Role / network / removal |
| --- | --- | --- | --- |
| `github.com/dustin/go-humanize` | `v1.0.1` | MIT | SQLite formatting helper; no runtime network; removed with the driver. |
| `github.com/google/go-cmp` | `v0.6.0` | BSD-3-Clause | Test/build support; no runtime network; removed with the driver. |
| `github.com/google/pprof` | `v0.0.0-20250317173921-a4b03ec1a45e` | Apache-2.0 | Driver/tooling support; no runtime network; removed with the driver. |
| `github.com/google/uuid` | `v1.6.0` | BSD-3-Clause | SQLite support helper; no runtime network; removed with the driver. |
| `github.com/mattn/go-isatty` | `v0.0.20` | MIT | Terminal capability helper; no runtime network; removed with the driver. |
| `github.com/ncruces/go-strftime` | `v0.1.9` | MIT | SQLite date formatting helper; no runtime network; removed with the driver. |
| `github.com/remyoudompheng/bigfft` | `v0.0.0-20230129092748-24d4a6f8daec` | BSD-3-Clause | Numeric support; no runtime network; removed with the driver. |
| `golang.org/x/exp` | `v0.0.0-20250620022241-b7579e27df2b` | BSD-3-Clause | Go support library; no runtime network; removed with the driver. |
| `golang.org/x/mod` | `v0.25.0` | BSD-3-Clause | Build support; no runtime network; removed with the driver. |
| `golang.org/x/sync` | `v0.15.0` | BSD-3-Clause | Build support; no runtime network; removed with the driver. |
| `golang.org/x/sys` | `v0.34.0` | BSD-3-Clause | OS support; no runtime network; removed with the driver. |
| `golang.org/x/tools` | `v0.34.0` | BSD-3-Clause | Build support; no runtime network; removed with the driver. |
| `modernc.org/cc/v4` | `v4.26.2` | BSD-3-Clause | C-to-Go build support; no runtime network; removed with the driver. |
| `modernc.org/ccgo/v4` | `v4.28.0` | BSD-3-Clause | C-to-Go build support; no runtime network; removed with the driver. |
| `modernc.org/fileutil` | `v1.3.8` | BSD-3-Clause | Driver file helper; no runtime network; removed with the driver. |
| `modernc.org/gc/v2` | `v2.6.5` | BSD-3-Clause | C-to-Go build support; no runtime network; removed with the driver. |
| `modernc.org/goabi0` | `v0.2.0` | BSD-3-Clause | C-to-Go ABI support; no runtime network; removed with the driver. |
| `modernc.org/libc` | `v1.66.3` | BSD-3-Clause | C-to-Go runtime support; no runtime network; removed with the driver. |
| `modernc.org/mathutil` | `v1.7.1` | BSD-3-Clause | Numeric support; no runtime network; removed with the driver. |
| `modernc.org/memory` | `v1.11.0` | BSD-3-Clause | Allocator support; no runtime network; removed with the driver. |
| `modernc.org/opt` | `v0.1.4` | BSD-3-Clause | C-to-Go build support; no runtime network; removed with the driver. |
| `modernc.org/sortutil` | `v1.2.1` | BSD-3-Clause | C-to-Go build support; no runtime network; removed with the driver. |
| `modernc.org/strutil` | `v1.2.1` | BSD-3-Clause | C-to-Go build support; no runtime network; removed with the driver. |
| `modernc.org/token` | `v1.1.0` | BSD-3-Clause | C-to-Go build support; no runtime network; removed with the driver. |

The Apache-2.0 entry above is explicitly reviewed and is permitted by this
policy. License names were checked against the pinned upstream license files;
the exact module graph is reproducible with `go list -m all` and integrity
checks with `go mod verify`. No copyleft, telemetry, credential-uploading, or
hosted-service dependency is present.

## Milestone 1 license and indirect-dependency audit

Audited 2026-08-22 before the Milestone 1 implementation commit. Milestone 1
adds no module to `go.mod` or `go.sum`; the complete indirect graph above is
unchanged and remains limited to the reviewed `modernc.org/sqlite` support
modules. The new collector, CLI, HTTP adapter, UI, and SQLite repositories use
the Go standard library and existing dependencies only. `go list -m all` and
`go mod verify` are part of the Milestone 1 verification record, and any future
module addition must append its exact version, license, runtime network
behavior, and removal path here before code is merged.
