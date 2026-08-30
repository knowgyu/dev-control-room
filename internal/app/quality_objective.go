package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/assurance"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/store"
)

const (
	qualityFindingResolvedReason       = "finding.resolved_without_worktree_proof"
	qualityFindingActiveReason         = "finding.active"
	qualityFindingStateInconclusive    = "finding.state_inconclusive"
	qualityCoverageProfileMissing      = "coverage.profile_missing"
	qualityCoverageRunnerUnavailable   = "coverage.runner_unavailable"
	qualityCoverageConfigMismatch      = "coverage.config_mismatch"
	qualityCoverageHeadMismatch        = "coverage.head_mismatch"
	qualityCoverageTechniqueMismatch   = "coverage.technique_mismatch"
	qualityCoverageTestFailed          = "coverage.tests_failed"
	qualityCoverageThresholdNotMet     = "coverage.threshold_not_met"
	qualityCoverageIdentityUnavailable = "coverage.identity_unavailable"
)

func (a *App) DecideQualityObjective(ctx context.Context, id string, input QualityObjectiveDecisionInput) (domain.QualityObjective, error) {
	objective, revision, err := a.qualityObjectiveWithRevision(ctx, id, input.ExpectedRevision)
	if err != nil {
		return domain.QualityObjective{}, err
	}
	decision := domain.QualityObjectiveDecision{
		Disposition: strings.TrimSpace(input.Disposition), Action: strings.TrimSpace(input.Action),
		Reason: strings.TrimSpace(input.Reason), Actor: strings.TrimSpace(input.Actor),
		MinimumPercent: input.MinimumPercent, DecidedAt: time.Now().UTC(),
	}
	updated := objective.Clone()
	if err := updated.ApplyDecision(decision); err != nil {
		return domain.QualityObjective{}, qualityObjectiveDecisionError(err)
	}
	if err := a.store.UpdateQualityObjectiveRevisionCAS(ctx, domain.QualityObjectiveKind, id, revision, updated); err != nil {
		return domain.QualityObjective{}, qualityObjectiveCASResult(err)
	}
	return updated, nil
}

func (a *App) RevalidateQualityObjective(ctx context.Context, id string, input QualityObjectiveRevalidationInput) (domain.QualityObjective, error) {
	objective, revision, err := a.qualityObjectiveWithRevision(ctx, id, input.ExpectedRevision)
	if err != nil {
		return domain.QualityObjective{}, err
	}
	if objective.Spec.PrimarySignal == nil {
		return domain.QualityObjective{}, contract.InvalidInput("quality objective primary signal is required")
	}
	if objective.Spec.Decision == nil {
		return domain.QualityObjective{}, contract.Conflict("quality objective must have a decision before revalidation")
	}

	revalidation, err := a.deriveQualityObjectiveRevalidation(ctx, objective, input)
	if err != nil {
		return domain.QualityObjective{}, err
	}
	updated := objective.Clone()
	if err := updated.RecordRevalidation(revalidation); err != nil {
		return domain.QualityObjective{}, qualityObjectiveMutationError(err)
	}
	if err := a.store.UpdateQualityObjectiveRevisionCAS(ctx, domain.QualityObjectiveKind, id, revision, updated); err != nil {
		return domain.QualityObjective{}, qualityObjectiveCASResult(err)
	}
	return updated, nil
}

