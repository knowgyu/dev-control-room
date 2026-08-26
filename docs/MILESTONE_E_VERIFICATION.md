# Milestone E verification

Status: partial; bounded artifacts and registered native Go runners are verified, mutation and restore lifecycle remain deferred
Updated: 2026-08-26

The v1 technique set now has a common report contract for static/security,
mutation, property, fuzz, and targeted E2E. Reports retain mutant status,
property/fuzz seeds, corpus or reproduction information, and bounded source
references. Artifact export uses a staging directory and per-file SHA-256
verification before the archive directory is renamed. Archive state is kept in
the manifest; local deletion requires the literal `DELETE` confirmation and
leaves the manifest marked deleted.

```text
scope: WSL; isolated generic Git fixture
commands:
  go test ./internal/assurance ./internal/app ./internal/store -run 'Test(AllV1Technique|QualityRun|AssuranceAuthoring)' -count=1  PASS
```

## v0.8.0 status update

The fresh journey now executes allowlisted native Go static/security, property,
fuzz, and targeted-E2E runners with `GOPROXY=off`, `GOSUMDB=off`,
`GOTOOLCHAIN=local`, `GOWORK=off`, private cache, and `-mod=readonly`.
`go-mutesting.exe` is absent in the verifier, so mutation is blocked/unavailable
instead of reported successful. Its adapter remains
[#5](https://github.com/knowgyu/dev-control-room/issues/5).

Archive restore, retention/quota/pin enforcement, and interrupted lifecycle
recovery remain [#11](https://github.com/knowgyu/dev-control-room/issues/11).
