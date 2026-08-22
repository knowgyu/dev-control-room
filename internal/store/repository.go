package store

// This file contains the typed SQLite repositories used by the application
// layer. The SQL schema is intentionally boring: the versioned domain object
// is the source of truth and indexed columns support the common list queries.
// Every object crosses the masking boundary before object_json is written.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/masking"
)

type Store struct {
	db     *sql.DB
	masker *masking.Masker
}

func New(db *sql.DB, masker *masking.Masker) (*Store, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	if masker == nil {
		masker = masking.New(nil, []string{"TOKEN", "PASSWORD", "SECRET", "API_KEY", "AUTHORIZATION"})
	}
	return &Store{db: db, masker: masker}, nil
}

func (s *Store) SetMasker(masker *masking.Masker) {
	if masker != nil {
		s.masker = masker
	}
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) SaveProject(ctx context.Context, project domain.Project) error {
	if err := project.Validate(); err != nil {
		return fmt.Errorf("validate project: %w", err)
	}
	object, err := s.maskedJSON(project)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin project transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
INSERT INTO projects(id, api_version, kind, name, spec_json) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET api_version=excluded.api_version, kind=excluded.kind,
name=excluded.name, spec_json=excluded.spec_json`,
		project.Metadata.ID, project.APIVersion, project.Kind, project.Metadata.Name, object); err != nil {
		return fmt.Errorf("save project: %w", err)
	}
	desiredRepositories := make(map[string]struct{}, len(project.Spec.Repositories))
	for _, repository := range project.Spec.Repositories {
		desiredRepositories[repository.Metadata.ID] = struct{}{}
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM repositories WHERE project_id = ?`, project.Metadata.ID)
	if err != nil {
		return fmt.Errorf("list existing project repositories: %w", err)
	}
	var removedRepositoryIDs []string
	for rows.Next() {
		var repositoryID string
		if err := rows.Scan(&repositoryID); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan existing project repository: %w", err)
		}
		if _, keep := desiredRepositories[repositoryID]; !keep {
			removedRepositoryIDs = append(removedRepositoryIDs, repositoryID)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("list existing project repositories: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close existing project repositories: %w", err)
	}
	for _, repositoryID := range removedRepositoryIDs {
		if _, err := tx.ExecContext(ctx, `DELETE FROM repositories WHERE project_id = ? AND id = ?`, project.Metadata.ID, repositoryID); err != nil {
			return fmt.Errorf("remove omitted project repository: %w", err)
		}
	}
	for _, repository := range project.Spec.Repositories {
		if err := insertRepository(ctx, tx, project.Metadata.ID, repository, s); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit project: %w", err)
	}
	return nil
}

func (s *Store) ListProjects(ctx context.Context) ([]domain.Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM projects ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan project id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list projects rows: %w", err)
	}
	_ = rows.Close()
	projects := make([]domain.Project, 0, len(ids))
	for _, id := range ids {
		project, err := s.GetProject(ctx, id)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, nil
}

func (s *Store) GetProject(ctx context.Context, id string) (domain.Project, error) {
	var object string
	if err := s.db.QueryRowContext(ctx, `SELECT spec_json FROM projects WHERE id = ?`, id).Scan(&object); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Project{}, sql.ErrNoRows
		}
		return domain.Project{}, fmt.Errorf("get project: %w", err)
	}
	var project domain.Project
	if err := json.Unmarshal([]byte(object), &project); err != nil {
		return domain.Project{}, fmt.Errorf("decode project: %w", err)
	}
	repositories, err := s.ListRepositories(ctx, id)
	if err != nil {
		return domain.Project{}, err
	}
	project.Spec.Repositories = repositories
	return project, nil
}

func (s *Store) DeleteProject(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) SaveRepository(ctx context.Context, projectID string, repository domain.Repository) error {
	if err := repository.Validate(); err != nil {
		return fmt.Errorf("validate repository: %w", err)
	}
	if strings.TrimSpace(projectID) == "" {
		return errors.New("project id is required")
	}
	if _, err := s.GetProject(ctx, projectID); err != nil {
		return err
	}
	return insertRepository(ctx, s.db, projectID, repository, s)
}

func insertRepository(ctx context.Context, execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, projectID string, repository domain.Repository, s *Store) error {
	object, err := s.maskedJSON(repository)
	if err != nil {
		return err
	}
	if _, err := execer.ExecContext(ctx, `
INSERT INTO repositories(project_id, id, api_version, kind, name, spec_json) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(project_id, id) DO UPDATE SET api_version=excluded.api_version, kind=excluded.kind,
name=excluded.name, spec_json=excluded.spec_json`,
		projectID, repository.Metadata.ID, repository.APIVersion, repository.Kind, repository.Metadata.Name, object); err != nil {
		return fmt.Errorf("save repository: %w", err)
	}
	return nil
}

