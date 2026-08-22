package app

import (
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

type Config struct {
	Version                  int                             `json:"version"`
	ScanIntervalSeconds      int                             `json:"scan_interval_seconds"`
	Projects                 []domain.Project                `json:"projects"`
	Environment              []domain.EnvironmentDeclaration `json:"environment,omitempty"`
	Connectors               []domain.ConnectorReference     `json:"connectors,omitempty"`
	AgentProfilesInitialized bool                            `json:"agent_profiles_initialized,omitempty"`
}

type RepositoryState struct {
	Path          string            `json:"path"`
	TopLevel      string            `json:"top_level,omitempty"`
	Branch        string            `json:"branch,omitempty"`
	Origin        string            `json:"origin,omitempty"`
	Detached      bool              `json:"detached"`
	Dirty         bool              `json:"dirty"`
	Ahead         int               `json:"ahead"`
	Behind        int               `json:"behind"`
	Upstream      string            `json:"upstream,omitempty"`
	Provider      string            `json:"provider,omitempty"`
	Capabilities  []string          `json:"capabilities,omitempty"`
	WorktreeCount int               `json:"worktree_count"`
	Worktrees     []domain.Worktree `json:"worktrees,omitempty"`
	UnsafeCleanup bool              `json:"unsafe_cleanup"`
	Error         string            `json:"error,omitempty"`
	ScannedAt     time.Time         `json:"scanned_at"`
}

type ProjectState struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Repos     []RepositoryState `json:"repos"`
	ScannedAt time.Time         `json:"scanned_at"`
}

type Snapshot struct {
	GeneratedAt time.Time      `json:"generated_at"`
	Projects    []ProjectState `json:"projects"`
}

type Event = domain.Event
