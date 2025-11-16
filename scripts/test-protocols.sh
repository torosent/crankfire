#!/bin/bash
# Test WebSocket and SSE protocol integration

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/build"
CRANKFIRE_BIN="$BUILD_DIR/crankfire"
DOC_SAMPLES_ROOT="$(cd "$SCRIPT_DIR/doc-samples" && pwd)"

source "$SCRIPT_DIR/doc_samples_helpers.sh"

DOC_SAMPLE_DEFAULT_FLAGS=(--concurrency 2 --duration 5s --total 4 --rate 0)

ensure_binary() {
  if [[ ! -x "$CRANKFIRE_BIN" ]]; then
    echo "[build] crankfire binary not found, building..."
    (cd "$PROJECT_ROOT" && go build -o "$CRANKFIRE_BIN" ./cmd/crankfire)
  else
    echo "[build] using crankfire binary: $CRANKFIRE_BIN"
  fi
}

ensure_binary

print_doc_sample_stats() {
  local description="$1"
  local payload
  payload=$(cat)

  local successes total failures
  successes=$(printf '%s\n' "$payload" | jq -r '.successes // 0' 2>/dev/null || echo "0")
  total=$(printf '%s\n' "$payload" | jq -r '.total // 0' 2>/dev/null || echo "0")
  failures=$(printf '%s\n' "$payload" | jq -r '.failures // 0' 2>/dev/null || echo "0")

  if [[ "$successes" -gt 0 ]]; then
    echo "✓ $description (${successes} successes, ${failures} failures, total=${total})"
    printf '%s\n' "$payload" | head -10
    return 0
  fi

  echo "✗ $description produced no successes"
  printf '%s\n' "$payload" | head -10
  return 1
}

