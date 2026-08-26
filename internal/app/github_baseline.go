package app

import (
	"context"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/assurance"
	"github.com/knowgyu/dev-control-room/internal/collector"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/environment"
)

const githubBaselineTimeout = 30 * time.Second

// githubBaselineExecutor is deliberately smaller than environment.Runner: its
// caller can provide only an already-constructed, fixed GitHub argv. Tests use
// it to supply compact provider responses without making a network request.
type githubBaselineExecutor func(context.Context, assurance.GitHubBaselineInvocation, string) (assurance.GitHubBaselineResponse, error)

func trustedGitHubCLIPath() (string, error) {
	return trustedGitHubCLIPathWith(exec.LookPath, os.Lstat)
}

// trustedGitHubCLIPathWith keeps lookup and filesystem inspection explicit so
// the execution boundary rejects a symlink/reparse-point launcher before a
// fixed gh argv is constructed.
func trustedGitHubCLIPathWith(lookPath func(string) (string, error), lstat func(string) (os.FileInfo, error)) (string, error) {
	path, err := lookPath("gh.exe")
	if err != nil {
		return "", errors.New("GitHub CLI is unavailable")
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", errors.New("GitHub CLI path is unavailable")
	}
	info, err := lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || !strings.EqualFold(filepath.Base(path), "gh.exe") {
		return "", errors.New("GitHub CLI executable is untrusted")
	}
	return path, nil
}

func executeGitHubBaseline(ctx context.Context, invocation assurance.GitHubBaselineInvocation, worktree string) (assurance.GitHubBaselineResponse, error) {
	result, err := (environment.ProcessRunner{OutputLimit: assurance.MaxGitHubBaselinePayloadBytes}).RunInDirectory(
		ctx,
		invocation.Executable,
		invocation.Arguments,
		githubBaselineEnvironment(),
		worktree,
		githubBaselineTimeout,
	)
	if err != nil {
		return assurance.GitHubBaselineResponse{}, err
	}
	return assurance.GitHubBaselineResponse{StatusCode: 200, Body: []byte(result.Stdout)}, nil
}

func githubBaselineEnvironment() []string {
	// GH_TOKEN and other credential-bearing environment variables are
	// intentionally excluded. GitHub CLI can use the user's existing local
	// credential-manager session while these values only locate its config.
	return environment.AllowlistedEnvironment([]string{"APPDATA", "LOCALAPPDATA", "GH_CONFIG_DIR"})
}

func (a *App) enrichPRCIBaseline(ctx context.Context, input BaselineInput, worktree domain.Worktree, localEntries []domain.BaselineEntry, localDigest string, localSources []string, observedAt time.Time) ([]domain.BaselineEntry, string, []string) {
	evidence, attempted := a.githubBaselineEvidence(ctx, input, worktree.Spec.CanonicalPath, observedAt)
	entries := appendProviderBaselineEntries(localEntries, evidence, attempted)
	return entries, combinedBaselineDigest(localDigest, evidence), combinedBaselineSources(localSources, evidence)
}

func (a *App) githubBaselineEvidence(ctx context.Context, input BaselineInput, worktree string, observedAt time.Time) ([]assurance.GitHubBaselineEvidence, bool) {
	owner, repository, found := a.githubRepository(ctx, input.ProjectID, input.RepositoryID)
	targetBranch := strings.TrimSpace(input.TargetBranch)
	if !found || targetBranch == "" {
		return nil, false
	}
	pathLookup := a.githubBaselinePath
	if pathLookup == nil {
		pathLookup = trustedGitHubCLIPath
	}
	ghPath, err := pathLookup()
	if err != nil {
		return unavailableGitHubBaselineEvidence(observedAt), true
	}
	executor := a.githubBaselineExecutor
	if executor == nil {
		executor = executeGitHubBaseline
	}
	evidence := make([]assurance.GitHubBaselineEvidence, 0, 2)
	for _, source := range []assurance.GitHubBaselineSource{assurance.GitHubBaselineSourceBranchProtection, assurance.GitHubBaselineSourceBranchRules} {
		metadata := assurance.GitHubBaselineMetadata{SourceID: string(source), ObservedAt: observedAt}
		invocation, buildErr := assurance.BuildGitHubBaselineInvocation(assurance.GitHubBaselineInput{Source: source, GHPath: ghPath, Owner: owner, Repository: repository, TargetBranch: targetBranch})
		if buildErr != nil {
			evidence = append(evidence, unknownGitHubBaselineEvidence(source, metadata))
			continue
		}
		response, executionErr := executor(ctx, invocation, worktree)
		if executionErr != nil {
			unavailable, _ := assurance.ParseGitHubBaselineResponseForSource(assurance.GitHubBaselineResponse{StatusCode: 503}, source, metadata)
			evidence = append(evidence, unavailable)
			continue
		}
		parsed, parseErr := assurance.ParseGitHubBaselineResponseForSource(response, source, metadata)
		if parseErr != nil {
			evidence = append(evidence, unknownGitHubBaselineEvidence(source, metadata))
			continue
		}
		evidence = append(evidence, parsed)
	}
	return evidence, true
}

