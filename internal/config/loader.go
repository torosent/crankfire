package config

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Loader struct{}

var ErrHelpRequested = errors.New("help requested")

func NewLoader() *Loader {
	return &Loader{}
}

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
	configPath := flagSet.Lookup("config").Value.String()
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

	return cfg, nil
}

func RegisterFlags(cmd *cobra.Command) {
	configureFlags(cmd.Flags())
}

func newFlagCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "crankfire",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.SetOut(os.Stdout)
	configureFlags(cmd.Flags())
	return cmd
}

func configureFlags(flags *pflag.FlagSet) {
	flags.String("target", "", "Target URL to load test")
	flags.String("method", "GET", "HTTP method to use")
	flags.StringSlice("header", nil, "Additional request header in key=value form")
	flags.String("body", "", "Inline request body payload")
	flags.String("body-file", "", "Path to file containing the request body")
	flags.IntP("concurrency", "c", 1, "Number of concurrent workers")
	flags.IntP("rate", "r", 0, "Requests per second limit (0 means unlimited)")
	flags.DurationP("duration", "d", 0, "How long to run the test (e.g. 30s, 1m)")
	flags.IntP("total", "t", 0, "Total number of requests to send (0 means unlimited)")
	flags.Duration("timeout", 30*time.Second, "Per-request timeout")
	flags.Int("retries", 0, "Number of retries per request")
	flags.String("arrival-model", string(ArrivalModelUniform), "Arrival model to use when pacing requests (uniform or poisson)")
	flags.Bool("json-output", false, "Emit JSON formatted output")
	flags.Bool("dashboard", false, "Show live terminal dashboard with metrics")
	flags.Bool("log-errors", false, "Log each failed request to stderr")
	flags.String("config", "", "Path to configuration file (JSON or YAML)")

	flags.String("feeder-path", "", "Path to CSV or JSON file containing data for per-request injection")
	flags.String("feeder-type", "", "Type of feeder file: 'csv' or 'json'")

	flags.String("protocol", "http", "Protocol mode: 'http', 'websocket', 'sse', or 'grpc'")
	flags.StringSlice("ws-messages", nil, "WebSocket messages to send (repeatable)")
	flags.Duration("ws-message-interval", 0, "Interval between WebSocket messages")
	flags.Duration("ws-receive-timeout", 10*time.Second, "WebSocket receive timeout")
	flags.Duration("ws-handshake-timeout", 30*time.Second, "WebSocket handshake timeout")
	flags.Duration("sse-read-timeout", 30*time.Second, "SSE read timeout")
	flags.Int("sse-max-events", 0, "Max SSE events to read (0=unlimited)")
	flags.String("grpc-proto-file", "", "Path to .proto file for gRPC")
	flags.String("grpc-service", "", "gRPC service name (e.g., helloworld.Greeter)")
	flags.String("grpc-method", "", "gRPC method name (e.g., SayHello)")
	flags.String("grpc-message", "", "gRPC message payload (JSON format)")
	flags.StringToString("grpc-metadata", nil, "gRPC metadata key=value pairs")
	flags.Duration("grpc-timeout", 30*time.Second, "gRPC per-call timeout")
	flags.Bool("grpc-tls", false, "Use TLS for gRPC connection")
	flags.Bool("grpc-insecure", false, "Skip TLS verification for gRPC")
	flags.StringSlice("threshold", nil, "Performance thresholds (repeatable, e.g., 'http_req_duration:p95 < 500')")
}

func displayHelp(cmd *cobra.Command) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Usage: %s\n\nFlags:\n", cmd.UseLine())
	fs := cmd.Flags()
	fs.SetOutput(out)
	fs.PrintDefaults()
}

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

	return nil
}

