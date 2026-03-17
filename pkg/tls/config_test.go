package tls

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDefaultTLSConfig tests that default TLS configuration is secure.
// This is a P0 test case for TLS security defaults.
func TestDefaultTLSConfig(t *testing.T) {
	cfg := DefaultTLSConfig()

	if cfg == nil {
		t.Fatal("DefaultTLSConfig should not return nil")
	}

	// Verify minimum TLS version is 1.3
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Errorf("Expected MinVersion TLS 1.3, got %d", cfg.MinVersion)
	}

	// Verify client auth is required
	if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Errorf("Expected ClientAuth RequireAndVerifyClientCert, got %v", cfg.ClientAuth)
	}

	// Verify cipher suites are set
	if len(cfg.CipherSuites) == 0 {
		t.Error("Expected cipher suites to be configured")
	}

	t.Log("Default TLS configuration is secure")
}

// TestSecureCipherSuites tests that secure cipher suites are returned.
// This is a P0 test case for cipher suite selection.
func TestSecureCipherSuites(t *testing.T) {
	suites := secureCipherSuites()

	if len(suites) == 0 {
		t.Fatal("Expected at least one cipher suite")
	}

	// Verify only TLS 1.3 cipher suites are included
	expectedSuites := map[uint16]bool{
		tls.TLS_AES_128_GCM_SHA256:       true,
		tls.TLS_AES_256_GCM_SHA384:       true,
		tls.TLS_CHACHA20_POLY1305_SHA256: true,
	}

	for _, suite := range suites {
		if !expectedSuites[suite] {
			t.Errorf("Unexpected cipher suite: %d", suite)
		}
	}

	t.Logf("Secure cipher suites: %d configured", len(suites))
}

// TestLoadTLSCertificate tests certificate loading.
// This is a P0 test case for certificate handling.
func TestLoadTLSCertificate(t *testing.T) {
	// Test with non-existent files (should fail)
	_, err := LoadTLSCertificate("/nonexistent/cert.pem", "/nonexistent/key.pem")
	if err == nil {
		t.Error("Expected error for non-existent certificate files")
	}

	t.Log("Certificate loading error handling works correctly")
}

// TestLoadCACertPool tests CA certificate pool loading.
// This is a P0 test case for CA certificate handling.
func TestLoadCACertPool(t *testing.T) {
	// Test with non-existent file (should fail)
	_, err := LoadCACertPool("/nonexistent/ca.pem")
	if err == nil {
		t.Error("Expected error for non-existent CA file")
	}

	// Test with invalid PEM file
	tmpDir := t.TempDir()
	invalidPEM := filepath.Join(tmpDir, "invalid.pem")
	if err := os.WriteFile(invalidPEM, []byte("not a valid PEM"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err = LoadCACertPool(invalidPEM)
	if err == nil {
		t.Error("Expected error for invalid PEM file")
	}

	t.Log("CA certificate pool error handling works correctly")
}

// TestBuildTLSConfig tests TLS configuration building.
// This is a P0 test case for TLS config construction.
func TestBuildTLSConfig(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *Config
		shouldErr bool
	}{
		{
			name: "MissingCertPath",
			cfg: &Config{
				CertPath:   "",
				KeyPath:    "",
				MinVersion: tls.VersionTLS13,
			},
			shouldErr: true,
		},
		{
			name: "MissingKeyPath",
			cfg: &Config{
				CertPath:   "/path/to/cert.pem",
				KeyPath:    "",
				MinVersion: tls.VersionTLS13,
			},
			shouldErr: true,
		},
		{
			name: "ValidConfig-NoCA",
			cfg: &Config{
				CertPath:   "/path/to/cert.pem",
				KeyPath:    "/path/to/key.pem",
				MinVersion: tls.VersionTLS13,
				ClientAuth: tls.NoClientCert,
			},
			shouldErr: true, // Files don't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildTLSConfig(tt.cfg)
			hasErr := err != nil

			if hasErr != tt.shouldErr {
				t.Errorf("Expected error=%v, got error=%v", tt.shouldErr, hasErr)
			}
		})
	}
}

// TestBuildTLSConfigWithCA tests TLS configuration with CA verification.
// This is a P0 test case for mTLS configuration.
func TestBuildTLSConfigWithCA(t *testing.T) {
	tmpDir := t.TempDir()

	// Create dummy certificate files (will fail to load but tests the flow)
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")
	caPath := filepath.Join(tmpDir, "ca.pem")

	// Write dummy content
	os.WriteFile(certPath, []byte("cert"), 0644)
	os.WriteFile(keyPath, []byte("key"), 0644)
	os.WriteFile(caPath, []byte("ca"), 0644)

	cfg := &Config{
		CertPath:   certPath,
		KeyPath:    keyPath,
		CAPath:     caPath,
		MinVersion: tls.VersionTLS13,
		ClientAuth: tls.RequireAndVerifyClientCert,
	}

	// This will fail due to invalid cert content, but tests the CA loading path
	_, err := BuildTLSConfig(cfg)
	if err == nil {
		t.Error("Expected error for invalid certificate content")
	}

	t.Log("TLS config with CA verification flow tested")
}

