package app

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

type QualityHome struct {
	Objectives []domain.QualityObjective `json:"objectives"`
	Queue      []QualityHomeQueueItem    `json:"queue"`
	Summary    QualityHomeSummary        `json:"summary"`
}

type QualityHomeQueueItem struct {
	ID           string          `json:"id"`
	Kind         string          `json:"kind"`
	State        string          `json:"state"`
	Title        string          `json:"title"`
	Summary      string          `json:"summary"`
	ProjectID    string          `json:"projectId,omitempty"`
	RepositoryID string          `json:"repositoryId,omitempty"`
	WorktreeID   string          `json:"worktreeId,omitempty"`
	ReferenceID  string          `json:"referenceId"`
	Severity     domain.Severity `json:"severity,omitempty"`
	UpdatedAt    time.Time       `json:"updatedAt"`
	priority     int
}

type QualityHomeSummary struct {
	ActiveObjectives    int `json:"activeObjectives"`
	QueueItems          int `json:"queueItems"`
	FailedRuns          int `json:"failedRuns"`
	RunningRuns         int `json:"runningRuns"`
	StaleBaselines      int `json:"staleBaselines"`
	UnreviewedProposals int `json:"unreviewedProposals"`
	OpenFindings        int `json:"openFindings"`
}

func (a *App) QualityObjectives(ctx context.Context) ([]domain.QualityObjective, error) {
	return a.store.ListQualityObjectives(ctx)
}

func (a *App) QualityObjective(ctx context.Context, id string) (domain.QualityObjective, error) {
	item, err := a.store.GetQualityObjective(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.QualityObjective{}, contract.NotFound("quality objective not found")
	}
	return item, err
}

func (a *App) CreateQualityObjective(ctx context.Context, input QualityObjectiveInput) (domain.QualityObjective, error) {
	worktree, err := a.Worktree(ctx, input.ProjectID, input.RepositoryID, input.WorktreeID)
	if err != nil {
		return domain.QualityObjective{}, err
	}
	owner := strings.TrimSpace(input.Owner)
	title := strings.TrimSpace(input.Title)
	if owner == "" || title == "" {
		return domain.QualityObjective{}, contract.InvalidInput("quality objective owner and title are required")
	}
	now := time.Now().UTC()
	item := domain.QualityObjective{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.QualityObjectiveKind},
		Metadata: domain.ObjectMeta{
			ID:   assuranceID("objective", input.ProjectID, input.RepositoryID, input.WorktreeID, title, now),
			Name: title,
		},
		Spec: domain.QualityObjectiveSpec{
			ProjectID:     input.ProjectID,
			RepositoryID:  input.RepositoryID,
			WorktreeID:    input.WorktreeID,
			Head:          worktree.Spec.Head,
			Owner:         owner,
			Title:         title,
			Description:   strings.TrimSpace(input.Description),
			State:         domain.QualityObjectiveStateDraft,
			Revision:      1,
			FindingIDs:    copyQualityObjectiveIDs(input.FindingIDs),
			SessionID:     strings.TrimSpace(input.SessionID),
			BaselineID:    strings.TrimSpace(input.BaselineID),
			CampaignID:    strings.TrimSpace(input.CampaignID),
			RunIDs:        copyQualityObjectiveIDs(input.RunIDs),
			ProposalIDs:   copyQualityObjectiveIDs(input.ProposalIDs),
			PrimarySignal: copyQualityObjectiveSignal(input.PrimarySignal),
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}
	primarySignal, err := a.qualityObjectivePrimarySignal(ctx, item, input.PrimarySignal)
	if err != nil {
		return domain.QualityObjective{}, err
	}
	item.Spec.PrimarySignal = primarySignal
	if err := item.Validate(); err != nil {
		return domain.QualityObjective{}, contract.InvalidInput(err.Error())
	}
	if err := a.validateQualityObjectiveLinks(ctx, item); err != nil {
		return domain.QualityObjective{}, err
	}
	if err := a.store.SaveQualityObjective(ctx, item); err != nil {
		return domain.QualityObjective{}, err
	}
	return item, nil
}

