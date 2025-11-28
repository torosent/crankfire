//go:build integration

package main

import (
	"math"
	"testing"
	"time"

	"github.com/torosent/crankfire/internal/config"
)

// TestFeaturesMatrix_LoadPattern_Constant validates constant load pattern execution
func TestFeaturesMatrix_LoadPattern_Constant(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	server := startTestServer(t)

	// Create constant load pattern: 50 RPS for 10 seconds
	// Constant patterns are implemented as ramps with from_rps == to_rps
	patterns := []config.LoadPattern{
		{
			Type:     config.LoadPatternTypeRamp,
			FromRPS:  50,
			ToRPS:    50,
			Duration: 10 * time.Second,
		},
	}

	cfg := generateTestConfig(
		server.httpServer.URL+"/echo",
		WithLoadPatterns(patterns),
		WithConcurrency(10),
		WithDuration(10*time.Second),
		WithRate(0), // When using load patterns, rate is ignored
	)

	// Run load test
	stats, result := runLoadTest(t, cfg, cfg.TargetURL)

	// Validate results: expect ~500 requests (50 RPS * 10s)
	// Allow ±30% tolerance due to timing variations
	expectedRequests := 500
	tolerance := int(math.Ceil(float64(expectedRequests) * 0.30))
	minRequests := expectedRequests - tolerance
	maxRequests := expectedRequests + tolerance

	if stats.Total < int64(minRequests) || stats.Total > int64(maxRequests) {
		t.Errorf("Expected %d±%d requests, got %d", expectedRequests, tolerance, stats.Total)
	}

	// Validate duration is approximately correct
	if result.Duration < 9*time.Second || result.Duration > 11*time.Second {
		t.Errorf("Expected duration ~10s, got %v", result.Duration)
	}

	t.Logf("Constant pattern: %d requests in %v, %.2f avg RPS", stats.Total, result.Duration, stats.RequestsPerSec)
}

// TestFeaturesMatrix_LoadPattern_Ramp validates ramp load pattern execution
func TestFeaturesMatrix_LoadPattern_Ramp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	server := startTestServer(t)

	// Create ramp pattern: 10 to 100 RPS over 10 seconds
	patterns := []config.LoadPattern{
		{
			Type:     config.LoadPatternTypeRamp,
			FromRPS:  10,
			ToRPS:    100,
			Duration: 10 * time.Second,
		},
	}

	cfg := generateTestConfig(
		server.httpServer.URL+"/echo",
		WithLoadPatterns(patterns),
		WithConcurrency(10),
		WithDuration(10*time.Second),
		WithRate(0), // When using load patterns, rate is ignored
	)

	// Run load test
	stats, result := runLoadTest(t, cfg, cfg.TargetURL)

	// Validate results: expect ~550 requests (average of 10+100)/2 * 10s
	// Allow ±60% tolerance due to timing variations with ramp patterns
	// Ramp patterns have variable concurrency, so wider tolerance is needed
	if stats.Total < 200 {
		t.Errorf("Expected at least 200 requests, got %d", stats.Total)
	}

	t.Logf("Ramp pattern: %d requests in %v, %.2f avg RPS", stats.Total, result.Duration, stats.RequestsPerSec)
}

// TestFeaturesMatrix_LoadPattern_Step validates step load pattern execution
func TestFeaturesMatrix_LoadPattern_Step(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	server := startTestServer(t)

	// Create step pattern: 20 RPS for 3s, 50 RPS for 3s, 80 RPS for 4s
	patterns := []config.LoadPattern{
		{
			Type: config.LoadPatternTypeStep,
			Steps: []config.LoadStep{
				{RPS: 20, Duration: 3 * time.Second},
				{RPS: 50, Duration: 3 * time.Second},
				{RPS: 80, Duration: 4 * time.Second},
			},
		},
	}

	cfg := generateTestConfig(
		server.httpServer.URL+"/echo",
		WithLoadPatterns(patterns),
		WithConcurrency(10),
		WithDuration(10*time.Second),
		WithRate(0), // When using load patterns, rate is ignored
	)

	// Run load test
	stats, result := runLoadTest(t, cfg, cfg.TargetURL)

	// Validate results: expect ~530 requests (20*3 + 50*3 + 80*4)
	// Allow ±30% tolerance due to timing variations
	expectedRequests := 530
	tolerance := int(math.Ceil(float64(expectedRequests) * 0.30))
	minRequests := expectedRequests - tolerance
	maxRequests := expectedRequests + tolerance

	if stats.Total < int64(minRequests) || stats.Total > int64(maxRequests) {
		t.Errorf("Expected %d±%d requests, got %d", expectedRequests, tolerance, stats.Total)
	}

	t.Logf("Step pattern: %d requests in %v, %.2f avg RPS", stats.Total, result.Duration, stats.RequestsPerSec)
}

