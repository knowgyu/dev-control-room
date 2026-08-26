// Package environment contains read-only Windows environment diagnostics.
// It intentionally returns metadata only; command output and environment
// values never cross this package boundary.
package environment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/masking"
)

const (
	DefaultTimeout = 8 * time.Second
	DefaultOutput  = 64 << 10
)

type ToolStatus struct {
	Name         string `json:"name"`
	Available    bool   `json:"available"`
	Required     bool   `json:"required"`
	State        string `json:"state"` // available, unavailable, optional
	CommandType  string `json:"commandType,omitempty"`
	ResolvedPath string `json:"resolvedPath,omitempty"`
	Version      string `json:"version,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

type ProfileStatus struct {
	ID           string   `json:"id"`
	Available    bool     `json:"available"`
	Required     bool     `json:"required"`
	State        string   `json:"state"` // available, unavailable, optional
	LaunchMode   string   `json:"launchMode"`
	CommandType  string   `json:"commandType,omitempty"`
	ResolvedPath string   `json:"resolvedPath,omitempty"`
	Version      string   `json:"version,omitempty"`
	Findings     []string `json:"findings,omitempty"`
	Reason       string   `json:"reason,omitempty"`
}

type EnvironmentVariableStatus struct {
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	State     string   `json:"state"` // declared, missing, conflict, duplicate
	Consumers []string `json:"consumers,omitempty"`
}

type ConnectorStatus struct {
	ID                     string `json:"id"`
	Configured             bool   `json:"configured"`
	SecretReferencePresent bool   `json:"secretReferencePresent"`
	LastValidationResult   string `json:"lastValidationResult,omitempty"`
	Reason                 string `json:"reason,omitempty"`
}

type Finding struct {
	Type                  string `json:"type"`
	Severity              string `json:"severity"`
	Target                string `json:"target,omitempty"`
	Summary               string `json:"summary"`
	RecommendedNextAction string `json:"recommendedNextAction"`
}

type Health struct {
	GeneratedAt time.Time                   `json:"generatedAt"`
	Available   bool                        `json:"available"`
	Tools       []ToolStatus                `json:"tools"`
	Profiles    []ProfileStatus             `json:"profiles"`
	Environment []EnvironmentVariableStatus `json:"environment"`
	Connectors  []ConnectorStatus           `json:"connectors"`
	Findings    []Finding                   `json:"findings"`
}

type Runner interface {
	Run(context.Context, string, []string, []string, time.Duration) (Result, error)
}

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type ProcessRunner struct {
	OutputLimit int
}

type LaunchResult struct {
	PID int
}

type Launcher interface {
	Launch(context.Context, string, []string, []string, string) (LaunchResult, error)
}

// ProcessLauncher starts a reviewed argv command without collecting its I/O.
// The caller owns the reviewed command and working-directory boundary.
type ProcessLauncher struct{}

func (ProcessLauncher) Launch(parent context.Context, executable string, args []string, env []string, directory string) (LaunchResult, error) {
	if strings.TrimSpace(executable) == "" {
		return LaunchResult{}, errors.New("executable is required")
	}
	if strings.TrimSpace(directory) == "" {
		return LaunchResult{}, errors.New("working directory is required")
	}
	if info, err := os.Stat(directory); err != nil || !info.IsDir() {
		return LaunchResult{}, errors.New("working directory is unavailable")
	}
	command := exec.CommandContext(context.WithoutCancel(parent), executable, args...)
	prepareDetachedCommand(command)
	command.Env = append([]string(nil), env...)
	command.Dir = filepath.Clean(directory)
	if err := command.Start(); err != nil {
		return LaunchResult{}, err
	}
	pid := command.Process.Pid
	if err := command.Process.Release(); err != nil {
		return LaunchResult{}, err
	}
	return LaunchResult{PID: pid}, nil
}

func (r ProcessRunner) Run(parent context.Context, executable string, args []string, env []string, timeout time.Duration) (Result, error) {
	return r.RunInDirectory(parent, executable, args, env, safeWorkingDirectory(env), timeout)
}

// SafeEnvironment returns the reviewed process environment used by diagnostics
// and detached launches. It never returns the complete parent environment.
func SafeEnvironment(allowlist []string) []string {
	return safeEnvironment(allowlist)
}

// RunInDirectory executes one typed argv command in an explicit directory.
// It preserves the same process-tree, environment, timeout, and output bounds
// as the diagnostic runner while allowing Action execution to bind to a
// verified Worktree instead of a temporary directory.
func (r ProcessRunner) RunInDirectory(parent context.Context, executable string, args []string, env []string, directory string, timeout time.Duration) (Result, error) {
	if strings.TrimSpace(executable) == "" {
		return Result{}, errors.New("executable is required")
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	limit := r.OutputLimit
	if limit <= 0 {
		limit = DefaultOutput
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	command := exec.CommandContext(ctx, executable, args...)
	prepareCommand(command)
	var cancelMu sync.RWMutex
	cancelProcess := func() error { return terminateProcessTree(command) }
	command.Cancel = func() error {
		cancelMu.RLock()
		cancel := cancelProcess
		cancelMu.RUnlock()
		return cancel()
	}
	// A nil Cmd.Env inherits the complete parent environment. Allocate even for
	// an empty allowlist result so the diagnostic child can never fall back to
	// implicit inheritance.
	command.Env = make([]string, len(env))
	copy(command.Env, env)
	if strings.TrimSpace(directory) != "" {
		if info, err := os.Stat(directory); err != nil || !info.IsDir() {
			return Result{}, errors.New("working directory is unavailable")
		}
		command.Dir = filepath.Clean(directory)
	} else {
		command.Dir = safeWorkingDirectory(env)
	}
	var stdout, stderr boundedBuffer
	stdout.limit, stderr.limit = limit, limit
	stdout.cancel, stderr.cancel = cancel, cancel
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Start()
	cleanup := func() {}
	if err == nil {
		var attachErr error
		var attachedCancel func() error
		cleanup, attachedCancel, attachErr = attachProcessTree(command)
		cancelMu.Lock()
		cancelProcess = attachedCancel
		cancelMu.Unlock()
		if attachErr != nil {
			_ = command.Process.Kill()
			_ = command.Wait()
			return Result{}, errors.New("process tree could not be isolated")
		}
		err = command.Wait()
	}
	cleanup()
	if stdout.limited || stderr.limited {
		return Result{}, errors.New("process output exceeded bounded limit")
	}
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Result{}, fmt.Errorf("process timed out after %s", timeout)
		}
		return Result{}, ctx.Err()
	}
	result := Result{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: exitCode(err)}
	if err != nil {
		return result, err
	}
	return result, nil
}

type boundedBuffer struct {
	bytes.Buffer
	limit   int
	cancel  context.CancelFunc
	limited bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		b.limited = true
		if b.cancel != nil {
			b.cancel()
		}
		return 0, errors.New("process output exceeded bounded limit")
	}
	return b.Buffer.Write(p)
}

func (b *boundedBuffer) ReadFrom(reader io.Reader) (int64, error) {
	var total int64
	buffer := make([]byte, 32<<10)
	for {
		count, err := reader.Read(buffer)
		if count > 0 {
			written, writeErr := b.Write(buffer[:count])
			total += int64(written)
			if writeErr != nil {
				return total, writeErr
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return total, nil
			}
			return total, err
		}
	}
}

type Doctor struct {
	Runner   Runner
	Masker   *masking.Masker
	Now      func() time.Time
	LookPath func(string) (string, error)
}

func NewDoctor(runner Runner, masker *masking.Masker) Doctor {
	if runner == nil {
		runner = ProcessRunner{}
	}
	if masker == nil {
		masker = masking.New(nil, []string{"TOKEN", "PASSWORD", "SECRET", "API_KEY", "AUTHORIZATION"})
	}
	return Doctor{Runner: runner, Masker: masker, Now: func() time.Time { return time.Now().UTC() }, LookPath: exec.LookPath}
}

func (d Doctor) Run(ctx context.Context, profiles []domain.AgentProfile, declarations []domain.EnvironmentDeclaration, connectors []domain.ConnectorReference) Health {
	now := time.Now().UTC()
	if d.Now != nil {
		now = d.Now().UTC()
	}
	health := Health{GeneratedAt: now, Available: true, Tools: []ToolStatus{}, Profiles: []ProfileStatus{}, Environment: []EnvironmentVariableStatus{}, Connectors: []ConnectorStatus{}, Findings: []Finding{}}
	for _, name := range []string{"pwsh", "git", "gh", "codex", "claude", "gemini"} {
		status := d.resolveDirect(ctx, name, []string{"--version"}, 0)
		status.Required = requiredTool(name)
		if status.Available {
			status.State = "available"
		} else if status.Required {
			status.State = "unavailable"
		} else {
			status.State = "optional"
		}
		health.Tools = append(health.Tools, status)
		if !status.Available {
			finding := unavailableFinding("tool."+name, name, status.Reason)
			if !status.Required {
				finding.Severity = "info"
				finding.RecommendedNextAction = "필요한 경우 Provider를 설정한 뒤 다시 점검합니다."
			} else {
				health.Available = false
			}
			health.Findings = append(health.Findings, finding)
		}
	}
	requiredProfileIDs := referencedProfileIDs(declarations)
	for _, profile := range profiles {
		status := d.resolveProfile(ctx, profile)
		status.Required = !isOptionalDefaultAgentProfile(profile) || requiredProfileIDs[profile.Metadata.ID]
		if status.Available {
			status.State = "available"
		} else if status.Required {
			status.State = "unavailable"
		} else {
			status.State = "optional"
		}
		health.Profiles = append(health.Profiles, status)
		if !status.Available {
			for _, finding := range status.Findings {
				profileFinding := unavailableFinding("agent_profile."+profile.Metadata.ID, profile.Metadata.ID, finding)
				if !status.Required {
					profileFinding.Severity = "info"
					profileFinding.RecommendedNextAction = "필요한 경우 Provider를 설정한 뒤 다시 점검합니다."
				} else {
					health.Available = false
				}
				health.Findings = append(health.Findings, profileFinding)
			}
		}
	}
	health.Environment, health.Findings = inspectEnvironment(declarations, health.Findings)
	for _, status := range health.Environment {
		if status.State != "declared" {
			health.Available = false
		}
	}
	for _, connector := range connectors {
		status := d.connectorStatus(connector)
		health.Connectors = append(health.Connectors, status)
		if status.SecretReferencePresent && (status.LastValidationResult == "failed" || status.LastValidationResult == "unavailable") {
			status.Reason = "last connector validation did not succeed"
			health.Connectors[len(health.Connectors)-1] = status
		}
		if !status.SecretReferencePresent || status.Reason != "" {
			health.Available = false
			health.Findings = append(health.Findings, unavailableFinding("connector."+connector.ID, connector.ID, status.Reason))
		}
	}
	sort.Slice(health.Findings, func(i, j int) bool { return health.Findings[i].Type < health.Findings[j].Type })
	return health
}

func requiredTool(name string) bool {
	return name == "pwsh" || name == "git"
}

func referencedProfileIDs(declarations []domain.EnvironmentDeclaration) map[string]bool {
	ids := make(map[string]bool)
	for _, declaration := range declarations {
		if id := strings.TrimSpace(declaration.ProfileID); id != "" {
			ids[id] = true
		}
	}
	return ids
}

// isOptionalDefaultAgentProfile identifies profiles seeded by the application
// before the operator chooses a provider. A changed command or launch
// configuration is an explicit profile configuration and remains required.
func isOptionalDefaultAgentProfile(profile domain.AgentProfile) bool {
	id := strings.ToLower(strings.TrimSpace(profile.Metadata.ID))
	command := strings.ToLower(strings.TrimSpace(profile.Spec.Command))
	expectedMode := domain.AgentLaunchDirect
	expectedTimeout := 8
	switch id {
	case "codex":
		expectedTimeout = 120
	case "claude", "gemini":
	case "claude-local":
		expectedMode = domain.AgentLaunchPowerShellProfile
	default:
		return false
	}
	if command != id || profile.Spec.LaunchMode != expectedMode || profile.Spec.DataBoundary != domain.AgentBoundaryLocal {
		return false
	}
	if profile.Spec.TimeoutSeconds != 0 && profile.Spec.TimeoutSeconds != expectedTimeout {
		return false
	}
	return len(profile.Spec.VersionProbe) == 0 && strings.TrimSpace(profile.Spec.ModelArgumentTemplate) == "" && len(profile.Spec.EnvironmentAllowlist) == 0
}

func (d Doctor) resolveDirect(ctx context.Context, name string, probe []string, timeoutSeconds int) ToolStatus {
	status := ToolStatus{Name: name, CommandType: "Application"}
	if len(probe) == 0 {
		probe = []string{"--version"}
	}
	lookPath := d.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	path, err := lookPath(name)
	if err != nil && name == "pwsh" {
		path, err = lookPath("pwsh.exe")
	}
	if err != nil {
		status.Reason = "command not found"
		return status
	}
	if !isLocalWindowsNativeExecutable(path) {
		status.Reason = "resolved executable is not a local Windows .exe"
		return status
	}
	status.ResolvedPath = filepath.Clean(path)
	result, err := d.Runner.Run(ctx, path, probe, safeEnvironment(nil), duration(timeoutSeconds))
	if err != nil {
		status.Reason = classifyProcessError(ctx, err)
		return status
	}
	if result.ExitCode != 0 {
		status.Reason = "version probe failed"
		return status
	}
	status.Version = parseVersion(d.Masker.Mask(result.Stdout + "\n" + result.Stderr))
	if status.Version == "" {
		status.Reason = "version unavailable"
		return status
	}
	status.Available = true
	return status
}

func (d Doctor) resolveProfile(ctx context.Context, profile domain.AgentProfile) ProfileStatus {
	status := ProfileStatus{ID: profile.Metadata.ID, Required: true, LaunchMode: string(profile.Spec.LaunchMode)}
	if err := profile.Validate(); err != nil {
		status.Findings = []string{"invalid profile configuration"}
		status.Reason = "invalid profile configuration"
		return status
	}
	timeout := profile.Spec.TimeoutSeconds
	switch profile.Spec.LaunchMode {
	case domain.AgentLaunchDirect:
		tool := d.resolveDirect(ctx, profile.Spec.Command, profile.Spec.VersionProbe, timeout)
		status.Available, status.CommandType, status.ResolvedPath, status.Version, status.Reason = tool.Available, tool.CommandType, tool.ResolvedPath, tool.Version, tool.Reason
		if !status.Available {
			status.Findings = []string{tool.Reason}
		}
	case domain.AgentLaunchPowerShellProfile:
		if !safeCommandName(profile.Spec.Command) {
			status.Reason = "unsupported PowerShell command name"
			status.Findings = []string{status.Reason}
			return status
		}
		lookPath := d.LookPath
		if lookPath == nil {
			lookPath = exec.LookPath
		}
		pwsh, err := lookPath("pwsh")
		if err != nil {
			pwsh, err = lookPath("pwsh.exe")
		}
		if err != nil {
			status.Reason = "PowerShell 7 command not found"
			status.Findings = []string{status.Reason}
			return status
		}
		if !isLocalWindowsNativeExecutable(pwsh) {
			status.Reason = "resolved PowerShell is not a local Windows .exe"
			status.Findings = []string{status.Reason}
			return status
		}
		status.CommandType = "PowerShellProfile"
		status.ResolvedPath = filepath.Clean(pwsh)
		probe := profile.Spec.VersionProbe
		if len(probe) == 0 {
			probe = []string{"--version"}
		}
		result, runErr := d.Runner.Run(ctx, pwsh, []string{"-NoLogo", "-Command", profileProbeScript(profile.Spec.Command, probe)}, safeEnvironment(profile.Spec.EnvironmentAllowlist), duration(timeout))
		if runErr != nil {
			status.Reason = classifyProcessError(ctx, runErr)
			status.Findings = []string{status.Reason}
			return status
		}
		if result.ExitCode != 0 {
			status.Reason = "PowerShell profile command resolution failed"
			status.Findings = []string{status.Reason}
			return status
		}
		metadata, metadataErr := parseProfileMetadata(d.Masker.Mask(result.Stdout))
		if metadataErr != nil || !metadata.Available {
			status.Reason = "PowerShell profile command unavailable"
			status.Findings = []string{status.Reason}
			return status
		}
		status.Available = true
		if metadata.CommandType != "" {
			status.CommandType = metadata.CommandType
		}
		status.ResolvedPath = d.Masker.Mask(metadata.Path)
		status.Version = parseVersion(d.Masker.Mask(metadata.Version))
		if status.Version == "" {
			status.Available = false
			status.Reason = "version unavailable"
			status.Findings = []string{status.Reason}
		}
	default:
		status.Reason = "unsupported launch mode"
		status.Findings = []string{status.Reason}
	}
	return status
}

type profileMetadata struct {
	CommandType string
	Path        string
	Version     string
	Available   bool
}

func parseProfileMetadata(output string) (profileMetadata, error) {
	lines := strings.Split(output, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		var metadata profileMetadata
		if err := json.Unmarshal([]byte(strings.TrimSpace(lines[index])), &metadata); err == nil && metadata.Available {
			return metadata, nil
		}
	}
	return profileMetadata{}, errors.New("profile metadata was not found")
}

func profileProbeScript(command string, probe []string) string {
	quoted := "'" + strings.ReplaceAll(command, "'", "''") + "'"
	args := make([]string, 0, len(probe))
	for _, argument := range probe {
		args = append(args, "'"+strings.ReplaceAll(argument, "'", "''")+"'")
	}
	probeArgs := "@()"
	if len(args) > 0 {
		probeArgs = "@(" + strings.Join(args, ",") + ")"
	}
	// This is a fixed metadata probe. The configured command receives only a
	// reviewed version argument. Its bounded output is captured in the child,
	// masked before parsing, and reduced to one version string by the caller.
	return "$ErrorActionPreference='Stop'; $c=Microsoft.PowerShell.Core\\Get-Command -Name " + quoted + " -ErrorAction Stop; if ($c.CommandType -eq 'Alias') { $c=Microsoft.PowerShell.Core\\Get-Command -Name $c.Definition -ErrorAction Stop }; if ($c.CommandType -in @('Application','ExternalScript') -and [string]$c.Path -notmatch '^[A-Za-z]:[\\\\/]') { throw 'non-local profile command rejected' }; $probeArgs=" + probeArgs + "; $global:LASTEXITCODE=0; $versionText=(& $c @probeArgs 2>&1 | Microsoft.PowerShell.Utility\\Out-String); if ($LASTEXITCODE -ne 0) { throw 'version probe failed' }; [pscustomobject]@{Available=$true;CommandType=[string]$c.CommandType;Path=[string]$c.Path;Version=[string]$versionText} | Microsoft.PowerShell.Utility\\ConvertTo-Json -Compress"
}

func inspectEnvironment(declarations []domain.EnvironmentDeclaration, findings []Finding) ([]EnvironmentVariableStatus, []Finding) {
	type entry struct {
		declaration domain.EnvironmentDeclaration
		present     bool
	}
	grouped := map[string][]entry{}
	for _, declaration := range declarations {
		present := false
		_, present = os.LookupEnv(declaration.Name)
		key := strings.ToUpper(strings.TrimSpace(declaration.Name))
		grouped[key] = append(grouped[key], entry{declaration: declaration, present: present})
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]EnvironmentVariableStatus, 0, len(keys))
	for _, key := range keys {
		entries := grouped[key]
		status := EnvironmentVariableStatus{Name: entries[0].declaration.Name}
		scopes := map[string]bool{}
		consumers := map[string]bool{}
		present := false
		for _, item := range entries {
			scope := strings.ToLower(strings.TrimSpace(item.declaration.Scope))
			scopes[scope] = true
			present = present || item.present
			if item.declaration.ProfileID != "" {
				consumers[item.declaration.ProfileID] = true
			}
			if item.declaration.Connector != "" {
				consumers[item.declaration.Connector] = true
			}
		}
		for scope := range scopes {
			status.Scopes = append(status.Scopes, scope)
		}
		for consumer := range consumers {
			status.Consumers = append(status.Consumers, consumer)
		}
		sort.Strings(status.Scopes)
		sort.Strings(status.Consumers)
		if len(entries) > 1 {
			status.State = "duplicate"
			if len(scopes) > 1 {
				status.State = "conflict"
			}
			findings = append(findings, Finding{Type: "environment." + strings.ToLower(key) + "." + status.State, Severity: "attention", Target: key, Summary: "environment variable declaration is duplicated or conflicting", RecommendedNextAction: "keep one declaration and make its scope explicit"})
		} else if !present {
			status.State = "missing"
			findings = append(findings, Finding{Type: "environment." + strings.ToLower(key) + ".missing", Severity: "high", Target: key, Summary: "declared environment variable is missing", RecommendedNextAction: "define the variable in the declared Windows scope"})
		} else {
			status.State = "declared"
		}
		result = append(result, status)
	}
	return result, findings
}

func (d Doctor) connectorStatus(connector domain.ConnectorReference) ConnectorStatus {
	status := ConnectorStatus{ID: connector.ID, Configured: strings.TrimSpace(connector.SecretReference) != ""}
	if strings.HasPrefix(strings.ToLower(connector.SecretReference), "env:") {
		name := strings.TrimSpace(connector.SecretReference[4:])
		_, status.SecretReferencePresent = os.LookupEnv(name)
	} else if strings.HasPrefix(strings.ToLower(connector.SecretReference), "credential_manager:") {
		status.Reason = "Windows Credential Manager validation is unavailable in this read-only build"
	} else {
		status.Reason = "unsupported secret reference"
	}
	status.LastValidationResult = connector.LastResult
	if !status.SecretReferencePresent && status.Reason == "" {
		status.Reason = "configured secret reference is unavailable"
	}
	return status
}

func safeEnvironment(allowlist []string) []string {
	allowed := map[string]bool{"PATH": true, "PATHEXT": true, "SYSTEMROOT": true, "WINDIR": true, "TEMP": true, "TMP": true, "USERPROFILE": true, "HOME": true, "PSMODULEPATH": true, "LANG": true}
	for _, name := range allowlist {
		allowed[strings.ToUpper(strings.TrimSpace(name))] = true
	}
	values := make([]string, 0)
	for _, value := range os.Environ() {
		index := strings.IndexByte(value, '=')
		if index <= 0 {
			continue
		}
		if allowed[strings.ToUpper(value[:index])] {
			values = append(values, value)
		}
	}
	sort.Strings(values)
	return values
}

// AllowlistedEnvironment returns the minimal inherited environment permitted
// for a typed child process. Values never leave this package unmasked.
func AllowlistedEnvironment(allowlist []string) []string { return safeEnvironment(allowlist) }

func safeWorkingDirectory(environment []string) string {
	for _, preferred := range []string{"TEMP", "TMP"} {
		for _, value := range environment {
			name, directory, found := strings.Cut(value, "=")
			if !found || !strings.EqualFold(name, preferred) || !filepath.IsAbs(directory) {
				continue
			}
			if info, err := os.Stat(directory); err == nil && info.IsDir() {
				return filepath.Clean(directory)
			}
		}
	}
	if directory := os.TempDir(); filepath.IsAbs(directory) {
		if info, err := os.Stat(directory); err == nil && info.IsDir() {
			return filepath.Clean(directory)
		}
	}
	return ""
}

func safeCommandName(value string) bool {
	if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\\/:;|&<>$`(){}[]\"'\x00\t\r\n ") {
		return false
	}
	return true
}

