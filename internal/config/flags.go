package config

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// RegisterFlags registers all CLI flags to a cobra command.
func RegisterFlags(cmd *cobra.Command) {
	configureFlags(cmd.Flags())
}

// newFlagCommand creates a cobra command with all flags configured.
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

// configureFlags sets up all CLI flags on the provided flag set.
func configureFlags(flags *pflag.FlagSet) {
	// Core request flags
	flags.String("target", "", "Target URL to load test")
	flags.String("method", "GET", "HTTP method to use")
	flags.StringSlice("header", nil, "Additional request header in key=value form")
	flags.String("body", "", "Inline request body payload")
	flags.String("body-file", "", "Path to file containing the request body")

	// Load control flags
	flags.IntP("concurrency", "c", 1, "Number of concurrent workers")
	flags.IntP("rate", "r", 0, "Requests per second limit (0 means unlimited)")
	flags.DurationP("duration", "d", 0, "How long to run the test (e.g. 30s, 1m)")
	flags.IntP("total", "t", 0, "Total number of requests to send (0 means unlimited)")
	flags.Duration("timeout", 30*time.Second, "Per-request timeout")
	flags.Duration("graceful-shutdown", 5*time.Second, "Max time to wait for in-flight requests after test ends (0=default, negative=cancel immediately)")
	flags.Int("retries", 0, "Number of retries per request")
	flags.String("arrival-model", string(ArrivalModelUniform), "Arrival model to use when pacing requests (uniform or poisson)")

	// Output flags
	flags.Bool("json-output", false, "Emit JSON formatted output")
	flags.Bool("dashboard", false, "Show live terminal dashboard with metrics")
	flags.Bool("log-errors", false, "Log each failed request to stderr")
	flags.String("html-output", "", "Generate HTML report to the specified file path")
	flags.String("config", "", "Path to configuration file (JSON or YAML)")

	// Feeder flags
	flags.String("feeder-path", "", "Path to CSV or JSON file containing data for per-request injection")
	flags.String("feeder-type", "", "Type of feeder file: 'csv' or 'json'")

	// Protocol flags
	flags.String("protocol", "http", "Protocol mode: 'http', 'websocket', 'sse', or 'grpc'")

	// WebSocket flags
	flags.StringSlice("ws-messages", nil, "WebSocket messages to send (repeatable)")
	flags.Duration("ws-message-interval", 0, "Interval between WebSocket messages")
	flags.Duration("ws-receive-timeout", 10*time.Second, "WebSocket receive timeout")
	flags.Duration("ws-handshake-timeout", 30*time.Second, "WebSocket handshake timeout")

	// SSE flags
	flags.Duration("sse-read-timeout", 30*time.Second, "SSE read timeout")
	flags.Int("sse-max-events", 0, "Max SSE events to read (0=unlimited)")

	// gRPC flags
	flags.String("grpc-proto-file", "", "Path to .proto file for gRPC")
	flags.String("grpc-service", "", "gRPC service name (e.g., helloworld.Greeter)")
	flags.String("grpc-method", "", "gRPC method name (e.g., SayHello)")
	flags.String("grpc-message", "", "gRPC message payload (JSON format)")
	flags.StringToString("grpc-metadata", nil, "gRPC metadata key=value pairs")
	flags.Duration("grpc-timeout", 30*time.Second, "gRPC per-call timeout")
	flags.Bool("grpc-tls", false, "Use TLS for gRPC connection")
	flags.Bool("grpc-insecure", false, "Skip TLS verification for gRPC")

	// Threshold flags
	flags.StringSlice("threshold", nil, "Performance thresholds (repeatable, e.g., 'http_req_duration:p95 < 500')")

	// HAR import flags
	flags.String("har", "", "Path to HAR file to import as endpoints")
	flags.String("har-filter", "", "Filter HAR entries (e.g., 'host:example.com' or 'method:GET,POST')")
}

// displayHelp prints the help message for a command.
func displayHelp(cmd *cobra.Command) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Usage: %s\n\nFlags:\n", cmd.UseLine())
	fs := cmd.Flags()
	fs.SetOutput(out)
	fs.PrintDefaults()
}

// applyFlagOverrides applies command-line flag values to the config, overriding
// values from the config file.
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
	if fs.Changed("graceful-shutdown") {
		val, err := fs.GetDuration("graceful-shutdown")
		if err != nil {
			return err
		}
		cfg.GracefulShutdown = val
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
	if fs.Changed("html-output") {
		val, err := fs.GetString("html-output")
		if err != nil {
			return err
		}
		cfg.HTMLOutput = strings.TrimSpace(val)
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

	if fs.Changed("har") {
		val, err := fs.GetString("har")
		if err != nil {
			return err
		}
		cfg.HARFile = strings.TrimSpace(val)
	}
	if fs.Changed("har-filter") {
		val, err := fs.GetString("har-filter")
		if err != nil {
			return err
		}
		cfg.HARFilter = strings.TrimSpace(val)
	}

	return nil
}
