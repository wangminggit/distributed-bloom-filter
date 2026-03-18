// Package sdk provides a Go client library for the Distributed Bloom Filter service.
package sdk

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
)

// ClientConfig holds configuration for the DBF client.
type ClientConfig struct {
	// Addresses is a list of gRPC server addresses to connect to.
	// The client will try these addresses in order and handle leader redirection.
	Addresses []string

	// APIKey is the API key for authentication.
	APIKey string

	// APISecret is the secret for signing requests.
	APISecret string

	// TLSCertFile is the path to the TLS certificate file.
	// If empty, insecure connection will be used.
	TLSCertFile string

	// EnableTLS enables TLS encryption.
	EnableTLS bool

	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int

	// RetryDelay is the delay between retries.
	RetryDelay time.Duration

	// Timeout is the default timeout for operations.
	Timeout time.Duration
}

// DefaultClientConfig returns a ClientConfig with sensible defaults.
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
		Timeout:    5 * time.Second,
	}
}

// Client is the main client for interacting with the Distributed Bloom Filter service.
type Client struct {
	config ClientConfig
	mu     sync.RWMutex

	// Current connection
	conn   *grpc.ClientConn
	client proto.DBFServiceClient

	// Connection pool
	connections map[string]*grpc.ClientConn

	// Current leader address
	leaderAddress string
}

// NewClient creates a new DBF client.
func NewClient(config ClientConfig) (*Client, error) {
	if len(config.Addresses) == 0 {
		return nil, fmt.Errorf("no addresses provided")
	}

	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 100 * time.Millisecond
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}

	c := &Client{
		config:      config,
		connections: make(map[string]*grpc.ClientConn),
	}

	// Try to connect to the first available address
	if err := c.connectToFirstAvailable(); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return c, nil
}

// connectToFirstAvailable tries to connect to the first available server.
func (c *Client) connectToFirstAvailable() error {
	for _, addr := range c.config.Addresses {
		if err := c.connect(addr); err == nil {
			return nil
		}
	}
	return fmt.Errorf("failed to connect to any address")
}

