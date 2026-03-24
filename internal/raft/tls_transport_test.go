package raft

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// generateTestCert generates a self-signed certificate for testing.
func generateTestCert() (certPEM, keyPEM []byte, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:    []string{"localhost"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	certPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}

	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	return certPEM, keyPEM, nil
}

// TestTLSStreamLayerNew tests NewTLSStreamLayer.
func TestTLSStreamLayerNew(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Generate test certificates
	certPEM, keyPEM, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate cert: %v", err)
	}
	
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")
	caPath := filepath.Join(tmpDir, "ca.pem")
	
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		t.Fatalf("Failed to write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0644); err != nil {
		t.Fatalf("Failed to write key: %v", err)
	}
	// For CA, write DER format (loadCAFile expects DER)
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("Failed to decode cert PEM")
	}
	if err := os.WriteFile(caPath, block.Bytes, 0644); err != nil {
		t.Fatalf("Failed to write CA: %v", err)
	}
	
	tlsConfig := &TLSConfig{
		CertFile: certPath,
		KeyFile:  keyPath,
		CAFile:   caPath,
		ServerName: "localhost",
		MinVersion: tls.VersionTLS12,
	}
	
	advertise, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9000")
	
	layer, err := NewTLSStreamLayer("127.0.0.1:0", advertise, tlsConfig)
	
	if err != nil {
		t.Fatalf("NewTLSStreamLayer failed: %v", err)
	}
	
	if layer == nil {
		t.Fatal("Expected non-nil layer")
	}
	
	defer layer.Close()
	t.Log("NewTLSStreamLayer test completed successfully")
}

// TestTLSStreamLayerNewInvalidCert tests NewTLSStreamLayer with invalid cert.
func TestTLSStreamLayerNewInvalidCert(t *testing.T) {
	tlsConfig := &TLSConfig{
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
	}
	
	advertise, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9000")
	
	_, err := NewTLSStreamLayer("127.0.0.1:0", advertise, tlsConfig)
	
	if err == nil {
		t.Error("Expected error with invalid cert paths")
	}
	
	t.Logf("NewTLSStreamLayer correctly failed: %v", err)
}

// TestTLSStreamLayerNewInvalidCA tests NewTLSStreamLayer with invalid CA.
func TestTLSStreamLayerNewInvalidCA(t *testing.T) {
	tmpDir := t.TempDir()
	
	certPEM, keyPEM, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate cert: %v", err)
	}
	
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")
	
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		t.Fatalf("Failed to write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0644); err != nil {
		t.Fatalf("Failed to write key: %v", err)
	}
	
	tlsConfig := &TLSConfig{
		CertFile: certPath,
		KeyFile:  keyPath,
		CAFile:   "/nonexistent/ca.pem",
	}
	
	advertise, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9000")
	
	_, err = NewTLSStreamLayer("127.0.0.1:0", advertise, tlsConfig)
	
	if err == nil {
		t.Error("Expected error with invalid CA path")
	}
	
	t.Logf("NewTLSStreamLayer correctly failed: %v", err)
}

// TestTLSStreamLayerAccept tests Accept method.
func TestTLSStreamLayerAccept(t *testing.T) {
	tmpDir := t.TempDir()
	
	certPEM, keyPEM, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate cert: %v", err)
	}
	
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")
	
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		t.Fatalf("Failed to write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0644); err != nil {
		t.Fatalf("Failed to write key: %v", err)
	}
	
	tlsConfig := &TLSConfig{
		CertFile: certPath,
		KeyFile:  keyPath,
		InsecureSkipVerify: true,
	}
	
	advertise, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9000")
	
	layer, err := NewTLSStreamLayer("127.0.0.1:0", advertise, tlsConfig)
	if err != nil {
		t.Fatalf("NewTLSStreamLayer failed: %v", err)
	}
	defer layer.Close()
	
	// Test Accept in a goroutine
	done := make(chan error, 1)
	go func() {
		_, err := layer.Accept()
		done <- err
	}()
	
	// Close after short wait to unblock Accept
	time.Sleep(100 * time.Millisecond)
	layer.Close()
	
	err = <-done
	t.Logf("Accept completed: %v", err)
}

