package tls

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"sync"
	"time"
)

// Config holds TLS configuration for secure communication.
type Config struct {
	// CertPath is the path to the certificate file
	CertPath string
	// KeyPath is the path to the private key file
	KeyPath string
	// CAPath is the path to the CA certificate file (for client verification)
	CAPath string
	// MinVersion is the minimum TLS version supported
	MinVersion uint16
	// CipherSuites is the list of enabled cipher suites
	CipherSuites []uint16
	// ClientAuth specifies the client authentication policy
	ClientAuth tls.ClientAuthType
}

// CertReloader handles certificate hot reloading.
type CertReloader struct {
	config    *Config
	cert      *tls.Certificate
	caPool    *x509.CertPool
	mu        sync.RWMutex
	lastLoad  time.Time
	reloadDur time.Duration
}

// DefaultTLSConfig returns a secure default TLS configuration.
func DefaultTLSConfig() *Config {
	return &Config{
		MinVersion:   tls.VersionTLS13,
		CipherSuites: secureCipherSuites(),
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
}

// secureCipherSuites returns a list of secure cipher suites.
func secureCipherSuites() []uint16 {
	return []uint16{
		tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_AES_256_GCM_SHA384,
		tls.TLS_CHACHA20_POLY1305_SHA256,
	}
}

// LoadTLSCertificate loads a TLS certificate from disk.
func LoadTLSCertificate(certPath, keyPath string) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}
	return &cert, nil
}

// LoadCACertPool loads CA certificates into a cert pool.
func LoadCACertPool(caPath string) (*x509.CertPool, error) {
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return caPool, nil
}

// BuildTLSConfig builds a tls.Config from the given configuration.
func BuildTLSConfig(cfg *Config) (*tls.Config, error) {
	// Load server certificate
	cert, err := LoadTLSCertificate(cfg.CertPath, cfg.KeyPath)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   cfg.MinVersion,
		CipherSuites: cfg.CipherSuites,
		ClientAuth:   cfg.ClientAuth,
	}

	// Load CA for client verification if specified
	if cfg.CAPath != "" && cfg.ClientAuth != tls.NoClientCert {
		caPool, err := LoadCACertPool(cfg.CAPath)
		if err != nil {
			return nil, err
		}
		tlsConfig.ClientCAs = caPool
	}

	return tlsConfig, nil
}

// NewCertReloader creates a new certificate reloader.
func NewCertReloader(cfg *Config, reloadInterval time.Duration) (*CertReloader, error) {
	reloader := &CertReloader{
		config:    cfg,
		reloadDur: reloadInterval,
	}

	// Initial load
	if err := reloader.reload(); err != nil {
		return nil, fmt.Errorf("failed to load initial certificates: %w", err)
	}

	return reloader, nil
}

// reload loads certificates from disk.
func (r *CertReloader) reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cert, err := LoadTLSCertificate(r.config.CertPath, r.config.KeyPath)
	if err != nil {
		return err
	}
	r.cert = cert

	if r.config.CAPath != "" {
		caPool, err := LoadCACertPool(r.config.CAPath)
		if err != nil {
			return err
		}
		r.caPool = caPool
	}

	r.lastLoad = time.Now()
	return nil
}

// GetTLSConfig returns the current TLS config, reloading if necessary.
func (r *CertReloader) GetTLSConfig() (*tls.Config, error) {
	r.mu.RLock()
	lastLoad := r.lastLoad
	r.mu.RUnlock()

	// Check if reload is needed
	if time.Since(lastLoad) > r.reloadDur {
		if err := r.reload(); err != nil {
			return nil, err
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*r.cert},
		MinVersion:   r.config.MinVersion,
		CipherSuites: r.config.CipherSuites,
		ClientAuth:   r.config.ClientAuth,
		GetCertificate: func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			r.mu.RLock()
			defer r.mu.RUnlock()
			return r.cert, nil
		},
	}

	if r.caPool != nil {
		tlsConfig.ClientCAs = r.caPool
	}

	return tlsConfig, nil
}

// ValidateCertificate validates a certificate against the CA pool.
func ValidateCertificate(certPEM []byte, caPool *x509.CertPool) error {
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return fmt.Errorf("failed to decode PEM block containing certificate")
	}

	clientCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	_, err = clientCert.Verify(x509.VerifyOptions{
		Roots: caPool,
	})
	if err != nil {
		return fmt.Errorf("certificate validation failed: %w", err)
	}

	return nil
}

// GenerateSelfSignedCert generates a self-signed certificate for testing.
// This should NOT be used in production.
func GenerateSelfSignedCert(host string) (certPEM, keyPEM []byte, err error) {
	// For testing purposes, we'll use a simple approach
	// In production, use proper CA-signed certificates
	return nil, nil, fmt.Errorf("use certgen tool for certificate generation")
}
