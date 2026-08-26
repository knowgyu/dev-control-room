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
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	APIVersion = "devroom/v1alpha1"

	ProjectKind             = "Project"
	RepositoryKind          = "Repository"
	WorktreeKind            = "Worktree"
	WorktreeObservationKind = "WorktreeObservation"
	DiscoveryKind           = "Discovery"
	ProposalKind            = "Proposal"
	ObservationKind         = "Observation"
	FindingKind             = "Finding"
	ChecksetKind            = "Checkset"
	CheckRunKind            = "CheckRun"
	CleanupCandidateKind    = "CleanupCandidate"
	ActionKind              = "Action"
	ActionPlanKind          = "ActionPlan"
	ActionRunKind           = "ActionRun"
	ApprovalKind            = "Approval"
	ActionEventKind         = "ActionEvent"
	AgentProfileKind        = "AgentProfile"
	EventKind               = "Event"
	ScanRunKind             = "ScanRun"
	FailureFingerprintKind  = "FailureFingerprint"
	SafeguardRuleKind       = "SafeguardRule"
)

var (
	identifierPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	planDigestPattern      = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	commandNamePattern     = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	windowsPathPattern     = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
	environmentNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)
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

// Worktree is a concrete, read-only-discovered checkout beneath a Repository.
// Its trust state is observation evidence, never execution authority.
type Worktree struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta   `json:"metadata"`
	Spec     WorktreeSpec `json:"spec"`
}

// CleanupCandidate is a read-only safety assessment. It is never permission
// to delete a branch, worktree, remote, or issue.
type CleanupCandidate struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta           `json:"metadata"`
	Spec     CleanupCandidateSpec `json:"spec"`
}

type CleanupCandidateSpec struct {
	ProjectID     string    `json:"projectId"`
	RepositoryID  string    `json:"repositoryId"`
	WorktreeID    string    `json:"worktreeId"`
	CanonicalPath string    `json:"canonicalPath"`
	Branch        string    `json:"branch,omitempty"`
	Head          string    `json:"head,omitempty"`
	Dirty         bool      `json:"dirty"`
	Untracked     bool      `json:"untracked"`
	Detached      bool      `json:"detached"`
	Locked        bool      `json:"locked"`
	Prunable      bool      `json:"prunable"`
	Ahead         int       `json:"ahead"`
	Behind        int       `json:"behind"`
	Upstream      string    `json:"upstream,omitempty"`
	Merged        bool      `json:"merged,omitempty"`
	MergeEvidence string    `json:"mergeEvidence,omitempty"`
	Decision      string    `json:"decision"`
	Reasons       []string  `json:"reasons"`
	ObservedAt    time.Time `json:"observedAt"`
}

const (
	CleanupBlocked    = "blocked"
	CleanupReviewable = "reviewable"
)

type WorktreeSpec struct {
	ProjectID              string     `json:"projectId"`
	RepositoryID           string     `json:"repositoryId"`
	CanonicalPath          string     `json:"canonicalPath"`
	PathFingerprint        string     `json:"pathFingerprint"`
	AssociationFingerprint string     `json:"associationFingerprint,omitempty"`
	Trust                  string     `json:"trust"`
	Primary                bool       `json:"primary"`
	Head                   string     `json:"head,omitempty"`
	Branch                 string     `json:"branch,omitempty"`
	Dirty                  bool       `json:"dirty"`
	Untracked              bool       `json:"untracked"`
	Upstream               string     `json:"upstream,omitempty"`
	Ahead                  int        `json:"ahead"`
	Behind                 int        `json:"behind"`
	Detached               bool       `json:"detached"`
	Locked                 bool       `json:"locked"`
	Prunable               bool       `json:"prunable"`
	Error                  string     `json:"error,omitempty"`
	LastObserved           time.Time  `json:"lastObserved"`
	TombstonedAt           *time.Time `json:"tombstonedAt,omitempty"`
}

const (
	WorktreeTrustVerifiedReadOnly = "verified_read_only"
	WorktreeTrustUnverified       = "unverified"
)

func (w Worktree) Validate() error {
	if err := validateResource(w.TypeMeta, WorktreeKind, w.Metadata); err != nil {
		return err
	}
	if err := validateProjectRepository(w.Spec.ProjectID, w.Spec.RepositoryID); err != nil {
		return err
	}
	if strings.TrimSpace(w.Spec.CanonicalPath) == "" || (w.Spec.Trust != WorktreeTrustVerifiedReadOnly && w.Spec.Trust != WorktreeTrustUnverified) {
		return errors.New("worktree path and trust are required")
	}
	if w.Spec.Primary != (w.Metadata.ID == "primary") {
		return errors.New("primary worktree identity is inconsistent")
	}
	if !w.Spec.Primary && w.Spec.Trust == WorktreeTrustVerifiedReadOnly && strings.TrimSpace(w.Spec.AssociationFingerprint) == "" {
		return errors.New("linked worktree requires an association fingerprint")
	}
	return nil
}

func (c CleanupCandidate) Validate() error {
	if err := validateResource(c.TypeMeta, CleanupCandidateKind, c.Metadata); err != nil {
		return err
	}
	if err := validateProjectRepository(c.Spec.ProjectID, c.Spec.RepositoryID); err != nil || !validIdentifier(c.Spec.WorktreeID) || strings.TrimSpace(c.Spec.CanonicalPath) == "" || (c.Spec.Decision != CleanupBlocked && c.Spec.Decision != CleanupReviewable) || c.Spec.ObservedAt.IsZero() || len(c.Spec.Reasons) == 0 {
		return errors.New("cleanup candidate requires an exact blocked target, reasons, and observation time")
	}
	return nil
}

type WorktreeObservation struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta              `json:"metadata"`
	Spec     WorktreeObservationSpec `json:"spec"`
}

type WorktreeObservationSpec struct {
	ProjectID    string    `json:"projectId"`
	RepositoryID string    `json:"repositoryId"`
	WorktreeID   string    `json:"worktreeId"`
	CollectedAt  time.Time `json:"collectedAt"`
	Object       Worktree  `json:"object"`
}

// Discovery records one bounded, read-only inspection of a selected Worktree.
// It is a response contract; Proposal is the durable review object.
type Discovery struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta    `json:"metadata"`
	Spec     DiscoverySpec `json:"spec"`
}

type DiscoverySpec struct {
	ProjectID    string    `json:"projectId"`
	RepositoryID string    `json:"repositoryId"`
	WorktreeID   string    `json:"worktreeId"`
	Branch       string    `json:"branch,omitempty"`
	Head         string    `json:"head"`
	DiscoveredAt time.Time `json:"discoveredAt"`
	ProposalIDs  []string  `json:"proposalIds"`
}

