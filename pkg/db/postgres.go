// Package db provides PostgreSQL/GORM database connection management.
package db

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"unicode"

	"github.com/Kisanlink/kisanlink-db/pkg/base"
	"github.com/cenkalti/backoff/v4"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PostgresManager manages PostgreSQL database connections with GORM
type PostgresManager struct {
	config     *Config
	primary    *gorm.DB
	replicas   []*gorm.DB
	circuit    *gobreaker.CircuitBreaker
	logger     *zap.Logger
	mu         sync.RWMutex
	healthChan chan bool
}

// NewPostgresManager creates a new PostgreSQL manager
func NewPostgresManager(config *Config, logger *zap.Logger) *PostgresManager {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "postgres",
		MaxRequests: 5,
		Interval:    0,
		Timeout:     5 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Info("postgres circuit breaker state changed",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	})

	return &PostgresManager{
		config:     config,
		circuit:    cb,
		logger:     logger,
		healthChan: make(chan bool, 1),
	}
}

// Connect establishes connection to PostgreSQL
func (pm *PostgresManager) Connect(ctx context.Context) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	primary, err := pm.connectDB(pm.config, true)
	if err != nil {
		return fmt.Errorf("failed to connect to primary: %w", err)
	}
	pm.primary = primary
	// Connect to read replicas if configured
	if len(pm.config.PostgresReadReplicas) > 0 {
		for _, replica := range pm.config.PostgresReadReplicas {
			replicaConfig := *pm.config // Create a copy of the config
			replicaConfig.PostgresHost = replica
			db, err := pm.connectDB(&replicaConfig, false)
			if err != nil {
				pm.logger.Warn("failed to connect to read replica",
					zap.String("replica", replica),
					zap.Error(err),
				)
				continue
			}
			pm.replicas = append(pm.replicas, db)
			pm.logger.Info("connected to read replica",
				zap.String("replica", replica))
		}
	}

	go pm.healthCheck(ctx)

	return nil
}

// connectDB establishes a new database connection
func (pm *PostgresManager) connectDB(config *Config, _ bool) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		config.PostgresHost, config.PostgresPort, config.PostgresUser,
		config.PostgresPassword, config.PostgresDBName, config.PostgresSSLMode)

	gormLogger := logger.New(
		&GormLogger{logger: pm.logger},
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	var db *gorm.DB
	operation := func() error {
		var err error
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger:      gormLogger,
			PrepareStmt: true,
		})
		return err
	}

	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.MaxElapsedTime = 30 * time.Second
	err := backoff.Retry(operation, backoffConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect after retries: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	sqlDB.SetMaxOpenConns(config.PostgresMaxConns)
	sqlDB.SetMaxIdleConns(config.PostgresIdleConns)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

// healthCheck periodically checks the health of the primary database
func (pm *PostgresManager) healthCheck(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			healthy := pm.checkHealth()
			select {
			case pm.healthChan <- healthy:
			default:
			}
		}
	}
}

// checkHealth pings the primary database to verify connectivity
func (pm *PostgresManager) checkHealth() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.primary == nil {
		return false
	}

	sqlDB, err := pm.primary.DB()
	if err != nil {
		return false
	}

	return sqlDB.Ping() == nil
}

// GetDB returns a database connection
func (pm *PostgresManager) GetDB(_ context.Context, readOnly bool) (*gorm.DB, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if readOnly && len(pm.replicas) > 0 {
		replica := pm.replicas[0]
		pm.replicas = append(pm.replicas[1:], replica)
		return replica, nil
	}

	return pm.primary, nil
}

// Close closes all database connections
func (pm *PostgresManager) Close() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.primary != nil {
		sqlDB, err := pm.primary.DB()
		if err != nil {
			return err
		}
		if cerr := sqlDB.Close(); cerr != nil {
			return cerr
		}
	}

	for _, replica := range pm.replicas {
		sqlDB, err := replica.DB()
		if err != nil {
			return err
		}
		if cerr := sqlDB.Close(); cerr != nil {
			return cerr
		}
	}

	return nil
}

