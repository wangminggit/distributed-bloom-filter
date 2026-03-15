package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/wangminggit/distributed-bloom-filter/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type BenchmarkResults struct {
	totalOps   int64
	successOps int64
	failedOps  int64
	latencies  []time.Duration
	mu         sync.Mutex
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: benchmark <single|cluster|mixed> [target]")
		os.Exit(1)
	}

	testType := os.Args[1]
	target := "localhost:50051"
	if len(os.Args) > 2 {
		target = os.Args[2]
	}

	fmt.Printf("Starting benchmark: %s on %s\n", testType, target)
	fmt.Println("=" + string(make([]byte, 60)))

	switch testType {
	case "single":
		runSingleNodeBenchmark(target)
	case "cluster":
		runClusterBenchmark(target)
	case "mixed":
		runMixedLoadBenchmark(target)
	default:
		log.Fatalf("Unknown test type: %s", testType)
	}
}

func runSingleNodeBenchmark(target string) {
	fmt.Println("\n📊 Single Node Benchmark (300 seconds)")
	fmt.Println("Operations: Add, Contains")
	fmt.Println("Concurrency: 50")
	fmt.Println("=" + string(make([]byte, 60)))

	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewDBFServiceClient(conn)
	ctx := context.Background()

	duration := 300 * time.Second
	concurrency := 50
	
	fmt.Printf("\nStarting Add benchmark...\n")
	addResults := runBenchmark(client, ctx, "Add", duration, concurrency)
	printResults("Add", addResults, duration)

	fmt.Printf("\nStarting Contains benchmark...\n")
	containsResults := runBenchmark(client, ctx, "Contains", duration, concurrency)
	printResults("Contains", containsResults, duration)
}

func runClusterBenchmark(target string) {
	fmt.Println("\n📊 Cluster Benchmark (3 nodes)")
	fmt.Println("Duration: 300 seconds")
	fmt.Println("Concurrency: 50")
	fmt.Println("=" + string(make([]byte, 60)))

	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewDBFServiceClient(conn)
	ctx := context.Background()

	duration := 300 * time.Second
	concurrency := 50

	results := runBenchmark(client, ctx, "Add", duration, concurrency)
	printResults("Cluster Add", results, duration)
}

func runMixedLoadBenchmark(target string) {
	fmt.Println("\n📊 Mixed Load Benchmark (3600 seconds)")
	fmt.Println("Workload: 80% Contains (read), 20% Add (write)")
	fmt.Println("Concurrency: 50")
	fmt.Println("=" + string(make([]byte, 60)))

	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewDBFServiceClient(conn)
	ctx := context.Background()

	duration := 3600 * time.Second
	concurrency := 50

	var wg sync.WaitGroup
	var totalOps, successOps, failedOps int64
	var latencies []time.Duration
	var mu sync.Mutex
	latencies = make([]time.Duration, 0, 100000)

	startTime := time.Now()
	endTime := startTime.Add(duration)

	worker := func(id int) {
		defer wg.Done()
		for time.Now().Before(endTime) {
			item := []byte(fmt.Sprintf("test-item-%d-%d", id, rand.Intn(1000000)))
			start := time.Now()

			var err error
			if rand.Float32() < 0.8 {
				_, err = client.Contains(ctx, &pb.ContainsRequest{Item: item})
			} else {
				_, err = client.Add(ctx, &pb.AddRequest{Item: item})
			}

			latency := time.Since(start)
			atomic.AddInt64(&totalOps, 1)

			if err == nil {
				atomic.AddInt64(&successOps, 1)
				mu.Lock()
				latencies = append(latencies, latency)
				mu.Unlock()
			} else {
				atomic.AddInt64(&failedOps, 1)
			}

			// Progress report every 5 minutes
			if time.Since(startTime)%(5*time.Minute) < time.Second {
				elapsed := time.Since(startTime)
				currentQPS := float64(atomic.LoadInt64(&successOps)) / elapsed.Seconds()
				fmt.Printf("[Progress] Elapsed: %v, Ops: %d, QPS: %.2f\n", 
					elapsed, atomic.LoadInt64(&totalOps), currentQPS)
			}
		}
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker(i)
	}

	wg.Wait()
	actualDuration := time.Since(startTime)

	// Calculate statistics
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	qps := float64(successOps) / actualDuration.Seconds()
	errorRate := float64(failedOps) / float64(totalOps) * 100
	
	var avgLatency time.Duration
	for _, l := range latencies {
		avgLatency += l
	}
	if len(latencies) > 0 {
		avgLatency /= time.Duration(len(latencies))
	}

	p99Idx := len(latencies) * 99 / 100
	p95Idx := len(latencies) * 95 / 100
	p50Idx := len(latencies) / 2

	fmt.Println("\n" + string(make([]byte, 60)))
	fmt.Println("📈 MIXED LOAD RESULTS (3600s)")
	fmt.Println(string(make([]byte, 60)))
	fmt.Printf("Duration:        %v\n", actualDuration)
	fmt.Printf("Total Ops:       %d\n", totalOps)
	fmt.Printf("Success Ops:     %d\n", successOps)
	fmt.Printf("Failed Ops:      %d\n", failedOps)
	fmt.Printf("QPS:             %.2f\n", qps)
	fmt.Printf("Error Rate:      %.2f%%\n", errorRate)
	fmt.Printf("Avg Latency:     %v\n", avgLatency)
	fmt.Printf("P50 Latency:     %v\n", latencies[p50Idx])
	fmt.Printf("P95 Latency:     %v\n", latencies[p95Idx])
	fmt.Printf("P99 Latency:     %v\n", latencies[p99Idx])
	fmt.Println(string(make([]byte, 60)))
}

