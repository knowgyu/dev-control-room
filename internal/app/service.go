package app

import (
	"context"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

// QueryService and CommandService are the application boundary. CLI and HTTP
// adapters depend on these interfaces instead of reaching into configuration,
// collectors, or storage directly.
type QueryService interface {
	Health(context.Context) Health
	Snapshot(context.Context) (Snapshot, error)
	Projects(context.Context) ([]domain.Project, error)
	Project(context.Context, string) (domain.Project, error)
	Repositories(context.Context, string) ([]domain.Repository, error)
	Repository(context.Context, string, string) (domain.Repository, error)
	Findings(context.Context, string, string) ([]domain.Finding, error)
	Finding(context.Context, string) (domain.Finding, error)
	Events(context.Context, int) ([]domain.Event, error)
}

type CommandService interface {
	QueueScan(context.Context, string) error
	RunScan(context.Context, string) error
	AddProject(context.Context, AddProjectInput) (domain.Project, error)
	UpdateProject(context.Context, string, UpdateProjectInput) (domain.Project, error)
	RemoveProject(context.Context, string) error
	AddRepository(context.Context, AddRepositoryInput) (domain.Repository, error)
	UpdateRepository(context.Context, string, string, UpdateRepositoryInput) (domain.Repository, error)
	RemoveRepository(context.Context, string, string) error
	AcknowledgeFinding(context.Context, string) error
	ExportProject(context.Context, string) ([]byte, error)
	ImportProject(context.Context, []byte) (domain.Project, error)
}

type ApplicationService interface {
	QueryService
	CommandService
}

type Health struct {
	OK            bool   `json:"ok"`
	Service       string `json:"service"`
	NetworkMode   string `json:"network_mode"`
	Telemetry     bool   `json:"telemetry"`
	Contract      string `json:"contract"`
	ConfigVersion int    `json:"config_version"`
}

type AddProjectInput struct {
	Name string
	Path string
}

type AddRepositoryInput struct {
	ProjectID string
	ID        string
	Name      string
	Path      string
}

type UpdateProjectInput struct {
	Name string
}

type UpdateRepositoryInput struct {
	Name string
	Path string
}
