package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type serverMode string

const (
	modeHTTP      serverMode = "http"
	modeSSE       serverMode = "sse"
	modeWebSocket serverMode = "websocket"
	modeGRPC      serverMode = "grpc"
)

func main() {
	mode := flag.String("mode", "", "Server mode: http, sse, websocket, grpc")
	port := flag.Int("port", 0, "Listening port")
	docSamples := flag.String("doc-samples", "", "Path to doc-samples directory (required for grpc mode)")
	flag.Parse()

	if *port <= 0 {
		log.Fatalf("port must be > 0")
	}

	switch serverMode(*mode) {
	case modeHTTP:
		log.Fatal(runHTTPServer(*port))
	case modeSSE:
		log.Fatal(runSSEServer(*port))
	case modeWebSocket:
		log.Fatal(runWebSocketServer(*port))
	case modeGRPC:
		if strings.TrimSpace(*docSamples) == "" {
			log.Fatalf("doc-samples path is required for grpc mode")
		}
		log.Fatal(runGRPCServer(*port, *docSamples))
	default:
		log.Fatalf("unknown mode %q", *mode)
	}
}

func runHTTPServer(port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", handleOAuthToken)
	mux.HandleFunc("/v1/users", handleUsers)
	mux.HandleFunc("/users/", handleUserProfile)
	mux.HandleFunc("/search", handleSearch)
	// Request chaining endpoints
	mux.HandleFunc("/api/users", handleAPIUsers)
	mux.HandleFunc("/api/orders", handleAPIOrders)
	mux.HandleFunc("/api/orders/", handleAPIOrderByID)
	// Features matrix test endpoints
	mux.HandleFunc("/timestamp", handleTimestamp)
	mux.HandleFunc("/echo", handleEcho)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]any{"ok": true, "path": r.URL.Path})
	})

	addr := fmt.Sprintf(":%d", port)
	log.Printf("doc-sample HTTP server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func handleOAuthToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"access_token": "doc-sample-token",
		"token_type":   "bearer",
		"expires_in":   3600,
	})
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		respondJSON(w, http.StatusOK, map[string]any{
			"users": []map[string]any{{"id": "u-1", "name": "Sample"}},
		})
	case http.MethodPost:
		respondJSON(w, http.StatusCreated, map[string]any{"status": "created"})
	default:
		respondJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
	}
}

func handleUserProfile(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, "/profile") {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	parts := strings.Split(r.URL.Path, "/")
	userID := "unknown"
	if len(parts) >= 2 {
		userID = parts[len(parts)-2]
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"user_id":    userID,
		"department": r.Header.Get("X-User-Department"),
		"role":       r.Header.Get("X-User-Role"),
	})
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{
		"query":    r.URL.Query().Get("query"),
		"category": r.URL.Query().Get("category"),
		"limit":    r.URL.Query().Get("limit"),
		"results":  []string{"alpha", "beta"},
	})
}

// Request chaining endpoints

var userCounter int
var orderCounter int

func handleAPIUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		userCounter++
		userID := fmt.Sprintf("user-%d", userCounter)
		token := fmt.Sprintf("token-%d-%d", userCounter, time.Now().UnixNano())
		respondJSON(w, http.StatusCreated, map[string]any{
			"id":    userID,
			"name":  "Test User",
			"email": "test@example.com",
			"auth": map[string]any{
				"token": token,
			},
			"profile_url": fmt.Sprintf("/users/%s/profile", userID),
			"created_at":  time.Now().Format(time.RFC3339),
		})
	case http.MethodGet:
		respondJSON(w, http.StatusOK, map[string]any{
			"users": []map[string]any{
				{"id": "user-1", "name": "User 1"},
				{"id": "user-2", "name": "User 2"},
			},
			"total": 2,
		})
	default:
		respondJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
	}
}

func handleAPIOrders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		orderCounter++
		orderID := fmt.Sprintf("order-%d", orderCounter)
		respondJSON(w, http.StatusCreated, map[string]any{
			"id":         orderID,
			"status":     "pending",
			"total":      99.99,
			"created_at": time.Now().Format(time.RFC3339),
			"user_id":    r.Header.Get("X-User-ID"),
		})
	case http.MethodGet:
		userID := r.URL.Query().Get("user_id")
		respondJSON(w, http.StatusOK, map[string]any{
			"orders": []map[string]any{
				{"id": "order-1", "status": "completed", "total": 50.00},
				{"id": "order-2", "status": "pending", "total": 75.50},
			},
			"user_id": userID,
			"total":   2,
		})
	default:
		respondJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
	}
}

func handleAPIOrderByID(w http.ResponseWriter, r *http.Request) {
	// Extract order ID from path like /api/orders/order-123
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/orders/"), "/")
	orderID := parts[0]

	if orderID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "order_id required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		respondJSON(w, http.StatusOK, map[string]any{
			"id":         orderID,
			"status":     "pending",
			"total":      149.99,
			"items":      []map[string]any{{"name": "Widget", "qty": 2}},
			"created_at": time.Now().Format(time.RFC3339),
		})
	case http.MethodPatch, http.MethodPut:
		respondJSON(w, http.StatusOK, map[string]any{
			"id":         orderID,
			"status":     "updated",
			"updated_at": time.Now().Format(time.RFC3339),
		})
	case http.MethodDelete:
		respondJSON(w, http.StatusOK, map[string]any{
			"id":      orderID,
			"deleted": true,
		})
	default:
		respondJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
	}
}

