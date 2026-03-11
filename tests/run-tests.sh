#!/bin/bash

# Integration Test Runner
# Author: David Wang
# Date: 2026-03-11

set -e

echo "🧪 Running Integration Tests..."
echo ""

cd "$(dirname "$0")"

# Run integration tests
echo "📊 Executing test cases..."
go test -v ./integration/... -timeout 30s

echo ""
echo "✅ All integration tests completed!"
