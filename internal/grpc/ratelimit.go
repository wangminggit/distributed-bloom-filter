package grpc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// TrustedProxies holds the list of trusted proxy IP addresses/CIDR ranges.
// Only requests from these proxies will have X-Forwarded-For headers trusted.
var TrustedProxies []string

// trustedProxyNets is the parsed version of TrustedProxies for efficient lookup.
var trustedProxyNets []*net.IPNet

// trustedProxyOnce ensures initialization happens only once.
var trustedProxyOnce sync.Once

const (
	// defaultRateLimit is the default rate limit (requests per second).
	defaultRateLimit = 100

	// defaultBurstSize is the default burst size for rate limiting.
	defaultBurstSize = 200

	// perClientRateLimit is the default per-client rate limit.
	perClientRateLimit = 20

	// perClientBurstSize is the default per-client burst size.
	perClientBurstSize = 40

	// clientCleanupInterval is how often to clean up inactive client limiters.
	clientCleanupInterval = 5 * time.Minute

	// clientLimiterTimeout is how long to keep a client limiter before cleanup.
	clientLimiterTimeout = 10 * time.Minute
)

// initTrustedProxies initializes the trusted proxy networks from the TrustedProxies list.
// This should be called once at startup. If TrustedProxies is empty, it defaults to
// trusting only localhost and private network ranges.
func initTrustedProxies() {
	trustedProxyNets = make([]*net.IPNet, 0)

	// If no trusted proxies configured, default to localhost and private networks
	if len(TrustedProxies) == 0 {
		// Default trusted ranges: localhost and private networks
		defaultRanges := []string{
			"127.0.0.0/8",     // IPv4 localhost
			"::1/128",         // IPv6 localhost
			"10.0.0.0/8",      // Private network
			"172.16.0.0/12",   // Private network
			"192.168.0.0/16",  // Private network
			"fc00::/7",        // Unique local address
			"fe80::/10",       // Link-local address
		}
		for _, cidr := range defaultRanges {
			_, network, err := net.ParseCIDR(cidr)
			if err == nil {
				trustedProxyNets = append(trustedProxyNets, network)
			}
		}
		return
	}

	// Parse configured trusted proxies
	for _, proxy := range TrustedProxies {
		// Check if it's a CIDR range
		if strings.Contains(proxy, "/") {
			_, network, err := net.ParseCIDR(proxy)
			if err == nil {
				trustedProxyNets = append(trustedProxyNets, network)
			}
		} else {
			// Single IP address - convert to /32 or /128
			ip := net.ParseIP(proxy)
			if ip != nil {
				if ip.To4() != nil {
					// IPv4
					_, network, _ := net.ParseCIDR(proxy + "/32")
					if network != nil {
						trustedProxyNets = append(trustedProxyNets, network)
					}
				} else {
					// IPv6
					_, network, _ := net.ParseCIDR(proxy + "/128")
					if network != nil {
						trustedProxyNets = append(trustedProxyNets, network)
					}
				}
			}
		}
	}
}

