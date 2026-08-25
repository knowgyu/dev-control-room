// Package assurance contains the narrow provider and typed Quality Runner
// contracts used by the application service. It has no shell execution path.
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
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/environment"
	"github.com/knowgyu/dev-control-room/internal/masking"
)

type ProviderState string

const (
	ProviderNotConfigured ProviderState = "not_configured"
	ProviderDetected      ProviderState = "detected"
	ProviderReady         ProviderState = "ready"
	ProviderUnavailable   ProviderState = "unavailable"
	ProviderAuthRequired  ProviderState = "auth_required"
)

type ProviderStatus struct {
	Provider        string        `json:"provider"`
	State           ProviderState `json:"state"`
	CommandFound    bool          `json:"commandFound"`
	LaunchTrusted   bool          `json:"launchTrusted"`
	ProfileReady    bool          `json:"profileReady"`
	ResolvedCommand []string      `json:"resolvedCommand,omitempty"`
	Version         string        `json:"version,omitempty"`
	ReasonCode      string        `json:"reasonCode,omitempty"`
	Detail          string        `json:"detail,omitempty"`
}

type CodexPackage struct {
	Name string `json:"name"`
	Bin  any    `json:"bin"`
}

type TypedCommand struct {
	Executable string   `json:"executable"`
	Arguments  []string `json:"arguments"`
}

func (c TypedCommand) Validate() error {
	if strings.TrimSpace(c.Executable) == "" || strings.ContainsAny(c.Executable, "\x00\r\n") {
		return errors.New("typed command executable is invalid")
	}
	for _, arg := range c.Arguments {
		if strings.ContainsAny(arg, "\x00\r\n") {
			return errors.New("typed command argument is invalid")
		}
	}
	return nil
}

var windowsAbsolutePath = regexp.MustCompile(`(?i)^[a-z]:[\\/].+`)

// ValidateCodexPackage accepts only the package identity and a declared
// codex.js entry point. It never follows arbitrary bin data.
func ValidateCodexPackage(packageJSON []byte) (string, error) {
	var manifest CodexPackage
	if err := json.Unmarshal(packageJSON, &manifest); err != nil {
		return "", errors.New("codex package metadata is invalid")
	}
	if manifest.Name != "@openai/codex" {
		return "", errors.New("codex package name is not trusted")
	}
	var entry string
	switch value := manifest.Bin.(type) {
	case string:
		entry = value
	case map[string]any:
		for _, name := range []string{"codex", "@openai/codex"} {
			if candidate, ok := value[name].(string); ok {
				entry = candidate
				break
			}
		}
	default:
		return "", errors.New("codex package does not declare a supported bin")
	}
	entry = filepath.Clean(strings.TrimSpace(entry))
	entry = strings.ReplaceAll(entry, "\\", "/")
	if entry == "." || entry == "" || filepath.IsAbs(entry) || strings.Contains(entry, "..") || !strings.EqualFold(filepath.Base(entry), "codex.js") {
		return "", errors.New("codex package bin must be a relative codex.js path")
	}
	return entry, nil
}

// BuildCodexNPMCommand turns an observed npm shim into node.exe plus the
// verified package script. The shim and cmd.exe are never executed.
func BuildCodexNPMCommand(nodePath, packageRoot string, packageJSON []byte, args []string) (TypedCommand, error) {
	if !isWindowsNativeExecutable(nodePath, "node.exe") {
		return TypedCommand{}, errors.New("Codex npm launcher requires a local node.exe")
	}
	if !windowsAbsolutePath.MatchString(packageRoot) {
		return TypedCommand{}, errors.New("Codex package root must be an absolute Windows path")
	}
	entry, err := ValidateCodexPackage(packageJSON)
	if err != nil {
		return TypedCommand{}, err
	}
	script := strings.TrimRight(packageRoot, "\\/") + "\\" + strings.ReplaceAll(entry, "/", "\\")
	if !strings.EqualFold(filepath.Base(strings.ReplaceAll(script, "\\", "/")), "codex.js") {
		return TypedCommand{}, errors.New("Codex package script is not codex.js")
	}
	command := TypedCommand{Executable: nodePath, Arguments: append([]string{script}, args...)}
	if err := command.Validate(); err != nil {
		return TypedCommand{}, err
	}
	return command, nil
}

