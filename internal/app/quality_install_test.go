package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/action"
	"github.com/knowgyu/dev-control-room/internal/assurance"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestPlanActionCannotCreateQualityToolInstallPlan(t *testing.T) {
	service, project := actionAdapterFixture(t)
	_, err := service.PlanAction(context.Background(), ActionPlanInput{
		ID:           "quality-tool-generic",
		Name:         "Generic quality tool install",
		ProjectID:    project.Metadata.ID,
		RepositoryID: "repo-1",
		WorktreeID:   "primary",
		ActionType:   domain.QualityToolInstallPythonAction,
		Inputs:       map[string]string{"package": "ruff", "version": "1.0.0"},
	})
	if contract.Classify(err).Code != contract.ErrorInvalidInput {
		t.Fatalf("generic quality tool plan error = %v", err)
	}
}

func TestDedicatedQualityToolInstallPlanPersistsApprovalRequiredAction(t *testing.T) {
	service, project := actionAdapterFixture(t)
	paths := fakeQualityInstallExecutables(t)
	planned, err := service.planQualityToolInstallAction(
		context.Background(),
		QualityToolInstallPlanInput{
			ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary",
			Kind: assurance.QualityToolInstallRuff, Version: "1.0.0", AffectedFiles: []string{"pyproject.toml"}, AllowGlobal: true,
		},
		assurance.Windows11Capability{Available: true, BuildNumber: 22631},
		func(names ...string) string {
			if len(names) == 0 {
				return ""
			}
			switch names[0] {
			case "python.exe":
				return paths.python
			case "node.exe":
				return paths.node
			case "npm.exe":
				return paths.npm
			default:
				return ""
			}
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if planned.Plan.Spec.ActionType != domain.QualityToolInstallPythonAction || planned.Plan.Spec.ApprovalRequired != true || planned.Plan.Spec.PolicyDecision != domain.PolicyApprovalRequired {
		t.Fatalf("quality tool action plan = %#v", planned.Plan.Spec)
	}
	if planned.Preview.Action == nil || planned.Preview.Action.Package != "ruff" || planned.Preview.Action.Version != "1.0.0" {
		t.Fatalf("quality tool preview = %#v", planned.Preview)
	}
	if planned.Plan.Spec.Execution.Executable != planned.Preview.Action.Command.Executable || !reflect.DeepEqual(planned.Plan.Spec.Execution.Arguments, planned.Preview.Action.Command.Arguments) {
		t.Fatalf("persisted command differs from preview: execution=%#v preview=%#v", planned.Plan.Spec.Execution, planned.Preview.Action.Command)
	}
	if planned.Plan.Spec.ToolVersion != "ruff@1.0.0" || !reflect.DeepEqual(planned.Plan.Spec.WritablePaths, planned.Preview.Action.WritablePaths) {
		t.Fatalf("quality tool approval binding = %#v, preview=%#v", planned.Plan.Spec, planned.Preview.Action)
	}
	persisted, err := service.store.GetActionPlan(context.Background(), planned.Plan.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Spec.ActionType != planned.Plan.Spec.ActionType || persisted.Spec.ApprovalRequired != true {
		t.Fatalf("persisted quality tool plan = %#v", persisted.Spec)
	}
}

func TestQualityToolInstallCannotExecuteWithoutHumanApproval(t *testing.T) {
	service, project := actionAdapterFixture(t)
	fakeRunner := &fakeActionProcessRunner{}
	broker, err := action.NewWithRunner(service.store, nil, fakeRunner)
	if err != nil {
		t.Fatal(err)
	}
	service.broker = broker
	paths := fakeQualityInstallExecutables(t)
	planned, err := service.planQualityToolInstallAction(
		context.Background(),
		QualityToolInstallPlanInput{
			ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: "primary",
			Kind: assurance.QualityToolInstallESLint, Version: "9.0.0", AffectedFiles: []string{"package.json"},
		},
		assurance.Windows11Capability{Available: true, BuildNumber: 22631},
		func(names ...string) string {
			if len(names) == 0 {
				return ""
			}
			switch names[0] {
			case "python.exe":
				return paths.python
			case "node.exe":
				return paths.node
			case "npm.exe":
				return paths.npm
			default:
				return ""
			}
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ExecuteAction(context.Background(), planned.Plan.Metadata.ID, "quality-runner", "quality-install-without-approval"); err == nil {
		t.Fatal("quality tool install executed without human approval")
	}
	if fakeRunner.calls != 0 {
		t.Fatalf("package manager process started without approval: %d", fakeRunner.calls)
	}
}

type qualityInstallExecutablePaths struct {
	python string
	node   string
	npm    string
	npmCLI string
}

func fakeQualityInstallExecutables(t *testing.T) qualityInstallExecutablePaths {
	t.Helper()
	root := t.TempDir()
	paths := qualityInstallExecutablePaths{
		python: filepath.Join(root, "python.exe"),
		node:   filepath.Join(root, "node.exe"),
		npm:    filepath.Join(root, "npm.exe"),
		npmCLI: filepath.Join(root, "npm-cli.js"),
	}
	for _, path := range []string{paths.python, paths.node, paths.npm, paths.npmCLI} {
		if err := os.WriteFile(path, []byte("test executable"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return paths
}