// WithTransaction executes a function within a transaction
func (pm *PostgresManager) WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	db, err := pm.GetDB(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	return db.Transaction(func(tx *gorm.DB) error {
		return fn(tx)
	})
}

// WithReadOnly executes a function using a read replica if available
func (pm *PostgresManager) WithReadOnly(ctx context.Context, fn func(tx *gorm.DB) error) error {
	db, err := pm.GetDB(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to get read-only database connection: %w", err)
	}

	return fn(db)
}

// IsConnected checks if the database is connected
func (pm *PostgresManager) IsConnected() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.primary != nil
}

// GetBackendType returns the type of backend this manager handles
func (pm *PostgresManager) GetBackendType() BackendType {
	return BackendGorm
}

// Create creates a new record in the database
func (pm *PostgresManager) Create(ctx context.Context, model interface{}) error {
	db, err := pm.GetDB(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}
	return db.WithContext(ctx).Create(model).Error
}

// GetByID retrieves a record by ID
func (pm *PostgresManager) GetByID(ctx context.Context, id interface{}, model interface{}) error {
	db, err := pm.GetDB(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	// Use Where clause to be more explicit about the ID condition
	return db.WithContext(ctx).Where("id = ?", id).First(model).Error
}

// Update updates an existing record
func (pm *PostgresManager) Update(ctx context.Context, model interface{}) error {
	db, err := pm.GetDB(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}
	return db.WithContext(ctx).Save(model).Error
}

// Delete deletes a record by ID
func (pm *PostgresManager) Delete(ctx context.Context, id interface{}, model interface{}) error {
	db, err := pm.GetDB(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	// Use the model to determine the table name
	tableName := pm.getTableName(model)
	return db.WithContext(ctx).Table(tableName).Where("id = ?", id).Delete("").Error
}

// SoftDelete soft deletes a record by setting deleted_at and deleted_by fields
func (pm *PostgresManager) SoftDelete(ctx context.Context, id interface{}, model interface{}, deletedBy string) error {
	db, err := pm.GetDB(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	// Extract table name from the model
	tableName := pm.getTableName(model)

	// Check if record exists and is not already deleted
	var count int64
	if err := db.WithContext(ctx).Table(tableName).Where("id = ? AND deleted_at IS NULL", id).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check record existence in table %s: %w", tableName, err)
	}

	if count == 0 {
		return fmt.Errorf("record with id %v not found in table %s or already deleted", id, tableName)
	}

	// Perform soft delete
	result := db.WithContext(ctx).Table(tableName).Where("id = ? AND deleted_at IS NULL", id).Updates(map[string]interface{}{
		"deleted_at": time.Now(),
		"deleted_by": deletedBy,
		"updated_at": time.Now(),
	})

	if result.Error != nil {
		return fmt.Errorf("failed to soft delete from table %s: %w", tableName, result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("no records were soft deleted from table %s", tableName)
	}

	return nil
}

// Restore restores a soft-deleted record
func (pm *PostgresManager) Restore(ctx context.Context, id interface{}, model interface{}) error {
	db, err := pm.GetDB(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	// Extract table name from the model
	tableName := pm.getTableName(model)

	// Check if record exists and is soft deleted
	var count int64
	if err := db.WithContext(ctx).Table(tableName).Where("id = ? AND deleted_at IS NOT NULL", id).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check record existence in table %s: %w", tableName, err)
	}

	if count == 0 {
		return fmt.Errorf("soft-deleted record with id %v not found in table %s", id, tableName)
	}

	// Restore the record
	result := db.WithContext(ctx).Table(tableName).Where("id = ? AND deleted_at IS NOT NULL", id).Updates(map[string]interface{}{
		"deleted_at": nil,
		"deleted_by": nil,
		"updated_at": time.Now(),
	})

	if result.Error != nil {
		return fmt.Errorf("failed to restore from table %s: %w", tableName, result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("no records were restored from table %s", tableName)
	}

	return nil
}

// ListWithDeleted retrieves records including soft-deleted ones
func (pm *PostgresManager) ListWithDeleted(ctx context.Context, limit, offset int, models interface{}) error {
	db, err := pm.GetDB(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	query := db.WithContext(ctx).Unscoped() // Include soft-deleted records
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	return query.Find(models).Error
}

// CountWithDeleted returns count including soft-deleted records
func (pm *PostgresManager) CountWithDeleted(ctx context.Context, model interface{}) (int64, error) {
	db, err := pm.GetDB(ctx, true)
	if err != nil {
		return 0, fmt.Errorf("failed to get database connection: %w", err)
	}

	var count int64
	err = db.WithContext(ctx).Unscoped().Model(model).Count(&count).Error
	return count, err
}

// ExistsWithDeleted checks if record exists including soft-deleted ones
func (pm *PostgresManager) ExistsWithDeleted(ctx context.Context, id interface{}) (bool, error) {
	db, err := pm.GetDB(ctx, true)
	if err != nil {
		return false, fmt.Errorf("failed to get database connection: %w", err)
	}

	var count int64
	err = db.WithContext(ctx).Unscoped().Model(&struct{}{}).Where("id = ?", id).Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetByCreatedBy gets records by creator
func (pm *PostgresManager) GetByCreatedBy(ctx context.Context, createdBy interface{}, limit, offset int, models interface{}) error {
	db, err := pm.GetDB(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	query := db.WithContext(ctx).Where("created_by = ?", createdBy)
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	return query.Find(models).Error
}

// GetByUpdatedBy gets records by updater
func (pm *PostgresManager) GetByUpdatedBy(ctx context.Context, updatedBy interface{}, limit, offset int, models interface{}) error {
	db, err := pm.GetDB(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	query := db.WithContext(ctx).Where("updated_by = ?", updatedBy)
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	return query.Find(models).Error
}

// GetByDeletedBy gets records by deleter
func (pm *PostgresManager) GetByDeletedBy(ctx context.Context, deletedBy interface{}, limit, offset int, models interface{}) error {
	db, err := pm.GetDB(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	query := db.WithContext(ctx).Unscoped().Where("deleted_by = ?", deletedBy)
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	return query.Find(models).Error
}

// CreateMany creates multiple records
func (pm *PostgresManager) CreateMany(ctx context.Context, models []interface{}) error {
	if len(models) == 0 {
		return nil
	}

	db, err := pm.GetDB(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	return db.WithContext(ctx).CreateInBatches(models, 100).Error
}

// UpdateMany updates multiple records
func (pm *PostgresManager) UpdateMany(ctx context.Context, models []interface{}) error {
	if len(models) == 0 {
		return nil
	}

	db, err := pm.GetDB(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	for _, model := range models {
		if err := db.WithContext(ctx).Save(model).Error; err != nil {
			return fmt.Errorf("failed to update model: %w", err)
		}
	}

	return nil
}

// DeleteMany deletes multiple records
func (pm *PostgresManager) DeleteMany(ctx context.Context, ids []interface{}) error {
	if len(ids) == 0 {
		return nil
	}

	db, err := pm.GetDB(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	return db.WithContext(ctx).Delete("", ids).Error
}

// validateSelectFields validates field names for SQL injection prevention
// Validates against max 50 fields and ensures each field matches the pattern:
// ^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)?$
func (pm *PostgresManager) validateSelectFields(fields []string) error {
	if len(fields) > 50 {
		return fmt.Errorf("too many select fields: %d (max: 50)", len(fields))
	}
	// Pattern allows: field_name or table.field_name
	validFieldPattern := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)?$`)
	for _, field := range fields {
		if !validFieldPattern.MatchString(field) {
			return fmt.Errorf("invalid field name: %s", field)
		}
	}
	return nil
}

// validatePreload validates preload relation names and depth
// Validates against max depth of 3 (e.g., "CropCycle.Crop.Variety")
func (pm *PostgresManager) validatePreload(preload base.Preload) error {
	// Pattern allows nested relations: Relation or Relation.NestedRelation
	validFieldPattern := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)*$`)
	if !validFieldPattern.MatchString(preload.Relation) {
		return fmt.Errorf("invalid preload relation: %s", preload.Relation)
	}

	// Check depth (max 3 levels: e.g., "CropCycle.Crop.Variety")
	depth := strings.Count(preload.Relation, ".") + 1
	if depth > 3 {
		return fmt.Errorf("preload depth %d exceeds maximum 3: %s", depth, preload.Relation)
	}

	return nil
}

// List retrieves records from PostgreSQL with filter support including pagination, sorting, preloads, and selects
// Operation order (critical for performance and security):
// 1. Apply filter conditions (reduce rows)
// 2. Apply select with validation (reduce columns)
// 3. Apply sorting
// 4. Apply pagination
// 5. Apply preloads with validation (only on paginated results)
func (pm *PostgresManager) List(ctx context.Context, filter *base.Filter, model interface{}) error {
	db, err := pm.GetDB(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	// Set the model/table context for GORM
	query := db.WithContext(ctx).Model(model)

	// 1. Apply filter conditions FIRST (reduce rows)
	if filter != nil {
		pm.applyFilterGroup(query, &filter.Group)
	}

	// 2. Apply select (reduce columns) with validation
	if filter != nil && len(filter.Selects) > 0 {
		if err := pm.validateSelectFields(filter.Selects); err != nil {
			return fmt.Errorf("invalid select fields: %w", err)
		}
		query = query.Select(filter.Selects)
	}

	// 3. Apply sorting
	if filter != nil && len(filter.Sort) > 0 {
		for _, sort := range filter.Sort {
			direction := "ASC"
			if sort.Direction == "desc" || sort.Direction == "DESC" {
				direction = "DESC"
			}
			query = query.Order(fmt.Sprintf("%s %s", sort.Field, direction))
		}
	}

	// 4. Apply pagination
	if filter != nil {
		if filter.Limit > 0 {
			query = query.Limit(filter.Limit)
		}
		if filter.Offset > 0 {
			query = query.Offset(filter.Offset)
		}
		// Alternative pagination using Page/PageSize
		if filter.Page > 0 && filter.PageSize > 0 {
			offset := (filter.Page - 1) * filter.PageSize
			query = query.Limit(filter.PageSize).Offset(offset)
		}
	}

	// 5. Apply preloads LAST (only on paginated results) with validation
	if filter != nil && len(filter.Preloads) > 0 {
		if len(filter.Preloads) > 5 {
			return fmt.Errorf("too many preloads: %d (max: 5)", len(filter.Preloads))
		}
		for _, preload := range filter.Preloads {
			if err := pm.validatePreload(preload); err != nil {
				return fmt.Errorf("invalid preload: %w", err)
			}
			if len(preload.Conditions) > 0 {
				query = query.Preload(preload.Relation, preload.Conditions...)
			} else {
				query = query.Preload(preload.Relation)
			}
		}
	}

	return query.Find(model).Error
}

// applyFilterGroup recursively applies filter groups with proper OR/AND logic
func (pm *PostgresManager) applyFilterGroup(query *gorm.DB, group *base.FilterGroup) {
	if group == nil || (len(group.Conditions) == 0 && len(group.Groups) == 0) {
		return
	}

	// Handle main group conditions
	if len(group.Conditions) > 0 {
		if group.Logic == base.LogicOr {
			// For OR logic, use GORM's native OR support
			// Start with the first condition
			if len(group.Conditions) > 0 {
				firstCondition := group.Conditions[0]
				if err := pm.applyFilterCondition(query, firstCondition); err != nil {
					pm.logger.Error("failed to apply first filter condition", zap.Error(err))
				}

				// Add subsequent conditions with OR
				for i := 1; i < len(group.Conditions); i++ {
					condition := group.Conditions[i]
					query = query.Or(pm.buildGormCondition(condition))
				}
			}
		} else {
			// For AND logic (default), apply conditions normally
			for _, condition := range group.Conditions {
				if err := pm.applyFilterCondition(query, condition); err != nil {
					pm.logger.Error("failed to apply filter condition", zap.Error(err))
					continue
				}
			}
		}
	}

	// Handle sub-groups recursively
	if len(group.Groups) > 0 {
		if group.Logic == base.LogicOr {
			// For OR logic with sub-groups, we need to handle this carefully
			// Since we can't easily combine complex nested OR groups, we'll process them as AND for now
			// This is a limitation of the current approach - we'd need a more sophisticated query builder
			for _, subGroup := range group.Groups {
				pm.applyFilterGroup(query, &subGroup)
			}
		} else {
			// For AND logic with sub-groups, apply them normally
			for _, subGroup := range group.Groups {
				pm.applyFilterGroup(query, &subGroup)
			}
		}
	}
}

// buildGormCondition builds a GORM condition for OR clauses
func (pm *PostgresManager) buildGormCondition(condition base.FilterCondition) *gorm.DB {
	// Create a new GORM DB instance just for building the condition
	// This is a bit of a hack, but it's the cleanest way to build OR conditions
	db, err := pm.GetDB(context.Background(), true)
	if err != nil {
		pm.logger.Error("failed to get DB for building condition", zap.Error(err))
		return db
	}

	switch condition.Operator {
	case base.OpEqual:
		return db.Where(condition.Field+" = ?", condition.Value)
	case base.OpNotEqual:
		return db.Where(condition.Field+" != ?", condition.Value)
	case base.OpGreaterThan:
		return db.Where(condition.Field+" > ?", condition.Value)
	case base.OpLessThan:
		return db.Where(condition.Field+" < ?", condition.Value)
	case base.OpGreaterEqual:
		return db.Where(condition.Field+" >= ?", condition.Value)
	case base.OpLessEqual:
		return db.Where(condition.Field+" <= ?", condition.Value)
	case base.OpIn:
		return db.Where(condition.Field+" IN ?", condition.Value)
	case base.OpNotIn:
		return db.Where(condition.Field+" NOT IN ?", condition.Value)
	case base.OpLike:
		return db.Where(condition.Field+" LIKE ?", condition.Value)
	case base.OpContains:
		return db.Where(condition.Field+" LIKE ?", fmt.Sprintf("%%%v%%", condition.Value))
	case base.OpStartsWith:
		return db.Where(condition.Field+" LIKE ?", fmt.Sprintf("%v%%", condition.Value))
	case base.OpEndsWith:
		return db.Where(condition.Field+" LIKE ?", fmt.Sprintf("%%%v", condition.Value))
	case base.OpIsNull:
		return db.Where(condition.Field + " IS NULL")
	case base.OpIsNotNull:
		return db.Where(condition.Field + " IS NOT NULL")
	default:
		pm.logger.Error("unsupported filter operator", zap.String("operator", string(condition.Operator)))
		return db
	}
}

// Count counts records in PostgreSQL with filter support
func (pm *PostgresManager) Count(ctx context.Context, filter *base.Filter, model interface{}) (int64, error) {
	db, err := pm.GetDB(ctx, true)
	if err != nil {
		return 0, fmt.Errorf("failed to get database connection: %w", err)
	}

	// Set the model/table context for GORM
	query := db.WithContext(ctx).Model(model)

	// Apply filter conditions if provided
	if filter != nil {
		pm.applyFilterGroup(query, &filter.Group)
	}

	var count int64
	return count, query.Count(&count).Error
}

// applyFilterCondition applies a single filter condition to the query
func (pm *PostgresManager) applyFilterCondition(query *gorm.DB, filter base.FilterCondition) error {
	switch filter.Operator {
	case base.OpEqual:
		query = query.Where(filter.Field+" = ?", filter.Value)
	case base.OpNotEqual:
		query = query.Where(filter.Field+" != ?", filter.Value)
	case base.OpGreaterThan:
		query = query.Where(filter.Field+" > ?", filter.Value)
	case base.OpLessThan:
		query = query.Where(filter.Field+" < ?", filter.Value)
	case base.OpGreaterEqual:
		query = query.Where(filter.Field+" >= ?", filter.Value)
	case base.OpLessEqual:
		query = query.Where(filter.Field+" <= ?", filter.Value)
	case base.OpIn:
		query = query.Where(filter.Field+" IN ?", filter.Value)
	case base.OpNotIn:
		query = query.Where(filter.Field+" NOT IN ?", filter.Value)
	case base.OpLike:
		query = query.Where(filter.Field+" LIKE ?", filter.Value)
	case base.OpContains:
		query = query.Where(filter.Field+" LIKE ?", "%"+fmt.Sprint(filter.Value)+"%")
	case base.OpStartsWith:
		query = query.Where(filter.Field+" LIKE ?", fmt.Sprint(filter.Value)+"%")
	case base.OpEndsWith:
		query = query.Where(filter.Field+" LIKE ?", "%"+fmt.Sprint(filter.Value))
	case base.OpIsNull:
		query = query.Where(filter.Field + " IS NULL")
	case base.OpIsNotNull:
		query = query.Where(filter.Field + " IS NOT NULL")
	default:
		return fmt.Errorf("unsupported filter operator: %s", filter.Operator)
	}
	return nil
}

// AutoMigrateModels runs automigration for specific models
func (pm *PostgresManager) AutoMigrateModels(ctx context.Context, models ...interface{}) error {
	db, err := pm.GetDB(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to get database connection for migration: %w", err)
	}

	pm.logger.Info("running automigration for models", zap.Int("model_count", len(models)))

	if err := db.WithContext(ctx).AutoMigrate(models...); err != nil {
		pm.logger.Error("automigration failed", zap.Error(err))
		return fmt.Errorf("automigration failed: %w", err)
	}

	pm.logger.Info("automigration completed successfully")
	return nil
}

// GormLogger implements the gorm.Logger interface for structured logging
type GormLogger struct {
	logger *zap.Logger
}

// Printf logs a formatted message using zap
func (l *GormLogger) Printf(format string, args ...interface{}) {
	l.logger.Sugar().Infof(format, args...)
}

// getTableName extracts the table name from a model
func (pm *PostgresManager) getTableName(model interface{}) string {
	// Try to get table name from the model if it implements a specific interface
	if tableModel, ok := model.(interface {
		GetTableName() string
	}); ok {
		return tableModel.GetTableName()
	}

	// Try to get table name from the model if it has a TableName method
	if tableModel, ok := model.(interface {
		TableName() string
	}); ok {
		return tableModel.TableName()
	}

	// Try to get table name from the model if it has a constant
	if tableModel, ok := model.(interface {
		GetTableConstant() string
	}); ok {
		return tableModel.GetTableConstant()
	}

	// Default fallback - use reflection to get the type name and convert to snake_case
	return pm.getDefaultTableName(model)
}

// getDefaultTableName provides a default table name based on the model type
func (pm *PostgresManager) getDefaultTableName(model interface{}) string {
	// This is a fallback that tries to infer the table name from the type
	// In practice, models should implement one of the table name interfaces above
	typeName := fmt.Sprintf("%T", model)

	// Remove package prefix and pointer
	if idx := strings.LastIndex(typeName, "."); idx != -1 {
		typeName = typeName[idx+1:]
	}
	if strings.HasPrefix(typeName, "*") {
		typeName = typeName[1:]
	}

	// Convert to snake_case and pluralize
	tableName := pm.toSnakeCase(typeName)
	if !strings.HasSuffix(tableName, "s") {
		tableName += "s"
	}

	return tableName
}

// toSnakeCase converts CamelCase to snake_case
func (pm *PostgresManager) toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			result.WriteByte('_')
		}
		result.WriteRune(unicode.ToLower(r))
	}
	return result.String()
}
