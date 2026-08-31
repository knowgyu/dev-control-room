package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/masking"
	"github.com/knowgyu/dev-control-room/internal/measurement"
)

func TestMeasurementRunRepositoryPersistsTypedManifestAndRejectsDuplicateID(t *testing.T) {
	database := openTestDatabase(t, "measurement-run-repository")
	persistence, err := New(database, masking.New(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	run := newStoreMeasurementRun(t, "dogfood-store", time.Date(2026, 8, 31, 1, 2, 3, 0, time.UTC), 120)
	ctx := context.Background()
	if err := persistence.SaveMeasurementRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	loaded, err := persistence.GetMeasurementRun(ctx, run.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Metadata.ID != run.Metadata.ID || loaded.Spec.Status != measurement.StatusPass || len(loaded.Spec.Measurements) != 2 {
		t.Fatalf("loaded measurement run = %#v", loaded)
	}
	if err := persistence.SaveMeasurementRun(ctx, run); !errors.Is(err, ErrMeasurementRunDuplicate) {
		t.Fatalf("duplicate import error = %v, want ErrMeasurementRunDuplicate", err)
	}
	items, err := persistence.ListMeasurementRuns(ctx)
	if err != nil || len(items) != 1 || items[0].Metadata.ID != run.Metadata.ID {
		t.Fatalf("measurement runs = %#v, err = %v", items, err)
	}
	var kind string
	if err := database.QueryRowContext(ctx, `SELECT kind FROM assurance_objects WHERE id = ?`, run.Metadata.ID).Scan(&kind); err != nil {
		t.Fatal(err)
	}
	if kind != measurement.MeasurementRunKind {
		t.Fatalf("stored assurance kind = %q", kind)
	}
}

func TestMeasurementRunRepositoryRejectsInvalidTypedManifest(t *testing.T) {
	database := openTestDatabase(t, "measurement-run-invalid")
	persistence, err := New(database, masking.New(nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*measurement.Run)
	}{
		{
			name: "unsupported version",
			mutate: func(run *measurement.Run) {
				run.APIVersion = "devroom/measurement/v0"
			},
		},
		{
			name: "absolute command path",
			mutate: func(run *measurement.Run) {
				run.Spec.Measurements[0].Spec.Command = `C:\private\runner.exe`
			},
		},
		{
			name: "malformed summary",
			mutate: func(run *measurement.Run) {
				wrong := 999.0
				run.Spec.Measurements[0].Spec.P50 = &wrong
			},
		},
		{
			name: "duplicate measurement id",
			mutate: func(run *measurement.Run) {
				run.Spec.Measurements[1].Metadata.ID = run.Spec.Measurements[0].Metadata.ID
			},
		},
		{
			name: "too many measurements",
			mutate: func(run *measurement.Run) {
				for index := len(run.Spec.Measurements); index <= measurement.MaxMeasurements; index++ {
					item := run.Spec.Measurements[0]
					item.Metadata.ID = "extra-" + strings.Repeat("a", index%4+1) + string(rune('a'+index%26))
					run.Spec.Measurements = append(run.Spec.Measurements, item)
				}
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			run := newStoreMeasurementRun(t, "dogfood-"+strings.ReplaceAll(testCase.name, " ", "-"), time.Date(2026, 8, 31, 1, 2, 3, 0, time.UTC), 120)
			testCase.mutate(&run)
			if err := persistence.SaveMeasurementRun(context.Background(), run); err == nil {
				t.Fatal("invalid measurement manifest was persisted")
			}
		})
	}
}

func newStoreMeasurementRun(t *testing.T, id string, endedAt time.Time, duration float64) measurement.Run {
	t.Helper()
	durationMeasurement, err := measurement.NewMeasurement(measurement.MeasurementInput{
		ID: "quality-go-test", Name: "quality.go.test", Category: measurement.CategoryQuality,
		Status: measurement.StatusPass, Provenance: measurement.ProvenanceMeasured, Unit: "milliseconds",
		Samples: []float64{duration, duration + 20}, CommandID: "go.test", Command: "go test -count=1 ./...", ExitCode: intPointer(0), Required: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	optional, err := measurement.NewMeasurement(measurement.MeasurementInput{
		ID: "performance-http-health", Name: "performance.http.health.latency", Category: measurement.CategoryPerformance,
		Status: measurement.StatusUnknown, Provenance: measurement.ProvenanceUnavailable, Unit: "milliseconds",
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := measurement.NewRun(measurement.Reproducibility{
		RunID: id, Commit: strings.Repeat("a", 40), Head: strings.Repeat("b", 40), DirtyState: measurement.DirtyClean,
		OS: "windows", Arch: "amd64", ToolVersions: map[string]string{"go": "go1.23.0", "powershell": "PowerShell 7.5"},
		ConfigurationDigest: measurement.SHA256Digest([]byte("dogfood-config-v1")), StartedAt: endedAt.Add(-time.Minute), EndedAt: endedAt,
	}, []measurement.Measurement{durationMeasurement, optional})
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func intPointer(value int) *int { return &value }
