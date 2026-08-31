package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/masking"
	"github.com/knowgyu/dev-control-room/internal/measurement"
)

func TestMeasurementManifestHTTPRejectsMalformedOrUnsafeInput(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	valid := newAppMeasurementRun(t, "dogfood-invalid-base", time.Date(2026, 8, 31, 1, 2, 3, 0, time.UTC), 120)
	cases := []struct {
		name string
		body func() []byte
	}{
		{
			name: "unsupported version",
			body: func() []byte {
				manifest := measurementManifestObject(t, valid)
				manifest["apiVersion"] = "devroom/measurement/v0"
				return marshalMeasurementObject(t, manifest)
			},
		},
		{
			name: "absolute command path",
			body: func() []byte {
				manifest := measurementManifestObject(t, valid)
				measurementObject := measurementManifestMeasurements(manifest)[0]
				measurementObject["spec"].(map[string]any)["command"] = `C:\private\runner.exe`
				return marshalMeasurementObject(t, manifest)
			},
		},
		{
			name: "malformed summary",
			body: func() []byte {
				manifest := measurementManifestObject(t, valid)
				measurementObject := measurementManifestMeasurements(manifest)[0]
				measurementObject["spec"].(map[string]any)["p50"] = 999.0
				return marshalMeasurementObject(t, manifest)
			},
		},
		{
			name: "duplicate measurement id",
			body: func() []byte {
				manifest := measurementManifestObject(t, valid)
				measurements := manifest["spec"].(map[string]any)["measurements"].([]any)
				manifest["spec"].(map[string]any)["measurements"] = append(measurements, measurements[0])
				return marshalMeasurementObject(t, manifest)
			},
		},
		{
			name: "too many measurements",
			body: func() []byte {
				manifest := measurementManifestObject(t, valid)
				measurements := manifest["spec"].(map[string]any)["measurements"].([]any)
				for len(measurements) <= measurement.MaxMeasurements {
					measurements = append(measurements, measurements[0])
				}
				manifest["spec"].(map[string]any)["measurements"] = measurements
				return marshalMeasurementObject(t, manifest)
			},
		},
		{
			name: "unknown output field",
			body: func() []byte {
				manifest := measurementManifestObject(t, valid)
				manifest["rawOutput"] = "secret-canary"
				return marshalMeasurementObject(t, manifest)
			},
		},
		{
			name: "multiple JSON values",
			body: func() []byte {
				return append(measurementManifestJSON(t, valid), []byte("\n{}")...)
			},
		},
		{
			name: "oversized request",
			body: func() []byte {
				return bytes.Repeat([]byte("x"), measurement.MaxManifestBytes+1)
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			response := postMeasurementManifest(t, service, testCase.body())
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			var envelope contract.Envelope[map[string]any]
			if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Error == nil || envelope.Error.Code != contract.ErrorInvalidInput {
				t.Fatalf("error envelope = %#v", envelope)
			}
		})
	}
}

func TestMeasurementImportListGetAndDashboardAPIs(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	prior := newAppMeasurementRun(t, "dogfood-prior", time.Date(2026, 8, 31, 1, 2, 3, 0, time.UTC), 100)
	latest := newAppMeasurementRun(t, "dogfood-latest", time.Date(2026, 8, 31, 2, 2, 3, 0, time.UTC), 300)
	for _, run := range []measurement.Run{prior, latest} {
		response := postMeasurementManifest(t, service, measurementManifestJSON(t, run))
		if response.Code != http.StatusCreated {
			t.Fatalf("import %s status = %d; body = %s", run.Metadata.ID, response.Code, response.Body.String())
		}
		var envelope contract.Envelope[MeasurementRunSummary]
		if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Data == nil || envelope.Data.RunID != run.Metadata.ID {
			t.Fatalf("import response = %#v", envelope)
		}
		assertMeasurementResponseIsSummary(t, envelopeBody(envelope))
	}

	listResponse := getMeasurementEndpoint(t, service, "/api/assurance/measurement-runs")
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d; body = %s", listResponse.Code, listResponse.Body.String())
	}
	var listEnvelope contract.Envelope[[]MeasurementRunSummary]
	if err := json.NewDecoder(listResponse.Body).Decode(&listEnvelope); err != nil {
		t.Fatal(err)
	}
	if listEnvelope.Data == nil || len(*listEnvelope.Data) != 2 {
		t.Fatalf("measurement list = %#v", listEnvelope)
	}
	assertMeasurementResponseIsSummary(t, listResponse.Body.String())

	detailResponse := getMeasurementEndpoint(t, service, "/api/assurance/measurement-runs/"+latest.Metadata.ID)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail status = %d; body = %s", detailResponse.Code, detailResponse.Body.String())
	}
	var detailEnvelope contract.Envelope[MeasurementRunSummary]
	if err := json.NewDecoder(detailResponse.Body).Decode(&detailEnvelope); err != nil {
		t.Fatal(err)
	}
	if detailEnvelope.Data == nil || detailEnvelope.Data.RunID != latest.Metadata.ID {
		t.Fatalf("measurement detail = %#v", detailEnvelope)
	}
	assertMeasurementResponseIsSummary(t, detailResponse.Body.String())

	dashboardResponse := getMeasurementEndpoint(t, service, "/api/assurance/measurement-runs/dashboard")
	if dashboardResponse.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d; body = %s", dashboardResponse.Code, dashboardResponse.Body.String())
	}
	var dashboardEnvelope contract.Envelope[MeasurementDashboard]
	if err := json.NewDecoder(dashboardResponse.Body).Decode(&dashboardEnvelope); err != nil {
		t.Fatal(err)
	}
	if dashboardEnvelope.Data == nil || dashboardEnvelope.Data.Latest == nil || dashboardEnvelope.Data.Latest.RunID != latest.Metadata.ID || dashboardEnvelope.Data.PreviousComparable == nil || dashboardEnvelope.Data.PreviousComparable.RunID != prior.Metadata.ID {
		t.Fatalf("measurement dashboard = %#v", dashboardEnvelope)
	}
	if dashboardEnvelope.Data.ComparisonState != MeasurementComparisonComparable || len(dashboardEnvelope.Data.Comparisons) != 2 {
		t.Fatalf("measurement comparison = %#v", dashboardEnvelope.Data)
	}
	if dashboardEnvelope.Data.Comparisons[0].DeltaP50 == nil || *dashboardEnvelope.Data.Comparisons[0].DeltaP50 <= 0 {
		t.Fatalf("measurement p50 delta = %#v", dashboardEnvelope.Data.Comparisons[0])
	}
	if !measurementActionPresent(dashboardEnvelope.Data.NextActions, "unavailable_probe") || !measurementActionPresent(dashboardEnvelope.Data.NextActions, "regression_comparable_metric") {
		t.Fatalf("measurement next actions = %#v", dashboardEnvelope.Data.NextActions)
	}
	assertMeasurementResponseIsSummary(t, dashboardResponse.Body.String())

	duplicate := postMeasurementManifest(t, service, measurementManifestJSON(t, latest))
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d; body = %s", duplicate.Code, duplicate.Body.String())
	}
	missing := getMeasurementEndpoint(t, service, "/api/assurance/measurement-runs/missing-run")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d; body = %s", missing.Code, missing.Body.String())
	}
}

