package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/masking"
)

type KubernetesPodStatus struct {
	Name         string `json:"name"`
	Phase        string `json:"phase"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount"`
}

type KubernetesStatus struct {
	IntegrationID string                `json:"integrationId"`
	Namespace     string                `json:"namespace"`
	Selector      string                `json:"selector"`
	Pods          []KubernetesPodStatus `json:"pods"`
	CheckedAt     time.Time             `json:"checkedAt"`
}

type KubernetesLogs struct {
	IntegrationID string    `json:"integrationId"`
	Namespace     string    `json:"namespace"`
	Pod           string    `json:"pod"`
	Container     string    `json:"container,omitempty"`
	TailLines     int       `json:"tailLines"`
	Logs          string    `json:"logs"`
	CheckedAt     time.Time `json:"checkedAt"`
}

type kubernetesPod struct {
	Name            string
	Phase           string
	ContainerStates []struct {
		Ready        bool  `json:"ready"`
		RestartCount int32 `json:"restartCount"`
	}
}

func (a *App) KubernetesStatus(ctx context.Context, id string) (KubernetesStatus, error) {
	integration, credential, err := a.kubernetesIntegration(id)
	if err != nil {
		return KubernetesStatus{}, err
	}
	pods, err := a.kubernetesPods(ctx, integration, credential)
	if err != nil {
		return KubernetesStatus{}, err
	}
	result := KubernetesStatus{IntegrationID: integration.ID, Namespace: integration.Values["namespace"], Selector: integration.Values["selector"], CheckedAt: time.Now().UTC()}
	for _, pod := range pods {
		ready := len(pod.ContainerStates) > 0
		var restarts int32
		for _, container := range pod.ContainerStates {
			ready = ready && container.Ready
			restarts += container.RestartCount
		}
		result.Pods = append(result.Pods, KubernetesPodStatus{Name: pod.Name, Phase: pod.Phase, Ready: ready, RestartCount: restarts})
	}
	return result, nil
}

func (a *App) KubernetesLogs(ctx context.Context, id string) (KubernetesLogs, error) {
	integration, credential, err := a.kubernetesIntegration(id)
	if err != nil {
		return KubernetesLogs{}, err
	}
	pods, err := a.kubernetesPods(ctx, integration, credential)
	if err != nil {
		return KubernetesLogs{}, err
	}
	if len(pods) == 0 {
		return KubernetesLogs{}, contract.CodedError{Code: contract.ErrorUnavailable, Message: "Kubernetes selector returned no Pods"}
	}
	pod := pods[0]
	for _, candidate := range pods {
		if candidate.Phase == "Running" {
			pod = candidate
			break
		}
	}
	tailLines := 200
	if value, parseErr := strconv.Atoi(strings.TrimSpace(integration.Values["tail_lines"])); parseErr == nil && value > 0 {
		tailLines = value
	}
	if tailLines > 1000 {
		tailLines = 1000
	}
	container := strings.TrimSpace(integration.Values["container"])
	query := url.Values{"tailLines": {strconv.Itoa(tailLines)}, "timestamps": {"true"}}
	if container != "" {
		query.Set("container", container)
	}
	path := "/api/v1/namespaces/" + url.PathEscape(integration.Values["namespace"]) + "/pods/" + url.PathEscape(pod.Name) + "/log"
	body, err := fetchKubernetes(ctx, integration, credential, path, query, 64<<10)
	if err != nil {
		return KubernetesLogs{}, err
	}
	logText := masking.New([]string{credential}, nil).Mask(string(body))
	return KubernetesLogs{IntegrationID: integration.ID, Namespace: integration.Values["namespace"], Pod: pod.Name, Container: container, TailLines: tailLines, Logs: a.masker.Mask(logText), CheckedAt: time.Now().UTC()}, nil
}

func (a *App) kubernetesIntegration(id string) (IntegrationConfig, string, error) {
	integration, err := a.integration(id)
	if err != nil {
		return IntegrationConfig{}, "", err
	}
	if integration.Kind != IntegrationKubernetes {
		return IntegrationConfig{}, "", contract.InvalidInput("integration is not Kubernetes")
	}
	if strings.TrimSpace(integration.Values["namespace"]) == "" || strings.TrimSpace(integration.Values["selector"]) == "" {
		return IntegrationConfig{}, "", contract.InvalidInput("Kubernetes integration requires namespace and selector values")
	}
	credential, present, credentialErr := integrationCredential(integration.CredentialRef)
	if credentialErr != nil || !present {
		if credentialErr != nil {
			return IntegrationConfig{}, "", contract.CodedError{Code: contract.ErrorUnavailable, Message: credentialErr.Error()}
		}
		return IntegrationConfig{}, "", contract.CodedError{Code: contract.ErrorUnavailable, Message: "Kubernetes bearer credential environment variable is not set"}
	}
	return integration, credential, nil
}

func (a *App) kubernetesPods(ctx context.Context, integration IntegrationConfig, credential string) ([]kubernetesPod, error) {
	query := url.Values{"labelSelector": {integration.Values["selector"]}}
	body, err := fetchKubernetes(ctx, integration, credential, "/api/v1/namespaces/"+url.PathEscape(integration.Values["namespace"])+"/pods", query, 512<<10)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Status struct {
				Phase             string `json:"phase"`
				ContainerStatuses []struct {
					Ready        bool  `json:"ready"`
					RestartCount int32 `json:"restartCount"`
				} `json:"containerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, contract.CodedError{Code: contract.ErrorUnavailable, Message: "Kubernetes Pod response was invalid"}
	}
	pods := make([]kubernetesPod, 0, len(payload.Items))
	for _, item := range payload.Items {
		pod := kubernetesPod{Name: item.Metadata.Name, Phase: item.Status.Phase}
		pod.ContainerStates = append(pod.ContainerStates, item.Status.ContainerStatuses...)
		pods = append(pods, pod)
	}
	return pods, nil
}

func fetchKubernetes(ctx context.Context, integration IntegrationConfig, credential, path string, query url.Values, limit int64) ([]byte, error) {
	endpoint := strings.TrimRight(integration.Endpoint, "/") + path
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, contract.InvalidInput("Kubernetes endpoint is invalid")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+credential)
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return nil, contract.CodedError{Code: contract.ErrorUnavailable, Message: "Kubernetes API could not be reached"}
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, contract.CodedError{Code: contract.ErrorUnavailable, Message: fmt.Sprintf("Kubernetes API returned HTTP %d", response.StatusCode)}
	}
	return io.ReadAll(io.LimitReader(response.Body, limit))
}
