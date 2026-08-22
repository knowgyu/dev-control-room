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

const CurrentSchemaVersion = 5

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
		expected := migration.Name + "\x00" + migrationChecksum(migration.SQL)
		if history, ok := seen[migration.Version]; !ok || history != expected {
			return 0, fmt.Errorf("schema migration history mismatch at version %d", migration.Version)
		}
	}
	return current, nil
}

func migrationChecksum(sqlText string) string {
	checksum := sha256.Sum256([]byte(sqlText))
	return hex.EncodeToString(checksum[:])
}
