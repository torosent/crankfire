package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Loader handles loading configuration from files and command-line arguments.
type Loader struct{}

// ErrHelpRequested is returned when the user requests help via --help flag.
var ErrHelpRequested = errors.New("help requested")

// NewLoader creates a new configuration Loader.
func NewLoader() *Loader {
	return &Loader{}
}

// Load parses command-line arguments and configuration files to produce a Config.
func (Loader) Load(args []string) (*Config, error) {
	cmd := newFlagCommand()
	if err := cmd.Flags().Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			displayHelp(cmd)
			return nil, ErrHelpRequested
		}
		return nil, err
	}

	flagSet := cmd.Flags()
	if helpFlag := flagSet.Lookup("help"); helpFlag != nil {
		if wantsHelp, err := strconv.ParseBool(helpFlag.Value.String()); err == nil && wantsHelp {
			displayHelp(cmd)
			return nil, ErrHelpRequested
		}
	}

	// If no arguments provided and no config file, show help/usage
	configPath := flagSet.Lookup("config").Value.String()
	if len(args) == 0 && configPath == "" {
		displayHelp(cmd)
		return nil, ErrHelpRequested
	}
	cfgViper := viper.New()
	if configPath != "" {
		cfgViper.SetConfigFile(configPath)
		if err := cfgViper.ReadInConfig(); err != nil {
			return nil, err
		}
	}

	settings := cfgViper.AllSettings()

	cfg := &Config{
		Method:      "GET",
		Headers:     map[string]string{},
		Concurrency: 1,
		Timeout:     30 * time.Second,
		ConfigFile:  configPath,
		Arrival:     ArrivalConfig{Model: ArrivalModelUniform},
	}

	if err := applyConfigSettings(cfg, settings); err != nil {
		return nil, err
	}

	if err := applyFlagOverrides(cfg, flagSet); err != nil {
		return nil, err
	}

	cfg.Method = strings.ToUpper(cfg.Method)
	cfg.TargetURL = strings.TrimSpace(cfg.TargetURL)
	cfg.BodyFile = strings.TrimSpace(cfg.BodyFile)

	if cfg.Headers == nil {
		cfg.Headers = map[string]string{}
	}

	if err := loadHAREndpoints(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// applyConfigSettings applies settings from a config file to the Config struct.
func applyConfigSettings(cfg *Config, settings map[string]interface{}) error {
	if len(settings) == 0 {
		return nil
	}

	if raw, ok := lookupSetting(settings, "target"); ok {
		val, err := asString(raw)
		if err != nil {
			return fmt.Errorf("target: %w", err)
		}
		cfg.TargetURL = strings.TrimSpace(val)
	}

	if raw, ok := lookupSetting(settings, "method"); ok {
		val, err := asString(raw)
		if err != nil {
			return fmt.Errorf("method: %w", err)
		}
		if val != "" {
			cfg.Method = val
		}
	}

	if raw, ok := lookupSetting(settings, "headers"); ok {
		hdrs, err := asStringMap(raw)
		if err != nil {
			return fmt.Errorf("headers: %w", err)
		}
		if cfg.Headers == nil {
			cfg.Headers = map[string]string{}
		}
		for k, v := range hdrs {
			cfg.Headers[http.CanonicalHeaderKey(k)] = v
		}
	}

	if raw, ok := lookupSetting(settings, "body"); ok {
		val, err := asString(raw)
		if err != nil {
			return fmt.Errorf("body: %w", err)
		}
		cfg.Body = val
	}

	if raw, ok := lookupSetting(settings, "bodyfile", "body_file", "body-file"); ok {
		val, err := asString(raw)
		if err != nil {
			return fmt.Errorf("bodyFile: %w", err)
		}
		cfg.BodyFile = val
	}

	if raw, ok := lookupSetting(settings, "concurrency"); ok {
		val, err := asInt(raw)
		if err != nil {
			return fmt.Errorf("concurrency: %w", err)
		}
		cfg.Concurrency = val
	}

	if raw, ok := lookupSetting(settings, "rate"); ok {
		val, err := asInt(raw)
		if err != nil {
			return fmt.Errorf("rate: %w", err)
		}
		cfg.Rate = val
	}

	if raw, ok := lookupSetting(settings, "duration"); ok {
		dur, err := asDuration(raw)
		if err != nil {
			return fmt.Errorf("duration: %w", err)
		}
		cfg.Duration = dur
	}

	if raw, ok := lookupSetting(settings, "total"); ok {
		val, err := asInt(raw)
		if err != nil {
			return fmt.Errorf("total: %w", err)
		}
		cfg.Total = val
	}

	if raw, ok := lookupSetting(settings, "timeout"); ok {
		dur, err := asDuration(raw)
		if err != nil {
			return fmt.Errorf("timeout: %w", err)
		}
		cfg.Timeout = dur
	}

	if raw, ok := lookupSetting(settings, "retries"); ok {
		val, err := asInt(raw)
		if err != nil {
			return fmt.Errorf("retries: %w", err)
		}
		cfg.Retries = val
	}

	if raw, ok := lookupSetting(settings, "jsonoutput", "json_output", "json-output"); ok {
		val, err := asBool(raw)
		if err != nil {
			return fmt.Errorf("jsonOutput: %w", err)
		}
		cfg.JSONOutput = val
	}

	if raw, ok := lookupSetting(settings, "dashboard"); ok {
		val, err := asBool(raw)
		if err != nil {
			return fmt.Errorf("dashboard: %w", err)
		}
		cfg.Dashboard = val
	}

	if raw, ok := lookupSetting(settings, "logerrors", "log_errors", "log-errors"); ok {
		val, err := asBool(raw)
		if err != nil {
			return fmt.Errorf("logErrors: %w", err)
		}
		cfg.LogErrors = val
	}

	if raw, ok := lookupSetting(settings, "htmloutput", "html_output", "html-output"); ok {
		val, err := asString(raw)
		if err != nil {
			return fmt.Errorf("htmlOutput: %w", err)
		}
		cfg.HTMLOutput = strings.TrimSpace(val)
	}

	if raw, ok := lookupSetting(settings, "loadpatterns", "load_patterns", "load-patterns"); ok {
		patterns, err := parseLoadPatterns(raw)
		if err != nil {
			return fmt.Errorf("loadPatterns: %w", err)
		}
		cfg.LoadPatterns = patterns
	}

	if raw, ok := lookupSetting(settings, "arrival"); ok {
		arrival, err := parseArrival(raw)
		if err != nil {
			return fmt.Errorf("arrival: %w", err)
		}
		if arrival.Model != "" {
			cfg.Arrival = arrival
		}
	} else if raw, ok := lookupSetting(settings, "arrivalmodel", "arrival_model", "arrival-model"); ok {
		arrival, err := parseArrival(raw)
		if err != nil {
			return fmt.Errorf("arrivalModel: %w", err)
		}
		if arrival.Model != "" {
			cfg.Arrival = arrival
		}
	}

	if raw, ok := lookupSetting(settings, "endpoints"); ok {
		endpoints, err := parseEndpoints(raw)
		if err != nil {
			return fmt.Errorf("endpoints: %w", err)
		}
		cfg.Endpoints = endpoints
	}

	if raw, ok := lookupSetting(settings, "auth"); ok {
		auth, err := parseAuth(raw)
		if err != nil {
			return fmt.Errorf("auth: %w", err)
		}
		cfg.Auth = auth
	}

	if raw, ok := lookupSetting(settings, "feeder"); ok {
		feeder, err := parseFeeder(raw)
		if err != nil {
			return fmt.Errorf("feeder: %w", err)
		}
		cfg.Feeder = feeder
	}

	if raw, ok := lookupSetting(settings, "protocol"); ok {
		val, err := asString(raw)
		if err != nil {
			return fmt.Errorf("protocol: %w", err)
		}
		cfg.Protocol = Protocol(strings.ToLower(strings.TrimSpace(val)))
	}

	if raw, ok := lookupSetting(settings, "websocket"); ok {
		ws, err := parseWebSocketConfig(raw)
		if err != nil {
			return fmt.Errorf("websocket: %w", err)
		}
		cfg.WebSocket = ws
	}

	if raw, ok := lookupSetting(settings, "sse"); ok {
		sse, err := parseSSEConfig(raw)
		if err != nil {
			return fmt.Errorf("sse: %w", err)
		}
		cfg.SSE = sse
	}

	if raw, ok := lookupSetting(settings, "grpc"); ok {
		grpc, err := parseGRPCConfig(raw)
		if err != nil {
			return fmt.Errorf("grpc: %w", err)
		}
		cfg.GRPC = grpc
	}

	if raw, ok := lookupSetting(settings, "thresholds"); ok {
		thresholds, err := asStringSlice(raw)
		if err != nil {
			return fmt.Errorf("thresholds: %w", err)
		}
		cfg.Thresholds = thresholds
	}

	if raw, ok := lookupSetting(settings, "harfile", "har_file", "har-file"); ok {
		val, err := asString(raw)
		if err != nil {
			return fmt.Errorf("harFile: %w", err)
		}
		cfg.HARFile = strings.TrimSpace(val)
	}

	if raw, ok := lookupSetting(settings, "harfilter", "har_filter", "har-filter"); ok {
		val, err := asString(raw)
		if err != nil {
			return fmt.Errorf("harFilter: %w", err)
		}
		cfg.HARFilter = strings.TrimSpace(val)
	}

	return nil
}

func parseLoadPatterns(value interface{}) ([]LoadPattern, error) {
	if value == nil {
		return nil, nil
	}
	items, err := toInterfaceSlice(value)
	if err != nil {
		return nil, err
	}
	patterns := make([]LoadPattern, 0, len(items))
	for idx, item := range items {
		entry, err := toStringKeyMap(item)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", idx, err)
		}
		pattern, err := buildLoadPattern(entry)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", idx, err)
		}
		patterns = append(patterns, pattern)
	}
	return patterns, nil
}

