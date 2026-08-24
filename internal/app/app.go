package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/knowgyu/dev-control-room/internal/action"
	"github.com/knowgyu/dev-control-room/internal/checkset"
	"github.com/knowgyu/dev-control-room/internal/collector"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/discovery"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/environment"
	"github.com/knowgyu/dev-control-room/internal/folderpicker"
	"github.com/knowgyu/dev-control-room/internal/masking"
	"github.com/knowgyu/dev-control-room/internal/reconcile"
	"github.com/knowgyu/dev-control-room/internal/scheduler"
	"github.com/knowgyu/dev-control-room/internal/store"
)

var safeID = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

type App struct {
	home          string
	listen        string
	config        Config
	configMu      sync.Mutex
	mutationToken string
	masker        *masking.Masker
	store         *store.Store
	broker        *action.Broker
	collector     collector.GitCollector
	doctor        environment.Doctor
	launcher      environment.Launcher
	scheduler     scheduler.Adapter
	scanNow       chan string
	scanMu        sync.Mutex
	environmentMu sync.Mutex
	safeguardMu   sync.Mutex
}

func New(home, listen string) (*App, error) {
	if err := requireLoopback(listen); err != nil {
		return nil, err
	}
	if strings.TrimSpace(home) == "" {
		return nil, errors.New("application home is required")
	}
	config, err := loadConfig(home)
	if err != nil {
		return nil, err
	}
	masker := masking.New(nil, []string{"TOKEN", "PASSWORD", "SECRET", "API_KEY", "AUTHORIZATION"})
	database, err := store.Open(context.Background(), filepath.Join(home, "state.db"))
	if err != nil {
		return nil, err
	}
	persistence, err := store.New(database, masker)
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	broker, err := action.New(persistence, nil)
	if err != nil {
		_ = persistence.Close()
		return nil, err
	}
	// M0 stored registered projects in config.json. Import them once into the
	// durable M1 repository. Import is idempotent so an interrupted multi-project
	// migration resumes on the next start instead of silently skipping entries.
	if len(config.Projects) > 0 {
		for _, project := range config.Projects {
			if _, lookupErr := persistence.GetProject(context.Background(), project.Metadata.ID); lookupErr == nil {
				continue
			} else if !errors.Is(lookupErr, sql.ErrNoRows) {
				_ = persistence.Close()
				return nil, fmt.Errorf("inspect configured project %q: %w", project.Metadata.ID, lookupErr)
			}
			if err := persistence.SaveProject(context.Background(), project); err != nil {
				_ = persistence.Close()
				return nil, fmt.Errorf("migrate configured project %q: %w", project.Metadata.ID, err)
			}
		}
		config.Projects = nil
		if err := saveConfig(home, config); err != nil {
			_ = persistence.Close()
			return nil, fmt.Errorf("finalize project configuration migration: %w", err)
		}
	}
	service := &App{
		home: home, listen: listen, config: config, mutationToken: randomToken(), masker: masker,
		store: persistence, broker: broker, collector: collector.NewGitCollector(nil), doctor: environment.NewDoctor(nil, masker), launcher: environment.ProcessLauncher{}, scheduler: scheduler.NewAdapter(), scanNow: make(chan string, 1),
	}
	var scheduled scheduler.Result
	if found, err := persistence.LoadSingleton(context.Background(), "scheduler_state", &scheduled); err != nil {
		_ = persistence.Close()
		return nil, fmt.Errorf("load scheduler state: %w", err)
	} else if found {
		if restorable, ok := service.scheduler.(interface{ Restore(bool) }); ok {
			restorable.Restore(scheduled.Exists)
		}
	}
	if !config.AgentProfilesInitialized {
		if err := service.ensureDefaultAgentProfiles(context.Background()); err != nil {
			_ = persistence.Close()
			return nil, err
		}
		config.AgentProfilesInitialized = true
		service.config = config
		if err := saveConfig(home, config); err != nil {
			_ = persistence.Close()
			return nil, fmt.Errorf("finalize default agent profile initialization: %w", err)
		}
	}
	return service, nil
}

func requireLoopback(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("initial release only permits a loopback listen address")
	}
	return nil
}

func (a *App) Close() error {
	if a == nil || a.store == nil {
		return nil
	}
	return a.store.Close()
}

func (a *App) RunDoctor(ctx context.Context) {
	_ = a.RunScan(ctx, "startup")
	_, _ = a.EnvironmentHealth(ctx, true)
	interval := time.Duration(a.config.ScanIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = a.RunScan(ctx, "schedule")
			_, _ = a.EnvironmentHealth(ctx, true)
		case trigger := <-a.scanNow:
			_ = a.RunScan(ctx, trigger)
		}
	}
}

func (a *App) Health(context.Context) Health {
	return Health{OK: true, Service: "dev-control-room", NetworkMode: "loopback-only", Telemetry: false, Contract: contract.EnvelopeSchema, ConfigVersion: a.config.Version}
}

