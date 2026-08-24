# Milestone 6 verification: persistent safeguards for implemented failure sources

Updated: 2026-08-25

## Implemented

- Collector, Checkset, and Action failures enter one normalized occurrence path. Fingerprints use stable source/type/status/exit/scope fields and never raw stdout, stderr, or error text.
- Three exact occurrences create one persistent `SafeguardRule` proposal in SQLite schema 13.
- Lifecycle is enforced in the domain model: `proposal -> shadow -> active -> retired`; rollback is `active -> shadow`; retired rules are terminal.
- Entering shadow mode requires an owner. Activation requires an exact shadow hit, positive human feedback, and zero false positives.
- Shadow and active rules evaluate only failures with the same category and exact Project/Repository/Worktree scope. Exact fingerprints record hits; a different fingerprint in that same category and scope records a miss.
- Metrics persist evaluations, hits, misses, positive feedback, false positives, and deterministic local evaluation cost units.
- Failure counts use an atomic SQLite increment. Rule lifecycle and metric writes use revision compare-and-swap retries, so the background service and separate CLI processes cannot silently overwrite each other.
- The Korean embedded UI shows scope, lifecycle, owner, exact-match explanation, metrics, feedback, activation, rollback, and retirement controls.
- State changes exist only under protected `/ui/safeguards/...` routes. Activation records the server-derived local human actor with its timestamp. CLI remains list-only and MCP advertises no safeguard mutation tool.
- Handoff preview is not treated as a verification result and cannot create failure learning. CI connector, hook execution, and launched Handoff verification sources do not exist yet and therefore are not falsely normalized.

## Automated checks

Focused coverage includes:

- invalid lifecycle transitions and activation prerequisites;
- migration 12 -> 13;
- persistence across application restart;
- concurrent writes from two `App` instances sharing one home, including occurrence and shadow-hit counts;
- exact hit versus same-scope miss accounting;
- feedback and metric constraints;
- Checkset and Action failure normalization;
- protected UI-only mutation routes and absent API mutation route;
- embedded Korean UI lifecycle controls.

Fresh WSL verification used WSL2 Linux `6.18.33.2` and Go `1.26.7` on
2026-08-25. The following all passed:

```text
node --check internal/app/ui/app.js
go test -count=1 ./...
CGO_ENABLED=1 go test -count=1 -race ./...
go vet ./...
go mod verify
CGO_ENABLED=0 go build ./cmd/dev-control-room
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/dev-control-room
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/dev-control-room
git diff --check
```

Cross-build SHA-256 values from that gate:

```text
windows/amd64 d62ab611bbc65673cd21925fc68c2bc408c7c87bf7d8aab32d9366da939f3551
windows/arm64 2ecba70896fca269d51944f68fe672a42d55698e929a70024e26a3b94f7e2fbc
linux/amd64   d1c4fe830cb97aefabe6ccc3feef6f24d236778c53a19d0eeddf5d4477312d1e
```

`gopls` was not installed in this WSL environment. Compiler diagnostics from
the full test/build matrix, `go vet`, and race detection were used instead.

## Deliberate boundary

Rules are deterministic exact-fingerprint evaluators, not generated repository commands or generic shell policy. CI/hook/launched-Handoff failure normalization must be added with those real producers; no generic external-failure ingestion endpoint is exposed in advance. Optional AI clustering remains unimplemented. Native Windows UI acceptance for the lifecycle controls remains separate from WSL and cross-build verification.
