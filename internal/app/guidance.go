package app

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/environment"
)

const guidanceFileLimit = 256 << 10
const handoffFindingLimit = 50
const handoffTextLimit = 2048

var guidanceLink = regexp.MustCompile(`\[[^\]]+\]\(([^)#]+)\)`)

func (a *App) Guidance(ctx context.Context, projectID, repositoryID, worktreeID string) (GuidanceReport, error) {
	worktree, err := a.Worktree(ctx, projectID, repositoryID, worktreeID)
	if err != nil {
		return GuidanceReport{}, err
	}
	root := filepath.Clean(worktree.Spec.CanonicalPath)
	if info, statErr := os.Stat(root); statErr != nil || !info.IsDir() {
		return GuidanceReport{}, contract.Unavailable("selected Worktree directory is unavailable")
	}
	report := GuidanceReport{ProjectID: projectID, RepositoryID: repositoryID, WorktreeID: worktreeID, CheckedAt: time.Now().UTC(), Files: []string{}, Findings: []GuidanceFinding{}}
	contents := map[string][]string{}
	for _, name := range []string{"AGENTS.md", "CLAUDE.md", "GEMINI.md"} {
		path := filepath.Join(root, name)
		info, statErr := os.Stat(path)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return GuidanceReport{}, contract.Unavailable("guidance file could not be inspected")
		}
		report.Files = append(report.Files, name)
		if info.Size() > guidanceFileLimit {
			report.Findings = append(report.Findings, GuidanceFinding{"attention", "guidance.too_large", name, "guidance file exceeds the bounded doctor size", "split the file into focused guidance documents"})
			continue
		}
		file, openErr := os.Open(path)
		if openErr != nil {
			return GuidanceReport{}, contract.Unavailable("guidance file could not be read")
		}
		lines, readErr := readGuidanceLines(file)
		_ = file.Close()
		if readErr != nil {
			return GuidanceReport{}, contract.Unavailable("guidance file could not be read")
		}
		contents[name] = lines
		if len(lines) == 0 {
			report.Findings = append(report.Findings, GuidanceFinding{"attention", "guidance.empty", name, "guidance file is empty", "add only instructions that are still valid"})
		}
		for _, line := range lines {
			match := guidanceLink.FindStringSubmatch(line)
			if len(match) != 2 || strings.Contains(match[1], "://") || strings.HasPrefix(match[1], "#") {
				continue
			}
			linked := filepath.Clean(filepath.Join(root, match[1]))
			if !withinPath(root, linked) {
				report.Findings = append(report.Findings, GuidanceFinding{"attention", "guidance.link_outside_worktree", name, "guidance links outside the selected Worktree", "replace the link with a bounded repository-relative reference"})
			} else if _, linkErr := os.Stat(linked); errors.Is(linkErr, os.ErrNotExist) {
				report.Findings = append(report.Findings, GuidanceFinding{"attention", "guidance.missing_reference", name, "guidance references a missing repository path", "remove or repair the stale reference"})
			}
		}
	}
	if len(report.Files) == 0 {
		report.Findings = append(report.Findings, GuidanceFinding{"attention", "guidance.missing", "", "no recognized guidance file was found", "add a small AGENTS.md, CLAUDE.md, or GEMINI.md only when the repository needs local instructions"})
	}
	seen := map[string]string{}
	for name, lines := range contents {
		for _, line := range lines {
			key := strings.TrimSpace(line)
			if len(key) < 24 || strings.HasPrefix(key, "#") {
				continue
			}
			if prior, ok := seen[key]; ok && prior != name {
				report.Findings = append(report.Findings, GuidanceFinding{"info", "guidance.duplicate", name, fmt.Sprintf("instruction is duplicated from %s", prior), "keep one authoritative instruction and link to it"})
			} else {
				seen[key] = name
			}
		}
	}
	return report, nil
}

