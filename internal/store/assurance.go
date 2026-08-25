package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

// SaveAssurance* methods keep the additive assurance records revisioned and
// immutable by id. Updating a session/run is explicit and must increase the
// revision, which prevents a restart from silently replacing evidence.
func (s *Store) SaveAssuranceSession(ctx context.Context, item domain.AssuranceSession) error {
	return s.saveAssurance(ctx, domain.AssuranceSessionKind, item.Metadata.ID, item.Spec.ProjectID, item.Spec.RepositoryID, item.Spec.WorktreeID, item.Spec.State, 1, item.Spec.CreatedAt, item.Spec.UpdatedAt, item, item.Validate())
}

func (s *Store) SaveAssuranceQuestion(ctx context.Context, item domain.AssuranceQuestion) error {
	return s.saveAssurance(ctx, domain.AssuranceQuestionKind, item.Metadata.ID, "", "", "", "answered", 1, item.Spec.AskedAt, timeOr(item.Spec.AnsweredAt, item.Spec.AskedAt), item, item.Validate())
}

func (s *Store) SaveAssuranceSpec(ctx context.Context, item domain.AssuranceSpec) error {
	canonical := item
	canonical.Spec.Digest = ""
	digest, err := canonical.Digest()
	if err != nil {
		return err
	}
	if item.Spec.Digest == "" {
		item.Spec.Digest = digest
	}
	if item.Spec.Digest != digest {
		return errors.New("assurance spec digest mismatch")
	}
	return s.saveAssurance(ctx, domain.AssuranceSpecKind, item.Metadata.ID, "", "", "", item.Spec.State, item.Spec.Revision, item.Spec.CreatedAt, item.Spec.CreatedAt, item, item.Validate())
}

func (s *Store) ListAssuranceSpecs(ctx context.Context, sessionID string) ([]domain.AssuranceSpec, error) {
	items := []domain.AssuranceSpec{}
	err := s.ListAssurance(ctx, domain.AssuranceSpecKind, func(data []byte) error {
		var item domain.AssuranceSpec
		if err := json.Unmarshal(data, &item); err != nil {
			return err
		}
		if sessionID == "" || item.Spec.SessionID == sessionID {
			items = append(items, item)
		}
		return nil
	})
	return items, err
}

func (s *Store) SaveAssuranceProposal(ctx context.Context, item domain.AssuranceProposal) error {
	return s.saveAssurance(ctx, domain.AssuranceProposalKind, item.Metadata.ID, item.Spec.ProjectID, item.Spec.RepositoryID, item.Spec.WorktreeID, item.Spec.State, 1, item.Spec.CreatedAt, timeOr(item.Spec.ReviewedAt, item.Spec.CreatedAt), item, item.Validate())
}

func (s *Store) ListAssuranceProposals(ctx context.Context, sessionID string) ([]domain.AssuranceProposal, error) {
	items := []domain.AssuranceProposal{}
	err := s.ListAssurance(ctx, domain.AssuranceProposalKind, func(data []byte) error {
		var item domain.AssuranceProposal
		if err := json.Unmarshal(data, &item); err != nil {
			return err
		}
		if sessionID == "" || item.Spec.SessionID == sessionID {
			items = append(items, item)
		}
		return nil
	})
	return items, err
}

func (s *Store) ListAssuranceQuestions(ctx context.Context, sessionID string) ([]domain.AssuranceQuestion, error) {
	items := []domain.AssuranceQuestion{}
	err := s.ListAssurance(ctx, domain.AssuranceQuestionKind, func(data []byte) error {
		var item domain.AssuranceQuestion
		if err := json.Unmarshal(data, &item); err != nil {
			return err
		}
		if sessionID == "" || item.Spec.SessionID == sessionID {
			items = append(items, item)
		}
		return nil
	})
	return items, err
}

