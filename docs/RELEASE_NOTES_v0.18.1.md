# Dev Control Room v0.18.1

Updated: 2026-09-13

## Repository-quality correctness

- Treats reviewed Ruff, pytest, ESLint, and Vitest exit code 1 as a structured
  result while keeping unexpected exit codes as tool failures.
- Maps failed Python and Node test runs to `tests_failed` and prevents
  deterministic self-improvement from adding unavailable checks.
- Verifies inspection result artifact size and SHA-256 before using it, and
  returns reviewed inspection plans with a current valid digest.
- Restores the selected repository's latest inspection run after a target
  switch and prevents stale browser responses from overwriting the new target.

## Component-aware installation and Action safety

- Detects the matching Python/FastAPI or Node/Vue component in a mixed
  repository, resolves its project environment, and executes the approved
  installer from that verified component directory.
- Binds component identity, component root, affected files, writable paths,
  executable, and working directory into the immutable Action Plan.
- Reuses repeated install plans without immutable-plan conflicts and repairs a
  missing `planned` audit event on a safe retry without creating duplicates.
- Keeps v0.18.0 install plans readable but blocks approval or execution when
  they lack a verified component binding. A pre-upgrade lock is released when
  such a plan is rejected.

## Windows storage reliability

- Distinguishes a file-lock timeout that occurs before a storage operation
  starts from an execution-time SQLite busy result.
- Retries only the pre-execution lock case in the cross-process stress harness,
  so an INSERT whose execution status is ambiguous is never replayed.
- Keeps the production five-second lock budget unchanged.

## Release boundary

- Publishes one Windows 11 amd64 ZIP and `SHA256SUMS`.
- Does not publish Linux, macOS, or arm64 assets.
- Package installation remains opt-in and requires the existing human approval
  ceremony; no package manager runs while generating a preview.
