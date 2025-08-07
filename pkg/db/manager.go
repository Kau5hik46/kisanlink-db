// Package db provides database connection management for multiple backends.
package db

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// BackendType represents the type of database backend
type BackendType string

const (
	BackendInMemory BackendType = "inmemory"
	BackendGorm     BackendType = "gorm"
	BackendDynamo   BackendType = "dynamodb"
	BackendSpiceDB  BackendType = "spicedb"
	BackendNeo4j    BackendType = "neo4j"
	BackendVector   BackendType = "vector"
	BackendS3       BackendType = "s3"
)

// DBManager defines the interface that all database managers must implement
type DBManager interface {
	// Connect establishes connection to the database
	Connect(ctx context.Context) error

	// Close closes the database connection
	Close() error

	// IsConnected checks if the database is connected
	IsConnected() bool

	// GetBackendType returns the type of backend this manager handles
	GetBackendType() BackendType

	// CRUD Operations
	Create(ctx context.Context, model interface{}) error
	GetByID(ctx context.Context, id interface{}, model interface{}) error
	Update(ctx context.Context, model interface{}) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters []Filter, model interface{}) error

	// Migration Operations
	AutoMigrateModels(ctx context.Context, models ...interface{}) error
}

// Filter represents a database filter
type Filter struct {
	Field    string         `json:"field"`
	Operator FilterOperator `json:"operator"`
	Value    interface{}    `json:"value"`
}

// FilterOperator represents the type of filter operation
type FilterOperator string

const (
	FilterOpEqual        FilterOperator = "eq"
	FilterOpNotEqual     FilterOperator = "ne"
	FilterOpGreaterThan  FilterOperator = "gt"
	FilterOpLessThan     FilterOperator = "lt"
	FilterOpGreaterEqual FilterOperator = "gte"
	FilterOpLessEqual    FilterOperator = "lte"
	FilterOpIn           FilterOperator = "in"
	FilterOpNotIn        FilterOperator = "nin"
	FilterOpLike         FilterOperator = "like"
	FilterOpILike        FilterOperator = "ilike"
	FilterOpContains     FilterOperator = "contains"
	FilterOpStartsWith   FilterOperator = "starts_with"
	FilterOpEndsWith     FilterOperator = "ends_with"
)

// DatabaseManager manages connections to different database backends
type DatabaseManager struct {
	mu sync.RWMutex

	// Backend managers
	postgresManager *PostgresManager
	dynamoManager   *DynamoManager
	spiceManager    *SpiceManager
	s3Manager       *S3Manager

	// Configuration
	config *Config
	logger *zap.Logger
}

// Config holds configuration for different database backends
type Config struct {
	// Backend selection
	PrimaryBackend BackendType `env:"DB_PRIMARY_BACKEND" envDefault:"gorm"`

	// PostgreSQL/GORM
	PostgresHost         string   `env:"DB_POSTGRES_HOST" envDefault:"localhost"`
	PostgresPort         string   `env:"DB_POSTGRES_PORT" envDefault:"5432"`
	PostgresUser         string   `env:"DB_POSTGRES_USER" envDefault:"postgres"`
	PostgresPassword     string   `env:"DB_POSTGRES_PASSWORD"`
	PostgresDBName       string   `env:"DB_POSTGRES_DBNAME" envDefault:"kisanlink"`
	PostgresSSLMode      string   `env:"DB_POSTGRES_SSLMODE" envDefault:"disable"`
	PostgresMaxConns     int      `env:"DB_POSTGRES_MAX_CONNS" envDefault:"10"`
	PostgresIdleConns    int      `env:"DB_POSTGRES_IDLE_CONNS" envDefault:"5"`
	PostgresReadReplicas []string `env:"DB_POSTGRES_READ_REPLICAS" envSeparator:","`

	// DynamoDB
	DynamoDBRegion string `env:"DB_DYNAMO_REGION" envDefault:"us-east-1"`
	DynamoDBTable  string `env:"DB_DYNAMO_TABLE"`

	// SpiceDB
	SpiceDBEndpoint string `env:"DB_SPICEDB_ENDPOINT"`
	SpiceDBToken    string `env:"DB_SPICEDB_TOKEN"`

	// S3
	S3Region string `env:"DB_S3_REGION" envDefault:"us-east-1"`
	S3Bucket string `env:"DB_S3_BUCKET"`

	// Logging
	LogLevel string `env:"DB_LOG_LEVEL" envDefault:"info"`
}

