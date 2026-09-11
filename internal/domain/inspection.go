package domain

import (
	"errors"
	"sort"
	"strings"
	"time"
)

const (
	InspectionPlanKind             = "InspectionPlan"
	RepositoryQualityScoreKind     = "RepositoryQualityScore"
	QualityImprovementProposalKind = "QualityImprovementProposal"
	InspectionPlanVersion          = "inspection-plan-v1"
	RepositoryQualityScoreVersion  = "repository-quality-v1"

	InspectionPlanStateProposed = "proposed"
	InspectionPlanStateReviewed = "reviewed"
	InspectionPlanStateApproved = "approved"
	InspectionPlanStateStale    = "stale"
	InspectionPlanStateRejected = "rejected"

	InspectionGenerationDeterministic = "deterministic"
	InspectionGenerationAI            = "ai"

	QualityImprovementStateProposed = "proposed"
	QualityImprovementStateReviewed = "reviewed"
	QualityImprovementStateApproved = "approved"
	QualityImprovementStateRejected = "rejected"
	QualityImprovementStateApplied  = "applied"
	QualityImprovementStateStale    = "stale"

	QualityImprovementActionAddCheck              = "add_check"
	QualityImprovementActionRemoveCheck           = "remove_check"
	QualityImprovementActionMarkRunnerUnavailable = "mark_runner_unavailable"

	QualityImprovementReasonApplicableCheck   = "applicable_check"
	QualityImprovementReasonFindingObserved   = "finding_observed"
	QualityImprovementReasonInconclusive      = "inconclusive"
	QualityImprovementReasonRunnerUnavailable = "runner_unavailable"

	InspectionCheckRuff              = "ruff"
	InspectionCheckPytest            = "pytest"
	InspectionCheckESLint            = "eslint"
	InspectionCheckVitest            = "vitest"
	InspectionCheckGoTest            = "quality.go.test"
	InspectionCheckGoTestRace        = "quality.go.test_race"
	InspectionCheckGoVet             = "quality.go.vet"
	InspectionCheckGoModVerify       = "quality.go.mod_verify"
	InspectionCheckGoBuild           = "quality.go.build"
	InspectionCheckGoCoverage        = "quality.go.coverage"
	InspectionCheckGoCoveragePercent = "quality.go.coverage_percent"
	InspectionCheckGoMutation        = "quality.go.mutation"
	InspectionCheckGoProperty        = "quality.go.property"
	InspectionCheckGoFuzz            = "quality.go.fuzz"
	InspectionCheckGoE2E             = "quality.go.e2e"
	InspectionCheckGoTestCoverage    = "quality.go.test_coverage"

	InspectionOutcomeClean             = "clean"
	InspectionOutcomeFindings          = "findings"
	InspectionOutcomeTestsFailed       = "tests_failed"
	InspectionOutcomeToolError         = "tool_error"
	InspectionOutcomeRunnerUnavailable = "runner_unavailable"
	InspectionOutcomeInconclusive      = "inconclusive"

	QualityScoreStatusFresh        = "fresh"
	QualityScoreStatusStale        = "stale"
	QualityScoreStatusInconclusive = "inconclusive"
	QualityComparisonComparable    = "comparable"
	QualityComparisonInconclusive  = "inconclusive"
)

var inspectionCheckIDs = map[string]struct{}{
	InspectionCheckRuff: {}, InspectionCheckPytest: {}, InspectionCheckESLint: {}, InspectionCheckVitest: {},
	InspectionCheckGoTest: {}, InspectionCheckGoTestRace: {}, InspectionCheckGoVet: {},
	InspectionCheckGoModVerify: {}, InspectionCheckGoBuild: {}, InspectionCheckGoCoverage: {},
	InspectionCheckGoCoveragePercent: {}, InspectionCheckGoMutation: {}, InspectionCheckGoProperty: {},
	InspectionCheckGoFuzz: {}, InspectionCheckGoE2E: {}, InspectionCheckGoTestCoverage: {},
}

type InspectionPlan struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta         `json:"metadata"`
	Spec     InspectionPlanSpec `json:"spec"`
}

