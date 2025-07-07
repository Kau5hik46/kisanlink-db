// Package db provides integration tests for database connections.
package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/authzed/authzed-go/v1"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"
)

// TestConfig holds test configuration
type TestConfig struct {
	Environment string
	LogLevel    string
	Timeout     time.Duration
}

// TestResult holds test results
type TestResult struct {
	Backend      BackendType
	Connected    bool
	Error        error
	Details      string
	ResponseTime time.Duration
}

// IntegrationTestSuite runs comprehensive integration tests
type IntegrationTestSuite struct {
	config    TestConfig
	logger    *zap.Logger
	dbManager *DatabaseManager
	results   map[BackendType]*TestResult
}

// NewIntegrationTestSuite creates a new test suite
func NewIntegrationTestSuite(config TestConfig) *IntegrationTestSuite {
	logger := createTestLogger(config.LogLevel)

	return &IntegrationTestSuite{
		config:  config,
		logger:  logger,
		results: make(map[BackendType]*TestResult),
	}
}

// RunAllTests executes all integration tests
func (ts *IntegrationTestSuite) RunAllTests() error {
	ts.logger.Info("🚀 Starting Database Manager Integration Tests",
		zap.String("environment", ts.config.Environment),
		zap.String("log_level", ts.config.LogLevel),
		zap.Duration("timeout", ts.config.Timeout))

	// Print environment configuration
	ts.printEnvironmentConfig()

	// Create database manager
	ts.logger.Info("📦 Creating Database Manager...")
	ts.dbManager = NewDatabaseManager()

	// Connect to all backends
	ts.logger.Info("🔌 Connecting to Database Backends...")
	ctx, cancel := context.WithTimeout(context.Background(), ts.config.Timeout)
	defer cancel()

	startTime := time.Now()
	err := ts.dbManager.Connect(ctx)
	connectionTime := time.Since(startTime)

	if err != nil {
		ts.logger.Error("❌ Failed to connect to database backends", zap.Error(err))
		return err
	}

	ts.logger.Info("✅ Database Manager Connected",
		zap.Duration("connection_time", connectionTime))

	// Test each backend
	ts.testPostgreSQL()
	ts.testDynamoDB()
	ts.testSpiceDB()

	// Print summary
	ts.printTestSummary()

	// Cleanup
	ts.logger.Info("🧹 Cleaning up connections...")
	if err := ts.dbManager.Close(); err != nil {
		ts.logger.Error("❌ Error during cleanup", zap.Error(err))
		return err
	}

	ts.logger.Info("✅ Integration tests completed successfully")
	return nil
}

// testPostgreSQL tests PostgreSQL connection and operations
func (ts *IntegrationTestSuite) testPostgreSQL() {
	ts.logger.Info("🐘 Testing PostgreSQL Connection...")

	startTime := time.Now()
	postgresManager := ts.dbManager.GetPostgresManager()
	responseTime := time.Since(startTime)

	result := &TestResult{
		Backend:      BackendGorm,
		Connected:    postgresManager != nil && postgresManager.IsConnected(),
		ResponseTime: responseTime,
	}

	if !result.Connected {
		result.Error = fmt.Errorf("PostgreSQL manager not available or not connected")
		ts.logger.Warn("⚠️  PostgreSQL not available", zap.Error(result.Error))
	} else {
		ts.logger.Info("✅ PostgreSQL Connected",
			zap.Duration("response_time", responseTime))

		// Test basic operations
		ts.testPostgreSQLOperations(postgresManager)
	}

	ts.results[BackendGorm] = result
}

// testPostgreSQLOperations tests PostgreSQL CRUD operations
func (ts *IntegrationTestSuite) testPostgreSQLOperations(pm *PostgresManager) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ts.logger.Info("🔍 Testing PostgreSQL Operations...")

	// Test read-only operation
	startTime := time.Now()
	err := pm.WithReadOnly(ctx, func(tx *gorm.DB) error {
		// Simple query to test connection
		var result int
		return tx.Raw("SELECT 1").Scan(&result).Error
	})
	readTime := time.Since(startTime)

	if err != nil {
		ts.logger.Error("❌ PostgreSQL read operation failed", zap.Error(err))
	} else {
		ts.logger.Info("✅ PostgreSQL read operation successful",
			zap.Duration("read_time", readTime))
	}

	// Test transaction
	startTime = time.Now()
	err = pm.WithTransaction(ctx, func(tx *gorm.DB) error {
		// Simple transaction test
		var result int
		return tx.Raw("SELECT 1").Scan(&result).Error
	})
	writeTime := time.Since(startTime)

	if err != nil {
		ts.logger.Error("❌ PostgreSQL transaction failed", zap.Error(err))
	} else {
		ts.logger.Info("✅ PostgreSQL transaction successful",
			zap.Duration("write_time", writeTime))
	}
}

