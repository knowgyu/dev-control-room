package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/scheduler"
)

func TestAgentProfileCRUDPersistsAcrossRestart(t *testing.T) {
	home := t.TempDir()
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	profile, err := service.AddAgentProfile(context.Background(), AddAgentProfileInput{ID: "fixture-agent", Name: "Fixture Agent", Command: "fixture-agent", VersionProbe: []string{"--version"}, TimeoutSeconds: 3, EnvironmentAllowlist: []string{"PATH"}, LaunchMode: domain.AgentLaunchDirect, DataBoundary: domain.AgentBoundaryLocal})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	got, err := restarted.AgentProfile(context.Background(), profile.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Spec.Command != "fixture-agent" || got.Spec.TimeoutSeconds != 3 {
		t.Fatalf("profile did not persist: %#v", got)
	}
	if err := restarted.RemoveAgentProfile(context.Background(), profile.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.AgentProfile(context.Background(), profile.Metadata.ID); contract.Classify(err).Code != contract.ErrorNotFound {
		t.Fatalf("removed profile was not not_found: %v", err)
	}
}

func TestRemovingEveryAgentProfileDoesNotReseedDefaults(t *testing.T) {
	home := t.TempDir()
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	profiles, err := service.AgentProfiles(context.Background())
	if err != nil || len(profiles) == 0 {
		t.Fatalf("default profiles were not initialized: %#v, %v", profiles, err)
	}
	for _, profile := range profiles {
		if err := service.RemoveAgentProfile(context.Background(), profile.Metadata.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	profiles, err = restarted.AgentProfiles(context.Background())
	if err != nil || len(profiles) != 0 {
		t.Fatalf("removed default profiles were unexpectedly reseeded: %#v, %v", profiles, err)
	}
}

func TestEnvironmentHealthStoresMetadataWithoutSecretValue(t *testing.T) {
	const canary = "secret-canary-value"
	if err := os.Setenv("DEVROOM_TEST_SECRET", canary); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("DEVROOM_TEST_SECRET") })
	home := t.TempDir()
	config := defaultConfig()
	config.Environment = []domain.EnvironmentDeclaration{{Name: "DEVROOM_TEST_SECRET", Scope: "process", Purpose: "fixture"}}
	config.Connectors = []domain.ConnectorReference{{ID: "fixture-connector", Name: "Fixture Connector", SecretReference: "env:DEVROOM_TEST_SECRET", LastResult: "not_checked"}}
	if err := saveConfig(home, config); err != nil {
		t.Fatal(err)
	}
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	health, err := service.EnvironmentHealth(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(health)
	if strings.Contains(string(encoded), canary) {
		t.Fatalf("health leaked environment value: %s", encoded)
	}
	var raw string
	if err := service.store.DB().QueryRow(`SELECT object_json FROM environment_health WHERE id = 1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, canary) {
		t.Fatalf("health value persisted: %s", raw)
	}
	assertSQLiteDoesNotContain(t, service.store.DB(), canary)
	if len(health.Connectors) != 1 || !health.Connectors[0].SecretReferencePresent {
		t.Fatalf("connector reference was not diagnosed: %#v", health.Connectors)
	}
	configData, err := os.ReadFile(filepath.Join(home, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(configData), canary) {
		t.Fatalf("secret value reached config: %s", configData)
	}
}

func assertSQLiteDoesNotContain(t *testing.T, db *sql.DB, value string) {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, table := range tables {
		values, err := db.Query(`SELECT * FROM ` + quoteSQLiteIdentifier(table))
		if err != nil {
			t.Fatal(err)
		}
		for values.Next() {
			columns, err := values.Columns()
			if err != nil {
				values.Close()
				t.Fatal(err)
			}
			cells := make([]any, len(columns))
			pointers := make([]any, len(cells))
			for i := range cells {
				pointers[i] = &cells[i]
			}
			if err := values.Scan(pointers...); err != nil {
				values.Close()
				t.Fatal(err)
			}
			for _, cell := range cells {
				var text string
				switch typed := cell.(type) {
				case string:
					text = typed
				case []byte:
					text = string(typed)
				}
				if strings.Contains(text, value) {
					values.Close()
					t.Fatalf("SQLite table %s contains secret value", table)
				}
			}
		}
		if err := values.Err(); err != nil {
			values.Close()
			t.Fatal(err)
		}
		values.Close()
	}
}

func quoteSQLiteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func TestEnvironmentDeclarationFindingsDoNotPersistValues(t *testing.T) {
	home := t.TempDir()
	config := defaultConfig()
	config.Environment = []domain.EnvironmentDeclaration{{Name: "DUPLICATE_FIXTURE", Scope: "user"}, {Name: "DUPLICATE_FIXTURE", Scope: "user"}, {Name: "DUPLICATE_FIXTURE", Scope: "machine"}}
	if err := saveConfig(home, config); err != nil {
		t.Fatal(err)
	}
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	health, err := service.EnvironmentHealth(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(health.Findings) == 0 || health.Environment[0].State != "conflict" {
		t.Fatalf("expected deterministic conflict finding: %#v", health)
	}
}

func TestEnvironmentHTTPStatusIsReadOnlyAndDoctorRequiresMutationToken(t *testing.T) {
	const canary = "secret-canary-value"
	if err := os.Setenv("DEVROOM_HTTP_SECRET", canary); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("DEVROOM_HTTP_SECRET") })
	home := t.TempDir()
	config := defaultConfig()
	config.Environment = []domain.EnvironmentDeclaration{{Name: "DEVROOM_HTTP_SECRET", Scope: "process"}}
	if err := saveConfig(home, config); err != nil {
		t.Fatal(err)
	}
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/environment?refresh=true", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected environment response status: %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), canary) {
		t.Fatalf("secret value leaked through HTTP: %s", recorder.Body.String())
	}
	var persisted int
	if err := service.store.DB().QueryRow(`SELECT count(*) FROM environment_health`).Scan(&persisted); err != nil {
		t.Fatal(err)
	}
	if persisted != 0 {
		t.Fatal("GET environment status ran and persisted a doctor refresh")
	}
	unauthorized := httptest.NewRecorder()
	service.Handler().ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, "/api/environment/doctor", nil))
	if unauthorized.Code != http.StatusForbidden {
		t.Fatalf("environment doctor accepted a request without mutation token: %d", unauthorized.Code)
	}
	authorizedRequest := httptest.NewRequest(http.MethodPost, "/api/environment/doctor", nil)
	authorizedRequest.Header.Set("X-Control-Room-Token", service.mutationToken)
	authorized := httptest.NewRecorder()
	service.Handler().ServeHTTP(authorized, authorizedRequest)
	if authorized.Code != http.StatusOK || strings.Contains(authorized.Body.String(), canary) {
		t.Fatalf("authorized environment doctor failed or leaked a secret: %d %s", authorized.Code, authorized.Body.String())
	}
}

func TestSchedulerStatusUsesTypedAdapterAndPersistsFakeState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native Task Scheduler mutations require explicit authorization")
	}
	home := t.TempDir()
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	operation, err := scheduler.Plan(scheduler.OperationInstall, `C:\Program Files\DevControlRoom\devroom.exe`, []string{"serve", "--home", `C:\Users\Fixture\AppData\Local\DevControlRoom`})
	if err != nil {
		t.Fatal(err)
	}
	installed, err := service.Schedule(context.Background(), operation)
	if err != nil || installed.Applied || !installed.Exists {
		t.Fatalf("fake install was not a non-mutating typed operation: %#v, %v", installed, err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	statusOperation, err := scheduler.Plan(scheduler.OperationStatus, operation.ExecutablePath, operation.Arguments)
	if err != nil {
		t.Fatal(err)
	}
	status, err := restarted.Schedule(context.Background(), statusOperation)
	if err != nil || !status.Exists || status.Applied {
		t.Fatalf("typed fake status did not report persisted state: %#v, %v", status, err)
	}
	if _, err := scheduler.Plan(scheduler.OperationDryRun, operation.ExecutablePath, []string{"powershell", "-Command", "Write-Host unsafe"}); err == nil {
		t.Fatal("generic shell arguments were accepted by scheduler plan")
	}
}
