#!/bin/bash

# Example: Running Database Manager Integration Tests
# This script demonstrates how to run integration tests step by step

echo "🚀 Database Manager Integration Test Example"
echo "============================================="
echo ""

# Step 1: Start test databases
echo "📦 Step 1: Starting test databases..."
docker-compose -f pkg/db/docker-compose.test.yml up -d

echo "⏳ Waiting for databases to be ready..."
sleep 10

# Check if databases are healthy
echo "🔍 Checking database health..."
docker-compose -f pkg/db/docker-compose.test.yml ps

echo ""

# Step 2: Run local integration tests
echo "🧪 Step 2: Running local integration tests..."
echo "Command: ./pkg/db/test_runner.sh -e local -l debug"
echo ""

./pkg/db/test_runner.sh -e local -l debug

echo ""

# Step 3: Cleanup
echo "🧹 Step 3: Cleaning up..."
docker-compose -f pkg/db/docker-compose.test.yml down

echo ""
echo "✅ Example completed!"
echo ""
echo "To run tests manually:"
echo "  ./pkg/db/test_runner.sh -e local -l debug"
echo ""
echo "To run tests for different environments:"
echo "  ./pkg/db/test_runner.sh -e beta -l info"
echo "  ./pkg/db/test_runner.sh -e prod -l warn" 