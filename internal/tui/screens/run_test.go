package screens_test

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/store"
	"github.com/torosent/crankfire/internal/tui/screens"
)

func TestRunScreenStartsAndCancels(t *testing.T) {
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	sess := store.Session{Name: "x", Config: config.Config{
		TargetURL: "http://127.0.0.1:0", Protocol: config.ProtocolHTTP, Total: 1,
		Concurrency: 1, Timeout: 100 * time.Millisecond,
	}}
	if err := s.SaveSession(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	m := screens.NewRun(s, list[0])
	var cur tea.Model = m
	cur.Init()
	// Send Esc to cancel; expect a finalize message back.
	cur, _ = cur.Update(tea.KeyMsg{Type: tea.KeyEsc})

	// Drive the model until it finalizes. We dispatch any tea.Cmd messages
	// the model emits so its background goroutines and tickers can deliver
	// their results.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		view := cur.View()
		if strings.Contains(view, "cancelled") || strings.Contains(view, "completed") || strings.Contains(view, "failed") {
			return
		}
		time.Sleep(20 * time.Millisecond)
		// Pump a no-op update so any pending tea.Cmds get drained via
		// follow-up messages produced by the goroutine.
		cur, _ = cur.Update(tickPing{})
	}
	t.Errorf("run screen did not finalize within deadline; view=%q", cur.View())
}

type tickPing struct{}

func TestRunScreenForwardsWindowSizeToDashboard(t *testing.T) {
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	sess := store.Session{Name: "x", Config: config.Config{Total: 1}}
	if err := s.SaveSession(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	m := screens.NewRun(s, list[0])
	cur, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})

	out := cur.View()
	if got := len(strings.Split(out, "\n")); got < 35 || got > 40 {
		t.Errorf("expected run screen to use available height without overflowing, got %d lines:\n%s", got, out)
	}
	if !strings.Contains(out, "[q/esc] cancel") {
		t.Errorf("expected footer to remain visible, got:\n%s", out)
	}
}

func TestRunScreenWiresRequestContext(t *testing.T) {
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	sess := store.Session{Name: "x", Config: config.Config{
		TargetURL:   "https://example.com/run?payloadsizekb=100",
		Method:      "POST",
		Protocol:    config.ProtocolHTTP,
		Concurrency: 5,
		Rate:        5,
		Duration:    10 * time.Minute,
		Timeout:     30 * time.Second,
	}}
	if err := s.SaveSession(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	m := screens.NewRun(s, list[0])
	ctx := m.ViewModelForTest().RequestContextForTest()

	if ctx.RawURL != "https://example.com/run?payloadsizekb=100" {
		t.Errorf("RawURL = %q, want %q", ctx.RawURL, "https://example.com/run?payloadsizekb=100")
	}
	if ctx.Method != "POST" {
		t.Errorf("Method = %q, want %q", ctx.Method, "POST")
	}
	if ctx.Protocol != "http" {
		t.Errorf("Protocol = %q, want %q", ctx.Protocol, "http")
	}
	if len(ctx.Params) < 2 {
		t.Fatalf("len(Params) = %d, want at least 2", len(ctx.Params))
	}
	// Verify Workers and Rate params are present
	foundWorkers := false
	foundRate := false
	for _, p := range ctx.Params {
		if p.Label == "Workers" && p.Value == "5" {
			foundWorkers = true
		}
		if p.Label == "Rate" && p.Value == "5/s" {
			foundRate = true
		}
	}
	if !foundWorkers {
		t.Errorf("missing Workers=5 in Params: %+v", ctx.Params)
	}
	if !foundRate {
		t.Errorf("missing Rate=5/s in Params: %+v", ctx.Params)
	}
	if len(ctx.QueryParams) != 1 {
		t.Fatalf("len(QueryParams) = %d, want 1", len(ctx.QueryParams))
	}
	if ctx.QueryParams[0].Label != "payloadsizekb" || ctx.QueryParams[0].Value != "100" {
		t.Errorf("QueryParams[0] = %+v, want {payloadsizekb, 100}", ctx.QueryParams[0])
	}
}

func TestRunScreenWiresRequestContextFromEndpointTarget(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want string
	}{
		{
			name: "endpoint url",
			cfg: config.Config{
				Method:   "POST",
				Protocol: config.ProtocolHTTP,
				Endpoints: []config.Endpoint{
					{URL: "https://example.com/run?payloadsizekb=100"},
				},
			},
			want: "https://example.com/run?payloadsizekb=100",
		},
		{
			name: "endpoint path",
			cfg: config.Config{
				Method:   "POST",
				Protocol: config.ProtocolHTTP,
				Endpoints: []config.Endpoint{
					{Path: "/orchestrations/start"},
				},
			},
			want: "/orchestrations/start",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := store.NewFS(t.TempDir())
			if err != nil {
				t.Fatalf("NewFS: %v", err)
			}
			sess := store.Session{Name: "x", Config: tt.cfg}
			if err := s.SaveSession(context.Background(), sess); err != nil {
				t.Fatal(err)
			}
			list, err := s.ListSessions(context.Background())
			if err != nil {
				t.Fatalf("ListSessions: %v", err)
			}

			m := screens.NewRun(s, list[0])
			ctx := m.ViewModelForTest().RequestContextForTest()

			if ctx.RawURL != tt.want {
				t.Errorf("RawURL = %q, want %q", ctx.RawURL, tt.want)
			}
		})
	}
}

