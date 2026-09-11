package assurance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/environment"
	"github.com/knowgyu/dev-control-room/internal/masking"
)

const (
	QualityAdapterRuffID         = "quality.python.ruff"
	QualityAdapterPytestID       = "quality.python.pytest"
	QualityParserRuffJSON        = "ruff.json.v1"
	QualityParserPytest          = "pytest.result.v1"
	QualityAdapterMaxOutput      = 256 << 10
	QualityAdapterMaxTimeout     = 10 * time.Minute
	QualityAdapterDefaultTimeout = 2 * time.Minute
)

type QualityOutcome string

const (
	QualityOutcomeClean        QualityOutcome = "clean"
	QualityOutcomeFindings     QualityOutcome = "findings"
	QualityOutcomeTestsFailed  QualityOutcome = "tests_failed"
	QualityOutcomeToolError    QualityOutcome = "tool_error"
	QualityOutcomeInconclusive QualityOutcome = "inconclusive"
)

type QualityFinding struct {
	Rule     string `json:"rule,omitempty"`
	Message  string `json:"message"`
	Severity string `json:"severity,omitempty"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
}

type QualityTestSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

type QualityAdapterRequest struct {
	WorktreeRoot    string
	ComponentRoot   string
	ConfigDigest    string
	ToolVersion     string
	InterpreterPath string
	NodePath        string
	NPMPath         string
	NodeModulesRoot string
	Timeout         time.Duration
	OutputLimit     int
	Masker          *masking.Masker
}

type QualityAdapterCommand struct {
	AdapterID      string
	ParserID       string
	Command        TypedCommand
	WorktreeRoot   string
	ComponentRoot  string
	Timeout        time.Duration
	MaxOutputBytes int
}

type QualityProcessOutput struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type QualityProcessRunner interface {
	Run(context.Context, TypedCommand, string, time.Duration, int) (QualityProcessOutput, error)
}

type nativeQualityProcessRunner struct{}

func (nativeQualityProcessRunner) Run(ctx context.Context, command TypedCommand, directory string, timeout time.Duration, limit int) (QualityProcessOutput, error) {
	if err := command.Validate(); err != nil {
		return QualityProcessOutput{}, err
	}
	result, err := (environment.ProcessRunner{OutputLimit: limit}).RunInDirectory(
		ctx,
		command.Executable,
		command.Arguments,
		environment.AllowlistedEnvironment(nil),
		directory,
		timeout,
	)
	return QualityProcessOutput{Stdout: result.Stdout, Stderr: result.Stderr, ExitCode: result.ExitCode}, err
}

type Windows11Capability struct {
	Available   bool   `json:"available"`
	BuildNumber int    `json:"buildNumber,omitempty"`
	ReasonCode  string `json:"reasonCode"`
	Detail      string `json:"detail"`
}

var qualityWindowsBuildReader = readNativeWindowsBuild

func CheckWindows11Capability(ctx context.Context) Windows11Capability {
	if runtime.GOOS != "windows" {
		return Windows11Capability{ReasonCode: "platform.non_windows", Detail: "native Windows 11 is required"}
	}
	build, err := qualityWindowsBuildReader(ctx)
	if err != nil {
		return Windows11Capability{ReasonCode: "platform.build_unavailable", Detail: "native Windows build could not be verified"}
	}
	if build < 22000 {
		return Windows11Capability{BuildNumber: build, ReasonCode: "platform.windows_10", Detail: "Windows build 22000 or newer is required"}
	}
	return Windows11Capability{Available: true, BuildNumber: build, ReasonCode: "platform.windows_11", Detail: "native Windows 11 capability verified"}
}

func readNativeWindowsBuild(ctx context.Context) (int, error) {
	path, err := exec.LookPath("reg.exe")
	if err != nil || !isVerifiedNativeQualityExecutable(path, "reg.exe") {
		return 0, errors.New("reg.exe is unavailable")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return 0, errors.New("reg.exe is not a regular native executable")
	}
	result, err := (nativeQualityProcessRunner{}).Run(ctx, TypedCommand{Executable: path, Arguments: []string{
		"query", `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "/v", "CurrentBuildNumber",
	}}, "", 5*time.Second, 16<<10)
	if err != nil || result.ExitCode != 0 {
		return 0, errors.New("Windows build query failed")
	}
	match := regexp.MustCompile(`(?im)CurrentBuildNumber\s+REG_\w+\s+(\d+)`).FindStringSubmatch(result.Stdout)
	if len(match) != 2 {
		return 0, errors.New("Windows build query was not understood")
	}
	return strconv.Atoi(match[1])
}

type QualityAdapterResult struct {
	AdapterID     string              `json:"adapterId"`
	ParserID      string              `json:"parserId"`
	ConfigDigest  string              `json:"configDigest,omitempty"`
	ComponentRoot string              `json:"componentRoot"`
	Command       TypedCommand        `json:"command"`
	Outcome       QualityOutcome      `json:"outcome"`
	ExitCode      int                 `json:"exitCode"`
	Findings      []QualityFinding    `json:"findings"`
	Tests         *QualityTestSummary `json:"tests,omitempty"`
	Evidence      string              `json:"evidence,omitempty"`
	ParserError   string              `json:"parserError,omitempty"`
	TimedOut      bool                `json:"timedOut,omitempty"`
	rawStdout     string
}

