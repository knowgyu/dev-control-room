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
	Snapshot(context.Context) Snapshot
	Events(context.Context, int) ([]domain.Event, error)
}

type CommandService interface {
	QueueScan(context.Context, string) error
	AddProject(context.Context, AddProjectInput) (domain.Project, error)
	RemoveProject(context.Context, string) error
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
