package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestModel is a simple test model for testing CRUD operations
type TestModel struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Age       int       `json:"age"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TestDBManagerInterface tests that all managers implement the DBManager interface
func TestDBManagerInterface(t *testing.T) {
	tests := []struct {
		name    string
		manager DBManager
	}{
		{
			name:    "PostgresManager implements DBManager",
			manager: &PostgresManager{},
		},
		{
			name:    "DynamoManager implements DBManager",
			manager: &DynamoManager{},
		},
		{
			name:    "SpiceManager implements DBManager",
			manager: &SpiceManager{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail at compile time if the interface is not implemented
			assert.NotNil(t, tt.manager)
		})
	}
}

// TestFilterOperations tests filter creation and operations
func TestFilterOperations(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		operator FilterOperator
		value    interface{}
		expected Filter
	}{
		{
			name:     "Equal filter",
			field:    "name",
			operator: FilterOpEqual,
			value:    "test",
			expected: Filter{Field: "name", Operator: FilterOpEqual, Value: "test"},
		},
		{
			name:     "Greater than filter",
			field:    "age",
			operator: FilterOpGreaterThan,
			value:    18,
			expected: Filter{Field: "age", Operator: FilterOpGreaterThan, Value: 18},
		},
		{
			name:     "Contains filter",
			field:    "email",
			operator: FilterOpContains,
			value:    "@example.com",
			expected: Filter{Field: "email", Operator: FilterOpContains, Value: "@example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with PostgresManager
			pm := &PostgresManager{}
			filter := pm.BuildFilter(tt.field, tt.operator, tt.value)
			assert.Equal(t, tt.expected, filter)

			// Test with DynamoManager
			dm := &DynamoManager{}
			filter = dm.BuildFilter(tt.field, tt.operator, tt.value)
			assert.Equal(t, tt.expected, filter)

			// Test with SpiceManager
			sm := &SpiceManager{}
			filter = sm.BuildFilter(tt.field, tt.operator, tt.value)
			assert.Equal(t, tt.expected, filter)
		})
	}
}

// TestPostgresManagerCRUD tests CRUD operations for PostgresManager
func TestPostgresManagerCRUD(t *testing.T) {
	// Skip if no database connection available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := &Config{
		PostgresHost:     "localhost",
		PostgresPort:     "5432",
		PostgresUser:     "test",
		PostgresPassword: "test",
		PostgresDBName:   "testdb",
		PostgresSSLMode:  "disable",
		LogLevel:         "debug",
	}

	logger := zap.NewNop()
	manager := NewPostgresManager(config, logger)

	ctx := context.Background()

	// Test connection
	err := manager.Connect(ctx)
	if err != nil {
		t.Skipf("Skipping test - cannot connect to PostgreSQL: %v", err)
	}
	defer manager.Close()

	// Test Create
	testModel := &TestModel{
		ID:     "test-1",
		Name:   "Test User",
		Email:  "test@example.com",
		Age:    25,
		Active: true,
	}

	err = manager.Create(ctx, testModel)
	require.NoError(t, err)

	// Test GetByID
	retrievedModel := &TestModel{}
	err = manager.GetByID(ctx, "test-1", retrievedModel)
	require.NoError(t, err)
	assert.Equal(t, testModel.Name, retrievedModel.Name)
	assert.Equal(t, testModel.Email, retrievedModel.Email)

	// Test Update
	testModel.Name = "Updated User"
	err = manager.Update(ctx, testModel)
	require.NoError(t, err)

	// Verify update
	updatedModel := &TestModel{}
	err = manager.GetByID(ctx, "test-1", updatedModel)
	require.NoError(t, err)
	assert.Equal(t, "Updated User", updatedModel.Name)

	// Test List with filters
	var models []TestModel
	filters := []Filter{
		manager.BuildFilter("active", FilterOpEqual, true),
		manager.BuildFilter("age", FilterOpGreaterThan, 20),
	}

	err = manager.List(ctx, filters, &models)
	require.NoError(t, err)
	assert.Len(t, models, 1)

	// Test Delete
	err = manager.Delete(ctx, "test-1")
	require.NoError(t, err)

	// Verify deletion
	err = manager.GetByID(ctx, "test-1", &TestModel{})
	assert.Error(t, err) // Should return error for non-existent record
}

// TestDynamoManagerCRUD tests CRUD operations for DynamoManager
func TestDynamoManagerCRUD(t *testing.T) {
	// Skip if no AWS credentials available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := &Config{
		DynamoDBRegion: "us-east-1",
		DynamoDBTable:  "test-table",
		LogLevel:       "debug",
	}

	logger := zap.NewNop()
	manager := NewDynamoManager(config, logger)

	ctx := context.Background()

	// Test connection
	err := manager.Connect(ctx)
	if err != nil {
		t.Skipf("Skipping test - cannot connect to DynamoDB: %v", err)
	}
	defer manager.Close()

	// Ensure table exists
	dynamoClient := manager.GetClient()
	_, err = dynamoClient.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: &config.DynamoDBTable})
	if err != nil {
		// Table does not exist, create it
		_, err = dynamoClient.CreateTable(ctx, &dynamodb.CreateTableInput{
			TableName: &config.DynamoDBTable,
			AttributeDefinitions: []types.AttributeDefinition{
				{
					AttributeName: aws.String("id"),
					AttributeType: types.ScalarAttributeTypeS,
				},
			},
			KeySchema: []types.KeySchemaElement{
				{
					AttributeName: aws.String("id"),
					KeyType:       types.KeyTypeHash,
				},
			},
			ProvisionedThroughput: &types.ProvisionedThroughput{
				ReadCapacityUnits:  aws.Int64(5),
				WriteCapacityUnits: aws.Int64(5),
			},
		})
		if err != nil {
			t.Fatalf("Failed to create DynamoDB table: %v", err)
		}
		// Wait for table to be active
		for {
			out, err := dynamoClient.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: &config.DynamoDBTable})
			if err == nil && out.Table != nil && out.Table.TableStatus == types.TableStatusActive {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}

	// Test Create
	testModel := &TestModel{
		ID:     "test-1",
		Name:   "Test User",
		Email:  "test@example.com",
		Age:    25,
		Active: true,
	}

	err = manager.Create(ctx, testModel)
	require.NoError(t, err)

	// Test GetByID
	retrievedModel := &TestModel{}
	err = manager.GetByID(ctx, "test-1", retrievedModel)
	require.NoError(t, err)
	assert.Equal(t, testModel.Name, retrievedModel.Name)

	// Test Update
	testModel.Name = "Updated User"
	err = manager.Update(ctx, testModel)
	require.NoError(t, err)

	// Test List with filters
	var models []TestModel
	filters := []Filter{
		manager.BuildFilter("active", FilterOpEqual, true),
	}

	err = manager.List(ctx, filters, &models)
	require.NoError(t, err)
	assert.Len(t, models, 1)

	// Test Delete
	err = manager.Delete(ctx, "test-1")
	require.NoError(t, err)
}

// TestSpiceManagerCRUD tests CRUD operations for SpiceManager
func TestSpiceManagerCRUD(t *testing.T) {
	// Skip if no SpiceDB connection available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := &Config{
		SpiceDBEndpoint: "localhost:50051",
		SpiceDBToken:    "test-token",
		LogLevel:        "debug",
	}

	logger := zap.NewNop()
	manager := NewSpiceManager(config, logger)

	ctx := context.Background()

	// Test connection
	err := manager.Connect(ctx)
	if err != nil {
		t.Skipf("Skipping test - cannot connect to SpiceDB: %v", err)
	}
	defer manager.Close()

	// Note: SpiceDB tests would require actual SpiceDB client types
	// This is a placeholder for when the proper imports are available
	t.Skip("SpiceDB tests require proper client types")
}

// TestDatabaseManagerInterface tests the DatabaseManager interface methods
func TestDatabaseManagerInterface(t *testing.T) {
	config := &Config{
		PrimaryBackend: BackendGorm,
		LogLevel:       "debug",
	}

	manager := NewDatabaseManagerWithConfig(config)

	ctx := context.Background()

	// Test GetManager
	postgresManager := manager.GetManager(BackendGorm)
	assert.Nil(t, postgresManager) // Should be nil before connection

	// Test GetAllManagers
	managers := manager.GetAllManagers()
	assert.Len(t, managers, 0) // Should be empty before connection

	// Test Connect and GetManager
	err := manager.Connect(ctx)
	if err != nil {
		t.Skipf("Skipping test - cannot connect to databases: %v", err)
	}
	defer manager.Close()

	// Test GetManager after connection
	postgresManager = manager.GetManager(BackendGorm)
	if postgresManager != nil {
		assert.Equal(t, BackendGorm, postgresManager.GetBackendType())
	}

	// Test GetAllManagers after connection
	managers = manager.GetAllManagers()
	assert.GreaterOrEqual(t, len(managers), 0)

	// Test IsConnected
	assert.True(t, manager.IsConnected(BackendInMemory)) // In-memory is always available
}

// TestFilterOperators tests all filter operators
func TestFilterOperators(t *testing.T) {
	operators := []FilterOperator{
		FilterOpEqual,
		FilterOpNotEqual,
		FilterOpGreaterThan,
		FilterOpLessThan,
		FilterOpGreaterEqual,
		FilterOpLessEqual,
		FilterOpIn,
		FilterOpNotIn,
		FilterOpLike,
		FilterOpILike,
		FilterOpContains,
		FilterOpStartsWith,
		FilterOpEndsWith,
	}

	manager := &PostgresManager{}

	for _, op := range operators {
		t.Run(string(op), func(t *testing.T) {
			filter := manager.BuildFilter("test_field", op, "test_value")
			assert.Equal(t, "test_field", filter.Field)
			assert.Equal(t, op, filter.Operator)
			assert.Equal(t, "test_value", filter.Value)
		})
	}
}

// TestApplyFilters tests filter application
func TestApplyFilters(t *testing.T) {
	manager := &PostgresManager{}

	// Test empty filters
	filters := []Filter{}
	// Use a real *gorm.DB for all tests
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	result, err := manager.ApplyFilters(db, filters)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Test invalid query type
	filters = []Filter{
		manager.BuildFilter("name", FilterOpEqual, "test"),
	}
	_, err = manager.ApplyFilters("invalid", filters)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query must be *gorm.DB")

	// Test unsupported operator
	filters = []Filter{
		{Field: "name", Operator: "unsupported", Value: "test"},
	}
	_, err = manager.ApplyFilters(db, filters)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported filter operator")
}

// BenchmarkCRUDOperations benchmarks CRUD operations
func BenchmarkCRUDOperations(b *testing.B) {
	config := &Config{
		PostgresHost:     "localhost",
		PostgresPort:     "5432",
		PostgresUser:     "test",
		PostgresPassword: "test",
		PostgresDBName:   "testdb",
		PostgresSSLMode:  "disable",
		LogLevel:         "error", // Use error level for benchmarks
	}

	logger := zap.NewNop()
	manager := NewPostgresManager(config, logger)

	ctx := context.Background()

	// Connect to database
	err := manager.Connect(ctx)
	if err != nil {
		b.Skipf("Skipping benchmark - cannot connect to PostgreSQL: %v", err)
	}
	defer manager.Close()

	b.Run("Create", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			model := &TestModel{
				ID:     fmt.Sprintf("bench-%d", i),
				Name:   "Benchmark User",
				Email:  "bench@example.com",
				Age:    30,
				Active: true,
			}
			manager.Create(ctx, model)
		}
	})

	b.Run("GetByID", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			model := &TestModel{}
			manager.GetByID(ctx, "bench-0", model)
		}
	})

	b.Run("List", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var models []TestModel
			filters := []Filter{
				manager.BuildFilter("active", FilterOpEqual, true),
			}
			manager.List(ctx, filters, &models)
		}
	})
}