func (d Discovery) Validate() error {
	if err := validateResource(d.TypeMeta, DiscoveryKind, d.Metadata); err != nil {
		return err
	}
	if err := validateProjectRepository(d.Spec.ProjectID, d.Spec.RepositoryID); err != nil || !validIdentifier(d.Spec.WorktreeID) || strings.TrimSpace(d.Spec.Head) == "" || d.Spec.DiscoveredAt.IsZero() {
		return errors.New("discovery scope, head, and collection time are required")
	}
	for _, id := range d.Spec.ProposalIDs {
		if !validIdentifier(id) {
			return errors.New("discovery proposal ids must be valid")
		}
	}
	return nil
}

type ProposalState string

const (
	ProposalPending  ProposalState = "pending"
	ProposalApplied  ProposalState = "applied"
	ProposalRejected ProposalState = "rejected"
	ProposalStale    ProposalState = "stale"
)

// Proposal is immutable discovery evidence until its lifecycle state changes.
// Command is descriptive only in Slice C; no discovered command is executed.
type Proposal struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta   `json:"metadata"`
	Spec     ProposalSpec `json:"spec"`
}

type ProposalSpec struct {
	ProjectID    string        `json:"projectId"`
	RepositoryID string        `json:"repositoryId"`
	WorktreeID   string        `json:"worktreeId"`
	Branch       string        `json:"branch,omitempty"`
	Head         string        `json:"head"`
	SourcePath   string        `json:"sourcePath"`
	SourceDigest string        `json:"sourceDigest"`
	CommandKind  string        `json:"commandKind"`
	Command      string        `json:"command"`
	TypedCommand *CheckCommand `json:"typedCommand,omitempty"`
	Inference    string        `json:"inference"`
	State        ProposalState `json:"state"`
	CreatedAt    time.Time     `json:"createdAt"`
	ReviewedAt   *time.Time    `json:"reviewedAt,omitempty"`
}

func (p Proposal) Validate() error {
	if err := validateResource(p.TypeMeta, ProposalKind, p.Metadata); err != nil {
		return err
	}
	if err := validateProjectRepository(p.Spec.ProjectID, p.Spec.RepositoryID); err != nil || !validIdentifier(p.Spec.WorktreeID) || strings.TrimSpace(p.Spec.Head) == "" || !validRelativePath(p.Spec.SourcePath) || !planDigestPattern.MatchString(p.Spec.SourceDigest) || strings.TrimSpace(p.Spec.CommandKind) == "" || strings.TrimSpace(p.Spec.Command) == "" || p.Spec.Inference != "deterministic" || p.Spec.CreatedAt.IsZero() {
		return errors.New("proposal scope, source evidence, deterministic command, and creation time are required")
	}
	if p.Spec.State != ProposalPending && p.Spec.State != ProposalApplied && p.Spec.State != ProposalRejected && p.Spec.State != ProposalStale {
		return errors.New("proposal has an invalid lifecycle state")
	}
	if p.Spec.TypedCommand != nil && !validCheckCommand(*p.Spec.TypedCommand) {
		return errors.New("proposal typed command is invalid")
	}
	if (p.Spec.State == ProposalApplied || p.Spec.State == ProposalRejected) != (p.Spec.ReviewedAt != nil) {
		return errors.New("reviewed proposals require a review time")
	}
	if p.Spec.ReviewedAt != nil && p.Spec.ReviewedAt.Before(p.Spec.CreatedAt) {
		return errors.New("proposal review cannot precede discovery")
	}
	return nil
}

func (o WorktreeObservation) Validate() error {
	if err := validateResource(o.TypeMeta, WorktreeObservationKind, o.Metadata); err != nil {
		return err
	}
	if err := validateProjectRepository(o.Spec.ProjectID, o.Spec.RepositoryID); err != nil || !validIdentifier(o.Spec.WorktreeID) || o.Spec.CollectedAt.IsZero() {
		return errors.New("worktree observation scope and collection time are required")
	}
	if o.Spec.Object.Metadata.ID != o.Spec.WorktreeID || o.Spec.Object.Spec.ProjectID != o.Spec.ProjectID || o.Spec.Object.Spec.RepositoryID != o.Spec.RepositoryID {
		return errors.New("worktree observation scope must match its worktree")
	}
	return o.Spec.Object.Validate()
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
	ProjectID    string        `json:"projectId"`
	RepositoryID string        `json:"repositoryId"`
	WorktreeID   string        `json:"worktreeId"`
	Head         string        `json:"head"`
	ProposalID   string        `json:"proposalId"`
	Name         string        `json:"name"`
	State        ChecksetState `json:"state"`
	Steps        []CheckStep   `json:"steps"`
}

type ChecksetState string

const (
	ChecksetDraft   ChecksetState = "draft"
	ChecksetApplied ChecksetState = "applied"
)

type CheckStep struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	DependsOn   []string     `json:"dependsOn,omitempty"`
	Command     CheckCommand `json:"command"`
}

// CheckCommand is an argv command, never a shell command. WorkingDirectory is
// deliberately fixed to the Checkset's verified Worktree.
type CheckCommand struct {
	Executable     string   `json:"executable"`
	Arguments      []string `json:"arguments"`
	Environment    []string `json:"environment,omitempty"`
	TimeoutSeconds int      `json:"timeoutSeconds"`
}

type CheckRun struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta   `json:"metadata"`
	Spec     CheckRunSpec `json:"spec"`
}

type CheckRunSpec struct {
	ChecksetID   string         `json:"checksetId"`
	ProjectID    string         `json:"projectId"`
	RepositoryID string         `json:"repositoryId"`
	WorktreeID   string         `json:"worktreeId"`
	Head         string         `json:"head"`
	StartedAt    time.Time      `json:"startedAt"`
	CompletedAt  time.Time      `json:"completedAt"`
	Status       CheckRunStatus `json:"status"`
	Steps        []CheckStepRun `json:"steps"`
}

type CheckStepRun struct {
	StepID   string         `json:"stepId"`
	Status   CheckRunStatus `json:"status"`
	ExitCode int            `json:"exitCode,omitempty"`
	Stdout   string         `json:"stdout,omitempty"`
	Stderr   string         `json:"stderr,omitempty"`
}

type CheckRunStatus string

const (
	CheckPassed      CheckRunStatus = "passed"
	CheckFailed      CheckRunStatus = "failed"
	CheckSkipped     CheckRunStatus = "skipped"
	CheckUnavailable CheckRunStatus = "unavailable"
	CheckCancelled   CheckRunStatus = "cancelled"
	CheckTimedOut    CheckRunStatus = "timed_out"
)

