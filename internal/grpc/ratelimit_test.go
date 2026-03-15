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

// TestGetClientIP_UntrustedProxy tests that X-Forwarded-For headers from
// untrusted proxies are ignored to prevent IP spoofing attacks.
func TestGetClientIP_UntrustedProxy(t *testing.T) {
	// Reset trusted proxies to only localhost for this test
	originalTrustedProxies := TrustedProxies
	TrustedProxies = []string{"127.0.0.0/8"}
	// Reset the initialization state
	trustedProxyOnce = sync.Once{}
	defer func() {
		TrustedProxies = originalTrustedProxies
		trustedProxyOnce = sync.Once{}
	}()

	t.Run("UntrustedProxyWithXFF", func(t *testing.T) {
		// Simulate a request from an untrusted external IP (e.g., 203.0.113.1)
		// with a spoofed X-Forwarded-For header
		peerAddr := &net.TCPAddr{
			IP:   net.ParseIP("203.0.113.1"),
			Port: 54321,
		}

		ctx := peer.NewContext(context.Background(), &peer.Peer{
			Addr: peerAddr,
		})

		// Add spoofed X-Forwarded-For header claiming to be from 10.0.0.1
		md := metadata.New(map[string]string{
			"x-forwarded-for": "10.0.0.1",
		})
		ctx = metadata.NewIncomingContext(ctx, md)

		clientIP := GetClientIP(ctx)

		// Should ignore the spoofed XFF and use the actual peer IP
		if clientIP != "203.0.113.1" {
			t.Errorf("Expected client IP 203.0.113.1 (peer IP), got %s", clientIP)
		}
	})

	t.Run("TrustedProxyWithXFF", func(t *testing.T) {
		// Simulate a request from a trusted proxy (localhost)
		peerAddr := &net.TCPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 54321,
		}

		ctx := peer.NewContext(context.Background(), &peer.Peer{
			Addr: peerAddr,
		})

		// Add X-Forwarded-For header from trusted proxy
		md := metadata.New(map[string]string{
			"x-forwarded-for": "192.168.1.100",
		})
		ctx = metadata.NewIncomingContext(ctx, md)

		clientIP := GetClientIP(ctx)

		// Should trust the XFF header and use the forwarded IP
		if clientIP != "192.168.1.100" {
			t.Errorf("Expected client IP 192.168.1.100 (from XFF), got %s", clientIP)
		}
	})

	t.Run("UntrustedProxyWithXRealIP", func(t *testing.T) {
		// Simulate a request from an untrusted external IP
		peerAddr := &net.TCPAddr{
			IP:   net.ParseIP("198.51.100.50"),
			Port: 54321,
		}

		ctx := peer.NewContext(context.Background(), &peer.Peer{
			Addr: peerAddr,
		})

		// Add spoofed X-Real-IP header
		md := metadata.New(map[string]string{
			"x-real-ip": "10.0.0.2",
		})
		ctx = metadata.NewIncomingContext(ctx, md)

		clientIP := GetClientIP(ctx)

		// Should ignore the spoofed X-Real-IP and use the actual peer IP
		if clientIP != "198.51.100.50" {
			t.Errorf("Expected client IP 198.51.100.50 (peer IP), got %s", clientIP)
		}
	})

	t.Run("TrustedProxyWithXRealIP", func(t *testing.T) {
		// Simulate a request from a trusted proxy
		peerAddr := &net.TCPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 54321,
		}

		ctx := peer.NewContext(context.Background(), &peer.Peer{
			Addr: peerAddr,
		})

		// Add X-Real-IP header from trusted proxy
		md := metadata.New(map[string]string{
			"x-real-ip": "192.168.1.200",
		})
		ctx = metadata.NewIncomingContext(ctx, md)

		clientIP := GetClientIP(ctx)

		// Should trust the X-Real-IP header
		if clientIP != "192.168.1.200" {
			t.Errorf("Expected client IP 192.168.1.200 (from X-Real-IP), got %s", clientIP)
		}
	})

	t.Run("NoHeaders", func(t *testing.T) {
		// Simulate a direct connection with no proxy headers
		peerAddr := &net.TCPAddr{
			IP:   net.ParseIP("192.0.2.1"),
			Port: 54321,
		}

		ctx := peer.NewContext(context.Background(), &peer.Peer{
			Addr: peerAddr,
		})

		clientIP := GetClientIP(ctx)

		// Should use the peer IP
		if clientIP != "192.0.2.1" {
			t.Errorf("Expected client IP 192.0.2.1 (peer IP), got %s", clientIP)
		}
	})

	t.Run("PrivateNetworkTrusted", func(t *testing.T) {
		// Test that private network IPs are trusted by default
		// Reset to use defaults (no explicit trusted proxies)
		TrustedProxies = []string{}
		trustedProxyOnce = sync.Once{}

		peerAddr := &net.TCPAddr{
			IP:   net.ParseIP("192.168.1.1"),
			Port: 54321,
		}

		ctx := peer.NewContext(context.Background(), &peer.Peer{
			Addr: peerAddr,
		})

		md := metadata.New(map[string]string{
			"x-forwarded-for": "10.0.0.5",
		})
		ctx = metadata.NewIncomingContext(ctx, md)

		clientIP := GetClientIP(ctx)

		// Should trust XFF from private network
		if clientIP != "10.0.0.5" {
			t.Errorf("Expected client IP 10.0.0.5 (from XFF), got %s", clientIP)
		}
	})
}

