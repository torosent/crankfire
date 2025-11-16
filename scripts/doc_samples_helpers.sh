#!/usr/bin/env bash

if [[ -z "${PROJECT_ROOT:-}" ]]; then
    echo "[doc-samples] PROJECT_ROOT must be set before sourcing this helper" >&2
    return 1 2>/dev/null || exit 1
fi

BUILD_DIR=${BUILD_DIR:-"$PROJECT_ROOT/build"}
DOC_SAMPLES_ROOT=${DOC_SAMPLES_ROOT:-"$PROJECT_ROOT/scripts/doc-samples"}
DOC_SAMPLE_SERVER_SRC="$PROJECT_ROOT/scripts/testservers/doc_sample_servers"
DOC_SAMPLE_SERVER_BIN="${DOC_SAMPLE_SERVER_BIN:-"$BUILD_DIR/doc_sample_servers"}"

mkdir -p "$BUILD_DIR"

find_free_port() {
    python3 - <<'PY'
import socket
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sock.bind(("127.0.0.1", 0))
port = sock.getsockname()[1]
sock.close()
print(port)
PY
}

wait_for_port() {
    local port="$1"
    local retries="${2:-40}"
    for ((i=0; i<retries; i++)); do
        if nc -z 127.0.0.1 "$port" >/dev/null 2>&1; then
            return 0
        fi
        sleep 0.25
    done
    return 1
}

ensure_doc_sample_server_binary() {
    (cd "$PROJECT_ROOT" && go build -o "$DOC_SAMPLE_SERVER_BIN" ./scripts/testservers/doc_sample_servers >/dev/null)
}

start_doc_sample_server() {
    local mode="$1"
    local port="$2"
    shift 2
    ensure_doc_sample_server_binary
    local log_file="$BUILD_DIR/doc-sample-${mode}.log"
    "$DOC_SAMPLE_SERVER_BIN" --mode "$mode" --port "$port" "$@" >"$log_file" 2>&1 &
    local pid=$!
    if ! wait_for_port "$port"; then
        echo "[doc-samples] failed to start $mode server on port $port" >&2
        kill "$pid" 2>/dev/null || true
        return 1
    fi
    echo "$pid"
}

start_doc_sample_http_server() {
    start_doc_sample_server "http" "$1"
}

start_doc_sample_sse_server() {
    start_doc_sample_server "sse" "$1"
}

start_doc_sample_ws_server() {
    start_doc_sample_server "websocket" "$1"
}

start_doc_sample_grpc_server() {
    local port="$1"
    start_doc_sample_server "grpc" "$port" --doc-samples "$DOC_SAMPLES_ROOT"
}

prepare_doc_sample_config() {
    local source_file="$1"
    shift
    local temp_file
    local ext="${source_file##*.}"
    temp_file=$(mktemp "$DOC_SAMPLES_ROOT/tmp.XXXXXX")
    local typed_file="${temp_file}.${ext}"
    mv "$temp_file" "$typed_file"
    cp "$source_file" "$typed_file"
    while [[ $# -gt 1 ]]; do
        local search="$1"
        local replace="$2"
        shift 2
        perl -0pi -e "s|\Q${search}\E|${replace}|g" "$typed_file"
    done
    echo "$typed_file"
}