func (c Checkset) Validate() error {
	if err := validateResource(c.TypeMeta, ChecksetKind, c.Metadata); err != nil {
		return err
	}
	if err := validateProjectRepository(c.Spec.ProjectID, c.Spec.RepositoryID); err != nil {
		return err
	}
	if !validIdentifier(c.Spec.WorktreeID) || strings.TrimSpace(c.Spec.Head) == "" || !validIdentifier(c.Spec.ProposalID) {
		return errors.New("checkset worktree, head, and proposal are required")
	}
	if strings.TrimSpace(c.Spec.Name) == "" || len(c.Spec.Steps) == 0 || (c.Spec.State != ChecksetDraft && c.Spec.State != ChecksetApplied) {
		return errors.New("checkset name and at least one step are required")
	}
	seen := make(map[string]struct{}, len(c.Spec.Steps))
	for _, step := range c.Spec.Steps {
		if !validIdentifier(step.ID) || strings.TrimSpace(step.Name) == "" || !validCheckCommand(step.Command) {
			return errors.New("checkset steps require valid unique ids and names")
		}
		if _, exists := seen[step.ID]; exists {
			return fmt.Errorf("duplicate checkset step id %q", step.ID)
		}
		seen[step.ID] = struct{}{}
	}
	for _, step := range c.Spec.Steps {
		for _, dependency := range step.DependsOn {
			if dependency == step.ID || !validIdentifier(dependency) {
				return errors.New("checkset step dependencies must name another valid step")
			}
			if _, ok := seen[dependency]; !ok {
				return errors.New("checkset step dependency is missing")
			}
		}
	}
	visiting, visited := map[string]bool{}, map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if visiting[id] {
			return false
		}
		if visited[id] {
			return true
		}
		visiting[id] = true
		for _, step := range c.Spec.Steps {
			if step.ID == id {
				for _, dependency := range step.DependsOn {
					if !visit(dependency) {
						return false
					}
				}
			}
		}
		visiting[id], visited[id] = false, true
		return true
	}
	for _, step := range c.Spec.Steps {
		if !visit(step.ID) {
			return errors.New("checkset step dependencies cannot cycle")
		}
	}
	return nil
}

func validCheckCommand(command CheckCommand) bool {
	if command.Executable != "git" || command.TimeoutSeconds < 1 || command.TimeoutSeconds > 300 {
		return false
	}
	for _, argument := range command.Arguments {
		if strings.ContainsRune(argument, '\x00') {
			return false
		}
	}
	if len(command.Arguments) == 0 {
		return false
	}
	for _, name := range command.Environment {
		if !environmentNamePattern.MatchString(name) {
			return false
		}
	}
	switch command.Arguments[0] {
	case "status":
		return len(command.Arguments) == 2 && command.Arguments[1] == "--porcelain"
	case "diff":
		return len(command.Arguments) == 2 && (command.Arguments[1] == "--check" || command.Arguments[1] == "--exit-code")
	case "config":
		return len(command.Arguments) == 3 && command.Arguments[1] == "--get" && commandNamePattern.MatchString(command.Arguments[2])
	default:
		return false
	}
}

func (r CheckRun) Validate() error {
	if err := validateResource(r.TypeMeta, CheckRunKind, r.Metadata); err != nil {
		return err
	}
	if err := validateProjectRepository(r.Spec.ProjectID, r.Spec.RepositoryID); err != nil || !validIdentifier(r.Spec.ChecksetID) || !validIdentifier(r.Spec.WorktreeID) || strings.TrimSpace(r.Spec.Head) == "" || r.Spec.StartedAt.IsZero() || r.Spec.CompletedAt.IsZero() || r.Spec.CompletedAt.Before(r.Spec.StartedAt) || !validCheckRunStatus(r.Spec.Status) {
		return errors.New("check run scope, timing, and status are required")
	}
	for _, step := range r.Spec.Steps {
		if !validIdentifier(step.StepID) || !validCheckRunStatus(step.Status) {
			return errors.New("check run steps require valid ids and statuses")
		}
	}
	return nil
}

func validCheckRunStatus(status CheckRunStatus) bool {
	return status == CheckPassed || status == CheckFailed || status == CheckSkipped || status == CheckUnavailable || status == CheckCancelled || status == CheckTimedOut
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
	ProjectID            string                   `json:"projectId"`
	RepositoryID         string                   `json:"repositoryId"`
	WorktreeID           string                   `json:"worktreeId"`
	ActionType           string                   `json:"actionType"`
	Risk                 ActionRisk               `json:"risk"`
	Inputs               map[string]string        `json:"inputs,omitempty"`
	Execution            ActionExecution          `json:"execution"`
	ExecutionContext     WorktreeExecutionContext `json:"executionContext"`
	Prechecks            []ActionEvidenceContract `json:"prechecks"`
	Postchecks           []ActionEvidenceContract `json:"postchecks"`
	PolicyDecision       string                   `json:"policyDecision"`
	ApprovalRequired     bool                     `json:"approvalRequired"`
	RequestedBy          Actor                    `json:"requestedBy"`
	RequestedAt          time.Time                `json:"requestedAt"`
	ApprovalScopeID      string                   `json:"approvalScopeId,omitempty"`
	ApprovalScopeDigest  string                   `json:"approvalScopeDigest,omitempty"`
	ProviderProfile      string                   `json:"providerProfile,omitempty"`
	Techniques           []string                 `json:"techniques,omitempty"`
	ToolSetup            []string                 `json:"toolSetup,omitempty"`
	ToolVersion          string                   `json:"toolVersion,omitempty"`
	ToolConfigDigest     string                   `json:"toolConfigDigest,omitempty"`
	ArgumentSchemaDigest string                   `json:"argumentSchemaDigest,omitempty"`
	WritablePaths        []string                 `json:"writablePaths,omitempty"`
	NetworkPolicy        string                   `json:"networkPolicy,omitempty"`
	DiskLimitBytes       int64                    `json:"diskLimitBytes,omitempty"`
	ScopeDeadline        time.Time                `json:"scopeDeadline,omitempty"`
	ProhibitedOperations []string                 `json:"prohibitedOperations,omitempty"`
	ScopeMatch           bool                     `json:"scopeMatch,omitempty"`
	ScopeMatchReasons    []string                 `json:"scopeMatchReasons,omitempty"`
	ScopeCheckedAt       time.Time                `json:"scopeCheckedAt,omitempty"`
}