// TestTLSStreamLayerClose tests Close method.
func TestTLSStreamLayerClose(t *testing.T) {
	tmpDir := t.TempDir()
	
	certPEM, keyPEM, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate cert: %v", err)
	}
	
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")
	
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		t.Fatalf("Failed to write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0644); err != nil {
		t.Fatalf("Failed to write key: %v", err)
	}
	
	tlsConfig := &TLSConfig{
		CertFile: certPath,
		KeyFile:  keyPath,
	}
	
	advertise, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9000")
	
	layer, err := NewTLSStreamLayer("127.0.0.1:0", advertise, tlsConfig)
	if err != nil {
		t.Fatalf("NewTLSStreamLayer failed: %v", err)
	}
	
	err = layer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
	
	t.Log("Close test completed")
}

// TestTLSStreamLayerAddr tests Addr method.
func TestTLSStreamLayerAddr(t *testing.T) {
	tmpDir := t.TempDir()
	
	certPEM, keyPEM, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate cert: %v", err)
	}
	
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")
	
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		t.Fatalf("Failed to write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0644); err != nil {
		t.Fatalf("Failed to write key: %v", err)
	}
	
	tlsConfig := &TLSConfig{
		CertFile: certPath,
		KeyFile:  keyPath,
	}
	
	advertise, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9000")
	
	layer, err := NewTLSStreamLayer("127.0.0.1:0", advertise, tlsConfig)
	if err != nil {
		t.Fatalf("NewTLSStreamLayer failed: %v", err)
	}
	defer layer.Close()
	
	addr := layer.Addr()
	
	if addr == nil {
		t.Error("Expected non-nil address")
	}
	
	// Should return advertise address if set
	if addr.String() != "127.0.0.1:9000" {
		t.Logf("Addr returned: %v", addr)
	}
	
	t.Log("Addr test completed")
}

// TestTLSStreamLayerAddrNoAdvertise tests Addr without advertise.
func TestTLSStreamLayerAddrNoAdvertise(t *testing.T) {
	tmpDir := t.TempDir()
	
	certPEM, keyPEM, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate cert: %v", err)
	}
	
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")
	
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		t.Fatalf("Failed to write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0644); err != nil {
		t.Fatalf("Failed to write key: %v", err)
	}
	
	tlsConfig := &TLSConfig{
		CertFile: certPath,
		KeyFile:  keyPath,
	}
	
	layer, err := NewTLSStreamLayer("127.0.0.1:0", nil, tlsConfig)
	if err != nil {
		t.Fatalf("NewTLSStreamLayer failed: %v", err)
	}
	defer layer.Close()
	
	addr := layer.Addr()
	
	if addr == nil {
		t.Error("Expected non-nil address")
	}
	
	t.Logf("Addr without advertise: %v", addr)
}

// TestTLSStreamLayerDial tests Dial method.
func TestTLSStreamLayerDial(t *testing.T) {
	tmpDir := t.TempDir()
	
	certPEM, keyPEM, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate cert: %v", err)
	}
	
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")
	
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		t.Fatalf("Failed to write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0644); err != nil {
		t.Fatalf("Failed to write key: %v", err)
	}
	
	tlsConfig := &TLSConfig{
		CertFile: certPath,
		KeyFile:  keyPath,
		ServerName: "localhost",
	}
	
	advertise, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9000")
	
	layer, err := NewTLSStreamLayer("127.0.0.1:0", advertise, tlsConfig)
	if err != nil {
		t.Fatalf("NewTLSStreamLayer failed: %v", err)
	}
	defer layer.Close()
	
	// Try to dial an invalid address
	_, err = layer.Dial("127.0.0.1:99999", 100*time.Millisecond)
	
	if err == nil {
		t.Error("Expected error when dialing invalid address")
	}
	
	t.Logf("Dial correctly failed: %v", err)
}

// TestTLSStreamLayerDialTimeout tests Dial with timeout.
func TestTLSStreamLayerDialTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	
	certPEM, keyPEM, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate cert: %v", err)
	}
	
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")
	
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		t.Fatalf("Failed to write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0644); err != nil {
		t.Fatalf("Failed to write key: %v", err)
	}
	
	tlsConfig := &TLSConfig{
		CertFile: certPath,
		KeyFile:  keyPath,
	}
	
	advertise, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9000")
	
	layer, err := NewTLSStreamLayer("127.0.0.1:0", advertise, tlsConfig)
	if err != nil {
		t.Fatalf("NewTLSStreamLayer failed: %v", err)
	}
	defer layer.Close()
	
	// Dial with very short timeout to non-existent server
	_, err = layer.Dial("127.0.0.1:59999", 1*time.Millisecond)
	
	if err == nil {
		t.Error("Expected error when dialing with short timeout")
	}
	
	t.Logf("Dial timeout correctly failed: %v", err)
}

