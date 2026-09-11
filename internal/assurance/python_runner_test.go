package assurance

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestPythonRunnerBuildsTypedCommandsAndParsesResults(t *testing.T) {
	root := adapterFixtureRoot(t)
	python := adapterExecutable(t, root, "python.exe")
	runner := PythonQualityRunner{Capability: availableWindows11, Process: fakeQualityProcess{output: QualityProcessOutput{ExitCode: 1, Stdout: `[{"code":"E501","message":"TOKEN=secret-value","filename":"main.py","location":{"row":4,"column":2}}]`}}}
	request := QualityAdapterRequest{WorktreeRoot: root, ComponentRoot: root, InterpreterPath: python, ConfigDigest: "sha256:fixture"}
	selected, err := runner.SelectRuff(request)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Command.Executable != python || len(selected.Command.Arguments) != 5 || selected.Command.Arguments[2] != "check" || selected.Command.Arguments[3] != "--output-format=json" {
		t.Fatalf("Ruff command = %#v", selected.Command)
	}
	result := runner.RunRuff(context.Background(), request)
	if result.Outcome != QualityOutcomeFindings || len(result.Findings) != 1 || result.Findings[0].Message != "TOKEN=[REDACTED]" || result.Evidence == "" {
		t.Fatalf("Ruff result = %#v", result)
	}
}

func TestPythonRunnerRejectsParserFailuresAndUnavailableWindows(t *testing.T) {
	root := adapterFixtureRoot(t)
	python := adapterExecutable(t, root, "python.exe")
	request := QualityAdapterRequest{WorktreeRoot: root, InterpreterPath: python}
	runner := PythonQualityRunner{Capability: availableWindows11, Process: fakeQualityProcess{output: QualityProcessOutput{ExitCode: 0, Stdout: "not json"}}}
	result := runner.RunRuff(context.Background(), request)
	if result.Outcome != QualityOutcomeInconclusive || result.ParserError == "" {
		t.Fatalf("malformed Ruff result = %#v", result)
	}
	called := &countingQualityProcess{}
	blocked := PythonQualityRunner{Capability: func(context.Context) Windows11Capability {
		return Windows11Capability{ReasonCode: "platform.non_windows"}
	}, Process: called}
	result = blocked.RunPytest(context.Background(), request)
	if result.Outcome != QualityOutcomeInconclusive || called.calls != 0 {
		t.Fatalf("unavailable pytest result = %#v, calls=%d", result, called.calls)
	}
}

func TestCheckWindows11CapabilityRejectsNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native capability is covered by the Windows acceptance gate")
	}
	capability := CheckWindows11Capability(context.Background())
	if capability.Available || capability.ReasonCode != "platform.non_windows" {
		t.Fatalf("capability = %#v", capability)
	}
}

func TestPythonRunnerBoundsTimeoutAndOutputConfiguration(t *testing.T) {
	root := adapterFixtureRoot(t)
	python := adapterExecutable(t, root, "python.exe")
	_, err := (PythonQualityRunner{}).SelectPytest(QualityAdapterRequest{WorktreeRoot: root, InterpreterPath: python, Timeout: 11 * time.Minute})
	if err == nil {
		t.Fatal("unbounded timeout accepted")
	}
	_, err = (PythonQualityRunner{}).SelectPytest(QualityAdapterRequest{WorktreeRoot: root, InterpreterPath: python, OutputLimit: QualityAdapterMaxOutput + 1})
	if err == nil {
		t.Fatal("unbounded output accepted")
	}
	if errors.Is(err, context.Canceled) {
		t.Fatal("unexpected context error")
	}
}

func TestPythonRunnerParsesExpectedNonzeroProcessExits(t *testing.T) {
	root := adapterFixtureRoot(t)
	python := adapterExecutable(t, root, "python.exe")
	request := QualityAdapterRequest{WorktreeRoot: root, InterpreterPath: python}

	var ruffProcessErr error
	ruff := &helperQualityProcess{kind: "ruff", processErr: &ruffProcessErr}
	ruffRunner := PythonQualityRunner{Capability: availableWindows11, Process: ruff}
	ruffResult := ruffRunner.RunRuff(context.Background(), request)
	assertExpectedProcessExit(t, ruffProcessErr)
	if ruffResult.Outcome != QualityOutcomeFindings || len(ruffResult.Findings) != 1 || ruffResult.ExitCode != 1 {
		t.Fatalf("Ruff result = %#v", ruffResult)
	}

	var pytestProcessErr error
	pytest := &helperQualityProcess{kind: "pytest", processErr: &pytestProcessErr}
	pytestRunner := PythonQualityRunner{Capability: availableWindows11, Process: pytest}
	pytestResult := pytestRunner.RunPytest(context.Background(), request)
	assertExpectedProcessExit(t, pytestProcessErr)
	if pytestResult.Outcome != QualityOutcomeTestsFailed || pytestResult.ExitCode != 1 {
		t.Fatalf("pytest result = %#v", pytestResult)
	}
}

func TestPythonRunnerKeepsLaunchErrorsAsToolErrors(t *testing.T) {
	root := adapterFixtureRoot(t)
	python := adapterExecutable(t, root, "python.exe")
	runner := PythonQualityRunner{
		Capability: availableWindows11,
		Process: fakeQualityProcess{
			output: QualityProcessOutput{ExitCode: 1, Stdout: `[{"code":"E501","message":"line too long","filename":"main.py","location":{"row":4,"column":2}}]`},
			err:    errors.New("process could not be launched"),
		},
	}
	result := runner.RunRuff(context.Background(), QualityAdapterRequest{WorktreeRoot: root, InterpreterPath: python})
	if result.Outcome != QualityOutcomeToolError || len(result.Findings) != 0 {
		t.Fatalf("launch failure result = %#v", result)
	}
}

func availableWindows11(context.Context) Windows11Capability {
	return Windows11Capability{Available: true, BuildNumber: 22631}
}

type fakeQualityProcess struct {
	output QualityProcessOutput
	err    error
}

func (f fakeQualityProcess) Run(context.Context, TypedCommand, string, time.Duration, int) (QualityProcessOutput, error) {
	return f.output, f.err
}

type countingQualityProcess struct{ calls int }

func (c *countingQualityProcess) Run(context.Context, TypedCommand, string, time.Duration, int) (QualityProcessOutput, error) {
	c.calls++
	return QualityProcessOutput{}, nil
}

func adapterFixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func adapterExecutable(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte("fixture executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
