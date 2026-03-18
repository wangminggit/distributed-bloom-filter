package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
)

// GatewayConfig holds configuration for the HTTP gateway.
type GatewayConfig struct {
	// Port is the HTTP port to listen on.
	Port int

	// GRPCAddress is the address of the gRPC server.
	GRPCAddress string

	// TLSCertFile is the path to the TLS certificate file (for gRPC client).
	TLSCertFile string

	// EnableTLS enables TLS for gRPC client connection.
	EnableTLS bool

	// APIKey is the API key for authenticating with the gRPC server.
	APIKey string

	// APISecret is the secret for signing requests.
	APISecret string

	// ReadTimeout is the maximum duration for reading the entire request.
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the response.
	WriteTimeout time.Duration
}

// APIResponse is the standard response format for the HTTP API.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Leader  string      `json:"leader,omitempty"`
}

// Gateway is the HTTP gateway for the gRPC service.
type Gateway struct {
	grpcClient proto.DBFServiceClient
	conn       *grpc.ClientConn
	config     GatewayConfig
	server     *http.Server
}

// NewGateway creates a new HTTP gateway.
func NewGateway(config GatewayConfig) (*Gateway, error) {
	// Create gRPC client connection
	var opts []grpc.DialOption
	if config.EnableTLS {
		creds, err := credentials.NewClientTLSFromFile(config.TLSCertFile, "")
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.Dial(config.GRPCAddress, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %w", err)
	}

	client := proto.NewDBFServiceClient(conn)

	g := &Gateway{
		grpcClient: client,
		conn:       conn,
		config:     config,
	}

	return g, nil
}

// Start starts the HTTP gateway server.
func (g *Gateway) Start() error {
	mux := http.NewServeMux()

	// Register handlers
	mux.HandleFunc("/api/v1/add", g.handleAdd)
	mux.HandleFunc("/api/v1/remove", g.handleRemove)
	mux.HandleFunc("/api/v1/contains", g.handleContains)
	mux.HandleFunc("/api/v1/batch/add", g.handleBatchAdd)
	mux.HandleFunc("/api/v1/batch/contains", g.handleBatchContains)
	mux.HandleFunc("/api/v1/status", g.handleStatus)
	mux.HandleFunc("/api/v1/cluster", g.handleCluster)

	g.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", g.config.Port),
		Handler:      g.loggingMiddleware(g.corsMiddleware(mux)),
		ReadTimeout:  g.config.ReadTimeout,
		WriteTimeout: g.config.WriteTimeout,
	}

	log.Printf("HTTP Gateway starting on port %d", g.config.Port)
	if err := g.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

// Stop gracefully stops the HTTP gateway server.
func (g *Gateway) Stop() error {
	if g.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := g.server.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown HTTP server: %w", err)
		}
	}
	if g.conn != nil {
		g.conn.Close()
	}
	return nil
}

// handleAdd handles POST /api/v1/add
func (g *Gateway) handleAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		g.sendError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Item string `json:"item"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.sendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Item == "" {
		g.sendError(w, "item cannot be empty", http.StatusBadRequest)
		return
	}

	// Create gRPC request
	grpcReq := &proto.AddRequest{
		Item: []byte(req.Item),
	}

	// Call gRPC service
	resp, err := g.grpcClient.Add(context.Background(), grpcReq)
	if err != nil {
		g.sendError(w, fmt.Sprintf("gRPC error: %v", err), http.StatusInternalServerError)
		return
	}

	if !resp.Success {
		// Check if it's a leader redirect
		if strings.Contains(resp.Error, "not the leader") {
			g.sendRedirect(w, resp.Error)
			return
		}
		g.sendError(w, resp.Error, http.StatusInternalServerError)
		return
	}

	g.sendSuccess(w, map[string]bool{"added": true}, "")
}

// handleRemove handles DELETE /api/v1/remove
func (g *Gateway) handleRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		g.sendError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Item string `json:"item"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.sendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Item == "" {
		g.sendError(w, "item cannot be empty", http.StatusBadRequest)
		return
	}

	// Create gRPC request
	grpcReq := &proto.RemoveRequest{
		Item: []byte(req.Item),
	}

	// Call gRPC service
	resp, err := g.grpcClient.Remove(context.Background(), grpcReq)
	if err != nil {
		g.sendError(w, fmt.Sprintf("gRPC error: %v", err), http.StatusInternalServerError)
		return
	}

	if !resp.Success {
		if strings.Contains(resp.Error, "not the leader") {
			g.sendRedirect(w, resp.Error)
			return
		}
		g.sendError(w, resp.Error, http.StatusInternalServerError)
		return
	}

	g.sendSuccess(w, map[string]bool{"removed": true}, "")
}

// handleContains handles POST /api/v1/contains
func (g *Gateway) handleContains(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		g.sendError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Item string `json:"item"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.sendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Item == "" {
		g.sendError(w, "item cannot be empty", http.StatusBadRequest)
		return
	}

	// Create gRPC request
	grpcReq := &proto.ContainsRequest{
		Item: []byte(req.Item),
	}

	// Call gRPC service
	resp, err := g.grpcClient.Contains(context.Background(), grpcReq)
	if err != nil {
		g.sendError(w, fmt.Sprintf("gRPC error: %v", err), http.StatusInternalServerError)
		return
	}

	if resp.Error != "" {
		g.sendError(w, resp.Error, http.StatusInternalServerError)
		return
	}

	g.sendSuccess(w, map[string]bool{"exists": resp.Exists}, "")
}

// handleBatchAdd handles POST /api/v1/batch/add
func (g *Gateway) handleBatchAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		g.sendError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Items []string `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.sendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Items) == 0 {
		g.sendError(w, "no items provided", http.StatusBadRequest)
		return
	}

	// Convert to bytes
	items := make([][]byte, len(req.Items))
	for i, item := range req.Items {
		items[i] = []byte(item)
	}

	// Create gRPC request
	grpcReq := &proto.BatchAddRequest{
		Items: items,
	}

	// Call gRPC service
	resp, err := g.grpcClient.BatchAdd(context.Background(), grpcReq)
	if err != nil {
		g.sendError(w, fmt.Sprintf("gRPC error: %v", err), http.StatusInternalServerError)
		return
	}

	if resp.FailureCount == int32(len(req.Items)) && len(resp.Errors) > 0 {
		if strings.Contains(resp.Errors[0], "not the leader") {
			g.sendRedirect(w, resp.Errors[0])
			return
		}
	}

	g.sendSuccess(w, map[string]interface{}{
		"success_count": resp.SuccessCount,
		"failure_count": resp.FailureCount,
		"errors":        resp.Errors,
	}, "")
}