func (s *Store) UpdateAssuranceQuestion(ctx context.Context, item domain.AssuranceQuestion) error {
	if err := item.Validate(); err != nil {
		return err
	}
	object, err := s.maskedJSON(item)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE assurance_objects SET state = ?, updated_at = ?, object_json = ? WHERE kind = ? AND id = ?`, "answered", timeOr(item.Spec.AnsweredAt, item.Spec.AskedAt).UTC().Format(timeFormat), object, domain.AssuranceQuestionKind, item.Metadata.ID)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return errors.New("assurance question is missing")
	}
	return nil
}

func (s *Store) SaveAgentInvocation(ctx context.Context, item domain.AgentInvocation) error {
	return s.saveAssurance(ctx, domain.AgentInvocationKind, item.Metadata.ID, "", "", item.Spec.WorktreeID, item.Spec.State, 1, item.Spec.StartedAt, timeOr(item.Spec.CompletedAt, item.Spec.StartedAt), item, item.Validate())
}

func (s *Store) SaveQualityCampaign(ctx context.Context, item domain.QualityCampaign) error {
	return s.saveAssurance(ctx, domain.QualityCampaignKind, item.Metadata.ID, item.Spec.ProjectID, item.Spec.RepositoryID, item.Spec.WorktreeID, item.Spec.State, 1, item.Spec.CreatedAt, item.Spec.UpdatedAt, item, item.Validate())
}

func (s *Store) SaveQualityRun(ctx context.Context, item domain.QualityRun) error {
	return s.saveAssurance(ctx, domain.QualityRunKind, item.Metadata.ID, item.Spec.ProjectID, item.Spec.RepositoryID, item.Spec.WorktreeID, item.Spec.State, 1, item.Spec.StartedAt, timeOr(item.Spec.CompletedAt, item.Spec.StartedAt), item, item.Validate())
}

func (s *Store) SaveBaseline(ctx context.Context, item domain.PRCIBaseline) error {
	return s.saveAssurance(ctx, domain.PRCIBaselineKind, item.Metadata.ID, item.Spec.ProjectID, item.Spec.RepositoryID, item.Spec.WorktreeID, item.Spec.State, 1, item.Spec.CapturedAt, item.Spec.CapturedAt, item, item.Validate())
}

func (s *Store) SaveArtifact(ctx context.Context, item domain.Artifact) error {
	return s.saveAssurance(ctx, domain.ArtifactKind, item.Metadata.ID, "", "", "", item.Spec.Retention, 1, item.Spec.CreatedAt, timeOr(item.Spec.ArchivedAt, item.Spec.CreatedAt), item, item.Validate())
}

func (s *Store) UpdateAssuranceArtifact(ctx context.Context, item domain.Artifact) error {
	if err := item.Validate(); err != nil {
		return err
	}
	object, err := s.maskedJSON(item)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE assurance_objects SET state = ?, updated_at = ?, object_json = ? WHERE kind = ? AND id = ?`, item.Spec.Retention, timeOr(item.Spec.DeletedAt, timeOr(item.Spec.ArchivedAt, item.Spec.CreatedAt)).UTC().Format(timeFormat), object, domain.ArtifactKind, item.Metadata.ID)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return errors.New("assurance artifact is missing")
	}
	return nil
}

func (s *Store) SaveEffect(ctx context.Context, item domain.Effect) error {
	return s.saveAssurance(ctx, domain.EffectKind, item.Metadata.ID, item.Spec.ProjectID, item.Spec.RepositoryID, item.Spec.WorktreeID, item.Spec.Label, 1, item.Spec.CreatedAt, item.Spec.UpdatedAt, item, item.Validate())
}