func (a *App) Snapshot(ctx context.Context) (Snapshot, error) {
	projects, err := a.store.ListProjects(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{GeneratedAt: time.Now().UTC(), Projects: make([]ProjectState, 0, len(projects))}
	for _, project := range projects {
		state := ProjectState{ID: project.Metadata.ID, Name: project.Metadata.Name, Repos: make([]RepositoryState, 0, len(project.Spec.Repositories))}
		for _, repository := range project.Spec.Repositories {
			observations, err := a.store.ListObservations(ctx, project.Metadata.ID, repository.Metadata.ID)
			if err != nil {
				return Snapshot{}, err
			}
			worktrees, err := a.store.ListWorktrees(ctx, project.Metadata.ID, repository.Metadata.ID)
			if err != nil {
				return Snapshot{}, err
			}
			if len(observations) == 0 {
				state.Repos = append(state.Repos, RepositoryState{ID: repository.Metadata.ID, Path: repository.Spec.Path, Worktrees: worktrees, WorktreeCount: len(worktrees)})
				continue
			}
			repositoryState, err := stateFromObservation(observations[0])
			if err != nil {
				return Snapshot{}, err
			}
			repositoryState.Worktrees = worktrees
			repositoryState.ID = repository.Metadata.ID
			if len(worktrees) > 0 {
				repositoryState.WorktreeCount = len(worktrees)
			}
			state.Repos = append(state.Repos, repositoryState)
			if repositoryState.ScannedAt.After(state.ScannedAt) {
				state.ScannedAt = repositoryState.ScannedAt
			}
		}
		snapshot.Projects = append(snapshot.Projects, state)
	}
	return snapshot, nil
}

func (a *App) Projects(ctx context.Context) ([]domain.Project, error) {
	return a.store.ListProjects(ctx)
}

func (a *App) Project(ctx context.Context, id string) (domain.Project, error) {
	project, err := a.store.GetProject(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Project{}, contract.NotFound("project not found")
	}
	return project, err
}

func (a *App) Repositories(ctx context.Context, projectID string) ([]domain.Repository, error) {
	if _, err := a.Project(ctx, projectID); err != nil {
		return nil, err
	}
	return a.store.ListRepositories(ctx, projectID)
}

func (a *App) Repository(ctx context.Context, projectID, repositoryID string) (domain.Repository, error) {
	if _, err := a.Project(ctx, projectID); err != nil {
		return domain.Repository{}, err
	}
	repository, err := a.store.GetRepository(ctx, projectID, repositoryID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Repository{}, contract.NotFound("repository not found")
	}
	return repository, err
}

func (a *App) Worktrees(ctx context.Context, projectID, repositoryID string) ([]domain.Worktree, error) {
	if _, err := a.Repository(ctx, projectID, repositoryID); err != nil {
		return nil, err
	}
	return a.store.ListWorktrees(ctx, projectID, repositoryID)
}

func (a *App) Worktree(ctx context.Context, projectID, repositoryID, worktreeID string) (domain.Worktree, error) {
	if _, err := a.Repository(ctx, projectID, repositoryID); err != nil {
		return domain.Worktree{}, err
	}
	item, err := a.store.GetWorktree(ctx, projectID, repositoryID, worktreeID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Worktree{}, contract.NotFound("worktree not found")
	}
	return item, err
}

func (a *App) CleanupCandidates(ctx context.Context, projectID string) ([]domain.CleanupCandidate, error) {
	projects, err := a.Projects(ctx)
	if err != nil {
		return nil, err
	}
	snapshot, err := a.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	origins := make(map[string]string)
	for _, project := range snapshot.Projects {
		for _, repository := range project.Repos {
			origins[project.ID+"\x00"+repository.ID] = repository.Origin
		}
	}
	items := []domain.CleanupCandidate{}
	for _, project := range projects {
		if projectID != "" && project.Metadata.ID != projectID {
			continue
		}
		for _, repository := range project.Spec.Repositories {
			worktrees, err := a.store.ListWorktrees(ctx, project.Metadata.ID, repository.Metadata.ID)
			if err != nil {
				return nil, err
			}
			for _, worktree := range worktrees {
				reasons := []string{"merged change evidence is unavailable without a configured provider"}
				if worktree.Spec.Primary {
					reasons = append(reasons, "primary Worktree is never an automatic cleanup target")
				}
				if worktree.Spec.Dirty {
					reasons = append(reasons, "Worktree is dirty")
				}
				if worktree.Spec.Untracked {
					reasons = append(reasons, "Worktree has untracked files")
				}
				if worktree.Spec.Detached {
					reasons = append(reasons, "Worktree is detached")
				}
				if worktree.Spec.Locked {
					reasons = append(reasons, "Worktree is locked")
				}
				if worktree.Spec.Prunable || worktree.Spec.TombstonedAt != nil {
					reasons = append(reasons, "Worktree is prunable or tombstoned")
				}
				if worktree.Spec.Upstream == "" {
					reasons = append(reasons, "Worktree has no upstream")
				}
				if worktree.Spec.Ahead > 0 {
					reasons = append(reasons, "Worktree has unpushed commits")
				}
				if worktree.Spec.Error != "" {
					reasons = append(reasons, "Worktree observation is incomplete")
				}
				observedAt := worktree.Spec.LastObserved
				if observedAt.IsZero() {
					observedAt = time.Now().UTC()
				}
				decision := domain.CleanupBlocked
				merged := false
				mergeEvidence := ""
				localSafetyPassed := !worktree.Spec.Primary && !worktree.Spec.Dirty && !worktree.Spec.Untracked && !worktree.Spec.Detached && !worktree.Spec.Locked && !worktree.Spec.Prunable && worktree.Spec.TombstonedAt == nil && worktree.Spec.Upstream != "" && worktree.Spec.Ahead == 0 && worktree.Spec.Error == "" && worktree.Spec.Head != ""
				if localSafetyPassed {
					merged, mergeEvidence, err = a.githubCommitMerged(ctx, origins[project.Metadata.ID+"\x00"+repository.Metadata.ID], worktree.Spec.Head)
					if err != nil {
						merged = false
					}
				}
				if merged {
					decision = domain.CleanupReviewable
					reasons = []string{mergeEvidence, "local Worktree safety checks passed; human review is still required"}
				}
				candidate := domain.CleanupCandidate{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.CleanupCandidateKind}, Metadata: domain.ObjectMeta{ID: cleanupCandidateID(project.Metadata.ID, repository.Metadata.ID, worktree.Metadata.ID), Name: "Cleanup candidate " + worktree.Metadata.ID}, Spec: domain.CleanupCandidateSpec{ProjectID: project.Metadata.ID, RepositoryID: repository.Metadata.ID, WorktreeID: worktree.Metadata.ID, CanonicalPath: worktree.Spec.CanonicalPath, Branch: worktree.Spec.Branch, Head: worktree.Spec.Head, Dirty: worktree.Spec.Dirty, Untracked: worktree.Spec.Untracked, Detached: worktree.Spec.Detached, Locked: worktree.Spec.Locked, Prunable: worktree.Spec.Prunable, Ahead: worktree.Spec.Ahead, Behind: worktree.Spec.Behind, Upstream: worktree.Spec.Upstream, Merged: merged, MergeEvidence: mergeEvidence, Decision: decision, Reasons: reasons, ObservedAt: observedAt}}
				if err := candidate.Validate(); err != nil {
					return nil, err
				}
				items = append(items, candidate)
			}
		}
	}
	return items, nil
}

func cleanupCandidateID(projectID, repositoryID, worktreeID string) string {
	sum := sha256.Sum256([]byte(projectID + "\x00" + repositoryID + "\x00" + worktreeID))
	return "cleanup-" + hex.EncodeToString(sum[:])[:56]
}

func (a *App) Proposals(ctx context.Context, projectID, repositoryID, worktreeID string) ([]domain.Proposal, error) {
	if _, err := a.Worktree(ctx, projectID, repositoryID, worktreeID); err != nil && worktreeID != "" {
		return nil, err
	}
	if _, err := a.Repository(ctx, projectID, repositoryID); err != nil {
		return nil, err
	}
	items, err := a.store.ListProposals(ctx, projectID, repositoryID, worktreeID)
	if err != nil {
		return nil, err
	}
	for index := range items {
		if items[index].Spec.State == domain.ProposalPending {
			if stale, err := a.proposalStale(ctx, items[index]); err != nil {
				return nil, err
			} else if stale {
				items[index].Spec.State = domain.ProposalStale
			}
		}
	}
	return items, nil
}

func (a *App) Proposal(ctx context.Context, id string) (domain.Proposal, error) {
	proposal, err := a.store.GetProposal(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Proposal{}, contract.NotFound("proposal not found")
	}
	if err != nil {
		return domain.Proposal{}, err
	}
	if proposal.Spec.State == domain.ProposalPending {
		stale, err := a.proposalStale(ctx, proposal)
		if err != nil {
			return domain.Proposal{}, err
		}
		if stale {
			proposal.Spec.State = domain.ProposalStale
		}
	}
	return proposal, nil
}

func (a *App) Checksets(ctx context.Context, projectID, repositoryID string) ([]domain.Checkset, error) {
	if _, err := a.Repository(ctx, projectID, repositoryID); err != nil {
		return nil, err
	}
	return a.store.ListChecksets(ctx, projectID, repositoryID)
}