// handleBatchContains handles POST /api/v1/batch/contains
func (g *Gateway) handleBatchContains(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		g.sendError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Items []string `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.sendError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Items) == 0 {
		g.sendError(w, "no items provided", http.StatusBadRequest)
		return
	}

	// Convert to bytes
	items := make([][]byte, len(req.Items))
	for i, item := range req.Items {
		items[i] = []byte(item)
	}

	// Create gRPC request
	grpcReq := &proto.BatchContainsRequest{
		Items: items,
	}

	// Call gRPC service
	resp, err := g.grpcClient.BatchContains(context.Background(), grpcReq)
	if err != nil {
		g.sendError(w, fmt.Sprintf("gRPC error: %v", err), http.StatusInternalServerError)
		return
	}

	if resp.Error != "" {
		g.sendError(w, resp.Error, http.StatusInternalServerError)
		return
	}

	// Convert results to map
	results := make(map[string]bool)
	for i, item := range req.Items {
		if i < len(resp.Results) {
			results[item] = resp.Results[i]
		}
	}

	g.sendSuccess(w, map[string]interface{}{
		"results": results,
	}, "")
}

// handleStatus handles GET /api/v1/status
func (g *Gateway) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		g.sendError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Create gRPC request
	grpcReq := &proto.GetStatsRequest{}

	// Call gRPC service
	resp, err := g.grpcClient.GetStats(context.Background(), grpcReq)
	if err != nil {
		g.sendError(w, fmt.Sprintf("gRPC error: %v", err), http.StatusInternalServerError)
		return
	}

	if resp.Error != "" {
		g.sendError(w, resp.Error, http.StatusInternalServerError)
		return
	}

	g.sendSuccess(w, map[string]interface{}{
		"node_id":     resp.NodeId,
		"is_leader":   resp.IsLeader,
		"raft_state":  resp.RaftState,
		"leader":      resp.Leader,
		"bloom_size":  resp.BloomSize,
		"bloom_k":     resp.BloomK,
		"bloom_count": resp.BloomCount,
		"raft_port":   resp.RaftPort,
	}, "")
}

// handleCluster handles GET /api/v1/cluster
func (g *Gateway) handleCluster(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		g.sendError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// For now, return the same info as status
	// In a full implementation, this would return cluster topology
	grpcReq := &proto.GetStatsRequest{}

	resp, err := g.grpcClient.GetStats(context.Background(), grpcReq)
	if err != nil {
		g.sendError(w, fmt.Sprintf("gRPC error: %v", err), http.StatusInternalServerError)
		return
	}

	if resp.Error != "" {
		g.sendError(w, resp.Error, http.StatusInternalServerError)
		return
	}

	g.sendSuccess(w, map[string]interface{}{
		"cluster_info": map[string]interface{}{
			"node_id":    resp.NodeId,
			"is_leader":  resp.IsLeader,
			"leader":     resp.Leader,
			"raft_state": resp.RaftState,
		},
	}, "")
}

// createAuthMetadata creates authentication metadata for gRPC requests.
func (g *Gateway) createAuthMetadata(method string) *proto.AuthMetadata {
	timestamp := time.Now().Unix()
	
	// Create signature: HMAC-SHA256(apiKey + timestamp + method, secret)
	signature := g.computeSignature(g.config.APIKey, timestamp, method, g.config.APISecret)

	return &proto.AuthMetadata{
		ApiKey:    g.config.APIKey,
		Timestamp: timestamp,
		Signature: signature,
	}
}

// computeSignature computes HMAC-SHA256 signature.
func (g *Gateway) computeSignature(apiKey string, timestamp int64, method, secret string) string {
	message := fmt.Sprintf("%s%d%s", apiKey, timestamp, method)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// sendSuccess sends a success response.
func (g *Gateway) sendSuccess(w http.ResponseWriter, data interface{}, leader string) {
	w.Header().Set("Content-Type", "application/json")
	resp := APIResponse{
		Success: true,
		Data:    data,
		Leader:  leader,
	}
	json.NewEncoder(w).Encode(resp)
}

// sendError sends an error response.
func (g *Gateway) sendError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp := APIResponse{
		Success: false,
		Error:   message,
	}
	json.NewEncoder(w).Encode(resp)
}

// sendRedirect sends a redirect response for non-leader nodes.
func (g *Gateway) sendRedirect(w http.ResponseWriter, errorMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTemporaryRedirect)
	resp := APIResponse{
		Success: false,
		Error:   errorMsg,
	}
	json.NewEncoder(w).Encode(resp)
}

// loggingMiddleware logs HTTP requests.
func (g *Gateway) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
		log.Printf("%s %s completed in %v", r.Method, r.URL.Path, time.Since(start))
	})
}

// corsMiddleware adds CORS headers.
func (g *Gateway) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}
