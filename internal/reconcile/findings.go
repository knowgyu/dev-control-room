// Package reconcile turns observations into deterministic, reviewable
// findings. It has no process or persistence side effects.
package reconcile

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/knowgyu/dev-control-room/internal/collector"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

var repositoryFindingTypes = map[string]struct{}{
	domain.FindingDirty: {}, domain.FindingUpstreamDrift: {}, domain.FindingDetachedHead: {},
	domain.FindingMissingRemote: {}, domain.FindingUnsafeCleanup: {}, domain.FindingCollectorError: {},
}

func RepositoryFindings(projectID, repositoryID string, state collector.State, evidenceID string, now time.Time, previous []domain.Finding) []domain.Finding {
	active := make(map[string]domain.Finding)
	add := func(kind string, severity domain.Severity, summary, next string) {
		fingerprint := fingerprint(projectID, repositoryID, kind)
		finding := domain.Finding{
			TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.FindingKind},
			Metadata: domain.ObjectMeta{ID: findingID(fingerprint), Name: kind},
			Spec: domain.FindingSpec{
				ProjectID: projectID, RepositoryID: repositoryID, FindingType: kind,
				Fingerprint: fingerprint, Severity: severity, Confidence: domain.ConfidenceConfirmed,
				Summary: summary, RecommendedNext: next, EvidenceRefs: []string{evidenceID},
				FirstObserved: now, LastObserved: now, State: domain.FindingOpen,
			},
		}
		for _, prior := range previous {
			if prior.Spec.Fingerprint == fingerprint {
				finding.Spec.FirstObserved = prior.Spec.FirstObserved
				if prior.Spec.State == domain.FindingAcknowledged || prior.Spec.State == domain.FindingSuppressed {
					finding.Spec.State = prior.Spec.State
				}
			}
		}
		active[kind] = finding
	}
	if state.Dirty {
		add(domain.FindingDirty, domain.SeverityAttention, "Repository has uncommitted changes", "Review the worktree before any cleanup or automation.")
	}
	if state.Ahead != 0 || state.Behind != 0 {
		add(domain.FindingUpstreamDrift, domain.SeverityAttention,
			fmt.Sprintf("Repository differs from upstream (%d ahead, %d behind)", state.Ahead, state.Behind),
			"Review the upstream diff before relying on automation.")
	}
	if state.Detached {
		add(domain.FindingDetachedHead, domain.SeverityHigh, "Repository HEAD is detached", "Check out an intentional branch before making changes.")
	}
	if len(state.Remotes) == 0 {
		add(domain.FindingMissingRemote, domain.SeverityAttention, "Repository has no normalized remote", "Configure a remote only after confirming the intended destination.")
	}
	if state.UnsafeCleanup {
		add(domain.FindingUnsafeCleanup, domain.SeverityHigh, "Worktree cleanup is unsafe", "Do not remove or reset this worktree until its local state is reviewed.")
	}
	if state.Error != "" {
		add(domain.FindingCollectorError, domain.SeverityHigh, "Repository observation was incomplete", "Check the registered worktree and Git installation before retrying.")
	}

	findings := make([]domain.Finding, 0, len(active)+len(previous))
	for _, finding := range active {
		findings = append(findings, finding)
	}
	for _, prior := range previous {
		if _, exists := active[prior.Spec.FindingType]; exists || prior.Spec.RepositoryID != repositoryID {
			continue
		}
		if _, known := repositoryFindingTypes[prior.Spec.FindingType]; !known {
			continue
		}
		prior.Spec.State = domain.FindingResolved
		prior.Spec.LastObserved = now
		findings = append(findings, prior)
	}
	sortFindings(findings)
	return findings
}

func StaleFinding(projectID string, now time.Time, previous []domain.Finding) domain.Finding {
	const kind = domain.FindingStaleScan
	fp := fingerprint(projectID, "", kind)
	finding := domain.Finding{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.FindingKind},
		Metadata: domain.ObjectMeta{ID: findingID(fp), Name: kind},
		Spec: domain.FindingSpec{
			ProjectID: projectID, FindingType: kind, Fingerprint: fp,
			Severity: domain.SeverityHigh, Confidence: domain.ConfidenceConfirmed,
			Summary: "Project has no recent completed scan", RecommendedNext: "Run a scan before relying on repository health.",
			FirstObserved: now, LastObserved: now, State: domain.FindingOpen,
		},
	}
	for _, prior := range previous {
		if prior.Spec.Fingerprint == fp {
			finding.Spec.FirstObserved = prior.Spec.FirstObserved
			if prior.Spec.State == domain.FindingAcknowledged || prior.Spec.State == domain.FindingSuppressed {
				finding.Spec.State = prior.Spec.State
			}
		}
	}
	return finding
}

func ResolvedStaleFinding(projectID string, now time.Time, previous []domain.Finding) (domain.Finding, bool) {
	for _, prior := range previous {
		if prior.Spec.ProjectID == projectID && prior.Spec.FindingType == domain.FindingStaleScan && prior.Spec.State != domain.FindingResolved {
			prior.Spec.State = domain.FindingResolved
			prior.Spec.LastObserved = now
			return prior, true
		}
	}
	return domain.Finding{}, false
}

func fingerprint(projectID, repositoryID, findingType string) string {
	sum := sha256.Sum256([]byte(projectID + "\x00" + repositoryID + "\x00" + findingType))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func findingID(fingerprint string) string {
	return "finding-" + fingerprint[len("sha256:"):len("sha256:")+48]
}

func sortFindings(findings []domain.Finding) {
	for i := 0; i < len(findings); i++ {
		for j := i + 1; j < len(findings); j++ {
			if findings[j].Metadata.ID < findings[i].Metadata.ID {
				findings[i], findings[j] = findings[j], findings[i]
			}
		}
	}
}
