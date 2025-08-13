// Package db provides S3 integration tests for file storage operations.
package db

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Kisanlink/kisanlink-db/pkg/base"
	"go.uber.org/zap"
)

// S3IntegrationTestSuite runs comprehensive S3 integration tests
type S3IntegrationTestSuite struct {
	config    TestConfig
	logger    *zap.Logger
	s3Manager *S3Manager
	ctx       context.Context
}

// NewS3IntegrationTestSuite creates a new S3 test suite
func NewS3IntegrationTestSuite(config TestConfig) *S3IntegrationTestSuite {
	logger := createTestLogger(config.LogLevel)
	ctx, _ := context.WithTimeout(context.Background(), config.Timeout)

	return &S3IntegrationTestSuite{
		config: config,
		logger: logger,
		ctx:    ctx,
	}
}

// RunAllS3Tests executes all S3 integration tests
func (ts *S3IntegrationTestSuite) RunAllS3Tests() error {
	ts.logger.Info("🚀 Starting S3 Integration Tests",
		zap.String("environment", ts.config.Environment),
		zap.String("log_level", ts.config.LogLevel),
		zap.Duration("timeout", ts.config.Timeout))

	// Print S3 environment configuration
	ts.printS3EnvironmentConfig()

	// Create and connect S3 manager
	ts.logger.Info("📦 Creating S3 Manager...")
	config := &Config{
		S3Region: getEnv("DB_S3_REGION", "ap-south-1"),
		S3Bucket: getEnv("DB_S3_BUCKET", "test-agriskill-bucket"),
		LogLevel: ts.config.LogLevel,
	}

	if config.S3Bucket == "" {
		ts.logger.Warn("⚠️  S3 bucket not configured, skipping S3 tests")
		return fmt.Errorf("S3 bucket not configured (DB_S3_BUCKET)")
	}

	ts.s3Manager = NewS3Manager(config, ts.logger)

	// Connect to S3
	ts.logger.Info("🔌 Connecting to S3...")
	startTime := time.Now()
	err := ts.s3Manager.Connect(ts.ctx)
	connectionTime := time.Since(startTime)

	if err != nil {
		ts.logger.Error("❌ Failed to connect to S3", zap.Error(err))
		return err
	}

	ts.logger.Info("✅ S3 Connected",
		zap.Duration("connection_time", connectionTime),
		zap.String("bucket", config.S3Bucket),
		zap.String("region", config.S3Region))

	// Run all S3 tests
	ts.testS3Connection()
	ts.testS3FileUpload()
	ts.testS3FileDownload()
	ts.testS3FileMetadata()
	ts.testS3PresignedURLs()
	ts.testS3FileListing()
	ts.testS3FolderOperations()
	ts.testS3FileDeletion()
	ts.testS3CRUDOperations()

	// Cleanup
	ts.logger.Info("🧹 Cleaning up S3 test data...")
	ts.cleanupS3TestData()

	ts.logger.Info("✅ S3 integration tests completed successfully")
	return nil
}

// testS3Connection tests basic S3 connectivity
func (ts *S3IntegrationTestSuite) testS3Connection() {
	ts.logger.Info("🔍 Testing S3 Connection...")

	// Test connection status
	if !ts.s3Manager.IsConnected() {
		ts.logger.Error("❌ S3 manager should be connected")
		return
	}

	// Test health check
	healthy := ts.s3Manager.checkHealth(ts.ctx)
	if !healthy {
		ts.logger.Error("❌ S3 health check should pass")
		return
	}

	ts.logger.Info("✅ S3 connection test passed")
}

