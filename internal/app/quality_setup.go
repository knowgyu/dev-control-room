package app

import (
	"context"
	"errors"
	"strings"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/qualitysetup"
)

const (
	maxQualitySetupLanguageQueryBytes = 128
	maxQualitySetupLanguages          = 4
)

var qualitySetupKnownLanguages = map[string]struct{}{
	"go":         {},
	"javascript": {},
	"python":     {},
	"typescript": {},
}

// QualitySetup inspects only the freshly revalidated registered Worktree.
// Project, Repository, and Worktree IDs are the authority for the source
// root; callers cannot provide an arbitrary filesystem path.
func (a *App) QualitySetup(
	ctx context.Context,
	projectID string,
	repositoryID string,
	worktreeID string,
	languages []string,
) (qualitysetup.Report, error) {
	var report qualitysetup.Report
	projectID = strings.TrimSpace(projectID)
	repositoryID = strings.TrimSpace(repositoryID)
	worktreeID = strings.TrimSpace(worktreeID)
	if projectID == "" || repositoryID == "" || worktreeID == "" {
		return report, contract.InvalidInput("project, repository, and worktree IDs are required")
	}
	languages, err := normalizeQualitySetupLanguages(languages)
	if err != nil {
		return report, err
	}
	if _, err := a.Worktree(ctx, projectID, repositoryID, worktreeID); err != nil {
		return report, err
	}

	current, changed, err := a.discoveryWorktree(ctx, projectID, repositoryID, worktreeID)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return report, err
		}
		if classified := contract.Classify(err); classified.Code != contract.ErrorInternal {
			return report, err
		}
		return report, contract.Unavailable("selected worktree could not be revalidated")
	}
	if changed {
		return report, contract.Conflict("selected worktree changed; refresh and try again")
	}
	root, err := registeredDirectory(current.Path)
	if err != nil || root != current.Path {
		return report, contract.Conflict("selected worktree path changed; refresh and try again")
	}

	report, err = qualitysetup.Inspect(ctx, root, languages)
	if err != nil {
		if errors.Is(err, qualitysetup.ErrInvalidLanguages) {
			return qualitysetup.Report{}, contract.InvalidInput("unsupported language selection")
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return qualitysetup.Report{}, err
		}
		return qualitysetup.Report{}, contract.Unavailable("quality setup could not be inspected")
	}

	a.maskQualitySetupReport(&report)
	report.ProjectID = projectID
	report.RepositoryID = repositoryID
	report.WorktreeID = worktreeID
	report.Head = current.Head
	return report, nil
}

func parseQualitySetupLanguages(raw string) ([]string, error) {
	if len(raw) > maxQualitySetupLanguageQueryBytes {
		return nil, contract.InvalidInput("language selection is too long")
	}
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	return normalizeQualitySetupLanguages(strings.Split(raw, ","))
}

func normalizeQualitySetupLanguages(languages []string) ([]string, error) {
	if len(languages) == 0 {
		return []string{}, nil
	}

	if len(languages) > maxQualitySetupLanguageQueryBytes {
		return nil, contract.InvalidInput("language selection is too long")
	}
	queryBytes := len(languages) - 1
	selected := make([]string, 0, maxQualitySetupLanguages)
	seen := make(map[string]struct{}, maxQualitySetupLanguages)
	for _, raw := range languages {
		if len(raw) > maxQualitySetupLanguageQueryBytes-queryBytes {
			return nil, contract.InvalidInput("language selection is too long")
		}
		queryBytes += len(raw)
		language := strings.ToLower(strings.TrimSpace(raw))
		if language == "" {
			return nil, contract.InvalidInput("languages must be comma-separated values")
		}
		if _, known := qualitySetupKnownLanguages[language]; !known {
			return nil, contract.InvalidInput("unsupported language selection")
		}
		if _, duplicate := seen[language]; duplicate {
			continue
		}
		seen[language] = struct{}{}
		if len(seen) > maxQualitySetupLanguages {
			return nil, contract.InvalidInput("too many selected languages")
		}
		selected = append(selected, language)
	}
	return selected, nil
}

// maskQualitySetupReport masks presentation text while leaving binding
// identities, HEAD, digest, and classification enums unchanged.
func (a *App) maskQualitySetupReport(report *qualitysetup.Report) {
	if report == nil {
		return
	}
	for i := range report.Warnings {
		report.Warnings[i] = a.masker.Mask(report.Warnings[i])
	}
	for i := range report.Components {
		component := &report.Components[i]
		component.Path = a.masker.Mask(component.Path)
		component.PackageManager = a.masker.Mask(component.PackageManager)
		for j := range component.Frameworks {
			component.Frameworks[j] = a.masker.Mask(component.Frameworks[j])
		}
		for j := range component.Evidence {
			component.Evidence[j].Path = a.masker.Mask(component.Evidence[j].Path)
		}
		for j := range component.Checks {
			check := &component.Checks[j]
			check.Label = a.masker.Mask(check.Label)
			check.Reason = a.masker.Mask(check.Reason)
			check.CommandPreview = a.masker.Mask(check.CommandPreview)
			for k := range check.SetupPreview {
				check.SetupPreview[k] = a.masker.Mask(check.SetupPreview[k])
			}
		}
	}
}
