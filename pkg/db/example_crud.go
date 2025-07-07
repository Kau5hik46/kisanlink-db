package db

import (
	"context"
	"fmt"
	"log"
	"time"
)

// ExampleUser represents a user model for demonstration
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
	filters := []Filter{
		postgresManager.BuildFilter("active", FilterOpEqual, true),
		postgresManager.BuildFilter("age", FilterOpGreaterThan, 25),
	}

	if err := postgresManager.List(ctx, filters, &users); err != nil {
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
	filters := []Filter{
		dynamoManager.BuildFilter("active", FilterOpEqual, true),
	}

	if err := dynamoManager.List(ctx, filters, &users); err != nil {
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

	// Example 1: Equal filter
	equalFilter := postgresManager.BuildFilter("email", FilterOpEqual, "john@example.com")

	// Example 2: Greater than filter
	ageFilter := postgresManager.BuildFilter("age", FilterOpGreaterThan, 25)

	// Example 3: Contains filter
	containsFilter := postgresManager.BuildFilter("name", FilterOpContains, "John")

	// Example 4: In filter
	inFilter := postgresManager.BuildFilter("status", FilterOpIn, []string{"active", "pending"})

	// Example 5: Like filter
	likeFilter := postgresManager.BuildFilter("email", FilterOpLike, "%@example.com")

	// Example 6: Between filter (using GreaterEqual and LessEqual)
	startDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC)
	startFilter := postgresManager.BuildFilter("created_at", FilterOpGreaterEqual, startDate)
	endFilter := postgresManager.BuildFilter("created_at", FilterOpLessEqual, endDate)

	// Combine filters
	filters := []Filter{
		equalFilter,
		ageFilter,
		containsFilter,
		inFilter,
		likeFilter,
		startFilter,
		endFilter,
	}

	var users []ExampleUser
	if err := postgresManager.List(ctx, filters, &users); err != nil {
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
	invalidFilters := []Filter{
		{Field: "invalid_field", Operator: "invalid_operator", Value: "test"},
	}

	if err := postgresManager.List(ctx, invalidFilters, &users); err != nil {
		log.Printf("Expected error for invalid filters: %v", err)
	}
}
