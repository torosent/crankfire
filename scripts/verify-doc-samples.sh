#!/usr/bin/env bash

# verify-doc-samples.sh - build crankfire (if needed) and validate each doc sample config

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/build"
CRANKFIRE_BIN="$BUILD_DIR/crankfire"
DOC_SAMPLE_DIR="$SCRIPT_DIR/doc-samples"

log() {
  printf '%s\n' "$1"
}

ensure_binary() {
  if [[ ! -x "$CRANKFIRE_BIN" ]]; then
    log "[build] crankfire binary not found, building..."
    (cd "$PROJECT_ROOT" && go build -o "$CRANKFIRE_BIN" ./cmd/crankfire)
  else
    log "[build] using existing crankfire binary at $CRANKFIRE_BIN"
  fi
}

verify_samples() {
  local configs=(
    loadtest.json
    loadtest.yaml
    feeder-test.yml
    websocket-test.yml
    ws-feeder-test.yml
    sse-test.json
    sse-feeder-test.yml
    sse-feeder.yml
    grpc-test.yml
    grpc-feeder-test.yml
    auth-feeder-test.yml
    grpc-complete-test.yml
  )

  local required_assets=(
    payload.json
    products.json
    users.csv
    topics.csv
    orders.csv
    subscriptions.json
    updates.csv
    search-queries.csv
  )

  for asset in "${required_assets[@]}"; do
    if [[ ! -f "$DOC_SAMPLE_DIR/$asset" ]]; then
      log "[error] Required sample asset missing: $asset"
      exit 1
    fi
  done

  log "[info] validating doc sample configs (parse only)"
  pushd "$DOC_SAMPLE_DIR" > /dev/null
  local failed=0
  for cfg in "${configs[@]}"; do
    if [[ ! -f "$cfg" ]]; then
      log "[error] missing config: $cfg"
      failed=1
      continue
    fi

    log "  - $cfg"
    if "$CRANKFIRE_BIN" --config "$cfg" --help > /dev/null 2>&1; then
      log "    ✓ parsed successfully"
    else
      log "    ✗ failed to parse"
      failed=1
    fi
  done
  popd > /dev/null

  if [[ $failed -ne 0 ]]; then
    log "[error] one or more doc samples failed validation"
    exit 1
  fi

  log "[info] all doc sample configs parsed successfully"
}

main() {
  if [[ ! -d "$DOC_SAMPLE_DIR" ]]; then
    log "[error] doc sample directory not found: $DOC_SAMPLE_DIR"
    exit 1
  fi

  ensure_binary
  verify_samples
}

main "$@"
