package websocket

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/torosent/crankfire/internal/clientmetrics"
)

// Message represents a WebSocket message to send or receive.
type Message struct {
	Type int // websocket.TextMessage or websocket.BinaryMessage
	Data []byte
}

// Metrics captures WebSocket-specific performance data.
type Metrics struct {
	ConnectionDuration time.Duration
	MessagesSent       int64
	MessagesReceived   int64
	BytesSent          int64
	BytesReceived      int64
	Errors             int64
}

// Client represents a WebSocket client connection.
type Client struct {
	url     string
	headers http.Header
	dialer  *websocket.Dialer
	conn    *websocket.Conn
	mu      sync.Mutex
	metrics *clientmetrics.ClientMetrics
}

// Config configures the WebSocket client behavior.
type Config struct {
	URL              string
	Headers          http.Header
	HandshakeTimeout time.Duration
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	MaxMessageSize   int64
}

// NewClient creates a new WebSocket client with the given configuration.
func NewClient(cfg Config) *Client {
	if cfg.HandshakeTimeout == 0 {
		cfg.HandshakeTimeout = 30 * time.Second
	}

	if cfg.MaxMessageSize == 0 {
		cfg.MaxMessageSize = 1024 * 1024 // 1MB default
	}

	dialer := &websocket.Dialer{
		HandshakeTimeout: cfg.HandshakeTimeout,
		Proxy:            http.ProxyFromEnvironment,
	}

	return &Client{
		url:     cfg.URL,
		headers: cfg.Headers,
		dialer:  dialer,
		metrics: clientmetrics.New(),
	}
}

// Connect establishes a WebSocket connection.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return fmt.Errorf("already connected")
	}

	conn, resp, err := c.dialer.DialContext(ctx, c.url, c.headers)
	if err != nil {
		c.metrics.IncrementErrors()
		if resp != nil {
			return fmt.Errorf("websocket dial failed with status %d: %w", resp.StatusCode, err)
		}
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	c.conn = conn
	c.metrics.MarkConnected()

	return nil
}

// SendMessage sends a message over the WebSocket connection.
func (c *Client) SendMessage(ctx context.Context, msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	if err := c.conn.WriteMessage(msg.Type, msg.Data); err != nil {
		c.metrics.IncrementErrors()
		return fmt.Errorf("write message: %w", err)
	}

	c.metrics.IncrementSent(int64(len(msg.Data)))

	return nil
}

// ReceiveMessage reads a message from the WebSocket connection.
// Returns an error if the connection is closed or times out.
func (c *Client) ReceiveMessage(ctx context.Context) (Message, error) {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return Message{}, fmt.Errorf("not connected")
	}

	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetReadDeadline(deadline); err != nil {
			return Message{}, fmt.Errorf("set read deadline: %w", err)
		}
	} else {
		// Clear deadline if no deadline in context
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			return Message{}, fmt.Errorf("clear read deadline: %w", err)
		}
	}

	msgType, data, err := conn.ReadMessage()
	if err != nil {
		c.metrics.IncrementErrors()
		return Message{}, fmt.Errorf("read message: %w", err)
	}

	c.metrics.IncrementReceived(int64(len(data)))

	return Message{Type: msgType, Data: data}, nil
}

// Close closes the WebSocket connection gracefully.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}

	// Send close frame
	err := c.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(5*time.Second),
	)

	closeErr := c.conn.Close()
	c.conn = nil

	if err != nil {
		return err
	}

	return closeErr
}

// Metrics returns the current metrics snapshot.
func (c *Client) Metrics() Metrics {
	snapshot := c.metrics.Snapshot()
	return Metrics{
		ConnectionDuration: snapshot.ConnectionDuration,
		MessagesSent:       snapshot.MessagesSent,
		MessagesReceived:   snapshot.MessagesReceived,
		BytesSent:          snapshot.BytesSent,
		BytesReceived:      snapshot.BytesReceived,
		Errors:             snapshot.Errors,
	}
}
