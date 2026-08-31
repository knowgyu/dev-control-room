package app

import (
	"strings"
	"testing"
)

func TestEmbeddedUIMeasurementDashboardContract(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	html := embeddedUIAsset(t, service, "/", "text/html")
	for _, value := range []string{
		`id="assurance-measurement-dashboard"`, `id="assurance-measurement-file"`, `type="file"`,
		`accept="application/json,.json"`, `id="assurance-measurement-empty"`, `aria-live="polite"`,
		"Dogfood Measurement · v1", "실제 측정 대시보드", "측정 manifest 가져오기", "재현성 metadata",
	} {
		if !strings.Contains(html, value) {
			t.Errorf("embedded measurement dashboard HTML missing %q", value)
		}
	}

	javascript := embeddedUIAsset(t, service, "/ui/app.js", "text/javascript")
	for _, value := range []string{
		"assuranceMeasurement", "loadAssuranceMeasurementData", "renderAssuranceMeasurementDashboard",
		"/api/assurance/measurement-runs/dashboard", "/api/assurance/measurement-runs/import",
		"file.size > maximumBytes", "dashboard.nextActions", "measurement-action-list", "item.baseline", "item.delta", "item.unit",
	} {
		if !strings.Contains(javascript, value) {
			t.Errorf("embedded measurement dashboard JavaScript missing %q", value)
		}
	}
	if strings.Contains(javascript, "file.path") || strings.Contains(javascript, "assurance-measurement-path") {
		t.Error("embedded measurement import must not expose a local path API")
	}
	if strings.Contains(javascript, "rawSamples") {
		t.Error("embedded measurement dashboard must not render raw samples")
	}
	if strings.Contains(javascript, "item.Baseline") || strings.Contains(javascript, "item.Delta") || strings.Contains(javascript, "item.Unit") {
		t.Error("embedded measurement dashboard must use JSON field names for measurement fields")
	}

	css := embeddedUIAsset(t, service, "/ui/app.css", "text/css")
	for _, value := range []string{
		".measurement-dashboard", ".measurement-gate", ".measurement-highlights", ".measurement-metric-grid",
		".measurement-comparison-list", ".measurement-action-list", ".measurement-reproducibility",
	} {
		if !strings.Contains(css, value) {
			t.Errorf("embedded measurement dashboard CSS missing %q", value)
		}
	}
}
