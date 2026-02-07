package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	gws "github.com/gorilla/websocket"
	"github.com/torosent/crankfire/internal/auth"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/httpclient"
	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/placeholders"
	"github.com/torosent/crankfire/internal/pool"
	"github.com/torosent/crankfire/internal/tracing"
	ws "github.com/torosent/crankfire/internal/websocket"
)

type websocketRequester struct {
	cfg       *config.WebSocketConfig
	target    string
	headers   map[string]string
	collector *metrics.Collector
	auth      auth.Provider
	feeder    httpclient.Feeder
	connPool  *pool.ConnectionPool
	helper    baseRequesterHelper
}

func newWebSocketRequester(cfg *config.Config, collector *metrics.Collector, provider auth.Provider, feeder httpclient.Feeder, tp *tracing.Provider) *websocketRequester {
	return &websocketRequester{
		cfg:       &cfg.WebSocket,
		target:    cfg.TargetURL,
		headers:   cfg.Headers,
		collector: collector,
		auth:      provider,
		feeder:    feeder,
		connPool:  pool.NewConnectionPool(cfg.Concurrency),
		helper: baseRequesterHelper{
			collector: collector,
			auth:      provider,
			feeder:    feeder,
			tracing:   tp,
		},
	}
}

func (w *websocketRequester) Do(ctx context.Context) error {
	ctx, start, meta := w.helper.initRequest(ctx, "websocket")

	ctx, span := tracing.StartRequestSpan(ctx, w.helper.tracer(), "websocket", "")
	var spanErr error
	defer func() { tracing.EndSpan(span, spanErr) }()

	record, err := w.helper.getFeederRecord(ctx)
	if err != nil {
		spanErr = err
		return w.helper.recordError(start, meta, "websocket", "feeder", err)
	}

	target := w.target
	if len(record) > 0 {
		target = placeholders.Apply(target, record)
	}

	wsHeaders, err := w.helper.prepareHeaders(ctx, w.headers, record)
	if err != nil {
		spanErr = err
		return w.helper.recordError(start, meta, "websocket", "auth", err)
	}

	if w.helper.shouldPropagate() {
		tracing.InjectHTTPHeaders(ctx, wsHeaders)
	}

	// Get or create client from pool
	poolKey := pool.MakePoolKey(target, wsHeaders)

	factory := func() pool.Poolable {
		wsCfg := ws.Config{
			URL:              target,
			Headers:          wsHeaders,
			HandshakeTimeout: w.cfg.HandshakeTimeout,
			ReadTimeout:      w.cfg.ReceiveTimeout,
			WriteTimeout:     5 * time.Second,
		}
		return ws.NewClient(wsCfg)
	}

	poolable, reused := w.connPool.Get(poolKey, factory)
	client := poolable.(*ws.Client)

	// Connect if not reused
	if !reused {
		if err := client.Connect(ctx); err != nil {
			spanErr = err
			return w.helper.recordError(start, meta, "websocket", "connect", err)
		}
	}

	// Capture metrics before operation to calculate delta
	startMetrics := client.Metrics()

	messages := append([]string(nil), w.cfg.Messages...)
	if len(record) > 0 {
		for i, msg := range messages {
			messages[i] = placeholders.Apply(msg, record)
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
				newPoolable, ok := w.connPool.RetryStaleConnection(ctx, client, factory)
				if !ok {
					opErr = fmt.Errorf("reconnect failed")
					break
				}
				client = newPoolable.(*ws.Client)
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
		spanErr = opErr
		return opErr
	}

	// If successful, return client to pool
	w.connPool.Put(poolKey, client)

	w.collector.RecordRequest(latency, nil, meta)
	return nil
}

// Close releases all WebSocket connections held in the connection pool.
func (w *websocketRequester) Close() error {
	return w.connPool.Close()
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
