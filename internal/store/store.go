// internal/store/store.go
package store

import (
	"context"
	"time"

	"github.com/torosent/crankfire/internal/config"
)

const SchemaVersion = 1

type Session struct {
	SchemaVersion int           `yaml:"schema_version"`
	ID            string        `yaml:"id"`
	Name          string        `yaml:"name"`
	Description   string        `yaml:"description,omitempty"`
	CreatedAt     time.Time     `yaml:"created_at"`
	UpdatedAt     time.Time     `yaml:"updated_at"`
	Config        config.Config `yaml:"config"`
}

type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)

type Run struct {
	SessionID string    `json:"session_id"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	Status    RunStatus `json:"status"`
	Dir       string    `json:"-"`
	Summary   RunSummary `json:"summary,omitempty"`
}

type RunSummary struct {
	TotalRequests int64   `json:"total_requests"`
	Errors        int64   `json:"errors"`
	DurationSec   float64 `json:"duration_sec"`
	P50Ms         float64 `json:"p50_ms"`
	P90Ms         float64 `json:"p90_ms"`
	P95Ms         float64 `json:"p95_ms"`
	P99Ms         float64 `json:"p99_ms"`
	ErrorMessage  string  `json:"error_message,omitempty"`
}

type Store interface {
	ListSessions(ctx context.Context) ([]Session, error)
	GetSession(ctx context.Context, id string) (Session, error)
	SaveSession(ctx context.Context, s Session) error
	DeleteSession(ctx context.Context, id string) error
	ImportSessionFromConfigFile(ctx context.Context, path, name string) (Session, error)

	ListRuns(ctx context.Context, sessionID string) ([]Run, error)
	CreateRun(ctx context.Context, sessionID string) (Run, error)
	FinalizeRun(ctx context.Context, run Run, summary RunSummary) error

	ListSets(ctx context.Context) ([]Set, error)
	GetSet(ctx context.Context, id string) (Set, error)
	SaveSet(ctx context.Context, s Set) error
	DeleteSet(ctx context.Context, id string) error

	ListSetRuns(ctx context.Context, setID string) ([]SetRun, error)
	CreateSetRun(ctx context.Context, setID string) (SetRun, error)
	FinalizeSetRun(ctx context.Context, run SetRun) error
}
