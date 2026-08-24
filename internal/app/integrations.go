package app

import (
	"context"
	"strings"

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
