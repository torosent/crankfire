package grpcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// Metrics holds performance metrics for gRPC calls
type Metrics struct {
	CallDuration time.Duration
	MessagesSent int64
	MessagesRecv int64
	BytesSent    int64
	BytesRecv    int64
	Errors       int64
	StatusCode   string
}

// Client represents a gRPC client
type Client struct {
	target    string
	conn      *grpc.ClientConn
	service   string
	method    string
	md        metadata.MD
	mu        sync.Mutex
	callCount int64
	bytesSent int64
	bytesRecv int64
	errors    int64
	useTLS    bool
	insecure  bool
}

// Config holds configuration for the gRPC client
type Config struct {
	Target   string
	Service  string
	Method   string
	Metadata map[string]string
	Timeout  time.Duration
	UseTLS   bool
	Insecure bool
}

// NewClient creates a new gRPC client with the given configuration
func NewClient(cfg Config) (*Client, error) {
	// Convert metadata map to metadata.MD
	md := metadata.New(cfg.Metadata)

	// Set default timeout if not provided
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	client := &Client{
		target:   cfg.Target,
		service:  cfg.Service,
		method:   cfg.Method,
		md:       md,
		useTLS:   cfg.UseTLS,
		insecure: cfg.Insecure,
	}

	return client, nil
}

// Connect establishes a gRPC connection
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already connected
	if c.conn != nil {
		return fmt.Errorf("client already connected")
	}

	var opts []grpc.DialOption

	// Configure TLS options
	if c.useTLS {
		if c.insecure {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else {
			creds := credentials.NewTLS(nil)
			opts = append(opts, grpc.WithTransportCredentials(creds))
		}
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Establish connection
	conn, err := grpc.DialContext(ctx, c.target, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", c.target, err)
	}

	c.conn = conn
	return nil
}

// Invoke makes a unary RPC call
func (c *Client) Invoke(ctx context.Context, req interface{}) (interface{}, error) {
	c.mu.Lock()
	if c.conn == nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("client not connected")
	}
	conn := c.conn
	c.mu.Unlock()

	// Add metadata to context
	if len(c.md) > 0 {
		ctx = metadata.NewOutgoingContext(ctx, c.md)
	}

	// Marshal request to JSON for size tracking
	reqBytes, err := json.Marshal(req)
	if err != nil {
		c.mu.Lock()
		c.errors++
		c.mu.Unlock()
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Format full method name
	fullMethod := fmt.Sprintf("/%s/%s", c.service, c.method)

	// Track call start time
	start := time.Now()

	// Make the RPC call
	var resp interface{}
	err = conn.Invoke(ctx, fullMethod, req, &resp)

	// Track call duration (could be used for per-call metrics in the future)
	_ = time.Since(start)

	// Update metrics
	c.mu.Lock()
	c.callCount++
	c.bytesSent += int64(len(reqBytes))
	if err != nil {
		c.errors++
	} else {
		// Track response size if successful
		if respBytes, marshalErr := json.Marshal(resp); marshalErr == nil {
			c.bytesRecv += int64(len(respBytes))
		}
	}
	c.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("RPC call failed: %w", err)
	}

	return resp, nil
}

// Close closes the gRPC connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}

	err := c.conn.Close()
	c.conn = nil
	return err
}

// Metrics returns the current metrics (thread-safe)
func (c *Client) Metrics() Metrics {
	c.mu.Lock()
	defer c.mu.Unlock()

	return Metrics{
		MessagesSent: c.callCount,
		MessagesRecv: c.callCount - c.errors,
		BytesSent:    c.bytesSent,
		BytesRecv:    c.bytesRecv,
		Errors:       c.errors,
		StatusCode:   "OK",
	}
}
