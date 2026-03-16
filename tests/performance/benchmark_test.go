package performance

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	pb "github.com/wangminggit/distributed-bloom-filter/api/proto"
	"github.com/wangminggit/distributed-bloom-filter/internal/grpc"
	"github.com/wangminggit/distributed-bloom-filter/internal/metadata"
	raftnode "github.com/wangminggit/distributed-bloom-filter/internal/raft"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	benchmarkDuration = 10 * time.Second
	warmupDuration    = 2 * time.Second
)

// BenchmarkCluster setup for performance tests
type BenchmarkCluster struct {
	grpcServer *grpc.DBFServer
	grpcConn   *grpc.ClientConn
	grpcClient pb.DBFServiceClient
	raftNode   *raftnode.Node
	dataDir    string
	grpcPort   int
	raftPort   int
}

// NewBenchmarkCluster creates a benchmark cluster
func NewBenchmarkCluster() *BenchmarkCluster {
	return &BenchmarkCluster{
		grpcPort: 25000,
		raftPort: 25001,
	}
}

// Start starts the benchmark cluster
func (bc *BenchmarkCluster) Start(t *testing.B) error {
	bc.dataDir = filepath.Join(os.TempDir(), fmt.Sprintf("dbf-bench-%d", time.Now().UnixNano()))
	os.MkdirAll(bc.dataDir, 0755)

	// Create Bloom filter
	bloomFilter := bloom.NewCountingBloomFilter(10485760, 7)

	// Create WAL encryptor
	walEncryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		return fmt.Errorf("failed to create WAL encryptor: %w", err)
	}

	// Create metadata service
	metadataService := metadata.NewService(bc.dataDir)

	// Create Raft node
	bc.raftNode = raftnode.NewNode("bench-node", bc.raftPort, bc.dataDir, bloomFilter, walEncryptor, metadataService)

	// Start Raft node
	if err := bc.raftNode.Start(true); err != nil {
		return fmt.Errorf("failed to start Raft node: %w", err)
	}

	// Wait for leader election
	time.Sleep(2 * time.Second)

	// Create gRPC server
	bc.grpcServer = grpc.NewDBFServer(bc.raftNode)

	// Start gRPC server
	addr := fmt.Sprintf("localhost:%d", bc.grpcPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	go func() {
		if err := bc.grpcServer.Serve(lis); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Create gRPC client
	bc.grpcConn, err = grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	bc.grpcClient = pb.NewDBFServiceClient(bc.grpcConn)

	return nil
}

// Stop stops the benchmark cluster
func (bc *BenchmarkCluster) Stop() {
	if bc.grpcConn != nil {
		bc.grpcConn.Close()
	}

	if bc.grpcServer != nil {
		bc.grpcServer.Stop()
	}

	if bc.raftNode != nil {
		bc.raftNode.Shutdown()
	}

	if bc.dataDir != "" {
		os.RemoveAll(bc.dataDir)
	}
}

// BenchmarkAddPerformance benchmarks Add operation performance
func BenchmarkAddPerformance(b *testing.B) {
	cluster := NewBenchmarkCluster()
	if err := cluster.Start(b); err != nil {
		b.Fatalf("Failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := []byte(fmt.Sprintf("perf-test-item-%d", i))
		req := &pb.AddRequest{Item: item}
		_, err := cluster.grpcClient.Add(ctx, req)
		if err != nil {
			b.Fatalf("Add failed: %v", err)
		}
	}
}

// BenchmarkContainsPerformance benchmarks Contains operation performance
func BenchmarkContainsPerformance(b *testing.B) {
	cluster := NewBenchmarkCluster()
	if err := cluster.Start(b); err != nil {
		b.Fatalf("Failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	ctx := context.Background()

	// Pre-populate with data
	for i := 0; i < 1000; i++ {
		item := []byte(fmt.Sprintf("perf-test-item-%d", i))
		req := &pb.AddRequest{Item: item}
		_, err := cluster.grpcClient.Add(ctx, req)
		require.NoError(b, err)
	}

	time.Sleep(200 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := []byte(fmt.Sprintf("perf-test-item-%d", i%1000))
		req := &pb.ContainsRequest{Item: item}
		_, err := cluster.grpcClient.Contains(ctx, req)
		if err != nil {
			b.Fatalf("Contains failed: %v", err)
		}
	}
}

// BenchmarkBatchAddPerformance benchmarks BatchAdd operation performance
func BenchmarkBatchAddPerformance(b *testing.B) {
	cluster := NewBenchmarkCluster()
	if err := cluster.Start(b); err != nil {
		b.Fatalf("Failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	ctx := context.Background()
	batchSize := 100

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		items := make([][]byte, batchSize)
		for j := 0; j < batchSize; j++ {
			items[j] = []byte(fmt.Sprintf("batch-perf-%d-%d", i, j))
		}

		req := &pb.BatchAddRequest{Items: items}
		resp, err := cluster.grpcClient.BatchAdd(ctx, req)
		if err != nil {
			b.Fatalf("BatchAdd failed: %v", err)
		}
		if resp.SuccessCount != int32(batchSize) {
			b.Fatalf("Expected %d successes, got %d", batchSize, resp.SuccessCount)
		}
	}

	b.SetBytes(int64(batchSize))
}

// BenchmarkMixedLoadPerformance benchmarks mixed read/write workload
func BenchmarkMixedLoadPerformance(b *testing.B) {
	cluster := NewBenchmarkCluster()
	if err := cluster.Start(b); err != nil {
		b.Fatalf("Failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	ctx := context.Background()

	// Pre-populate with data
	for i := 0; i < 10000; i++ {
		item := []byte(fmt.Sprintf("mixed-load-item-%d", i))
		req := &pb.AddRequest{Item: item}
		_, err := cluster.grpcClient.Add(ctx, req)
		require.NoError(b, err)
	}

	time.Sleep(500 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 80% reads, 20% writes
		if i%5 == 0 {
			// Write
			item := []byte(fmt.Sprintf("mixed-load-new-%d", i))
			req := &pb.AddRequest{Item: item}
			_, err := cluster.grpcClient.Add(ctx, req)
			if err != nil {
				b.Fatalf("Add failed: %v", err)
			}
		} else {
			// Read
			item := []byte(fmt.Sprintf("mixed-load-item-%d", i%10000))
			req := &pb.ContainsRequest{Item: item}
			_, err := cluster.grpcClient.Contains(ctx, req)
			if err != nil {
				b.Fatalf("Contains failed: %v", err)
			}
		}
	}
}

// BenchmarkConcurrentAdd benchmarks concurrent Add operations
func BenchmarkConcurrentAdd(b *testing.B) {
	cluster := NewBenchmarkCluster()
	if err := cluster.Start(b); err != nil {
		b.Fatalf("Failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	ctx := context.Background()
	concurrency := 10
	opsPerWorker := b.N / concurrency

	b.ResetTimer()

	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < opsPerWorker; i++ {
				item := []byte(fmt.Sprintf("concurrent-add-%d-%d", workerID, i))
				req := &pb.AddRequest{Item: item}
				_, err := cluster.grpcClient.Add(ctx, req)
				if err != nil {
					b.Errorf("Worker %d Add failed: %v", workerID, err)
				}
			}
		}(w)
	}

	wg.Wait()
}

// BenchmarkConcurrentContains benchmarks concurrent Contains operations
func BenchmarkConcurrentContains(b *testing.B) {
	cluster := NewBenchmarkCluster()
	if err := cluster.Start(b); err != nil {
		b.Fatalf("Failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		item := []byte(fmt.Sprintf("concurrent-item-%d", i))
		req := &pb.AddRequest{Item: item}
		_, err := cluster.grpcClient.Add(ctx, req)
		require.NoError(b, err)
	}

	time.Sleep(200 * time.Millisecond)

	concurrency := 10
	opsPerWorker := b.N / concurrency

	b.ResetTimer()

	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < opsPerWorker; i++ {
				item := []byte(fmt.Sprintf("concurrent-item-%d", i%1000))
				req := &pb.ContainsRequest{Item: item}
				_, err := cluster.grpcClient.Contains(ctx, req)
				if err != nil {
					b.Errorf("Worker %d Contains failed: %v", workerID, err)
				}
			}
		}(w)
	}

	wg.Wait()
}

// BenchmarkThroughput measures throughput over time
func BenchmarkThroughput(b *testing.B) {
	cluster := NewBenchmarkCluster()
	if err := cluster.Start(b); err != nil {
		b.Fatalf("Failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	ctx := context.Background()

	var totalOps int64
	var successOps int64
	var mu sync.Mutex

	done := make(chan bool)

	// Worker goroutines
	workerCount := 5
	for w := 0; w < workerCount; w++ {
		go func(workerID int) {
			for {
				select {
				case <-done:
					return
				default:
					item := []byte(fmt.Sprintf("throughput-test-%d-%d", workerID, time.Now().UnixNano()))
					req := &pb.AddRequest{Item: item}
					resp, err := cluster.grpcClient.Add(ctx, req)
					mu.Lock()
					totalOps++
					if err == nil && resp.Success {
						successOps++
					}
					mu.Unlock()
				}
			}
		}(w)
	}

	// Run for benchmark duration
	time.Sleep(benchmarkDuration)
	close(done)

	// Calculate throughput
	throughput := float64(successOps) / benchmarkDuration.Seconds()
	b.Logf("Throughput: %.2f ops/sec", throughput)
	b.Logf("Total ops: %d, Success ops: %d", totalOps, successOps)
}

// BenchmarkLatencyDistribution measures latency distribution
func BenchmarkLatencyDistribution(b *testing.B) {
	cluster := NewBenchmarkCluster()
	if err := cluster.Start(b); err != nil {
		b.Fatalf("Failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	ctx := context.Background()

	latencies := make([]time.Duration, 0, b.N)
	var mu sync.Mutex

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := []byte(fmt.Sprintf("latency-test-%d", i))
		req := &pb.AddRequest{Item: item}

		start := time.Now()
		resp, err := cluster.grpcClient.Add(ctx, req)
		latency := time.Since(start)

		if err == nil && resp.Success {
			mu.Lock()
			latencies = append(latencies, latency)
			mu.Unlock()
		}
	}

	// Calculate percentiles
	if len(latencies) > 0 {
		// Sort latencies
		for i := 0; i < len(latencies)-1; i++ {
			for j := i + 1; j < len(latencies); j++ {
				if latencies[i] > latencies[j] {
					latencies[i], latencies[j] = latencies[j], latencies[i]
				}
			}
		}

		p50 := latencies[len(latencies)*50/100]
		p95 := latencies[len(latencies)*95/100]
		p99 := latencies[len(latencies)*99/100]

		b.Logf("Latency P50: %v", p50)
		b.Logf("Latency P95: %v", p95)
		b.Logf("Latency P99: %v", p99)
	}
}

// BenchmarkLongRunning tests stability over extended period
func BenchmarkLongRunning(b *testing.B) {
	cluster := NewBenchmarkCluster()
	if err := cluster.Start(b); err != nil {
		b.Fatalf("Failed to start cluster: %v", err)
	}
	defer cluster.Stop()

	ctx := context.Background()

	var totalOps int64
	var errorCount int64
	var mu sync.Mutex

	done := make(chan bool)

	// Continuous workers
	workerCount := 3
	for w := 0; w < workerCount; w++ {
		go func(workerID int) {
			for {
				select {
				case <-done:
					return
				default:
					item := []byte(fmt.Sprintf("long-running-%d-%d", workerID, time.Now().UnixNano()))
					req := &pb.AddRequest{Item: item}
					resp, err := cluster.grpcClient.Add(ctx, req)
					mu.Lock()
					totalOps++
					if err != nil || !resp.Success {
						errorCount++
					}
					mu.Unlock()

					time.Sleep(10 * time.Millisecond)
				}
			}
		}(w)
	}

	// Run for extended period (use shorter duration for tests)
	testDuration := 30 * time.Second
	time.Sleep(testDuration)
	close(done)

	// Report results
	errorRate := float64(errorCount) / float64(totalOps) * 100
	b.Logf("Test duration: %v", testDuration)
	b.Logf("Total operations: %d", totalOps)
	b.Logf("Errors: %d (%.2f%%)", errorCount, errorRate)
	b.Logf("Average QPS: %.2f", float64(totalOps)/testDuration.Seconds())

	if errorRate > 1.0 {
		b.Errorf("Error rate too high: %.2f%%", errorRate)
	}
}
