// Package domain contains the versioned data contracts used by every
// Dev Control Room surface. It intentionally contains no I/O or execution
// behavior.
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	APIVersion = "devroom/v1alpha1"

	ProjectKind            = "Project"
	RepositoryKind         = "Repository"
	ObservationKind        = "Observation"
	FindingKind            = "Finding"
	ChecksetKind           = "Checkset"
	ActionKind             = "Action"
	ActionPlanKind         = "ActionPlan"
	ApprovalKind           = "Approval"
	AgentProfileKind       = "AgentProfile"
	EventKind              = "Event"
	ScanRunKind            = "ScanRun"
	FailureFingerprintKind = "FailureFingerprint"
)

var (
	identifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	planDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type TypeMeta struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
}

type ObjectMeta struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Project struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta  `json:"metadata"`
	Spec     ProjectSpec `json:"spec"`
}

type ProjectSpec struct {
	Repositories  []Repository  `json:"repositories"`
	Capabilities  Capabilities  `json:"capabilities,omitempty"`
	AgentDefaults AgentDefaults `json:"agentDefaults,omitempty"`
	Policy        ProjectPolicy `json:"policy,omitempty"`
}

type Capabilities struct {
	Jenkins            bool `json:"jenkins,omitempty"`
	Release            bool `json:"release,omitempty"`
	KubernetesReadOnly bool `json:"kubernetesReadOnly,omitempty"`
	GitHub             bool `json:"github,omitempty"`
}

type AgentDefaults struct {
	Profile string `json:"profile,omitempty"`
}

type ProjectPolicy struct {
	AllowSafeLocalAutomation bool `json:"allowSafeLocalAutomation,omitempty"`
}

type Repository struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta     `json:"metadata"`
	Spec     RepositorySpec `json:"spec"`
}

type RepositorySpec struct {
	Path      string              `json:"path"`
	Checksets map[string][]string `json:"checksets,omitempty"`
}

type Observation struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta      `json:"metadata"`
	Spec     ObservationSpec `json:"spec"`
}

type ObservationSpec struct {
	ProjectID       string         `json:"projectId"`
	RepositoryID    string         `json:"repositoryId,omitempty"`
	Collector       string         `json:"collector"`
	ObservationType string         `json:"type"`
	Fingerprint     string         `json:"fingerprint"`
	CollectedAt     time.Time      `json:"collectedAt"`
	Evidence        map[string]any `json:"evidence,omitempty"`
}

func (o Observation) Validate() error {
	if err := validateResource(o.TypeMeta, ObservationKind, o.Metadata); err != nil {
		return err
	}
	if err := validateProjectRepository(o.Spec.ProjectID, o.Spec.RepositoryID); err != nil {
		return err
	}
	if o.Spec.Collector == "" || o.Spec.ObservationType == "" || o.Spec.Fingerprint == "" || o.Spec.CollectedAt.IsZero() {
		return errors.New("observation project, collector, type, fingerprint, and collection time are required")
	}
	return nil
}

type Finding struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta  `json:"metadata"`
	Spec     FindingSpec `json:"spec"`
}

type FindingSpec struct {
	ProjectID       string       `json:"projectId"`
	RepositoryID    string       `json:"repositoryId,omitempty"`
	FindingType     string       `json:"type"`
	Fingerprint     string       `json:"fingerprint"`
	Severity        Severity     `json:"severity"`
	Confidence      Confidence   `json:"confidence"`
	Summary         string       `json:"summary"`
	RecommendedNext string       `json:"recommendedNextAction"`
	EvidenceRefs    []string     `json:"evidenceRefs,omitempty"`
	FirstObserved   time.Time    `json:"firstObserved"`
	LastObserved    time.Time    `json:"lastObserved"`
	State           FindingState `json:"state"`
}

type Severity string

const (
	SeverityInfo      Severity = "info"
	SeverityAttention Severity = "attention"
	SeverityHigh      Severity = "high"
	SeverityCritical  Severity = "critical"
)

type Confidence string