func buildLoadPattern(settings map[string]interface{}) (LoadPattern, error) {
	var pattern LoadPattern
	if raw, ok := lookupSetting(settings, "name"); ok {
		val, err := asString(raw)
		if err != nil {
			return LoadPattern{}, fmt.Errorf("name: %w", err)
		}
		pattern.Name = strings.TrimSpace(val)
	}
	if raw, ok := lookupSetting(settings, "type"); ok {
		val, err := asString(raw)
		if err != nil {
			return LoadPattern{}, fmt.Errorf("type: %w", err)
		}
		pattern.Type = LoadPatternType(strings.ToLower(strings.TrimSpace(val)))
	}
	if raw, ok := lookupSetting(settings, "fromrps", "from_rps", "from-rps"); ok {
		val, err := asInt(raw)
		if err != nil {
			return LoadPattern{}, fmt.Errorf("from_rps: %w", err)
		}
		pattern.FromRPS = val
	}
	if raw, ok := lookupSetting(settings, "torps", "to_rps", "to-rps"); ok {
		val, err := asInt(raw)
		if err != nil {
			return LoadPattern{}, fmt.Errorf("to_rps: %w", err)
		}
		pattern.ToRPS = val
	}
	if raw, ok := lookupSetting(settings, "duration"); ok {
		dur, err := asDuration(raw)
		if err != nil {
			return LoadPattern{}, fmt.Errorf("duration: %w", err)
		}
		pattern.Duration = dur
	}
	if raw, ok := lookupSetting(settings, "steps"); ok {
		steps, err := parseLoadSteps(raw)
		if err != nil {
			return LoadPattern{}, fmt.Errorf("steps: %w", err)
		}
		pattern.Steps = steps
	}
	if raw, ok := lookupSetting(settings, "rps"); ok {
		val, err := asInt(raw)
		if err != nil {
			return LoadPattern{}, fmt.Errorf("rps: %w", err)
		}
		pattern.RPS = val
	}
	return pattern, nil
}