func (s *Store) ListRepositories(ctx context.Context, projectID string) ([]domain.Repository, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT spec_json FROM repositories WHERE project_id = ? ORDER BY id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}
	defer rows.Close()
	repositories := make([]domain.Repository, 0)
	for rows.Next() {
		var object string
		if err := rows.Scan(&object); err != nil {
			return nil, fmt.Errorf("scan repository: %w", err)
		}
		var repository domain.Repository
		if err := json.Unmarshal([]byte(object), &repository); err != nil {
			return nil, fmt.Errorf("decode repository: %w", err)
		}
		repositories = append(repositories, repository)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list repositories rows: %w", err)
	}
	return repositories, nil
}

func (s *Store) GetRepository(ctx context.Context, projectID, repositoryID string) (domain.Repository, error) {
	var object string
	if err := s.db.QueryRowContext(ctx, `SELECT spec_json FROM repositories WHERE project_id = ? AND id = ?`, projectID, repositoryID).Scan(&object); err != nil {
		return domain.Repository{}, err
	}
	var repository domain.Repository
	if err := json.Unmarshal([]byte(object), &repository); err != nil {
		return domain.Repository{}, fmt.Errorf("decode repository: %w", err)
	}
	return repository, nil
}

func (s *Store) DeleteRepository(ctx context.Context, projectID, repositoryID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM repositories WHERE project_id = ? AND id = ?`, projectID, repositoryID)
	if err != nil {
		return fmt.Errorf("delete repository: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) SaveObservation(ctx context.Context, observation domain.Observation) error {
	if err := observation.Validate(); err != nil {
		return fmt.Errorf("validate observation: %w", err)
	}
	object, err := s.maskedJSON(observation)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO observations(id, project_id, repository_id, api_version, kind, fingerprint, collected_at, object_json)
VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET project_id=excluded.project_id, repository_id=excluded.repository_id,
api_version=excluded.api_version, kind=excluded.kind, fingerprint=excluded.fingerprint,
collected_at=excluded.collected_at, object_json=excluded.object_json`,
		observation.Metadata.ID, observation.Spec.ProjectID, observation.Spec.RepositoryID, observation.APIVersion,
		observation.Kind, observation.Spec.Fingerprint, observation.Spec.CollectedAt.UTC().Format(timeFormat), object)
	if err != nil {
		return fmt.Errorf("save observation: %w", err)
	}
	return nil
}

func (s *Store) ListObservations(ctx context.Context, projectID, repositoryID string) ([]domain.Observation, error) {
	query := `SELECT object_json FROM observations WHERE project_id = ?`
	args := []any{projectID}
	if repositoryID != "" {
		query += ` AND repository_id = ?`
		args = append(args, repositoryID)
	}
	query += ` ORDER BY collected_at DESC, id DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list observations: %w", err)
	}
	defer rows.Close()
	observations := make([]domain.Observation, 0)
	for rows.Next() {
		var object string
		if err := rows.Scan(&object); err != nil {
			return nil, fmt.Errorf("scan observation: %w", err)
		}
		var observation domain.Observation
		if err := json.Unmarshal([]byte(object), &observation); err != nil {
			return nil, fmt.Errorf("decode observation: %w", err)
		}
		observations = append(observations, observation)
	}
	return observations, rows.Err()
}

func (s *Store) SaveFinding(ctx context.Context, finding domain.Finding) error {
	if err := finding.Validate(); err != nil {
		return fmt.Errorf("validate finding: %w", err)
	}
	object, err := s.maskedJSON(finding)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO findings(id, project_id, repository_id, api_version, kind, fingerprint, state, last_observed, object_json)
VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET project_id=excluded.project_id, repository_id=excluded.repository_id,
api_version=excluded.api_version, kind=excluded.kind, fingerprint=excluded.fingerprint,
state=excluded.state, last_observed=excluded.last_observed, object_json=excluded.object_json`,
		finding.Metadata.ID, finding.Spec.ProjectID, finding.Spec.RepositoryID, finding.APIVersion,
		finding.Kind, finding.Spec.Fingerprint, finding.Spec.State, finding.Spec.LastObserved.UTC().Format(timeFormat), object)
	if err != nil {
		return fmt.Errorf("save finding: %w", err)
	}
	return nil
}

