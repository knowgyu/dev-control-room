package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
)

type GitHubLatestRun struct {
	IntegrationID string    `json:"integrationId"`
	Owner         string    `json:"owner"`
	Repository    string    `json:"repository"`
	Workflow      string    `json:"workflow"`
	RunID         int64     `json:"runId"`
	Status        string    `json:"status"`
	Conclusion    string    `json:"conclusion,omitempty"`
	Branch        string    `json:"branch,omitempty"`
	URL           string    `json:"url,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	CheckedAt     time.Time `json:"checkedAt"`
}

func (a *App) GitHubLatestRun(ctx context.Context, id string) (GitHubLatestRun, error) {
	integration, err := a.integration(id)
	if err != nil {
		return GitHubLatestRun{}, err
	}
	if integration.Kind != IntegrationGitHub {
		return GitHubLatestRun{}, contract.InvalidInput("integration is not GitHub")
	}
	owner, repository, workflow := integration.Values["owner"], integration.Values["repository"], integration.Values["workflow"]
	if owner == "" || repository == "" || workflow == "" {
		return GitHubLatestRun{}, contract.InvalidInput("GitHub integration requires owner, repository, and workflow values")
	}
	credential, present, credentialErr := integrationCredential(integration.CredentialRef)
	if credentialErr != nil {
		return GitHubLatestRun{}, contract.CodedError{Code: contract.ErrorUnavailable, Message: credentialErr.Error()}
	}
	endpoint := strings.TrimRight(integration.Endpoint, "/") + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repository) + "/actions/workflows/" + url.PathEscape(workflow) + "/runs?per_page=1"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return GitHubLatestRun{}, contract.InvalidInput("GitHub endpoint is invalid")
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	if present {
		request.Header.Set("Authorization", "Bearer "+credential)
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return GitHubLatestRun{}, contract.CodedError{Code: contract.ErrorUnavailable, Message: "GitHub workflow run could not be reached"}
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return GitHubLatestRun{}, contract.CodedError{Code: contract.ErrorUnavailable, Message: fmt.Sprintf("GitHub workflow run returned HTTP %d", response.StatusCode)}
	}
	var payload struct {
		Runs []struct {
			ID         int64     `json:"id"`
			Status     string    `json:"status"`
			Conclusion string    `json:"conclusion"`
			Branch     string    `json:"head_branch"`
			URL        string    `json:"html_url"`
			CreatedAt  time.Time `json:"created_at"`
		} `json:"workflow_runs"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 256<<10))
	if err := decoder.Decode(&payload); err != nil {
		return GitHubLatestRun{}, contract.CodedError{Code: contract.ErrorUnavailable, Message: "GitHub workflow response was invalid"}
	}
	if len(payload.Runs) == 0 {
		return GitHubLatestRun{IntegrationID: integration.ID, Owner: owner, Repository: repository, Workflow: workflow, CheckedAt: time.Now().UTC()}, nil
	}
	run := payload.Runs[0]
	return GitHubLatestRun{IntegrationID: integration.ID, Owner: owner, Repository: repository, Workflow: workflow, RunID: run.ID, Status: run.Status, Conclusion: run.Conclusion, Branch: run.Branch, URL: run.URL, CreatedAt: run.CreatedAt, CheckedAt: time.Now().UTC()}, nil
}

func (a *App) integration(id string) (IntegrationConfig, error) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	for _, item := range a.config.Integrations {
		if item.ID == strings.TrimSpace(id) {
			return cloneIntegration(item), nil
		}
	}
	return IntegrationConfig{}, contract.NotFound("integration not found")
}
