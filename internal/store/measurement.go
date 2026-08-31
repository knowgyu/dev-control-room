package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/knowgyu/dev-control-room/internal/measurement"
)

var ErrMeasurementRunDuplicate = errors.New("measurement run id already exists")

// SaveMeasurementRun stores a validated v1 manifest in the existing immutable
// assurance-object table. Measurement run IDs are intentionally not
// idempotent: importing the same ID twice is a duplicate evidence record.
func (s *Store) SaveMeasurementRun(ctx context.Context, item measurement.Run) error {
	if err := item.Validate(); err != nil {
		return fmt.Errorf("validate measurement run: %w", err)
	}
	object, err := s.maskedJSON(item)
	if err != nil {
		return err
	}
	digest, err := assuranceJSONDigest(object)
	if err != nil {
		return err
	}
	reproducibility := item.Spec.Reproducibility
	result, err := s.db.ExecContext(ctx, `
INSERT INTO assurance_objects(id, kind, state, revision, digest, created_at, updated_at, object_json)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO NOTHING`,
		item.Metadata.ID,
		measurement.MeasurementRunKind,
		string(item.Spec.Status),
		1,
		digest,
		reproducibility.StartedAt.UTC().Format(timeFormat),
		reproducibility.EndedAt.UTC().Format(timeFormat),
		object,
	)
	if err != nil {
		return fmt.Errorf("save measurement run: %w", err)
	}
	if count, _ := result.RowsAffected(); count > 0 {
		return nil
	}
	return fmt.Errorf("%w: %q", ErrMeasurementRunDuplicate, item.Metadata.ID)
}

func (s *Store) GetMeasurementRun(ctx context.Context, id string) (measurement.Run, error) {
	var item measurement.Run
	if err := s.GetAssurance(ctx, measurement.MeasurementRunKind, id, &item); err != nil {
		return measurement.Run{}, err
	}
	if err := item.Validate(); err != nil {
		return measurement.Run{}, fmt.Errorf("validate stored measurement run: %w", err)
	}
	return item, nil
}

func (s *Store) ListMeasurementRuns(ctx context.Context) ([]measurement.Run, error) {
	items := []measurement.Run{}
	err := s.ListAssurance(ctx, measurement.MeasurementRunKind, func(data []byte) error {
		var item measurement.Run
		if err := json.Unmarshal(data, &item); err != nil {
			return err
		}
		if err := item.Validate(); err != nil {
			return fmt.Errorf("validate stored measurement run: %w", err)
		}
		items = append(items, item)
		return nil
	})
	return items, err
}