func unavailableGitHubBaselineEvidence(observedAt time.Time) []assurance.GitHubBaselineEvidence {
	result := make([]assurance.GitHubBaselineEvidence, 0, 2)
	for _, source := range []assurance.GitHubBaselineSource{assurance.GitHubBaselineSourceBranchProtection, assurance.GitHubBaselineSourceBranchRules} {
		evidence, _ := assurance.ParseGitHubBaselineResponseForSource(assurance.GitHubBaselineResponse{StatusCode: 503}, source, assurance.GitHubBaselineMetadata{SourceID: string(source), ObservedAt: observedAt})
		result = append(result, evidence)
	}
	return result
}

func unknownGitHubBaselineEvidence(source assurance.GitHubBaselineSource, metadata assurance.GitHubBaselineMetadata) assurance.GitHubBaselineEvidence {
	// The parser owns the compact reason/digest. The empty object contains no
	// provider data and maps to a source-specific, fail-closed unknown state.
	evidence, _ := assurance.ParseGitHubBaselineResponseForSource(assurance.GitHubBaselineResponse{StatusCode: 200, Body: []byte("{}")}, source, metadata)
	return evidence
}

func (a *App) githubRepository(ctx context.Context, projectID, repositoryID string) (string, string, bool) {
	snapshot, err := a.Snapshot(ctx)
	if err != nil {
		return "", "", false
	}
	for _, project := range snapshot.Projects {
		if project.ID != projectID {
			continue
		}
		for _, repository := range project.Repos {
			if repository.ID != repositoryID {
				continue
			}
			normalized, host, provider, _ := collector.NormalizeRemote(repository.Origin)
			if host != "github.com" || provider != collector.ProviderGitHub {
				return "", "", false
			}
			parsed, parseErr := url.Parse(normalized)
			if parseErr != nil {
				return "", "", false
			}
			segments := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
			if len(segments) != 2 {
				return "", "", false
			}
			owner, ownerErr := url.PathUnescape(segments[0])
			name, nameErr := url.PathUnescape(segments[1])
			if ownerErr != nil || nameErr != nil {
				return "", "", false
			}
			return owner, name, true
		}
	}
	return "", "", false
}

func appendProviderBaselineEntries(entries []domain.BaselineEntry, evidence []assurance.GitHubBaselineEvidence, attempted bool) []domain.BaselineEntry {
	entries = append([]domain.BaselineEntry(nil), entries...)
	if !attempted {
		return append(entries, providerBaselineUnknownEntry(false))
	}
	merged := assurance.MergeGitHubBaselineEvidence(evidence...)
	if merged.Availability != assurance.BaselineAvailable {
		return append(entries, providerBaselineUnknownEntry(true))
	}
	if len(merged.RequiredContexts) == 0 {
		return append(entries, domain.BaselineEntry{ID: "github-required-checks", Name: "GitHub required checks", Classification: domain.BaselineObserved, SourcePath: "github.authoritative", Observed: true})
	}
	for _, context := range merged.RequiredContexts {
		entries = append(entries, domain.BaselineEntry{ID: "github-required-" + digestText(context)[7:23], Name: context, Classification: domain.BaselineRequired, SourcePath: "github.authoritative", Observed: true, Required: true})
	}
	return entries
}

func providerBaselineUnknownEntry(observed bool) domain.BaselineEntry {
	return domain.BaselineEntry{ID: "provider-rules-unknown", Name: "provider-enforced PR rules", Classification: domain.BaselineUnknown, SourcePath: "github.authoritative", Observed: observed}
}

func combinedBaselineDigest(localDigest string, evidence []assurance.GitHubBaselineEvidence) string {
	digests := make([]string, 0, len(evidence))
	for _, item := range evidence {
		digests = append(digests, item.SourceID+"\x00"+item.SourceDigest)
	}
	sort.Strings(digests)
	return digestText("pr-ci-baseline.v2", localDigest, digests)
}

func combinedBaselineSources(localSources []string, evidence []assurance.GitHubBaselineEvidence) []string {
	set := make(map[string]struct{}, len(localSources)+len(evidence))
	for _, source := range localSources {
		if strings.TrimSpace(source) != "" {
			set[source] = struct{}{}
		}
	}
	for _, item := range evidence {
		if item.SourceID != "" {
			set[item.SourceID] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for source := range set {
		result = append(result, source)
	}
	sort.Strings(result)
	return result
}