func parseLoadSteps(value interface{}) ([]LoadStep, error) {
	items, err := toInterfaceSlice(value)
	if err != nil {
		return nil, err
	}
	steps := make([]LoadStep, 0, len(items))
	for idx, item := range items {
		entry, err := toStringKeyMap(item)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", idx, err)
		}
		var step LoadStep
		if raw, ok := lookupSetting(entry, "rps"); ok {
			val, err := asInt(raw)
			if err != nil {
				return nil, fmt.Errorf("index %d rps: %w", idx, err)
			}
			step.RPS = val
		}
		if raw, ok := lookupSetting(entry, "duration"); ok {
			dur, err := asDuration(raw)
			if err != nil {
				return nil, fmt.Errorf("index %d duration: %w", idx, err)
			}
			step.Duration = dur
		}
		steps = append(steps, step)
	}
	return steps, nil
}

func parseArrival(value interface{}) (ArrivalConfig, error) {
	if value == nil {
		return ArrivalConfig{}, nil
	}
	switch v := value.(type) {
	case string:
		model := strings.ToLower(strings.TrimSpace(v))
		if model == "" {
			return ArrivalConfig{}, nil
		}
		return ArrivalConfig{Model: ArrivalModel(model)}, nil
	default:
		entry, err := toStringKeyMap(value)
		if err != nil {
			return ArrivalConfig{}, err
		}
		if raw, ok := lookupSetting(entry, "model"); ok {
			val, err := asString(raw)
			if err != nil {
				return ArrivalConfig{}, fmt.Errorf("model: %w", err)
			}
			return ArrivalConfig{Model: ArrivalModel(strings.ToLower(strings.TrimSpace(val)))}, nil
		}
		return ArrivalConfig{}, fmt.Errorf("model field is required")
	}
}

