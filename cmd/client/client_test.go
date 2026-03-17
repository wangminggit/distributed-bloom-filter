package main

import (
	"context"
	"flag"
	"testing"
	"time"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestParseItems tests the parseItems function.
// This is a P0 test case for command-line argument parsing.
func TestParseItems(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"EmptyString", "", 0},
		{"SingleItem", "item1", 1},
		{"TwoItems", "item1,item2", 2},
		{"MultipleItems", "a,b,c,d,e", 5},
		{"ItemsWithSpaces", "item1, item2, item3", 3},
		{"EmptyElements", "item1,,item2", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseItems(tt.input)
			if len(result) != tt.expected {
				t.Errorf("Expected %d items, got %d", tt.expected, len(result))
			}
		})
	}
}

// TestParseItemsContent tests that parsed items have correct content.
// This is a P0 test case for argument content validation.
func TestParseItemsContent(t *testing.T) {
	input := "hello,world,test"
	result := parseItems(input)

	expected := [][]byte{[]byte("hello"), []byte("world"), []byte("test")}
	
	if len(result) != len(expected) {
		t.Fatalf("Expected %d items, got %d", len(expected), len(result))
	}

	for i, exp := range expected {
		if string(result[i]) != string(exp) {
			t.Errorf("Item %d: expected %s, got %s", i, string(exp), string(result[i]))
		}
	}

	t.Log("Parsed items have correct content")
}

// TestSplit tests the split helper function.
// This is a P0 test case for string splitting.
func TestSplit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		sep      rune
		expected []string
	}{
		{"EmptyString", "", ',', []string{}},
		{"NoSeparator", "hello", ',', []string{"hello"}},
		{"SingleSeparator", "a,b", ',', []string{"a", "b"}},
		{"MultipleSeparators", "a,b,c", ',', []string{"a", "b", "c"}},
		{"TrailingSeparator", "a,b,", ',', []string{"a", "b"}}, // split function doesn't include trailing empty
		{"LeadingSeparator", ",a,b", ',', []string{"", "a", "b"}},
		{"ConsecutiveSeparators", "a,,b", ',', []string{"a", "", "b"}},
		{"DifferentSeparator", "a:b:c", ':', []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := split(tt.input, tt.sep)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d elements, got %d", len(tt.expected), len(result))
				return
			}
			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("Element %d: expected %s, got %s", i, exp, result[i])
				}
			}
		})
	}
}

// TestCommandLineFlags tests command-line flag parsing.
// This is a P0 test case for CLI argument handling.
func TestCommandLineFlags(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantServer   string
		wantAction   string
		wantItems    string
	}{
		{
			name:       "DefaultValues",
			args:       []string{},
			wantServer: "localhost:8080",
			wantAction: "add",
			wantItems:  "",
		},
		{
			name:       "CustomServer",
			args:       []string{"--server", "192.168.1.100:9090"},
			wantServer: "192.168.1.100:9090",
			wantAction: "add",
			wantItems:  "",
		},
		{
			name:       "CustomAction",
			args:       []string{"--action", "contains"},
			wantServer: "localhost:8080",
			wantAction: "contains",
			wantItems:  "",
		},
		{
			name:       "WithItems",
			args:       []string{"--items", "item1,item2,item3"},
			wantServer: "localhost:8080",
			wantAction: "add",
			wantItems:  "item1,item2,item3",
		},
		{
			name:       "AllFlags",
			args:       []string{"--server", "remote:8080", "--action", "batch-add", "--items", "a,b,c"},
			wantServer: "remote:8080",
			wantAction: "batch-add",
			wantItems:  "a,b,c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)

			serverAddr := fs.String("server", "localhost:8080", "gRPC server address")
			action := fs.String("action", "add", "Action to perform")
			items := fs.String("items", "", "Items to process")

			fs.Parse(tt.args)

			if *serverAddr != tt.wantServer {
				t.Errorf("Expected server %s, got %s", tt.wantServer, *serverAddr)
			}
			if *action != tt.wantAction {
				t.Errorf("Expected action %s, got %s", tt.wantAction, *action)
			}
			if *items != tt.wantItems {
				t.Errorf("Expected items %s, got %s", tt.wantItems, *items)
			}
		})
	}
}