func applyFlagOverrides(cfg *Config, fs *pflag.FlagSet) error {
	if fs.Changed("target") {
		val, err := fs.GetString("target")
		if err != nil {
			return err
		}
		cfg.TargetURL = strings.TrimSpace(val)
	}
	if fs.Changed("method") {
		val, err := fs.GetString("method")
		if err != nil {
			return err
		}
		cfg.Method = val
	}
	if fs.Changed("body") {
		val, err := fs.GetString("body")
		if err != nil {
			return err
		}
		cfg.Body = val
		cfg.BodyFile = ""
	}
	if fs.Changed("body-file") {
		val, err := fs.GetString("body-file")
		if err != nil {
			return err
		}
		cfg.BodyFile = val
		cfg.Body = ""
	}
	if fs.Changed("concurrency") {
		val, err := fs.GetInt("concurrency")
		if err != nil {
			return err
		}
		cfg.Concurrency = val
	}
	if fs.Changed("rate") {
		val, err := fs.GetInt("rate")
		if err != nil {
			return err
		}
		cfg.Rate = val
	}
	if fs.Changed("duration") {
		val, err := fs.GetDuration("duration")
		if err != nil {
			return err
		}
		cfg.Duration = val
	}
	if fs.Changed("total") {
		val, err := fs.GetInt("total")
		if err != nil {
			return err
		}
		cfg.Total = val
	}
	if fs.Changed("timeout") {
		val, err := fs.GetDuration("timeout")
		if err != nil {
			return err
		}
		cfg.Timeout = val
	}
	if fs.Changed("retries") {
		val, err := fs.GetInt("retries")
		if err != nil {
			return err
		}
		cfg.Retries = val
	}
	if fs.Changed("arrival-model") {
		val, err := fs.GetString("arrival-model")
		if err != nil {
			return err
		}
		cfg.Arrival.Model = ArrivalModel(strings.ToLower(strings.TrimSpace(val)))
	}
	if fs.Changed("json-output") {
		val, err := fs.GetBool("json-output")
		if err != nil {
			return err
		}
		cfg.JSONOutput = val
	}
	if fs.Changed("dashboard") {
		val, err := fs.GetBool("dashboard")
		if err != nil {
			return err
		}
		cfg.Dashboard = val
	}
	if fs.Changed("log-errors") {
		val, err := fs.GetBool("log-errors")
		if err != nil {
			return err
		}
		cfg.LogErrors = val
	}

	vals, err := fs.GetStringSlice("header")
	if err != nil {
		return err
	}
	if len(vals) > 0 {
		if cfg.Headers == nil {
			cfg.Headers = map[string]string{}
		}
		for _, entry := range vals {
			parts := strings.SplitN(entry, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("header must be in key=value format: %s", entry)
			}
			key := http.CanonicalHeaderKey(strings.TrimSpace(parts[0]))
			if key == "" {
				return fmt.Errorf("header key cannot be empty")
			}
			cfg.Headers[key] = strings.TrimSpace(parts[1])
		}
	}

	if fs.Changed("feeder-path") {
		val, err := fs.GetString("feeder-path")
		if err != nil {
			return err
		}
		cfg.Feeder.Path = strings.TrimSpace(val)
	}
	if fs.Changed("feeder-type") {
		val, err := fs.GetString("feeder-type")
		if err != nil {
			return err
		}
		cfg.Feeder.Type = strings.TrimSpace(val)
	}

	if fs.Changed("protocol") {
		val, err := fs.GetString("protocol")
		if err != nil {
			return err
		}
		cfg.Protocol = Protocol(strings.ToLower(strings.TrimSpace(val)))
	}
	if fs.Changed("ws-messages") {
		val, err := fs.GetStringSlice("ws-messages")
		if err != nil {
			return err
		}
		cfg.WebSocket.Messages = val
	}
	if fs.Changed("ws-message-interval") {
		val, err := fs.GetDuration("ws-message-interval")
		if err != nil {
			return err
		}
		cfg.WebSocket.MessageInterval = val
	}
	if fs.Changed("ws-receive-timeout") {
		val, err := fs.GetDuration("ws-receive-timeout")
		if err != nil {
			return err
		}
		cfg.WebSocket.ReceiveTimeout = val
	}
	if fs.Changed("ws-handshake-timeout") {
		val, err := fs.GetDuration("ws-handshake-timeout")
		if err != nil {
			return err
		}
		cfg.WebSocket.HandshakeTimeout = val
	}
	if fs.Changed("sse-read-timeout") {
		val, err := fs.GetDuration("sse-read-timeout")
		if err != nil {
			return err
		}
		cfg.SSE.ReadTimeout = val
	}
	if fs.Changed("sse-max-events") {
		val, err := fs.GetInt("sse-max-events")
		if err != nil {
			return err
		}
		cfg.SSE.MaxEvents = val
	}
	if fs.Changed("grpc-proto-file") {
		val, err := fs.GetString("grpc-proto-file")
		if err != nil {
			return err
		}
		cfg.GRPC.ProtoFile = val
	}
	if fs.Changed("grpc-service") {
		val, err := fs.GetString("grpc-service")
		if err != nil {
			return err
		}
		cfg.GRPC.Service = val
	}
	if fs.Changed("grpc-method") {
		val, err := fs.GetString("grpc-method")
		if err != nil {
			return err
		}
		cfg.GRPC.Method = val
	}
	if fs.Changed("grpc-message") {
		val, err := fs.GetString("grpc-message")
		if err != nil {
			return err
		}
		cfg.GRPC.Message = val
	}
	if fs.Changed("grpc-metadata") {
		val, err := fs.GetStringToString("grpc-metadata")
		if err != nil {
			return err
		}
		cfg.GRPC.Metadata = val
	}
	if fs.Changed("grpc-timeout") {
		val, err := fs.GetDuration("grpc-timeout")
		if err != nil {
			return err
		}
		cfg.GRPC.Timeout = val
	}
	if fs.Changed("grpc-tls") {
		val, err := fs.GetBool("grpc-tls")
		if err != nil {
			return err
		}
		cfg.GRPC.TLS = val
	}
	if fs.Changed("grpc-insecure") {
		val, err := fs.GetBool("grpc-insecure")
		if err != nil {
			return err
		}
		cfg.GRPC.Insecure = val
	}
	if fs.Changed("threshold") {
		val, err := fs.GetStringSlice("threshold")
		if err != nil {
			return err
		}
		cfg.Thresholds = val
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
	return endpoint, nil
}

func toInterfaceSlice(value interface{}) ([]interface{}, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case []interface{}:
		return v, nil
	case []map[string]interface{}:
		items := make([]interface{}, len(v))
		for i := range v {
			items[i] = v[i]
		}
		return items, nil
	case []map[interface{}]interface{}:
		items := make([]interface{}, len(v))
		for i := range v {
			items[i] = v[i]
		}
		return items, nil
	default:
		return nil, fmt.Errorf("expected list, got %T", value)
	}
}

