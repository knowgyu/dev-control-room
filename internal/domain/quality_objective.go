package domain

import (
	"errors"
	"sort"
	"strings"
	"time"
)

const (
	QualityObjectiveKind = "QualityObjective"

	QualityObjectiveStateDraft           = "draft"
	QualityObjectiveStateBaselinePending = "baseline_pending"
	QualityObjectiveStateReady           = "ready"
	QualityObjectiveStateRunning         = "running"
	QualityObjectiveStateReview          = "review"
	QualityObjectiveStateAdopted         = "adopted"
	QualityObjectiveStateRejected        = "rejected"
	QualityObjectiveStateStale           = "stale"
	QualityObjectiveStateBlocked         = "blocked"

	QualityObjectiveSignalKindFinding    = "finding"
	QualityObjectiveSignalKindGoCoverage = "go_coverage"

	QualityObjectiveDispositionPursue  = "pursue"
	QualityObjectiveDispositionDefer   = "defer"
	QualityObjectiveDispositionDismiss = "dismiss"

	QualityObjectiveRevalidationImproved     = "improved"
	QualityObjectiveRevalidationNotImproved  = "not_improved"
	QualityObjectiveRevalidationInconclusive = "inconclusive"

	QualityObjectiveMaxRevalidations = 20
)

// QualityObjective is a user-owned quality improvement item. It keeps the
// durable links needed to connect a quality goal to existing assurance data,
// without introducing a second execution workflow.
type QualityObjective struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta           `json:"metadata"`
	Spec     QualityObjectiveSpec `json:"spec"`
}

type QualityObjectiveSpec struct {
	ProjectID     string                         `json:"projectId"`
	RepositoryID  string                         `json:"repositoryId"`
	WorktreeID    string                         `json:"worktreeId"`
	Head          string                         `json:"head"`
	Owner         string                         `json:"owner"`
	Title         string                         `json:"title"`
	Description   string                         `json:"description,omitempty"`
	State         string                         `json:"state"`
	Revision      int                            `json:"revision"`
	FindingIDs    []string                       `json:"findingIds,omitempty"`
	SessionID     string                         `json:"sessionId,omitempty"`
	BaselineID    string                         `json:"baselineId,omitempty"`
	CampaignID    string                         `json:"campaignId,omitempty"`
	RunIDs        []string                       `json:"runIds,omitempty"`
	ProposalIDs   []string                       `json:"proposalIds,omitempty"`
	PrimarySignal *QualityObjectiveSignal        `json:"primarySignal,omitempty"`
	Decision      *QualityObjectiveDecision      `json:"decision,omitempty"`
	Revalidations []QualityObjectiveRevalidation `json:"revalidations,omitempty"`
	CreatedAt     time.Time                      `json:"createdAt"`
	UpdatedAt     time.Time                      `json:"updatedAt"`
}

// QualityObjectiveSignal identifies the bounded, deterministic observation
// that started an objective. It deliberately stores references and metadata,
// not copied tool output or artifacts.
type QualityObjectiveSignal struct {
	Kind         string    `json:"kind"`
	ID           string    `json:"id"`
	Fingerprint  string    `json:"fingerprint,omitempty"`
	Head         string    `json:"head,omitempty"`
	ConfigDigest string    `json:"configDigest,omitempty"`
	ObservedAt   time.Time `json:"observedAt,omitempty"`
}

// QualityObjectiveDecision records the human disposition for an objective.
// The target objective state is derived from Disposition; callers cannot set
// an arbitrary state through this model.
type QualityObjectiveDecision struct {
	Disposition    string    `json:"disposition"`
	Action         string    `json:"action,omitempty"`
	Reason         string    `json:"reason,omitempty"`
	Actor          string    `json:"actor"`
	MinimumPercent float64   `json:"minimumPercent,omitempty"`
	DecidedAt      time.Time `json:"decidedAt"`
}

// QualityObjectiveRevalidation points at a later deterministic observation.
// It intentionally has no artifact field; evidence remains in its source
// record and is linked by SourceKind/SourceID.
type QualityObjectiveRevalidation struct {
	SourceKind   string    `json:"sourceKind"`
	SourceID     string    `json:"sourceId"`
	Outcome      string    `json:"outcome"`
	ReasonCode   string    `json:"reasonCode"`
	Head         string    `json:"head"`
	ConfigDigest string    `json:"configDigest,omitempty"`
	CheckedAt    time.Time `json:"checkedAt"`
	Sequence     uint64    `json:"sequence,omitempty"`
}