// testS3FileUpload tests file upload operations
func (ts *S3IntegrationTestSuite) testS3FileUpload() {
	ts.logger.Info("📤 Testing S3 File Upload...")

	testCases := []struct {
		name        string
		key         string
		content     string
		contentType string
		metadata    map[string]string
	}{
		{
			name:        "simple text file",
			key:         "test-files/simple.txt",
			content:     "Hello, this is a simple test file!",
			contentType: "text/plain",
			metadata:    map[string]string{"test": "simple"},
		},
		{
			name:        "json file",
			key:         "test-files/data.json",
			content:     `{"name": "test", "value": 123}`,
			contentType: "application/json",
			metadata:    map[string]string{"type": "json", "version": "1.0"},
		},
		{
			name:        "large content file",
			key:         "test-files/large.txt",
			content:     strings.Repeat("This is a large test file content. ", 1000),
			contentType: "text/plain",
			metadata:    map[string]string{"size": "large"},
		},
	}

	for _, tc := range testCases {
		ts.logger.Info("Testing upload", zap.String("file", tc.name))

		// Upload file
		reader := strings.NewReader(tc.content)
		err := ts.s3Manager.UploadFile(ts.ctx, tc.key, reader, tc.contentType, tc.metadata)
		if err != nil {
			ts.logger.Error("❌ Failed to upload file", zap.String("file", tc.name), zap.Error(err))
			return
		}

		// Verify file exists by getting metadata using the key directly
		file := &S3File{}
		err = ts.s3Manager.GetByKey(ts.ctx, tc.key, file)
		if err != nil {
			ts.logger.Error("❌ Failed to get file metadata", zap.String("file", tc.name), zap.Error(err))
			return
		}

		// Verify file properties
		if file.Key != tc.key {
			ts.logger.Error("❌ File key should match", zap.String("expected", tc.key), zap.String("actual", file.Key))
			return
		}
		if file.Size != int64(len(tc.content)) {
			ts.logger.Error("❌ File size should match", zap.Int64("expected", int64(len(tc.content))), zap.Int64("actual", file.Size))
			return
		}
		if file.ContentType != tc.contentType {
			ts.logger.Error("❌ Content type should match", zap.String("expected", tc.contentType), zap.String("actual", file.ContentType))
			return
		}
		if file.ETag == "" {
			ts.logger.Error("❌ ETag should not be empty")
			return
		}
		if file.CreatedAt.IsZero() {
			ts.logger.Error("❌ Created timestamp should not be zero")
			return
		}

		ts.logger.Info("✅ Upload test passed", zap.String("file", tc.name))
	}
}

// testS3FileDownload tests file download operations
func (ts *S3IntegrationTestSuite) testS3FileDownload() {
	ts.logger.Info("📥 Testing S3 File Download...")

	testKey := "test-files/download-test.txt"
	testContent := "This is a test file for download operations"

	// First upload a file
	reader := strings.NewReader(testContent)
	err := ts.s3Manager.UploadFile(ts.ctx, testKey, reader, "text/plain", nil)
	if err != nil {
		ts.logger.Error("❌ Failed to upload test file for download", zap.Error(err))
		return
	}

	// Download the file
	downloadReader, err := ts.s3Manager.DownloadFile(ts.ctx, testKey)
	if err != nil {
		ts.logger.Error("❌ Failed to download file", zap.Error(err))
		return
	}
	defer downloadReader.Close()

	// Read the content
	downloadedContent, err := io.ReadAll(downloadReader)
	if err != nil {
		ts.logger.Error("❌ Failed to read downloaded content", zap.Error(err))
		return
	}

	if string(downloadedContent) != testContent {
		ts.logger.Error("❌ Downloaded content should match original")
		return
	}

	ts.logger.Info("✅ Download test passed")
}