// ActionExecution is the complete process contract copied from a reviewed
// definition. It is descriptive only until a future runner consumes it.
type ActionExecution struct {
	Executable           string   `json:"executable"`
	Arguments            []string `json:"arguments"`
	EnvironmentAllowlist []string `json:"environmentAllowlist,omitempty"`
	TimeoutSeconds       int      `json:"timeoutSeconds"`
	MaxOutputBytes       int      `json:"maxOutputBytes"`
}

// WorktreeExecutionContext freezes the exact observed checkout a future
// runner must revalidate; a repository-level identity is never sufficient.
type WorktreeExecutionContext struct {
	ProjectID              string `json:"projectId"`
	RepositoryID           string `json:"repositoryId"`
	WorktreeID             string `json:"worktreeId"`
	CanonicalPath          string `json:"canonicalPath"`
	PathFingerprint        string `json:"pathFingerprint"`
	AssociationFingerprint string `json:"associationFingerprint,omitempty"`
	Head                   string `json:"head"`
	Branch                 string `json:"branch,omitempty"`
}

type ActionEvidenceKind string

const (
	EvidenceWorktreeIdentity ActionEvidenceKind = "worktree_identity"
	EvidenceWorktreeHead     ActionEvidenceKind = "worktree_head"
	EvidenceProcessExit      ActionEvidenceKind = "process_exit"
)

// ActionEvidenceContract declares evidence a future runner must capture for
// a precheck or postcheck. It deliberately has no free-form command field.
type ActionEvidenceContract struct {
	ID       string             `json:"id"`
	Kind     ActionEvidenceKind `json:"kind"`
	Required bool               `json:"required"`
}

// ActionEvidence is the bounded, masked result of one reviewed evidence
// contract. Detail contains metadata only; command output lives on the run.
type ActionEvidence struct {
	ID     string             `json:"id"`
	Kind   ActionEvidenceKind `json:"kind"`
	Passed bool               `json:"passed"`
	Detail string             `json:"detail,omitempty"`
}

type ActionRun struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta    `json:"metadata"`
	Spec     ActionRunSpec `json:"spec"`
}

type ActionRunSpec struct {
	ActionPlanID     string                   `json:"actionPlanId"`
	ActionPlanDigest string                   `json:"actionPlanDigest"`
	ProjectID        string                   `json:"projectId"`
	RepositoryID     string                   `json:"repositoryId"`
	WorktreeID       string                   `json:"worktreeId"`
	Holder           string                   `json:"holder"`
	ExecutionContext WorktreeExecutionContext `json:"executionContext"`
	StartedAt        time.Time                `json:"startedAt"`
	CompletedAt      time.Time                `json:"completedAt"`
	Status           ActionRunStatus          `json:"status"`
	ExitCode         int                      `json:"exitCode,omitempty"`
	Stdout           string                   `json:"stdout,omitempty"`
	Stderr           string                   `json:"stderr,omitempty"`
	Prechecks        []ActionEvidence         `json:"prechecks"`
	Postchecks       []ActionEvidence         `json:"postchecks"`
}

type ActionRunStatus string

const (
	ActionRunPrecheckFailed  ActionRunStatus = "precheck_failed"
	ActionRunRunning         ActionRunStatus = "running"
	ActionRunSucceeded       ActionRunStatus = "succeeded"
	ActionRunFailed          ActionRunStatus = "failed"
	ActionRunCancelled       ActionRunStatus = "cancelled"
	ActionRunTimedOut        ActionRunStatus = "timed_out"
	ActionRunPostcheckFailed ActionRunStatus = "postcheck_failed"
	ActionRunUnavailable     ActionRunStatus = "unavailable"
)

// WorktreeExecutionTrust is the explicit transition from read-only discovery
// to eligibility for a future execution contract. It binds that grant to one
// immutable observed context; a later observation invalidates it.
type WorktreeExecutionTrust struct {
	Context   WorktreeExecutionContext `json:"context"`
	TrustedAt time.Time                `json:"trustedAt"`
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
	DecidedAt        time.Time     `json:"decidedAt"`
}

// ActionEvent is an immutable audit record produced only by the broker core.
type ActionEvent struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta      `json:"metadata"`
	Spec     ActionEventSpec `json:"spec"`
}

type ActionEventSpec struct {
	ActionPlanID     string    `json:"actionPlanId"`
	ActionPlanDigest string    `json:"actionPlanDigest"`
	EventType        string    `json:"eventType"`
	Actor            Actor     `json:"actor"`
	OccurredAt       time.Time `json:"occurredAt"`
}

// ActionDefinition is server-owned policy for one reviewed Action type.
type ActionDefinition struct {
	ActionType       string
	Risk             ActionRisk
	PolicyDecision   string
	ApprovalRequired bool
	Inputs           []string
	Execution        ActionExecution
	Prechecks        []ActionEvidenceContract
	Postchecks       []ActionEvidenceContract
}

