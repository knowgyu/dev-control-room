package domain

// This file contains the additive AI-assisted Code Assurance contracts. They
// deliberately do not share the ActionPlan lifecycle: an assurance run may
// produce evidence or a proposal, but it cannot grant approval or adopt code.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	AssuranceSessionKind           = "AssuranceSession"
	AssuranceSpecKind              = "AssuranceSpec"
	AgentInvocationKind            = "AgentInvocation"
	QualityCampaignKind            = "QualityCampaign"
	QualityRunKind                 = "QualityRun"
	PRCIBaselineKind               = "PRCIBaseline"
	ArtifactKind                   = "AssuranceArtifact"
	EffectKind                     = "AssuranceEffect"
	ProviderPricingSnapshotKind    = "ProviderPricingSnapshot"
	UnattendedApprovalScopeKind    = "UnattendedApprovalScope"
	AssuranceQuestionKind          = "AssuranceQuestion"
	AssuranceProposalKind          = "AssuranceProposal"
	AssuranceStateDraft            = "draft"
	AssuranceStateAwaitingAnswer   = "awaiting_answer"
	AssuranceStateReady            = "ready"
	AssuranceStateQueued           = "queued"
	AssuranceStateRunning          = "running"
	AssuranceStateCancelling       = "cancelling"
	AssuranceStateInterrupted      = "interrupted"
	AssuranceStateSucceeded        = "succeeded"
	AssuranceStateFailed           = "failed"
	AssuranceStateTimedOut         = "timed_out"
	AssuranceStateCancelled        = "cancelled"
	AssuranceStateStale            = "stale"
	AssuranceStateExpired          = "expired"
	BaselineRequired               = "required"
	BaselineObserved               = "observed"
	BaselineLocalEquivalent        = "local_equivalent"
	BaselineUnknown                = "unknown"
	QualityTechniqueStaticSecurity = "static_security"
	QualityTechniqueMutation       = "mutation"
	QualityTechniqueProperty       = "property"
	QualityTechniqueFuzz           = "fuzz"
	QualityTechniqueTargetedE2E    = "targeted_e2e"
	ArtifactRetentionActive        = "active"
	ArtifactRetentionPinned        = "pinned"
	ArtifactRetentionArchived      = "archived"
	ArtifactRetentionDeleted       = "deleted"
	EffectMeasured                 = "measured"
	EffectPreventedRegression      = "prevented_regression"
	EffectUserEstimated            = "user_estimated"
	EffectAIInference              = "ai_inference"
	EffectUnavailable              = "unavailable"
	UnattendedScopeDraft           = "draft"
	UnattendedScopeApproved        = "approved"
	UnattendedScopeRevoked         = "revoked"
	UnattendedScopeExpired         = "expired"
	NetworkPolicyOffline           = "offline"
	NetworkPolicyPublicDocs        = "public_docs"
	NetworkPolicyAllowlist         = "allowlist"
)

var requiredUnattendedProhibitions = []string{
	"ci_edit",
	"commit",
	"delete",
	"pull_request",
	"push",
	"remote_dispatch",
	"scope_expansion",
}

type AssuranceSession struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta           `json:"metadata"`
	Spec     AssuranceSessionSpec `json:"spec"`
}

type AssuranceSessionSpec struct {
	ProjectID      string      `json:"projectId"`
	RepositoryID   string      `json:"repositoryId"`
	WorktreeID     string      `json:"worktreeId"`
	Head           string      `json:"head"`
	State          string      `json:"state"`
	Provider       string      `json:"provider,omitempty"`
	RequestedModel string      `json:"requestedModel,omitempty"`
	CurrentSpecID  string      `json:"currentSpecId,omitempty"`
	QuestionIDs    []string    `json:"questionIds,omitempty"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
	LeaseExpiresAt *time.Time  `json:"leaseExpiresAt,omitempty"`
	ResumeBrief    ResumeBrief `json:"resumeBrief"`
}

type ResumeBrief struct {
	SessionID       string   `json:"sessionId"`
	CurrentHead     string   `json:"currentHead"`
	Completed       []string `json:"completed,omitempty"`
	Pending         []string `json:"pending,omitempty"`
	FailedEvidence  []string `json:"failedEvidence,omitempty"`
	WaitingQuestion string   `json:"waitingQuestion,omitempty"`
	ProposedPatch   string   `json:"proposedPatch,omitempty"`
	NextSafeAction  string   `json:"nextSafeAction,omitempty"`
}

type AssuranceQuestion struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta            `json:"metadata"`
	Spec     AssuranceQuestionSpec `json:"spec"`
}

type AssuranceQuestionSpec struct {
	SessionID  string     `json:"sessionId"`
	Prompt     string     `json:"prompt"`
	Answer     string     `json:"answer,omitempty"`
	Required   bool       `json:"required"`
	AskedAt    time.Time  `json:"askedAt"`
	AnsweredAt *time.Time `json:"answeredAt,omitempty"`
}

type AssuranceSpec struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta        `json:"metadata"`
	Spec     AssuranceSpecSpec `json:"spec"`
}

type AssuranceProposal struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta            `json:"metadata"`
	Spec     AssuranceProposalSpec `json:"spec"`
}

type AssuranceProposalSpec struct {
	SessionID        string     `json:"sessionId"`
	ProjectID        string     `json:"projectId"`
	RepositoryID     string     `json:"repositoryId"`
	WorktreeID       string     `json:"worktreeId"`
	BaseHead         string     `json:"baseHead"`
	IsolationPath    string     `json:"isolationPath"`
	PatchArtifactID  string     `json:"patchArtifactId,omitempty"`
	PatchDigest      string     `json:"patchDigest"`
	Purpose          string     `json:"purpose"`
	State            string     `json:"state"`
	CriticSummary    string     `json:"criticSummary,omitempty"`
	CriticConfidence string     `json:"criticConfidence,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	ReviewedAt       *time.Time `json:"reviewedAt,omitempty"`
}