type PythonQualityRunner struct {
	Capability func(context.Context) Windows11Capability
	Process    QualityProcessRunner
}

func NewPythonQualityRunner() PythonQualityRunner {
	return PythonQualityRunner{Capability: func(ctx context.Context) Windows11Capability { return CheckWindows11Capability(ctx) }, Process: nativeQualityProcessRunner{}}
}

func (r PythonQualityRunner) SelectRuff(request QualityAdapterRequest) (QualityAdapterCommand, error) {
	root, component, timeout, limit, err := validateQualityAdapterRequest(request, request.InterpreterPath, "python.exe")
	if err != nil {
		return QualityAdapterCommand{}, err
	}
	return QualityAdapterCommand{AdapterID: QualityAdapterRuffID, ParserID: QualityParserRuffJSON, Command: TypedCommand{Executable: request.InterpreterPath, Arguments: []string{"-m", "ruff", "check", "--output-format=json", "."}}, WorktreeRoot: root, ComponentRoot: component, Timeout: timeout, MaxOutputBytes: limit}, nil
}

func (r PythonQualityRunner) SelectPytest(request QualityAdapterRequest) (QualityAdapterCommand, error) {
	root, component, timeout, limit, err := validateQualityAdapterRequest(request, request.InterpreterPath, "python.exe")
	if err != nil {
		return QualityAdapterCommand{}, err
	}
	return QualityAdapterCommand{AdapterID: QualityAdapterPytestID, ParserID: QualityParserPytest, Command: TypedCommand{Executable: request.InterpreterPath, Arguments: []string{"-m", "pytest", "-q", "."}}, WorktreeRoot: root, ComponentRoot: component, Timeout: timeout, MaxOutputBytes: limit}, nil
}

func (r PythonQualityRunner) RunRuff(ctx context.Context, request QualityAdapterRequest) QualityAdapterResult {
	command, err := r.SelectRuff(request)
	if err != nil {
		return adapterUnavailable(QualityAdapterRuffID, QualityParserRuffJSON, request, err)
	}
	return r.runJSON(ctx, request, command, func(data []byte, masker *masking.Masker) ([]QualityFinding, *QualityTestSummary, error) {
		findings, err := ParseRuffJSON(data, masker)
		return findings, nil, err
	})
}

func (r PythonQualityRunner) RunPytest(ctx context.Context, request QualityAdapterRequest) QualityAdapterResult {
	command, err := r.SelectPytest(request)
	if err != nil {
		return adapterUnavailable(QualityAdapterPytestID, QualityParserPytest, request, err)
	}
	return r.runResult(ctx, request, command)
}

func (r PythonQualityRunner) runJSON(ctx context.Context, request QualityAdapterRequest, command QualityAdapterCommand, parse func([]byte, *masking.Masker) ([]QualityFinding, *QualityTestSummary, error)) QualityAdapterResult {
	result := r.runProcess(ctx, request, command)
	if result.TimedOut || result.ParserError != "" || result.Outcome == QualityOutcomeToolError {
		return result
	}
	findings, tests, err := parse([]byte(result.rawStdout), request.Masker)
	if err != nil {
		result.Outcome = QualityOutcomeInconclusive
		result.ParserError = "quality JSON parser rejected the bounded report"
		return result
	}
	result.Findings = findings
	result.Tests = tests
	if len(findings) > 0 {
		result.Outcome = QualityOutcomeFindings
	} else if result.ExitCode == 0 {
		result.Outcome = QualityOutcomeClean
	} else {
		result.Outcome = QualityOutcomeToolError
	}
	return result
}

func (r PythonQualityRunner) runResult(ctx context.Context, request QualityAdapterRequest, command QualityAdapterCommand) QualityAdapterResult {
	result := r.runProcess(ctx, request, command)
	if result.TimedOut || result.ParserError != "" || result.Outcome == QualityOutcomeToolError || result.Outcome == QualityOutcomeInconclusive {
		return result
	}
	if result.ExitCode == 0 {
		result.Outcome = QualityOutcomeClean
	} else {
		result.Outcome = QualityOutcomeTestsFailed
	}
	return result
}

