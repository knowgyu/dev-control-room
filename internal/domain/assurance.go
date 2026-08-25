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
)

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
	WorktreeID      string         `json:"worktreeId"`
	Head            string         `json:"head"`
	Provider        string         `json:"provider"`
	ProfileID       string         `json:"profileId"`
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
	CampaignID    string         `json:"campaignId"`
	ProjectID     string         `json:"projectId"`
	RepositoryID  string         `json:"repositoryId"`
	WorktreeID    string         `json:"worktreeId"`
	Head          string         `json:"head"`
	Technique     string         `json:"technique"`
	Runner        string         `json:"runner"`
	Command       CheckCommand   `json:"command"`
	ConfigDigest  string         `json:"configDigest"`
	State         string         `json:"state"`
	Summary       string         `json:"summary,omitempty"`
	ExitCode      int            `json:"exitCode,omitempty"`
	StartedAt     time.Time      `json:"startedAt"`
	CompletedAt   *time.Time     `json:"completedAt,omitempty"`
	ArtifactIDs   []string       `json:"artifactIds,omitempty"`
	InvocationIDs []string       `json:"invocationIds,omitempty"`
	Evidence      map[string]any `json:"evidence,omitempty"`
	StaleReason   string         `json:"staleReason,omitempty"`
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
	SourceType  string     `json:"sourceType"`
	SourceID    string     `json:"sourceId"`
	Path        string     `json:"path"`
	MIME        string     `json:"mime"`
	Size        int64      `json:"size"`
	SHA256      string     `json:"sha256"`
	Retention   string     `json:"retention"`
	CreatedAt   time.Time  `json:"createdAt"`
	ArchivedAt  *time.Time `json:"archivedAt,omitempty"`
	ArchivePath string     `json:"archivePath,omitempty"`
	DeletedAt   *time.Time `json:"deletedAt,omitempty"`
	SourceRef   string     `json:"sourceRef,omitempty"`
}

type Effect struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta `json:"metadata"`
	Spec     EffectSpec `json:"spec"`
}

type EffectSpec struct {
	Fingerprint  string    `json:"fingerprint"`
	ProjectID    string    `json:"projectId"`
	RepositoryID string    `json:"repositoryId"`
	WorktreeID   string    `json:"worktreeId"`
	Kind         string    `json:"kind"`
	SourceRunID  string    `json:"sourceRunId,omitempty"`
	EvidenceIDs  []string  `json:"evidenceIds,omitempty"`
	Adopted      bool      `json:"adopted"`
	Reverified   bool      `json:"reverified"`
	Label        string    `json:"label"`
	Value        float64   `json:"value,omitempty"`
	Unit         string    `json:"unit,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
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
	ProjectID       string     `json:"projectId"`
	RepositoryID    string     `json:"repositoryId"`
	WorktreeID      string     `json:"worktreeId"`
	ProviderProfile string     `json:"providerProfile"`
	Techniques      []string   `json:"techniques"`
	WritablePaths   []string   `json:"writablePaths"`
	NetworkPolicy   string     `json:"networkPolicy"`
	DiskLimitBytes  int64      `json:"diskLimitBytes"`
	Deadline        time.Time  `json:"deadline"`
	Prohibited      []string   `json:"prohibited"`
	State           string     `json:"state"`
	ApprovedBy      string     `json:"approvedBy"`
	ApprovedAt      *time.Time `json:"approvedAt,omitempty"`
}

func assuranceResource(meta TypeMeta, kind string, object ObjectMeta) error {
	if meta.APIVersion != APIVersion || meta.Kind != kind || !validIdentifier(object.ID) || strings.TrimSpace(object.Name) == "" {
		return fmt.Errorf("invalid %s resource", kind)
	}
	return nil
}

func validateAssuranceScope(projectID, repositoryID, worktreeID, head string) error {
	if err := validateProjectRepository(projectID, repositoryID); err != nil || !validIdentifier(worktreeID) || strings.TrimSpace(head) == "" {
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
	return nil
}

func (e Effect) Validate() error {
	if err := assuranceResource(e.TypeMeta, EffectKind, e.Metadata); err != nil {
		return err
	}
	if strings.TrimSpace(e.Spec.Fingerprint) == "" || strings.TrimSpace(e.Spec.Kind) == "" || !validEffectKind(e.Spec.Kind) || e.Spec.CreatedAt.IsZero() || e.Spec.UpdatedAt.IsZero() {
		return errors.New("effect requires a stable fingerprint, kind, and timestamps")
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
	if strings.TrimSpace(s.Spec.ProviderProfile) == "" || s.Spec.DiskLimitBytes <= 0 || s.Spec.Deadline.IsZero() || len(s.Spec.Prohibited) == 0 {
		return errors.New("unattended scope is not bounded")
	}
	return nil
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
