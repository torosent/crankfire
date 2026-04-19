package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
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
	tplrender "github.com/torosent/crankfire/internal/template"
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
		fmt.Fprintln(stderr, "usage: crankfire set <list|show|run|new|diff> [args]")
		return ExitUsage
	}
	switch args[0] {
	case "list":
		return setList(ctx, st, args[1:], stdout, stderr)
	case "show":
		return setShow(ctx, st, args[1:], stdout, stderr)
	case "run":
		return setRun(ctx, st, args[1:], stdout, stderr)
	case "new":
		return setNew(ctx, st, args[1:], stdout, stderr)
	case "diff":
		return setDiff(ctx, st, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand: %s\n", args[0])
		return ExitUsage
	}
}

func setList(ctx context.Context, st store.Store, args []string, stdout, stderr io.Writer) int {
	fs := pflag.NewFlagSet("set list", pflag.ContinueOnError)
	fs.SetOutput(stderr)
	var tagFlags []string
	fs.StringArrayVar(&tagFlags, "tag", nil, "filter by tag (repeat for AND, comma for OR; transitive via items→sessions)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	matchers, err := buildMatchers(tagFlags)
	if err != nil {
		fmt.Fprintf(stderr, "tag: %v\n", err)
		return ExitUsage
	}
	sets, err := st.ListSets(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "list sets: %v\n", err)
		return ExitRunnerError
	}
	if len(sets) == 0 {
		fmt.Fprintln(stdout, "No sets defined. Create one in the TUI (crankfire tui) or by writing a YAML file.")
		return ExitOK
	}
	// Cache session lookups for transitive filter.
	cache := map[string][]string{}
	getTags := func(sessionID string) []string {
		if tags, ok := cache[sessionID]; ok {
			return tags
		}
		sess, err := st.GetSession(ctx, sessionID)
		var tags []string
		if err == nil {
			tags = sess.Tags
		}
		cache[sessionID] = tags
		return tags
	}
	setMatches := func(set store.Set) bool {
		if len(matchers) == 0 {
			return true
		}
		// Set matches if ANY referenced session satisfies ALL matchers.
		for _, stage := range set.Stages {
			for _, item := range stage.Items {
				tags := getTags(item.SessionID)
				if matchAll(matchers, tags) {
					return true
				}
			}
		}
		return false
	}
	fmt.Fprintf(stdout, "%-26s  %-30s  %s\n", "ID", "NAME", "STAGES")
	for _, s := range sets {
		if !setMatches(s) {
			continue
		}
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

// setNew materializes a template into a real Set.
func setNew(ctx context.Context, st store.Store, args []string, stdout, stderr io.Writer) int {
	fs := pflag.NewFlagSet("set new", pflag.ContinueOnError)
	fs.SetOutput(stderr)
	var fromTemplate, name string
	var paramFlags []string
	fs.StringVar(&fromTemplate, "from-template", "", "template ID to instantiate (required)")
	fs.StringVar(&name, "name", "", "override the rendered set name")
	fs.StringArrayVar(&paramFlags, "param", nil, "template param key=value (repeatable)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fromTemplate == "" {
		fmt.Fprintln(stderr, "usage: crankfire set new --from-template <id> [--param k=v]... [--name N]")
		return ExitUsage
	}
	params, err := parseParamFlags(paramFlags)
	if err != nil {
		fmt.Fprintf(stderr, "param: %v\n", err)
		return ExitUsage
	}
	body, err := st.GetTemplate(ctx, fromTemplate)
	if err != nil {
		fmt.Fprintf(stderr, "get template %s: %v\n", fromTemplate, err)
		return ExitUsage
	}
	rendered, err := tplrender.Render(body, params)
	if err != nil {
		fmt.Fprintf(stderr, "render template: %v\n", err)
		return ExitUsage
	}
	rendered = stripTemplateMarker(rendered)
	var set store.Set
	if err := yaml.Unmarshal(rendered, &set); err != nil {
		fmt.Fprintf(stderr, "unmarshal rendered template: %v\n", err)
		return ExitUsage
	}
	set.ID = ""
	set.SchemaVersion = 0
	set.CreatedAt = time.Time{}
	set.UpdatedAt = time.Time{}
	if name != "" {
		set.Name = name
	}
	if err := st.SaveSet(ctx, set); err != nil {
		fmt.Fprintf(stderr, "save set: %v\n", err)
		return ExitRunnerError
	}
	// Re-list to find the newly-created set ID (SaveSet assigns the ULID).
	sets, _ := st.ListSets(ctx)
	if len(sets) > 0 {
		newest := sets[0]
		for _, s := range sets[1:] {
			if s.CreatedAt.After(newest.CreatedAt) {
				newest = s
			}
		}
		fmt.Fprintln(stdout, newest.ID)
	}
	return ExitOK
}

// parseParamFlags converts ["key=value", ...] into a map.
func parseParamFlags(flags []string) (map[string]string, error) {
	out := make(map[string]string, len(flags))
	for _, f := range flags {
		idx := strings.Index(f, "=")
		if idx <= 0 {
			return nil, fmt.Errorf("param must be key=value, got %q", f)
		}
		out[f[:idx]] = f[idx+1:]
	}
	return out, nil
}

// stripTemplateMarker removes a top-level `template: true` line from
// rendered template bytes so the result unmarshals cleanly into Set.
func stripTemplateMarker(in []byte) []byte {
	lines := strings.Split(string(in), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "template: true" || trimmed == "template: \"true\"" {
			continue
		}
		out = append(out, line)
	}
	return []byte(strings.Join(out, "\n"))
}

// setDiff implements `crankfire set diff <run-id-a> <run-id-b>`.
// Outputs a text table by default; --json for JSON, --html PATH for standalone HTML.
func setDiff(ctx context.Context, _ store.Store, args []string, stdout, stderr io.Writer) int {
	fs := pflag.NewFlagSet("set diff", pflag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "emit JSON instead of text table")
	htmlPath := fs.String("html", "", "write standalone HTML to PATH")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(stderr, "usage: crankfire set diff <run-id-a> <run-id-b> [--json | --html PATH]")
		return ExitUsage
	}
	idA, idB := fs.Arg(0), fs.Arg(1)
	dataDir, err := store.ResolveDataDir("")
	if err != nil {
		fmt.Fprintf(stderr, "data dir: %v\n", err)
		return ExitRunnerError
	}
	runA, setA, err := resolveRunID(dataDir, idA)
	if err != nil {
		fmt.Fprintf(stderr, "resolve %s: %v\n", idA, err)
		return ExitUsage
	}
	runB, setB, err := resolveRunID(dataDir, idB)
	if err != nil {
		fmt.Fprintf(stderr, "resolve %s: %v\n", idB, err)
		return ExitUsage
	}
	if setA != setB {
		fmt.Fprintf(stderr, "runs belong to different sets (%s vs %s)\n", setA, setB)
		return ExitUsage
	}
	res := setrunner.Diff(runA, runB)
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			fmt.Fprintf(stderr, "encode: %v\n", err)
			return ExitRunnerError
		}
		return ExitOK
	}
	if *htmlPath != "" {
		if err := writeDiffHTML(*htmlPath, res); err != nil {
			fmt.Fprintf(stderr, "write html: %v\n", err)
			return ExitRunnerError
		}
		fmt.Fprintf(stdout, "wrote %s\n", *htmlPath)
		return ExitOK
	}
	writeDiffText(stdout, res)
	return ExitOK
}

