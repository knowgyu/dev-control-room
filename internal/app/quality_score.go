package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

const (
	QualityScorePenaltyInfo           = 5
	QualityScorePenaltyAttention      = 15
	QualityScorePenaltyHigh           = 30
	QualityScorePenaltyCritical       = 50
	qualityScoreUnavailableConfidence = 25
)

type RepositoryQualityScoreInput struct {
	Plan    domain.InspectionPlan
	Results []domain.InspectionResult
	Current *domain.InspectionFreshness
}

type QualityScoreInput = RepositoryQualityScoreInput

func CalculateRepositoryQualityScore(input RepositoryQualityScoreInput) (domain.RepositoryQualityScore, error) {
	if err := input.Plan.Validate(); err != nil {
		return domain.RepositoryQualityScore{}, err
	}
	results := make(map[string]domain.InspectionResult, len(input.Results))
	for _, result := range input.Results {
		if !validInspectionResult(result) {
			return domain.RepositoryQualityScore{}, errors.New("inspection result is invalid")
		}
		key := result.ComponentID + "\x00" + result.CheckID
		if _, exists := results[key]; exists {
			return domain.RepositoryQualityScore{}, errors.New("inspection results must be unique")
		}
		results[key] = result
	}

	components := make([]domain.RepositoryQualityComponent, 0, len(input.Plan.Spec.Checks))
	status := domain.QualityScoreStatusInconclusive
	if input.Current != nil && completeInspectionFreshness(*input.Current) {
		status = domain.QualityScoreStatusFresh
	}
	planChecks := make(map[string]struct{}, len(input.Plan.Spec.Checks))
	for _, check := range input.Plan.Spec.Checks {
		planChecks[check.ComponentID+"\x00"+check.ID] = struct{}{}
	}
	for key := range results {
		if _, ok := planChecks[key]; !ok {
			return domain.RepositoryQualityScore{}, errors.New("inspection result does not belong to plan")
		}
	}
	confidenceTotal := 0
	overallTotal := 0
	for _, check := range input.Plan.Spec.Checks {
		key := check.ComponentID + "\x00" + check.ID
		result, ok := results[key]
		component := domain.RepositoryQualityComponent{ComponentID: check.ComponentID, CheckID: check.ID, Outcome: domain.InspectionOutcomeInconclusive, Confidence: qualityScoreUnavailableConfidence}
		if ok {
			component.Value, component.Confidence = inspectionValue(result)
			component.Outcome = result.Outcome
			component.FindingCount = inspectionFindingCount(result.Findings)
			if result.Outcome == domain.InspectionOutcomeToolError || result.Outcome == domain.InspectionOutcomeRunnerUnavailable || result.Outcome == domain.InspectionOutcomeInconclusive {
				status = domain.QualityScoreStatusInconclusive
			}
		} else {
			status = domain.QualityScoreStatusInconclusive
		}
		components = append(components, component)
		overallTotal += component.Value
		confidenceTotal += component.Confidence
	}
	if input.Current != nil && input.Plan.IsStale(*input.Current) {
		status = domain.QualityScoreStatusStale
	}

	checks := append([]domain.InspectionCheck{}, input.Plan.Spec.Checks...)
	sort.Slice(checks, func(i, j int) bool { return inspectionCheckKey(checks[i]) < inspectionCheckKey(checks[j]) })
	checkSet := make([]string, 0, len(checks))
	for _, check := range checks {
		checkSet = append(checkSet, check.ComponentID+":"+check.ID)
	}
	basisDigest, err := qualityBasisDigest(input.Plan, checks)
	if err != nil {
		return domain.RepositoryQualityScore{}, err
	}
	now := input.Plan.Spec.UpdatedAt
	if now.IsZero() {
		now = input.Plan.Spec.CreatedAt
	}
	score := domain.RepositoryQualityScore{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.RepositoryQualityScoreKind},
		Metadata: domain.ObjectMeta{ID: input.Plan.Metadata.ID + "-score", Name: input.Plan.Metadata.Name + " quality score"},
		Spec: domain.RepositoryQualityScoreSpec{
			ProjectID: input.Plan.Spec.ProjectID, RepositoryID: input.Plan.Spec.RepositoryID, WorktreeID: input.Plan.Spec.WorktreeID,
			Branch: input.Plan.Spec.Branch, Head: input.Plan.Spec.Head, ScoreVersion: domain.RepositoryQualityScoreVersion,
			Overall: clampScore(overallTotal / len(components)), Confidence: clampScore(confidenceTotal / len(components)), Status: status,
			Components: components, Checks: checks, CheckSet: checkSet, BaselineDigest: input.Plan.Spec.BaselineDigest,
			ConfigDigest: input.Plan.Spec.ConfigDigest, EvidenceDigest: input.Plan.Spec.EvidenceDigest, ToolDigest: input.Plan.Spec.ToolDigest,
			BasisDigest: basisDigest, Revision: 1, CreatedAt: input.Plan.Spec.CreatedAt, UpdatedAt: now,
		},
	}
	return score, nil
}

func ScoreRepositoryQuality(plan domain.InspectionPlan, results []domain.InspectionResult) (domain.RepositoryQualityScore, error) {
	return CalculateRepositoryQualityScore(RepositoryQualityScoreInput{Plan: plan, Results: results})
}

func CompareQualityScores(before, after domain.RepositoryQualityScore) domain.QualityComparison {
	return CompareQualityScorePair(domain.QualityComparisonInput{Before: &before, After: &after})
}

