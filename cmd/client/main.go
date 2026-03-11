package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
)

func main() {
	// Parse command-line flags
	serverAddr := flag.String("server", "localhost:8080", "gRPC server address")
	action := flag.String("action", "add", "Action to perform: add, remove, contains, batch-add, batch-contains, stats")
	items := flag.String("items", "", "Items to process (comma-separated)")
	flag.Parse()

	// Connect to the server
	conn, err := grpc.NewClient(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Create client
	client := proto.NewDBFServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Parse items
	itemList := parseItems(*items)

	// Execute action
	switch *action {
	case "add":
		if len(itemList) == 0 {
			log.Fatal("No items provided for add")
		}
		for _, item := range itemList {
			addItem(ctx, client, item)
		}

	case "remove":
		if len(itemList) == 0 {
			log.Fatal("No items provided for remove")
		}
		for _, item := range itemList {
			removeItem(ctx, client, item)
		}

	case "contains":
		if len(itemList) == 0 {
			log.Fatal("No items provided for contains")
		}
		for _, item := range itemList {
			containsItem(ctx, client, item)
		}

	case "batch-add":
		if len(itemList) == 0 {
			log.Fatal("No items provided for batch-add")
		}
		batchAdd(ctx, client, itemList)

	case "batch-contains":
		if len(itemList) == 0 {
			log.Fatal("No items provided for batch-contains")
		}
		batchContains(ctx, client, itemList)

	case "stats":
		getStats(ctx, client)

	default:
		log.Fatalf("Unknown action: %s", *action)
	}
}

func parseItems(items string) [][]byte {
	if items == "" {
		return nil
	}
	result := make([][]byte, 0)
	for _, item := range split(items, ',') {
		result = append(result, []byte(item))
	}
	return result
}

// split splits a string by a separator (simple implementation without strings package)
func split(s string, sep rune) []string {
	var result []string
	var current string
	for _, c := range s {
		if c == sep {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func addItem(ctx context.Context, client proto.DBFServiceClient, item []byte) {
	req := &proto.AddRequest{Item: item}
	resp, err := client.Add(ctx, req)
	if err != nil {
		log.Printf("Error adding %s: %v", string(item), err)
		return
	}
	if resp.Success {
		fmt.Printf("✓ Added: %s\n", string(item))
	} else {
		fmt.Printf("✗ Failed to add %s: %s\n", string(item), resp.Error)
	}
}

func removeItem(ctx context.Context, client proto.DBFServiceClient, item []byte) {
	req := &proto.RemoveRequest{Item: item}
	resp, err := client.Remove(ctx, req)
	if err != nil {
		log.Printf("Error removing %s: %v", string(item), err)
		return
	}
	if resp.Success {
		fmt.Printf("✓ Removed: %s\n", string(item))
	} else {
		fmt.Printf("✗ Failed to remove %s: %s\n", string(item), resp.Error)
	}
}

func containsItem(ctx context.Context, client proto.DBFServiceClient, item []byte) {
	req := &proto.ContainsRequest{Item: item}
	resp, err := client.Contains(ctx, req)
	if err != nil {
		log.Printf("Error checking %s: %v", string(item), err)
		return
	}
	if resp.Error != "" {
		fmt.Printf("✗ Error: %s\n", resp.Error)
	} else if resp.Exists {
		fmt.Printf("✓ Contains: %s\n", string(item))
	} else {
		fmt.Printf("✗ Not found: %s\n", string(item))
	}
}

func batchAdd(ctx context.Context, client proto.DBFServiceClient, items [][]byte) {
	req := &proto.BatchAddRequest{Items: items}
	resp, err := client.BatchAdd(ctx, req)
	if err != nil {
		log.Fatalf("Error in batch add: %v", err)
	}
	fmt.Printf("Batch Add Result: %d succeeded, %d failed\n", resp.SuccessCount, resp.FailureCount)
	for i, err := range resp.Errors {
		if err != "" {
			fmt.Printf("  Item %d (%s): %s\n", i, string(items[i]), err)
		}
	}
}

func batchContains(ctx context.Context, client proto.DBFServiceClient, items [][]byte) {
	req := &proto.BatchContainsRequest{Items: items}
	resp, err := client.BatchContains(ctx, req)
	if err != nil {
		log.Fatalf("Error in batch contains: %v", err)
	}
	if resp.Error != "" {
		fmt.Printf("Error: %s\n", resp.Error)
		return
	}
	fmt.Println("Batch Contains Result:")
	for i, exists := range resp.Results {
		status := "✗ Not found"
		if exists {
			status = "✓ Contains"
		}
		fmt.Printf("  %s: %s\n", status, string(items[i]))
	}
}

func getStats(ctx context.Context, client proto.DBFServiceClient) {
	req := &proto.GetStatsRequest{}
	resp, err := client.GetStats(ctx, req)
	if err != nil {
		log.Fatalf("Error getting stats: %v", err)
	}
	if resp.Error != "" {
		fmt.Printf("Error: %s\n", resp.Error)
		return
	}
	fmt.Println("Server Statistics:")
	fmt.Printf("  Node ID:      %s\n", resp.NodeId)
	fmt.Printf("  Is Leader:    %v\n", resp.IsLeader)
	fmt.Printf("  Raft State:   %s\n", resp.RaftState)
	fmt.Printf("  Leader:       %s\n", resp.Leader)
	fmt.Printf("  Bloom Size:   %d bits\n", resp.BloomSize)
	fmt.Printf("  Bloom K:      %d\n", resp.BloomK)
	fmt.Printf("  Bloom Count:  ~%d items\n", resp.BloomCount)
	fmt.Printf("  Raft Port:    %d\n", resp.RaftPort)
}