var localWindowsExecutable = regexp.MustCompile(`(?i)^[A-Za-z]:[\\/].*\.exe$`)

func isLocalWindowsNativeExecutable(value string) bool {
	value = strings.TrimSpace(value)
	if !localWindowsExecutable.MatchString(value) || strings.Contains(value[2:], ":") {
		return false
	}
	for _, segment := range strings.Split(strings.ReplaceAll(value[3:], "/", `\`), `\`) {
		if segment == "" || segment == "." || segment == ".." || strings.HasSuffix(segment, " ") || strings.HasSuffix(segment, ".") {
			return false
		}
	}
	return true
}

var versionPattern = regexp.MustCompile(`\b\d+(?:\.\d+){1,3}(?:[-+][0-9A-Za-z.-]+)?\b`)

func parseVersion(output string) string {
	matches := versionPattern.FindAllString(output, -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}

func duration(seconds int) time.Duration {
	if seconds <= 0 {
		return DefaultTimeout
	}
	return time.Duration(seconds) * time.Second
}

func classifyProcessError(ctx context.Context, err error) string {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "timeout"
	}
	if strings.Contains(strings.ToLower(err.Error()), "timed out") {
		return "timeout"
	}
	if strings.Contains(strings.ToLower(err.Error()), "bounded limit") {
		return "output limit exceeded"
	}
	return "process probe failed"
}

func unavailableFinding(kind, target, reason string) Finding {
	if reason == "" {
		reason = "unavailable"
	}
	return Finding{Type: kind, Severity: "attention", Target: target, Summary: reason, RecommendedNextAction: "install or configure the reported capability, then run env doctor again"}
}

func exitCode(err error) int {
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	return 0
}

var _ io.Writer = (*boundedBuffer)(nil)
