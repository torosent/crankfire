package grpcclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/torosent/crankfire/internal/clientmetrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
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
	target     string
	conn       *grpc.ClientConn
	service    string
	method     string
	md         metadata.MD
	mu         sync.Mutex
	metrics    *clientmetrics.ClientMetrics
	callCount  int64 // Track calls separately for gRPC-specific logic
	useTLS     bool
	insecure   bool
	lastStatus string
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
	md := metadata.New(cfg.Metadata)
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	client := &Client{
		target:     cfg.Target,
		service:    cfg.Service,
		method:     cfg.Method,
		md:         md,
		useTLS:     cfg.UseTLS,
		insecure:   cfg.Insecure,
		lastStatus: "UNSET",
		metrics:    clientmetrics.New(),
	}
	return client, nil
}

// Connect establishes a gRPC connection
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return fmt.Errorf("client already connected")
	}

	conn, err := Dial(ctx, Config{
		Target:   c.target,
		UseTLS:   c.useTLS,
		Insecure: c.insecure,
		Timeout:  30 * time.Second, // Default, though Dial context governs timeout
	})
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

// Dial establishes a gRPC connection based on configuration
func Dial(ctx context.Context, cfg Config) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	if cfg.UseTLS {
		if cfg.Insecure {
			// Use TLS but skip certificate verification
			creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
			opts = append(opts, grpc.WithTransportCredentials(creds))
		} else {
			// Use TLS with proper certificate verification
			creds := credentials.NewClientTLSFromCert(nil, "")
			opts = append(opts, grpc.WithTransportCredentials(creds))
		}
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// grpc.NewClient is non-blocking and doesn't take a context for dialing itself
	return grpc.NewClient(cfg.Target, opts...)
}

// NewClientWithConn creates a new gRPC client using an existing connection
func NewClientWithConn(conn *grpc.ClientConn, cfg Config) *Client {
	md := metadata.New(cfg.Metadata)
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &Client{
		target:     cfg.Target,
		conn:       conn,
		service:    cfg.Service,
		method:     cfg.Method,
		md:         md,
		useTLS:     cfg.UseTLS,
		insecure:   cfg.Insecure,
		lastStatus: "UNSET",
		metrics:    clientmetrics.New(),
	}
}

// Invoke makes a unary RPC call
func (c *Client) Invoke(ctx context.Context, req proto.Message, resp proto.Message) error {
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}
	if resp == nil {
		return fmt.Errorf("response cannot be nil")
	}

	c.mu.Lock()
	if c.conn == nil {
		c.mu.Unlock()
		return fmt.Errorf("client not connected")
	}
	conn := c.conn
	c.mu.Unlock()

	if len(c.md) > 0 {
		ctx = metadata.NewOutgoingContext(ctx, c.md)
	}

	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	fullMethod := fmt.Sprintf("/%s/%s", c.service, c.method)
	start := time.Now()
	err = conn.Invoke(ctx, fullMethod, req, resp)
	_ = time.Since(start)

	var respBytes []byte
	if err == nil {
		if b, marshalErr := proto.Marshal(resp); marshalErr == nil {
			respBytes = b
		}
	}

	code := status.Code(err).String()

	c.mu.Lock()
	c.callCount++
	c.lastStatus = code
	c.mu.Unlock()

	c.metrics.IncrementSent(int64(len(reqBytes)))
	if err != nil {
		c.metrics.IncrementErrors()
	} else {
		c.metrics.IncrementReceived(int64(len(respBytes)))
	}

	if err != nil {
		return fmt.Errorf("RPC call failed: %w", err)
	}
	return nil
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
	lastStatus := c.lastStatus
	c.mu.Unlock()

	snapshot := c.metrics.Snapshot()
	return Metrics{
		CallDuration: snapshot.ConnectionDuration,
		MessagesSent: snapshot.MessagesSent,
		MessagesRecv: snapshot.MessagesReceived,
		BytesSent:    snapshot.BytesSent,
		BytesRecv:    snapshot.BytesReceived,
		Errors:       snapshot.Errors,
		StatusCode:   lastStatus,
	}
}