const (
	ConfidenceConfirmed Confidence = "confirmed"
	ConfidenceLikely    Confidence = "likely"
	ConfidenceUncertain Confidence = "uncertain"
)

type FindingState string

const (
	FindingOpen         FindingState = "open"
	FindingAcknowledged FindingState = "acknowledged"
	FindingResolved     FindingState = "resolved"
	FindingSuppressed   FindingState = "suppressed"
	FindingExpired      FindingState = "expired"
)

type Checkset struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta   `json:"metadata"`
	Spec     ChecksetSpec `json:"spec"`
}

type ChecksetSpec struct {
	ProjectID    string      `json:"projectId"`
	RepositoryID string      `json:"repositoryId,omitempty"`
	Name         string      `json:"name"`
	Steps        []CheckStep `json:"steps"`
}

type CheckStep struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	DependsOn   []string `json:"dependsOn,omitempty"`
}

func (c Checkset) Validate() error {
	if err := validateResource(c.TypeMeta, ChecksetKind, c.Metadata); err != nil {
		return err
	}
	if err := validateProjectRepository(c.Spec.ProjectID, c.Spec.RepositoryID); err != nil {
		return err
	}
	if strings.TrimSpace(c.Spec.Name) == "" || len(c.Spec.Steps) == 0 {
		return errors.New("checkset name and at least one step are required")
	}
	seen := make(map[string]struct{}, len(c.Spec.Steps))
	for _, step := range c.Spec.Steps {
		if !validIdentifier(step.ID) || strings.TrimSpace(step.Name) == "" {
			return errors.New("checkset steps require valid unique ids and names")
		}
		if _, exists := seen[step.ID]; exists {
			return fmt.Errorf("duplicate checkset step id %q", step.ID)
		}
		seen[step.ID] = struct{}{}
	}
	return nil
}

type Action struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta `json:"metadata"`
	Spec     ActionSpec `json:"spec"`
}

type ActionSpec struct {
	ActionType  string     `json:"type"`
	Risk        ActionRisk `json:"risk"`
	Description string     `json:"description"`
}

type ActionRisk string

const (
	RiskReadOnly       ActionRisk = "read_only"
	RiskSafeLocal      ActionRisk = "safe_local"
	RiskExternalChange ActionRisk = "external_change"
	RiskHighImpact     ActionRisk = "high_impact"
)

const (
	PolicyAllowed          = "allowed"
	PolicyDenied           = "denied"
	PolicyApprovalRequired = "approval_required"
)

type ActionPlan struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta     `json:"metadata"`
	Spec     ActionPlanSpec `json:"spec"`
}

type ActionPlanSpec struct {
	ProjectID        string            `json:"projectId"`
	RepositoryID     string            `json:"repositoryId,omitempty"`
	ActionType       string            `json:"actionType"`
	Risk             ActionRisk        `json:"risk"`
	Inputs           map[string]string `json:"inputs,omitempty"`
	Preconditions    []string          `json:"preconditions,omitempty"`
	Postconditions   []string          `json:"postconditions,omitempty"`
	PolicyDecision   string            `json:"policyDecision"`
	ApprovalRequired bool              `json:"approvalRequired"`
}

type Approval struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta   `json:"metadata"`
	Spec     ApprovalSpec `json:"spec"`
}

type ApprovalSpec struct {
	ActionPlanID     string        `json:"actionPlanId"`
	ActionPlanDigest string        `json:"actionPlanDigest"`
	Status           ApprovalState `json:"status"`
	RequestedBy      Actor         `json:"requestedBy"`
	ApprovedBy       *Actor        `json:"approvedBy,omitempty"`
	Reason           string        `json:"reason,omitempty"`
	ExpiresAt        *time.Time    `json:"expiresAt,omitempty"`
}

type ApprovalState string

const (
	ApprovalPending  ApprovalState = "pending"
	ApprovalGranted  ApprovalState = "granted"
	ApprovalRejected ApprovalState = "rejected"
	ApprovalExpired  ApprovalState = "expired"
)

type ActorKind string

