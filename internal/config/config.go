package config

import (
	"fmt"
	"strings"
	"time"
)

type Config struct {
	TargetURL   string            `mapstructure:"target"`
	Method      string            `mapstructure:"method"`
	Headers     map[string]string `mapstructure:"headers"`
	Body        string            `mapstructure:"body"`
	BodyFile    string            `mapstructure:"body_file"`
	Concurrency int               `mapstructure:"concurrency"`
	Rate        int               `mapstructure:"rate"`
	Duration    time.Duration     `mapstructure:"duration"`
	Total       int               `mapstructure:"total"`
	Timeout     time.Duration     `mapstructure:"timeout"`
	Retries     int               `mapstructure:"retries"`
	JSONOutput  bool              `mapstructure:"json_output"`
	Dashboard   bool              `mapstructure:"dashboard"`
	LogErrors   bool              `mapstructure:"log_errors"`
	ConfigFile  string            `mapstructure:"-"`
}

type ValidationError struct {
	issues []string
}

func (e ValidationError) Error() string {
	if len(e.issues) == 0 {
		return "validation failed"
	}
	return fmt.Sprintf("validation failed: %s", strings.Join(e.issues, "; "))
}

func (e ValidationError) Issues() []string {
	return append([]string(nil), e.issues...)
}

func (c Config) Validate() error {
	var issues []string

	if strings.TrimSpace(c.TargetURL) == "" {
		issues = append(issues, "target is required")
	}

	if c.Concurrency < 1 {
		issues = append(issues, "concurrency must be >= 1")
	}
	if c.Rate < 0 {
		issues = append(issues, "rate must be >= 0")
	}
	if c.Total < 0 {
		issues = append(issues, "total must be >= 0")
	}
	if c.Timeout < 0 {
		issues = append(issues, "timeout must be >= 0")
	}
	if c.Retries < 0 {
		issues = append(issues, "retries must be >= 0")
	}
	if c.Duration < 0 {
		issues = append(issues, "duration must be >= 0")
	}
	if strings.TrimSpace(c.Body) != "" && strings.TrimSpace(c.BodyFile) != "" {
		issues = append(issues, "body and bodyFile are mutually exclusive")
	}
	if c.Dashboard && c.JSONOutput {
		issues = append(issues, "dashboard and json-output are mutually exclusive")
	}

	if len(issues) > 0 {
		return ValidationError{issues: issues}
	}

	return nil
}
