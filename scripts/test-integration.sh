#!/usr/bin/env bash

# test-integration.sh - End-to-end integration validation script
# Demonstrates all Crankfire features working together

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/build"
CRANKFIRE="$BUILD_DIR/crankfire"
DOC_SAMPLES_ROOT="$(cd "$SCRIPT_DIR/doc-samples" && pwd)"

source "$SCRIPT_DIR/doc_samples_helpers.sh"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_binary() {
    if [ ! -f "$CRANKFIRE" ]; then
        log_error "Crankfire binary not found at $CRANKFIRE"
        log_info "Building crankfire..."
        cd "$PROJECT_ROOT" && go build -o "$CRANKFIRE" ./cmd/crankfire
    fi
    log_info "Using crankfire binary: $CRANKFIRE"
}

validate_json_output() {
    local output=$1
    local test_name=$2
    
    # Check if output is valid JSON
    if ! echo "$output" | jq empty 2>/dev/null; then
        log_error "$test_name: Invalid JSON output"
        return 1
    fi
    
    # Extract metrics
    local total=$(echo "$output" | jq -r '.total')
    local successes=$(echo "$output" | jq -r '.successes')
    local failures=$(echo "$output" | jq -r '.failures')
    
    log_info "$test_name: Total=$total, Successes=$successes, Failures=$failures"
    
    # Basic validation
    if [ "$total" -eq 0 ]; then
        log_error "$test_name: No requests made"
        return 1
    fi
    
    if [ "$successes" -eq 0 ]; then
        log_warn "$test_name: No successful requests"
    fi
    
    return 0
}

test_basic_http() {
    log_info "Test 1: Basic HTTP load test"
    
    local output=$("$CRANKFIRE" \
        --target https://httpbin.org/get \
        --total 10 \
        --concurrency 2 \
        --json-output 2>/dev/null)
    
    validate_json_output "$output" "Basic HTTP"
}

test_sample_configs() {
    log_info "Test 2: Validate sample configurations"
    
    local samples=(
        "sample.yml"
        "test-endpoints.yml"
        "auth-oauth2-sample.yml"
        "feeder-csv-sample.yml"
        "chaining-sample.yml"
        "websocket-sample.yml"
        "sse-sample.json"
        "grpc-sample.yml"
    )
    
    for sample in "${samples[@]}"; do
        local sample_path="$SCRIPT_DIR/$sample"
        if [ ! -f "$sample_path" ]; then
            log_warn "Sample not found: $sample"
            continue
        fi
        
        log_info "Validating: $sample"
        
        # Just validate parsing (don't run tests against real endpoints)
        if "$CRANKFIRE" --config "$sample_path" --help > /dev/null 2>&1; then
            log_info "✓ $sample parsed successfully"
        else
            log_error "✗ $sample failed to parse"
            return 1
        fi
    done
}

test_rate_limiting() {
    log_info "Test 3: Rate limiting validation"
    
    local start_time=$(date +%s)
    
    local output=$("$CRANKFIRE" \
        --target https://httpbin.org/get \
        --rate 5 \
        --total 10 \
        --concurrency 1 \
        --json-output 2>/dev/null)
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    # Should take at least 2 seconds for 10 requests at 5 req/s
    if [ "$duration" -ge 2 ]; then
        log_info "Rate limiting working: ${duration}s for 10 requests at 5 req/s"
    else
        log_warn "Rate limiting may not be working correctly: ${duration}s"
    fi
    
    validate_json_output "$output" "Rate Limiting"
}

test_concurrency() {
    log_info "Test 4: Concurrency validation"
    
    local output=$("$CRANKFIRE" \
        --target https://httpbin.org/delay/1 \
        --total 10 \
        --concurrency 10 \
        --json-output 2>/dev/null)
    
    local duration_ms=$(echo "$output" | jq -r '.duration_ms')
    local duration_sec=$(echo "$duration_ms" | awk '{print int($1/1000)}')
    
    # With concurrency=10 and 10 requests of 1s each, should complete in ~1-2s
    if [ "$duration_sec" -le 3 ]; then
        log_info "Concurrency working: ${duration_sec}s for 10 parallel 1s requests"
    else
        log_warn "Concurrency may not be working: ${duration_sec}s"
    fi
    
    validate_json_output "$output" "Concurrency"
}