func parseEndpoints(value interface{}) ([]Endpoint, error) {
	if value == nil {
		return nil, nil
	}
	items, err := toInterfaceSlice(value)
	if err != nil {
		return nil, err
	}
	endpoints := make([]Endpoint, 0, len(items))
	for idx, item := range items {
		entry, err := toStringKeyMap(item)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", idx, err)
		}
		endpoint, err := buildEndpoint(entry)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", idx, err)
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, nil
}

func buildEndpoint(settings map[string]interface{}) (Endpoint, error) {
	endpoint := Endpoint{Weight: 1}
	if raw, ok := lookupSetting(settings, "name"); ok {
		val, err := asString(raw)
		if err != nil {
			return Endpoint{}, fmt.Errorf("name: %w", err)
		}
		endpoint.Name = strings.TrimSpace(val)
	}
	if raw, ok := lookupSetting(settings, "weight"); ok {
		val, err := asInt(raw)
		if err != nil {
			return Endpoint{}, fmt.Errorf("weight: %w", err)
		}
		endpoint.Weight = val
	}
	if raw, ok := lookupSetting(settings, "method"); ok {
		val, err := asString(raw)
		if err != nil {
			return Endpoint{}, fmt.Errorf("method: %w", err)
		}
		endpoint.Method = strings.ToUpper(strings.TrimSpace(val))
	}
	if raw, ok := lookupSetting(settings, "url", "target"); ok {
		val, err := asString(raw)
		if err != nil {
			return Endpoint{}, fmt.Errorf("url: %w", err)
		}
		endpoint.URL = strings.TrimSpace(val)
	}
	if raw, ok := lookupSetting(settings, "path"); ok {
		val, err := asString(raw)
		if err != nil {
			return Endpoint{}, fmt.Errorf("path: %w", err)
		}
		endpoint.Path = strings.TrimSpace(val)
	}
	if raw, ok := lookupSetting(settings, "body"); ok {
		val, err := asString(raw)
		if err != nil {
			return Endpoint{}, fmt.Errorf("body: %w", err)
		}
		endpoint.Body = val
	}
	if raw, ok := lookupSetting(settings, "bodyfile", "body_file", "body-file"); ok {
		val, err := asString(raw)
		if err != nil {
			return Endpoint{}, fmt.Errorf("bodyFile: %w", err)
		}
		endpoint.BodyFile = val
	}
	if raw, ok := lookupSetting(settings, "headers"); ok {
		hdrs, err := asStringMap(raw)
		if err != nil {
			return Endpoint{}, fmt.Errorf("headers: %w", err)
		}
		if len(hdrs) > 0 {
			endpoint.Headers = map[string]string{}
			for key, value := range hdrs {
				trimmedKey := strings.TrimSpace(key)
				if trimmedKey == "" {
					return Endpoint{}, fmt.Errorf("headers: key cannot be empty")
				}
				canonical := http.CanonicalHeaderKey(trimmedKey)
				endpoint.Headers[canonical] = value
			}
		}
	}
	if raw, ok := lookupSetting(settings, "extractors"); ok {
		extractors, err := parseExtractors(raw)
		if err != nil {
			return Endpoint{}, fmt.Errorf("extractors: %w", err)
		}
		endpoint.Extractors = extractors
	}
	return endpoint, nil
}

