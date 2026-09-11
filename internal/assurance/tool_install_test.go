package assurance

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBuildQualityToolInstallActionUsesExplicitTypedVersionAndAffectedFiles(t *testing.T) {
	root := adapterFixtureRoot(t)
	python := adapterExecutable(t, root, "python.exe")
	action, err := BuildQualityToolInstallAction(QualityToolInstallRequest{Kind: QualityToolInstallRuff, ComponentID: "component-root", WorktreeRoot: root, ComponentRoot: root, InterpreterPath: python, Version: "0.6.9", AffectedFiles: []string{"pyproject.toml", "requirements.txt"}}, availableWindows11(context.Background()))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-m", "pip", "install", "--disable-pip-version-check", "--no-input", "ruff==0.6.9"}
	if !reflect.DeepEqual(action.Command.Arguments, want) || !reflect.DeepEqual(action.AffectedFiles, []string{"pyproject.toml", "requirements.txt"}) {
		t.Fatalf("install action = %#v", action)
	}
	for _, value := range action.Command.Arguments {
		if value == "cmd.exe" || filepath.Ext(value) == ".cmd" || filepath.Ext(value) == ".bat" {
			t.Fatalf("shell surface in install action: %#v", action.Command)
		}
	}
}

func TestBuildQualityToolInstallActionValidatesNodeAndBrokerIsTheOnlyExecutor(t *testing.T) {
	root := adapterFixtureRoot(t)
	node := adapterExecutable(t, root, "node.exe")
	npm := adapterExecutable(t, root, "npm.cmd")
	npmCLI := filepath.Join(root, "npm-cli.js")
	if err := os.WriteFile(npmCLI, []byte("// reviewed npm entry point"), 0o600); err != nil {
		t.Fatal(err)
	}
	action, err := BuildQualityToolInstallAction(QualityToolInstallRequest{Kind: QualityToolInstallESLint, ComponentID: "component-root", WorktreeRoot: root, ComponentRoot: root, NodePath: node, NPMPath: npm, Version: "9.0.0", AffectedFiles: []string{"package.json", "package-lock.json"}}, availableWindows11(context.Background()))
	if err != nil {
		t.Fatal(err)
	}
	if action.Command.Executable != node || action.Command.Arguments[0] != npmCLI || !containsString(action.Command.Arguments, "--ignore-scripts") || action.Package != "eslint" {
		t.Fatalf("Node install action = %#v", action)
	}
	broker := &recordingInstallBroker{}
	result, err := (QualityToolInstallExecutor{Broker: broker}).Execute(context.Background(), action)
	if err != nil || result.Status != "approved-executed" || broker.calls != 1 {
		t.Fatalf("broker execution = %#v, err=%v, calls=%d", result, err, broker.calls)
	}
}

func TestBuildQualityToolInstallActionRejectsImplicitOrUnsafeInstallation(t *testing.T) {
	root := adapterFixtureRoot(t)
	python := adapterExecutable(t, root, "python.exe")
	base := QualityToolInstallRequest{Kind: QualityToolInstallPytest, ComponentID: "component-root", WorktreeRoot: root, ComponentRoot: root, InterpreterPath: python, Version: "latest", AffectedFiles: []string{"requirements.txt"}}
	if _, err := BuildQualityToolInstallAction(base, availableWindows11(context.Background())); err == nil {
		t.Fatal("implicit version accepted")
	}
	base.Version = "8.3.2"
	base.AffectedFiles = []string{"..\\outside.txt"}
	if _, err := BuildQualityToolInstallAction(base, availableWindows11(context.Background())); err == nil {
		t.Fatal("escaping affected file accepted")
	}
	if _, err := BuildQualityToolInstallAction(base, Windows11Capability{}); err == nil {
		t.Fatal("non-Windows capability accepted")
	}
}

type recordingInstallBroker struct{ calls int }

func (b *recordingInstallBroker) ExecuteApprovedQualityToolInstall(_ context.Context, action QualityToolInstallAction) (QualityToolInstallResult, error) {
	b.calls++
	return QualityToolInstallResult{Action: action, Status: "approved-executed"}, nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
