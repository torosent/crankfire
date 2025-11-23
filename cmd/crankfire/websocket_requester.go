package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	gws "github.com/gorilla/websocket"
	"github.com/torosent/crankfire/internal/auth"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/httpclient"
	"github.com/torosent/crankfire/internal/metrics"
	ws "github.com/torosent/crankfire/internal/websocket"
)

type websocketRequester struct {
	cfg         *config.WebSocketConfig
	target      string
	headers     map[string]string
	collector   *metrics.Collector
	auth        auth.Provider
	feeder      httpclient.Feeder
	concurrency int
	pools       sync.Map // map[string]chan *ws.Client
}

func newWebSocketRequester(cfg *config.Config, collector *metrics.Collector, provider auth.Provider, feeder httpclient.Feeder) *websocketRequester {
	return &websocketRequester{
		cfg:         &cfg.WebSocket,
		target:      cfg.TargetURL,
		headers:     cfg.Headers,
		collector:   collector,
		auth:        provider,
		feeder:      feeder,
		concurrency: cfg.Concurrency,
	}
}

func (w *websocketRequester) Do(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	start := time.Now()

	record, err := nextFeederRecord(ctx, w.feeder)
	if err != nil {
		meta := annotateStatus(&metrics.RequestMetadata{Protocol: "websocket"}, "websocket", fallbackStatusCode(err))
		w.collector.RecordRequest(time.Since(start), err, meta)
		return fmt.Errorf("websocket feeder: %w", err)
	}

	target := w.target
	if len(record) > 0 {
		target = applyPlaceholders(target, record)
	}

	headerMap := applyPlaceholdersToMap(w.headers, record)
	wsHeaders := makeHeaders(headerMap)
	if err := ensureAuthHeader(ctx, w.auth, wsHeaders); err != nil {
		meta := annotateStatus(&metrics.RequestMetadata{Protocol: "websocket"}, "websocket", fallbackStatusCode(err))
		w.collector.RecordRequest(time.Since(start), err, meta)
		return fmt.Errorf("websocket auth header: %w", err)
	}

	// Get or create pool for this target+headers combination
	poolKey := makePoolKey(target, wsHeaders)
	poolVal, _ := w.pools.LoadOrStore(poolKey, make(chan *ws.Client, w.concurrency))
	pool := poolVal.(chan *ws.Client)

	var client *ws.Client
	var reused bool

	// Try to get an existing connection from the pool
	select {
	case client = <-pool:
		reused = true
	default:
		// Create new client if pool is empty
		wsCfg := ws.Config{
			URL:              target,
			Headers:          wsHeaders,
			HandshakeTimeout: w.cfg.HandshakeTimeout,
			ReadTimeout:      w.cfg.ReceiveTimeout,
			WriteTimeout:     5 * time.Second,
		}
		client = ws.NewClient(wsCfg)
	}

	meta := &metrics.RequestMetadata{Protocol: "websocket"}

	// Connect if not reused
	if !reused {
		if err := client.Connect(ctx); err != nil {
			meta = annotateStatus(meta, "websocket", websocketStatusFromError(err))
			w.collector.RecordRequest(time.Since(start), err, meta)
			return fmt.Errorf("websocket connect: %w", err)
		}
	}

	// Capture metrics before operation to calculate delta
	startMetrics := client.Metrics()

	messages := append([]string(nil), w.cfg.Messages...)
	if len(record) > 0 {
		for i, msg := range messages {
			messages[i] = applyPlaceholders(msg, record)
		}
	}

	var opErr error

	// Send configured messages
	for i, msg := range messages {
		if ctx.Err() != nil {
			opErr = ctx.Err()
			break
		}

		if err := client.SendMessage(ctx, ws.Message{
			Type: 1, // TextMessage
			Data: []byte(msg),
		}); err != nil {
			// If this is a reused connection and we failed on the first message,
			// it's likely a stale connection. Try to reconnect once.
			if reused && i == 0 {
				client.Close()
				if connErr := client.Connect(ctx); connErr == nil {
					reused = false
					// Retry sending the message
					if err := client.SendMessage(ctx, ws.Message{
						Type: 1, // TextMessage
						Data: []byte(msg),
					}); err != nil {
						opErr = fmt.Errorf("send message: %w", err)
						break
					}
				} else {
					opErr = fmt.Errorf("reconnect failed: %w", connErr)
					break
				}
			} else {
				opErr = fmt.Errorf("send message: %w", err)
				break
			}
		}

		// Wait between messages if configured
		if w.cfg.MessageInterval > 0 && len(w.cfg.Messages) > 1 {
			select {
			case <-time.After(w.cfg.MessageInterval):
			case <-ctx.Done():
				opErr = ctx.Err()
				break
			}
		}
		if opErr != nil {
			break
		}
	}

	// Optionally receive messages (with timeout)
	if opErr == nil && w.cfg.ReceiveTimeout > 0 {
		receiveCtx, cancel := context.WithTimeout(ctx, w.cfg.ReceiveTimeout)
		defer cancel()

		for {
			_, err := client.ReceiveMessage(receiveCtx)
			if err != nil {
				// Timeout or context cancellation is expected
				if receiveCtx.Err() != nil {
					break
				}

				// Check for net.Error timeout (e.g. read deadline exceeded)
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					break
				}

				// Other errors
				opErr = fmt.Errorf("receive message: %w", err)
				break
			}
		}
	}

	// Calculate delta metrics
	endMetrics := client.Metrics()
	latency := time.Since(start)

	meta.CustomMetrics = map[string]interface{}{
		"connection_duration_ms": endMetrics.ConnectionDuration.Milliseconds(),
		"messages_sent":          endMetrics.MessagesSent - startMetrics.MessagesSent,
		"messages_received":      endMetrics.MessagesReceived - startMetrics.MessagesReceived,
		"bytes_sent":             endMetrics.BytesSent - startMetrics.BytesSent,
		"bytes_received":         endMetrics.BytesReceived - startMetrics.BytesReceived,
	}

	if opErr != nil {
		// If error occurred, close client and do not return to pool
		client.Close()
		meta = annotateStatus(meta, "websocket", websocketStatusFromError(opErr))
		w.collector.RecordRequest(latency, opErr, meta)
		return opErr
	}

	// If successful, return client to pool
	select {
	case pool <- client:
		// Returned to pool
	default:
		// Pool full (shouldn't happen if sized correctly, but safe to close)
		client.Close()
	}

	w.collector.RecordRequest(latency, nil, meta)
	return nil
}

func makePoolKey(target string, headers http.Header) string {
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
		// We only care about the first value for the key typically, or join them
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

func websocketStatusFromError(err error) string {
	if err == nil {
		return ""
	}
	var closeErr *gws.CloseError
	if errors.As(err, &closeErr) && closeErr.Code != 0 {
		return strconv.Itoa(closeErr.Code)
	}
	return fallbackStatusCode(err)
}