func readGuidanceLines(reader io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(io.LimitReader(reader, guidanceFileLimit+1))
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func withinPath(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (a *App) PrepareHandoff(ctx context.Context, input HandoffInput) (HandoffPreview, error) {
	preview, _, err := a.prepareHandoff(ctx, input)
	return preview, err
}

type handoffLaunchSpec struct {
	executable       string
	arguments        []string
	environment      []string
	workingDirectory string
}

type handoffDigestInput struct {
	ProfileID              string   `json:"profileId"`
	ProjectID              string   `json:"projectId"`
	RepositoryID           string   `json:"repositoryId"`
	WorktreeID             string   `json:"worktreeId"`
	Model                  string   `json:"model,omitempty"`
	LaunchMode             string   `json:"launchMode"`
	DataBoundary           string   `json:"dataBoundary"`
	Head                   string   `json:"head,omitempty"`
	Branch                 string   `json:"branch,omitempty"`
	PathFingerprint        string   `json:"pathFingerprint"`
	AssociationFingerprint string   `json:"associationFingerprint"`
	Arguments              []string `json:"arguments"`
}

type handoffPayload struct {
	ProjectID            string           `json:"projectId"`
	RepositoryID         string           `json:"repositoryId"`
	WorktreeID           string           `json:"worktreeId"`
	Head                 string           `json:"head,omitempty"`
	Branch               string           `json:"branch,omitempty"`
	Dirty                bool             `json:"dirty"`
	Untracked            bool             `json:"untracked"`
	Findings             []HandoffFinding `json:"findings"`
	VerificationCommands []string         `json:"verificationCommands"`
}

func (a *App) prepareHandoff(ctx context.Context, input HandoffInput) (HandoffPreview, handoffLaunchSpec, error) {
	profile, err := a.AgentProfile(ctx, input.ProfileID)
	if err != nil {
		return HandoffPreview{}, handoffLaunchSpec{}, err
	}
	worktree, err := a.Worktree(ctx, input.ProjectID, input.RepositoryID, input.WorktreeID)
	if err != nil {
		return HandoffPreview{}, handoffLaunchSpec{}, err
	}
	current, changed, err := a.discoveryWorktree(ctx, input.ProjectID, input.RepositoryID, input.WorktreeID)
	if err != nil {
		return HandoffPreview{}, handoffLaunchSpec{}, err
	}
	if changed {
		return HandoffPreview{}, handoffLaunchSpec{}, contract.Conflict("selected Worktree evidence is no longer current; scan again before handoff")
	}
	if current.Head != worktree.Spec.Head || current.Branch != worktree.Spec.Branch || current.Dirty != worktree.Spec.Dirty || current.Untracked != worktree.Spec.Untracked {
		return HandoffPreview{}, handoffLaunchSpec{}, contract.Conflict("selected Worktree changed since the last scan; scan again before handoff")
	}
	if len(input.Model) > 256 {
		return HandoffPreview{}, handoffLaunchSpec{}, contract.InvalidInput("selected model is too long")
	}
	findings, err := a.Findings(ctx, input.ProjectID, input.RepositoryID)
	if err != nil {
		return HandoffPreview{}, handoffLaunchSpec{}, err
	}
	selected := make([]HandoffFinding, 0, minInt(len(findings), handoffFindingLimit))
	for _, finding := range findings {
		if len(selected) == handoffFindingLimit {
			break
		}
		selected = append(selected, HandoffFinding{ID: finding.Metadata.ID, Severity: string(finding.Spec.Severity), Summary: boundHandoffText(a.masker.Mask(finding.Spec.Summary)), Next: boundHandoffText(a.masker.Mask(finding.Spec.RecommendedNext))})
	}
	verificationCommands := []string{"devroom finding list --project " + input.ProjectID + " --repository " + input.RepositoryID}
	payload, err := json.Marshal(handoffPayload{ProjectID: input.ProjectID, RepositoryID: input.RepositoryID, WorktreeID: input.WorktreeID, Head: worktree.Spec.Head, Branch: worktree.Spec.Branch, Dirty: worktree.Spec.Dirty, Untracked: worktree.Spec.Untracked, Findings: selected, VerificationCommands: verificationCommands})
	if err != nil {
		return HandoffPreview{}, handoffLaunchSpec{}, err
	}
	prompt := "Dev Control Room bounded handoff. Work only in the selected Worktree. Do not access unrelated projects, secrets, or transcripts. Review the evidence, make only the requested repository changes, and run the listed verification commands.\nContext JSON:\n" + string(payload)
	modelArguments := handoffModelArguments(profile.Spec.ModelArgumentTemplate, input.Model)
	logicalArguments := append(append([]string(nil), modelArguments...), prompt)
	executable := profile.Spec.Command
	arguments := logicalArguments
	if profile.Spec.LaunchMode == domain.AgentLaunchPowerShellProfile {
		executable = "pwsh"
		arguments = []string{"-NoLogo", "-NoExit", "-Command", powerShellHandoffScript(profile.Spec.Command, logicalArguments)}
	}
	digestBytes, err := json.Marshal(handoffDigestInput{ProfileID: profile.Metadata.ID, ProjectID: input.ProjectID, RepositoryID: input.RepositoryID, WorktreeID: input.WorktreeID, Model: input.Model, LaunchMode: string(profile.Spec.LaunchMode), DataBoundary: string(profile.Spec.DataBoundary), Head: worktree.Spec.Head, Branch: worktree.Spec.Branch, PathFingerprint: worktree.Spec.PathFingerprint, AssociationFingerprint: worktree.Spec.AssociationFingerprint, Arguments: arguments})
	if err != nil {
		return HandoffPreview{}, handoffLaunchSpec{}, err
	}
	digest := sha256.Sum256(digestBytes)
	preview := HandoffPreview{
		ProfileID: profile.Metadata.ID, ProfileName: profile.Metadata.Name, ProfileCommand: profile.Spec.Command,
		Model: input.Model, ModelArgumentTemplate: profile.Spec.ModelArgumentTemplate,
		LaunchMode: string(profile.Spec.LaunchMode), DataBoundary: string(profile.Spec.DataBoundary),
		ProjectID: input.ProjectID, RepositoryID: input.RepositoryID, WorktreeID: input.WorktreeID,
		WorkingDirectory: a.masker.Mask(worktree.Spec.CanonicalPath), Scope: []string{"선택한 Finding 요약", "선택한 Worktree 메타데이터", "검증 명령 이름"},
		Findings: selected, VerificationCommands: verificationCommands, TranscriptIncluded: false,
		Head: worktree.Spec.Head, Branch: worktree.Spec.Branch, Dirty: worktree.Spec.Dirty, Untracked: worktree.Spec.Untracked,
		PreviewDigest: hex.EncodeToString(digest[:]), Arguments: append([]string(nil), arguments...),
	}
	if current.Path != "" {
		worktree.Spec.CanonicalPath = current.Path
	}
	return preview, handoffLaunchSpec{executable: executable, arguments: arguments, environment: environment.SafeEnvironment(profile.Spec.EnvironmentAllowlist), workingDirectory: worktree.Spec.CanonicalPath}, nil
}

func (a *App) LaunchHandoff(ctx context.Context, input HandoffLaunchInput) (HandoffLaunch, error) {
	if strings.TrimSpace(input.PreviewDigest) == "" {
		return HandoffLaunch{}, contract.InvalidInput("handoff launch requires the preview digest")
	}
	preview, spec, err := a.prepareHandoff(ctx, input.HandoffInput)
	if err != nil {
		return HandoffLaunch{}, err
	}
	if subtle.ConstantTimeCompare([]byte(preview.PreviewDigest), []byte(input.PreviewDigest)) != 1 {
		return HandoffLaunch{}, contract.Conflict("handoff preview is no longer current; preview again before launch")
	}
	if a.launcher == nil {
		return HandoffLaunch{}, contract.Unavailable("agent launcher is unavailable")
	}
	started, err := a.launcher.Launch(context.WithoutCancel(ctx), spec.executable, spec.arguments, spec.environment, spec.workingDirectory)
	if err != nil {
		return HandoffLaunch{}, contract.Unavailable("configured Agent could not be launched")
	}
	startedAt := time.Now().UTC()
	if err := a.recordEvent(domain.Event{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.EventKind}, Metadata: domain.ObjectMeta{ID: eventID("handoff"), Name: "Agent Handoff launch"}, Spec: domain.EventSpec{ProjectID: input.ProjectID, RepositoryID: input.RepositoryID, EventType: "handoff.launched", Actor: "user", Summary: "Agent Handoff launched", Data: map[string]any{"profile_id": input.ProfileID, "worktree_id": input.WorktreeID, "launch_mode": preview.LaunchMode, "preview_digest": preview.PreviewDigest, "pid": started.PID, "transcript_included": false}, OccurredAt: startedAt}}); err != nil {
		return HandoffLaunch{}, err
	}
	return HandoffLaunch{ProfileID: preview.ProfileID, ProfileName: preview.ProfileName, Model: preview.Model, LaunchMode: preview.LaunchMode, DataBoundary: preview.DataBoundary, ProjectID: preview.ProjectID, RepositoryID: preview.RepositoryID, WorktreeID: preview.WorktreeID, WorkingDirectory: preview.WorkingDirectory, PreviewDigest: preview.PreviewDigest, PID: started.PID, StartedAt: startedAt, TranscriptIncluded: false}, nil
}

func handoffModelArguments(template, model string) []string {
	if strings.TrimSpace(model) == "" || strings.TrimSpace(template) == "" {
		return nil
	}
	arguments := make([]string, 0, 2)
	for _, token := range strings.Fields(template) {
		arguments = append(arguments, strings.ReplaceAll(token, "{model}", model))
	}
	return arguments
}

func powerShellHandoffScript(command string, arguments []string) string {
	quoted := make([]string, 0, len(arguments))
	for index, argument := range arguments {
		if index == len(arguments)-1 {
			encoded := base64.StdEncoding.EncodeToString([]byte(argument))
			quoted = append(quoted, "$([Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('"+encoded+"')))")
			continue
		}
		quoted = append(quoted, "'"+strings.ReplaceAll(argument, "'", "''")+"'")
	}
	return "$ErrorActionPreference='Stop'; $c=Microsoft.PowerShell.Core\\Get-Command -Name '" + strings.ReplaceAll(command, "'", "''") + "' -ErrorAction Stop; $handoffArgs=@(" + strings.Join(quoted, ",") + "); & $c @handoffArgs"
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func boundHandoffText(value string) string {
	if len(value) <= handoffTextLimit {
		return value
	}
	runes := []rune(value)
	if len(runes) <= handoffTextLimit {
		return value
	}
	return string(runes[:handoffTextLimit]) + "…"
}

func (a *App) FailureFingerprints(ctx context.Context, limit int) ([]domain.FailureFingerprint, error) {
	if limit < 0 || limit > 1000 {
		return nil, contract.InvalidInput("failure fingerprint limit must be between 0 and 1000")
	}
	return a.store.ListFailureFingerprints(ctx, limit)
}