// Features matrix test handlers

func handleTimestamp(w http.ResponseWriter, r *http.Request) {
	// Return current server timestamp for timing validation
	respondJSON(w, http.StatusOK, map[string]any{
		"timestamp": time.Now().UnixNano(),
		"iso":       time.Now().UTC().Format(time.RFC3339Nano),
		"unix":      time.Now().Unix(),
	})
}

func handleEcho(w http.ResponseWriter, r *http.Request) {
	// Echo request details (method, headers, body) for validation
	body := ""
	if r.Body != nil {
		bodyBytes, _ := io.ReadAll(r.Body)
		body = string(bodyBytes)
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"method":       r.Method,
		"path":         r.URL.Path,
		"query":        r.URL.RawQuery,
		"headers":      r.Header,
		"body":         body,
		"content_type": r.Header.Get("Content-Type"),
		"timestamp":    time.Now().UnixNano(),
	})
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func runSSEServer(port int) error {
	mux := http.NewServeMux()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		topic := r.URL.Query().Get("topic")
		if topic == "" {
			topic = "default"
		}

		for i := 0; i < 10; i++ {
			fmt.Fprintf(w, "event: message\ndata: {\"topic\":\"%s\", \"seq\": %d}\n\n", topic, i)
			flusher.Flush()
			time.Sleep(50 * time.Millisecond)
		}
	}
	mux.HandleFunc("/events", handler)
	mux.HandleFunc("/stream", handler)
	mux.HandleFunc("/", handler)

	addr := fmt.Sprintf(":%d", port)
	log.Printf("doc-sample SSE server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func runWebSocketServer(port int) error {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade failed: %v", err)
			return
		}
		go handleWebSocketConn(conn)
	})

	addr := fmt.Sprintf(":%d", port)
	log.Printf("doc-sample WebSocket server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func handleWebSocketConn(conn *websocket.Conn) {
	defer conn.Close()
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"status":"connected"}`))
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if err := conn.WriteMessage(msgType, data); err != nil {
			return
		}
	}
}

type dynamicResponder func(ctx context.Context, method *desc.MethodDescriptor, req *dynamic.Message) (*dynamic.Message, error)

func runGRPCServer(port int, docSamples string) error {
	parser := protoparse.Parser{ImportPaths: []string{docSamples}}
	files, err := parser.ParseFiles(
		filepath.ToSlash("users.proto"),
		filepath.ToSlash("api/service.proto"),
		filepath.ToSlash("api/orders.proto"),
	)
	if err != nil {
		return fmt.Errorf("parse proto files: %w", err)
	}

	server := grpc.NewServer()
	for _, file := range files {
		for _, svc := range file.GetServices() {
			registerDynamicService(server, svc, docSampleResponder)
		}
	}

	addr := fmt.Sprintf(":%d", port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	log.Printf("doc-sample gRPC server listening on %s", addr)
	return server.Serve(lis)
}

func registerDynamicService(server *grpc.Server, svc *desc.ServiceDescriptor, responder dynamicResponder) {
	serviceDesc := grpc.ServiceDesc{
		ServiceName: svc.GetFullyQualifiedName(),
		HandlerType: (*docSampleGRPCService)(nil),
	}
	handler := &docSampleGRPCHandler{}

	for _, method := range svc.GetMethods() {
		m := method
		serviceDesc.Methods = append(serviceDesc.Methods, grpc.MethodDesc{
			MethodName: m.GetName(),
			Handler: func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
				req := dynamic.NewMessage(m.GetInputType())
				if err := dec(req); err != nil {
					return nil, err
				}
				invoke := func(ctx context.Context, req interface{}) (interface{}, error) {
					return responder(ctx, m, req.(*dynamic.Message))
				}
				if interceptor == nil {
					return invoke(ctx, req)
				}
				info := &grpc.UnaryServerInfo{
					Server:     srv,
					FullMethod: fmt.Sprintf("/%s/%s", svc.GetFullyQualifiedName(), m.GetName()),
				}
				return interceptor(ctx, req, info, invoke)
			},
		})
	}

	server.RegisterService(&serviceDesc, handler)
}

type docSampleGRPCService interface{}

type docSampleGRPCHandler struct{}

func docSampleResponder(ctx context.Context, method *desc.MethodDescriptor, req *dynamic.Message) (*dynamic.Message, error) {
	resp := dynamic.NewMessage(method.GetOutputType())
	switch method.GetFullyQualifiedName() {
	case "myapp.UserService.GetUser":
		userID, _ := req.TryGetFieldByName("user_id")
		if userIDStr, ok := userID.(string); ok {
			_ = resp.TrySetFieldByName("user_id", userIDStr)
			_ = resp.TrySetFieldByName("name", fmt.Sprintf("User-%s", userIDStr))
		} else {
			_ = resp.TrySetFieldByName("name", "User")
		}
	case "users.UserService.UpdateUser":
		_ = resp.TrySetFieldByName("ok", true)
	case "orders.OrderService.CreateOrder":
		_ = resp.TrySetFieldByName("status", "CREATED")
	default:
		return nil, status.Errorf(codes.Unimplemented, "method %s not supported", method.GetFullyQualifiedName())
	}
	return resp, nil
}