type InspectionPlanSpec struct {
	ProjectID           string                `json:"projectId"`
	RepositoryID        string                `json:"repositoryId"`
	WorktreeID          string                `json:"worktreeId"`
	Branch              string                `json:"branch"`
	Head                string                `json:"head"`
	BaselineDigest      string                `json:"baselineDigest"`
	ConfigDigest        string                `json:"configDigest"`
	EvidenceDigest      string                `json:"evidenceDigest"`
	ToolDigest          string                `json:"toolDigest"`
	Components          []InspectionComponent `json:"components"`
	Checks              []InspectionCheck     `json:"checks"`
	GenerationSource    string                `json:"generationSource,omitempty"`
	GenerationProvider  string                `json:"generationProvider,omitempty"`
	GenerationModel     string                `json:"generationModel,omitempty"`
	AIProposalDigest    string                `json:"aiProposalDigest,omitempty"`
	AIProposalDecisions []string              `json:"aiProposalDecisions,omitempty"`
	State               string                `json:"state"`
	Revision            int                   `json:"revision"`
	Version             string                `json:"version"`
	Digest              string                `json:"digest"`
	CreatedAt           time.Time             `json:"createdAt"`
	UpdatedAt           time.Time             `json:"updatedAt"`
}

type InspectionComponent struct {
	ID        string `json:"id"`
	AdapterID string `json:"adapterId,omitempty"`
	ParserID  string `json:"parserId,omitempty"`
}

type InspectionCheck struct {
	ID          string `json:"id"`
	ComponentID string `json:"componentId"`
	AdapterID   string `json:"adapterId"`
	ParserID    string `json:"parserId"`
}

type InspectionFreshness struct {
	Head           string
	ConfigDigest   string
	ToolDigest     string
	EvidenceDigest string
}

type InspectionFinding struct {
	ID       string   `json:"id,omitempty"`
	Severity Severity `json:"severity"`
	Count    int      `json:"count,omitempty"`
}

type InspectionResult struct {
	ComponentID string              `json:"componentId"`
	CheckID     string              `json:"checkId"`
	Outcome     string              `json:"outcome"`
	Findings    []InspectionFinding `json:"findings,omitempty"`
	Coverage    *QualityCoverage    `json:"coverage,omitempty"`
	Prompt      string              `json:"prompt,omitempty"`
	Model       string              `json:"model,omitempty"`
	Usage       map[string]any      `json:"usage,omitempty"`
}

type RepositoryQualityScore struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta                 `json:"metadata"`
	Spec     RepositoryQualityScoreSpec `json:"spec"`
}

type RepositoryQualityScoreSpec struct {
	ProjectID      string                       `json:"projectId"`
	RepositoryID   string                       `json:"repositoryId"`
	WorktreeID     string                       `json:"worktreeId"`
	Branch         string                       `json:"branch"`
	Head           string                       `json:"head"`
	ScoreVersion   string                       `json:"scoreVersion"`
	Overall        int                          `json:"overall"`
	Confidence     int                          `json:"confidence"`
	Status         string                       `json:"status"`
	Components     []RepositoryQualityComponent `json:"components"`
	Checks         []InspectionCheck            `json:"checks"`
	CheckSet       []string                     `json:"checkSet"`
	BaselineDigest string                       `json:"baselineDigest"`
	ConfigDigest   string                       `json:"configDigest"`
	EvidenceDigest string                       `json:"evidenceDigest"`
	ToolDigest     string                       `json:"toolDigest"`
	BasisDigest    string                       `json:"basisDigest"`
	Revision       int                          `json:"revision"`
	CreatedAt      time.Time                    `json:"createdAt"`
	UpdatedAt      time.Time                    `json:"updatedAt"`
}

type RepositoryQualityComponent struct {
	ComponentID  string `json:"componentId"`
	CheckID      string `json:"checkId"`
	Value        int    `json:"value"`
	Confidence   int    `json:"confidence"`
	Outcome      string `json:"outcome"`
	FindingCount int    `json:"findingCount"`
}

