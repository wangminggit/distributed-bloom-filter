#!/bin/bash

# Quick Start Script for Integration Testing
# Author: David Wang
# Date: 2026-03-11

set -e

echo "🚀 Starting gRPC Server for Integration Testing..."
echo ""

# Configuration
PORT=8080
RAFT_PORT=8081
NODE_ID=node1
DATA_DIR=./data

# Clean previous data (optional - comment out to persist data)
echo "🧹 Cleaning previous data..."
rm -rf ${DATA_DIR}/*

# Start server
echo "📡 Starting gRPC server on port ${PORT}..."
echo "📊 Raft consensus on port ${RAFT_PORT}..."
echo ""

cd "$(dirname "$0")/.."

go run cmd/server/main.go \
    -port ${PORT} \
    -raft-port ${RAFT_PORT} \
    -node-id ${NODE_ID} \
    -bootstrap

echo ""
echo "✅ Server ready for testing!"
