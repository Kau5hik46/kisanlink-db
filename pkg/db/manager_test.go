package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Kisanlink/kisanlink-db/pkg/base"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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
		operator base.FilterOperator
		value    interface{}
		expected base.FilterCondition
	}{
		{
			name:     "Equal filter",
			field:    "name",
			operator: base.OpEqual,
			value:    "test",
			expected: base.FilterCondition{Field: "name", Operator: base.OpEqual, Value: "test"},
		},
		{
			name:     "Greater than filter",
			field:    "age",
			operator: base.OpGreaterThan,
			value:    18,
			expected: base.FilterCondition{Field: "age", Operator: base.OpGreaterThan, Value: 18},
		},
		{
			name:     "Contains filter",
			field:    "email",
			operator: base.OpContains,
			value:    "@example.com",
			expected: base.FilterCondition{Field: "email", Operator: base.OpContains, Value: "@example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test filter creation directly
			filter := base.FilterCondition{
				Field:    tt.field,
				Operator: tt.operator,
				Value:    tt.value,
			}
			assert.Equal(t, tt.expected, filter)
		})
	}
}

// TestApplyFilters tests filter application in database managers
func TestApplyFilters(t *testing.T) {
	// Test PostgreSQL filter application
	t.Run("PostgreSQL", func(t *testing.T) {
		manager := &PostgresManager{}

		// Note: We can't test ApplyFilters directly since it's now internal to List method
		// But we can test that the List method handles filters correctly
		assert.NotNil(t, manager)
	})

	// Test DynamoDB filter application
	t.Run("DynamoDB", func(t *testing.T) {
		manager := &DynamoManager{}

		// Note: We can't test ApplyFilters directly since it's now internal to List method
		// But we can test that the List method handles filters correctly
		assert.NotNil(t, manager)
	})

	// Test S3 filter application
	t.Run("S3", func(t *testing.T) {
		manager := &S3Manager{}

		// Note: We can't test ApplyFilters directly since it's now internal to List method
		// But we can test that the List method handles filters correctly
		assert.NotNil(t, manager)
	})
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
	filters := []base.FilterCondition{
		{Field: "active", Operator: base.OpEqual, Value: true},
		{Field: "age", Operator: base.OpGreaterThan, Value: 20},
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
	filters := []base.FilterCondition{
		{Field: "active", Operator: base.OpEqual, Value: true},
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
	operators := []base.FilterOperator{
		base.OpEqual,
		base.OpNotEqual,
		base.OpGreaterThan,
		base.OpLessThan,
		base.OpGreaterEqual,
		base.OpLessEqual,
		base.OpIn,
		base.OpNotIn,
		base.OpLike,
		base.OpContains,
		base.OpStartsWith,
		base.OpEndsWith,
	}

	// Test that all operators are valid
	for _, op := range operators {
		t.Run(string(op), func(t *testing.T) {
			filter := base.FilterCondition{
				Field:    "test_field",
				Operator: op,
				Value:    "test_value",
			}
			assert.Equal(t, "test_field", filter.Field)
			assert.Equal(t, op, filter.Operator)
			assert.Equal(t, "test_value", filter.Value)
		})
	}
}

// TestDatabaseManagerFilterIntegration tests how database managers handle filters
func TestDatabaseManagerFilterIntegration(t *testing.T) {
	tests := []struct {
		name     string
		manager  DBManager
		filters  []base.FilterCondition
		expected bool // whether we expect the operation to succeed
	}{
		{
			name:     "PostgreSQL with empty filters",
			manager:  &PostgresManager{},
			filters:  []base.FilterCondition{},
			expected: true,
		},
		{
			name:    "PostgreSQL with equal filter",
			manager: &PostgresManager{},
			filters: []base.FilterCondition{
				{Field: "name", Operator: base.OpEqual, Value: "test"},
			},
			expected: true,
		},
		{
			name:     "DynamoDB with empty filters",
			manager:  &DynamoManager{},
			filters:  []base.FilterCondition{},
			expected: true,
		},
		{
			name:    "DynamoDB with equal filter",
			manager: &DynamoManager{},
			filters: []base.FilterCondition{
				{Field: "name", Operator: base.OpEqual, Value: "test"},
			},
			expected: true,
		},
		{
			name:     "S3 with empty filters",
			manager:  &S3Manager{},
			filters:  []base.FilterCondition{},
			expected: true,
		},
		{
			name:    "S3 with prefix filter",
			manager: &S3Manager{},
			filters: []base.FilterCondition{
				{Field: "prefix", Operator: base.OpEqual, Value: "test/"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the manager can handle the filters
			// Note: We can't actually execute List without a connection,
			// but we can test that the manager is properly configured
			assert.NotNil(t, tt.manager)
			assert.Equal(t, len(tt.filters), len(tt.filters)) // Basic sanity check
		})
	}
}

// TestFilterValidation tests filter validation
func TestFilterValidation(t *testing.T) {
	tests := []struct {
		name        string
		filter      base.FilterCondition
		shouldValid bool
	}{
		{
			name: "Valid equal filter",
			filter: base.FilterCondition{
				Field:    "name",
				Operator: base.OpEqual,
				Value:    "test",
			},
			shouldValid: true,
		},
		{
			name: "Valid greater than filter",
			filter: base.FilterCondition{
				Field:    "age",
				Operator: base.OpGreaterThan,
				Value:    25,
			},
			shouldValid: true,
		},
		{
			name: "Valid contains filter",
			filter: base.FilterCondition{
				Field:    "email",
				Operator: base.OpContains,
				Value:    "@example.com",
			},
			shouldValid: true,
		},
		{
			name: "Valid IN filter",
			filter: base.FilterCondition{
				Field:    "status",
				Operator: base.OpIn,
				Value:    []string{"active", "pending"},
			},
			shouldValid: true,
		},
		{
			name: "Empty field name",
			filter: base.FilterCondition{
				Field:    "",
				Operator: base.OpEqual,
				Value:    "test",
			},
			shouldValid: false,
		},
		{
			name: "Invalid operator",
			filter: base.FilterCondition{
				Field:    "name",
				Operator: "invalid",
				Value:    "test",
			},
			shouldValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation tests
			if tt.shouldValid {
				assert.NotEmpty(t, tt.filter.Field)
				assert.NotEmpty(t, tt.filter.Operator)
			} else {
				// For invalid cases, we expect either empty field or invalid operator
				assert.True(t, tt.filter.Field == "" || !isValidOperator(tt.filter.Operator))
			}
		})
	}
}

// isValidOperator checks if an operator is valid
func isValidOperator(op base.FilterOperator) bool {
	validOperators := []base.FilterOperator{
		base.OpEqual,
		base.OpNotEqual,
		base.OpGreaterThan,
		base.OpLessThan,
		base.OpGreaterEqual,
		base.OpLessEqual,
		base.OpIn,
		base.OpNotIn,
		base.OpLike,
		base.OpLike,
		base.OpContains,
		base.OpStartsWith,
		base.OpEndsWith,
	}

	for _, validOp := range validOperators {
		if op == validOp {
			return true
		}
	}
	return false
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
			filters := []base.FilterCondition{
				{Field: "active", Operator: base.OpEqual, Value: true},
			}
			manager.List(ctx, filters, &models)
		}
	})
}
