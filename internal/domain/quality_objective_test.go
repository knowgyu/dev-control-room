package domain

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func validQualityObjectiveFixture(state string) QualityObjective {
	now := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	return QualityObjective{
		TypeMeta: TypeMeta{APIVersion: APIVersion, Kind: QualityObjectiveKind},
		Metadata: ObjectMeta{ID: "objective-1", Name: "Improve checks"},
		Spec: QualityObjectiveSpec{
			ProjectID:    "project-1",
			RepositoryID: "repo-1",
			WorktreeID:   "primary",
			Head:         "abc123",
			Owner:        "knowgyu",
			Title:        "Improve checks",
			State:        state,
			Revision:     1,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
}

func TestQualityObjectiveValidateLifecycleStates(t *testing.T) {
	states := []string{
		QualityObjectiveStateDraft,
		QualityObjectiveStateBaselinePending,
		QualityObjectiveStateReady,
		QualityObjectiveStateRunning,
		QualityObjectiveStateReview,
		QualityObjectiveStateAdopted,
		QualityObjectiveStateRejected,
		QualityObjectiveStateStale,
		QualityObjectiveStateBlocked,
	}
	for _, state := range states {
		t.Run(state, func(t *testing.T) {
			if err := validQualityObjectiveFixture(state).Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestQualityObjectiveValidateRejectsInvalidLinksAndState(t *testing.T) {
	testCases := []struct {
		name     string
		mutate   func(*QualityObjective)
		contains string
	}{
		{
			name: "invalid state",
			mutate: func(item *QualityObjective) {
				item.Spec.State = "unknown"
			},
			contains: "state",
		},
		{
			name: "duplicate finding",
			mutate: func(item *QualityObjective) {
				item.Spec.FindingIDs = []string{"finding-1", "finding-1"}
			},
			contains: "duplicates",
		},
		{
			name: "invalid linked identifier",
			mutate: func(item *QualityObjective) {
				item.Spec.SessionID = "not valid"
			},
			contains: "identifiers",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			item := validQualityObjectiveFixture(QualityObjectiveStateDraft)
			testCase.mutate(&item)
			err := item.Validate()
			if err == nil || !strings.Contains(err.Error(), testCase.contains) {
				t.Fatalf("Validate() error = %v, want %q", err, testCase.contains)
			}
		})
	}
}

func TestQualityObjectiveCanTransition(t *testing.T) {
	testCases := []struct {
		name    string
		current string
		next    string
		wantErr bool
	}{
		{name: "draft to baseline", current: QualityObjectiveStateDraft, next: QualityObjectiveStateBaselinePending},
		{name: "review to adopted", current: QualityObjectiveStateReview, next: QualityObjectiveStateAdopted},
		{name: "stale to ready", current: QualityObjectiveStateStale, next: QualityObjectiveStateReady},
		{name: "blocked to ready", current: QualityObjectiveStateBlocked, next: QualityObjectiveStateReady},
		{name: "draft cannot run", current: QualityObjectiveStateDraft, next: QualityObjectiveStateRunning, wantErr: true},
		{name: "adopted cannot return to draft", current: QualityObjectiveStateAdopted, next: QualityObjectiveStateDraft, wantErr: true},
		{name: "adopted cannot become stale", current: QualityObjectiveStateAdopted, next: QualityObjectiveStateStale, wantErr: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := validQualityObjectiveFixture(testCase.current).CanTransition(testCase.next)
			if (err != nil) != testCase.wantErr {
				t.Fatalf("CanTransition() error = %v, wantErr=%v", err, testCase.wantErr)
			}
		})
	}
}

func validQualityObjectiveDecision(disposition string) QualityObjectiveDecision {
	decision := QualityObjectiveDecision{
		Disposition: disposition,
		Actor:       "knowgyu",
		DecidedAt:   time.Date(2026, 8, 30, 1, 0, 0, 0, time.UTC),
	}
	if disposition == QualityObjectiveDispositionPursue {
		decision.Action = "add boundary tests"
	} else {
		decision.Reason = "handled in a later cycle"
	}
	return decision
}

func validQualityObjectiveRevalidation(outcome string, checkedAt time.Time) QualityObjectiveRevalidation {
	return QualityObjectiveRevalidation{
		SourceKind: QualityObjectiveSignalKindFinding,
		SourceID:   "finding-2",
		Outcome:    outcome,
		ReasonCode: "finding_resolved",
		Head:       "abc123",
		CheckedAt:  checkedAt,
	}
}

func validQualityObjectiveCoverageRevalidation(outcome string, checkedAt time.Time) QualityObjectiveRevalidation {
	return QualityObjectiveRevalidation{
		SourceKind:   QualityObjectiveSignalKindGoCoverage,
		SourceID:     "run-2",
		Outcome:      outcome,
		ReasonCode:   "coverage_collected",
		Head:         "abc123",
		ConfigDigest: "sha256:config",
		CheckedAt:    checkedAt,
	}
}

func TestQualityObjectiveApplyDecisionMapsDispositions(t *testing.T) {
	testCases := []struct {
		name         string
		disposition  string
		wantState    string
		wantDecision bool
	}{
		{name: "pursue", disposition: QualityObjectiveDispositionPursue, wantState: QualityObjectiveStateReady, wantDecision: true},
		{name: "defer", disposition: QualityObjectiveDispositionDefer, wantState: QualityObjectiveStateBlocked, wantDecision: true},
		{name: "dismiss", disposition: QualityObjectiveDispositionDismiss, wantState: QualityObjectiveStateRejected, wantDecision: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			item := validQualityObjectiveFixture(QualityObjectiveStateDraft)
			if err := item.ApplyDecision(validQualityObjectiveDecision(testCase.disposition)); err != nil {
				t.Fatalf("ApplyDecision() error = %v", err)
			}
			if item.Spec.State != testCase.wantState || (item.Spec.Decision != nil) != testCase.wantDecision {
				t.Fatalf("decision result = state %q, decision=%v", item.Spec.State, item.Spec.Decision)
			}
			if item.Spec.Revision != 2 {
				t.Fatalf("revision = %d, want 2", item.Spec.Revision)
			}
		})
	}
}

func TestQualityObjectiveApplyDecisionResumesDeferredObjective(t *testing.T) {
	item := validQualityObjectiveFixture(QualityObjectiveStateBlocked)
	deferred := validQualityObjectiveDecision(QualityObjectiveDispositionDefer)
	item.Spec.Decision = &deferred

	if err := item.ApplyDecision(validQualityObjectiveDecision(QualityObjectiveDispositionPursue)); err != nil {
		t.Fatalf("ApplyDecision(pursue) error = %v", err)
	}
	if item.Spec.State != QualityObjectiveStateReady {
		t.Fatalf("resumed state = %q, want %q", item.Spec.State, QualityObjectiveStateReady)
	}
	if item.Spec.Decision == nil || item.Spec.Decision.Disposition != QualityObjectiveDispositionPursue {
		t.Fatalf("resumed decision = %#v, want pursue decision", item.Spec.Decision)
	}
	if item.Spec.Revision != 2 {
		t.Fatalf("revision = %d, want 2", item.Spec.Revision)
	}
}

func TestQualityObjectiveApplyDecisionRejectsInvalidInputsAndTransitions(t *testing.T) {
	testCases := []struct {
		name     string
		item     QualityObjective
		decision QualityObjectiveDecision
		wantErr  string
	}{
		{name: "unknown disposition", item: validQualityObjectiveFixture(QualityObjectiveStateDraft), decision: validQualityObjectiveDecision("unknown"), wantErr: "disposition"},
		{name: "pursue without action", item: validQualityObjectiveFixture(QualityObjectiveStateDraft), decision: QualityObjectiveDecision{Disposition: QualityObjectiveDispositionPursue, Actor: "knowgyu", DecidedAt: time.Date(2026, 8, 30, 1, 0, 0, 0, time.UTC)}, wantErr: "action"},
		{name: "defer without reason", item: validQualityObjectiveFixture(QualityObjectiveStateDraft), decision: QualityObjectiveDecision{Disposition: QualityObjectiveDispositionDefer, Actor: "knowgyu", DecidedAt: time.Date(2026, 8, 30, 1, 0, 0, 0, time.UTC)}, wantErr: "reason"},
		{name: "dismiss without actor", item: validQualityObjectiveFixture(QualityObjectiveStateDraft), decision: QualityObjectiveDecision{Disposition: QualityObjectiveDispositionDismiss, Reason: "not needed", DecidedAt: time.Date(2026, 8, 30, 1, 0, 0, 0, time.UTC)}, wantErr: "actor"},
		{name: "adopted cannot be pursued", item: validQualityObjectiveFixture(QualityObjectiveStateAdopted), decision: validQualityObjectiveDecision(QualityObjectiveDispositionPursue), wantErr: "not allowed"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.item.ApplyDecision(testCase.decision); err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("ApplyDecision() error = %v, want %q", err, testCase.wantErr)
			}
		})
	}
}

func TestQualityObjectiveRevalidationValidation(t *testing.T) {
	checkedAt := time.Date(2026, 8, 30, 1, 0, 0, 0, time.UTC)
	testCases := []struct {
		name    string
		mutate  func(*QualityObjectiveRevalidation)
		wantErr string
	}{
		{name: "invalid source kind", mutate: func(item *QualityObjectiveRevalidation) { item.SourceKind = "mutation" }, wantErr: "source"},
		{name: "invalid outcome", mutate: func(item *QualityObjectiveRevalidation) { item.Outcome = "stale" }, wantErr: "outcome"},
		{name: "missing source id", mutate: func(item *QualityObjectiveRevalidation) { item.SourceID = "" }, wantErr: "source"},
		{name: "missing reason code", mutate: func(item *QualityObjectiveRevalidation) { item.ReasonCode = "" }, wantErr: "reason code"},
		{name: "missing head", mutate: func(item *QualityObjectiveRevalidation) { item.Head = "" }, wantErr: "reason code and head"},
		{name: "coverage requires config", mutate: func(item *QualityObjectiveRevalidation) { item.SourceKind = QualityObjectiveSignalKindGoCoverage }, wantErr: "config digest"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			item := validQualityObjectiveRevalidation(QualityObjectiveRevalidationImproved, checkedAt)
			testCase.mutate(&item)
			if err := item.Validate(); err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("Validate() error = %v, want %q", err, testCase.wantErr)
			}
		})
	}
}

