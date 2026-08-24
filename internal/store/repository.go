package store

// This file contains the typed SQLite repositories used by the application
// layer. The SQL schema is intentionally boring: the versioned domain object
// is the source of truth and indexed columns support the common list queries.
// Every object crosses the masking boundary before object_json is written.

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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

var (
	ErrActionPlanImmutable            = errors.New("action plan is immutable")
	ErrApprovalImmutable              = errors.New("approval is immutable")
	ErrActionEventImmutable           = errors.New("action event is immutable")
	ErrActionLockHeld                 = errors.New("action lock is held")
	ErrActionIdempotencyClaimed       = errors.New("action idempotency key is already claimed")
	ErrWorktreeExecutionTrustRequired = errors.New("worktree execution trust is required")
)

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

// ReplaceWorktrees stores a complete, verified enumeration atomically. Missing
// entries are tombstoned only after the caller proves enumeration completed.
func (s *Store) ReplaceWorktrees(ctx context.Context, projectID, repositoryID string, items []domain.Worktree, complete bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin worktree transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC().Format(timeFormat)
	seen := make([]any, 0, len(items))
	for _, item := range items {
		if item.Spec.Primary { // registered root is unconditionally primary.
			item.Metadata.ID = "primary"
		} else {
			var durableID string
			// A verified association is definitive. Path fallback is only safe if
			// one side is unverified; a different verified association at the
			// same path is a new worktree identity.
			if item.Spec.AssociationFingerprint != "" {
				err = tx.QueryRowContext(ctx, `SELECT id FROM worktrees WHERE project_id=? AND repository_id=? AND tombstoned_at IS NULL AND association_fingerprint=?`, projectID, repositoryID, item.Spec.AssociationFingerprint).Scan(&durableID)
			} else {
				err = tx.QueryRowContext(ctx, `SELECT id FROM worktrees WHERE project_id=? AND repository_id=? AND tombstoned_at IS NULL AND path_fingerprint=?`, projectID, repositoryID, item.Spec.PathFingerprint).Scan(&durableID)
			}
			if err == nil {
				item.Metadata.ID = durableID
			} else if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("reconcile worktree identity: %w", err)
			}
		}
		if err := item.Validate(); err != nil {
			return fmt.Errorf("validate worktree: %w", err)
		}
		if item.Spec.ProjectID != projectID || item.Spec.RepositoryID != repositoryID {
			return errors.New("worktree scope does not match repository")
		}
		object, err := s.maskedJSON(item)
		if err != nil {
			return err
		}
		association := any(nil)
		if !item.Spec.Primary {
			association = item.Spec.AssociationFingerprint
			if association == "" {
				association = "unverified:" + item.Spec.PathFingerprint
			}
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO worktrees(project_id, repository_id, id, association_fingerprint, path_fingerprint, canonical_path, trust, is_primary, last_observed, tombstoned_at, object_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?)
ON CONFLICT(project_id, repository_id, id) DO UPDATE SET association_fingerprint=excluded.association_fingerprint, path_fingerprint=excluded.path_fingerprint, canonical_path=excluded.canonical_path, trust=excluded.trust, is_primary=excluded.is_primary, last_observed=excluded.last_observed, tombstoned_at=NULL, object_json=excluded.object_json`, projectID, repositoryID, item.Metadata.ID, association, item.Spec.PathFingerprint, item.Spec.CanonicalPath, item.Spec.Trust, item.Spec.Primary, item.Spec.LastObserved.UTC().Format(timeFormat), object); err != nil {
			return fmt.Errorf("save worktree: %w", err)
		}
		observation := domain.WorktreeObservation{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.WorktreeObservationKind}, Metadata: domain.ObjectMeta{ID: worktreeObservationID(item), Name: "Git worktree state"}, Spec: domain.WorktreeObservationSpec{ProjectID: projectID, RepositoryID: repositoryID, WorktreeID: item.Metadata.ID, CollectedAt: item.Spec.LastObserved, Object: item}}
		observationJSON, err := s.maskedJSON(observation)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO worktree_observations(id, project_id, repository_id, worktree_id, collected_at, object_json) VALUES (?, ?, ?, ?, ?, ?)`, observation.Metadata.ID, projectID, repositoryID, item.Metadata.ID, item.Spec.LastObserved.UTC().Format(timeFormat), observationJSON); err != nil {
			return fmt.Errorf("save worktree observation: %w", err)
		}
		seen = append(seen, item.Metadata.ID)
	}
	if complete {
		query := `UPDATE worktrees SET tombstoned_at = ? WHERE project_id = ? AND repository_id = ? AND tombstoned_at IS NULL`
		args := []any{now, projectID, repositoryID}
		if len(seen) > 0 {
			query += ` AND id NOT IN (` + strings.TrimRight(strings.Repeat("?,", len(seen)), ",") + `)`
			args = append(args, seen...)
		}
		if _, err = tx.ExecContext(ctx, query, args...); err != nil {
			return fmt.Errorf("tombstone missing worktrees: %w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit worktrees: %w", err)
	}
	return nil
}