func (a *App) Checkset(ctx context.Context, id string) (domain.Checkset, error) {
	item, err := a.store.GetCheckset(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Checkset{}, contract.NotFound("checkset not found")
	}
	return item, err
}

func (a *App) CheckRuns(ctx context.Context, checksetID string) ([]domain.CheckRun, error) {
	if _, err := a.Checkset(ctx, checksetID); err != nil {
		return nil, err
	}
	return a.store.ListCheckRuns(ctx, checksetID)
}

func (a *App) CreateCheckset(ctx context.Context, input CreateChecksetInput) (domain.Checkset, error) {
	if strings.TrimSpace(input.ID) == "" || safeID.MatchString(input.ID) {
		return domain.Checkset{}, contract.InvalidInput("checkset id is invalid")
	}
	proposal, err := a.Proposal(ctx, input.ProposalID)
	if err != nil {
		return domain.Checkset{}, err
	}
	if proposal.Spec.State != domain.ProposalApplied {
		return domain.Checkset{}, contract.InvalidInput("checkset requires an applied proposal")
	}
	if proposal.Spec.TypedCommand == nil || len(input.Steps) == 0 {
		return domain.Checkset{}, contract.InvalidInput("checkset command must exactly match reviewed typed proposal evidence")
	}
	for _, step := range input.Steps {
		if !reflect.DeepEqual(step.Command, *proposal.Spec.TypedCommand) {
			return domain.Checkset{}, contract.InvalidInput("checkset command must exactly match reviewed typed proposal evidence")
		}
	}
	if stale, err := a.proposalStale(ctx, proposal); err != nil {
		return domain.Checkset{}, contract.Unavailable("proposal evidence cannot be verified")
	} else if stale {
		return domain.Checkset{}, contract.InvalidInput("checkset proposal evidence is stale")
	}
	item := domain.Checkset{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ChecksetKind}, Metadata: domain.ObjectMeta{ID: input.ID, Name: input.Name}, Spec: domain.ChecksetSpec{ProjectID: proposal.Spec.ProjectID, RepositoryID: proposal.Spec.RepositoryID, WorktreeID: proposal.Spec.WorktreeID, Head: proposal.Spec.Head, ProposalID: proposal.Metadata.ID, Name: input.Name, State: domain.ChecksetDraft, Steps: input.Steps}}
	if err := item.Validate(); err != nil {
		return domain.Checkset{}, contract.InvalidInput(err.Error())
	}
	if err := a.store.SaveCheckset(ctx, item); err != nil {
		return domain.Checkset{}, err
	}
	return item, nil
}

func (a *App) ApplyCheckset(ctx context.Context, id string) (domain.Checkset, error) {
	item, err := a.Checkset(ctx, id)
	if err != nil {
		return domain.Checkset{}, err
	}
	if item.Spec.State != domain.ChecksetDraft {
		return domain.Checkset{}, contract.InvalidInput("only draft checksets can be applied")
	}
	item.Spec.State = domain.ChecksetApplied
	if err := a.store.SaveCheckset(ctx, item); err != nil {
		return domain.Checkset{}, err
	}
	return item, nil
}

func (a *App) RunCheckset(ctx context.Context, id string) (domain.CheckRun, error) {
	item, err := a.Checkset(ctx, id)
	if err != nil {
		return domain.CheckRun{}, err
	}
	if err := item.Validate(); err != nil {
		return domain.CheckRun{}, contract.InvalidInput("stored checkset is invalid")
	}
	if item.Spec.State != domain.ChecksetApplied {
		return domain.CheckRun{}, contract.InvalidInput("only applied checksets can run")
	}
	proposal, err := a.Proposal(ctx, item.Spec.ProposalID)
	if err != nil || proposal.Spec.State != domain.ProposalApplied {
		return domain.CheckRun{}, contract.Unavailable("checkset proposal evidence is unavailable")
	}
	if stale, err := a.proposalStale(ctx, proposal); err != nil || stale {
		return domain.CheckRun{}, contract.Unavailable("checkset proposal evidence is no longer current")
	}
	worktree, changed, err := a.discoveryWorktree(ctx, item.Spec.ProjectID, item.Spec.RepositoryID, item.Spec.WorktreeID)
	if err != nil || changed || worktree.Head != item.Spec.Head {
		return domain.CheckRun{}, contract.Unavailable("checkset worktree evidence is no longer current")
	}
	started := time.Now().UTC()
	run := domain.CheckRun{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.CheckRunKind}, Metadata: domain.ObjectMeta{ID: eventID("check-run"), Name: item.Metadata.Name}, Spec: domain.CheckRunSpec{ChecksetID: item.Metadata.ID, ProjectID: item.Spec.ProjectID, RepositoryID: item.Spec.RepositoryID, WorktreeID: item.Spec.WorktreeID, Head: worktree.Head, StartedAt: started, Status: domain.CheckPassed, Steps: make([]domain.CheckStepRun, 0, len(item.Spec.Steps))}}
	states := make(map[string]domain.CheckRunStatus, len(item.Spec.Steps))
	remaining := append([]domain.CheckStep(nil), item.Spec.Steps...)
	for len(remaining) != 0 {
		index := -1
		for candidate, step := range remaining {
			ready := true
			for _, dependency := range step.DependsOn {
				if _, done := states[dependency]; !done {
					ready = false
					break
				}
			}
			if ready {
				index = candidate
				break
			}
		}
		if index < 0 {
			return domain.CheckRun{}, contract.InvalidInput("checkset dependencies cannot be scheduled")
		}
		step := remaining[index]
		remaining = append(remaining[:index], remaining[index+1:]...)
		stepRun := domain.CheckStepRun{StepID: step.ID}
		blocked := false
		for _, dependency := range step.DependsOn {
			if states[dependency] != domain.CheckPassed {
				blocked = true
				break
			}
		}
		if blocked {
			stepRun.Status = domain.CheckSkipped
		} else {
			stepRun = checkset.Run(ctx, worktree.Path, step.Command, a.masker)
			stepRun.StepID = step.ID
		}
		states[step.ID] = stepRun.Status
		run.Spec.Steps = append(run.Spec.Steps, stepRun)
		if stepRun.Status != domain.CheckPassed && run.Spec.Status == domain.CheckPassed {
			run.Spec.Status = stepRun.Status
		}
		if stepRun.Status == domain.CheckCancelled || stepRun.Status == domain.CheckTimedOut {
			for _, pending := range remaining {
				run.Spec.Steps = append(run.Spec.Steps, domain.CheckStepRun{StepID: pending.ID, Status: domain.CheckSkipped})
			}
			break
		}
	}
	run.Spec.CompletedAt = time.Now().UTC()
	if err := a.store.SaveCheckRun(context.WithoutCancel(ctx), run); err != nil {
		return domain.CheckRun{}, err
	}
	if shouldRecordCheckRunFailure(run.Spec.Status) {
		if err := a.recordFailureOccurrence(context.WithoutCancel(ctx), checkRunFailureOccurrence(item, run)); err != nil {
			return domain.CheckRun{}, err
		}
	}
	return run, nil
}

