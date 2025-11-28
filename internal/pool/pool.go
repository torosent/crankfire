package pool

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

// Poolable represents any client that can be pooled and reused.
type Poolable interface {
	Connect(ctx context.Context) error
	Close() error
}

// MetricsProvider allows clients to expose their current metrics state.
type MetricsProvider interface {
	Metrics() interface{}
}

// ConnectionPool manages a pool of reusable connections keyed by target+headers.
type ConnectionPool struct {
	pools sync.Map // map[string]chan Poolable
	size  int      // max connections per pool
}

// NewConnectionPool creates a new connection pool with the specified max size per key.
func NewConnectionPool(size int) *ConnectionPool {
	if size <= 0 {
		size = 10 // default size
	}
	return &ConnectionPool{
		size: size,
	}
}

// Get retrieves or creates a connection from the pool.
// If reused is true, the connection was taken from the pool.
// If reused is false, a new connection was created and the caller should connect it.
func (p *ConnectionPool) Get(key string, factory func() Poolable) (client Poolable, reused bool) {
	poolVal, _ := p.pools.LoadOrStore(key, make(chan Poolable, p.size))
	pool := poolVal.(chan Poolable)

	// Try to get an existing connection from the pool
	select {
	case client = <-pool:
		return client, true
	default:
		// Create new client if pool is empty
		return factory(), false
	}
}

// Put returns a connection to the pool for reuse.
// If the pool is full, the connection is closed instead.
func (p *ConnectionPool) Put(key string, client Poolable) error {
	poolVal, ok := p.pools.Load(key)
	if !ok {
		// Pool doesn't exist, close the client
		return client.Close()
	}

	pool := poolVal.(chan Poolable)

	select {
	case pool <- client:
		// Successfully returned to pool
		return nil
	default:
		// Pool full, close the connection
		return client.Close()
	}
}

// RetryStaleConnection attempts to reconnect a stale connection once.
// Returns the connected client and true if successful, or nil and false if failed.
func (p *ConnectionPool) RetryStaleConnection(ctx context.Context, client Poolable, factory func() Poolable) (Poolable, bool) {
	// Close the stale connection
	client.Close()

	// Create a new client and try to connect
	newClient := factory()
	if err := newClient.Connect(ctx); err != nil {
		return nil, false
	}

	return newClient, true
}

// Close closes all connections in all pools.
func (p *ConnectionPool) Close() error {
	var errs []string

	p.pools.Range(func(key, value interface{}) bool {
		if pool, ok := value.(chan Poolable); ok {
			close(pool)
			for client := range pool {
				if err := client.Close(); err != nil {
					errs = append(errs, err.Error())
				}
			}
		}
		return true
	})

	if len(errs) > 0 {
		return fmt.Errorf("pool close errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// MakePoolKey generates a deterministic key from a target URL and headers.
func MakePoolKey(target string, headers http.Header) string {
	var sb strings.Builder
	sb.WriteString(target)
	sb.WriteString("|")

	// Sort keys for deterministic key generation
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString("=")
		vals := headers[k]
		for i, v := range vals {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(v)
		}
		sb.WriteString(";")
	}
	return sb.String()
}
