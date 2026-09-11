package assurance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type QualityToolInstallKind string

const (
	QualityToolInstallRuff   QualityToolInstallKind = "python.ruff"
	QualityToolInstallPytest QualityToolInstallKind = "python.pytest"
	QualityToolInstallESLint QualityToolInstallKind = "node.eslint"
	QualityToolInstallVitest QualityToolInstallKind = "node.vitest"
)

type QualityToolInstallRequest struct {
	Kind             QualityToolInstallKind
	ComponentID      string
	WorktreeRoot     string
	ComponentRoot    string
	InterpreterPath  string
	NodePath         string
	NPMPath          string
	NPMCLIPath       string
	EnvironmentScope string
	AllowGlobal      bool
	Version          string
	AffectedFiles    []string
}

type QualityToolInstallAction struct {
	Kind             QualityToolInstallKind `json:"kind"`
	Package          string                 `json:"package"`
	Version          string                 `json:"version"`
	ComponentID      string                 `json:"componentId"`
	WorktreeRoot     string                 `json:"worktreeRoot"`
	ComponentRoot    string                 `json:"componentRoot"`
	Command          TypedCommand           `json:"command"`
	AffectedFiles    []string               `json:"affectedFiles"`
	WritablePaths    []string               `json:"writablePaths"`
	EnvironmentScope string                 `json:"environmentScope"`
}

type QualityToolInstallResult struct {
	Action QualityToolInstallAction `json:"action"`
	Status string                   `json:"status"`
}

type QualityToolInstallBroker interface {
	ExecuteApprovedQualityToolInstall(context.Context, QualityToolInstallAction) (QualityToolInstallResult, error)
}

type QualityToolInstallExecutor struct{ Broker QualityToolInstallBroker }

func (e QualityToolInstallExecutor) Execute(ctx context.Context, action QualityToolInstallAction) (QualityToolInstallResult, error) {
	if e.Broker == nil {
		return QualityToolInstallResult{}, errors.New("quality tool installation requires the existing Action Broker")
	}
	return e.Broker.ExecuteApprovedQualityToolInstall(ctx, action)
}

func BuildQualityToolInstallAction(request QualityToolInstallRequest, capability Windows11Capability) (QualityToolInstallAction, error) {
	if !capability.Available {
		return QualityToolInstallAction{}, errors.New("quality tool installation requires native Windows 11")
	}
	root, component, err := installRoots(request.WorktreeRoot, request.ComponentRoot)
	if err != nil {
		return QualityToolInstallAction{}, err
	}
	componentID := strings.TrimSpace(request.ComponentID)
	if componentID == "" || strings.ContainsAny(componentID, "\x00\r\n") {
		return QualityToolInstallAction{}, errors.New("quality tool installation requires a verified component")
	}
	if !validExactToolVersion(request.Version) {
		return QualityToolInstallAction{}, errors.New("quality tool installation requires an explicit exact version")
	}
	files, err := declaredAffectedFiles(root, request.AffectedFiles)
	if err != nil {
		return QualityToolInstallAction{}, err
	}
	packageName, err := installPackage(request.Kind)
	if err != nil {
		return QualityToolInstallAction{}, err
	}
	var command TypedCommand
	scope := strings.TrimSpace(request.EnvironmentScope)
	if scope == "" {
		scope = "project"
	}
	if scope != "project" && scope != "global" {
		return QualityToolInstallAction{}, errors.New("quality tool installation environment scope is invalid")
	}
	if scope == "global" && !request.AllowGlobal {
		return QualityToolInstallAction{}, errors.New("global quality tool installation requires explicit approval")
	}
	switch request.Kind {
	case QualityToolInstallRuff, QualityToolInstallPytest:
		if err := validateInstallExecutable(request.InterpreterPath, "python.exe"); err != nil {
			return QualityToolInstallAction{}, err
		}
		command = TypedCommand{Executable: request.InterpreterPath, Arguments: []string{"-m", "pip", "install", "--disable-pip-version-check", "--no-input", packageName + "==" + request.Version}}
	case QualityToolInstallESLint, QualityToolInstallVitest:
		if err := validateInstallExecutable(request.NodePath, "node.exe"); err != nil {
			return QualityToolInstallAction{}, err
		}
		npmCLIPath, resolveErr := resolveNPMCLIPath(request.NodePath, request.NPMPath, request.NPMCLIPath)
		if resolveErr != nil {
			return QualityToolInstallAction{}, resolveErr
		}
		command = TypedCommand{Executable: request.NodePath, Arguments: []string{npmCLIPath, "install", "--save-dev", "--ignore-scripts", "--no-audit", "--no-fund", packageName + "@" + request.Version}}
	}
	if err := command.Validate(); err != nil {
		return QualityToolInstallAction{}, err
	}
	writablePaths := make([]string, 0, len(files))
	for _, file := range files {
		writablePaths = append(writablePaths, filepath.Clean(filepath.Join(root, filepath.FromSlash(file))))
	}
	for _, file := range files {
		if !qualityPathWithin(component, filepath.Join(root, filepath.FromSlash(file))) {
			return QualityToolInstallAction{}, errors.New("declared affected file is outside the verified component")
		}
	}
	return QualityToolInstallAction{Kind: request.Kind, Package: packageName, Version: request.Version, ComponentID: componentID, WorktreeRoot: root, ComponentRoot: component, Command: command, AffectedFiles: files, WritablePaths: writablePaths, EnvironmentScope: scope}, nil
}