type AssuranceSpecSpec struct {
	SessionID  string    `json:"sessionId"`
	Revision   int       `json:"revision"`
	Digest     string    `json:"digest"`
	Intent     string    `json:"intent"`
	Questions  []string  `json:"questions,omitempty"`
	Properties []string  `json:"properties,omitempty"`
	Targets    []string  `json:"targets,omitempty"`
	ToolSetup  []string  `json:"toolSetup,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	Source     string    `json:"source"`
	State      string    `json:"state"`
}

type AgentInvocation struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta          `json:"metadata"`
	Spec     AgentInvocationSpec `json:"spec"`
}

type AgentInvocationSpec struct {
	SessionID       string         `json:"sessionId"`
	ParentID        string         `json:"parentId,omitempty"`
	ProjectID       string         `json:"projectId,omitempty"`
	RepositoryID    string         `json:"repositoryId,omitempty"`
	WorktreeID      string         `json:"worktreeId"`
	Branch          string         `json:"branch,omitempty"`
	Head            string         `json:"head"`
	Provider        string         `json:"provider"`
	ProfileID       string         `json:"profileId"`
	ToolVersion     string         `json:"toolVersion,omitempty"`
	RequestedModel  string         `json:"requestedModel,omitempty"`
	ResolvedModel   string         `json:"resolvedModel,omitempty"`
	SelectionSource string         `json:"selectionSource"`
	State           string         `json:"state"`
	IdempotencyKey  string         `json:"idempotencyKey"`
	LeaseExpiresAt  *time.Time     `json:"leaseExpiresAt,omitempty"`
	StartedAt       time.Time      `json:"startedAt"`
	CompletedAt     *time.Time     `json:"completedAt,omitempty"`
	Structured      map[string]any `json:"structured,omitempty"`
	Usage           Usage          `json:"usage"`
	ArtifactIDs     []string       `json:"artifactIds,omitempty"`
	InputDigest     string         `json:"inputDigest,omitempty"`
	OutputDigest    string         `json:"outputDigest,omitempty"`
	TraceID         string         `json:"traceId,omitempty"`
	FailureCode     string         `json:"failureCode,omitempty"`
	RawTranscript   bool           `json:"rawTranscript"`
}

type Usage struct {
	InputTokens       *int64 `json:"inputTokens,omitempty"`
	CachedInputTokens *int64 `json:"cachedInputTokens,omitempty"`
	OutputTokens      *int64 `json:"outputTokens,omitempty"`
	ReasoningTokens   *int64 `json:"reasoningTokens,omitempty"`
	ToolTokens        *int64 `json:"toolTokens,omitempty"`
	TotalTokens       *int64 `json:"totalTokens,omitempty"`
	ElapsedMillis     int64  `json:"elapsedMillis,omitempty"`
	Attribution       string `json:"attribution"`
}

type QualityCampaign struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta          `json:"metadata"`
	Spec     QualityCampaignSpec `json:"spec"`
}

type QualityCampaignSpec struct {
	ProjectID    string    `json:"projectId"`
	RepositoryID string    `json:"repositoryId"`
	WorktreeID   string    `json:"worktreeId"`
	Name         string    `json:"name"`
	State        string    `json:"state"`
	RunIDs       []string  `json:"runIds,omitempty"`
	SessionID    string    `json:"sessionId,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type QualityRun struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta     `json:"metadata"`
	Spec     QualityRunSpec `json:"spec"`
}

type QualityRunSpec struct {
	CampaignID             string         `json:"campaignId"`
	ProjectID              string         `json:"projectId"`
	RepositoryID           string         `json:"repositoryId"`
	WorktreeID             string         `json:"worktreeId"`
	Branch                 string         `json:"branch,omitempty"`
	Head                   string         `json:"head"`
	BaselineID             string         `json:"baselineId,omitempty"`
	Technique              string         `json:"technique"`
	Runner                 string         `json:"runner"`
	Command                CheckCommand   `json:"command"`
	ConfigDigest           string         `json:"configDigest"`
	State                  string         `json:"state"`
	Summary                string         `json:"summary,omitempty"`
	ExitCode               int            `json:"exitCode,omitempty"`
	StartedAt              time.Time      `json:"startedAt"`
	CompletedAt            *time.Time     `json:"completedAt,omitempty"`
	ArtifactIDs            []string       `json:"artifactIds,omitempty"`
	InvocationIDs          []string       `json:"invocationIds,omitempty"`
	InputDigest            string         `json:"inputDigest,omitempty"`
	OutputDigest           string         `json:"outputDigest,omitempty"`
	TraceID                string         `json:"traceId,omitempty"`
	IsReverification       bool           `json:"isReverification,omitempty"`
	ReverificationEffectID string         `json:"reverificationEffectId,omitempty"`
	Evidence               map[string]any `json:"evidence,omitempty"`
	StaleReason            string         `json:"staleReason,omitempty"`
}

type BaselineEntry struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Classification string `json:"classification"`
	SourcePath     string `json:"sourcePath,omitempty"`
	Command        string `json:"command,omitempty"`
	Observed       bool   `json:"observed"`
	Required       bool   `json:"required"`
}

type PRCIBaseline struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta       `json:"metadata"`
	Spec     PRCIBaselineSpec `json:"spec"`
}

