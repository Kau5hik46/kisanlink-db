// Package db provides PostgreSQL/GORM database connection management.
package db

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	return db.WithContext(ctx).First(model, id).Error
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
func (pm *PostgresManager) Delete(ctx context.Context, id interface{}) error {
	db, err := pm.GetDB(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}
	return db.WithContext(ctx).Delete("", id).Error
}

// List retrieves records from PostgreSQL with basic filtering
func (pm *PostgresManager) List(ctx context.Context, filters []base.FilterCondition, model interface{}) error {
	db, err := pm.GetDB(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	query := db.WithContext(ctx)

	// Apply basic filters if provided
	if len(filters) > 0 {
		for _, filter := range filters {
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
			default:
				return fmt.Errorf("unsupported filter operator: %s", filter.Operator)
			}
		}
	}

	return query.Find(model).Error
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
