# Database CRUD Operations and Filters

This document describes the common CRUD (Create, Read, Update, Delete) operations and filtering capabilities available across all database managers in the KisanLink database library.

## Overview

All database managers now implement the `DBManager` interface, which provides a unified API for database operations across different backends:

- **PostgreSQL/GORM** (`PostgresManager`)
- **DynamoDB** (`DynamoManager`) 
- **SpiceDB** (`SpiceManager`)

## DBManager Interface

```go
type DBManager interface {
    // Connection management
    Connect(ctx context.Context) error
    Close() error
    IsConnected() bool
    GetBackendType() BackendType
    
    // CRUD Operations
    Create(ctx context.Context, model interface{}) error
    GetByID(ctx context.Context, id interface{}, model interface{}) error
    Update(ctx context.Context, model interface{}) error
    Delete(ctx context.Context, id interface{}) error
    List(ctx context.Context, filters []Filter, model interface{}) error
    
    // Filter Operations
    ApplyFilters(query interface{}, filters []Filter) (interface{}, error)
    BuildFilter(field string, operator FilterOperator, value interface{}) Filter
}
```

## CRUD Operations

### 1. Create

Creates a new record in the database.

```go
user := &User{
    ID:     "user-1",
    Name:   "John Doe",
    Email:  "john@example.com",
    Age:    30,
    Active: true,
}

err := manager.Create(ctx, user)
```

### 2. GetByID

Retrieves a record by its ID.

```go
user := &User{}
err := manager.GetByID(ctx, "user-1", user)
```

### 3. Update

Updates an existing record.

```go
user.Name = "John Smith"
err := manager.Update(ctx, user)
```

### 4. Delete

Deletes a record by ID.

```go
err := manager.Delete(ctx, "user-1")
```

### 5. List

Retrieves multiple records with optional filters.

```go
var users []User
filters := []Filter{
    manager.BuildFilter("active", FilterOpEqual, true),
    manager.BuildFilter("age", FilterOpGreaterThan, 25),
}

err := manager.List(ctx, filters, &users)
```

## Filter System

The library provides a comprehensive filtering system that works across all database backends.

### Filter Structure

```go
type Filter struct {
    Field    string         `json:"field"`
    Operator FilterOperator `json:"operator"`
    Value    interface{}    `json:"value"`
}
```

### Available Filter Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `FilterOpEqual` | Equal to | `age = 25` |
| `FilterOpNotEqual` | Not equal to | `status != "inactive"` |
| `FilterOpGreaterThan` | Greater than | `age > 18` |
| `FilterOpLessThan` | Less than | `price < 100` |
| `FilterOpGreaterEqual` | Greater than or equal | `age >= 21` |
| `FilterOpLessEqual` | Less than or equal | `price <= 50` |
| `FilterOpIn` | In list | `status IN ("active", "pending")` |
| `FilterOpNotIn` | Not in list | `category NOT IN ("deleted", "archived")` |
| `FilterOpLike` | Pattern matching | `name LIKE "%john%"` |
| `FilterOpILike` | Case-insensitive pattern matching | `email ILIKE "%@gmail.com"` |
| `FilterOpContains` | Contains substring | `description CONTAINS "important"` |
| `FilterOpStartsWith` | Starts with | `name STARTS WITH "John"` |
| `FilterOpEndsWith` | Ends with | `email ENDS WITH ".com"` |

### Building Filters

```go
// Simple equality filter
equalFilter := manager.BuildFilter("status", FilterOpEqual, "active")

// Range filter
ageFilter := manager.BuildFilter("age", FilterOpGreaterThan, 18)
maxAgeFilter := manager.BuildFilter("age", FilterOpLessEqual, 65)

// Pattern matching
emailFilter := manager.BuildFilter("email", FilterOpContains, "@gmail.com")

// List membership
statusFilter := manager.BuildFilter("status", FilterOpIn, []string{"active", "pending"})
```

### Combining Filters

Filters are combined using AND logic when passed to the `List` method:

```go
filters := []Filter{
    manager.BuildFilter("active", FilterOpEqual, true),
    manager.BuildFilter("age", FilterOpGreaterThan, 18),
    manager.BuildFilter("email", FilterOpContains, "@example.com"),
}

var users []User
err := manager.List(ctx, filters, &users)
// This will find users who are active AND over 18 AND have @example.com in their email
```