func copyQualityObjectiveSignal(signal *domain.QualityObjectiveSignal) *domain.QualityObjectiveSignal {
	if signal == nil {
		return nil
	}
	copy := *signal
	return &copy
}

func (a *App) validateQualityObjectiveLinks(ctx context.Context, objective domain.QualityObjective) error {
	for _, id := range objective.Spec.FindingIDs {
		finding, err := a.Finding(ctx, id)
		if err != nil {
			return err
		}
		if err := validateQualityObjectiveLinkScope(objective.Spec, finding.Spec.ProjectID, finding.Spec.RepositoryID, ""); err != nil {
			return err
		}
	}

	if objective.Spec.SessionID != "" {
		session, err := a.AssuranceSession(ctx, objective.Spec.SessionID)
		if err != nil {
			return err
		}
		if err := validateQualityObjectiveLinkScope(objective.Spec, session.Spec.ProjectID, session.Spec.RepositoryID, session.Spec.WorktreeID); err != nil {
			return err
		}
	}

	if objective.Spec.BaselineID != "" {
		baselines, err := a.PRCIBaselines(ctx)
		if err != nil {
			return err
		}
		var baseline domain.PRCIBaseline
		for _, candidate := range baselines {
			if candidate.Metadata.ID == objective.Spec.BaselineID {
				baseline = candidate
				break
			}
		}
		if baseline.Metadata.ID == "" {
			return contract.NotFound("PR CI baseline not found")
		}
		if err := validateQualityObjectiveLinkScope(objective.Spec, baseline.Spec.ProjectID, baseline.Spec.RepositoryID, baseline.Spec.WorktreeID); err != nil {
			return err
		}
	}

	if objective.Spec.CampaignID != "" {
		campaigns, err := a.QualityCampaigns(ctx)
		if err != nil {
			return err
		}
		var campaign domain.QualityCampaign
		for _, candidate := range campaigns {
			if candidate.Metadata.ID == objective.Spec.CampaignID {
				campaign = candidate
				break
			}
		}
		if campaign.Metadata.ID == "" {
			return contract.NotFound("quality campaign not found")
		}
		if err := validateQualityObjectiveLinkScope(objective.Spec, campaign.Spec.ProjectID, campaign.Spec.RepositoryID, campaign.Spec.WorktreeID); err != nil {
			return err
		}
	}

	if len(objective.Spec.RunIDs) > 0 {
		runs, err := a.QualityRuns(ctx)
		if err != nil {
			return err
		}
		for _, id := range objective.Spec.RunIDs {
			var run domain.QualityRun
			for _, candidate := range runs {
				if candidate.Metadata.ID == id {
					run = candidate
					break
				}
			}
			if run.Metadata.ID == "" {
				return contract.NotFound("quality run not found")
			}
			if err := validateQualityObjectiveLinkScope(objective.Spec, run.Spec.ProjectID, run.Spec.RepositoryID, run.Spec.WorktreeID); err != nil {
				return err
			}
		}
	}

	for _, id := range objective.Spec.ProposalIDs {
		proposal, err := a.Proposal(ctx, id)
		if err != nil {
			return err
		}
		if err := validateQualityObjectiveLinkScope(objective.Spec, proposal.Spec.ProjectID, proposal.Spec.RepositoryID, proposal.Spec.WorktreeID); err != nil {
			return err
		}
	}

	return nil
}

func validateQualityObjectiveLinkScope(objective domain.QualityObjectiveSpec, projectID, repositoryID, worktreeID string) error {
	if projectID != objective.ProjectID || repositoryID != objective.RepositoryID || (worktreeID != "" && worktreeID != objective.WorktreeID) {
		return contract.InvalidInput("quality objective links must match the selected project, repository, and worktree")
	}
	return nil
}

