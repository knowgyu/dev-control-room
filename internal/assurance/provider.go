// Package assurance contains the narrow provider and typed Quality Runner
// contracts used by the application service. It has no shell execution path.
package assurance

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

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

// CodexPromptMaxBytes is intentionally small. Prompts are execution input,
// not durable assurance evidence, and are kept to one bounded argv value.
const CodexPromptMaxBytes = 2000

const (
	codexOutputSchemaDirectory = "runtime/codex"
	codexOutputSchemaFile      = "output-schema.json"
	codexResultSummaryMaxBytes = 2000
	codexFindingMaxBytes       = 1000
	codexFindingsMaxItems      = 20
	codexNextActionMaxBytes    = 2000
	codexWorktreeArgumentIndex = 9
	codexSchemaArgumentIndex   = 11
	codexPromptSeparatorIndex  = 12
)

var codexOutputSchema = []byte(`{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "summary": {"type": "string", "minLength": 1, "maxLength": 2000},
    "findings": {"type": "array", "maxItems": 20, "items": {"type": "string", "maxLength": 1000}},
    "nextAction": {"type": "string", "minLength": 1, "maxLength": 2000}
  },
  "required": ["summary", "findings", "nextAction"]
}`)

type CodexInvocationOptions struct {
	Worktree   string
	SchemaPath string
	Prompt     string
}

// ValidateCodexPrompt normalizes the caller's prompt without retaining it.
// NUL/CR/LF are rejected before trimming so a single-line invariant cannot be
// bypassed with surrounding line breaks. The byte bound is UTF-8 encoded size.
func ValidateCodexPrompt(prompt string) (string, error) {
	if strings.ContainsAny(prompt, "\x00\r\n") {
		return "", errors.New("Codex prompt must be one line without NUL, CR, or LF")
	}
	if !utf8.ValidString(prompt) {
		return "", errors.New("Codex prompt must be valid UTF-8")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", errors.New("Codex prompt is required")
	}
	if len([]byte(prompt)) > CodexPromptMaxBytes {
		return "", fmt.Errorf("Codex prompt must be at most %d UTF-8 bytes", CodexPromptMaxBytes)
	}
	return prompt, nil
}

// CodexOutputSchema returns a copy so callers cannot mutate the fixed schema.
func CodexOutputSchema() []byte {
	return append([]byte(nil), codexOutputSchema...)
}

// VerifyCodexOutputSchema checks the generated schema again at the execution
// boundary. Callers must not substitute a path or schema with the same suffix.
func VerifyCodexOutputSchema(path string) error {
	return verifyCodexOutputSchema(path)
}

// WriteCodexOutputSchema creates the only schema path accepted by the Codex
// command builder. The file contains no prompt or provider output.
func WriteCodexOutputSchema(home string) (string, error) {
	home = strings.TrimSpace(home)
	if !isAbsolutePath(home) || strings.ContainsAny(home, "\x00\r\n") {
		return "", errors.New("Codex schema home must be an absolute local path")
	}
	runtimeDirectory := filepath.Join(home, "runtime")
	directory := filepath.Join(runtimeDirectory, "codex")
	for _, directory := range []string{runtimeDirectory, directory} {
		if info, err := os.Lstat(directory); err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return "", errors.New("Codex schema directory is not a private local directory")
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect Codex schema directory: %w", err)
		}
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return "", fmt.Errorf("create Codex schema directory: %w", err)
		}
		info, err := os.Lstat(directory)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", errors.New("Codex schema directory is not a private local directory")
		}
		if err := os.Chmod(directory, 0o700); err != nil {
			return "", fmt.Errorf("restrict Codex schema directory: %w", err)
		}
	}
	path := filepath.Join(directory, codexOutputSchemaFile)
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", errors.New("Codex schema path is not a regular local file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect Codex schema path: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("open Codex schema file: %w", err)
	}
	if _, err := file.Write(codexOutputSchema); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("write Codex schema file: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("restrict Codex schema file: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close Codex schema file: %w", err)
	}
	return path, nil
}

