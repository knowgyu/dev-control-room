package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateCreatesContractSchemaAndIsIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", "file:migration-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, CurrentSchemaVersion)
	}
	for _, table := range []string{"projects", "repositories", "observations", "findings", "checksets", "check_runs", "actions", "action_plans", "approvals", "action_locks", "action_idempotency", "action_events", "agent_profiles", "events", "scan_runs", "failure_fingerprints", "environment_health", "scheduler_state", "proposals"} {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("table %q was not created", table)
		}
	}
}

func TestMigrationEightAppliesForwardFromVersionSeven(t *testing.T) {
	db := openUnmigratedTestDatabase(t, "check-runs-v7-forward")
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations[:7] {
		if _, err := db.Exec(migration.SQL); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations(version, name, checksum) VALUES (?, ?, ?)`, migration.Version, migration.Name, migrationChecksum(migration.SQL)); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='check_runs'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("check_runs migration = %d, %v", count, err)
	}
}

func TestMigrationNineAppliesActionBrokerContractsFromVersionEight(t *testing.T) {
	db := openUnmigratedTestDatabase(t, "action-broker-v8-forward")
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations[:8] {
		if _, err := db.Exec(migration.SQL); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations(version, name, checksum) VALUES (?, ?, ?)`, migration.Version, migration.Name, migrationChecksum(migration.SQL)); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"action_locks", "action_idempotency"} {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("action broker table %s = %d, %v", table, count, err)
		}
	}
}

func TestMigrationTenAppliesActionAuditFromVersionNine(t *testing.T) {
	db := openUnmigratedTestDatabase(t, "action-audit-v9-forward")
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations[:9] {
		if _, err := db.Exec(migration.SQL); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations(version, name, checksum) VALUES (?, ?, ?)`, migration.Version, migration.Name, migrationChecksum(migration.SQL)); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='action_events'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("action event migration = %d, %v", count, err)
	}
}

func TestMigrationElevenAddsExecutionTrustFromVersionTen(t *testing.T) {
	db := openUnmigratedTestDatabase(t, "action-execution-v10-forward")
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations[:10] {
		if _, err := db.Exec(migration.SQL); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations(version, name, checksum) VALUES (?, ?, ?)`, migration.Version, migration.Name, migrationChecksum(migration.SQL)); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	for _, target := range []struct{ table, column string }{{"worktree_execution_trusts", "object_json"}, {"action_plans", "execution_context_digest"}} {
		rows, err := db.Query(`PRAGMA table_info(` + target.table + `)`)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for rows.Next() {
			var cid int
			var name, typ string
			var notNull, pk int
			var defaultValue any
			if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
				t.Fatal(err)
			}
			found = found || name == target.column
		}
		if err := rows.Close(); err != nil || !found {
			t.Fatalf("migration 11 %s.%s = found:%t err:%v", target.table, target.column, found, err)
		}
	}
}

func TestCheckRunsEnforceReferencesAndCascadeWithCheckset(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "check-runs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO check_runs(id, checkset_id, project_id, repository_id, worktree_id, started_at, completed_at, status, object_json) VALUES ('bad','missing','missing','repo','primary','now','now','passed','{}')`); err == nil {
		t.Fatal("missing check run references accepted")
	}
	if _, err := db.Exec(`INSERT INTO projects(id, api_version, kind, name, spec_json) VALUES ('p','v','Project','p','{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO repositories(project_id,id,api_version,kind,name,spec_json) VALUES ('p','r','v','Repository','r','{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO worktrees(project_id,repository_id,id,association_fingerprint,canonical_path,trust,is_primary,last_observed,object_json) VALUES ('p','r','primary',NULL,'/fixture','verified_read_only',1,'now','{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO checksets(id,project_id,repository_id,object_json) VALUES ('c','p','r','{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO check_runs(id,checkset_id,project_id,repository_id,worktree_id,started_at,completed_at,status,object_json) VALUES ('run','c','p','r','primary','now','now','passed','{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM checksets WHERE id = 'c'`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM check_runs WHERE id = 'run'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("check run cascade = %d, %v", count, err)
	}
}

func TestOpenCreatesAndMigratesFileDatabase(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "state.db")
	db, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, CurrentSchemaVersion)
	}
	var busyTimeout int
	if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatal(err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("busy_timeout = %d, want 5000", busyTimeout)
	}
	var journalMode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
}

func TestMigrationFourAppliesForwardFromVersionTwo(t *testing.T) {
	db := openUnmigratedTestDatabase(t, "forward-migration")
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations[:2] {
		if _, err := db.Exec(migration.SQL); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations(version, name, checksum) VALUES (?, ?, ?)`, migration.Version, migration.Name, migrationChecksum(migration.SQL)); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil || version != CurrentSchemaVersion {
		t.Fatalf("forward migration did not reach version %d: %d, %v", CurrentSchemaVersion, version, err)
	}
	for _, table := range []string{"environment_health", "scheduler_state"} {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("forward migration did not create %s: %d, %v", table, count, err)
		}
	}
	for _, table := range []string{"worktrees", "worktree_observations", "proposals"} {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("forward migration did not create %s: %d, %v", table, count, err)
		}
	}
}

func TestRepositoryForeignKeysAreScopedByProject(t *testing.T) {
	db := openTestDatabase(t, "fk-scope")
	var foreignKeys int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
	for _, projectID := range []string{"project-a", "project-b"} {
		if _, err := db.Exec(`INSERT INTO projects(id, api_version, kind, name, spec_json) VALUES (?, ?, ?, ?, ?)`, projectID, "devroom/v1alpha1", "Project", projectID, "{}"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO repositories(project_id, id, api_version, kind, name, spec_json) VALUES (?, ?, ?, ?, ?, ?)`, projectID, "backend", "devroom/v1alpha1", "Repository", "Backend", "{}"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO repositories(project_id, id, api_version, kind, name, spec_json) VALUES (?, ?, ?, ?, ?, ?)`, "missing-project", "backend", "devroom/v1alpha1", "Repository", "Backend", "{}"); err == nil {
		t.Fatal("expected repository with missing project to fail foreign key validation")
	}
	if _, err := db.Exec(`INSERT INTO observations(id, project_id, repository_id, api_version, kind, fingerprint, collected_at, object_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, "observation-1", "project-a", "missing-repository", "devroom/v1alpha1", "Observation", "fingerprint", "now", "{}"); err == nil {
		t.Fatal("expected observation with missing project-scoped repository to fail foreign key validation")
	}
	if _, err := db.Exec(`DELETE FROM projects WHERE id = ?`, "project-a"); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := db.QueryRow(`SELECT count(*) FROM repositories WHERE project_id = ?`, "project-a").Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("project cascade left %d repositories", remaining)
	}
}

