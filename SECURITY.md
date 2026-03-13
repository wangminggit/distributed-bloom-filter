# Security Policy

## 📬 Reporting a Vulnerability

We take the security of our project seriously. If you believe you've found a security vulnerability, please report it responsibly.

### How to Report

1. **DO NOT** create a public GitHub issue for security vulnerabilities
2. Send a detailed report to the project maintainers via secure channels
3. Include the following information:
   - Description of the vulnerability
   - Steps to reproduce the issue
   - Potential impact assessment
   - Any suggested fixes (if available)

### Response Timeline

- We will acknowledge your report within **48 hours**
- We will provide a status update within **7 days**
- We aim to resolve critical issues within **30 days**

## 🔐 Security Best Practices

### General Guidelines

1. **Principle of Least Privilege**: Only grant necessary permissions to services and users
2. **Defense in Depth**: Implement multiple layers of security controls
3. **Regular Updates**: Keep all dependencies up to date with security patches
4. **Monitoring**: Enable logging and monitoring for suspicious activities
5. **Backup**: Maintain regular encrypted backups of critical data

### Code Review

- All security-sensitive code changes require review by at least one senior engineer
- Use automated security scanning tools in CI/CD pipeline
- Follow secure coding guidelines for Go applications

## 🔑 Encryption Configuration

### WAL Encryption

This project uses **AES-256-GCM** for Write-Ahead Log (WAL) encryption.

#### Key Management

```go
// Configuration constants
const (
    MaxKeyCacheSize = 100  // Maximum cached keys to prevent memory growth
    KeyCacheDuration = 5 * time.Minute  // Key cache expiration time
)
```

#### Best Practices

1. **Production Environment**:
   - Always use K8s Secrets or a secure key management service
   - Never use test mode (random keys) in production
   - Rotate keys regularly (recommended: every 90 days)

2. **Key Storage**:
   - Store keys in K8s Secrets with restricted access
   - Use mounted secret volumes, not environment variables
   - Ensure proper file permissions (0600 or stricter)

3. **Test Mode Warning**:
   - Test mode generates random keys that are NOT persisted
   - **Data will be lost on restart** when using test mode
   - Only use test mode for development and testing

### Key Rotation

To rotate encryption keys:

```go
encryptor.RotateKey()
```

The system maintains a cache of previous keys to decrypt existing data.

## 🔒 TLS/mTLS Configuration

### TLS Setup

For production deployments, always enable TLS:

1. **Obtain Certificates**:
   - Use Let's Encrypt for public services
   - Use internal CA for private services

2. **Configure TLS**:
   ```yaml
   tls:
     cert_file: /path/to/cert.pem
     key_file: /path/to/key.pem
     min_version: TLS1.2
   ```

3. **Cipher Suites**:
   - Use only strong cipher suites
   - Prefer AEAD modes (GCM, ChaCha20-Poly1305)

### mTLS (Mutual TLS)

For service-to-service communication:

1. **Certificate Authority**:
   - Set up internal CA for issuing client certificates
   - Maintain certificate revocation lists (CRL)

2. **Client Certificates**:
   - Require client certificates for all internal services
   - Validate certificate chain and expiration

3. **Configuration**:
   ```yaml
   mtls:
     enabled: true
     ca_file: /path/to/ca.pem
     client_auth: RequireAndVerifyClientCert
   ```

## 🛡️ Security Headers

When exposing HTTP endpoints, implement these security headers:

- `Strict-Transport-Security`: Enforce HTTPS
- `Content-Security-Policy`: Prevent XSS attacks
- `X-Content-Type-Options`: Prevent MIME sniffing
- `X-Frame-Options`: Prevent clickjacking

## 📋 Security Checklist

### Before Deployment

- [ ] All secrets stored in secure vault (K8s Secrets, HashiCorp Vault, etc.)
- [ ] TLS enabled with strong cipher suites
- [ ] mTLS configured for internal services
- [ ] Logging enabled (without sensitive data)
- [ ] Firewall rules configured (least privilege)
- [ ] Security scanning passed (no known vulnerabilities)

### Regular Maintenance

- [ ] Review and rotate encryption keys (quarterly)
- [ ] Update dependencies (monthly)
- [ ] Review access logs (weekly)
- [ ] Audit user permissions (quarterly)
- [ ] Security training for team (annually)

## 🚨 Incident Response

If a security incident occurs:

1. **Contain**: Isolate affected systems
2. **Assess**: Determine scope and impact
3. **Notify**: Alert stakeholders and users (if data exposed)
4. **Remediate**: Apply fixes and verify
5. **Review**: Conduct post-mortem and update processes

## 📚 Additional Resources

- [Go Security Best Practices](https://go.dev/doc/security)
- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes)

---

**Last Updated**: 2026-03-13  
**Version**: 1.0.0