func TestQualityObjectiveRecordRevalidationCapsNewestAndMovesToReview(t *testing.T) {
	item := validQualityObjectiveFixture(QualityObjectiveStateReady)
	for i := 0; i < QualityObjectiveMaxRevalidations+1; i++ {
		checkedAt := item.Spec.CreatedAt.Add(time.Duration(i+1) * time.Minute)
		revalidation := validQualityObjectiveRevalidation(QualityObjectiveRevalidationNotImproved, checkedAt)
		revalidation.SourceID = "finding-" + string(rune('a'+i))
		if err := item.RecordRevalidation(revalidation); err != nil {
			t.Fatalf("RecordRevalidation(%d) error = %v", i, err)
		}
	}
	if len(item.Spec.Revalidations) != QualityObjectiveMaxRevalidations {
		t.Fatalf("revalidation count = %d, want %d", len(item.Spec.Revalidations), QualityObjectiveMaxRevalidations)
	}
	if item.Spec.Revalidations[0].CheckedAt != item.Spec.CreatedAt.Add(21*time.Minute) || item.Spec.Revalidations[19].CheckedAt != item.Spec.CreatedAt.Add(2*time.Minute) {
		t.Fatalf("revalidation history was not kept newest-first: first=%v last=%v", item.Spec.Revalidations[0].CheckedAt, item.Spec.Revalidations[19].CheckedAt)
	}
	if item.Spec.State != QualityObjectiveStateReady {
		t.Fatalf("state after non-improved revalidations = %q, want ready", item.Spec.State)
	}

	item = validQualityObjectiveFixture(QualityObjectiveStateReady)
	improved := validQualityObjectiveRevalidation(QualityObjectiveRevalidationImproved, item.Spec.CreatedAt.Add(time.Minute))
	if err := item.RecordRevalidation(improved); err != nil {
		t.Fatalf("RecordRevalidation(improved) error = %v", err)
	}
	if item.Spec.State != QualityObjectiveStateReview {
		t.Fatalf("state after improved revalidation = %q, want review", item.Spec.State)
	}
}