type PRCIBaselineSpec struct {
	ProjectID    string          `json:"projectId"`
	RepositoryID string          `json:"repositoryId"`
	WorktreeID   string          `json:"worktreeId"`
	TargetBranch string          `json:"targetBranch,omitempty"`
	Head         string          `json:"head"`
	SourceDigest string          `json:"sourceDigest"`
	CapturedAt   time.Time       `json:"capturedAt"`
	FreshUntil   time.Time       `json:"freshUntil"`
	State        string          `json:"state"`
	Entries      []BaselineEntry `json:"entries"`
	Sources      []string        `json:"sources,omitempty"`
	StaleReason  string          `json:"staleReason,omitempty"`
}

type Artifact struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta   `json:"metadata"`
	Spec     ArtifactSpec `json:"spec"`
}

type ArtifactSpec struct {
	ManifestVersion     string     `json:"manifestVersion,omitempty"`
	StorageKey          string     `json:"storageKey,omitempty"`
	SourceType          string     `json:"sourceType"`
	SourceID            string     `json:"sourceId"`
	Path                string     `json:"path"`
	MIME                string     `json:"mime"`
	Size                int64      `json:"size"`
	SHA256              string     `json:"sha256"`
	Retention           string     `json:"retention"`
	CreatedAt           time.Time  `json:"createdAt"`
	ArchivedAt          *time.Time `json:"archivedAt,omitempty"`
	ArchivePath         string     `json:"archivePath,omitempty"`
	ArchiveManifest     string     `json:"archiveManifest,omitempty"`
	ArchiveSHA256       string     `json:"archiveSha256,omitempty"`
	ArchiveID           string     `json:"archiveId,omitempty"`
	ArchiveVerifiedAt   *time.Time `json:"archiveVerifiedAt,omitempty"`
	RestoredAt          *time.Time `json:"restoredAt,omitempty"`
	PinnedAt            *time.Time `json:"pinnedAt,omitempty"`
	PinReason           string     `json:"pinReason,omitempty"`
	RetentionUntil      *time.Time `json:"retentionUntil,omitempty"`
	MaskingPolicyDigest string     `json:"maskingPolicyDigest,omitempty"`
	RedactionState      string     `json:"redactionState,omitempty"`
	TraceID             string     `json:"traceId,omitempty"`
	DeletedAt           *time.Time `json:"deletedAt,omitempty"`
	SourceRef           string     `json:"sourceRef,omitempty"`
}

type Effect struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta `json:"metadata"`
	Spec     EffectSpec `json:"spec"`
}