func verifyCodexOutputSchema(path string) error {
	validated, err := validateCodexSchemaPath(path)
	if err != nil || validated != path {
		return errors.New("Codex output schema path is not controlled")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("Codex output schema is not a regular local file")
	}
	directoryInfo, err := os.Lstat(filepath.Dir(path))
	if err != nil || directoryInfo.Mode()&os.ModeSymlink != 0 || !directoryInfo.IsDir() || (runtime.GOOS != "windows" && directoryInfo.Mode().Perm()&0o077 != 0) {
		return errors.New("Codex output schema directory is not private")
	}
	runtimeInfo, err := os.Lstat(filepath.Dir(filepath.Dir(path)))
	if err != nil || runtimeInfo.Mode()&os.ModeSymlink != 0 || !runtimeInfo.IsDir() || (runtime.GOOS != "windows" && runtimeInfo.Mode().Perm()&0o077 != 0) {
		return errors.New("Codex output schema runtime directory is not private")
	}
	// Windows exposes the read-only attribute through os.FileMode rather than
	// POSIX ACL bits. The file is created/chmod'd 0600 and lives below the
	// app-local home; POSIX mode enforcement remains strict where available.
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return errors.New("Codex output schema permissions are too broad")
	}
	content, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(content, codexOutputSchema) {
		return errors.New("Codex output schema content is not the fixed schema")
	}
	return nil
}

func (c TypedCommand) Validate() error {
	if strings.TrimSpace(c.Executable) == "" || strings.ContainsAny(c.Executable, "\x00\r\n") {
		return errors.New("typed command executable is invalid")
	}
	if forbiddenShellSurface(c.Executable) {
		return errors.New("typed command cannot execute a shell surface")
	}
	for _, arg := range c.Arguments {
		if strings.ContainsAny(arg, "\x00\r\n") {
			return errors.New("typed command argument is invalid")
		}
		if forbiddenShellSurface(arg) {
			return errors.New("typed command cannot execute a shell surface")
		}
	}
	return nil
}

func forbiddenShellSurface(value string) bool {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	base := strings.ToLower(normalized)
	if index := strings.LastIndexByte(base, '/'); index >= 0 {
		base = base[index+1:]
	}
	return base == "cmd" || base == "cmd.exe" || strings.HasSuffix(base, ".cmd") || strings.HasSuffix(base, ".bat")
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
	entry = strings.ReplaceAll(strings.TrimSpace(entry), "\\", "/")
	if !validRelativeCodexScript(entry) {
		return "", errors.New("codex package bin must be a relative codex.js path")
	}
	return entry, nil
}