func parseExtractors(value interface{}) ([]Extractor, error) {
	if value == nil {
		return nil, nil
	}
	items, err := toInterfaceSlice(value)
	if err != nil {
		return nil, err
	}
	extractors := make([]Extractor, 0, len(items))
	for idx, item := range items {
		entry, err := toStringKeyMap(item)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", idx, err)
		}
		extractor, err := buildExtractor(entry)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", idx, err)
		}
		extractors = append(extractors, extractor)
	}
	return extractors, nil
}

func buildExtractor(settings map[string]interface{}) (Extractor, error) {
	var extractor Extractor
	if raw, ok := lookupSetting(settings, "jsonpath"); ok {
		val, err := asString(raw)
		if err != nil {
			return Extractor{}, fmt.Errorf("jsonpath: %w", err)
		}
		extractor.JSONPath = strings.TrimSpace(val)
	}
	if raw, ok := lookupSetting(settings, "regex"); ok {
		val, err := asString(raw)
		if err != nil {
			return Extractor{}, fmt.Errorf("regex: %w", err)
		}
		extractor.Regex = strings.TrimSpace(val)
	}
	if raw, ok := lookupSetting(settings, "var"); ok {
		val, err := asString(raw)
		if err != nil {
			return Extractor{}, fmt.Errorf("var: %w", err)
		}
		extractor.Variable = strings.TrimSpace(val)
	}
	if raw, ok := lookupSetting(settings, "onerror", "on_error", "on-error"); ok {
		val, err := asBool(raw)
		if err != nil {
			return Extractor{}, fmt.Errorf("on_error: %w", err)
		}
		extractor.OnError = val
	}
	return extractor, nil
}

func parseAuth(value interface{}) (AuthConfig, error) {
	if value == nil {
		return AuthConfig{}, nil
	}
	entry, err := toStringKeyMap(value)
	if err != nil {
		return AuthConfig{}, err
	}
	return buildAuthConfig(entry)
}

func buildAuthConfig(settings map[string]interface{}) (AuthConfig, error) {
	var auth AuthConfig
	if raw, ok := lookupSetting(settings, "type"); ok {
		val, err := asString(raw)
		if err != nil {
			return AuthConfig{}, fmt.Errorf("type: %w", err)
		}
		auth.Type = AuthType(strings.ToLower(strings.TrimSpace(val)))
	}
	if raw, ok := lookupSetting(settings, "tokenurl", "token_url", "token-url"); ok {
		val, err := asString(raw)
		if err != nil {
			return AuthConfig{}, fmt.Errorf("token_url: %w", err)
		}
		auth.TokenURL = strings.TrimSpace(val)
	}
	if raw, ok := lookupSetting(settings, "clientid", "client_id", "client-id"); ok {
		val, err := asString(raw)
		if err != nil {
			return AuthConfig{}, fmt.Errorf("client_id: %w", err)
		}
		auth.ClientID = strings.TrimSpace(val)
	}
	if raw, ok := lookupSetting(settings, "clientsecret", "client_secret", "client-secret"); ok {
		val, err := asString(raw)
		if err != nil {
			return AuthConfig{}, fmt.Errorf("client_secret: %w", err)
		}
		auth.ClientSecret = strings.TrimSpace(val)
	}
	// Fallback to environment variable if client_secret is empty
	if auth.ClientSecret == "" {
		if envSecret := os.Getenv("CRANKFIRE_AUTH_CLIENT_SECRET"); envSecret != "" {
			auth.ClientSecret = envSecret
		}
	}
	if raw, ok := lookupSetting(settings, "username"); ok {
		val, err := asString(raw)
		if err != nil {
			return AuthConfig{}, fmt.Errorf("username: %w", err)
		}
		auth.Username = strings.TrimSpace(val)
	}
	if raw, ok := lookupSetting(settings, "password"); ok {
		val, err := asString(raw)
		if err != nil {
			return AuthConfig{}, fmt.Errorf("password: %w", err)
		}
		auth.Password = strings.TrimSpace(val)
	}
	// Fallback to environment variable if password is empty
	if auth.Password == "" {
		if envPassword := os.Getenv("CRANKFIRE_AUTH_PASSWORD"); envPassword != "" {
			auth.Password = envPassword
		}
	}
	if raw, ok := lookupSetting(settings, "scopes"); ok {
		scopes, err := asStringSlice(raw)
		if err != nil {
			return AuthConfig{}, fmt.Errorf("scopes: %w", err)
		}
		auth.Scopes = scopes
	}
	if raw, ok := lookupSetting(settings, "statictoken", "static_token", "static-token"); ok {
		val, err := asString(raw)
		if err != nil {
			return AuthConfig{}, fmt.Errorf("static_token: %w", err)
		}
		auth.StaticToken = strings.TrimSpace(val)
	}
	// Fallback to environment variable if static_token is empty
	if auth.StaticToken == "" {
		if envToken := os.Getenv("CRANKFIRE_AUTH_STATIC_TOKEN"); envToken != "" {
			auth.StaticToken = envToken
		}
	}
	if raw, ok := lookupSetting(settings, "refreshbeforeexpiry", "refresh_before_expiry", "refresh-before-expiry"); ok {
		dur, err := asDuration(raw)
		if err != nil {
			return AuthConfig{}, fmt.Errorf("refresh_before_expiry: %w", err)
		}
		auth.RefreshBeforeExpiry = dur
	}
	return auth, nil
}