func TestQualityObjectiveLatestRevalidationUsesNewerInsertionOnTimestampTie(t *testing.T) {
	item := validQualityObjectiveFixture(QualityObjectiveStateReady)
	checkedAt := item.Spec.CreatedAt.Add(time.Hour)
	improved := validQualityObjectiveCoverageRevalidation(QualityObjectiveRevalidationImproved, checkedAt)
	notImproved := validQualityObjectiveCoverageRevalidation(QualityObjectiveRevalidationNotImproved, checkedAt)
	notImproved.SourceID = "run-3"

	if err := item.RecordRevalidation(improved); err != nil {
		t.Fatalf("RecordRevalidation(improved) error = %v", err)
	}
	if err := item.RecordRevalidation(notImproved); err != nil {
		t.Fatalf("RecordRevalidation(not improved) error = %v", err)
	}
	latest, ok := item.LatestRevalidation()
	if !ok || latest.Outcome != QualityObjectiveRevalidationNotImproved || latest.SourceID != "run-3" {
		t.Fatalf("latest revalidation = %#v, ok=%v, want newer not-improved result", latest, ok)
	}
	if item.Spec.Revalidations[0].Sequence <= item.Spec.Revalidations[1].Sequence {
		t.Fatalf("history sequence order = %#v, want newest first", item.Spec.Revalidations)
	}
	input := QualityObjectiveFreshnessInput{Head: item.Spec.Head, ConfigDigest: "sha256:config", AsOf: checkedAt.Add(time.Minute), MaxAge: time.Hour}
	if err := item.ConfirmAdoption(input); err == nil || !strings.Contains(err.Error(), "did not improve") {
		t.Fatalf("ConfirmAdoption() error = %v, want latest not-improved to block adoption", err)
	}
}

