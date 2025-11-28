package sse

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/torosent/crankfire/internal/clientmetrics"
)

// Event represents a Server-Sent Event.
type Event struct {
	ID    string
	Event string
	Data  string
}

// StatusError is returned when the SSE endpoint responds with a non-200 status code.
type StatusError struct {
	Code int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("unexpected status code: %d", e.Code)
}

// Metrics captures SSE-specific performance data.
type Metrics struct {
	ConnectionDuration time.Duration
	EventsReceived     int64
	BytesReceived      int64
	Errors             int64
}

// Client represents an SSE client connection.
type Client struct {
	url        string
	headers    http.Header
	httpClient *http.Client
	resp       *http.Response
	reader     *bufio.Reader
	mu         sync.Mutex
	metrics    *clientmetrics.ClientMetrics
	eventsRecv int64 // SSE-specific: count of complete events (not lines)
}

// Config configures the SSE client behavior.
type Config struct {
	URL     string
	Headers http.Header
	Timeout time.Duration
}

// NewClient creates a new SSE client with the given configuration.
func NewClient(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &Client{
		url:     cfg.URL,
		headers: cfg.Headers,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		metrics: clientmetrics.New(),
	}
}

// Connect establishes an SSE connection.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.resp != nil {
		return fmt.Errorf("already connected")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		c.metrics.IncrementErrors()
		return fmt.Errorf("create request: %w", err)
	}

	// Set SSE-specific headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// Copy custom headers
	for key, values := range c.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.metrics.IncrementErrors()
		return fmt.Errorf("http request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.metrics.IncrementErrors()
		resp.Body.Close()
		return &StatusError{Code: resp.StatusCode}
	}

	c.resp = resp
	c.reader = bufio.NewReader(resp.Body)
	c.metrics.MarkConnected()

	return nil
}

// ReadEvent reads the next SSE event from the stream.
func (c *Client) ReadEvent(ctx context.Context) (Event, error) {
	c.mu.Lock()
	reader := c.reader
	c.mu.Unlock()

	if reader == nil {
		return Event{}, fmt.Errorf("not connected")
	}

	event := Event{}
	var dataLines []string

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return Event{}, ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			c.metrics.IncrementErrors()
			if err == io.EOF {

				return Event{}, fmt.Errorf("connection closed")
			}
			return Event{}, fmt.Errorf("read line: %w", err)
		}

		c.metrics.IncrementReceived(int64(len(line)))

		line = strings.TrimRight(line, "\r\n")

		// Empty line marks end of event
		if line == "" {
			if len(dataLines) > 0 || event.Event != "" || event.ID != "" {
				event.Data = strings.Join(dataLines, "\n")
				c.mu.Lock()
				c.eventsRecv++
				c.mu.Unlock()
				return event, nil
			}
			continue
		}

		// Comment line, ignore
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Parse field
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			// Malformed line, skip
			continue
		}

		field := line[:colonIdx]
		value := line[colonIdx+1:]

		// Strip leading space
		if len(value) > 0 && value[0] == ' ' {
			value = value[1:]
		}

		switch field {
		case "id":
			event.ID = value
		case "event":
			event.Event = value
		case "data":
			dataLines = append(dataLines, value)
		}
	}
}

// Close closes the SSE connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.resp == nil {
		return nil
	}

	err := c.resp.Body.Close()
	c.resp = nil
	c.reader = nil

	return err
}

// Metrics returns the current metrics snapshot.
func (c *Client) Metrics() Metrics {
	c.mu.Lock()
	eventsRecv := c.eventsRecv
	c.mu.Unlock()

	snapshot := c.metrics.Snapshot()
	return Metrics{
		ConnectionDuration: snapshot.ConnectionDuration,
		EventsReceived:     eventsRecv, // Use SSE-specific event counter
		BytesReceived:      snapshot.BytesReceived,
		Errors:             snapshot.Errors,
	}
}