// Discover persists only evidence-derived proposals. It never runs a proposed
// command, writes the selected worktree, or scans paths outside it.
func (a *App) Discover(ctx context.Context, projectID, repositoryID, worktreeID string) (domain.Discovery, error) {
	worktree, changed, err := a.discoveryWorktree(ctx, projectID, repositoryID, worktreeID)
	if err != nil {
		return domain.Discovery{}, contract.Unavailable("selected worktree could not be revalidated")
	}
	if changed {
		return domain.Discovery{}, contract.Unavailable("selected worktree identity no longer matches current Git evidence")
	}
	candidates, err := discovery.Discover(worktree.Path)
	if err != nil {
		return domain.Discovery{}, contract.Unavailable("selected worktree sources could not be read")
	}
	now := time.Now().UTC()
	proposalIDs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		proposal := newProposal(projectID, repositoryID, worktreeID, worktree.Branch, worktree.Head, candidate, now)
		if err := a.store.SaveProposal(ctx, proposal); err != nil {
			return domain.Discovery{}, err
		}
		proposalIDs = append(proposalIDs, proposal.Metadata.ID)
	}
	result := domain.Discovery{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.DiscoveryKind},
		Metadata: domain.ObjectMeta{ID: discoveryID(projectID, repositoryID, worktreeID, worktree.Head, now), Name: "deterministic repository discovery"},
		Spec:     domain.DiscoverySpec{ProjectID: projectID, RepositoryID: repositoryID, WorktreeID: worktreeID, Branch: worktree.Branch, Head: worktree.Head, DiscoveredAt: now, ProposalIDs: proposalIDs},
	}
	if err := result.Validate(); err != nil {
		return domain.Discovery{}, err
	}
	if err := a.recordEvent(domain.Event{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.EventKind}, Metadata: domain.ObjectMeta{ID: eventID("discovery"), Name: "repository discovery"}, Spec: domain.EventSpec{ProjectID: projectID, RepositoryID: repositoryID, EventType: "proposal.discovered", Actor: "service", Summary: fmt.Sprintf("discovered %d proposal(s) in %s", len(proposalIDs), worktreeID), Data: map[string]any{"worktree_id": worktreeID, "proposal_count": len(proposalIDs)}, OccurredAt: now}}); err != nil {
		return domain.Discovery{}, err
	}
	return result, nil
}

func (a *App) ApplyProposal(ctx context.Context, id string) (domain.Proposal, error) {
	return a.reviewProposal(ctx, id, domain.ProposalApplied)
}

func (a *App) RejectProposal(ctx context.Context, id string) (domain.Proposal, error) {
	return a.reviewProposal(ctx, id, domain.ProposalRejected)
}

func (a *App) reviewProposal(ctx context.Context, id string, state domain.ProposalState) (domain.Proposal, error) {
	proposal, err := a.Proposal(ctx, id)
	if err != nil {
		return domain.Proposal{}, err
	}
	if proposal.Spec.State != domain.ProposalPending {
		if proposal.Spec.State == domain.ProposalStale {
			stored, err := a.store.GetProposal(ctx, proposal.Metadata.ID)
			if err != nil {
				return domain.Proposal{}, err
			}
			if stored.Spec.State == domain.ProposalPending {
				stored.Spec.State = domain.ProposalStale
				if err := a.store.MarkProposalStale(ctx, stored); err != nil {
					return domain.Proposal{}, err
				}
			}
		}
		return domain.Proposal{}, contract.InvalidInput("only pending proposals can be reviewed")
	}
	now := time.Now().UTC()
	proposal.Spec.State = state
	proposal.Spec.ReviewedAt = &now
	event := domain.Event{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.EventKind}, Metadata: domain.ObjectMeta{ID: eventID("proposal-review"), Name: "proposal review"}, Spec: domain.EventSpec{ProjectID: proposal.Spec.ProjectID, RepositoryID: proposal.Spec.RepositoryID, EventType: "proposal." + string(state), Actor: "user", Summary: fmt.Sprintf("proposal %s", state), Data: map[string]any{"proposal_id": proposal.Metadata.ID, "worktree_id": proposal.Spec.WorktreeID}, OccurredAt: now}}
	updated, err := a.store.ReviewProposal(ctx, proposal, event)
	if err != nil {
		return domain.Proposal{}, err
	}
	if !updated {
		return domain.Proposal{}, contract.Conflict("proposal has already been reviewed")
	}
	return proposal, nil
}

func (a *App) proposalStale(ctx context.Context, proposal domain.Proposal) (bool, error) {
	worktree, changed, err := a.discoveryWorktree(ctx, proposal.Spec.ProjectID, proposal.Spec.RepositoryID, proposal.Spec.WorktreeID)
	if err != nil {
		return false, contract.Unavailable("proposal worktree could not be revalidated")
	}
	if changed {
		return true, nil
	}
	if worktree.Head != proposal.Spec.Head {
		return true, nil
	}
	candidates, err := discovery.Discover(worktree.Path)
	if err != nil {
		return false, contract.Unavailable("proposal source could not be revalidated")
	}
	for _, candidate := range candidates {
		if candidate.SourcePath == proposal.Spec.SourcePath && candidate.SourceDigest == proposal.Spec.SourceDigest {
			return false, nil
		}
	}
	return true, nil
}

// discoveryWorktree replays the Git common-directory proof from Slice B before
// using a persisted worktree identity for any source read.
func (a *App) discoveryWorktree(ctx context.Context, projectID, repositoryID, worktreeID string) (collector.Worktree, bool, error) {
	stored, err := a.Worktree(ctx, projectID, repositoryID, worktreeID)
	if err != nil {
		if contract.Classify(err).Code == contract.ErrorNotFound {
			return collector.Worktree{}, true, nil
		}
		return collector.Worktree{}, false, err
	}
	if stored.Spec.Trust != domain.WorktreeTrustVerifiedReadOnly || stored.Spec.TombstonedAt != nil || stored.Spec.PathFingerprint == "" || stored.Spec.AssociationFingerprint == "" {
		return collector.Worktree{}, true, nil
	}
	repository, err := a.Repository(ctx, projectID, repositoryID)
	if err != nil {
		return collector.Worktree{}, false, err
	}
	state, err := a.collector.Collect(ctx, repository.Spec.Path)
	if err != nil {
		return collector.Worktree{}, false, err
	}
	if !state.WorktreeEnumerationComplete {
		return collector.Worktree{}, false, errors.New("Git worktree enumeration is incomplete")
	}
	worktrees, _ := a.collector.WorktreeDetails(ctx, repository.Spec.Path, state.Worktrees)
	for _, current := range worktrees {
		if current.ID != worktreeID {
			if current.Error != "" && worktreePathFingerprint(current.Path) == stored.Spec.PathFingerprint {
				return collector.Worktree{}, false, errors.New("selected worktree association proof is incomplete")
			}
			continue
		}
		if current.Error != "" {
			return collector.Worktree{}, false, errors.New("selected worktree association proof is incomplete")
		}
		if current.Trust != string(domain.WorktreeTrustVerifiedReadOnly) || current.Prunable || current.Primary != stored.Spec.Primary || worktreePathFingerprint(current.Path) != stored.Spec.PathFingerprint || current.AssociationFingerprint != stored.Spec.AssociationFingerprint {
			break
		}
		return current, false, nil
	}
	return collector.Worktree{}, true, nil
}

