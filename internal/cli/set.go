package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/output/setreport"
	"github.com/torosent/crankfire/internal/setrunner"
	"github.com/torosent/crankfire/internal/store"
)

// Exit codes.
const (
	ExitOK              = 0
	ExitUsage           = 1
	ExitThresholdFailed = 2
	ExitRunnerError     = 3
)

// RunSet is the entry point invoked from cmd/crankfire/main.go for `set ...`.
// stdout / stderr are injected so tests can capture them.
func RunSet(ctx context.Context, st store.Store, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: crankfire set <list|show|run> [args]")
		return ExitUsage
	}
	switch args[0] {
	case "list":
		return setList(ctx, st, stdout, stderr)
	case "show":
		return setShow(ctx, st, args[1:], stdout, stderr)
	case "run":
		return setRun(ctx, st, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand: %s\n", args[0])
		return ExitUsage
	}
}

func setList(ctx context.Context, st store.Store, stdout, stderr io.Writer) int {
	sets, err := st.ListSets(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "list sets: %v\n", err)
		return ExitRunnerError
	}
	if len(sets) == 0 {
		fmt.Fprintln(stdout, "No sets defined. Create one in the TUI (crankfire tui) or by writing a YAML file.")
		return ExitOK
	}
	fmt.Fprintf(stdout, "%-26s  %-30s  %s\n", "ID", "NAME", "STAGES")
	for _, s := range sets {
		fmt.Fprintf(stdout, "%-26s  %-30s  %d\n", s.ID, s.Name, len(s.Stages))
	}
	return ExitOK
}

func setShow(ctx context.Context, st store.Store, args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "usage: crankfire set show <id>")
		return ExitUsage
	}
	set, err := st.GetSet(ctx, args[0])
	if err != nil {
		fmt.Fprintf(stderr, "get set: %v\n", err)
		return ExitUsage
	}
	data, err := yaml.Marshal(set)
	if err != nil {
		fmt.Fprintf(stderr, "marshal: %v\n", err)
		return ExitRunnerError
	}
	stdout.Write(data)
	return ExitOK
}

func setRun(ctx context.Context, st store.Store, args []string, stdout, stderr io.Writer) int {
	fs := pflag.NewFlagSet("set run", pflag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "emit final SetRun as JSON to stdout")
	htmlPath := fs.String("html", "", "write HTML report to this path")
	thresholds := fs.StringArray("threshold", nil, "extra threshold (metric:op:value[:scope]); repeatable")
	overrides := fs.StringArray("override", nil, "override (item.field=value); repeatable")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "usage: crankfire set run [flags] <id>")
		return ExitUsage
	}
	setID := fs.Arg(0)

	set, err := st.GetSet(ctx, setID)
	if err != nil {
		fmt.Fprintf(stderr, "get set: %v\n", err)
		return ExitUsage
	}
	if extras, err := parseThresholdFlags(*thresholds); err != nil {
		fmt.Fprintf(stderr, "parse --threshold: %v\n", err)
		return ExitUsage
	} else {
		set.Thresholds = append(set.Thresholds, extras...)
	}
	if err := applyOverrideFlags(&set, *overrides); err != nil {
		fmt.Fprintf(stderr, "parse --override: %v\n", err)
		return ExitUsage
	}
	// Persist the in-memory mutation only for this run; write to a temp ID-less buffer is overkill.
	// We pass the mutated `set` to the runner via a one-shot saved copy under a transient ID? No —
	// runner reads from store. Save with the same ID (idempotent) so the run honors the overrides.
	if err := st.SaveSet(ctx, set); err != nil {
		fmt.Fprintf(stderr, "save set with overrides: %v\n", err)
		return ExitUsage
	}

	r := setrunner.New(st, &cliBuilderAdapter{})
	run, err := r.Run(ctx, setID, nil)
	if err != nil {
		fmt.Fprintf(stderr, "run set: %v\n", err)
		return ExitRunnerError
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(run)
	} else {
		fmt.Fprintf(stdout, "Set %s — %s in %s\n", run.SetName, run.Status, run.EndedAt.Sub(run.StartedAt).Round(time.Millisecond))
		for _, st := range run.Stages {
			fmt.Fprintf(stdout, "  Stage %s\n", st.Name)
			for _, it := range st.Items {
				errRate := 0.0
				if it.Summary.TotalRequests > 0 {
					errRate = float64(it.Summary.Errors) / float64(it.Summary.TotalRequests)
				}
				rps := 0.0
				if it.Summary.DurationSec > 0 {
					rps = float64(it.Summary.TotalRequests) / it.Summary.DurationSec
				}
				fmt.Fprintf(stdout, "    %-20s %-10s p95=%.0fms err=%.2f%% rps=%.1f\n",
					it.Name, it.Status, it.Summary.P95Ms, errRate*100, rps)
			}
		}
		for _, th := range run.Thresholds {
			mark := "✓"
			if !th.Passed {
				mark = "✗"
			}
			fmt.Fprintf(stdout, "  %s %s %s %v (actual %.3f) [%s]\n", mark, th.Metric, th.Op, th.Value, th.Actual, th.Scope)
		}
	}
	if *htmlPath != "" {
		data, err := setreport.Render(run)
		if err != nil {
			fmt.Fprintf(stderr, "render html: %v\n", err)
			return ExitRunnerError
		}
		if err := os.MkdirAll(filepath.Dir(*htmlPath), 0o755); err != nil {
			fmt.Fprintf(stderr, "mkdir html: %v\n", err)
			return ExitRunnerError
		}
		if err := os.WriteFile(*htmlPath, data, 0o644); err != nil {
			fmt.Fprintf(stderr, "write html: %v\n", err)
			return ExitRunnerError
		}
	}

	if !run.AllThresholdsPassed {
		return ExitThresholdFailed
	}
	if run.Status != store.SetRunCompleted {
		return ExitRunnerError
	}
	return ExitOK
}