// QualityObjectiveFreshnessInput is supplied by application code when it
// needs to decide whether the latest revalidation is still current. A zero
// MaxAge means that no age limit is imposed; head and AsOf are always needed.
type QualityObjectiveFreshnessInput struct {
	Head         string
	ConfigDigest string
	AsOf         time.Time
	MaxAge       time.Duration
}

func (o QualityObjective) Validate() error {
	if err := assuranceResource(o.TypeMeta, QualityObjectiveKind, o.Metadata); err != nil {
		return err
	}
	if err := validateAssuranceScope(o.Spec.ProjectID, o.Spec.RepositoryID, o.Spec.WorktreeID, o.Spec.Head); err != nil {
		return err
	}
	if strings.TrimSpace(o.Spec.Owner) == "" || strings.TrimSpace(o.Spec.Title) == "" {
		return errors.New("quality objective requires an owner and title")
	}
	if !validQualityObjectiveState(o.Spec.State) || o.Spec.Revision < 1 || o.Spec.CreatedAt.IsZero() || o.Spec.UpdatedAt.IsZero() {
		return errors.New("quality objective state, revision, and timestamps are required")
	}
	if o.Spec.UpdatedAt.Before(o.Spec.CreatedAt) {
		return errors.New("quality objective update cannot precede creation")
	}
	for _, id := range append(append(append(append([]string{}, o.Spec.FindingIDs...), o.Spec.RunIDs...), o.Spec.ProposalIDs...), o.Spec.SessionID, o.Spec.BaselineID, o.Spec.CampaignID) {
		if id != "" && !validIdentifier(id) {
			return errors.New("quality objective links must use valid identifiers")
		}
	}
	if err := validateUniqueIdentifiers(o.Spec.FindingIDs); err != nil {
		return err
	}
	if err := validateUniqueIdentifiers(o.Spec.RunIDs); err != nil {
		return err
	}
	if err := validateUniqueIdentifiers(o.Spec.ProposalIDs); err != nil {
		return err
	}
	if o.Spec.PrimarySignal != nil {
		if err := o.Spec.PrimarySignal.Validate(); err != nil {
			return err
		}
	}
	if o.Spec.Decision != nil {
		if err := o.Spec.Decision.Validate(); err != nil {
			return err
		}
	}
	if len(o.Spec.Revalidations) > QualityObjectiveMaxRevalidations {
		return errors.New("quality objective revalidation history exceeds the limit")
	}
	for _, revalidation := range o.Spec.Revalidations {
		if err := revalidation.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// CanTransition validates the explicit lifecycle without mutating the
// aggregate. Persistence callers can then apply the next revision through the
// existing assurance-object revision CAS.
func (o QualityObjective) CanTransition(next string) error {
	if err := o.Validate(); err != nil {
		return err
	}
	if !validQualityObjectiveState(next) {
		return errors.New("quality objective target state is invalid")
	}
	if next == o.Spec.State {
		return nil
	}
	allowed := map[string][]string{
		QualityObjectiveStateDraft: {
			QualityObjectiveStateBaselinePending,
			QualityObjectiveStateReady,
			QualityObjectiveStateBlocked,
			QualityObjectiveStateRejected,
		},
		QualityObjectiveStateBaselinePending: {
			QualityObjectiveStateReady,
			QualityObjectiveStateBlocked,
			QualityObjectiveStateStale,
			QualityObjectiveStateRejected,
		},
		QualityObjectiveStateReady: {
			QualityObjectiveStateRunning,
			QualityObjectiveStateReview,
			QualityObjectiveStateBlocked,
			QualityObjectiveStateStale,
			QualityObjectiveStateRejected,
		},
		QualityObjectiveStateRunning: {
			QualityObjectiveStateReview,
			QualityObjectiveStateBlocked,
			QualityObjectiveStateStale,
		},
		QualityObjectiveStateReview: {
			QualityObjectiveStateAdopted,
			QualityObjectiveStateRejected,
			QualityObjectiveStateRunning,
			QualityObjectiveStateStale,
		},
		QualityObjectiveStateAdopted: {},
		QualityObjectiveStateStale: {
			QualityObjectiveStateBaselinePending,
			QualityObjectiveStateReady,
			QualityObjectiveStateRejected,
		},
		QualityObjectiveStateBlocked: {
			QualityObjectiveStateDraft,
			QualityObjectiveStateBaselinePending,
			QualityObjectiveStateReady,
			QualityObjectiveStateRejected,
		},
	}
	for _, candidate := range allowed[o.Spec.State] {
		if candidate == next {
			return nil
		}
	}
	return errors.New("quality objective lifecycle transition is not allowed")
}

// ApplyDecision records a human disposition and derives the corresponding
// lifecycle state. It is the only domain method for applying a disposition.
func (o *QualityObjective) ApplyDecision(decision QualityObjectiveDecision) error {
	if o == nil {
		return errors.New("quality objective is required")
	}
	if err := o.Validate(); err != nil {
		return err
	}
	if qualityObjectiveStateIsTerminal(o.Spec.State) {
		return errors.New("quality objective terminal state: decision is not allowed")
	}
	if err := decision.Validate(); err != nil {
		return err
	}
	if decision.DecidedAt.Before(o.Spec.CreatedAt) {
		return errors.New("quality objective decision cannot precede creation")
	}

	target := qualityObjectiveStateForDisposition(decision.Disposition)
	if err := o.CanTransition(target); err != nil {
		return err
	}
	o.Spec.Decision = &decision
	o.Spec.State = target
	o.touchQualityObjective(decision.DecidedAt)
	return nil
}

// RecordRevalidation stores only a bounded reference to a later observation.
// An improved observation moves an active objective to review so adoption can
// be explicitly confirmed; it never silently marks an objective stale.
func (o *QualityObjective) RecordRevalidation(revalidation QualityObjectiveRevalidation) error {
	if o == nil {
		return errors.New("quality objective is required")
	}
	if err := o.Validate(); err != nil {
		return err
	}
	if qualityObjectiveStateIsTerminal(o.Spec.State) {
		return errors.New("quality objective terminal state: revalidation is not allowed")
	}
	if err := revalidation.Validate(); err != nil {
		return err
	}
	if o.Spec.PrimarySignal != nil && revalidation.SourceKind != o.Spec.PrimarySignal.Kind {
		return errors.New("quality objective revalidation source kind does not match the primary signal")
	}
	if revalidation.CheckedAt.Before(o.Spec.CreatedAt) {
		return errors.New("quality objective revalidation cannot precede creation")
	}

	history := append([]QualityObjectiveRevalidation{}, o.Spec.Revalidations...)
	nextSequence, err := nextQualityObjectiveRevalidationSequence(history)
	if err != nil {
		return err
	}
	revalidation.Sequence = nextSequence
	history = append(history, revalidation)
	sort.SliceStable(history, func(i, j int) bool {
		if history[i].CheckedAt.Equal(history[j].CheckedAt) {
			return history[i].Sequence > history[j].Sequence
		}
		return history[i].CheckedAt.After(history[j].CheckedAt)
	})
	if len(history) > QualityObjectiveMaxRevalidations {
		history = history[:QualityObjectiveMaxRevalidations]
	}
	o.Spec.Revalidations = history
	if revalidation.Outcome == QualityObjectiveRevalidationImproved &&
		(o.Spec.State == QualityObjectiveStateReady || o.Spec.State == QualityObjectiveStateRunning) {
		if err := o.CanTransition(QualityObjectiveStateReview); err != nil {
			return err
		}
		o.Spec.State = QualityObjectiveStateReview
	}
	o.touchQualityObjective(revalidation.CheckedAt)
	return nil
}

// LatestRevalidation returns the newest recorded reference without exposing
// the aggregate's backing slice.
func (o QualityObjective) LatestRevalidation() (QualityObjectiveRevalidation, bool) {
	if len(o.Spec.Revalidations) == 0 {
		return QualityObjectiveRevalidation{}, false
	}
	latest := o.Spec.Revalidations[0]
	for _, candidate := range o.Spec.Revalidations[1:] {
		if qualityObjectiveRevalidationIsNewer(candidate, latest) {
			latest = candidate
		}
	}
	return latest, true
}

// ValidateLatestRevalidationFresh lets a read model present stale evidence
// without mutating the stored objective.
func (o QualityObjective) ValidateLatestRevalidationFresh(input QualityObjectiveFreshnessInput) error {
	if err := o.Validate(); err != nil {
		return err
	}
	if err := input.Validate(); err != nil {
		return err
	}
	latest, ok := o.LatestRevalidation()
	if !ok {
		return errors.New("quality objective has no revalidation")
	}
	if latest.Outcome != QualityObjectiveRevalidationImproved {
		return errors.New("latest quality objective revalidation did not improve")
	}
	if latest.SourceKind == QualityObjectiveSignalKindFinding {
		return errors.New("finding revalidation cannot prove the current worktree and head")
	}
	if latest.CheckedAt.After(input.AsOf) {
		return errors.New("quality objective revalidation is newer than the freshness check")
	}
	if latest.Head != input.Head {
		return errors.New("quality objective revalidation head is stale")
	}
	if latest.SourceKind == QualityObjectiveSignalKindGoCoverage {
		if strings.TrimSpace(latest.ConfigDigest) == "" || strings.TrimSpace(input.ConfigDigest) == "" {
			return errors.New("go coverage revalidation requires a config digest for freshness")
		}
		if latest.ConfigDigest != input.ConfigDigest {
			return errors.New("quality objective revalidation configuration is stale")
		}
	}
	if input.MaxAge > 0 && input.AsOf.Sub(latest.CheckedAt) > input.MaxAge {
		return errors.New("quality objective revalidation is too old")
	}
	return nil
}

// LatestRevalidationIsFresh is a convenient read-only predicate for UI/read
// model code. It never changes the objective's state.
func (o QualityObjective) LatestRevalidationIsFresh(input QualityObjectiveFreshnessInput) bool {
	return o.ValidateLatestRevalidationFresh(input) == nil
}

// ConfirmAdoption completes the lifecycle only after a fresh improved latest
// revalidation. Freshness is evaluated against caller-supplied current data.
func (o *QualityObjective) ConfirmAdoption(input QualityObjectiveFreshnessInput) error {
	if o == nil {
		return errors.New("quality objective is required")
	}
	if err := o.Validate(); err != nil {
		return err
	}
	if qualityObjectiveStateIsTerminal(o.Spec.State) {
		return errors.New("quality objective terminal state: adoption confirmation is not allowed")
	}
	if err := o.ValidateLatestRevalidationFresh(input); err != nil {
		return err
	}
	if err := o.CanTransition(QualityObjectiveStateAdopted); err != nil {
		return err
	}
	o.Spec.State = QualityObjectiveStateAdopted
	o.touchQualityObjective(input.AsOf)
	return nil
}

// Clone returns an independent aggregate, including all newly added nested
// values and slices.
func (o QualityObjective) Clone() QualityObjective {
	clone := o
	clone.Spec.FindingIDs = cloneQualityObjectiveStrings(o.Spec.FindingIDs)
	clone.Spec.RunIDs = cloneQualityObjectiveStrings(o.Spec.RunIDs)
	clone.Spec.ProposalIDs = cloneQualityObjectiveStrings(o.Spec.ProposalIDs)
	if o.Spec.PrimarySignal != nil {
		signal := *o.Spec.PrimarySignal
		clone.Spec.PrimarySignal = &signal
	}
	if o.Spec.Decision != nil {
		decision := *o.Spec.Decision
		clone.Spec.Decision = &decision
	}
	if o.Spec.Revalidations != nil {
		clone.Spec.Revalidations = append([]QualityObjectiveRevalidation{}, o.Spec.Revalidations...)
	}
	return clone
}

func (s QualityObjectiveSignal) Validate() error {
	if !validQualityObjectiveSignalKind(s.Kind) || !validIdentifier(s.ID) {
		return errors.New("quality objective signal kind and id are invalid")
	}
	if s.Fingerprint != "" && !validBoundedText(s.Fingerprint) {
		return errors.New("quality objective signal fingerprint is invalid")
	}
	if s.Head != "" && !validBoundedText(s.Head) {
		return errors.New("quality objective signal head is invalid")
	}
	if s.ConfigDigest != "" && !validBoundedText(s.ConfigDigest) {
		return errors.New("quality objective signal config digest is invalid")
	}
	return nil
}

func (d QualityObjectiveDecision) Validate() error {
	if d.DecidedAt.IsZero() || !validBoundedText(d.Actor) {
		return errors.New("quality objective decision actor and timestamp are required")
	}
	if d.MinimumPercent < 0 || d.MinimumPercent > 100 {
		return errors.New("quality objective minimum percent must be between 0 and 100")
	}
	switch d.Disposition {
	case QualityObjectiveDispositionPursue:
		if !validBoundedText(d.Action) {
			return errors.New("pursue decision requires an action")
		}
	case QualityObjectiveDispositionDefer, QualityObjectiveDispositionDismiss:
		if !validBoundedText(d.Reason) {
			return errors.New("defer and dismiss decisions require a reason")
		}
	default:
		return errors.New("quality objective decision disposition is invalid")
	}
	if d.Action != "" && !validBoundedText(d.Action) {
		return errors.New("quality objective decision action is invalid")
	}
	if d.Reason != "" && !validBoundedText(d.Reason) {
		return errors.New("quality objective decision reason is invalid")
	}
	return nil
}

func (r QualityObjectiveRevalidation) Validate() error {
	if !validQualityObjectiveSignalKind(r.SourceKind) || !validIdentifier(r.SourceID) || r.CheckedAt.IsZero() {
		return errors.New("quality objective revalidation source and timestamp are invalid")
	}
	switch r.Outcome {
	case QualityObjectiveRevalidationImproved, QualityObjectiveRevalidationNotImproved, QualityObjectiveRevalidationInconclusive:
	default:
		return errors.New("quality objective revalidation outcome is invalid")
	}
	if !validBoundedText(r.ReasonCode) || !validBoundedText(r.Head) {
		return errors.New("quality objective revalidation reason code and head are required")
	}
	if r.ConfigDigest != "" && !validBoundedText(r.ConfigDigest) {
		return errors.New("quality objective revalidation config digest is invalid")
	}
	if r.SourceKind == QualityObjectiveSignalKindGoCoverage && r.ConfigDigest == "" {
		return errors.New("go coverage revalidation requires a config digest")
	}
	return nil
}

func (input QualityObjectiveFreshnessInput) Validate() error {
	if !validBoundedText(input.Head) || input.AsOf.IsZero() || input.MaxAge < 0 {
		return errors.New("quality objective freshness input is invalid")
	}
	if input.ConfigDigest != "" && !validBoundedText(input.ConfigDigest) {
		return errors.New("quality objective freshness config digest is invalid")
	}
	return nil
}

func validQualityObjectiveSignalKind(value string) bool {
	return value == QualityObjectiveSignalKindFinding || value == QualityObjectiveSignalKindGoCoverage
}

func qualityObjectiveStateForDisposition(disposition string) string {
	switch disposition {
	case QualityObjectiveDispositionPursue:
		return QualityObjectiveStateReady
	case QualityObjectiveDispositionDefer:
		return QualityObjectiveStateBlocked
	case QualityObjectiveDispositionDismiss:
		return QualityObjectiveStateRejected
	default:
		return ""
	}
}

func (o *QualityObjective) touchQualityObjective(at time.Time) {
	if at.After(o.Spec.UpdatedAt) {
		o.Spec.UpdatedAt = at
	}
	if o.Spec.Revision < int(^uint(0)>>1) {
		o.Spec.Revision++
	}
}

func cloneQualityObjectiveStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}

func nextQualityObjectiveRevalidationSequence(history []QualityObjectiveRevalidation) (uint64, error) {
	var highest uint64
	for _, revalidation := range history {
		if revalidation.Sequence > highest {
			highest = revalidation.Sequence
		}
	}
	if highest == ^uint64(0) {
		return 0, errors.New("quality objective revalidation sequence exhausted")
	}
	return highest + 1, nil
}

func qualityObjectiveRevalidationIsNewer(candidate, current QualityObjectiveRevalidation) bool {
	if candidate.CheckedAt.After(current.CheckedAt) {
		return true
	}
	if candidate.CheckedAt.Before(current.CheckedAt) {
		return false
	}
	if candidate.Sequence != current.Sequence {
		return candidate.Sequence > current.Sequence
	}
	return true
}

func qualityObjectiveStateIsTerminal(state string) bool {
	return state == QualityObjectiveStateAdopted || state == QualityObjectiveStateRejected
}

func validQualityObjectiveState(value string) bool {
	switch value {
	case QualityObjectiveStateDraft,
		QualityObjectiveStateBaselinePending,
		QualityObjectiveStateReady,
		QualityObjectiveStateRunning,
		QualityObjectiveStateReview,
		QualityObjectiveStateAdopted,
		QualityObjectiveStateRejected,
		QualityObjectiveStateStale,
		QualityObjectiveStateBlocked:
		return true
	}
	return false
}

func validateUniqueIdentifiers(values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			return errors.New("quality objective links cannot be empty")
		}
		if _, ok := seen[value]; ok {
			return errors.New("quality objective links cannot contain duplicates")
		}
		seen[value] = struct{}{}
	}
	return nil
}