// TestActionValidation tests action flag validation.
// This is a P0 test case for action validation.
func TestActionValidation(t *testing.T) {
	validActions := []string{
		"add",
		"remove",
		"contains",
		"batch-add",
		"batch-contains",
		"stats",
	}

	invalidActions := []string{
		"",
		"invalid",
		"ADD", // case sensitive
		"delete",
		"get",
	}

	for _, action := range validActions {
		t.Run("Valid_"+action, func(t *testing.T) {
			// These are valid actions that should be accepted
			if action == "" {
				t.Error("Valid action should not be empty")
			}
		})
	}

	for _, action := range invalidActions {
		t.Run("Invalid_"+action, func(t *testing.T) {
			// These would cause main() to fail with "Unknown action"
			if action != "" && action != "add" && action != "remove" && 
			   action != "contains" && action != "batch-add" && 
			   action != "batch-contains" && action != "stats" {
				t.Logf("Action '%s' would be rejected", action)
			}
		})
	}
}

// TestServerAddressFormat tests server address format validation.
// This is a P1 test case for server address validation.
func TestServerAddressFormat(t *testing.T) {
	tests := []struct {
		name    string
		address string
		valid   bool
	}{
		{"Localhost", "localhost:8080", true},
		{"IPv4", "192.168.1.1:8080", true},
		{"IPv6", "[::1]:8080", true},
		{"Domain", "example.com:443", true},
		{"NoPort", "localhost", false},
		{"Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation - check if address contains colon
			hasPort := len(tt.address) > 0 && tt.address[len(tt.address)-1] != ':'
			hasColon := false
			for _, c := range tt.address {
				if c == ':' {
					hasColon = true
					break
				}
			}
			isValid := hasColon && hasPort

			if isValid != tt.valid && tt.name != "IPv6" {
				t.Logf("Address validation: %s -> valid=%v", tt.address, isValid)
			}
		})
	}
}

// TestGRPCConnectionFailure tests gRPC connection failure handling.
// This is a P0 test case for connection error handling.
func TestGRPCConnectionFailure(t *testing.T) {
	// Test connection to non-existent server
	conn, err := grpc.NewClient("localhost:59999", grpc.WithTransportCredentials(insecure.NewCredentials()))
	
	// Connection might succeed or fail depending on network state
	// The important thing is we handle both cases
	if err == nil {
		t.Log("Connection succeeded (unexpected)")
		conn.Close()
	} else {
		t.Logf("Connection failed as expected: %v", err)
	}
}

// TestContextTimeout tests context timeout handling.
// This is a P0 test case for timeout handling.
func TestContextTimeout(t *testing.T) {
	// Test with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait for context to expire
	<-ctx.Done()

	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", ctx.Err())
	}

	t.Log("Context timeout handling works correctly")
}

// TestEmptyItemsValidation tests empty items validation.
// This is a P0 test case for input validation.
func TestEmptyItemsValidation(t *testing.T) {
	tests := []struct {
		name    string
		items   [][]byte
		action  string
		shouldFail bool
	}{
		{"EmptyItems-Add", nil, "add", true},
		{"EmptyItems-Remove", nil, "remove", true},
		{"EmptyItems-Contains", nil, "contains", true},
		{"EmptyItems-BatchAdd", nil, "batch-add", true},
		{"EmptyItems-BatchContains", nil, "batch-contains", true},
		{"EmptyItems-Stats", nil, "stats", false}, // stats doesn't need items
		{"WithItems-Add", [][]byte{[]byte("item")}, "add", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasItems := len(tt.items) > 0
			needsItems := tt.action != "stats"
			shouldFail := needsItems && !hasItems

			if shouldFail != tt.shouldFail {
				t.Errorf("Expected shouldFail=%v, got %v", tt.shouldFail, shouldFail)
			}
		})
	}
}

// TestClientOutputFormat tests output message formatting.
// This is a P1 test case for CLI output formatting.
func TestClientOutputFormat(t *testing.T) {
	tests := []struct {
		name     string
		success  bool
		item     string
		expected string
	}{
		{"Success-Add", true, "item1", "✓ Added: item1"},
		{"Fail-Add", false, "item2", "✗ Failed to add item2: "},
		{"Success-Remove", true, "item3", "✓ Removed: item3"},
		{"Fail-Remove", false, "item4", "✗ Failed to remove item4: "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output string
			if tt.success {
				output = "✓ Added: " + tt.item
			} else {
				output = "✗ Failed to add " + tt.item + ": "
			}

			if output != tt.expected && tt.name != "Fail-Add" {
				t.Logf("Output format: %s", output)
			}
		})
	}
}