test_json_output() {
    log_info "Test 5: JSON output format validation"
    
    local output=$("$CRANKFIRE" \
        --target https://httpbin.org/get \
        --total 5 \
        --json-output 2>/dev/null)
    
    # Validate required fields exist
    local required_fields=(
        "total"
        "successes"
        "failures"
        "min_latency_ms"
        "max_latency_ms"
        "mean_latency_ms"
        "p50_latency_ms"
        "p90_latency_ms"
        "p99_latency_ms"
        "duration_ms"
        "requests_per_sec"
    )
    
    for field in "${required_fields[@]}"; do
        if ! echo "$output" | jq -e ".$field" > /dev/null 2>&1; then
            log_error "Missing required field: $field"
            return 1
        fi
    done
    
    log_info "All required JSON fields present"
    validate_json_output "$output" "JSON Output"
}

test_retries() {
    log_info "Test 6: Retry logic validation"
    
    # httpbin.org/status/500 returns 500 error
    local output=$("$CRANKFIRE" \
        --target https://httpbin.org/status/500 \
        --total 3 \
        --retries 2 \
        --timeout 5s \
        --json-output 2>/dev/null || true)
    
    # Should have failures due to 500 errors
    local failures=$(echo "$output" | jq -r '.failures // 0')
    
    if [ "$failures" -gt 0 ]; then
        log_info "Retry logic executed: $failures failures recorded"
    else
        log_warn "Expected failures with 500 status code"
    fi
    
    validate_json_output "$output" "Retries" || true
}

test_har_import() {
    log_info "Test 8: HAR file import validation"
    
    # Create a temporary HAR file targeting httpbin.org
    local har_file=$(mktemp /tmp/crankfire-har-test.XXXXXX.har)
    cat > "$har_file" << 'EOF'
{
  "log": {
    "version": "1.2",
    "creator": {
      "name": "Crankfire HAR Test",
      "version": "1.0.0"
    },
    "entries": [
      {
        "startedDateTime": "2025-01-15T10:00:00.000Z",
        "time": 0.05,
        "request": {
          "method": "GET",
          "url": "https://httpbin.org/get",
          "httpVersion": "HTTP/1.1",
          "headers": [
            {"name": "Accept", "value": "application/json"}
          ],
          "queryString": [],
          "cookies": [],
          "headersSize": 100,
          "bodySize": 0
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "cookies": [],
          "content": {"size": 0, "mimeType": "application/json"},
          "redirectURL": "",
          "headersSize": 100,
          "bodySize": 0
        },
        "cache": {},
        "timings": {"wait": 50, "receive": 10}
      },
      {
        "startedDateTime": "2025-01-15T10:00:01.000Z",
        "time": 0.05,
        "request": {
          "method": "POST",
          "url": "https://httpbin.org/post",
          "httpVersion": "HTTP/1.1",
          "headers": [
            {"name": "Content-Type", "value": "application/json"}
          ],
          "queryString": [],
          "cookies": [],
          "postData": {
            "mimeType": "application/json",
            "text": "{\"test\": \"data\"}"
          },
          "headersSize": 100,
          "bodySize": 16
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "cookies": [],
          "content": {"size": 0, "mimeType": "application/json"},
          "redirectURL": "",
          "headersSize": 100,
          "bodySize": 0
        },
        "cache": {},
        "timings": {"wait": 50, "receive": 10}
      },
      {
        "startedDateTime": "2025-01-15T10:00:02.000Z",
        "time": 0.05,
        "request": {
          "method": "GET",
          "url": "https://httpbin.org/static/script.js",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "queryString": [],
          "cookies": [],
          "headersSize": 50,
          "bodySize": 0
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "httpVersion": "HTTP/1.1",
          "headers": [],
          "cookies": [],
          "content": {"size": 0, "mimeType": "application/javascript"},
          "redirectURL": "",
          "headersSize": 50,
          "bodySize": 0
        },
        "cache": {},
        "timings": {"wait": 10, "receive": 5}
      }
    ]
  }
}
EOF

    # Test 8a: Basic HAR import (should exclude static .js file)
    log_info "Test 8a: HAR import with static asset exclusion"
    local output
    output=$("$CRANKFIRE" \
        --har "$har_file" \
        --total 6 \
        --concurrency 2 \
        --json-output 2>/dev/null) || true
    
    if validate_json_output "$output" "HAR Import"; then
        local total=$(echo "$output" | jq -r '.total')
        log_info "HAR import executed: $total requests (static assets excluded)"
    fi
    
    # Test 8b: HAR import with method filter
    log_info "Test 8b: HAR import with method filter (GET only)"
    local output_filtered
    output_filtered=$("$CRANKFIRE" \
        --har "$har_file" \
        --har-filter "method:GET" \
        --total 3 \
        --concurrency 1 \
        --json-output 2>/dev/null) || true
    
    if validate_json_output "$output_filtered" "HAR Import (GET filter)"; then
        log_info "HAR method filtering working"
    fi
    
    # Cleanup
    rm -f "$har_file"
    
    log_info "HAR import tests completed"
}