func validRelativeCodexScript(entry string) bool {
	if entry == "" || strings.HasPrefix(entry, "/") || windowsAbsolutePath.MatchString(entry) {
		return false
	}
	parts := strings.Split(entry, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return strings.Join(parts, "/") == "bin/codex.js"
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
	if !validAbsoluteCodexScript(script) {
		return TypedCommand{}, errors.New("Codex package script is not codex.js")
	}
	command := TypedCommand{Executable: nodePath, Arguments: append([]string{script}, args...)}
	if err := command.Validate(); err != nil {
		return TypedCommand{}, err
	}
	return command, nil
}

func isWindowsNativeExecutable(path, base string) bool {
	path = strings.TrimSpace(path)
	if !windowsAbsolutePath.MatchString(path) || !strings.EqualFold(filepath.Base(strings.ReplaceAll(path, "\\", "/")), base) || strings.Contains(path[3:], ":") {
		return false
	}
	for _, segment := range strings.Split(strings.ReplaceAll(path[3:], "/", "\\"), "\\") {
		if segment == "" || segment == "." || segment == ".." || strings.HasSuffix(segment, " ") || strings.HasSuffix(segment, ".") {
			return false
		}
	}
	return true
}

type CodexResolver struct {
	LookPath func(string) (string, error)
	ReadFile func(string) ([]byte, error)
	// StatFile returns the mode from an Lstat-style check. The resolver must
	// distinguish a regular local file from a symlink before reporting Ready.
	StatFile func(string) (os.FileMode, error)
}

// ResolveCodex distinguishes discovery from a trusted execution method. An
// npm shim result (.cmd or .ps1) is only a location hint; Ready follows only
// the package metadata, readable entry, and node.exe validation path.
func (r CodexResolver) Resolve() ProviderStatus {
	lookPath := r.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	readFile := r.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	// Do not ask the platform to resolve bare "codex": PATH/PATHEXT can
	// select a shadow executable. npm's Windows shims are location hints only.
	path, err := lookPath("codex.cmd")
	if err != nil {
		path, err = lookPath("codex.ps1")
	}
	if err != nil {
		return ProviderStatus{Provider: "codex", State: ProviderNotConfigured, ReasonCode: "provider.not_found", Detail: "Codex is not installed or not on PATH"}
	}
	status := ProviderStatus{Provider: "codex", CommandFound: true, State: ProviderDetected, Detail: "Codex launcher detected; execution method needs confirmation"}
	if !isCodexNPMLauncher(path) {
		status.ReasonCode = "provider.untrusted_launcher"
		status.Detail = "Codex must be launched through a verified npm package with node.exe"
		return status
	}
	nodePath, nodeErr := lookPath("node")
	if nodeErr != nil || !isWindowsNativeExecutable(nodePath, "node.exe") {
		status.ReasonCode = "provider.node_required"
		status.Detail = "Codex npm launcher was found; local node.exe is required"
		return status
	}
	statFile := r.StatFile
	if statFile == nil {
		statFile = func(path string) (os.FileMode, error) {
			info, err := os.Lstat(path)
			if err != nil {
				return 0, err
			}
			return info.Mode(), nil
		}
	}
	nodeMode, statErr := statFile(nodePath)
	if statErr != nil {
		status.ReasonCode = "provider.node_missing"
		status.Detail = "Codex npm launcher requires a local node.exe file"
		return status
	}
	if !nodeMode.IsRegular() || nodeMode&os.ModeSymlink != 0 {
		status.ReasonCode = "provider.node_not_regular"
		status.Detail = "Codex npm launcher requires a regular, non-symlink node.exe"
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
	scriptMode, statErr := statFile(command.Arguments[0])
	if statErr != nil {
		status.ReasonCode = "provider.package_entry_missing"
		status.Detail = "Codex package bin/codex.js was not found next to the launcher"
		return status
	}
	if !scriptMode.IsRegular() || scriptMode&os.ModeSymlink != 0 {
		status.ReasonCode = "provider.package_entry_not_regular"
		status.Detail = "Codex package bin/codex.js must be a regular, non-symlink file"
		return status
	}
	if _, readErr := readFile(command.Arguments[0]); readErr != nil {
		status.ReasonCode = "provider.package_entry_unreadable"
		status.Detail = "Codex package bin/codex.js could not be read"
		return status
	}
	status.LaunchTrusted, status.ProfileReady, status.State = true, true, ProviderReady
	status.ResolvedCommand = append([]string{command.Executable}, command.Arguments...)
	status.Detail = "Codex uses node.exe and the verified @openai/codex bin/codex.js"
	return status
}

func isCodexNPMLauncher(path string) bool {
	if !windowsAbsolutePath.MatchString(strings.TrimSpace(path)) {
		return false
	}
	normalized := strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
	base := normalized
	if index := strings.LastIndexByte(base, '/'); index >= 0 {
		base = base[index+1:]
	}
	ext := ""
	if index := strings.LastIndexByte(base, '.'); index >= 0 {
		ext = strings.ToLower(base[index:])
	}
	// npm emits both codex.cmd and codex.ps1 on Windows. Neither is ever
	// executed; they are discovery anchors for the adjacent package only.
	return (ext == ".cmd" || ext == ".ps1") && strings.EqualFold(base[:len(base)-len(ext)], "codex")
}

type RunRequest struct {
	Provider string
	Model    string
	Command  TypedCommand
	Worktree string
	Timeout  time.Duration
}

// TypedRunner is the narrow process seam used by the application service. A
// production execution uses RunTyped; tests can inject a recorder without
// starting a provider process.
type TypedRunner func(context.Context, RunRequest, *masking.Masker) RunResult

// CodexExecution carries the two dependencies needed by the application
// service. The context helper is an explicit test seam; the default remains
// the real resolver and typed runner.
type CodexExecution struct {
	Resolver func() ProviderStatus
	Runner   TypedRunner
}

type codexExecutionContextKey struct{}

func WithCodexExecution(ctx context.Context, execution CodexExecution) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, codexExecutionContextKey{}, execution)
}