func runBenchmark(client pb.DBFServiceClient, ctx context.Context, operation string, duration time.Duration, concurrency int) *BenchmarkResults {
	results := &BenchmarkResults{
		latencies: make([]time.Duration, 0, 100000),
	}

	var wg sync.WaitGroup
	startTime := time.Now()
	endTime := startTime.Add(duration)

	worker := func(id int) {
		defer wg.Done()
		for time.Now().Before(endTime) {
			item := []byte(fmt.Sprintf("test-item-%d-%d", id, rand.Intn(1000000)))
			start := time.Now()

			var err error
			switch operation {
			case "Add":
				_, err = client.Add(ctx, &pb.AddRequest{Item: item})
			case "Contains":
				_, err = client.Contains(ctx, &pb.ContainsRequest{Item: item})
			}

			latency := time.Since(start)
			atomic.AddInt64(&results.totalOps, 1)

			if err == nil {
				atomic.AddInt64(&results.successOps, 1)
				results.mu.Lock()
				results.latencies = append(results.latencies, latency)
				results.mu.Unlock()
			} else {
				atomic.AddInt64(&results.failedOps, 1)
			}

			// Progress report every minute
			if time.Since(startTime)%time.Minute < time.Second {
				elapsed := time.Since(startTime)
				currentQPS := float64(atomic.LoadInt64(&results.successOps)) / elapsed.Seconds()
				fmt.Printf("[%s] Elapsed: %v, Ops: %d, QPS: %.2f\n", 
					operation, elapsed, atomic.LoadInt64(&results.totalOps), currentQPS)
			}
		}
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker(i)
	}

	wg.Wait()
	return results
}

func printResults(name string, results *BenchmarkResults, duration time.Duration) {
	sort.Slice(results.latencies, func(i, j int) bool {
		return results.latencies[i] < results.latencies[j]
	})

	qps := float64(results.successOps) / duration.Seconds()
	errorRate := float64(results.failedOps) / float64(results.totalOps) * 100
	
	var avgLatency time.Duration
	for _, l := range results.latencies {
		avgLatency += l
	}
	if len(results.latencies) > 0 {
		avgLatency /= time.Duration(len(results.latencies))
	}

	p99Idx := len(results.latencies) * 99 / 100
	p95Idx := len(results.latencies) * 95 / 100
	p50Idx := len(results.latencies) / 2

	fmt.Println("\n" + string(make([]byte, 60)))
	fmt.Printf("📈 %s RESULTS\n", name)
	fmt.Println(string(make([]byte, 60)))
	fmt.Printf("Duration:        %v\n", duration)
	fmt.Printf("Total Ops:       %d\n", results.totalOps)
	fmt.Printf("Success Ops:     %d\n", results.successOps)
	fmt.Printf("Failed Ops:      %d\n", results.failedOps)
	fmt.Printf("QPS:             %.2f\n", qps)
	fmt.Printf("Error Rate:      %.2f%%\n", errorRate)
	fmt.Printf("Avg Latency:     %v\n", avgLatency)
	if len(results.latencies) > 0 {
		fmt.Printf("P50 Latency:     %v\n", results.latencies[p50Idx])
		fmt.Printf("P95 Latency:     %v\n", results.latencies[p95Idx])
		fmt.Printf("P99 Latency:     %v\n", results.latencies[p99Idx])
	}
	fmt.Println(string(make([]byte, 60)))
}