func (a *App) ConfirmQualityObjective(ctx context.Context, id string, input QualityObjectiveConfirmationInput) (domain.QualityObjective, error) {
	objective, revision, err := a.qualityObjectiveWithRevision(ctx, id, input.ExpectedRevision)
	if err != nil {
		return domain.QualityObjective{}, err
	}
	storedWorktree, err := a.Worktree(ctx, objective.Spec.ProjectID, objective.Spec.RepositoryID, objective.Spec.WorktreeID)
	if err != nil {
		return domain.QualityObjective{}, err
	}
	if _, err := a.revalidateAssuranceWorktree(ctx, storedWorktree); err != nil {
		return domain.QualityObjective{}, contract.Conflict("quality objective Worktree could not be freshly verified")
	}

	latest, ok := objective.LatestRevalidation()
	if !ok {
		return domain.QualityObjective{}, contract.Conflict("quality objective has no fresh revalidation")
	}
	configDigest := ""
	if latest.SourceKind == domain.QualityObjectiveSignalKindGoCoverage {
		run, found, findErr := a.qualityRun(ctx, latest.SourceID)
		if findErr != nil {
			return domain.QualityObjective{}, findErr
		}
		if !found {
			return domain.QualityObjective{}, contract.NotFound("quality run not found")
		}
		coveragePath, pathErr := qualityCoveragePathFromRun(a.home, run)
		if pathErr != nil {
			return domain.QualityObjective{}, contract.Conflict("quality coverage configuration could not be freshly verified")
		}
		selection, selectErr := assurance.NewQualityRunnerRegistry().Select(assurance.QualityRunnerSelectionRequest{TechniqueID: domain.QualityTechniqueGoTestCoverage, WorktreeRoot: storedWorktree.Spec.CanonicalPath, CoveragePath: coveragePath})
		if selectErr != nil || selection.State != assurance.QualityRunnerSelectionAvailable || selection.Metadata.ConfigDigest == "" {
			return domain.QualityObjective{}, contract.Conflict("quality coverage runner is unavailable")
		}
		if run.Spec.Coverage == nil || strings.TrimSpace(run.Spec.Coverage.ProfileArtifactID) == "" {
			return domain.QualityObjective{}, contract.Conflict("quality coverage profile artifact is not available or cannot be verified")
		}
		profileVerified, profileErr := a.qualityCoverageProfileArtifactVerified(ctx, run.Spec.Coverage.ProfileArtifactID)
		if profileErr != nil {
			return domain.QualityObjective{}, profileErr
		}
		if !profileVerified {
			return domain.QualityObjective{}, contract.Conflict("quality coverage profile artifact is not available or cannot be verified")
		}
		configDigest = selection.Metadata.ConfigDigest
	}

	updated := objective.Clone()
	if err := updated.ConfirmAdoption(domain.QualityObjectiveFreshnessInput{Head: storedWorktree.Spec.Head, ConfigDigest: configDigest, AsOf: time.Now().UTC()}); err != nil {
		return domain.QualityObjective{}, contract.Conflict("quality objective revalidation is not fresh enough to confirm adoption")
	}
	if err := a.store.UpdateQualityObjectiveRevisionCAS(ctx, domain.QualityObjectiveKind, id, revision, updated); err != nil {
		return domain.QualityObjective{}, qualityObjectiveCASResult(err)
	}
	return updated, nil
}

func (a *App) qualityObjectiveWithRevision(ctx context.Context, id string, expectedRevision int) (domain.QualityObjective, int, error) {
	if expectedRevision < 1 {
		return domain.QualityObjective{}, 0, contract.InvalidInput("expectedRevision must be a positive integer")
	}
	var objective domain.QualityObjective
	revision, err := a.store.GetAssuranceWithRevision(ctx, domain.QualityObjectiveKind, id, &objective)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.QualityObjective{}, 0, contract.NotFound("quality objective not found")
	}
	if err != nil {
		return domain.QualityObjective{}, 0, err
	}
	if err := objective.Validate(); err != nil {
		return domain.QualityObjective{}, 0, err
	}
	if objective.Spec.Revision != revision {
		return domain.QualityObjective{}, 0, errors.New("quality objective stored revision does not match its snapshot")
	}
	if expectedRevision != revision {
		return domain.QualityObjective{}, 0, contract.Conflict("quality objective revision is stale")
	}
	return objective, revision, nil
}

