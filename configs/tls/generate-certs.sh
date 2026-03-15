#!/bin/bash

# TLS Certificate Generation Script
# Generates self-signed certificates for development and testing
# 
# ⚠️  WARNING: These certificates are for development only!
#              For production, use certificates from a trusted CA.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🔐 TLS Certificate Generator"
echo "============================"
echo ""
echo "⚠️  WARNING: Generating self-signed certificates for development only!"
echo "   DO NOT use these certificates in production!"
echo ""

# Configuration
DAYS_VALID=365
KEY_SIZE=4096
COUNTRY="US"
STATE="State"
LOCALITY="City"
ORGANIZATION="Distributed Bloom Filter"
ORGANIZATIONAL_UNIT="Development"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if OpenSSL is installed
if ! command -v openssl &> /dev/null; then
    log_error "OpenSSL is not installed. Please install it first."
    exit 1
fi

# Clean up old certificates
log_info "Cleaning up old certificates..."
rm -f ca-key.pem ca-cert.pem ca-cert.srl
rm -f server-key.pem server-cert.pem server.csr
rm -f client-key.pem client-cert.pem client.csr

# 1. Generate CA Certificate
log_info "Generating CA private key and certificate..."
openssl genrsa -out ca-key.pem $KEY_SIZE 2>/dev/null

openssl req -new -x509 -key ca-key.pem -out ca-cert.pem -days $DAYS_VALID \
    -subj "/C=$COUNTRY/ST=$STATE/L=$LOCALITY/O=$ORGANIZATION/OU=$ORGANIZATIONAL_UNIT/CN=Development CA" \
    2>/dev/null

log_info "✓ CA certificate generated (ca-cert.pem)"

# 2. Generate Server Certificate
log_info "Generating server private key and certificate..."
openssl genrsa -out server-key.pem $KEY_SIZE 2>/dev/null

openssl req -new -key server-key.pem -out server.csr \
    -subj "/C=$COUNTRY/ST=$STATE/L=$LOCALITY/O=$ORGANIZATION/OU=$ORGANIZATIONAL_UNIT/CN=localhost" \
    2>/dev/null

# Create extension file for Subject Alternative Names
cat > server-ext.cnf <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = *.localhost
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

openssl x509 -req -in server.csr -CA ca-cert.pem -CAkey ca-key.pem \
    -CAcreateserial -out server-cert.pem -days $DAYS_VALID \
    -extfile server-ext.cnf 2>/dev/null

log_info "✓ Server certificate generated (server-cert.pem)"
rm -f server-ext.cnf

# 3. Generate Client Certificate
log_info "Generating client private key and certificate..."
openssl genrsa -out client-key.pem $KEY_SIZE 2>/dev/null

openssl req -new -key client-key.pem -out client.csr \
    -subj "/C=$COUNTRY/ST=$STATE/L=$LOCALITY/O=$ORGANIZATION/OU=$ORGANIZATIONAL_UNIT/CN=client" \
    2>/dev/null

openssl x509 -req -in client.csr -CA ca-cert.pem -CAkey ca-key.pem \
    -CAcreateserial -out client-cert.pem -days $DAYS_VALID 2>/dev/null

log_info "✓ Client certificate generated (client-cert.pem)"

# 4. Clean up CSR files
log_info "Cleaning up temporary files..."
rm -f server.csr client.csr ca-cert.srl

# 5. Set appropriate file permissions
log_info "Setting file permissions..."
chmod 600 *-key.pem
chmod 644 *-cert.pem
chmod 644 ca-cert.pem

# 6. Verify certificates
log_info "Verifying certificates..."
echo ""

echo "CA Certificate:"
openssl x509 -in ca-cert.pem -noout -subject -issuer -dates
echo ""

echo "Server Certificate:"
openssl x509 -in server-cert.pem -noout -subject -issuer -dates
echo ""

echo "Client Certificate:"
openssl x509 -in client-cert.pem -noout -subject -issuer -dates
echo ""

# Verify certificate chain
log_info "Verifying certificate chain..."
if openssl verify -CAfile ca-cert.pem server-cert.pem > /dev/null 2>&1; then
    log_info "✓ Server certificate verification: PASSED"
else
    log_error "✗ Server certificate verification: FAILED"
fi

if openssl verify -CAfile ca-cert.pem client-cert.pem > /dev/null 2>&1; then
    log_info "✓ Client certificate verification: PASSED"
else
    log_error "✗ Client certificate verification: FAILED"
fi

echo ""
log_info "✅ Certificate generation complete!"
echo ""
echo "📁 Generated files:"
echo "   - ca-cert.pem     (CA certificate)"
echo "   - server-cert.pem (Server certificate)"
echo "   - server-key.pem  (Server private key)"
echo "   - client-cert.pem (Client certificate)"
echo "   - client-key.pem  (Client private key)"
echo ""
echo "🔒 File permissions set:"
echo "   - Private keys: 600 (owner read/write only)"
echo "   - Certificates: 644 (owner read/write, others read)"
echo ""
log_warn "Remember: These are self-signed certificates for development only!"
log_warn "For production, use certificates from a trusted CA like Let's Encrypt."
echo ""