func (a *App) QualityHome(ctx context.Context) (QualityHome, error) {
	objectives, err := a.QualityObjectives(ctx)
	if err != nil {
		return QualityHome{}, err
	}
	runs, err := a.QualityRuns(ctx)
	if err != nil {
		return QualityHome{}, err
	}
	baselines, err := a.PRCIBaselines(ctx)
	if err != nil {
		return QualityHome{}, err
	}
	assuranceProposals, err := a.AssuranceProposals(ctx, "")
	if err != nil {
		return QualityHome{}, err
	}
	projects, err := a.Projects(ctx)
	if err != nil {
		return QualityHome{}, err
	}

	result := QualityHome{
		Objectives: append([]domain.QualityObjective{}, objectives...),
		Queue:      []QualityHomeQueueItem{},
	}
	attachedFindingIDs := make(map[string]struct{})
	for _, objective := range objectives {
		for _, findingID := range objective.Spec.FindingIDs {
			attachedFindingIDs[findingID] = struct{}{}
		}
		if objective.Spec.State != domain.QualityObjectiveStateAdopted && objective.Spec.State != domain.QualityObjectiveStateRejected {
			result.Summary.ActiveObjectives++
		}
		if objective.Spec.State == domain.QualityObjectiveStateBlocked || objective.Spec.State == domain.QualityObjectiveStateStale {
			result.Queue = append(result.Queue, QualityHomeQueueItem{
				ID:           "objective:" + objective.Metadata.ID,
				Kind:         domain.QualityObjectiveKind,
				State:        objective.Spec.State,
				Title:        objective.Spec.Title,
				Summary:      "quality objective needs attention",
				ProjectID:    objective.Spec.ProjectID,
				RepositoryID: objective.Spec.RepositoryID,
				WorktreeID:   objective.Spec.WorktreeID,
				ReferenceID:  objective.Metadata.ID,
				UpdatedAt:    objective.Spec.UpdatedAt,
				priority:     10,
			})
		}
	}
	for _, run := range runs {
		priority := 0
		summary := ""
		switch run.Spec.State {
		case domain.AssuranceStateQueued, domain.AssuranceStateRunning, domain.AssuranceStateCancelling:
			result.Summary.RunningRuns++
			priority = 20
			summary = "quality run is in progress"
		case domain.AssuranceStateFailed, domain.AssuranceStateTimedOut, domain.AssuranceStateInterrupted:
			result.Summary.FailedRuns++
			priority = 15
			summary = "quality run requires review"
		default:
			continue
		}
		result.Queue = append(result.Queue, QualityHomeQueueItem{
			ID:           "run:" + run.Metadata.ID,
			Kind:         domain.QualityRunKind,
			State:        run.Spec.State,
			Title:        run.Spec.Technique,
			Summary:      summary,
			ProjectID:    run.Spec.ProjectID,
			RepositoryID: run.Spec.RepositoryID,
			WorktreeID:   run.Spec.WorktreeID,
			ReferenceID:  run.Metadata.ID,
			UpdatedAt:    qualityRunUpdatedAt(run),
			priority:     priority,
		})
	}
	for _, baseline := range baselines {
		if baseline.Spec.State != domain.AssuranceStateStale && baseline.Spec.State != domain.QualityObjectiveStateStale {
			continue
		}
		result.Summary.StaleBaselines++
		result.Queue = append(result.Queue, QualityHomeQueueItem{
			ID:           "baseline:" + baseline.Metadata.ID,
			Kind:         domain.PRCIBaselineKind,
			State:        baseline.Spec.State,
			Title:        "PR/CI baseline",
			Summary:      baseline.Spec.StaleReason,
			ProjectID:    baseline.Spec.ProjectID,
			RepositoryID: baseline.Spec.RepositoryID,
			WorktreeID:   baseline.Spec.WorktreeID,
			ReferenceID:  baseline.Metadata.ID,
			UpdatedAt:    baseline.Spec.CapturedAt,
			priority:     25,
		})
	}
	for _, proposal := range assuranceProposals {
		if proposal.Spec.State != "proposed" && proposal.Spec.State != "critic_advisory" {
			continue
		}
		result.Summary.UnreviewedProposals++
		result.Queue = append(result.Queue, QualityHomeQueueItem{
			ID:           "assurance-proposal:" + proposal.Metadata.ID,
			Kind:         domain.AssuranceProposalKind,
			State:        proposal.Spec.State,
			Title:        proposal.Spec.Purpose,
			Summary:      proposal.Spec.CriticSummary,
			ProjectID:    proposal.Spec.ProjectID,
			RepositoryID: proposal.Spec.RepositoryID,
			WorktreeID:   proposal.Spec.WorktreeID,
			ReferenceID:  proposal.Metadata.ID,
			UpdatedAt:    proposal.Spec.CreatedAt,
			priority:     30,
		})
	}

	for _, project := range projects {
		findings, findingErr := a.store.ListFindings(ctx, project.Metadata.ID, "")
		if findingErr != nil {
			return QualityHome{}, findingErr
		}
		for _, finding := range findings {
			if finding.Spec.State != domain.FindingOpen {
				continue
			}
			result.Summary.OpenFindings++
			if _, attached := attachedFindingIDs[finding.Metadata.ID]; attached {
				continue
			}
			result.Queue = append(result.Queue, QualityHomeQueueItem{
				ID:           "finding:" + finding.Metadata.ID,
				Kind:         domain.FindingKind,
				State:        string(finding.Spec.State),
				Title:        finding.Spec.Summary,
				Summary:      finding.Spec.RecommendedNext,
				ProjectID:    finding.Spec.ProjectID,
				RepositoryID: finding.Spec.RepositoryID,
				ReferenceID:  finding.Metadata.ID,
				Severity:     finding.Spec.Severity,
				UpdatedAt:    finding.Spec.LastObserved,
				priority:     35,
			})
		}
		for _, repository := range project.Spec.Repositories {
			proposals, proposalErr := a.store.ListProposals(ctx, project.Metadata.ID, repository.Metadata.ID, "")
			if proposalErr != nil {
				return QualityHome{}, proposalErr
			}
			for _, proposal := range proposals {
				if proposal.Spec.State != domain.ProposalPending && proposal.Spec.State != domain.ProposalStale {
					continue
				}
				result.Summary.UnreviewedProposals++
				result.Queue = append(result.Queue, QualityHomeQueueItem{
					ID:           "proposal:" + proposal.Metadata.ID,
					Kind:         domain.ProposalKind,
					State:        string(proposal.Spec.State),
					Title:        proposal.Metadata.Name,
					Summary:      proposal.Spec.Command,
					ProjectID:    proposal.Spec.ProjectID,
					RepositoryID: proposal.Spec.RepositoryID,
					WorktreeID:   proposal.Spec.WorktreeID,
					ReferenceID:  proposal.Metadata.ID,
					UpdatedAt:    proposal.Spec.CreatedAt,
					priority:     30,
				})
			}
		}
	}
	sort.SliceStable(result.Queue, func(i, j int) bool {
		if result.Queue[i].priority != result.Queue[j].priority {
			return result.Queue[i].priority < result.Queue[j].priority
		}
		if !result.Queue[i].UpdatedAt.Equal(result.Queue[j].UpdatedAt) {
			return result.Queue[i].UpdatedAt.After(result.Queue[j].UpdatedAt)
		}
		return result.Queue[i].ID < result.Queue[j].ID
	})
	result.Summary.QueueItems = len(result.Queue)
	return result, nil
}

func copyQualityObjectiveIDs(values []string) []string {
	return append([]string{}, values...)
}

func qualityRunUpdatedAt(run domain.QualityRun) time.Time {
	if run.Spec.CompletedAt != nil {
		return *run.Spec.CompletedAt
	}
	return run.Spec.StartedAt
}
