// Package db provides DynamoDB database connection management.
package db

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kisanlink/kisanlink-db/pkg/base"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
)

// DynamoManager manages DynamoDB database connections
type DynamoManager struct {
	config     *Config
	client     *dynamodb.Client
	circuit    *gobreaker.CircuitBreaker
	logger     *zap.Logger
	mu         sync.RWMutex
	healthChan chan bool
}

// NewDynamoManager creates a new DynamoDB manager
func NewDynamoManager(config *Config, logger *zap.Logger) *DynamoManager {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "dynamodb",
		MaxRequests: 5,
		Interval:    0,
		Timeout:     5 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Info("dynamodb circuit breaker state changed",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	})

	return &DynamoManager{
		config:     config,
		circuit:    cb,
		logger:     logger,
		healthChan: make(chan bool, 1),
	}
}

// Connect establishes connection to DynamoDB
func (dm *DynamoManager) Connect(ctx context.Context) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.client != nil {
		return nil // Already connected
	}

	cfg, err := dm.loadAWSConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	dm.client = dynamodb.NewFromConfig(cfg)

	// Verify connection by listing tables
	_, err = dm.client.ListTables(ctx, &dynamodb.ListTablesInput{})
	if err != nil {
		return fmt.Errorf("failed to verify DynamoDB connection: %w", err)
	}

	go dm.healthCheck(ctx)

	return nil
}

// loadAWSConfig loads AWS configuration for DynamoDB
func (dm *DynamoManager) loadAWSConfig(ctx context.Context) (aws.Config, error) {
	if dm.config.DynamoDBRegion == "" {
		dm.config.DynamoDBRegion = "us-east-1" // Default region
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(dm.config.DynamoDBRegion))
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return cfg, nil
}

// healthCheck periodically checks the health of DynamoDB
func (dm *DynamoManager) healthCheck(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			healthy := dm.checkHealth(ctx)
			select {
			case dm.healthChan <- healthy:
			default:
			}
		}
	}
}

// checkHealth verifies DynamoDB connectivity
func (dm *DynamoManager) checkHealth(ctx context.Context) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if dm.client == nil {
		return false
	}

	// Simple health check by listing tables
	_, err := dm.client.ListTables(ctx, &dynamodb.ListTablesInput{Limit: aws.Int32(1)})
	return err == nil
}

// GetClient returns the DynamoDB client
func (dm *DynamoManager) GetClient() *dynamodb.Client {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.client
}

// Close closes the DynamoDB connection
func (dm *DynamoManager) Close() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// DynamoDB client doesn't need explicit closing
	dm.client = nil
	return nil
}

// WithClient executes a function with the DynamoDB client
func (dm *DynamoManager) WithClient(ctx context.Context, fn func(client *dynamodb.Client) error) error {
	client := dm.GetClient()
	if client == nil {
		return fmt.Errorf("dynamodb client not connected")
	}

	return fn(client)
}

// IsConnected checks if DynamoDB is connected
func (dm *DynamoManager) IsConnected() bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.client != nil
}

// GetTableName returns the configured DynamoDB table name
func (dm *DynamoManager) GetTableName() string {
	return dm.config.DynamoDBTable
}

// GetBackendType returns the type of backend this manager handles
func (dm *DynamoManager) GetBackendType() BackendType {
	return BackendDynamo
}

// Create creates a new record in DynamoDB
func (dm *DynamoManager) Create(ctx context.Context, model interface{}) error {
	client := dm.GetClient()
	if client == nil {
		return fmt.Errorf("dynamodb client not connected")
	}

	// Convert model to DynamoDB item
	item, err := dm.marshalToItem(model)
	if err != nil {
		return fmt.Errorf("failed to marshal model: %w", err)
	}

	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(dm.config.DynamoDBTable),
		Item:      item,
	})

	return err
}

// GetByID retrieves a record by ID from DynamoDB
func (dm *DynamoManager) GetByID(ctx context.Context, id interface{}, model interface{}) error {
	client := dm.GetClient()
	if client == nil {
		return fmt.Errorf("dynamodb client not connected")
	}

	key := map[string]types.AttributeValue{
		"id": &types.AttributeValueMemberS{Value: fmt.Sprint(id)},
	}

	result, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(dm.config.DynamoDBTable),
		Key:       key,
	})
	if err != nil {
		return err
	}

	if result.Item == nil {
		return fmt.Errorf("item not found")
	}

	// Convert DynamoDB item to model
	return dm.unmarshalFromItem(result.Item, model)
}

// Update updates an existing record in DynamoDB
func (dm *DynamoManager) Update(ctx context.Context, model interface{}) error {
	// For DynamoDB, update is similar to create (PutItem)
	return dm.Create(ctx, model)
}

// Delete deletes a record by ID from DynamoDB
func (dm *DynamoManager) Delete(ctx context.Context, id interface{}) error {
	client := dm.GetClient()
	if client == nil {
		return fmt.Errorf("dynamodb client not connected")
	}

	key := map[string]types.AttributeValue{
		"id": &types.AttributeValueMemberS{Value: fmt.Sprint(id)},
	}

	_, err := client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(dm.config.DynamoDBTable),
		Key:       key,
	})

	return err
}

