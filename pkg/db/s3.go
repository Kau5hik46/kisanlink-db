// Package db provides S3 database connection management for file storage.
package db

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/cenkalti/backoff/v4"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
)

// S3Manager manages S3 bucket connections for file storage
type S3Manager struct {
	config     *Config
	client     *s3.Client
	circuit    *gobreaker.CircuitBreaker
	logger     *zap.Logger
	mu         sync.RWMutex
	healthChan chan bool
}

// S3File represents a file stored in S3
type S3File struct {
	ID          string            `json:"id"`
	Key         string            `json:"key"`
	Bucket      string            `json:"bucket"`
	Size        int64             `json:"size"`
	ContentType string            `json:"content_type"`
	ETag        string            `json:"etag"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// NewS3Manager creates a new S3 manager
func NewS3Manager(config *Config, logger *zap.Logger) *S3Manager {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "s3",
		MaxRequests: 5,
		Interval:    0,
		Timeout:     5 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Info("s3 circuit breaker state changed",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	})

	return &S3Manager{
		config:     config,
		circuit:    cb,
		logger:     logger,
		healthChan: make(chan bool, 1),
	}
}

// Connect establishes connection to S3
func (sm *S3Manager) Connect(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(sm.config.S3Region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	sm.client = s3.NewFromConfig(cfg)

	// Test connection by checking if bucket exists
	if err := sm.testConnection(ctx); err != nil {
		return fmt.Errorf("failed to test S3 connection: %w", err)
	}

	go sm.healthCheck(ctx)

	sm.logger.Info("connected to S3",
		zap.String("region", sm.config.S3Region),
		zap.String("bucket", sm.config.S3Bucket))

	return nil
}

// testConnection verifies the S3 connection by checking bucket existence
func (sm *S3Manager) testConnection(ctx context.Context) error {
	operation := func() error {
		_, err := sm.client.HeadBucket(ctx, &s3.HeadBucketInput{
			Bucket: aws.String(sm.config.S3Bucket),
		})
		return err
	}

	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.MaxElapsedTime = 30 * time.Second
	return backoff.Retry(operation, backoffConfig)
}

// healthCheck periodically checks the health of the S3 connection
func (sm *S3Manager) healthCheck(ctx context.Context) {
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

// checkHealth pings the S3 bucket to verify connectivity
func (sm *S3Manager) checkHealth(ctx context.Context) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.client == nil {
		return false
	}

	_, err := sm.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(sm.config.S3Bucket),
	})
	return err == nil
}

// Close closes the S3 connection
func (sm *S3Manager) Close() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.client = nil
	return nil
}

// IsConnected checks if the S3 connection is active
func (sm *S3Manager) IsConnected() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.client != nil
}

// GetBackendType returns the type of backend this manager handles
func (sm *S3Manager) GetBackendType() BackendType {
	return BackendS3
}

// Create uploads a file to S3
func (sm *S3Manager) Create(ctx context.Context, model interface{}) error {
	file, ok := model.(*S3File)
	if !ok {
		return fmt.Errorf("model must be of type *S3File")
	}

	// Generate key if not provided
	if file.Key == "" {
		file.Key = sm.generateKey(file.ID)
	}

	// Set bucket if not provided
	if file.Bucket == "" {
		file.Bucket = sm.config.S3Bucket
	}

	// Set timestamps
	now := time.Now()
	if file.CreatedAt.IsZero() {
		file.CreatedAt = now
	}
	file.UpdatedAt = now

	// Create an empty file with metadata
	emptyContent := ""
	reader := strings.NewReader(emptyContent)

	input := &s3.PutObjectInput{
		Bucket:      aws.String(file.Bucket),
		Key:         aws.String(file.Key),
		Body:        reader,
		ContentType: aws.String(file.ContentType),
	}

	if file.Metadata != nil {
		input.Metadata = file.Metadata
	}

	_, err := sm.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	return nil
}

// GetByID retrieves file metadata from S3
func (sm *S3Manager) GetByID(ctx context.Context, id interface{}, model interface{}) error {
	file, ok := model.(*S3File)
	if !ok {
		return fmt.Errorf("model must be of type *S3File")
	}

	key := sm.generateKey(fmt.Sprintf("%v", id))

	output, err := sm.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(sm.config.S3Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to get file metadata: %w", err)
	}

	file.ID = fmt.Sprintf("%v", id)
	file.Key = key
	file.Bucket = sm.config.S3Bucket
	if output.ContentLength != nil {
		file.Size = *output.ContentLength
	}
	file.ContentType = aws.ToString(output.ContentType)
	file.ETag = strings.Trim(aws.ToString(output.ETag), "\"")
	file.CreatedAt = aws.ToTime(output.LastModified)
	file.UpdatedAt = aws.ToTime(output.LastModified)
	file.Metadata = output.Metadata

	sm.logger.Debug("📖 GetByID: Retrieved file metadata",
		zap.String("key", file.Key),
		zap.Time("last_modified", file.UpdatedAt),
		zap.Any("metadata", file.Metadata))

	// Read updated_at from metadata if available
	if file.Metadata != nil {
		if updatedAtStr, exists := file.Metadata["updated_at"]; exists {
			if updatedAt, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
				file.UpdatedAt = updatedAt
			}
		}
	}

	return nil
}

// GetByKey retrieves file metadata from S3 using the key directly
func (sm *S3Manager) GetByKey(ctx context.Context, key string, model interface{}) error {
	file, ok := model.(*S3File)
	if !ok {
		return fmt.Errorf("model must be of type *S3File")
	}

	output, err := sm.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(sm.config.S3Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to get file metadata: %w", err)
	}

	file.ID = sm.extractIDFromKey(key)
	file.Key = key
	file.Bucket = sm.config.S3Bucket
	if output.ContentLength != nil {
		file.Size = *output.ContentLength
	}
	file.ContentType = aws.ToString(output.ContentType)
	file.ETag = strings.Trim(aws.ToString(output.ETag), "\"")
	file.CreatedAt = aws.ToTime(output.LastModified)
	file.UpdatedAt = aws.ToTime(output.LastModified)
	file.Metadata = output.Metadata

	// Read updated_at from metadata if available
	if file.Metadata != nil {
		if updatedAtStr, exists := file.Metadata["updated_at"]; exists {
			if updatedAt, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
				file.UpdatedAt = updatedAt
			}
		}
	}

	return nil
}

// Update updates file metadata in S3
func (sm *S3Manager) Update(ctx context.Context, model interface{}) error {
	file, ok := model.(*S3File)
	if !ok {
		return fmt.Errorf("model must be of type *S3File")
	}

	// Update the timestamp
	file.UpdatedAt = time.Now()

	// Add the updated timestamp to metadata so it gets stored in S3
	if file.Metadata == nil {
		file.Metadata = make(map[string]string)
	}
	// Store timestamp in UTC to match S3's LastModified format
	file.Metadata["updated_at"] = file.UpdatedAt.UTC().Format(time.RFC3339)

	sm.logger.Debug("🔄 Updating file metadata",
		zap.String("key", file.Key),
		zap.Time("updated_at", file.UpdatedAt),
		zap.String("updated_at_utc", file.Metadata["updated_at"]),
		zap.Any("all_metadata", file.Metadata))

	// Copy the object with updated metadata
	_, err := sm.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:            aws.String(sm.config.S3Bucket),
		CopySource:        aws.String(fmt.Sprintf("%s/%s", sm.config.S3Bucket, file.Key)),
		Key:               aws.String(file.Key),
		Metadata:          file.Metadata,
		MetadataDirective: types.MetadataDirectiveReplace,
	})
	if err != nil {
		return fmt.Errorf("failed to update file metadata: %w", err)
	}

	sm.logger.Debug("✅ CopyObject completed", zap.String("key", file.Key))

	return nil
}

// Delete removes a file from S3
func (sm *S3Manager) Delete(ctx context.Context, id interface{}) error {
	// Check if the id is already a key (contains path separators)
	idStr := fmt.Sprintf("%v", id)
	var key string

	if strings.Contains(idStr, "/") {
		// It's already a key, use it directly
		key = idStr
	} else {
		// It's an ID, generate the key
		key = sm.generateKey(idStr)
	}

	_, err := sm.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(sm.config.S3Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// List lists files in S3 with optional filters
func (sm *S3Manager) List(ctx context.Context, filters []Filter, model interface{}) error {
	files, ok := model.(*[]S3File)
	if !ok {
		return fmt.Errorf("model must be of type *[]S3File")
	}

	prefix := ""
	for _, filter := range filters {
		if filter.Field == "prefix" && filter.Operator == FilterOpEqual {
			if prefixStr, ok := filter.Value.(string); ok {
				prefix = prefixStr
			}
		}
	}

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(sm.config.S3Bucket),
	}
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}

	output, err := sm.client.ListObjectsV2(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to list files: %w", err)
	}

	*files = make([]S3File, 0, len(output.Contents))
	for _, obj := range output.Contents {
		size := int64(0)
		if obj.Size != nil {
			size = *obj.Size
		}
		file := S3File{
			ID:        sm.extractIDFromKey(aws.ToString(obj.Key)),
			Key:       aws.ToString(obj.Key),
			Bucket:    sm.config.S3Bucket,
			Size:      size,
			CreatedAt: aws.ToTime(obj.LastModified),
			UpdatedAt: aws.ToTime(obj.LastModified),
		}
		*files = append(*files, file)
	}

	return nil
}

// ApplyFilters applies filters to S3 queries (limited support)
func (sm *S3Manager) ApplyFilters(query interface{}, filters []Filter) (interface{}, error) {
	// S3 has limited filtering capabilities, mainly prefix-based
	// This is a simplified implementation
	return query, nil
}

// BuildFilter builds a filter for S3 queries
func (sm *S3Manager) BuildFilter(field string, operator FilterOperator, value interface{}) Filter {
	return Filter{
		Field:    field,
		Operator: operator,
		Value:    value,
	}
}

// UploadFile uploads a file to S3
func (sm *S3Manager) UploadFile(ctx context.Context, key string, reader io.Reader, contentType string, metadata map[string]string) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(sm.config.S3Bucket),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(contentType),
	}

	if metadata != nil {
		input.Metadata = metadata
	}

	_, err := sm.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	return nil
}

// DownloadFile downloads a file from S3
func (sm *S3Manager) DownloadFile(ctx context.Context, key string) (io.ReadCloser, error) {
	output, err := sm.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(sm.config.S3Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	return output.Body, nil
}

// GetPresignedURL generates a presigned URL for file access
func (sm *S3Manager) GetPresignedURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(sm.client)

	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(sm.config.S3Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return request.URL, nil
}

// generateKey generates an S3 key from an ID
func (sm *S3Manager) generateKey(id string) string {
	// Create a folder structure based on the ID
	// This helps with organization and performance
	hash := fmt.Sprintf("%x", id)
	if len(hash) >= 4 {
		return filepath.Join(hash[:2], hash[2:4], id)
	}
	return id
}

// extractIDFromKey extracts the ID from an S3 key
func (sm *S3Manager) extractIDFromKey(key string) string {
	parts := strings.Split(key, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return key
}

// CreateFolder creates a folder structure in S3
func (sm *S3Manager) CreateFolder(ctx context.Context, folderPath string) error {
	// S3 doesn't have real folders, but we can create an empty object with a trailing slash
	key := strings.TrimSuffix(folderPath, "/") + "/"

	_, err := sm.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(sm.config.S3Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to create folder: %w", err)
	}

	return nil
}

// DeleteFolder deletes a folder and all its contents from S3
func (sm *S3Manager) DeleteFolder(ctx context.Context, folderPath string) error {
	prefix := strings.TrimSuffix(folderPath, "/") + "/"

	// List all objects in the folder
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(sm.config.S3Bucket),
		Prefix: aws.String(prefix),
	}

	output, err := sm.client.ListObjectsV2(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to list folder contents: %w", err)
	}

	// Delete all objects in the folder
	if len(output.Contents) > 0 {
		var objects []types.ObjectIdentifier
		for _, obj := range output.Contents {
			objects = append(objects, types.ObjectIdentifier{
				Key: obj.Key,
			})
		}

		_, err = sm.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(sm.config.S3Bucket),
			Delete: &types.Delete{
				Objects: objects,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to delete folder contents: %w", err)
		}
	}

	return nil
}