func (a *App) Findings(ctx context.Context, projectID, repositoryID string) ([]domain.Finding, error) {
	if projectID != "" {
		if _, err := a.Project(ctx, projectID); err != nil {
			return nil, err
		}
	} else {
		projects, err := a.Projects(ctx)
		if err != nil {
			return nil, err
		}
		for _, project := range projects {
			if err := a.refreshStale(ctx, project.Metadata.ID); err != nil {
				return nil, err
			}
		}
	}
	if projectID != "" {
		if err := a.refreshStale(ctx, projectID); err != nil {
			return nil, err
		}
	}
	return a.store.ListFindings(ctx, projectID, repositoryID)
}

func (a *App) Finding(ctx context.Context, id string) (domain.Finding, error) {
	finding, err := a.store.GetFinding(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Finding{}, contract.NotFound("finding not found")
	}
	return finding, err
}

func (a *App) Events(ctx context.Context, limit int) ([]domain.Event, error) {
	return a.store.ListEvents(ctx, limit)
}

func (a *App) QueueScan(_ context.Context, trigger string) error {
	if strings.TrimSpace(trigger) == "" {
		trigger = "manual"
	}
	select {
	case a.scanNow <- trigger:
	default:
	}
	return nil
}

func (a *App) RunScan(ctx context.Context, trigger string) error {
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	if strings.TrimSpace(trigger) == "" {
		trigger = "manual"
	}
	projects, err := a.store.ListProjects(ctx)
	if err != nil {
		return err
	}
	failedProjects := 0
	for _, project := range projects {
		collectorFailed, err := a.scanProject(ctx, project, trigger)
		if err != nil {
			return err
		}
		if collectorFailed {
			failedProjects++
		}
	}
	if err := a.recordEvent(domain.Event{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.EventKind}, Metadata: domain.ObjectMeta{ID: eventID("scan"), Name: "diagnosis scan"}, Spec: domain.EventSpec{EventType: "diagnosis.scan.completed", Actor: "service", Summary: fmt.Sprintf("%s scan completed for %d project(s)", trigger, len(projects)), Data: map[string]any{"trigger": trigger, "project_count": len(projects), "failed_project_count": failedProjects}, OccurredAt: time.Now().UTC()}}); err != nil {
		return err
	}
	if failedProjects > 0 {
		return contract.Unavailable("one or more projects could not be fully scanned")
	}
	return nil
}

func (a *App) scanProject(ctx context.Context, project domain.Project, trigger string) (collectorFailed bool, resultErr error) {
	started := time.Now().UTC()
	run := domain.ScanRun{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ScanRunKind}, Metadata: domain.ObjectMeta{ID: scanID(project.Metadata.ID, started), Name: "repository scan"}, Spec: domain.ScanRunSpec{ProjectID: project.Metadata.ID, Trigger: trigger, Status: domain.ScanRunning, StartedAt: started, RepositoryCount: len(project.Spec.Repositories)}}
	if err := a.store.SaveScanRun(ctx, run); err != nil {
		return false, err
	}
	defer func() {
		if resultErr == nil {
			return
		}
		completed := time.Now().UTC()
		run.Spec.Status = domain.ScanFailed
		run.Spec.CompletedAt = &completed
		if err := a.store.SaveScanRun(context.Background(), run); err != nil {
			resultErr = fmt.Errorf("%w; additionally failed to finalize scan run: %v", resultErr, err)
		}
	}()
	findingCount := 0
	for _, repository := range project.Spec.Repositories {
		state, collectErr := a.collector.Collect(ctx, repository.Spec.Path)
		if collectErr == nil {
			var complete bool
			state.Worktrees, complete = a.collector.WorktreeDetails(ctx, repository.Spec.Path, state.Worktrees)
			if !complete || !state.WorktreeEnumerationComplete {
				state.Error = "one or more worktree states could not be collected"
				collectorFailed = true
			}
			if err := a.store.ReplaceWorktrees(ctx, project.Metadata.ID, repository.Metadata.ID, domainWorktrees(a.masker, project.Metadata.ID, repository.Metadata.ID, state.Worktrees, state.CollectedAt), state.WorktreeEnumerationComplete); err != nil {
				return collectorFailed, err
			}
		}
		if collectErr != nil {
			collectorFailed = true
		}
		state.Path = repository.Spec.Path
		observation, err := newObservation(a.masker, project.Metadata.ID, repository.Metadata.ID, state)
		if err != nil {
			return collectorFailed, err
		}
		if err := a.store.SaveObservation(ctx, observation); err != nil {
			return collectorFailed, err
		}
		previous, err := a.store.ListFindings(ctx, project.Metadata.ID, repository.Metadata.ID)
		if err != nil {
			return collectorFailed, err
		}
		findings := reconcile.RepositoryFindings(project.Metadata.ID, repository.Metadata.ID, state, observation.Metadata.ID, state.CollectedAt, previous)
		for _, finding := range findings {
			if err := a.store.SaveFinding(ctx, finding); err != nil {
				return collectorFailed, err
			}
			if finding.Spec.State == domain.FindingOpen || finding.Spec.State == domain.FindingAcknowledged {
				findingCount++
			}
		}
		if collectErr != nil {
			if err := a.recordCollectorFailure(ctx, project.Metadata.ID, repository.Metadata.ID, collectErr); err != nil {
				return collectorFailed, err
			}
		}
	}
	completed := time.Now().UTC()
	run.Spec.Status = domain.ScanSucceeded
	if collectorFailed {
		run.Spec.Status = domain.ScanFailed
	}
	run.Spec.CompletedAt = &completed
	run.Spec.FindingCount = findingCount
	if err := a.store.SaveScanRun(ctx, run); err != nil {
		return collectorFailed, err
	}
	return collectorFailed, nil
}

func (a *App) refreshStale(ctx context.Context, projectID string) error {
	previous, err := a.store.ListFindings(ctx, projectID, "")
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	latest, latestErr := a.store.LatestScanRun(ctx, projectID)
	interval := time.Duration(a.config.ScanIntervalSeconds) * time.Second
	isStale := latestErr != nil || latest.Spec.Status != domain.ScanSucceeded || latest.Spec.CompletedAt == nil || now.Sub(*latest.Spec.CompletedAt) > 2*interval
	if isStale {
		return a.store.SaveFinding(ctx, reconcile.StaleFinding(projectID, now, previous))
	}
	if finding, ok := reconcile.ResolvedStaleFinding(projectID, now, previous); ok {
		return a.store.SaveFinding(ctx, finding)
	}
	return nil
}