func (s *Store) ListWorktrees(ctx context.Context, projectID, repositoryID string) ([]domain.Worktree, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT object_json, tombstoned_at FROM worktrees WHERE project_id = ? AND repository_id = ? ORDER BY is_primary DESC, id`, projectID, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}
	defer rows.Close()
	items := []domain.Worktree{}
	for rows.Next() {
		var object string
		var tombstoned sql.NullString
		if err := rows.Scan(&object, &tombstoned); err != nil {
			return nil, err
		}
		var item domain.Worktree
		if err := json.Unmarshal([]byte(object), &item); err != nil {
			return nil, fmt.Errorf("decode worktree: %w", err)
		}
		if tombstoned.Valid {
			at, err := time.Parse(timeFormat, tombstoned.String)
			if err != nil {
				return nil, err
			}
			item.Spec.TombstonedAt = &at
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetWorktree(ctx context.Context, projectID, repositoryID, id string) (domain.Worktree, error) {
	items, err := s.ListWorktrees(ctx, projectID, repositoryID)
	if err != nil {
		return domain.Worktree{}, err
	}
	for _, item := range items {
		if item.Metadata.ID == id {
			return item, nil
		}
	}
	return domain.Worktree{}, sql.ErrNoRows
}

// TrustWorktreeForExecution is the sole persistence transition from bounded
// read-only discovery to a future execution target. It records the exact
// observed context; this method does not execute anything or grant approval.
func (s *Store) TrustWorktreeForExecution(ctx context.Context, projectID, repositoryID, worktreeID string, trustedAt time.Time) (domain.WorktreeExecutionTrust, error) {
	worktree, err := s.GetWorktree(ctx, projectID, repositoryID, worktreeID)
	if err != nil {
		return domain.WorktreeExecutionTrust{}, err
	}
	trust, err := domain.NewWorktreeExecutionTrust(worktree, trustedAt)
	if err != nil {
		return domain.WorktreeExecutionTrust{}, err
	}
	object, err := s.maskedJSON(trust)
	if err != nil {
		return domain.WorktreeExecutionTrust{}, err
	}
	digest, err := executionContextDigest(trust.Context)
	if err != nil {
		return domain.WorktreeExecutionTrust{}, err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO worktree_execution_trusts(project_id, repository_id, worktree_id, context_digest, trusted_at, object_json)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(project_id, repository_id, worktree_id) DO UPDATE SET context_digest=excluded.context_digest, trusted_at=excluded.trusted_at, object_json=excluded.object_json`,
		projectID, repositoryID, worktreeID, digest, trust.TrustedAt.UTC().Format(timeFormat), object)
	if err != nil {
		return domain.WorktreeExecutionTrust{}, fmt.Errorf("trust worktree for execution: %w", err)
	}
	return trust, nil
}

func (s *Store) GetWorktreeExecutionTrust(ctx context.Context, projectID, repositoryID, worktreeID string) (domain.WorktreeExecutionTrust, error) {
	var object string
	if err := s.db.QueryRowContext(ctx, `SELECT object_json FROM worktree_execution_trusts WHERE project_id=? AND repository_id=? AND worktree_id=?`, projectID, repositoryID, worktreeID).Scan(&object); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.WorktreeExecutionTrust{}, ErrWorktreeExecutionTrustRequired
		}
		return domain.WorktreeExecutionTrust{}, fmt.Errorf("get worktree execution trust: %w", err)
	}
	var trust domain.WorktreeExecutionTrust
	if err := json.Unmarshal([]byte(object), &trust); err != nil {
		return domain.WorktreeExecutionTrust{}, fmt.Errorf("decode worktree execution trust: %w", err)
	}
	if err := trust.Validate(); err != nil {
		return domain.WorktreeExecutionTrust{}, fmt.Errorf("validate worktree execution trust: %w", err)
	}
	return trust, nil
}

