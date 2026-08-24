package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/contract"
)

func TestIntegrationReferencesPersistWithoutSecretValues(t *testing.T) {
	home := t.TempDir()
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	item, err := service.AddIntegration(context.Background(), AddIntegrationInput{ID: "ci", Name: "Fixture CI", Kind: IntegrationJenkins, Endpoint: "https://ci.example.invalid/", CredentialRef: "env:FIXTURE_CI_TOKEN", Values: map[string]string{"job": "sample-build", "branch": "main"}})
	if err != nil {
		t.Fatal(err)
	}
	if item.CredentialRef != "env:FIXTURE_CI_TOKEN" || item.Values["job"] != "sample-build" {
		t.Fatalf("integration = %#v", item)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	items, err := restarted.Integrations(context.Background())
	if err != nil || len(items) != 1 || items[0].CredentialRef != "env:FIXTURE_CI_TOKEN" {
		t.Fatalf("restarted integrations = %#v, err = %v", items, err)
	}
	data, err := os.ReadFile(filepath.Join(home, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("FIXTURE_CI_TOKEN_VALUE")) {
		t.Fatal("credential value was persisted")
	}
}

func TestIntegrationHTTPMutationsAreProtectedAndRejectSecretValueFields(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	body, _ := json.Marshal(AddIntegrationInput{ID: "github", Name: "GitHub", Kind: IntegrationGitHub, Endpoint: "https://api.github.example.invalid", CredentialRef: "env:GITHUB_TOKEN", Values: map[string]string{"owner": "sample-owner", "token": "must-not-be-here"}})
	request := httptest.NewRequest(http.MethodPost, "/api/integrations", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unprotected integration mutation = %d", recorder.Code)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/integrations", bytes.NewReader(body))
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	recorder = httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("secret integration value = %d: %s", recorder.Code, recorder.Body.String())
	}
	var failure contract.Envelope[map[string]any]
	if err := json.Unmarshal(recorder.Body.Bytes(), &failure); err != nil || failure.Error == nil || failure.Error.Code != contract.ErrorInvalidInput {
		t.Fatalf("invalid integration response = %#v, err = %v", failure, err)
	}
}

func TestCheckIntegrationResolvesEnvironmentReferenceWithoutExposingResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("secret-response-body"))
	}))
	defer server.Close()
	t.Setenv("DEVROOM_FIXTURE_TOKEN", "secret-token-value")
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.AddIntegration(context.Background(), AddIntegrationInput{ID: "health", Name: "Health", Kind: IntegrationGitHub, Endpoint: server.URL, CredentialRef: "env:DEVROOM_FIXTURE_TOKEN"}); err != nil {
		t.Fatal(err)
	}
	health, err := service.CheckIntegration(context.Background(), "health")
	if err != nil || health.Status != "passed" || !health.CredentialPresent || health.HTTPStatus != http.StatusOK {
		t.Fatalf("health = %#v, err = %v", health, err)
	}
	data, _ := json.Marshal(health)
	if bytes.Contains(data, []byte("secret-response-body")) || bytes.Contains(data, []byte("secret-token-value")) {
		t.Fatalf("health exposed secret material: %s", data)
	}
	os.Unsetenv("DEVROOM_FIXTURE_TOKEN")
	health, err = service.CheckIntegration(context.Background(), "health")
	if err != nil || health.Status != "unavailable" || health.CredentialPresent {
		t.Fatalf("missing credential health = %#v, err = %v", health, err)
	}
}
