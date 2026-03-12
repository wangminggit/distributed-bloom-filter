#!/bin/bash
# generate-certs.sh - Generate self-signed CA and server/client certificates for mTLS
# Usage: ./scripts/generate-certs.sh [output_directory]

set -e

OUTPUT_DIR="${1:-./certs}"
CA_NAME="DBF-CA"
SERVER_NAME="DBF-Server"
CLIENT_NAME="DBF-Client"
VALIDITY_DAYS=365

echo "🔐 Generating mTLS certificates for DBF project..."
echo "Output directory: $OUTPUT_DIR"

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Generate CA private key
echo "Generating CA private key..."
openssl genrsa -out "$OUTPUT_DIR/ca.key" 4096

# Generate CA certificate
echo "Generating CA certificate..."
openssl req -new -x509 -days $VALIDITY_DAYS \
    -key "$OUTPUT_DIR/ca.key" \
    -out "$OUTPUT_DIR/ca.crt" \
    -subj "/C=CN/ST=Beijing/L=Beijing/O=DBF/OU=Security/CN=$CA_NAME"

# Generate server private key
echo "Generating server private key..."
openssl genrsa -out "$OUTPUT_DIR/server.key" 2048

# Generate server certificate signing request (CSR)
echo "Generating server CSR..."
openssl req -new -key "$OUTPUT_DIR/server.key" \
    -out "$OUTPUT_DIR/server.csr" \
    -subj "/C=CN/ST=Beijing/L=Beijing/O=DBF/OU=Server/CN=$SERVER_NAME"

# Create server certificate extensions config
cat > "$OUTPUT_DIR/server_ext.cnf" << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = dbf-server
IP.1 = 127.0.0.1
EOF

# Sign server certificate with CA
echo "Signing server certificate..."
openssl x509 -req -days $VALIDITY_DAYS \
    -in "$OUTPUT_DIR/server.csr" \
    -CA "$OUTPUT_DIR/ca.crt" \
    -CAkey "$OUTPUT_DIR/ca.key" \
    -CAcreateserial \
    -out "$OUTPUT_DIR/server.crt" \
    -extfile "$OUTPUT_DIR/server_ext.cnf"

# Generate client private key
echo "Generating client private key..."
openssl genrsa -out "$OUTPUT_DIR/client.key" 2048

# Generate client certificate signing request (CSR)
echo "Generating client CSR..."
openssl req -new -key "$OUTPUT_DIR/client.key" \
    -out "$OUTPUT_DIR/client.csr" \
    -subj "/C=CN/ST=Beijing/L=Beijing/O=DBF/OU=Client/CN=$CLIENT_NAME"

# Create client certificate extensions config
cat > "$OUTPUT_DIR/client_ext.cnf" << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
EOF

# Sign client certificate with CA
echo "Signing client certificate..."
openssl x509 -req -days $VALIDITY_DAYS \
    -in "$OUTPUT_DIR/client.csr" \
    -CA "$OUTPUT_DIR/ca.crt" \
    -CAkey "$OUTPUT_DIR/ca.key" \
    -CAcreateserial \
    -out "$OUTPUT_DIR/client.crt" \
    -extfile "$OUTPUT_DIR/client_ext.cnf"

# Clean up temporary files
rm -f "$OUTPUT_DIR/server.csr" "$OUTPUT_DIR/client.csr"
rm -f "$OUTPUT_DIR/server_ext.cnf" "$OUTPUT_DIR/client_ext.cnf"
rm -f "$OUTPUT_DIR/ca.srl"

# Set proper permissions
chmod 600 "$OUTPUT_DIR"/*.key
chmod 644 "$OUTPUT_DIR"/*.crt

# Verify certificates
echo ""
echo "✅ Certificates generated successfully!"
echo ""
echo "Certificate files:"
ls -la "$OUTPUT_DIR"/*.crt "$OUTPUT_DIR"/*.key
echo ""

# Display certificate information
echo "📋 CA Certificate Info:"
openssl x509 -in "$OUTPUT_DIR/ca.crt" -noout -subject -issuer -dates
echo ""

echo "📋 Server Certificate Info:"
openssl x509 -in "$OUTPUT_DIR/server.crt" -noout -subject -issuer -dates
echo ""

echo "📋 Client Certificate Info:"
openssl x509 -in "$OUTPUT_DIR/client.crt" -noout -subject -issuer -dates
echo ""

# Verify certificate chain
echo "🔍 Verifying certificate chain..."
openssl verify -CAfile "$OUTPUT_DIR/ca.crt" "$OUTPUT_DIR/server.crt"
openssl verify -CAfile "$OUTPUT_DIR/ca.crt" "$OUTPUT_DIR/client.crt"
echo ""

echo "✅ All certificates verified successfully!"
echo ""
echo "Usage:"
echo "  Server: --enable-mtls --ca-cert $OUTPUT_DIR/ca.crt --server-cert $OUTPUT_DIR/server.crt --server-key $OUTPUT_DIR/server.key"
echo "  Client: Use client.crt and client.key for mutual TLS authentication"