func installRoots(root, component string) (string, string, error) {
	root = filepath.Clean(root)
	component = filepath.Clean(component)
	if err := validateQualityWorktreeRoot(root, os.Stat, os.Lstat); err != nil {
		return "", "", err
	}
	if component == "." || component == "" {
		component = root
	}
	if !qualityPathWithin(root, component) || qualityPathContainsSymlink(component, os.Lstat) {
		return "", "", errors.New("install component root is outside or linked from the worktree")
	}
	info, err := os.Lstat(component)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", "", errors.New("install component root is unavailable")
	}
	return root, component, nil
}

func declaredAffectedFiles(root string, values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, errors.New("quality tool installation must declare affected files")
	}
	result := append([]string(nil), values...)
	sort.Strings(result)
	for i, value := range result {
		if value == "" || filepath.IsAbs(value) || filepath.VolumeName(value) != "" || strings.ContainsAny(value, "\x00\r\n") || strings.TrimSpace(value) != value {
			return nil, errors.New("declared affected file is unsafe")
		}
		clean := filepath.Clean(value)
		if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
			return nil, errors.New("declared affected file escapes the worktree")
		}
		result[i] = filepath.ToSlash(clean)
	}
	_ = root
	return result, nil
}

func installPackage(kind QualityToolInstallKind) (string, error) {
	switch kind {
	case QualityToolInstallRuff:
		return "ruff", nil
	case QualityToolInstallPytest:
		return "pytest", nil
	case QualityToolInstallESLint:
		return "eslint", nil
	case QualityToolInstallVitest:
		return "vitest", nil
	default:
		return "", fmt.Errorf("quality tool %q is not reviewed", kind)
	}
}

var exactVersionPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+){1,3}(?:[-+][0-9A-Za-z.-]+)?$`)

func validExactToolVersion(value string) bool {
	return exactVersionPattern.MatchString(value) && !strings.ContainsAny(value, "\r\n")
}

func validateInstallExecutable(path, base string) error {
	if !isVerifiedNativeQualityExecutable(path, base) || strings.HasSuffix(strings.ToLower(path), ".cmd") || strings.HasSuffix(strings.ToLower(path), ".bat") {
		return fmt.Errorf("selected %s must be an absolute native executable", base)
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("selected %s is not a regular executable", base)
	}
	return nil
}

func resolveNPMCLIPath(nodePath, npmPath, requested string) (string, error) {
	candidates := []string{}
	if strings.TrimSpace(requested) != "" {
		requested = filepath.Clean(requested)
		if !qualityPathWithin(filepath.Dir(nodePath), requested) && (strings.TrimSpace(npmPath) == "" || !qualityPathWithin(filepath.Dir(npmPath), requested)) {
			return "", errors.New("npm-cli.js must be located with the server-resolved Node installation")
		}
		candidates = append(candidates, requested)
	}
	for _, executable := range []string{nodePath, npmPath} {
		if strings.TrimSpace(executable) == "" {
			continue
		}
		directory := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(directory, "npm-cli.js"),
			filepath.Join(directory, "node_modules", "npm", "bin", "npm-cli.js"),
		)
	}
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if _, ok := seen[candidate]; ok || !filepath.IsAbs(candidate) || strings.ContainsAny(candidate, "\x00\r\n") {
			continue
		}
		seen[candidate] = struct{}{}
		if !strings.EqualFold(filepath.Ext(candidate), ".js") {
			continue
		}
		info, err := os.Lstat(candidate)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			continue
		}
		return candidate, nil
	}
	return "", errors.New("npm-cli.js is unavailable; npm.cmd or npm.exe cannot be used as the installer")
}
