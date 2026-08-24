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

func TestGitHubLatestRunUsesReferenceAndReturnsBoundedMetadata(t *testing.T) {
	const credential = "fixture-github-token"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/sample-owner/sample-repository/actions/workflows/checks.yml/runs" || request.URL.Query().Get("per_page") != "1" {
			t.Fatalf("unexpected GitHub request: %s", request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer "+credential {
			t.Fatalf("authorization header = %q", request.Header.Get("Authorization"))
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"workflow_runs":[{"id":42,"status":"completed","conclusion":"success","head_branch":"main","html_url":"https://github.example.invalid/run/42","created_at":"2026-08-25T00:00:00Z"}]}`))
	}))
	defer server.Close()
	t.Setenv("DEVROOM_GITHUB_TOKEN", credential)
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.AddIntegration(context.Background(), AddIntegrationInput{
		ID: "github", Name: "Fixture GitHub", Kind: IntegrationGitHub, Endpoint: server.URL,
		CredentialRef: "env:DEVROOM_GITHUB_TOKEN",
		Values:        map[string]string{"owner": "sample-owner", "repository": "sample-repository", "workflow": "checks.yml"},
	}); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/integrations/github/github/latest-run", nil)
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unprotected latest run = %d", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/integrations/github/github/latest-run", nil)
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	recorder = httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("latest run = %d: %s", recorder.Code, recorder.Body.String())
	}
	var envelope contract.Envelope[GitHubLatestRun]
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error != nil || envelope.Data.RunID != 42 || envelope.Data.Conclusion != "success" || envelope.Data.Branch != "main" {
		t.Fatalf("latest run envelope = %#v", envelope)
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte(credential)) {
		t.Fatal("latest run response exposed credential")
	}
}

func TestGitHubLatestRunIsUnavailableWithoutCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.AddIntegration(context.Background(), AddIntegrationInput{
		ID: "github", Name: "Fixture GitHub", Kind: IntegrationGitHub, Endpoint: server.URL,
		CredentialRef: "env:DEVROOM_MISSING_GITHUB_TOKEN",
		Values:        map[string]string{"owner": "sample-owner", "repository": "sample-repository", "workflow": "checks.yml"},
	}); err != nil {
		t.Fatal(err)
	}
	_, err = service.GitHubLatestRun(context.Background(), "github")
	if contract.Classify(err).Code != contract.ErrorUnavailable {
		t.Fatalf("missing credential error = %v", err)
	}
}

func TestJenkinsLatestBuildSupportsBasicAuthAndReturnsMetadata(t *testing.T) {
	const credential = "fixture-jenkins-password"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/job/folder/job/sample-build/lastBuild/api/json" {
			t.Errorf("unexpected Jenkins request path: %s", request.URL.Path)
		}
		username, password, ok := request.BasicAuth()
		if !ok || username != "fixture-user" || password != credential {
			t.Errorf("basic auth = %q, %q, %t", username, password, ok)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"number":17,"building":false,"result":"SUCCESS","displayName":"#17","url":"https://jenkins.example.invalid/job/17","timestamp":1724544000000}`))
	}))
	defer server.Close()
	t.Setenv("DEVROOM_JENKINS_PASSWORD", credential)
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.AddIntegration(context.Background(), AddIntegrationInput{
		ID: "jenkins", Name: "Fixture Jenkins", Kind: IntegrationJenkins, Endpoint: server.URL,
		CredentialRef: "env:DEVROOM_JENKINS_PASSWORD",
		Values:        map[string]string{"job": "folder/sample-build", "username": "fixture-user"},
	}); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/integrations/jenkins/jenkins/latest-build", nil)
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unprotected latest build = %d", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/integrations/jenkins/jenkins/latest-build", nil)
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	recorder = httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("latest build = %d: %s", recorder.Code, recorder.Body.String())
	}
	var envelope contract.Envelope[JenkinsLatestBuild]
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error != nil || envelope.Data.BuildNumber != 17 || envelope.Data.Result != "SUCCESS" || envelope.Data.Job != "folder/sample-build" {
		t.Fatalf("latest build envelope = %#v", envelope)
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte(credential)) {
		t.Fatal("latest build response exposed credential")
	}
}

func TestKubernetesStatusAndLogsResolvePodAndMaskCredential(t *testing.T) {
	const credential = "fixture-kubernetes-token"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+credential {
			t.Errorf("authorization header = %q", request.Header.Get("Authorization"))
		}
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v1/namespaces/sample-space/pods":
			if request.URL.Query().Get("labelSelector") != "app=sample-api" {
				t.Errorf("label selector = %q", request.URL.Query().Get("labelSelector"))
			}
			_, _ = response.Write([]byte(`{"items":[{"metadata":{"name":"sample-pod"},"status":{"phase":"Running","containerStatuses":[{"ready":true,"restartCount":2}]}}]}`))
		case "/api/v1/namespaces/sample-space/pods/sample-pod/log":
			if request.URL.Query().Get("container") != "api" || request.URL.Query().Get("tailLines") != "25" || request.URL.Query().Get("timestamps") != "true" {
				t.Errorf("log query = %s", request.URL.RawQuery)
			}
			_, _ = response.Write([]byte("2026-08-25T00:00:00Z token=" + credential + "\\nready\\n"))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("DEVROOM_K8S_TOKEN", credential)
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.AddIntegration(context.Background(), AddIntegrationInput{
		ID: "kubernetes", Name: "Fixture Kubernetes", Kind: IntegrationKubernetes, Endpoint: server.URL,
		CredentialRef: "env:DEVROOM_K8S_TOKEN",
		Values:        map[string]string{"namespace": "sample-space", "selector": "app=sample-api", "container": "api", "tail_lines": "25"},
	}); err != nil {
		t.Fatal(err)
	}

	status, err := service.KubernetesStatus(context.Background(), "kubernetes")
	if err != nil || len(status.Pods) != 1 || status.Pods[0].Name != "sample-pod" || !status.Pods[0].Ready || status.Pods[0].RestartCount != 2 {
		t.Fatalf("Kubernetes status = %#v, err = %v", status, err)
	}
	logs, err := service.KubernetesLogs(context.Background(), "kubernetes")
	if err != nil || logs.Pod != "sample-pod" || !bytes.Contains([]byte(logs.Logs), []byte("[REDACTED]")) || bytes.Contains([]byte(logs.Logs), []byte(credential)) {
		t.Fatalf("Kubernetes logs = %#v, err = %v", logs, err)
	}
}