// isTrustedProxy checks if the given IP address is from a trusted proxy.
func isTrustedProxy(ip net.IP) bool {
	trustedProxyOnce.Do(initTrustedProxies)

	for _, network := range trustedProxyNets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// RateLimitInterceptor implements rate limiting for gRPC requests.
// It provides both global and per-client rate limiting using the token bucket algorithm.
type RateLimitInterceptor struct {
	globalLimiter   *rate.Limiter
	clientLimiters  map[string]*rate.Limiter
	clientMu        sync.RWMutex
	clientLastSeen  map[string]time.Time
	clientLastSeenMu sync.RWMutex
	stopChan        chan struct{}
	stopOnce        sync.Once // ensures Stop is only called once
	
	// Configuration
	rps              int
	burstSize        int
	perClientRPS     int
	perClientBurst   int
	enablePerClient  bool
}

// RateLimitConfig holds configuration for rate limiting.
type RateLimitConfig struct {
	// RequestsPerSecond is the global rate limit (requests per second).
	RequestsPerSecond int

	// BurstSize is the global burst size.
	BurstSize int

	// PerClientRPS is the per-client rate limit (0 to disable).
	PerClientRPS int

	// PerClientBurst is the per-client burst size.
	PerClientBurst int

	// EnablePerClient enables per-client rate limiting.
	EnablePerClient bool
}

// NewRateLimitInterceptor creates a new rate limiting interceptor with default settings.
// For backward compatibility, per-client rate limiting is disabled by default.
// Use NewRateLimitInterceptorWithConfig to enable per-client limiting.
func NewRateLimitInterceptor(requestsPerSecond int, burstSize int) *RateLimitInterceptor {
	config := RateLimitConfig{
		RequestsPerSecond: requestsPerSecond,
		BurstSize:         burstSize,
		PerClientRPS:      perClientRateLimit,
		PerClientBurst:    perClientBurstSize,
		EnablePerClient:   false, // Disabled by default for backward compatibility
	}
	return NewRateLimitInterceptorWithConfig(config)
}

// NewRateLimitInterceptorWithConfig creates a new rate limiting interceptor with custom configuration.
func NewRateLimitInterceptorWithConfig(config RateLimitConfig) *RateLimitInterceptor {
	// Ensure valid values
	if config.RequestsPerSecond <= 0 {
		config.RequestsPerSecond = defaultRateLimit
	}
	if config.BurstSize <= 0 {
		config.BurstSize = defaultBurstSize
	}
	if config.PerClientRPS <= 0 {
		config.PerClientRPS = perClientRateLimit
	}
	if config.PerClientBurst <= 0 {
		config.PerClientBurst = perClientBurstSize
	}

	interceptor := &RateLimitInterceptor{
		globalLimiter:  rate.NewLimiter(rate.Limit(config.RequestsPerSecond), config.BurstSize),
		clientLimiters: make(map[string]*rate.Limiter),
		clientLastSeen: make(map[string]time.Time),
		stopChan:       make(chan struct{}),
		rps:            config.RequestsPerSecond,
		burstSize:      config.BurstSize,
		perClientRPS:   config.PerClientRPS,
		perClientBurst: config.PerClientBurst,
		enablePerClient: config.EnablePerClient,
	}

	// Start periodic cleanup of inactive client limiters
	go interceptor.periodicCleanup()

	return interceptor
}

// Stop stops the background cleanup goroutine.
// Safe to call multiple times.
func (r *RateLimitInterceptor) Stop() {
	r.stopOnce.Do(func() {
		close(r.stopChan)
	})
}

// periodicCleanup periodically removes inactive client limiters to prevent memory leaks.
func (r *RateLimitInterceptor) periodicCleanup() {
	ticker := time.NewTicker(clientCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.cleanupInactiveClients()
		case <-r.stopChan:
			return
		}
	}
}

// cleanupInactiveClients removes client limiters that haven't been used recently.
func (r *RateLimitInterceptor) cleanupInactiveClients() {
	now := time.Now()
	cutoff := now.Add(-clientLimiterTimeout)

	r.clientMu.Lock()
	defer r.clientMu.Unlock()

	r.clientLastSeenMu.RLock()
	defer r.clientLastSeenMu.RUnlock()

	for clientID, lastSeen := range r.clientLastSeen {
		if lastSeen.Before(cutoff) {
			delete(r.clientLimiters, clientID)
			delete(r.clientLastSeen, clientID)
		}
	}
}

// UnaryInterceptor returns a gRPC unary server interceptor that enforces rate limiting.
func (r *RateLimitInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Check global rate limit first
		if !r.globalLimiter.Allow() {
			return nil, status.Error(codes.ResourceExhausted, "global rate limit exceeded")
		}

		// Check per-client rate limit if enabled
		if r.enablePerClient {
			clientID := r.getClientID(ctx)
			if clientID != "" {
				if !r.allowClient(clientID) {
					return nil, status.Error(codes.ResourceExhausted, fmt.Sprintf("per-client rate limit exceeded for %s", clientID))
				}
			}
		}

		// Proceed with the handler
		return handler(ctx, req)
	}
}

