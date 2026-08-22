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
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/knowgyu/dev-control-room/internal/collector"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/environment"
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
	mutationToken string
	masker        *masking.Masker
	store         *store.Store
	collector     collector.GitCollector
	doctor        environment.Doctor
	scheduler     scheduler.Adapter
	scanNow       chan string
	scanMu        sync.Mutex
	environmentMu sync.Mutex
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
		store: persistence, collector: collector.NewGitCollector(nil), doctor: environment.NewDoctor(nil, masker), scheduler: scheduler.NewAdapter(), scanNow: make(chan string, 1),
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
				state.Repos = append(state.Repos, RepositoryState{Path: repository.Spec.Path, Worktrees: worktrees, WorktreeCount: len(worktrees)})
				continue
			}
			repositoryState, err := stateFromObservation(observations[0])
			if err != nil {
				return Snapshot{}, err
			}
			repositoryState.Worktrees = worktrees
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
			if !complete {
				state.Error = "one or more worktree states could not be collected"
				collectorFailed = true
			}
			if err := a.store.ReplaceWorktrees(ctx, project.Metadata.ID, repository.Metadata.ID, domainWorktrees(project.Metadata.ID, repository.Metadata.ID, state.Worktrees, state.CollectedAt), state.WorktreeEnumerationComplete); err != nil {
				return collectorFailed, err
			}
		}
		if collectErr != nil {
			collectorFailed = true
		}
		state.Path = repository.Spec.Path
		observation, err := newObservation(project.Metadata.ID, repository.Metadata.ID, state)
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
			if err := a.recordCollectorFailure(ctx, collectErr); err != nil {
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

func (a *App) recordCollectorFailure(ctx context.Context, err error) error {
	data := []byte(err.Error())
	sum := sha256.Sum256(data)
	fingerprint := "sha256:" + hex.EncodeToString(sum[:])
	now := time.Now().UTC()
	item := domain.FailureFingerprint{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.FailureFingerprintKind}, Metadata: domain.ObjectMeta{ID: "failure-" + hex.EncodeToString(sum[:])[:48], Name: "Git collector failure"}, Spec: domain.FailureFingerprintSpec{Fingerprint: fingerprint, Category: "collector.git", FirstSeen: now, LastSeen: now, OccurrenceCount: 1}}
	if prior, lookupErr := a.store.GetFailureFingerprint(ctx, fingerprint); lookupErr == nil {
		item.Spec.FirstSeen = prior.Spec.FirstSeen
		item.Spec.OccurrenceCount = prior.Spec.OccurrenceCount + 1
	} else if !errors.Is(lookupErr, sql.ErrNoRows) {
		return lookupErr
	}
	return a.store.SaveFailureFingerprint(ctx, item)
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

func normalizeAppID(value string) string {
	return strings.Trim(safeID.ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), "-"), "-")
}

func eventID(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano()) }

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

func newObservation(projectID, repositoryID string, state collector.State) (domain.Observation, error) {
	// Paths from Git porcelain are untrusted discovery input. The durable
	// repository observation retains no linked-worktree path; Worktree records
	// expose only a stable path fingerprint.
	persisted := state
	for i := range persisted.Worktrees {
		persisted.Worktrees[i].Path = worktreePathFingerprint(persisted.Worktrees[i].Path)
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

func domainWorktrees(projectID, repositoryID string, items []collector.Worktree, observed time.Time) []domain.Worktree {
	worktrees := make([]domain.Worktree, 0, len(items))
	for _, item := range items {
		worktrees = append(worktrees, domain.Worktree{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.WorktreeKind}, Metadata: domain.ObjectMeta{ID: item.ID, Name: item.ID}, Spec: domain.WorktreeSpec{ProjectID: projectID, RepositoryID: repositoryID, CanonicalPath: worktreePathFingerprint(item.Path), AssociationFingerprint: item.AssociationFingerprint, Trust: item.Trust, Primary: item.Primary, Head: item.Head, Branch: item.Branch, Dirty: item.Dirty, Untracked: item.Untracked, Upstream: item.Upstream, Ahead: item.Ahead, Behind: item.Behind, Detached: item.Detached, Locked: item.Locked, Prunable: item.Prunable, Error: item.Error, LastObserved: observed}})
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
