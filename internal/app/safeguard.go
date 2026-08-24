package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

type failureOccurrence struct {
	Category     string
	SourceType   string
	Status       string
	ExitCode     int
	ProjectID    string
	RepositoryID string
	WorktreeID   string
	EvidenceRef  string
}

func (a *App) Safeguards(ctx context.Context, limit int) ([]domain.SafeguardRule, error) {
	if limit < 0 || limit > 1000 {
		return nil, contract.InvalidInput("safeguard limit must be between 0 and 1000")
	}
	return a.store.ListSafeguardRules(ctx, limit)
}

func (a *App) Safeguard(ctx context.Context, id string) (domain.SafeguardRule, error) {
	rule, err := a.store.GetSafeguardRule(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SafeguardRule{}, contract.NotFound("safeguard not found")
	}
	return rule, err
}

func (a *App) ReviewSafeguard(ctx context.Context, id, owner string) (domain.SafeguardRule, error) {
	return a.transitionSafeguard(ctx, id, domain.SafeguardShadow, owner)
}

func (a *App) ActivateSafeguard(ctx context.Context, id string) (domain.SafeguardRule, error) {
	return a.transitionSafeguard(ctx, id, domain.SafeguardActive, "local-user")
}

func (a *App) RollbackSafeguard(ctx context.Context, id string) (domain.SafeguardRule, error) {
	return a.transitionSafeguard(ctx, id, domain.SafeguardShadow, "")
}

func (a *App) RetireSafeguard(ctx context.Context, id string) (domain.SafeguardRule, error) {
	return a.transitionSafeguard(ctx, id, domain.SafeguardRetired, "")
}

func (a *App) transitionSafeguard(ctx context.Context, id string, state domain.SafeguardRuleState, owner string) (domain.SafeguardRule, error) {
	a.safeguardMu.Lock()
	defer a.safeguardMu.Unlock()
	return a.updateSafeguardRule(ctx, id, func(rule *domain.SafeguardRule) (bool, error) {
		if err := rule.Transition(state, owner, time.Now().UTC()); err != nil {
			return false, contract.InvalidInput(err.Error())
		}
		return true, nil
	})
}

func (a *App) FeedbackSafeguard(ctx context.Context, id string, feedback domain.SafeguardFeedback) (domain.SafeguardRule, error) {
	a.safeguardMu.Lock()
	defer a.safeguardMu.Unlock()
	return a.updateSafeguardRule(ctx, id, func(rule *domain.SafeguardRule) (bool, error) {
		if err := rule.RecordFeedback(feedback, time.Now().UTC()); err != nil {
			return false, contract.InvalidInput(err.Error())
		}
		return true, nil
	})
}