func resolveRunID(dataDir, runID string) (store.SetRun, string, error) {
	root := filepath.Join(dataDir, "runs", "sets")
	matches, err := filepath.Glob(filepath.Join(root, "*", runID, "set-run.json"))
	if err != nil {
		return store.SetRun{}, "", err
	}
	if len(matches) == 0 {
		return store.SetRun{}, "", fmt.Errorf("no run with id %q under %s", runID, root)
	}
	if len(matches) > 1 {
		return store.SetRun{}, "", fmt.Errorf("ambiguous run id %q: matches %d sets — disambiguate by set ID", runID, len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return store.SetRun{}, "", fmt.Errorf("read %s: %w", matches[0], err)
	}
	var run store.SetRun
	if err := json.Unmarshal(data, &run); err != nil {
		return store.SetRun{}, "", fmt.Errorf("unmarshal: %w", err)
	}
	setID := filepath.Base(filepath.Dir(filepath.Dir(matches[0])))
	run.Dir = filepath.Dir(matches[0])
	return run, setID, nil
}

func writeDiffText(w io.Writer, r setrunner.DiffResult) {
	fmt.Fprintf(w, "verdict: %s\n", r.OverallVerdict)
	fmt.Fprintf(w, "%-20s  %-10s  %12s  %12s  %12s  %12s  %12s\n",
		"item", "stage", "dP50ms", "dP95ms", "dP99ms", "dErrRate", "dRPS")
	for _, row := range r.Rows {
		marker := ""
		if !row.APresent {
			marker = " (B-only)"
		} else if !row.BPresent {
			marker = " (A-only)"
		}
		fmt.Fprintf(w, "%-20s  %-10s  %12.2f  %12.2f  %12.2f  %12.4f  %12.2f%s\n",
			row.ItemName, row.Stage,
			row.P50DeltaMs, row.P95DeltaMs, row.P99DeltaMs,
			row.ErrRateDelta, row.RPSDelta, marker)
	}
}

func writeDiffHTML(path string, r setrunner.DiffResult) error {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "<!DOCTYPE html>\n<html><head><meta charset=\"utf-8\"><title>Crankfire diff</title>\n")
	buf.WriteString(`<style>
body{font:14px -apple-system,BlinkMacSystemFont,sans-serif;padding:24px;color:#222}
.verdict{display:inline-block;padding:4px 12px;border-radius:4px;font-weight:600}
.verdict.improved{background:#d4edda;color:#155724}
.verdict.regressed{background:#f8d7da;color:#721c24}
.verdict.mixed{background:#fff3cd;color:#856404}
.verdict.unchanged{background:#e2e3e5;color:#383d41}
table{border-collapse:collapse;margin-top:16px;width:100%}
th,td{padding:6px 10px;border-bottom:1px solid #eee;text-align:right}
th:first-child,td:first-child,th:nth-child(2),td:nth-child(2){text-align:left}
.bad{color:#c42;font-weight:600}
.good{color:#262;font-weight:600}
</style></head><body>`)
	fmt.Fprintf(&buf, "<h1>Crankfire diff &mdash; <span class=\"verdict %s\">%s</span></h1>\n",
		html.EscapeString(r.OverallVerdict), html.EscapeString(r.OverallVerdict))
	buf.WriteString("<table><thead><tr><th>item</th><th>stage</th><th>dP50ms</th><th>dP95ms</th><th>dP99ms</th><th>dErrRate</th><th>dRPS</th></tr></thead><tbody>\n")
	for _, row := range r.Rows {
		buf.WriteString("<tr>")
		fmt.Fprintf(&buf, "<td>%s</td><td>%s</td>", html.EscapeString(row.ItemName), html.EscapeString(row.Stage))
		for _, v := range []float64{row.P50DeltaMs, row.P95DeltaMs, row.P99DeltaMs} {
			cls := ""
			if v > 0 {
				cls = "bad"
			} else if v < 0 {
				cls = "good"
			}
			fmt.Fprintf(&buf, "<td class=\"%s\">%+.2f</td>", cls, v)
		}
		cls := ""
		if row.ErrRateDelta > 0 {
			cls = "bad"
		} else if row.ErrRateDelta < 0 {
			cls = "good"
		}
		fmt.Fprintf(&buf, "<td class=\"%s\">%+.4f</td>", cls, row.ErrRateDelta)
		cls = ""
		if row.RPSDelta > 0 {
			cls = "good"
		} else if row.RPSDelta < 0 {
			cls = "bad"
		}
		fmt.Fprintf(&buf, "<td class=\"%s\">%+.2f</td>", cls, row.RPSDelta)
		buf.WriteString("</tr>\n")
	}
	buf.WriteString("</tbody></table></body></html>\n")
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
