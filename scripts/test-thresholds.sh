#!/bin/bash

# Example CI/CD script demonstrating threshold usage

echo "=== Running Performance Tests with Thresholds ==="

# Test 1: Basic latency threshold
echo ""
echo "Test 1: Basic latency test (should pass)"
./build/crankfire \
  --target https://httpbin.org/status/200 \
  --total 10 \
  --concurrency 2 \
  --threshold "http_req_duration:p99 < 5000" \
  --threshold "http_req_failed:rate < 0.5"

if [ $? -eq 0 ]; then
  echo "✓ Test 1 passed"
else
  echo "✗ Test 1 failed"
  exit 1
fi

# Test 2: Multiple thresholds with config file
echo ""
echo "Test 2: Config file with multiple thresholds"
./build/crankfire --config scripts/threshold-example.yml

if [ $? -eq 0 ]; then
  echo "✓ Test 2 passed"
else
  echo "✗ Test 2 failed"
  exit 1
fi

# Test 3: JSON output with thresholds
echo ""
echo "Test 3: JSON output format"
./build/crankfire \
  --target https://httpbin.org/status/200 \
  --total 5 \
  --threshold "http_req_duration:p95 < 5000" \
  --json-output > /tmp/perf-results.json

if [ $? -eq 0 ]; then
  echo "✓ Test 3 passed"
  echo "Results saved to /tmp/perf-results.json"
  cat /tmp/perf-results.json | grep -o '"passed":[0-9]*' || true
else
  echo "✗ Test 3 failed"
  exit 1
fi

echo ""
echo "=== All Performance Tests Passed ==="
