package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/masking"
)

func TestChecksetRequiresAppliedProposalAndExplicitAppliedState(t *testing.T) {
	service, proposal := checksetFixture(t)
	step := domain.CheckStep{ID: "status", Name: "Git status", Command: domain.CheckCommand{Executable: "git", Arguments: []string{"status", "--porcelain"}, TimeoutSeconds: 30}}
	if _, err := service.CreateCheckset(context.Background(), CreateChecksetInput{ID: "checks-1", Name: "Checks", ProposalID: proposal.Metadata.ID, Steps: []domain.CheckStep{step}}); contract.Classify(err).Code != contract.ErrorInvalidInput {
		t.Fatalf("unreviewed proposal created checkset: %v", err)
	}
	if _, err := service.ApplyProposal(context.Background(), proposal.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateCheckset(context.Background(), CreateChecksetInput{ID: "checks-1", Name: "Checks", ProposalID: proposal.Metadata.ID, Steps: []domain.CheckStep{step}})
	if err != nil || created.Spec.State != domain.ChecksetDraft {
		t.Fatalf("create = %#v, %v", created, err)
	}
	if _, err := service.RunCheckset(context.Background(), created.Metadata.ID); contract.Classify(err).Code != contract.ErrorInvalidInput {
		t.Fatalf("draft ran: %v", err)
	}
	applied, err := service.ApplyCheckset(context.Background(), created.Metadata.ID)
	if err != nil || applied.Spec.State != domain.ChecksetApplied {
		t.Fatalf("apply = %#v, %v", applied, err)
	}
	run, err := service.RunCheckset(context.Background(), created.Metadata.ID)
	if err != nil || run.Spec.Status != domain.CheckPassed || len(run.Spec.Steps) != 1 || run.Spec.Steps[0].Status != domain.CheckPassed {
		t.Fatalf("run = %#v, %v", run, err)
	}
}

func TestChecksetMasksPersistedAndHTTPOutput(t *testing.T) {
	const secret = "checkset-secret-canary"
	service, proposal := checksetFixture(t)
	service.masker = masking.New([]string{secret}, nil)
	if _, err := service.ApplyProposal(context.Background(), proposal.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateCheckset(context.Background(), CreateChecksetInput{ID: "checks-2", Name: "Checks", ProposalID: proposal.Metadata.ID, Steps: []domain.CheckStep{{ID: "secret", Name: "Read fixture value", Command: domain.CheckCommand{Executable: "git", Arguments: []string{"status", "--porcelain"}, TimeoutSeconds: 30}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyCheckset(context.Background(), created.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RunCheckset(context.Background(), created.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/checksets/"+created.Metadata.ID+"/runs", nil)
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || containsSecret(recorder.Body.String(), secret) {
		t.Fatalf("run HTTP leaked secret: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestChecksetRejectsChangedWorktree(t *testing.T) {
	service, proposal := checksetFixture(t)
	if _, err := service.ApplyProposal(context.Background(), proposal.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateCheckset(context.Background(), CreateChecksetInput{ID: "checks-4", Name: "Checks", ProposalID: proposal.Metadata.ID, Steps: []domain.CheckStep{{ID: "status", Name: "Git status", Command: domain.CheckCommand{Executable: "git", Arguments: []string{"status", "--porcelain"}, TimeoutSeconds: 30}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyCheckset(context.Background(), created.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	repository, err := service.Repository(context.Background(), proposal.Spec.ProjectID, proposal.Spec.RepositoryID)
	if err != nil {
		t.Fatal(err)
	}
	gitFixture(t, repository.Spec.Path, "add", ".github/workflows/checks.yml")
	gitFixture(t, repository.Spec.Path, "commit", "-m", "change check source")
	if _, err := service.RunCheckset(context.Background(), created.Metadata.ID); contract.Classify(err).Code != contract.ErrorUnavailable {
		t.Fatalf("changed worktree ran checkset: %v", err)
	}
}

func TestChecksetHTTPMutationRequiresToken(t *testing.T) {
	service, proposal := checksetFixture(t)
	if _, err := service.ApplyProposal(context.Background(), proposal.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(CreateChecksetInput{ID: "checks-3", Name: "Checks", ProposalID: proposal.Metadata.ID, Steps: []domain.CheckStep{{ID: "status", Name: "Git status", Command: domain.CheckCommand{Executable: "git", Arguments: []string{"status", "--porcelain"}, TimeoutSeconds: 30}}}})
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/checksets", bytesReader(body)))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unprotected checkset create = %d", recorder.Code)
	}
}

func TestChecksetRejectsMismatchedAndMutatingCommands(t *testing.T) {
	service, proposal := checksetFixture(t)
	if _, err := service.ApplyProposal(context.Background(), proposal.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	for _, command := range []domain.CheckCommand{
		{Executable: "git", Arguments: []string{"diff", "--check"}, TimeoutSeconds: 30},
		{Executable: "git", Arguments: []string{"reset", "--hard"}, TimeoutSeconds: 30},
	} {
		_, err := service.CreateCheckset(context.Background(), CreateChecksetInput{ID: "checks-mismatch", Name: "Checks", ProposalID: proposal.Metadata.ID, Steps: []domain.CheckStep{{ID: "status", Name: "Status", Command: command}}})
		if contract.Classify(err).Code != contract.ErrorInvalidInput {
			t.Fatalf("command %#v accepted: %v", command, err)
		}
	}
}

func checksetFixture(t *testing.T) (*App, domain.Proposal) {
	t.Helper()
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	repository := tempGitRepository(t, "checkset")
	gitFixture(t, repository, "config", "devroom.canary", "checkset-secret-canary")
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Checks", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repository, ".github", "workflows"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, ".github", "workflows", "checks.yml"), []byte("jobs:\n  checks:\n    steps:\n      - run: git status --porcelain\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	discovered, err := service.Discover(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil || len(discovered.Spec.ProposalIDs) != 1 {
		t.Fatalf("discover = %#v, %v", discovered, err)
	}
	proposal, err := service.Proposal(context.Background(), discovered.Spec.ProposalIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Spec.TypedCommand == nil {
		t.Fatal("normal discovery did not create reviewed typed command")
	}
	return service, proposal
}

func containsSecret(value, secret string) bool { return strings.Contains(value, secret) }
func bytesReader(value []byte) *bytes.Reader   { return bytes.NewReader(value) }
