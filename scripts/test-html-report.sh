#!/bin/bash
set -e

# Build the binary
echo "Building crankfire..."
go build -o build/crankfire ./cmd/crankfire

# Run a test and generate HTML report
echo "Running load test and generating HTML report..."
./build/crankfire \
  --target https://httpbin.org/get \
  --concurrency 5 \
  --total 100 \
  --html-output report.html

echo "Report generated: report.html"

# Run a multi-endpoint test
echo "Running multi-endpoint load test..."
cat <<EOF > multi_endpoint_test.json
{
  "concurrency": 5,
  "total": 50,
  "endpoints": [
    {
      "name": "Get Data",
      "url": "https://httpbin.org/get",
      "method": "GET"
    },
    {
      "name": "Post Data",
      "url": "https://httpbin.org/post",
      "method": "POST",
      "body": "{\"test\": \"data\"}"
    }
  ]
}
EOF

./build/crankfire \
  --config multi_endpoint_test.json \
  --html-output report-multi.html

echo "Report generated: report-multi.html"
rm multi_endpoint_test.json

echo "Opening reports..."

# Open the report based on OS
if [[ "$OSTYPE" == "darwin"* ]]; then
  open report.html report-multi.html
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
  xdg-open report.html
  xdg-open report-multi.html
else
  echo "Please open report.html and report-multi.html in your browser."
fi
