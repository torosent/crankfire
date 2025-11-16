#!/bin/bash

# Quick test script for dashboard functionality

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/build"
CRANKFIRE_BIN="$BUILD_DIR/crankfire"

ensure_binary() {
	if [[ ! -x "$CRANKFIRE_BIN" ]]; then
		echo "[build] crankfire binary not found, building..."
		(cd "$PROJECT_ROOT" && go build -o "$CRANKFIRE_BIN" ./cmd/crankfire)
	else
		echo "[build] using crankfire binary: $CRANKFIRE_BIN"
	fi
}

ensure_binary

echo "Testing Dashboard with httpbin.org..."
echo "Press 'q' or Ctrl+C to exit the dashboard"
echo ""
echo "This test will:"
echo "  - Send 30 requests"
echo "  - Use 5 concurrent workers"
echo "  - Show the live dashboard"
echo ""
echo "Starting in 3 seconds..."
sleep 3

"$CRANKFIRE_BIN" --target https://httpbin.org/get --concurrency 5 --total 30 --dashboard

echo ""
echo "Test completed! Check that:"
echo "  1. ✓ Dashboard displayed with live metrics"
echo "  2. ✓ RPS counter showed values"
echo "  3. ✓ Metrics table showed request counts"
echo "  4. ✓ Latency sparkline showed data"
echo "  5. ✓ Final report displayed after dashboard closed"
echo "  6. ✓ Process exited cleanly (you see this message)"