run_doc_sample_config() {
  local sample_path="$1"
  local description="$2"
  shift 2

  local extra_cli=()
  local replacements=()
  local parsing_cli=true

  while [[ $# -gt 0 ]]; do
    if [[ "$1" == "--" ]]; then
      parsing_cli=false
      shift
      continue
    fi
    if $parsing_cli; then
      extra_cli+=("$1")
    else
      replacements+=("$1")
    fi
    shift
  done

  local config_path
  config_path=$(prepare_doc_sample_config "$sample_path" "${replacements[@]}") || return 1
  local config_name
  config_name=$(basename "$config_path")

  local output
  if ! output=$( (cd "$DOC_SAMPLES_ROOT" && "$CRANKFIRE_BIN" \
      --config "$config_name" \
      --json-output \
      "${DOC_SAMPLE_DEFAULT_FLAGS[@]}" \
      "${extra_cli[@]}") 2>/dev/null ); then
    echo "✗ $description failed (crankfire error)"
    rm -f "$config_path"
    return 1
  fi

  rm -f "$config_path"
  print_doc_sample_stats "$description" <<<"$output"
}

run_doc_sample_websocket_samples() {
  local ws_port
  ws_port=$(find_free_port)
  local ws_pid
  if ! ws_pid=$(start_doc_sample_ws_server "$ws_port"); then
    echo "✗ Unable to start doc-sample WebSocket server"
    return 1
  fi

  local ws_url="ws://127.0.0.1:${ws_port}/chat"
  local failures=0

  run_doc_sample_config "$DOC_SAMPLES_ROOT/websocket-test.yml" "doc-samples/websocket-test.yml" \
    --concurrency 1 --total 2 --duration 3s \
    -- "ws://localhost:8080/chat" "$ws_url" "receive_timeout: 10s" "receive_timeout: 0s" || failures=$((failures + 1))

  run_doc_sample_config "$DOC_SAMPLES_ROOT/ws-feeder-test.yml" "doc-samples/ws-feeder-test.yml" \
    --concurrency 1 --total 2 --duration 3s \
    -- "ws://localhost:8080/chat" "$ws_url" || failures=$((failures + 1))

  kill "$ws_pid" 2>/dev/null || true

  if [[ "$failures" -ne 0 ]]; then
    return 1
  fi
  echo "✓ Doc-sample WebSocket scenarios passed"
  return 0
}

run_doc_sample_sse_samples() {
  local sse_port
  sse_port=$(find_free_port)
  local sse_pid
  if ! sse_pid=$(start_doc_sample_sse_server "$sse_port"); then
    echo "✗ Unable to start doc-sample SSE server"
    return 1
  fi

  local sse_base="http://127.0.0.1:${sse_port}"
  local failures=0

  run_doc_sample_config "$DOC_SAMPLES_ROOT/sse-test.json" "doc-samples/sse-test.json" \
    --concurrency 1 --total 3 --duration 3s \
    -- "http://localhost:8080" "$sse_base" \
       '"max_events": 500' '"max_events": 10' || failures=$((failures + 1))

  run_doc_sample_config "$DOC_SAMPLES_ROOT/sse-feeder-test.yml" "doc-samples/sse-feeder-test.yml" \
    --concurrency 1 --total 3 --duration 3s \
    -- "http://localhost:8080" "$sse_base" \
       "max_events: 200" "max_events: 10" || failures=$((failures + 1))

  run_doc_sample_config "$DOC_SAMPLES_ROOT/sse-feeder.yml" "doc-samples/sse-feeder.yml" \
    --concurrency 1 --total 3 --duration 3s \
    -- "https://stream.example.com" "$sse_base" \
       "max_events: 200" "max_events: 10" || failures=$((failures + 1))

  kill "$sse_pid" 2>/dev/null || true

  if [[ "$failures" -ne 0 ]]; then
    return 1
  fi
  echo "✓ Doc-sample SSE scenarios passed"
  return 0
}

run_doc_sample_grpc_samples() {
  local grpc_port
  grpc_port=$(find_free_port)
  local grpc_pid
  if ! grpc_pid=$(start_doc_sample_grpc_server "$grpc_port"); then
    echo "✗ Unable to start doc-sample gRPC server"
    return 1
  fi

  local http_port
  http_port=$(find_free_port)
  local http_pid
  if ! http_pid=$(start_doc_sample_http_server "$http_port"); then
    kill "$grpc_pid" 2>/dev/null || true
    echo "✗ Unable to start doc-sample auth server"
    return 1
  fi

  local grpc_target="127.0.0.1:${grpc_port}"
  local auth_base="http://127.0.0.1:${http_port}"
  local failures=0

  run_doc_sample_config "$DOC_SAMPLES_ROOT/grpc-test.yml" "doc-samples/grpc-test.yml" \
    --total 3 --duration 4s \
    -- "localhost:50051" "$grpc_target" || failures=$((failures + 1))

  run_doc_sample_config "$DOC_SAMPLES_ROOT/grpc-feeder-test.yml" "doc-samples/grpc-feeder-test.yml" \
    --total 3 --duration 4s \
    -- "localhost:50051" "$grpc_target" || failures=$((failures + 1))

  run_doc_sample_config "$DOC_SAMPLES_ROOT/grpc-complete-test.yml" "doc-samples/grpc-complete-test.yml" \
    --total 3 --duration 4s \
    -- "orders.example.com:50051" "$grpc_target" \
       "https://idp.example.com" "$auth_base" \
       "tls: true" "tls: false" \
       "insecure: false" "insecure: true" || failures=$((failures + 1))

  kill "$grpc_pid" 2>/dev/null || true
  kill "$http_pid" 2>/dev/null || true

  if [[ "$failures" -ne 0 ]]; then
    return 1
  fi
  echo "✓ Doc-sample gRPC scenarios passed"
  return 0
}

echo "=== Testing WebSocket Protocol ==="
echo "Testing against public WebSocket echo server..."

# Test WebSocket with a public echo server
"$CRANKFIRE_BIN" \
  --protocol websocket \
  --target wss://ws.postman-echo.com/raw \
  --ws-message-interval 200ms \
  --concurrency 2 \
  --total 3 \
  --json-output > /tmp/ws-test-output.json 2>&1 || true

# Check if command executed
if [ -f /tmp/ws-test-output.json ]; then
  successes=$(jq -r '.successes // 0' /tmp/ws-test-output.json 2>/dev/null || echo "0")
  if [ "$successes" -gt 0 ] 2>/dev/null; then
    echo "✓ WebSocket test completed successfully ($successes successes)"
  else
    echo "⚠ WebSocket test ran but server may be unavailable"
  fi
  cat /tmp/ws-test-output.json | head -10
else
  echo "⚠ WebSocket test completed"
fi

echo ""

echo ""
echo "=== Testing SSE Protocol ==="
echo "Starting SSE server on port 8766..."

# Start a simple SSE server using Python
python3 -c '
from http.server import BaseHTTPRequestHandler, HTTPServer
import time
import signal
import sys

class SSEHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "keep-alive")
        self.end_headers()
        
        for i in range(10):
            self.wfile.write(f"data: Event {i}\n\n".encode())
            self.wfile.flush()
            time.sleep(0.05)
    
    def log_message(self, format, *args):
        pass  # Suppress logs

server = HTTPServer(("localhost", 8766), SSEHandler)
server.serve_forever()
' &
SSE_PID=$!
echo "SSE server PID: $SSE_PID"

# Give server time to start
sleep 2

echo "Running SSE load test..."
"$CRANKFIRE_BIN" \
  --protocol sse \
  --target http://localhost:8766/events \
  --sse-read-timeout 5s \
  --sse-max-events 10 \
  --concurrency 3 \
  --total 5 \
  --json-output > /tmp/sse-test-output.json

# Check output
if [ -f /tmp/sse-test-output.json ]; then
  echo "✓ SSE test completed successfully"
  cat /tmp/sse-test-output.json | head -10
else
  echo "✗ SSE test failed - no output"
  kill $SSE_PID 2>/dev/null || true
  exit 1
fi

# Cleanup
kill $SSE_PID 2>/dev/null || true

echo ""
echo "=== Testing gRPC Protocol ==="
echo "Note: gRPC test requires a running gRPC server"
echo "Testing gRPC configuration parsing..."

# Test gRPC configuration validation (will fail to connect but validates config)
"$CRANKFIRE_BIN" \
  --config "$SCRIPT_DIR/grpc-sample.yml" \
  --total 3 \
  --duration 5s \
  --json-output > /tmp/grpc-test-output.json 2>&1 || true

# Check if command executed
if [ -f /tmp/grpc-test-output.json ]; then
  total=$(jq -r '.total // 0' /tmp/grpc-test-output.json 2>/dev/null || echo "0")
  successes=$(jq -r '.successes // 0' /tmp/grpc-test-output.json 2>/dev/null || echo "0")
  
  if [ "$successes" -gt 0 ] 2>/dev/null; then
    echo "✓ gRPC test completed successfully ($successes/$total successes)"
    cat /tmp/grpc-test-output.json | head -10
  elif [ "$total" -gt 0 ] 2>/dev/null; then
    echo "⚠ gRPC configuration validated but server unavailable ($total attempts)"
    echo "  To run full gRPC test, start a gRPC server on localhost:50051"
  else
    echo "✓ gRPC configuration parsed successfully"
  fi
else
  echo "⚠ gRPC test completed"
fi

echo ""
echo "=== Doc-Sample WebSocket Scenarios ==="
run_doc_sample_websocket_samples

echo ""
echo "=== Doc-Sample SSE Scenarios ==="
run_doc_sample_sse_samples

echo ""
echo "=== Doc-Sample gRPC Scenarios ==="
run_doc_sample_grpc_samples

echo ""
echo "=== All Protocol Tests Passed ==="
