package raft

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/hashicorp/raft"
)

// TLSStreamLayer implements raft.StreamLayer for TLS-encrypted connections.
// It wraps a TLS listener and provides TLS dialing for Raft node communication.
type TLSStreamLayer struct {
	listener   net.Listener
	tlsConfig  *tls.Config
	advertise  net.Addr
	serverName string
}

// TLSConfig holds the configuration for TLS encryption.
type TLSConfig struct {
	// CAFile is the path to the CA certificate file
	CAFile string

	// CertFile is the path to the certificate file
	CertFile string

	// KeyFile is the path to the private key file
	KeyFile string

	// ServerName is the expected server name for certificate verification
	ServerName string

	// InsecureSkipVerify disables certificate verification (development only)
	InsecureSkipVerify bool

	// MinVersion is the minimum TLS version to accept
	MinVersion uint16
}

// NewTLSStreamLayer creates a new TLS stream layer for Raft.
func NewTLSStreamLayer(bindAddr string, advertise net.Addr, tlsConfig *TLSConfig) (*TLSStreamLayer, error) {
	// Load server certificate
	cert, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	// Create CA certificate pool for client verification
	caCertPool := x509.NewCertPool()
	if tlsConfig.CAFile != "" {
		caCert, err := loadCAFile(tlsConfig.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA certificate: %w", err)
		}
		caCertPool.AddCert(caCert)
	}

	// Create TLS configuration for server
	serverTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert, // Enforce mTLS
		ClientCAs:    caCertPool,
		MinVersion:   tlsConfig.MinVersion,
		ServerName:   tlsConfig.ServerName,
	}

	// For development, allow insecure connections
	if tlsConfig.InsecureSkipVerify {
		serverTLSConfig.ClientAuth = tls.RequireAnyClientCert
	}

	// Listen on the bind address
	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	// Wrap with TLS
	tlsListener := tls.NewListener(listener, serverTLSConfig)

	return &TLSStreamLayer{
		listener:   tlsListener,
		tlsConfig:  serverTLSConfig,
		advertise:  advertise,
		serverName: tlsConfig.ServerName,
	}, nil
}

// loadCAFile loads a CA certificate from file.
func loadCAFile(caFile string) (*x509.Certificate, error) {
	caCertPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}

	caCert, err := x509.ParseCertificate(caCertPEM)
	if err != nil {
		return nil, err
	}

	return caCert, nil
}

// Accept accepts a new TLS connection.
func (t *TLSStreamLayer) Accept() (net.Conn, error) {
	return t.listener.Accept()
}

// Close closes the TLS listener.
func (t *TLSStreamLayer) Close() error {
	return t.listener.Close()
}

// Addr returns the address of the TLS listener.
func (t *TLSStreamLayer) Addr() net.Addr {
	if t.advertise != nil {
		return t.advertise
	}
	return t.listener.Addr()
}

// Dial creates a new outgoing TLS connection to a Raft node.
func (t *TLSStreamLayer) Dial(address raft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	// Create TLS config for client
	clientTLSConfig := t.tlsConfig.Clone()
	clientTLSConfig.ServerName = t.serverName
	clientTLSConfig.InsecureSkipVerify = false

	// Dial the remote address
	conn, err := net.DialTimeout("tcp", string(address), timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}

	// Wrap with TLS
	tlsConn := tls.Client(conn, clientTLSConfig)

	// Set deadline for handshake
	if err := tlsConn.SetDeadline(time.Now().Add(timeout)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to set deadline: %w", err)
	}

	// Perform TLS handshake
	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	// Clear deadline
	if err := tlsConn.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to clear deadline: %w", err)
	}

	return tlsConn, nil
}

// NewTCPTransportWithTLS creates a Raft TCP transport with TLS encryption.
// This is a convenience function that combines NewTLSStreamLayer with NewNetworkTransportWithConfig.
func NewTCPTransportWithTLS(
	bindAddr string,
	advertise net.Addr,
	maxPool int,
	timeout time.Duration,
	logOutput io.Writer,
	tlsConfig *TLSConfig,
) (*raft.NetworkTransport, error) {
	// Create TLS stream layer
	streamLayer, err := NewTLSStreamLayer(bindAddr, advertise, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS stream layer: %w", err)
	}

	// Create network transport config
	config := &raft.NetworkTransportConfig{
		Stream:        streamLayer,
		MaxPool:       maxPool,
		Timeout:       timeout,
		Logger:        nil, // Will use default logger
		MaxRPCsInFlight: 2, // Default value for good performance
	}

	// Create network transport
	transport := raft.NewNetworkTransportWithConfig(config)

	return transport, nil
}
