package environment

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/masking"
)

type fakeRunner struct {
	results map[string]Result
	err     error
}

type recordingRunner struct {
	args [][]string
}

func (r *recordingRunner) Run(_ context.Context, _ string, args []string, _ []string, _ time.Duration) (Result, error) {
	r.args = append(r.args, append([]string(nil), args...))
	return Result{Stdout: `{"Available":true,"CommandType":"Function","Path":"","Version":"2.0.1"}`, ExitCode: 0}, nil
}

func (f fakeRunner) Run(_ context.Context, executable string, args []string, _ []string, _ time.Duration) (Result, error) {
	if f.err != nil {
		return Result{}, f.err
	}
	if strings.Contains(strings.ToLower(executable), "pwsh") && len(args) > 1 {
		return Result{Stdout: "profile output secret-canary-value\n{" + `"Available":true,"CommandType":"Function","Path":"","Version":"2.0.1"}`}, nil
	}
	return f.results[executable], nil
}

func TestDoctorResolvesDirectAndPowerShellProfilesWithoutOutput(t *testing.T) {
	canary := "secret-canary-value"
	runner := fakeRunner{results: map[string]Result{
		"C:\\tools\\pwsh.exe":    {Stdout: "7.6.5", ExitCode: 0},
		"C:\\tools\\git.exe":     {Stdout: "git version 2.45.0", ExitCode: 0},
		"C:\\tools\\gh.exe":      {Stdout: "gh version 2.50.0", ExitCode: 0},
		"C:\\tools\\codex.exe":   {Stdout: canary + " 1.2.3", ExitCode: 0},
		"C:\\tools\\claude.exe":  {Stdout: "1.0.0", ExitCode: 0},
		"C:\\tools\\gemini.exe":  {Stdout: "1.0.0", ExitCode: 0},
		"C:\\tools\\profile.exe": {Stdout: "1.0.0", ExitCode: 0},
	}}
	paths := map[string]string{"pwsh": "C:\\tools\\pwsh.exe", "git": "C:\\tools\\git.exe", "gh": "C:\\tools\\gh.exe", "codex": "C:\\tools\\codex.exe", "claude": "C:\\tools\\claude.exe", "gemini": "C:\\tools\\gemini.exe", "pwsh.exe": "C:\\tools\\pwsh.exe", "C:\\tools\\profile.exe": "C:\\tools\\profile.exe"}
	doctor := NewDoctor(runner, masking.New([]string{canary}, nil))
	doctor.LookPath = func(name string) (string, error) {
		if path, ok := paths[name]; ok {
			return path, nil
		}
		return "", errors.New("not found")
	}
	profiles := []domain.AgentProfile{
		{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AgentProfileKind}, Metadata: domain.ObjectMeta{ID: "direct", Name: "Direct"}, Spec: domain.AgentProfileSpec{Command: "C:\\tools\\profile.exe", LaunchMode: domain.AgentLaunchDirect, DataBoundary: domain.AgentBoundaryLocal}},
		{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AgentProfileKind}, Metadata: domain.ObjectMeta{ID: "profile", Name: "Profile"}, Spec: domain.AgentProfileSpec{Command: "claude-local", LaunchMode: domain.AgentLaunchPowerShellProfile, DataBoundary: domain.AgentBoundaryLocal}},
	}
	health := doctor.Run(context.Background(), profiles, nil, nil)
	encoded, _ := json.Marshal(health)
	if strings.Contains(string(encoded), canary) {
		t.Fatalf("secret output crossed doctor boundary: %s", encoded)
	}
	if !health.Profiles[0].Available || health.Profiles[0].CommandType != "Application" {
		t.Fatalf("direct profile not resolved: %#v", health.Profiles[0])
	}
	if !health.Profiles[1].Available || health.Profiles[1].CommandType != "Function" {
		t.Fatalf("PowerShell profile not resolved: %#v", health.Profiles[1])
	}
	if health.Profiles[1].ResolvedPath != "" {
		t.Fatalf("function profile unexpectedly exposed a path: %#v", health.Profiles[1])
	}
}

