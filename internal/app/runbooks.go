package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
)

type AddPowerShellRunbookInput struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	ScriptPath           string   `json:"scriptPath"`
	Parameters           []string `json:"parameters"`
	EnvironmentAllowlist []string `json:"environmentAllowlist"`
	TimeoutSeconds       int      `json:"timeoutSeconds"`
}

type UpdatePowerShellRunbookInput struct {
	Name                 string   `json:"name"`
	ScriptPath           string   `json:"scriptPath"`
	Parameters           []string `json:"parameters"`
	EnvironmentAllowlist []string `json:"environmentAllowlist"`
	TimeoutSeconds       int      `json:"timeoutSeconds"`
}

type PowerShellRunbookPlanInput struct {
	RunbookID    string            `json:"runbookId"`
	ProjectID    string            `json:"projectId"`
	RepositoryID string            `json:"repositoryId"`
	WorktreeID   string            `json:"worktreeId"`
	Parameters   map[string]string `json:"parameters"`
}

func (a *App) Runbooks(_ context.Context) ([]PowerShellRunbookConfig, error) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	items := make([]PowerShellRunbookConfig, len(a.config.Runbooks))
	copy(items, a.config.Runbooks)
	for index := range items {
		items[index].Parameters = append([]string(nil), items[index].Parameters...)
		items[index].EnvironmentAllowlist = append([]string(nil), items[index].EnvironmentAllowlist...)
	}
	return items, nil
}

func (a *App) AddPowerShellRunbook(_ context.Context, input AddPowerShellRunbookInput) (PowerShellRunbookConfig, error) {
	item := PowerShellRunbookConfig{ID: normalizeAppID(input.ID), Name: strings.TrimSpace(input.Name), ScriptPath: strings.TrimSpace(input.ScriptPath), Parameters: append([]string(nil), input.Parameters...), EnvironmentAllowlist: append([]string(nil), input.EnvironmentAllowlist...), TimeoutSeconds: input.TimeoutSeconds}
	if item.ID == "" {
		item.ID = normalizeAppID(item.Name)
	}
	if item.TimeoutSeconds == 0 {
		item.TimeoutSeconds = 300
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	for _, existing := range a.config.Runbooks {
		if existing.ID == item.ID {
			return PowerShellRunbookConfig{}, contract.Conflict("PowerShell runbook already exists")
		}
	}
	if err := validateRunbookConfig(item); err != nil {
		return PowerShellRunbookConfig{}, contract.InvalidInput(err.Error())
	}
	previous := a.config
	a.config.Runbooks = append(append([]PowerShellRunbookConfig(nil), a.config.Runbooks...), item)
	if err := saveConfig(a.home, a.config); err != nil {
		a.config = previous
		return PowerShellRunbookConfig{}, err
	}
	return item, nil
}

func (a *App) UpdatePowerShellRunbook(_ context.Context, id string, input UpdatePowerShellRunbookInput) (PowerShellRunbookConfig, error) {
	id = strings.TrimSpace(id)
	item := PowerShellRunbookConfig{ID: id, Name: strings.TrimSpace(input.Name), ScriptPath: strings.TrimSpace(input.ScriptPath), Parameters: append([]string(nil), input.Parameters...), EnvironmentAllowlist: append([]string(nil), input.EnvironmentAllowlist...), TimeoutSeconds: input.TimeoutSeconds}
	if item.TimeoutSeconds == 0 {
		item.TimeoutSeconds = 300
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	index := -1
	for candidate, existing := range a.config.Runbooks {
		if existing.ID == id {
			index = candidate
			break
		}
	}
	if index < 0 {
		return PowerShellRunbookConfig{}, contract.NotFound("PowerShell runbook not found")
	}
	if err := validateRunbookConfig(item); err != nil {
		return PowerShellRunbookConfig{}, contract.InvalidInput(err.Error())
	}
	previous := a.config
	items := append([]PowerShellRunbookConfig(nil), a.config.Runbooks...)
	items[index] = item
	a.config.Runbooks = items
	if err := saveConfig(a.home, a.config); err != nil {
		a.config = previous
		return PowerShellRunbookConfig{}, err
	}
	return item, nil
}

func (a *App) RemovePowerShellRunbook(_ context.Context, id string) error {
	id = strings.TrimSpace(id)
	a.configMu.Lock()
	defer a.configMu.Unlock()
	index := -1
	for candidate, item := range a.config.Runbooks {
		if item.ID == id {
			index = candidate
			break
		}
	}
	if index < 0 {
		return contract.NotFound("PowerShell runbook not found")
	}
	previous := a.config
	items := append([]PowerShellRunbookConfig(nil), a.config.Runbooks...)
	a.config.Runbooks = append(items[:index], items[index+1:]...)
	if err := saveConfig(a.home, a.config); err != nil {
		a.config = previous
		return err
	}
	return nil
}

func (a *App) PlanPowerShellRunbook(ctx context.Context, input PowerShellRunbookPlanInput) (domain.ActionPlan, error) {
	runbook, err := a.powerShellRunbook(input.RunbookID)
	if err != nil {
		return domain.ActionPlan{}, err
	}
	arguments := make([]string, 0, len(input.Parameters)*2)
	seen := make(map[string]struct{}, len(input.Parameters))
	for key, value := range input.Parameters {
		if !runbookParameterPattern.MatchString(key) || strings.TrimSpace(value) == "" || len(value) > 2048 || strings.ContainsRune(value, '\x00') || isSecretParameter(key) {
			return domain.ActionPlan{}, contract.InvalidInput("runbook parameter names and values must be non-secret and bounded")
		}
		seen[strings.ToLower(key)] = struct{}{}
	}
	for _, parameter := range runbook.Parameters {
		for key, value := range input.Parameters {
			if strings.EqualFold(key, parameter) {
				arguments = append(arguments, "-"+parameter, value)
			}
		}
	}
	for key := range seen {
		found := false
		for _, parameter := range runbook.Parameters {
			if strings.EqualFold(key, parameter) {
				found = true
				break
			}
		}
		if !found {
			return domain.ActionPlan{}, contract.InvalidInput("runbook parameter is not declared")
		}
	}
	argumentsJSON, _ := json.Marshal(arguments)
	environmentJSON, _ := json.Marshal(runbook.EnvironmentAllowlist)
	return a.PlanAction(ctx, ActionPlanInput{ID: fmt.Sprintf("runbook-%s-%d", runbook.ID, time.Now().UnixNano()), Name: runbook.Name, ProjectID: input.ProjectID, RepositoryID: input.RepositoryID, WorktreeID: input.WorktreeID, ActionType: "powershell.runbook", Inputs: map[string]string{"script": runbook.ScriptPath, "arguments": string(argumentsJSON), "environment": string(environmentJSON), "timeout": strconv.Itoa(runbook.TimeoutSeconds)}})
}

func (a *App) powerShellRunbook(id string) (PowerShellRunbookConfig, error) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	for _, item := range a.config.Runbooks {
		if item.ID == strings.TrimSpace(id) {
			item.Parameters = append([]string(nil), item.Parameters...)
			item.EnvironmentAllowlist = append([]string(nil), item.EnvironmentAllowlist...)
			return item, nil
		}
	}
	return PowerShellRunbookConfig{}, contract.NotFound("PowerShell runbook not found")
}

func validateRunbookConfig(item PowerShellRunbookConfig) error {
	config := Config{Version: currentConfigVersion, ScanIntervalSeconds: 10, Runbooks: []PowerShellRunbookConfig{item}}
	return validateConfig(config)
}

func isSecretParameter(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "api_key")
}