func parseFeeder(value interface{}) (FeederConfig, error) {
	if value == nil {
		return FeederConfig{}, nil
	}
	entry, err := toStringKeyMap(value)
	if err != nil {
		return FeederConfig{}, err
	}
	return buildFeederConfig(entry)
}

func buildFeederConfig(settings map[string]interface{}) (FeederConfig, error) {
	var feeder FeederConfig
	if raw, ok := lookupSetting(settings, "path"); ok {
		val, err := asString(raw)
		if err != nil {
			return FeederConfig{}, fmt.Errorf("path: %w", err)
		}
		feeder.Path = strings.TrimSpace(val)
	}
	if raw, ok := lookupSetting(settings, "type"); ok {
		val, err := asString(raw)
		if err != nil {
			return FeederConfig{}, fmt.Errorf("type: %w", err)
		}
		feeder.Type = strings.ToLower(strings.TrimSpace(val))
	}
	return feeder, nil
}

func parseWebSocketConfig(value interface{}) (WebSocketConfig, error) {
	if value == nil {
		return WebSocketConfig{}, nil
	}
	entry, err := toStringKeyMap(value)
	if err != nil {
		return WebSocketConfig{}, err
	}
	return buildWebSocketConfig(entry)
}

func buildWebSocketConfig(settings map[string]interface{}) (WebSocketConfig, error) {
	var ws WebSocketConfig
	if raw, ok := lookupSetting(settings, "messages"); ok {
		messages, err := asStringSlice(raw)
		if err != nil {
			return WebSocketConfig{}, fmt.Errorf("messages: %w", err)
		}
		ws.Messages = messages
	}
	if raw, ok := lookupSetting(settings, "messageinterval", "message_interval", "message-interval"); ok {
		dur, err := asDuration(raw)
		if err != nil {
			return WebSocketConfig{}, fmt.Errorf("message_interval: %w", err)
		}
		ws.MessageInterval = dur
	}
	if raw, ok := lookupSetting(settings, "receivetimeout", "receive_timeout", "receive-timeout"); ok {
		dur, err := asDuration(raw)
		if err != nil {
			return WebSocketConfig{}, fmt.Errorf("receive_timeout: %w", err)
		}
		ws.ReceiveTimeout = dur
	}
	if raw, ok := lookupSetting(settings, "handshaketimeout", "handshake_timeout", "handshake-timeout"); ok {
		dur, err := asDuration(raw)
		if err != nil {
			return WebSocketConfig{}, fmt.Errorf("handshake_timeout: %w", err)
		}
		ws.HandshakeTimeout = dur
	}
	return ws, nil
}

func parseSSEConfig(value interface{}) (SSEConfig, error) {
	if value == nil {
		return SSEConfig{}, nil
	}
	entry, err := toStringKeyMap(value)
	if err != nil {
		return SSEConfig{}, err
	}
	return buildSSEConfig(entry)
}

func buildSSEConfig(settings map[string]interface{}) (SSEConfig, error) {
	var sse SSEConfig
	if raw, ok := lookupSetting(settings, "readtimeout", "read_timeout", "read-timeout"); ok {
		dur, err := asDuration(raw)
		if err != nil {
			return SSEConfig{}, fmt.Errorf("read_timeout: %w", err)
		}
		sse.ReadTimeout = dur
	}
	if raw, ok := lookupSetting(settings, "maxevents", "max_events", "max-events"); ok {
		val, err := asInt(raw)
		if err != nil {
			return SSEConfig{}, fmt.Errorf("max_events: %w", err)
		}
		sse.MaxEvents = val
	}
	return sse, nil
}