func TestMeasurementImportPreservesUnknownAndUnavailableSeparately(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	run := newAppMeasurementRun(t, "dogfood-unknown", time.Date(2026, 8, 31, 3, 2, 3, 0, time.UTC), 120)
	response := postMeasurementManifest(t, service, measurementManifestJSON(t, run))
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	var envelope contract.Envelope[MeasurementRunSummary]
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data == nil {
		t.Fatal("import response has no data")
	}
	var found *MeasurementSummary
	for index := range envelope.Data.Measurements {
		if envelope.Data.Measurements[index].ID == "performance-http-health" {
			found = &envelope.Data.Measurements[index]
			break
		}
	}
	if found == nil || found.Status != measurement.StatusUnknown || found.Provenance != measurement.ProvenanceUnavailable || found.P50 != nil {
		t.Fatalf("optional measurement summary = %#v", found)
	}
}

func TestMeasurementImportMasksStoredMetadataBeforeResponse(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	secret := "measurement-secret-canary"
	service.masker = masking.New([]string{secret}, nil)
	run := newAppMeasurementRun(t, "dogfood-masked", time.Date(2026, 8, 31, 4, 2, 3, 0, time.UTC), 120)
	run.Spec.Reproducibility.ToolVersions["runner"] = secret
	response := postMeasurementManifest(t, service, measurementManifestJSON(t, run))
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), secret) {
		t.Fatalf("secret leaked in import response: %s", response.Body.String())
	}
	detail := getMeasurementEndpoint(t, service, "/api/assurance/measurement-runs/"+run.Metadata.ID)
	if strings.Contains(detail.Body.String(), secret) {
		t.Fatalf("secret leaked in detail response: %s", detail.Body.String())
	}
}

func newAppMeasurementRun(t *testing.T, id string, endedAt time.Time, duration float64) measurement.Run {
	t.Helper()
	durationMeasurement, err := measurement.NewMeasurement(measurement.MeasurementInput{
		ID: "quality-go-test", Name: "quality.go.test", Category: measurement.CategoryQuality,
		Status: measurement.StatusPass, Provenance: measurement.ProvenanceMeasured, Unit: "milliseconds",
		Samples: []float64{duration, duration + 20}, CommandID: "go.test", Command: "go test -count=1 ./...", ExitCode: appIntPointer(0), Required: true,
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

func postMeasurementManifest(t *testing.T, service *App, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/assurance/measurement-runs/import", bytes.NewReader(body))
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	return recorder
}

func getMeasurementEndpoint(t *testing.T, service *App, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func measurementManifestJSON(t *testing.T, run measurement.Run) []byte {
	t.Helper()
	encoded, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func measurementManifestObject(t *testing.T, run measurement.Run) map[string]any {
	t.Helper()
	var object map[string]any
	if err := json.Unmarshal(measurementManifestJSON(t, run), &object); err != nil {
		t.Fatal(err)
	}
	return object
}

func measurementManifestMeasurements(manifest map[string]any) []map[string]any {
	raw := manifest["spec"].(map[string]any)["measurements"].([]any)
	items := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		items = append(items, item.(map[string]any))
	}
	return items
}

func marshalMeasurementObject(t *testing.T, object map[string]any) []byte {
	t.Helper()
	encoded, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func measurementActionPresent(actions []MeasurementNextAction, code string) bool {
	for _, action := range actions {
		if action.Code == code {
			return true
		}
	}
	return false
}

func assertMeasurementResponseIsSummary(t *testing.T, body string) {
	t.Helper()
	if strings.Contains(body, "rawSamples") || strings.Contains(body, "go test -count=1 ./...") || strings.Contains(body, "secret-canary") {
		t.Fatalf("measurement response exposed non-summary content: %s", body)
	}
}

func envelopeBody[T any](envelope contract.Envelope[T]) string {
	encoded, _ := json.Marshal(envelope)
	return string(encoded)
}

func appIntPointer(value int) *int { return &value }
