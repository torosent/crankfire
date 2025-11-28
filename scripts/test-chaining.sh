#!/usr/bin/env bash

# test-chaining.sh - Integration tests for request chaining and variable extraction
# Tests extracting values from responses and using them in subsequent requests

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/build"
CRANKFIRE="$BUILD_DIR/crankfire"
DOC_SAMPLES_ROOT="$SCRIPT_DIR/doc-samples"

source "$SCRIPT_DIR/doc_samples_helpers.sh"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

check_binary() {
    if [ ! -f "$CRANKFIRE" ]; then
        log_info "Building crankfire..."
        cd "$PROJECT_ROOT" && go build -o "$CRANKFIRE" ./cmd/crankfire
    fi
    log_info "Using crankfire binary: $CRANKFIRE"
}

validate_json_output() {
    local output=$1
    local test_name=$2
    
    if ! echo "$output" | jq empty 2>/dev/null; then
        log_error "$test_name: Invalid JSON output"
        echo "$output"
        return 1
    fi
    
    local total=$(echo "$output" | jq -r '.total')
    local successes=$(echo "$output" | jq -r '.successes')
    local failures=$(echo "$output" | jq -r '.failures')
    
    log_info "$test_name: Total=$total, Successes=$successes, Failures=$failures"
    
    if [ "$total" -eq 0 ]; then
        log_error "$test_name: No requests made"
        return 1
    fi
    
    return 0
}

run_chaining_config() {
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
        --total 10 \
        --rate 0) 2>&1 ); then
        log_error "$test_name: crankfire execution failed"
        echo "$output"
        rm -f "$config_path"
        return 1
    fi

    rm -f "$config_path"
    validate_json_output "$output" "$test_name"
}

test_basic_chaining() {
    log_info "=== Test 1: Basic Request Chaining ==="
    log_info "Testing extraction of user_id and auth_token, then using them in subsequent requests"
    
    local http_port
    http_port=$(find_free_port)
    local http_pid
    if ! http_pid=$(start_doc_sample_http_server "$http_port"); then
        log_error "Failed to start HTTP server"
        return 1
    fi
    
    local base_url="http://127.0.0.1:${http_port}"
    
    run_chaining_config "$DOC_SAMPLES_ROOT/chaining-test.yml" "Basic Chaining" \
        "http://localhost:8080" "$base_url"
    local result=$?
    
    kill "$http_pid" 2>/dev/null || true
    
    if [ $result -eq 0 ]; then
        log_info "✓ Basic request chaining test passed"
    else
        log_error "✗ Basic request chaining test failed"
    fi
    
    return $result
}

test_chaining_with_defaults() {
    log_info "=== Test 2: Request Chaining with Default Values ==="
    log_info "Testing {{var|default}} syntax when variables are not yet extracted"
    
    local http_port
    http_port=$(find_free_port)
    local http_pid
    if ! http_pid=$(start_doc_sample_http_server "$http_port"); then
        log_error "Failed to start HTTP server"
        return 1
    fi
    
    local base_url="http://127.0.0.1:${http_port}"
    
    # Create a config that tests default values heavily
    local config_file=$(mktemp "$DOC_SAMPLES_ROOT/tmp.XXXXXX.yml")
    cat > "$config_file" << EOF
target: $base_url

endpoints:
  # This endpoint uses defaults since no extraction has happened yet
  - name: get-with-defaults
    path: /api/orders/{{order_id|fallback-order}}
    method: GET
    weight: 5
    headers:
      Authorization: "Bearer {{token|no-auth}}"
      X-User: "{{user_name|anonymous}}"
EOF
    
    local config_name
    config_name=$(basename "$config_file")
    
    local output
    if ! output=$( (cd "$DOC_SAMPLES_ROOT" && "$CRANKFIRE" \
        --config "$config_name" \
        --json-output \
        --concurrency 1 \
        --total 5) 2>&1 ); then
        log_error "Default values test: crankfire execution failed"
        echo "$output"
        rm -f "$config_file"
        kill "$http_pid" 2>/dev/null || true
        return 1
    fi
    
    rm -f "$config_file"
    kill "$http_pid" 2>/dev/null || true
    
    validate_json_output "$output" "Chaining with Defaults"
    local result=$?
    
    if [ $result -eq 0 ]; then
        log_info "✓ Default values test passed"
    else
        log_error "✗ Default values test failed"
    fi
    
    return $result
}