func TestQualityObjectiveConfirmAdoptionRequiresFreshImprovedLatest(t *testing.T) {
	base := validQualityObjectiveFixture(QualityObjectiveStateReady)
	checkedAt := base.Spec.CreatedAt.Add(time.Hour)
	input := QualityObjectiveFreshnessInput{Head: base.Spec.Head, ConfigDigest: "sha256:config", AsOf: checkedAt.Add(10 * time.Minute), MaxAge: time.Hour}

	testCases := []struct {
		name     string
		outcome  string
		coverage bool
		input    QualityObjectiveFreshnessInput
		wantErr  string
	}{
		{name: "no revalidation", input: input, wantErr: "no revalidation"},
		{name: "latest not improved", outcome: QualityObjectiveRevalidationNotImproved, input: input, wantErr: "did not improve"},
		{name: "finding cannot prove current worktree", outcome: QualityObjectiveRevalidationImproved, input: input, wantErr: "current worktree"},
		{name: "head changed", outcome: QualityObjectiveRevalidationImproved, coverage: true, input: QualityObjectiveFreshnessInput{Head: "different", ConfigDigest: input.ConfigDigest, AsOf: input.AsOf, MaxAge: input.MaxAge}, wantErr: "head"},
		{name: "too old", outcome: QualityObjectiveRevalidationImproved, coverage: true, input: QualityObjectiveFreshnessInput{Head: input.Head, ConfigDigest: input.ConfigDigest, AsOf: input.AsOf.Add(2 * time.Hour), MaxAge: time.Hour}, wantErr: "old"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			item := base.Clone()
			if testCase.outcome != "" {
				revalidation := validQualityObjectiveRevalidation(testCase.outcome, checkedAt)
				if testCase.coverage {
					revalidation = validQualityObjectiveCoverageRevalidation(testCase.outcome, checkedAt)
				}
				if err := item.RecordRevalidation(revalidation); err != nil {
					t.Fatalf("RecordRevalidation() error = %v", err)
				}
			}
			before := item.Clone()
			if err := item.ConfirmAdoption(testCase.input); err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("ConfirmAdoption() error = %v, want %q", err, testCase.wantErr)
			}
			if !reflect.DeepEqual(item, before) {
				t.Fatalf("failed confirmation mutated objective: before=%#v after=%#v", before, item)
			}
		})
	}

	item := base.Clone()
	if err := item.RecordRevalidation(validQualityObjectiveCoverageRevalidation(QualityObjectiveRevalidationImproved, checkedAt)); err != nil {
		t.Fatalf("RecordRevalidation() error = %v", err)
	}
	if err := item.ConfirmAdoption(input); err != nil {
		t.Fatalf("ConfirmAdoption() error = %v", err)
	}
	if item.Spec.State != QualityObjectiveStateAdopted {
		t.Fatalf("state = %q, want adopted", item.Spec.State)
	}
}