func isWindowsNativeExecutable(path, base string) bool {
	return windowsAbsolutePath.MatchString(strings.TrimSpace(path)) && strings.EqualFold(filepath.Base(strings.ReplaceAll(path, "\\", "/")), base) && !strings.Contains(path[3:], ":")
}

type CodexResolver struct {
	LookPath func(string) (string, error)
	ReadFile func(string) ([]byte, error)
}

// ResolveCodex distinguishes discovery from a trusted execution method. A
// codex.cmd result is detected, but Ready only follows the package metadata
// and node.exe validation path.
func (r CodexResolver) Resolve() ProviderStatus {
	lookPath := r.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	readFile := r.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	path, err := lookPath("codex")
	if err != nil {
		path, err = lookPath("codex.cmd")
	}
	if err != nil {
		return ProviderStatus{Provider: "codex", State: ProviderNotConfigured, ReasonCode: "provider.not_found", Detail: "Codex is not installed or not on PATH"}
	}
	status := ProviderStatus{Provider: "codex", CommandFound: true, State: ProviderDetected, Detail: "Codex launcher detected; execution method needs confirmation"}
	if isWindowsNativeExecutable(path, "codex.exe") {
		status.LaunchTrusted, status.ProfileReady, status.State = true, true, ProviderReady
		status.ResolvedCommand = []string{path}
		return status
	}
	if !strings.EqualFold(filepath.Ext(path), ".cmd") {
		status.ReasonCode = "provider.untrusted_launcher"
		return status
	}
	nodePath, nodeErr := lookPath("node")
	if nodeErr != nil || !isWindowsNativeExecutable(nodePath, "node.exe") {
		status.ReasonCode = "provider.node_required"
		status.Detail = "Codex npm launcher was found; local node.exe is required"
		return status
	}
	root := filepath.Join(filepath.Dir(path), "node_modules", "@openai", "codex")
	manifest, readErr := readFile(filepath.Join(root, "package.json"))
	if readErr != nil {
		status.ReasonCode = "provider.package_metadata_missing"
		status.Detail = "Codex npm package metadata was not found next to the launcher"
		return status
	}
	command, commandErr := BuildCodexNPMCommand(nodePath, root, manifest, nil)
	if commandErr != nil {
		status.ReasonCode = "provider.package_not_trusted"
		status.Detail = commandErr.Error()
		return status
	}
	status.LaunchTrusted, status.ProfileReady, status.State = true, true, ProviderReady
	status.ResolvedCommand = append([]string{command.Executable}, command.Arguments...)
	status.Detail = "Codex uses node.exe and the verified @openai/codex bin/codex.js"
	return status
}

type RunRequest struct {
	Provider string
	Model    string
	Command  TypedCommand
	Worktree string
	Timeout  time.Duration
}

type RunResult struct {
	State         string           `json:"state"`
	ExitCode      int              `json:"exitCode"`
	Structured    map[string]any   `json:"structured,omitempty"`
	Summary       string           `json:"summary,omitempty"`
	Usage         map[string]int64 `json:"usage,omitempty"`
	FailureCode   string           `json:"failureCode,omitempty"`
	RawTranscript bool             `json:"rawTranscript"`
}

type Adapter interface {
	Name() string
	Detect(context.Context) ProviderStatus
	Run(context.Context, RunRequest) RunResult
}

type FakeScenario string

