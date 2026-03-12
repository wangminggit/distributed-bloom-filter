package grpc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"
	"github.com/golang-jwt/jwt/v5"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateTestCertificates creates self-signed CA and server/client certificates for testing.
func generateTestCertificates(t *testing.T, dir string) (caCertPath, serverCertPath, serverKeyPath, clientCertPath, clientKeyPath string) {
	// Generate CA private key
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Generate CA certificate
	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"DBF Test CA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	// Save CA certificate
	caCertPath = filepath.Join(dir, "ca.crt")
	caCertPEM := &pem.Block{Type: "CERTIFICATE", Bytes: caCertDER}
	caCertBytes := pem.EncodeToMemory(caCertPEM)
	err = os.WriteFile(caCertPath, caCertBytes, 0644)
	require.NoError(t, err)

	// Generate server private key
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Generate server certificate
	serverTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"DBF Server"},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		DNSNames:     []string{"localhost"},
	}

	serverCertDER, err := x509.CreateCertificate(rand.Reader, &serverTemplate, &caTemplate, &serverKey.PublicKey, caKey)
	require.NoError(t, err)

	// Save server certificate
	serverCertPath = filepath.Join(dir, "server.crt")
	serverCertPEM := &pem.Block{Type: "CERTIFICATE", Bytes: serverCertDER}
	serverCertBytes := pem.EncodeToMemory(serverCertPEM)
	err = os.WriteFile(serverCertPath, serverCertBytes, 0644)
	require.NoError(t, err)

	// Save server private key
	serverKeyPath = filepath.Join(dir, "server.key")
	serverKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)}
	serverKeyBytes := pem.EncodeToMemory(serverKeyPEM)
	err = os.WriteFile(serverKeyPath, serverKeyBytes, 0600)
	require.NoError(t, err)

	// Generate client private key
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Generate client certificate
	clientTemplate := x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			Organization: []string{"DBF Client"},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	clientCertDER, err := x509.CreateCertificate(rand.Reader, &clientTemplate, &caTemplate, &clientKey.PublicKey, caKey)
	require.NoError(t, err)

	// Save client certificate
	clientCertPath = filepath.Join(dir, "client.crt")
	clientCertPEM := &pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER}
	clientCertBytes := pem.EncodeToMemory(clientCertPEM)
	err = os.WriteFile(clientCertPath, clientCertBytes, 0644)
	require.NoError(t, err)

	// Save client private key
	clientKeyPath = filepath.Join(dir, "client.key")
	clientKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)}
	clientKeyBytes := pem.EncodeToMemory(clientKeyPEM)
	err = os.WriteFile(clientKeyPath, clientKeyBytes, 0600)
	require.NoError(t, err)

	return caCertPath, serverCertPath, serverKeyPath, clientCertPath, clientKeyPath
}

func TestNewAuthInterceptor(t *testing.T) {
	tmpDir := t.TempDir()
	caCertPath, _, _, _, _ := generateTestCertificates(t, tmpDir)

	t.Run("valid config with mTLS", func(t *testing.T) {
		config := &AuthConfig{
			EnableMTLS:   true,
			CACertPath:   caCertPath,
			ServerCertPath: filepath.Join(tmpDir, "server.crt"),
			ServerKeyPath:  filepath.Join(tmpDir, "server.key"),
		}

		interceptor, err := NewAuthInterceptor(config)
		assert.NoError(t, err)
		assert.NotNil(t, interceptor)
		assert.NotNil(t, interceptor.caCert)
	})

	t.Run("config without mTLS", func(t *testing.T) {
		config := &AuthConfig{
			EnableMTLS: false,
		}

		interceptor, err := NewAuthInterceptor(config)
		assert.NoError(t, err)
		assert.NotNil(t, interceptor)
		assert.Nil(t, interceptor.caCert)
	})

	t.Run("invalid CA cert path", func(t *testing.T) {
		config := &AuthConfig{
			EnableMTLS: true,
			CACertPath: "/nonexistent/ca.crt",
		}

		interceptor, err := NewAuthInterceptor(config)
		assert.Error(t, err)
		assert.Nil(t, interceptor)
	})
}