type EffectSpec struct {
	Fingerprint         string     `json:"fingerprint"`
	ProjectID           string     `json:"projectId"`
	RepositoryID        string     `json:"repositoryId"`
	WorktreeID          string     `json:"worktreeId"`
	Kind                string     `json:"kind"`
	MetricKey           string     `json:"metricKey,omitempty"`
	BaselineID          string     `json:"baselineId,omitempty"`
	SourceRunID         string     `json:"sourceRunId,omitempty"`
	SourceFindingID     string     `json:"sourceFindingId,omitempty"`
	EvidenceIDs         []string   `json:"evidenceIds,omitempty"`
	TraceIDs            []string   `json:"traceIds,omitempty"`
	TraceID             string     `json:"traceId,omitempty"`
	Adopted             bool       `json:"adopted"`
	Reverified          bool       `json:"reverified"`
	AdoptedAt           *time.Time `json:"adoptedAt,omitempty"`
	ReverifiedAt        *time.Time `json:"reverifiedAt,omitempty"`
	AdoptedCommit       string     `json:"adoptedCommit,omitempty"`
	ReverificationRunID string     `json:"reverificationRunId,omitempty"`
	ReverifiedCommit    string     `json:"reverifiedCommit,omitempty"`
	Label               string     `json:"label"`
	Value               float64    `json:"value,omitempty"`
	ValueKnown          bool       `json:"valueKnown"`
	Unit                string     `json:"unit,omitempty"`
	BaselineValue       *float64   `json:"baselineValue,omitempty"`
	BaselineUnit        string     `json:"baselineUnit,omitempty"`
	PeriodStart         *time.Time `json:"periodStart,omitempty"`
	PeriodEnd           *time.Time `json:"periodEnd,omitempty"`
	Outcome             string     `json:"outcome,omitempty"`
	RecordedBy          string     `json:"recordedBy,omitempty"`
	Reason              string     `json:"reason,omitempty"`
	Note                string     `json:"note,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

type ProviderPricingSnapshot struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta          `json:"metadata"`
	Spec     ProviderPricingSpec `json:"spec"`
}

type ProviderPricingSpec struct {
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	OfficialURL      string    `json:"officialUrl"`
	Currency         string    `json:"currency"`
	InputPerMillion  float64   `json:"inputPerMillion"`
	CachedPerMillion float64   `json:"cachedPerMillion,omitempty"`
	OutputPerMillion float64   `json:"outputPerMillion"`
	EffectiveAt      time.Time `json:"effectiveAt"`
	RetrievedAt      time.Time `json:"retrievedAt"`
	Status           string    `json:"status"`
}

type UnattendedApprovalScope struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta             `json:"metadata"`
	Spec     UnattendedApprovalSpec `json:"spec"`
}

type UnattendedApprovalSpec struct {
	ProjectID            string       `json:"projectId"`
	RepositoryID         string       `json:"repositoryId"`
	WorktreeID           string       `json:"worktreeId"`
	ProviderProfile      string       `json:"providerProfile"`
	ActionTypes          []string     `json:"actionTypes"`
	RiskClasses          []ActionRisk `json:"riskClasses"`
	Techniques           []string     `json:"techniques"`
	ToolSetup            []string     `json:"toolSetup"`
	ToolVersion          string       `json:"toolVersion"`
	ToolConfigDigest     string       `json:"toolConfigDigest"`
	ArgumentSchemaDigest string       `json:"argumentSchemaDigest"`
	WritablePaths        []string     `json:"writablePaths"`
	NetworkPolicy        string       `json:"networkPolicy"`
	DiskLimitBytes       int64        `json:"diskLimitBytes"`
	Deadline             time.Time    `json:"deadline"`
	Prohibited           []string     `json:"prohibited"`
	State                string       `json:"state"`
	ApprovedBy           string       `json:"approvedBy"`
	ApprovedAt           *time.Time   `json:"approvedAt,omitempty"`
	Revision             int          `json:"revision"`
	CreatedAt            time.Time    `json:"createdAt"`
	UpdatedAt            time.Time    `json:"updatedAt"`
}

// UnattendedApprovalRequest is the exact, resolved envelope checked before a
// potentially writable ActionPlan is admitted. It is deliberately separate
// from the approval record so callers cannot pass an approval decision or
// actor as part of a request.
type UnattendedApprovalRequest struct {
	ScopeID              string
	ScopeDigest          string
	ProjectID            string
	RepositoryID         string
	WorktreeID           string
	ProviderProfile      string
	ActionType           string
	Risk                 ActionRisk
	Techniques           []string
	ToolSetup            []string
	ToolVersion          string
	ToolConfigDigest     string
	ArgumentSchemaDigest string
	WritablePaths        []string
	NetworkPolicy        string
	DiskBytes            int64
	Deadline             time.Time
	Prohibited           []string
}

type UnattendedApprovalMatch struct {
	ScopeID     string    `json:"scopeId"`
	ScopeDigest string    `json:"scopeDigest"`
	Matched     bool      `json:"matched"`
	Reasons     []string  `json:"reasons"`
	CheckedAt   time.Time `json:"checkedAt"`
}

func assuranceResource(meta TypeMeta, kind string, object ObjectMeta) error {
	if meta.APIVersion != APIVersion || meta.Kind != kind || !validIdentifier(object.ID) || strings.TrimSpace(object.Name) == "" {
		return fmt.Errorf("invalid %s resource", kind)
	}
	return nil
}

func validateAssuranceScope(projectID, repositoryID, worktreeID, head string) error {
	if !validIdentifier(projectID) || !validIdentifier(repositoryID) || !validIdentifier(worktreeID) || strings.TrimSpace(head) == "" {
		return errors.New("assurance scope requires project, repository, worktree, and HEAD")
	}
	return nil
}

func (s AssuranceSession) Validate() error {
	if err := assuranceResource(s.TypeMeta, AssuranceSessionKind, s.Metadata); err != nil {
		return err
	}
	if err := validateAssuranceScope(s.Spec.ProjectID, s.Spec.RepositoryID, s.Spec.WorktreeID, s.Spec.Head); err != nil {
		return err
	}
	if !validAssuranceState(s.Spec.State) || s.Spec.CreatedAt.IsZero() || s.Spec.UpdatedAt.IsZero() {
		return errors.New("assurance session state and timestamps are required")
	}
	return nil
}

func (q AssuranceQuestion) Validate() error {
	if err := assuranceResource(q.TypeMeta, AssuranceQuestionKind, q.Metadata); err != nil {
		return err
	}
	if !validIdentifier(q.Spec.SessionID) || strings.TrimSpace(q.Spec.Prompt) == "" || q.Spec.AskedAt.IsZero() {
		return errors.New("assurance question requires a session, prompt, and asked time")
	}
	if q.Spec.AnsweredAt != nil && q.Spec.AnsweredAt.Before(q.Spec.AskedAt) {
		return errors.New("assurance answer cannot precede its question")
	}
	return nil
}

func (s AssuranceSpec) Validate() error {
	if err := assuranceResource(s.TypeMeta, AssuranceSpecKind, s.Metadata); err != nil {
		return err
	}
	if !validIdentifier(s.Spec.SessionID) || s.Spec.Revision < 1 || strings.TrimSpace(s.Spec.Intent) == "" || strings.TrimSpace(s.Spec.Source) == "" || s.Spec.CreatedAt.IsZero() {
		return errors.New("assurance spec requires session, revision, intent, source, and creation time")
	}
	if s.Spec.State == "" {
		return errors.New("assurance spec state is required")
	}
	return nil
}

func (p AssuranceProposal) Validate() error {
	if err := assuranceResource(p.TypeMeta, AssuranceProposalKind, p.Metadata); err != nil {
		return err
	}
	if err := validateAssuranceScope(p.Spec.ProjectID, p.Spec.RepositoryID, p.Spec.WorktreeID, p.Spec.BaseHead); err != nil {
		return err
	}
	if !validIdentifier(p.Spec.SessionID) || strings.TrimSpace(p.Spec.IsolationPath) == "" || strings.TrimSpace(p.Spec.PatchDigest) == "" || strings.TrimSpace(p.Spec.Purpose) == "" || p.Spec.CreatedAt.IsZero() {
		return errors.New("assurance proposal requires isolated scope, patch, purpose, and time")
	}
	switch p.Spec.State {
	case "proposed", "critic_advisory", "adopted", "rejected", "stale":
	default:
		return errors.New("assurance proposal state is invalid")
	}
	if (p.Spec.State == "adopted" || p.Spec.State == "rejected") != (p.Spec.ReviewedAt != nil) {
		return errors.New("reviewed proposal requires a review time")
	}
	return nil
}

func (i AgentInvocation) Validate() error {
	if err := assuranceResource(i.TypeMeta, AgentInvocationKind, i.Metadata); err != nil {
		return err
	}
	if !validIdentifier(i.Spec.SessionID) || !validIdentifier(i.Spec.WorktreeID) || strings.TrimSpace(i.Spec.Provider) == "" || strings.TrimSpace(i.Spec.ProfileID) == "" || strings.TrimSpace(i.Spec.IdempotencyKey) == "" || i.Spec.StartedAt.IsZero() || !validAssuranceState(i.Spec.State) {
		return errors.New("agent invocation contract is incomplete")
	}
	return nil
}

func (c QualityCampaign) Validate() error {
	if err := assuranceResource(c.TypeMeta, QualityCampaignKind, c.Metadata); err != nil {
		return err
	}
	if err := validateAssuranceScope(c.Spec.ProjectID, c.Spec.RepositoryID, c.Spec.WorktreeID, "head"); err != nil {
		return err
	}
	if strings.TrimSpace(c.Spec.Name) == "" || c.Spec.CreatedAt.IsZero() || c.Spec.UpdatedAt.IsZero() {
		return errors.New("quality campaign requires a name and timestamps")
	}
	return nil
}

func (r QualityRun) Validate() error {
	if err := assuranceResource(r.TypeMeta, QualityRunKind, r.Metadata); err != nil {
		return err
	}
	if err := validateAssuranceScope(r.Spec.ProjectID, r.Spec.RepositoryID, r.Spec.WorktreeID, r.Spec.Head); err != nil {
		return err
	}
	if !validIdentifier(r.Spec.CampaignID) || !validQualityTechnique(r.Spec.Technique) || strings.TrimSpace(r.Spec.Runner) == "" || !validAssuranceState(r.Spec.State) || r.Spec.StartedAt.IsZero() {
		return errors.New("quality run contract is incomplete")
	}
	return nil
}

func (b PRCIBaseline) Validate() error {
	if err := assuranceResource(b.TypeMeta, PRCIBaselineKind, b.Metadata); err != nil {
		return err
	}
	if err := validateAssuranceScope(b.Spec.ProjectID, b.Spec.RepositoryID, b.Spec.WorktreeID, b.Spec.Head); err != nil {
		return err
	}
	if b.Spec.CapturedAt.IsZero() || b.Spec.FreshUntil.IsZero() || b.Spec.SourceDigest == "" || len(b.Spec.Entries) == 0 {
		return errors.New("PR CI baseline requires source, freshness, and entries")
	}
	return nil
}

func (a Artifact) Validate() error {
	if err := assuranceResource(a.TypeMeta, ArtifactKind, a.Metadata); err != nil {
		return err
	}
	if strings.TrimSpace(a.Spec.SourceType) == "" || strings.TrimSpace(a.Spec.SourceID) == "" || strings.TrimSpace(a.Spec.Path) == "" || a.Spec.Size < 0 || len(a.Spec.SHA256) != 64 || !validArtifactRetention(a.Spec.Retention) || a.Spec.CreatedAt.IsZero() {
		return errors.New("artifact manifest is incomplete")
	}
	if a.Spec.ArchiveSHA256 != "" && len(a.Spec.ArchiveSHA256) != 64 {
		return errors.New("artifact archive hash is invalid")
	}
	if a.Spec.ArchiveManifest != "" && strings.TrimSpace(a.Spec.ArchivePath) == "" {
		return errors.New("artifact archive manifest requires an archive path")
	}
	if a.Spec.RestoredAt != nil && a.Spec.RestoredAt.Before(a.Spec.CreatedAt) {
		return errors.New("artifact restore cannot precede creation")
	}
	return nil
}

func (e Effect) Validate() error {
	if err := assuranceResource(e.TypeMeta, EffectKind, e.Metadata); err != nil {
		return err
	}
	if strings.TrimSpace(e.Spec.Fingerprint) == "" || strings.TrimSpace(e.Spec.Kind) == "" || !validEffectKind(e.Spec.Kind) || e.Spec.CreatedAt.IsZero() || e.Spec.UpdatedAt.IsZero() {
		return errors.New("effect requires a stable fingerprint, kind, and timestamps")
	}
	if e.Spec.BaselineValue != nil && e.Spec.BaselineUnit != "" && e.Spec.Unit != "" && e.Spec.BaselineUnit != e.Spec.Unit {
		return errors.New("effect baseline and observed units must match")
	}
	if e.Spec.Outcome != "" && !validEffectOutcome(e.Spec.Outcome) {
		return errors.New("effect outcome is invalid")
	}
	if e.Spec.AdoptedAt != nil && e.Spec.AdoptedAt.Before(e.Spec.CreatedAt) {
		return errors.New("effect adoption cannot precede creation")
	}
	if e.Spec.ReverifiedAt != nil && e.Spec.ReverifiedAt.Before(e.Spec.CreatedAt) {
		return errors.New("effect reverification cannot precede creation")
	}
	return nil
}

func (p ProviderPricingSnapshot) Validate() error {
	if err := assuranceResource(p.TypeMeta, ProviderPricingSnapshotKind, p.Metadata); err != nil {
		return err
	}
	if strings.TrimSpace(p.Spec.Provider) == "" || strings.TrimSpace(p.Spec.Model) == "" || strings.TrimSpace(p.Spec.OfficialURL) == "" || p.Spec.Currency == "" || p.Spec.InputPerMillion < 0 || p.Spec.OutputPerMillion < 0 || p.Spec.EffectiveAt.IsZero() || p.Spec.RetrievedAt.IsZero() {
		return errors.New("pricing snapshot is incomplete")
	}
	return nil
}

func (s UnattendedApprovalScope) Validate() error {
	if err := assuranceResource(s.TypeMeta, UnattendedApprovalScopeKind, s.Metadata); err != nil {
		return err
	}
	if err := validateAssuranceScope(s.Spec.ProjectID, s.Spec.RepositoryID, s.Spec.WorktreeID, "scope"); err != nil {
		return err
	}
	if !validIdentifier(s.Spec.ProviderProfile) || s.Spec.DiskLimitBytes <= 0 || s.Spec.Deadline.IsZero() || len(s.Spec.Prohibited) == 0 || s.Spec.Revision < 1 || s.Spec.CreatedAt.IsZero() || s.Spec.UpdatedAt.IsZero() {
		return errors.New("unattended scope is not bounded")
	}
	if s.Spec.Deadline.Before(s.Spec.CreatedAt) || s.Spec.UpdatedAt.Before(s.Spec.CreatedAt) {
		return errors.New("unattended scope timestamps are invalid")
	}
	if !validUnattendedScopeState(s.Spec.State) {
		return errors.New("unattended scope state is invalid")
	}
	if len(s.Spec.ActionTypes) == 0 || len(s.Spec.RiskClasses) == 0 || len(s.Spec.Techniques) == 0 || len(s.Spec.WritablePaths) == 0 {
		return errors.New("unattended scope must enumerate actions, risks, techniques, and writable paths")
	}
	if err := validateApprovalIdentifiers(s.Spec.ActionTypes, true, func(value string) bool {
		_, ok := ActionDefinitionFor(value)
		return ok
	}); err != nil {
		return fmt.Errorf("unattended scope action types: %w", err)
	}
	if err := validateApprovalIdentifiers(s.Spec.Techniques, true, validQualityTechnique); err != nil {
		return fmt.Errorf("unattended scope techniques: %w", err)
	}
	if err := validateApprovalIdentifiers(s.Spec.ToolSetup, false, validIdentifier); err != nil {
		return fmt.Errorf("unattended scope tool setup: %w", err)
	}
	if err := validateApprovalRisks(s.Spec.RiskClasses); err != nil {
		return err
	}
	for _, path := range s.Spec.WritablePaths {
		if _, err := normalizeApprovalPath(path); err != nil {
			return fmt.Errorf("unattended scope writable path: %w", err)
		}
	}
	if !validNetworkPolicy(s.Spec.NetworkPolicy) || !validBoundedText(s.Spec.ToolVersion) || !planDigestPattern.MatchString(s.Spec.ToolConfigDigest) || !planDigestPattern.MatchString(s.Spec.ArgumentSchemaDigest) {
		return errors.New("unattended scope tool and network bounds are invalid")
	}
	if err := validateApprovalIdentifiers(s.Spec.Prohibited, true, validIdentifier); err != nil {
		return fmt.Errorf("unattended scope prohibitions: %w", err)
	}
	for _, required := range requiredUnattendedProhibitions {
		if !containsString(s.Spec.Prohibited, required) {
			return fmt.Errorf("unattended scope must prohibit %s", required)
		}
	}
	switch s.Spec.State {
	case UnattendedScopeDraft:
		if s.Spec.ApprovedBy != "" || s.Spec.ApprovedAt != nil {
			return errors.New("draft unattended scope cannot contain approval evidence")
		}
	case UnattendedScopeApproved, UnattendedScopeRevoked, UnattendedScopeExpired:
		if !validBoundedText(s.Spec.ApprovedBy) || s.Spec.ApprovedAt == nil || s.Spec.ApprovedAt.Before(s.Spec.CreatedAt) {
			return errors.New("non-draft unattended scope requires approval evidence")
		}
	}
	return nil
}

func (s UnattendedApprovalScope) Digest() (string, error) {
	return assuranceDigest(s.Spec)
}

func (s UnattendedApprovalScope) ValidateForApprovalAt(now time.Time) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if s.Spec.State != UnattendedScopeDraft {
		return errors.New("only a draft unattended scope can be approved")
	}
	if !s.Spec.Deadline.After(now.UTC()) {
		return errors.New("unattended scope deadline has passed")
	}
	return nil
}

func (s UnattendedApprovalScope) Match(request UnattendedApprovalRequest, now time.Time) UnattendedApprovalMatch {
	digest, err := s.Digest()
	result := UnattendedApprovalMatch{ScopeID: s.Metadata.ID, ScopeDigest: digest, CheckedAt: now.UTC(), Matched: err == nil}
	if err != nil {
		result.Reasons = []string{"scope_invalid"}
		return result
	}
	if err := s.ValidateForExecutionAt(now); err != nil {
		result.Matched = false
		result.Reasons = append(result.Reasons, reasonForScopeError(err))
	}
	if request.ScopeID != s.Metadata.ID {
		result.Matched = false
		result.Reasons = append(result.Reasons, "scope_id_mismatch")
	}
	if request.ScopeDigest != digest {
		result.Matched = false
		result.Reasons = append(result.Reasons, "scope_digest_mismatch")
	}
	if request.ProjectID != s.Spec.ProjectID || request.RepositoryID != s.Spec.RepositoryID || request.WorktreeID != s.Spec.WorktreeID {
		result.Matched = false
		result.Reasons = append(result.Reasons, "worktree_scope_mismatch")
	}
	if request.ProviderProfile != s.Spec.ProviderProfile {
		result.Matched = false
		result.Reasons = append(result.Reasons, "provider_profile_mismatch")
	}
	if !containsString(s.Spec.ActionTypes, request.ActionType) {
		result.Matched = false
		result.Reasons = append(result.Reasons, "action_type_not_allowed")
	}
	if !containsRisk(s.Spec.RiskClasses, request.Risk) {
		result.Matched = false
		result.Reasons = append(result.Reasons, "risk_not_allowed")
	}
	if !containsAll(s.Spec.Techniques, request.Techniques) {
		result.Matched = false
		result.Reasons = append(result.Reasons, "technique_not_allowed")
	}
	if !containsAll(s.Spec.ToolSetup, request.ToolSetup) || request.ToolVersion != s.Spec.ToolVersion || request.ToolConfigDigest != s.Spec.ToolConfigDigest || request.ArgumentSchemaDigest != s.Spec.ArgumentSchemaDigest {
		result.Matched = false
		result.Reasons = append(result.Reasons, "tool_contract_mismatch")
	}
	if !pathsWithin(s.Spec.WritablePaths, request.WritablePaths) {
		result.Matched = false
		result.Reasons = append(result.Reasons, "writable_path_not_allowed")
	}
	if request.NetworkPolicy != s.Spec.NetworkPolicy {
		result.Matched = false
		result.Reasons = append(result.Reasons, "network_policy_mismatch")
	}
	if request.DiskBytes <= 0 || request.DiskBytes > s.Spec.DiskLimitBytes {
		result.Matched = false
		result.Reasons = append(result.Reasons, "disk_limit_exceeded")
	}
	if request.Deadline.IsZero() || request.Deadline.After(s.Spec.Deadline) || !request.Deadline.After(now.UTC()) {
		result.Matched = false
		result.Reasons = append(result.Reasons, "deadline_invalid")
	}
	if !containsAll(request.Prohibited, s.Spec.Prohibited) {
		result.Matched = false
		result.Reasons = append(result.Reasons, "prohibition_set_weakened")
	}
	if result.Matched {
		result.Reasons = []string{"exact_match"}
	}
	return result
}

func (s UnattendedApprovalScope) ValidateForExecutionAt(now time.Time) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if s.Spec.State != UnattendedScopeApproved {
		return errors.New("unattended scope is not approved")
	}
	if !s.Spec.Deadline.After(now.UTC()) {
		return errors.New("unattended scope is expired")
	}
	return nil
}

func (a ActionPlan) UnattendedApprovalRequest() UnattendedApprovalRequest {
	return UnattendedApprovalRequest{
		ScopeID:              a.Spec.ApprovalScopeID,
		ScopeDigest:          a.Spec.ApprovalScopeDigest,
		ProjectID:            a.Spec.ProjectID,
		RepositoryID:         a.Spec.RepositoryID,
		WorktreeID:           a.Spec.WorktreeID,
		ProviderProfile:      a.Spec.ProviderProfile,
		ActionType:           a.Spec.ActionType,
		Risk:                 a.Spec.Risk,
		Techniques:           append([]string(nil), a.Spec.Techniques...),
		ToolSetup:            append([]string(nil), a.Spec.ToolSetup...),
		ToolVersion:          a.Spec.ToolVersion,
		ToolConfigDigest:     a.Spec.ToolConfigDigest,
		ArgumentSchemaDigest: a.Spec.ArgumentSchemaDigest,
		WritablePaths:        append([]string(nil), a.Spec.WritablePaths...),
		NetworkPolicy:        a.Spec.NetworkPolicy,
		DiskBytes:            a.Spec.DiskLimitBytes,
		Deadline:             a.Spec.ScopeDeadline,
		Prohibited:           append([]string(nil), a.Spec.ProhibitedOperations...),
	}
}

func validateActionPlanApprovalScope(spec ActionPlanSpec) error {
	if spec.ApprovalScopeID == "" {
		if spec.ApprovalScopeDigest != "" || spec.ProviderProfile != "" || len(spec.Techniques) > 0 || len(spec.ToolSetup) > 0 || spec.ToolVersion != "" || spec.ToolConfigDigest != "" || spec.ArgumentSchemaDigest != "" || len(spec.WritablePaths) > 0 || spec.NetworkPolicy != "" || spec.DiskLimitBytes != 0 || !spec.ScopeDeadline.IsZero() || len(spec.ProhibitedOperations) > 0 || spec.ScopeMatch || len(spec.ScopeMatchReasons) > 0 || !spec.ScopeCheckedAt.IsZero() {
			return errors.New("action plan contains an unbound approval scope contract")
		}
		return nil
	}
	if !validIdentifier(spec.ApprovalScopeID) || !planDigestPattern.MatchString(spec.ApprovalScopeDigest) || spec.ScopeCheckedAt.IsZero() || spec.ScopeDeadline.IsZero() {
		return errors.New("action plan approval scope evidence is incomplete")
	}
	request := ActionPlan{Spec: spec}.UnattendedApprovalRequest()
	if err := validateUnattendedApprovalRequest(request); err != nil {
		return err
	}
	if spec.ScopeMatch && len(spec.ScopeMatchReasons) == 0 {
		return errors.New("matched approval scope requires a match reason")
	}
	if !spec.ScopeMatch && len(spec.ScopeMatchReasons) == 0 {
		return errors.New("rejected approval scope requires mismatch reasons")
	}
	return nil
}

func validateUnattendedApprovalRequest(request UnattendedApprovalRequest) error {
	if !validIdentifier(request.ScopeID) || !planDigestPattern.MatchString(request.ScopeDigest) || !validIdentifier(request.ProjectID) || !validIdentifier(request.RepositoryID) || !validIdentifier(request.WorktreeID) || !validIdentifier(request.ProviderProfile) || strings.TrimSpace(request.ActionType) == "" || !validRisk(request.Risk) || !validBoundedText(request.ToolVersion) || !planDigestPattern.MatchString(request.ToolConfigDigest) || !planDigestPattern.MatchString(request.ArgumentSchemaDigest) || !validNetworkPolicy(request.NetworkPolicy) || request.DiskBytes <= 0 || request.Deadline.IsZero() {
		return errors.New("action plan approval scope request is incomplete")
	}
	if _, ok := ActionDefinitionFor(request.ActionType); !ok {
		return errors.New("action plan approval scope action is not reviewed")
	}
	if err := validateApprovalIdentifiers(request.Techniques, true, validQualityTechnique); err != nil {
		return errors.New("action plan approval scope techniques are invalid")
	}
	if err := validateApprovalIdentifiers(request.ToolSetup, false, validIdentifier); err != nil {
		return errors.New("action plan approval scope tool setup is invalid")
	}
	if len(request.WritablePaths) == 0 {
		return errors.New("action plan approval scope requires writable paths")
	}
	for _, path := range request.WritablePaths {
		if _, err := normalizeApprovalPath(path); err != nil {
			return errors.New("action plan approval scope writable path is invalid")
		}
	}
	if err := validateApprovalIdentifiers(request.Prohibited, true, validIdentifier); err != nil {
		return errors.New("action plan approval scope prohibitions are invalid")
	}
	for _, required := range requiredUnattendedProhibitions {
		if !containsString(request.Prohibited, required) {
			return errors.New("action plan approval scope weakens a required prohibition")
		}
	}
	return nil
}

func validateApprovalIdentifiers(values []string, required bool, valid func(string) bool) error {
	if required && len(values) == 0 {
		return errors.New("at least one value is required")
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != value || !valid(value) {
			return errors.New("contains an invalid value")
		}
		if _, ok := seen[value]; ok {
			return errors.New("contains a duplicate value")
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateApprovalRisks(values []ActionRisk) error {
	seen := make(map[ActionRisk]struct{}, len(values))
	for _, value := range values {
		if !validRisk(value) {
			return errors.New("unattended scope risk class is invalid")
		}
		if _, ok := seen[value]; ok {
			return errors.New("unattended scope risk classes contain a duplicate")
		}
		seen[value] = struct{}{}
	}
	return nil
}

func normalizeApprovalPath(value string) (string, error) {
	if !validApprovalPathText(value) || strings.ContainsAny(value, "*?") || (!filepath.IsAbs(value) && !windowsPathPattern.MatchString(value)) {
		return "", errors.New("path must be an absolute path without wildcards")
	}
	cleaned := filepath.Clean(value)
	if cleaned == filepath.VolumeName(cleaned)+string(filepath.Separator) || cleaned == string(filepath.Separator) {
		return "", errors.New("path cannot be a filesystem root")
	}
	return cleaned, nil
}

func validApprovalPathText(value string) bool {
	return len(value) > 0 && len(value) <= 32767 && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}

func pathsWithin(roots, paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	for _, path := range paths {
		target, err := normalizeApprovalPath(path)
		if err != nil {
			return false
		}
		allowed := false
		for _, root := range roots {
			base, err := normalizeApprovalPath(root)
			if err == nil && approvalPathContains(base, target) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}
	return true
}

func approvalPathContains(root, target string) bool {
	if runtime.GOOS == "windows" {
		root = strings.ToLower(root)
		target = strings.ToLower(target)
	}
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative))
}

func containsAll(allowed, requested []string) bool {
	for _, value := range requested {
		if !containsString(allowed, value) {
			return false
		}
	}
	return true
}

func containsRisk(allowed []ActionRisk, requested ActionRisk) bool {
	for _, value := range allowed {
		if value == requested {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func validUnattendedScopeState(value string) bool {
	return value == UnattendedScopeDraft || value == UnattendedScopeApproved || value == UnattendedScopeRevoked || value == UnattendedScopeExpired
}

func validNetworkPolicy(value string) bool {
	return value == NetworkPolicyOffline || value == NetworkPolicyPublicDocs || value == NetworkPolicyAllowlist
}

func validBoundedText(value string) bool {
	return len(value) > 0 && len(value) <= 128 && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}

func reasonForScopeError(err error) string {
	if strings.Contains(err.Error(), "expired") || strings.Contains(err.Error(), "deadline") {
		return "scope_expired"
	}
	return "scope_not_approved"
}

func (a AgentInvocation) Digest() (string, error) { return assuranceDigest(a) }
func (r QualityRun) Digest() (string, error)      { return assuranceDigest(r) }
func (s AssuranceSpec) Digest() (string, error)   { return assuranceDigest(s.Spec) }

func assuranceDigest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func validAssuranceState(value string) bool {
	switch value {
	case AssuranceStateDraft, AssuranceStateAwaitingAnswer, AssuranceStateReady, AssuranceStateQueued, AssuranceStateRunning, AssuranceStateCancelling, AssuranceStateInterrupted, AssuranceStateSucceeded, AssuranceStateFailed, AssuranceStateTimedOut, AssuranceStateCancelled, AssuranceStateStale, AssuranceStateExpired:
		return true
	}
	return false
}
func validQualityTechnique(value string) bool {
	switch value {
	case QualityTechniqueStaticSecurity, QualityTechniqueMutation, QualityTechniqueProperty, QualityTechniqueFuzz, QualityTechniqueTargetedE2E:
		return true
	}
	return false
}
func validArtifactRetention(value string) bool {
	switch value {
	case ArtifactRetentionActive, ArtifactRetentionPinned, ArtifactRetentionArchived, ArtifactRetentionDeleted:
		return true
	}
	return false
}
func validEffectKind(value string) bool {
	switch value {
	case EffectMeasured, EffectPreventedRegression, EffectUserEstimated, EffectAIInference, EffectUnavailable:
		return true
	}
	return false
}

func validEffectOutcome(value string) bool {
	switch value {
	case "improved", "unchanged", "regressed", "unknown":
		return true
	}
	return false
}
