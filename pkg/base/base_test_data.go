package base

import (
	"github.com/Kisanlink/kisanlink-db/pkg/core/hash"
)

// Test data for TestNewBaseModel
var NewBaseModelTests = []struct {
	name            string
	tableIdentifier string
	tableSize       hash.TableSize
	expectedPrefix  string
	expectedLength  int
}{
	{
		name:            "User model with Medium size",
		tableIdentifier: "USER",
		tableSize:       hash.Medium,
		expectedPrefix:  "USER",
		expectedLength:  12,
	},
	{
		name:            "Product model with Large size",
		tableIdentifier: "PROD",
		tableSize:       hash.Large,
		expectedPrefix:  "PROD",
		expectedLength:  14,
	},
	{
		name:            "Category model with Small size",
		tableIdentifier: "CATE",
		tableSize:       hash.Small,
		expectedPrefix:  "CATE",
		expectedLength:  10,
	},
}

// Test data for TestUserBeforeCreate
var UserBeforeCreateTests = []struct {
	name        string
	email       string
	shouldError bool
}{
	{
		name:        "Valid user",
		email:       "test@example.com",
		shouldError: false,
	},
	{
		name:        "Empty name",
		email:       "test@example.com",
		shouldError: true,
	},
	{
		name:        "Empty email",
		email:       "",
		shouldError: true,
	},
}

// Test data for audit field operations
var AuditFieldTests = []struct {
	name        string
	createdBy   string
	updatedBy   string
	deletedBy   string
	shouldError bool
}{
	{
		name:        "Valid audit fields",
		createdBy:   "user123",
		updatedBy:   "user456",
		deletedBy:   "user789",
		shouldError: false,
	},
	{
		name:        "Empty created by",
		createdBy:   "",
		updatedBy:   "user456",
		deletedBy:   "user789",
		shouldError: false,
	},
	{
		name:        "Empty updated by",
		createdBy:   "user123",
		updatedBy:   "",
		deletedBy:   "user789",
		shouldError: false,
	},
}

// Test data for soft delete operations
var SoftDeleteTests = []struct {
	name        string
	deletedBy   string
	shouldError bool
}{
	{
		name:        "Valid soft delete",
		deletedBy:   "user123",
		shouldError: false,
	},
	{
		name:        "Empty deleted by",
		deletedBy:   "",
		shouldError: false,
	},
}

// Test data for bulk operations
var BulkOperationTests = []struct {
	name        string
	count       int
	shouldError bool
}{
	{
		name:        "Small bulk operation",
		count:       5,
		shouldError: false,
	},
	{
		name:        "Medium bulk operation",
		count:       50,
		shouldError: false,
	},
	{
		name:        "Large bulk operation",
		count:       100,
		shouldError: false,
	},
}

// Helper function to create test models with audit fields
func CreateTestModelWithAudit(tableIdentifier string, createdBy, updatedBy string) *BaseModel {
	model := NewBaseModel(tableIdentifier, hash.Medium)
	model.SetCreatedBy(createdBy)
	model.SetUpdatedBy(updatedBy)
	return model
}

// Helper function to create test user with audit fields
func CreateTestUserWithAudit(name, email, createdBy, updatedBy string) *User {
	user := NewUser(name, email)
	user.SetCreatedBy(createdBy)
	user.SetUpdatedBy(updatedBy)
	return user
}

// Helper function to check if model has expected audit fields
func ValidateAuditFields(model ModelInterface, expectedCreatedBy, expectedUpdatedBy string) bool {
	return model.GetCreatedBy() == expectedCreatedBy &&
		model.GetUpdatedBy() == expectedUpdatedBy
}

// Helper function to check if model is soft deleted
func ValidateSoftDelete(model ModelInterface, expectedDeletedBy string) bool {
	if !model.IsDeleted() {
		return false
	}

	deletedBy := model.GetDeletedBy()
	if deletedBy == nil {
		return expectedDeletedBy == ""
	}

	return *deletedBy == expectedDeletedBy
}