func (a *App) AddProject(ctx context.Context, input AddProjectInput) (domain.Project, error) {
	path, err := registeredDirectory(input.Path)
	if err != nil {
		return domain.Project{}, contract.InvalidInput(err.Error())
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = filepath.Base(path)
	}
	id := normalizeAppID(name)
	if id == "" {
		return domain.Project{}, contract.InvalidInput("project name must contain letters or numbers")
	}
	project := domain.NewProject(id, name, []domain.Repository{domain.NewRepository("repo-1", name, path)})
	projects, err := a.store.ListProjects(ctx)
	if err != nil {
		return domain.Project{}, err
	}
	for _, item := range projects {
		if item.Metadata.ID == id {
			return domain.Project{}, contract.Conflict("project already exists")
		}
		for _, repository := range item.Spec.Repositories {
			if strings.EqualFold(repository.Spec.Path, path) {
				return domain.Project{}, contract.Conflict("repository path is already registered")
			}
		}
	}
	if err := a.store.SaveProject(ctx, project); err != nil {
		return domain.Project{}, err
	}
	if err := a.recordEvent(projectEvent("project.added", project.Metadata.ID, "Project added")); err != nil {
		return domain.Project{}, err
	}
	return project, nil
}

func (a *App) DiscoverRepositories(_ context.Context, root string) ([]RepositoryCandidate, error) {
	paths, err := collector.DiscoverGitRoots(root)
	if err != nil {
		return nil, contract.InvalidInput(err.Error())
	}
	items := make([]RepositoryCandidate, 0, len(paths))
	for _, path := range paths {
		items = append(items, RepositoryCandidate{Name: filepath.Base(path), Path: path})
	}
	return items, nil
}

func (a *App) PickDirectory(_ context.Context) (string, error) {
	path, err := folderpicker.Pick()
	if err != nil {
		if errors.Is(err, folderpicker.ErrCancelled) {
			return "", nil
		}
		if errors.Is(err, folderpicker.ErrUnavailable) {
			return "", contract.CodedError{Code: contract.ErrorUnavailable, Message: "native folder picker is unavailable"}
		}
		return "", err
	}
	return path, nil
}

func (a *App) AddProjectTree(ctx context.Context, input AddProjectTreeInput) (domain.Project, error) {
	root, err := registeredDirectory(input.Root)
	if err != nil {
		return domain.Project{}, contract.InvalidInput(err.Error())
	}
	paths := input.Paths
	if len(paths) == 0 {
		paths, err = collector.DiscoverGitRoots(root)
		if err != nil {
			return domain.Project{}, contract.InvalidInput(err.Error())
		}
	}
	canonicalPaths := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, value := range paths {
		path, pathErr := registeredDirectory(value)
		if pathErr != nil || !pathWithin(root, path) {
			return domain.Project{}, contract.InvalidInput("repository path must be an existing directory below the selected folder")
		}
		if _, exists := seen[strings.ToLower(path)]; exists {
			continue
		}
		seen[strings.ToLower(path)] = struct{}{}
		canonicalPaths = append(canonicalPaths, path)
	}
	if len(canonicalPaths) == 0 {
		return domain.Project{}, contract.InvalidInput("no Git repositories were found below the selected folder")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = filepath.Base(root)
	}
	id := normalizeAppID(name)
	if id == "" {
		return domain.Project{}, contract.InvalidInput("project name must contain letters or numbers")
	}
	projects, err := a.store.ListProjects(ctx)
	if err != nil {
		return domain.Project{}, err
	}
	for _, item := range projects {
		if item.Metadata.ID == id {
			return domain.Project{}, contract.Conflict("project already exists")
		}
		for _, repository := range item.Spec.Repositories {
			for _, path := range canonicalPaths {
				if strings.EqualFold(repository.Spec.Path, path) {
					return domain.Project{}, contract.Conflict("repository path is already registered")
				}
			}
		}
	}
	repositories := make([]domain.Repository, 0, len(canonicalPaths))
	usedIDs := make(map[string]int, len(canonicalPaths))
	for _, path := range canonicalPaths {
		base := normalizeAppID(filepath.Base(path))
		if base == "" {
			base = "repo"
		}
		usedIDs[base]++
		repositoryID := base
		if usedIDs[base] > 1 {
			repositoryID = fmt.Sprintf("%s-%d", base, usedIDs[base])
		}
		repositories = append(repositories, domain.NewRepository(repositoryID, filepath.Base(path), path))
	}
	project := domain.NewProject(id, name, repositories)
	if err := a.store.SaveProject(ctx, project); err != nil {
		return domain.Project{}, err
	}
	if err := a.recordEvent(projectEvent("project.added", project.Metadata.ID, fmt.Sprintf("Project added with %d repositories", len(repositories)))); err != nil {
		return domain.Project{}, err
	}
	return project, nil
}