func (a *App) qualityObjectivePrimarySignal(ctx context.Context, objective domain.QualityObjective, requested *domain.QualityObjectiveSignal) (*domain.QualityObjectiveSignal, error) {
	signal := copyQualityObjectiveSignal(requested)
	if signal == nil {
		if len(objective.Spec.FindingIDs) == 1 {
			signal = &domain.QualityObjectiveSignal{Kind: domain.QualityObjectiveSignalKindFinding, ID: objective.Spec.FindingIDs[0]}
		} else if len(objective.Spec.RunIDs) == 1 {
			signal = &domain.QualityObjectiveSignal{Kind: domain.QualityObjectiveSignalKindGoCoverage, ID: objective.Spec.RunIDs[0]}
		} else {
			return nil, nil
		}
	}
	signal.Kind, signal.ID = strings.TrimSpace(signal.Kind), strings.TrimSpace(signal.ID)
	switch signal.Kind {
	case domain.QualityObjectiveSignalKindFinding:
		finding, err := a.Finding(ctx, signal.ID)
		if err != nil {
			return nil, err
		}
		if err := validateQualityObjectiveLinkScope(objective.Spec, finding.Spec.ProjectID, finding.Spec.RepositoryID, ""); err != nil {
			return nil, err
		}
		if signal.Fingerprint != "" && signal.Fingerprint != finding.Spec.Fingerprint {
			return nil, contract.InvalidInput("quality objective primary finding fingerprint does not match the stored finding")
		}
		signal.Fingerprint = finding.Spec.Fingerprint
		signal.Head, signal.ConfigDigest, signal.ObservedAt = "", "", finding.Spec.LastObserved
	case domain.QualityObjectiveSignalKindGoCoverage:
		run, found, err := a.qualityRun(ctx, signal.ID)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, contract.NotFound("quality run not found")
		}
		if err := validateQualityObjectiveLinkScope(objective.Spec, run.Spec.ProjectID, run.Spec.RepositoryID, run.Spec.WorktreeID); err != nil {
			return nil, err
		}
		if run.Spec.Technique != domain.QualityTechniqueGoTestCoverage {
			return nil, contract.InvalidInput("quality objective coverage signal must reference a go coverage run")
		}
		if signal.ConfigDigest != "" && signal.ConfigDigest != run.Spec.ConfigDigest {
			return nil, contract.InvalidInput("quality objective primary coverage config does not match the stored run")
		}
		signal.ConfigDigest = run.Spec.ConfigDigest
		if signal.ConfigDigest == "" {
			return nil, contract.InvalidInput("quality objective primary coverage config is required")
		}
		if signal.Fingerprint == "" {
			signal.Fingerprint = run.Spec.OutputDigest
		}
		signal.Head, signal.ObservedAt = run.Spec.Head, run.Spec.StartedAt
		if run.Spec.CompletedAt != nil {
			signal.ObservedAt = *run.Spec.CompletedAt
		}
	default:
		return nil, contract.InvalidInput("quality objective primary signal kind is invalid")
	}
	return signal, nil
}

func (a *App) qualityRun(ctx context.Context, id string) (domain.QualityRun, bool, error) {
	runs, err := a.QualityRuns(ctx)
	if err != nil {
		return domain.QualityRun{}, false, err
	}
	for _, run := range runs {
		if run.Metadata.ID == id {
			return run, true, nil
		}
	}
	return domain.QualityRun{}, false, nil
}