// StreamInterceptor returns a gRPC stream server interceptor that enforces rate limiting.
func (r *RateLimitInterceptor) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Check global rate limit
		if !r.globalLimiter.Allow() {
			return status.Error(codes.ResourceExhausted, "global rate limit exceeded")
		}

		// Check per-client rate limit if enabled
		if r.enablePerClient {
			clientID := r.getClientID(ss.Context())
			if clientID != "" {
				if !r.allowClient(clientID) {
					return status.Error(codes.ResourceExhausted, fmt.Sprintf("per-client rate limit exceeded for %s", clientID))
				}
			}
		}

		// Proceed with the handler
		return handler(srv, ss)
	}
}

// allowClient checks if a client is allowed to make a request based on their rate limit.
func (r *RateLimitInterceptor) allowClient(clientID string) bool {
	r.clientMu.RLock()
	limiter, exists := r.clientLimiters[clientID]
	r.clientMu.RUnlock()

	if !exists {
		// Create new limiter for this client
		r.clientMu.Lock()
		// Double-check after acquiring write lock
		limiter, exists = r.clientLimiters[clientID]
		if !exists {
			limiter = rate.NewLimiter(rate.Limit(r.perClientRPS), r.perClientBurst)
			r.clientLimiters[clientID] = limiter
		}
		r.clientMu.Unlock()
	}

	// Update last seen time
	r.clientLastSeenMu.Lock()
	r.clientLastSeen[clientID] = time.Now()
	r.clientLastSeenMu.Unlock()

	return limiter.Allow()
}

// getClientID extracts a unique client identifier from the context.
// It uses client IP address or other identifying information.
// 
// Security: X-Forwarded-For and X-Real-IP headers are only trusted if the
// direct connection is from a trusted proxy. Otherwise, the peer address
// is used to prevent IP spoofing attacks.
func (r *RateLimitInterceptor) getClientID(ctx context.Context) string {
	// Get the direct peer address first (this cannot be spoofed)
	var peerIP net.IP
	if p, ok := peer.FromContext(ctx); ok {
		if addr, ok := p.Addr.(*net.TCPAddr); ok {
			peerIP = addr.IP
		}
	}

	// Try to get client IP from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		// Check for x-forwarded-for header (for proxied requests)
		// ONLY trust this if the direct connection is from a trusted proxy
		if xff := md.Get("x-forwarded-for"); len(xff) > 0 {
			if peerIP != nil && isTrustedProxy(peerIP) {
				// Trusted proxy - use the first IP in the chain (original client)
				ips := strings.Split(xff[0], ",")
				if len(ips) > 0 {
					return strings.TrimSpace(ips[0])
				}
			}
			// Not from trusted proxy - ignore X-Forwarded-For to prevent spoofing
		}

		// Check for x-real-ip header
		// ONLY trust this if the direct connection is from a trusted proxy
		if xri := md.Get("x-real-ip"); len(xri) > 0 {
			if peerIP != nil && isTrustedProxy(peerIP) {
				return xri[0]
			}
			// Not from trusted proxy - ignore X-Real-IP to prevent spoofing
		}
	}

	// Fall back to peer address (the actual direct connection)
	if peerIP != nil {
		return peerIP.String()
	}

	// Last resort: return peer address as string
	if p, ok := peer.FromContext(ctx); ok {
		return p.Addr.String()
	}

	return "unknown"
}