func (s *Store) SaveProposal(ctx context.Context, proposal domain.Proposal) error {
	if err := proposal.Validate(); err != nil {
		return fmt.Errorf("validate proposal: %w", err)
	}
	object, err := s.maskedJSON(proposal)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO proposals(id, project_id, repository_id, worktree_id, state, source_path, source_digest, created_at, object_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO NOTHING`,
		proposal.Metadata.ID, proposal.Spec.ProjectID, proposal.Spec.RepositoryID, proposal.Spec.WorktreeID,
		proposal.Spec.State, s.masker.Mask(proposal.Spec.SourcePath), proposal.Spec.SourceDigest, proposal.Spec.CreatedAt.UTC().Format(timeFormat), object)
	if err != nil {
		return fmt.Errorf("save proposal: %w", err)
	}
	return nil
}

func (s *Store) ListProposals(ctx context.Context, projectID, repositoryID, worktreeID string) ([]domain.Proposal, error) {
	query := `SELECT object_json FROM proposals WHERE project_id = ? AND repository_id = ?`
	args := []any{projectID, repositoryID}
	if worktreeID != "" {
		query += ` AND worktree_id = ?`
		args = append(args, worktreeID)
	}
	query += ` ORDER BY created_at DESC, id DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list proposals: %w", err)
	}
	defer rows.Close()
	items := make([]domain.Proposal, 0)
	for rows.Next() {
		var object string
		if err := rows.Scan(&object); err != nil {
			return nil, fmt.Errorf("scan proposal: %w", err)
		}
		var item domain.Proposal
		if err := json.Unmarshal([]byte(object), &item); err != nil {
			return nil, fmt.Errorf("decode proposal: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list proposal rows: %w", err)
	}
	return items, nil
}

func (s *Store) GetProposal(ctx context.Context, id string) (domain.Proposal, error) {
	var object string
	if err := s.db.QueryRowContext(ctx, `SELECT object_json FROM proposals WHERE id = ?`, id).Scan(&object); err != nil {
		return domain.Proposal{}, err
	}
	var proposal domain.Proposal
	if err := json.Unmarshal([]byte(object), &proposal); err != nil {
		return domain.Proposal{}, fmt.Errorf("decode proposal: %w", err)
	}
	return proposal, nil
}

func (s *Store) UpdateProposal(ctx context.Context, proposal domain.Proposal) error {
	if err := proposal.Validate(); err != nil {
		return fmt.Errorf("validate proposal: %w", err)
	}
	object, err := s.maskedJSON(proposal)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE proposals SET state = ?, object_json = ? WHERE id = ?`, proposal.Spec.State, object, proposal.Metadata.ID)
	if err != nil {
		return fmt.Errorf("update proposal: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ReviewProposal accepts exactly one pending-to-terminal review transition and
// writes its audit event in the same transaction.
func (s *Store) ReviewProposal(ctx context.Context, proposal domain.Proposal, event domain.Event) (bool, error) {
	if err := proposal.Validate(); err != nil {
		return false, fmt.Errorf("validate proposal: %w", err)
	}
	if proposal.Spec.State != domain.ProposalApplied && proposal.Spec.State != domain.ProposalRejected {
		return false, errors.New("proposal review requires an applied or rejected state")
	}
	if err := event.Validate(); err != nil {
		return false, fmt.Errorf("validate proposal review event: %w", err)
	}
	object, err := s.maskedJSON(proposal)
	if err != nil {
		return false, err
	}
	eventObject, err := s.maskedJSON(event)
	if err != nil {
		return false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin proposal review: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE proposals SET state = ?, object_json = ? WHERE id = ? AND state = ?`, proposal.Spec.State, object, proposal.Metadata.ID, domain.ProposalPending)
	if err != nil {
		return false, fmt.Errorf("review proposal: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read proposal review result: %w", err)
	}
	if count == 0 {
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO events(id, project_id, repository_id, event_type, occurred_at, object_json)
VALUES (?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET project_id=excluded.project_id, repository_id=excluded.repository_id,
event_type=excluded.event_type, occurred_at=excluded.occurred_at, object_json=excluded.object_json`,
		event.Metadata.ID, event.Spec.ProjectID, event.Spec.RepositoryID, event.Spec.EventType, event.Spec.OccurredAt.UTC().Format(timeFormat), eventObject); err != nil {
		return false, fmt.Errorf("save proposal review event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit proposal review: %w", err)
	}
	return true, nil
}

func (s *Store) MarkProposalStale(ctx context.Context, proposal domain.Proposal) error {
	if proposal.Spec.State != domain.ProposalStale {
		return errors.New("proposal stale transition requires stale state")
	}
	if err := proposal.Validate(); err != nil {
		return fmt.Errorf("validate proposal: %w", err)
	}
	object, err := s.maskedJSON(proposal)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE proposals SET state = ?, object_json = ? WHERE id = ? AND state = ?`, domain.ProposalStale, object, proposal.Metadata.ID, domain.ProposalPending)
	if err != nil {
		return fmt.Errorf("mark proposal stale: %w", err)
	}
	return nil
}

func worktreeObservationID(item domain.Worktree) string {
	sum := sha256.Sum256([]byte(item.Spec.ProjectID + "\x00" + item.Spec.RepositoryID + "\x00" + item.Metadata.ID + "\x00" + item.Spec.LastObserved.UTC().Format(time.RFC3339Nano)))
	return "worktree-observation-" + hex.EncodeToString(sum[:])[:48]
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

func (s *Store) RecordFailureFingerprint(ctx context.Context, occurrence domain.FailureFingerprint) (domain.FailureFingerprint, error) {
	if err := occurrence.Validate(); err != nil {
		return domain.FailureFingerprint{}, fmt.Errorf("validate failure occurrence: %w", err)
	}
	if occurrence.Spec.OccurrenceCount != 1 {
		return domain.FailureFingerprint{}, errors.New("failure occurrence count must start at one")
	}
	object, err := s.maskedJSON(occurrence)
	if err != nil {
		return domain.FailureFingerprint{}, err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO failure_fingerprints(fingerprint, first_seen, last_seen, occurrence_count, object_json)
VALUES (?, ?, ?, 1, ?)
ON CONFLICT(fingerprint) DO UPDATE SET
first_seen=CASE WHEN excluded.first_seen < failure_fingerprints.first_seen THEN excluded.first_seen ELSE failure_fingerprints.first_seen END,
last_seen=CASE WHEN excluded.last_seen > failure_fingerprints.last_seen THEN excluded.last_seen ELSE failure_fingerprints.last_seen END,
occurrence_count=failure_fingerprints.occurrence_count + 1`, occurrence.Spec.Fingerprint,
		occurrence.Spec.FirstSeen.UTC().Format(timeFormat), occurrence.Spec.LastSeen.UTC().Format(timeFormat), object)
	if err != nil {
		return domain.FailureFingerprint{}, fmt.Errorf("record failure fingerprint: %w", err)
	}
	for range 20 {
		item, storedObject, err := s.failureFingerprintRecord(ctx, occurrence.Spec.Fingerprint)
		if err != nil {
			return domain.FailureFingerprint{}, err
		}
		item.Spec.EvidenceRefs = mergeEvidenceRefs(item.Spec.EvidenceRefs, occurrence.Spec.EvidenceRefs)
		updatedObject, err := s.maskedJSON(item)
		if err != nil {
			return domain.FailureFingerprint{}, err
		}
		result, err := s.db.ExecContext(ctx, `UPDATE failure_fingerprints SET object_json = ? WHERE fingerprint = ? AND occurrence_count = ? AND object_json = ?`,
			updatedObject, item.Spec.Fingerprint, item.Spec.OccurrenceCount, storedObject)
		if err != nil {
			return domain.FailureFingerprint{}, fmt.Errorf("merge failure fingerprint evidence: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected == 1 {
			return item, nil
		}
	}
	return domain.FailureFingerprint{}, errors.New("failure fingerprint update conflicted repeatedly")
}

func (s *Store) GetFailureFingerprint(ctx context.Context, value string) (domain.FailureFingerprint, error) {
	item, _, err := s.failureFingerprintRecord(ctx, value)
	return item, err
}

func (s *Store) ListFailureFingerprints(ctx context.Context, limit int) ([]domain.FailureFingerprint, error) {
	query := `SELECT first_seen, last_seen, occurrence_count, object_json FROM failure_fingerprints ORDER BY last_seen DESC, fingerprint`
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
		var firstSeen, lastSeen, object string
		var occurrenceCount int
		if err := rows.Scan(&firstSeen, &lastSeen, &occurrenceCount, &object); err != nil {
			return nil, err
		}
		item, err := decodeFailureFingerprint(object, firstSeen, lastSeen, occurrenceCount)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) failureFingerprintRecord(ctx context.Context, fingerprint string) (domain.FailureFingerprint, string, error) {
	var firstSeen, lastSeen, object string
	var occurrenceCount int
	if err := s.db.QueryRowContext(ctx, `SELECT first_seen, last_seen, occurrence_count, object_json FROM failure_fingerprints WHERE fingerprint = ?`, fingerprint).
		Scan(&firstSeen, &lastSeen, &occurrenceCount, &object); err != nil {
		return domain.FailureFingerprint{}, "", err
	}
	item, err := decodeFailureFingerprint(object, firstSeen, lastSeen, occurrenceCount)
	return item, object, err
}

func decodeFailureFingerprint(object, firstSeen, lastSeen string, occurrenceCount int) (domain.FailureFingerprint, error) {
	var item domain.FailureFingerprint
	if err := json.Unmarshal([]byte(object), &item); err != nil {
		return domain.FailureFingerprint{}, fmt.Errorf("decode failure fingerprint: %w", err)
	}
	first, err := time.Parse(timeFormat, firstSeen)
	if err != nil {
		return domain.FailureFingerprint{}, fmt.Errorf("decode failure fingerprint first seen: %w", err)
	}
	last, err := time.Parse(timeFormat, lastSeen)
	if err != nil {
		return domain.FailureFingerprint{}, fmt.Errorf("decode failure fingerprint last seen: %w", err)
	}
	item.Spec.FirstSeen = first
	item.Spec.LastSeen = last
	item.Spec.OccurrenceCount = occurrenceCount
	return item, nil
}

func mergeEvidenceRefs(current, added []string) []string {
	merged := append([]string(nil), current...)
	for _, value := range added {
		if value == "" || slices.Contains(merged, value) {
			continue
		}
		merged = append(merged, value)
	}
	if len(merged) > 20 {
		merged = merged[len(merged)-20:]
	}
	return merged
}

func (s *Store) CreateSafeguardRule(ctx context.Context, rule domain.SafeguardRule) (bool, error) {
	if err := rule.Validate(); err != nil {
		return false, fmt.Errorf("validate safeguard rule: %w", err)
	}
	object, err := s.maskedJSON(rule)
	if err != nil {
		return false, err
	}
	result, err := s.db.ExecContext(ctx, `
INSERT INTO safeguard_rules(id, fingerprint, category, project_id, repository_id, worktree_id, state, revision, updated_at, object_json)
VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?)
ON CONFLICT DO NOTHING`, rule.Metadata.ID, rule.Spec.Fingerprint, rule.Spec.Category, rule.Spec.ProjectID, rule.Spec.RepositoryID, rule.Spec.WorktreeID, rule.Spec.State, rule.Spec.Revision,
		rule.Spec.UpdatedAt.UTC().Format(timeFormat), object)
	if err != nil {
		return false, fmt.Errorf("create safeguard rule: %w", err)
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func (s *Store) UpdateSafeguardRule(ctx context.Context, rule domain.SafeguardRule, previousRevision int64) (bool, error) {
	if err := rule.Validate(); err != nil {
		return false, fmt.Errorf("validate safeguard rule: %w", err)
	}
	if previousRevision < 1 || rule.Spec.Revision != previousRevision+1 {
		return false, errors.New("safeguard revision must advance exactly once")
	}
	object, err := s.maskedJSON(rule)
	if err != nil {
		return false, err
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE safeguard_rules SET fingerprint = ?, category = ?, project_id = ?, repository_id = ?, worktree_id = NULLIF(?, ''),
state = ?, revision = ?, updated_at = ?, object_json = ? WHERE id = ? AND revision = ?`,
		rule.Spec.Fingerprint, rule.Spec.Category, rule.Spec.ProjectID, rule.Spec.RepositoryID, rule.Spec.WorktreeID,
		rule.Spec.State, rule.Spec.Revision, rule.Spec.UpdatedAt.UTC().Format(timeFormat), object, rule.Metadata.ID, previousRevision)
	if err != nil {
		return false, fmt.Errorf("update safeguard rule: %w", err)
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

func (s *Store) GetSafeguardRule(ctx context.Context, id string) (domain.SafeguardRule, error) {
	return s.getSafeguardRule(ctx, `SELECT object_json FROM safeguard_rules WHERE id = ?`, id)
}

func (s *Store) GetSafeguardRuleByFingerprint(ctx context.Context, fingerprint string) (domain.SafeguardRule, error) {
	return s.getSafeguardRule(ctx, `SELECT object_json FROM safeguard_rules WHERE fingerprint = ?`, fingerprint)
}

func (s *Store) getSafeguardRule(ctx context.Context, query, value string) (domain.SafeguardRule, error) {
	var object string
	if err := s.db.QueryRowContext(ctx, query, value).Scan(&object); err != nil {
		return domain.SafeguardRule{}, err
	}
	var rule domain.SafeguardRule
	if err := json.Unmarshal([]byte(object), &rule); err != nil {
		return domain.SafeguardRule{}, fmt.Errorf("decode safeguard rule: %w", err)
	}
	return rule, nil
}

func (s *Store) ListSafeguardRules(ctx context.Context, limit int) ([]domain.SafeguardRule, error) {
	query := `SELECT object_json FROM safeguard_rules ORDER BY updated_at DESC, id`
	args := []any{}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list safeguard rules: %w", err)
	}
	defer rows.Close()
	items := make([]domain.SafeguardRule, 0)
	for rows.Next() {
		var object string
		if err := rows.Scan(&object); err != nil {
			return nil, err
		}
		var item domain.SafeguardRule
		if err := json.Unmarshal([]byte(object), &item); err != nil {
			return nil, fmt.Errorf("decode safeguard rule: %w", err)
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

func (s *Store) SaveCheckset(ctx context.Context, checkset domain.Checkset) error {
	if err := checkset.Validate(); err != nil {
		return fmt.Errorf("validate checkset: %w", err)
	}
	object, err := s.maskedJSON(checkset)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO checksets(id, project_id, repository_id, object_json) VALUES (?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET object_json=excluded.object_json`, checkset.Metadata.ID, checkset.Spec.ProjectID, checkset.Spec.RepositoryID, object)
	return err
}

func (s *Store) GetCheckset(ctx context.Context, id string) (domain.Checkset, error) {
	var object string
	if err := s.db.QueryRowContext(ctx, `SELECT object_json FROM checksets WHERE id = ?`, id).Scan(&object); err != nil {
		return domain.Checkset{}, err
	}
	var checkset domain.Checkset
	if err := json.Unmarshal([]byte(object), &checkset); err != nil {
		return domain.Checkset{}, fmt.Errorf("decode checkset: %w", err)
	}
	return checkset, nil
}

func (s *Store) ListChecksets(ctx context.Context, projectID, repositoryID string) ([]domain.Checkset, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT object_json FROM checksets WHERE project_id = ? AND repository_id = ? ORDER BY id`, projectID, repositoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Checkset{}
	for rows.Next() {
		var object string
		if err := rows.Scan(&object); err != nil {
			return nil, err
		}
		var item domain.Checkset
		if err := json.Unmarshal([]byte(object), &item); err != nil {
			return nil, fmt.Errorf("decode checkset: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SaveCheckRun(ctx context.Context, run domain.CheckRun) error {
	if err := run.Validate(); err != nil {
		return fmt.Errorf("validate check run: %w", err)
	}
	object, err := s.maskedJSON(run)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO check_runs(id, checkset_id, project_id, repository_id, worktree_id, started_at, completed_at, status, object_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, run.Metadata.ID, run.Spec.ChecksetID, run.Spec.ProjectID, run.Spec.RepositoryID, run.Spec.WorktreeID, run.Spec.StartedAt.UTC().Format(timeFormat), run.Spec.CompletedAt.UTC().Format(timeFormat), run.Spec.Status, object)
	return err
}

func (s *Store) ListCheckRuns(ctx context.Context, checksetID string) ([]domain.CheckRun, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT object_json FROM check_runs WHERE checkset_id = ? ORDER BY started_at DESC, id DESC`, checksetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.CheckRun{}
	for rows.Next() {
		var object string
		if err := rows.Scan(&object); err != nil {
			return nil, err
		}
		var item domain.CheckRun
		if err := json.Unmarshal([]byte(object), &item); err != nil {
			return nil, fmt.Errorf("decode check run: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// SaveActionRunAndEvent commits one terminal execution result with its audit
// event. The run and event are both masked before they cross the persistence
// boundary and must refer to the same immutable plan digest.
func (s *Store) SaveActionRunAndEvent(ctx context.Context, run domain.ActionRun, event domain.ActionEvent, now time.Time) error {
	if err := run.Validate(); err != nil {
		return fmt.Errorf("validate action run: %w", err)
	}
	if err := event.Validate(); err != nil {
		return fmt.Errorf("validate action run event: %w", err)
	}
	plan, err := s.GetActionPlan(ctx, run.Spec.ActionPlanID)
	if err != nil {
		return fmt.Errorf("get action run action plan: %w", err)
	}
	digest, err := plan.Digest()
	if err != nil {
		return fmt.Errorf("digest action run action plan: %w", err)
	}
	if run.Spec.ActionPlanDigest != digest || event.Spec.ActionPlanID != plan.Metadata.ID || event.Spec.ActionPlanDigest != digest {
		return errors.New("action run or event digest does not match plan")
	}
	if run.Spec.ProjectID != plan.Spec.ProjectID || run.Spec.RepositoryID != plan.Spec.RepositoryID || run.Spec.WorktreeID != plan.Spec.WorktreeID || run.Spec.ExecutionContext != plan.Spec.ExecutionContext {
		return errors.New("action run target does not match plan")
	}
	runJSON, err := s.maskedJSON(run)
	if err != nil {
		return err
	}
	eventJSON, err := s.maskedJSON(event)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin action run: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
INSERT INTO action_runs(id, action_plan_id, project_id, repository_id, worktree_id, action_plan_digest, started_at, completed_at, status, object_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO NOTHING`, run.Metadata.ID, run.Spec.ActionPlanID, run.Spec.ProjectID, run.Spec.RepositoryID, run.Spec.WorktreeID, run.Spec.ActionPlanDigest, run.Spec.StartedAt.UTC().Format(timeFormat), run.Spec.CompletedAt.UTC().Format(timeFormat), run.Spec.Status, runJSON); err != nil {
		return fmt.Errorf("save action run: %w", err)
	}
	var existing string
	if err := tx.QueryRowContext(ctx, `SELECT object_json FROM action_runs WHERE id = ?`, run.Metadata.ID).Scan(&existing); err != nil {
		return fmt.Errorf("read action run: %w", err)
	}
	if existing != runJSON {
		return errors.New("action run is immutable")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO action_events(id, action_plan_id, action_plan_digest, event_type, actor_kind, actor_id, occurred_at, object_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO NOTHING`, event.Metadata.ID, event.Spec.ActionPlanID, event.Spec.ActionPlanDigest, event.Spec.EventType, event.Spec.Actor.Kind, event.Spec.Actor.ID, event.Spec.OccurredAt.UTC().Format(timeFormat), eventJSON); err != nil {
		return fmt.Errorf("save action run event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit action run: %w", err)
	}
	return nil
}

func (s *Store) GetActionRun(ctx context.Context, id string) (domain.ActionRun, error) {
	var object string
	if err := s.db.QueryRowContext(ctx, `SELECT object_json FROM action_runs WHERE id = ?`, id).Scan(&object); err != nil {
		return domain.ActionRun{}, err
	}
	var run domain.ActionRun
	if err := json.Unmarshal([]byte(object), &run); err != nil {
		return domain.ActionRun{}, fmt.Errorf("decode action run: %w", err)
	}
	return run, nil
}

func (s *Store) ListActionRuns(ctx context.Context, actionPlanID string) ([]domain.ActionRun, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT object_json FROM action_runs WHERE action_plan_id = ? ORDER BY started_at DESC, id DESC`, actionPlanID)
	if err != nil {
		return nil, fmt.Errorf("list action runs: %w", err)
	}
	defer rows.Close()
	items := []domain.ActionRun{}
	for rows.Next() {
		var object string
		if err := rows.Scan(&object); err != nil {
			return nil, fmt.Errorf("scan action run: %w", err)
		}
		var run domain.ActionRun
		if err := json.Unmarshal([]byte(object), &run); err != nil {
			return nil, fmt.Errorf("decode action run: %w", err)
		}
		items = append(items, run)
	}
	return items, rows.Err()
}

// SaveActionPlan persists one immutable, typed plan. It never replaces a
// plan with the same ID because that would invalidate a previously reviewed
// approval without changing its reference.
func (s *Store) SaveActionPlan(ctx context.Context, plan domain.ActionPlan) error {
	if err := plan.Validate(); err != nil {
		return fmt.Errorf("validate action plan: %w", err)
	}
	if plan.Spec.RepositoryID != "" {
		worktree, err := s.GetWorktree(ctx, plan.Spec.ProjectID, plan.Spec.RepositoryID, plan.Spec.WorktreeID)
		if err != nil {
			return fmt.Errorf("get action plan worktree: %w", err)
		}
		if worktree.Spec.TombstonedAt != nil {
			return errors.New("action plan worktree is no longer active")
		}
		context, err := domain.ExecutionContextForWorktree(worktree)
		if err != nil || context != plan.Spec.ExecutionContext {
			return errors.New("action plan execution context does not match the observed worktree")
		}
	}
	object, err := s.maskedJSON(plan)
	if err != nil {
		return err
	}
	var persisted domain.ActionPlan
	if err := json.Unmarshal([]byte(object), &persisted); err != nil {
		return fmt.Errorf("decode masked action plan: %w", err)
	}
	digest, err := persisted.Digest()
	if err != nil {
		return fmt.Errorf("digest action plan: %w", err)
	}
	contextDigest, err := executionContextDigest(persisted.Spec.ExecutionContext)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `
INSERT INTO action_plans(id, project_id, repository_id, worktree_id, action_type, risk, policy_decision, digest, execution_context_digest, created_at, requester_kind, requester_id, requested_at, object_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO NOTHING`,
		persisted.Metadata.ID, persisted.Spec.ProjectID, persisted.Spec.RepositoryID, persisted.Spec.WorktreeID,
		persisted.Spec.ActionType, persisted.Spec.Risk, persisted.Spec.PolicyDecision, digest, contextDigest, time.Now().UTC().Format(timeFormat),
		persisted.Spec.RequestedBy.Kind, persisted.Spec.RequestedBy.ID, persisted.Spec.RequestedAt.UTC().Format(timeFormat), object)
	if err != nil {
		return fmt.Errorf("save action plan: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 0 {
		return nil
	}
	var existingDigest string
	if err := s.db.QueryRowContext(ctx, `SELECT digest FROM action_plans WHERE id = ?`, persisted.Metadata.ID).Scan(&existingDigest); err != nil {
		return fmt.Errorf("read existing action plan: %w", err)
	}
	if existingDigest != digest {
		return ErrActionPlanImmutable
	}
	return nil
}

func executionContextDigest(context domain.WorktreeExecutionContext) (string, error) {
	data, err := json.Marshal(context)
	if err != nil {
		return "", fmt.Errorf("marshal execution context: %w", err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func (s *Store) GetActionPlan(ctx context.Context, id string) (domain.ActionPlan, error) {
	var object string
	if err := s.db.QueryRowContext(ctx, `SELECT object_json FROM action_plans WHERE id = ?`, id).Scan(&object); err != nil {
		return domain.ActionPlan{}, err
	}
	var plan domain.ActionPlan
	if err := json.Unmarshal([]byte(object), &plan); err != nil {
		return domain.ActionPlan{}, fmt.Errorf("decode action plan: %w", err)
	}
	return plan, nil
}

func (s *Store) ListActionPlans(ctx context.Context) ([]domain.ActionPlan, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT object_json FROM action_plans ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list action plans: %w", err)
	}
	defer rows.Close()
	items := []domain.ActionPlan{}
	for rows.Next() {
		var object string
		if err := rows.Scan(&object); err != nil {
			return nil, fmt.Errorf("scan action plan: %w", err)
		}
		var plan domain.ActionPlan
		if err := json.Unmarshal([]byte(object), &plan); err != nil {
			return nil, fmt.Errorf("decode action plan: %w", err)
		}
		items = append(items, plan)
	}
	return items, rows.Err()
}

func (s *Store) SaveApproval(ctx context.Context, approval domain.Approval) error {
	return s.SaveApprovalAt(ctx, approval, time.Now().UTC())
}

func (s *Store) SaveApprovalAt(ctx context.Context, approval domain.Approval, now time.Time) error {
	if err := approval.Validate(); err != nil {
		return fmt.Errorf("validate approval: %w", err)
	}
	plan, err := s.GetActionPlan(ctx, approval.Spec.ActionPlanID)
	if err != nil {
		return fmt.Errorf("get approval action plan: %w", err)
	}
	if err := approval.ValidateForAt(plan, now); err != nil {
		return fmt.Errorf("validate approval for plan: %w", err)
	}
	object, err := s.maskedJSON(approval)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `
INSERT INTO approvals(id, action_plan_id, action_plan_digest, status, expires_at, decided_at, object_json)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO NOTHING`, approval.Metadata.ID, approval.Spec.ActionPlanID, approval.Spec.ActionPlanDigest,
		approval.Spec.Status, nullableTime(approval.Spec.ExpiresAt), approval.Spec.DecidedAt.UTC().Format(timeFormat), object)
	if err != nil {
		return fmt.Errorf("save approval: %w", err)
	}
	if count, _ := result.RowsAffected(); count != 0 {
		return nil
	}
	var existing string
	if err := s.db.QueryRowContext(ctx, `SELECT object_json FROM approvals WHERE id = ?`, approval.Metadata.ID).Scan(&existing); err != nil {
		return fmt.Errorf("read existing approval: %w", err)
	}
	if existing != object {
		return ErrApprovalImmutable
	}
	return nil
}

func (s *Store) ListApprovals(ctx context.Context, actionPlanID string) ([]domain.Approval, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT object_json FROM approvals WHERE action_plan_id = ? ORDER BY id`, actionPlanID)
	if err != nil {
		return nil, fmt.Errorf("list approvals: %w", err)
	}
	defer rows.Close()
	items := []domain.Approval{}
	for rows.Next() {
		var object string
		if err := rows.Scan(&object); err != nil {
			return nil, fmt.Errorf("scan approval: %w", err)
		}
		var approval domain.Approval
		if err := json.Unmarshal([]byte(object), &approval); err != nil {
			return nil, fmt.Errorf("decode approval: %w", err)
		}
		items = append(items, approval)
	}
	return items, rows.Err()
}

type ActionLock struct {
	Scope, ActionPlanID, ActionPlanDigest, Holder string
	ExpiresAt                                     time.Time
}

func (s *Store) AcquireActionLock(ctx context.Context, lock ActionLock, now time.Time) error {
	if strings.TrimSpace(lock.Scope) == "" || strings.TrimSpace(lock.ActionPlanID) == "" || strings.TrimSpace(lock.ActionPlanDigest) == "" || strings.TrimSpace(lock.Holder) == "" || !lock.ExpiresAt.After(now) {
		return errors.New("action lock fields are invalid")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin action lock: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM action_locks WHERE scope = ? AND expires_at <= ?`, lock.Scope, now.UTC().Format(timeFormat)); err != nil {
		return fmt.Errorf("expire action lock: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO action_locks(scope, action_plan_id, action_plan_digest, holder, expires_at) VALUES (?, ?, ?, ?, ?)`, lock.Scope, lock.ActionPlanID, lock.ActionPlanDigest, lock.Holder, lock.ExpiresAt.UTC().Format(timeFormat)); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrActionLockHeld
		}
		return fmt.Errorf("acquire action lock: %w", err)
	}
	return tx.Commit()
}

func (s *Store) ReleaseActionLock(ctx context.Context, lock ActionLock) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM action_locks WHERE scope = ? AND action_plan_id = ? AND action_plan_digest = ? AND holder = ?`, lock.Scope, lock.ActionPlanID, lock.ActionPlanDigest, lock.Holder)
	if err != nil {
		return fmt.Errorf("release action lock: %w", err)
	}
	return nil
}

func (s *Store) ClaimActionIdempotency(ctx context.Context, key, planID, digest string, now time.Time) error {
	if !validActionIdempotencyKey(key) || strings.TrimSpace(planID) == "" || strings.TrimSpace(digest) == "" {
		return errors.New("action idempotency fields are invalid")
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO action_idempotency(idempotency_key, action_plan_id, action_plan_digest, claimed_at) VALUES (?, ?, ?, ?) ON CONFLICT(idempotency_key) DO NOTHING`, key, planID, digest, now.UTC().Format(timeFormat))
	if err != nil {
		return fmt.Errorf("claim action idempotency: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return ErrActionIdempotencyClaimed
	}
	return nil
}

func (s *Store) ReleaseActionIdempotency(ctx context.Context, key, planID, digest string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM action_idempotency WHERE idempotency_key = ? AND action_plan_id = ? AND action_plan_digest = ?`, key, planID, digest)
	if err != nil {
		return fmt.Errorf("release action idempotency: %w", err)
	}
	return nil
}

// SaveApprovalAndActionEvent is the broker's all-or-nothing human decision
// record: an immutable approval is never committed without its audit event.
func (s *Store) SaveApprovalAndActionEvent(ctx context.Context, approval domain.Approval, event domain.ActionEvent, now time.Time) error {
	if err := approval.Validate(); err != nil {
		return fmt.Errorf("validate approval: %w", err)
	}
	if err := event.Validate(); err != nil {
		return fmt.Errorf("validate action event: %w", err)
	}
	plan, err := s.GetActionPlan(ctx, approval.Spec.ActionPlanID)
	if err != nil {
		return fmt.Errorf("get approval action plan: %w", err)
	}
	if err := approval.ValidateForAt(plan, now); err != nil {
		return fmt.Errorf("validate approval for plan: %w", err)
	}
	digest, err := plan.Digest()
	if err != nil {
		return fmt.Errorf("digest action plan: %w", err)
	}
	if event.Spec.ActionPlanID != plan.Metadata.ID || event.Spec.ActionPlanDigest != digest {
		return errors.New("approval audit event does not match plan")
	}
	approvalJSON, err := s.maskedJSON(approval)
	if err != nil {
		return err
	}
	eventJSON, err := s.maskedJSON(event)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin approval audit: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO approvals(id, action_plan_id, action_plan_digest, status, expires_at, decided_at, object_json) VALUES (?, ?, ?, ?, ?, ?, ?)`, approval.Metadata.ID, approval.Spec.ActionPlanID, approval.Spec.ActionPlanDigest, approval.Spec.Status, nullableTime(approval.Spec.ExpiresAt), approval.Spec.DecidedAt.UTC().Format(timeFormat), approvalJSON); err != nil {
		return fmt.Errorf("save approval: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO action_events(id, action_plan_id, action_plan_digest, event_type, actor_kind, actor_id, occurred_at, object_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, event.Metadata.ID, event.Spec.ActionPlanID, event.Spec.ActionPlanDigest, event.Spec.EventType, event.Spec.Actor.Kind, event.Spec.Actor.ID, event.Spec.OccurredAt.UTC().Format(timeFormat), eventJSON); err != nil {
		return fmt.Errorf("save action event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit approval audit: %w", err)
	}
	return nil
}

func (s *Store) RenewActionLock(ctx context.Context, lock ActionLock, now time.Time, expiresAt time.Time) (ActionLock, error) {
	if !expiresAt.After(now) {
		return ActionLock{}, errors.New("action lock renewal must extend into the future")
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE action_locks SET expires_at = ?
WHERE scope = ? AND action_plan_id = ? AND action_plan_digest = ? AND holder = ? AND expires_at > ?`,
		expiresAt.UTC().Format(timeFormat), lock.Scope, lock.ActionPlanID, lock.ActionPlanDigest, lock.Holder, now.UTC().Format(timeFormat))
	if err != nil {
		return ActionLock{}, fmt.Errorf("renew action lock: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return ActionLock{}, ErrActionLockHeld
	}
	lock.ExpiresAt = expiresAt
	return lock, nil
}

func (s *Store) SaveActionEvent(ctx context.Context, event domain.ActionEvent) error {
	if err := event.Validate(); err != nil {
		return fmt.Errorf("validate action event: %w", err)
	}
	plan, err := s.GetActionPlan(ctx, event.Spec.ActionPlanID)
	if err != nil {
		return fmt.Errorf("get action event plan: %w", err)
	}
	digest, err := plan.Digest()
	if err != nil {
		return fmt.Errorf("digest action event plan: %w", err)
	}
	if event.Spec.ActionPlanDigest != digest {
		return errors.New("action event digest does not match plan")
	}
	object, err := s.maskedJSON(event)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `
INSERT INTO action_events(id, action_plan_id, action_plan_digest, event_type, actor_kind, actor_id, occurred_at, object_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO NOTHING`, event.Metadata.ID, event.Spec.ActionPlanID, event.Spec.ActionPlanDigest,
		event.Spec.EventType, event.Spec.Actor.Kind, event.Spec.Actor.ID, event.Spec.OccurredAt.UTC().Format(timeFormat), object)
	if err != nil {
		return fmt.Errorf("save action event: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return ErrActionEventImmutable
	}
	return nil
}

func (s *Store) ListActionEvents(ctx context.Context, actionPlanID string) ([]domain.ActionEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT object_json FROM action_events WHERE action_plan_id = ? ORDER BY occurred_at, id`, actionPlanID)
	if err != nil {
		return nil, fmt.Errorf("list action events: %w", err)
	}
	defer rows.Close()
	items := []domain.ActionEvent{}
	for rows.Next() {
		var object string
		if err := rows.Scan(&object); err != nil {
			return nil, fmt.Errorf("scan action event: %w", err)
		}
		var event domain.ActionEvent
		if err := json.Unmarshal([]byte(object), &event); err != nil {
			return nil, fmt.Errorf("decode action event: %w", err)
		}
		items = append(items, event)
	}
	return items, rows.Err()
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(timeFormat)
}

func validActionIdempotencyKey(value string) bool {
	return len(value) <= 128 && strings.TrimSpace(value) == value && value != "" && !strings.ContainsAny(value, "\x00\r\n")
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
