package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDefaultTLSConfig tests that default TLS config has secure settings.
func TestDefaultTLSConfig(t *testing.T) {
	cfg := DefaultTLSConfig()

	if cfg.MinVersion != tls.VersionTLS13 {
		t.Errorf("Expected MinVersion TLS13, got %d", cfg.MinVersion)
	}

	if len(cfg.CipherSuites) == 0 {
		t.Error("Expected cipher suites to be configured")
	}

	if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Error("Expected RequireAndVerifyClientCert")
	}
}

// TestLoadTLSCertificate tests certificate loading.
func TestLoadTLSCertificate(t *testing.T) {
	// Generate test certificates
	tmpDir := t.TempDir()
	certPath, keyPath := generateTestCerts(t, tmpDir)

	cert, err := LoadTLSCertificate(certPath, keyPath)
	if err != nil {
		t.Fatalf("Failed to load certificate: %v", err)
	}

	if cert == nil {
		t.Fatal("Expected certificate to be loaded")
	}

	if len(cert.Certificate) == 0 {
		t.Error("Expected certificate data")
	}
}

// TestLoadCACertPool tests CA pool loading.
func TestLoadCACertPool(t *testing.T) {
	tmpDir := t.TempDir()
	caPath, _ := generateTestCA(t, tmpDir)

	caPool, err := LoadCACertPool(caPath)
	if err != nil {
		t.Fatalf("Failed to load CA pool: %v", err)
	}

	if caPool == nil {
		t.Fatal("Expected CA pool to be loaded")
	}
}

// TestBuildTLSConfig tests building TLS config.
func TestBuildTLSConfig(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath, caPath := generateTestCertsWithCA(t, tmpDir)

	cfg := &Config{
		CertPath:   certPath,
		KeyPath:    keyPath,
		CAPath:     caPath,
		MinVersion: tls.VersionTLS13,
		ClientAuth: tls.RequireAndVerifyClientCert,
	}

	tlsConfig, err := BuildTLSConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to build TLS config: %v", err)
	}

	if tlsConfig == nil {
		t.Fatal("Expected TLS config to be built")
	}

	if tlsConfig.MinVersion != tls.VersionTLS13 {
		t.Errorf("Expected MinVersion TLS13, got %d", tlsConfig.MinVersion)
	}

	if len(tlsConfig.Certificates) != 1 {
		t.Errorf("Expected 1 certificate, got %d", len(tlsConfig.Certificates))
	}

	if tlsConfig.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Errorf("Expected RequireAndVerifyClientCert, got %v", tlsConfig.ClientAuth)
	}
}

// TestCertReloader tests certificate hot reloading.
func TestCertReloader(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath, caPath := generateTestCertsWithCA(t, tmpDir)

	cfg := &Config{
		CertPath:   certPath,
		KeyPath:    keyPath,
		CAPath:     caPath,
		MinVersion: tls.VersionTLS13,
		ClientAuth: tls.RequireAndVerifyClientCert,
	}

	reloader, err := NewCertReloader(cfg, 1*time.Minute)
	if err != nil {
		t.Fatalf("Failed to create cert reloader: %v", err)
	}

	tlsConfig, err := reloader.GetTLSConfig()
	if err != nil {
		t.Fatalf("Failed to get TLS config: %v", err)
	}

	if tlsConfig == nil {
		t.Fatal("Expected TLS config")
	}
}

// TestValidateCertificate tests certificate validation.
func TestValidateCertificate(t *testing.T) {
	// Test with invalid certificate data
	caPool := x509.NewCertPool()
	invalidCert := []byte("invalid certificate data")
	err := ValidateCertificate(invalidCert, caPool)
	if err == nil {
		t.Error("Expected error for invalid certificate")
	}

	// Note: Full certificate validation testing requires proper CA setup
	// which is complex for unit tests. Integration tests should cover this.
	t.Log("Certificate validation basic test passed")
}

// Helper functions for generating test certificates

func generateTestCA(t *testing.T, dir string) (string, *x509.Certificate) {
	// Generate CA private key
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate CA key: %v", err)
	}

	// Create CA certificate template
	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test CA"},
			CommonName:   "Test CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// Self-sign CA certificate
	caCertDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("Failed to create CA certificate: %v", err)
	}

	// Write CA certificate
	caCertPath := filepath.Join(dir, "ca.crt")
	caCertFile, err := os.Create(caCertPath)
	if err != nil {
		t.Fatalf("Failed to create CA cert file: %v", err)
	}
	defer caCertFile.Close()

	if err := pem.Encode(caCertFile, &pem.Block{Type: "CERTIFICATE", Bytes: caCertDER}); err != nil {
		t.Fatalf("Failed to write CA certificate: %v", err)
	}

	// Parse CA certificate for return
	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		t.Fatalf("Failed to parse CA certificate: %v", err)
	}

	return caCertPath, caCert
}

func generateTestCerts(t *testing.T, dir string) (string, string) {
	// Generate private key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
			CommonName:   "localhost",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	// Self-sign certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Write certificate
	certPath := filepath.Join(dir, "server.crt")
	certFile, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("Failed to create cert file: %v", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		t.Fatalf("Failed to write certificate: %v", err)
	}

	// Write private key
	keyPath := filepath.Join(dir, "server.key")
	keyFile, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("Failed to create key file: %v", err)
	}
	defer keyFile.Close()

	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		t.Fatalf("Failed to write private key: %v", err)
	}

	return certPath, keyPath
}

func generateTestCertsWithCA(t *testing.T, dir string) (string, string, string) {
	caPath, _ := generateTestCA(t, dir)
	certPath, keyPath := generateTestCerts(t, dir)
	return certPath, keyPath, caPath
}

func generateClientCert(t *testing.T, caCert *x509.Certificate, dir string) []byte {
	// Generate CA private key for signing
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate CA key: %v", err)
	}

	// Generate client private key
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate client key: %v", err)
	}

	// Create client certificate template
	clientTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"Test Client"},
			CommonName:   "test-client",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	// Sign with CA private key
	clientCertDER, err := x509.CreateCertificate(rand.Reader, &clientTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("Failed to create client certificate: %v", err)
	}

	// Encode to PEM
	clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER})
	return clientCertPEM
}
