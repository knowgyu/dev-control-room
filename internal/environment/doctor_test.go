package environment

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	if byName["gemini"].Required || byName["gemini"].State != "optional" {
		t.Fatalf("optional Provider was not classified as optional: %#v", byName["gemini"])
	}
	if byName["git"].State != "unavailable" || health.Available {
		t.Fatalf("required tool failure did not affect health: %#v", health)
	}
}

func TestDoctorKeepsMissingOptionalToolsFromGlobalHealthFailure(t *testing.T) {
	doctor := NewDoctor(fakeRunner{results: map[string]Result{
		"C:\\tools\\pwsh.exe": {Stdout: "7.6.5", ExitCode: 0},
		"C:\\tools\\git.exe":  {Stdout: "git version 2.45.0", ExitCode: 0},
	}}, masking.New(nil, nil))
	doctor.LookPath = func(name string) (string, error) {
		if name == "pwsh" || name == "git" {
			return "C:\\tools\\" + name + ".exe", nil
		}
		return "", errors.New("missing")
	}
	health := doctor.Run(context.Background(), nil, nil, nil)
	if !health.Available {
		t.Fatalf("optional Provider absence made environment unhealthy: %#v", health)
	}
	for _, tool := range health.Tools {
		if tool.Name == "codex" && (tool.State != "optional" || tool.Required) {
			t.Fatalf("codex status = %#v", tool)
		}
	}
}

func TestDoctorKeepsUnconfiguredDefaultProviderProfilesNeutral(t *testing.T) {
	doctor := providerProfileDoctor()
	profiles := []domain.AgentProfile{
		testDefaultProviderProfile("codex", domain.AgentLaunchDirect, 120),
		testDefaultProviderProfile("claude", domain.AgentLaunchDirect, 8),
		testDefaultProviderProfile("gemini", domain.AgentLaunchDirect, 8),
	}

	health := doctor.Run(context.Background(), profiles, nil, nil)
	if !health.Available {
		t.Fatalf("unconfigured default providers made environment unhealthy: %#v", health)
	}
	if len(health.Profiles) != len(profiles) {
		t.Fatalf("provider rows were not preserved: %#v", health.Profiles)
	}
	for _, profile := range health.Profiles {
		if profile.Required || profile.Available || profile.State != "optional" {
			t.Fatalf("default provider was not neutral: %#v", profile)
		}
		if !hasFinding(health.Findings, "agent_profile."+profile.ID, "info") {
			t.Fatalf("optional provider finding was not preserved as informational: %#v", health.Findings)
		}
	}
}

func TestDoctorKeepsExplicitProviderProfileRequired(t *testing.T) {
	doctor := providerProfileDoctor()
	profile := testDefaultProviderProfile("codex", domain.AgentLaunchDirect, 120)
	profile.Metadata.Name = "Codex team"
	profile.Spec.Command = "codex-team"
	health := doctor.Run(context.Background(), []domain.AgentProfile{profile}, nil, nil)

	if health.Available {
		t.Fatalf("missing explicitly configured provider left environment available: %#v", health)
	}
	if len(health.Profiles) != 1 || !health.Profiles[0].Required || health.Profiles[0].State != "unavailable" {
		t.Fatalf("explicit provider was not required: %#v", health.Profiles)
	}
	if !hasFinding(health.Findings, "agent_profile.codex", "attention") {
		t.Fatalf("required provider finding was not preserved: %#v", health.Findings)
	}
}

func TestDoctorPromotesReferencedDefaultProviderToRequired(t *testing.T) {
	const selection = "DEVROOM_SELECTED_PROVIDER"
	doctor := providerProfileDoctor()
	profile := testDefaultProviderProfile("gemini", domain.AgentLaunchDirect, 8)
	declarations := []domain.EnvironmentDeclaration{{Name: selection, Scope: "process", ProfileID: profile.Metadata.ID}}
	t.Setenv(selection, "gemini")
	health := doctor.Run(context.Background(), []domain.AgentProfile{profile}, declarations, nil)

	if health.Available {
		t.Fatalf("missing selected provider left environment available: %#v", health)
	}
	if len(health.Profiles) != 1 || !health.Profiles[0].Required || health.Profiles[0].State != "unavailable" {
		t.Fatalf("selected default provider was not required: %#v", health.Profiles)
	}
}

