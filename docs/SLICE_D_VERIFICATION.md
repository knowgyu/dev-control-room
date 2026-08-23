# Slice D verification: typed Checkset runner

Status: WSL validation passed on 2026-08-23.

The runner keeps command execution separate from Slice C discovery: a proposal
must first be reviewed as `applied`; an explicitly supplied typed Checkset is
created as `draft`; only a separately applied Checkset can run. It binds every
run to the persisted Worktree ID and HEAD, revalidates current Git identity,
uses argv (never a shell), an allowlisted child environment, per-step timeout,
bounded stdout/stderr, and masking before persistence or HTTP/CLI output.

CLI exposes `check list|show|create|apply|run|runs`; loopback HTTP exposes the
same application-service operations. The embedded UI is intentionally deferred.

Regression coverage in `internal/app/slice_d_test.go` covers proposal/checkset
lifecycle, draft-run rejection, Worktree-bound successful execution and changed
HEAD rejection, masking, and loopback mutation-token enforcement. `gofmt`,
`go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...`, and
`go mod verify` passed. Windows amd64/arm64 cross-builds passed. Native Windows
11 / PowerShell 7.6 runtime smoke remains unverified; cross-compilation does
not prove process cancellation, path handling, or child-environment behavior.
