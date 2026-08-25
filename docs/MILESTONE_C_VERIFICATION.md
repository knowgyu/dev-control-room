# Milestone C verification

Status: accepted for local fixture and typed-runner scope
Updated: 2026-08-26

The baseline reader inspects package scripts, GitHub Actions `run:` entries,
and an explicit local required-check declaration. It labels provider-enforced
rules unknown when authoritative provider evidence is unavailable. A baseline
is stale when its freshness window expires or the exact Worktree HEAD changes.
Quality Runs use an allowlisted typed `git diff --check` runner and persist a
bounded masked report; no discovered command is executed.

```text
scope: WSL; isolated generic Git fixture
commands:
  go test ./internal/app -run 'TestBaselineDiscovers|TestFakeProvider' -count=1  PASS
  go test ./internal/app -run 'TestQualityRun|TestBaselineDiscovers' -count=1  PASS
```

gaps: installed `gh`, Jenkins, and native Windows runner acceptance were not
available in this WSL check. Provider API/credential calls and production CI
were not contacted.

