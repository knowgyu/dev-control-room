package measurement

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
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
		{name: "embedded Windows command path", mutate: func(run *Run) { run.Spec.Measurements[0].Spec.Command = `go test C:\\Users\\Alice\\repo` }},
		{name: "embedded UNC command path", mutate: func(run *Run) { run.Spec.Measurements[0].Spec.Command = `go test \\server\share` }},
		{name: "embedded Unix command path", mutate: func(run *Run) { run.Spec.Measurements[0].Spec.Command = `go test /Users/Alice/repo` }},
		{name: "embedded Windows tool version path", mutate: func(run *Run) { run.Spec.Reproducibility.ToolVersions["go"] = `go1.26.7 C:\\Go\\bin` }},
		{name: "embedded UNC tool version path", mutate: func(run *Run) { run.Spec.Reproducibility.ToolVersions["go"] = `go1.26.7 \\server\go` }},
		{name: "embedded Unix tool version path", mutate: func(run *Run) { run.Spec.Reproducibility.ToolVersions["go"] = `go1.26.7 /usr/local/go` }},
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

func TestSafeTextAllowsVersionAndHTTPPathTokens(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "normal Go version", value: "go version go1.26.7 windows/amd64"},
		{name: "fixed HTTP endpoint command", value: "GET /api/health (probe disabled)"},
		{name: "relative Go package path", value: "go test ./..."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !validSafeText(test.value, 256) {
				t.Fatalf("validSafeText(%q) = false", test.value)
			}
		})
	}
}

func TestMeasurementManifestRoundTripProperty(t *testing.T) {
	random := rand.New(rand.NewSource(20260831))
	categories := []Category{CategoryQuality, CategoryPerformance, CategoryProcess, CategoryRuntime}
	statuses := []Status{StatusPass, StatusFail, StatusUnknown}
	provenances := []Provenance{ProvenanceMeasured, ProvenanceEstimated, ProvenanceInferred}
	for iteration := 0; iteration < 64; iteration++ {
		t.Run(fmt.Sprintf("case-%02d", iteration), func(t *testing.T) {
			measurementCount := 1 + random.Intn(5)
			items := make([]Measurement, 0, measurementCount)
			for index := 0; index < measurementCount; index++ {
				samples := make([]float64, random.Intn(9))
				for sampleIndex := range samples {
					samples[sampleIndex] = float64(random.Intn(10000))/100 + float64(sampleIndex)/10
				}
				status := statuses[random.Intn(len(statuses))]
				provenance := provenances[random.Intn(len(provenances))]
				if len(samples) == 0 {
					status = StatusUnknown
					provenance = ProvenanceUnavailable
				}
				item, err := NewMeasurement(MeasurementInput{
					ID:         fmt.Sprintf("property-measurement-%d-%d", iteration, index),
					Name:       fmt.Sprintf("property.%s.%d.%d", categories[index%len(categories)], iteration, index),
					Category:   categories[index%len(categories)],
					Status:     status,
					Provenance: provenance,
					Unit:       "milliseconds",
					Samples:    samples,
					CommandID:  "property.check",
					Required:   random.Intn(2) == 0,
				})
				if err != nil {
					t.Fatalf("NewMeasurement() error = %v", err)
				}
				items = append(items, item)
			}

			now := time.Date(2026, 8, 31, 7, 0, 0, 0, time.UTC).Add(time.Duration(iteration) * time.Minute)
			run, err := NewRun(Reproducibility{
				RunID:               fmt.Sprintf("dogfood-property-%02d", iteration),
				Commit:              strings.Repeat("a", 40),
				Head:                strings.Repeat("b", 40),
				DirtyState:          DirtyClean,
				OS:                  "windows",
				Arch:                "amd64",
				ToolVersions:        map[string]string{"go": "go1.26.7", "powershell": "PowerShell 7.6.4"},
				ConfigurationDigest: SHA256Digest([]byte("property-config-v1")),
				StartedAt:           now,
				EndedAt:             now.Add(time.Second),
			}, items)
			if err != nil {
				t.Fatalf("NewRun() error = %v", err)
			}
			encoded, err := json.Marshal(run)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			var decoded Run
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if err := decoded.Validate(); err != nil {
				t.Fatalf("round-tripped manifest is invalid: %v", err)
			}
			if !reflect.DeepEqual(decoded, run) {
				t.Fatalf("round trip changed the manifest:\n got %#v\nwant %#v", decoded, run)
			}
			status, failures := requiredStatus(decoded.Spec.Measurements)
			if decoded.Spec.Status != status || !sameStrings(decoded.Spec.RequiredFailures, failures) {
				t.Fatalf("required gate is not derived deterministically: status=%q failures=%v", decoded.Spec.Status, decoded.Spec.RequiredFailures)
			}

			invalid := decoded
			invalid.Spec.Measurements = append([]Measurement{}, decoded.Spec.Measurements...)
			invalid.Spec.Measurements[0].Spec.SampleCount++
			if err := invalid.Validate(); err == nil {
				t.Fatal("manifest with an inconsistent sample count was accepted")
			}
		})
	}
}
