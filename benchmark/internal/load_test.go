package internal

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pb "github.com/wangminggit/distributed-bloom-filter/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// BenchmarkConfig holds configuration for benchmarks
type BenchmarkConfig struct {
	target     string
	concurrency int
	duration   time.Duration
	itemCount  int
}

// BenchmarkResults holds benchmark metrics
type BenchmarkResults struct {
	totalOps     int64
	successOps   int64
	failedOps    int64
	totalLatency time.Duration
	minLatency   time.Duration
	maxLatency   time.Duration
	p50Latency   time.Duration
	p95Latency   time.Duration
	p99Latency   time.Duration
	qps          float64
	errorRate    float64
}

var latencies []time.Duration
var latencyMu sync.Mutex

// setupConn creates a gRPC connection
func setupConn(target string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	return conn, nil
}

// generateTestItem generates a random test item
func generateTestItem(id int) []byte {
	return []byte(fmt.Sprintf("test-item-%d-%d", id, rand.Intn(1000000)))
}

// runLoadTest executes a load test with the given configuration
func runLoadTest(b *testing.B, config BenchmarkConfig, operation string) *BenchmarkResults {
	conn, err := setupConn(config.target)
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewDBFServiceClient(conn)
	ctx := context.Background()

	var wg sync.WaitGroup
	var totalOps, successOps, failedOps int64
	latencies = make([]time.Duration, 0)

	startTime := time.Now()
	endTime := startTime.Add(config.duration)

	// Worker function
	worker := func(id int) {
		defer wg.Done()
		for time.Now().Before(endTime) {
			item := generateTestItem(id)
			start := time.Now()

			var err error
			switch operation {
			case "Add":
				_, err = client.Add(ctx, &pb.AddRequest{Item: item})
			case "Contains":
				_, err = client.Contains(ctx, &pb.ContainsRequest{Item: item})
			case "Remove":
				_, err = client.Remove(ctx, &pb.RemoveRequest{Item: item})
			}

			latency := time.Since(start)
			atomic.AddInt64(&totalOps, 1)

			if err == nil {
				atomic.AddInt64(&successOps, 1)
				latencyMu.Lock()
				latencies = append(latencies, latency)
				latencyMu.Unlock()
			} else {
				atomic.AddInt64(&failedOps, 1)
			}
		}
	}

	// Start workers
	for i := 0; i < config.concurrency; i++ {
		wg.Add(1)
		go worker(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// Calculate statistics
	results := calculateStats(totalOps, successOps, failedOps, duration)
	return results
}

// calculateStats calculates benchmark statistics
func calculateStats(totalOps, successOps, failedOps int64, duration time.Duration) *BenchmarkResults {
	results := &BenchmarkResults{
		totalOps:   totalOps,
		successOps: successOps,
		failedOps:  failedOps,
		qps:        float64(successOps) / duration.Seconds(),
		errorRate:  float64(failedOps) / float64(totalOps) * 100,
	}

	// Calculate latency percentiles
	if len(latencies) > 0 {
		latencyMu.Lock()
		defer latencyMu.Unlock()
		
		// Sort latencies
		sorted := make([]time.Duration, len(latencies))
		copy(sorted, latencies)
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i] > sorted[j] {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}

		results.minLatency = sorted[0]
		results.maxLatency = sorted[len(sorted)-1]
		results.p50Latency = sorted[len(sorted)/2]
		results.p95Latency = sorted[len(sorted)*95/100]
		results.p99Latency = sorted[len(sorted)*99/100]

		var total time.Duration
		for _, l := range latencies {
			total += l
		}
		results.totalLatency = total / time.Duration(len(latencies))
	}

	return results
}

// BenchmarkSingleAdd tests single Add operation performance
func BenchmarkSingleAdd(b *testing.B) {
	config := BenchmarkConfig{
		target:      "localhost:50051",
		concurrency: 10,
		duration:    300 * time.Second, // 300 seconds
		itemCount:   10000,
	}
	
	b.Run("Add_300s", func(b *testing.B) {
		results := runLoadTest(b, config, "Add")
		b.Logf("QPS: %.2f, P99: %v, Error Rate: %.2f%%", 
			results.qps, results.p99Latency, results.errorRate)
	})
}

// BenchmarkSingleContains tests single Contains operation performance
func BenchmarkSingleContains(b *testing.B) {
	config := BenchmarkConfig{
		target:      "localhost:50051",
		concurrency: 10,
		duration:    300 * time.Second,
		itemCount:   10000,
	}
	
	b.Run("Contains_300s", func(b *testing.B) {
		results := runLoadTest(b, config, "Contains")
		b.Logf("QPS: %.2f, P99: %v, Error Rate: %.2f%%", 
			results.qps, results.p99Latency, results.errorRate)
	})
}

// BenchmarkMixedLoad tests mixed read/write workload (80% read, 20% write)
func BenchmarkMixedLoad(b *testing.B) {
	conn, err := setupConn("localhost:50051")
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewDBFServiceClient(conn)
	ctx := context.Background()

	var wg sync.WaitGroup
	var totalOps, successOps, failedOps int64
	latencies = make([]time.Duration, 0)

	duration := 3600 * time.Second // 3600 seconds
	startTime := time.Now()
	endTime := startTime.Add(duration)

	worker := func(id int) {
		defer wg.Done()
		for time.Now().Before(endTime) {
			item := generateTestItem(id)
			start := time.Now()

			var err error
			// 80% read (Contains), 20% write (Add)
			if rand.Float32() < 0.8 {
				_, err = client.Contains(ctx, &pb.ContainsRequest{Item: item})
			} else {
				_, err = client.Add(ctx, &pb.AddRequest{Item: item})
			}

			latency := time.Since(start)
			atomic.AddInt64(&totalOps, 1)

			if err == nil {
				atomic.AddInt64(&successOps, 1)
				latencyMu.Lock()
				latencies = append(latencies, latency)
				latencyMu.Unlock()
			} else {
				atomic.AddInt64(&failedOps, 1)
			}
		}
	}

	concurrency := 50
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker(i)
	}

	wg.Wait()
	duration = time.Since(startTime)

	results := calculateStats(totalOps, successOps, failedOps, duration)
	b.Logf("Mixed Load (80%% read, 20%% write):")
	b.Logf("  Duration: %v", duration)
	b.Logf("  Total Ops: %d, Success: %d, Failed: %d", totalOps, successOps, failedOps)
	b.Logf("  QPS: %.2f, P99: %v, Error Rate: %.2f%%", 
		results.qps, results.p99Latency, results.errorRate)
}
