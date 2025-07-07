# Database Manager

A lightweight, multi-backend database connection manager for Go microservices. Supports PostgreSQL (GORM), DynamoDB, and SpiceDB with minimal configuration.

## Features

- **Multiple Backends**: Support for PostgreSQL, DynamoDB, and SpiceDB
- **Environment-based Configuration**: Easy configuration via environment variables
- **Connection Pooling**: Built-in connection pooling for PostgreSQL
- **Circuit Breaker**: Automatic circuit breaker for fault tolerance
- **Health Checks**: Periodic health monitoring
- **Structured Logging**: Comprehensive logging with Zap
- **Graceful Shutdown**: Proper connection cleanup

## Quick Start

### 1. Import the package

```go
import "github.com/Kisanlink/kisanlink-db/pkg/db"
```

### 2. Set environment variables

```bash
# Primary backend selection
export DB_PRIMARY_BACKEND=gorm

# PostgreSQL Configuration
export DB_POSTGRES_HOST=localhost
export DB_POSTGRES_PORT=5432
export DB_POSTGRES_USER=postgres
export DB_POSTGRES_PASSWORD=password
export DB_POSTGRES_DBNAME=kisanlink
export DB_POSTGRES_SSLMODE=disable
export DB_POSTGRES_MAX_CONNS=10
export DB_POSTGRES_IDLE_CONNS=5
export DB_POSTGRES_READ_REPLICAS=replica1:5432,replica2:5432

# DynamoDB Configuration (optional)
export DB_DYNAMO_REGION=us-east-1
export DB_DYNAMO_TABLE=kisanlink-table

# SpiceDB Configuration (optional)
export DB_SPICEDB_ENDPOINT=localhost:50051
export DB_SPICEDB_TOKEN=your-token

# Logging Configuration
export DB_LOG_LEVEL=info
```

### 3. Use in your microservice

```go
package main

import (
    "context"
    "log"
    "github.com/Kisanlink/kisanlink-db/pkg/db"
)

func main() {
    // Create database manager with environment-based configuration
    dbManager := db.NewDatabaseManager()
    
    ctx := context.Background()
    if err := dbManager.Connect(ctx); err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }
    defer dbManager.Close()

    // Use PostgreSQL
    if postgresManager := dbManager.GetPostgresManager(); postgresManager != nil {
        // Write operations (uses primary)
        err := postgresManager.WithTransaction(ctx, func(tx *gorm.DB) error {
            // Your database operations here
            return nil
        })
        if err != nil {
            log.Printf("Transaction failed: %v", err)
        }

        // Read operations (uses read replicas if available)
        err = postgresManager.WithReadOnly(ctx, func(tx *gorm.DB) error {
            // Your read-only operations here
            return nil
        })
        if err != nil {
            log.Printf("Read operation failed: %v", err)
        }
    }

    // Use DynamoDB
    if dynamoManager := dbManager.GetDynamoManager(); dynamoManager != nil {
        err := dynamoManager.WithClient(ctx, func(client *dynamodb.Client) error {
            // Your DynamoDB operations here
            return nil
        })
        if err != nil {
            log.Printf("DynamoDB operation failed: %v", err)
        }
    }

    // Use SpiceDB
    if spiceManager := dbManager.GetSpiceManager(); spiceManager != nil {
        err := spiceManager.WithClient(ctx, func(client *authzed.Client) error {
            // Your SpiceDB operations here
            return nil
        })
        if err != nil {
            log.Printf("SpiceDB operation failed: %v", err)
        }
    }
}
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_PRIMARY_BACKEND` | Primary database backend (`gorm`, `dynamodb`, `spicedb`) | `gorm` |
| `DB_POSTGRES_HOST` | PostgreSQL host | `localhost` |
| `DB_POSTGRES_PORT` | PostgreSQL port | `5432` |
| `DB_POSTGRES_USER` | PostgreSQL username | `postgres` |
| `DB_POSTGRES_PASSWORD` | PostgreSQL password | (required) |
| `DB_POSTGRES_DBNAME` | PostgreSQL database name | `kisanlink` |
| `DB_POSTGRES_SSLMODE` | PostgreSQL SSL mode | `disable` |
| `DB_POSTGRES_MAX_CONNS` | Maximum open connections | `10` |
| `DB_POSTGRES_IDLE_CONNS` | Maximum idle connections | `5` |
| `DB_POSTGRES_READ_REPLICAS` | Comma-separated list of read replica hosts | (optional) |
| `DB_DYNAMO_REGION` | AWS DynamoDB region | `us-east-1` |
| `DB_DYNAMO_TABLE` | DynamoDB table name | (required for DynamoDB) |
| `DB_SPICEDB_ENDPOINT` | SpiceDB endpoint | (required for SpiceDB) |
| `DB_SPICEDB_TOKEN` | SpiceDB API token | (required for SpiceDB) |
| `DB_LOG_LEVEL` | Logging level | `info` |

### Custom Configuration

```go
config := &db.Config{
    PrimaryBackend:      db.BackendGorm,
    PostgresHost:        "localhost",
    PostgresPort:        "5432",
    PostgresUser:        "postgres",
    PostgresPassword:    "password",
    PostgresDBName:      "kisanlink",
    PostgresSSLMode:     "disable",
    PostgresMaxConns:    10,
    PostgresIdleConns:   5,
    PostgresReadReplicas: []string{"replica1:5432", "replica2:5432"},
    LogLevel:            "info",
}
```

dbManager := db.NewDatabaseManagerWithConfig(config)
```

## Backend Support

### PostgreSQL (GORM)

- Connection pooling
- Transaction support
- Read replica support
- Automatic migrations (via GORM)

#### Read Replicas

PostgreSQL supports read replicas for load balancing read operations:

```bash
# Configure read replicas (comma-separated)
export DB_POSTGRES_READ_REPLICAS=replica1:5432,replica2:5432,replica3:5432
```

The manager automatically distributes read operations across available replicas using round-robin load balancing.

### DynamoDB

- AWS SDK v2 integration
- Automatic AWS credential loading
- Table management
- Batch operations

### SpiceDB

- Authzed client integration
- Schema management
- Permission checking
- Relationship queries

## Health Monitoring

The database manager includes built-in health monitoring:

- Periodic health checks (every 30 seconds)
- Circuit breaker pattern for fault tolerance
- Connection status monitoring
- Automatic reconnection attempts

## Logging

Structured logging is provided using Zap:

```go
// Log levels: debug, info, warn, error
export DB_LOG_LEVEL=debug
```

## Error Handling

The database manager provides comprehensive error handling:

```go
if err := dbManager.Connect(ctx); err != nil {
    // Handle connection errors
    log.Fatalf("Database connection failed: %v", err)
}

// Check if specific backend is connected
if dbManager.IsConnected(db.BackendGorm) {
    // PostgreSQL is available
}
```

## Best Practices

1. **Environment Variables**: Use environment variables for configuration in production
2. **Graceful Shutdown**: Always call `dbManager.Close()` on shutdown
3. **Context Usage**: Pass context to all database operations
4. **Error Handling**: Check for nil managers before use
5. **Connection Limits**: Set appropriate connection pool sizes

## Dependencies

The following dependencies are required:

```go
go get github.com/cenkalti/backoff/v4
go get github.com/sony/gobreaker
go get go.uber.org/zap
go get gorm.io/gorm
go get gorm.io/driver/postgres
go get github.com/aws/aws-sdk-go-v2
go get github.com/authzed/authzed-go/v1
```

## License

This package is part of the Kisanlink database library and follows the same license terms. 