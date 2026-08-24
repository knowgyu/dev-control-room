// Package store owns the SQLite connection and forward-only schema migrations.
// Milestone 0 only establishes the storage contract; feature repositories are
// intentionally left for Milestone 1.
package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const CurrentSchemaVersion = 13

// acceptedHistoricalMigrationChecksums permits only the checksum emitted by a
// released build whose migration-11 SQL was later corrected before the next
// schema version shipped. Keeping this narrow exception preserves the
// tamper-detection guarantee for every other migration history entry while
// allowing those existing local databases to advance to version 12.
var acceptedHistoricalMigrationChecksums = map[int]map[string]struct{}{
	11: {
		"702c15eb78f7a8a8df908067791f986258907d6620ad0a8a9dd45f302e19c772": {},
	},
}

type Migration struct {
	Version int
	Name    string
	SQL     string
}

var migrations = []Migration{
	{
		Version: 1,
		Name:    "initial-contract-schema",
		SQL: `
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    api_version TEXT NOT NULL,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    spec_json TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS repositories (
    project_id TEXT NOT NULL,
    id TEXT NOT NULL CHECK (length(trim(id)) > 0),
    api_version TEXT NOT NULL,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    spec_json TEXT NOT NULL,
    PRIMARY KEY (project_id, id),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS observations (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    project_id TEXT NOT NULL,
    repository_id TEXT,
    api_version TEXT NOT NULL,
    kind TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    collected_at TEXT NOT NULL,
    object_json TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, repository_id) REFERENCES repositories(project_id, id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS findings (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    project_id TEXT NOT NULL,
    repository_id TEXT,
    api_version TEXT NOT NULL,
    kind TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    state TEXT NOT NULL,
    last_observed TEXT NOT NULL,
    object_json TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, repository_id) REFERENCES repositories(project_id, id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS checksets (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    project_id TEXT NOT NULL,
    repository_id TEXT,
    object_json TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, repository_id) REFERENCES repositories(project_id, id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS actions (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    action_type TEXT NOT NULL,
    risk TEXT NOT NULL,
    object_json TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS action_plans (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    action_type TEXT NOT NULL,
    risk TEXT NOT NULL,
    policy_decision TEXT NOT NULL,
    object_json TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS approvals (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    action_plan_id TEXT NOT NULL REFERENCES action_plans(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    object_json TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS agent_profiles (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    object_json TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    project_id TEXT,
    repository_id TEXT,
    event_type TEXT NOT NULL,
    occurred_at TEXT NOT NULL,
    object_json TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id, repository_id) REFERENCES repositories(project_id, id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS scan_runs (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    project_id TEXT,
    trigger TEXT NOT NULL,
    status TEXT NOT NULL,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    object_json TEXT NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS failure_fingerprints (
    fingerprint TEXT PRIMARY KEY CHECK (length(trim(fingerprint)) > 0),
    first_seen TEXT NOT NULL,
    last_seen TEXT NOT NULL,
    occurrence_count INTEGER NOT NULL,
    object_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_repositories_project ON repositories(project_id);
CREATE INDEX IF NOT EXISTS idx_findings_project_state ON findings(project_id, state);
CREATE INDEX IF NOT EXISTS idx_events_occurred_at ON events(occurred_at);
		`,
	},
	{
		Version: 2,
		Name:    "query-performance-indexes",
		SQL: `
CREATE INDEX IF NOT EXISTS idx_observations_repository_collected_at
    ON observations(project_id, repository_id, collected_at);
CREATE INDEX IF NOT EXISTS idx_findings_repository_state
    ON findings(project_id, repository_id, state, last_observed);
CREATE INDEX IF NOT EXISTS idx_scan_runs_project_started_at
    ON scan_runs(project_id, started_at);
CREATE INDEX IF NOT EXISTS idx_failure_fingerprints_last_seen
    ON failure_fingerprints(last_seen);
`,
	},
	{
		Version: 3,
		Name:    "environment-health-and-scheduler-state",
		SQL: `
CREATE TABLE IF NOT EXISTS environment_health (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    object_json TEXT NOT NULL,
    checked_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS scheduler_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    object_json TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
`,
	},
	{
		Version: 4,
		Name:    "worktree-identities-and-observations",
		SQL: `
CREATE TABLE IF NOT EXISTS worktrees (
    project_id TEXT NOT NULL,
    repository_id TEXT NOT NULL,
    id TEXT NOT NULL CHECK (length(trim(id)) > 0),
    association_fingerprint TEXT,
    canonical_path TEXT NOT NULL,
    trust TEXT NOT NULL CHECK (trust IN ('verified_read_only', 'unverified')),
    is_primary INTEGER NOT NULL CHECK (is_primary IN (0, 1)),
    last_observed TEXT NOT NULL,
    tombstoned_at TEXT,
    object_json TEXT NOT NULL,
    PRIMARY KEY (project_id, repository_id, id),
    UNIQUE (project_id, repository_id, association_fingerprint),
    FOREIGN KEY (project_id, repository_id) REFERENCES repositories(project_id, id) ON DELETE CASCADE,
    CHECK ((is_primary = 1 AND id = 'primary' AND association_fingerprint IS NULL) OR (is_primary = 0 AND association_fingerprint IS NOT NULL))
);
CREATE TABLE IF NOT EXISTS worktree_observations (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    project_id TEXT NOT NULL,
    repository_id TEXT NOT NULL,
    worktree_id TEXT NOT NULL,
    collected_at TEXT NOT NULL,
    object_json TEXT NOT NULL,
    FOREIGN KEY (project_id, repository_id, worktree_id) REFERENCES worktrees(project_id, repository_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_worktrees_repository_active ON worktrees(project_id, repository_id, tombstoned_at, last_observed);
CREATE INDEX IF NOT EXISTS idx_worktree_observations_scope ON worktree_observations(project_id, repository_id, worktree_id, collected_at);
`,
	},
	{
		Version: 5,
		Name:    "worktree-primary-identity-invariant",
		SQL: `
CREATE TRIGGER IF NOT EXISTS worktrees_primary_identity_insert
BEFORE INSERT ON worktrees
WHEN (NEW.is_primary = 1) != (NEW.id = 'primary')
BEGIN SELECT RAISE(ABORT, 'worktree primary identity is inconsistent'); END;
CREATE TRIGGER IF NOT EXISTS worktrees_primary_identity_update
BEFORE UPDATE OF id, is_primary ON worktrees
WHEN (NEW.is_primary = 1) != (NEW.id = 'primary')
BEGIN SELECT RAISE(ABORT, 'worktree primary identity is inconsistent'); END;
`,
	},
	{
		Version: 6,
		Name:    "worktree-path-fingerprint-and-primary-repair",
		SQL: `
ALTER TABLE worktrees ADD COLUMN path_fingerprint TEXT;
CREATE TABLE worktrees_v6_conflict_guard (ok INTEGER CHECK (ok = 1));
INSERT INTO worktrees_v6_conflict_guard(ok)
SELECT 0 WHERE EXISTS (
 SELECT 1 FROM worktrees bad
 WHERE bad.id = 'primary' AND bad.is_primary = 0
 AND EXISTS (SELECT 1 FROM worktrees current_primary
             WHERE current_primary.project_id = bad.project_id AND current_primary.repository_id = bad.repository_id
             AND current_primary.is_primary = 1)
);
UPDATE worktrees SET path_fingerprint = COALESCE(path_fingerprint, json_extract(object_json, '$.spec.pathFingerprint'),
 CASE WHEN canonical_path LIKE 'sha256:%' THEN canonical_path END);
UPDATE worktrees SET association_fingerprint = NULL, is_primary = 1,
 object_json = json_remove(json_set(object_json, '$.metadata.id', 'primary', '$.spec.primary', json('true'), '$.spec.pathFingerprint', COALESCE(path_fingerprint, canonical_path, '')), '$.spec.associationFingerprint')
WHERE id = 'primary' AND is_primary = 0;
UPDATE worktrees SET is_primary = 0 WHERE id <> 'primary' AND is_primary <> 0;
CREATE UNIQUE INDEX IF NOT EXISTS idx_worktrees_one_primary ON worktrees(project_id, repository_id) WHERE is_primary = 1;
CREATE INDEX IF NOT EXISTS idx_worktrees_path_fingerprint ON worktrees(project_id, repository_id, path_fingerprint);
DROP TABLE worktrees_v6_conflict_guard;
`,
	},
	{
		Version: 7,
		Name:    "discovery-proposals",
		SQL: `
CREATE TABLE IF NOT EXISTS proposals (
 id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
 project_id TEXT NOT NULL,
 repository_id TEXT NOT NULL,
 worktree_id TEXT NOT NULL,
 state TEXT NOT NULL CHECK (state IN ('pending', 'applied', 'rejected', 'stale')),
 source_path TEXT NOT NULL,
 source_digest TEXT NOT NULL,
 created_at TEXT NOT NULL,
 object_json TEXT NOT NULL,
 FOREIGN KEY (project_id, repository_id, worktree_id) REFERENCES worktrees(project_id, repository_id, id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_proposals_scope_state ON proposals(project_id, repository_id, worktree_id, state, created_at);
`,
	},
	{
		Version: 8,
		Name:    "checkset-runs",
		SQL: `
CREATE TABLE IF NOT EXISTS check_runs (
 id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
 checkset_id TEXT NOT NULL REFERENCES checksets(id) ON DELETE CASCADE,
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 repository_id TEXT NOT NULL,
 worktree_id TEXT NOT NULL,
 started_at TEXT NOT NULL,
 completed_at TEXT NOT NULL,
 status TEXT NOT NULL CHECK (status IN ('passed', 'failed', 'skipped', 'unavailable', 'cancelled', 'timed_out')),
 object_json TEXT NOT NULL,
 FOREIGN KEY (project_id, repository_id, worktree_id) REFERENCES worktrees(project_id, repository_id, id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_check_runs_checkset_started ON check_runs(checkset_id, started_at DESC);
`,
	},
	{
		Version: 9,
		Name:    "action-broker-contracts",
		SQL: `
ALTER TABLE action_plans ADD COLUMN repository_id TEXT;
ALTER TABLE action_plans ADD COLUMN worktree_id TEXT;
ALTER TABLE action_plans ADD COLUMN digest TEXT;
ALTER TABLE action_plans ADD COLUMN created_at TEXT;
ALTER TABLE approvals ADD COLUMN action_plan_digest TEXT;
ALTER TABLE approvals ADD COLUMN expires_at TEXT;
CREATE TABLE IF NOT EXISTS action_locks (
 scope TEXT PRIMARY KEY CHECK (length(trim(scope)) > 0),
 action_plan_id TEXT NOT NULL REFERENCES action_plans(id) ON DELETE CASCADE,
 action_plan_digest TEXT NOT NULL,
 holder TEXT NOT NULL,
 expires_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS action_idempotency (
 idempotency_key TEXT PRIMARY KEY CHECK (length(trim(idempotency_key)) > 0),
 action_plan_id TEXT NOT NULL REFERENCES action_plans(id) ON DELETE CASCADE,
 action_plan_digest TEXT NOT NULL,
 claimed_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_approvals_plan_status ON approvals(action_plan_id, status);
CREATE INDEX IF NOT EXISTS idx_action_locks_expires_at ON action_locks(expires_at);
`,
	},
	{
		Version: 10,
		Name:    "action-broker-audit-and-authority",
		SQL: `
ALTER TABLE action_plans ADD COLUMN requester_kind TEXT;
ALTER TABLE action_plans ADD COLUMN requester_id TEXT;
ALTER TABLE action_plans ADD COLUMN requested_at TEXT;
ALTER TABLE approvals ADD COLUMN decided_at TEXT;
CREATE TABLE IF NOT EXISTS action_events (
 id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
 action_plan_id TEXT NOT NULL REFERENCES action_plans(id) ON DELETE CASCADE,
 action_plan_digest TEXT NOT NULL,
 event_type TEXT NOT NULL,
 actor_kind TEXT NOT NULL,
 actor_id TEXT NOT NULL,
 occurred_at TEXT NOT NULL,
 object_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_action_events_plan_occurred ON action_events(action_plan_id, occurred_at);
`,
	},
	{
		Version: 11,
		Name:    "action-execution-contracts",
		SQL: `
ALTER TABLE action_plans ADD COLUMN execution_context_digest TEXT;
CREATE TABLE IF NOT EXISTS worktree_execution_trusts (
 project_id TEXT NOT NULL,
 repository_id TEXT NOT NULL,
 worktree_id TEXT NOT NULL,
 context_digest TEXT NOT NULL,
 trusted_at TEXT NOT NULL,
 object_json TEXT NOT NULL,
 PRIMARY KEY (project_id, repository_id, worktree_id),
 FOREIGN KEY (project_id, repository_id, worktree_id) REFERENCES worktrees(project_id, repository_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_action_plans_execution_context ON action_plans(project_id, repository_id, worktree_id, execution_context_digest);
		`,
	},
	{
		Version: 12,
		Name:    "action-run-results",
		SQL: `
CREATE TABLE IF NOT EXISTS action_runs (
 id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
 action_plan_id TEXT NOT NULL REFERENCES action_plans(id) ON DELETE CASCADE,
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 repository_id TEXT NOT NULL,
 worktree_id TEXT NOT NULL,
 action_plan_digest TEXT NOT NULL,
 started_at TEXT NOT NULL,
 completed_at TEXT NOT NULL,
 status TEXT NOT NULL CHECK (status IN ('precheck_failed', 'running', 'succeeded', 'failed', 'cancelled', 'timed_out', 'postcheck_failed', 'unavailable')),
 object_json TEXT NOT NULL,
 FOREIGN KEY (project_id, repository_id, worktree_id) REFERENCES worktrees(project_id, repository_id, id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_action_runs_plan_started ON action_runs(action_plan_id, started_at DESC, id DESC);
		`,
	},
	{
		Version: 13,
		Name:    "safeguard-rule-lifecycle",
		SQL: `
CREATE TABLE IF NOT EXISTS safeguard_rules (
 id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
 fingerprint TEXT NOT NULL UNIQUE,
 category TEXT NOT NULL,
 project_id TEXT NOT NULL,
 repository_id TEXT NOT NULL,
 worktree_id TEXT,
 state TEXT NOT NULL CHECK (state IN ('proposal', 'shadow', 'active', 'retired')),
 revision INTEGER NOT NULL CHECK (revision > 0),
 updated_at TEXT NOT NULL,
 object_json TEXT NOT NULL,
 FOREIGN KEY (project_id, repository_id) REFERENCES repositories(project_id, id) ON DELETE CASCADE,
 FOREIGN KEY (project_id, repository_id, worktree_id) REFERENCES worktrees(project_id, repository_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_safeguard_rules_state_updated ON safeguard_rules(state, updated_at DESC, id);
CREATE INDEX IF NOT EXISTS idx_safeguard_rules_scope_category ON safeguard_rules(project_id, repository_id, worktree_id, category, id);
`,
	},
}