func TestQualityObjectiveCoverageFreshnessRequiresMatchingConfigDigest(t *testing.T) {
	base := validQualityObjectiveFixture(QualityObjectiveStateReady)
	checkedAt := base.Spec.CreatedAt.Add(time.Hour)
	if err := base.RecordRevalidation(validQualityObjectiveCoverageRevalidation(QualityObjectiveRevalidationImproved, checkedAt)); err != nil {
		t.Fatalf("RecordRevalidation() error = %v", err)
	}

	testCases := []struct {
		name         string
		configDigest string
		wantErr      string
	}{
		{name: "missing caller digest", wantErr: "requires a config digest"},
		{name: "mismatched caller digest", configDigest: "sha256:other", wantErr: "configuration is stale"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			item := base.Clone()
			input := QualityObjectiveFreshnessInput{Head: base.Spec.Head, ConfigDigest: testCase.configDigest, AsOf: checkedAt.Add(time.Minute), MaxAge: time.Hour}
			before := item.Clone()
			if err := item.ValidateLatestRevalidationFresh(input); err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("ValidateLatestRevalidationFresh() error = %v, want %q", err, testCase.wantErr)
			}
			if err := item.ConfirmAdoption(input); err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("ConfirmAdoption() error = %v, want %q", err, testCase.wantErr)
			}
			if !reflect.DeepEqual(item, before) {
				t.Fatalf("failed confirmation mutated objective: before=%#v after=%#v", before, item)
			}
		})
	}
}

func TestQualityObjectiveTerminalLifecycleCommandsDoNotMutate(t *testing.T) {
	testCases := []struct {
		name       string
		state      string
		decision   QualityObjectiveDecision
		wantReason string
	}{
		{name: "adopted decision", state: QualityObjectiveStateAdopted, decision: validQualityObjectiveDecision(QualityObjectiveDispositionPursue), wantReason: "decision is not allowed"},
		{name: "rejected same-state decision", state: QualityObjectiveStateRejected, decision: validQualityObjectiveDecision(QualityObjectiveDispositionDismiss), wantReason: "decision is not allowed"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			item := validQualityObjectiveFixture(testCase.state)
			before := item.Clone()
			if err := item.CanTransition(testCase.state); err != nil {
				t.Fatalf("CanTransition(same state) error = %v, want convenience no-op", err)
			}
			if err := item.ApplyDecision(testCase.decision); err == nil || !strings.Contains(err.Error(), testCase.wantReason) {
				t.Fatalf("ApplyDecision() error = %v, want %q", err, testCase.wantReason)
			}
			if !reflect.DeepEqual(item, before) {
				t.Fatalf("failed terminal decision mutated objective: before=%#v after=%#v", before, item)
			}
		})
	}

	for _, state := range []string{QualityObjectiveStateAdopted, QualityObjectiveStateRejected} {
		t.Run(state+" revalidation", func(t *testing.T) {
			item := validQualityObjectiveFixture(state)
			item.Spec.Revalidations = []QualityObjectiveRevalidation{validQualityObjectiveRevalidation(QualityObjectiveRevalidationNotImproved, item.Spec.CreatedAt.Add(time.Minute))}
			before := item.Clone()
			err := item.RecordRevalidation(validQualityObjectiveRevalidation(QualityObjectiveRevalidationImproved, item.Spec.CreatedAt.Add(time.Hour)))
			if err == nil || !strings.Contains(err.Error(), "revalidation is not allowed") {
				t.Fatalf("RecordRevalidation() error = %v, want terminal rejection", err)
			}
			if !reflect.DeepEqual(item, before) {
				t.Fatalf("failed terminal revalidation mutated objective: before=%#v after=%#v", before, item)
			}
		})
	}
}