func TestRunScreenPreservesResponsiveDashboardOnShortTerminal(t *testing.T) {
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	sess := store.Session{Name: "x", Config: config.Config{
		TargetURL:   "https://example.com/api?param1=val1&param2=val2",
		Concurrency: 5,
		Rate:        10,
	}}
	if err := s.SaveSession(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	m := screens.NewRun(s, list[0])
	cur, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})

	out := cur.View()
	lines := strings.Count(out, "\n") + 1

	// Screen includes dashboard + footer, so it should fit within 24 lines
	if lines > 24 {
		t.Errorf("short terminal output %d lines exceeds budget of 24:\n%s", lines, out)
	}

	// Verify high-priority Request Context panel remains visible
	if !strings.Contains(out, "Request Context") {
		t.Errorf("Request Context panel missing in short terminal:\n%s", out)
	}

	// Verify structured query-param detail lines survive clipping.
	// Each query param appears twice: once in RawURL, once in dedicated detail line.
	// Check BOTH query params to ensure complete Request Context detail is preserved.
	for _, param := range []string{"param1=val1", "param2=val2"} {
		count := strings.Count(out, param)
		if count < 2 {
			t.Errorf("query param %q appears %d times (want >= 2 for structured detail line):\n%s", param, count, out)
		}
	}
}