// connect establishes a connection to a specific address.
func (c *Client) connect(address string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we already have a connection
	if conn, ok := c.connections[address]; ok {
		c.conn = conn
		c.client = proto.NewDBFServiceClient(conn)
		return nil
	}

	// Create new connection
	var opts []grpc.DialOption
	if c.config.EnableTLS && c.config.TLSCertFile != "" {
		creds, err := credentials.NewClientTLSFromFile(c.config.TLSCertFile, "")
		if err != nil {
			return fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.Dial(address, opts...)
	if err != nil {
		return fmt.Errorf("failed to dial %s: %w", address, err)
	}

	c.connections[address] = conn
	c.conn = conn
	c.client = proto.NewDBFServiceClient(conn)

	return nil
}

// Add adds an item to the Bloom filter.
func (c *Client) Add(key string) error {
	return c.AddWithTimeout(key, c.config.Timeout)
}

// AddWithTimeout adds an item with a custom timeout.
func (c *Client) AddWithTimeout(key string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req := &proto.AddRequest{
		Item: []byte(key),
	}

	return c.executeWithRetry(ctx, func(client proto.DBFServiceClient) error {
		resp, err := client.Add(ctx, req)
		if err != nil {
			return err
		}
		if !resp.Success {
			if isLeaderRedirect(resp.Error) {
				return ErrLeaderRedirect{Message: resp.Error}
			}
			return fmt.Errorf("add failed: %s", resp.Error)
		}
		return nil
	})
}

// Remove removes an item from the Bloom filter.
func (c *Client) Remove(key string) error {
	return c.RemoveWithTimeout(key, c.config.Timeout)
}

// RemoveWithTimeout removes an item with a custom timeout.
func (c *Client) RemoveWithTimeout(key string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req := &proto.RemoveRequest{
		Item: []byte(key),
	}

	return c.executeWithRetry(ctx, func(client proto.DBFServiceClient) error {
		resp, err := client.Remove(ctx, req)
		if err != nil {
			return err
		}
		if !resp.Success {
			if isLeaderRedirect(resp.Error) {
				return ErrLeaderRedirect{Message: resp.Error}
			}
			return fmt.Errorf("remove failed: %s", resp.Error)
		}
		return nil
	})
}

// Contains checks if an item exists in the Bloom filter.
func (c *Client) Contains(key string) (bool, error) {
	return c.ContainsWithTimeout(key, c.config.Timeout)
}

// ContainsWithTimeout checks if an item exists with a custom timeout.
func (c *Client) ContainsWithTimeout(key string, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req := &proto.ContainsRequest{
		Item: []byte(key),
	}

	var exists bool
	err := c.executeWithRetry(ctx, func(client proto.DBFServiceClient) error {
		resp, err := client.Contains(ctx, req)
		if err != nil {
			return err
		}
		if resp.Error != "" {
			return fmt.Errorf("contains failed: %s", resp.Error)
		}
		exists = resp.Exists
		return nil
	})

	return exists, err
}

// BatchAdd adds multiple items to the Bloom filter.
func (c *Client) BatchAdd(keys []string) error {
	return c.BatchAddWithTimeout(keys, c.config.Timeout)
}

// BatchAddWithTimeout adds multiple items with a custom timeout.
func (c *Client) BatchAddWithTimeout(keys []string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	items := make([][]byte, len(keys))
	for i, key := range keys {
		items[i] = []byte(key)
	}

	req := &proto.BatchAddRequest{
		Items: items,
	}

	return c.executeWithRetry(ctx, func(client proto.DBFServiceClient) error {
		resp, err := client.BatchAdd(ctx, req)
		if err != nil {
			return err
		}
		if resp.FailureCount == int32(len(keys)) && len(resp.Errors) > 0 {
			if isLeaderRedirect(resp.Errors[0]) {
				return ErrLeaderRedirect{Message: resp.Errors[0]}
			}
		}
		if resp.FailureCount > 0 {
			return fmt.Errorf("batch add partially failed: %d successes, %d failures",
				resp.SuccessCount, resp.FailureCount)
		}
		return nil
	})
}

// BatchContains checks if multiple items exist in the Bloom filter.
func (c *Client) BatchContains(keys []string) (map[string]bool, error) {
	return c.BatchContainsWithTimeout(keys, c.config.Timeout)
}

// BatchContainsWithTimeout checks if multiple items exist with a custom timeout.
func (c *Client) BatchContainsWithTimeout(keys []string, timeout time.Duration) (map[string]bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	items := make([][]byte, len(keys))
	for i, key := range keys {
		items[i] = []byte(key)
	}

	req := &proto.BatchContainsRequest{
		Items: items,
	}

	results := make(map[string]bool)
	err := c.executeWithRetry(ctx, func(client proto.DBFServiceClient) error {
		resp, err := client.BatchContains(ctx, req)
		if err != nil {
			return err
		}
		if resp.Error != "" {
			return fmt.Errorf("batch contains failed: %s", resp.Error)
		}
		for i, key := range keys {
			if i < len(resp.Results) {
				results[key] = resp.Results[i]
			}
		}
		return nil
	})

	return results, err
}

// GetStatus returns the current status of the node.
func (c *Client) GetStatus() (*NodeStatus, error) {
	return c.GetStatusWithTimeout(c.config.Timeout)
}

// GetStatusWithTimeout returns the status with a custom timeout.
func (c *Client) GetStatusWithTimeout(timeout time.Duration) (*NodeStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req := &proto.GetStatsRequest{}

	var status *NodeStatus
	err := c.executeWithRetry(ctx, func(client proto.DBFServiceClient) error {
		resp, err := client.GetStats(ctx, req)
		if err != nil {
			return err
		}
		if resp.Error != "" {
			return fmt.Errorf("get stats failed: %s", resp.Error)
		}
		status = &NodeStatus{
			NodeID:     resp.NodeId,
			IsLeader:   resp.IsLeader,
			RaftState:  resp.RaftState,
			Leader:     resp.Leader,
			BloomSize:  resp.BloomSize,
			BloomK:     resp.BloomK,
			BloomCount: resp.BloomCount,
			RaftPort:   resp.RaftPort,
		}
		return nil
	})

	return status, err
}

// Close closes all connections.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var lastErr error
	for _, conn := range c.connections {
		if err := conn.Close(); err != nil {
			lastErr = err
		}
	}
	c.connections = make(map[string]*grpc.ClientConn)
	c.conn = nil
	c.client = nil

	return lastErr
}

// executeWithRetry executes a function with retry logic and leader redirection.
func (c *Client) executeWithRetry(ctx context.Context, fn func(proto.DBFServiceClient) error) error {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		c.mu.RLock()
		client := c.client
		c.mu.RUnlock()

		if client == nil {
			return fmt.Errorf("no connection available")
		}

		err := fn(client)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we need to redirect to leader
		var redirectErr ErrLeaderRedirect
		if As(err, &redirectErr) {
			if err := c.switchToLeader(redirectErr.Message); err != nil {
				return fmt.Errorf("failed to switch to leader: %w", err)
			}
			continue
		}

		// For other errors, retry with backoff
		if attempt < c.config.MaxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.config.RetryDelay * time.Duration(attempt+1)):
				// Retry
			}
		}
	}

	return lastErr
}

