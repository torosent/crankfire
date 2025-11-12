#!/bin/bash

# Quick test script for dashboard functionality

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

./crankfire --target https://httpbin.org/get --concurrency 5 --total 30 --dashboard

echo ""
echo "Test completed! Check that:"
echo "  1. ✓ Dashboard displayed with live metrics"
echo "  2. ✓ RPS counter showed values"
echo "  3. ✓ Metrics table showed request counts"
echo "  4. ✓ Latency sparkline showed data"
echo "  5. ✓ Final report displayed after dashboard closed"
echo "  6. ✓ Process exited cleanly (you see this message)"
