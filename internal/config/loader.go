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
