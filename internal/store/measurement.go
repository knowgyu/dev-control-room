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
	object, err := s.maskedMeasurementRunJSON(item)
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

func (s *Store) maskedMeasurementRunJSON(item measurement.Run) (string, error) {
	object, err := s.maskedJSON(item)
	if err != nil {
		return "", err
	}
	var masked measurement.Run
	if err := json.Unmarshal([]byte(object), &masked); err != nil {
		return "", fmt.Errorf("decode masked measurement run: %w", err)
	}
	if err := restoreMeasurementRunIdentity(&masked, item); err != nil {
		return "", err
	}
	if err := masked.Validate(); err != nil {
		return "", fmt.Errorf("validate masked measurement run: %w", err)
	}
	encoded, err := json.Marshal(masked)
	if err != nil {
		return "", fmt.Errorf("marshal masked measurement run: %w", err)
	}
	return string(encoded), nil
}

func restoreMeasurementRunIdentity(masked *measurement.Run, original measurement.Run) error {
	if len(masked.Spec.Measurements) != len(original.Spec.Measurements) {
		return errors.New("masked measurement run changed its measurement count")
	}
	masked.APIVersion = original.APIVersion
	masked.Kind = original.Kind
	masked.Metadata.ID = original.Metadata.ID
	masked.Spec.Status = original.Spec.Status
	masked.Spec.RequiredFailures = append([]string{}, original.Spec.RequiredFailures...)
	masked.Spec.Reproducibility.RunID = original.Spec.Reproducibility.RunID
	masked.Spec.Reproducibility.Commit = original.Spec.Reproducibility.Commit
	masked.Spec.Reproducibility.Head = original.Spec.Reproducibility.Head
	masked.Spec.Reproducibility.DirtyState = original.Spec.Reproducibility.DirtyState
	masked.Spec.Reproducibility.OS = original.Spec.Reproducibility.OS
	masked.Spec.Reproducibility.Arch = original.Spec.Reproducibility.Arch
	masked.Spec.Reproducibility.ConfigurationDigest = original.Spec.Reproducibility.ConfigurationDigest
	for index := range original.Spec.Measurements {
		maskedItem := &masked.Spec.Measurements[index]
		originalItem := original.Spec.Measurements[index]
		maskedItem.APIVersion = originalItem.APIVersion
		maskedItem.Kind = originalItem.Kind
		maskedItem.Metadata.ID = originalItem.Metadata.ID
		maskedItem.Spec.Name = originalItem.Spec.Name
		maskedItem.Spec.Category = originalItem.Spec.Category
		maskedItem.Spec.Status = originalItem.Spec.Status
		maskedItem.Spec.Provenance = originalItem.Spec.Provenance
		maskedItem.Spec.Unit = originalItem.Spec.Unit
		maskedItem.Spec.CommandID = originalItem.Spec.CommandID
	}
	return nil
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