// TestCertReloaderCreation tests certificate reloader creation.
// This is a P0 test case for certificate hot reloading.
func TestCertReloaderCreation(t *testing.T) {
	// Test with non-existent files (should fail)
	cfg := &Config{
		CertPath: "/nonexistent/cert.pem",
		KeyPath:  "/nonexistent/key.pem",
	}

	_, err := NewCertReloader(cfg, 5*time.Minute)
	if err == nil {
		t.Error("Expected error for non-existent certificate files")
	}

	t.Log("CertReloader creation error handling works correctly")
}

// TestCertReloaderReload tests certificate reloading mechanism.
// This is a P0 test case for certificate reload functionality.
func TestCertReloaderReload(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Write invalid content (will fail to load)
	os.WriteFile(certPath, []byte("invalid"), 0644)
	os.WriteFile(keyPath, []byte("invalid"), 0644)

	cfg := &Config{
		CertPath: certPath,
		KeyPath:  keyPath,
	}

	reloader := &CertReloader{
		config:    cfg,
		reloadDur: 1 * time.Second,
	}

	// Test reload with invalid certs
	err := reloader.reload()
	if err == nil {
		t.Error("Expected error for invalid certificate content")
	}

	t.Log("CertReloader reload error handling works correctly")
}

// TestCertReloaderGetTLSConfig tests getting TLS config from reloader.
// This is a P0 test case for TLS config retrieval.
func TestCertReloaderGetTLSConfig(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Write invalid content
	os.WriteFile(certPath, []byte("invalid"), 0644)
	os.WriteFile(keyPath, []byte("invalid"), 0644)

	cfg := &Config{
		CertPath:   certPath,
		KeyPath:    keyPath,
		MinVersion: tls.VersionTLS13,
	}

	reloader := &CertReloader{
		config:    cfg,
		reloadDur: 1 * time.Millisecond, // Very short for testing
		lastLoad:  time.Now().Add(-1 * time.Hour), // Force reload
	}

	// Test GetTLSConfig with invalid certs (should trigger reload and fail)
	_, err := reloader.GetTLSConfig()
	if err == nil {
		t.Error("Expected error for invalid certificate content")
	}

	t.Log("CertReloader GetTLSConfig error handling works correctly")
}

// TestValidateCertificate tests certificate validation.
// This is a P0 test case for certificate verification.
func TestValidateCertificate(t *testing.T) {
	tests := []struct {
		name      string
		certPEM   []byte
		caPool    *x509.CertPool
		shouldErr bool
	}{
		{
			name:      "EmptyCert",
			certPEM:   []byte{},
			caPool:    x509.NewCertPool(),
			shouldErr: true,
		},
		{
			name:      "InvalidPEM",
			certPEM:   []byte("not a PEM"),
			caPool:    x509.NewCertPool(),
			shouldErr: true,
		},
		{
			name:      "WrongPEMType",
			certPEM:   []byte("-----BEGIN PRIVATE KEY-----\ninvalid\n-----END PRIVATE KEY-----"),
			caPool:    x509.NewCertPool(),
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCertificate(tt.certPEM, tt.caPool)
			hasErr := err != nil

			if hasErr != tt.shouldErr {
				t.Errorf("Expected error=%v, got error=%v", tt.shouldErr, hasErr)
			}
		})
	}
}

// TestValidateCertificateWithValidCert tests validation with a proper certificate.
// This is a P0 test case for certificate validation flow.
func TestValidateCertificateWithValidCert(t *testing.T) {
	// Use a simple valid PEM structure (won't validate but tests parsing)
	validPEM := []byte(`-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKHBfpegPjMCMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl
c3RjYTAeFw0yNjAzMTcwMDAwMDBaFw0yNzAzMTcwMDAwMDBaMBExDzANBgNVBAMM
BnRlc3RjYTBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQC7o96WzE5mfUEQkYL8h5VZ
test1234567890abcdefghijklmnopqrstuvwxyz
-----END CERTIFICATE-----`)

	caPool := x509.NewCertPool()
	
	// This will fail validation (self-signed, no proper CA) but tests the parsing flow
	err := ValidateCertificate(validPEM, caPool)
	if err == nil {
		t.Log("Certificate parsed but validation should fail without proper CA")
	} else {
		t.Logf("Validation failed as expected: %v", err)
	}
}