// NewDatabaseManager creates a new database manager with environment-based configuration
func NewDatabaseManager() *DatabaseManager {
	config := loadConfigFromEnv()
	logger := createLogger(config.LogLevel)

	return &DatabaseManager{
		config: config,
		logger: logger,
	}
}

// NewDatabaseManagerWithConfig creates a new database manager with custom configuration
func NewDatabaseManagerWithConfig(config *Config) *DatabaseManager {
	logger := createLogger(config.LogLevel)

	return &DatabaseManager{
		config: config,
		logger: logger,
	}
}

// Connect establishes connections to all configured backends
func (dm *DatabaseManager) Connect(ctx context.Context) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	var errors []error

	// Connect to PostgreSQL if configured
	if dm.shouldConnectPostgres() {
		dm.postgresManager = NewPostgresManager(dm.config, dm.logger)
		if err := dm.postgresManager.Connect(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to connect to PostgreSQL: %w", err))
		} else {
			dm.logger.Info("connected to PostgreSQL")
		}
	}

	// Connect to DynamoDB if configured
	if dm.shouldConnectDynamo() {
		dm.dynamoManager = NewDynamoManager(dm.config, dm.logger)
		if err := dm.dynamoManager.Connect(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to connect to DynamoDB: %w", err))
		} else {
			dm.logger.Info("connected to DynamoDB")
		}
	}

	// Connect to SpiceDB if configured
	if dm.shouldConnectSpice() {
		dm.spiceManager = NewSpiceManager(dm.config, dm.logger)
		if err := dm.spiceManager.Connect(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to connect to SpiceDB: %w", err))
		} else {
			dm.logger.Info("connected to SpiceDB")
		}
	}

	// Connect to S3 if configured
	if dm.shouldConnectS3() {
		dm.s3Manager = NewS3Manager(dm.config, dm.logger)
		if err := dm.s3Manager.Connect(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to connect to S3: %w", err))
		} else {
			dm.logger.Info("connected to S3")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("connection errors: %v", errors)
	}

	return nil
}

// GetPostgresManager returns the PostgreSQL manager
func (dm *DatabaseManager) GetPostgresManager() *PostgresManager {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.postgresManager
}

// GetDynamoManager returns the DynamoDB manager
func (dm *DatabaseManager) GetDynamoManager() *DynamoManager {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.dynamoManager
}

// GetSpiceManager returns the SpiceDB manager
func (dm *DatabaseManager) GetSpiceManager() *SpiceManager {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.spiceManager
}

// GetS3Manager returns the S3 manager
func (dm *DatabaseManager) GetS3Manager() *S3Manager {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.s3Manager
}

// GetManager returns a database manager by backend type
func (dm *DatabaseManager) GetManager(backend BackendType) DBManager {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	switch backend {
	case BackendGorm:
		return dm.postgresManager
	case BackendDynamo:
		return dm.dynamoManager
	case BackendSpiceDB:
		return dm.spiceManager
	case BackendS3:
		return dm.s3Manager
	case BackendInMemory:
		return nil // In-memory doesn't have a manager yet
	default:
		return nil
	}
}

// GetAllManagers returns all connected database managers
func (dm *DatabaseManager) GetAllManagers() []DBManager {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	var managers []DBManager
	if dm.postgresManager != nil {
		managers = append(managers, dm.postgresManager)
	}
	if dm.dynamoManager != nil {
		managers = append(managers, dm.dynamoManager)
	}
	if dm.spiceManager != nil {
		managers = append(managers, dm.spiceManager)
	}
	if dm.s3Manager != nil {
		managers = append(managers, dm.s3Manager)
	}
	return managers
}

// Close closes all database connections
func (dm *DatabaseManager) Close() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	var errors []error

	if dm.postgresManager != nil {
		if err := dm.postgresManager.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close PostgreSQL: %w", err))
		}
	}

	if dm.dynamoManager != nil {
		if err := dm.dynamoManager.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close DynamoDB: %w", err))
		}
	}

	if dm.spiceManager != nil {
		if err := dm.spiceManager.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close SpiceDB: %w", err))
		}
	}

	if dm.s3Manager != nil {
		if err := dm.s3Manager.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close S3: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing connections: %v", errors)
	}

	return nil
}