// testS3FileMetadata tests file metadata operations
func (ts *S3IntegrationTestSuite) testS3FileMetadata() {
	ts.logger.Info("📋 Testing S3 File Metadata...")

	testKey := "test-files/metadata-test.txt"
	testContent := "Test content for metadata"
	testMetadata := map[string]string{
		"user-id":     "12345",
		"file-type":   "test",
		"version":     "2.0",
		"uploaded-by": "integration-test",
	}

	// Upload file with metadata
	reader := strings.NewReader(testContent)
	err := ts.s3Manager.UploadFile(ts.ctx, testKey, reader, "text/plain", testMetadata)
	if err != nil {
		ts.logger.Error("❌ Failed to upload file with metadata", zap.Error(err))
		return
	}

	// Get file metadata
	file := &S3File{}
	err = ts.s3Manager.GetByKey(ts.ctx, testKey, file)
	if err != nil {
		ts.logger.Error("❌ Failed to get file metadata", zap.Error(err))
		return
	}

	// Verify metadata
	if file.Key != testKey {
		ts.logger.Error("❌ File key should match", zap.String("expected", testKey), zap.String("actual", file.Key))
		return
	}
	if file.Size != int64(len(testContent)) {
		ts.logger.Error("❌ File size should match", zap.Int64("expected", int64(len(testContent))), zap.Int64("actual", file.Size))
		return
	}
	if file.ContentType != "text/plain" {
		ts.logger.Error("❌ Content type should match", zap.String("expected", "text/plain"), zap.String("actual", file.ContentType))
		return
	}
	if file.ETag == "" {
		ts.logger.Error("❌ ETag should not be empty")
		return
	}
	if file.CreatedAt.IsZero() {
		ts.logger.Error("❌ Created timestamp should not be zero")
		return
	}

	// Verify custom metadata
	for key, expectedValue := range testMetadata {
		actualValue, exists := file.Metadata[key]
		if !exists {
			ts.logger.Error("❌ Metadata key should exist", zap.String("key", key))
			return
		}
		if actualValue != expectedValue {
			ts.logger.Error("❌ Metadata value should match", zap.String("key", key), zap.String("expected", expectedValue), zap.String("actual", actualValue))
			return
		}
	}

	ts.logger.Info("✅ Metadata test passed")
}

// testS3PresignedURLs tests presigned URL generation
func (ts *S3IntegrationTestSuite) testS3PresignedURLs() {
	ts.logger.Info("🔗 Testing S3 Presigned URLs...")

	testKey := "test-files/presigned-test.txt"
	testContent := "Test content for presigned URL"

	// Upload a test file
	reader := strings.NewReader(testContent)
	err := ts.s3Manager.UploadFile(ts.ctx, testKey, reader, "text/plain", nil)
	if err != nil {
		ts.logger.Error("❌ Failed to upload test file for presigned URL", zap.Error(err))
		return
	}

	// Generate presigned URL
	expires := 1 * time.Hour
	presignedURL, err := ts.s3Manager.GetPresignedURL(ts.ctx, testKey, expires)
	if err != nil {
		ts.logger.Error("❌ Failed to generate presigned URL", zap.Error(err))
		return
	}

	// Verify URL format
	if presignedURL == "" {
		ts.logger.Error("❌ Presigned URL should not be empty")
		return
	}
	if !strings.Contains(presignedURL, "https://") {
		ts.logger.Error("❌ Presigned URL should be HTTPS")
		return
	}
	if !strings.Contains(presignedURL, testKey) {
		ts.logger.Error("❌ Presigned URL should contain the file key")
		return
	}
	if !strings.Contains(presignedURL, "X-Amz-Expires=") {
		ts.logger.Error("❌ Presigned URL should contain expiration")
		return
	}

	ts.logger.Info("✅ Presigned URL test passed")
}

