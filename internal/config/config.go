package config

import (
	"fmt"
	"strings"
	"time"
)

type Config struct {
	TargetURL    string            `mapstructure:"target"`
	Method       string            `mapstructure:"method"`
	Headers      map[string]string `mapstructure:"headers"`
	Body         string            `mapstructure:"body"`
	BodyFile     string            `mapstructure:"body_file"`
	Concurrency  int               `mapstructure:"concurrency"`
	Rate         int               `mapstructure:"rate"`
	Duration     time.Duration     `mapstructure:"duration"`
	Total        int               `mapstructure:"total"`
	Timeout      time.Duration     `mapstructure:"timeout"`
	Retries      int               `mapstructure:"retries"`
	JSONOutput   bool              `mapstructure:"json_output"`
	Dashboard    bool              `mapstructure:"dashboard"`
	LogErrors    bool              `mapstructure:"log_errors"`
	ConfigFile   string            `mapstructure:"-"`
	LoadPatterns []LoadPattern     `mapstructure:"load_patterns"`
	Arrival      ArrivalConfig     `mapstructure:"arrival"`
	Endpoints    []Endpoint        `mapstructure:"endpoints"`
}

type LoadPatternType string

const (
	LoadPatternTypeRamp  LoadPatternType = "ramp"
	LoadPatternTypeStep  LoadPatternType = "step"
	LoadPatternTypeSpike LoadPatternType = "spike"
)

type LoadPattern struct {
	Name     string          `mapstructure:"name"`
	Type     LoadPatternType `mapstructure:"type"`
	FromRPS  int             `mapstructure:"from_rps"`
	ToRPS    int             `mapstructure:"to_rps"`
	Duration time.Duration   `mapstructure:"duration"`
	Steps    []LoadStep      `mapstructure:"steps"`
	RPS      int             `mapstructure:"rps"`
}

type LoadStep struct {
	RPS      int           `mapstructure:"rps"`
	Duration time.Duration `mapstructure:"duration"`
}

type ArrivalModel string

const (
	ArrivalModelUniform ArrivalModel = "uniform"
	ArrivalModelPoisson ArrivalModel = "poisson"
)

type ArrivalConfig struct {
	Model ArrivalModel `mapstructure:"model"`
}

type Endpoint struct {
	Name     string            `mapstructure:"name"`
	Weight   int               `mapstructure:"weight"`
	Method   string            `mapstructure:"method"`
	URL      string            `mapstructure:"url"`
	Path     string            `mapstructure:"path"`
	Headers  map[string]string `mapstructure:"headers"`
	Body     string            `mapstructure:"body"`
	BodyFile string            `mapstructure:"body_file"`
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
		targetSatisfied := false
		if len(c.Endpoints) > 0 {
			allProvideURL := true
			for _, ep := range c.Endpoints {
				if strings.TrimSpace(ep.URL) == "" {
					allProvideURL = false
					break
				}
			}
			if allProvideURL {
				targetSatisfied = true
			}
		}
		if !targetSatisfied {
			issues = append(issues, "target is required (use --help for usage information)")
		}
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

	arrivalIssues := validateArrivalConfig(c.Arrival)
	if len(arrivalIssues) > 0 {
		issues = append(issues, arrivalIssues...)
	}

	patternIssues := validateLoadPatterns(c.LoadPatterns)
	if len(patternIssues) > 0 {
		issues = append(issues, patternIssues...)
	}

	endpointIssues := validateEndpoints(c.Endpoints)
	if len(endpointIssues) > 0 {
		issues = append(issues, endpointIssues...)
	}

	if len(issues) > 0 {
		return ValidationError{issues: issues}
	}

	return nil
}

func validateArrivalConfig(arr ArrivalConfig) []string {
	model := arr.Model
	if model == "" {
		model = ArrivalModelUniform
	}
	switch model {
	case ArrivalModelUniform, ArrivalModelPoisson:
		return nil
	default:
		return []string{fmt.Sprintf("arrival model %q is not supported", model)}
	}
}

func validateLoadPatterns(patterns []LoadPattern) []string {
	var issues []string
	for idx, pattern := range patterns {
		typeLabel := strings.TrimSpace(string(pattern.Type))
		if typeLabel == "" {
			issues = append(issues, fmt.Sprintf("loadPatterns[%d]: type is required", idx))
			continue
		}
		switch LoadPatternType(strings.ToLower(typeLabel)) {
		case LoadPatternTypeRamp:
			if pattern.Duration <= 0 {
				issues = append(issues, fmt.Sprintf("loadPatterns[%d]: duration must be > 0 for ramp", idx))
			}
			if pattern.FromRPS < 0 || pattern.ToRPS < 0 {
				issues = append(issues, fmt.Sprintf("loadPatterns[%d]: from_rps and to_rps must be >= 0", idx))
			}
		case LoadPatternTypeStep:
			if len(pattern.Steps) == 0 {
				issues = append(issues, fmt.Sprintf("loadPatterns[%d]: steps are required for step pattern", idx))
			}
			for stepIdx, step := range pattern.Steps {
				if step.RPS < 0 {
					issues = append(issues, fmt.Sprintf("loadPatterns[%d].steps[%d]: rps must be >= 0", idx, stepIdx))
				}
				if step.Duration <= 0 {
					issues = append(issues, fmt.Sprintf("loadPatterns[%d].steps[%d]: duration must be > 0", idx, stepIdx))
				}
			}
		case LoadPatternTypeSpike:
			if pattern.RPS <= 0 {
				issues = append(issues, fmt.Sprintf("loadPatterns[%d]: rps must be > 0 for spike", idx))
			}
			if pattern.Duration <= 0 {
				issues = append(issues, fmt.Sprintf("loadPatterns[%d]: duration must be > 0 for spike", idx))
			}
		default:
			issues = append(issues, fmt.Sprintf("loadPatterns[%d]: unsupported type %q", idx, pattern.Type))
		}
	}
	return issues
}

func validateEndpoints(endpoints []Endpoint) []string {
	var issues []string
	seenNames := map[string]int{}
	for idx, ep := range endpoints {
		if ep.Weight <= 0 {
			issues = append(issues, fmt.Sprintf("endpoints[%d]: weight must be >= 1", idx))
		}
		if strings.TrimSpace(ep.Body) != "" && strings.TrimSpace(ep.BodyFile) != "" {
			issues = append(issues, fmt.Sprintf("endpoints[%d]: body and bodyFile are mutually exclusive", idx))
		}
		name := strings.TrimSpace(ep.Name)
		if name != "" {
			key := strings.ToLower(name)
			if prev, ok := seenNames[key]; ok {
				issues = append(issues, fmt.Sprintf("endpoints[%d]: duplicate name also defined at index %d", idx, prev))
			} else {
				seenNames[key] = idx
			}
		}
	}
	return issues
}
