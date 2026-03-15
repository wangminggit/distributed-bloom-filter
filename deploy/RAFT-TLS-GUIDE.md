# Raft TLS Configuration Guide

## Overview

Raft node-to-node communication is now encrypted using TLS with mutual authentication (mTLS). This ensures:
- **Confidentiality**: All Raft consensus traffic is encrypted
- **Integrity**: Messages cannot be tampered with
- **Authentication**: Both server and client verify each other's identity

## Quick Start

### Development (Self-Signed Certificates)

1. **Generate certificates** (if not already done):
   ```bash
   cd configs/tls
   ./generate-certs.sh
   ```

2. **Configure Raft node** with TLS enabled:
   ```go
   config := raft.DefaultConfig()
   config.TLSEnabled = true
   config.TLSConfig = &raft.TLSRaftConfig{
       CAFile:   "configs/tls/ca-cert.pem",
       CertFile: "configs/tls/server-cert.pem",
       KeyFile:  "configs/tls/server-key.pem",
       ServerName: "localhost",
       MinVersion: tls.VersionTLS12,
   }
   
   node, err := raft.NewNode(config, bloomFilter, walEncryptor, metadataService)
   ```

3. **Start the node** - TLS is automatically enabled:
   ```bash
   go run cmd/server/main.go --node-id=node1 --raft-port=7001 --bootstrap
   ```

### Production (CA-Signed Certificates)

1. **Obtain certificates** from a trusted CA or internal PKI:
   - Server certificate with SANs for all node hostnames/IPs
   - Client certificate for each node
   - CA certificate chain

2. **Configure TLS** in your deployment:
   ```yaml
   # configs/tls/raft-tls-config.yaml
   raft_tls:
     enabled: true
     ca_file: /etc/ssl/certs/ca.pem
     cert_file: /etc/ssl/certs/server.pem
     key_file: /etc/ssl/private/server.key
     server_name: "raft.cluster.local"
     min_version: "TLS1.3"
   
   mtls:
     required: true
     client_cert_file: /etc/ssl/certs/client.pem
     client_key_file: /etc/ssl/private/client.key
   ```

3. **Deploy with Kubernetes** (example):
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: raft-tls-certs
   type: kubernetes.io/tls
   data:
     ca.crt: <base64-encoded-ca-cert>
     tls.crt: <base64-encoded-server-cert>
     tls.key: <base64-encoded-server-key>
   ```

## Configuration Options

### TLSRaftConfig Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `CAFile` | string | Yes | Path to CA certificate for verifying peer certificates |
| `CertFile` | string | Yes | Path to server certificate |
| `KeyFile` | string | Yes | Path to server private key |
| `ClientCertFile` | string | No | Path to client certificate (for outbound connections) |
| `ClientKeyFile` | string | No | Path to client private key |
| `ServerName` | string | Yes | Expected server name for certificate verification |
| `MinVersion` | uint16 | No | Minimum TLS version (default: TLS 1.2) |
| `InsecureSkipVerify` | bool | No | Skip certificate verification (development only!) |

### Security Best Practices

1. **Use TLS 1.3** when possible:
   ```go
   config.TLSConfig.MinVersion = tls.VersionTLS13
   ```

2. **Never disable certificate verification** in production:
   ```go
   config.TLSConfig.InsecureSkipVerify = false // Always!
   ```

3. **Use strong cipher suites** (TLS 1.3 has secure defaults)

4. **Rotate certificates** before expiration:
   - Set up monitoring for certificate expiry
   - Automate renewal with cert-manager or similar

5. **Protect private keys**:
   - File permissions: `chmod 600 *.key`
   - Use secret management (Vault, AWS Secrets Manager, etc.)
   - Never commit keys to version control

## Testing TLS

### Verify Certificate Chain

```bash
# Verify server certificate
openssl verify -CAfile configs/tls/ca-cert.pem configs/tls/server-cert.pem

# Verify client certificate
openssl verify -CAfile configs/tls/ca-cert.pem configs/tls/client-cert.pem
```

### Test TLS Connection

```bash
# Test TLS handshake
openssl s_client -connect localhost:7001 -CAfile configs/tls/ca-cert.pem \
  -cert configs/tls/client-cert.pem -key configs/tls/client-key.pem
```

### Check Raft Logs

Look for TLS-related log messages:
```
Raft node node1: TLS transport enabled on 127.0.0.1:7001
```

If TLS is disabled (development):
```
Raft node node1: Plain TCP transport enabled on 127.0.0.1:7001 (INSECURE)
```

## Troubleshooting

### Certificate Verification Failed

**Error**: `TLS handshake failed: x509: certificate signed by unknown authority`

**Solution**:
- Ensure CA certificate is correct and included in `CAFile`
- Verify certificate chain: `openssl verify -CAfile ca-cert.pem server-cert.pem`

### Hostname Mismatch

**Error**: `TLS handshake failed: x509: certificate is valid for X, not Y`

**Solution**:
- Set `ServerName` to match the certificate's CN or SAN
- Ensure certificate includes all node hostnames/IPs in SAN

### Connection Refused

**Error**: `failed to dial 127.0.0.1:7001: connection refused`

**Solution**:
- Verify the remote node is running
- Check firewall rules allow Raft port
- Ensure TLS is enabled on both nodes

### mTLS Issues

**Error**: `remote error: tls: bad certificate`

**Solution**:
- Ensure both nodes have valid certificates signed by the same CA
- Check client certificate is configured for outbound connections
- Verify `ClientAuth` is set to `RequireAndVerifyClientCert`

## Migration from Plain TCP

To migrate existing clusters from plain TCP to TLS:

1. **Generate certificates** for all nodes
2. **Update configuration** to enable TLS on all nodes
3. **Rolling restart** - nodes can temporarily run in mixed mode:
   - New nodes with TLS can connect to old nodes without TLS
   - Once all nodes have TLS, disable plain TCP fallback
4. **Verify** all connections are encrypted

## Performance Considerations

- **TLS Handshake**: Initial connection setup is slower (~1-2ms)
- **Encryption Overhead**: Minimal with modern CPUs (AES-NI)
- **Connection Pooling**: Raft maintains connection pools to amortize handshake cost
- **Recommended**: Keep `MaxPool` at default (3) or higher for busy clusters

## Compliance

TLS encryption for Raft consensus helps meet requirements for:
- **SOC 2**: Encryption in transit
- **HIPAA**: Data protection
- **GDPR**: Security of processing
- **PCI DSS**: Encryption of cardholder data

---

*Last updated: 2026-03-14*
