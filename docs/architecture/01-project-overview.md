# Project Overview

## What is Crankfire?

Crankfire is a high-signal load testing tool for HTTP, WebSocket, SSE (Server-Sent Events), and gRPC protocols, designed to run from the command line. It enables engineers to describe realistic workloads against these protocols using a cohesive configuration model, with a focus on proper arrival modeling, protocol-aware metrics, and tight CI/CD integration.

Unlike heavyweight load-testing platforms that require clusters or web UIs, Crankfire is distributed as a single binary with minimal runtime dependencies, making it ideal for quick performance testing, stress testing, and automated performance regression testing in CI/CD pipelines.

## Core Purpose

1. **Multi-protocol load testing** - Test HTTP, WebSocket, SSE, and gRPC endpoints with unified configuration and reporting
2. **Production-grade metrics** - HDR histogram percentiles (P50/P90/P95/P99), per-endpoint statistics, and protocol-specific error categorization
3. **Realistic traffic simulation** - Support for ramp/step/spike load patterns with uniform or Poisson arrival models
4. **CI/CD integration** - JSON output with threshold-based pass/fail criteria for automated performance gates
5. **Developer productivity** - Live terminal dashboard, HTML reports, HAR import, and request chaining

## Technology Stack

| Category | Technology |
|----------|------------|
| **Language** | Go 1.24+ |
| **CLI Framework** | Cobra + Viper |
| **Metrics** | HdrHistogram-go |
| **Terminal UI** | Gizak termui/v3 |
| **WebSocket** | Gorilla WebSocket |
| **gRPC** | google.golang.org/grpc + protoreflect |
| **JSON Processing** | tidwall/gjson |
| **Rate Limiting** | golang.org/x/time/rate |

## Key Features

### Protocol Support
- **HTTP/1.1 & HTTP/2** - Full REST API testing with headers, bodies, and authentication
- **WebSocket** - Bidirectional messaging with handshake and message timing
- **SSE (Server-Sent Events)** - Event stream consumption with connection metrics
- **gRPC** - Unary RPC calls with Protocol Buffer support

### Load Patterns
- **Constant** - Fixed RPS for steady-state testing
- **Ramp** - Linear RPS growth for stress testing
- **Step** - Discrete RPS stages for capacity testing
- **Spike** - Burst traffic patterns

### Traffic Control
- **Uniform arrival** - Evenly spaced requests
- **Poisson arrival** - Realistic random arrival times
- **Concurrency control** - Parallel workers with rate limiting

### Data-Driven Testing
- **CSV/JSON feeders** - Inject dynamic data per request
- **Request chaining** - Extract values (JSONPath/regex) and reuse across requests
- **HAR import** - Replay recorded browser sessions

### Authentication
- **OAuth2 Client Credentials** - Service-to-service auth
- **OAuth2 Resource Owner** - User password flow (legacy)
- **OIDC Implicit/Auth Code** - OpenID Connect flows
- **Static tokens** - Pre-configured bearer tokens

### Output & Reporting
- **Live dashboard** - Real-time terminal UI with sparklines and gauges
- **HTML reports** - Standalone reports with interactive charts
- **JSON output** - Structured data for programmatic processing
- **Threshold assertions** - CI/CD pass/fail criteria

## Project Structure