// List retrieves records from DynamoDB with basic filtering
func (dm *DynamoManager) List(ctx context.Context, filters []base.FilterCondition, model interface{}) error {
	client := dm.GetClient()
	if client == nil {
		return fmt.Errorf("dynamodb client not connected")
	}

	// For DynamoDB, we'll do a scan with filter expressions
	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(dm.config.DynamoDBTable),
	}

	// Apply basic filters if provided
	if len(filters) > 0 {
		var filterExpressions []string
		var expressionAttributeNames map[string]string
		var expressionAttributeValues map[string]types.AttributeValue

		for i, filter := range filters {
			fieldName := fmt.Sprintf("#field%d", i)
			valueName := fmt.Sprintf(":value%d", i)

			if expressionAttributeNames == nil {
				expressionAttributeNames = make(map[string]string)
			}
			if expressionAttributeValues == nil {
				expressionAttributeValues = make(map[string]types.AttributeValue)
			}

			expressionAttributeNames[fieldName] = filter.Field

			// Convert value to DynamoDB attribute value
			attrValue, err := dm.valueToAttributeValue(filter.Value)
			if err != nil {
				return fmt.Errorf("failed to convert value: %w", err)
			}
			expressionAttributeValues[valueName] = attrValue

			var expression string
			switch filter.Operator {
			case base.OpEqual:
				expression = fmt.Sprintf("%s = %s", fieldName, valueName)
			case base.OpNotEqual:
				expression = fmt.Sprintf("%s <> %s", fieldName, valueName)
			case base.OpGreaterThan:
				expression = fmt.Sprintf("%s > %s", fieldName, valueName)
			case base.OpLessThan:
				expression = fmt.Sprintf("%s < %s", fieldName, valueName)
			case base.OpGreaterEqual:
				expression = fmt.Sprintf("%s >= %s", fieldName, valueName)
			case base.OpLessEqual:
				expression = fmt.Sprintf("%s <= %s", fieldName, valueName)
			case base.OpContains:
				expression = fmt.Sprintf("contains(%s, %s)", fieldName, valueName)
			default:
				return fmt.Errorf("unsupported filter operator for DynamoDB: %s", filter.Operator)
			}

			filterExpressions = append(filterExpressions, expression)
		}

		if len(filterExpressions) > 0 {
			scanInput.FilterExpression = aws.String(strings.Join(filterExpressions, " AND "))
			scanInput.ExpressionAttributeNames = expressionAttributeNames
			scanInput.ExpressionAttributeValues = expressionAttributeValues
		}
	}

	result, err := client.Scan(ctx, scanInput)
	if err != nil {
		return err
	}

	// Convert results to slice
	sliceValue := reflect.ValueOf(model).Elem()
	for _, item := range result.Items {
		itemModel := reflect.New(sliceValue.Type().Elem()).Interface()
		if err := dm.unmarshalFromItem(item, itemModel); err != nil {
			return err
		}
		sliceValue.Set(reflect.Append(sliceValue, reflect.ValueOf(itemModel).Elem()))
	}

	return nil
}

// AutoMigrateModels runs automigration for specific models (DynamoDB doesn't support schema migration)
func (dm *DynamoManager) AutoMigrateModels(ctx context.Context, models ...interface{}) error {
	// DynamoDB doesn't support schema migration like traditional databases
	// This is a no-op for DynamoDB
	dm.logger.Info("automigration skipped for DynamoDB (not supported)",
		zap.Int("model_count", len(models)))
	return nil
}

// Helper methods for DynamoDB operations
func (dm *DynamoManager) marshalToItem(model interface{}) (map[string]types.AttributeValue, error) {
	item := make(map[string]types.AttributeValue)

	val := reflect.ValueOf(model).Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Use json tag if present, else field name
		jsonTag := fieldType.Tag.Get("json")
		fieldName := fieldType.Name
		if jsonTag != "" && jsonTag != "-" {
			commaIdx := strings.Index(jsonTag, ",")
			if commaIdx > 0 {
				fieldName = jsonTag[:commaIdx]
			} else {
				fieldName = jsonTag
			}
		}

		switch field.Kind() {
		case reflect.String:
			item[fieldName] = &types.AttributeValueMemberS{Value: field.String()}
		case reflect.Int, reflect.Int64:
			item[fieldName] = &types.AttributeValueMemberN{Value: fmt.Sprint(field.Int())}
		case reflect.Bool:
			item[fieldName] = &types.AttributeValueMemberBOOL{Value: field.Bool()}
		}
	}

	// Ensure 'id' key is present if 'ID' or 'id' field exists
	if _, ok := item["id"]; !ok {
		if v, ok := item["ID"]; ok {
			item["id"] = v
		}
	}

	return item, nil
}

func (dm *DynamoManager) unmarshalFromItem(item map[string]types.AttributeValue, model interface{}) error {
	val := reflect.ValueOf(model).Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Use json tag if present, else field name
		jsonTag := fieldType.Tag.Get("json")
		fieldName := fieldType.Name
		if jsonTag != "" && jsonTag != "-" {
			commaIdx := strings.Index(jsonTag, ",")
			if commaIdx > 0 {
				fieldName = jsonTag[:commaIdx]
			} else {
				fieldName = jsonTag
			}
		}

		if attrValue, exists := item[fieldName]; exists {
			switch v := attrValue.(type) {
			case *types.AttributeValueMemberS:
				field.SetString(v.Value)
			case *types.AttributeValueMemberN:
				if num, err := strconv.ParseInt(v.Value, 10, 64); err == nil {
					field.SetInt(num)
				}
			case *types.AttributeValueMemberBOOL:
				field.SetBool(v.Value)
			}
		}
	}

	return nil
}

func (dm *DynamoManager) valueToAttributeValue(value interface{}) (types.AttributeValue, error) {
	switch v := value.(type) {
	case string:
		return &types.AttributeValueMemberS{Value: v}, nil
	case int, int64:
		return &types.AttributeValueMemberN{Value: fmt.Sprint(v)}, nil
	case bool:
		return &types.AttributeValueMemberBOOL{Value: v}, nil
	default:
		return nil, fmt.Errorf("unsupported value type: %T", value)
	}
}