func CompareQualityScorePair(input domain.QualityComparisonInput) domain.QualityComparison {
	inconclusive := func(reason string) domain.QualityComparison {
		return domain.QualityComparison{Status: domain.QualityComparisonInconclusive, Reason: reason}
	}
	if input.Before == nil || input.After == nil {
		return inconclusive("before_and_after_are_required")
	}
	if err := input.Before.Validate(); err != nil {
		return inconclusive("before_score_invalid")
	}
	if err := input.After.Validate(); err != nil {
		return inconclusive("after_score_invalid")
	}
	before, after := input.Before.Spec, input.After.Spec
	if before.ScoreVersion != after.ScoreVersion {
		return inconclusive("score_version_mismatch")
	}
	if before.ProjectID != after.ProjectID || before.RepositoryID != after.RepositoryID || before.WorktreeID != after.WorktreeID {
		return inconclusive("target_identity_mismatch")
	}
	if before.Head != after.Head {
		return inconclusive("head_mismatch")
	}
	if before.BaselineDigest != after.BaselineDigest {
		return inconclusive("baseline_digest_mismatch")
	}
	if !sameChecks(before.Checks, after.Checks) || !sameStrings(before.CheckSet, after.CheckSet) {
		return inconclusive("check_set_mismatch")
	}
	if before.ConfigDigest != after.ConfigDigest {
		return inconclusive("config_digest_mismatch")
	}
	if before.ToolDigest != after.ToolDigest {
		return inconclusive("tool_digest_mismatch")
	}
	if before.EvidenceDigest != after.EvidenceDigest {
		return inconclusive("evidence_digest_mismatch")
	}
	if before.BasisDigest != after.BasisDigest {
		return inconclusive("basis_digest_mismatch")
	}
	if before.Status != domain.QualityScoreStatusFresh || after.Status != domain.QualityScoreStatusFresh {
		return inconclusive("score_is_not_fresh")
	}
	return domain.QualityComparison{Status: domain.QualityComparisonComparable, BeforeOverall: before.Overall, AfterOverall: after.Overall, Delta: after.Overall - before.Overall}
}

func CompareRepositoryQuality(before, after domain.RepositoryQualityScore) domain.QualityComparison {
	return CompareQualityScores(before, after)
}

func inspectionValue(result domain.InspectionResult) (int, int) {
	switch result.Outcome {
	case domain.InspectionOutcomeClean:
		return 100, 100
	case domain.InspectionOutcomeFindings:
		value := 100
		for _, finding := range result.Findings {
			penalty := 0
			switch finding.Severity {
			case domain.SeverityInfo:
				penalty = QualityScorePenaltyInfo
			case domain.SeverityAttention:
				penalty = QualityScorePenaltyAttention
			case domain.SeverityHigh:
				penalty = QualityScorePenaltyHigh
			case domain.SeverityCritical:
				penalty = QualityScorePenaltyCritical
			}
			count := finding.Count
			if count < 1 {
				count = 1
			}
			value -= penalty * count
		}
		return clampScore(value), 100
	case domain.InspectionOutcomeTestsFailed:
		return 0, 100
	default:
		return 0, qualityScoreUnavailableConfidence
	}
}

func validInspectionResult(result domain.InspectionResult) bool {
	if result.ComponentID == "" || result.CheckID == "" {
		return false
	}
	for _, finding := range result.Findings {
		if finding.Count < 0 || (finding.Severity != domain.SeverityInfo && finding.Severity != domain.SeverityAttention && finding.Severity != domain.SeverityHigh && finding.Severity != domain.SeverityCritical) {
			return false
		}
	}
	return result.Outcome == domain.InspectionOutcomeClean || result.Outcome == domain.InspectionOutcomeFindings || result.Outcome == domain.InspectionOutcomeTestsFailed || result.Outcome == domain.InspectionOutcomeToolError || result.Outcome == domain.InspectionOutcomeRunnerUnavailable || result.Outcome == domain.InspectionOutcomeInconclusive
}

func completeInspectionFreshness(value domain.InspectionFreshness) bool {
	return value.Head != "" && value.ConfigDigest != "" && value.ToolDigest != "" && value.EvidenceDigest != ""
}

func inspectionFindingCount(findings []domain.InspectionFinding) int {
	total := 0
	for _, finding := range findings {
		count := finding.Count
		if count < 1 {
			count = 1
		}
		total += count
	}
	return total
}
func clampScore(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}
func inspectionCheckKey(check domain.InspectionCheck) string {
	return check.ComponentID + "\x00" + check.ID + "\x00" + check.AdapterID + "\x00" + check.ParserID
}
func sameStrings(first, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	for index := range first {
		if first[index] != second[index] {
			return false
		}
	}
	return true
}
func sameChecks(first, second []domain.InspectionCheck) bool {
	if len(first) != len(second) {
		return false
	}
	for index := range first {
		if inspectionCheckKey(first[index]) != inspectionCheckKey(second[index]) {
			return false
		}
	}
	return true
}

func qualityBasisDigest(plan domain.InspectionPlan, checks []domain.InspectionCheck) (string, error) {
	payload := struct {
		ScoreVersion, BaselineDigest, ConfigDigest, EvidenceDigest, ToolDigest string
		Checks                                                                 []domain.InspectionCheck
	}{
		ScoreVersion: domain.RepositoryQualityScoreVersion, BaselineDigest: plan.Spec.BaselineDigest, ConfigDigest: plan.Spec.ConfigDigest,
		EvidenceDigest: plan.Spec.EvidenceDigest, ToolDigest: plan.Spec.ToolDigest, Checks: checks,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
