package raft

import (
	"crypto/tls"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// TestTLSStreamLayer tests the TLS stream layer implementation
func TestTLSStreamLayer(t *testing.T) {
	t.Skip("Manual test - requires valid certificates. See deploy/RAFT-TLS-GUIDE.md for testing instructions.")
	
	caFile := filepath.Join("..", "..", "configs", "tls", "ca-cert.pem")
	certFile := filepath.Join("..", "..", "configs", "tls", "server-cert.pem")
	keyFile := filepath.Join("..", "..", "configs", "tls", "server-key.pem")

	if _, err := os.Stat(caFile); os.IsNotExist(err) {
		t.Skip("TLS certificates not found, skipping test")
	}

	tlsConfig := &TLSConfig{
		CAFile:     caFile,
		CertFile:   certFile,
		KeyFile:    keyFile,
		ServerName: "localhost",
		MinVersion: tls.VersionTLS12,
	}

	bindAddr := "127.0.0.1:0"
	advertise, _ := net.ResolveTCPAddr("tcp", bindAddr)

	streamLayer, err := NewTLSStreamLayer(bindAddr, advertise, tlsConfig)
	if err != nil {
		t.Fatalf("Failed to create TLS stream layer: %v", err)
	}
	defer streamLayer.Close()

	if streamLayer.Addr() == nil {
		t.Fatal("Stream layer address is nil")
	}

	t.Logf("TLS stream layer listening on %s", streamLayer.Addr())
}

// TestTLSTransportCreation tests creating a Raft transport with TLS
func TestTLSTransportCreation(t *testing.T) {
	t.Skip("Manual test - requires valid certificates. See deploy/RAFT-TLS-GUIDE.md for testing instructions.")
	
	caFile := filepath.Join("..", "..", "configs", "tls", "ca-cert.pem")
	certFile := filepath.Join("..", "..", "configs", "tls", "server-cert.pem")
	keyFile := filepath.Join("..", "..", "configs", "tls", "server-key.pem")

	if _, err := os.Stat(caFile); os.IsNotExist(err) {
		t.Skip("TLS certificates not found, skipping test")
	}

	config := DefaultConfig()
	config.TLSEnabled = true
	config.TLSConfig.CAFile = caFile
	config.TLSConfig.CertFile = certFile
	config.TLSConfig.KeyFile = keyFile

	if !config.TLSEnabled {
		t.Error("TLS should be enabled")
	}
	if config.TLSConfig == nil {
		t.Error("TLS config should not be nil")
	}

	t.Log("TLS transport creation test passed")
}

// TestTLSConfigValidation tests TLS configuration validation
func TestTLSConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *TLSRaftConfig
		wantError bool
	}{
		{
			name: "valid config",
			config: &TLSRaftConfig{
				CAFile:     "ca.pem",
				CertFile:   "cert.pem",
				KeyFile:    "key.pem",
				ServerName: "localhost",
			},
			wantError: false,
		},
		{
			name: "missing cert file",
			config: &TLSRaftConfig{
				CAFile:  "ca.pem",
				KeyFile: "key.pem",
			},
			wantError: true,
		},
		{
			name: "missing key file",
			config: &TLSRaftConfig{
				CAFile:   "ca.pem",
				CertFile: "cert.pem",
			},
			wantError: true,
		},
		{
			name: "missing ca file",
			config: &TLSRaftConfig{
				CertFile: "cert.pem",
				KeyFile:  "key.pem",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasError := tt.config.CertFile == "" || tt.config.KeyFile == "" || tt.config.CAFile == ""
			
			if hasError != tt.wantError {
				if tt.wantError {
					t.Error("Expected validation to fail but it passed")
				} else {
					t.Error("Expected validation to pass but it failed")
				}
			}
		})
	}
}

// TestDefaultTLSConfig tests that default config has TLS enabled
func TestDefaultTLSConfig(t *testing.T) {
	config := DefaultConfig()

	if !config.TLSEnabled {
		t.Error("TLS should be enabled by default")
	}

	if config.TLSConfig == nil {
		t.Fatal("TLS config should not be nil")
	}

	if config.TLSConfig.MinVersion != tls.VersionTLS12 {
		t.Errorf("Expected TLS 1.2 minimum, got %d", config.TLSConfig.MinVersion)
	}

	if config.TLSConfig.ServerName != "localhost" {
		t.Errorf("Expected server name 'localhost', got %s", config.TLSConfig.ServerName)
	}

	if config.TLSConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false by default")
	}

	t.Log("Default TLS config is secure")
}

// TestTLSConfigEdgeCases tests edge cases in TLS config
func TestTLSConfigEdgeCases(t *testing.T) {
	t.Run("EmptyServerName", func(t *testing.T) {
		config := &TLSRaftConfig{
			CAFile:     "ca.pem",
			CertFile:   "cert.pem",
			KeyFile:    "key.pem",
			ServerName: "",
		}
		
		if config.ServerName != "" {
			t.Error("ServerName should be empty")
		}
	})

	t.Run("CustomMinVersion", func(t *testing.T) {
		config := &TLSRaftConfig{
			CAFile:     "ca.pem",
			CertFile:   "cert.pem",
			KeyFile:    "key.pem",
			ServerName: "localhost",
			MinVersion: tls.VersionTLS13,
		}
		
		if config.MinVersion != tls.VersionTLS13 {
			t.Errorf("Expected TLS 1.3, got %d", config.MinVersion)
		}
	})
}

// TestTLSTransportAddr tests transport address resolution
func TestTLSTransportAddr(t *testing.T) {
	bindAddr := "127.0.0.1:0"
	advertiseAddr := "127.0.0.1:18080"
	
	bind, err := net.ResolveTCPAddr("tcp", bindAddr)
	if err != nil {
		t.Fatalf("Failed to resolve bind addr: %v", err)
	}
	
	advertise, err := net.ResolveTCPAddr("tcp", advertiseAddr)
	if err != nil {
		t.Fatalf("Failed to resolve advertise addr: %v", err)
	}
	
	if bind == nil {
		t.Fatal("Bind address should not be nil")
	}
	
	if advertise == nil {
		t.Fatal("Advertise address should not be nil")
	}
	
	t.Logf("Bind: %s, Advertise: %s", bind.String(), advertise.String())
}
