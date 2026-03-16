# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o /build/dbf ./cmd/server

# Build gateway (if exists)
# RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o /build/dbf-gateway ./cmd/gateway

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for TLS
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 dbf && \
    adduser -D -u 1000 -G dbf dbf

# Copy binary from builder
COPY --from=builder /build/dbf /app/dbf

# Create data directories
RUN mkdir -p /data/wal /data/snapshot && \
    chown -R dbf:dbf /data /app

# Switch to non-root user
USER dbf

# Expose ports
EXPOSE 50051 8081 9090

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/dbf", "--help"]

# Run server
ENTRYPOINT ["/app/dbf"]
CMD ["--port=50051", "--raft-port=8081", "--data-dir=/data"]