// GetClientIP extracts the client IP address from the context.
// This is a utility function that can be used by other middleware.
// 
// Security: X-Forwarded-For and X-Real-IP headers are only trusted if the
// direct connection is from a trusted proxy. Otherwise, the peer address
// is used to prevent IP spoofing attacks.
func GetClientIP(ctx context.Context) string {
	// Get the direct peer address first (this cannot be spoofed)
	var peerIP net.IP
	if p, ok := peer.FromContext(ctx); ok {
		if addr, ok := p.Addr.(*net.TCPAddr); ok {
			peerIP = addr.IP
		}
	}

	// Try to get client IP from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		// Check for x-forwarded-for header (for proxied requests)
		// ONLY trust this if the direct connection is from a trusted proxy
		if xff := md.Get("x-forwarded-for"); len(xff) > 0 {
			if peerIP != nil && isTrustedProxy(peerIP) {
				// Trusted proxy - use the first IP in the chain (original client)
				ips := strings.Split(xff[0], ",")
				if len(ips) > 0 {
					return strings.TrimSpace(ips[0])
				}
			}
			// Not from trusted proxy - ignore X-Forwarded-For to prevent spoofing
		}

		// Check for x-real-ip header
		// ONLY trust this if the direct connection is from a trusted proxy
		if xri := md.Get("x-real-ip"); len(xri) > 0 {
			if peerIP != nil && isTrustedProxy(peerIP) {
				return xri[0]
			}
			// Not from trusted proxy - ignore X-Real-IP to prevent spoofing
		}
	}

	// Fall back to peer address (the actual direct connection)
	if peerIP != nil {
		return peerIP.String()
	}

	// Last resort: return peer address as string
	if p, ok := peer.FromContext(ctx); ok {
		return p.Addr.String()
	}

	return ""
}

// GetClientHTTP extracts the client IP from an HTTP request context.
// This is useful for gRPC-Gateway or HTTP-facing services.
// 
// Security: X-Forwarded-For and X-Real-IP headers are only trusted if the
// direct connection is from a trusted proxy. Otherwise, the RemoteAddr
// is used to prevent IP spoofing attacks.
func GetClientHTTP(r *http.Request) string {
	// Get the direct remote address first (this cannot be spoofed)
	remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteIP = r.RemoteAddr
	}

	// Parse the remote IP
	ip := net.ParseIP(remoteIP)

	// Check X-Forwarded-For header
	// ONLY trust this if the direct connection is from a trusted proxy
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		if ip != nil && isTrustedProxy(ip) {
			// Trusted proxy - use the first IP in the chain (original client)
			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				return strings.TrimSpace(ips[0])
			}
		}
		// Not from trusted proxy - ignore X-Forwarded-For to prevent spoofing
	}

	// Check X-Real-IP header
	// ONLY trust this if the direct connection is from a trusted proxy
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		if ip != nil && isTrustedProxy(ip) {
			return xri
		}
		// Not from trusted proxy - ignore X-Real-IP to prevent spoofing
	}

	// Fall back to remote address (the actual direct connection)
	return remoteIP
}

// TokenBucketLimiter is a simple token bucket implementation for custom use cases.
type TokenBucketLimiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewTokenBucketLimiter creates a new token bucket limiter.
func NewTokenBucketLimiter(ratePerSecond float64, burstSize float64) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		tokens:     burstSize,
		maxTokens:  burstSize,
		refillRate: ratePerSecond,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed and consumes a token if so.
func (t *TokenBucketLimiter) Allow() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(t.lastRefill).Seconds()
	t.tokens = min(t.maxTokens, t.tokens+elapsed*t.refillRate)
	t.lastRefill = now

	if t.tokens >= 1 {
		t.tokens--
		return true
	}
	return false
}

// Wait waits until a token is available.
func (t *TokenBucketLimiter) Wait(ctx context.Context) error {
	for {
		if t.Allow() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			// Try again
		}
	}
}

// Tokens returns the current number of available tokens.
func (t *TokenBucketLimiter) Tokens() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(t.lastRefill).Seconds()
	return min(t.maxTokens, t.tokens+elapsed*t.refillRate)
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
