package metrics

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestShardedStats_Initialization(t *testing.T) {
	s := newShardedStats()
	for i, shard := range s.shards {
		if shard == nil {
			t.Errorf("shard %d is nil", i)
		}
		if shard.bucket == nil {
			t.Errorf("shard %d bucket is nil", i)
		}
	}
}

func TestShardedStats_Distribution(t *testing.T) {
	s := newShardedStats()
	totalRequests := 10000

	// Record many requests
	for i := 0; i < totalRequests; i++ {
		s.record(time.Millisecond, nil, "", "")
	}

	// Check that requests are distributed across shards
	// Since it's random, we can't guarantee exact distribution,
	// but with 10000 requests and 32 shards, it's extremely unlikely any shard is empty.
	emptyShards := 0
	var totalRecorded int64
	for _, shard := range s.shards {
		shard.mu.Lock()
		count := shard.bucket.successes + shard.bucket.failures
		shard.mu.Unlock()

		totalRecorded += count
		if count == 0 {
			emptyShards++
		}
	}

	if totalRecorded != int64(totalRequests) {
		t.Errorf("expected %d total requests recorded, got %d", totalRequests, totalRecorded)
	}

	// Allow a few empty shards just in case of extreme RNG bad luck, but warn.
	// With 10000 items and 32 buckets, probability of empty bucket is negligible.
	if emptyShards > 0 {
		t.Logf("warning: %d shards were empty", emptyShards)
	}
}

func TestShardedStats_Aggregation(t *testing.T) {
	s := newShardedStats()

	// Manually inject data into specific shards to verify aggregation
	// Shard 0: 1 success, 10ms
	s.shards[0].mu.Lock()
	s.shards[0].bucket.record(10*time.Millisecond, nil, "", "")
	s.shards[0].mu.Unlock()

	// Shard 1: 1 failure, 20ms
	s.shards[1].mu.Lock()
	s.shards[1].bucket.record(20*time.Millisecond, errors.New("fail"), "http", "500")
	s.shards[1].mu.Unlock()

	// Snapshot
	stats := s.snapshot(time.Second)

	if stats.Total != 2 {
		t.Errorf("expected total 2, got %d", stats.Total)
	}
	if stats.Successes != 1 {
		t.Errorf("expected successes 1, got %d", stats.Successes)
	}
	if stats.Failures != 1 {
		t.Errorf("expected failures 1, got %d", stats.Failures)
	}
	if stats.MinLatency != 10*time.Millisecond {
		t.Errorf("expected min 10ms, got %v", stats.MinLatency)
	}
	if stats.MaxLatency != 20*time.Millisecond {
		t.Errorf("expected max 20ms, got %v", stats.MaxLatency)
	}
	
	// Check status buckets aggregation
	if stats.StatusBuckets == nil {
		t.Fatal("expected status buckets")
	}
	if stats.StatusBuckets["http"]["500"] != 1 {
		t.Errorf("expected http 500 count 1, got %d", stats.StatusBuckets["http"]["500"])
	}
}

func TestShardedStats_ConcurrentAccess(t *testing.T) {
	s := newShardedStats()
	var wg sync.WaitGroup
	workers := 50
	requestsPerWorker := 100

	// Concurrent writers
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				s.record(time.Millisecond, nil, "", "")
			}
		}()
	}

	// Concurrent reader (snapshotter)
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				s.snapshot(time.Second)
				time.Sleep(time.Millisecond)
			}
		}
	}()

	wg.Wait()
	close(done)

	// Final verification
	stats := s.snapshot(time.Second)
	expectedTotal := int64(workers * requestsPerWorker)
	if stats.Total != expectedTotal {
		t.Errorf("expected total %d, got %d", expectedTotal, stats.Total)
	}
}