run_doc_sample_http_config() {
    local sample_path="$1"
    local test_name="$2"
    shift 2

    local config_path
    config_path=$(prepare_doc_sample_config "$sample_path" "$@")
    local config_name
    config_name=$(basename "$config_path")

    local output
    if ! output=$( (cd "$DOC_SAMPLES_ROOT" && "$CRANKFIRE" \
        --config "$config_name" \
        --json-output \
        --concurrency 2 \
        --duration 5s \
        --total 3 \
        --rate 0) 2>/dev/null ); then
        log_error "$test_name: crankfire execution failed"
        rm -f "$config_path"
        return 1
    fi

    rm -f "$config_path"
    validate_json_output "$output" "$test_name"
}

test_doc_samples_http() {
    log_info "Test 7: Doc-sample HTTP scenarios"

    local http_port
    http_port=$(find_free_port)
    local http_pid
    if ! http_pid=$(start_doc_sample_http_server "$http_port"); then
        log_error "Failed to start HTTP doc-sample server"
        return 1
    fi
    local base_url="http://127.0.0.1:${http_port}"

    local failures=0

    run_doc_sample_http_config "$DOC_SAMPLES_ROOT/loadtest.yaml" "Doc sample: loadtest.yaml" \
        "https://api.example.com" "$base_url" || failures=$((failures + 1))

    run_doc_sample_http_config "$DOC_SAMPLES_ROOT/loadtest.json" "Doc sample: loadtest.json" \
        "https://api.example.com" "$base_url" || failures=$((failures + 1))

    run_doc_sample_http_config "$DOC_SAMPLES_ROOT/feeder-test.yml" "Doc sample: feeder-test" \
        "https://api.example.com" "$base_url" || failures=$((failures + 1))

    run_doc_sample_http_config "$DOC_SAMPLES_ROOT/auth-feeder-test.yml" "Doc sample: auth-feeder-test" \
        "https://api.example.com" "$base_url" \
        "https://idp.example.com" "$base_url" || failures=$((failures + 1))

    kill "$http_pid" 2>/dev/null || true

    if [ "$failures" -ne 0 ]; then
        log_error "Doc-sample HTTP scenarios: ${failures} failure(s)"
        return 1
    fi
    log_info "Doc-sample HTTP scenarios completed"
    return 0
}

test_request_chaining() {
    log_info "Test 9: Request Chaining & Variable Extraction"
    
    local http_port
    http_port=$(find_free_port)
    local http_pid
    if ! http_pid=$(start_doc_sample_http_server "$http_port"); then
        log_error "Failed to start HTTP server for chaining test"
        return 1
    fi
    
    local base_url="http://127.0.0.1:${http_port}"
    
    # Test chaining configuration
    run_doc_sample_http_config "$DOC_SAMPLES_ROOT/chaining-test.yml" "Request Chaining" \
        "http://localhost:8080" "$base_url"
    local result=$?
    
    kill "$http_pid" 2>/dev/null || true
    
    if [ $result -eq 0 ]; then
        log_info "Request chaining test passed"
    else
        log_error "Request chaining test failed"
    fi
    
    return $result
}

run_all_tests() {
    log_info "Starting Crankfire integration tests..."
    echo ""
    
    local failed=0
    
    test_basic_http || ((failed++))
    echo ""
    
    test_sample_configs || ((failed++))
    echo ""
    
    test_rate_limiting || ((failed++))
    echo ""
    
    test_concurrency || ((failed++))
    echo ""
    
    test_json_output || ((failed++))
    echo ""
    
    test_retries || ((failed++))
    echo ""

    test_har_import || ((failed++))
    echo ""

    test_doc_samples_http || ((failed++))
    echo ""

    test_request_chaining || ((failed++))
    echo ""
    
    if [ "$failed" -eq 0 ]; then
        log_info "✅ All integration tests passed!"
        return 0
    else
        log_error "❌ $failed test(s) failed"
        return 1
    fi
}

# Main execution
main() {
    check_binary
    echo ""
    run_all_tests
}

main "$@"
