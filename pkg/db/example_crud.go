package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Kisanlink/kisanlink-db/pkg/base"
)

// ExampleUser is a simple example model for CRUD operations
type ExampleUser struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Age       int       `json:"age"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ExampleCRUD demonstrates CRUD operations using the DBManager interface
func ExampleCRUD() {
	// Create a database manager
	config := &Config{
		PrimaryBackend: BackendGorm,
		PostgresHost:   "localhost",
		PostgresPort:   "5432",
		PostgresUser:   "postgres",
		PostgresDBName: "example",
		LogLevel:       "info",
	}

	manager := NewDatabaseManagerWithConfig(config)
	ctx := context.Background()

	// Connect to the database
	if err := manager.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer manager.Close()

	// Get the PostgreSQL manager
	postgresManager := manager.GetManager(BackendGorm)
	if postgresManager == nil {
		log.Fatal("PostgreSQL manager not available")
	}

	// Example 1: Create a new user
	user := &ExampleUser{
		ID:     "user-1",
		Name:   "John Doe",
		Email:  "john@example.com",
		Age:    30,
		Active: true,
	}

	if err := postgresManager.Create(ctx, user); err != nil {
		log.Printf("Failed to create user: %v", err)
	} else {
		fmt.Printf("Created user: %s\n", user.Name)
	}

	// Example 2: Retrieve user by ID
	retrievedUser := &ExampleUser{}
	if err := postgresManager.GetByID(ctx, "user-1", retrievedUser); err != nil {
		log.Printf("Failed to retrieve user: %v", err)
	} else {
		fmt.Printf("Retrieved user: %s (%s)\n", retrievedUser.Name, retrievedUser.Email)
	}

	// Example 3: Update user
	user.Name = "John Smith"
	if err := postgresManager.Update(ctx, user); err != nil {
		log.Printf("Failed to update user: %v", err)
	} else {
		fmt.Printf("Updated user: %s\n", user.Name)
	}

	// Example 4: List users with filters
	var users []ExampleUser
	listFilter := &base.Filter{
		Group: base.FilterGroup{
			Conditions: []base.FilterCondition{
				{Field: "active", Operator: base.OpEqual, Value: true},
				{Field: "age", Operator: base.OpGreaterThan, Value: 25},
			},
			Logic: base.LogicAnd,
		},
	}

	if err := postgresManager.List(ctx, listFilter, &users); err != nil {
		log.Printf("Failed to list users: %v", err)
	} else {
		fmt.Printf("Found %d active users over 25\n", len(users))
		for _, u := range users {
			fmt.Printf("  - %s (%s), Age: %d\n", u.Name, u.Email, u.Age)
		}
	}

	// Example 5: Delete user
	if err := postgresManager.Delete(ctx, "user-1"); err != nil {
		log.Printf("Failed to delete user: %v", err)
	} else {
		fmt.Printf("Deleted user: %s\n", user.Name)
	}
}

// ExampleDynamoDBCRUD demonstrates CRUD operations with DynamoDB
func ExampleDynamoDBCRUD() {
	config := &Config{
		PrimaryBackend: BackendDynamo,
		DynamoDBRegion: "us-east-1",
		DynamoDBTable:  "users",
		LogLevel:       "info",
	}

	manager := NewDatabaseManagerWithConfig(config)
	ctx := context.Background()

	// Connect to DynamoDB
	if err := manager.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to DynamoDB: %v", err)
	}
	defer manager.Close()

	dynamoManager := manager.GetManager(BackendDynamo)
	if dynamoManager == nil {
		log.Fatal("DynamoDB manager not available")
	}

	// Create a user
	user := &ExampleUser{
		ID:     "user-2",
		Name:   "Jane Doe",
		Email:  "jane@example.com",
		Age:    28,
		Active: true,
	}

	if err := dynamoManager.Create(ctx, user); err != nil {
		log.Printf("Failed to create user in DynamoDB: %v", err)
	} else {
		fmt.Printf("Created user in DynamoDB: %s\n", user.Name)
	}

	// List users with filters
	var users []ExampleUser
	dynamoFilter := &base.Filter{
		Group: base.FilterGroup{
			Conditions: []base.FilterCondition{
				{Field: "active", Operator: base.OpEqual, Value: true},
			},
			Logic: base.LogicAnd,
		},
	}

	if err := dynamoManager.List(ctx, dynamoFilter, &users); err != nil {
		log.Printf("Failed to list users from DynamoDB: %v", err)
	} else {
		fmt.Printf("Found %d active users in DynamoDB\n", len(users))
	}
}

// ExampleFilterOperations demonstrates various filter operations
func ExampleFilterOperations() {
	config := &Config{
		PrimaryBackend: BackendGorm,
		PostgresHost:   "localhost",
		PostgresPort:   "5432",
		PostgresUser:   "postgres",
		PostgresDBName: "example",
		LogLevel:       "info",
	}

	manager := NewDatabaseManagerWithConfig(config)
	ctx := context.Background()

	if err := manager.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer manager.Close()

	postgresManager := manager.GetManager(BackendGorm)
	if postgresManager == nil {
		log.Fatal("PostgreSQL manager not available")
	}

	// Create a filter with multiple conditions and pagination
	filter := &base.Filter{
		Group: base.FilterGroup{
			Conditions: []base.FilterCondition{
				{Field: "email", Operator: base.OpEqual, Value: "john@example.com"},
				{Field: "age", Operator: base.OpGreaterThan, Value: 25},
				{Field: "name", Operator: base.OpContains, Value: "John"},
				{Field: "status", Operator: base.OpIn, Value: []string{"active", "pending"}},
				{Field: "email", Operator: base.OpLike, Value: "%@example.com"},
				{Field: "created_at", Operator: base.OpGreaterEqual, Value: time.Now().AddDate(0, -1, 0)},
				{Field: "created_at", Operator: base.OpLessEqual, Value: time.Now()},
			},
			Logic: base.LogicAnd,
		},
		Sort: []base.SortField{
			{Field: "created_at", Direction: "desc"},
		},
		Limit:  10,
		Offset: 0,
	}

	var users []ExampleUser
	if err := postgresManager.List(ctx, filter, &users); err != nil {
		log.Printf("Failed to list users with filters: %v", err)
	} else {
		fmt.Printf("Found %d users matching all filters\n", len(users))
	}
}

// ExampleMultiBackend demonstrates using multiple backends
func ExampleMultiBackend() {
	config := &Config{
		PrimaryBackend: BackendGorm,
		PostgresHost:   "localhost",
		PostgresPort:   "5432",
		PostgresUser:   "postgres",
		PostgresDBName: "example",
		DynamoDBRegion: "us-east-1",
		DynamoDBTable:  "users",
		LogLevel:       "info",
	}

	manager := NewDatabaseManagerWithConfig(config)
	ctx := context.Background()

	// Connect to all configured backends
	if err := manager.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer manager.Close()

	// Get all available managers
	managers := manager.GetAllManagers()
	fmt.Printf("Connected to %d database backends\n", len(managers))

	for _, dbManager := range managers {
		backendType := dbManager.GetBackendType()
		fmt.Printf("Backend: %s, Connected: %t\n", backendType, dbManager.IsConnected())

		// Perform operations on each backend
		user := &ExampleUser{
			ID:     fmt.Sprintf("user-%s", backendType),
			Name:   fmt.Sprintf("User from %s", backendType),
			Email:  fmt.Sprintf("user@%s.com", backendType),
			Age:    25,
			Active: true,
		}

		if err := dbManager.Create(ctx, user); err != nil {
			log.Printf("Failed to create user in %s: %v", backendType, err)
		} else {
			fmt.Printf("Created user in %s: %s\n", backendType, user.Name)
		}
	}
}

// ExampleErrorHandling demonstrates proper error handling
func ExampleErrorHandling() {
	config := &Config{
		PrimaryBackend: BackendGorm,
		PostgresHost:   "localhost",
		PostgresPort:   "5432",
		PostgresUser:   "postgres",
		PostgresDBName: "example",
		LogLevel:       "info",
	}

	manager := NewDatabaseManagerWithConfig(config)
	ctx := context.Background()

	// Connect with error handling
	if err := manager.Connect(ctx); err != nil {
		log.Printf("Connection failed: %v", err)
		return
	}
	defer manager.Close()

	postgresManager := manager.GetManager(BackendGorm)
	if postgresManager == nil {
		log.Fatal("PostgreSQL manager not available")
	}

	// Example: Handle specific error types
	user := &ExampleUser{
		ID:     "user-3",
		Name:   "Error Test User",
		Email:  "error@example.com",
		Age:    30,
		Active: true,
	}

	// Create user
	if err := postgresManager.Create(ctx, user); err != nil {
		log.Printf("Create failed: %v", err)
		return
	}

	// Try to create duplicate (should fail)
	if err := postgresManager.Create(ctx, user); err != nil {
		log.Printf("Expected error for duplicate: %v", err)
	}

	// Try to get non-existent user
	nonExistentUser := &ExampleUser{}
	if err := postgresManager.GetByID(ctx, "non-existent", nonExistentUser); err != nil {
		log.Printf("Expected error for non-existent user: %v", err)
	}

	// Try to apply invalid filters
	var users []ExampleUser
	invalidFilter := &base.Filter{
		Group: base.FilterGroup{
			Conditions: []base.FilterCondition{
				{Field: "invalid_field", Operator: "invalid_operator", Value: "test"},
			},
			Logic: base.LogicAnd,
		},
	}

	if err := postgresManager.List(ctx, invalidFilter, &users); err != nil {
		log.Printf("Expected error for invalid filters: %v", err)
	}
}

// ExampleBaseFilterableRepository demonstrates using the base filterable repository
func ExampleBaseFilterableRepository() {
	fmt.Println("=== Base Filterable Repository Examples ===")

	// Example 1: Creating a filter using the base filter builder
	filter := base.NewFilterBuilder().
		Where("name", base.OpEqual, "John Doe").
		Where("age", base.OpGreaterThan, 25).
		Where("email", base.OpContains, "@example.com").
		Sort("created_at", "desc").
		Page(1, 10).
		Build()

	fmt.Printf("Created complex filter with %d conditions\n", len(filter.Group.Conditions))

	// Example 2: Using OR conditions
	orFilter := base.NewFilterBuilder().
		Where("status", base.OpEqual, "active").
		Or(
			base.FilterCondition{Field: "role", Operator: base.OpEqual, Value: "admin"},
			base.FilterCondition{Field: "role", Operator: base.OpEqual, Value: "moderator"},
		).
		Build()

	fmt.Printf("Created OR filter with %d groups\n", len(orFilter.Group.Groups))

	// Example 3: Date range filtering
	dateFilter := base.NewFilterBuilder().
		WhereBetween("created_at", time.Now().AddDate(0, -1, 0), time.Now()).
		Where("is_deleted", base.OpEqual, false).
		Build()

	fmt.Printf("Created date range filter with %d conditions\n", len(dateFilter.Group.Conditions))

	// Example 4: IN clause filtering
	inFilter := base.NewFilterBuilder().
		WhereIn("category", []interface{}{"electronics", "books", "clothing"}).
		Where("price", base.OpGreaterEqual, 10.0).
		Build()

	fmt.Printf("Created IN filter with %d conditions\n", len(inFilter.Group.Conditions))

	// Example 5: Text search with multiple conditions
	searchFilter := base.NewFilterBuilder().
		Where("title", base.OpContains, "search term").
		Or(
			base.FilterCondition{Field: "description", Operator: base.OpContains, Value: "search term"},
			base.FilterCondition{Field: "tags", Operator: base.OpContains, Value: "search term"},
		).
		Sort("relevance", "desc").
		Sort("created_at", "desc").
		Page(1, 20).
		Build()

	fmt.Printf("Created search filter with %d conditions and %d sort fields\n",
		len(searchFilter.Group.Conditions), len(searchFilter.Sort))
}
