package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/contract"
)

func TestAddProjectTreeRegistersNestedRepositories(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "services", "api")
	second := filepath.Join(root, "services", "web")
	for _, path := range []string{filepath.Join(first, ".git"), filepath.Join(second, ".git")} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	wantFirst := canonicalTestDirectory(t, first)
	wantSecond := canonicalTestDirectory(t, second)
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProjectTree(context.Background(), AddProjectTreeInput{Name: "Workspace", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(project.Spec.Repositories) != 2 || project.Spec.Repositories[0].Spec.Path != wantFirst || project.Spec.Repositories[1].Spec.Path != wantSecond {
		t.Fatalf("registered repositories = %#v", project.Spec.Repositories)
	}
}

func TestRepositoryDiscoveryEndpointUsesProtectedApplicationService(t *testing.T) {
	root := t.TempDir()
	repository := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(repository, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	wantRepository := canonicalTestDirectory(t, repository)
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	body, err := json.Marshal(map[string]string{"path": root})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/projects/discover", bytes.NewReader(body))
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	request.Header.Set("Origin", "http://127.0.0.1:38471")
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("discovery response = %d: %s", recorder.Code, recorder.Body.String())
	}
	var envelope contract.Envelope[[]RepositoryCandidate]
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil || !envelope.OK || len(*envelope.Data) != 1 || (*envelope.Data)[0].Path != wantRepository {
		t.Fatalf("discovery envelope = %#v, err = %v", envelope, err)
	}
}

func TestRepositoryDiscoveryDetailsEndpointRetainsPartialResults(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a-deep")
	current := deep
	for depth := 1; depth <= 10; depth++ {
		if err := os.Mkdir(current, 0o700); err != nil {
			t.Fatal(err)
		}
		current = filepath.Join(current, "level")
	}
	repository := filepath.Join(root, "b-repository")
	if err := os.MkdirAll(filepath.Join(repository, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	wantRepository := canonicalTestDirectory(t, repository)
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	body, err := json.Marshal(map[string]string{"path": root})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/projects/discover-details", bytes.NewReader(body))
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	request.Header.Set("Origin", "http://127.0.0.1:38471")
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("detailed discovery response = %d: %s", recorder.Code, recorder.Body.String())
	}
	var envelope contract.Envelope[RepositoryDiscoveryDetails]
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil || !envelope.OK || envelope.Data == nil {
		t.Fatalf("detailed discovery envelope = %#v, err = %v", envelope, err)
	}
	if !envelope.Data.Partial || len(envelope.Data.Repositories) != 1 || envelope.Data.Repositories[0].Path != wantRepository {
		t.Fatalf("detailed discovery data = %#v, want partial result containing %q", envelope.Data, wantRepository)
	}
	if len(envelope.Data.Warnings) == 0 {
		t.Fatal("detailed discovery returned no bounded-traversal warning")
	}
}

func TestRepositoryDiscoveryDetailsEndpointRejectsInvalidProtection(t *testing.T) {
	root := t.TempDir()
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	tests := []struct {
		name   string
		token  string
		origin string
	}{
		{name: "missing token", origin: "http://127.0.0.1:38471"},
		{name: "cross origin", token: service.mutationToken, origin: "http://example.invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]string{"path": root})
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/projects/discover-details", bytes.NewReader(body))
			request.Header.Set("X-Control-Room-Token", tt.token)
			request.Header.Set("Origin", tt.origin)
			recorder := httptest.NewRecorder()
			service.Handler().ServeHTTP(recorder, request)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("protected details response = %d: %s", recorder.Code, recorder.Body.String())
			}
			var envelope contract.Envelope[RepositoryDiscoveryDetails]
			if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil || envelope.OK || envelope.Error == nil || envelope.Error.Code != contract.ErrorForbidden {
				t.Fatalf("protected details envelope = %#v, err = %v", envelope, err)
			}
		})
	}
}

func TestAddProjectTreeRejectsPartialDiscoveryButAllowsExplicitPaths(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a-deep")
	current := deep
	for depth := 1; depth <= 10; depth++ {
		if err := os.Mkdir(current, 0o700); err != nil {
			t.Fatal(err)
		}
		current = filepath.Join(current, "level")
	}
	repository := filepath.Join(root, "b-repository")
	if err := os.MkdirAll(filepath.Join(repository, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	if _, err := service.AddProjectTree(context.Background(), AddProjectTreeInput{Name: "Partial Workspace", Root: root}); contract.Classify(err).Code != contract.ErrorUnavailable {
		t.Fatalf("partial AddProjectTree err = %v, want unavailable capability", err)
	}
	projects, err := service.Projects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Fatalf("projects after partial discovery = %#v, want none", projects)
	}

	project, err := service.AddProjectTree(context.Background(), AddProjectTreeInput{
		Name:  "Explicit Workspace",
		Root:  root,
		Paths: []string{repository},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(project.Spec.Repositories) != 1 || project.Spec.Repositories[0].Spec.Path != canonicalTestDirectory(t, repository) {
		t.Fatalf("explicit repositories = %#v", project.Spec.Repositories)
	}
}

func TestRepositoryDiscoveryContextCancellationIsNotInvalidInput(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	_, err = service.DiscoverRepositories(ctx, root)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("discovery err = %v, want context.Canceled", err)
	}
	if contract.Classify(err).Code == contract.ErrorInvalidInput {
		t.Fatalf("context cancellation was classified as invalid input: %v", err)
	}
}