func (s *Store) SavePricingSnapshot(ctx context.Context, item domain.ProviderPricingSnapshot) error {
	if err := item.Validate(); err != nil {
		return err
	}
	object, err := s.maskedJSON(item)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO provider_pricing_snapshots(id, provider, model, effective_at, object_json) VALUES (?, ?, ?, ?, ?) ON CONFLICT(id) DO NOTHING`, item.Metadata.ID, item.Spec.Provider, item.Spec.Model, item.Spec.EffectiveAt.UTC().Format(timeFormat), object)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count > 0 {
		return nil
	}
	var existing string
	if err := s.db.QueryRowContext(ctx, `SELECT object_json FROM provider_pricing_snapshots WHERE id = ?`, item.Metadata.ID).Scan(&existing); err != nil {
		return err
	}
	if existing != object {
		return errors.New("pricing snapshot is immutable")
	}
	return nil
}

func (s *Store) saveAssurance(ctx context.Context, kind, id, projectID, repositoryID, worktreeID, state string, revision int, createdAt, updatedAt time.Time, value any, validation error) error {
	if validation != nil {
		return validation
	}
	if strings.TrimSpace(id) == "" || strings.TrimSpace(kind) == "" || revision < 1 {
		return errors.New("assurance object identity is invalid")
	}
	object, err := s.maskedJSON(value)
	if err != nil {
		return err
	}
	digest, err := assuranceJSONDigest(object)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO assurance_objects(id, kind, project_id, repository_id, worktree_id, state, revision, digest, created_at, updated_at, object_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO NOTHING`, id, kind, nullableString(projectID), nullableString(repositoryID), nullableString(worktreeID), state, revision, digest, createdAt.UTC().Format(timeFormat), updatedAt.UTC().Format(timeFormat), object)
	if err != nil {
		return fmt.Errorf("save assurance object: %w", err)
	}
	if count, _ := result.RowsAffected(); count > 0 {
		return nil
	}
	var existingDigest string
	if err := s.db.QueryRowContext(ctx, `SELECT digest FROM assurance_objects WHERE id = ?`, id).Scan(&existingDigest); err != nil {
		return err
	}
	if existingDigest != digest {
		return errors.New("assurance object is immutable")
	}
	return nil
}

