package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/torosent/crankfire/internal/auth"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/grpcclient"
	"github.com/torosent/crankfire/internal/httpclient"
	"github.com/torosent/crankfire/internal/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/protoadapt"
)

type grpcRequester struct {
	cfg        *config.GRPCConfig
	target     string
	collector  *metrics.Collector
	auth       auth.Provider
	feeder     httpclient.Feeder
	methodOnce sync.Once
	methodDesc *desc.MethodDescriptor
	methodErr  error
	conns      sync.Map // map[string]*grpc.ClientConn
	helper     baseRequesterHelper
}

func newGRPCRequester(cfg *config.Config, collector *metrics.Collector, provider auth.Provider, feeder httpclient.Feeder) *grpcRequester {
	return &grpcRequester{
		cfg:       &cfg.GRPC,
		target:    cfg.TargetURL,
		collector: collector,
		auth:      provider,
		feeder:    feeder,
		helper: baseRequesterHelper{
			collector: collector,
			auth:      provider,
			feeder:    feeder,
		},
	}
}

func (g *grpcRequester) Do(ctx context.Context) error {
	ctx, start, meta := g.helper.initRequest(ctx, "grpc")

	record, err := g.helper.getFeederRecord(ctx)
	if err != nil {
		return g.helper.recordError(start, meta, "grpc", "feeder", err)
	}

	methodDesc, err := g.methodDescriptor()
	if err != nil {
		meta = annotateStatus(meta, "grpc", fallbackStatusCode(err))
		g.collector.RecordRequest(time.Since(start), err, meta)
		return fmt.Errorf("load proto descriptor: %w", err)
	}

	target := g.target
	if len(record) > 0 {
		target = applyPlaceholders(target, record)
	}

	metadata := buildGRPCMetadata(g.cfg.Metadata, record)
	if metadata, err = injectGRPCAuth(ctx, g.auth, metadata); err != nil {
		meta = annotateStatus(meta, "grpc", fallbackStatusCode(err))
		g.collector.RecordRequest(time.Since(start), err, meta)
		return fmt.Errorf("grpc auth metadata: %w", err)
	}

	messagePayload := g.cfg.Message
	if len(record) > 0 {
		messagePayload = applyPlaceholders(messagePayload, record)
	}

	reqMsg, err := buildDynamicRequest(methodDesc, messagePayload)
	if err != nil {
		meta = annotateStatus(meta, "grpc", fallbackStatusCode(err))
		g.collector.RecordRequest(time.Since(start), err, meta)
		return fmt.Errorf("grpc request payload: %w", err)
	}
	respMsg := dynamic.NewMessage(methodDesc.GetOutputType())

	grpcCfg := grpcclient.Config{
		Target:   target,
		Service:  g.cfg.Service,
		Method:   g.cfg.Method,
		Metadata: metadata,
		Timeout:  g.cfg.Timeout,
		UseTLS:   g.cfg.TLS,
		Insecure: g.cfg.Insecure,
	}

	// Get or create connection
	var conn *grpc.ClientConn
	if v, ok := g.conns.Load(target); ok {
		conn = v.(*grpc.ClientConn)
	} else {
		// Create new connection
		// Note: In high concurrency with dynamic targets, this might create duplicates
		// which are thrown away by LoadOrStore, but that's acceptable.
		newConn, err := grpcclient.Dial(ctx, grpcCfg)
		if err != nil {
			meta = annotateStatus(meta, "grpc", grpcStatusCode(err))
			g.collector.RecordRequest(time.Since(start), err, meta)
			return fmt.Errorf("grpc connect: %w", err)
		}

		if actual, loaded := g.conns.LoadOrStore(target, newConn); loaded {
			newConn.Close() // Close duplicate
			conn = actual.(*grpc.ClientConn)
		} else {
			conn = newConn
		}
	}

	client := grpcclient.NewClientWithConn(conn, grpcCfg)
	// Do NOT close client, as it would close the shared connection

	callCtx := ctx
	if g.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, g.cfg.Timeout)
		defer cancel()
	}

	reqProto := protoadapt.MessageV2Of(reqMsg)
	respProto := protoadapt.MessageV2Of(respMsg)
	if err := client.Invoke(callCtx, reqProto, respProto); err != nil {
		latency := time.Since(start)
		meta = annotateStatus(meta, "grpc", grpcStatusCode(err))
		g.collector.RecordRequest(latency, err, meta)
		return fmt.Errorf("grpc invoke: %w", err)
	}

	latency := time.Since(start)
	grpcMetrics := client.Metrics()
	meta = ensureProtocolMeta(meta, "grpc")
	meta.CustomMetrics = map[string]interface{}{
		"messages_sent":     grpcMetrics.MessagesSent,
		"messages_received": grpcMetrics.MessagesRecv,
		"bytes_sent":        grpcMetrics.BytesSent,
		"bytes_received":    grpcMetrics.BytesRecv,
		"status_code":       grpcMetrics.StatusCode,
	}

	g.collector.RecordRequest(latency, nil, meta)
	return nil
}