func (a *App) recordFailureOccurrence(ctx context.Context, occurrence failureOccurrence) error {
	a.safeguardMu.Lock()
	defer a.safeguardMu.Unlock()
	fingerprint, err := occurrence.fingerprint()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	failure := domain.FailureFingerprint{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.FailureFingerprintKind},
		Metadata: domain.ObjectMeta{ID: "failure-" + strings.TrimPrefix(fingerprint, "sha256:")[:48], Name: occurrence.Category + " failure"},
		Spec: domain.FailureFingerprintSpec{
			Fingerprint: fingerprint, Category: occurrence.Category, FirstSeen: now, LastSeen: now,
			OccurrenceCount: 1, EvidenceRefs: appendEvidenceRef(nil, occurrence.EvidenceRef),
		},
	}
	failure, err = a.store.RecordFailureFingerprint(ctx, failure)
	if err != nil {
		return err
	}
	if failure.Spec.OccurrenceCount >= 3 {
		rule := domain.SafeguardRule{
			TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.SafeguardRuleKind},
			Metadata: domain.ObjectMeta{ID: "safeguard-" + strings.TrimPrefix(fingerprint, "sha256:")[:48], Name: occurrence.Category + " safeguard"},
			Spec: domain.SafeguardRuleSpec{
				Fingerprint: fingerprint, Category: occurrence.Category,
				ProjectID: occurrence.ProjectID, RepositoryID: occurrence.RepositoryID, WorktreeID: occurrence.WorktreeID,
				State: domain.SafeguardProposal, Revision: 1, OccurrenceCount: failure.Spec.OccurrenceCount,
				FirstSeen: failure.Spec.FirstSeen, LastSeen: failure.Spec.LastSeen, CreatedAt: now, UpdatedAt: now,
			},
		}
		if _, err := a.store.CreateSafeguardRule(ctx, rule); err != nil {
			return err
		}
	}
	rules, err := a.store.ListSafeguardRules(ctx, 0)
	if err != nil {
		return err
	}
	for _, listed := range rules {
		if listed.Spec.Category != occurrence.Category || !sameSafeguardScope(listed, occurrence) {
			continue
		}
		_, err := a.updateSafeguardRule(ctx, listed.Metadata.ID, func(rule *domain.SafeguardRule) (bool, error) {
			changed := false
			exact := rule.Spec.Fingerprint == fingerprint
			if exact && failure.Spec.OccurrenceCount > rule.Spec.OccurrenceCount {
				rule.Spec.OccurrenceCount = failure.Spec.OccurrenceCount
				rule.Spec.FirstSeen = failure.Spec.FirstSeen
				rule.Spec.LastSeen = failure.Spec.LastSeen
				rule.Spec.UpdatedAt = time.Now().UTC()
				changed = true
			}
			if rule.Spec.State == domain.SafeguardShadow || rule.Spec.State == domain.SafeguardActive {
				if err := rule.RecordEvaluation(exact, time.Now().UTC()); err != nil {
					return false, err
				}
				changed = true
			}
			return changed, nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *App) updateSafeguardRule(ctx context.Context, id string, mutate func(*domain.SafeguardRule) (bool, error)) (domain.SafeguardRule, error) {
	for range 20 {
		rule, err := a.Safeguard(ctx, id)
		if err != nil {
			return domain.SafeguardRule{}, err
		}
		previousRevision := rule.Spec.Revision
		changed, err := mutate(&rule)
		if err != nil {
			return domain.SafeguardRule{}, err
		}
		if !changed {
			return rule, nil
		}
		rule.Spec.Revision = previousRevision + 1
		updated, err := a.store.UpdateSafeguardRule(ctx, rule, previousRevision)
		if err != nil {
			return domain.SafeguardRule{}, err
		}
		if updated {
			return rule, nil
		}
	}
	return domain.SafeguardRule{}, contract.Conflict("safeguard changed repeatedly; retry the operation")
}

func sameSafeguardScope(rule domain.SafeguardRule, occurrence failureOccurrence) bool {
	return rule.Spec.ProjectID == occurrence.ProjectID && rule.Spec.RepositoryID == occurrence.RepositoryID && rule.Spec.WorktreeID == occurrence.WorktreeID
}

func (o failureOccurrence) fingerprint() (string, error) {
	if strings.TrimSpace(o.Category) == "" || strings.TrimSpace(o.SourceType) == "" || strings.TrimSpace(o.Status) == "" {
		return "", errors.New("failure occurrence requires category, source type, and status")
	}
	invalidScope := o.ProjectID == "" || o.RepositoryID == "" || safeID.MatchString(o.ProjectID) || safeID.MatchString(o.RepositoryID)
	if o.WorktreeID != "" && safeID.MatchString(o.WorktreeID) {
		invalidScope = true
	}
	if invalidScope {
		return "", errors.New("failure occurrence requires a valid project and repository scope")
	}
	signature := strings.Join([]string{o.Category, o.SourceType, o.Status, fmt.Sprintf("%d", o.ExitCode), o.ProjectID, o.RepositoryID, o.WorktreeID}, "\x00")
	sum := sha256.Sum256([]byte(signature))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func appendEvidenceRef(refs []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return append([]string(nil), refs...)
	}
	for _, item := range refs {
		if item == value {
			return append([]string(nil), refs...)
		}
	}
	updated := append(append([]string(nil), refs...), value)
	if len(updated) > 20 {
		updated = updated[len(updated)-20:]
	}
	return updated
}