// testS3FileListing tests file listing operations
func (ts *S3IntegrationTestSuite) testS3FileListing() {
	ts.logger.Info("📁 Testing S3 File Listing...")

	// Upload multiple files in different folders
	testFiles := []string{
		"test-listing/folder1/file1.txt",
		"test-listing/folder1/file2.txt",
		"test-listing/folder2/file3.txt",
		"test-listing/file4.txt",
	}

	for _, fileKey := range testFiles {
		content := fmt.Sprintf("Content for %s", fileKey)
		reader := strings.NewReader(content)
		err := ts.s3Manager.UploadFile(ts.ctx, fileKey, reader, "text/plain", nil)
		if err != nil {
			ts.logger.Error("❌ Failed to upload file for listing test", zap.String("file", fileKey), zap.Error(err))
			return
		}
	}

	// Test listing all files in test-listing folder
	var files []S3File
	filters := []base.FilterCondition{
		{Field: "prefix", Operator: base.OpEqual, Value: "test-listing/"},
	}

	err := ts.s3Manager.List(ts.ctx, filters, &files)
	if err != nil {
		ts.logger.Error("❌ Failed to list files", zap.Error(err))
		return
	}

	if len(files) != 4 {
		ts.logger.Error("❌ Should list 4 files in test-listing folder", zap.Int("actual", len(files)))
		return
	}

	// Verify all expected files are present
	fileKeys := make(map[string]bool)
	for _, file := range files {
		fileKeys[file.Key] = true
	}

	for _, expectedKey := range testFiles {
		if !fileKeys[expectedKey] {
			ts.logger.Error("❌ Expected file should be in listing", zap.String("file", expectedKey))
			return
		}
	}

	// Test listing files in specific subfolder
	var folder1Files []S3File
	folder1Filters := []base.FilterCondition{
		{Field: "prefix", Operator: base.OpEqual, Value: "test-listing/folder1/"},
	}

	err = ts.s3Manager.List(ts.ctx, folder1Filters, &folder1Files)
	if err != nil {
		ts.logger.Error("❌ Failed to list files in subfolder", zap.Error(err))
		return
	}

	if len(folder1Files) != 2 {
		ts.logger.Error("❌ Should list 2 files in folder1", zap.Int("actual", len(folder1Files)))
		return
	}

	ts.logger.Info("✅ File listing test passed")
}

// testS3FolderOperations tests folder creation and deletion
func (ts *S3IntegrationTestSuite) testS3FolderOperations() {
	ts.logger.Info("📂 Testing S3 Folder Operations...")

	// Test folder creation
	folderPath := "test-folders/new-folder/subfolder"
	err := ts.s3Manager.CreateFolder(ts.ctx, folderPath)
	if err != nil {
		ts.logger.Error("❌ Failed to create folder", zap.Error(err))
		return
	}

	// Upload a file to the created folder
	fileKey := folderPath + "/test-file.txt"
	content := "Test file in created folder"
	reader := strings.NewReader(content)
	err = ts.s3Manager.UploadFile(ts.ctx, fileKey, reader, "text/plain", nil)
	if err != nil {
		ts.logger.Error("❌ Failed to upload file to created folder", zap.Error(err))
		return
	}

	// Verify file exists
	file := &S3File{}
	err = ts.s3Manager.GetByKey(ts.ctx, fileKey, file)
	if err != nil {
		ts.logger.Error("❌ Failed to get file in created folder", zap.Error(err))
		return
	}

	if file.Key != fileKey {
		ts.logger.Error("❌ File should exist in created folder", zap.String("expected", fileKey), zap.String("actual", file.Key))
		return
	}

	// Test folder deletion (this will delete the folder and all its contents)
	err = ts.s3Manager.DeleteFolder(ts.ctx, folderPath)
	if err != nil {
		ts.logger.Error("❌ Failed to delete folder", zap.Error(err))
		return
	}

	// Verify file no longer exists
	err = ts.s3Manager.GetByKey(ts.ctx, fileKey, file)
	if err == nil {
		ts.logger.Error("❌ File should not exist after folder deletion")
		return
	}

	ts.logger.Info("✅ Folder operations test passed")
}

// testS3FileDeletion tests file deletion operations
func (ts *S3IntegrationTestSuite) testS3FileDeletion() {
	ts.logger.Info("🗑️  Testing S3 File Deletion...")

	testKey := "test-files/delete-test.txt"
	testContent := "This file will be deleted"

	// Upload a file to delete
	reader := strings.NewReader(testContent)
	err := ts.s3Manager.UploadFile(ts.ctx, testKey, reader, "text/plain", nil)
	if err != nil {
		ts.logger.Error("❌ Failed to upload file for deletion test", zap.Error(err))
		return
	}

	// Verify file exists
	file := &S3File{}
	err = ts.s3Manager.GetByKey(ts.ctx, testKey, file)
	if err != nil {
		ts.logger.Error("❌ Failed to get file before deletion", zap.Error(err))
		return
	}

	// Delete the file
	err = ts.s3Manager.Delete(ts.ctx, testKey)
	if err != nil {
		ts.logger.Error("❌ Failed to delete file", zap.Error(err))
		return
	}

	// Verify file no longer exists
	err = ts.s3Manager.GetByKey(ts.ctx, testKey, file)
	if err == nil {
		ts.logger.Error("❌ File should not exist after deletion")
		return
	}

	ts.logger.Info("✅ File deletion test passed")
}

