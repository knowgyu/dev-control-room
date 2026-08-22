package app

import (
	"time"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

type Config struct {
	Version             int              `json:"version"`
	ScanIntervalSeconds int              `json:"scan_interval_seconds"`
	Projects            []domain.Project `json:"projects"`
}

type RepositoryState struct {
	Path      string    `json:"path"`
	Branch    string    `json:"branch,omitempty"`
	Origin    string    `json:"origin,omitempty"`
	Dirty     bool      `json:"dirty"`
	Ahead     int       `json:"ahead"`
	Behind    int       `json:"behind"`
	Error     string    `json:"error,omitempty"`
	ScannedAt time.Time `json:"scanned_at"`
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
