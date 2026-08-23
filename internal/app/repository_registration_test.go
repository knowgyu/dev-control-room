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

func TestAddProjectTreeRegistersNestedRepositories(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "services", "api")
	second := filepath.Join(root, "services", "web")
	for _, path := range []string{filepath.Join(first, ".git"), filepath.Join(second, ".git")} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProjectTree(context.Background(), AddProjectTreeInput{Name: "Workspace", Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(project.Spec.Repositories) != 2 || project.Spec.Repositories[0].Spec.Path != first || project.Spec.Repositories[1].Spec.Path != second {
		t.Fatalf("registered repositories = %#v", project.Spec.Repositories)
	}
}

func TestRepositoryDiscoveryEndpointUsesProtectedApplicationService(t *testing.T) {
	root := t.TempDir()
	repository := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(repository, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
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
	if err := json.NewDecoder(recorder.Body).Decode(&envelope); err != nil || !envelope.OK || len(*envelope.Data) != 1 || (*envelope.Data)[0].Path != repository {
		t.Fatalf("discovery envelope = %#v, err = %v", envelope, err)
	}
}
