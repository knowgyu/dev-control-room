package measurement

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNewMeasurementCopiesSamplesAndValidatesSummary(t *testing.T) {
	samples := []float64{3, 1, 2}
	baseline := 4.0
	delta := -1.0
	exitCode := 0
	item, err := NewMeasurement(MeasurementInput{
		ID:         "quality-go-test",
		Name:       "quality.go.test",
		Category:   CategoryQuality,
		Status:     StatusPass,
		Provenance: ProvenanceMeasured,
		Unit:       "milliseconds",
		Samples:    samples,
		Baseline:   &baseline,
		Delta:      &delta,
		CommandID:  "go.test",
		ExitCode:   &exitCode,
		Required:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	samples[0] = 99
	if item.Spec.RawSamples[0] != 3 {
		t.Fatalf("measurement retained mutable input samples: %v", item.Spec.RawSamples)
	}
	if err := item.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestNewMeasurementAllowsUnavailableUnknownWithoutSamples(t *testing.T) {
	item, err := NewMeasurement(MeasurementInput{
		ID:         "server-health",
		Name:       "performance.http.health.latency",
		Category:   CategoryPerformance,
		Status:     StatusUnknown,
		Provenance: ProvenanceUnavailable,
		Unit:       "milliseconds",
		Samples:    []float64{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.Spec.SampleCount != 0 || len(item.Spec.RawSamples) != 0 || item.Spec.P50 != nil {
		t.Fatalf("unavailable measurement = %#v", item.Spec)
	}
}

func TestNewMeasurementRejectsInvalidSampleData(t *testing.T) {
	tests := []struct {
		name    string
		samples []float64
		status  Status
		want    error
	}{
		{name: "non-finite sample", samples: []float64{1, math.NaN()}, status: StatusPass, want: ErrInvalidSample},
		{name: "unbounded sample set", samples: make([]float64, MaxRawSamples+1), status: StatusPass, want: ErrTooManySamples},
		{name: "pass without evidence", samples: []float64{}, status: StatusPass, want: ErrNoSamples},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewMeasurement(MeasurementInput{
				ID:         "invalid-measurement",
				Name:       "quality.invalid",
				Category:   CategoryQuality,
				Status:     test.status,
				Provenance: ProvenanceMeasured,
				Unit:       "milliseconds",
				Samples:    test.samples,
			})
			if !errors.Is(err, test.want) {
				t.Fatalf("NewMeasurement() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestRunJSONRoundTripAndRequiredGate(t *testing.T) {
	now := time.Date(2026, 8, 31, 1, 2, 3, 0, time.UTC)
	pass, err := NewMeasurement(MeasurementInput{
		ID:         "quality-go-test",
		Name:       "quality.go.test",
		Category:   CategoryQuality,
		Status:     StatusPass,
		Provenance: ProvenanceMeasured,
		Unit:       "milliseconds",
		Samples:    []float64{8, 4, 6},
		Required:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := NewMeasurement(MeasurementInput{
		ID:         "server-state",
		Name:       "performance.http.state.latency",
		Category:   CategoryPerformance,
		Status:     StatusUnknown,
		Provenance: ProvenanceUnavailable,
		Unit:       "milliseconds",
	})
	if err != nil {
		t.Fatal(err)
	}
	digest := SHA256Digest([]byte("fixed configuration"))
	run, err := NewRun(Reproducibility{
		RunID:               "dogfood-run-1",
		Commit:              strings.Repeat("a", 40),
		Head:                strings.Repeat("a", 40),
		DirtyState:          DirtyClean,
		OS:                  "windows",
		Arch:                "amd64",
		ToolVersions:        map[string]string{"go": "go1.26.7", "node": "v24.15.0"},
		ConfigurationDigest: digest,
		StartedAt:           now,
		EndedAt:             now.Add(time.Second),
	}, []Measurement{pass, unknown})
	if err != nil {
		t.Fatal(err)
	}
	if run.Spec.Status != StatusPass || len(run.Spec.RequiredFailures) != 0 {
		t.Fatalf("run gate = %#v", run.Spec)
	}
	encoded, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"kind":"DogfoodMeasurementRun"`) || !strings.Contains(string(encoded), `"kind":"Measurement"`) || !strings.Contains(string(encoded), `"configurationDigest":"`+digest+`"`) {
		t.Fatalf("encoded contract is missing versioned fields: %s", encoded)
	}
	var decoded Run
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, run) {
		t.Fatalf("JSON round trip changed the run:\n got %#v\nwant %#v", decoded, run)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsUnsafeOrInconsistentRecords(t *testing.T) {
	now := time.Date(2026, 8, 31, 1, 2, 3, 0, time.UTC)
	validMeasurement, err := NewMeasurement(MeasurementInput{
		ID:         "quality-check",
		Name:       "quality.check",
		Category:   CategoryQuality,
		Status:     StatusPass,
		Provenance: ProvenanceMeasured,
		Unit:       "milliseconds",
		Samples:    []float64{1},
		Required:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	newRun := func() Run {
		return Run{
			APIVersion: APIVersion,
			Kind:       MeasurementRunKind,
			Metadata:   ObjectMetadata{ID: "dogfood-run-1"},
			Spec: RunSpec{
				Status:           StatusPass,
				RequiredFailures: []string{},
				Reproducibility: Reproducibility{
					RunID:               "dogfood-run-1",
					Commit:              strings.Repeat("a", 40),
					Head:                strings.Repeat("a", 40),
					DirtyState:          DirtyClean,
					OS:                  "windows",
					Arch:                "amd64",
					ToolVersions:        map[string]string{"go": "go1.26.7"},
					ConfigurationDigest: SHA256Digest([]byte("config")),
					StartedAt:           now,
					EndedAt:             now.Add(time.Second),
				},
				Measurements: []Measurement{validMeasurement},
			},
		}
	}
	tests := []struct {
		name   string
		mutate func(*Run)
	}{
		{name: "absolute measurement name", mutate: func(run *Run) { run.Spec.Measurements[0].Spec.Name = `C:\\secret\metric` }},
		{name: "sample count mismatch", mutate: func(run *Run) { run.Spec.Measurements[0].Spec.SampleCount = 2 }},
		{name: "required status mismatch", mutate: func(run *Run) { run.Spec.Status = StatusFail; run.Spec.RequiredFailures = []string{"other"} }},
		{name: "absolute tool version", mutate: func(run *Run) { run.Spec.Reproducibility.ToolVersions["go"] = `C:\\Go\\bin` }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			run := newRun()
			test.mutate(&run)
			if err := run.Validate(); err == nil {
				t.Fatal("unsafe or inconsistent record was accepted")
			}
		})
	}
}