## Backend-Specific Implementations

### PostgreSQL/GORM

The PostgreSQL manager uses GORM for database operations and supports all filter operators.

**Features:**
- Full SQL query support
- Transaction support
- Read replica support
- All filter operators supported

```go
// PostgreSQL-specific features
err := postgresManager.WithTransaction(ctx, func(tx *gorm.DB) error {
    // Transaction operations
    return nil
})

err := postgresManager.WithReadOnly(ctx, func(tx *gorm.DB) error {
    // Read-only operations using replica
    return nil
})
```

### DynamoDB

The DynamoDB manager uses the AWS SDK v2 and supports a subset of filter operators.

**Supported Operators:**
- `FilterOpEqual`
- `FilterOpNotEqual`
- `FilterOpGreaterThan`
- `FilterOpLessThan`
- `FilterOpGreaterEqual`
- `FilterOpLessEqual`
- `FilterOpContains`

**Features:**
- Scan operations with filter expressions
- Automatic marshaling/unmarshaling
- AWS SDK v2 integration

```go
// DynamoDB-specific features
client := dynamoManager.GetClient()
tableName := dynamoManager.GetTableName()
```

### SpiceDB

The SpiceDB manager uses the Authzed client and is designed for relationship-based queries.

**Supported Operators:**
- `FilterOpEqual` (for resource_type, relation, subject_type)

**Features:**
- Relationship-based queries
- Permission checking
- Schema management

```go
// SpiceDB-specific features
client := spiceManager.GetClient()
endpoint := spiceManager.GetEndpoint()
```

## Usage Examples

### Basic CRUD Operations

```go
package main

import (
    "context"
    "log"
    "time"
    
    "your-project/pkg/db"
)

type User struct {
    ID        string    `json:"id" gorm:"primaryKey"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    Age       int       `json:"age"`
    Active    bool      `json:"active"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

func main() {
    // Create database manager
    config := &db.Config{
        PrimaryBackend: db.BackendGorm,
        PostgresHost:   "localhost",
        PostgresPort:   "5432",
        PostgresUser:   "postgres",
        PostgresDBName: "example",
    }
    
    manager := db.NewDatabaseManagerWithConfig(config)
    ctx := context.Background()
    
    // Connect
    if err := manager.Connect(ctx); err != nil {
        log.Fatal(err)
    }
    defer manager.Close()
    
    // Get PostgreSQL manager
    postgresManager := manager.GetManager(db.BackendGorm)
    
    // Create user
    user := &User{
        ID:     "user-1",
        Name:   "John Doe",
        Email:  "john@example.com",
        Age:    30,
        Active: true,
    }
    
    if err := postgresManager.Create(ctx, user); err != nil {
        log.Printf("Failed to create user: %v", err)
        return
    }
    
    // Retrieve user
    retrievedUser := &User{}
    if err := postgresManager.GetByID(ctx, "user-1", retrievedUser); err != nil {
        log.Printf("Failed to retrieve user: %v", err)
        return
    }
    
    // Update user
    user.Name = "John Smith"
    if err := postgresManager.Update(ctx, user); err != nil {
        log.Printf("Failed to update user: %v", err)
        return
    }
    
    // List users with filters
    var users []User
    filters := []db.Filter{
        postgresManager.BuildFilter("active", db.FilterOpEqual, true),
        postgresManager.BuildFilter("age", db.FilterOpGreaterThan, 25),
    }
    
    if err := postgresManager.List(ctx, filters, &users); err != nil {
        log.Printf("Failed to list users: %v", err)
        return
    }
    
    // Delete user
    if err := postgresManager.Delete(ctx, "user-1"); err != nil {
        log.Printf("Failed to delete user: %v", err)
        return
    }
}
```

### Advanced Filtering

