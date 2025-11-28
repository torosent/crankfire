// Package runner provides the core load test execution engine for crankfire.
//
// The runner package orchestrates concurrent request execution with support for:
//   - Configurable concurrency levels
//   - Rate limiting (requests per second)
//   - Duration-based and count-based test termination
//   - Multiple arrival models (uniform, Poisson)
//   - Dynamic load patterns (constant, ramp-up, step)
//
// # Basic Usage
//
// Create a runner with options and a requester implementation:
//
//	opts := runner.Options{
//		Concurrency:   10,
//		TotalRequests: 1000,
//		Duration:      time.Minute,
//		RatePerSecond: 100,
//		Requester:     myRequester,
//	}
//	r := runner.New(opts)
//	result := r.Run(ctx)
//
// # Requester Interface
//
// The [Requester] interface defines what a runner executes:
//
//	type Requester interface {
//		Do(ctx context.Context) error
//	}
//
// Implement this interface for different protocols (HTTP, WebSocket, gRPC, SSE).
//
// # Rate Limiting & Arrival Models
//
// The runner supports different arrival models for request pacing:
//   - [ArrivalModelUniform]: Requests at fixed intervals
//   - [ArrivalModelPoisson]: Requests following Poisson distribution for realistic traffic
//
// # Load Patterns
//
// Define dynamic load profiles using [LoadPattern]:
//   - Constant: Fixed RPS for a duration
//   - Ramp: Gradual increase/decrease in RPS
//   - Step: Discrete RPS changes over time
//
// # Middleware
//
// Enhance requesters with middleware:
//   - [WithLogging]: Log request failures
//   - [WithRetry]: Automatic retry with backoff
//
// # Error Handling
//
// The [HTTPError] type provides structured error information for HTTP requests:
//
//	if httpErr, ok := err.(*runner.HTTPError); ok {
//		fmt.Printf("Status: %d, Body: %s\n", httpErr.StatusCode, httpErr.Body)
//	}
package runner