func TestDoctorFindingsAreDeterministicForEnvironmentDeclarations(t *testing.T) {
	name := "DEVROOM_MISSING_CANARY"
	_ = name
	doctor := NewDoctor(fakeRunner{}, masking.New(nil, nil))
	health := doctor.Run(context.Background(), nil, []domain.EnvironmentDeclaration{
		{Name: "DUPLICATE_VAR", Scope: "user"}, {Name: "DUPLICATE_VAR", Scope: "user"},
		{Name: "CONFLICT_VAR", Scope: "user"}, {Name: "CONFLICT_VAR", Scope: "machine"},
		{Name: "MISSING_VAR", Scope: "process"},
	}, nil)
	if len(health.Environment) != 3 {
		t.Fatalf("unexpected environment statuses: %#v", health.Environment)
	}
	states := map[string]string{}
	for _, status := range health.Environment {
		states[status.Name] = status.State
	}
	if states["DUPLICATE_VAR"] != "duplicate" || states["CONFLICT_VAR"] != "conflict" || states["MISSING_VAR"] != "missing" {
		t.Fatalf("unexpected states: %#v", states)
	}
	if health.Findings[0].Type > health.Findings[len(health.Findings)-1].Type {
		t.Fatalf("findings are not sorted: %#v", health.Findings)
	}
	if health.Available {
		t.Fatal("environment conflicts and missing declarations left health available")
	}
}

func TestDoctorReportsMissingAndFailedProbes(t *testing.T) {
	doctor := NewDoctor(fakeRunner{results: map[string]Result{"C:\\tools\\pwsh.exe": {Stdout: "7.6.5", ExitCode: 0}, "C:\\tools\\git.exe": {ExitCode: 1}}}, masking.New(nil, nil))
	doctor.LookPath = func(name string) (string, error) {
		if name == "gemini" {
			return "", errors.New("missing")
		}
		return "C:\\tools\\" + name + ".exe", nil
	}
	health := doctor.Run(context.Background(), nil, nil, nil)
	byName := map[string]ToolStatus{}
	for _, tool := range health.Tools {
		byName[tool.Name] = tool
	}
	if byName["gemini"].Reason != "command not found" || byName["git"].Reason != "version probe failed" {
		t.Fatalf("missing/failed probes were not deterministic: %#v", byName)
	}
}

func TestDirectProbeRejectsNonNativeWindowsShimBeforeExecution(t *testing.T) {
	runner := &recordingRunner{}
	doctor := NewDoctor(runner, masking.New(nil, nil))
	doctor.LookPath = func(string) (string, error) { return `C:\tools\tool.cmd`, nil }
	status := doctor.resolveDirect(context.Background(), "tool", []string{"--version"}, 1)
	if status.Available || len(runner.args) != 0 || !strings.Contains(status.Reason, "local Windows .exe") {
		t.Fatalf("non-native command shim crossed direct execution boundary: %#v, %#v", status, runner.args)
	}
}

