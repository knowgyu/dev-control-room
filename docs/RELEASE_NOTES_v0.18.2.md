# Dev Control Room v0.18.2

Updated: 2026-09-13

v0.18.2 is a backward-compatible patch release preparation on top of
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
- Native Windows, browser regression, package, and release acceptance remain
  pending until recorded in `docs/VERIFICATION_v0.18.2.md`.