func (a *App) UpdateProject(ctx context.Context, id string, input UpdateProjectInput) (domain.Project, error) {
	project, err := a.Project(ctx, id)
	if err != nil {
		return domain.Project{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return domain.Project{}, contract.InvalidInput("project name is required")
	}
	project.Metadata.Name = name
	if err := a.store.SaveProject(ctx, project); err != nil {
		return domain.Project{}, err
	}
	if err := a.recordEvent(projectEvent("project.updated", id, "Project updated")); err != nil {
		return domain.Project{}, err
	}
	return project, nil
}

func (a *App) RemoveProject(ctx context.Context, id string) error {
	if _, err := a.Project(ctx, id); err != nil {
		return err
	}
	if err := a.store.DeleteProject(ctx, id); errors.Is(err, sql.ErrNoRows) {
		return contract.NotFound("project not found")
	} else if err != nil {
		return err
	}
	return a.recordEvent(removalEvent("project.removed", "Project removed; repository files were not changed", map[string]any{"project_id": id}))
}

func (a *App) AddRepository(ctx context.Context, input AddRepositoryInput) (domain.Repository, error) {
	if _, err := a.Project(ctx, input.ProjectID); err != nil {
		return domain.Repository{}, err
	}
	path, err := registeredDirectory(input.Path)
	if err != nil {
		return domain.Repository{}, contract.InvalidInput(err.Error())
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = filepath.Base(path)
	}
	id := normalizeAppID(input.ID)
	if id == "" {
		id = normalizeAppID(name)
	}
	if id == "" {
		return domain.Repository{}, contract.InvalidInput("repository id must contain letters or numbers")
	}
	repositories, err := a.store.ListRepositories(ctx, input.ProjectID)
	if err != nil {
		return domain.Repository{}, err
	}
	for _, existing := range repositories {
		if existing.Metadata.ID == id {
			return domain.Repository{}, contract.Conflict("repository already exists")
		}
		if strings.EqualFold(existing.Spec.Path, path) {
			return domain.Repository{}, contract.Conflict("repository path is already registered in this project")
		}
	}
	repository := domain.NewRepository(id, name, path)
	if err := a.store.SaveRepository(ctx, input.ProjectID, repository); err != nil {
		return domain.Repository{}, err
	}
	if err := a.recordEvent(projectEventWithRepository("repository.added", input.ProjectID, id, "Repository added")); err != nil {
		return domain.Repository{}, err
	}
	return repository, nil
}

func (a *App) UpdateRepository(ctx context.Context, projectID, repositoryID string, input UpdateRepositoryInput) (domain.Repository, error) {
	if _, err := a.Project(ctx, projectID); err != nil {
		return domain.Repository{}, err
	}
	repository, err := a.Repository(ctx, projectID, repositoryID)
	if err != nil {
		return domain.Repository{}, err
	}
	if name := strings.TrimSpace(input.Name); name != "" {
		repository.Metadata.Name = name
	}
	if strings.TrimSpace(input.Path) != "" {
		path, err := registeredDirectory(input.Path)
		if err != nil {
			return domain.Repository{}, contract.InvalidInput(err.Error())
		}
		repository.Spec.Path = path
	}
	repositories, err := a.store.ListRepositories(ctx, projectID)
	if err != nil {
		return domain.Repository{}, err
	}
	for _, existing := range repositories {
		if existing.Metadata.ID != repositoryID && strings.EqualFold(existing.Spec.Path, repository.Spec.Path) {
			return domain.Repository{}, contract.Conflict("repository path is already registered in this project")
		}
	}
	if err := a.store.SaveRepository(ctx, projectID, repository); err != nil {
		return domain.Repository{}, err
	}
	if err := a.recordEvent(projectEventWithRepository("repository.updated", projectID, repositoryID, "Repository updated")); err != nil {
		return domain.Repository{}, err
	}
	return repository, nil
}

func (a *App) RemoveRepository(ctx context.Context, projectID, repositoryID string) error {
	project, err := a.Project(ctx, projectID)
	if err != nil {
		return err
	}
	if len(project.Spec.Repositories) <= 1 {
		return contract.Conflict("a project must retain at least one repository")
	}
	if _, err := a.Repository(ctx, projectID, repositoryID); err != nil {
		return err
	}
	if err := a.store.DeleteRepository(ctx, projectID, repositoryID); errors.Is(err, sql.ErrNoRows) {
		return contract.NotFound("repository not found")
	} else if err != nil {
		return err
	}
	return a.recordEvent(removalEvent("repository.removed", "Repository removed; repository files were not changed", map[string]any{"project_id": projectID, "repository_id": repositoryID}))
}

func (a *App) AcknowledgeFinding(ctx context.Context, id string) error {
	finding, err := a.Finding(ctx, id)
	if err != nil {
		return err
	}
	finding.Spec.State = domain.FindingAcknowledged
	return a.store.SaveFinding(ctx, finding)
}

type projectExport struct {
	Version int            `json:"version"`
	Project domain.Project `json:"project"`
}

func (a *App) ExportProject(ctx context.Context, id string) ([]byte, error) {
	project, err := a.Project(ctx, id)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(projectExport{Version: 1, Project: project})
	if err != nil {
		return nil, err
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return json.MarshalIndent(a.masker.MaskValue(raw), "", "  ")
}

func (a *App) ImportProject(ctx context.Context, data []byte) (domain.Project, error) {
	var exported projectExport
	if err := json.Unmarshal(data, &exported); err != nil || exported.Version != 1 {
		return domain.Project{}, contract.InvalidInput("invalid project export")
	}
	if err := exported.Project.Validate(); err != nil {
		return domain.Project{}, contract.InvalidInput("invalid project export")
	}
	for index := range exported.Project.Spec.Repositories {
		path, err := registeredDirectory(exported.Project.Spec.Repositories[index].Spec.Path)
		if err != nil {
			return domain.Project{}, contract.InvalidInput("project export contains an unavailable repository path")
		}
		exported.Project.Spec.Repositories[index].Spec.Path = path
	}
	if _, err := a.Project(ctx, exported.Project.Metadata.ID); err == nil {
		return domain.Project{}, contract.Conflict("project already exists")
	} else if _, isCoded := err.(contract.CodedError); !isCoded && !errors.Is(err, sql.ErrNoRows) {
		return domain.Project{}, err
	}
	if err := a.store.SaveProject(ctx, exported.Project); err != nil {
		return domain.Project{}, err
	}
	if err := a.recordEvent(projectEvent("project.imported", exported.Project.Metadata.ID, "Project imported")); err != nil {
		return domain.Project{}, err
	}
	return exported.Project, nil
}

func (a *App) recordCollectorFailure(ctx context.Context, projectID, repositoryID string, err error) error {
	return a.recordFailureOccurrence(ctx, failureOccurrence{
		Category: "collector.git", SourceType: fmt.Sprintf("%T", err), Status: string(contract.Classify(err).Code),
		ExitCode: -1, ProjectID: projectID, RepositoryID: repositoryID,
	})
}

func checkRunFailureOccurrence(checkset domain.Checkset, run domain.CheckRun) failureOccurrence {
	parts := make([]string, 0, len(run.Spec.Steps)+1)
	parts = append(parts, checkset.Metadata.ID)
	exitCode := 0
	for _, step := range run.Spec.Steps {
		if step.Status == domain.CheckPassed || step.Status == domain.CheckSkipped {
			continue
		}
		parts = append(parts, step.StepID+"="+string(step.Status)+":"+fmt.Sprintf("%d", step.ExitCode))
		if exitCode == 0 {
			exitCode = step.ExitCode
		}
	}
	return failureOccurrence{
		Category: "checkset", SourceType: strings.Join(parts, ","), Status: string(run.Spec.Status), ExitCode: exitCode,
		ProjectID: run.Spec.ProjectID, RepositoryID: run.Spec.RepositoryID, WorktreeID: run.Spec.WorktreeID, EvidenceRef: run.Metadata.ID,
	}
}

func shouldRecordCheckRunFailure(status domain.CheckRunStatus) bool {
	switch status {
	case domain.CheckFailed, domain.CheckTimedOut, domain.CheckUnavailable:
		return true
	default:
		return false
	}
}

func (a *App) recordEvent(event domain.Event) error {
	masked, err := maskEvent(a.masker, event)
	if err != nil {
		return err
	}
	a.store.SetMasker(a.masker)
	return a.store.SaveEvent(context.Background(), masked)
}

func maskEvent(masker *masking.Masker, event domain.Event) (domain.Event, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return domain.Event{}, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return domain.Event{}, err
	}
	masked, ok := masker.MaskValue(raw).(map[string]any)
	if !ok {
		return domain.Event{}, errors.New("masked event is not an object")
	}
	data, err = json.Marshal(masked)
	if err != nil {
		return domain.Event{}, err
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return domain.Event{}, err
	}
	return event, nil
}

func registeredDirectory(value string) (string, error) {
	path, err := filepath.Abs(strings.TrimSpace(value))
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return "", errors.New("path must be an existing directory")
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", errors.New("path could not be resolved")
	}
	return filepath.Clean(canonical), nil
}

func pathWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func normalizeAppID(value string) string {
	return strings.Trim(safeID.ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), "-"), "-")
}

func eventID(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano()) }