var actionDefinitions = map[string]ActionDefinition{
	"repository.refresh":         {ActionType: "repository.refresh", Risk: RiskSafeLocal, PolicyDecision: PolicyAllowed, Execution: ActionExecution{Executable: "git", Arguments: []string{"fetch", "--prune"}, TimeoutSeconds: 60, MaxOutputBytes: 64 << 10}, Prechecks: worktreePrechecks, Postchecks: processExitPostcheck},
	"repository.sync":            {ActionType: "repository.sync", Risk: RiskSafeLocal, PolicyDecision: PolicyAllowed, Execution: ActionExecution{Executable: "git", Arguments: []string{"pull", "--ff-only", "--prune"}, TimeoutSeconds: 300, MaxOutputBytes: 64 << 10}, Prechecks: worktreePrechecks, Postchecks: processExitPostcheck},
	"release.production":         {ActionType: "release.production", Risk: RiskHighImpact, PolicyDecision: PolicyApprovalRequired, ApprovalRequired: true, Inputs: []string{"commit"}, Execution: ActionExecution{Executable: "devroom-release-production", Arguments: []string{"--commit", "{commit}"}, TimeoutSeconds: 300, MaxOutputBytes: 64 << 10}, Prechecks: worktreePrechecks, Postchecks: processExitPostcheck},
	"cleanup.destructive":        {ActionType: "cleanup.destructive", Risk: RiskHighImpact, PolicyDecision: PolicyApprovalRequired, ApprovalRequired: true, Inputs: []string{"candidate_id", "candidate_digest"}, Execution: ActionExecution{Executable: "devroom-cleanup-destructive", Arguments: []string{"--candidate", "{candidate_id}", "--digest", "{candidate_digest}"}, TimeoutSeconds: 300, MaxOutputBytes: 64 << 10}, Prechecks: worktreePrechecks, Postchecks: processExitPostcheck},
	"powershell.runbook":         {ActionType: "powershell.runbook", Risk: RiskHighImpact, PolicyDecision: PolicyApprovalRequired, ApprovalRequired: true, Inputs: []string{"script", "arguments", "environment", "timeout"}, Execution: ActionExecution{Executable: "pwsh", TimeoutSeconds: 300, MaxOutputBytes: 64 << 10}, Prechecks: worktreePrechecks, Postchecks: processExitPostcheck},
	"external.jenkins.group":     {ActionType: "external.jenkins.group", Risk: RiskHighImpact, PolicyDecision: PolicyApprovalRequired, ApprovalRequired: true, Inputs: []string{"group_id", "group_digest"}, Execution: ActionExecution{Executable: "devroom-external-jenkins", Arguments: []string{"--group", "{group_id}", "--digest", "{group_digest}"}, TimeoutSeconds: 1800, MaxOutputBytes: 64 << 10}, Prechecks: worktreePrechecks, Postchecks: processExitPostcheck},
	"release.jenkins.stage":      {ActionType: "release.jenkins.stage", Risk: RiskExternalChange, PolicyDecision: PolicyApprovalRequired, ApprovalRequired: true, Inputs: []string{"group_id", "group_digest", "environment", "expected_revision"}, Execution: ActionExecution{Executable: "devroom-release-jenkins", Arguments: []string{"--group", "{group_id}", "--digest", "{group_digest}", "--environment", "{environment}", "--expected-revision", "{expected_revision}"}, TimeoutSeconds: 1800, MaxOutputBytes: 64 << 10}, Prechecks: worktreePrechecks, Postchecks: processExitPostcheck},
	"release.jenkins.production": {ActionType: "release.jenkins.production", Risk: RiskHighImpact, PolicyDecision: PolicyApprovalRequired, ApprovalRequired: true, Inputs: []string{"group_id", "group_digest", "environment", "expected_revision"}, Execution: ActionExecution{Executable: "devroom-release-jenkins", Arguments: []string{"--group", "{group_id}", "--digest", "{group_digest}", "--environment", "{environment}", "--expected-revision", "{expected_revision}"}, TimeoutSeconds: 1800, MaxOutputBytes: 64 << 10}, Prechecks: worktreePrechecks, Postchecks: processExitPostcheck},
}

var (
	worktreePrechecks    = []ActionEvidenceContract{{ID: "worktree-identity", Kind: EvidenceWorktreeIdentity, Required: true}, {ID: "worktree-head", Kind: EvidenceWorktreeHead, Required: true}}
	processExitPostcheck = []ActionEvidenceContract{{ID: "process-exit", Kind: EvidenceProcessExit, Required: true}}
)

func ActionDefinitionFor(actionType string) (ActionDefinition, bool) {
	definition, ok := actionDefinitions[actionType]
	definition.Inputs = append([]string(nil), definition.Inputs...)
	definition.Execution.Arguments = append([]string(nil), definition.Execution.Arguments...)
	definition.Execution.EnvironmentAllowlist = append([]string(nil), definition.Execution.EnvironmentAllowlist...)
	definition.Prechecks = append([]ActionEvidenceContract(nil), definition.Prechecks...)
	definition.Postchecks = append([]ActionEvidenceContract(nil), definition.Postchecks...)
	return definition, ok
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
	VersionProbe          []string          `json:"versionProbe,omitempty"`
	TimeoutSeconds        int               `json:"timeoutSeconds,omitempty"`
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

// EnvironmentDeclaration is metadata about a required variable. Value is
// intentionally absent from this contract; the doctor only records presence
// and declaration conflicts.
type EnvironmentDeclaration struct {
	Name      string `json:"name"`
	Scope     string `json:"scope"`
	Purpose   string `json:"purpose,omitempty"`
	ProfileID string `json:"profileId,omitempty"`
	Connector string `json:"connector,omitempty"`
}

type ConnectorReference struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	SecretReference string     `json:"secretReference"`
	LastValidatedAt *time.Time `json:"lastValidatedAt,omitempty"`
	LastResult      string     `json:"lastResult,omitempty"`
}

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

type SafeguardRule struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta        `json:"metadata"`
	Spec     SafeguardRuleSpec `json:"spec"`
}

type SafeguardRuleSpec struct {
	Fingerprint          string             `json:"fingerprint"`
	Category             string             `json:"category"`
	ProjectID            string             `json:"projectId,omitempty"`
	RepositoryID         string             `json:"repositoryId,omitempty"`
	WorktreeID           string             `json:"worktreeId,omitempty"`
	State                SafeguardRuleState `json:"state"`
	Revision             int64              `json:"revision"`
	Owner                string             `json:"owner,omitempty"`
	OccurrenceCount      int                `json:"occurrenceCount"`
	FirstSeen            time.Time          `json:"firstSeen"`
	LastSeen             time.Time          `json:"lastSeen"`
	CreatedAt            time.Time          `json:"createdAt"`
	UpdatedAt            time.Time          `json:"updatedAt"`
	ActivatedAt          *time.Time         `json:"activatedAt,omitempty"`
	ActivationApprovedBy string             `json:"activationApprovedBy,omitempty"`
	RetiredAt            *time.Time         `json:"retiredAt,omitempty"`
	Metrics              SafeguardMetrics   `json:"metrics"`
}

type SafeguardMetrics struct {
	Evaluations         int `json:"evaluations"`
	Hits                int `json:"hits"`
	Misses              int `json:"misses"`
	PositiveFeedback    int `json:"positiveFeedback"`
	FalsePositives      int `json:"falsePositives"`
	EvaluationCostUnits int `json:"evaluationCostUnits"`
}

type SafeguardRuleState string

const (
	SafeguardProposal SafeguardRuleState = "proposal"
	SafeguardShadow   SafeguardRuleState = "shadow"
	SafeguardActive   SafeguardRuleState = "active"
	SafeguardRetired  SafeguardRuleState = "retired"
)

type SafeguardFeedback string

const (
	SafeguardFeedbackPositive      SafeguardFeedback = "positive"
	SafeguardFeedbackFalsePositive SafeguardFeedback = "false_positive"
)

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