func parseGRPCConfig(value interface{}) (GRPCConfig, error) {
	if value == nil {
		return GRPCConfig{}, nil
	}
	entry, err := toStringKeyMap(value)
	if err != nil {
		return GRPCConfig{}, err
	}
	return buildGRPCConfig(entry)
}

func buildGRPCConfig(settings map[string]interface{}) (GRPCConfig, error) {
	var grpc GRPCConfig
	if raw, ok := lookupSetting(settings, "protofile", "proto_file", "proto-file"); ok {
		val, err := asString(raw)
		if err != nil {
			return GRPCConfig{}, fmt.Errorf("proto_file: %w", err)
		}
		grpc.ProtoFile = val
	}
	if raw, ok := lookupSetting(settings, "service"); ok {
		val, err := asString(raw)
		if err != nil {
			return GRPCConfig{}, fmt.Errorf("service: %w", err)
		}
		grpc.Service = val
	}
	if raw, ok := lookupSetting(settings, "method"); ok {
		val, err := asString(raw)
		if err != nil {
			return GRPCConfig{}, fmt.Errorf("method: %w", err)
		}
		grpc.Method = val
	}
	if raw, ok := lookupSetting(settings, "message"); ok {
		val, err := asString(raw)
		if err != nil {
			return GRPCConfig{}, fmt.Errorf("message: %w", err)
		}
		grpc.Message = val
	}
	if raw, ok := lookupSetting(settings, "metadata"); ok {
		val, err := asStringMap(raw)
		if err != nil {
			return GRPCConfig{}, fmt.Errorf("metadata: %w", err)
		}
		grpc.Metadata = val
	}
	if raw, ok := lookupSetting(settings, "timeout"); ok {
		dur, err := asDuration(raw)
		if err != nil {
			return GRPCConfig{}, fmt.Errorf("timeout: %w", err)
		}
		grpc.Timeout = dur
	}
	if raw, ok := lookupSetting(settings, "tls"); ok {
		val, err := asBool(raw)
		if err != nil {
			return GRPCConfig{}, fmt.Errorf("tls: %w", err)
		}
		grpc.TLS = val
	}
	if raw, ok := lookupSetting(settings, "insecure"); ok {
		val, err := asBool(raw)
		if err != nil {
			return GRPCConfig{}, fmt.Errorf("insecure: %w", err)
		}
		grpc.Insecure = val
	}
	return grpc, nil
}

// parseHARFilter parses a HAR filter string and returns a map representation.
// Format examples:
//   - "host:example.com"
//   - "host:api.example.com,cdn.example.com"
//   - "method:GET"
//   - "method:GET,POST"
//   - "host:example.com;method:GET,POST"
func parseHARFilter(filter string) map[string][]string {
	opts := make(map[string][]string)

	if filter == "" {
		return opts
	}

	parts := strings.Split(filter, ";")
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(kv[0]))
		value := strings.TrimSpace(kv[1])

		switch key {
		case "host":
			hosts := strings.Split(value, ",")
			for i := range hosts {
				hosts[i] = strings.TrimSpace(hosts[i])
			}
			opts["hosts"] = hosts
		case "method":
			methods := strings.Split(value, ",")
			for i := range methods {
				methods[i] = strings.TrimSpace(methods[i])
			}
			opts["methods"] = methods
		}
	}

	return opts
}

// loadHAREndpoints validates that the HAR file exists and is readable JSON.
// The actual HAR conversion to endpoints happens separately via LoadHAREndpointsFromConfig function
// which is called from the cmd layer to avoid circular import issues.
func loadHAREndpoints(cfg *Config) error {
	if strings.TrimSpace(cfg.HARFile) == "" {
		return nil
	}

	// Verify the file exists and is readable
	data, err := os.ReadFile(cfg.HARFile)
	if err != nil {
		return fmt.Errorf("failed to open HAR file: %w", err)
	}

	// Validate it's valid JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to parse HAR JSON: %w", err)
	}

	// Print security warning
	fmt.Fprintln(os.Stderr, "WARNING: HAR file may contain sensitive data (cookies, auth tokens). Review endpoints before use in production.")

	return nil
}