func CodexExecutionFromContext(ctx context.Context) CodexExecution {
	execution := CodexExecution{Resolver: CodexResolver{}.Resolve, Runner: RunTyped}
	if ctx == nil {
		return execution
	}
	if injected, ok := ctx.Value(codexExecutionContextKey{}).(CodexExecution); ok {
		if injected.Resolver != nil {
			execution.Resolver = injected.Resolver
		}
		if injected.Runner != nil {
			execution.Runner = injected.Runner
		}
	}
	return execution
}

// BuildCodexInvocationCommand accepts only the resolver's two-element trusted
// command [node.exe, absolute ...\\bin\\codex.js]. Provider arguments are
// fixed typed values, never interpolated into a shell command. The prompt is
// the final positional argument after an explicit argv separator, so a leading
// hyphen can never turn it into a CLI option.
func BuildCodexInvocationCommand(status ProviderStatus, model string, options CodexInvocationOptions) (TypedCommand, error) {
	if status.State != ProviderReady || !status.CommandFound || !status.LaunchTrusted || !status.ProfileReady {
		return TypedCommand{}, errors.New("Codex provider does not have a trusted launcher")
	}
	if len(status.ResolvedCommand) != 2 || !isWindowsNativeExecutable(status.ResolvedCommand[0], "node.exe") || !validAbsoluteCodexScript(status.ResolvedCommand[1]) {
		return TypedCommand{}, errors.New("Codex provider resolved command is not node.exe plus bin/codex.js")
	}
	worktree, err := validateCodexWorktree(options.Worktree)
	if err != nil {
		return TypedCommand{}, err
	}
	schemaPath, err := validateCodexSchemaPath(options.SchemaPath)
	if err != nil {
		return TypedCommand{}, err
	}
	prompt, err := ValidateCodexPrompt(options.Prompt)
	if err != nil {
		return TypedCommand{}, err
	}
	model, err = validateCodexModel(model)
	if err != nil {
		return TypedCommand{}, err
	}
	arguments := []string{
		status.ResolvedCommand[1],
		"exec",
		"--json",
		"--sandbox", "read-only",
		"--ephemeral",
		"--ignore-user-config",
		"--ignore-rules",
		"--cd", worktree,
		"--output-schema", schemaPath,
	}
	if model != "" {
		arguments = append(arguments, "--model", model)
	}
	arguments = append(arguments, "--", prompt)
	command := TypedCommand{Executable: status.ResolvedCommand[0], Arguments: arguments}
	if err := ValidateCodexInvocationCommand(command); err != nil {
		return TypedCommand{}, err
	}
	return command, nil
}

// ValidateCodexInvocationCommand re-checks the complete argv shape immediately
// before execution. This prevents a future runner seam or caller from adding a
// shell, approval, sandbox, config, or arbitrary provider flag.
func ValidateCodexInvocationCommand(command TypedCommand) error {
	if !isWindowsNativeExecutable(command.Executable, "node.exe") || len(command.Arguments) < codexPromptSeparatorIndex+2 {
		return errors.New("Codex command must use an absolute node.exe launcher and fixed argv")
	}
	if !validAbsoluteCodexScript(command.Arguments[0]) {
		return errors.New("Codex command script is not the verified codex.js")
	}
	fixed := []string{"exec", "--json", "--sandbox", "read-only", "--ephemeral", "--ignore-user-config", "--ignore-rules", "--cd"}
	for index, want := range fixed {
		if command.Arguments[index+1] != want {
			return errors.New("Codex command contains an unexpected control or ordering")
		}
	}
	worktreeIndex := codexWorktreeArgumentIndex
	worktree, err := validateCodexWorktree(command.Arguments[worktreeIndex])
	if err != nil || worktree != command.Arguments[worktreeIndex] {
		return errors.New("Codex command worktree is not an explicit absolute path")
	}
	if command.Arguments[worktreeIndex+1] != "--output-schema" {
		return errors.New("Codex command must use the fixed output schema flag")
	}
	schemaIndex := codexSchemaArgumentIndex
	schemaPath, err := validateCodexSchemaPath(command.Arguments[schemaIndex])
	if err != nil || schemaPath != command.Arguments[schemaIndex] {
		return errors.New("Codex command schema path is not controlled")
	}
	index := schemaIndex + 1
	if index < len(command.Arguments) && command.Arguments[index] == "--model" {
		if index+1 >= len(command.Arguments) {
			return errors.New("Codex command model is missing")
		}
		model, err := validateCodexModel(command.Arguments[index+1])
		if err != nil || model != command.Arguments[index+1] {
			return errors.New("Codex command model is invalid")
		}
		index += 2
	}
	if index >= len(command.Arguments) || command.Arguments[index] != "--" {
		return errors.New("Codex command must use the fixed prompt separator")
	}
	index++
	if index != len(command.Arguments)-1 {
		return errors.New("Codex command must end with one prompt argument")
	}
	if _, err := ValidateCodexPrompt(command.Arguments[index]); err != nil {
		return err
	}
	return command.Validate()
}

