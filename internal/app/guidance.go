package app

import (
	"bufio"
	"context"
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
)

const guidanceFileLimit = 256 << 10

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
	profile, err := a.AgentProfile(ctx, input.ProfileID)
	if err != nil {
		return HandoffPreview{}, err
	}
	worktree, err := a.Worktree(ctx, input.ProjectID, input.RepositoryID, input.WorktreeID)
	if err != nil {
		return HandoffPreview{}, err
	}
	findings, err := a.Findings(ctx, input.ProjectID, input.RepositoryID)
	if err != nil {
		return HandoffPreview{}, err
	}
	preview := HandoffPreview{
		ProfileID: profile.Metadata.ID, ProfileName: profile.Metadata.Name, ProfileCommand: profile.Spec.Command,
		Model: input.Model, ModelArgumentTemplate: profile.Spec.ModelArgumentTemplate,
		LaunchMode: string(profile.Spec.LaunchMode), DataBoundary: string(profile.Spec.DataBoundary),
		ProjectID: input.ProjectID, RepositoryID: input.RepositoryID, WorktreeID: input.WorktreeID,
		WorkingDirectory: a.masker.Mask(worktree.Spec.CanonicalPath), Scope: []string{"selected finding summaries", "selected Worktree metadata", "verification command names"},
		Findings: []HandoffFinding{}, VerificationCommands: []string{"devroom finding list --project " + input.ProjectID + " --repository " + input.RepositoryID}, TranscriptIncluded: false,
	}
	for _, finding := range findings {
		preview.Findings = append(preview.Findings, HandoffFinding{ID: finding.Metadata.ID, Severity: string(finding.Spec.Severity), Summary: a.masker.Mask(finding.Spec.Summary), Next: a.masker.Mask(finding.Spec.RecommendedNext)})
	}
	return preview, nil
}

func (a *App) FailureFingerprints(ctx context.Context, limit int) ([]domain.FailureFingerprint, error) {
	if limit < 0 || limit > 1000 {
		return nil, contract.InvalidInput("failure fingerprint limit must be between 0 and 1000")
	}
	return a.store.ListFailureFingerprints(ctx, limit)
}
