package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/collector"
	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/masking"
)

func TestDiscoveryProposalsAreReadOnlyReviewableAndStale(t *testing.T) {
	home := t.TempDir()
	repository := tempGitRepository(t, "discovery")
	if err := os.WriteFile(filepath.Join(repository, "package.json"), []byte(`{"scripts":{"test":"go test ./..."}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := New(home, "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Discovery", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	discovered, err := service.Discover(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil || len(discovered.Spec.ProposalIDs) != 1 {
		t.Fatalf("discover = %#v, %v", discovered, err)
	}
	proposals, err := service.Proposals(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil || len(proposals) != 1 || proposals[0].Spec.Command != "npm run test" || proposals[0].Spec.State != domain.ProposalPending {
		t.Fatalf("proposals = %#v, %v", proposals, err)
	}
	if _, err := service.ApplyProposal(context.Background(), proposals[0].Metadata.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyProposal(context.Background(), proposals[0].Metadata.ID); contract.Classify(err).Code != contract.ErrorInvalidInput {
		t.Fatalf("second review = %v", err)
	}

	if err := os.WriteFile(filepath.Join(repository, "package.json"), []byte(`{"scripts":{"lint":"go vet ./..."}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	discovered, err = service.Discover(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil || len(discovered.Spec.ProposalIDs) != 1 {
		t.Fatalf("second discovery = %#v, %v", discovered, err)
	}
	items, err := service.Proposals(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil || len(items) != 2 {
		t.Fatalf("second proposals = %#v, %v", items, err)
	}
	var pending domain.Proposal
	for _, item := range items {
		if item.Spec.State == domain.ProposalPending {
			pending = item
		}
	}
	if pending.Metadata.ID == "" {
		t.Fatalf("missing pending proposal: %#v", items)
	}
	if err := os.WriteFile(filepath.Join(repository, "package.json"), []byte(`{"scripts":{"lint":"go test ./..."}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	stale, err := service.Proposal(context.Background(), pending.Metadata.ID)
	if err != nil || stale.Spec.State != domain.ProposalStale {
		t.Fatalf("stale proposal = %#v, %v", stale, err)
	}
	persisted, err := service.store.GetProposal(context.Background(), pending.Metadata.ID)
	if err != nil || persisted.Spec.State != domain.ProposalPending {
		t.Fatalf("read-only stale lookup persisted state: %#v, %v", persisted, err)
	}
	if _, err := service.ApplyProposal(context.Background(), pending.Metadata.ID); contract.Classify(err).Code != contract.ErrorInvalidInput {
		t.Fatalf("stale proposal review = %v", err)
	}
	persisted, err = service.store.GetProposal(context.Background(), pending.Metadata.ID)
	if err != nil || persisted.Spec.State != domain.ProposalStale {
		t.Fatalf("authorized stale review did not persist stale state: %#v, %v", persisted, err)
	}

	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/projects/"+project.Metadata.ID+"/repositories/repo-1/proposals?worktree_id=primary", nil))
	var response contract.Envelope[[]domain.Proposal]
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &response) != nil || response.Data == nil || len(*response.Data) != 2 {
		t.Fatalf("proposal HTTP surface = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestDiscoveryHTTPRequiresTokenAndUsesSharedService(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	repository := tempGitRepository(t, "discovery-http")
	if err := os.WriteFile(filepath.Join(repository, "package.json"), []byte(`{"scripts":{"test":"go test ./..."}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Discovery HTTP", Path: repository})
	if err != nil || service.RunScan(context.Background(), "manual") != nil {
		t.Fatalf("fixture setup: %v", err)
	}
	path := "/api/projects/" + project.Metadata.ID + "/repositories/repo-1/worktrees/primary/discover"
	recorder := httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, nil))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unprotected discovery response = %d %s", recorder.Code, recorder.Body.String())
	}
	request := httptest.NewRequest(http.MethodPost, path, nil)
	request.Header.Set("X-Control-Room-Token", service.mutationToken)
	recorder = httptest.NewRecorder()
	service.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated || !strings.Contains(recorder.Body.String(), "proposalIds") {
		t.Fatalf("discovery response = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestDiscoveryMasksProposalCommandBeforePersistence(t *testing.T) {
	const secret = "proposal-secret-canary"
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	service.masker = masking.New([]string{secret}, nil)
	repository := tempGitRepository(t, "discovery-mask")
	if err := os.WriteFile(filepath.Join(repository, "package.json"), []byte(`{"scripts":{"test":"echo `+secret+`"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Discovery Mask", Path: repository})
	if err != nil || service.RunScan(context.Background(), "manual") != nil {
		t.Fatalf("fixture setup: %v", err)
	}
	if _, err := service.Discover(context.Background(), project.Metadata.ID, "repo-1", "primary"); err != nil {
		t.Fatal(err)
	}
	items, err := service.Proposals(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil || len(items) != 1 || strings.Contains(items[0].Spec.Command, secret) {
		t.Fatalf("proposal surface leaked secret: %#v, %v", items, err)
	}
	var sourcePath, raw string
	if err := service.store.DB().QueryRow(`SELECT source_path, object_json FROM proposals WHERE id = ?`, items[0].Metadata.ID).Scan(&sourcePath, &raw); err != nil || strings.Contains(sourcePath, secret) || strings.Contains(raw, secret) {
		t.Fatalf("proposal persistence leaked secret: %q %q, %v", sourcePath, raw, err)
	}
}

func TestDiscoveryRevalidatesPersistedWorktreeAssociation(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	repository := tempGitRepository(t, "discovery-association")
	if err := os.WriteFile(filepath.Join(repository, "package.json"), []byte(`{"scripts":{"test":"go test ./..."}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Discovery Association", Path: repository})
	if err != nil || service.RunScan(context.Background(), "manual") != nil {
		t.Fatalf("fixture setup: %v", err)
	}
	worktree, err := service.store.GetWorktree(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil || worktree.Spec.AssociationFingerprint == "" {
		t.Fatalf("persisted primary association = %#v, %v", worktree, err)
	}
	worktree.Spec.AssociationFingerprint = "sha256:wrong-association"
	data, err := json.Marshal(worktree)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.store.DB().Exec(`UPDATE worktrees SET object_json = ? WHERE project_id = ? AND repository_id = ? AND id = ?`, string(data), project.Metadata.ID, "repo-1", "primary"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Discover(context.Background(), project.Metadata.ID, "repo-1", "primary"); contract.Classify(err).Code != contract.ErrorUnavailable {
		t.Fatalf("mismatched worktree association discovery = %v", err)
	}
}

func TestProposalReviewHasOneConcurrentWinnerAndOneEvent(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	repository := tempGitRepository(t, "discovery-review")
	if err := os.WriteFile(filepath.Join(repository, "package.json"), []byte(`{"scripts":{"test":"go test ./..."}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Discovery Review", Path: repository})
	if err != nil || service.RunScan(context.Background(), "manual") != nil {
		t.Fatalf("fixture setup: %v", err)
	}
	discovered, err := service.Discover(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	errs := make(chan error, 2)
	group.Add(2)
	go func() {
		defer group.Done()
		_, err := service.ApplyProposal(context.Background(), discovered.Spec.ProposalIDs[0])
		errs <- err
	}()
	go func() {
		defer group.Done()
		_, err := service.RejectProposal(context.Background(), discovered.Spec.ProposalIDs[0])
		errs <- err
	}()
	group.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent review successes = %d", successes)
	}
	events, err := service.Events(context.Background(), 20)
	if err != nil {
		t.Fatal(err)
	}
	reviews := 0
	for _, event := range events {
		if event.Spec.Data["proposal_id"] == discovered.Spec.ProposalIDs[0] {
			reviews++
		}
	}
	if reviews != 1 {
		t.Fatalf("proposal review events = %d", reviews)
	}
}

type unavailableDiscoveryRunner struct{}

func (unavailableDiscoveryRunner) Run(context.Context, string, []string, string) (collector.CommandResult, error) {
	return collector.CommandResult{}, errors.New("temporary git failure")
}

type linkedProofFailureRunner struct {
	linked string
}

func (runner linkedProofFailureRunner) Run(ctx context.Context, executable string, args []string, directory string) (collector.CommandResult, error) {
	if len(args) >= 2 && args[0] == "rev-parse" && args[1] == "--show-toplevel" {
		directoryInfo, directoryErr := os.Stat(directory)
		linkedInfo, linkedErr := os.Stat(runner.linked)
		if directoryErr == nil && linkedErr == nil && os.SameFile(directoryInfo, linkedInfo) {
			return collector.CommandResult{ExitCode: 1}, errors.New("temporary linked worktree proof failure")
		}
	}
	return (collector.ProcessRunner{}).Run(ctx, executable, args, directory)
}

func TestProposalReadDoesNotStaleOnTemporaryRevalidationFailure(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	repository := tempGitRepository(t, "discovery-unavailable")
	if err := os.WriteFile(filepath.Join(repository, "package.json"), []byte(`{"scripts":{"test":"go test ./..."}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Discovery Unavailable", Path: repository})
	if err != nil || service.RunScan(context.Background(), "manual") != nil {
		t.Fatalf("fixture setup: %v", err)
	}
	discovered, err := service.Discover(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil {
		t.Fatal(err)
	}
	service.collector = collector.NewGitCollector(unavailableDiscoveryRunner{})
	if _, err := service.Proposal(context.Background(), discovered.Spec.ProposalIDs[0]); contract.Classify(err).Code != contract.ErrorUnavailable {
		t.Fatalf("temporary revalidation failure = %v", err)
	}
	persisted, err := service.store.GetProposal(context.Background(), discovered.Spec.ProposalIDs[0])
	if err != nil || persisted.Spec.State != domain.ProposalPending {
		t.Fatalf("temporary failure changed proposal lifecycle: %#v, %v", persisted, err)
	}
}

func TestProposalReadDoesNotStaleOnIncompleteWorktreeEnumeration(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	repository := tempGitRepository(t, "discovery-enumeration")
	if err := os.WriteFile(filepath.Join(repository, "package.json"), []byte(`{"scripts":{"test":"go test ./..."}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Discovery Enumeration", Path: repository})
	if err != nil || service.RunScan(context.Background(), "manual") != nil {
		t.Fatalf("fixture setup: %v", err)
	}
	discovered, err := service.Discover(context.Background(), project.Metadata.ID, "repo-1", "primary")
	if err != nil {
		t.Fatal(err)
	}
	service.collector = collector.NewGitCollector(worktreeListFailureRunner{directory: repository})
	if _, err := service.Proposal(context.Background(), discovered.Spec.ProposalIDs[0]); contract.Classify(err).Code != contract.ErrorUnavailable {
		t.Fatalf("incomplete enumeration = %v", err)
	}
	persisted, err := service.store.GetProposal(context.Background(), discovered.Spec.ProposalIDs[0])
	if err != nil || persisted.Spec.State != domain.ProposalPending {
		t.Fatalf("incomplete enumeration changed proposal lifecycle: %#v, %v", persisted, err)
	}
}

func TestLinkedProposalReadDoesNotStaleOnAssociationProofFailure(t *testing.T) {
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	repository := tempGitRepository(t, "discovery-linked-proof")
	linked := filepath.Join(t.TempDir(), "linked")
	gitFixture(t, repository, "worktree", "add", "-b", "linked", linked)
	if err := os.WriteFile(filepath.Join(linked, "package.json"), []byte(`{"scripts":{"test":"go test ./..."}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Discovery Linked Proof", Path: repository})
	if err != nil || service.RunScan(context.Background(), "manual") != nil {
		t.Fatalf("fixture setup: %v", err)
	}
	worktrees, err := service.Worktrees(context.Background(), project.Metadata.ID, "repo-1")
	if err != nil {
		t.Fatal(err)
	}
	linkedID := ""
	for _, worktree := range worktrees {
		if !worktree.Spec.Primary {
			linkedID = worktree.Metadata.ID
		}
	}
	if linkedID == "" {
		t.Fatal("linked worktree was not persisted")
	}
	discovered, err := service.Discover(context.Background(), project.Metadata.ID, "repo-1", linkedID)
	if err != nil {
		t.Fatal(err)
	}
	service.collector = collector.NewGitCollector(linkedProofFailureRunner{linked: linked})
	if _, err := service.Proposal(context.Background(), discovered.Spec.ProposalIDs[0]); contract.Classify(err).Code != contract.ErrorUnavailable {
		t.Fatalf("linked proof failure = %v", err)
	}
	persisted, err := service.store.GetProposal(context.Background(), discovered.Spec.ProposalIDs[0])
	if err != nil || persisted.Spec.State != domain.ProposalPending {
		t.Fatalf("linked proof failure changed proposal lifecycle: %#v, %v", persisted, err)
	}
}
