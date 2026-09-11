package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/knowgyu/dev-control-room/internal/action"
	"github.com/knowgyu/dev-control-room/internal/assurance"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/qualitysetup"
)

// QualityToolInstallActionPlan is the persisted broker plan together with the
// server-validated installation preview. The preview is descriptive; only the
// persisted ActionPlan can reach approval, admission, and execution.
type QualityToolInstallActionPlan struct {
	Plan    domain.ActionPlan         `json:"plan"`
	Preview QualityToolInstallPreview `json:"preview"`
}

type qualityToolInstallLookPath func(...string) string

// PlanQualityToolInstallAction validates the native Windows installation
// request, persists a server-owned ActionPlan, and returns the exact preview
// that was used to build it. It intentionally does not execute a package
// manager or accept an executable/argv from the HTTP request.
func (a *App) PlanQualityToolInstallAction(ctx context.Context, input QualityToolInstallPlanInput) (QualityToolInstallActionPlan, error) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return QualityToolInstallActionPlan{}, contract.Unavailable("quality tool installation requires Windows 11 amd64")
	}
	return a.planQualityToolInstallAction(ctx, input, assurance.CheckWindows11Capability(ctx), qualityLookPath)
}

func (a *App) planQualityToolInstallAction(ctx context.Context, input QualityToolInstallPlanInput, capability assurance.Windows11Capability, lookPath qualityToolInstallLookPath) (QualityToolInstallActionPlan, error) {
	input.ProjectID = strings.TrimSpace(input.ProjectID)
	input.RepositoryID = strings.TrimSpace(input.RepositoryID)
	input.WorktreeID = strings.TrimSpace(input.WorktreeID)
	input.ComponentID = strings.TrimSpace(input.ComponentID)
	if input.ProjectID == "" || input.RepositoryID == "" || input.WorktreeID == "" {
		return QualityToolInstallActionPlan{}, contract.InvalidInput("project, repository, and worktree IDs are required")
	}
	if lookPath == nil {
		return QualityToolInstallActionPlan{}, errors.New("quality tool installation executable lookup is unavailable")
	}
	current, changed, err := a.discoveryWorktree(ctx, input.ProjectID, input.RepositoryID, input.WorktreeID)
	if err != nil {
		return QualityToolInstallActionPlan{}, contract.Unavailable("selected worktree could not be revalidated")
	}
	if changed {
		return QualityToolInstallActionPlan{}, contract.Conflict("selected worktree changed; refresh and try again")
	}
	componentID, componentRoot, affectedFiles, err := a.verifiedQualityInstallTarget(ctx, current.Path, input)
	if err != nil {
		return QualityToolInstallActionPlan{}, contract.InvalidInput(err.Error())
	}
	interpreterPath, environmentScope := "", "project"
	if input.Kind == assurance.QualityToolInstallRuff || input.Kind == assurance.QualityToolInstallPytest {
		interpreterPath, environmentScope = resolveQualityPython(current.Path, componentRoot, lookPath)
	}
	request := assurance.QualityToolInstallRequest{
		Kind:             input.Kind,
		ComponentID:      componentID,
		WorktreeRoot:     current.Path,
		ComponentRoot:    componentRoot,
		InterpreterPath:  interpreterPath,
		NodePath:         lookPath("node.exe", "node"),
		NPMPath:          lookPath("npm.exe", "npm.cmd", "npm"),
		EnvironmentScope: environmentScope,
		AllowGlobal:      input.AllowGlobal,
		Version:          strings.TrimSpace(input.Version),
		AffectedFiles:    affectedFiles,
	}
	actionPreview, err := assurance.BuildQualityToolInstallAction(request, capability)
	if err != nil {
		if !capability.Available {
			return QualityToolInstallActionPlan{}, contract.Unavailable("quality tool installation requires native Windows 11 amd64")
		}
		return QualityToolInstallActionPlan{}, contract.InvalidInput(err.Error())
	}
	inputs := qualityToolInstallPlanInputs(actionPreview)
	actionType, err := qualityToolInstallActionType(input.Kind)
	if err != nil {
		return QualityToolInstallActionPlan{}, contract.InvalidInput(err.Error())
	}
	plan, err := a.broker.Plan(ctx, action.PlanRequest{
		ID:               assuranceID("quality-tool-install", input.ProjectID, input.RepositoryID, input.WorktreeID, current.Head, actionType, actionPreview.ComponentID, actionPreview.Package, actionPreview.Version, actionPreview.EnvironmentScope, actionPreview.Command.Executable, strings.Join(actionPreview.Command.Arguments, "\x00"), strings.Join(actionPreview.WritablePaths, "\x00")),
		Name:             "Quality tool install: " + actionPreview.Package,
		ProjectID:        input.ProjectID,
		RepositoryID:     input.RepositoryID,
		WorktreeID:       input.WorktreeID,
		ActionType:       actionType,
		Inputs:           inputs,
		RequestedBy:      domain.Actor{Kind: domain.ActorSystem, ID: "quality-tool-install-service"},
		ToolVersion:      actionPreview.Package + "@" + actionPreview.Version,
		ToolConfigDigest: digestText("quality-tool-install-config-v1", actionPreview.ComponentID, actionPreview.ComponentRoot, actionPreview.EnvironmentScope, actionPreview.Command, actionPreview.AffectedFiles, actionPreview.WritablePaths),
		WritablePaths:    append([]string(nil), actionPreview.WritablePaths...),
	})
	if err != nil {
		return QualityToolInstallActionPlan{}, classifyActionError(err)
	}
	return QualityToolInstallActionPlan{
		Plan: plan,
		Preview: QualityToolInstallPreview{
			Available: true,
			Action:    &actionPreview,
		},
	}, nil
}

