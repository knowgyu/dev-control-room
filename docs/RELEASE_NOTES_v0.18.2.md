# Dev Control Room v0.18.2

Released 2026-09-13. The source version is `0.18.2`, and the Windows amd64
package, CI, and remote asset verification are complete.

v0.18.2 is a backward-compatible patch release on top of
v0.18.1. It keeps the existing application, CLI, HTTP, and artifact contracts
and does not add a public API or persistence schema migration.

## P1 correctness and first-use fixes

- Resets the Assurance AI-candidate checkbox layout so its text remains
  readable beside the control.
- Separates artifact evidence state and reason from retention state. Coverage
  profiles from failed or timed-out runs are partial; truncated or unparseable
  profiles are invalid. Non-valid artifacts do not count as assurance evidence,
  score input, trace completion, or report evidence.
- Uses one selected Worktree state across Work, discovery, action, external,
  guidance, and Assurance controls.
- Preserves the selected Worktree across discovery requests and refreshes, and
  reports an explicit empty result for the requested target.

## P2 product-usability fixes

- Groups repositories and Worktrees discovered from a parent folder instead of
  presenting unrelated checkouts as one flat list.
- Makes the first Assurance action and the plan → approve → run sequence
  explicit when no plan or score exists yet.
- Reduces repeated scheduled no-op activity noise and keeps meaningful events
  prominent.
- Compacts Diagnostics provider rows and replaces excessive status-rail space
  with readable status content.
- Gives dynamic selects and checkbox controls stable form identity where they
  participate in a form.

## Release boundary

- The only release asset target is a Windows 11 amd64 ZIP plus `SHA256SUMS`.
- Linux, macOS, and arm64 packages are not release assets.
- No production, Jenkins, Scheduler, destructive cleanup, or package-manager
  action is performed by this patch or by package generation.
- Native Full, browser regression, package, CI, and release acceptance are
  complete and recorded in `docs/VERIFICATION_v0.18.2.md`; separate
  company/provider/second-device acceptance remains outside this evidence.

## Publication

- Release: [v0.18.2](https://github.com/knowgyu/dev-control-room/releases/tag/v0.18.2),
  published `2026-09-13T07:01:20Z`, non-draft and non-prerelease.
- Source/tag: commit `6e2a52ff1f5886adb95cdc50b39c63c4ddf80719`, annotated tag
  object `e599973b771e65c2e13654c4af7b1d7cb4b8e306`.
- CI: [run `34743438251`](https://github.com/knowgyu/dev-control-room/actions/runs/34743438251)
  passed; Linux completed `2026-09-13T06:43:55Z` and Windows completed
  `2026-09-13T07:00:16Z`. Node20/cache warnings were non-fatal.
- Published assets are exactly `dev-control-room_0.18.2_windows_amd64.zip`
  (`13,812,720` bytes) and `SHA256SUMS` (`109` bytes). The downloaded ZIP
  SHA-256 is
  `bf19ea76591ff1216c12aca319ebc960fb5e7adba22f776906b043fd2a69036d`, and
  the extracted native Windows executable version JSON reports `0.18.2`.
