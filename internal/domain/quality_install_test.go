package domain

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestQualityToolInstallDefinitionsUseFixedReviewedExecution(t *testing.T) {
	tests := []struct {
		name       string
		actionType string
		inputs     map[string]string
		want       ActionExecution
	}{
		{
			name:       "python",
			actionType: QualityToolInstallPythonAction,
			inputs:     map[string]string{"package": "ruff", "version": "0.6.9"},
			want:       ActionExecution{Executable: "python.exe", Arguments: []string{"-m", "pip", "install", "--disable-pip-version-check", "--no-input", "ruff==0.6.9"}, TimeoutSeconds: 300, MaxOutputBytes: 64 << 10},
		},
		{
			name:       "node",
			actionType: QualityToolInstallNodeAction,
			inputs:     map[string]string{"package": "eslint", "version": "9.0.0"},
			want:       ActionExecution{Executable: "npm.exe", Arguments: []string{"install", "--save-dev", "--ignore-scripts", "--no-audit", "--no-fund", "eslint@9.0.0"}, TimeoutSeconds: 300, MaxOutputBytes: 64 << 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definition, ok := ActionDefinitionFor(tt.actionType)
			if !ok || definition.Risk != RiskExternalChange || definition.PolicyDecision != PolicyApprovalRequired || !definition.ApprovalRequired {
				t.Fatalf("definition = %#v, found = %v", definition, ok)
			}
			execution, err := definition.ExecutionFor(tt.inputs)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(execution, tt.want) {
				t.Fatalf("execution = %#v, want %#v", execution, tt.want)
			}
		})
	}
}

func TestQualityToolInstallExecutionRejectsUnreviewedInputs(t *testing.T) {
	definition, ok := ActionDefinitionFor(QualityToolInstallPythonAction)
	if !ok {
		t.Fatal("python quality tool install definition is missing")
	}
	tests := []map[string]string{
		{"package": "pip", "version": "1.0.0"},
		{"package": "ruff", "version": "latest"},
		{"package": "ruff", "version": "1.0.0", "argv": "arbitrary"},
	}
	for _, inputs := range tests {
		if _, err := definition.ExecutionFor(inputs); err == nil {
			t.Fatalf("unreviewed inputs accepted: %#v", inputs)
		}
	}
}

func TestQualityToolInstallActionPlanBindsAffectedFilesToWritablePaths(t *testing.T) {
	definition, ok := ActionDefinitionFor(QualityToolInstallPythonAction)
	if !ok {
		t.Fatal("python quality tool install definition is missing")
	}
	inputs := map[string]string{
		"package":          "ruff",
		"version":          "1.0.0",
		"executable":       `C:\\Python\\python.exe`,
		"environmentScope": "project",
	}
	affected, err := json.Marshal([]string{"pyproject.toml"})
	if err != nil {
		t.Fatal(err)
	}
	inputs["affectedFiles"] = string(affected)
	execution, err := definition.ExecutionFor(inputs)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	plan := ActionPlan{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: ActionPlanKind},
		Metadata: ObjectMeta{ID: "quality-install-plan", Name: "Quality install"},
		Spec: ActionPlanSpec{
			ProjectID: "project-a", RepositoryID: "repo-a", WorktreeID: "primary",
			ActionType: QualityToolInstallPythonAction, Risk: definition.Risk, Inputs: inputs,
			Execution: execution, ExecutionContext: WorktreeExecutionContext{ProjectID: "project-a", RepositoryID: "repo-a", WorktreeID: "primary", CanonicalPath: `C:\\fixture`, PathFingerprint: "sha256:path", Head: "head-1", Branch: "main"},
			Prechecks: definition.Prechecks, Postchecks: definition.Postchecks, PolicyDecision: definition.PolicyDecision, ApprovalRequired: definition.ApprovalRequired,
			RequestedBy: Actor{Kind: ActorSystem, ID: "system"}, RequestedAt: now,
			ToolVersion: "ruff@1.0.0", ToolConfigDigest: "sha256:" + strings.Repeat("a", 64), WritablePaths: []string{`C:\\fixture\\pyproject.toml`},
		},
	}
	if err := plan.Validate(); err != nil {
		t.Fatal(err)
	}

	plan.Spec.WritablePaths[0] = `C:\\fixture\\other.toml`
	if err := plan.Validate(); err == nil {
		t.Fatal("action plan accepted a writable path unrelated to affectedFiles")
	}
	plan.Spec.WritablePaths[0] = `C:\\fixture\\pyproject.toml`
	plan.Spec.Inputs["affectedFiles"] = ` ["..\\outside.txt"] `
	if err := plan.Validate(); err == nil {
		t.Fatal("action plan accepted an affected file outside the worktree")
	}
}