const (
	ActorHuman  ActorKind = "human"
	ActorAgent  ActorKind = "agent"
	ActorSystem ActorKind = "system"
)

// Actor identifies the authority behind a request or decision. Actor kind,
// rather than string identity equality, determines whether approval is legal:
// the single human user may approve their own request, while agents and the
// scheduler can never approve a protected action.
type Actor struct {
	Kind ActorKind `json:"kind"`
	ID   string    `json:"id"`
}

type AgentProfile struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta       `json:"metadata"`
	Spec     AgentProfileSpec `json:"spec"`
}

type AgentProfileSpec struct {
	Command               string            `json:"command"`
	ModelArgumentTemplate string            `json:"modelArgumentTemplate,omitempty"`
	DataBoundary          AgentDataBoundary `json:"dataBoundary"`
	EnvironmentAllowlist  []string          `json:"environmentAllowlist,omitempty"`
	LaunchMode            AgentLaunchMode   `json:"launchMode"`
}

type AgentDataBoundary string

const (
	AgentBoundaryEnterprise AgentDataBoundary = "enterprise"
	AgentBoundaryLocal      AgentDataBoundary = "local"
)

type AgentLaunchMode string

const (
	AgentLaunchDirect            AgentLaunchMode = "direct"
	AgentLaunchPowerShellProfile AgentLaunchMode = "powershell_profile"
)

type Event struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta `json:"metadata"`
	Spec     EventSpec  `json:"spec"`
}

type EventSpec struct {
	EventType    string         `json:"type"`
	Actor        string         `json:"actor,omitempty"`
	ProjectID    string         `json:"projectId,omitempty"`
	RepositoryID string         `json:"repositoryId,omitempty"`
	Summary      string         `json:"summary"`
	Data         map[string]any `json:"data,omitempty"`
	OccurredAt   time.Time      `json:"occurredAt"`
}

// ScanRun records one bounded collection/reconciliation pass. Trigger is
// deliberately descriptive metadata; manual and scheduled runs use the same
// scan implementation and differ only in this field.
type ScanRun struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta  `json:"metadata"`
	Spec     ScanRunSpec `json:"spec"`
}