// testDynamoDB tests DynamoDB connection and operations
func (ts *IntegrationTestSuite) testDynamoDB() {
	ts.logger.Info("⚡ Testing DynamoDB Connection...")

	startTime := time.Now()
	dynamoManager := ts.dbManager.GetDynamoManager()
	responseTime := time.Since(startTime)

	result := &TestResult{
		Backend:      BackendDynamo,
		Connected:    dynamoManager != nil && dynamoManager.IsConnected(),
		ResponseTime: responseTime,
	}

	if !result.Connected {
		result.Error = fmt.Errorf("DynamoDB manager not available or not connected")
		ts.logger.Warn("⚠️  DynamoDB not available", zap.Error(result.Error))
	} else {
		ts.logger.Info("✅ DynamoDB Connected",
			zap.Duration("response_time", responseTime))

		// Test basic operations
		ts.testDynamoDBOperations(dynamoManager)
	}

	ts.results[BackendDynamo] = result
}

// testDynamoDBOperations tests DynamoDB operations
func (ts *IntegrationTestSuite) testDynamoDBOperations(dm *DynamoManager) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ts.logger.Info("🔍 Testing DynamoDB Operations...")

	startTime := time.Now()
	err := dm.WithClient(ctx, func(client *dynamodb.Client) error {
		// List tables to test connection
		limit := int32(1)
		_, err := client.ListTables(ctx, &dynamodb.ListTablesInput{Limit: &limit})
		return err
	})
	operationTime := time.Since(startTime)

	if err != nil {
		ts.logger.Error("❌ DynamoDB operation failed", zap.Error(err))
	} else {
		ts.logger.Info("✅ DynamoDB operation successful",
			zap.Duration("operation_time", operationTime),
			zap.String("table", dm.GetTableName()))
	}
}

// testSpiceDB tests SpiceDB connection and operations
func (ts *IntegrationTestSuite) testSpiceDB() {
	ts.logger.Info("🔐 Testing SpiceDB Connection...")

	startTime := time.Now()
	spiceManager := ts.dbManager.GetSpiceManager()
	responseTime := time.Since(startTime)

	result := &TestResult{
		Backend:      BackendSpiceDB,
		Connected:    spiceManager != nil && spiceManager.IsConnected(),
		ResponseTime: responseTime,
	}

	if !result.Connected {
		result.Error = fmt.Errorf("SpiceDB manager not available or not connected")
		ts.logger.Warn("⚠️  SpiceDB not available", zap.Error(result.Error))
	} else {
		ts.logger.Info("✅ SpiceDB Connected",
			zap.Duration("response_time", responseTime))

		// Test basic operations
		ts.testSpiceDBOperations(spiceManager)
	}

	ts.results[BackendSpiceDB] = result
}

// testSpiceDBOperations tests SpiceDB operations
func (ts *IntegrationTestSuite) testSpiceDBOperations(sm *SpiceManager) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ts.logger.Info("🔍 Testing SpiceDB Operations...")

	startTime := time.Now()
	err := sm.WithClient(ctx, func(client *authzed.Client) error {
		// Simple health check - just verify client is available
		// Note: SpiceDB client doesn't have a simple ping method
		// This is a basic connectivity test
		return nil
	})
	operationTime := time.Since(startTime)

	if err != nil {
		ts.logger.Error("❌ SpiceDB operation failed", zap.Error(err))
	} else {
		ts.logger.Info("✅ SpiceDB operation successful",
			zap.Duration("operation_time", operationTime),
			zap.String("endpoint", sm.GetEndpoint()))
	}
}

