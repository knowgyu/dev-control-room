package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
)

type AddIntegrationInput struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Kind          IntegrationKind   `json:"kind"`
	Endpoint      string            `json:"endpoint"`
	CredentialRef string            `json:"credentialRef,omitempty"`
	Values        map[string]string `json:"values,omitempty"`
}

type UpdateIntegrationInput struct {
	Name          string            `json:"name"`
	Kind          IntegrationKind   `json:"kind"`
	Endpoint      string            `json:"endpoint"`
	CredentialRef string            `json:"credentialRef,omitempty"`
	Values        map[string]string `json:"values,omitempty"`
}

func (a *App) Integrations(_ context.Context) ([]IntegrationConfig, error) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	items := make([]IntegrationConfig, len(a.config.Integrations))
	for index, item := range a.config.Integrations {
		items[index] = cloneIntegration(item)
	}
	return items, nil
}

func (a *App) CheckIntegration(ctx context.Context, id string) (IntegrationHealth, error) {
	a.configMu.Lock()
	var integration *IntegrationConfig
	for index := range a.config.Integrations {
		if a.config.Integrations[index].ID == strings.TrimSpace(id) {
			item := cloneIntegration(a.config.Integrations[index])
			integration = &item
			break
		}
	}
	a.configMu.Unlock()
	if integration == nil {
		return IntegrationHealth{}, contract.NotFound("integration not found")
	}
	checkedAt := time.Now().UTC()
	health := IntegrationHealth{ID: integration.ID, Kind: integration.Kind, Endpoint: integration.Endpoint, CredentialReference: integration.CredentialRef, Status: "unavailable", CheckedAt: checkedAt}
	credential, present, err := integrationCredential(integration.CredentialRef)
	health.CredentialPresent = present
	if err != nil {
		health.Message = err.Error()
		return health, nil
	}
	client := &http.Client{Timeout: 10 * time.Second}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, integration.Endpoint, nil)
	if err != nil {
		health.Status = "failed"
		health.Message = "연동 주소를 요청할 수 없습니다."
		return health, nil
	}
	request.Header.Set("Accept", "application/json")
	if credential != "" {
		request.Header.Set("Authorization", "Bearer "+credential)
	}
	response, err := client.Do(request)
	if err != nil {
		health.Status = "failed"
		health.Message = "연동 주소에 연결하지 못했습니다."
		return health, nil
	}
	defer response.Body.Close()
	health.HTTPStatus = response.StatusCode
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		health.Status = "passed"
		health.Message = "연동 주소에 연결했습니다. 응답 본문은 저장하거나 노출하지 않습니다."
		return health, nil
	}
	health.Status = "failed"
	health.Message = "연동 주소가 오류 상태를 반환했습니다. 응답 본문은 저장하거나 노출하지 않습니다."
	return health, nil
}

func integrationCredential(reference string) (string, bool, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return "", false, nil
	}
	if strings.HasPrefix(strings.ToLower(reference), "credential_manager:") {
		return "", false, errors.New("Windows Credential Manager 연동은 아직 사용할 수 없습니다.")
	}
	if !strings.HasPrefix(strings.ToLower(reference), "env:") {
		return "", false, errors.New("지원하지 않는 credential reference입니다.")
	}
	name := strings.TrimSpace(reference[len("env:"):])
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		return "", false, errors.New("credential 환경 변수가 설정되지 않았습니다.")
	}
	return value, true, nil
}

func (a *App) AddIntegration(_ context.Context, input AddIntegrationInput) (IntegrationConfig, error) {
	item := IntegrationConfig{ID: strings.TrimSpace(input.ID), Name: strings.TrimSpace(input.Name), Kind: input.Kind, Endpoint: strings.TrimSpace(input.Endpoint), CredentialRef: strings.TrimSpace(input.CredentialRef), Values: cloneValues(input.Values)}
	if err := validateIntegration(item); err != nil {
		return IntegrationConfig{}, contract.InvalidInput(err.Error())
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	for _, existing := range a.config.Integrations {
		if existing.ID == item.ID {
			return IntegrationConfig{}, contract.Conflict("integration already exists")
		}
	}
	previous := a.config
	a.config.Integrations = append(append([]IntegrationConfig(nil), a.config.Integrations...), item)
	if err := saveConfig(a.home, a.config); err != nil {
		a.config = previous
		return IntegrationConfig{}, err
	}
	return cloneIntegration(item), nil
}

func (a *App) UpdateIntegration(_ context.Context, id string, input UpdateIntegrationInput) (IntegrationConfig, error) {
	id = strings.TrimSpace(id)
	item := IntegrationConfig{ID: id, Name: strings.TrimSpace(input.Name), Kind: input.Kind, Endpoint: strings.TrimSpace(input.Endpoint), CredentialRef: strings.TrimSpace(input.CredentialRef), Values: cloneValues(input.Values)}
	if err := validateIntegration(item); err != nil {
		return IntegrationConfig{}, contract.InvalidInput(err.Error())
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	index := -1
	for candidate, existing := range a.config.Integrations {
		if existing.ID == id {
			index = candidate
			break
		}
	}
	if index < 0 {
		return IntegrationConfig{}, contract.NotFound("integration not found")
	}
	previous := a.config
	items := append([]IntegrationConfig(nil), a.config.Integrations...)
	items[index] = item
	a.config.Integrations = items
	if err := saveConfig(a.home, a.config); err != nil {
		a.config = previous
		return IntegrationConfig{}, err
	}
	return cloneIntegration(item), nil
}

func (a *App) RemoveIntegration(_ context.Context, id string) error {
	id = strings.TrimSpace(id)
	a.configMu.Lock()
	defer a.configMu.Unlock()
	index := -1
	for candidate, item := range a.config.Integrations {
		if item.ID == id {
			index = candidate
			break
		}
	}
	if index < 0 {
		return contract.NotFound("integration not found")
	}
	previous := a.config
	items := append([]IntegrationConfig(nil), a.config.Integrations...)
	a.config.Integrations = append(items[:index], items[index+1:]...)
	if err := saveConfig(a.home, a.config); err != nil {
		a.config = previous
		return err
	}
	return nil
}

func validateIntegration(item IntegrationConfig) error {
	config := Config{Version: currentConfigVersion, ScanIntervalSeconds: 10, Integrations: []IntegrationConfig{item}}
	return validateConfig(config)
}

func cloneIntegration(item IntegrationConfig) IntegrationConfig {
	item.Values = cloneValues(item.Values)
	return item
}

func cloneValues(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	copyOf := make(map[string]string, len(values))
	for key, value := range values {
		copyOf[key] = value
	}
	return copyOf
}
