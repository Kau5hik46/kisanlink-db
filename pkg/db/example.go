// Package db provides database connection management for multiple backends.
// This file contains examples of how to use the database manager.
package db

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/authzed/authzed-go/v1"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"gorm.io/gorm"
)

// Example usage of the database manager in a microservice
func ExampleUsage() {
	// Method 1: Using environment variables (recommended for microservices)
	// Set these environment variables in your deployment:
	// DB_PRIMARY_BACKEND=gorm
	// DB_POSTGRES_HOST=localhost
	// DB_POSTGRES_PORT=5432
	// DB_POSTGRES_USER=postgres
	// DB_POSTGRES_PASSWORD=password
	// DB_POSTGRES_DBNAME=kisanlink
	// DB_POSTGRES_SSLMODE=disable

	dbManager := NewDatabaseManager()

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

// ExampleUsageWithConfig shows how to use custom configuration
func ExampleUsageWithConfig() {
	// Method 2: Using custom configuration
	config := &Config{
		PrimaryBackend:    BackendGorm,
		PostgresHost:      "localhost",
		PostgresPort:      "5432",
		PostgresUser:      "postgres",
		PostgresPassword:  "password",
		PostgresDBName:    "kisanlink",
		PostgresSSLMode:   "disable",
		PostgresMaxConns:  10,
		PostgresIdleConns: 5,
		LogLevel:          "info",
	}

	dbManager := NewDatabaseManagerWithConfig(config)

	ctx := context.Background()
	if err := dbManager.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbManager.Close()

	// Check connection status
	if dbManager.IsConnected(BackendGorm) {
		fmt.Println("PostgreSQL is connected")
	}
}

// ExampleEnvironmentSetup shows the environment variables needed
func ExampleEnvironmentSetup() {
	// PostgreSQL Configuration
	os.Setenv("DB_PRIMARY_BACKEND", "gorm")
	os.Setenv("DB_POSTGRES_HOST", "localhost")
	os.Setenv("DB_POSTGRES_PORT", "5432")
	os.Setenv("DB_POSTGRES_USER", "postgres")
	os.Setenv("DB_POSTGRES_PASSWORD", "password")
	os.Setenv("DB_POSTGRES_DBNAME", "kisanlink")
	os.Setenv("DB_POSTGRES_SSLMODE", "disable")
	os.Setenv("DB_POSTGRES_MAX_CONNS", "10")
	os.Setenv("DB_POSTGRES_IDLE_CONNS", "5")
	os.Setenv("DB_POSTGRES_READ_REPLICAS", "replica1:5432,replica2:5432")

	// DynamoDB Configuration
	os.Setenv("DB_DYNAMO_REGION", "us-east-1")
	os.Setenv("DB_DYNAMO_TABLE", "kisanlink-table")

	// SpiceDB Configuration
	os.Setenv("DB_SPICEDB_ENDPOINT", "localhost:50051")
	os.Setenv("DB_SPICEDB_TOKEN", "your-token")

	// Logging Configuration
	os.Setenv("DB_LOG_LEVEL", "info")
}

// ExampleMicroserviceIntegration shows how to integrate in a microservice
func ExampleMicroserviceIntegration() {
	// In your main.go or service initialization
	dbManager := NewDatabaseManager()

	ctx := context.Background()
	if err := dbManager.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Make dbManager available to your service handlers
	// For example, in a web service:
	// server := &Server{
	//     dbManager: dbManager,
	// }

	// Graceful shutdown
	defer func() {
		if err := dbManager.Close(); err != nil {
			log.Printf("Error closing database connections: %v", err)
		}
	}()
}
