package cli

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/config"
)

type seenRequest struct{ path, auth, token, body string }

// Runs a config through the real wiring (BuildRunner) against a real server
// and checks what the server received. The unit tests build these parts by
// hand, so auth, feeder data and chained values once went missing unnoticed.
func TestBuildRunner_EndToEnd(t *testing.T) {
	var mu sync.Mutex
	var seen []seenRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		seen = append(seen, seenRequest{r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("X-Token"), string(body)})
		mu.Unlock()
		_, _ = w.Write([]byte(`{"token":"abc123"}`))
	}))
	defer srv.Close()

	csvPath := filepath.Join(t.TempDir(), "users.csv")
	if err := os.WriteFile(csvPath, []byte("user_id,name\n7,al\n8,roberta\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		TargetURL:   srv.URL,
		Concurrency: 1, // one worker: requests arrive in order
		Total:       40,
		Timeout:     5 * time.Second,
		Auth:        config.AuthConfig{Type: config.AuthTypeOIDCImplicit, StaticToken: "static-tok"},
		Feeder:      config.FeederConfig{Path: csvPath, Type: "csv"},
		Endpoints: []config.Endpoint{
			{Name: "login", Path: "/login", Weight: 1,
				Extractors: []config.Extractor{{JSONPath: "token", Variable: "token"}}},
			{Name: "user", Path: "/users/{{user_id}}", Method: "POST", Weight: 1,
				Body:    `{"name":"{{name}}"}`,
				Headers: map[string]string{"X-Token": "{{token|NONE}}"}},
		},
	}

	r, collector, cleanup, err := BuildRunner(context.Background(), cfg)
	if err != nil {
		t.Fatalf("BuildRunner: %v", err)
	}
	defer cleanup()

	result := r.Run(context.Background())
	if result.Errors != 0 || collector.Stats(result.Duration).Failures != 0 {
		t.Fatalf("expected no failed requests, got %d", result.Errors)
	}
	if len(seen) != 40 {
		t.Fatalf("server saw %d requests, want 40", len(seen))
	}

	loggedIn, chained := false, 0
	for i, req := range seen {
		if req.auth != "Bearer static-tok" {
			t.Fatalf("request %d (%s): Authorization = %q", i, req.path, req.auth)
		}
		if req.path == "/login" {
			loggedIn = true
			continue
		}
		if req.path != "/users/7" && req.path != "/users/8" {
			t.Fatalf("request %d: feeder value missing from path %q", i, req.path)
		}
		if req.body != `{"name":"al"}` && req.body != `{"name":"roberta"}` {
			t.Fatalf("request %d: feeder value missing from body %q", i, req.body)
		}
		if strings.HasSuffix(req.path, "/7") != (req.body == `{"name":"al"}`) {
			t.Fatalf("request %d: path %q and body %q come from different feeder rows", i, req.path, req.body)
		}
		if loggedIn {
			if req.token != "abc123" {
				t.Fatalf("request %d: token from the earlier login was not passed on, X-Token = %q", i, req.token)
			}
			chained++
		}
	}
	if chained == 0 {
		t.Fatalf("no user request came after a login; the test proved nothing about chaining")
	}
}