func (r SafeguardRule) Validate() error {
	if err := validateResource(r.TypeMeta, SafeguardRuleKind, r.Metadata); err != nil {
		return err
	}
	if !planDigestPattern.MatchString(r.Spec.Fingerprint) || strings.TrimSpace(r.Spec.Category) == "" ||
		r.Spec.OccurrenceCount < 3 || r.Spec.FirstSeen.IsZero() || r.Spec.LastSeen.IsZero() ||
		r.Spec.CreatedAt.IsZero() || r.Spec.UpdatedAt.IsZero() || r.Spec.LastSeen.Before(r.Spec.FirstSeen) ||
		r.Spec.UpdatedAt.Before(r.Spec.CreatedAt) || r.Spec.Revision < 1 || !validSafeguardState(r.Spec.State) {
		return errors.New("safeguard rule requires repeated failure evidence, lifecycle state, and timestamps")
	}
	if validateProjectRepository(r.Spec.ProjectID, r.Spec.RepositoryID) != nil || (r.Spec.WorktreeID != "" && !validIdentifier(r.Spec.WorktreeID)) {
		return errors.New("safeguard requires a valid project and repository scope")
	}
	metrics := r.Spec.Metrics
	if metrics.Evaluations < 0 || metrics.Hits < 0 || metrics.Misses < 0 || metrics.PositiveFeedback < 0 ||
		metrics.FalsePositives < 0 || metrics.EvaluationCostUnits < 0 ||
		metrics.Evaluations != metrics.Hits+metrics.Misses || metrics.EvaluationCostUnits != metrics.Evaluations ||
		metrics.PositiveFeedback+metrics.FalsePositives > metrics.Hits {
		return errors.New("safeguard metrics are inconsistent")
	}
	if (r.Spec.State == SafeguardShadow || r.Spec.State == SafeguardActive) && strings.TrimSpace(r.Spec.Owner) == "" {
		return errors.New("shadow and active safeguards require an owner")
	}
	if r.Spec.State == SafeguardProposal && (r.Spec.ActivatedAt != nil || r.Spec.ActivationApprovedBy != "" || r.Spec.RetiredAt != nil) {
		return errors.New("proposed safeguard cannot have lifecycle completion timestamps")
	}
	if (r.Spec.ActivatedAt == nil) != (strings.TrimSpace(r.Spec.ActivationApprovedBy) == "") {
		return errors.New("safeguard activation time and human approval must be recorded together")
	}
	if r.Spec.State == SafeguardActive && r.Spec.ActivatedAt == nil {
		return errors.New("active safeguard requires a human-approved activation")
	}
	if (r.Spec.State == SafeguardRetired) != (r.Spec.RetiredAt != nil) {
		return errors.New("retired safeguard requires exactly one retirement time")
	}
	return nil
}

func (r *SafeguardRule) Transition(next SafeguardRuleState, owner string, at time.Time) error {
	if r == nil || at.IsZero() || at.Before(r.Spec.UpdatedAt) {
		return errors.New("safeguard transition requires a current rule and monotonic time")
	}
	updated := *r
	switch r.Spec.State {
	case SafeguardProposal:
		if next != SafeguardShadow && next != SafeguardRetired {
			return errors.New("proposed safeguard can only enter shadow mode or retire")
		}
	case SafeguardShadow:
		if next != SafeguardActive && next != SafeguardRetired {
			return errors.New("shadow safeguard can only activate or retire")
		}
	case SafeguardActive:
		if next != SafeguardShadow && next != SafeguardRetired {
			return errors.New("active safeguard can only roll back or retire")
		}
	case SafeguardRetired:
		return errors.New("retired safeguard is terminal")
	default:
		return errors.New("safeguard has an invalid lifecycle state")
	}
	if r.Spec.State == SafeguardProposal && next == SafeguardShadow {
		updated.Spec.Owner = strings.TrimSpace(owner)
		if updated.Spec.Owner == "" {
			return errors.New("shadow safeguard requires an owner")
		}
	}
	if next == SafeguardActive {
		metrics := r.Spec.Metrics
		if metrics.Hits < 1 || metrics.PositiveFeedback < 1 || metrics.FalsePositives != 0 {
			return errors.New("safeguard activation requires an exact hit, positive feedback, and no false positives")
		}
		updated.Spec.ActivationApprovedBy = strings.TrimSpace(owner)
		if updated.Spec.ActivationApprovedBy == "" {
			return errors.New("safeguard activation requires a human approver")
		}
		activated := at.UTC()
		updated.Spec.ActivatedAt = &activated
	}
	if next == SafeguardRetired {
		retired := at.UTC()
		updated.Spec.RetiredAt = &retired
	}
	updated.Spec.State = next
	updated.Spec.UpdatedAt = at.UTC()
	if err := updated.Validate(); err != nil {
		return err
	}
	*r = updated
	return nil
}

func (r *SafeguardRule) RecordEvaluation(hit bool, at time.Time) error {
	if r == nil || (r.Spec.State != SafeguardShadow && r.Spec.State != SafeguardActive) || at.IsZero() || at.Before(r.Spec.UpdatedAt) {
		return errors.New("only current shadow or active safeguards can record evaluations")
	}
	updated := *r
	updated.Spec.Metrics.Evaluations++
	updated.Spec.Metrics.EvaluationCostUnits++
	if hit {
		updated.Spec.Metrics.Hits++
	} else {
		updated.Spec.Metrics.Misses++
	}
	updated.Spec.UpdatedAt = at.UTC()
	if err := updated.Validate(); err != nil {
		return err
	}
	*r = updated
	return nil
}

func (r *SafeguardRule) RecordFeedback(feedback SafeguardFeedback, at time.Time) error {
	if r == nil || (r.Spec.State != SafeguardShadow && r.Spec.State != SafeguardActive) || at.IsZero() || at.Before(r.Spec.UpdatedAt) {
		return errors.New("only current shadow or active safeguards can receive feedback")
	}
	metrics := r.Spec.Metrics
	if metrics.PositiveFeedback+metrics.FalsePositives >= metrics.Hits {
		return errors.New("safeguard feedback requires an unreviewed exact evaluation hit")
	}
	updated := *r
	switch feedback {
	case SafeguardFeedbackPositive:
		updated.Spec.Metrics.PositiveFeedback++
	case SafeguardFeedbackFalsePositive:
		updated.Spec.Metrics.FalsePositives++
	default:
		return errors.New("safeguard feedback is invalid")
	}
	updated.Spec.UpdatedAt = at.UTC()
	if err := updated.Validate(); err != nil {
		return err
	}
	*r = updated
	return nil
}

