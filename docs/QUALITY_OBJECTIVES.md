# Quality objectives and QualityHome

This slice adds a small, user-owned quality-improvement aggregate without introducing a second workflow engine or a separate read-model database.

## Aggregate

`domain.QualityObjective` is stored as an `assurance_objects` row with kind `QualityObjective`, immutable object identity, and the existing revision/CAS update path. Its scope is the existing project/repository/worktree plus the exact captured `head`. The minimal durable links are finding, session, baseline, campaign, run, and proposal IDs.

The lifecycle is explicit and validated in the domain:

`draft` → `baseline_pending` → `ready` → `running` → `review` → `adopted`

`blocked`, `stale`, and `rejected` are terminal/exception paths with only the transitions declared by `CanTransition`. The create service starts new objectives in `draft`; an update command is intentionally outside this slice.

## QualityHome read model

`App.QualityHome` computes a deterministic queue from persisted objectives, quality runs, PR/CI baselines, assurance proposals, regular proposals, and open findings. It returns non-nil empty arrays when no data exists. Queue ordering is stable by attention priority, update time, and ID.

The query does not write a projection and does not refresh findings. Baseline freshness is evaluated through the existing in-memory PR/CI freshness check; persisted data remains the source of truth. This keeps the first home view read-only and avoids hidden demo data or side effects.

## HTTP contract

- `GET /api/quality/home` — computed home and queue.
- `GET /api/quality/objectives` — persisted objectives.
- `GET /api/quality/objectives/{objectiveID}` — one objective.
- `POST /api/quality/objectives` — protected mutation using the existing mutation token/listen guard and response envelope.

The existing `/api/assurance/...` routes remain unchanged. OpenAPI, MCP, UI, source snapshots, and impact-graph work are deliberately deferred.
