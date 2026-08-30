package assurance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const qualityToolProbeMarkerEnv = "DCR_QUALITY_TOOL_PROBE_MARKER"

func TestMain(m *testing.M) {
	if marker := os.Getenv(qualityToolProbeMarkerEnv); marker != "" {
		if err := os.WriteFile(marker, []byte("quality tool was executed"), 0600); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestQualityToolInspectorReportsAvailableMissingAndCapabilityStates(t *testing.T) {
	checkedAt := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	paths := map[string]string{
		QualityRunnerGoToolID:       `C:\tools\go.exe`,
		QualityRunnerMutationToolID: ".",
		QualityToolGovulncheckID:    "",
		QualityToolStaticcheckID:    `relative\staticcheck.exe`,
		QualityToolGosecID:          "",
		QualityToolGitleaksID:       `C:\tools\gitleaks.exe`,
	}
	inspector := QualityToolInspector{
		lookPath: func(name string) (string, error) {
			path, ok := paths[name]
			if !ok || path == "" {
				return "", errors.New("tool is missing")
			}
			return path, nil
		},
		now: func() time.Time { return checkedAt },
	}

	model, err := inspector.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if model.CheckedAt.IsZero() || len(model.Tools) == 0 || len(model.Capabilities) == 0 {
		t.Fatalf("quality tools model = %#v", model)
	}

	toolTests := []struct {
		name       string
		id         string
		state      QualityToolState
		path       string
		reasonCode string
	}{
		{name: "go candidate is unverified", id: QualityRunnerGoToolID, state: QualityToolStatePresentUnverified, path: `C:\tools\go.exe`, reasonCode: QualityToolReasonPresentUnverified},
		{name: "dot candidate is unverified", id: QualityRunnerMutationToolID, state: QualityToolStatePresentUnverified, path: ".", reasonCode: QualityToolReasonPresentUnverified},
		{name: "empty lookup is missing", id: QualityToolGovulncheckID, state: QualityToolStateMissing, reasonCode: QualityToolReasonMissing},
		{name: "relative candidate is unverified", id: QualityToolStaticcheckID, state: QualityToolStatePresentUnverified, path: `relative\staticcheck.exe`, reasonCode: QualityToolReasonPresentUnverified},
		{name: "lookup error is missing", id: QualityToolGosecID, state: QualityToolStateMissing, reasonCode: QualityToolReasonMissing},
		{name: "gitleaks candidate is unverified", id: QualityToolGitleaksID, state: QualityToolStatePresentUnverified, path: `C:\tools\gitleaks.exe`, reasonCode: QualityToolReasonPresentUnverified},
	}
	for _, test := range toolTests {
		t.Run(test.name, func(t *testing.T) {
			status, ok := findQualityTool(model, test.id)
			if !ok {
				t.Fatalf("tool %q was not reported", test.id)
			}
			if status.State != test.state || status.DiscoveryState != test.state || status.Path != test.path ||
				status.ExecutionReadiness != QualityExecutionReadinessNotEvaluated || status.Version != "" ||
				status.CheckedAt != model.CheckedAt || status.Reason == "" || status.ReasonCode != test.reasonCode ||
				!containsQualityReasonCode(status.ReasonCodes, test.reasonCode) ||
				!containsQualityReasonCode(status.ReasonCodes, QualityExecutionReasonNotEvaluated) ||
				status.InstallGuidance == "" {
				t.Fatalf("tool %q = %#v, want state %q", test.id, status, test.state)
			}
		})
	}

	capabilityTests := []struct {
		name       string
		id         string
		state      QualityCapabilityState
		runnerID   string
		reasonCode string
	}{
		{name: "go test is not registered", id: "go_test", state: QualityCapabilityStateInstalledNotRegistered, reasonCode: QualityRunnerReasonUnregistered},
		{name: "go vet is unverified", id: "go_vet", state: QualityCapabilityStatePresentUnverified, runnerID: QualityRunnerGoVetID, reasonCode: QualityToolReasonPresentUnverified},
		{name: "coverage is unverified", id: "go_coverage", state: QualityCapabilityStatePresentUnverified, runnerID: QualityRunnerGoTestCoverageID, reasonCode: QualityToolReasonPresentUnverified},
		{name: "mutation is unverified", id: "mutation", state: QualityCapabilityStatePresentUnverified, runnerID: QualityRunnerGoMutationID, reasonCode: QualityToolReasonPresentUnverified},
		{name: "property needs target", id: "property", state: QualityCapabilityStateNeedsTarget, runnerID: QualityRunnerGoPropertyID, reasonCode: QualityTargetReasonNotEvaluated},
		{name: "fuzz needs target", id: "fuzz", state: QualityCapabilityStateNeedsTarget, runnerID: QualityRunnerGoFuzzID, reasonCode: QualityTargetReasonNotEvaluated},
		{name: "targeted e2e needs target", id: "targeted_e2e", state: QualityCapabilityStateNeedsTarget, runnerID: QualityRunnerGoE2EID, reasonCode: QualityTargetReasonNotEvaluated},
		{name: "govulncheck is missing", id: "govulncheck", state: QualityCapabilityStateMissing, reasonCode: QualityToolReasonMissing},
		{name: "staticcheck is not registered", id: "staticcheck", state: QualityCapabilityStateInstalledNotRegistered, reasonCode: QualityRunnerReasonUnregistered},
		{name: "gosec is missing", id: "gosec", state: QualityCapabilityStateMissing, reasonCode: QualityToolReasonMissing},
		{name: "gitleaks is not registered", id: "gitleaks", state: QualityCapabilityStateInstalledNotRegistered, reasonCode: QualityRunnerReasonUnregistered},
	}
	for _, test := range capabilityTests {
		t.Run(test.name, func(t *testing.T) {
			status, ok := findQualityCapability(model, test.id)
			if !ok {
				t.Fatalf("capability %q was not reported", test.id)
			}
			if status.State != test.state || status.RunnerID != test.runnerID ||
				status.ExecutionReadiness != QualityExecutionReadinessNotEvaluated || status.CheckedAt != model.CheckedAt ||
				status.Reason == "" || status.ReasonCode != test.reasonCode ||
				!containsQualityReasonCode(status.ReasonCodes, test.reasonCode) ||
				!containsQualityReasonCode(status.ReasonCodes, QualityExecutionReasonNotEvaluated) ||
				status.InstallGuidance == "" {
				t.Fatalf("capability %q = %#v, want state %q", test.id, status, test.state)
			}
		})
	}
}

func TestQualityToolInspectorRejectsUntrustedExecutablePath(t *testing.T) {
	inspector := QualityToolInspector{
		lookPath: func(name string) (string, error) {
			if name == QualityRunnerGoToolID {
				return name, nil
			}
			return "", errors.New("tool is missing")
		},
	}

	model, err := inspector.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	tool, ok := findQualityTool(model, QualityRunnerGoToolID)
	if !ok || tool.State != QualityToolStatePresentUnverified || tool.Path != QualityRunnerGoToolID || tool.Version != "" {
		t.Fatalf("unverified Go candidate = %#v", tool)
	}
	capability, ok := findQualityCapability(model, "go_test")
	if !ok || capability.State != QualityCapabilityStateInstalledNotRegistered || capability.RunnerID != "" {
		t.Fatalf("unregistered Go capability = %#v", capability)
	}
}

func TestQualityToolInspectorDoesNotExecuteDiscoveredPath(t *testing.T) {
	toolDir := t.TempDir()
	markerPath := filepath.Join(toolDir, "probe.marker")
	fakeGoPath := filepath.Join(toolDir, QualityRunnerGoToolID)

	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(testBinary)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fakeGoPath, binary, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(fakeGoPath, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", toolDir)
	t.Setenv(qualityToolProbeMarkerEnv, markerPath)

	model, err := NewQualityToolInspector().Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	tool, ok := findQualityTool(model, QualityRunnerGoToolID)
	if !ok {
		t.Fatal("Go tool was not reported")
	}
	if tool.State != QualityToolStatePresentUnverified || tool.Version != "" || tool.ExecutionReadiness != QualityExecutionReadinessNotEvaluated {
		t.Fatalf("discovered Go tool = %#v", tool)
	}
	if _, err := os.Stat(markerPath); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			t.Fatal("quality tools discovery executed the discovered Go candidate")
		}
		t.Fatalf("probe marker stat = %v", err)
	}
}

func TestQualityToolCapabilitiesLinkOnlyToRegisteredQualityRunners(t *testing.T) {
	model, err := (QualityToolInspector{
		lookPath: func(string) (string, error) { return `C:\tools\candidate.exe`, nil },
	}).Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for _, definition := range qualityCapabilityDefinitions {
		t.Run(definition.ID, func(t *testing.T) {
			capability, ok := findQualityCapability(model, definition.ID)
			if !ok {
				t.Fatalf("capability %q was not reported", definition.ID)
			}
			registration, registered := qualityRunnerRegistry[definition.TechniqueID]
			if registered {
				if capability.RunnerID != registration.definition.RunnerID || capability.ToolID != registration.definition.ToolID {
					t.Fatalf("capability %q = %#v, registry = %#v", definition.ID, capability, registration.definition)
				}
				return
			}
			if capability.RunnerID != "" {
				t.Fatalf("unregistered capability %q has runner ID %q", definition.ID, capability.RunnerID)
			}
		})
	}
}

func containsQualityReasonCode(codes []string, want string) bool {
	for _, code := range codes {
		if code == want {
			return true
		}
	}
	return false
}

func findQualityTool(model QualityToolsReadModel, id string) (QualityToolStatus, bool) {
	for _, status := range model.Tools {
		if status.ID == id {
			return status, true
		}
	}
	return QualityToolStatus{}, false
}

func findQualityCapability(model QualityToolsReadModel, id string) (QualityCapabilityStatus, bool) {
	for _, status := range model.Capabilities {
		if status.ID == id {
			return status, true
		}
	}
	return QualityCapabilityStatus{}, false
}