// TestIsTrustedProxy tests the isTrustedProxy function with various IP ranges.
func TestIsTrustedProxy(t *testing.T) {
	// Reset to defaults
	TrustedProxies = []string{}
	trustedProxyOnce = sync.Once{}

	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		// Trusted ranges
		{"IPv4 localhost", "127.0.0.1", true},
		{"IPv4 localhost range", "127.255.255.255", true},
		{"IPv6 localhost", "::1", true},
		{"Private network 10.x", "10.0.0.1", true},
		{"Private network 10.x", "10.255.255.255", true},
		{"Private network 172.16.x", "172.16.0.1", true},
		{"Private network 172.31.x", "172.31.255.255", true},
		{"Private network 192.168.x", "192.168.0.1", true},
		{"Private network 192.168.x", "192.168.255.255", true},
		{"IPv6 ULA", "fc00::1", true},
		{"IPv6 link-local", "fe80::1", true},

		// Untrusted ranges
		{"Public IP 1", "8.8.8.8", false},
		{"Public IP 2", "1.1.1.1", false},
		{"Documentation IP", "192.0.2.1", false},
		{"Test network IP", "198.51.100.1", false},
		{"Benchmark IP", "203.0.113.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Failed to parse IP: %s", tt.ip)
			}

			result := isTrustedProxy(ip)
			if result != tt.expected {
				t.Errorf("isTrustedProxy(%s) = %v, expected %v", tt.ip, result, tt.expected)
			}
		})
	}
}

// TestGetClientIP_CustomTrustedProxies tests custom trusted proxy configuration.
func TestGetClientIP_CustomTrustedProxies(t *testing.T) {
	// Configure custom trusted proxies
	TrustedProxies = []string{
		"203.0.113.0/24",  // Specific public range
		"198.51.100.50",   // Single IP
	}
	trustedProxyOnce = sync.Once{}

	defer func() {
		TrustedProxies = []string{}
		trustedProxyOnce = sync.Once{}
	}()

	t.Run("CustomTrustedRange", func(t *testing.T) {
		peerAddr := &net.TCPAddr{
			IP:   net.ParseIP("203.0.113.10"),
			Port: 54321,
		}

		ctx := peer.NewContext(context.Background(), &peer.Peer{
			Addr: peerAddr,
		})

		md := metadata.New(map[string]string{
			"x-forwarded-for": "10.0.0.1",
		})
		ctx = metadata.NewIncomingContext(ctx, md)

		clientIP := GetClientIP(ctx)

		// Should trust XFF from custom trusted range
		if clientIP != "10.0.0.1" {
			t.Errorf("Expected client IP 10.0.0.1 (from XFF), got %s", clientIP)
		}
	})

	t.Run("CustomTrustedSingleIP", func(t *testing.T) {
		peerAddr := &net.TCPAddr{
			IP:   net.ParseIP("198.51.100.50"),
			Port: 54321,
		}

		ctx := peer.NewContext(context.Background(), &peer.Peer{
			Addr: peerAddr,
		})

		md := metadata.New(map[string]string{
			"x-forwarded-for": "10.0.0.2",
		})
		ctx = metadata.NewIncomingContext(ctx, md)

		clientIP := GetClientIP(ctx)

		// Should trust XFF from custom trusted single IP
		if clientIP != "10.0.0.2" {
			t.Errorf("Expected client IP 10.0.0.2 (from XFF), got %s", clientIP)
		}
	})

	t.Run("CustomUntrustedIP", func(t *testing.T) {
		peerAddr := &net.TCPAddr{
			IP:   net.ParseIP("8.8.8.8"),
			Port: 54321,
		}

		ctx := peer.NewContext(context.Background(), &peer.Peer{
			Addr: peerAddr,
		})

		md := metadata.New(map[string]string{
			"x-forwarded-for": "10.0.0.3",
		})
		ctx = metadata.NewIncomingContext(ctx, md)

		clientIP := GetClientIP(ctx)

		// Should NOT trust XFF from untrusted IP
		if clientIP != "8.8.8.8" {
			t.Errorf("Expected client IP 8.8.8.8 (peer IP), got %s", clientIP)
		}
	})
}

// TestGetClientHTTP_UntrustedProxy tests GetClientHTTP with untrusted proxies.
func TestGetClientHTTP_UntrustedProxy(t *testing.T) {
	// Reset to only trust localhost
	TrustedProxies = []string{"127.0.0.0/8"}
	trustedProxyOnce = sync.Once{}

	defer func() {
		TrustedProxies = []string{}
		trustedProxyOnce = sync.Once{}
	}()

	t.Run("UntrustedProxy", func(t *testing.T) {
		req := &http.Request{
			RemoteAddr: "203.0.113.1:12345",
			Header: map[string][]string{
				"X-Forwarded-For": {"10.0.0.1"},
			},
		}

		clientIP := GetClientHTTP(req)

		// Should ignore spoofed XFF
		if clientIP != "203.0.113.1" {
			t.Errorf("Expected client IP 203.0.113.1 (peer IP), got %s", clientIP)
		}
	})

	t.Run("TrustedProxy", func(t *testing.T) {
		req := &http.Request{
			RemoteAddr: "127.0.0.1:12345",
			Header: map[string][]string{
				"X-Forwarded-For": {"192.168.1.100"},
			},
		}

		clientIP := GetClientHTTP(req)

		// Should trust XFF from localhost
		if clientIP != "192.168.1.100" {
			t.Errorf("Expected client IP 192.168.1.100 (from XFF), got %s", clientIP)
		}
	})
}