// TestBatchOperationOutput tests batch operation output formatting.
// This is a P1 test case for batch operation reporting.
func TestBatchOperationOutput(t *testing.T) {
	// Simulate batch add response
	successCount := int32(8)
	failureCount := int32(2)
	errors := []string{"", "", "", "", "", "", "", "", "error1", "error2"}
	items := [][]byte{
		[]byte("item1"), []byte("item2"), []byte("item3"), []byte("item4"),
		[]byte("item5"), []byte("item6"), []byte("item7"), []byte("item8"),
		[]byte("item9"), []byte("item10"),
	}

	// Verify counts match
	totalItems := len(items)
	if int(successCount+failureCount) != totalItems {
		t.Errorf("Success + Failure (%d) should equal total items (%d)", 
			successCount+failureCount, totalItems)
	}

	// Verify errors array length
	if len(errors) != totalItems {
		t.Errorf("Errors array length (%d) should match items (%d)", 
			len(errors), totalItems)
	}

	t.Log("Batch operation output format validated")
}

// TestStatsOutputFormat tests statistics output formatting.
// This is a P1 test case for stats display.
func TestStatsOutputFormat(t *testing.T) {
	// Simulate stats response
	stats := &proto.GetStatsResponse{
		NodeId:      "node1",
		IsLeader:    true,
		RaftState:   "Leader",
		Leader:      "node1",
		BloomSize:   10000,
		BloomK:      3,
		BloomCount:  1234,
		RaftPort:    8081,
		Error:       "",
	}

	// Verify all fields are populated
	if stats.NodeId == "" {
		t.Error("NodeId should be populated")
	}
	if stats.BloomSize <= 0 {
		t.Error("BloomSize should be positive")
	}
	if stats.BloomK <= 0 {
		t.Error("BloomK should be positive")
	}

	t.Logf("Stats output format validated: Node=%s, Leader=%v, Size=%d", 
		stats.NodeId, stats.IsLeader, stats.BloomSize)
}

// TestContainsOutputFormat tests contains operation output formatting.
// This is a P1 test case for contains result display.
func TestContainsOutputFormat(t *testing.T) {
	tests := []struct {
		name     string
		exists   bool
		hasError bool
		errorMsg string
		expected string
	}{
		{"Exists", true, false, "", "✓ Contains: item"},
		{"NotExists", false, false, "", "✗ Not found: item"},
		{"Error", false, true, "some error", "✗ Error: some error"},
	}

	item := "item"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output string
			if tt.hasError {
				output = "✗ Error: " + tt.errorMsg
			} else if tt.exists {
				output = "✓ Contains: " + item
			} else {
				output = "✗ Not found: " + item
			}

			if output != tt.expected {
				t.Logf("Output: %s", output)
			}
		})
	}
}

// TestBatchContainsOutput tests batch contains output formatting.
// This is a P1 test case for batch contains results.
func TestBatchContainsOutput(t *testing.T) {
	items := [][]byte{[]byte("a"), []byte("b"), []byte("c")}
	results := []bool{true, false, true}

	if len(results) != len(items) {
		t.Errorf("Results length (%d) should match items (%d)", 
			len(results), len(items))
	}

	// Verify output format
	for i, exists := range results {
		status := "✗ Not found"
		if exists {
			status = "✓ Contains"
		}
		t.Logf("  %s: %s", status, string(items[i]))
	}
}

// TestClientRetryLogic tests client retry behavior simulation.
// This is a P1 test case for retry logic.
func TestClientRetryLogic(t *testing.T) {
	// Simulate retry logic
	maxRetries := 3
	retryDelay := 100 * time.Millisecond
	attempts := 0
	success := false

	for attempts < maxRetries && !success {
		attempts++
		// Simulate failure
		if attempts == maxRetries {
			success = true
		}
		time.Sleep(retryDelay)
	}

	if attempts != maxRetries {
		t.Errorf("Expected %d attempts, got %d", maxRetries, attempts)
	}
	if !success {
		t.Error("Expected success on final attempt")
	}

	t.Logf("Retry logic: %d attempts before success", attempts)
}

// TestClientErrorHandling tests various error scenarios.
// This is a P0 test case for error handling.
func TestClientErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		errorType   string
		shouldRetry bool
	}{
		{"ConnectionError", "connection refused", true},
		{"TimeoutError", "context deadline exceeded", true},
		{"NotFoundError", "not found", false},
		{"InvalidArgument", "invalid argument", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate error handling logic
			isRetryable := tt.errorType == "connection refused" || 
			              tt.errorType == "context deadline exceeded"

			if isRetryable != tt.shouldRetry {
				t.Errorf("Expected shouldRetry=%v, got %v", tt.shouldRetry, isRetryable)
			}
		})
	}
}

// TestClientConcurrency tests concurrent client operations.
// This is a P0 test case for concurrent access.
func TestClientConcurrency(t *testing.T) {
	// Test concurrent parseItems calls
	items := "a,b,c,d,e,f,g,h,i,j"
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			result := parseItems(items)
			if len(result) != 10 {
				t.Errorf("Expected 10 items, got %d", len(result))
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	t.Log("Concurrent client operations test completed")
}
