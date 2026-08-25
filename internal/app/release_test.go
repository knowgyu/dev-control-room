package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestReleasePlansSeparateStageAndProductionPolicy(t *testing.T) {
	service, project, group, worktree := newReleaseFixture(t, http.NotFoundHandler())
	stage, err := service.PlanRelease(context.Background(), ReleasePlanInput{GroupID: group.ID, Environment: "stage", ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: worktree.Metadata.ID})
	if err != nil {
		t.Fatal(err)
	}
	production, err := service.PlanRelease(context.Background(), ReleasePlanInput{GroupID: group.ID, Environment: "production", ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: worktree.Metadata.ID})
	if err != nil {
		t.Fatal(err)
	}
	if stage.ActionPlan.Spec.ActionType != "release.jenkins.stage" || stage.ActionPlan.Spec.Risk != domain.RiskExternalChange || !stage.ActionPlan.Spec.ApprovalRequired {
		t.Fatalf("stage policy = %#v", stage.ActionPlan.Spec)
	}
	if production.ActionPlan.Spec.ActionType != "release.jenkins.production" || production.ActionPlan.Spec.Risk != domain.RiskHighImpact || !production.ActionPlan.Spec.ApprovalRequired {
		t.Fatalf("production policy = %#v", production.ActionPlan.Spec)
	}
	if _, err := service.ExecuteRelease(context.Background(), production.ActionPlan.Metadata.ID, "fixture", "release-without-approval"); err == nil || !strings.Contains(err.Error(), "approval") {
		t.Fatalf("missing production approval error = %v", err)
	}
}

func TestReleaseSuccessfulBuildAndExpectedRevisionPostcheck(t *testing.T) {
	const credential = "fixture-release-token"
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, password, ok := request.BasicAuth()
		if !ok || password != credential {
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodPost && strings.Contains(request.URL.Path, "/build"):
			response.Header().Set("Location", server.URL+"/queue/item/1")
			response.WriteHeader(http.StatusCreated)
		case request.Method == http.MethodGet && request.URL.Path == "/queue/item/1/api/json":
			_, _ = response.Write([]byte(`{"executable":{"number":42}}`))
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/42/api/json"):
			_, _ = response.Write([]byte(`{"number":42,"building":false,"result":"SUCCESS","url":"` + server.URL + `/jenkins/job/folder/job/release/42"}`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("FIXTURE_RELEASE_TOKEN", credential)
	service, project, group, worktree := newReleaseFixture(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusNotFound)
	}))
	_ = service
	_ = project
	_ = group
	_ = worktree
	// Replace the fixture's integration endpoint and group target with the live local test server.
	if _, err := service.UpdateIntegration(context.Background(), "jenkins", UpdateIntegrationInput{Name: "Fixture Jenkins", Kind: IntegrationJenkins, Endpoint: server.URL + "/jenkins", CredentialRef: "env:FIXTURE_RELEASE_TOKEN", Values: map[string]string{"username": "fixture-user"}}); err != nil {
		t.Fatal(err)
	}
	group, err := service.UpdateExternalWorkGroup(context.Background(), group.ID, ExternalWorkGroupConfig{ID: group.ID, Name: group.Name, Targets: []ExternalJenkinsTargetConfig{
		{ID: "stage", IntegrationID: "jenkins", CompletedBuildURL: server.URL + "/jenkins/job/folder/job/release/41/console"},
		{ID: "production", IntegrationID: "jenkins", CompletedBuildURL: server.URL + "/jenkins/job/folder/job/release/41/console"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanRelease(context.Background(), ReleasePlanInput{GroupID: group.ID, Environment: "stage", ExpectedRevision: "revision-not-provided-by-fixture", ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: worktree.Metadata.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.TrustActionWorktree(context.Background(), plan.ActionPlan.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	digest, err := plan.ActionPlan.Digest()
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Now().UTC().Add(time.Hour)
	approval := domain.Approval{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ApprovalKind}, Metadata: domain.ObjectMeta{ID: "release-approval", Name: "Fixture approval"}, Spec: domain.ApprovalSpec{ActionPlanID: plan.ActionPlan.Metadata.ID, ActionPlanDigest: digest, Status: domain.ApprovalGranted, RequestedBy: plan.ActionPlan.Spec.RequestedBy, ApprovedBy: &domain.Actor{Kind: domain.ActorHuman, ID: "local-user"}, ExpiresAt: &expires, DecidedAt: time.Now().UTC()}}
	if err := service.store.SaveApprovalAndActionEvent(context.Background(), approval, externalActionEvent(plan.ActionPlan, "approval_granted", "local-user", time.Now().UTC(), "approval"), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	result, err := service.ExecuteRelease(context.Background(), plan.ActionPlan.Metadata.ID, "fixture", "release-run-1")
	if err == nil || result.Status != "postcheck_failed" || result.External.Status != "succeeded" {
		t.Fatalf("release expected-revision result = %#v, err = %v", result, err)
	}
	if result.Postchecks[0].ID != "successful-build" || result.Postchecks[0].Status != "passed" || result.Postchecks[1].Status != "failed" {
		t.Fatalf("release postchecks = %#v", result.Postchecks)
	}
}

func newReleaseFixture(t *testing.T, handler http.Handler) (*App, domain.Project, ExternalWorkGroupConfig, domain.Worktree) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	repository := tempGitRepository(t, "release-fixture")
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "Release", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddIntegration(context.Background(), AddIntegrationInput{ID: "jenkins", Name: "Fixture Jenkins", Kind: IntegrationJenkins, Endpoint: server.URL + "/jenkins", CredentialRef: "env:FIXTURE_RELEASE_TOKEN", Values: map[string]string{"username": "fixture-user"}}); err != nil {
		t.Fatal(err)
	}
	group := ExternalWorkGroupConfig{ID: "release", Name: "Fixture release", Targets: []ExternalJenkinsTargetConfig{
		{ID: "stage", IntegrationID: "jenkins", CompletedBuildURL: server.URL + "/jenkins/job/folder/job/stage/41/console"},
		{ID: "production", IntegrationID: "jenkins", CompletedBuildURL: server.URL + "/jenkins/job/folder/job/production/41/console"},
	}}
	if _, err := service.AddExternalWorkGroup(context.Background(), group); err != nil {
		t.Fatal(err)
	}
	worktrees, err := service.Worktrees(context.Background(), project.Metadata.ID, "repo-1")
	if err != nil || len(worktrees) != 1 {
		t.Fatalf("worktrees = %#v, %v", worktrees, err)
	}
	return service, project, group, worktrees[0]
}