func (a *App) deriveQualityObjectiveRevalidation(ctx context.Context, objective domain.QualityObjective, input QualityObjectiveRevalidationInput) (domain.QualityObjectiveRevalidation, error) {
	signal := objective.Spec.PrimarySignal
	findingID := strings.TrimSpace(input.FindingID)
	qualityRunID := strings.TrimSpace(input.QualityRunID)
	if (findingID == "") == (qualityRunID == "") {
		return domain.QualityObjectiveRevalidation{}, contract.InvalidInput("revalidation requires exactly one findingId or qualityRunId")
	}
	if findingID != "" {
		if signal.Kind != domain.QualityObjectiveSignalKindFinding {
			return domain.QualityObjectiveRevalidation{}, contract.InvalidInput("finding revalidation does not match the primary signal")
		}
		finding, err := a.Finding(ctx, findingID)
		if err != nil {
			return domain.QualityObjectiveRevalidation{}, err
		}
		if err := validateQualityObjectiveLinkScope(objective.Spec, finding.Spec.ProjectID, finding.Spec.RepositoryID, ""); err != nil {
			return domain.QualityObjectiveRevalidation{}, err
		}
		if finding.Spec.Fingerprint != signal.Fingerprint {
			return domain.QualityObjectiveRevalidation{}, contract.InvalidInput("finding revalidation fingerprint does not match the primary signal")
		}
		outcome, reason := domain.QualityObjectiveRevalidationInconclusive, qualityFindingStateInconclusive
		switch finding.Spec.State {
		case domain.FindingResolved:
			outcome, reason = domain.QualityObjectiveRevalidationImproved, qualityFindingResolvedReason
		case domain.FindingOpen, domain.FindingAcknowledged:
			outcome, reason = domain.QualityObjectiveRevalidationNotImproved, qualityFindingActiveReason
		}
		return domain.QualityObjectiveRevalidation{SourceKind: signal.Kind, SourceID: finding.Metadata.ID, Outcome: outcome, ReasonCode: reason, Head: objective.Spec.Head, CheckedAt: time.Now().UTC()}, nil
	}

	run, found, err := a.qualityRun(ctx, qualityRunID)
	if err != nil {
		return domain.QualityObjectiveRevalidation{}, err
	}
	if !found {
		return domain.QualityObjectiveRevalidation{}, contract.NotFound("quality run not found")
	}
	if err := validateQualityObjectiveLinkScope(objective.Spec, run.Spec.ProjectID, run.Spec.RepositoryID, run.Spec.WorktreeID); err != nil {
		return domain.QualityObjectiveRevalidation{}, err
	}
	if signal.Kind != domain.QualityObjectiveSignalKindGoCoverage {
		return domain.QualityObjectiveRevalidation{}, contract.InvalidInput("coverage revalidation does not match the primary signal")
	}
	worktree, err := a.Worktree(ctx, objective.Spec.ProjectID, objective.Spec.RepositoryID, objective.Spec.WorktreeID)
	if err != nil {
		return domain.QualityObjectiveRevalidation{}, err
	}
	configDigest := run.Spec.ConfigDigest
	if configDigest == "" {
		configDigest = signal.ConfigDigest
	}
	result := domain.QualityObjectiveRevalidation{SourceKind: signal.Kind, SourceID: run.Metadata.ID, Outcome: domain.QualityObjectiveRevalidationInconclusive, ReasonCode: qualityCoverageIdentityUnavailable, Head: run.Spec.Head, ConfigDigest: configDigest, CheckedAt: time.Now().UTC()}
	if result.Head == "" {
		result.Head = worktree.Spec.Head
	}
	if configDigest == "" {
		return domain.QualityObjectiveRevalidation{}, contract.Conflict("coverage revalidation has no server-derived config identity")
	}
	if run.Spec.Technique != domain.QualityTechniqueGoTestCoverage {
		result.ReasonCode = qualityCoverageTechniqueMismatch
		return result, nil
	}
	if run.Spec.ConfigDigest != signal.ConfigDigest {
		result.ReasonCode = qualityCoverageConfigMismatch
		return result, nil
	}
	if run.Spec.Head != worktree.Spec.Head {
		result.ReasonCode = qualityCoverageHeadMismatch
		return result, nil
	}
	if run.Spec.Coverage == nil || strings.TrimSpace(run.Spec.Coverage.ProfileArtifactID) == "" {
		result.ReasonCode = qualityCoverageProfileMissing
		return result, nil
	}
	profileVerified, profileErr := a.qualityCoverageProfileArtifactVerified(ctx, run.Spec.Coverage.ProfileArtifactID)
	if profileErr != nil {
		return domain.QualityObjectiveRevalidation{}, profileErr
	}
	if !profileVerified {
		result.ReasonCode = qualityCoverageProfileMissing
		return result, nil
	}
	if run.Spec.State == domain.AssuranceStateFailed && run.Spec.Outcome == domain.QualityRunOutcomeTestsFailed {
		result.Outcome, result.ReasonCode = domain.QualityObjectiveRevalidationNotImproved, qualityCoverageTestFailed
		return result, nil
	}
	if run.Spec.State != domain.AssuranceStateSucceeded || run.Spec.Outcome != domain.QualityRunOutcomeCoverageCollected {
		result.ReasonCode = qualityCoverageRunnerUnavailable
		return result, nil
	}
	if run.Spec.Coverage.Percent < objective.Spec.Decision.MinimumPercent {
		result.Outcome, result.ReasonCode = domain.QualityObjectiveRevalidationNotImproved, qualityCoverageThresholdNotMet
		return result, nil
	}
	result.Outcome = domain.QualityObjectiveRevalidationImproved
	result.ReasonCode = "coverage.improved"
	return result, nil
}

func qualityCoveragePathFromRun(home string, run domain.QualityRun) (string, error) {
	for _, argument := range run.Spec.Command.Arguments {
		if !strings.HasPrefix(argument, "-coverprofile=") {
			continue
		}
		path := strings.TrimPrefix(argument, "-coverprofile=")
		absolute, err := filepath.Abs(path)
		if err != nil || !filepath.IsAbs(absolute) || !strings.EqualFold(filepath.Ext(absolute), ".out") {
			return "", errors.New("quality coverage profile path is invalid")
		}
		root, rootErr := filepath.Abs(home)
		if rootErr != nil || !pathWithin(root, absolute) {
			return "", errors.New("quality coverage profile path is outside the application home")
		}
		return absolute, nil
	}
	return "", errors.New("quality coverage profile path is missing")
}