func TestQualityObjectiveFreshnessInputValidation(t *testing.T) {
	testCases := []struct {
		name    string
		input   QualityObjectiveFreshnessInput
		wantErr bool
	}{
		{name: "valid", input: QualityObjectiveFreshnessInput{Head: "abc123", AsOf: time.Now()}},
		{name: "missing head", input: QualityObjectiveFreshnessInput{AsOf: time.Now()}, wantErr: true},
		{name: "missing as of", input: QualityObjectiveFreshnessInput{Head: "abc123"}, wantErr: true},
		{name: "negative age", input: QualityObjectiveFreshnessInput{Head: "abc123", AsOf: time.Now(), MaxAge: -time.Minute}, wantErr: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.input.Validate(); (err != nil) != testCase.wantErr {
				t.Fatalf("Validate() error = %v, wantErr=%v", err, testCase.wantErr)
			}
		})
	}
}

func TestQualityObjectiveCloneDeepCopiesNewFields(t *testing.T) {
	item := validQualityObjectiveFixture(QualityObjectiveStateReview)
	item.Spec.FindingIDs = []string{"finding-1"}
	item.Spec.PrimarySignal = &QualityObjectiveSignal{Kind: QualityObjectiveSignalKindFinding, ID: "finding-1", Fingerprint: "fingerprint"}
	decision := validQualityObjectiveDecision(QualityObjectiveDispositionPursue)
	item.Spec.Decision = &decision
	item.Spec.Revalidations = []QualityObjectiveRevalidation{validQualityObjectiveRevalidation(QualityObjectiveRevalidationImproved, item.Spec.CreatedAt.Add(time.Minute))}

	clone := item.Clone()
	clone.Spec.FindingIDs[0] = "finding-2"
	clone.Spec.PrimarySignal.ID = "finding-2"
	clone.Spec.Decision.Action = "different action"
	clone.Spec.Revalidations[0].SourceID = "finding-3"

	if item.Spec.FindingIDs[0] != "finding-1" || item.Spec.PrimarySignal.ID != "finding-1" || item.Spec.Decision.Action != "add boundary tests" || item.Spec.Revalidations[0].SourceID != "finding-2" {
		t.Fatal("Clone() did not deep-copy nested objective fields")
	}
}

func TestQualityObjectiveLegacyJSONLeavesOptionalLoopFieldsUnset(t *testing.T) {
	var item QualityObjective
	legacy := `{"apiVersion":"devroom/v1alpha1","kind":"QualityObjective","metadata":{"id":"objective-1","name":"Improve checks"},"spec":{"projectId":"project-1","repositoryId":"repo-1","worktreeId":"primary","head":"abc123","owner":"knowgyu","title":"Improve checks","state":"running","revision":2,"createdAt":"2026-08-30T00:00:00Z","updatedAt":"2026-08-30T00:00:00Z"}}`
	if err := json.Unmarshal([]byte(legacy), &item); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if err := item.Validate(); err != nil {
		t.Fatalf("legacy Validate() error = %v", err)
	}
	if item.Spec.PrimarySignal != nil || item.Spec.Decision != nil || item.Spec.Revalidations != nil {
		t.Fatal("legacy JSON populated optional decision-loop fields")
	}
}