func (r PythonQualityRunner) runProcess(ctx context.Context, request QualityAdapterRequest, command QualityAdapterCommand) QualityAdapterResult {
	result := QualityAdapterResult{AdapterID: command.AdapterID, ParserID: command.ParserID, ConfigDigest: request.ConfigDigest, ComponentRoot: command.ComponentRoot, Command: command.Command, Findings: []QualityFinding{}}
	capability := CheckWindows11Capability
	if r.Capability != nil {
		capability = r.Capability
	}
	if !capability(ctx).Available {
		result.Outcome = QualityOutcomeInconclusive
		result.ParserError = "native Windows 11 capability is unavailable"
		return result
	}
	process := r.Process
	if process == nil {
		process = nativeQualityProcessRunner{}
	}
	output, err := process.Run(ctx, command.Command, command.ComponentRoot, command.Timeout, command.MaxOutputBytes)
	result.ExitCode = output.ExitCode
	result.rawStdout = output.Stdout
	result.Evidence = maskQualityEvidence(output.Stdout, output.Stderr, request.Masker, command.MaxOutputBytes)
	if err != nil && !isExpectedQualityProcessExit(err, result.ExitCode) {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timed out") {
			result.TimedOut = true
			result.Outcome = QualityOutcomeInconclusive
		}
		if result.Outcome == "" {
			result.Outcome = QualityOutcomeToolError
		}
	}
	return result
}

// isExpectedQualityProcessExit distinguishes a reviewed tool reporting a
// finding or failed test from a process that could not be launched or
// otherwise failed at the infrastructure boundary. ProcessRunner preserves
// the command output and returns *exec.ExitError for ordinary non-zero exits.
func isExpectedQualityProcessExit(err error, exitCode int) bool {
	if err == nil || exitCode == 0 {
		return false
	}
	var exitError *exec.ExitError
	return errors.As(err, &exitError) && exitError.ExitCode() == exitCode
}

func ParseRuffJSON(data []byte, masker *masking.Masker) ([]QualityFinding, error) {
	var entries []struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Filename string `json:"filename"`
		Location struct {
			Row    int `json:"row"`
			Column int `json:"column"`
		} `json:"location"`
	}
	if err := json.Unmarshal(data, &entries); err != nil || len(entries) > 2000 {
		return nil, errors.New("invalid Ruff JSON")
	}
	findings := make([]QualityFinding, 0, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.Message) == "" || entry.Location.Row < 0 || entry.Location.Column < 0 {
			return nil, errors.New("invalid Ruff finding")
		}
		findings = append(findings, QualityFinding{Rule: maskText(masker, entry.Code), Message: maskText(masker, entry.Message), Severity: "error", File: maskText(masker, entry.Filename), Line: entry.Location.Row, Column: entry.Location.Column})
	}
	return findings, nil
}

func validateQualityAdapterRequest(request QualityAdapterRequest, executable, base string) (string, string, time.Duration, int, error) {
	root := filepath.Clean(request.WorktreeRoot)
	if err := validateQualityWorktreeRoot(root, os.Stat, os.Lstat); err != nil {
		return "", "", 0, 0, err
	}
	component := root
	if request.ComponentRoot != "" {
		component = filepath.Clean(request.ComponentRoot)
	}
	if !qualityPathWithin(root, component) || qualityPathContainsSymlink(component, os.Lstat) {
		return "", "", 0, 0, errors.New("component root is outside or linked from the worktree")
	}
	info, err := os.Lstat(component)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", "", 0, 0, errors.New("component root is not a regular directory")
	}
	if !isVerifiedNativeQualityExecutable(executable, base) {
		return "", "", 0, 0, fmt.Errorf("selected %s must be an absolute native %s", base, base)
	}
	info, err = os.Lstat(executable)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", "", 0, 0, errors.New("selected interpreter is not a regular executable")
	}
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = QualityAdapterDefaultTimeout
	}
	if timeout > QualityAdapterMaxTimeout {
		return "", "", 0, 0, errors.New("quality adapter timeout exceeds the fixed bound")
	}
	limit := request.OutputLimit
	if limit <= 0 {
		limit = QualityAdapterMaxOutput
	}
	if limit > QualityAdapterMaxOutput {
		return "", "", 0, 0, errors.New("quality adapter output exceeds the fixed bound")
	}
	return root, component, timeout, limit, nil
}

func adapterUnavailable(adapterID, parserID string, request QualityAdapterRequest, err error) QualityAdapterResult {
	return QualityAdapterResult{AdapterID: adapterID, ParserID: parserID, ConfigDigest: request.ConfigDigest, ComponentRoot: request.ComponentRoot, Findings: []QualityFinding{}, Outcome: QualityOutcomeToolError, ParserError: "quality adapter request was rejected"}
}

func maskQualityEvidence(stdout, stderr string, masker *masking.Masker, limit int) string {
	evidence := stdout
	if stderr != "" {
		evidence += "\n[stderr]\n" + stderr
	}
	if masker == nil {
		masker = masking.New(nil, []string{"TOKEN", "PASSWORD", "SECRET", "API_KEY", "AUTHORIZATION"})
	}
	evidence = masker.Mask(evidence)
	if len(evidence) > limit {
		return evidence[:limit]
	}
	return evidence
}

func maskText(masker *masking.Masker, value string) string {
	if masker == nil {
		return masking.New(nil, []string{"TOKEN", "PASSWORD", "SECRET", "API_KEY", "AUTHORIZATION"}).Mask(value)
	}
	return masker.Mask(value)
}

func qualityPathWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && (relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)))
}