func TestMigrationHistoryRejectsFutureAndMismatchedSchemas(t *testing.T) {
	future := openUnmigratedTestDatabase(t, "future-schema")
	if _, err := future.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := future.Exec(`INSERT INTO schema_migrations(version, name, checksum) VALUES (?, ?, ?)`, CurrentSchemaVersion+1, "future", strings.Repeat("0", 64)); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(context.Background(), future); err == nil || !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("expected future schema rejection, got %v", err)
	}

	mismatched := openUnmigratedTestDatabase(t, "mismatched-schema")
	if _, err := mismatched.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := mismatched.Exec(`INSERT INTO schema_migrations(version, name, checksum) VALUES (?, ?, ?)`, CurrentSchemaVersion, "renamed-migration", strings.Repeat("0", 64)); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(context.Background(), mismatched); err == nil || !strings.Contains(err.Error(), "history mismatch") {
		t.Fatalf("expected migration history rejection, got %v", err)
	}
}

func openTestDatabase(t *testing.T, name string) *sql.DB {
	t.Helper()
	db := openUnmigratedTestDatabase(t, name)
	if err := Migrate(context.Background(), db); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func openUnmigratedTestDatabase(t *testing.T, name string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+name+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db
}

func TestMigrationFourPreservesRepositoryIdentityAndScopesWorktrees(t *testing.T) {
	db := openUnmigratedTestDatabase(t, "worktree-v3-forward")
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations[:3] {
		if _, err := db.Exec(migration.SQL); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations(version,name,checksum) VALUES(?,?,?)`, migration.Version, migration.Name, migrationChecksum(migration.SQL)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO projects(id,api_version,kind,name,spec_json) VALUES('p','v','Project','p','{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO repositories(project_id,id,api_version,kind,name,spec_json) VALUES('p','r','v','Repository','r','{}')`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO worktrees(project_id,repository_id,id,association_fingerprint,canonical_path,trust,is_primary,last_observed,object_json) VALUES('p','r','primary',NULL,'/fixture','verified_read_only',1,'now','{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO worktree_observations(id,project_id,repository_id,worktree_id,collected_at,object_json) VALUES('o','p','r','primary','now','{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO worktrees(project_id,repository_id,id,association_fingerprint,canonical_path,trust,is_primary,last_observed,object_json) VALUES('p','missing','primary',NULL,'/fixture','verified_read_only',1,'now','{}')`); err == nil {
		t.Fatal("worktree foreign key was not scoped to repository")
	}
	if _, err := db.Exec(`DELETE FROM repositories WHERE project_id='p' AND id='r'`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM worktree_observations`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("worktree cascade failed: %d %v", count, err)
	}
}

func TestMigrationSixBackfillsPathFingerprintAndEnforcesSinglePrimary(t *testing.T) {
	db := openUnmigratedTestDatabase(t, "worktree-v5-forward")
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, name TEXT NOT NULL, checksum TEXT NOT NULL, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	for _, migration := range migrations[:5] {
		if _, err := db.Exec(migration.SQL); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations(version,name,checksum) VALUES(?,?,?)`, migration.Version, migration.Name, migrationChecksum(migration.SQL)); err != nil {
			t.Fatal(err)
		}
	}
	for _, statement := range []string{
		`INSERT INTO projects(id,api_version,kind,name,spec_json) VALUES('p','v','Project','p','{}')`,
		`INSERT INTO repositories(project_id,id,api_version,kind,name,spec_json) VALUES('p','r','v','Repository','r','{}')`,
		`INSERT INTO worktrees(project_id,repository_id,id,association_fingerprint,canonical_path,trust,is_primary,last_observed,object_json) VALUES('p','r','primary',NULL,'/masked','verified_read_only',1,'now','{"metadata":{"id":"primary"},"spec":{"primary":true,"pathFingerprint":"sha256:legacy-path"}}')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var fingerprint string
	if err := db.QueryRow(`SELECT path_fingerprint FROM worktrees WHERE project_id='p' AND repository_id='r' AND id='primary'`).Scan(&fingerprint); err != nil || fingerprint != "sha256:legacy-path" {
		t.Fatalf("v6 did not backfill path fingerprint: %q %v", fingerprint, err)
	}
	if _, err := db.Exec(`INSERT INTO worktrees(project_id,repository_id,id,association_fingerprint,canonical_path,path_fingerprint,trust,is_primary,last_observed,object_json) VALUES('p','r','linked','sha256:linked','/masked2','sha256:linked-path','verified_read_only',1,'now','{}')`); err == nil {
		t.Fatal("conflicting primary was accepted")
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM worktrees WHERE project_id='p' AND repository_id='r'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("primary conflict deleted or added rows: %d %v", count, err)
	}
}