func validateCodexModel(model string) (string, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", nil
	}
	if !utf8.ValidString(model) || len([]byte(model)) > 256 || strings.ContainsAny(model, "\x00\r\n") || strings.HasPrefix(model, "-") || forbiddenShellSurface(model) {
		return "", errors.New("Codex model is not a safe value")
	}
	return model, nil
}

func validateCodexWorktree(worktree string) (string, error) {
	worktree = strings.TrimSpace(worktree)
	if !isAbsolutePath(worktree) || strings.ContainsAny(worktree, "\x00\r\n") {
		return "", errors.New("Codex worktree must be an absolute local path")
	}
	return worktree, nil
}

func validateCodexSchemaPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if !isAbsolutePath(path) || strings.ContainsAny(path, "\x00\r\n") {
		return "", errors.New("Codex output schema must be an absolute local path")
	}
	normalized := strings.ReplaceAll(path, "\\", "/")
	parts := strings.Split(normalized, "/")
	if !strings.HasSuffix(strings.ToLower(normalized), "/"+codexOutputSchemaDirectory+"/"+codexOutputSchemaFile) {
		return "", errors.New("Codex output schema path is not the app runtime schema")
	}
	for _, part := range parts {
		if part == "." || part == ".." {
			return "", errors.New("Codex output schema path contains traversal")
		}
	}
	return path, nil
}

func isAbsolutePath(path string) bool {
	return filepath.IsAbs(path) || windowsAbsolutePath.MatchString(path)
}