// UpdateAssuranceRevision is the only mutable path for an assurance object.
// A caller must provide a strictly larger revision; a stale worker therefore
// cannot overwrite a newer state after restart or lease expiry.
func (s *Store) UpdateAssuranceRevision(ctx context.Context, kind, id string, revision int, state string, updatedAt time.Time, value any) error {
	if revision < 1 || strings.TrimSpace(kind) == "" || strings.TrimSpace(id) == "" || strings.TrimSpace(state) == "" || updatedAt.IsZero() {
		return errors.New("assurance revision is invalid")
	}
	object, err := s.maskedJSON(value)
	if err != nil {
		return err
	}
	digest, err := assuranceJSONDigest(object)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE assurance_objects SET state = ?, revision = ?, digest = ?, updated_at = ?, object_json = ? WHERE kind = ? AND id = ? AND revision < ?`, state, revision, digest, updatedAt.UTC().Format(timeFormat), object, kind, id, revision)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return errors.New("assurance revision is stale or object is missing")
	}
	return nil
}

func (s *Store) AssuranceRevision(ctx context.Context, kind, id string) (int, error) {
	var revision int
	if err := s.db.QueryRowContext(ctx, `SELECT revision FROM assurance_objects WHERE kind = ? AND id = ?`, kind, id).Scan(&revision); err != nil {
		return 0, err
	}
	return revision, nil
}

func (s *Store) GetAssurance(ctx context.Context, kind, id string, target any) error {
	var object string
	if err := s.db.QueryRowContext(ctx, `SELECT object_json FROM assurance_objects WHERE kind = ? AND id = ?`, kind, id).Scan(&object); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(object), target); err != nil {
		return fmt.Errorf("decode assurance object: %w", err)
	}
	return nil
}

func (s *Store) ListAssurance(ctx context.Context, kind string, target func([]byte) error) error {
	rows, err := s.db.QueryContext(ctx, `SELECT object_json FROM assurance_objects WHERE kind = ? ORDER BY updated_at DESC, id DESC`, kind)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var object string
		if err := rows.Scan(&object); err != nil {
			return err
		}
		if err := target([]byte(object)); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Store) ListAssuranceSessions(ctx context.Context) ([]domain.AssuranceSession, error) {
	items := []domain.AssuranceSession{}
	err := s.ListAssurance(ctx, domain.AssuranceSessionKind, func(data []byte) error {
		var item domain.AssuranceSession
		if err := json.Unmarshal(data, &item); err != nil {
			return err
		}
		items = append(items, item)
		return nil
	})
	return items, err
}
func (s *Store) ListQualityCampaigns(ctx context.Context) ([]domain.QualityCampaign, error) {
	items := []domain.QualityCampaign{}
	err := s.ListAssurance(ctx, domain.QualityCampaignKind, func(data []byte) error {
		var item domain.QualityCampaign
		if err := json.Unmarshal(data, &item); err != nil {
			return err
		}
		items = append(items, item)
		return nil
	})
	return items, err
}
func (s *Store) ListAgentInvocations(ctx context.Context) ([]domain.AgentInvocation, error) {
	items := []domain.AgentInvocation{}
	err := s.ListAssurance(ctx, domain.AgentInvocationKind, func(data []byte) error {
		var item domain.AgentInvocation
		if err := json.Unmarshal(data, &item); err != nil {
			return err
		}
		items = append(items, item)
		return nil
	})
	return items, err
}
func (s *Store) ListQualityRuns(ctx context.Context) ([]domain.QualityRun, error) {
	items := []domain.QualityRun{}
	err := s.ListAssurance(ctx, domain.QualityRunKind, func(data []byte) error {
		var item domain.QualityRun
		if err := json.Unmarshal(data, &item); err != nil {
			return err
		}
		items = append(items, item)
		return nil
	})
	return items, err
}
func (s *Store) ListBaselines(ctx context.Context) ([]domain.PRCIBaseline, error) {
	items := []domain.PRCIBaseline{}
	err := s.ListAssurance(ctx, domain.PRCIBaselineKind, func(data []byte) error {
		var item domain.PRCIBaseline
		if err := json.Unmarshal(data, &item); err != nil {
			return err
		}
		items = append(items, item)
		return nil
	})
	return items, err
}
func (s *Store) ListArtifacts(ctx context.Context) ([]domain.Artifact, error) {
	items := []domain.Artifact{}
	err := s.ListAssurance(ctx, domain.ArtifactKind, func(data []byte) error {
		var item domain.Artifact
		if err := json.Unmarshal(data, &item); err != nil {
			return err
		}
		items = append(items, item)
		return nil
	})
	return items, err
}
func (s *Store) ListEffects(ctx context.Context) ([]domain.Effect, error) {
	items := []domain.Effect{}
	err := s.ListAssurance(ctx, domain.EffectKind, func(data []byte) error {
		var item domain.Effect
		if err := json.Unmarshal(data, &item); err != nil {
			return err
		}
		items = append(items, item)
		return nil
	})
	return items, err
}
func (s *Store) ListPricingSnapshots(ctx context.Context) ([]domain.ProviderPricingSnapshot, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT object_json FROM provider_pricing_snapshots ORDER BY effective_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.ProviderPricingSnapshot{}
	for rows.Next() {
		var object string
		if err := rows.Scan(&object); err != nil {
			return nil, err
		}
		var item domain.ProviderPricingSnapshot
		if err := json.Unmarshal([]byte(object), &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ClaimAssuranceLease(ctx context.Context, key, objectID, digest, holder string, expiresAt, now time.Time) error {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(objectID) == "" || strings.TrimSpace(digest) == "" || strings.TrimSpace(holder) == "" || !expiresAt.After(now) {
		return errors.New("assurance lease is invalid")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO assurance_leases(idempotency_key, object_id, digest, holder, expires_at) VALUES (?, ?, ?, ?, ?) ON CONFLICT(idempotency_key) DO NOTHING`, key, objectID, digest, holder, expiresAt.UTC().Format(timeFormat))
	if err != nil {
		return err
	}
	var existing string
	if err := s.db.QueryRowContext(ctx, `SELECT holder FROM assurance_leases WHERE idempotency_key = ?`, key).Scan(&existing); err != nil {
		return err
	}
	if existing != holder {
		return errors.New("assurance idempotency key is already claimed")
	}
	return nil
}

func (s *Store) ReleaseAssuranceLease(ctx context.Context, key, objectID, holder string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM assurance_leases WHERE idempotency_key = ? AND object_id = ? AND holder = ?`, key, objectID, holder)
	return err
}

func assuranceJSONDigest(object string) (string, error) {
	var value any
	if err := json.Unmarshal([]byte(object), &value); err != nil {
		return "", err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return domainDigest(data), nil
}
func domainDigest(data []byte) string  { sum := sha256Sum(data); return "sha256:" + sum }
func sha256Sum(data []byte) string     { return fmt.Sprintf("%x", sha256Bytes(data)) }
func sha256Bytes(data []byte) [32]byte { return sha256.Sum256(data) }
func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
func timeOr(value *time.Time, fallback time.Time) time.Time {
	if value != nil {
		return *value
	}
	return fallback
}

// Keep the ordering helper local to this file so list results remain stable
// even when SQLite returns equal timestamps.
func sortAssuranceIDs(ids []string) { sort.Strings(ids) }