func TestRunScreenNarrowStackedLayoutIsHeightSafe(t *testing.T) {
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	sess := store.Session{Name: "x", Config: config.Config{
		TargetURL:   "https://example.com/api?payloadsizekb=100&orchestrationcount=1",
		Concurrency: 5,
		Rate:        10,
	}}
	if err := s.SaveSession(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	m := screens.NewRun(s, list[0])
	cur, _ := m.Update(tea.WindowSizeMsg{Width: 72, Height: 32})

	out := cur.View()
	lines := strings.Count(out, "\n") + 1

	// Screen wrapper reserves 2 lines for footer (Height-2 to runview)
	if lines > 32 {
		t.Errorf("narrow stacked output %d lines exceeds budget of 32:\n%s", lines, out)
	}

	// Verify Request Context panel remains visible
	if !strings.Contains(out, "Request Context") {
		t.Errorf("Request Context panel missing in narrow stacked layout:\n%s", out)
	}

	// Verify structured query-param detail survives stacked-layout overhead
	for _, param := range []string{"payloadsizekb=100", "orchestrationcount=1"} {
		count := strings.Count(out, param)
		if count < 2 {
			t.Errorf("query param %q appears %d times (want >= 2 for structured detail) in narrow stacked layout:\n%s", param, count, out)
		}
	}
}

func TestRunScreenNarrowShortStackedLayoutIsHeightSafe(t *testing.T) {
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	sess := store.Session{Name: "x", Config: config.Config{
		TargetURL:   "https://example.com/api?payloadsizekb=100&orchestrationcount=1",
		Concurrency: 5,
		Rate:        10,
	}}
	if err := s.SaveSession(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	m := screens.NewRun(s, list[0])
	cur, _ := m.Update(tea.WindowSizeMsg{Width: 72, Height: 24})

	out := cur.View()
	lines := strings.Count(out, "\n") + 1

	// Screen wrapper reserves 2 lines for footer (Height-2 to runview)
	if lines > 24 {
		t.Errorf("narrow short stacked output %d lines exceeds budget of 24:\n%s", lines, out)
	}

	// Verify Request Context panel remains visible
	if !strings.Contains(out, "Request Context") {
		t.Errorf("Request Context panel missing in narrow short stacked layout:\n%s", out)
	}

	// Verify structured query-param detail survives even on narrow short terminal
	for _, param := range []string{"payloadsizekb=100", "orchestrationcount=1"} {
		count := strings.Count(out, param)
		if count < 2 {
			t.Errorf("query param %q appears %d times (want >= 2 for structured detail) in narrow short stacked layout:\n%s", param, count, out)
		}
	}
}

func TestRunScreenMinimalStackedLayoutIsHeightSafe(t *testing.T) {
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	sess := store.Session{Name: "x", Config: config.Config{
		TargetURL:   "https://api.example.com/v2/test?apiKey=abc123&userId=user456",
		Concurrency: 10,
		Rate:        20,
	}}
	if err := s.SaveSession(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	m := screens.NewRun(s, list[0])
	const width, height = 72, 22
	cur, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})

	out := cur.View()
	lines := strings.Count(out, "\n") + 1
	if lines > height {
		t.Errorf("minimal stacked output %d lines exceeds budget of %d:\n%s", lines, height, out)
	}

	for _, param := range []string{"apiKey=abc123", "userId=user456"} {
		count := strings.Count(out, param)
		if count < 2 {
			t.Errorf("query param %q appears %d times (want >= 2 for structured detail) in minimal stacked layout:\n%s", param, count, out)
		}
	}
}

// TestStackedScreenBoundaryHeights verifies responsive layout across boundary heights in narrow stacked mode via screen wrapper.
// Screen passes Height-2 to runview and adds 2-line footer, so test height = runview height + 2.
func TestStackedScreenBoundaryHeights(t *testing.T) {
	tests := []struct {
		name         string
		width        int
		screenHeight int
		desc         string
	}{
		{"72x22", 72, 22, "minimal screen height with panel skipping"},
		{"72x23", 72, 23, "boundary above minimal screen"},
		{"72x24", 72, 24, "short screen height"},
		{"72x25", 72, 25, "short-plus-one screen height"},
		{"72x31", 72, 31, "medium screen height"},
		{"72x33", 72, 33, "boundary between medium and tall screen"},
		{"72x35", 72, 35, "tall screen height"},
		{"72x37", 72, 37, "extra tall screen height"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := store.NewFS(t.TempDir())
			if err != nil {
				t.Fatalf("NewFS: %v", err)
			}
			sess := store.Session{Name: "x", Config: config.Config{
				TargetURL:   "https://api.example.com/v1/users?apiKey=abc123&format=json",
				Concurrency: 10,
				Rate:        100,
			}}
			if err := s.SaveSession(context.Background(), sess); err != nil {
				t.Fatal(err)
			}
			list, err := s.ListSessions(context.Background())
			if err != nil {
				t.Fatalf("ListSessions: %v", err)
			}

			m := screens.NewRun(s, list[0])
			cur, _ := m.Update(tea.WindowSizeMsg{Width: tt.width, Height: tt.screenHeight})

			// Render and validate
			out := cur.View()
			lines := strings.Count(out, "\n") + 1

			if lines > tt.screenHeight {
				t.Errorf("%s: screen output has %d lines, exceeds height %d:\n%s", tt.desc, lines, tt.screenHeight, out)
			}

			// Verify stacked layout (panels appear vertically in screen output)
			if !strings.Contains(out, "Run Summary") || !strings.Contains(out, "Requests / sec") {
				t.Errorf("%s: missing stacked panels in screen (Run Summary or Requests / sec not found)", tt.desc)
			}

			// Verify query params survive (occurrence count >= 2: once in URL, once as detail line)
			for _, param := range []string{"apiKey=abc123", "format=json"} {
				count := strings.Count(out, param)
				if count < 2 {
					t.Errorf("%s: query param %q appears %d times in screen (want >= 2):\n%s", tt.desc, param, count, out)
				}
			}
		})
	}
}