func toStringKeyMap(value interface{}) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	switch v := value.(type) {
	case map[string]interface{}:
		for key, val := range v {
			result[strings.ToLower(strings.TrimSpace(key))] = val
		}
	case map[interface{}]interface{}:
		for key, val := range v {
			str, err := asString(key)
			if err != nil {
				return nil, err
			}
			result[strings.ToLower(strings.TrimSpace(str))] = val
		}
	default:
		return nil, fmt.Errorf("expected map, got %T", value)
	}
	return result, nil
}

func lookupSetting(settings map[string]interface{}, candidates ...string) (interface{}, bool) {
	for _, key := range candidates {
		if val, ok := settings[key]; ok {
			return val, true
		}
		lower := strings.ToLower(key)
		if val, ok := settings[lower]; ok {
			return val, true
		}
	}
	return nil, false
}

func asString(value interface{}) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case fmt.Stringer:
		return v.String(), nil
	case []byte:
		return string(v), nil
	default:
		return fmt.Sprint(v), nil
	}
}

func asInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case nil:
		return 0, nil
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case uint:
		return int(v), nil
	case uint8:
		return int(v), nil
	case uint16:
		return int(v), nil
	case uint32:
		return int(v), nil
	case uint64:
		return int(v), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, nil
		}
		i, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, err
		}
		return i, nil
	default:
		return 0, fmt.Errorf("unsupported numeric type %T", value)
	}
}

func asBool(value interface{}) (bool, error) {
	switch v := value.(type) {
	case nil:
		return false, nil
	case bool:
		return v, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return false, nil
		}
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return false, err
		}
		return b, nil
	default:
		return false, fmt.Errorf("unsupported boolean type %T", value)
	}
}

func asDuration(value interface{}) (time.Duration, error) {
	switch v := value.(type) {
	case nil:
		return 0, nil
	case time.Duration:
		return v, nil
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return 0, nil
		}
		d, err := time.ParseDuration(v)
		if err != nil {
			return 0, err
		}
		return d, nil
	case int, int8, int16, int32, int64:
		iv, _ := asInt(v)
		return time.Duration(iv) * time.Second, nil
	case uint, uint8, uint16, uint32, uint64:
		iv, _ := asInt(v)
		return time.Duration(iv) * time.Second, nil
	case float32, float64:
		iv, _ := asInt(v)
		return time.Duration(iv) * time.Second, nil
	default:
		return 0, fmt.Errorf("unsupported duration type %T", value)
	}
}

func asStringMap(value interface{}) (map[string]string, error) {
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case map[string]string:
		result := make(map[string]string, len(v))
		for k, val := range v {
			result[k] = val
		}
		return result, nil
	case map[string]interface{}:
		result := make(map[string]string, len(v))
		for k, val := range v {
			str, err := asString(val)
			if err != nil {
				return nil, err
			}
			result[k] = str
		}
		return result, nil
	case map[interface{}]interface{}:
		result := make(map[string]string, len(v))
		for k, val := range v {
			key, err := asString(k)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(key) == "" {
				return nil, fmt.Errorf("header key cannot be empty")
			}
			str, err := asString(val)
			if err != nil {
				return nil, err
			}
			result[key] = str
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported headers type %T", value)
	}
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

func asStringSlice(value interface{}) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case []string:
		return v, nil
	case []interface{}:
		result := make([]string, len(v))
		for i, item := range v {
			str, err := asString(item)
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			result[i] = str
		}
		return result, nil
	case string:
		return []string{v}, nil
	default:
		return nil, fmt.Errorf("unsupported string slice type %T", value)
	}
}