func qualityToolInstallActionType(kind assurance.QualityToolInstallKind) (string, error) {
	switch kind {
	case assurance.QualityToolInstallRuff, assurance.QualityToolInstallPytest:
		return domain.QualityToolInstallPythonAction, nil
	case assurance.QualityToolInstallESLint, assurance.QualityToolInstallVitest:
		return domain.QualityToolInstallNodeAction, nil
	default:
		return "", errors.New("quality tool is not reviewed")
	}
}

func isQualityToolInstallActionType(actionType string) bool {
	switch strings.TrimSpace(actionType) {
	case domain.QualityToolInstallPythonAction, domain.QualityToolInstallNodeAction:
		return true
	default:
		return false
	}
}

func qualityToolInstallPlanInputs(action assurance.QualityToolInstallAction) map[string]string {
	affectedFiles, _ := json.Marshal(action.AffectedFiles)
	inputs := map[string]string{
		"package":          action.Package,
		"version":          action.Version,
		"componentId":      action.ComponentID,
		"componentRoot":    action.ComponentRoot,
		"executable":       action.Command.Executable,
		"affectedFiles":    string(affectedFiles),
		"environmentScope": action.EnvironmentScope,
	}
	if len(action.Command.Arguments) > 0 && strings.EqualFold(filepath.Ext(action.Command.Arguments[0]), ".js") {
		inputs["npmCliPath"] = action.Command.Arguments[0]
	}
	return inputs
}

func (a *App) verifiedQualityInstallTarget(ctx context.Context, root string, input QualityToolInstallPlanInput) (string, string, []string, error) {
	setup, err := qualitysetup.Inspect(ctx, root, nil)
	if err != nil {
		return "", "", nil, errors.New("quality tool installation component could not be inspected")
	}
	requestedID := strings.TrimSpace(input.ComponentID)
	component := qualitysetup.Component{ID: "repository", Path: "."}
	if requestedID != "" && requestedID != "repository" {
		found := false
		for _, candidate := range setup.Components {
			if candidate.ID == requestedID {
				component = candidate
				found = true
				break
			}
		}
		if !found {
			return "", "", nil, errors.New("componentId is not a verified worktree component")
		}
	} else if requestedID == "" && len(setup.Components) > 0 {
		candidates := make([]qualitysetup.Component, 0, len(setup.Components))
		for _, candidate := range setup.Components {
			if qualityToolKindMatchesComponent(input.Kind, candidate) && qualityInstallAffectedFilesMatchComponent(candidate, setup.Components, input.AffectedFiles) {
				candidates = append(candidates, candidate)
			}
		}
		switch len(candidates) {
		case 1:
			component = candidates[0]
		case 0:
			if len(setup.Components) == 1 {
				component = setup.Components[0]
			} else {
				return "", "", nil, errors.New("componentId is required because no unique component matches this tool and its affected files")
			}
		default:
			return "", "", nil, errors.New("componentId is required because multiple components match this tool and its affected files")
		}
	}
	componentPath := filepath.ToSlash(filepath.Clean(component.Path))
	componentRoot := qualityComponentRoot(root, componentPath)
	if !qualityInstallPathWithin(root, componentRoot) || qualityInstallPathContainsSymlink(componentRoot) {
		return "", "", nil, errors.New("verified component root is outside or linked from the worktree")
	}
	info, err := os.Stat(componentRoot)
	if err != nil || !info.IsDir() {
		return "", "", nil, errors.New("verified component root is unavailable")
	}
	affectedFiles, err := normalizeQualityInstallAffectedFiles(component, setup.Components, input.AffectedFiles)
	if err != nil {
		return "", "", nil, err
	}
	return component.ID, filepath.Clean(componentRoot), affectedFiles, nil
}