// TestStackedScreenFullRequestContextPayload verifies that all Request Context fields
// survive rendering through the screen wrapper at common constrained sizes (72x24).
// A realistic payload includes: long URL with multiple query params, Method+Protocol,
// and 6 config params. This ensures screen-level integration preserves all fields.
func TestStackedScreenFullRequestContextPayload(t *testing.T) {
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	sess := store.Session{
		Name: "x",
		Config: config.Config{
			TargetURL:   "https://api.example.com/v2/users/search?apiKey=abc123def456&format=json&limit=100&offset=0",
			Method:      "POST",
			Protocol:    config.ProtocolHTTP,
			Concurrency: 10,
			Rate:        100,
			Duration:    5 * time.Minute,
			Timeout:     30 * time.Second,
			Retries:     3,
			ConfigFile:  "prod.yml",
		},
	}
	if err := s.SaveSession(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	m := screens.NewRun(s, list[0])
	cur, _ := m.Update(tea.WindowSizeMsg{Width: 72, Height: 24})
	out := cur.View()

	// Verify output lines <= screen height budget
	lines := strings.Count(out, "\n") + 1
	if lines > 24 {
		t.Errorf("screen output has %d lines, exceeds height 24:\n%s", lines, out)
	}

	// Extract Request Context section
	rcSection := extractSection(out, "Request Context")
	if rcSection == "" {
		t.Fatalf("Request Context panel not found in screen output:\n%s", out)
	}

	// All config-derived fields must be present (check values, as labels may be wrapped)
	requiredValues := []string{
		"POST",     // from Method
		"http",     // from Protocol
		"10",       // from Concurrency (Workers)
		"100/s",    // from Rate
		"5m",       // from Duration
		"30s",      // from Timeout
		"3",        // from Retries
		"prod.yml", // from ConfigFile
	}
	for _, val := range requiredValues {
		if !strings.Contains(rcSection, val) {
			t.Errorf("Request Context missing value %q in screen:\n%s", val, rcSection)
		}
	}
	// Also verify the labels are present (they should be, even if wrapped)
	requiredLabels := []string{"Workers:", "Rate:", "Duration:", "Timeout:", "Retries:", "Config:"}
	for _, label := range requiredLabels {
		if !strings.Contains(rcSection, label) {
			t.Errorf("Request Context missing label %q in screen:\n%s", label, rcSection)
		}
	}

	// All query params must be present as detail lines
	requiredQueryParams := []string{
		"apiKey=abc123def456",
		"format=json",
		"limit=100",
		"offset=0",
	}
	for _, qp := range requiredQueryParams {
		if !strings.Contains(rcSection, qp) {
			t.Errorf("Request Context missing query param %q in screen:\n%s", qp, rcSection)
		}
	}

	// Slice top row (everything before Request Context) to verify compact strip behavior
	requestContextStart := strings.Index(out, "┌ Request Context")
	if requestContextStart == -1 {
		t.Fatalf("Request Context panel not found for top-row slicing:\n%s", out)
	}
	topRow := out[:requestContextStart]

	// OLD BEHAVIOR: Reject boxed RPS vs Target panel in top row
	if strings.Contains(topRow, "┌ RPS vs Target") {
		t.Error("top row still contains old boxed '┌ RPS vs Target' panel; should use compact inline strip")
	}

	// NEW BEHAVIOR: Verify compact inline RPS strip in top row
	if !strings.Contains(topRow, "Run Summary") {
		t.Error("Run Summary should be visible in top row at screen 72x24")
	}
	if !strings.Contains(topRow, "Requests / sec") {
		t.Error("RPS strip 'Requests / sec' should be present in top row at screen 72x24")
	}
	if !strings.Contains(topRow, "█") && !strings.Contains(topRow, "░") {
		t.Error("RPS strip bar (█ or ░) should be present in top row at screen 72x24")
	}

	// Verify Run Summary has meaningful content (not shell-only)
	summarySection := extractSection(out, "Run Summary")
	if summarySection == "" {
		t.Error("Run Summary panel should be present at screen 72x24")
	} else if panelContentLines(summarySection) == 0 {
		t.Errorf("Run Summary should have content lines, found shell-only in screen:\n%s", summarySection)
	}

	latencyTrendSection := extractSection(out, "Latency Trend")
	latencyStatsSection := extractSection(out, "Latency Stats / Health")
	if panelContentLines(latencyTrendSection) == 0 && panelContentLines(latencyStatsSection) == 0 {
		t.Errorf("screen 72x24 should preserve meaningful latency content:\n%s", out)
	}

	// Verify Protocol and Endpoints are absent (lower priority)
	if strings.Contains(out, "Protocol Metrics") {
		t.Error("Protocol Metrics panel should be absent at 72x24 with full Request Context in screen")
	}
	if strings.Contains(out, "Endpoints") {
		t.Error("Endpoints panel should be absent at 72x24 with full Request Context in screen")
	}
}

// TestStackedScreenPriorityDegradationOrder verifies that lower-priority panels disappear
// before higher-priority ones lose content in the screen wrapper context.
func TestStackedScreenPriorityDegradationOrder(t *testing.T) {
	s, err := store.NewFS(t.TempDir())
	if err != nil {
		t.Fatalf("NewFS: %v", err)
	}
	sess := store.Session{Name: "x", Config: config.Config{
		TargetURL:   "https://example.com/api/resource?id=123&detailed=true",
		Concurrency: 5,
		Rate:        10,
	}}
	if err := s.SaveSession(context.Background(), sess); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	m := screens.NewRun(s, list[0])
	cur, _ := m.Update(tea.WindowSizeMsg{Width: 72, Height: 23})

	out := cur.View()

	lines := strings.Split(out, "\n")
	actualLines := len(lines)
	if actualLines > 23 {
		t.Errorf("screen output %d lines exceeds height budget 23", actualLines)
	}

	// Query params should survive
	for _, param := range []string{"id=123", "detailed=true"} {
		count := strings.Count(out, param)
		if count < 2 {
			t.Errorf("query param %q appears %d times (want >= 2)", param, count)
		}
	}

	// Slice top row (everything before Request Context) to verify compact strip behavior
	requestContextStart := strings.Index(out, "┌ Request Context")
	if requestContextStart == -1 {
		t.Fatalf("Request Context panel not found for top-row slicing:\n%s", out)
	}
	topRow := out[:requestContextStart]

	// OLD BEHAVIOR: Reject boxed RPS vs Target panel in top row
	if strings.Contains(topRow, "┌ RPS vs Target") {
		t.Error("top row still contains old boxed '┌ RPS vs Target' panel; should use compact inline strip")
	}

	// NEW BEHAVIOR: Verify compact inline RPS strip in top row
	if !strings.Contains(topRow, "Run Summary") {
		t.Error("Run Summary should be visible in top row at 72x23 screen")
	}
	if !strings.Contains(topRow, "Requests / sec") {
		t.Error("RPS strip 'Requests / sec' should be present in top row at 72x23 screen")
	}
	if !strings.Contains(topRow, "█") && !strings.Contains(topRow, "░") {
		t.Error("RPS strip bar (█ or ░) should be present in top row at 72x23 screen")
	}

	// Protocol panel should be absent (lower priority)
	if strings.Contains(out, "Protocol") {
		t.Error("Protocol panel should be absent at 72x23 screen (lower priority than Latency)")
	}

	summarySection := extractSection(out, "Run Summary")
	if summarySection == "" {
		t.Error("Run Summary panel should be present at 72x23 screen")
	} else if panelContentLines(summarySection) == 0 {
		t.Errorf("Run Summary should have content lines in screen degradation test:\n%s", summarySection)
	}

	latencyTrendSection := extractSection(out, "Latency Trend")
	latencyStatsSection := extractSection(out, "Latency Stats / Health")
	if panelContentLines(latencyTrendSection) == 0 && panelContentLines(latencyStatsSection) == 0 {
		t.Errorf("screen degradation test should preserve meaningful latency content:\n%s", out)
	}
}

// TestStackedScreenVeryShortHeights verifies extremely short screen heights don't overflow.
func TestStackedScreenVeryShortHeights(t *testing.T) {
	tests := []struct {
		height int
	}{
		{16},
		{17},
	}

	for _, tt := range tests {
		t.Run(string(rune('0'+tt.height/10))+string(rune('0'+tt.height%10)), func(t *testing.T) {
			s, err := store.NewFS(t.TempDir())
			if err != nil {
				t.Fatalf("NewFS: %v", err)
			}
			sess := store.Session{Name: "x", Config: config.Config{
				TargetURL:   "https://example.com/api/resource?id=123&detailed=true",
				Concurrency: 5,
				Rate:        10,
			}}
			if err := s.SaveSession(context.Background(), sess); err != nil {
				t.Fatal(err)
			}
			list, err := s.ListSessions(context.Background())
			if err != nil {
				t.Fatalf("ListSessions: %v", err)
			}

			m := screens.NewRun(s, list[0])
			cur, _ := m.Update(tea.WindowSizeMsg{Width: 72, Height: tt.height})

			out := cur.View()

			lines := strings.Split(out, "\n")
			actualLines := len(lines)
			if actualLines > tt.height {
				t.Errorf("screen output %d lines exceeds height budget %d:\n%s", actualLines, tt.height, out)
			}

			// Query params should survive
			for _, param := range []string{"id=123", "detailed=true"} {
				count := strings.Count(out, param)
				if count < 2 {
					t.Errorf("query param %q appears %d times (want >= 2)", param, count)
				}
			}
		})
	}
}

// extractSection finds a panel section by title in the output
func extractSection(output, title string) string {
	lines := strings.Split(output, "\n")
	var section []string
	inSection := false
	for _, line := range lines {
		// Check for panel title in border (┌ for top border or │ for title line)
		if strings.Contains(line, "┌ "+title+" ") || strings.Contains(line, "┌ "+title) ||
			strings.Contains(line, "│ "+title+" ") || strings.Contains(line, "│"+title+" ") {
			inSection = true
		}
		if inSection {
			section = append(section, line)
			if strings.HasPrefix(strings.TrimSpace(line), "└") || strings.HasPrefix(strings.TrimSpace(line), "┘") {
				break
			}
		}
	}
	return strings.Join(section, "\n")
}

func panelContentLines(section string) int {
	if section == "" {
		return 0
	}

	count := 0
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "│ ") || !strings.HasSuffix(line, " │") {
			continue
		}

		inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "│ "), " │"))
		if inner != "" {
			count++
		}
	}

	return count
}