func validSafeguardState(state SafeguardRuleState) bool {
	switch state {
	case SafeguardProposal, SafeguardShadow, SafeguardActive, SafeguardRetired:
		return true
	default:
		return false
	}
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
	if !validIdentifier(a.Spec.ProjectID) || !validIdentifier(a.Spec.RepositoryID) || !validIdentifier(a.Spec.WorktreeID) || !validActor(a.Spec.RequestedBy) || a.Spec.RequestedAt.IsZero() {
		return errors.New("action plan requires an exact target and requester")
	}
	definition, ok := ActionDefinitionFor(a.Spec.ActionType)
	if !ok || a.Spec.Risk != definition.Risk || a.Spec.PolicyDecision != definition.PolicyDecision || a.Spec.ApprovalRequired != definition.ApprovalRequired {
		return errors.New("action plan does not match a reviewed action definition")
	}
	if len(a.Spec.Inputs) != len(definition.Inputs) {
		return errors.New("action plan inputs do not match its reviewed definition")
	}
	for _, name := range definition.Inputs {
		if strings.TrimSpace(a.Spec.Inputs[name]) == "" {
			return errors.New("action plan is missing a reviewed input")
		}
	}
	if err := validateActionPlanApprovalScope(a.Spec); err != nil {
		return err
	}
	execution, err := definition.ExecutionFor(a.Spec.Inputs)
	if err != nil || !reflect.DeepEqual(a.Spec.Execution, execution) || !validExecutionContext(a.Spec.ExecutionContext) || a.Spec.ExecutionContext.ProjectID != a.Spec.ProjectID || a.Spec.ExecutionContext.RepositoryID != a.Spec.RepositoryID || a.Spec.ExecutionContext.WorktreeID != a.Spec.WorktreeID || !sameEvidenceContracts(a.Spec.Prechecks, definition.Prechecks) || !sameEvidenceContracts(a.Spec.Postchecks, definition.Postchecks) {
		return errors.New("action plan execution contract does not match its reviewed definition and target")
	}
	return nil
}

func (r ActionRun) Validate() error {
	if err := validateResource(r.TypeMeta, ActionRunKind, r.Metadata); err != nil {
		return err
	}
	if !validIdentifier(r.Spec.ActionPlanID) || !planDigestPattern.MatchString(r.Spec.ActionPlanDigest) ||
		validateProjectRepository(r.Spec.ProjectID, r.Spec.RepositoryID) != nil || !validIdentifier(r.Spec.WorktreeID) ||
		strings.TrimSpace(r.Spec.Holder) == "" || !validExecutionContext(r.Spec.ExecutionContext) ||
		r.Spec.ExecutionContext.ProjectID != r.Spec.ProjectID || r.Spec.ExecutionContext.RepositoryID != r.Spec.RepositoryID ||
		r.Spec.ExecutionContext.WorktreeID != r.Spec.WorktreeID || r.Spec.StartedAt.IsZero() || r.Spec.CompletedAt.IsZero() ||
		r.Spec.CompletedAt.Before(r.Spec.StartedAt) || !validActionRunStatus(r.Spec.Status) {
		return errors.New("action run requires an exact target, holder, context, timing, and status")
	}
	for _, evidence := range append(append([]ActionEvidence{}, r.Spec.Prechecks...), r.Spec.Postchecks...) {
		if !validIdentifier(evidence.ID) || !validEvidenceKind(evidence.Kind) {
			return errors.New("action run evidence requires valid ids and kinds")
		}
	}
	return nil
}

func validActionRunStatus(status ActionRunStatus) bool {
	switch status {
	case ActionRunPrecheckFailed, ActionRunRunning, ActionRunSucceeded, ActionRunFailed, ActionRunCancelled, ActionRunTimedOut, ActionRunPostcheckFailed, ActionRunUnavailable:
		return true
	default:
		return false
	}
}

func (d ActionDefinition) ExecutionFor(inputs map[string]string) (ActionExecution, error) {
	if d.ActionType == "powershell.runbook" {
		script := strings.TrimSpace(inputs["script"])
		if script == "" || strings.ContainsRune(script, '\x00') {
			return ActionExecution{}, errors.New("PowerShell runbook script is invalid")
		}
		var arguments []string
		if err := json.Unmarshal([]byte(inputs["arguments"]), &arguments); err != nil {
			return ActionExecution{}, errors.New("PowerShell runbook arguments are invalid")
		}
		var environmentAllowlist []string
		if raw := inputs["environment"]; raw != "" {
			if err := json.Unmarshal([]byte(raw), &environmentAllowlist); err != nil {
				return ActionExecution{}, errors.New("PowerShell runbook environment is invalid")
			}
		}
		timeoutSeconds := d.Execution.TimeoutSeconds
		if raw := strings.TrimSpace(inputs["timeout"]); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				return ActionExecution{}, errors.New("PowerShell runbook timeout is invalid")
			}
			timeoutSeconds = parsed
		}
		execution := ActionExecution{Executable: "pwsh", Arguments: append([]string{"-NoProfile", "-File", script}, arguments...), EnvironmentAllowlist: environmentAllowlist, TimeoutSeconds: timeoutSeconds, MaxOutputBytes: d.Execution.MaxOutputBytes}
		if !validActionExecution(execution) {
			return ActionExecution{}, errors.New("PowerShell runbook execution contract is invalid")
		}
		return execution, nil
	}
	execution := d.Execution
	execution.Arguments = append([]string(nil), execution.Arguments...)
	for index, argument := range execution.Arguments {
		if strings.HasPrefix(argument, "{") && strings.HasSuffix(argument, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(argument, "{"), "}")
			value := inputs[name]
			if !validIdentifier(name) || strings.TrimSpace(value) == "" || strings.ContainsRune(value, '\x00') {
				return ActionExecution{}, errors.New("action argument placeholder is invalid or unresolved")
			}
			execution.Arguments[index] = value
		}
	}
	if !validActionExecution(execution) {
		return ActionExecution{}, errors.New("action execution contract is invalid")
	}
	return execution, nil
}

func ExecutionContextForWorktree(worktree Worktree) (WorktreeExecutionContext, error) {
	if err := worktree.Validate(); err != nil || worktree.Spec.TombstonedAt != nil || strings.TrimSpace(worktree.Spec.Head) == "" {
		return WorktreeExecutionContext{}, errors.New("worktree is not an active observed execution target")
	}
	return WorktreeExecutionContext{ProjectID: worktree.Spec.ProjectID, RepositoryID: worktree.Spec.RepositoryID, WorktreeID: worktree.Metadata.ID, CanonicalPath: worktree.Spec.CanonicalPath, PathFingerprint: worktree.Spec.PathFingerprint, AssociationFingerprint: worktree.Spec.AssociationFingerprint, Head: worktree.Spec.Head, Branch: worktree.Spec.Branch}, nil
}

