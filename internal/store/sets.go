package store

import (
	"errors"
	"path/filepath"
	"time"
)

var ErrInvalidSet = errors.New("invalid set")

// Threshold is a set-level pass/fail assertion evaluated after a SetRun completes.
// Scope: "aggregate" | "per_item" | "<item-name>".
type Threshold struct {
	Metric string  `yaml:"metric" json:"metric"`
	Op     string  `yaml:"op" json:"op"`
	Value  float64 `yaml:"value" json:"value"`
	Scope  string  `yaml:"scope,omitempty" json:"scope,omitempty"`
}

// Override is intentionally a *typed pointer-field* struct; nil means "not set".
// The runtime apply lives in internal/setrunner/overrides.go (so store stays free
// of runner deps). YAML is the canonical wire format.
type Override struct {
	TargetURL     *string           `yaml:"target_url,omitempty" json:"target_url,omitempty"`
	Method        *string           `yaml:"method,omitempty" json:"method,omitempty"`
	Headers       map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Body          *string           `yaml:"body,omitempty" json:"body,omitempty"`
	TotalRequests *int              `yaml:"total_requests,omitempty" json:"total_requests,omitempty"`
	Rate          *int              `yaml:"rate,omitempty" json:"rate,omitempty"`
	Concurrency   *int              `yaml:"concurrency,omitempty" json:"concurrency,omitempty"`
	Duration      *time.Duration    `yaml:"duration,omitempty" json:"duration,omitempty"`
	Timeout       *time.Duration    `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	AuthToken     *string           `yaml:"auth_token,omitempty" json:"auth_token,omitempty"`
	Tags          map[string]string `yaml:"tags,omitempty" json:"tags,omitempty"`
}

type SetItem struct {
	Name      string   `yaml:"name" json:"name"`
	SessionID string   `yaml:"session_id" json:"session_id"`
	Overrides Override `yaml:"overrides,omitempty" json:"overrides,omitempty"`
}

type StageOnFailure string

const (
	OnFailureAbort    StageOnFailure = "abort"
	OnFailureContinue StageOnFailure = "continue"
)

type Stage struct {
	Name      string         `yaml:"name" json:"name"`
	OnFailure StageOnFailure `yaml:"on_failure,omitempty" json:"on_failure,omitempty"`
	Items     []SetItem      `yaml:"items" json:"items"`
}

type Set struct {
	SchemaVersion int         `yaml:"schema_version" json:"schema_version"`
	ID            string      `yaml:"id" json:"id"`
	Name          string      `yaml:"name" json:"name"`
	Description   string      `yaml:"description,omitempty" json:"description,omitempty"`
	CreatedAt     time.Time   `yaml:"created_at" json:"created_at"`
	UpdatedAt     time.Time   `yaml:"updated_at" json:"updated_at"`
	Thresholds    []Threshold `yaml:"thresholds,omitempty" json:"thresholds,omitempty"`
	Stages        []Stage     `yaml:"stages" json:"stages"`
}

type SetRunStatus string

const (
	SetRunRunning   SetRunStatus = "running"
	SetRunCompleted SetRunStatus = "completed"
	SetRunFailed    SetRunStatus = "failed"
	SetRunCancelled SetRunStatus = "cancelled"
)

// ItemResult is the per-item outcome embedded in SetRun.Stages[].Items[].
type ItemResult struct {
	Name      string    `json:"name"`
	SessionID string    `json:"session_id"`
	RunDir    string    `json:"run_dir,omitempty"`
	Status    RunStatus `json:"status"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	Summary   RunSummary `json:"summary,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type StageResult struct {
	Name      string       `json:"name"`
	StartedAt time.Time    `json:"started_at"`
	EndedAt   time.Time    `json:"ended_at,omitempty"`
	Items     []ItemResult `json:"items"`
}

type ThresholdResult struct {
	Threshold
	Actual float64 `json:"actual"`
	Passed bool    `json:"passed"`
}

type SetRun struct {
	SchemaVersion       int               `json:"schema_version"`
	SetID               string            `json:"set_id"`
	SetName             string            `json:"set_name"`
	StartedAt           time.Time         `json:"started_at"`
	EndedAt             time.Time         `json:"ended_at,omitempty"`
	Status              SetRunStatus      `json:"status"`
	Stages              []StageResult     `json:"stages"`
	Thresholds          []ThresholdResult `json:"thresholds,omitempty"`
	AllThresholdsPassed bool              `json:"all_thresholds_passed"`
	ErrorMessage        string            `json:"error_message,omitempty"`
	Dir                 string            `json:"-"`
}

func setsDir(root string) string    { return filepath.Join(root, "sets") }
func setRunsDir(root string) string { return filepath.Join(root, "runs", "sets") }

func setPath(root, id string) string         { return filepath.Join(setsDir(root), id+".yaml") }
func setRunDir(root, setID, ts string) string {
	return filepath.Join(setRunsDir(root), setID, ts)
}
