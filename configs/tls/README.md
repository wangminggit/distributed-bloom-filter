# TLS Configuration

This directory contains TLS certificates and keys for secure gRPC communication.

## 📁 Directory Structure

```
configs/tls/
├── README.md           # This file
├── server-cert.pem     # Server certificate
├── server-key.pem      # Server private key
├── client-cert.pem     # Client certificate (for mTLS)
├── client-key.pem      # Client private key (for mTLS)
└── ca-cert.pem         # CA certificate (for client verification)
```

## 🔐 Security Levels

### Level 1: TLS Encryption (Minimum)
- Encrypts all traffic between client and server
- Server identity verified by client
- **Files needed**: `server-cert.pem`, `server-key.pem`

### Level 2: Mutual TLS (Recommended for Production)
- Both server and client verify each other's identity
- Prevents unauthorized clients from connecting
- **Files needed**: All certificate files

## 🚀 Quick Start

### Generate Self-Signed Certificates (Development)

Run the following command to generate all necessary certificates:

```bash
cd configs/tls
./generate-certs.sh
```

This will create:
- Server certificate and key
- Client certificate and key  
- CA certificate for verification

**⚠️ Warning**: Self-signed certificates are for development only. For production, use certificates from a trusted CA like Let's Encrypt.

### Generate Certificates Manually

#### 1. Generate CA Certificate

```bash
# Generate CA private key
openssl genrsa -out ca-key.pem 4096

# Generate CA certificate
openssl req -new -x509 -key ca-key.pem -out ca-cert.pem -days 365 \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=MyCA"
```

#### 2. Generate Server Certificate

```bash
# Generate server private key
openssl genrsa -out server-key.pem 4096

# Generate server certificate signing request
openssl req -new -key server-key.pem -out server.csr \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=localhost"

# Sign server certificate with CA
openssl x509 -req -in server.csr -CA ca-cert.pem -CAkey ca-key.pem \
  -CAcreateserial -out server-cert.pem -days 365 \
  -extfile <(echo "subjectAltName=DNS:localhost,IP:127.0.0.1")
```

#### 3. Generate Client Certificate

```bash
# Generate client private key
openssl genrsa -out client-key.pem 4096

# Generate client certificate signing request
openssl req -new -key client-key.pem -out client.csr \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=client"

# Sign client certificate with CA
openssl x509 -req -in client.csr -CA ca-cert.pem -CAkey ca-key.pem \
  -CAcreateserial -out client-cert.pem -days 365
```

#### 4. Clean Up CSR Files

```bash
rm -f server.csr client.csr ca-cert.srl
```

## 🔧 Configuration

### Server Configuration

In your gRPC server configuration:

```go
config := grpc.ServerConfig{
    Port:        8443,
    EnableTLS:   true,
    TLSCertFile: "configs/tls/server-cert.pem",
    TLSKeyFile:  "configs/tls/server-key.pem",
    
    // For mTLS (client certificate verification)
    // Add client CA pool configuration
}
```

### Client Configuration

In your gRPC client configuration:

```go
// Load client certificates
creds, err := credentials.NewClientTLSFromFile(
    "configs/tls/ca-cert.pem", // CA certificate to verify server
    "localhost",               // Server name override
)
if err != nil {
    log.Fatal(err)
}

// For mTLS (client presents certificate to server)
cert, err := tls.LoadX509KeyPair(
    "configs/tls/client-cert.pem",
    "configs/tls/client-key.pem",
)
if err != nil {
    log.Fatal(err)
}

tlsConfig := creds.Info().(*credentials.TLSInfo).State.ClientVerification
tlsConfig.Certificates = []tls.Certificate{cert}
```

## 📋 Production Checklist

- [ ] Replace self-signed certificates with CA-signed certificates
- [ ] Use Let's Encrypt or commercial CA for public deployments
- [ ] Implement certificate rotation (auto-renewal)
- [ ] Store private keys securely (HSM, secret manager)
- [ ] Set appropriate file permissions (600 for keys, 644 for certs)
- [ ] Enable OCSP stapling for certificate revocation checking
- [ ] Monitor certificate expiration dates
- [ ] Document certificate renewal procedures

## 🔒 Security Best Practices

1. **Key Size**: Use at least 4096-bit RSA keys or 256-bit EC keys
2. **Validity Period**: Keep certificate validity short (90 days for Let's Encrypt)
3. **Key Storage**: Never commit private keys to version control
4. **Access Control**: Restrict file permissions (owner read-only for keys)
5. **Rotation**: Rotate certificates before expiration (automate with cert-manager)
6. **Revocation**: Implement CRL or OCSP for certificate revocation
7. **Cipher Suites**: Use strong cipher suites only (TLS 1.3 preferred)

## 🛠️ Troubleshooting

### Certificate Verification Failed

```bash
# Verify server certificate
openssl verify -CAfile ca-cert.pem server-cert.pem

# Verify client certificate
openssl verify -CAfile ca-cert.pem client-cert.pem

# Check certificate details
openssl x509 -in server-cert.pem -text -noout
```

### Connection Issues

```bash
# Test TLS connection
openssl s_client -connect localhost:8443 -CAfile ca-cert.pem

# Check certificate chain
openssl s_client -connect localhost:8443 -showcerts
```

## 📚 References

- [gRPC TLS Documentation](https://grpc.io/docs/guides/auth/)
- [OpenSSL Cookbook](https://www.feistyduck.com/library/openssl-cookbook/)
- [Let's Encrypt](https://letsencrypt.org/)
- [TLS 1.3 RFC 8446](https://tools.ietf.org/html/rfc8446)

---

*Last updated: 2026-03-14*
