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

// SoftDelete soft deletes a record by setting deleted_at and deleted_by fields
func (dm *DynamoManager) SoftDelete(ctx context.Context, id interface{}, deletedBy string) error {
	client := dm.GetClient()
	if client == nil {
		return fmt.Errorf("dynamodb client not connected")
	}

	// For DynamoDB, we'll update the item to add deleted_at and deleted_by fields
	// This is a simplified implementation
	key := map[string]types.AttributeValue{
		"id": &types.AttributeValueMemberS{Value: fmt.Sprint(id)},
	}

	updateExpression := "SET deleted_at = :deleted_at, deleted_by = :deleted_by, updated_at = :updated_at"
	expressionAttributeValues := map[string]types.AttributeValue{
		":deleted_at": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		":deleted_by": &types.AttributeValueMemberS{Value: deletedBy},
		":updated_at": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
	}

	_, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(dm.config.DynamoDBTable),
		Key:                       key,
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeValues: expressionAttributeValues,
		ConditionExpression:       aws.String("attribute_exists(id)"),
	})

	if err != nil {
		return fmt.Errorf("failed to soft delete record: %w", err)
	}

	return nil
}

// SoftDeleteMany soft deletes multiple records
func (dm *DynamoManager) SoftDeleteMany(ctx context.Context, ids []interface{}, deletedBy string) error {
	if len(ids) == 0 {
		return nil
	}

	client := dm.GetClient()
	if client == nil {
		return fmt.Errorf("dynamodb client not connected")
	}

	// For DynamoDB, we'll process each ID individually
	// In a production environment, you might want to use BatchWriteItem
	for _, id := range ids {
		if err := dm.SoftDelete(ctx, id, deletedBy); err != nil {
			return fmt.Errorf("failed to soft delete record %v: %w", id, err)
		}
	}

	return nil
}

// Restore restores a soft-deleted record
func (dm *DynamoManager) Restore(ctx context.Context, id interface{}) error {
	client := dm.GetClient()
	if client == nil {
		return fmt.Errorf("dynamodb client not connected")
	}

	key := map[string]types.AttributeValue{
		"id": &types.AttributeValueMemberS{Value: fmt.Sprint(id)},
	}

	updateExpression := "REMOVE deleted_at, deleted_by SET updated_at = :updated_at"
	expressionAttributeValues := map[string]types.AttributeValue{
		":updated_at": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
	}

	_, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(dm.config.DynamoDBTable),
		Key:                       key,
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeValues: expressionAttributeValues,
		ConditionExpression:       aws.String("attribute_exists(id)"),
	})

	if err != nil {
		return fmt.Errorf("failed to restore record: %w", err)
	}

	return nil
}

// ListWithDeleted retrieves records including soft-deleted ones
func (dm *DynamoManager) ListWithDeleted(ctx context.Context, limit, offset int, models interface{}) error {
	// For DynamoDB, we'll use the regular List method since we can't easily filter by deleted_at
	return dm.List(ctx, &base.Filter{}, models)
}

// CountWithDeleted returns count including soft-deleted records
func (dm *DynamoManager) CountWithDeleted(ctx context.Context) (int64, error) {
	// For DynamoDB, we'll return an approximate count
	client := dm.GetClient()
	if client == nil {
		return 0, fmt.Errorf("dynamodb client not connected")
	}

	// This is a simplified implementation - in production you might want to use a different approach
	result, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(dm.config.DynamoDBTable),
	})
	if err != nil {
		return 0, err
	}

	if result.Table.ItemCount == nil {
		return 0, nil
	}

	return *result.Table.ItemCount, nil
}

// Exists checks if a record exists
func (dm *DynamoManager) Exists(ctx context.Context, id interface{}) (bool, error) {
	client := dm.GetClient()
	if client == nil {
		return false, fmt.Errorf("dynamodb client not connected")
	}

	key := map[string]types.AttributeValue{
		"id": &types.AttributeValueMemberS{Value: fmt.Sprint(id)},
	}

	result, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(dm.config.DynamoDBTable),
		Key:       key,
	})
	if err != nil {
		return false, err
	}

	return result.Item != nil, nil
}

// ExistsWithDeleted checks if record exists including soft-deleted ones
func (dm *DynamoManager) ExistsWithDeleted(ctx context.Context, id interface{}) (bool, error) {
	// For DynamoDB, we'll use the regular Exists method
	return dm.Exists(ctx, id)
}

// GetByCreatedBy gets records by creator
func (dm *DynamoManager) GetByCreatedBy(ctx context.Context, createdBy interface{}, limit, offset int, models interface{}) error {
	// For DynamoDB, we'll use a GSI if available, otherwise return empty
	// This is a simplified implementation
	return fmt.Errorf("GetByCreatedBy not implemented for DynamoDB")
}

// GetByUpdatedBy gets records by updater
func (dm *DynamoManager) GetByUpdatedBy(ctx context.Context, updatedBy interface{}, limit, offset int, models interface{}) error {
	// For DynamoDB, we'll use a GSI if available, otherwise return empty
	// This is a simplified implementation
	return fmt.Errorf("GetByUpdatedBy not implemented for DynamoDB")
}

// GetByDeletedBy gets records by deleter
func (dm *DynamoManager) GetByDeletedBy(ctx context.Context, deletedBy interface{}, limit, offset int, models interface{}) error {
	// For DynamoDB, we'll use a GSI if available, otherwise return empty
	// This is a simplified implementation
	return fmt.Errorf("GetByDeletedBy not implemented for DynamoDB")
}

