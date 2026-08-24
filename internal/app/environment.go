package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/knowgyu/dev-control-room/internal/contract"
	"github.com/knowgyu/dev-control-room/internal/domain"
	"github.com/knowgyu/dev-control-room/internal/environment"
	"github.com/knowgyu/dev-control-room/internal/scheduler"
)

func (a *App) EnvironmentHealth(ctx context.Context, refresh bool) (environment.Health, error) {
	if !refresh {
		var health environment.Health
		if found, err := a.store.LoadSingleton(ctx, "environment_health", &health); err != nil {
			return environment.Health{}, err
		} else if found {
			return health, nil
		}
		return environment.Health{
			GeneratedAt: time.Now().UTC(), Available: false,
			Tools: []environment.ToolStatus{}, Profiles: []environment.ProfileStatus{}, Environment: []environment.EnvironmentVariableStatus{}, Connectors: []environment.ConnectorStatus{},
			Findings: []environment.Finding{{Type: "environment.not_checked", Severity: "attention", Summary: "environment doctor has not run yet", RecommendedNextAction: "run env doctor before relying on environment health"}},
		}, nil
	}
	a.environmentMu.Lock()
	defer a.environmentMu.Unlock()
	profiles, err := a.AgentProfiles(ctx)
	if err != nil {
		return environment.Health{}, err
	}
	health := a.doctor.Run(ctx, profiles, a.config.Environment, a.config.Connectors)
	if err := a.store.SaveSingleton(ctx, "environment_health", health, health.GeneratedAt); err != nil {
		return environment.Health{}, err
	}
	return health, nil
}

func (a *App) AgentProfiles(ctx context.Context) ([]domain.AgentProfile, error) {
	return a.store.ListAgentProfiles(ctx)
}

func (a *App) AgentProfile(ctx context.Context, id string) (domain.AgentProfile, error) {
	profile, err := a.store.GetAgentProfile(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AgentProfile{}, contract.NotFound("agent profile not found")
	}
	return profile, err
}

func (a *App) AddAgentProfile(ctx context.Context, input AddAgentProfileInput) (domain.AgentProfile, error) {
	id := normalizeAppID(input.ID)
	if id == "" {
		id = normalizeAppID(input.Name)
	}
	if id == "" || strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Command) == "" {
		return domain.AgentProfile{}, contract.InvalidInput("agent profile id, name, and command are required")
	}
	if _, err := a.store.GetAgentProfile(ctx, id); err == nil {
		return domain.AgentProfile{}, contract.Conflict("agent profile already exists")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return domain.AgentProfile{}, err
	}
	profile := domain.AgentProfile{
		TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AgentProfileKind},
		Metadata: domain.ObjectMeta{ID: id, Name: strings.TrimSpace(input.Name)},
		Spec:     domain.AgentProfileSpec{Command: strings.TrimSpace(input.Command), VersionProbe: append([]string(nil), input.VersionProbe...), TimeoutSeconds: input.TimeoutSeconds, ModelArgumentTemplate: strings.TrimSpace(input.ModelArgumentTemplate), EnvironmentAllowlist: append([]string(nil), input.EnvironmentAllowlist...), LaunchMode: input.LaunchMode, DataBoundary: input.DataBoundary},
	}
	if profile.Spec.LaunchMode == "" {
		profile.Spec.LaunchMode = domain.AgentLaunchDirect
	}
	if profile.Spec.DataBoundary == "" {
		profile.Spec.DataBoundary = domain.AgentBoundaryLocal
	}
	if err := profile.Validate(); err != nil {
		return domain.AgentProfile{}, contract.InvalidInput(err.Error())
	}
	if err := a.store.SaveAgentProfile(ctx, profile); err != nil {
		return domain.AgentProfile{}, err
	}
	return profile, nil
}

func (a *App) UpdateAgentProfile(ctx context.Context, id string, input UpdateAgentProfileInput) (domain.AgentProfile, error) {
	profile, err := a.AgentProfile(ctx, id)
	if err != nil {
		return domain.AgentProfile{}, err
	}
	if strings.TrimSpace(input.Name) != "" {
		profile.Metadata.Name = strings.TrimSpace(input.Name)
	}
	if strings.TrimSpace(input.Command) != "" {
		profile.Spec.Command = strings.TrimSpace(input.Command)
	}
	if input.VersionProbe != nil {
		profile.Spec.VersionProbe = append([]string(nil), input.VersionProbe...)
	}
	if input.TimeoutSeconds != 0 {
		profile.Spec.TimeoutSeconds = input.TimeoutSeconds
	}
	if strings.TrimSpace(input.ModelArgumentTemplate) != "" {
		profile.Spec.ModelArgumentTemplate = strings.TrimSpace(input.ModelArgumentTemplate)
	}
	if input.EnvironmentAllowlist != nil {
		profile.Spec.EnvironmentAllowlist = append([]string(nil), input.EnvironmentAllowlist...)
	}
	if input.LaunchMode != "" {
		profile.Spec.LaunchMode = input.LaunchMode
	}
	if input.DataBoundary != "" {
		profile.Spec.DataBoundary = input.DataBoundary
	}
	if err := profile.Validate(); err != nil {
		return domain.AgentProfile{}, contract.InvalidInput(err.Error())
	}
	if err := a.store.SaveAgentProfile(ctx, profile); err != nil {
		return domain.AgentProfile{}, err
	}
	return profile, nil
}

func (a *App) RemoveAgentProfile(ctx context.Context, id string) error {
	if err := a.store.DeleteAgentProfile(ctx, id); errors.Is(err, sql.ErrNoRows) {
		return contract.NotFound("agent profile not found")
	} else if err != nil {
		return err
	}
	return nil
}

func (a *App) Schedule(ctx context.Context, operation scheduler.Operation) (scheduler.Result, error) {
	if err := scheduler.Validate(operation); err != nil {
		return scheduler.Result{}, contract.InvalidInput(err.Error())
	}
	result, err := a.scheduler.Apply(ctx, operation)
	if err != nil {
		return scheduler.Result{}, contract.InvalidInput(err.Error())
	}
	if err := a.store.SaveSingleton(ctx, "scheduler_state", result, time.Now().UTC()); err != nil {
		return scheduler.Result{}, err
	}
	return result, nil
}

func (a *App) ensureDefaultAgentProfiles(ctx context.Context) error {
	defaults := []domain.AgentProfile{
		newDefaultProfile("codex", "Codex", "codex", domain.AgentLaunchDirect),
		newDefaultProfile("claude", "Claude", "claude", domain.AgentLaunchDirect),
		newDefaultProfile("gemini", "Gemini", "gemini", domain.AgentLaunchDirect),
		newDefaultProfile("claude-local", "Claude Local", "claude-local", domain.AgentLaunchPowerShellProfile),
	}
	for _, profile := range defaults {
		if _, err := a.store.GetAgentProfile(ctx, profile.Metadata.ID); err == nil {
			continue
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err := a.store.SaveAgentProfile(ctx, profile); err != nil {
			return err
		}
	}
	return nil
}

func newDefaultProfile(id, name, command string, mode domain.AgentLaunchMode) domain.AgentProfile {
	return domain.AgentProfile{TypeMeta: domain.TypeMeta{APIVersion: domain.APIVersion, Kind: domain.AgentProfileKind}, Metadata: domain.ObjectMeta{ID: id, Name: name}, Spec: domain.AgentProfileSpec{Command: command, LaunchMode: mode, DataBoundary: domain.AgentBoundaryLocal, TimeoutSeconds: 8}}
}