func (s *Store) ListFindings(ctx context.Context, projectID, repositoryID string) ([]domain.Finding, error) {
	query := `SELECT object_json FROM findings WHERE (? = '' OR project_id = ?)`
	args := []any{projectID, projectID}
	if repositoryID != "" {
		query += ` AND repository_id = ?`
		args = append(args, repositoryID)
	}
	query += ` ORDER BY CASE state WHEN 'open' THEN 0 WHEN 'acknowledged' THEN 1 ELSE 2 END, last_observed DESC, id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list findings: %w", err)
	}
	defer rows.Close()
	findings := make([]domain.Finding, 0)
	for rows.Next() {
		var object string
		if err := rows.Scan(&object); err != nil {
			return nil, fmt.Errorf("scan finding: %w", err)
		}
		var finding domain.Finding
		if err := json.Unmarshal([]byte(object), &finding); err != nil {
			return nil, fmt.Errorf("decode finding: %w", err)
		}
		findings = append(findings, finding)
	}
	return findings, rows.Err()
}

func (s *Store) GetFinding(ctx context.Context, id string) (domain.Finding, error) {
	var object string
	if err := s.db.QueryRowContext(ctx, `SELECT object_json FROM findings WHERE id = ?`, id).Scan(&object); err != nil {
		return domain.Finding{}, err
	}
	var finding domain.Finding
	if err := json.Unmarshal([]byte(object), &finding); err != nil {
		return domain.Finding{}, fmt.Errorf("decode finding: %w", err)
	}
	return finding, nil
}

func (s *Store) SaveEvent(ctx context.Context, event domain.Event) error {
	if err := event.Validate(); err != nil {
		return fmt.Errorf("validate event: %w", err)
	}
	object, err := s.maskedJSON(event)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO events(id, project_id, repository_id, event_type, occurred_at, object_json)
VALUES (?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET project_id=excluded.project_id, repository_id=excluded.repository_id,
event_type=excluded.event_type, occurred_at=excluded.occurred_at, object_json=excluded.object_json`,
		event.Metadata.ID, event.Spec.ProjectID, event.Spec.RepositoryID, event.Spec.EventType,
		event.Spec.OccurredAt.UTC().Format(timeFormat), object)
	if err != nil {
		return fmt.Errorf("save event: %w", err)
	}
	return nil
}