func TestValidateToken(t *testing.T) {
	tmpDir := t.TempDir()
	caCertPath, _, _, _, _ := generateTestCertificates(t, tmpDir)

	config := &AuthConfig{
		EnableMTLS:      true,
		CACertPath:      caCertPath,
		EnableTokenAuth: true,
		JWTSecretKey:    "test-secret-key-12345",
		TokenExpiry:     time.Hour,
	}

	interceptor, err := NewAuthInterceptor(config)
	require.NoError(t, err)

	t.Run("valid token", func(t *testing.T) {
		token, err := GenerateToken(config, "node1", []string{"read", "write"})
		require.NoError(t, err)

		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Bearer " + token},
		})

		err = interceptor.validateToken(ctx)
		assert.NoError(t, err)
	})

	t.Run("missing authorization header", func(t *testing.T) {
		ctx := context.Background()
		err = interceptor.validateToken(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing metadata")
	})

	t.Run("invalid token format", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"InvalidFormat"},
		})

		err = interceptor.validateToken(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid authorization format")
	})

	t.Run("expired token", func(t *testing.T) {
		expiredConfig := &AuthConfig{
			EnableTokenAuth: true,
			JWTSecretKey:    "test-secret-key-12345",
			TokenExpiry:     -time.Hour, // Already expired
		}

		token, err := GenerateToken(expiredConfig, "node1", []string{"read"})
		require.NoError(t, err)

		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Bearer " + token},
		})

		err = interceptor.validateToken(ctx)
		assert.Error(t, err)
		// JWT library returns "token has invalid claims: token is expired"
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("missing node_id", func(t *testing.T) {
		claims := TokenClaims{
			NodeID:      "",
			Permissions: []string{"read"},
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(config.JWTSecretKey))
		require.NoError(t, err)

		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{"Bearer " + tokenString},
		})

		err = interceptor.validateToken(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing node_id")
	})
}

func TestGenerateToken(t *testing.T) {
	config := &AuthConfig{
		EnableTokenAuth: true,
		JWTSecretKey:    "test-secret-key",
		TokenExpiry:     time.Hour,
	}

	t.Run("generate valid token", func(t *testing.T) {
		token, err := GenerateToken(config, "node1", []string{"read", "write", "admin"})
		assert.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("token with empty permissions", func(t *testing.T) {
		token, err := GenerateToken(config, "node2", []string{})
		assert.NoError(t, err)
		assert.NotEmpty(t, token)
	})
}

func TestLoadTLSCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	caCertPath, serverCertPath, serverKeyPath, _, _ := generateTestCertificates(t, tmpDir)

	t.Run("valid TLS credentials", func(t *testing.T) {
		config := &AuthConfig{
			EnableMTLS:     true,
			CACertPath:     caCertPath,
			ServerCertPath: serverCertPath,
			ServerKeyPath:  serverKeyPath,
		}

		creds, err := LoadTLSCredentials(config)
		assert.NoError(t, err)
		assert.NotNil(t, creds)
	})

	t.Run("invalid server cert path", func(t *testing.T) {
		config := &AuthConfig{
			EnableMTLS:     true,
			CACertPath:     caCertPath,
			ServerCertPath: "/nonexistent/server.crt",
			ServerKeyPath:  serverKeyPath,
		}

		creds, err := LoadTLSCredentials(config)
		assert.Error(t, err)
		assert.Nil(t, creds)
	})

	t.Run("invalid server key path", func(t *testing.T) {
		config := &AuthConfig{
			EnableMTLS:     true,
			CACertPath:     caCertPath,
			ServerCertPath: serverCertPath,
			ServerKeyPath:  "/nonexistent/server.key",
		}

		creds, err := LoadTLSCredentials(config)
		assert.Error(t, err)
		assert.Nil(t, creds)
	})
}

func TestAuthInterceptorInterceptors(t *testing.T) {
	tmpDir := t.TempDir()
	caCertPath, _, _, _, _ := generateTestCertificates(t, tmpDir)

	config := &AuthConfig{
		EnableMTLS:      true,
		CACertPath:      caCertPath,
		EnableTokenAuth: true,
		JWTSecretKey:    "test-secret",
		TokenExpiry:     time.Hour,
	}

	interceptor, err := NewAuthInterceptor(config)
	require.NoError(t, err)

	t.Run("unary interceptor exists", func(t *testing.T) {
		unaryInterceptor := interceptor.UnaryInterceptor()
		assert.NotNil(t, unaryInterceptor)
	})

	t.Run("stream interceptor exists", func(t *testing.T) {
		streamInterceptor := interceptor.StreamInterceptor()
		assert.NotNil(t, streamInterceptor)
	})
}
