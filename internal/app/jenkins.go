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

type JenkinsLatestBuild struct {
	IntegrationID string    `json:"integrationId"`
	Job           string    `json:"job"`
	BuildNumber   int64     `json:"buildNumber"`
	Building      bool      `json:"building"`
	Result        string    `json:"result,omitempty"`
	DisplayName   string    `json:"displayName,omitempty"`
	URL           string    `json:"url,omitempty"`
	StartedAt     time.Time `json:"startedAt"`
	CheckedAt     time.Time `json:"checkedAt"`
}

func (a *App) JenkinsLatestBuild(ctx context.Context, id string) (JenkinsLatestBuild, error) {
	integration, err := a.integration(id)
	if err != nil {
		return JenkinsLatestBuild{}, err
	}
	if integration.Kind != IntegrationJenkins {
		return JenkinsLatestBuild{}, contract.InvalidInput("integration is not Jenkins")
	}
	job := strings.Trim(integration.Values["job"], "/")
	if job == "" {
		return JenkinsLatestBuild{}, contract.InvalidInput("Jenkins integration requires a job value")
	}
	credential, present, credentialErr := integrationCredential(integration.CredentialRef)
	if credentialErr != nil {
		return JenkinsLatestBuild{}, contract.CodedError{Code: contract.ErrorUnavailable, Message: credentialErr.Error()}
	}
	endpoint := strings.TrimRight(integration.Endpoint, "/")
	for _, segment := range strings.Split(job, "/") {
		if segment == "" {
			continue
		}
		endpoint += "/job/" + url.PathEscape(segment)
	}
	endpoint += "/lastBuild/api/json"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return JenkinsLatestBuild{}, contract.InvalidInput("Jenkins endpoint is invalid")
	}
	request.Header.Set("Accept", "application/json")
	if present {
		if username := strings.TrimSpace(integration.Values["username"]); username != "" {
			request.SetBasicAuth(username, credential)
		} else {
			request.Header.Set("Authorization", "Bearer "+credential)
		}
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return JenkinsLatestBuild{}, contract.CodedError{Code: contract.ErrorUnavailable, Message: "Jenkins latest build could not be reached"}
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return JenkinsLatestBuild{}, contract.CodedError{Code: contract.ErrorUnavailable, Message: fmt.Sprintf("Jenkins latest build returned HTTP %d", response.StatusCode)}
	}
	var payload struct {
		Number      int64  `json:"number"`
		Building    bool   `json:"building"`
		Result      string `json:"result"`
		DisplayName string `json:"displayName"`
		URL         string `json:"url"`
		Timestamp   int64  `json:"timestamp"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 256<<10)).Decode(&payload); err != nil {
		return JenkinsLatestBuild{}, contract.CodedError{Code: contract.ErrorUnavailable, Message: "Jenkins latest build response was invalid"}
	}
	startedAt := time.Time{}
	if payload.Timestamp > 0 {
		startedAt = time.UnixMilli(payload.Timestamp).UTC()
	}
	return JenkinsLatestBuild{IntegrationID: integration.ID, Job: job, BuildNumber: payload.Number, Building: payload.Building, Result: payload.Result, DisplayName: payload.DisplayName, URL: payload.URL, StartedAt: startedAt, CheckedAt: time.Now().UTC()}, nil
}
