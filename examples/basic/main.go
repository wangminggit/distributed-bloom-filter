// Package main demonstrates basic usage of the Distributed Bloom Filter SDK.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/wangminggit/distributed-bloom-filter/sdk"
)

func main() {
	// Create a new client
	client, err := sdk.NewClient(sdk.ClientConfig{
		Addresses: []string{
			"localhost:7000",
			"localhost:7001",
			"localhost:7002",
		},
		APIKey:    "test-api-key",
		APISecret: "test-api-secret",
		EnableTLS: false, // Set to true in production
		Timeout:   5 * time.Second,
		MaxRetries: 3,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Println("=== Distributed Bloom Filter SDK Example ===")
	fmt.Println()

	// Example 1: Add a single item
	fmt.Println("1. Adding item 'user:123'...")
	if err := client.Add("user:123"); err != nil {
		log.Printf("Error adding item: %v", err)
	} else {
		fmt.Println("   ✓ Item added successfully")
	}

	// Example 2: Check if an item exists
	fmt.Println("\n2. Checking if 'user:123' exists...")
	exists, err := client.Contains("user:123")
	if err != nil {
		log.Printf("Error checking item: %v", err)
	} else {
		fmt.Printf("   ✓ Exists: %v\n", exists)
	}

	// Example 3: Add multiple items
	fmt.Println("\n3. Adding multiple items...")
	items := []string{"user:456", "user:789", "session:abc"}
	if err := client.BatchAdd(items); err != nil {
		log.Printf("Error batch adding: %v", err)
	} else {
		fmt.Println("   ✓ All items added successfully")
	}

	// Example 4: Batch check items
	fmt.Println("\n4. Checking multiple items...")
	checkItems := []string{"user:123", "user:456", "user:999"}
	results, err := client.BatchContains(checkItems)
	if err != nil {
		log.Printf("Error batch checking: %v", err)
	} else {
		fmt.Println("   ✓ Results:")
		for _, item := range checkItems {
			exists := results[item]
			fmt.Printf("      %s: %v\n", item, exists)
		}
	}

	// Example 5: Remove an item
	fmt.Println("\n5. Removing item 'user:456'...")
	if err := client.Remove("user:456"); err != nil {
		log.Printf("Error removing item: %v", err)
	} else {
		fmt.Println("   ✓ Item removed successfully")
	}

	// Example 6: Verify removal
	fmt.Println("\n6. Verifying removal of 'user:456'...")
	exists, err = client.Contains("user:456")
	if err != nil {
		log.Printf("Error checking item: %v", err)
	} else {
		fmt.Printf("   ✓ Exists after removal: %v\n", exists)
	}

	// Example 7: Get node status
	fmt.Println("\n7. Getting node status...")
	status, err := client.GetStatus()
	if err != nil {
		log.Printf("Error getting status: %v", err)
	} else {
		fmt.Println("   ✓ Node Status:")
		fmt.Printf("      Node ID: %s\n", status.NodeID)
		fmt.Printf("      Is Leader: %v\n", status.IsLeader)
		fmt.Printf("      Raft State: %s\n", status.RaftState)
		fmt.Printf("      Leader: %s\n", status.Leader)
		fmt.Printf("      Bloom Filter Size: %d bits\n", status.BloomSize)
		fmt.Printf("      Hash Functions (K): %d\n", status.BloomK)
		fmt.Printf("      Approximate Count: %d\n", status.BloomCount)
	}

	fmt.Println("\n=== Example Complete ===")
}
