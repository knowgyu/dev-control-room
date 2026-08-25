package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

var externalParameterPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,63}$`)

func validateExternalWorkGroup(group ExternalWorkGroupConfig) error {
	if !validConfigID(group.ID) || strings.TrimSpace(group.Name) == "" || len(group.Targets) < 2 || len(group.Targets) > 32 {
		return errors.New("group requires a valid id, name, and two to 32 targets")
	}
	seen := make(map[string]struct{}, len(group.Targets))
	for _, target := range group.Targets {
		if !validConfigID(target.ID) || !validConfigID(target.IntegrationID) {
			return errors.New("target requires valid id and integration id")
		}
		if _, exists := seen[target.ID]; exists {
			return errors.New("target ids must be unique")
		}
		seen[target.ID] = struct{}{}
		if _, err := parseJenkinsBuildURL(target.CompletedBuildURL); err != nil {
			return fmt.Errorf("target %q: %w", target.ID, err)
		}
		if len(target.Parameters) > 32 {
			return errors.New("target has too many parameters")
		}
		for key, value := range target.Parameters {
			lower := strings.ToLower(key)
			if !externalParameterPattern.MatchString(key) || strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "api_key") || strings.TrimSpace(value) == "" || len(value) > 1024 || strings.ContainsAny(value, "\x00\r\n") {
				return errors.New("target parameters must be bounded non-secret values")
			}
		}
		if target.FallbackRunbookID != "" && !validConfigID(target.FallbackRunbookID) {
			return errors.New("fallback runbook id is invalid")
		}
	}
	return nil
}

func cloneExternalGroup(group ExternalWorkGroupConfig) ExternalWorkGroupConfig {
	group.Targets = append([]ExternalJenkinsTargetConfig(nil), group.Targets...)
	for index := range group.Targets {
		group.Targets[index].Parameters = cloneValues(group.Targets[index].Parameters)
	}
	return group
}

func (a *App) ExternalWorkGroups(_ context.Context) ([]ExternalWorkGroupConfig, error) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	items := make([]ExternalWorkGroupConfig, len(a.config.ExternalWorkGroups))
	for index, group := range a.config.ExternalWorkGroups {
		items[index] = cloneExternalGroup(group)
	}
	return items, nil
}

func (a *App) AddExternalWorkGroup(_ context.Context, input ExternalWorkGroupConfig) (ExternalWorkGroupConfig, error) {
	item := cloneExternalGroup(input)
	item.ID = strings.TrimSpace(item.ID)
	item.Name = strings.TrimSpace(item.Name)
	if err := validateExternalWorkGroup(item); err != nil {
		return ExternalWorkGroupConfig{}, contract.InvalidInput(err.Error())
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	for _, existing := range a.config.ExternalWorkGroups {
		if existing.ID == item.ID {
			return ExternalWorkGroupConfig{}, contract.Conflict("external work group already exists")
		}
	}
	previous := a.config
	a.config.ExternalWorkGroups = append(append([]ExternalWorkGroupConfig(nil), a.config.ExternalWorkGroups...), item)
	if err := saveConfig(a.home, a.config); err != nil {
		a.config = previous
		return ExternalWorkGroupConfig{}, err
	}
	return cloneExternalGroup(item), nil
}

func (a *App) UpdateExternalWorkGroup(_ context.Context, id string, input ExternalWorkGroupConfig) (ExternalWorkGroupConfig, error) {
	item := cloneExternalGroup(input)
	item.ID = strings.TrimSpace(id)
	item.Name = strings.TrimSpace(item.Name)
	if err := validateExternalWorkGroup(item); err != nil {
		return ExternalWorkGroupConfig{}, contract.InvalidInput(err.Error())
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	index := -1
	for candidate, existing := range a.config.ExternalWorkGroups {
		if existing.ID == item.ID {
			index = candidate
			break
		}
	}
	if index < 0 {
		return ExternalWorkGroupConfig{}, contract.NotFound("external work group not found")
	}
	previous := a.config
	items := append([]ExternalWorkGroupConfig(nil), a.config.ExternalWorkGroups...)
	items[index] = item
	a.config.ExternalWorkGroups = items
	if err := saveConfig(a.home, a.config); err != nil {
		a.config = previous
		return ExternalWorkGroupConfig{}, err
	}
	return cloneExternalGroup(item), nil
}

func (a *App) RemoveExternalWorkGroup(_ context.Context, id string) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	index := -1
	for candidate, group := range a.config.ExternalWorkGroups {
		if group.ID == strings.TrimSpace(id) {
			index = candidate
			break
		}
	}
	if index < 0 {
		return contract.NotFound("external work group not found")
	}
	previous := a.config
	items := append([]ExternalWorkGroupConfig(nil), a.config.ExternalWorkGroups...)
	a.config.ExternalWorkGroups = append(items[:index], items[index+1:]...)
	if err := saveConfig(a.home, a.config); err != nil {
		a.config = previous
		return err
	}
	return nil
}

func (a *App) externalGroup(id string) (ExternalWorkGroupConfig, error) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	for _, group := range a.config.ExternalWorkGroups {
		if group.ID == strings.TrimSpace(id) {
			return cloneExternalGroup(group), nil
		}
	}
	return ExternalWorkGroupConfig{}, contract.NotFound("external work group not found")
}

func (a *App) externalGroupDigest(group ExternalWorkGroupConfig) (string, error) {
	integrations := make([]IntegrationConfig, len(group.Targets))
	for index, target := range group.Targets {
		integration, err := a.integration(target.IntegrationID)
		if err != nil {
			return "", err
		}
		if integration.Kind != IntegrationJenkins {
			return "", contract.InvalidInput("external work target integration must be Jenkins")
		}
		parsed, parseErr := parseJenkinsBuildURL(target.CompletedBuildURL)
		if parseErr != nil {
			return "", contract.InvalidInput(parseErr.Error())
		}
		if strings.TrimRight(strings.TrimSpace(integration.Endpoint), "/") != parsed.BaseURL {
			return "", contract.InvalidInput("Jenkins target URL does not match its configured integration endpoint")
		}
		integrations[index] = cloneIntegration(integration)
	}
	payload := struct {
		Group        ExternalWorkGroupConfig `json:"group"`
		Integrations []IntegrationConfig     `json:"integrations"`
	}{Group: cloneExternalGroup(group), Integrations: integrations}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func (a *App) PlanExternalWork(ctx context.Context, input ExternalWorkPlanInput) (ExternalWorkGroupPlan, error) {
	group, err := a.externalGroup(input.GroupID)
	if err != nil {
		return ExternalWorkGroupPlan{}, err
	}
	digest, err := a.externalGroupDigest(group)
	if err != nil {
		return ExternalWorkGroupPlan{}, classifyActionError(err)
	}
	targets := make([]ExternalJenkinsTargetPlan, len(group.Targets))
	for index, target := range group.Targets {
		parsed, parseErr := parseJenkinsBuildURL(target.CompletedBuildURL)
		if parseErr != nil {
			return ExternalWorkGroupPlan{}, contract.InvalidInput(parseErr.Error())
		}
		integration, integrationErr := a.integration(target.IntegrationID)
		if integrationErr != nil {
			return ExternalWorkGroupPlan{}, integrationErr
		}
		targets[index] = ExternalJenkinsTargetPlan{ID: target.ID, IntegrationID: target.IntegrationID, UsernameReference: jenkinsUsernameReference(integration), CredentialReference: jenkinsCredentialReference(integration), BaseURL: parsed.BaseURL, Job: parsed.Job, BuildEndpoint: parsed.BuildEndpoint(target.Parameters), Parameters: cloneValues(target.Parameters)}
	}
	plan, err := a.PlanAction(ctx, ActionPlanInput{ID: fmt.Sprintf("external-%s-%d", group.ID, time.Now().UnixNano()), Name: group.Name, ProjectID: input.ProjectID, RepositoryID: input.RepositoryID, WorktreeID: input.WorktreeID, ActionType: "external.jenkins.group", Inputs: map[string]string{"group_id": group.ID, "group_digest": digest}})
	if err != nil {
		return ExternalWorkGroupPlan{}, err
	}
	return ExternalWorkGroupPlan{ActionPlan: plan, Group: group, Digest: digest, Targets: targets, CreatedAt: plan.Spec.RequestedAt}, nil
}

func (a *App) ExternalWorkPlan(ctx context.Context, planID string) (ExternalWorkGroupPlan, error) {
	plan, err := a.store.GetActionPlan(ctx, planID)
	if err != nil {
		return ExternalWorkGroupPlan{}, classifyActionError(err)
	}
	if plan.Spec.ActionType != "external.jenkins.group" {
		return ExternalWorkGroupPlan{}, contract.InvalidInput("action plan is not an external Jenkins group")
	}
	group, err := a.externalGroup(plan.Spec.Inputs["group_id"])
	if err != nil {
		return ExternalWorkGroupPlan{}, err
	}
	digest, err := a.externalGroupDigest(group)
	if err != nil {
		return ExternalWorkGroupPlan{}, classifyActionError(err)
	}
	if digest != plan.Spec.Inputs["group_digest"] {
		return ExternalWorkGroupPlan{}, contract.CodedError{Code: contract.ErrorConflict, Message: "external work group changed; the plan is stale"}
	}
	preview, err := a.externalTargetPlans(group)
	if err != nil {
		return ExternalWorkGroupPlan{}, contract.InvalidInput(err.Error())
	}
	return ExternalWorkGroupPlan{ActionPlan: plan, Group: group, Digest: digest, Targets: preview, CreatedAt: plan.Spec.RequestedAt}, nil
}

func (a *App) externalTargetPlans(group ExternalWorkGroupConfig) ([]ExternalJenkinsTargetPlan, error) {
	targets := make([]ExternalJenkinsTargetPlan, len(group.Targets))
	for index, target := range group.Targets {
		parsed, err := parseJenkinsBuildURL(target.CompletedBuildURL)
		if err != nil {
			return nil, err
		}
		integration, integrationErr := a.integration(target.IntegrationID)
		if integrationErr != nil {
			return nil, integrationErr
		}
		targets[index] = ExternalJenkinsTargetPlan{ID: target.ID, IntegrationID: target.IntegrationID, UsernameReference: jenkinsUsernameReference(integration), CredentialReference: jenkinsCredentialReference(integration), BaseURL: parsed.BaseURL, Job: parsed.Job, BuildEndpoint: parsed.BuildEndpoint(target.Parameters), Parameters: cloneValues(target.Parameters)}
	}
	return targets, nil
}

func (a *App) ExternalWorkResult(_ context.Context, planID string) (ExternalWorkGroupResult, error) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	for _, record := range a.config.ExternalWorkResults {
		if record.PlanID == planID {
			return record.Result, nil
		}
	}
	return ExternalWorkGroupResult{}, contract.NotFound("external work result not found")
}

func (a *App) ExecuteExternalWork(ctx context.Context, planID, holder, idempotencyKey string) (ExternalWorkGroupResult, error) {
	groupPlan, err := a.ExternalWorkPlan(ctx, planID)
	if err != nil {
		return ExternalWorkGroupResult{}, err
	}
	admission, err := a.broker.Admit(ctx, planID, holder, idempotencyKey)
	if err != nil {
		return ExternalWorkGroupResult{}, classifyActionError(err)
	}
	if _, changed, checkErr := a.discoveryWorktree(ctx, groupPlan.ActionPlan.Spec.ProjectID, groupPlan.ActionPlan.Spec.RepositoryID, groupPlan.ActionPlan.Spec.WorktreeID); checkErr != nil || changed {
		_ = a.broker.Release(context.Background(), admission)
		return ExternalWorkGroupResult{}, contract.Conflict("external worktree evidence changed; the plan is stale")
	}
	started := time.Now().UTC()
	_ = a.store.SaveActionEvent(ctx, externalActionEvent(groupPlan.ActionPlan, "external_group_started", holder, started, "started"))
	result := runJenkinsGroup(ctx, a, groupPlan, started)
	result.PlanID = planID
	saveExternalTargetEvents(ctx, a, groupPlan.ActionPlan, holder, result)
	_ = a.store.SaveActionEvent(ctx, externalActionEvent(groupPlan.ActionPlan, "external_group_"+result.Status, holder, result.CompletedAt, result.Status))
	_ = a.broker.Release(context.Background(), admission)
	if saveErr := a.saveExternalWorkResult(result); saveErr != nil {
		return result, saveErr
	}
	if result.Status != "succeeded" {
		return result, contract.CodedError{Code: contract.ErrorExecutionFailed, Message: "one or more external targets did not complete"}
	}
	return result, nil
}

func (a *App) saveExternalWorkResult(result ExternalWorkGroupResult) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	records := append([]ExternalWorkResultRecord(nil), a.config.ExternalWorkResults...)
	updated := false
	for index := range records {
		if records[index].PlanID == result.PlanID {
			records[index] = ExternalWorkResultRecord{PlanID: result.PlanID, Result: result}
			updated = true
			break
		}
	}
	if !updated {
		records = append(records, ExternalWorkResultRecord{PlanID: result.PlanID, Result: result})
	}
	if len(records) > 128 {
		records = records[len(records)-128:]
	}
	previous := a.config
	a.config.ExternalWorkResults = records
	if err := saveConfig(a.home, a.config); err != nil {
		a.config = previous
		return err
	}
	return nil
}

type jenkinsBuildTarget struct {
	BaseURL string
	Job     string
}

func parseJenkinsBuildURL(raw string) (jenkinsBuildTarget, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.User != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return jenkinsBuildTarget{}, errors.New("completed Jenkins build URL must be an http(s) URL without credentials or query values")
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	firstJob := -1
	jobs := []string{}
	for index := 0; index < len(segments); {
		if segments[index] != "job" || index+1 >= len(segments) || strings.TrimSpace(segments[index+1]) == "" {
			index++
			continue
		}
		if firstJob < 0 {
			firstJob = index
		}
		job, decodeErr := url.PathUnescape(segments[index+1])
		if decodeErr != nil || strings.ContainsAny(job, "\x00/\\") {
			return jenkinsBuildTarget{}, errors.New("completed Jenkins build URL contains an invalid job segment")
		}
		jobs = append(jobs, job)
		index += 2
	}
	if firstJob < 0 || len(jobs) == 0 {
		return jenkinsBuildTarget{}, errors.New("completed Jenkins build URL must contain one or more /job/name segments")
	}
	base := strings.TrimRight(parsed.Scheme+"://"+parsed.Host+"/"+strings.Join(segments[:firstJob], "/"), "/")
	return jenkinsBuildTarget{BaseURL: base, Job: strings.Join(jobs, "/")}, nil
}

func (t jenkinsBuildTarget) jobURL() string {
	result := t.BaseURL
	for _, segment := range strings.Split(t.Job, "/") {
		result += "/job/" + url.PathEscape(segment)
	}
	return result
}

func (t jenkinsBuildTarget) BuildEndpoint(parameters map[string]string) string {
	path := "/build"
	if len(parameters) > 0 {
		path = "/buildWithParameters"
	}
	return t.jobURL() + path
}

func runJenkinsGroup(ctx context.Context, app *App, groupPlan ExternalWorkGroupPlan, started time.Time) ExternalWorkGroupResult {
	result := ExternalWorkGroupResult{PlanID: groupPlan.ActionPlan.Metadata.ID, Status: "succeeded", Outcomes: make([]ExternalWorkTargetResult, len(groupPlan.Group.Targets))}
	sem := make(chan struct{}, 2) // ponytail: two provider calls per group; raise only after measuring service limits.
	var wait sync.WaitGroup
	for index, target := range groupPlan.Group.Targets {
		index, target := index, target
		wait.Add(1)
		go func() {
			defer wait.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			result.Outcomes[index] = app.runJenkinsTarget(ctx, target, started)
		}()
	}
	wait.Wait()
	for _, outcome := range result.Outcomes {
		if outcome.Status != "succeeded" {
			result.Status = "failed"
			break
		}
	}
	result.CompletedAt = time.Now().UTC()
	return result
}

func (a *App) runJenkinsTarget(ctx context.Context, target ExternalJenkinsTargetConfig, started time.Time) ExternalWorkTargetResult {
	outcome := ExternalWorkTargetResult{TargetID: target.ID, Status: "failed", StartedAt: started, CompletedAt: time.Now().UTC()}
	integration, err := a.integration(target.IntegrationID)
	if err == nil {
		parsed, parseErr := parseJenkinsBuildURL(target.CompletedBuildURL)
		if parseErr == nil {
			build, triggerErr := triggerJenkins(ctx, integration, parsed, target.Parameters)
			if triggerErr == nil {
				outcome.BuildNumber, outcome.BuildURL, outcome.Result = build.Number, safeJenkinsBuildURL(build.URL), build.Result
				if strings.EqualFold(build.Result, "SUCCESS") {
					outcome.Status = "succeeded"
				} else {
					outcome.Failure = fmt.Sprintf("Jenkins build completed with result %q", build.Result)
				}
			} else {
				outcome.Failure = triggerErr.Error()
			}
		} else {
			outcome.Failure = parseErr.Error()
		}
	} else {
		outcome.Failure = "Jenkins integration is unavailable"
	}
	outcome.CompletedAt = time.Now().UTC()
	return outcome
}

func safeJenkinsBuildURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.User != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ""
	}
	return parsed.String()
}

func jenkinsCredentialReference(integration IntegrationConfig) string {
	if strings.TrimSpace(integration.CredentialRef) != "" {
		return integration.CredentialRef
	}
	return "env:JENKINS_TOKEN"
}

func jenkinsUsernameReference(integration IntegrationConfig) string {
	if reference := strings.TrimSpace(integration.Values["username_ref"]); reference != "" {
		return reference
	}
	return "env:JENKINS_USERNAME"
}

type jenkinsBuildResult struct {
	Number int64
	URL    string
	Result string
}

func triggerJenkins(ctx context.Context, integration IntegrationConfig, target jenkinsBuildTarget, parameters map[string]string) (jenkinsBuildResult, error) {
	credentialRef := jenkinsCredentialReference(integration)
	credential, present, err := integrationCredential(credentialRef)
	if err != nil || !present {
		return jenkinsBuildResult{}, errors.New("Jenkins credential reference is unavailable")
	}
	username := strings.TrimSpace(integration.Values["username"])
	if reference := strings.TrimSpace(integration.Values["username_ref"]); reference != "" {
		resolved, present, resolveErr := integrationCredential(reference)
		if resolveErr != nil || !present {
			return jenkinsBuildResult{}, errors.New("Jenkins username reference is unavailable")
		}
		username = resolved
	} else if username == "" {
		resolved, present, resolveErr := integrationCredential(jenkinsUsernameReference(integration))
		if resolveErr != nil || !present {
			return jenkinsBuildResult{}, errors.New("Jenkins username reference is unavailable")
		}
		username = resolved
	}
	if username == "" {
		return jenkinsBuildResult{}, errors.New("Jenkins username reference is unavailable")
	}
	requestBody := io.Reader(nil)
	if len(parameters) > 0 {
		values := url.Values{}
		for key, value := range parameters {
			values.Set(key, value)
		}
		requestBody = strings.NewReader(values.Encode())
	}
	endpoint := target.BuildEndpoint(parameters)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, requestBody)
	if err != nil {
		return jenkinsBuildResult{}, errors.New("Jenkins build endpoint is invalid")
	}
	if len(parameters) > 0 {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	request.SetBasicAuth(username, credential)
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return jenkinsBuildResult{}, errors.New("Jenkins build trigger could not be reached")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return jenkinsBuildResult{}, fmt.Errorf("Jenkins build trigger returned HTTP %d", response.StatusCode)
	}
	queueURL := response.Header.Get("Location")
	if queueURL == "" {
		return jenkinsBuildResult{}, errors.New("Jenkins build trigger did not return a queue location")
	}
	return pollJenkinsQueue(ctx, queueURL, target, username, credential)
}

func pollJenkinsQueue(ctx context.Context, queueURL string, target jenkinsBuildTarget, username, credential string) (jenkinsBuildResult, error) {
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return jenkinsBuildResult{}, errors.New("Jenkins queue wait was cancelled")
		case <-deadline.C:
			return jenkinsBuildResult{}, errors.New("Jenkins queue wait timed out")
		case <-ticker.C:
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(queueURL, "/")+"/api/json", nil)
			if err != nil {
				return jenkinsBuildResult{}, errors.New("Jenkins queue URL is invalid")
			}
			request.SetBasicAuth(username, credential)
			response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
			if err != nil {
				continue
			}
			var payload struct {
				Cancelled  bool `json:"cancelled"`
				Executable *struct {
					Number int64 `json:"number"`
				} `json:"executable"`
			}
			decodeErr := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&payload)
			_ = response.Body.Close()
			if decodeErr != nil || response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
				continue
			}
			if payload.Cancelled {
				return jenkinsBuildResult{}, errors.New("Jenkins queue item was cancelled")
			}
			if payload.Executable != nil && payload.Executable.Number > 0 {
				return pollJenkinsBuild(ctx, target, payload.Executable.Number, username, credential)
			}
		}
	}
}

func pollJenkinsBuild(ctx context.Context, target jenkinsBuildTarget, number int64, username, credential string) (jenkinsBuildResult, error) {
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	endpoint := fmt.Sprintf("%s/%d/api/json", target.jobURL(), number)
	for {
		select {
		case <-ctx.Done():
			return jenkinsBuildResult{}, errors.New("Jenkins build wait was cancelled")
		case <-deadline.C:
			return jenkinsBuildResult{}, errors.New("Jenkins build wait timed out")
		case <-ticker.C:
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
			if err != nil {
				return jenkinsBuildResult{}, errors.New("Jenkins build URL is invalid")
			}
			request.SetBasicAuth(username, credential)
			response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
			if err != nil {
				continue
			}
			var payload struct {
				Number   int64  `json:"number"`
				Building bool   `json:"building"`
				Result   string `json:"result"`
				URL      string `json:"url"`
			}
			decodeErr := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&payload)
			_ = response.Body.Close()
			if decodeErr != nil || response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
				continue
			}
			if !payload.Building {
				if payload.Number == 0 {
					payload.Number = number
				}
				return jenkinsBuildResult{Number: payload.Number, URL: payload.URL, Result: payload.Result}, nil
			}
		}
	}
}

func externalActionEvent(plan domain.ActionPlan, eventType, holder string, at time.Time, nonce string) domain.ActionEvent {
	digest, _ := plan.Digest()
	sum := sha256.Sum256([]byte(plan.Metadata.ID + "\x00" + eventType + "\x00" + holder + "\x00" + nonce))
	return domain.ActionEvent{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.ActionEventKind}, Metadata: domain.ObjectMeta{ID: "action-" + hex.EncodeToString(sum[:])[:57], Name: eventType}, Spec: domain.ActionEventSpec{ActionPlanID: plan.Metadata.ID, ActionPlanDigest: digest, EventType: eventType, Actor: domain.Actor{Kind: domain.ActorSystem, ID: holder}, OccurredAt: at}}
}

func saveExternalTargetEvents(ctx context.Context, app *App, plan domain.ActionPlan, holder string, result ExternalWorkGroupResult) {
	for _, outcome := range result.Outcomes {
		eventType := "external_target_" + outcome.TargetID + "_" + outcome.Status
		_ = app.store.SaveActionEvent(ctx, externalActionEvent(plan, eventType, holder, outcome.CompletedAt, outcome.TargetID))
	}
}
