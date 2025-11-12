package config

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Loader struct{}

func NewLoader() *Loader {
	return &Loader{}
}

func (Loader) Load(args []string) (*Config, error) {
	cmd := newFlagCommand()
	if err := cmd.Flags().Parse(args); err != nil {
		return nil, err
	}

	flagSet := cmd.Flags()
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
	configureFlags(cmd.Flags())
	return cmd
}

func configureFlags(flags *pflag.FlagSet) {
	flags.String("target", "", "Target URL to load test")
	flags.String("method", "GET", "HTTP method to use")
	flags.StringSlice("header", nil, "Additional request header in key=value form")
	flags.String("body", "", "Inline request body payload")
	flags.String("body-file", "", "Path to file containing the request body")
	flags.Int("concurrency", 1, "Number of concurrent workers")
	flags.Int("rate", 0, "Requests per second limit (0 means unlimited)")
	flags.Duration("duration", 0, "How long to run the test (e.g. 30s, 1m)")
	flags.Int("total", 0, "Total number of requests to send (0 means unlimited)")
	flags.Duration("timeout", 30*time.Second, "Per-request timeout")
	flags.Int("retries", 0, "Number of retries per request")
	flags.Bool("json-output", false, "Emit JSON formatted output")
	flags.Bool("dashboard", false, "Show live terminal dashboard with metrics")
	flags.String("config", "", "Path to configuration file (JSON or YAML)")
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