func (g *grpcRequester) methodDescriptor() (*desc.MethodDescriptor, error) {
	g.methodOnce.Do(func() {
		g.methodDesc, g.methodErr = loadMethodDescriptor(g.cfg)
	})
	return g.methodDesc, g.methodErr
}

func loadMethodDescriptor(cfg *config.GRPCConfig) (*desc.MethodDescriptor, error) {
	protoPath := strings.TrimSpace(cfg.ProtoFile)
	if protoPath == "" {
		return nil, fmt.Errorf("grpc proto_file is required")
	}
	parser := protoparse.Parser{
		ImportPaths: []string{filepath.Dir(protoPath)},
	}
	files, err := parser.ParseFiles(filepath.Base(protoPath))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no descriptors parsed from %s", protoPath)
	}
	serviceName := strings.TrimSpace(cfg.Service)
	methodName := strings.TrimSpace(cfg.Method)
	for _, file := range files {
		for _, svc := range file.GetServices() {
			if matchesServiceName(svc, serviceName) {
				if method := svc.FindMethodByName(methodName); method != nil {
					return method, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("method %s not found in service %s", methodName, serviceName)
}

func matchesServiceName(svc *desc.ServiceDescriptor, target string) bool {
	if target == "" {
		return false
	}
	if svc.GetFullyQualifiedName() == target {
		return true
	}
	return svc.GetName() == target || strings.HasSuffix(target, "."+svc.GetName())
}

func buildDynamicRequest(method *desc.MethodDescriptor, payload string) (*dynamic.Message, error) {
	msg := dynamic.NewMessage(method.GetInputType())
	body := strings.TrimSpace(payload)
	if body == "" {
		body = "{}"
	}
	if err := msg.UnmarshalJSON([]byte(body)); err != nil {
		return nil, err
	}
	return msg, nil
}

func buildGRPCMetadata(base map[string]string, record map[string]string) map[string]string {
	if len(base) == 0 && len(record) == 0 {
		return nil
	}
	plhdr := applyPlaceholdersToMap(base, record)
	if len(plhdr) == 0 {
		return nil
	}
	result := make(map[string]string, len(plhdr))
	for key, value := range plhdr {
		trimmed := strings.ToLower(strings.TrimSpace(key))
		if trimmed == "" {
			continue
		}
		result[trimmed] = value
	}
	return result
}

func injectGRPCAuth(ctx context.Context, provider auth.Provider, metadata map[string]string) (map[string]string, error) {
	if provider == nil {
		return metadata, nil
	}
	token, err := provider.Token(ctx)
	if err != nil {
		return metadata, err
	}
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["authorization"] = fmt.Sprintf("Bearer %s", token)
	return metadata, nil
}

// Close releases all gRPC connections held by the requester.
func (g *grpcRequester) Close() error {
	g.conns.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*grpc.ClientConn); ok {
			conn.Close()
		}
		return true
	})
	return nil
}

func grpcStatusCode(err error) string {
	if err == nil {
		return ""
	}
	if st, ok := status.FromError(err); ok {
		return st.Code().String()
	}
	return fallbackStatusCode(err)
}