func qualityToolKindMatchesComponent(kind assurance.QualityToolInstallKind, component qualitysetup.Component) bool {
	wantsPython := kind == assurance.QualityToolInstallRuff || kind == assurance.QualityToolInstallPytest
	for _, language := range component.Languages {
		if wantsPython && language == "python" {
			return true
		}
		if !wantsPython && (language == "javascript" || language == "typescript") {
			return true
		}
	}
	return false
}

func qualityInstallAffectedFilesMatchComponent(component qualitysetup.Component, components []qualitysetup.Component, values []string) bool {
	_, err := normalizeQualityInstallAffectedFiles(component, components, values)
	return err == nil
}

func normalizeQualityInstallAffectedFiles(component qualitysetup.Component, components []qualitysetup.Component, values []string) ([]string, error) {
	componentPath := filepath.ToSlash(filepath.Clean(component.Path))
	if componentPath == "." {
		componentPath = ""
	}
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" || filepath.IsAbs(value) || filepath.VolumeName(value) != "" || strings.ContainsAny(value, "\x00\r\n") || strings.TrimSpace(raw) != raw {
			return nil, errors.New("affected file must be a relative path")
		}
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, errors.New("affected file escapes the selected component")
		}
		if componentPath != "" && !strings.HasPrefix(clean, componentPath+"/") && clean != componentPath {
			for _, other := range components {
				otherPath := filepath.ToSlash(filepath.Clean(other.Path))
				if otherPath != "." && otherPath != componentPath && (clean == otherPath || strings.HasPrefix(clean, otherPath+"/")) {
					return nil, errors.New("affected file belongs to a different component")
				}
			}
			clean = componentPath + "/" + clean
		}
		result = append(result, clean)
	}
	return result, nil
}

func resolveQualityPython(root, component string, lookPath qualityToolInstallLookPath) (string, string) {
	root = filepath.Clean(root)
	component = filepath.Clean(component)
	for _, candidate := range []string{
		filepath.Join(component, ".venv", "Scripts", "python.exe"),
		filepath.Join(component, "venv", "Scripts", "python.exe"),
		filepath.Join(component, ".venv", "python.exe"),
		filepath.Join(component, "venv", "python.exe"),
	} {
		if !qualityInstallPathWithin(root, candidate) || qualityInstallPathContainsSymlink(candidate) {
			continue
		}
		info, err := os.Lstat(candidate)
		if err == nil && info.Mode().IsRegular() {
			return candidate, "project"
		}
	}
	if lookPath != nil {
		return lookPath("python.exe", "python"), "global"
	}
	return "", "global"
}

func qualityInstallPathWithin(root, target string) bool {
	root, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return false
	}
	target, err = filepath.Abs(filepath.Clean(target))
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func qualityInstallPathContainsSymlink(path string) bool {
	path = filepath.Clean(path)
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
	}
}

func qualityToolInstallPreviewReason(err error, environmentScope string) string {
	if environmentScope == "global" {
		return "프로젝트 전용 Python 환경을 찾지 못했습니다. 전역 설치는 명시적으로 허용하고 사람 승인을 받아야 합니다: " + err.Error()
	}
	return "설치 미리보기를 만들 수 없습니다: " + err.Error()
}