type cliBuilderAdapter struct{}

func (cliBuilderAdapter) Build(ctx context.Context, cfg config.Config, _ string) (setrunner.ItemRun, error) {
	rnr, collector, cleanup, err := BuildRunner(ctx, cfg)
	if err != nil {
		return setrunner.ItemRun{}, err
	}
	return setrunner.ItemRun{
		Run: func(ctx context.Context) (store.RunSummary, error) {
			result := rnr.Run(ctx)
			stats := collector.Stats(result.Duration)
			return store.RunSummary{
				TotalRequests: stats.Total,
				Errors:        stats.Failures,
				DurationSec:   result.Duration.Seconds(),
				P50Ms:         stats.P50LatencyMs,
				P95Ms:         stats.P95LatencyMs,
				P99Ms:         stats.P99LatencyMs,
			}, nil
		},
		Snapshot: func() setrunner.MetricSnapshot {
			stats := collector.Stats(0)
			er := 0.0
			if stats.Total > 0 {
				er = float64(stats.Failures) / float64(stats.Total)
			}
			rps := 0.0
			if stats.Duration.Seconds() > 0 {
				rps = float64(stats.Total) / stats.Duration.Seconds()
			}
			return setrunner.MetricSnapshot{
				P50:         stats.P50LatencyMs,
				P95:         stats.P95LatencyMs,
				P99:         stats.P99LatencyMs,
				ErrorRate:   er,
				RPS:         rps,
				TotalErrors: float64(stats.Failures),
			}
		},
		Cleanup: cleanup,
	}, nil
}

// NewSetBuilder returns the production Builder used by the CLI set runner.
// Other packages (e.g. the TUI) can use this to avoid duplicating wiring.
func NewSetBuilder() setrunner.Builder { return &cliBuilderAdapter{} }

func parseThresholdFlags(flags []string) ([]store.Threshold, error) {
	var out []store.Threshold
	for _, raw := range flags {
		parts := strings.Split(raw, ":")
		if len(parts) < 3 || len(parts) > 4 {
			return nil, fmt.Errorf("expected metric:op:value[:scope], got %q", raw)
		}
		v, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return nil, fmt.Errorf("value %q: %w", parts[2], err)
		}
		t := store.Threshold{Metric: parts[0], Op: parts[1], Value: v, Scope: "aggregate"}
		if len(parts) == 4 {
			t.Scope = parts[3]
		}
		out = append(out, t)
	}
	return out, nil
}

// applyOverrideFlags accepts entries of the form "item.field=value".
// Supported fields: target_url, total_requests, rate, concurrency, duration, timeout, auth_token.
func applyOverrideFlags(set *store.Set, flags []string) error {
	for _, raw := range flags {
		eq := strings.Index(raw, "=")
		if eq < 0 {
			return fmt.Errorf("expected item.field=value, got %q", raw)
		}
		left, value := raw[:eq], raw[eq+1:]
		dot := strings.Index(left, ".")
		if dot < 0 {
			return fmt.Errorf("missing item name in %q", raw)
		}
		itemName, field := left[:dot], left[dot+1:]
		item := findItem(set, itemName)
		if item == nil {
			return fmt.Errorf("unknown item %q", itemName)
		}
		if err := setOverrideField(&item.Overrides, field, value); err != nil {
			return fmt.Errorf("%s.%s: %w", itemName, field, err)
		}
	}
	return nil
}

func findItem(set *store.Set, name string) *store.SetItem {
	for si := range set.Stages {
		for ii := range set.Stages[si].Items {
			if set.Stages[si].Items[ii].Name == name {
				return &set.Stages[si].Items[ii]
			}
		}
	}
	return nil
}

func setOverrideField(o *store.Override, field, value string) error {
	switch field {
	case "target_url":
		o.TargetURL = &value
	case "auth_token":
		o.AuthToken = &value
	case "total_requests":
		n, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		o.TotalRequests = &n
	case "rate":
		n, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		o.Rate = &n
	case "concurrency":
		n, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		o.Concurrency = &n
	case "duration":
		d, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		o.Duration = &d
	case "timeout":
		d, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		o.Timeout = &d
	default:
		return fmt.Errorf("unsupported field %q", field)
	}
	return nil
}