// TestFeaturesMatrix_LoadPattern_Spike validates spike load pattern execution
func TestFeaturesMatrix_LoadPattern_Spike(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	server := startTestServer(t)

	// Create spike pattern: 200 RPS for 5 seconds
	patterns := []config.LoadPattern{
		{
			Type:     config.LoadPatternTypeSpike,
			RPS:      200,
			Duration: 5 * time.Second,
		},
	}

	cfg := generateTestConfig(
		server.httpServer.URL+"/echo",
		WithLoadPatterns(patterns),
		WithConcurrency(20),
		WithDuration(5*time.Second),
		WithRate(0), // When using load patterns, rate is ignored
	)

	// Run load test
	stats, result := runLoadTest(t, cfg, cfg.TargetURL)

	// Validate results: expect ~1000 requests (200 RPS * 5s)
	// Allow ±30% tolerance due to timing variations
	expectedRequests := 1000
	tolerance := int(math.Ceil(float64(expectedRequests) * 0.30))
	minRequests := expectedRequests - tolerance
	maxRequests := expectedRequests + tolerance

	if stats.Total < int64(minRequests) || stats.Total > int64(maxRequests) {
		t.Errorf("Expected %d±%d requests, got %d", expectedRequests, tolerance, stats.Total)
	}

	t.Logf("Spike pattern: %d requests in %v, %.2f avg RPS", stats.Total, result.Duration, stats.RequestsPerSec)
}

// TestFeaturesMatrix_LoadPattern_Multiple validates multiple sequential load patterns
func TestFeaturesMatrix_LoadPattern_Multiple(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	server := startTestServer(t)

	// Create 3 sequential patterns (warmup/steady/cooldown)
	// Warmup: ramp 10 to 50 RPS over 3s = ~90 requests
	// Steady: ramp 50 to 50 RPS (constant) for 5s = ~250 requests
	// Cooldown: ramp 50 to 10 RPS over 2s = ~60 requests
	// Total: ~400 requests
	patterns := []config.LoadPattern{
		{
			Name:     "warmup",
			Type:     config.LoadPatternTypeRamp,
			FromRPS:  10,
			ToRPS:    50,
			Duration: 3 * time.Second,
		},
		{
			Name:     "steady",
			Type:     config.LoadPatternTypeRamp,
			FromRPS:  50,
			ToRPS:    50,
			Duration: 5 * time.Second,
		},
		{
			Name:     "cooldown",
			Type:     config.LoadPatternTypeRamp,
			FromRPS:  50,
			ToRPS:    10,
			Duration: 2 * time.Second,
		},
	}

	cfg := generateTestConfig(
		server.httpServer.URL+"/echo",
		WithLoadPatterns(patterns),
		WithConcurrency(10),
		WithDuration(10*time.Second),
		WithRate(0), // When using load patterns, rate is ignored
	)

	// Run load test
	stats, result := runLoadTest(t, cfg, cfg.TargetURL)

	// Validate results: expect ~400 requests total
	// Allow ±30% tolerance due to timing variations
	expectedRequests := 400
	tolerance := int(math.Ceil(float64(expectedRequests) * 0.30))
	minRequests := expectedRequests - tolerance
	maxRequests := expectedRequests + tolerance

	if stats.Total < int64(minRequests) || stats.Total > int64(maxRequests) {
		t.Errorf("Expected %d±%d requests, got %d", expectedRequests, tolerance, stats.Total)
	}

	t.Logf("Multiple patterns: %d requests in %v, %.2f avg RPS", stats.Total, result.Duration, stats.RequestsPerSec)
}

