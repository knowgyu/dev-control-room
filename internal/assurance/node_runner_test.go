package assurance

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNodeRunnerUsesProjectLocalScriptsAndOneShotVitest(t *testing.T) {
	root := adapterFixtureRoot(t)
	node := adapterExecutable(t, root, "node.exe")
	writeNodePackage(t, root, "eslint", "9.0.0", `{"eslint":"./bin/eslint.js"}`, "bin/eslint.js")
	writeNodePackage(t, root, "vitest", "2.0.0", `{"vitest":"./vitest.mjs"}`, "vitest.mjs")
	runner := NodeQualityRunner{}
	eslint, err := runner.SelectESLint(QualityAdapterRequest{WorktreeRoot: root, NodePath: node, ToolVersion: "9.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if eslint.Command.Executable != node || len(eslint.Command.Arguments) != 4 || eslint.Command.Arguments[1] != "--format" || eslint.Command.Arguments[2] != "json" || eslint.Command.Arguments[3] != "." {
		t.Fatalf("ESLint command = %#v", eslint.Command)
	}
	vitest, err := runner.SelectVitest(QualityAdapterRequest{WorktreeRoot: root, NodePath: node, ToolVersion: "2.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vitest.Command.Arguments) != 3 || vitest.Command.Arguments[1] != "run" || vitest.Command.Arguments[2] != "--reporter=json" {
		t.Fatalf("Vitest command = %#v", vitest.Command)
	}
}

func TestNodeRunnerParsesESLintAndVitestJSON(t *testing.T) {
	eslint, _, err := ParseESLintJSON([]byte(`[{"filePath":"src/app.ts","messages":[{"ruleId":"no-secret","message":"API_KEY=hidden","severity":2,"line":3,"column":4}]}]`), nil)
	if err != nil || len(eslint) != 1 || eslint[0].Severity != "error" || eslint[0].Message != "API_KEY=[REDACTED]" {
		t.Fatalf("ESLint findings = %#v, err=%v", eslint, err)
	}
	summary, err := ParseVitestJSON([]byte(`{"numTotalTests":3,"numPassedTests":2,"numFailedTests":1,"numPendingTests":0,"testResults":[]}`))
	if err != nil || summary.Total != 3 || summary.Failed != 1 {
		t.Fatalf("Vitest summary = %#v, err=%v", summary, err)
	}
	if _, err := ParseVitestJSON([]byte(`{"numTotalTests":1}`)); err == nil {
		t.Fatal("partial Vitest report accepted")
	}
}

func TestNodeRunnerParsesExpectedNonzeroProcessExits(t *testing.T) {
	root := adapterFixtureRoot(t)
	node := adapterExecutable(t, root, "node.exe")
	writeNodePackage(t, root, "eslint", "9.0.0", `{"eslint":"./bin/eslint.js"}`, "bin/eslint.js")
	writeNodePackage(t, root, "vitest", "2.0.0", `{"vitest":"./vitest.mjs"}`, "vitest.mjs")
	request := QualityAdapterRequest{WorktreeRoot: root, NodePath: node, ToolVersion: "9.0.0"}

	var eslintProcessErr error
	eslint := &helperQualityProcess{kind: "eslint", processErr: &eslintProcessErr}
	eslintRunner := NodeQualityRunner{Capability: availableWindows11, Process: eslint}
	eslintResult := eslintRunner.RunESLint(context.Background(), request)
	assertExpectedProcessExit(t, eslintProcessErr)
	if eslintResult.Outcome != QualityOutcomeFindings || len(eslintResult.Findings) != 1 || eslintResult.ExitCode != 1 {
		t.Fatalf("ESLint result = %#v", eslintResult)
	}

	var vitestProcessErr error
	vitest := &helperQualityProcess{kind: "vitest", processErr: &vitestProcessErr}
	vitestRunner := NodeQualityRunner{Capability: availableWindows11, Process: vitest}
	vitestResult := vitestRunner.RunVitest(context.Background(), QualityAdapterRequest{WorktreeRoot: root, NodePath: node, ToolVersion: "2.0.0"})
	assertExpectedProcessExit(t, vitestProcessErr)
	if vitestResult.Outcome != QualityOutcomeTestsFailed || vitestResult.ExitCode != 1 {
		t.Fatalf("Vitest result = %#v", vitestResult)
	}
}

func TestNodeRunnerRejectsUnexpectedNonzeroExitWithParseableOutput(t *testing.T) {
	root := adapterFixtureRoot(t)
	node := adapterExecutable(t, root, "node.exe")
	writeNodePackage(t, root, "eslint", "9.0.0", `{"eslint":"./bin/eslint.js"}`, "bin/eslint.js")
	writeNodePackage(t, root, "vitest", "2.0.0", `{"vitest":"./vitest.mjs"}`, "vitest.mjs")

	var eslintProcessErr error
	eslintRunner := NodeQualityRunner{
		Capability: availableWindows11,
		Process:    &helperQualityProcess{kind: "eslint", exitCode: 2, processErr: &eslintProcessErr},
	}
	eslintResult := eslintRunner.RunESLint(context.Background(), QualityAdapterRequest{WorktreeRoot: root, NodePath: node, ToolVersion: "9.0.0"})
	assertExpectedProcessExitCode(t, eslintProcessErr, 2)
	if eslintResult.Outcome != QualityOutcomeToolError || len(eslintResult.Findings) != 0 || eslintResult.ExitCode != 2 {
		t.Fatalf("unexpected ESLint exit result = %#v", eslintResult)
	}

	var vitestProcessErr error
	vitestRunner := NodeQualityRunner{
		Capability: availableWindows11,
		Process:    &helperQualityProcess{kind: "vitest", exitCode: 2, processErr: &vitestProcessErr},
	}
	vitestResult := vitestRunner.RunVitest(context.Background(), QualityAdapterRequest{WorktreeRoot: root, NodePath: node, ToolVersion: "2.0.0"})
	assertExpectedProcessExitCode(t, vitestProcessErr, 2)
	if vitestResult.Outcome != QualityOutcomeToolError || vitestResult.ExitCode != 2 {
		t.Fatalf("unexpected Vitest exit result = %#v", vitestResult)
	}
}

func writeNodePackage(t *testing.T, root, name, version, bin, script string) {
	t.Helper()
	packageRoot := filepath.Join(root, "node_modules", name)
	if err := os.MkdirAll(filepath.Join(packageRoot, filepath.Dir(script)), 0o700); err != nil {
		t.Fatal(err)
	}
	content := `{"name":"` + name + `","version":"` + version + `","bin":` + bin + `}`
	if err := os.WriteFile(filepath.Join(packageRoot, "package.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageRoot, script), []byte("fixture script"), 0o600); err != nil {
		t.Fatal(err)
	}
}