```go
// Complex filtering example
func advancedFiltering(manager db.DBManager, ctx context.Context) {
    // Date range filter
    startDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
    endDate := time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC)
    
    filters := []db.Filter{
        manager.BuildFilter("active", db.FilterOpEqual, true),
        manager.BuildFilter("age", db.FilterOpGreaterEqual, 18),
        manager.BuildFilter("age", db.FilterOpLessEqual, 65),
        manager.BuildFilter("email", db.FilterOpContains, "@example.com"),
        manager.BuildFilter("created_at", db.FilterOpGreaterEqual, startDate),
        manager.BuildFilter("created_at", db.FilterOpLessEqual, endDate),
        manager.BuildFilter("status", db.FilterOpIn, []string{"active", "pending"}),
    }
    
    var users []User
    if err := manager.List(ctx, filters, &users); err != nil {
        log.Printf("Failed to list users: %v", err)
        return
    }
    
    log.Printf("Found %d users matching criteria", len(users))
}
```

### Multi-Backend Operations

```go
func multiBackendOperations() {
    config := &db.Config{
        PrimaryBackend: db.BackendGorm,
        PostgresHost:   "localhost",
        PostgresPort:   "5432",
        PostgresUser:   "postgres",
        PostgresDBName: "example",
        DynamoDBRegion: "us-east-1",
        DynamoDBTable:  "users",
    }
    
    manager := db.NewDatabaseManagerWithConfig(config)
    ctx := context.Background()
    
    if err := manager.Connect(ctx); err != nil {
        log.Fatal(err)
    }
    defer manager.Close()
    
    // Get all available managers
    managers := manager.GetAllManagers()
    
    for _, dbManager := range managers {
        backendType := dbManager.GetBackendType()
        log.Printf("Operating on backend: %s", backendType)
        
        // Perform operations on each backend
        user := &User{
            ID:     fmt.Sprintf("user-%s", backendType),
            Name:   fmt.Sprintf("User from %s", backendType),
            Email:  fmt.Sprintf("user@%s.com", backendType),
            Age:    25,
            Active: true,
        }
        
        if err := dbManager.Create(ctx, user); err != nil {
            log.Printf("Failed to create user in %s: %v", backendType, err)
        }
    }
}
```

## Error Handling

The library provides comprehensive error handling for all operations:

```go
// Handle specific error types
if err := manager.Create(ctx, user); err != nil {
    if strings.Contains(err.Error(), "duplicate key") {
        log.Printf("User already exists")
    } else if strings.Contains(err.Error(), "connection") {
        log.Printf("Database connection error")
    } else {
        log.Printf("Unknown error: %v", err)
    }
    return
}

// Handle filter errors
filters := []db.Filter{
    {Field: "invalid_field", Operator: "invalid_operator", Value: "test"},
}

if err := manager.List(ctx, filters, &users); err != nil {
    if strings.Contains(err.Error(), "unsupported filter operator") {
        log.Printf("Invalid filter operator")
    } else {
        log.Printf("Filter error: %v", err)
    }
    return
}
```

## Performance Considerations

### PostgreSQL
- Use read replicas for read operations
- Leverage GORM's query optimization
- Use transactions for multiple operations

### DynamoDB
- Use scan operations sparingly (they scan the entire table)
- Consider using query operations with indexes for better performance
- Implement pagination for large result sets

### SpiceDB
- Use specific relationship filters to reduce query scope
- Cache frequently accessed relationships
- Use batch operations when possible

## Testing

The library includes comprehensive unit tests for all CRUD operations and filters:

```bash
# Run all tests
go test ./pkg/db/...

# Run specific test
go test ./pkg/db/ -run TestPostgresManagerCRUD

# Run benchmarks
go test ./pkg/db/ -bench=.

# Run integration tests (requires database connections)
go test ./pkg/db/ -tags=integration
```

## Migration Guide

If you're upgrading from a previous version:

1. **Interface Changes**: All managers now implement the `DBManager` interface
2. **New Methods**: CRUD operations are now available on all managers
3. **Filter System**: Use the new filter system instead of custom query building
4. **Error Handling**: Review error handling for new operation types

### Before (Old API)
```go
// Old way - backend-specific operations
db, err := postgresManager.GetDB(ctx, false)
if err != nil {
    return err
}
return db.Create(user).Error
```

### After (New API)
```go
// New way - unified interface
return postgresManager.Create(ctx, user)
```

## Contributing

When adding new database backends:

1. Implement the `DBManager` interface
2. Add CRUD operations
3. Implement filter support
4. Add comprehensive tests
5. Update documentation

## Support

For issues and questions:

1. Check the test files for usage examples
2. Review the example files in `example_crud.go`
3. Consult the backend-specific documentation
4. Open an issue with detailed error information 