func TestDoctorMarksFailedConnectorValidationUnavailable(t *testing.T) {
	if err := os.Setenv("DEVROOM_CONNECTOR_REFERENCE", "present"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("DEVROOM_CONNECTOR_REFERENCE") })
	results := map[string]Result{}
	doctor := NewDoctor(fakeRunner{results: results}, masking.New(nil, nil))
	doctor.LookPath = func(name string) (string, error) {
		path := `C:\tools\` + name + `.exe`
		results[path] = Result{Stdout: name + " 1.2.3", ExitCode: 0}
		return path, nil
	}
	health := doctor.Run(context.Background(), nil, nil, []domain.ConnectorReference{{ID: "example", Name: "Example", SecretReference: "env:DEVROOM_CONNECTOR_REFERENCE", LastResult: "failed"}})
	if health.Available || len(health.Connectors) != 1 || health.Connectors[0].Reason == "" {
		t.Fatalf("failed connector validation left health available: %#v", health)
	}
}

func TestPowerShellProfileDefaultsToVersionProbe(t *testing.T) {
	runner := &recordingRunner{}
	doctor := NewDoctor(runner, masking.New(nil, nil))
	doctor.LookPath = func(name string) (string, error) {
		if name == "pwsh" {
			return `C:\Windows\System32\pwsh.exe`, nil
		}
		return "", errors.New("not used")
	}
	profile := domain.AgentProfile{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AgentProfileKind}, Metadata: domain.ObjectMeta{ID: "ps-profile", Name: "PS Profile"}, Spec: domain.AgentProfileSpec{Command: "Get-Command", LaunchMode: domain.AgentLaunchPowerShellProfile, DataBoundary: domain.AgentBoundaryLocal}}
	status := doctor.resolveProfile(context.Background(), profile)
	if !status.Available || len(runner.args) != 1 || !strings.Contains(runner.args[0][2], "--version") {
		t.Fatalf("PowerShell profile did not use safe default version probe: %#v, %#v", status, runner.args)
	}
}

func TestSafeEnvironmentDoesNotInheritParentEnvironment(t *testing.T) {
	if err := os.Setenv("DEVROOM_UNALLOWLISTED_CANARY", "secret-canary-value"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("DEVROOM_ALLOWED_FIXTURE", "allowed"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv("DEVROOM_UNALLOWLISTED_CANARY")
		_ = os.Unsetenv("DEVROOM_ALLOWED_FIXTURE")
	})
	environment := strings.Join(safeEnvironment([]string{"DEVROOM_ALLOWED_FIXTURE"}), "\n")
	if strings.Contains(environment, "DEVROOM_UNALLOWLISTED_CANARY") || strings.Contains(environment, "secret-canary-value") {
		t.Fatalf("unallowlisted parent environment crossed process boundary: %s", environment)
	}
	if !strings.Contains(environment, "DEVROOM_ALLOWED_FIXTURE=allowed") {
		t.Fatalf("configured allowlisted environment was not passed: %s", environment)
	}
}

func TestProcessRunnerWithEmptyEnvironmentDoesNotFallBackToParent(t *testing.T) {
	if err := os.Setenv("DEVROOM_UNALLOWLISTED_CANARY", "secret-canary-value"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("DEVROOM_UNALLOWLISTED_CANARY") })
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	result, err := (ProcessRunner{}).Run(context.Background(), executable, processRunnerHelperArgs(), []string{}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(result.Stdout) != "" {
		t.Fatalf("empty child environment inherited parent values: %q", result.Stdout)
	}
}

func TestVersionParserUsesFinalProbeVersion(t *testing.T) {
	if got := parseVersion("host Ubuntu-24.04 warning\nfixture-version 1.2.3\n"); got != "1.2.3" {
		t.Fatalf("version parser selected unrelated host version: %q", got)
	}
}

func TestProcessRunnerTimeoutCancellationAndBoundedStreams(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (ProcessRunner{}).Run(context.Background(), executable, processRunnerHelperArgs(), processRunnerHelperEnvironment("wait"), 10*time.Millisecond); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout, got %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (ProcessRunner{}).Run(ctx, executable, processRunnerHelperArgs(), processRunnerHelperEnvironment("wait"), time.Second); err == nil {
		t.Fatal("expected cancellation")
	}
	if _, err := (ProcessRunner{OutputLimit: 8}).Run(context.Background(), executable, processRunnerHelperArgs(), processRunnerHelperEnvironment("output"), time.Second); err == nil || !strings.Contains(err.Error(), "bounded") {
		t.Fatalf("expected bounded output cancellation, got %v", err)
	}
}

func TestProcessRunnerHelper(_ *testing.T) {
	if os.Getenv("DEVROOM_PROCESS_RUNNER_HELPER") != "1" && !strings.Contains(strings.Join(os.Args, "\x00"), "-test.run=^TestProcessRunnerHelper$") {
		return
	}
	switch os.Getenv("DEVROOM_PROCESS_RUNNER_MODE") {
	case "wait":
		for {
			time.Sleep(time.Hour)
		}
	case "output":
		_, _ = os.Stdout.WriteString(strings.Repeat("x", 1024))
	case "":
		_, _ = os.Stdout.WriteString(os.Getenv("DEVROOM_UNALLOWLISTED_CANARY"))
	default:
		os.Exit(2)
	}
	os.Exit(0)
}

func processRunnerHelperArgs() []string {
	return []string{"-test.run=^TestProcessRunnerHelper$"}
}

func processRunnerHelperEnvironment(mode string) []string {
	return []string{"DEVROOM_PROCESS_RUNNER_HELPER=1", "DEVROOM_PROCESS_RUNNER_MODE=" + mode}
}
