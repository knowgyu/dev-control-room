package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/masking"
	"github.com/knowgyu/dev-control-room/internal/store"
)

func TestRequireLoopback(t *testing.T) {
	for _, address := range []string{"127.0.0.1:38471", "[::1]:38471"} {
		if err := requireLoopback(address); err != nil {
			t.Fatalf("expected %s to be accepted: %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:38471", "192.168.0.10:38471"} {
		if err := requireLoopback(address); err == nil {
			t.Fatalf("expected %s to be rejected", address)
		}
	}
}

func TestConfigRoundTrip(t *testing.T) {
	home := t.TempDir()
	want := Config{
		Version:             currentConfigVersion,
		ScanIntervalSeconds: 60,
		Projects: []domain.Project{domain.NewProject(
			"backend",
			"Backend",
			[]domain.Repository{domain.NewRepository("repo-1", "Backend", `C:\work\backend`)},
		)},
	}
	if err := saveConfig(home, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadConfig(home)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != currentConfigVersion || len(got.Projects) != 1 || got.Projects[0].Metadata.ID != "backend" {
		t.Fatalf("unexpected config: %#v", got)
	}
}

func TestConfigV1MigrationDropsPersistedMutationToken(t *testing.T) {
	home := t.TempDir()
	legacy := `{
  "version": 1,
  "mutation_token": "secret-token-canary",
  "scan_interval_seconds": 45,
  "workbenches": [{"id":"backend","name":"Backend","repos":["C:\\work\\backend"]}]
}`
	if err := os.WriteFile(filepath.Join(home, "config.json"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig(home)
	if err != nil {
		t.Fatal(err)
	}
	if config.Version != currentConfigVersion || len(config.Projects) != 1 || config.Projects[0].Metadata.ID != "backend" {
		t.Fatalf("unexpected migrated config: %#v", config)
	}
	saved, err := os.ReadFile(filepath.Join(home, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(saved) == legacy || string(saved) == "" || contains(string(saved), "secret-token-canary") || contains(string(saved), "workbenches") {
		t.Fatalf("legacy secret or terminology survived migration: %s", saved)
	}
}

func TestProjectConfigMigrationResumesAfterPartialImport(t *testing.T) {
	home := t.TempDir()
	first := domain.NewProject("first", "First", []domain.Repository{domain.NewRepository("repo-1", "First", t.TempDir())})
	second := domain.NewProject("second", "Second", []domain.Repository{domain.NewRepository("repo-1", "Second", t.TempDir())})
	config := defaultConfig()
	config.Projects = []domain.Project{first, second}
	if err := saveConfig(home, config); err != nil {
		t.Fatal(err)
	}
	database, err := store.Open(context.Background(), filepath.Join(home, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	persistence, err := store.New(database, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := persistence.SaveProject(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := persistence.Close(); err != nil {
		t.Fatal(err)
	}

	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	projects, err := service.Projects(context.Background())
	if err != nil || len(projects) != 2 {
		t.Fatalf("partial config migration did not resume: %#v, %v", projects, err)
	}
	migratedConfig, err := loadConfig(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(migratedConfig.Projects) != 0 {
		t.Fatalf("migrated projects remained in config: %#v", migratedConfig.Projects)
	}
}

func TestHTTPUsesStableEnvelope(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	var response contract.Envelope[Health]
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Schema != contract.EnvelopeSchema || !response.OK || response.Data == nil || response.Data.Contract != contract.EnvelopeSchema {
		t.Fatalf("unexpected envelope: %#v", response)
	}
}

func TestHTTPDoesNotExposeInternalErrorDetails(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeServiceError(recorder, errors.New(`sqlite: no such table: events at C:\private\secret-canary`))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	var response contract.Envelope[map[string]any]
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if response.Error == nil || response.Error.Code != contract.ErrorInternal || response.Error.Message != "internal error" || contains(string(data), "secret-canary") || contains(string(data), "no such table") {
		t.Fatalf("internal details leaked from HTTP envelope: %s", data)
	}
}

func TestEventPersistenceMasksBeforeWriting(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	service.masker = masking.New([]string{"persist-secret-canary"}, nil)
	event := domain.Event{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.EventKind},
		Metadata: domain.ObjectMeta{ID: "event-1", Name: "command result"},
		Spec: domain.EventSpec{
			EventType:  "check.output",
			Summary:    "command completed",
			Data:       map[string]any{"stdout": "persist-secret-canary"},
			OccurredAt: time.Now().UTC(),
		},
	}
	if err := service.recordEvent(event); err != nil {
		t.Fatal(err)
	}
	events, err := service.Events(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Spec.Data["stdout"] != masking.Replacement {
		t.Fatalf("event was not masked before persistence: %#v", events)
	}
}

func contains(value, needle string) bool {
	for index := 0; index+len(needle) <= len(value); index++ {
		if value[index:index+len(needle)] == needle {
			return true
		}
	}
	return false
}