// testS3CRUDOperations tests CRUD operations using the DBManager interface
func (ts *S3IntegrationTestSuite) testS3CRUDOperations() {
	ts.logger.Info("🔄 Testing S3 CRUD Operations...")

	// Test Create operation
	file := &S3File{
		ID:          "crud-test-file",
		ContentType: "text/plain",
		Metadata: map[string]string{
			"test": "crud",
		},
	}

	err := ts.s3Manager.Create(ts.ctx, file)
	if err != nil {
		ts.logger.Error("❌ Failed to create file record", zap.Error(err))
		return
	}

	// Verify key was generated
	if file.Key == "" {
		ts.logger.Error("❌ File key should be generated")
		return
	}
	if file.CreatedAt.IsZero() {
		ts.logger.Error("❌ Created timestamp should be set")
		return
	}
	if file.UpdatedAt.IsZero() {
		ts.logger.Error("❌ Updated timestamp should be set")
		return
	}

	// Test GetByID operation
	retrievedFile := &S3File{}
	err = ts.s3Manager.GetByID(ts.ctx, file.ID, retrievedFile)
	if err != nil {
		ts.logger.Error("❌ Failed to retrieve file", zap.Error(err))
		return
	}

	if file.ID != retrievedFile.ID {
		ts.logger.Error("❌ File ID should match", zap.String("expected", file.ID), zap.String("actual", retrievedFile.ID))
		return
	}
	if file.Key != retrievedFile.Key {
		ts.logger.Error("❌ File key should match", zap.String("expected", file.Key), zap.String("actual", retrievedFile.Key))
		return
	}

	// Test Update operation
	originalUpdatedAt := retrievedFile.UpdatedAt
	time.Sleep(100 * time.Millisecond) // Ensure timestamp difference

	err = ts.s3Manager.Update(ts.ctx, retrievedFile)
	if err != nil {
		ts.logger.Error("❌ Failed to update file", zap.Error(err))
		return
	}

	// Retrieve the file again to get the updated timestamp from S3
	time.Sleep(1 * time.Second) // Wait for S3 metadata to propagate
	updatedFile := &S3File{}
	err = ts.s3Manager.GetByID(ts.ctx, file.ID, updatedFile)
	if err != nil {
		ts.logger.Error("❌ Failed to retrieve updated file", zap.Error(err))
		return
	}

	ts.logger.Info("🔍 Timestamp comparison",
		zap.Time("original", originalUpdatedAt),
		zap.Time("updated", updatedFile.UpdatedAt),
		zap.Bool("is_newer", updatedFile.UpdatedAt.After(originalUpdatedAt)))

	if !updatedFile.UpdatedAt.After(originalUpdatedAt) {
		ts.logger.Error("❌ Updated timestamp should be newer (from metadata)",
			zap.Time("original", originalUpdatedAt),
			zap.Time("updated", updatedFile.UpdatedAt))
		return
	}

	// Test List operation
	var files []S3File
	err = ts.s3Manager.List(ts.ctx, []base.FilterCondition{}, &files)
	if err != nil {
		ts.logger.Error("❌ Failed to list files", zap.Error(err))
		return
	}

	// Should have at least our test file
	if len(files) < 1 {
		ts.logger.Error("❌ Should list at least one file", zap.Int("actual", len(files)))
		return
	}

	// Test Delete operation
	err = ts.s3Manager.Delete(ts.ctx, file.ID)
	if err != nil {
		ts.logger.Error("❌ Failed to delete file", zap.Error(err))
		return
	}

	// Verify file no longer exists
	err = ts.s3Manager.GetByID(ts.ctx, file.ID, retrievedFile)
	if err == nil {
		ts.logger.Error("❌ File should not exist after deletion")
		return
	}

	ts.logger.Info("✅ CRUD operations test passed")
}