func (s *Store) ListEvents(ctx context.Context, limit int) ([]domain.Event, error) {
	if limit <= 0 {
		return []domain.Event{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT object_json FROM events ORDER BY occurred_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()
	events := make([]domain.Event, 0, limit)
	for rows.Next() {
		var object string
		if err := rows.Scan(&object); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		var event domain.Event
		if err := json.Unmarshal([]byte(object), &event); err != nil {
			return nil, fmt.Errorf("decode event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].Spec.OccurredAt.Before(events[j].Spec.OccurredAt) })
	return events, nil
}

const timeFormat = time.RFC3339Nano

func (s *Store) SaveScanRun(ctx context.Context, run domain.ScanRun) error {
	if err := run.Validate(); err != nil {
		return fmt.Errorf("validate scan run: %w", err)
	}
	object, err := s.maskedJSON(run)
	if err != nil {
		return err
	}
	var completed any
	if run.Spec.CompletedAt != nil {
		completed = run.Spec.CompletedAt.UTC().Format(timeFormat)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO scan_runs(id, project_id, trigger, status, started_at, completed_at, object_json)
VALUES (?, NULLIF(?, ''), ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET project_id=excluded.project_id, trigger=excluded.trigger,
status=excluded.status, started_at=excluded.started_at, completed_at=excluded.completed_at, object_json=excluded.object_json`,
		run.Metadata.ID, run.Spec.ProjectID, run.Spec.Trigger, run.Spec.Status,
		run.Spec.StartedAt.UTC().Format(timeFormat), completed, object)
	if err != nil {
		return fmt.Errorf("save scan run: %w", err)
	}
	return nil
}

func (s *Store) LatestScanRun(ctx context.Context, projectID string) (domain.ScanRun, error) {
	var object string
	if err := s.db.QueryRowContext(ctx, `SELECT object_json FROM scan_runs WHERE project_id = ? ORDER BY started_at DESC, id DESC LIMIT 1`, projectID).Scan(&object); err != nil {
		return domain.ScanRun{}, err
	}
	var run domain.ScanRun
	if err := json.Unmarshal([]byte(object), &run); err != nil {
		return domain.ScanRun{}, fmt.Errorf("decode scan run: %w", err)
	}
	return run, nil
}

func (s *Store) SaveFailureFingerprint(ctx context.Context, fingerprint domain.FailureFingerprint) error {
	if err := fingerprint.Validate(); err != nil {
		return fmt.Errorf("validate failure fingerprint: %w", err)
	}
	object, err := s.maskedJSON(fingerprint)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO failure_fingerprints(fingerprint, first_seen, last_seen, occurrence_count, object_json)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(fingerprint) DO UPDATE SET first_seen=excluded.first_seen,
last_seen=excluded.last_seen, occurrence_count=excluded.occurrence_count, object_json=excluded.object_json`,
		fingerprint.Spec.Fingerprint, fingerprint.Spec.FirstSeen.UTC().Format(timeFormat),
		fingerprint.Spec.LastSeen.UTC().Format(timeFormat), fingerprint.Spec.OccurrenceCount, object)
	if err != nil {
		return fmt.Errorf("save failure fingerprint: %w", err)
	}
	return nil
}

func (s *Store) GetFailureFingerprint(ctx context.Context, value string) (domain.FailureFingerprint, error) {
	var object string
	if err := s.db.QueryRowContext(ctx, `SELECT object_json FROM failure_fingerprints WHERE fingerprint = ?`, value).Scan(&object); err != nil {
		return domain.FailureFingerprint{}, err
	}
	var item domain.FailureFingerprint
	if err := json.Unmarshal([]byte(object), &item); err != nil {
		return domain.FailureFingerprint{}, fmt.Errorf("decode failure fingerprint: %w", err)
	}
	return item, nil
}

func (s *Store) ListFailureFingerprints(ctx context.Context, limit int) ([]domain.FailureFingerprint, error) {
	query := `SELECT object_json FROM failure_fingerprints ORDER BY last_seen DESC, fingerprint`
	args := []any{}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list failure fingerprints: %w", err)
	}
	defer rows.Close()
	items := make([]domain.FailureFingerprint, 0)
	for rows.Next() {
		var object string
		if err := rows.Scan(&object); err != nil {
			return nil, err
		}
		var item domain.FailureFingerprint
		if err := json.Unmarshal([]byte(object), &item); err != nil {
			return nil, fmt.Errorf("decode failure fingerprint: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SaveAgentProfile(ctx context.Context, profile domain.AgentProfile) error {
	if err := profile.Validate(); err != nil {
		return err
	}
	object, err := s.maskedJSON(profile)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO agent_profiles(id, object_json) VALUES (?, ?) ON CONFLICT(id) DO UPDATE SET object_json=excluded.object_json`, profile.Metadata.ID, object)
	return err
}

func (s *Store) ListAgentProfiles(ctx context.Context) ([]domain.AgentProfile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT object_json FROM agent_profiles ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := []domain.AgentProfile{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var profile domain.AgentProfile
		if err := json.Unmarshal([]byte(raw), &profile); err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (s *Store) GetAgentProfile(ctx context.Context, id string) (domain.AgentProfile, error) {
	var raw string
	if err := s.db.QueryRowContext(ctx, `SELECT object_json FROM agent_profiles WHERE id = ?`, id).Scan(&raw); err != nil {
		return domain.AgentProfile{}, err
	}
	var profile domain.AgentProfile
	if err := json.Unmarshal([]byte(raw), &profile); err != nil {
		return domain.AgentProfile{}, err
	}
	return profile, nil
}

func (s *Store) DeleteAgentProfile(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM agent_profiles WHERE id = ?`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) SaveSingleton(ctx context.Context, table string, value any, at time.Time) error {
	if table != "environment_health" && table != "scheduler_state" {
		return errors.New("unsupported singleton table")
	}
	object, err := s.maskedJSON(value)
	if err != nil {
		return err
	}
	column := map[string]string{"environment_health": "checked_at", "scheduler_state": "updated_at"}[table]
	_, err = s.db.ExecContext(ctx, `INSERT INTO `+table+`(id, object_json, `+column+`) VALUES (1, ?, ?) ON CONFLICT(id) DO UPDATE SET object_json=excluded.object_json, `+column+`=excluded.`+column, object, at.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) LoadSingleton(ctx context.Context, table string, target any) (bool, error) {
	if table != "environment_health" && table != "scheduler_state" {
		return false, errors.New("unsupported singleton table")
	}
	var raw string
	if err := s.db.QueryRowContext(ctx, `SELECT object_json FROM `+table+` WHERE id = 1`).Scan(&raw); errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) maskedJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal object: %w", err)
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("decode object for masking: %w", err)
	}
	masked := s.masker.MaskValue(raw)
	data, err = json.Marshal(masked)
	if err != nil {
		return "", fmt.Errorf("marshal masked object: %w", err)
	}
	return string(data), nil
}
