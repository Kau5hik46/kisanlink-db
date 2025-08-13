// Package db provides SpiceDB database connection management.
package db

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/Kisanlink/kisanlink-db/pkg/base"
	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// SpiceManager manages SpiceDB database connections
type SpiceManager struct {
	config     *Config
	client     *authzed.Client
	circuit    *gobreaker.CircuitBreaker
	logger     *zap.Logger
	mu         sync.RWMutex
	healthChan chan bool
}

// NewSpiceManager creates a new SpiceDB manager
func NewSpiceManager(config *Config, logger *zap.Logger) *SpiceManager {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "spicedb",
		MaxRequests: 5,
		Interval:    0,
		Timeout:     5 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Info("spicedb circuit breaker state changed",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	})

	return &SpiceManager{
		config:     config,
		circuit:    cb,
		logger:     logger,
		healthChan: make(chan bool, 1),
	}
}

// Connect establishes connection to SpiceDB
func (sm *SpiceManager) Connect(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.client != nil {
		return nil // Already connected
	}

	if sm.config.SpiceDBEndpoint == "" || sm.config.SpiceDBToken == "" {
		return fmt.Errorf("SpiceDB endpoint or token not configured")
	}

	// Create gRPC dial options with authentication
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			// Add authentication token to metadata
			md := metadata.New(map[string]string{
				"authorization": "Bearer " + sm.config.SpiceDBToken,
			})
			ctx = metadata.NewOutgoingContext(ctx, md)
			return invoker(ctx, method, req, reply, cc, opts...)
		}),
	}

	client, err := authzed.NewClient(
		sm.config.SpiceDBEndpoint,
		dialOpts...,
	)
	if err != nil {
		return fmt.Errorf("failed to connect to SpiceDB: %w", err)
	}

	sm.client = client

	// Verify connection by checking service health
	err = sm.verifyConnection(ctx)
	if err != nil {
		return fmt.Errorf("failed to verify SpiceDB connection: %w", err)
	}

	go sm.healthCheck(ctx)

	return nil
}

// verifyConnection verifies the SpiceDB connection
func (sm *SpiceManager) verifyConnection(ctx context.Context) error {
	// Simple health check - try to read the schema
	_, err := sm.client.ReadSchema(ctx, &v1.ReadSchemaRequest{})
	if err != nil {
		return fmt.Errorf("failed to read schema: %w", err)
	}
	return nil
}

// healthCheck periodically checks the health of SpiceDB
func (sm *SpiceManager) healthCheck(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			healthy := sm.checkHealth(ctx)
			select {
			case sm.healthChan <- healthy:
			default:
			}
		}
	}
}

// checkHealth verifies SpiceDB connectivity
func (sm *SpiceManager) checkHealth(ctx context.Context) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.client == nil {
		return false
	}

	// Simple health check by reading schema
	_, err := sm.client.ReadSchema(ctx, &v1.ReadSchemaRequest{})
	return err == nil
}

// GetClient returns the SpiceDB client
func (sm *SpiceManager) GetClient() *authzed.Client {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.client
}

// Close closes the SpiceDB connection
func (sm *SpiceManager) Close() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// SpiceDB client doesn't need explicit closing
	sm.client = nil
	return nil
}

// WithClient executes a function with the SpiceDB client
func (sm *SpiceManager) WithClient(ctx context.Context, fn func(client *authzed.Client) error) error {
	client := sm.GetClient()
	if client == nil {
		return fmt.Errorf("spicedb client not connected")
	}

	return fn(client)
}

// IsConnected checks if SpiceDB is connected
func (sm *SpiceManager) IsConnected() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.client != nil
}

// GetEndpoint returns the configured SpiceDB endpoint
func (sm *SpiceManager) GetEndpoint() string {
	return sm.config.SpiceDBEndpoint
}

// GetBackendType returns the type of backend this manager handles
func (sm *SpiceManager) GetBackendType() BackendType {
	return BackendSpiceDB
}

// Create creates a new relationship in SpiceDB
func (sm *SpiceManager) Create(ctx context.Context, model interface{}) error {
	client := sm.GetClient()
	if client == nil {
		return fmt.Errorf("spicedb client not connected")
	}

	// For SpiceDB, we'll assume the model contains relationship data
	relationship, ok := model.(*v1.Relationship)
	if !ok {
		return fmt.Errorf("model must be *v1.Relationship for SpiceDB")
	}

	_, err := client.WriteRelationships(ctx, &v1.WriteRelationshipsRequest{
		Updates: []*v1.RelationshipUpdate{
			{
				Operation:    v1.RelationshipUpdate_OPERATION_CREATE,
				Relationship: relationship,
			},
		},
	})

	return err
}