type ScanRunSpec struct {
	ProjectID       string     `json:"projectId,omitempty"`
	Trigger         string     `json:"trigger"`
	Status          string     `json:"status"`
	StartedAt       time.Time  `json:"startedAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	RepositoryCount int        `json:"repositoryCount"`
	FindingCount    int        `json:"findingCount"`
}

const (
	ScanRunning   = "running"
	ScanSucceeded = "succeeded"
	ScanFailed    = "failed"
)

type FailureFingerprint struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta             `json:"metadata"`
	Spec     FailureFingerprintSpec `json:"spec"`
}

type FailureFingerprintSpec struct {
	Fingerprint     string    `json:"fingerprint"`
	Category        string    `json:"category"`
	FirstSeen       time.Time `json:"firstSeen"`
	LastSeen        time.Time `json:"lastSeen"`
	OccurrenceCount int       `json:"occurrenceCount"`
	EvidenceRefs    []string  `json:"evidenceRefs,omitempty"`
}

const (
	FindingDirty          = "dirty_state"
	FindingUpstreamDrift  = "upstream_drift"
	FindingDetachedHead   = "detached_head"
	FindingMissingRemote  = "missing_remote"
	FindingStaleScan      = "stale_scan"
	FindingUnsafeCleanup  = "unsafe_cleanup"
	FindingCollectorError = "collector_error"
)

func (s ScanRun) Validate() error {
	if err := validateResource(s.TypeMeta, ScanRunKind, s.Metadata); err != nil {
		return err
	}
	if s.Spec.ProjectID != "" && !validIdentifier(s.Spec.ProjectID) {
		return errors.New("scan run has an invalid project identifier")
	}
	if s.Spec.Trigger == "" || (s.Spec.Status != ScanRunning && s.Spec.Status != ScanSucceeded && s.Spec.Status != ScanFailed) || s.Spec.StartedAt.IsZero() {
		return errors.New("scan run trigger, status, and start time are required")
	}
	if s.Spec.CompletedAt != nil && s.Spec.CompletedAt.Before(s.Spec.StartedAt) {
		return errors.New("scan run completion cannot precede start")
	}
	return nil
}

func (f FailureFingerprint) Validate() error {
	if err := validateResource(f.TypeMeta, FailureFingerprintKind, f.Metadata); err != nil {
		return err
	}
	if strings.TrimSpace(f.Spec.Fingerprint) == "" || strings.TrimSpace(f.Spec.Category) == "" || f.Spec.FirstSeen.IsZero() || f.Spec.LastSeen.IsZero() || f.Spec.OccurrenceCount < 1 {
		return errors.New("failure fingerprint fields are required")
	}
	if f.Spec.LastSeen.Before(f.Spec.FirstSeen) {
		return errors.New("failure fingerprint last seen cannot precede first seen")
	}
	return nil
}

func NewProject(id, name string, repositories []Repository) Project {
	return Project{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: ProjectKind},
		Metadata: ObjectMeta{ID: id, Name: name},
		Spec:     ProjectSpec{Repositories: repositories},
	}
}

func NewRepository(id, name, path string) Repository {
	return Repository{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: RepositoryKind},
		Metadata: ObjectMeta{ID: id, Name: name},
		Spec:     RepositorySpec{Path: path},
	}
}

// RepositoryKey is the storage identity of a Repository. Repository IDs are
// unique within a Project, so the project scope is part of the durable key.
func RepositoryKey(projectID, repositoryID string) string {
	return projectID + "/" + repositoryID
}

func (p Project) Validate() error {
	if err := validateResource(p.TypeMeta, ProjectKind, p.Metadata); err != nil {
		return err
	}
	if len(p.Spec.Repositories) == 0 {
		return errors.New("project requires at least one repository")
	}
	seen := make(map[string]struct{}, len(p.Spec.Repositories))
	for _, repository := range p.Spec.Repositories {
		if err := repository.Validate(); err != nil {
			return fmt.Errorf("repository %q: %w", repository.Metadata.ID, err)
		}
		if _, ok := seen[repository.Metadata.ID]; ok {
			return fmt.Errorf("duplicate repository id %q", repository.Metadata.ID)
		}
		seen[repository.Metadata.ID] = struct{}{}
	}
	return nil
}

func (r Repository) Validate() error {
	if err := validateResource(r.TypeMeta, RepositoryKind, r.Metadata); err != nil {
		return err
	}
	if strings.TrimSpace(r.Spec.Path) == "" {
		return errors.New("repository path is required")
	}
	return nil
}

func (f Finding) Validate() error {
	if err := validateResource(f.TypeMeta, FindingKind, f.Metadata); err != nil {
		return err
	}
	if err := validateProjectRepository(f.Spec.ProjectID, f.Spec.RepositoryID); err != nil {
		return err
	}
	if f.Spec.FindingType == "" || f.Spec.Fingerprint == "" {
		return errors.New("finding type and fingerprint are required")
	}
	if strings.TrimSpace(f.Spec.Summary) == "" || strings.TrimSpace(f.Spec.RecommendedNext) == "" {
		return errors.New("finding summary and recommended next action are required")
	}
	if f.Spec.FirstObserved.IsZero() || f.Spec.LastObserved.IsZero() {
		return errors.New("finding first and last observation times are required")
	}
	if f.Spec.LastObserved.Before(f.Spec.FirstObserved) {
		return errors.New("finding last observation cannot precede its first observation")
	}
	if !validSeverity(f.Spec.Severity) || !validConfidence(f.Spec.Confidence) || !validFindingState(f.Spec.State) {
		return errors.New("finding contains an invalid severity, confidence, or lifecycle state")
	}
	return nil
}

func (a ActionPlan) Validate() error {
	if err := validateResource(a.TypeMeta, ActionPlanKind, a.Metadata); err != nil {
		return err
	}
	if !validIdentifier(a.Spec.ProjectID) || a.Spec.ActionType == "" || !validRisk(a.Spec.Risk) {
		return errors.New("action plan project, action type, and risk are required")
	}
	if !validOptionalIdentifier(a.Spec.RepositoryID) {
		return errors.New("action plan has an invalid repository identifier")
	}
	if a.Spec.PolicyDecision != PolicyAllowed && a.Spec.PolicyDecision != PolicyDenied && a.Spec.PolicyDecision != PolicyApprovalRequired {
		return errors.New("action plan has an invalid policy decision")
	}
	if (a.Spec.Risk == RiskExternalChange || a.Spec.Risk == RiskHighImpact) && !a.Spec.ApprovalRequired {
		return errors.New("external-change and high-impact actions always require approval")
	}
	if a.Spec.ApprovalRequired && a.Spec.PolicyDecision == PolicyAllowed {
		return errors.New("an approval-required action cannot have an allowed policy decision")
	}
	if !a.Spec.ApprovalRequired && a.Spec.PolicyDecision == PolicyApprovalRequired {
		return errors.New("approval-required policy decision needs approval_required=true")
	}
	return nil
}

// Digest binds an approval to the complete immutable ActionPlan. Callers must
// create a new plan ID when any plan field changes; execution verifies this
// digest again immediately before running the action.
func (a ActionPlan) Digest() (string, error) {
	if err := a.Validate(); err != nil {
		return "", err
	}
	payload, err := json.Marshal(a)
	if err != nil {
		return "", fmt.Errorf("marshal action plan digest: %w", err)
	}
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func (a Action) Validate() error {
	if err := validateResource(a.TypeMeta, ActionKind, a.Metadata); err != nil {
		return err
	}
	if a.Spec.ActionType == "" || !validRisk(a.Spec.Risk) || strings.TrimSpace(a.Spec.Description) == "" {
		return errors.New("action type, risk, and description are required")
	}
	return nil
}

func (a Approval) Validate() error {
	if err := validateResource(a.TypeMeta, ApprovalKind, a.Metadata); err != nil {
		return err
	}
	if !validIdentifier(a.Spec.ActionPlanID) || !planDigestPattern.MatchString(a.Spec.ActionPlanDigest) || !validActor(a.Spec.RequestedBy) || !validApprovalState(a.Spec.Status) {
		return errors.New("approval action plan, digest, requester, and status are required")
	}
	switch a.Spec.Status {
	case ApprovalPending:
		if a.Spec.ApprovedBy != nil {
			return errors.New("pending approval cannot have an approver")
		}
	case ApprovalGranted:
		if a.Spec.ApprovedBy == nil || !validActor(*a.Spec.ApprovedBy) {
			return errors.New("granted approval requires a valid approver")
		}
		if a.Spec.ApprovedBy.Kind != ActorHuman {
			return errors.New("only a human can grant approval")
		}
	case ApprovalRejected:
		if a.Spec.ApprovedBy == nil || !validActor(*a.Spec.ApprovedBy) || a.Spec.ApprovedBy.Kind != ActorHuman || strings.TrimSpace(a.Spec.Reason) == "" {
			return errors.New("rejected approval requires a human decision maker and reason")
		}
	case ApprovalExpired:
		if a.Spec.ApprovedBy != nil {
			return errors.New("expired approval cannot have an approver")
		}
	}
	return nil
}

func (a Approval) ValidateFor(plan ActionPlan) error {
	return a.ValidateForAt(plan, time.Now().UTC())
}

// ValidateForAt exists so the Action broker can use one captured clock value
// and tests do not depend on wall-clock timing.
func (a Approval) ValidateForAt(plan ActionPlan, now time.Time) error {
	if err := a.Validate(); err != nil {
		return err
	}
	if a.Spec.ActionPlanID != plan.Metadata.ID {
		return errors.New("approval does not belong to the action plan")
	}
	digest, err := plan.Digest()
	if err != nil {
		return fmt.Errorf("action plan: %w", err)
	}
	if a.Spec.ActionPlanDigest != digest {
		return errors.New("approval action-plan digest does not match")
	}
	if !plan.Spec.ApprovalRequired && a.Spec.Status == ApprovalGranted {
		return errors.New("an action without required approval cannot receive a grant")
	}
	if plan.Spec.Risk == RiskHighImpact && a.Spec.Status == ApprovalGranted {
		if a.Spec.ExpiresAt == nil || !a.Spec.ExpiresAt.After(now) {
			return errors.New("high-impact approval must have a future expiry")
		}
	}
	return nil
}

func (a AgentProfile) Validate() error {
	if err := validateResource(a.TypeMeta, AgentProfileKind, a.Metadata); err != nil {
		return err
	}
	if strings.TrimSpace(a.Spec.Command) == "" {
		return errors.New("agent profile command is required")
	}
	if a.Spec.DataBoundary != AgentBoundaryEnterprise && a.Spec.DataBoundary != AgentBoundaryLocal {
		return errors.New("agent profile has an invalid data boundary")
	}
	if a.Spec.LaunchMode != AgentLaunchDirect && a.Spec.LaunchMode != AgentLaunchPowerShellProfile {
		return errors.New("agent profile has an invalid launch mode")
	}
	seen := make(map[string]struct{}, len(a.Spec.EnvironmentAllowlist))
	for _, name := range a.Spec.EnvironmentAllowlist {
		name = strings.TrimSpace(name)
		if name == "" || strings.ContainsAny(name, "=\x00") {
			return errors.New("agent environment allowlist contains an invalid variable name")
		}
		key := strings.ToUpper(name)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate agent environment variable %q", name)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func (e Event) Validate() error {
	if err := validateResource(e.TypeMeta, EventKind, e.Metadata); err != nil {
		return err
	}
	if e.Spec.ProjectID != "" || e.Spec.RepositoryID != "" {
		if err := validateProjectRepository(e.Spec.ProjectID, e.Spec.RepositoryID); err != nil {
			return err
		}
	}
	if e.Spec.EventType == "" || e.Spec.Summary == "" || e.Spec.OccurredAt.IsZero() {
		return errors.New("event type, summary, and occurrence time are required")
	}
	return nil
}

func validateResource(meta TypeMeta, wantKind string, object ObjectMeta) error {
	if meta.APIVersion != APIVersion {
		return fmt.Errorf("unsupported apiVersion %q", meta.APIVersion)
	}
	if meta.Kind != wantKind {
		return fmt.Errorf("expected kind %q, got %q", wantKind, meta.Kind)
	}
	if !validIdentifier(object.ID) || strings.TrimSpace(object.Name) == "" {
		return errors.New("metadata id and name are required")
	}
	return nil
}

func validIdentifier(value string) bool {
	return identifierPattern.MatchString(value)
}

func validOptionalIdentifier(value string) bool {
	return value == "" || validIdentifier(value)
}

func validateProjectRepository(projectID, repositoryID string) error {
	if !validIdentifier(projectID) || !validOptionalIdentifier(repositoryID) {
		return errors.New("project and repository identifiers are invalid")
	}
	return nil
}

func validSeverity(value Severity) bool {
	return value == SeverityInfo || value == SeverityAttention || value == SeverityHigh || value == SeverityCritical
}

func validConfidence(value Confidence) bool {
	return value == ConfidenceConfirmed || value == ConfidenceLikely || value == ConfidenceUncertain
}

func validFindingState(value FindingState) bool {
	return value == FindingOpen || value == FindingAcknowledged || value == FindingResolved || value == FindingSuppressed || value == FindingExpired
}

func validRisk(value ActionRisk) bool {
	return value == RiskReadOnly || value == RiskSafeLocal || value == RiskExternalChange || value == RiskHighImpact
}

func validApprovalState(value ApprovalState) bool {
	return value == ApprovalPending || value == ApprovalGranted || value == ApprovalRejected || value == ApprovalExpired
}

func validActor(actor Actor) bool {
	if strings.TrimSpace(actor.ID) == "" {
		return false
	}
	return actor.Kind == ActorHuman || actor.Kind == ActorAgent || actor.Kind == ActorSystem
}
