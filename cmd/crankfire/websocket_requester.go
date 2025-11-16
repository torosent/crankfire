package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	gws "github.com/gorilla/websocket"
	"github.com/torosent/crankfire/internal/auth"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/httpclient"
	"github.com/torosent/crankfire/internal/metrics"
	ws "github.com/torosent/crankfire/internal/websocket"
)

type websocketRequester struct {
	cfg       *config.WebSocketConfig
	target    string
	headers   map[string]string
	collector *metrics.Collector
	auth      auth.Provider
	feeder    httpclient.Feeder
}

func newWebSocketRequester(cfg *config.Config, collector *metrics.Collector, provider auth.Provider, feeder httpclient.Feeder) *websocketRequester {
	return &websocketRequester{
		cfg:       &cfg.WebSocket,
		target:    cfg.TargetURL,
		headers:   cfg.Headers,
		collector: collector,
		auth:      provider,
		feeder:    feeder,
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

	messages := append([]string(nil), w.cfg.Messages...)
	if len(record) > 0 {
		for i, msg := range messages {
			messages[i] = applyPlaceholders(msg, record)
		}
	}

	// Create WebSocket client config
	wsCfg := ws.Config{
		URL:              target,
		Headers:          wsHeaders,
		HandshakeTimeout: w.cfg.HandshakeTimeout,
		ReadTimeout:      w.cfg.ReceiveTimeout,
		WriteTimeout:     5 * time.Second, // Default write timeout
	}

	meta := &metrics.RequestMetadata{Protocol: "websocket"}
	client := ws.NewClient(wsCfg)

	// Connect to WebSocket server
	if err := client.Connect(ctx); err != nil {
		meta = annotateStatus(meta, "websocket", websocketStatusFromError(err))
		w.collector.RecordRequest(time.Since(start), err, meta)
		return fmt.Errorf("websocket connect: %w", err)
	}
	defer client.Close()

	// Send configured messages
	for _, msg := range messages {
		if ctx.Err() != nil {
			break
		}

		if err := client.SendMessage(ctx, ws.Message{
			Type: 1, // TextMessage
			Data: []byte(msg),
		}); err != nil {
			meta = annotateStatus(meta, "websocket", websocketStatusFromError(err))
			w.collector.RecordRequest(time.Since(start), err, meta)
			return fmt.Errorf("send message: %w", err)
		}

		// Wait between messages if configured
		if w.cfg.MessageInterval > 0 && len(w.cfg.Messages) > 1 {
			select {
			case <-time.After(w.cfg.MessageInterval):
			case <-ctx.Done():
				break
			}
		}
	}

	// Optionally receive messages (with timeout)
	if w.cfg.ReceiveTimeout > 0 {
		receiveCtx, cancel := context.WithTimeout(ctx, w.cfg.ReceiveTimeout)
		defer cancel()

		for {
			_, err := client.ReceiveMessage(receiveCtx)
			if err != nil {
				// Timeout or context cancellation is expected
				if receiveCtx.Err() != nil {
					break
				}
				// Other errors
				meta = annotateStatus(meta, "websocket", websocketStatusFromError(err))
				w.collector.RecordRequest(time.Since(start), err, meta)
				return fmt.Errorf("receive message: %w", err)
			}
		}
	}

	// Get metrics and record
	wsMetrics := client.Metrics()
	latency := time.Since(start)

	// Record as successful request with WebSocket-specific metadata
	meta.CustomMetrics = map[string]interface{}{
		"connection_duration_ms": wsMetrics.ConnectionDuration.Milliseconds(),
		"messages_sent":          wsMetrics.MessagesSent,
		"messages_received":      wsMetrics.MessagesReceived,
		"bytes_sent":             wsMetrics.BytesSent,
		"bytes_received":         wsMetrics.BytesReceived,
	}

	w.collector.RecordRequest(latency, nil, meta)
	return nil
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