type QualityComparison struct {
	Status        string `json:"status"`
	BeforeOverall int    `json:"beforeOverall,omitempty"`
	AfterOverall  int    `json:"afterOverall,omitempty"`
	Delta         int    `json:"delta,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

type QualityComparisonInput struct {
	Before *RepositoryQualityScore
	After  *RepositoryQualityScore
}

// QualityImprovementProposal is a reviewable, enum-only change to an
// inspection plan. It never carries commands, paths, packages, versions, or
// model text.
type QualityImprovementProposal struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta                     `json:"metadata"`
	Spec     QualityImprovementProposalSpec `json:"spec"`
}

type QualityImprovementProposalSpec struct {
	ProjectID        string                     `json:"projectId"`
	RepositoryID     string                     `json:"repositoryId"`
	WorktreeID       string                     `json:"worktreeId"`
	PlanID           string                     `json:"planId"`
	BaseScoreID      string                     `json:"baseScoreId"`
	BaseScoreDigest  string                     `json:"baseScoreDigest"`
	BasePlanRevision int                        `json:"basePlanRevision"`
	BasePlanDigest   string                     `json:"basePlanDigest"`
	Head             string                     `json:"head"`
	ConfigDigest     string                     `json:"configDigest"`
	EvidenceDigest   string                     `json:"evidenceDigest"`
	ToolDigest       string                     `json:"toolDigest"`
	Changes          []QualityImprovementChange `json:"changes"`
	RationaleDigest  string                     `json:"rationaleDigest"`
	RationaleCode    string                     `json:"rationaleCode"`
	Source           string                     `json:"source"`
	Provider         string                     `json:"provider,omitempty"`
	Model            string                     `json:"model,omitempty"`
	State            string                     `json:"state"`
	Revision         int                        `json:"revision"`
	CreatedAt        time.Time                  `json:"createdAt"`
	UpdatedAt        time.Time                  `json:"updatedAt"`
	ReviewedAt       *time.Time                 `json:"reviewedAt,omitempty"`
	AppliedAt        *time.Time                 `json:"appliedAt,omitempty"`
}

type QualityImprovementChange struct {
	Action      string `json:"action"`
	ComponentID string `json:"componentId"`
	CheckID     string `json:"checkId"`
	Reason      string `json:"reason"`
}

func (p InspectionPlan) Validate() error {
	if err := assuranceResource(p.TypeMeta, InspectionPlanKind, p.Metadata); err != nil {
		return err
	}
	if !validInspectionText(p.Spec.ProjectID, 64) || !validInspectionText(p.Spec.RepositoryID, 64) || !validInspectionText(p.Spec.WorktreeID, 64) || !validInspectionText(p.Spec.Branch, 256) || !validInspectionText(p.Spec.Head, 256) {
		return errors.New("inspection plan scope is incomplete")
	}
	for _, digest := range []string{p.Spec.BaselineDigest, p.Spec.ConfigDigest, p.Spec.EvidenceDigest, p.Spec.ToolDigest} {
		if !planDigestPattern.MatchString(digest) {
			return errors.New("inspection plan digests are invalid")
		}
	}
	if p.Spec.Version != InspectionPlanVersion || p.Spec.Revision < 1 || !validInspectionPlanState(p.Spec.State) || len(p.Spec.Components) == 0 || len(p.Spec.Checks) == 0 {
		return errors.New("inspection plan version, state, revision, and checks are required")
	}
	if err := validateInspectionGeneration(p.Spec); err != nil {
		return err
	}
	components := make(map[string]struct{}, len(p.Spec.Components))
	for _, component := range p.Spec.Components {
		if !validOpaqueID(component.ID) {
			return errors.New("inspection component id is invalid")
		}
		if _, ok := components[component.ID]; ok {
			return errors.New("inspection component ids must be unique")
		}
		components[component.ID] = struct{}{}
		if (component.AdapterID != "" && !validStableID(component.AdapterID)) || (component.ParserID != "" && !validStableID(component.ParserID)) {
			return errors.New("inspection component adapter or parser is invalid")
		}
	}
	seenChecks := make(map[string]struct{}, len(p.Spec.Checks))
	for _, check := range p.Spec.Checks {
		if !validInspectionCheckID(check.ID) || !validOpaqueID(check.ComponentID) || !validStableID(check.AdapterID) || !validStableID(check.ParserID) {
			return errors.New("inspection check is invalid")
		}
		if _, ok := components[check.ComponentID]; !ok {
			return errors.New("inspection check references an unknown component")
		}
		key := check.ComponentID + "\x00" + check.ID
		if _, ok := seenChecks[key]; ok {
			return errors.New("inspection checks must be unique")
		}
		seenChecks[key] = struct{}{}
	}
	if p.Spec.Digest != "" {
		digest, err := p.Digest()
		if err != nil || digest != p.Spec.Digest {
			return errors.New("inspection plan digest mismatch")
		}
	}
	return nil
}

func (p InspectionPlan) Digest() (string, error) {
	copyPlan := p
	copyPlan.Spec.Digest = ""
	copyPlan.Spec.Components = append([]InspectionComponent{}, p.Spec.Components...)
	copyPlan.Spec.Checks = append([]InspectionCheck{}, p.Spec.Checks...)
	sort.Slice(copyPlan.Spec.Components, func(i, j int) bool { return copyPlan.Spec.Components[i].ID < copyPlan.Spec.Components[j].ID })
	sort.Slice(copyPlan.Spec.Checks, func(i, j int) bool {
		if copyPlan.Spec.Checks[i].ComponentID != copyPlan.Spec.Checks[j].ComponentID {
			return copyPlan.Spec.Checks[i].ComponentID < copyPlan.Spec.Checks[j].ComponentID
		}
		return copyPlan.Spec.Checks[i].ID < copyPlan.Spec.Checks[j].ID
	})
	return assuranceDigest(copyPlan.Spec)
}

func (p InspectionPlan) StaleReasons(current InspectionFreshness) []string {
	reasons := []string{}
	if current.Head != p.Spec.Head {
		reasons = append(reasons, "head")
	}
	if current.ConfigDigest != p.Spec.ConfigDigest {
		reasons = append(reasons, "config")
	}
	if current.ToolDigest != p.Spec.ToolDigest {
		reasons = append(reasons, "tool")
	}
	if current.EvidenceDigest != p.Spec.EvidenceDigest {
		reasons = append(reasons, "evidence")
	}
	return reasons
}

func (p InspectionPlan) IsStale(current InspectionFreshness) bool {
	return len(p.StaleReasons(current)) != 0
}

func (s RepositoryQualityScore) Validate() error {
	if err := assuranceResource(s.TypeMeta, RepositoryQualityScoreKind, s.Metadata); err != nil {
		return err
	}
	if !validInspectionText(s.Spec.ProjectID, 64) || !validInspectionText(s.Spec.RepositoryID, 64) || !validInspectionText(s.Spec.WorktreeID, 64) || !validInspectionText(s.Spec.Branch, 256) || !validInspectionText(s.Spec.Head, 256) || s.Spec.ScoreVersion != RepositoryQualityScoreVersion || s.Spec.Revision < 1 {
		return errors.New("repository quality score scope or version is incomplete")
	}
	if s.Spec.Overall < 0 || s.Spec.Overall > 100 || s.Spec.Confidence < 0 || s.Spec.Confidence > 100 || !validQualityScoreStatus(s.Spec.Status) || len(s.Spec.Components) == 0 || len(s.Spec.Checks) == 0 || len(s.Spec.CheckSet) == 0 || !planDigestPattern.MatchString(s.Spec.BasisDigest) {
		return errors.New("repository quality score values are invalid")
	}
	for _, digest := range []string{s.Spec.BaselineDigest, s.Spec.ConfigDigest, s.Spec.EvidenceDigest, s.Spec.ToolDigest} {
		if !planDigestPattern.MatchString(digest) {
			return errors.New("repository quality score evidence is invalid")
		}
	}
	for _, check := range s.Spec.Checks {
		if !validInspectionCheckID(check.ID) || !validOpaqueID(check.ComponentID) || !validStableID(check.AdapterID) || !validStableID(check.ParserID) {
			return errors.New("repository quality score check is invalid")
		}
	}
	for _, component := range s.Spec.Components {
		if !validOpaqueID(component.ComponentID) || !validInspectionCheckID(component.CheckID) || component.Value < 0 || component.Value > 100 || component.Confidence < 0 || component.Confidence > 100 || !validInspectionOutcome(component.Outcome) || component.FindingCount < 0 {
			return errors.New("repository quality score component is invalid")
		}
	}
	return nil
}

func (s RepositoryQualityScore) StaleReasons(current InspectionFreshness) []string {
	reasons := []string{}
	if current.Head != s.Spec.Head {
		reasons = append(reasons, "head")
	}
	if current.ConfigDigest != s.Spec.ConfigDigest {
		reasons = append(reasons, "config")
	}
	if current.ToolDigest != s.Spec.ToolDigest {
		reasons = append(reasons, "tool")
	}
	if current.EvidenceDigest != s.Spec.EvidenceDigest {
		reasons = append(reasons, "evidence")
	}
	return reasons
}

func (s RepositoryQualityScore) IsStale(current InspectionFreshness) bool {
	return len(s.StaleReasons(current)) != 0
}

func (p QualityImprovementProposal) Validate() error {
	if err := assuranceResource(p.TypeMeta, QualityImprovementProposalKind, p.Metadata); err != nil {
		return err
	}
	if err := validateAssuranceScope(p.Spec.ProjectID, p.Spec.RepositoryID, p.Spec.WorktreeID, p.Spec.Head); err != nil {
		return err
	}
	if !validIdentifier(p.Spec.PlanID) || !validIdentifier(p.Spec.BaseScoreID) || p.Spec.BasePlanRevision < 1 ||
		!planDigestPattern.MatchString(p.Spec.BasePlanDigest) || !planDigestPattern.MatchString(p.Spec.BaseScoreDigest) ||
		!planDigestPattern.MatchString(p.Spec.ConfigDigest) || !planDigestPattern.MatchString(p.Spec.EvidenceDigest) ||
		!planDigestPattern.MatchString(p.Spec.ToolDigest) || !planDigestPattern.MatchString(p.Spec.RationaleDigest) ||
		len(p.Spec.Changes) == 0 || len(p.Spec.Changes) > 20 || p.Spec.Revision < 1 ||
		p.Spec.CreatedAt.IsZero() || p.Spec.UpdatedAt.IsZero() || p.Spec.UpdatedAt.Before(p.Spec.CreatedAt) {
		return errors.New("quality improvement proposal fields are invalid")
	}
	if !validQualityImprovementState(p.Spec.State) || !validQualityImprovementSource(p.Spec.Source) || !validQualityImprovementReason(p.Spec.RationaleCode) {
		return errors.New("quality improvement proposal state or source is invalid")
	}
	if p.Spec.Source == InspectionGenerationAI {
		if !validBoundedText(p.Spec.Provider) || (p.Spec.Model != "" && !validBoundedText(p.Spec.Model)) {
			return errors.New("quality improvement AI provider metadata is invalid")
		}
	} else if p.Spec.Provider != "" || p.Spec.Model != "" {
		return errors.New("deterministic quality improvement cannot contain AI metadata")
	}
	if p.Spec.ReviewedAt != nil && p.Spec.ReviewedAt.Before(p.Spec.CreatedAt) {
		return errors.New("quality improvement review cannot precede creation")
	}
	if p.Spec.AppliedAt != nil && (p.Spec.ReviewedAt == nil || p.Spec.AppliedAt.Before(*p.Spec.ReviewedAt)) {
		return errors.New("quality improvement apply time is invalid")
	}
	if (p.Spec.State == QualityImprovementStateReviewed || p.Spec.State == QualityImprovementStateApproved || p.Spec.State == QualityImprovementStateRejected || p.Spec.State == QualityImprovementStateApplied) && p.Spec.ReviewedAt == nil {
		return errors.New("reviewed quality improvement requires a review time")
	}
	if p.Spec.State == QualityImprovementStateApplied && p.Spec.AppliedAt == nil {
		return errors.New("applied quality improvement requires an apply time")
	}
	seen := make(map[string]struct{}, len(p.Spec.Changes))
	for _, change := range p.Spec.Changes {
		if !validQualityImprovementAction(change.Action) || !validOpaqueID(change.ComponentID) || !validInspectionCheckID(change.CheckID) || !validQualityImprovementReason(change.Reason) {
			return errors.New("quality improvement change is invalid")
		}
		key := change.Action + "\x00" + change.ComponentID + "\x00" + change.CheckID
		if _, exists := seen[key]; exists {
			return errors.New("quality improvement changes must be unique")
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validInspectionPlanState(value string) bool {
	return value == InspectionPlanStateProposed || value == InspectionPlanStateReviewed || value == InspectionPlanStateApproved || value == InspectionPlanStateStale || value == InspectionPlanStateRejected
}

func validateInspectionGeneration(spec InspectionPlanSpec) error {
	if spec.GenerationSource == "" {
		if spec.GenerationProvider != "" || spec.GenerationModel != "" || spec.AIProposalDigest != "" || len(spec.AIProposalDecisions) > 0 {
			return errors.New("inspection generation metadata is incomplete")
		}
		return nil
	}
	if spec.GenerationSource != InspectionGenerationDeterministic && spec.GenerationSource != InspectionGenerationAI {
		return errors.New("inspection generation source is invalid")
	}
	if spec.GenerationSource == InspectionGenerationDeterministic {
		if spec.GenerationProvider != "" || spec.GenerationModel != "" || spec.AIProposalDigest != "" || len(spec.AIProposalDecisions) > 0 {
			return errors.New("deterministic inspection generation cannot contain AI metadata")
		}
		return nil
	}
	if !validBoundedText(spec.GenerationProvider) || (spec.GenerationModel != "" && !validBoundedText(spec.GenerationModel)) || !planDigestPattern.MatchString(spec.AIProposalDigest) || len(spec.AIProposalDecisions) > 20 {
		return errors.New("AI inspection generation metadata is invalid")
	}
	seen := make(map[string]struct{}, len(spec.AIProposalDecisions))
	for _, decision := range spec.AIProposalDecisions {
		kind, checkID, ok := strings.Cut(decision, ":")
		if !ok || (kind != "include" && kind != "exclude") || !validInspectionCheckID(checkID) {
			return errors.New("AI inspection generation decision is invalid")
		}
		if _, exists := seen[decision]; exists {
			return errors.New("AI inspection generation decisions must be unique")
		}
		seen[decision] = struct{}{}
	}
	return nil
}

func validQualityImprovementState(value string) bool {
	return value == QualityImprovementStateProposed || value == QualityImprovementStateReviewed || value == QualityImprovementStateApproved || value == QualityImprovementStateRejected || value == QualityImprovementStateApplied || value == QualityImprovementStateStale
}

func validQualityImprovementSource(value string) bool {
	return value == InspectionGenerationDeterministic || value == InspectionGenerationAI
}

func validQualityImprovementAction(value string) bool {
	return value == QualityImprovementActionAddCheck || value == QualityImprovementActionRemoveCheck || value == QualityImprovementActionMarkRunnerUnavailable
}

func validQualityImprovementReason(value string) bool {
	return value == QualityImprovementReasonApplicableCheck || value == QualityImprovementReasonFindingObserved || value == QualityImprovementReasonInconclusive || value == QualityImprovementReasonRunnerUnavailable
}
func validQualityScoreStatus(value string) bool {
	return value == QualityScoreStatusFresh || value == QualityScoreStatusStale || value == QualityScoreStatusInconclusive
}
func validInspectionOutcome(value string) bool {
	return value == InspectionOutcomeClean || value == InspectionOutcomeFindings || value == InspectionOutcomeTestsFailed || value == InspectionOutcomeToolError || value == InspectionOutcomeRunnerUnavailable || value == InspectionOutcomeInconclusive
}
func validInspectionCheckID(value string) bool { _, ok := inspectionCheckIDs[value]; return ok }

// IsKnownInspectionCheckID is the public boundary for callers that accept
// bounded check selections. The registry remains private so callers cannot
// provide executable definitions or runner arguments.
func IsKnownInspectionCheckID(value string) bool { return validInspectionCheckID(value) }

func validOpaqueID(value string) bool {
	return len(value) > 0 && len(value) <= 128 && value == strings.TrimSpace(value) && !strings.ContainsAny(value, "\x00\r\n")
}
func validStableID(value string) bool {
	return len(value) > 0 && len(value) <= 128 && value == strings.TrimSpace(value) && !strings.ContainsAny(value, "\x00\r\n")
}
func validInspectionText(value string, max int) bool {
	return len(value) > 0 && len(value) <= max && value == strings.TrimSpace(value) && !strings.ContainsAny(value, "\x00\r\n")
}

func (c QualityComparison) IsInconclusive() bool { return c.Status == QualityComparisonInconclusive }
