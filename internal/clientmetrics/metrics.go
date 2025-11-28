package clientmetrics

import (
	"sync"
	"time"
)

// ClientMetrics tracks connection and message statistics for protocol clients.
type ClientMetrics struct {
	mu              sync.Mutex
	connectTime     time.Time
	messagesSent    int64
	messagesRecv    int64
	bytesSent       int64
	bytesRecv       int64
	errors          int64
}

// New creates a new ClientMetrics instance.
func New() *ClientMetrics {
	return &ClientMetrics{}
}

// MarkConnected records the connection time.
func (m *ClientMetrics) MarkConnected() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectTime = time.Now()
}

// IncrementSent increments messages sent and bytes sent counters.
func (m *ClientMetrics) IncrementSent(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messagesSent++
	m.bytesSent += bytes
}

// IncrementReceived increments messages received and bytes received counters.
func (m *ClientMetrics) IncrementReceived(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messagesRecv++
	m.bytesRecv += bytes
}

// IncrementErrors increments the error counter.
func (m *ClientMetrics) IncrementErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors++
}

// Reset clears the connection time (used when disconnecting).
func (m *ClientMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectTime = time.Time{}
}

// ConnectionDuration returns the duration since connection was established.
// Returns 0 if not connected.
func (m *ClientMetrics) ConnectionDuration() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connectTime.IsZero() {
		return 0
	}
	return time.Since(m.connectTime)
}

// MessagesSent returns the total messages sent.
func (m *ClientMetrics) MessagesSent() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.messagesSent
}

// MessagesReceived returns the total messages received.
func (m *ClientMetrics) MessagesReceived() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.messagesRecv
}

// BytesSent returns the total bytes sent.
func (m *ClientMetrics) BytesSent() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bytesSent
}

// BytesReceived returns the total bytes received.
func (m *ClientMetrics) BytesReceived() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bytesRecv
}

// Errors returns the total error count.
func (m *ClientMetrics) Errors() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.errors
}

// Snapshot returns a snapshot of all metrics at a point in time.
type Snapshot struct {
	ConnectionDuration time.Duration
	MessagesSent       int64
	MessagesReceived   int64
	BytesSent          int64
	BytesReceived      int64
	Errors             int64
}

// Snapshot returns a consistent snapshot of all metrics.
func (m *ClientMetrics) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	duration := time.Duration(0)
	if !m.connectTime.IsZero() {
		duration = time.Since(m.connectTime)
	}
	
	return Snapshot{
		ConnectionDuration: duration,
		MessagesSent:       m.messagesSent,
		MessagesReceived:   m.messagesRecv,
		BytesSent:          m.bytesSent,
		BytesReceived:      m.bytesRecv,
		Errors:             m.errors,
	}
}
