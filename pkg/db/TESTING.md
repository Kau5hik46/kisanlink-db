# Database Manager Integration Testing

This document explains how to run integration tests for the database manager to verify connections and functionality in different environments.

## Overview

The integration tests verify:
- ✅ Database connection establishment
- ✅ Connection pooling and health checks
- ✅ Read/write operations
- ✅ Read replica load balancing
- ✅ Circuit breaker functionality
- ✅ Error handling and recovery
- ✅ Graceful shutdown

## Prerequisites

### Required Tools
- Go 1.21 or later
- Docker and Docker Compose (for local testing)
- AWS CLI (for DynamoDB testing in beta/prod)

### Dependencies
```bash
# Install required Go dependencies
go mod tidy
```

## Quick Start

### 1. Start Test Databases
```bash
docker-compose -f pkg/db/docker-compose.test.yml up -d
```

### 2. Run Integration Tests
```bash
# Local testing with debug logging
./pkg/db/test_runner.sh -e local -l debug

# Beta testing
./pkg/db/test_runner.sh -e beta -l info

# Production testing
./pkg/db/test_runner.sh -e prod -l warn
```

## Test Runner Options
- `-e, --environment`: local, beta, prod
- `-l, --log-level`: debug, info, warn, error
- `-t, --timeout`: Test timeout in seconds
- `-h, --help`: Show help

## Manual Testing
```bash
# Run specific environment tests
RUN_INTEGRATION_TESTS=true TEST_ENVIRONMENT=local go test -v -run TestIntegrationLocal ./pkg/db

# Run all integration tests
RUN_INTEGRATION_TESTS=true go test -v ./pkg/db
```

## What Tests Verify
- ✅ Database connection establishment
- ✅ Connection pooling and health checks
- ✅ Read/write operations
- ✅ Read replica load balancing
- ✅ Circuit breaker functionality
- ✅ Error handling and recovery

## Troubleshooting
1. Ensure Docker containers are running: `docker-compose -f pkg/db/docker-compose.test.yml ps`
2. Check environment variables are set correctly
3. Increase timeout if needed: `-t 120`
4. Use debug logging for detailed output: `-l debug`

## Environment Configurations

### Local Environment
The local environment uses Docker containers for all databases:

```bash
# PostgreSQL
DB_PRIMARY_BACKEND=gorm
DB_POSTGRES_HOST=localhost
DB_POSTGRES_PORT=5432
DB_POSTGRES_USER=postgres
DB_POSTGRES_PASSWORD=password
DB_POSTGRES_DBNAME=kisanlink_test
DB_POSTGRES_SSLMODE=disable
DB_POSTGRES_MAX_CONNS=5
DB_POSTGRES_IDLE_CONNS=2
DB_POSTGRES_READ_REPLICAS=localhost:5433

# DynamoDB Local
DB_DYNAMO_REGION=us-east-1
DB_DYNAMO_TABLE=kisanlink_test

# SpiceDB
DB_SPICEDB_ENDPOINT=localhost:50051
DB_SPICEDB_TOKEN=test-token
```

### Beta Environment
For beta testing, you need to set up your own database instances and configure the environment variables accordingly.

### Production Environment
For production testing, use your production database credentials and ensure proper security measures.

## Test Output

The integration tests provide detailed, colored output showing:

### Connection Status
```
🚀 Starting Database Manager Integration Tests
📋 Environment Configuration
📦 Creating Database Manager...
🔌 Connecting to Database Backends...
✅ Database Manager Connected (connection_time=1.2s)
```

### Backend Testing
```
🐘 Testing PostgreSQL Connection...
✅ PostgreSQL Connected (response_time=50ms)
🔍 Testing PostgreSQL Operations...
✅ PostgreSQL read operation successful (read_time=10ms)
✅ PostgreSQL transaction successful (write_time=15ms)

⚡ Testing DynamoDB Connection...
✅ DynamoDB Connected (response_time=100ms)
🔍 Testing DynamoDB Operations...
✅ DynamoDB operation successful (operation_time=25ms)

🔐 Testing SpiceDB Connection...
✅ SpiceDB Connected (response_time=75ms)
🔍 Testing SpiceDB Operations...
✅ SpiceDB operation successful (operation_time=20ms)
```

### Test Summary
```
📊 Test Results Summary
✅ Backend Available (backend=gorm, response_time=50ms)
✅ Backend Available (backend=dynamodb, response_time=100ms)
✅ Backend Available (backend=spicedb, response_time=75ms)
🎯 Summary (total_backends=3, available_backends=3, unavailable_backends=0)
```

## Continuous Integration

### GitHub Actions Example
```yaml
name: Database Integration Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: password
          POSTGRES_DB: kisanlink_test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5432:5432

    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run Integration Tests
        env:
          RUN_INTEGRATION_TESTS: true
          TEST_ENVIRONMENT: local
          DB_LOG_LEVEL: info
          DB_PRIMARY_BACKEND: gorm
          DB_POSTGRES_HOST: localhost
          DB_POSTGRES_PORT: 5432
          DB_POSTGRES_USER: postgres
          DB_POSTGRES_PASSWORD: password
          DB_POSTGRES_DBNAME: kisanlink_test
          DB_POSTGRES_SSLMODE: disable
        run: |
          go test -v -run TestIntegrationLocal ./pkg/db
```

## Performance Testing

For performance testing, you can modify the test configuration:

```go
config := TestConfig{
    Environment: "local",
    LogLevel:    "info",
    Timeout:     300 * time.Second, // 5 minutes
}
```

## Security Considerations

1. **Never commit real credentials** to version control
2. **Use environment variables** for sensitive configuration
3. **Use test databases** for integration testing
4. **Clean up test data** after testing
5. **Use read-only credentials** when possible for production testing

## Support

If you encounter issues with the integration tests:

1. Check the troubleshooting section above
2. Verify all prerequisites are installed
3. Ensure Docker containers are healthy
4. Check environment variable configuration
5. Review the detailed logs with debug level