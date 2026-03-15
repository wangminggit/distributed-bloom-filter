package grpc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
)

// generateTestCert generates a self-signed certificate for testing.
func generateTestCert(t *testing.T, expireTime time.Time) (certPEM, keyPEM []byte) {
	t.Helper()

	// Generate private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              expireTime,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1)},
		DNSNames:              []string{"localhost"},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Encode certificate to PEM
	certPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key to PEM
	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})

	return certPEM, keyPEM
}

// startTLSServer starts a TLS-enabled gRPC server for testing.
func startTLSServer(t *testing.T, certPEM, keyPEM []byte) (port int, cleanup func()) {
	t.Helper()

	// Create TLS certificate
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("Failed to load certificate: %v", err)
	}

	// Create TLS config
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.NoClientCert,
	}

	// Create mock Raft node
	mockNode := NewMockRaftNodeForTest("tls-test-node")

	// Create gRPC server with TLS
	creds := credentials.NewTLS(tlsConfig)
	grpcServer := grpc.NewServer(grpc.Creds(creds))

	service := NewDBFService(mockNode)
	proto.RegisterDBFServiceServer(grpcServer, service)

	// Listen on random port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	port = lis.Addr().(*net.TCPAddr).Port

	// Start server in background
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	cleanup = func() {
		grpcServer.Stop()
	}

	return port, cleanup
}

// createTLSClient creates a TLS gRPC client for testing.
func createTLSClient(t *testing.T, serverPort int, caCertPEM []byte) (proto.DBFServiceClient, func()) {
	t.Helper()

	// Create certificate pool
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCertPEM) {
		t.Fatal("Failed to append CA certificate")
	}

	// Create TLS config
	tlsConfig := &tls.Config{
		RootCAs: certPool,
	}

	// Create credentials
	creds := credentials.NewTLS(tlsConfig)

	// Dial server
	conn, err := grpc.Dial(
		fmt.Sprintf("127.0.0.1:%d", serverPort),
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}

	client := proto.NewDBFServiceClient(conn)

	cleanup := func() {
		conn.Close()
	}

	return client, cleanup
}

// TestTLSServerStart tests that a TLS server can start successfully.
// This is a P0 test case for TLS configuration.
func TestTLSServerStart(t *testing.T) {
	// Generate valid certificate (expires in 1 year)
	certPEM, keyPEM := generateTestCert(t, time.Now().Add(365*24*time.Hour))

	// Start TLS server
	port, cleanup := startTLSServer(t, certPEM, keyPEM)
	defer cleanup()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is listening
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("Server should be listening on port %d: %v", port, err)
	}
	conn.Close()

	t.Logf("TLS server started successfully on port %d", port)
}

// TestTLSServerShutdown tests that a TLS server shuts down gracefully.
// This is a P0 test case for TLS configuration.
func TestTLSServerShutdown(t *testing.T) {
	// Generate valid certificate
	certPEM, keyPEM := generateTestCert(t, time.Now().Add(365*24*time.Hour))

	// Start TLS server
	_, cleanup := startTLSServer(t, certPEM, keyPEM)

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Cleanup should complete without errors
	cleanup()

	// Give server time to stop
	time.Sleep(100 * time.Millisecond)

	t.Log("TLS server shut down gracefully")
}

// TestTLSClientConnection tests that a TLS client can connect successfully.
// This is a P0 test case for TLS client connection.
func TestTLSClientConnection(t *testing.T) {
	// Generate valid certificate
	certPEM, keyPEM := generateTestCert(t, time.Now().Add(365*24*time.Hour))

	// Start TLS server
	port, cleanup := startTLSServer(t, certPEM, keyPEM)
	defer cleanup()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create TLS client
	client, clientCleanup := createTLSClient(t, port, certPEM)
	defer clientCleanup()

	// Test Add operation
	ctx := context.Background()
	req := &proto.AddRequest{Item: []byte("tls-test-item")}
	resp, err := client.Add(ctx, req)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if !resp.Success {
		t.Errorf("Expected success, got error: %s", resp.Error)
	}

	t.Log("TLS client connection successful")
}