// CreateMany creates multiple records
func (dm *DynamoManager) CreateMany(ctx context.Context, models []interface{}) error {
	if len(models) == 0 {
		return nil
	}

	client := dm.GetClient()
	if client == nil {
		return fmt.Errorf("dynamodb client not connected")
	}

	// For DynamoDB, we'll process each model individually
	// In a production environment, you might want to use BatchWriteItem
	for _, model := range models {
		if err := dm.Create(ctx, model); err != nil {
			return fmt.Errorf("failed to create model: %w", err)
		}
	}

	return nil
}

// UpdateMany updates multiple records
func (dm *DynamoManager) UpdateMany(ctx context.Context, models []interface{}) error {
	if len(models) == 0 {
		return nil
	}

	client := dm.GetClient()
	if client == nil {
		return fmt.Errorf("dynamodb client not connected")
	}

	// For DynamoDB, we'll process each model individually
	for _, model := range models {
		if err := dm.Update(ctx, model); err != nil {
			return fmt.Errorf("failed to update model: %w", err)
		}
	}

	return nil
}

// DeleteMany deletes multiple records
func (dm *DynamoManager) DeleteMany(ctx context.Context, ids []interface{}) error {
	if len(ids) == 0 {
		return nil
	}

	client := dm.GetClient()
	if client == nil {
		return fmt.Errorf("dynamodb client not connected")
	}

	// For DynamoDB, we'll process each ID individually
	// In a production environment, you might want to use BatchWriteItem
	for _, id := range ids {
		if err := dm.Delete(ctx, id); err != nil {
			return fmt.Errorf("failed to delete record %v: %w", id, err)
		}
	}

	return nil
}

// List retrieves records from DynamoDB with filter support including pagination
func (dm *DynamoManager) List(ctx context.Context, filter *base.Filter, model interface{}) error {
	client := dm.GetClient()
	if client == nil {
		return fmt.Errorf("dynamodb client not connected")
	}

	// For DynamoDB, we'll do a scan with filter expressions
	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(dm.config.DynamoDBTable),
	}

	// Apply filter conditions if provided
	if filter != nil && len(filter.Group.Conditions) > 0 {
		var filterExpressions []string
		var expressionAttributeNames map[string]string
		var expressionAttributeValues map[string]types.AttributeValue

		for i, condition := range filter.Group.Conditions {
			fieldName := fmt.Sprintf("#field%d", i)
			valueName := fmt.Sprintf(":value%d", i)

			if expressionAttributeNames == nil {
				expressionAttributeNames = make(map[string]string)
			}
			if expressionAttributeValues == nil {
				expressionAttributeValues = make(map[string]types.AttributeValue)
			}

			expressionAttributeNames[fieldName] = condition.Field

			// Convert value to DynamoDB attribute value
			attrValue, err := dm.valueToAttributeValue(condition.Value)
			if err != nil {
				return fmt.Errorf("failed to convert value: %w", err)
			}
			expressionAttributeValues[valueName] = attrValue

			var expression string
			switch condition.Operator {
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
				return fmt.Errorf("unsupported filter operator for DynamoDB: %s", condition.Operator)
			}

			filterExpressions = append(filterExpressions, expression)
		}

		if len(filterExpressions) > 0 {
			scanInput.FilterExpression = aws.String(strings.Join(filterExpressions, " AND "))
			scanInput.ExpressionAttributeNames = expressionAttributeNames
			scanInput.ExpressionAttributeValues = expressionAttributeValues
		}
	}

	// Apply pagination if provided
	if filter != nil && filter.Limit > 0 {
		scanInput.Limit = aws.Int32(int32(filter.Limit))
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

// Count counts records in DynamoDB with filter support
func (dm *DynamoManager) Count(ctx context.Context, filter *base.Filter, model interface{}) (int64, error) {
	client := dm.GetClient()
	if client == nil {
		return 0, fmt.Errorf("dynamodb client not connected")
	}

	// For DynamoDB, we'll do a scan with filter expressions and count
	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(dm.config.DynamoDBTable),
		Select:    types.SelectCount,
	}

	// Apply filter conditions if provided (same logic as List)
	if filter != nil && len(filter.Group.Conditions) > 0 {
		var filterExpressions []string
		var expressionAttributeNames map[string]string
		var expressionAttributeValues map[string]types.AttributeValue

		for i, condition := range filter.Group.Conditions {
			fieldName := fmt.Sprintf("#field%d", i)
			valueName := fmt.Sprintf(":value%d", i)

			if expressionAttributeNames == nil {
				expressionAttributeNames = make(map[string]string)
			}
			if expressionAttributeValues == nil {
				expressionAttributeValues = make(map[string]types.AttributeValue)
			}

			expressionAttributeNames[fieldName] = condition.Field

			// Convert value to DynamoDB attribute value
			attrValue, err := dm.valueToAttributeValue(condition.Value)
			if err != nil {
				return 0, fmt.Errorf("failed to convert value: %w", err)
			}
			expressionAttributeValues[valueName] = attrValue

			var expression string
			switch condition.Operator {
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
				return 0, fmt.Errorf("unsupported filter operator for DynamoDB: %s", condition.Operator)
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
		return 0, fmt.Errorf("failed to count objects: %w", err)
	}

	return int64(result.Count), nil
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