func newProposal(projectID, repositoryID, worktreeID, branch, head string, candidate discovery.Candidate, discoveredAt time.Time) domain.Proposal {
	typed := reviewedTypedCommand(candidate)
	typedIdentity := ""
	if typed != nil {
		typedIdentity = typed.Executable + "\x00" + strings.Join(typed.Arguments, "\x00")
	}
	identity := projectID + "\x00" + repositoryID + "\x00" + worktreeID + "\x00" + head + "\x00" + candidate.SourcePath + "\x00" + candidate.SourceDigest + "\x00" + candidate.CommandKind + "\x00" + candidate.Command + "\x00" + typedIdentity
	sum := sha256.Sum256([]byte(identity))
	return domain.Proposal{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ProposalKind},
		Metadata: domain.ObjectMeta{ID: "proposal-" + hex.EncodeToString(sum[:])[:48], Name: candidate.Name},
		Spec:     domain.ProposalSpec{ProjectID: projectID, RepositoryID: repositoryID, WorktreeID: worktreeID, Branch: branch, Head: head, SourcePath: candidate.SourcePath, SourceDigest: candidate.SourceDigest, CommandKind: candidate.CommandKind, Command: candidate.Command, TypedCommand: typed, Inference: "deterministic", State: domain.ProposalPending, CreatedAt: discoveredAt},
	}
}

func reviewedTypedCommand(candidate discovery.Candidate) *domain.CheckCommand {
	if candidate.CommandKind != "github_actions_run" {
		return nil
	}
	for command, arguments := range map[string][]string{
		"git status --porcelain": []string{"status", "--porcelain"},
		"git diff --check":       []string{"diff", "--check"},
		"git diff --exit-code":   []string{"diff", "--exit-code"},
	} {
		if candidate.Command == command {
			return &domain.CheckCommand{Executable: "git", Arguments: arguments, TimeoutSeconds: 30}
		}
	}
	return nil
}

func discoveryID(projectID, repositoryID, worktreeID, head string, discoveredAt time.Time) string {
	sum := sha256.Sum256([]byte(projectID + "\x00" + repositoryID + "\x00" + worktreeID + "\x00" + head + "\x00" + discoveredAt.Format(time.RFC3339Nano)))
	return "discovery-" + hex.EncodeToString(sum[:])[:48]
}

func scanID(projectID string, started time.Time) string {
	sum := sha256.Sum256([]byte(projectID + started.Format(time.RFC3339Nano)))
	return "scan-" + hex.EncodeToString(sum[:])[:48]
}

func projectEvent(eventType, projectID, summary string) domain.Event {
	return domain.Event{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.EventKind}, Metadata: domain.ObjectMeta{ID: eventID("event"), Name: eventType}, Spec: domain.EventSpec{EventType: eventType, Actor: "user", ProjectID: projectID, Summary: summary, OccurredAt: time.Now().UTC()}}
}

func projectEventWithRepository(eventType, projectID, repositoryID, summary string) domain.Event {
	event := projectEvent(eventType, projectID, summary)
	event.Spec.RepositoryID = repositoryID
	return event
}

func removalEvent(eventType, summary string, data map[string]any) domain.Event {
	event := projectEvent(eventType, "", summary)
	event.Spec.Data = data
	return event
}

func newObservation(masker *masking.Masker, projectID, repositoryID string, state collector.State) (domain.Observation, error) {
	// Paths from Git porcelain are untrusted discovery input. The durable
	// repository observation retains no linked-worktree path; Worktree records
	// expose only a stable path fingerprint.
	persisted := state
	for i := range persisted.Worktrees {
		persisted.Worktrees[i].Path = masker.Mask(persisted.Worktrees[i].Path)
	}
	data, err := json.Marshal(persisted)
	if err != nil {
		return domain.Observation{}, err
	}
	var evidence map[string]any
	if err := json.Unmarshal(data, &evidence); err != nil {
		return domain.Observation{}, err
	}
	stable := persisted
	stable.CollectedAt = time.Time{}
	stableData, _ := json.Marshal(stable)
	fingerprintSum := sha256.Sum256(append([]byte(projectID+"\x00"+repositoryID+"\x00"), stableData...))
	fingerprint := "sha256:" + hex.EncodeToString(fingerprintSum[:])
	identitySum := sha256.Sum256([]byte(projectID + "\x00" + repositoryID + "\x00" + state.CollectedAt.Format(time.RFC3339Nano) + "\x00" + fingerprint))
	return domain.Observation{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ObservationKind}, Metadata: domain.ObjectMeta{ID: "observation-" + hex.EncodeToString(identitySum[:])[:48], Name: "Git repository state"}, Spec: domain.ObservationSpec{ProjectID: projectID, RepositoryID: repositoryID, Collector: "git", ObservationType: "repository_state", Fingerprint: fingerprint, CollectedAt: state.CollectedAt, Evidence: evidence}}, nil
}

func stateFromObservation(observation domain.Observation) (RepositoryState, error) {
	data, err := json.Marshal(observation.Spec.Evidence)
	if err != nil {
		return RepositoryState{}, err
	}
	var collected collector.State
	if err := json.Unmarshal(data, &collected); err != nil {
		return RepositoryState{}, err
	}
	state := RepositoryState{Path: collected.Path, TopLevel: collected.TopLevel, Branch: collected.Branch, Detached: collected.Detached, Dirty: collected.Dirty, Ahead: collected.Ahead, Behind: collected.Behind, Upstream: collected.Upstream, WorktreeCount: len(collected.Worktrees), UnsafeCleanup: collected.UnsafeCleanup, Error: collected.Error, ScannedAt: observation.Spec.CollectedAt}
	if len(collected.Remotes) > 0 {
		state.Origin = collected.Remotes[0].URL
		state.Provider = collected.Remotes[0].Provider
		state.Capabilities = collected.Remotes[0].Capabilities
	}
	return state, nil
}

func domainWorktrees(masker *masking.Masker, projectID, repositoryID string, items []collector.Worktree, observed time.Time) []domain.Worktree {
	worktrees := make([]domain.Worktree, 0, len(items))
	for _, item := range items {
		worktrees = append(worktrees, domain.Worktree{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.WorktreeKind}, Metadata: domain.ObjectMeta{ID: item.ID, Name: item.ID}, Spec: domain.WorktreeSpec{ProjectID: projectID, RepositoryID: repositoryID, CanonicalPath: masker.Mask(item.Path), PathFingerprint: worktreePathFingerprint(item.Path), AssociationFingerprint: item.AssociationFingerprint, Trust: item.Trust, Primary: item.Primary, Head: item.Head, Branch: item.Branch, Dirty: item.Dirty, Untracked: item.Untracked, Upstream: item.Upstream, Ahead: item.Ahead, Behind: item.Behind, Detached: item.Detached, Locked: item.Locked, Prunable: item.Prunable, Error: item.Error, LastObserved: observed}})
	}
	return worktrees
}

func worktreePathFingerprint(path string) string {
	sum := sha256.Sum256([]byte(path))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (a *App) Handler() http.Handler { return newHTTPHandler(a, a.listen, a.mutationToken) }

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		start := time.Now()
		next.ServeHTTP(response, request)
		log.Printf("%s %s %s", request.Method, request.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

var _ ApplicationService = (*App)(nil)
