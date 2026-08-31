package app

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/measurement"
)

func FuzzMeasurementManifestValidationAndComparison(f *testing.F) {
	seed, err := fuzzMeasurementManifestSeed()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed, uint8(0), uint8(0))
	f.Add(seed, uint8(1), uint8(1))
	f.Add([]byte("not-json"), uint8(2), uint8(2))

	f.Fuzz(func(t *testing.T, data []byte, currentDirty, previousDirty uint8) {
		var run measurement.Run
		if err := json.Unmarshal(data, &run); err != nil {
			return
		}
		if err := run.Validate(); err != nil {
			return
		}
		encoded, err := json.Marshal(run)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		var decoded measurement.Run
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if err := decoded.Validate(); err != nil {
			t.Fatalf("round-tripped manifest is invalid: %v", err)
		}
		if !reflect.DeepEqual(decoded, run) {
			t.Fatal("valid manifest changed during JSON round trip")
		}

		current := measurementRunSummary(run)
		previous := current
		previous.RunID = "dogfood-fuzz-previous"
		previous.EndedAt = current.EndedAt.Add(-time.Second)
		current.DirtyState = fuzzDirtyState(currentDirty)
		previous.DirtyState = fuzzDirtyState(previousDirty)
		comparable := comparableMeasurementRuns(current, previous)
		wantComparable := knownMeasurementRunIdentity(current) &&
			knownMeasurementRunIdentity(previous) &&
			current.DirtyState == measurement.DirtyClean &&
			previous.DirtyState == measurement.DirtyClean
		if comparable != wantComparable {
			t.Fatalf("comparableMeasurementRuns() = %v, want %v", comparable, wantComparable)
		}
		dashboard := measurementDashboard([]MeasurementRunSummary{current, previous})
		if (dashboard.PreviousComparable != nil) != wantComparable {
			t.Fatalf("dashboard previous comparable present = %v, want %v", dashboard.PreviousComparable != nil, wantComparable)
		}
		if wantComparable && dashboard.ComparisonState != MeasurementComparisonComparable {
			t.Fatalf("dashboard state = %q, want %q", dashboard.ComparisonState, MeasurementComparisonComparable)
		}
		if !wantComparable && dashboard.ComparisonState == MeasurementComparisonComparable {
			t.Fatal("dashboard exposed an incomparable manifest pair as comparable")
		}
	})
}

func fuzzMeasurementManifestSeed() ([]byte, error) {
	item, err := measurement.NewMeasurement(measurement.MeasurementInput{
		ID:         "fuzz-quality-check",
		Name:       "quality.fuzz.check",
		Category:   measurement.CategoryQuality,
		Status:     measurement.StatusPass,
		Provenance: measurement.ProvenanceMeasured,
		Unit:       "milliseconds",
		Samples:    []float64{1, 2, 3},
		CommandID:  "go.test",
		Required:   true,
	})
	if err != nil {
		return nil, err
	}
	run, err := measurement.NewRun(measurement.Reproducibility{
		RunID:               "dogfood-fuzz-seed",
		Commit:              strings.Repeat("a", 40),
		Head:                strings.Repeat("b", 40),
		DirtyState:          measurement.DirtyClean,
		OS:                  "windows",
		Arch:                "amd64",
		ToolVersions:        map[string]string{"go": "go1.26.7"},
		ConfigurationDigest: measurement.SHA256Digest([]byte("fuzz-config-v1")),
		StartedAt:           time.Date(2026, 8, 31, 8, 0, 0, 0, time.UTC),
		EndedAt:             time.Date(2026, 8, 31, 8, 0, 1, 0, time.UTC),
	}, []measurement.Measurement{item})
	if err != nil {
		return nil, err
	}
	return json.Marshal(run)
}

func fuzzDirtyState(value uint8) measurement.DirtyState {
	switch value % 3 {
	case 0:
		return measurement.DirtyClean
	case 1:
		return measurement.DirtyDirty
	default:
		return measurement.DirtyUnknown
	}
}
