# S3 Integration Tests

This document describes how to run the S3 integration tests for the database manager.

## Overview

The S3 integration tests verify that the S3 backend works correctly for file storage operations. These tests cover:

- File upload and download
- File metadata management
- Presigned URL generation
- File listing and filtering
- Folder operations (create/delete)
- CRUD operations through the DBManager interface
- Connection health checks

## Prerequisites

1. **AWS Credentials**: You need valid AWS credentials configured
2. **S3 Bucket**: An S3 bucket for testing
3. **Go Environment**: Go 1.24 or later

## Environment Variables

Set the following environment variables:

```bash
# Required
export DB_S3_BUCKET="your-test-bucket-name"

# Optional (defaults shown)
export DB_S3_REGION="us-east-1"
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_SESSION_TOKEN="your-session-token"  # if using temporary credentials

# Test configuration
export RUN_INTEGRATION_TESTS="true"
export TEST_ENVIRONMENT="local"
export DB_LOG_LEVEL="info"
```

## Running the Tests

### Method 1: Direct Go Test

```bash
# Run all S3 integration tests
go test -v ./pkg/db -run TestS3Integration

# Run local S3 integration tests
go test -v ./pkg/db -run TestS3IntegrationLocal
```

### Method 2: Using the Test Runner Script

```bash
# Navigate to the db package directory
cd pkg/db

# Run with local environment
./test_runner.sh -e local -l debug

# Run with custom timeout
./test_runner.sh -e local -l info -t 120
```

### Method 3: Manual Test Suite

```go
package main

import (
    "log"
    "time"
    
    "github.com/Kisanlink/kisanlink-db/pkg/db"
)

func main() {
    config := db.TestConfig{
        Environment: "local",
        LogLevel:    "debug",
        Timeout:     120 * time.Second,
    }
    
    suite := db.NewS3IntegrationTestSuite(config)
    if err := suite.RunAllS3Tests(); err != nil {
        log.Fatalf("S3 integration tests failed: %v", err)
    }
    
    log.Println("S3 integration tests completed successfully")
}
```

## Test Coverage

The S3 integration tests cover the following operations:

### 1. Connection Tests
- Basic connectivity verification
- Health check validation
- Connection status monitoring

### 2. File Upload Tests
- Simple text file upload
- JSON file upload with metadata
- Large file upload
- Content type and metadata validation

### 3. File Download Tests
- File download verification
- Content integrity checks

### 4. Metadata Tests
- File metadata retrieval
- Custom metadata validation
- Timestamp verification

### 5. Presigned URL Tests
- URL generation
- URL format validation
- Expiration parameter verification

### 6. File Listing Tests
- List all files in a folder
- Filter by prefix
- Subfolder listing

### 7. Folder Operations
- Folder creation
- File upload to folders
- Folder deletion with contents

### 8. File Deletion Tests
- Single file deletion
- Verification of deletion

### 9. CRUD Operations
- Create file records
- Retrieve file metadata
- Update file records
- List files
- Delete file records

## Test Data Management

The tests automatically:
- Create test files in organized folders
- Clean up all test data after completion
- Use unique prefixes to avoid conflicts

Test data is created under these prefixes:
- `test-files/` - General file operations
- `test-listing/` - File listing tests
- `test-folders/` - Folder operations

## Troubleshooting

### Common Issues

1. **Bucket Not Found**
   ```
   Error: S3 bucket not configured (DB_S3_BUCKET)
   ```
   Solution: Set the `DB_S3_BUCKET` environment variable

2. **Access Denied**
   ```
   Error: failed to connect to S3: operation error S3: HeadBucket, access denied
   ```
   Solution: Verify AWS credentials and bucket permissions

3. **Region Mismatch**
   ```
   Error: failed to connect to S3: operation error S3: HeadBucket, no such bucket
   ```
   Solution: Check if the bucket exists in the specified region

4. **Timeout Issues**
   ```
   Error: context deadline exceeded
   ```
   Solution: Increase the timeout value or check network connectivity

### Debug Mode

Enable debug logging for detailed information:

```bash
export DB_LOG_LEVEL="debug"
go test -v ./pkg/db -run TestS3Integration
```

### AWS Credentials

Ensure AWS credentials are properly configured:

```bash
# Using AWS CLI
aws configure

# Or set environment variables
export AWS_ACCESS_KEY_ID="your-key"
export AWS_SECRET_ACCESS_KEY="your-secret"
export AWS_DEFAULT_REGION="us-east-1"
```

## Security Considerations

- Use a dedicated test bucket
- Apply appropriate IAM policies
- Consider using temporary credentials
- Clean up test data regularly
- Monitor AWS costs

## Example IAM Policy

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:GetObject",
                "s3:PutObject",
                "s3:DeleteObject",
                "s3:ListBucket",
                "s3:HeadBucket",
                "s3:HeadObject"
            ],
            "Resource": [
                "arn:aws:s3:::your-test-bucket",
                "arn:aws:s3:::your-test-bucket/*"
            ]
        }
    ]
}
```

## Integration with CI/CD

Add to your CI/CD pipeline:

```yaml
# GitHub Actions example
- name: Run S3 Integration Tests
  env:
    DB_S3_BUCKET: ${{ secrets.S3_TEST_BUCKET }}
    AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
    AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
    RUN_INTEGRATION_TESTS: "true"
  run: |
    go test -v ./pkg/db -run TestS3Integration
```

## Performance Considerations

- Tests use exponential backoff for retries
- Circuit breaker pattern for fault tolerance
- Health checks every 30 seconds
- Connection pooling for efficiency

## Monitoring

The tests provide detailed logging including:
- Connection times
- Operation latencies
- Error details
- Success/failure counts
- Resource cleanup status 