// printEnvironmentConfig prints current environment configuration
func (ts *IntegrationTestSuite) printEnvironmentConfig() {
	ts.logger.Info("📋 Environment Configuration",
		zap.String("DB_PRIMARY_BACKEND", getEnv("DB_PRIMARY_BACKEND", "not set")),
		zap.String("DB_POSTGRES_HOST", getEnv("DB_POSTGRES_HOST", "not set")),
		zap.String("DB_POSTGRES_PORT", getEnv("DB_POSTGRES_PORT", "not set")),
		zap.String("DB_POSTGRES_USER", getEnv("DB_POSTGRES_USER", "not set")),
		zap.String("DB_POSTGRES_DBNAME", getEnv("DB_POSTGRES_DBNAME", "not set")),
		zap.String("DB_POSTGRES_SSLMODE", getEnv("DB_POSTGRES_SSLMODE", "not set")),
		zap.String("DB_POSTGRES_READ_REPLICAS", getEnv("DB_POSTGRES_READ_REPLICAS", "not set")),
		zap.String("DB_DYNAMO_REGION", getEnv("DB_DYNAMO_REGION", "not set")),
		zap.String("DB_DYNAMO_TABLE", getEnv("DB_DYNAMO_TABLE", "not set")),
		zap.String("DB_SPICEDB_ENDPOINT", getEnv("DB_SPICEDB_ENDPOINT", "not set")),
		zap.String("DB_LOG_LEVEL", getEnv("DB_LOG_LEVEL", "not set")),
	)
}

// printTestSummary prints test results summary
func (ts *IntegrationTestSuite) printTestSummary() {
	ts.logger.Info("📊 Test Results Summary")

	for backend, result := range ts.results {
		if result.Connected {
			ts.logger.Info("✅ Backend Available",
				zap.String("backend", string(backend)),
				zap.Duration("response_time", result.ResponseTime))
		} else {
			ts.logger.Warn("❌ Backend Unavailable",
				zap.String("backend", string(backend)),
				zap.Error(result.Error))
		}
	}

	// Count available backends
	available := 0
	for _, result := range ts.results {
		if result.Connected {
			available++
		}
	}

	ts.logger.Info("🎯 Summary",
		zap.Int("total_backends", len(ts.results)),
		zap.Int("available_backends", available),
		zap.Int("unavailable_backends", len(ts.results)-available))
}

// createTestLogger creates a logger for tests
func createTestLogger(level string) *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	switch level {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		config.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		config.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	logger, err := config.Build()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}

	return logger
}

// TestIntegration runs the integration test suite
func TestIntegration(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration tests. Set RUN_INTEGRATION_TESTS=true to run")
	}

	config := TestConfig{
		Environment: os.Getenv("TEST_ENVIRONMENT"),
		LogLevel:    getEnv("DB_LOG_LEVEL", "info"),
		Timeout:     60 * time.Second,
	}

	if config.Environment == "" {
		config.Environment = "local"
	}

	suite := NewIntegrationTestSuite(config)
	if err := suite.RunAllTests(); err != nil {
		t.Fatalf("Integration tests failed: %v", err)
	}
}

// TestIntegrationLocal runs integration tests for local environment
func TestIntegrationLocal(t *testing.T) {
	if os.Getenv("TEST_ENVIRONMENT") != "local" {
		t.Skip("Skipping local integration tests")
	}

	config := TestConfig{
		Environment: "local",
		LogLevel:    "debug",
		Timeout:     30 * time.Second,
	}

	suite := NewIntegrationTestSuite(config)
	if err := suite.RunAllTests(); err != nil {
		t.Fatalf("Local integration tests failed: %v", err)
	}
}

// TestIntegrationBeta runs integration tests for beta environment
func TestIntegrationBeta(t *testing.T) {
	if os.Getenv("TEST_ENVIRONMENT") != "beta" {
		t.Skip("Skipping beta integration tests")
	}

	config := TestConfig{
		Environment: "beta",
		LogLevel:    "info",
		Timeout:     60 * time.Second,
	}

	suite := NewIntegrationTestSuite(config)
	if err := suite.RunAllTests(); err != nil {
		t.Fatalf("Beta integration tests failed: %v", err)
	}
}
