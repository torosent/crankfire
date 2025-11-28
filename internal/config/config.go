package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Protocol string

const (
	ProtocolHTTP      Protocol = "http"
	ProtocolWebSocket Protocol = "websocket"
	ProtocolSSE       Protocol = "sse"
	ProtocolGRPC      Protocol = "grpc"
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
	HTMLOutput   string            `mapstructure:"html_output"`
	ConfigFile   string            `mapstructure:"-"`
	LoadPatterns []LoadPattern     `mapstructure:"load_patterns"`
	Arrival      ArrivalConfig     `mapstructure:"arrival"`
	Endpoints    []Endpoint        `mapstructure:"endpoints"`
	Auth         AuthConfig        `mapstructure:"auth"`
	Feeder       FeederConfig      `mapstructure:"feeder"`
	Protocol     Protocol          `mapstructure:"protocol"`
	WebSocket    WebSocketConfig   `mapstructure:"websocket"`
	SSE          SSEConfig         `mapstructure:"sse"`
	GRPC         GRPCConfig        `mapstructure:"grpc"`
	Thresholds   []string          `mapstructure:"thresholds"`
	HARFile      string            `mapstructure:"har_file"`
	HARFilter    string            `mapstructure:"har_filter"`
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

type FeederConfig struct {
	Path string `mapstructure:"path"`
	Type string `mapstructure:"type"` // "csv" or "json"
}

type WebSocketConfig struct {
	Messages         []string      `mapstructure:"messages"`          // Messages to send
	MessageInterval  time.Duration `mapstructure:"message_interval"`  // Interval between messages
	ReceiveTimeout   time.Duration `mapstructure:"receive_timeout"`   // Timeout for receiving responses
	HandshakeTimeout time.Duration `mapstructure:"handshake_timeout"` // WebSocket handshake timeout
}

type SSEConfig struct {
	ReadTimeout time.Duration `mapstructure:"read_timeout"` // Timeout for reading events
	MaxEvents   int           `mapstructure:"max_events"`   // Max events to read (0=unlimited)
}

type GRPCConfig struct {
	ProtoFile string            `mapstructure:"proto_file"` // Path to .proto file
	Service   string            `mapstructure:"service"`    // Service name (e.g., "helloworld.Greeter")
	Method    string            `mapstructure:"method"`     // Method name (e.g., "SayHello")
	Message   string            `mapstructure:"message"`    // JSON message payload (supports templates)
	Metadata  map[string]string `mapstructure:"metadata"`   // gRPC metadata (headers)
	Timeout   time.Duration     `mapstructure:"timeout"`    // Per-call timeout
	TLS       bool              `mapstructure:"tls"`        // Use TLS
	Insecure  bool              `mapstructure:"insecure"`   // Skip TLS verification
}

type AuthType string

const (
	AuthTypeOAuth2ClientCredentials AuthType = "oauth2_client_credentials"
	AuthTypeOAuth2ResourceOwner     AuthType = "oauth2_resource_owner"
	AuthTypeOIDCImplicit            AuthType = "oidc_implicit"
	AuthTypeOIDCAuthCode            AuthType = "oidc_auth_code"
)

type AuthConfig struct {
	Type                AuthType      `mapstructure:"type"`
	TokenURL            string        `mapstructure:"token_url"`
	ClientID            string        `mapstructure:"client_id"`
	ClientSecret        string        `mapstructure:"client_secret"`
	Username            string        `mapstructure:"username"`
	Password            string        `mapstructure:"password"`
	Scopes              []string      `mapstructure:"scopes"`
	StaticToken         string        `mapstructure:"static_token"`
	RefreshBeforeExpiry time.Duration `mapstructure:"refresh_before_expiry"`
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
	var warnings []string

	if strings.TrimSpace(c.TargetURL) == "" {
		targetSatisfied := false
		// HAR file provides its own URLs, so target is not required
		if strings.TrimSpace(c.HARFile) != "" {
			targetSatisfied = true
		} else if len(c.Endpoints) > 0 {
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

	// Security warnings for high rate/concurrency
	if c.Rate > 1000 {
		warnings = append(warnings, fmt.Sprintf("WARNING: High rate limit configured (%d RPS). Ensure you have authorization to test the target system.", c.Rate))
	}
	if c.Concurrency > 500 {
		warnings = append(warnings, fmt.Sprintf("WARNING: High concurrency configured (%d workers). Ensure you have authorization to test the target system.", c.Concurrency))
	}

	// Print warnings to stderr
	if len(warnings) > 0 {
		for _, w := range warnings {
			fmt.Fprintln(os.Stderr, w)
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

	authIssues := validateAuthConfig(c.Auth)
	if len(authIssues) > 0 {
		issues = append(issues, authIssues...)
	}

	// Security warnings for unsafe auth patterns
	if c.Auth.Type == AuthTypeOAuth2ResourceOwner {
		fmt.Fprintln(os.Stderr, "WARNING: oauth2_resource_owner (password grant) is a legacy flow and is NOT RECOMMENDED. Consider using oauth2_client_credentials or authorization code with PKCE instead.")
	}
	if c.Auth.Type == AuthTypeOIDCImplicit || c.Auth.Type == AuthTypeOIDCAuthCode {
		if c.Auth.StaticToken != "" {
			fmt.Fprintln(os.Stderr, "WARNING: Using static tokens increases security risk. Prefer short-lived tokens and avoid passing secrets via CLI flags.")
		}
	}

	feederIssues := validateFeederConfig(c.Feeder)
	if len(feederIssues) > 0 {
		issues = append(issues, feederIssues...)
	}

	protocolIssues := validateProtocolConfig(c.Protocol, c.WebSocket, c.SSE, c.GRPC)
	if len(protocolIssues) > 0 {
		issues = append(issues, protocolIssues...)
	}

	// Security warning for insecure gRPC
	if c.Protocol == ProtocolGRPC && c.GRPC.Insecure {
		fmt.Fprintln(os.Stderr, "WARNING: gRPC TLS verification is DISABLED (insecure: true). This should ONLY be used in development/testing environments. Man-in-the-middle attacks are possible.")
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

func validateAuthConfig(auth AuthConfig) []string {
	var issues []string
	if auth.Type == "" {
		return nil
	}

	switch auth.Type {
	case AuthTypeOAuth2ClientCredentials:
		if strings.TrimSpace(auth.TokenURL) == "" {
			issues = append(issues, "auth: token_url is required for oauth2_client_credentials")
		}
		if strings.TrimSpace(auth.ClientID) == "" {
			issues = append(issues, "auth: client_id is required for oauth2_client_credentials")
		}
		if strings.TrimSpace(auth.ClientSecret) == "" {
			issues = append(issues, "auth: client_secret is required for oauth2_client_credentials")
		}
	case AuthTypeOAuth2ResourceOwner:
		if strings.TrimSpace(auth.TokenURL) == "" {
			issues = append(issues, "auth: token_url is required for oauth2_resource_owner")
		}
		if strings.TrimSpace(auth.ClientID) == "" {
			issues = append(issues, "auth: client_id is required for oauth2_resource_owner")
		}
		if strings.TrimSpace(auth.ClientSecret) == "" {
			issues = append(issues, "auth: client_secret is required for oauth2_resource_owner")
		}
		if strings.TrimSpace(auth.Username) == "" {
			issues = append(issues, "auth: username is required for oauth2_resource_owner")
		}
		if strings.TrimSpace(auth.Password) == "" {
			issues = append(issues, "auth: password is required for oauth2_resource_owner")
		}
	case AuthTypeOIDCImplicit:
		if strings.TrimSpace(auth.StaticToken) == "" {
			issues = append(issues, "auth: static_token is required for oidc_implicit")
		}
	case AuthTypeOIDCAuthCode:
		if strings.TrimSpace(auth.StaticToken) == "" {
			issues = append(issues, "auth: static_token is required for oidc_auth_code")
		}
	default:
		issues = append(issues, fmt.Sprintf("auth: unsupported type %q", auth.Type))
	}

	return issues
}

func validateFeederConfig(feeder FeederConfig) []string {
	var issues []string
	if strings.TrimSpace(feeder.Path) == "" {
		return nil // No feeder configured
	}

	if strings.TrimSpace(feeder.Type) == "" {
		issues = append(issues, "feeder: type is required when path is specified")
	} else if feeder.Type != "csv" && feeder.Type != "json" {
		issues = append(issues, fmt.Sprintf("feeder: type must be 'csv' or 'json', got %q", feeder.Type))
	}

	return issues
}

func validateProtocolConfig(protocol Protocol, ws WebSocketConfig, sse SSEConfig, grpc GRPCConfig) []string {
	var issues []string

	// Default to HTTP if not specified
	if protocol == "" {
		return nil
	}

	// Validate protocol value
	switch protocol {
	case ProtocolHTTP, ProtocolWebSocket, ProtocolSSE, ProtocolGRPC:
		// Valid protocols
	default:
		issues = append(issues, fmt.Sprintf("protocol: must be 'http', 'websocket', 'sse', or 'grpc', got %q", protocol))
		return issues
	}

	// Protocol-specific validation
	if protocol == ProtocolWebSocket {
		if ws.MessageInterval < 0 {
			issues = append(issues, "websocket: message_interval must be >= 0")
		}
		if ws.ReceiveTimeout < 0 {
			issues = append(issues, "websocket: receive_timeout must be >= 0")
		}
		if ws.HandshakeTimeout < 0 {
			issues = append(issues, "websocket: handshake_timeout must be >= 0")
		}
	}

	if protocol == ProtocolSSE {
		if sse.ReadTimeout < 0 {
			issues = append(issues, "sse: read_timeout must be >= 0")
		}
		if sse.MaxEvents < 0 {
			issues = append(issues, "sse: max_events must be >= 0")
		}
	}

	if protocol == ProtocolGRPC {
		if grpc.Service == "" {
			issues = append(issues, "grpc: service is required")
		}
		if grpc.Method == "" {
			issues = append(issues, "grpc: method is required")
		}
		if grpc.Timeout < 0 {
			issues = append(issues, "grpc: timeout must be >= 0")
		}
	}

	return issues
}
