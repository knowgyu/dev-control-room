package app

import (
	"context"
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

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/masking"
)

var safeID = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

type App struct {
	home          string
	listen        string
	config        Config
	mutationToken string
	ledger        *eventLedger
	masker        *masking.Masker
	mu            sync.RWMutex
	snapshot      Snapshot
	scanNow       chan string
}

func New(home, listen string) (*App, error) {
	if err := requireLoopback(listen); err != nil {
		return nil, err
	}
	config, err := loadConfig(home)
	if err != nil {
		return nil, err
	}
	return &App{
		home:          home,
		listen:        listen,
		config:        config,
		mutationToken: randomToken(),
		ledger:        newEventLedger(home),
		masker:        masking.New(nil, []string{"TOKEN", "PASSWORD", "SECRET", "API_KEY", "AUTHORIZATION"}),
		scanNow:       make(chan string, 1),
	}, nil
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

func (a *App) RunDoctor(ctx context.Context) {
	a.scan(ctx, "startup")
	interval := time.Duration(a.config.ScanIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.scan(ctx, "schedule")
		case trigger := <-a.scanNow:
			a.scan(ctx, trigger)
		}
	}
}

func (a *App) scan(ctx context.Context, trigger string) {
	a.mu.RLock()
	projects := append([]domain.Project(nil), a.config.Projects...)
	a.mu.RUnlock()

	now := time.Now().UTC()
	snapshot := Snapshot{GeneratedAt: now, Projects: make([]ProjectState, 0, len(projects))}
	for _, project := range projects {
		state := ProjectState{ID: project.Metadata.ID, Name: project.Metadata.Name, ScannedAt: now}
		for _, repository := range project.Spec.Repositories {
			state.Repos = append(state.Repos, scanRepo(ctx, repository.Spec.Path))
		}
		snapshot.Projects = append(snapshot.Projects, state)
	}

	a.mu.Lock()
	a.snapshot = snapshot
	a.mu.Unlock()
	a.recordEvent(domain.Event{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.EventKind},
		Metadata: domain.ObjectMeta{ID: fmt.Sprintf("scan-%d", now.UnixNano()), Name: "diagnosis scan"},
		Spec: domain.EventSpec{
			EventType:  "diagnosis.scan.completed",
			Actor:      "service",
			Summary:    fmt.Sprintf("%s scan completed for %d project(s)", trigger, len(projects)),
			Data:       map[string]any{"trigger": trigger, "project_count": len(projects)},
			OccurredAt: now,
		},
	})
}

func (a *App) Health(context.Context) Health {
	a.mu.RLock()
	version := a.config.Version
	a.mu.RUnlock()
	return Health{
		OK:            true,
		Service:       "dev-control-room",
		NetworkMode:   "loopback-only",
		Telemetry:     false,
		Contract:      contract.EnvelopeSchema,
		ConfigVersion: version,
	}
}

func (a *App) Snapshot(context.Context) Snapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.snapshot
}

func (a *App) Events(_ context.Context, limit int) ([]domain.Event, error) {
	return a.ledger.recent(limit)
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

func (a *App) AddProject(_ context.Context, input AddProjectInput) (domain.Project, error) {
	path, err := filepath.Abs(strings.TrimSpace(input.Path))
	if err != nil {
		return domain.Project{}, contract.InvalidInput(err.Error())
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return domain.Project{}, contract.InvalidInput("path must be an existing directory")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = filepath.Base(path)
	}
	id := normalizeAppID(name)
	if id == "" {
		return domain.Project{}, contract.InvalidInput("project name must contain letters or numbers")
	}
	repository := domain.NewRepository("repo-1", name, path)
	project := domain.NewProject(id, name, []domain.Repository{repository})

	a.mu.Lock()
	defer a.mu.Unlock()
	for _, item := range a.config.Projects {
		if item.Metadata.ID == id || (len(item.Spec.Repositories) == 1 && strings.EqualFold(item.Spec.Repositories[0].Spec.Path, path)) {
			return domain.Project{}, contract.Conflict("project already exists")
		}
	}
	a.config.Projects = append(a.config.Projects, project)
	if err := saveConfig(a.home, a.config); err != nil {
		return domain.Project{}, err
	}
	_ = a.recordEvent(domain.Event{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.EventKind},
		Metadata: domain.ObjectMeta{ID: fmt.Sprintf("project-added-%d", time.Now().UnixNano()), Name: "project added"},
		Spec:     domain.EventSpec{EventType: "project.added", Actor: "user", ProjectID: id, Summary: "Project added", OccurredAt: time.Now().UTC()},
	})
	select {
	case a.scanNow <- "project.added":
	default:
	}
	return project, nil
}

func (a *App) RemoveProject(_ context.Context, id string) error {
	a.mu.Lock()
	index := -1
	for current, project := range a.config.Projects {
		if project.Metadata.ID == id {
			index = current
			break
		}
	}
	if index < 0 {
		a.mu.Unlock()
		return contract.NotFound("project not found")
	}
	a.config.Projects = append(a.config.Projects[:index], a.config.Projects[index+1:]...)
	if err := saveConfig(a.home, a.config); err != nil {
		a.mu.Unlock()
		return err
	}
	a.mu.Unlock()
	_ = a.recordEvent(domain.Event{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.EventKind},
		Metadata: domain.ObjectMeta{ID: fmt.Sprintf("project-removed-%d", time.Now().UnixNano()), Name: "project removed"},
		Spec:     domain.EventSpec{EventType: "project.removed", Actor: "user", ProjectID: id, Summary: "Project removed; repository files were not changed", OccurredAt: time.Now().UTC()},
	})
	return nil
}

func (a *App) recordEvent(event domain.Event) error {
	masked, err := maskEvent(a.masker, event)
	if err != nil {
		return err
	}
	return a.ledger.append(masked)
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

func normalizeAppID(value string) string {
	return strings.Trim(safeID.ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), "-"), "-")
}

func (a *App) Handler() http.Handler {
	return newHTTPHandler(a, a.listen, a.mutationToken)
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		start := time.Now()
		next.ServeHTTP(response, request)
		log.Printf("%s %s %s", request.Method, request.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