// switchToLeader switches the connection to the leader node.
func (c *Client) switchToLeader(errorMsg string) error {
	// Parse leader address from error message
	// Format: "not the leader, redirect to: node-1 (127.0.0.1:7000)"
	leaderAddr := extractLeaderAddress(errorMsg)
	if leaderAddr == "" {
		return fmt.Errorf("could not extract leader address from: %s", errorMsg)
	}

	c.mu.Lock()
	c.leaderAddress = leaderAddr
	c.mu.Unlock()

	// Try to connect to the leader
	if err := c.connect(leaderAddr); err != nil {
		return err
	}

	return nil
}

// createAuthMetadata creates authentication metadata for gRPC requests.
func (c *Client) createAuthMetadata(method string) *proto.AuthMetadata {
	timestamp := time.Now().Unix()
	signature := c.computeSignature(c.config.APIKey, timestamp, method, c.config.APISecret)

	return &proto.AuthMetadata{
		ApiKey:    c.config.APIKey,
		Timestamp: timestamp,
		Signature: signature,
	}
}

// computeSignature computes HMAC-SHA256 signature.
func (c *Client) computeSignature(apiKey string, timestamp int64, method, secret string) string {
	message := fmt.Sprintf("%s%d%s", apiKey, timestamp, method)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// NodeStatus represents the status of a DBF node.
type NodeStatus struct {
	NodeID     string
	IsLeader   bool
	RaftState  string
	Leader     string
	BloomSize  int64
	BloomK     int32
	BloomCount int64
	RaftPort   int32
}

// ErrLeaderRedirect is returned when a request needs to be redirected to the leader.
type ErrLeaderRedirect struct {
	Message string
}

func (e ErrLeaderRedirect) Error() string {
	return e.Message
}

// As implements errors.As interface for type assertion.
func As(err error, target interface{}) bool {
	switch t := target.(type) {
	case *ErrLeaderRedirect:
		if e, ok := err.(ErrLeaderRedirect); ok {
			*t = e
			return true
		}
	}
	return false
}

// isLeaderRedirect checks if an error message indicates a leader redirect.
func isLeaderRedirect(msg string) bool {
	if len(msg) < 14 {
		return false
	}
	prefix := msg[:14]
	return prefix == "not the leader" || prefix == "Not the leader"
}

// extractLeaderAddress extracts the leader address from an error message.
func extractLeaderAddress(errorMsg string) string {
	// Format: "not the leader, redirect to: node-1 (127.0.0.1:7000)"
	// Try to extract address in parentheses
	start := -1
	end := -1
	for i, ch := range errorMsg {
		if ch == '(' {
			start = i + 1
		} else if ch == ')' && start != -1 {
			end = i
			break
		}
	}
	
	if start != -1 && end != -1 && end > start {
		return errorMsg[start:end]
	}
	
	return ""
}
