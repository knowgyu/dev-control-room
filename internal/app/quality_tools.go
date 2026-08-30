package app

import (
	"context"

	"github.com/knowgyu/dev-control-room/internal/assurance"
)

// QualityTools returns a live, read-only view of the fixed quality tools and
// reviewed capabilities available to the local application.
func (a *App) QualityTools(ctx context.Context) (assurance.QualityToolsReadModel, error) {
	return assurance.NewQualityToolInspector().Snapshot(ctx)
}