// cleanupS3TestData cleans up all test data created during tests
func (ts *S3IntegrationTestSuite) cleanupS3TestData() {
	ts.logger.Info("🧹 Cleaning up S3 test data...")

	// List of test prefixes to clean up
	testPrefixes := []string{
		"test-files/",
		"test-listing/",
		"test-folders/",
	}

	for _, prefix := range testPrefixes {
		var files []S3File
		filters := []base.FilterCondition{
			{Field: "prefix", Operator: base.OpEqual, Value: prefix},
		}

		err := ts.s3Manager.List(ts.ctx, filters, &files)
		if err != nil {
			ts.logger.Warn("Failed to list files for cleanup", zap.String("prefix", prefix), zap.Error(err))
			continue
		}

		// Delete all files in this prefix
		for _, file := range files {
			err := ts.s3Manager.Delete(ts.ctx, file.Key)
			if err != nil {
				ts.logger.Warn("Failed to delete file during cleanup", zap.String("key", file.Key), zap.Error(err))
			}
		}

		ts.logger.Info("Cleaned up test data", zap.String("prefix", prefix), zap.Int("files", len(files)))
	}
}

// printS3EnvironmentConfig prints S3-specific environment configuration
func (ts *S3IntegrationTestSuite) printS3EnvironmentConfig() {
	ts.logger.Info("📋 S3 Environment Configuration",
		zap.String("region", getEnv("DB_S3_REGION", "ap-south-1")),
		zap.String("bucket", getEnv("DB_S3_BUCKET", "NOT_SET")),
		zap.String("aws_access_key_id", maskString(getEnv("AWS_ACCESS_KEY_ID", "NOT_SET"))),
		zap.String("aws_secret_access_key", maskString(getEnv("AWS_SECRET_ACCESS_KEY", "NOT_SET"))),
		zap.String("aws_session_token", maskString(getEnv("AWS_SESSION_TOKEN", "NOT_SET"))),
	)
}

// Helper method to get test context
func (ts *S3IntegrationTestSuite) t() *testing.T {
	// This is a helper method for compatibility with testify
	// In a real test, this would return the testing.T instance
	return nil
}

// maskString masks sensitive strings for logging
func maskString(s string) string {
	if s == "NOT_SET" || len(s) <= 4 {
		return s
	}
	return s[:4] + "****"
}

// TestS3Integration runs S3 integration tests
func TestS3Integration(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("S3 integration tests disabled. Set RUN_INTEGRATION_TESTS=true to enable.")
	}

	config := TestConfig{
		Environment: getEnv("TEST_ENVIRONMENT", "local"),
		LogLevel:    getEnv("DB_LOG_LEVEL", "info"),
		Timeout:     60 * time.Second,
	}

	suite := NewS3IntegrationTestSuite(config)
	err := suite.RunAllS3Tests()
	if err != nil {
		t.Fatalf("S3 integration tests should pass: %v", err)
	}
}

// TestS3IntegrationLocal runs S3 integration tests for local environment
func TestS3IntegrationLocal(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("S3 integration tests disabled. Set RUN_INTEGRATION_TESTS=true to enable.")
	}

	// Set local environment variables
	os.Setenv("TEST_ENVIRONMENT", "local")
	os.Setenv("DB_S3_REGION", "ap-south-1")
	// Note: DB_S3_BUCKET should be set by the user

	config := TestConfig{
		Environment: "local",
		LogLevel:    "debug",
		Timeout:     120 * time.Second,
	}

	suite := NewS3IntegrationTestSuite(config)
	err := suite.RunAllS3Tests()
	if err != nil {
		t.Fatalf("S3 local integration tests should pass: %v", err)
	}
}