test_multi_step_workflow() {
    log_info "=== Test 3: Multi-Step Workflow ==="
    log_info "Testing complete workflow: create user -> create order -> get order -> update order"
    
    local http_port
    http_port=$(find_free_port)
    local http_pid
    if ! http_pid=$(start_doc_sample_http_server "$http_port"); then
        log_error "Failed to start HTTP server"
        return 1
    fi
    
    local base_url="http://127.0.0.1:${http_port}"
    
    # Create a multi-step workflow config
    local config_file=$(mktemp "$DOC_SAMPLES_ROOT/tmp.XXXXXX.yml")
    cat > "$config_file" << EOF
target: $base_url

endpoints:
  # Step 1: Create user
  - name: step1-create-user
    path: /api/users
    method: POST
    weight: 100
    body: '{"name": "Workflow User"}'
    headers:
      Content-Type: application/json
    extractors:
      - jsonpath: "id"
        var: user_id
      - jsonpath: "auth.token"
        var: token

  # Step 2: Create order with user context
  - name: step2-create-order
    path: /api/orders
    method: POST
    weight: 80
    body: '{"product": "Test Product"}'
    headers:
      Content-Type: application/json
      X-User-ID: "{{user_id|unknown}}"
      Authorization: "Bearer {{token|none}}"
    extractors:
      - jsonpath: "id"
        var: order_id
      - jsonpath: "total"
        var: total

  # Step 3: Get order details
  - name: step3-get-order
    path: /api/orders/{{order_id|pending}}
    method: GET
    weight: 60
    headers:
      Authorization: "Bearer {{token|none}}"
    extractors:
      - jsonpath: "status"
        var: status

  # Step 4: Update order
  - name: step4-update-order
    path: /api/orders/{{order_id|pending}}
    method: PATCH
    weight: 40
    body: '{"status": "confirmed"}'
    headers:
      Content-Type: application/json
      Authorization: "Bearer {{token|none}}"
EOF

    local config_name
    config_name=$(basename "$config_file")
    
    local output
    if ! output=$( (cd "$DOC_SAMPLES_ROOT" && "$CRANKFIRE" \
        --config "$config_name" \
        --json-output \
        --concurrency 3 \
        --duration 8s \
        --total 20) 2>&1 ); then
        log_error "Multi-step workflow: crankfire execution failed"
        echo "$output"
        rm -f "$config_file"
        kill "$http_pid" 2>/dev/null || true
        return 1
    fi
    
    rm -f "$config_file"
    kill "$http_pid" 2>/dev/null || true
    
    validate_json_output "$output" "Multi-Step Workflow"
    local result=$?
    
    # Check endpoint breakdown if available
    local endpoints=$(echo "$output" | jq -r '.endpoints // empty' 2>/dev/null)
    if [ -n "$endpoints" ] && [ "$endpoints" != "null" ]; then
        log_info "Endpoint breakdown:"
        echo "$output" | jq '.endpoints' 2>/dev/null || true
    fi
    
    if [ $result -eq 0 ]; then
        log_info "✓ Multi-step workflow test passed"
    else
        log_error "✗ Multi-step workflow test failed"
    fi
    
    return $result
}

test_regex_extraction() {
    log_info "=== Test 4: Regex Extraction ==="
    log_info "Testing regex-based value extraction from responses"
    
    local http_port
    http_port=$(find_free_port)
    local http_pid
    if ! http_pid=$(start_doc_sample_http_server "$http_port"); then
        log_error "Failed to start HTTP server"
        return 1
    fi
    
    local base_url="http://127.0.0.1:${http_port}"
    
    # Create a config that uses regex extraction
    local config_file=$(mktemp "$DOC_SAMPLES_ROOT/tmp.XXXXXX.yml")
    cat > "$config_file" << EOF
target: $base_url

endpoints:
  - name: create-and-extract
    path: /api/users
    method: POST
    weight: 10
    body: '{"name": "Regex Test"}'
    headers:
      Content-Type: application/json
    extractors:
      # JSON path extraction
      - jsonpath: "id"
        var: user_id
      # Regex extraction - extract user number from id like "user-123"
      - regex: 'user-(\d+)'
        var: user_number

  - name: use-extracted
    path: /api/orders
    method: POST
    weight: 5
    body: '{"user_number": "{{user_number|0}}"}'
    headers:
      Content-Type: application/json
      X-User-ID: "{{user_id|unknown}}"
EOF

    local config_name
    config_name=$(basename "$config_file")
    
    local output
    if ! output=$( (cd "$DOC_SAMPLES_ROOT" && "$CRANKFIRE" \
        --config "$config_name" \
        --json-output \
        --concurrency 2 \
        --total 10) 2>&1 ); then
        log_error "Regex extraction: crankfire execution failed"
        echo "$output"
        rm -f "$config_file"
        kill "$http_pid" 2>/dev/null || true
        return 1
    fi
    
    rm -f "$config_file"
    kill "$http_pid" 2>/dev/null || true
    
    validate_json_output "$output" "Regex Extraction"
    local result=$?
    
    if [ $result -eq 0 ]; then
        log_info "✓ Regex extraction test passed"
    else
        log_error "✗ Regex extraction test failed"
    fi
    
    return $result
}

test_sample_config_parsing() {
    log_info "=== Test 5: Sample Configuration Parsing ==="
    log_info "Validating chaining-sample.yml parses correctly"
    
    if "$CRANKFIRE" --config "$SCRIPT_DIR/chaining-sample.yml" --help > /dev/null 2>&1; then
        log_info "✓ chaining-sample.yml parsed successfully"
        return 0
    else
        log_error "✗ chaining-sample.yml failed to parse"
        return 1
    fi
}

run_all_tests() {
    log_info "=========================================="
    log_info "Request Chaining Integration Tests"
    log_info "=========================================="
    echo ""
    
    local failed=0
    
    test_sample_config_parsing || ((failed++))
    echo ""
    
    test_basic_chaining || ((failed++))
    echo ""
    
    test_chaining_with_defaults || ((failed++))
    echo ""
    
    test_multi_step_workflow || ((failed++))
    echo ""
    
    test_regex_extraction || ((failed++))
    echo ""
    
    log_info "=========================================="
    if [ "$failed" -eq 0 ]; then
        log_info "✅ All request chaining tests passed!"
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