func Open(ctx context.Context, path string) (*sql.DB, error) {
	if path == "" {
		return nil, errors.New("database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout = 5000`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure sqlite busy timeout: %w", err)
	}
	var journalMode string
	if err := db.QueryRowContext(ctx, `PRAGMA journal_mode = WAL`).Scan(&journalMode); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable sqlite WAL mode: %w", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite WAL mode is unavailable: %s", journalMode)
	}
	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("restrict sqlite database permissions: %w", err)
	}
	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("database is required")
	}
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    checksum TEXT NOT NULL,
    applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (version > 0),
    CHECK (length(trim(name)) > 0),
    CHECK (length(trim(checksum)) = 64)
)`); err != nil {
		return fmt.Errorf("create schema migration table: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	var foreignKeys int
	if err := db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		return fmt.Errorf("read sqlite foreign-key setting: %w", err)
	}
	if foreignKeys != 1 {
		return errors.New("sqlite foreign keys are not enabled")
	}

	current, err := validateMigrationHistory(ctx, db)
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		if migration.Version <= current {
			continue
		}
		if migration.Version != current+1 {
			return fmt.Errorf("migration sequence gap before version %d", migration.Version)
		}
		transaction, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", migration.Version, err)
		}
		if _, err := transaction.ExecContext(ctx, migration.SQL); err != nil {
			_ = transaction.Rollback()
			return fmt.Errorf("apply migration %d (%s): %w", migration.Version, migration.Name, err)
		}
		if _, err := transaction.ExecContext(ctx, `INSERT INTO schema_migrations(version, name, checksum) VALUES (?, ?, ?)`, migration.Version, migration.Name, migrationChecksum(migration.SQL)); err != nil {
			_ = transaction.Rollback()
			return fmt.Errorf("record migration %d: %w", migration.Version, err)
		}
		if err := transaction.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", migration.Version, err)
		}
		current = migration.Version
	}
	if current != CurrentSchemaVersion {
		return fmt.Errorf("schema migration set stops at version %d, want %d", current, CurrentSchemaVersion)
	}
	return nil
}

func validateMigrationHistory(ctx context.Context, db *sql.DB) (int, error) {
	rows, err := db.QueryContext(ctx, `SELECT version, name, checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		return 0, fmt.Errorf("read schema migration history: %w", err)
	}
	defer rows.Close()

	seen := make(map[int]string, len(migrations))
	current := 0
	for rows.Next() {
		var version int
		var name string
		var checksum string
		if err := rows.Scan(&version, &name, &checksum); err != nil {
			return 0, fmt.Errorf("scan schema migration history: %w", err)
		}
		if version > CurrentSchemaVersion {
			return 0, fmt.Errorf("database schema version %d is newer than supported version %d", version, CurrentSchemaVersion)
		}
		if version <= 0 || strings.TrimSpace(name) == "" || strings.TrimSpace(checksum) == "" {
			return 0, errors.New("schema migration history contains an invalid version or name")
		}
		seen[version] = name + "\x00" + checksum
		if version > current {
			current = version
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read schema migration history: %w", err)
	}
	for _, migration := range migrations {
		if migration.Version > current {
			break
		}
		expectedChecksum := migrationChecksum(migration.SQL)
		history, ok := seen[migration.Version]
		if !ok || !migrationHistoryMatches(migration.Version, migration.Name, expectedChecksum, history) {
			return 0, fmt.Errorf("schema migration history mismatch at version %d", migration.Version)
		}
	}
	return current, nil
}

func migrationHistoryMatches(version int, name, expectedChecksum, history string) bool {
	expected := name + "\x00" + expectedChecksum
	if history == expected {
		return true
	}
	legacyChecksums, ok := acceptedHistoricalMigrationChecksums[version]
	if !ok {
		return false
	}
	legacyPrefix := name + "\x00"
	if !strings.HasPrefix(history, legacyPrefix) {
		return false
	}
	_, ok = legacyChecksums[strings.TrimPrefix(history, legacyPrefix)]
	return ok
}

func migrationChecksum(sqlText string) string {
	checksum := sha256.Sum256([]byte(sqlText))
	return hex.EncodeToString(checksum[:])
}