func validAbsoluteCodexScript(path string) bool {
	if !windowsAbsolutePath.MatchString(strings.TrimSpace(path)) {
		return false
	}
	normalized := strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
	if forbiddenShellSurface(normalized) {
		return false
	}
	parts := strings.Split(normalized, "/")
	for index, part := range parts {
		if index > 0 && (part == "" || part == "." || part == "..") {
			return false
		}
	}
	return strings.HasSuffix(strings.ToLower(strings.Join(parts, "/")), "/node_modules/@openai/codex/bin/codex.js")
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

// RunTyped executes only an already validated argv command and reduces Codex
// JSONL output to a bounded structured result. It is separate from FakeAdapter
// so tests never need a real provider process.
func RunTyped(ctx context.Context, request RunRequest, masker *masking.Masker) RunResult {
	if strings.EqualFold(request.Provider, "codex") {
		if err := ValidateCodexInvocationCommand(request.Command); err != nil {
			return RunResult{State: "failed", FailureCode: "provider.invalid_command"}
		}
		if request.Worktree == "" || request.Worktree != request.Command.Arguments[codexWorktreeArgumentIndex] {
			return RunResult{State: "failed", FailureCode: "provider.invalid_command"}
		}
		if err := verifyCodexOutputSchema(request.Command.Arguments[codexSchemaArgumentIndex]); err != nil {
			return RunResult{State: "failed", FailureCode: "provider.invalid_command"}
		}
	} else if err := request.Command.Validate(); err != nil {
		return RunResult{State: "failed", FailureCode: "provider.invalid_command"}
	}
	if request.Timeout <= 0 {
		request.Timeout = 2 * time.Minute
	}
	if masker == nil {
		masker = masking.New(nil, nil)
	}
	runner := environment.ProcessRunner{OutputLimit: 256 << 10}
	result, err := runner.RunInDirectory(ctx, request.Command.Executable, request.Command.Arguments, environment.AllowlistedEnvironment([]string{"CODEX_HOME"}), request.Worktree, request.Timeout)
	if err != nil {
		failureCode := classifyRunError(ctx, err)
		state := "failed"
		if failureCode == "provider.timeout" {
			state = "timed_out"
		} else if failureCode == "provider.cancelled" {
			state = "cancelled"
		}
		return RunResult{State: state, ExitCode: result.ExitCode, FailureCode: failureCode}
	}
	structured, usage, err := ParseCodexOutputWithUsage(result.Stdout, masker)
	if err != nil {
		return RunResult{State: "failed", ExitCode: result.ExitCode, FailureCode: "provider.invalid_output"}
	}
	return RunResult{State: "succeeded", ExitCode: result.ExitCode, Structured: structured, Usage: usage}
}

// ParseCodexOutput accepts only a bounded Codex JSONL stream with a supported
// final result. Human text and incomplete/unknown event streams fail closed
// rather than being persisted as a transcript.
func ParseCodexOutput(stdout string, masker *masking.Masker) (map[string]any, error) {
	structured, _, err := ParseCodexOutputWithUsage(stdout, masker)
	return structured, err
}

// ParseCodexOutputWithUsage reduces Codex exec's bounded JSONL event stream to
// the final result and numeric usage only. It deliberately requires a final
// turn.completed event with a result-bearing agent message (or an equivalent
// result field); unknown or incomplete event streams are rejected.
func ParseCodexOutputWithUsage(stdout string, masker *masking.Masker) (map[string]any, map[string]int64, error) {
	if masker == nil {
		masker = masking.New(nil, nil)
	}
	const maxOutput = 256 << 10
	const maxLine = 128 << 10
	scanner := bufio.NewScanner(strings.NewReader(stdout))
	scanner.Buffer(make([]byte, 64<<10), maxLine)
	var totalBytes int
	var finalResult any
	var finalResultSeen bool
	var finalSeen bool
	var eventCount int
	usage := make(map[string]int64)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		totalBytes += len(line) + 1
		if totalBytes > maxOutput {
			return nil, nil, errors.New("Codex output exceeded the bounded JSONL limit")
		}
		if line == "" {
			continue
		}
		if finalSeen {
			return nil, nil, errors.New("Codex output continued after turn.completed")
		}
		decoder := json.NewDecoder(strings.NewReader(line))
		decoder.UseNumber()
		event := make(map[string]any)
		if err := decoder.Decode(&event); err != nil || event == nil {
			return nil, nil, errors.New("Codex output contains an invalid JSONL event")
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			return nil, nil, errors.New("Codex output contains more than one JSON value per line")
		}
		eventType, ok := event["type"].(string)
		if !ok || strings.TrimSpace(eventType) == "" {
			return nil, nil, errors.New("Codex output event type is missing")
		}
		eventCount++
		switch eventType {
		case "thread.started":
			// Thread identifiers are execution metadata, not part of the result.
		case "turn.started", "item.started":
			// Progress events are intentionally discarded.
		case "item.completed":
			item, ok := event["item"].(map[string]any)
			if !ok {
				return nil, nil, errors.New("Codex item.completed event is malformed")
			}
			itemType, _ := item["type"].(string)
			if itemType != "agent_message" && itemType != "message" {
				continue
			}
			if value, ok := item["result"]; ok {
				finalResult = value
				finalResultSeen = true
			} else if text, ok := item["text"].(string); ok && strings.TrimSpace(text) != "" {
				finalResult = text
				finalResultSeen = true
			} else {
				return nil, nil, errors.New("Codex agent message has no result")
			}
		case "turn.completed":
			finalSeen = true
			if value, ok := event["result"]; ok {
				finalResult = value
				finalResultSeen = true
			} else if text, ok := event["text"].(string); ok && strings.TrimSpace(text) != "" {
				finalResult = text
				finalResultSeen = true
			}
			if rawUsage, ok := event["usage"].(map[string]any); ok {
				if err := collectUsage(rawUsage, usage); err != nil {
					return nil, nil, err
				}
			}
		default:
			return nil, nil, fmt.Errorf("Codex output event type %q is unsupported", eventType)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, errors.New("Codex output could not be bounded and decoded")
	}
	if eventCount == 0 || !finalSeen || !finalResultSeen {
		return nil, nil, errors.New("Codex output has no supported final result")
	}
	structured, err := reduceCodexSchemaResult(finalResult, masker)
	if err != nil {
		return nil, nil, err
	}
	if len(usage) > 0 {
		if _, ok := usage["total"]; !ok {
			if input, inputOK := usage["input"]; inputOK {
				if output, outputOK := usage["output"]; outputOK && input <= math.MaxInt64-output {
					usage["total"] = input + output
				}
			}
		}
	}
	return structured, usage, nil
}

func reduceCodexSchemaResult(value any, masker *masking.Masker) (map[string]any, error) {
	if text, ok := value.(string); ok {
		decoder := json.NewDecoder(strings.NewReader(text))
		decoder.UseNumber()
		var decoded any
		if err := decoder.Decode(&decoded); err != nil {
			return nil, errors.New("Codex final message is not valid schema JSON")
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			return nil, errors.New("Codex final message contains more than one JSON value")
		}
		value = decoded
	}
	object, ok := value.(map[string]any)
	if !ok || len(object) != 3 {
		return nil, errors.New("Codex final result does not match the fixed schema")
	}
	for key := range object {
		if key != "summary" && key != "findings" && key != "nextAction" {
			return nil, errors.New("Codex final result contains an unknown schema field")
		}
	}
	summary, ok := object["summary"].(string)
	if !ok || strings.TrimSpace(summary) == "" || !validSchemaText(summary, codexResultSummaryMaxBytes) {
		return nil, errors.New("Codex final result summary is invalid")
	}
	nextAction, ok := object["nextAction"].(string)
	if !ok || strings.TrimSpace(nextAction) == "" || !validSchemaText(nextAction, codexNextActionMaxBytes) {
		return nil, errors.New("Codex final result nextAction is invalid")
	}
	findings, ok := object["findings"].([]any)
	if !ok || len(findings) > codexFindingsMaxItems {
		return nil, errors.New("Codex final result findings are invalid")
	}
	maskedFindings := make([]any, len(findings))
	for index, item := range findings {
		finding, ok := item.(string)
		if !ok || !validSchemaText(finding, codexFindingMaxBytes) {
			return nil, errors.New("Codex final result contains an invalid finding")
		}
		maskedFindings[index] = masker.Mask(finding)
	}
	return map[string]any{
		"summary":    masker.Mask(summary),
		"findings":   maskedFindings,
		"nextAction": masker.Mask(nextAction),
	}, nil
}

func validSchemaText(value string, maxBytes int) bool {
	return utf8.ValidString(value) && len([]byte(value)) <= maxBytes
}

func collectUsage(raw map[string]any, usage map[string]int64) error {
	fields := map[string][]string{
		"input":     {"input_tokens", "inputTokens", "input"},
		"cached":    {"cached_input_tokens", "cachedInputTokens", "cached_input", "cached"},
		"output":    {"output_tokens", "outputTokens", "output"},
		"reasoning": {"reasoning_tokens", "reasoningTokens", "reasoning_output_tokens", "reasoning"},
		"tool":      {"tool_tokens", "toolTokens", "tool"},
		"total":     {"total_tokens", "totalTokens", "total"},
	}
	for normalized, candidates := range fields {
		for _, candidate := range candidates {
			value, ok := raw[candidate]
			if !ok {
				continue
			}
			number, ok := value.(json.Number)
			if !ok {
				return errors.New("Codex usage contains a non-numeric field")
			}
			parsed, err := number.Int64()
			if err != nil || parsed < 0 {
				return errors.New("Codex usage contains an invalid integer")
			}
			usage[normalized] = parsed
			break
		}
	}
	return nil
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