```
crankfire/
├── cmd/
│   └── crankfire/              # CLI application entry point
│       ├── main.go             # Application bootstrap
│       ├── http_requester.go   # HTTP protocol implementation
│       ├── grpc_requester.go   # gRPC protocol implementation
│       ├── sse_requester.go    # SSE protocol implementation
│       ├── websocket_requester.go  # WebSocket implementation
│       ├── endpoints.go        # Multi-endpoint routing
│       ├── auth_helpers.go     # Authentication factory
│       ├── feeder_helpers.go   # Data feeder factory
│       └── placeholders.go     # Template variable substitution
│
├── internal/                   # Private packages
│   ├── auth/                   # Authentication providers
│   │   ├── provider.go         # Provider interface
│   │   ├── oauth2.go           # OAuth2 implementation
│   │   └── static.go           # Static token provider
│   │
│   ├── config/                 # Configuration management
│   │   ├── config.go           # Config struct & validation
│   │   └── loader.go           # CLI/file loading
│   │
│   ├── dashboard/              # Live terminal UI
│   │   └── dashboard.go        # termui widgets
│   │
│   ├── extractor/              # Response value extraction
│   │   └── extractor.go        # JSONPath/regex extractors
│   │
│   ├── feeder/                 # Test data injection
│   │   ├── feeder.go           # Feeder interface
│   │   ├── csv.go              # CSV file reader
│   │   └── json.go             # JSON file reader
│   │
│   ├── grpcclient/             # gRPC client wrapper
│   │   └── client.go           # Connection & invocation
│   │
│   ├── har/                    # HAR file processing
│   │   ├── parser.go           # HAR JSON parsing
│   │   ├── converter.go        # HAR→endpoint conversion
│   │   └── types.go            # HAR data structures
│   │
│   ├── httpclient/             # HTTP client wrapper
│   │   ├── client.go           # HTTP request execution
│   │   └── builder.go          # Request construction
│   │
│   ├── metrics/                # Metrics collection
│   │   └── collector.go        # HDR histogram aggregation
│   │
│   ├── output/                 # Report generation
│   │   ├── report.go           # Text/JSON output
│   │   ├── html.go             # HTML report template
│   │   └── progress.go         # Progress bar
│   │
│   ├── placeholders/           # Template processing
│   │   └── placeholders.go     # Variable substitution
│   │
│   ├── pool/                   # Object pooling
│   │   └── pool.go             # Buffer pools
│   │
│   ├── runner/                 # Core execution engine
│   │   ├── runner.go           # Worker coordination
│   │   ├── options.go          # Configuration options
│   │   ├── arrival.go          # Arrival models
│   │   ├── pattern_plan.go     # Load pattern scheduler
│   │   └── retry.go            # Retry logic
│   │
│   ├── sse/                    # SSE client
│   │   └── sse.go              # Event stream reader
│   │
│   ├── threshold/              # Pass/fail assertions
│   │   └── threshold.go        # Threshold parsing & evaluation
│   │
│   ├── variables/              # Per-worker state
│   │   └── store.go            # Variable storage
│   │
│   └── websocket/              # WebSocket client
│       └── websocket.go        # Connection management
│
├── docs/                       # User documentation (Jekyll)
│   ├── authentication.md
│   ├── configuration.md
│   ├── dashboard-reporting.md
│   ├── developer-guide.md
│   ├── feeders.md
│   ├── getting-started.md
│   ├── har-import.md
│   ├── protocols.md
│   ├── request-chaining.md
│   ├── thresholds.md
│   └── thresholds-quick-reference.md
│
├── scripts/                    # Test scripts & samples
│   ├── sample.yml              # Example configuration
│   ├── testservers/            # Mock servers for testing
│   └── doc-samples/            # Documentation examples
│
├── go.mod                      # Go module definition
├── Dockerfile                  # Container build
└── README.md                   # Project documentation
```

## Getting Started

### Installation

**Homebrew (macOS and Linux):**
```bash
brew tap torosent/crankfire
brew install crankfire
```

**Docker:**
```bash
docker run ghcr.io/torosent/crankfire --target https://example.com --total 100
```

**Go Install:**
```bash
go install github.com/torosent/crankfire/cmd/crankfire@latest
```

**Build from Source:**
```bash
git clone https://github.com/torosent/crankfire.git
cd crankfire
go build -o build/crankfire ./cmd/crankfire
```

### Quick Start

```bash
# Simple load test
crankfire --target https://api.example.com --concurrency 10 --total 100

# With live dashboard
crankfire --target https://api.example.com --concurrency 20 --rate 100 --duration 30s --dashboard

# Generate HTML report
crankfire --target https://example.com --total 1000 --html-output report.html

# CI/CD with thresholds
crankfire --target https://api.example.com \
  --threshold "http_req_duration:p95 < 500" \
  --threshold "http_req_failed:rate < 0.01"
```

## Architecture Summary

Crankfire follows a modular, pipeline-based architecture:

```
┌─────────────┐    ┌──────────────┐    ┌─────────────┐    ┌───────────┐
│   Config    │───▶│    Runner    │───▶│  Requester  │───▶│  Metrics  │
│   Loader    │    │   (Workers)  │    │ (Protocol)  │    │ Collector │
└─────────────┘    └──────────────┘    └─────────────┘    └───────────┘
                          │                   │                  │
                          ▼                   ▼                  ▼
                   ┌──────────────┐    ┌─────────────┐    ┌───────────┐
                   │  Rate Limiter│    │    Auth     │    │  Output   │
                   │  (Arrival)   │    │  Provider   │    │ (Reports) │
                   └──────────────┘    └─────────────┘    └───────────┘
```

1. **Config Loader** - Parses CLI flags and YAML/JSON config files
2. **Runner** - Orchestrates concurrent workers with rate limiting
3. **Requester** - Protocol-specific request execution (HTTP, WS, SSE, gRPC)
4. **Metrics Collector** - Aggregates latency histograms and error counts
5. **Output** - Renders reports (terminal, JSON, HTML)

The architecture supports extensibility through well-defined interfaces:
- `Requester` interface for adding new protocols
- `Provider` interface for authentication mechanisms
- `Feeder` interface for data sources
- `Store` interface for variable persistence

See [Architecture Overview](2.%20Architecture%20Overview.md) for detailed component diagrams.