// TestLoadCAFile tests loadCAFile function.
func TestLoadCAFile(t *testing.T) {
	tmpDir := t.TempDir()
	
	certPEM, _, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate cert: %v", err)
	}
	
	// Decode PEM to get DER bytes
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("Failed to decode cert PEM")
	}
	
	caPath := filepath.Join(tmpDir, "ca.der")
	if err := os.WriteFile(caPath, block.Bytes, 0644); err != nil {
		t.Fatalf("Failed to write CA: %v", err)
	}
	
	caCert, err := loadCAFile(caPath)
	
	if err != nil {
		t.Fatalf("loadCAFile failed: %v", err)
	}
	
	if caCert == nil {
		t.Error("Expected non-nil CA certificate")
	}
	
	t.Log("loadCAFile test completed successfully")
}

// TestLoadCAFileNotFound tests loadCAFile with non-existent file.
func TestLoadCAFileNotFound(t *testing.T) {
	_, err := loadCAFile("/nonexistent/ca.pem")
	
	if err == nil {
		t.Error("Expected error when loading non-existent file")
	}
	
	t.Logf("loadCAFile correctly failed: %v", err)
}

// TestLoadCAFileInvalidContent tests loadCAFile with invalid content.
func TestLoadCAFileInvalidContent(t *testing.T) {
	tmpDir := t.TempDir()
	caPath := filepath.Join(tmpDir, "invalid.pem")
	
	if err := os.WriteFile(caPath, []byte("not a valid certificate"), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	
	_, err := loadCAFile(caPath)
	
	if err == nil {
		t.Error("Expected error when loading invalid content")
	}
	
	t.Logf("loadCAFile correctly rejected invalid content: %v", err)
}

// TestNewTCPTransportWithTLS tests NewTCPTransportWithTLS.
func TestNewTCPTransportWithTLS(t *testing.T) {
	tmpDir := t.TempDir()
	
	certPEM, keyPEM, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate cert: %v", err)
	}
	
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")
	
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		t.Fatalf("Failed to write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0644); err != nil {
		t.Fatalf("Failed to write key: %v", err)
	}
	
	tlsConfig := &TLSConfig{
		CertFile: certPath,
		KeyFile:  keyPath,
	}
	
	advertise, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9000")
	
	transport, err := NewTCPTransportWithTLS(
		"127.0.0.1:0",
		advertise,
		3,
		10*time.Second,
		nil,
		tlsConfig,
	)
	
	if err != nil {
		t.Fatalf("NewTCPTransportWithTLS failed: %v", err)
	}
	
	if transport == nil {
		t.Error("Expected non-nil transport")
	}
	
	// Clean up
	transport.Close()
	
	t.Log("NewTCPTransportWithTLS test completed successfully")
}

// TestNewTCPTransportWithTLSInvalidConfig tests NewTCPTransportWithTLS with invalid config.
func TestNewTCPTransportWithTLSInvalidConfig(t *testing.T) {
	tlsConfig := &TLSConfig{
		CertFile: "/invalid/cert.pem",
		KeyFile:  "/invalid/key.pem",
	}
	
	advertise, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9000")
	
	_, err := NewTCPTransportWithTLS(
		"127.0.0.1:0",
		advertise,
		3,
		10*time.Second,
		nil,
		tlsConfig,
	)
	
	if err == nil {
		t.Error("Expected error with invalid config")
	}
	
	t.Logf("NewTCPTransportWithTLS correctly failed: %v", err)
}

// TestTLSConfig tests TLSConfig structure.
func TestTLSConfig(t *testing.T) {
	config := &TLSConfig{
		CAFile:             "/path/to/ca.pem",
		CertFile:           "/path/to/cert.pem",
		KeyFile:            "/path/to/key.pem",
		ServerName:         "example.com",
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}
	
	if config.CAFile != "/path/to/ca.pem" {
		t.Errorf("Expected CAFile=/path/to/ca.pem, got %s", config.CAFile)
	}
	
	if config.ServerName != "example.com" {
		t.Errorf("Expected ServerName=example.com, got %s", config.ServerName)
	}
	
	t.Log("TLSConfig structure test completed")
}
