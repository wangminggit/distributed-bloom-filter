package grpc

import (
	"context"
	"net"
	"net/http"
	"sync"
	"testing"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// TestIPSpoofingProtection verifies that the IP spoofing vulnerability is fixed.
// This is the main test for P1 issue #7.
func TestIPSpoofingProtection(t *testing.T) {
	// Reset to default trusted proxies (localhost + private networks)
	TrustedProxies = []string{}
	trustedProxyOnce = sync.Once{}

	t.Run("ExternalIPCannotSpoofXFF", func(t *testing.T) {
		// Attacker from external IP tries to spoof X-Forwarded-For
		peerAddr := &net.TCPAddr{
			IP:   net.ParseIP("203.0.113.66"), // External attacker
			Port: 54321,
		}

		ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: peerAddr})
		md := metadata.New(map[string]string{
			"x-forwarded-for": "10.0.0.1", // Spoofed internal IP
		})
		ctx = metadata.NewIncomingContext(ctx, md)

		clientIP := GetClientIP(ctx)

		// Should ignore spoofed XFF and use actual peer IP
		if clientIP != "203.0.113.66" {
			t.Errorf("IP spoofing not prevented! Expected 203.0.113.66, got %s", clientIP)
		}
	})

	t.Run("TrustedProxyCanUseXFF", func(t *testing.T) {
		// Legitimate request from trusted proxy (load balancer)
		peerAddr := &net.TCPAddr{
			IP:   net.ParseIP("192.168.1.1"), // Trusted internal LB
			Port: 54321,
		}

		ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: peerAddr})
		md := metadata.New(map[string]string{
			"x-forwarded-for": "10.0.0.100", // Real client IP
		})
		ctx = metadata.NewIncomingContext(ctx, md)

		clientIP := GetClientIP(ctx)

		// Should trust XFF from trusted proxy
		if clientIP != "10.0.0.100" {
			t.Errorf("Trusted proxy XFF not working! Expected 10.0.0.100, got %s", clientIP)
		}
	})

	t.Run("RateLimitBypassPrevented", func(t *testing.T) {
		// Attacker tries to bypass rate limiting by spoofing different IPs
		attackerIP := net.ParseIP("198.51.100.50")
		interceptor := NewRateLimitInterceptorWithConfig(RateLimitConfig{
			RequestsPerSecond: 10,
			BurstSize:         20,
			PerClientRPS:      5,
			PerClientBurst:    10,
			EnablePerClient:   true,
		})
		defer interceptor.Stop()

		// Try 100 requests with different spoofed XFF IPs
		blockedCount := 0
		for i := 0; i < 100; i++ {
			peerAddr := &net.TCPAddr{
				IP:   attackerIP,
				Port: 54321,
			}

			ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: peerAddr})
			// Spoof different client IPs
			md := metadata.New(map[string]string{
				"x-forwarded-for": "10.0.0." + string(rune(i)),
			})
			ctx = metadata.NewIncomingContext(ctx, md)

			clientID := interceptor.getClientID(ctx)
			
			// All requests should be identified as coming from the same attacker IP
			if clientID != "198.51.100.50" {
				t.Errorf("Request %d: Client ID spoofed! Expected 198.51.100.50, got %s", i, clientID)
			}
			
			if !interceptor.allowClient(clientID) {
				blockedCount++
			}
		}

		// Most requests should be blocked due to rate limiting
		if blockedCount < 90 {
			t.Errorf("Rate limiting bypass possible! Only %d/100 requests blocked", blockedCount)
		}
	})
}

// TestGetClientHTTP_SpoofingProtection tests HTTP request IP extraction.
func TestGetClientHTTP_SpoofingProtection(t *testing.T) {
	TrustedProxies = []string{"127.0.0.0/8"}
	trustedProxyOnce = sync.Once{}
	defer func() {
		TrustedProxies = []string{}
		trustedProxyOnce = sync.Once{}
	}()

	t.Run("UntrustedRemoteAddr", func(t *testing.T) {
		req := &http.Request{
			RemoteAddr: "203.0.113.1:12345",
			Header: map[string][]string{
				"X-Forwarded-For": {"10.0.0.1"},
				"X-Real-IP":       {"10.0.0.2"},
			},
		}

		ip := GetClientHTTP(req)
		if ip != "203.0.113.1" {
			t.Errorf("Expected 203.0.113.1, got %s", ip)
		}
	})

	t.Run("TrustedRemoteAddr", func(t *testing.T) {
		req := &http.Request{
			RemoteAddr: "127.0.0.1:12345",
			Header: map[string][]string{
				"X-Forwarded-For": {"192.168.1.100"},
			},
		}

		ip := GetClientHTTP(req)
		if ip != "192.168.1.100" {
			t.Errorf("Expected 192.168.1.100, got %s", ip)
		}
	})
}
