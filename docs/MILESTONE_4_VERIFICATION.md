# Milestone 4 verification: generic cleanup safety base

Updated: 2026-08-24

## Implemented

- `CleanupCandidate` is a read-only object bound to one Project, Repository,
  and observed Worktree.
- The application service exposes the same candidate list to CLI, loopback
  HTTP, and the embedded control-room UI.
- Candidates remain `blocked` when merged-change evidence is unavailable or
  when the Worktree is primary, dirty, untracked, detached, locked, prunable,
  missing upstream, ahead of upstream, or incompletely observed.
- Candidate reasons, branch, HEAD, path, and observation time are returned for
  operator review; no cleanup mutation endpoint exists.

## Checks

The following passed in WSL with the temporary Linux Go toolchain:

```text
go test -count=1 ./...
git diff --check
```

Focused coverage includes cleanup candidate generation, primary and
untracked-state blocking, HTTP output, CLI JSON envelope, and embedded UI
presence.

## Deliberate boundary

GitHub/Jenkins collectors, merged-PR correlation, company release procedures,
and cleanup execution are not inferred without user-provided provider and
policy inputs. The generic queue passed its later native Windows fixture gate;
see `NATIVE_WINDOWS_SMOKE.md`.
