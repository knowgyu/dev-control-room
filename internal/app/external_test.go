package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

func TestParseJenkinsCompletedBuildURLPreservesBaseAndNestedJob(t *testing.T) {
	parsed, err := parseJenkinsBuildURL("https://jenkins.example.invalid/jenkins/job/folder/job/sample/42/console/api/json")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.BaseURL != "https://jenkins.example.invalid/jenkins" || parsed.Job != "folder/sample" {
		t.Fatalf("parsed Jenkins URL = %#v", parsed)
	}
	if got := parsed.BuildEndpoint(map[string]string{"Environment": "stage"}); got != "https://jenkins.example.invalid/jenkins/job/folder/job/sample/buildWithParameters" {
		t.Fatalf("build endpoint = %q", got)
	}
}

func TestExternalJenkinsGroupPlansBindCredentialReferenceAndRejectStaleChange(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	repository := tempGitRepository(t, "external-plan")
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "External", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddIntegration(context.Background(), AddIntegrationInput{ID: "jenkins", Name: "Fixture Jenkins", Kind: IntegrationJenkins, Endpoint: server.URL + "/jenkins", CredentialRef: "env:FIXTURE_JENKINS_TOKEN", Values: map[string]string{"username": "fixture-user"}}); err != nil {
		t.Fatal(err)
	}
	group := ExternalWorkGroupConfig{ID: "release", Name: "Fixture release", Targets: []ExternalJenkinsTargetConfig{
		{ID: "stage", IntegrationID: "jenkins", CompletedBuildURL: server.URL + "/jenkins/job/folder/job/stage/41/console", Parameters: map[string]string{"Environment": "stage"}},
		{ID: "production", IntegrationID: "jenkins", CompletedBuildURL: server.URL + "/jenkins/job/folder/job/production/41/console"},
	}}
	if _, err := service.AddExternalWorkGroup(context.Background(), group); err != nil {
		t.Fatal(err)
	}
	worktrees, err := service.Worktrees(context.Background(), project.Metadata.ID, "repo-1")
	if err != nil || len(worktrees) != 1 {
		t.Fatalf("worktrees = %#v, %v", worktrees, err)
	}
	if _, err := service.UpdateIntegration(context.Background(), "jenkins", UpdateIntegrationInput{Name: "Fixture Jenkins", Kind: IntegrationJenkins, Endpoint: server.URL, CredentialRef: "env:FIXTURE_JENKINS_TOKEN", Values: map[string]string{"username": "fixture-user"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PlanExternalWork(context.Background(), ExternalWorkPlanInput{GroupID: group.ID, ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: worktrees[0].Metadata.ID}); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched Jenkins endpoint error = %v", err)
	}
	if _, err := service.UpdateIntegration(context.Background(), "jenkins", UpdateIntegrationInput{Name: "Fixture Jenkins", Kind: IntegrationJenkins, Endpoint: server.URL + "/jenkins", CredentialRef: "env:FIXTURE_JENKINS_TOKEN", Values: map[string]string{"username": "fixture-user"}}); err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanExternalWork(context.Background(), ExternalWorkPlanInput{GroupID: group.ID, ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: worktrees[0].Metadata.ID})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Targets[0].UsernameReference != "env:JENKINS_USERNAME" || plan.Targets[0].CredentialReference != "env:FIXTURE_JENKINS_TOKEN" {
		t.Fatalf("plan credential references = %#v", plan.Targets[0])
	}
	encoded, _ := json.Marshal(plan)
	if strings.Contains(string(encoded), "FIXTURE_JENKINS_TOKEN_VALUE") || !strings.Contains(string(encoded), "env:FIXTURE_JENKINS_TOKEN") {
		t.Fatalf("plan credential boundary = %s", encoded)
	}
	if _, err := service.UpdateIntegration(context.Background(), "jenkins", UpdateIntegrationInput{Name: "Fixture Jenkins", Kind: IntegrationJenkins, Endpoint: server.URL + "/jenkins", CredentialRef: "env:CHANGED_JENKINS_TOKEN", Values: map[string]string{"username": "fixture-user"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ExternalWorkPlan(context.Background(), plan.ActionPlan.Metadata.ID); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale group plan error = %v", err)
	}
}

func TestExternalJenkinsGroupTracksQueueBuildAndPartialFailure(t *testing.T) {
	const credential = "fixture-jenkins-token"
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, password, ok := request.BasicAuth()
		if !ok || password != credential {
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/job/stage/buildWithParameters"):
			response.Header().Set("Location", server.URL+"/queue/item/1")
			response.WriteHeader(http.StatusCreated)
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/job/production/build"):
			response.Header().Set("Location", server.URL+"/queue/item/2")
			response.WriteHeader(http.StatusCreated)
		case request.Method == http.MethodGet && request.URL.Path == "/queue/item/1/api/json":
			_, _ = response.Write([]byte(`{"executable":{"number":42}}`))
		case request.Method == http.MethodGet && request.URL.Path == "/queue/item/2/api/json":
			_, _ = response.Write([]byte(`{"executable":{"number":43}}`))
		case request.Method == http.MethodGet && request.URL.Path == "/jenkins/job/folder/job/stage/42/api/json":
			_, _ = response.Write([]byte(`{"number":42,"building":false,"result":"SUCCESS","url":"` + server.URL + `/jenkins/job/folder/job/stage/42"}`))
		case request.Method == http.MethodGet && request.URL.Path == "/jenkins/job/folder/job/production/43/api/json":
			_, _ = response.Write([]byte(`{"number":43,"building":false,"result":"FAILURE","url":"` + server.URL + `/jenkins/job/folder/job/production/43"}`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("FIXTURE_JENKINS_USERNAME", "fixture-user")
	t.Setenv("FIXTURE_JENKINS_TOKEN", credential)
	service, err := New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	repository := tempGitRepository(t, "external-execute")
	project, err := service.AddProject(context.Background(), AddProjectInput{Name: "External", Path: repository})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunScan(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddIntegration(context.Background(), AddIntegrationInput{ID: "jenkins", Name: "Fixture Jenkins", Kind: IntegrationJenkins, Endpoint: server.URL + "/jenkins", CredentialRef: "env:FIXTURE_JENKINS_TOKEN", Values: map[string]string{"username_ref": "env:FIXTURE_JENKINS_USERNAME"}}); err != nil {
		t.Fatal(err)
	}
	group := ExternalWorkGroupConfig{ID: "release", Name: "Fixture release", Targets: []ExternalJenkinsTargetConfig{
		{ID: "stage", IntegrationID: "jenkins", CompletedBuildURL: server.URL + "/jenkins/job/folder/job/stage/41/console", Parameters: map[string]string{"Environment": "stage"}},
		{ID: "production", IntegrationID: "jenkins", CompletedBuildURL: server.URL + "/jenkins/job/folder/job/production/41/console"},
	}}
	if _, err := service.AddExternalWorkGroup(context.Background(), group); err != nil {
		t.Fatal(err)
	}
	worktrees, _ := service.Worktrees(context.Background(), project.Metadata.ID, "repo-1")
	plan, err := service.PlanExternalWork(context.Background(), ExternalWorkPlanInput{GroupID: group.ID, ProjectID: project.Metadata.ID, RepositoryID: "repo-1", WorktreeID: worktrees[0].Metadata.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.TrustActionWorktree(context.Background(), plan.ActionPlan.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	digest, _ := plan.ActionPlan.Digest()
	expires := time.Now().UTC().Add(time.Hour)
	approval := domain.Approval{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ApprovalKind}, Metadata: domain.ObjectMeta{ID: "external-approval", Name: "Fixture approval"}, Spec: domain.ApprovalSpec{ActionPlanID: plan.ActionPlan.Metadata.ID, ActionPlanDigest: digest, Status: domain.ApprovalGranted, RequestedBy: plan.ActionPlan.Spec.RequestedBy, ApprovedBy: &domain.Actor{Kind: domain.ActorHuman, ID: "local-user"}, ExpiresAt: &expires, DecidedAt: time.Now().UTC()}}
	if err := service.store.SaveApprovalAndActionEvent(context.Background(), approval, externalActionEvent(plan.ActionPlan, "approval_granted", "local-user", time.Now().UTC(), "approval"), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	result, err := service.ExecuteExternalWork(context.Background(), plan.ActionPlan.Metadata.ID, "fixture", "external-run-1")
	if err == nil || result.Status != "failed" || len(result.Outcomes) != 2 {
		t.Fatalf("partial group result = %#v, err = %v", result, err)
	}
	if result.Outcomes[0].Status != "succeeded" || result.Outcomes[0].BuildNumber != 42 || result.Outcomes[1].Status == "succeeded" || result.Outcomes[1].Result != "FAILURE" || result.Outcomes[1].Failure == "" {
		t.Fatalf("group outcomes = %#v", result.Outcomes)
	}
	if _, err := service.ExternalWorkResult(context.Background(), plan.ActionPlan.Metadata.ID); err != nil {
		t.Fatal(err)
	}
	actionStatus, err := service.ActionStatus(context.Background(), plan.ActionPlan.Metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	seenTargetEvent := false
	for _, event := range actionStatus.Events {
		if strings.HasPrefix(event.Spec.EventType, "external_target_") {
			seenTargetEvent = true
			break
		}
	}
	if !seenTargetEvent {
		t.Fatalf("target audit event missing: %#v", actionStatus.Events)
	}
}