func NewWorktreeExecutionTrust(worktree Worktree, trustedAt time.Time) (WorktreeExecutionTrust, error) {
	if worktree.Spec.Trust != WorktreeTrustVerifiedReadOnly || trustedAt.IsZero() {
		return WorktreeExecutionTrust{}, errors.New("only a verified read-only worktree can become trusted for execution")
	}
	context, err := ExecutionContextForWorktree(worktree)
	if err != nil {
		return WorktreeExecutionTrust{}, err
	}
	return WorktreeExecutionTrust{Context: context, TrustedAt: trustedAt.UTC()}, nil
}

func (t WorktreeExecutionTrust) Validate() error {
	if !validExecutionContext(t.Context) || t.TrustedAt.IsZero() {
		return errors.New("worktree execution trust context and time are required")
	}
	return nil
}

func validActionExecution(execution ActionExecution) bool {
	if !commandNamePattern.MatchString(execution.Executable) || execution.TimeoutSeconds < 1 || execution.TimeoutSeconds > 3600 || execution.MaxOutputBytes < 1024 || execution.MaxOutputBytes > 1<<20 {
		return false
	}
	seen := map[string]struct{}{}
	for _, name := range execution.EnvironmentAllowlist {
		if !environmentNamePattern.MatchString(name) {
			return false
		}
		key := strings.ToUpper(name)
		if _, duplicate := seen[key]; duplicate {
			return false
		}
		seen[key] = struct{}{}
	}
	for _, argument := range execution.Arguments {
		if strings.ContainsAny(argument, "\x00\r\n") || strings.Contains(argument, "{") || strings.Contains(argument, "}") {
			return false
		}
	}
	return true
}

func validExecutionContext(context WorktreeExecutionContext) bool {
	return validateProjectRepository(context.ProjectID, context.RepositoryID) == nil && validIdentifier(context.WorktreeID) && strings.TrimSpace(context.CanonicalPath) != "" && strings.TrimSpace(context.PathFingerprint) != "" && strings.TrimSpace(context.Head) != ""
}

func sameEvidenceContracts(actual, expected []ActionEvidenceContract) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index, contract := range actual {
		if !validEvidenceContract(contract) || contract != expected[index] {
			return false
		}
	}
	return true
}

func validEvidenceContract(contract ActionEvidenceContract) bool {
	if !validIdentifier(contract.ID) {
		return false
	}
	return contract.Kind == EvidenceWorktreeIdentity || contract.Kind == EvidenceWorktreeHead || contract.Kind == EvidenceProcessExit
}

func validEvidenceKind(kind ActionEvidenceKind) bool {
	return kind == EvidenceWorktreeIdentity || kind == EvidenceWorktreeHead || kind == EvidenceProcessExit
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
	if !validIdentifier(a.Spec.ActionPlanID) || !planDigestPattern.MatchString(a.Spec.ActionPlanDigest) || !validActor(a.Spec.RequestedBy) || !validApprovalState(a.Spec.Status) || a.Spec.DecidedAt.IsZero() {
		return errors.New("approval action plan, digest, requester, decision time, and status are required")
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
	if a.Spec.DecidedAt.After(now) {
		return errors.New("approval decision cannot be in the future")
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

func (e ActionEvent) Validate() error {
	if err := validateResource(e.TypeMeta, ActionEventKind, e.Metadata); err != nil {
		return err
	}
	if !validIdentifier(e.Spec.ActionPlanID) || !planDigestPattern.MatchString(e.Spec.ActionPlanDigest) || strings.TrimSpace(e.Spec.EventType) == "" || !validActor(e.Spec.Actor) || e.Spec.OccurredAt.IsZero() {
		return errors.New("action event requires plan, digest, type, actor, and timestamp")
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
	if strings.ContainsAny(a.Spec.Command, "|&<>$`(){}[]\"'\x00\r\n") || strings.HasPrefix(strings.TrimSpace(a.Spec.Command), "-") {
		return errors.New("agent profile command contains unsupported shell syntax")
	}
	if a.Spec.TimeoutSeconds < 0 || a.Spec.TimeoutSeconds > 300 {
		return errors.New("agent profile timeout must be between 0 and 300 seconds")
	}
	if a.Spec.DataBoundary != AgentBoundaryEnterprise && a.Spec.DataBoundary != AgentBoundaryLocal {
		return errors.New("agent profile has an invalid data boundary")
	}
	if a.Spec.LaunchMode != AgentLaunchDirect && a.Spec.LaunchMode != AgentLaunchPowerShellProfile {
		return errors.New("agent profile has an invalid launch mode")
	}
	if a.Spec.LaunchMode == AgentLaunchPowerShellProfile && !commandNamePattern.MatchString(a.Spec.Command) {
		return errors.New("PowerShell profile command must be a command name")
	}
	if a.Spec.LaunchMode == AgentLaunchDirect && !commandNamePattern.MatchString(a.Spec.Command) && !validLocalWindowsExecutablePath(a.Spec.Command) {
		return errors.New("direct profile command must be an executable name or local absolute Windows executable path")
	}
	seen := make(map[string]struct{}, len(a.Spec.EnvironmentAllowlist))
	for _, name := range a.Spec.EnvironmentAllowlist {
		name = strings.TrimSpace(name)
		if !environmentNamePattern.MatchString(name) {
			return errors.New("agent environment allowlist contains an invalid variable name")
		}
		key := strings.ToUpper(name)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate agent environment variable %q", name)
		}
		seen[key] = struct{}{}
	}
	if len(a.Spec.VersionProbe) > 0 && !validVersionProbe(a.Spec.VersionProbe) {
		return errors.New("agent version probe must contain one reviewed version argument")
	}
	return nil
}

func validVersionProbe(arguments []string) bool {
	if len(arguments) != 1 {
		return false
	}
	switch arguments[0] {
	case "--version", "-version", "version", "-V", "-v", "/version":
		return true
	default:
		return false
	}
}

func validLocalWindowsExecutablePath(value string) bool {
	if strings.TrimSpace(value) != value || !windowsPathPattern.MatchString(value) || !strings.EqualFold(filepath.Ext(value), ".exe") || strings.Contains(value[2:], ":") {
		return false
	}
	segments := strings.Split(strings.ReplaceAll(value[3:], "/", `\`), `\`)
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." || strings.HasSuffix(segment, " ") || strings.HasSuffix(segment, ".") {
			return false
		}
	}
	return true
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

func validRelativePath(value string) bool {
	if strings.Contains(value, "\\") {
		return false
	}
	value = filepath.ToSlash(value)
	return value != "" && !strings.HasPrefix(value, "/") && !strings.Contains(value, "\x00") && !strings.HasPrefix(value, "../") && value != ".." && !strings.Contains(value, "/../")
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