// qualityCoverageProfileArtifactVerified proves that a QualityRun's profile
// reference still points to intact, server-owned evidence. The ID alone is
// only a link and must never make a revalidation improved.
func (a *App) qualityCoverageProfileArtifactVerified(ctx context.Context, id string) (bool, error) {
	item, err := a.assuranceArtifact(ctx, strings.TrimSpace(id))
	if err != nil {
		var coded contract.CodedError
		if errors.As(err, &coded) && coded.Code == contract.ErrorNotFound {
			return false, nil
		}
		return false, err
	}
	if item.Spec.Retention == domain.ArtifactRetentionDeleted {
		return false, nil
	}
	root, rootErr := filepath.Abs(filepath.Join(a.home, "artifacts", "assurance"))
	path, pathErr := filepath.Abs(filepath.Clean(item.Spec.Path))
	if rootErr != nil || pathErr != nil || !pathWithin(root, path) {
		return false, nil
	}
	rootInfo, infoErr := os.Lstat(root)
	if infoErr != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return false, nil
	}
	resolvedRoot, rootResolveErr := filepath.EvalSymlinks(root)
	resolvedPath, pathResolveErr := filepath.EvalSymlinks(path)
	if rootResolveErr != nil || pathResolveErr != nil || !pathWithin(resolvedRoot, resolvedPath) {
		return false, nil
	}
	data, readErr := readRegularFile(path)
	if readErr != nil {
		return false, nil
	}
	if int64(len(data)) != item.Spec.Size || artifactHash(data) != item.Spec.SHA256 {
		return false, nil
	}
	return true, nil
}

func qualityObjectiveDecisionError(err error) error {
	if strings.Contains(err.Error(), "terminal") || strings.Contains(err.Error(), "not allowed") {
		return contract.Conflict("quality objective decision is not allowed in its current state")
	}
	return contract.InvalidInput(err.Error())
}

func qualityObjectiveMutationError(err error) error {
	if strings.Contains(err.Error(), "terminal") || strings.Contains(err.Error(), "not allowed") {
		return contract.Conflict("quality objective revalidation is not allowed in its current state")
	}
	return contract.InvalidInput(err.Error())
}

func qualityObjectiveCASResult(err error) error {
	switch {
	case errors.Is(err, store.ErrQualityObjectiveNotFound), errors.Is(err, sql.ErrNoRows):
		return contract.NotFound("quality objective not found")
	case errors.Is(err, store.ErrQualityObjectiveRevisionStale):
		return contract.Conflict("quality objective revision is stale")
	default:
		return err
	}
}

func decodeQualityObjectiveRevalidationBody(response http.ResponseWriter, request *http.Request) (QualityObjectiveRevalidationInput, error) {
	var input QualityObjectiveRevalidationInput
	if err := decodeQualityObjectiveJSONBody(response, request, &input); err != nil {
		return QualityObjectiveRevalidationInput{}, errors.New("invalid revalidation body")
	}
	input.FindingID = strings.TrimSpace(input.FindingID)
	input.QualityRunID = strings.TrimSpace(input.QualityRunID)
	if (input.FindingID == "") == (input.QualityRunID == "") {
		return QualityObjectiveRevalidationInput{}, errors.New("revalidation requires exactly one source")
	}
	return input, nil
}

func decodeQualityObjectiveDecisionBody(response http.ResponseWriter, request *http.Request) (QualityObjectiveDecisionInput, error) {
	var input QualityObjectiveDecisionInput
	if err := decodeQualityObjectiveJSONBody(response, request, &input); err != nil {
		return QualityObjectiveDecisionInput{}, errors.New("invalid decision body")
	}
	return input, nil
}

func decodeQualityObjectiveConfirmationBody(response http.ResponseWriter, request *http.Request) (QualityObjectiveConfirmationInput, error) {
	var input QualityObjectiveConfirmationInput
	if err := decodeQualityObjectiveJSONBody(response, request, &input); err != nil {
		return QualityObjectiveConfirmationInput{}, errors.New("invalid confirmation body")
	}
	return input, nil
}

func decodeQualityObjectiveJSONBody(response http.ResponseWriter, request *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("trailing JSON value")
	}
	return nil
}