func providerProfileDoctor() Doctor {
	doctor := NewDoctor(fakeRunner{results: map[string]Result{
		`C:\tools\pwsh.exe`: {Stdout: "PowerShell 7.6.5", ExitCode: 0},
		`C:\tools\git.exe`:  {Stdout: "git version 2.45.0", ExitCode: 0},
	}}, masking.New(nil, nil))
	doctor.LookPath = func(name string) (string, error) {
		if name == "pwsh" || name == "git" {
			return `C:\tools\` + name + `.exe`, nil
		}
		return "", errors.New("missing")
	}
	return doctor
}

func testDefaultProviderProfile(id string, mode domain.AgentLaunchMode, timeout int) domain.AgentProfile {
	return domain.AgentProfile{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AgentProfileKind},
		Metadata: domain.ObjectMeta{ID: id, Name: strings.ToUpper(id[:1]) + id[1:]},
		Spec:     domain.AgentProfileSpec{Command: id, LaunchMode: mode, DataBoundary: domain.AgentBoundaryLocal, TimeoutSeconds: timeout},
	}
}

func hasFinding(findings []Finding, target, severity string) bool {
	for _, finding := range findings {
		if (finding.Type == target || finding.Target == target) && finding.Severity == severity {
			return true
		}
	}
	return false
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

func TestProcessRunnerClosedStdinReturnsEOF(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	result, err := (ProcessRunner{}).Run(context.Background(), executable, processRunnerHelperArgs(), processRunnerHelperEnvironment("stdin"), 5*time.Second)
	if err != nil {
		t.Fatalf("closed stdin helper failed: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "stdin-bytes=0" {
		t.Fatalf("child did not observe immediate EOF: %q", result.Stdout)
	}
}

func TestProcessRunnerTimeoutKillsProcessTree(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "timeout-tree")
	result, err := (ProcessRunner{}).Run(context.Background(), executable, processRunnerHelperArgs(), processRunnerHelperEnvironmentWithMarker("spawn-parent", marker), 100*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected bounded timeout, result=%#v err=%v", result, err)
	}
	assertProcessTreeDidNotSurvive(t, marker)
}

func TestProcessRunnerCancellationKillsProcessTree(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "cancel-tree")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultCh := make(chan struct {
		result Result
		err    error
	}, 1)
	go func() {
		result, runErr := (ProcessRunner{}).Run(ctx, executable, processRunnerHelperArgs(), processRunnerHelperEnvironmentWithMarker("spawn-parent", marker), 5*time.Second)
		resultCh <- struct {
			result Result
			err    error
		}{result: result, err: runErr}
	}()
	waitForProcessMarker(t, marker+".started", time.Second)
	cancel()
	select {
	case outcome := <-resultCh:
		if outcome.err == nil || !errors.Is(outcome.err, context.Canceled) {
			t.Fatalf("expected context cancellation, result=%#v err=%v", outcome.result, outcome.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("process runner did not return after cancellation")
	}
	assertProcessTreeDidNotSurvive(t, marker)
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
	case "stdin":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			os.Exit(3)
		}
		_, _ = os.Stdout.WriteString("stdin-bytes=" + strconv.Itoa(len(data)))
	case "spawn-parent":
		marker := os.Getenv("DEVROOM_PROCESS_RUNNER_MARKER")
		if marker == "" {
			os.Exit(4)
		}
		child := exec.Command(os.Args[0], processRunnerHelperArgs()...)
		child.Env = processRunnerHelperEnvironmentWithMarker("spawn-child", marker)
		if err := child.Start(); err != nil {
			os.Exit(5)
		}
		for {
			time.Sleep(time.Hour)
		}
	case "spawn-child":
		marker := os.Getenv("DEVROOM_PROCESS_RUNNER_MARKER")
		if marker == "" {
			os.Exit(6)
		}
		_ = os.WriteFile(marker+".started", []byte(strconv.Itoa(os.Getpid())), 0o600)
		time.Sleep(300 * time.Millisecond)
		_ = os.WriteFile(marker+".survived", []byte(strconv.Itoa(os.Getpid())), 0o600)
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

func processRunnerHelperEnvironmentWithMarker(mode, marker string) []string {
	return append(processRunnerHelperEnvironment(mode), "DEVROOM_PROCESS_RUNNER_MARKER="+marker)
}

func waitForProcessMarker(t *testing.T, path string, timeout time.Duration) []byte {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil {
			return data
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("process marker was not written: %s", path)
	return nil
}

func assertProcessTreeDidNotSurvive(t *testing.T, marker string) {
	t.Helper()
	survivedPath := marker + ".survived"
	var survived []byte
	deadline := time.Now().Add(750 * time.Millisecond)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(survivedPath); err == nil {
			survived = data
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(survived) > 0 {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(survived))); err == nil {
			if process, findErr := os.FindProcess(pid); findErr == nil {
				_ = process.Kill()
			}
		}
		t.Fatalf("child process survived tree termination: pid=%s", strings.TrimSpace(string(survived)))
	}
}