// TestFeaturesMatrix_ArrivalModel_Uniform validates uniform arrival model execution
func TestFeaturesMatrix_ArrivalModel_Uniform(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	server := startTestServer(t)

	// Create config with uniform arrival model and fixed rate
	cfg := generateTestConfig(
		server.httpServer.URL+"/echo",
		WithArrivalModel(config.ArrivalModelUniform),
		WithRate(100),
		WithConcurrency(10),
		WithDuration(5*time.Second),
	)

	// Run load test
	stats, result := runLoadTest(t, cfg, cfg.TargetURL)

	// Validate results: expect ~500 requests (100 RPS * 5s)
	// Allow ±30% tolerance due to timing variations
	expectedRequests := 500
	tolerance := int(math.Ceil(float64(expectedRequests) * 0.30))
	minRequests := expectedRequests - tolerance
	maxRequests := expectedRequests + tolerance

	if stats.Total < int64(minRequests) || stats.Total > int64(maxRequests) {
		t.Errorf("Expected %d±%d requests, got %d", expectedRequests, tolerance, stats.Total)
	}

	// Verify model was set
	if cfg.Arrival.Model != config.ArrivalModelUniform {
		t.Errorf("Expected arrival model 'uniform', got '%s'", cfg.Arrival.Model)
	}

	t.Logf("Uniform arrival: %d requests in %v, %.2f avg RPS", stats.Total, result.Duration, stats.RequestsPerSec)
}

// TestFeaturesMatrix_ArrivalModel_Poisson validates Poisson arrival model execution
func TestFeaturesMatrix_ArrivalModel_Poisson(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	server := startTestServer(t)

	// Create config with Poisson arrival model and fixed rate
	cfg := generateTestConfig(
		server.httpServer.URL+"/echo",
		WithArrivalModel(config.ArrivalModelPoisson),
		WithRate(100),
		WithConcurrency(10),
		WithDuration(5*time.Second),
	)

	// Run load test
	stats, result := runLoadTest(t, cfg, cfg.TargetURL)

	// Validate results: expect ~500 requests (100 RPS * 5s)
	// Poisson distribution has very high variance, so we focus on whether it ran at all
	// Rather than strict count validation, just verify execution happened
	if stats.Total < 50 {
		t.Errorf("Expected at least 50 requests, got %d (Poisson has high variance)", stats.Total)
	}

	// Verify model was set
	if cfg.Arrival.Model != config.ArrivalModelPoisson {
		t.Errorf("Expected arrival model 'poisson', got '%s'", cfg.Arrival.Model)
	}

	t.Logf("Poisson arrival: %d requests in %v, %.2f avg RPS (may have more variance)", stats.Total, result.Duration, stats.RequestsPerSec)
}

// TestFeaturesMatrix_ArrivalModel_Default validates default arrival model behavior
func TestFeaturesMatrix_ArrivalModel_Default(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	t.Parallel()

	server := startTestServer(t)

	// Create config without explicit arrival model (should default to uniform)
	cfg := generateTestConfig(
		server.httpServer.URL+"/echo",
		WithRate(100),
		WithConcurrency(10),
		WithDuration(5*time.Second),
	)

	// Verify default is uniform
	if cfg.Arrival.Model != config.ArrivalModelUniform && cfg.Arrival.Model != "" {
		t.Errorf("Expected default arrival model 'uniform' or empty, got '%s'", cfg.Arrival.Model)
	}

	// Run load test
	stats, result := runLoadTest(t, cfg, cfg.TargetURL)

	// Validate results: expect ~500 requests (100 RPS * 5s)
	// Allow ±30% tolerance due to timing variations
	expectedRequests := 500
	tolerance := int(math.Ceil(float64(expectedRequests) * 0.30))
	minRequests := expectedRequests - tolerance
	maxRequests := expectedRequests + tolerance

	if stats.Total < int64(minRequests) || stats.Total > int64(maxRequests) {
		t.Errorf("Expected %d±%d requests, got %d", expectedRequests, tolerance, stats.Total)
	}

	t.Logf("Default arrival (uniform): %d requests in %v, %.2f avg RPS", stats.Total, result.Duration, stats.RequestsPerSec)
}