const (
	FakeSuccess         FakeScenario = "success"
	FakeMalformedOutput FakeScenario = "malformed_output"
	FakeTimeout         FakeScenario = "timeout"
	FakeCancelled       FakeScenario = "cancelled"
	FakeAuthFailure     FakeScenario = "auth_failure"
	FakeApprovalPrompt  FakeScenario = "approval_prompt"
	FakeMissingUsage    FakeScenario = "missing_usage"
	FakeNestedLaunch    FakeScenario = "nested_launch"
	FakeProviderFailure FakeScenario = "provider_failure"
)

type FakeAdapter struct {
	Provider string
	Scenario FakeScenario
	Status   ProviderStatus
}

func (f FakeAdapter) Name() string { return f.Provider }
func (f FakeAdapter) Detect(context.Context) ProviderStatus {
	if f.Status.Provider == "" {
		f.Status.Provider = f.Provider
	}
	return f.Status
}
func (f FakeAdapter) Run(ctx context.Context, _ RunRequest) RunResult {
	select {
	case <-ctx.Done():
		return RunResult{State: "cancelled", FailureCode: "provider.cancelled"}
	default:
	}
	switch f.Scenario {
	case FakeMalformedOutput:
		return RunResult{State: "failed", FailureCode: "provider.invalid_output"}
	case FakeTimeout:
		return RunResult{State: "timed_out", FailureCode: "provider.timeout"}
	case FakeCancelled:
		return RunResult{State: "cancelled", FailureCode: "provider.cancelled"}
	case FakeAuthFailure:
		return RunResult{State: "failed", FailureCode: "provider.auth_required"}
	case FakeApprovalPrompt:
		return RunResult{State: "failed", FailureCode: "provider.interactive_prompt"}
	case FakeNestedLaunch:
		return RunResult{State: "succeeded", Structured: map[string]any{"nested": true}, Summary: "nested provider use is recorded as child evidence"}
	case FakeMissingUsage:
		return RunResult{State: "succeeded", Structured: map[string]any{"answer": "fixture"}, Summary: "usage unavailable"}
	case FakeProviderFailure:
		return RunResult{State: "failed", FailureCode: "provider.failed"}
	default:
		return RunResult{State: "succeeded", Structured: map[string]any{"answer": "fixture", "proposal": "bounded"}, Usage: map[string]int64{"input": 10, "output": 5}, Summary: "fake provider completed"}
	}
}

// RunTyped executes only an already validated argv command and reduces output
// to structured JSON. It is separate from FakeAdapter so tests never need a
// real provider process.
func RunTyped(ctx context.Context, request RunRequest, masker *masking.Masker) RunResult {
	if err := request.Command.Validate(); err != nil {
		return RunResult{State: "failed", FailureCode: "provider.invalid_command"}
	}
	if request.Timeout <= 0 {
		request.Timeout = 2 * time.Minute
	}
	if masker == nil {
		masker = masking.New(nil, nil)
	}
	runner := environment.ProcessRunner{OutputLimit: 256 << 10}
	result, err := runner.RunInDirectory(ctx, request.Command.Executable, request.Command.Arguments, environment.AllowlistedEnvironment(nil), request.Worktree, request.Timeout)
	if err != nil {
		return RunResult{State: "failed", ExitCode: result.ExitCode, FailureCode: classifyRunError(ctx, err)}
	}
	text := strings.TrimSpace(masker.Mask(result.Stdout))
	var structured map[string]any
	if err := json.Unmarshal([]byte(text), &structured); err != nil {
		return RunResult{State: "failed", ExitCode: result.ExitCode, FailureCode: "provider.invalid_output"}
	}
	return RunResult{State: "succeeded", ExitCode: result.ExitCode, Structured: structured}
}

func classifyRunError(ctx context.Context, err error) string {
	if errors.Is(ctx.Err(), context.Canceled) {
		return "provider.cancelled"
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timed out") {
		return "provider.timeout"
	}
	return fmt.Sprintf("provider.process_failed")
}