// GetByID retrieves a relationship by ID from SpiceDB
func (sm *SpiceManager) GetByID(ctx context.Context, id interface{}, model interface{}) error {
	client := sm.GetClient()
	if client == nil {
		return fmt.Errorf("spicedb client not connected")
	}

	// For SpiceDB, we'll assume the model is a relationship
	relationship, ok := model.(*v1.Relationship)
	if !ok {
		return fmt.Errorf("model must be *v1.Relationship for SpiceDB")
	}

	// Convert ID to string for resource ID
	resourceID, ok := id.(string)
	if !ok {
		return fmt.Errorf("id must be string for SpiceDB")
	}

	// Set the resource ID
	relationship.Resource.ObjectId = resourceID

	// Read the relationship
	_, err := client.ReadRelationships(ctx, &v1.ReadRelationshipsRequest{
		RelationshipFilter: &v1.RelationshipFilter{
			ResourceType:       relationship.Resource.ObjectType,
			OptionalResourceId: resourceID,
		},
	})

	return err
}

// Update updates a relationship in SpiceDB
func (sm *SpiceManager) Update(ctx context.Context, model interface{}) error {
	client := sm.GetClient()
	if client == nil {
		return fmt.Errorf("spicedb client not connected")
	}

	// For SpiceDB, we'll assume the model contains relationship data
	relationship, ok := model.(*v1.Relationship)
	if !ok {
		return fmt.Errorf("model must be *v1.Relationship for SpiceDB")
	}

	_, err := client.WriteRelationships(ctx, &v1.WriteRelationshipsRequest{
		Updates: []*v1.RelationshipUpdate{
			{
				Operation:    v1.RelationshipUpdate_OPERATION_TOUCH,
				Relationship: relationship,
			},
		},
	})

	return err
}

// Delete deletes a relationship from SpiceDB
func (sm *SpiceManager) Delete(ctx context.Context, id interface{}) error {
	client := sm.GetClient()
	if client == nil {
		return fmt.Errorf("spicedb client not connected")
	}

	// For SpiceDB, we need to construct a relationship to delete
	// This is a simplified implementation - in practice, you'd need more context
	resourceID, ok := id.(string)
	if !ok {
		return fmt.Errorf("id must be string for SpiceDB")
	}

	// Create a minimal relationship for deletion
	relationship := &v1.Relationship{
		Resource: &v1.ObjectReference{
			ObjectType: "unknown",
			ObjectId:   resourceID,
		},
		Relation: "unknown",
		Subject: &v1.SubjectReference{
			Object: &v1.ObjectReference{
				ObjectType: "unknown",
				ObjectId:   "unknown",
			},
		},
	}

	_, err := client.WriteRelationships(ctx, &v1.WriteRelationshipsRequest{
		Updates: []*v1.RelationshipUpdate{
			{
				Operation:    v1.RelationshipUpdate_OPERATION_DELETE,
				Relationship: relationship,
			},
		},
	})

	return err
}

// List retrieves relationships from SpiceDB based on filters
func (sm *SpiceManager) List(ctx context.Context, filters []base.FilterCondition, model interface{}) error {
	client := sm.GetClient()
	if client == nil {
		return fmt.Errorf("spicedb client not connected")
	}

	// For SpiceDB, we'll return an empty result as this is complex to implement
	// In practice, you'd need to convert filters to SpiceDB query format
	sm.logger.Warn("List operation not fully implemented for SpiceDB")

	// Set model to empty slice
	modelValue := reflect.ValueOf(model)
	if modelValue.Kind() == reflect.Ptr {
		modelValue = modelValue.Elem()
	}
	if modelValue.Kind() == reflect.Slice {
		modelValue.Set(reflect.MakeSlice(modelValue.Type(), 0, 0))
	}

	return nil
}

// AutoMigrateModels runs automigration for specific models (SpiceDB doesn't support schema migration)
func (sm *SpiceManager) AutoMigrateModels(ctx context.Context, models ...interface{}) error {
	// SpiceDB doesn't support schema migration like traditional databases
	// This is a no-op for SpiceDB
	sm.logger.Info("automigration skipped for SpiceDB (not supported)",
		zap.Int("model_count", len(models)))
	return nil
}