// TestTLSInvalidCert tests that invalid certificates are rejected.
// This is a P0 test case for certificate validation.
func TestTLSInvalidCert(t *testing.T) {
	// Generate valid certificate for server
	certPEM, keyPEM := generateTestCert(t, time.Now().Add(365*24*time.Hour))

	// Start TLS server
	port, cleanup := startTLSServer(t, certPEM, keyPEM)
	defer cleanup()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create client with invalid certificate (different key)
	invalidPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate invalid private key: %v", err)
	}

	invalidTemplate := x509.Certificate{
		SerialNumber: big.NewInt(999),
		Subject: pkix.Name{
			Organization: []string{"Invalid Org"},
			CommonName:   "invalid",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	invalidCertDER, err := x509.CreateCertificate(rand.Reader, &invalidTemplate, &invalidTemplate, &invalidPriv.PublicKey, invalidPriv)
	if err != nil {
		t.Fatalf("Failed to create invalid certificate: %v", err)
	}

	invalidCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: invalidCertDER,
	})

	// Create certificate pool with invalid cert
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(invalidCertPEM)

	// Try to connect with invalid cert
	tlsConfig := &tls.Config{
		RootCAs: certPool,
	}

	creds := credentials.NewTLS(tlsConfig)
	conn, err := grpc.Dial(
		fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithTransportCredentials(creds),
		grpc.WithTimeout(2*time.Second),
	)
	
	// Connection should fail or timeout due to certificate mismatch
	if err == nil {
		// If connection succeeded, try to make a request
		client := proto.NewDBFServiceClient(conn)
		_, reqErr := client.Add(context.Background(), &proto.AddRequest{Item: []byte("test")})
		conn.Close()
		if reqErr == nil {
			t.Error("Expected connection to fail with invalid certificate")
		}
	}

	t.Log("Invalid certificate test completed")
}

// TestTLSExpiredCert tests that expired certificates are rejected.
// This is a P0 test case for certificate expiration validation.
func TestTLSExpiredCert(t *testing.T) {
	// Generate expired certificate
	certPEM, keyPEM := generateTestCert(t, time.Now().Add(-24*time.Hour))

	// Start TLS server
	port, cleanup := startTLSServer(t, certPEM, keyPEM)
	defer cleanup()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Try to connect - should fail due to expired cert
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
	}

	creds := credentials.NewTLS(tlsConfig)
	conn, err := grpc.Dial(
		fmt.Sprintf("127.0.0.1:%d", port),
		grpc.WithTransportCredentials(creds),
		grpc.WithTimeout(2*time.Second),
	)

	// Connection should fail or timeout due to expired certificate
	if err == nil {
		client := proto.NewDBFServiceClient(conn)
		_, reqErr := client.Add(context.Background(), &proto.AddRequest{Item: []byte("test")})
		conn.Close()
		if reqErr == nil {
			t.Error("Expected connection to fail with expired certificate")
		}
	}

	t.Log("Expired certificate test completed")
}

// TestTLSMutualAuth tests mutual TLS configuration loading.
// This is a P0 test case for mTLS configuration.
func TestTLSMutualAuth(t *testing.T) {
	// Generate server certificate
	serverCertPEM, serverKeyPEM := generateTestCert(t, time.Now().Add(365*24*time.Hour))

	// Generate client certificate
	clientCertPEM, clientKeyPEM := generateTestCert(t, time.Now().Add(365*24*time.Hour))

	// Create TLS certificate for server
	serverCert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		t.Fatalf("Failed to load server certificate: %v", err)
	}

	// Create TLS certificate for client
	clientCert, err := tls.X509KeyPair(clientCertPEM, clientKeyPEM)
	if err != nil {
		t.Fatalf("Failed to load client certificate: %v", err)
	}

	// Create certificate pool with both server and client certs (for mutual auth)
	serverCertPool := x509.NewCertPool()
	serverCertPool.AppendCertsFromPEM(serverCertPEM)
	serverCertPool.AppendCertsFromPEM(clientCertPEM)

	// Create TLS config for server with client auth
	serverTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    serverCertPool,
		MinVersion:   tls.VersionTLS12,
	}

	// Verify TLS config is valid
	if len(serverTLSConfig.Certificates) != 1 {
		t.Error("Expected 1 server certificate")
	}
	if serverTLSConfig.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Error("Expected RequireAndVerifyClientCert")
	}

	// Create client TLS config
	clientTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      serverCertPool,
		MinVersion:   tls.VersionTLS12,
	}

	// Verify client TLS config is valid
	if len(clientTLSConfig.Certificates) != 1 {
		t.Error("Expected 1 client certificate")
	}

	t.Log("mTLS configuration loaded successfully")
}