// IsConnected checks if a specific backend is connected
func (dm *DatabaseManager) IsConnected(backend BackendType) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	switch backend {
	case BackendGorm:
		return dm.postgresManager != nil && dm.postgresManager.IsConnected()
	case BackendDynamo:
		return dm.dynamoManager != nil && dm.dynamoManager.IsConnected()
	case BackendSpiceDB:
		return dm.spiceManager != nil && dm.spiceManager.IsConnected()
	case BackendS3:
		return dm.s3Manager != nil && dm.s3Manager.IsConnected()
	case BackendInMemory:
		return true // In-memory is always available
	default:
		return false
	}
}

// shouldConnectPostgres determines if PostgreSQL should be connected
func (dm *DatabaseManager) shouldConnectPostgres() bool {
	return dm.config.PrimaryBackend == BackendGorm ||
		(dm.config.PostgresHost != "" && dm.config.PostgresPassword != "")
}

// shouldConnectDynamo determines if DynamoDB should be connected
func (dm *DatabaseManager) shouldConnectDynamo() bool {
	return dm.config.PrimaryBackend == BackendDynamo ||
		(dm.config.DynamoDBRegion != "" && dm.config.DynamoDBTable != "")
}

// shouldConnectSpice determines if SpiceDB should be connected
func (dm *DatabaseManager) shouldConnectSpice() bool {
	return dm.config.PrimaryBackend == BackendSpiceDB ||
		(dm.config.SpiceDBEndpoint != "" && dm.config.SpiceDBToken != "")
}

// shouldConnectS3 determines if S3 should be connected
func (dm *DatabaseManager) shouldConnectS3() bool {
	return dm.config.PrimaryBackend == BackendS3 ||
		dm.config.S3Bucket != ""
}

// loadConfigFromEnv loads configuration from environment variables
func loadConfigFromEnv() *Config {
	return &Config{
		PrimaryBackend:       BackendType(getEnv("DB_PRIMARY_BACKEND", "gorm")),
		PostgresHost:         getEnv("DB_POSTGRES_HOST", "localhost"),
		PostgresPort:         getEnv("DB_POSTGRES_PORT", "5432"),
		PostgresUser:         getEnv("DB_POSTGRES_USER", "postgres"),
		PostgresPassword:     getEnv("DB_POSTGRES_PASSWORD", ""),
		PostgresDBName:       getEnv("DB_POSTGRES_DBNAME", "kisanlink"),
		PostgresSSLMode:      getEnv("DB_POSTGRES_SSLMODE", "disable"),
		PostgresMaxConns:     getEnvAsInt("DB_POSTGRES_MAX_CONNS", 10),
		PostgresIdleConns:    getEnvAsInt("DB_POSTGRES_IDLE_CONNS", 5),
		PostgresReadReplicas: getEnvAsSlice("DB_POSTGRES_READ_REPLICAS", ","),
		DynamoDBRegion:       getEnv("DB_DYNAMO_REGION", "us-east-1"),
		DynamoDBTable:        getEnv("DB_DYNAMO_TABLE", ""),
		SpiceDBEndpoint:      getEnv("DB_SPICEDB_ENDPOINT", ""),
		SpiceDBToken:         getEnv("DB_SPICEDB_TOKEN", ""),
		S3Region:             getEnv("DB_S3_REGION", "us-east-1"),
		S3Bucket:             getEnv("DB_S3_BUCKET", ""),
		LogLevel:             getEnv("DB_LOG_LEVEL", "info"),
	}
}

// Helper functions for environment variable loading
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := parseInt(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsSlice(key, separator string) []string {
	if value := os.Getenv(key); value != "" {
		if value == "" {
			return []string{}
		}
		return strings.Split(value, separator)
	}
	return []string{}
}

func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

func createLogger(level string) *zap.Logger {
	config := zap.NewProductionConfig()

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
		// Fallback to default logger
		logger, _ = zap.NewProduction()
	}

	return logger
}