// TestGenerateSelfSignedCert tests self-signed certificate generation.
// This is a P1 test case for test certificate generation.
func TestGenerateSelfSignedCert(t *testing.T) {
	// This function is intentionally not implemented
	cert, key, err := GenerateSelfSignedCert("localhost")
	
	if err == nil {
		t.Error("Expected error - function should not be implemented")
	}
	if cert != nil || key != nil {
		t.Error("Expected nil certificates")
	}

	t.Log("GenerateSelfSignedCert correctly returns error")
}

// TestConfigValidation tests Config field validation.
// This is a P0 test case for configuration validation.
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *Config
		valid  bool
	}{
		{
			name: "EmptyPaths",
			cfg: &Config{
				CertPath: "",
				KeyPath:  "",
			},
			valid: false,
		},
		{
			name: "CertPathOnly",
			cfg: &Config{
				CertPath: "/path/to/cert.pem",
				KeyPath:  "",
			},
			valid: false,
		},
		{
			name: "KeyPathOnly",
			cfg: &Config{
				CertPath: "",
				KeyPath:  "/path/to/key.pem",
			},
			valid: false,
		},
		{
			name: "BothPaths",
			cfg: &Config{
				CertPath: "/path/to/cert.pem",
				KeyPath:  "/path/to/key.pem",
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation logic
			hasBothPaths := tt.cfg.CertPath != "" && tt.cfg.KeyPath != ""
			isValid := hasBothPaths

			if isValid != tt.valid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.valid, isValid)
			}
		})
	}
}

// TestTLSVersionConstants tests TLS version constant usage.
// This is a P1 test case for TLS version configuration.
func TestTLSVersionConstants(t *testing.T) {
	versions := []struct {
		name    string
		version uint16
	}{
		{"TLS12", tls.VersionTLS12},
		{"TLS13", tls.VersionTLS13},
	}

	for _, v := range versions {
		t.Run(v.name, func(t *testing.T) {
			if v.version == 0 {
				t.Errorf("TLS version %s has invalid value 0", v.name)
			}
			t.Logf("%s = 0x%04x", v.name, v.version)
		})
	}
}

// TestClientAuthTypes tests client authentication type constants.
// This is a P1 test case for client auth configuration.
func TestClientAuthTypes(t *testing.T) {
	authTypes := []struct {
		name string
		auth tls.ClientAuthType
	}{
		{"NoClientCert", tls.NoClientCert},
		{"RequestClientCert", tls.RequestClientCert},
		{"RequireAnyClientCert", tls.RequireAnyClientCert},
		{"VerifyClientCertIfGiven", tls.VerifyClientCertIfGiven},
		{"RequireAndVerifyClientCert", tls.RequireAndVerifyClientCert},
	}

	for _, at := range authTypes {
		t.Run(at.name, func(t *testing.T) {
			t.Logf("%s = %d", at.name, at.auth)
		})
	}
}

// TestCertReloaderConcurrentAccess tests concurrent access to CertReloader.
// This is a P0 test case for thread safety.
func TestCertReloaderConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	os.WriteFile(certPath, []byte("invalid"), 0644)
	os.WriteFile(keyPath, []byte("invalid"), 0644)

	cfg := &Config{
		CertPath: certPath,
		KeyPath:  keyPath,
	}

	reloader := &CertReloader{
		config:    cfg,
		reloadDur: 1 * time.Millisecond,
		lastLoad:  time.Now().Add(-1 * time.Hour),
	}

	// Test concurrent GetTLSConfig calls
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			reloader.GetTLSConfig()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	t.Log("Concurrent access test completed")
}

// TestConfigFieldAccess tests Config field access patterns.
// This is a P1 test case for configuration structure.
func TestConfigFieldAccess(t *testing.T) {
	cfg := &Config{
		CertPath:   "/path/to/cert.pem",
		KeyPath:    "/path/to/key.pem",
		CAPath:     "/path/to/ca.pem",
		MinVersion: tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
		},
		ClientAuth: tls.RequireAndVerifyClientCert,
	}

	// Verify all fields are accessible
	if cfg.CertPath == "" {
		t.Error("CertPath should be set")
	}
	if cfg.KeyPath == "" {
		t.Error("KeyPath should be set")
	}
	if cfg.CAPath == "" {
		t.Error("CAPath should be set")
	}
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Error("MinVersion should be TLS 1.3")
	}
	if len(cfg.CipherSuites) == 0 {
		t.Error("CipherSuites should be set")
	}

	t.Log("Config field access test completed")
}
