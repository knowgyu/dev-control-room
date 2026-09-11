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
	interpreterPath, environmentScope := "", "project"
	if input.Kind == assurance.QualityToolInstallRuff || input.Kind == assurance.QualityToolInstallPytest {
		interpreterPath, environmentScope = resolveQualityPython(current.Path, current.Path, lookPath)
	}
	request := assurance.QualityToolInstallRequest{
		Kind:             input.Kind,
		WorktreeRoot:     current.Path,
		ComponentRoot:    current.Path,
		InterpreterPath:  interpreterPath,
		NodePath:         lookPath("node.exe", "node"),
		NPMPath:          lookPath("npm.exe", "npm.cmd", "npm"),
		EnvironmentScope: environmentScope,
		AllowGlobal:      input.AllowGlobal,
		Version:          strings.TrimSpace(input.Version),
		AffectedFiles:    input.AffectedFiles,
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
		ID:               assuranceID("quality-tool-install", input.ProjectID, input.RepositoryID, input.WorktreeID, current.Head, actionType, actionPreview.Package, actionPreview.Version, actionPreview.EnvironmentScope, actionPreview.Command.Executable, strings.Join(actionPreview.Command.Arguments, "\x00"), strings.Join(actionPreview.WritablePaths, "\x00")),
		Name:             "Quality tool install: " + actionPreview.Package,
		ProjectID:        input.ProjectID,
		RepositoryID:     input.RepositoryID,
		WorktreeID:       input.WorktreeID,
		ActionType:       actionType,
		Inputs:           inputs,
		RequestedBy:      domain.Actor{Kind: domain.ActorSystem, ID: "quality-tool-install-service"},
		ToolVersion:      actionPreview.Package + "@" + actionPreview.Version,
		ToolConfigDigest: digestText("quality-tool-install-config-v1", actionPreview.EnvironmentScope, actionPreview.Command, actionPreview.AffectedFiles, actionPreview.WritablePaths),
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
		"executable":       action.Command.Executable,
		"affectedFiles":    string(affectedFiles),
		"environmentScope": action.EnvironmentScope,
	}
	if len(action.Command.Arguments) > 0 && strings.EqualFold(filepath.Ext(action.Command.Arguments[0]), ".js") {
		inputs["npmCliPath"] = action.Command.Arguments[0]
	}
	return inputs
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
