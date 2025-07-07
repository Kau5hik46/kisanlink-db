package base

import (
	"context"
	"testing"
	"time"

	"github.com/Kisanlink/kisanlink-db/pkg/core/hash"
)

func TestNewBaseModel(t *testing.T) {
	for _, tt := range NewBaseModelTests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewBaseModel(tt.tableIdentifier, tt.tableSize)

			if model == nil {
				t.Fatal("NewBaseModel returned nil")
			}

			if len(model.ID) != tt.expectedLength {
				t.Errorf("Expected ID length %d, got %d", tt.expectedLength, len(model.ID))
			}

			if model.GetTableIdentifier() != tt.tableIdentifier {
				t.Errorf("Expected table identifier %s, got %s", tt.tableIdentifier, model.GetTableIdentifier())
			}

			if model.GetTableSize() != tt.tableSize {
				t.Errorf("Expected table size %s, got %s", tt.tableSize, model.GetTableSize())
			}

			// Verify timestamps are set
			if model.CreatedAt.IsZero() {
				t.Error("CreatedAt should not be zero")
			}

			if model.UpdatedAt.IsZero() {
				t.Error("UpdatedAt should not be zero")
			}

			// Verify ID starts with table identifier
			if len(model.ID) < 4 || model.ID[:4] != tt.tableIdentifier {
				t.Errorf("Expected ID to start with %s, got %s", tt.tableIdentifier, model.ID)
			}
		})
	}
}

func TestBaseModelInterface(t *testing.T) {
	model := NewBaseModel("TEST", hash.Medium)

	// Test GetID/SetID
	if model.GetID() != model.ID {
		t.Errorf("GetID() returned %s, expected %s", model.GetID(), model.ID)
	}
	model.SetID("NEWID")
	if model.GetID() != "NEWID" {
		t.Errorf("SetID() failed, got %s, expected NEWID", model.GetID())
	}

	// Test GetCreatedAt/SetCreatedAt
	if model.GetCreatedAt() != model.CreatedAt {
		t.Errorf("GetCreatedAt() returned %v, expected %v", model.GetCreatedAt(), model.CreatedAt)
	}
	newTime := time.Now().Add(time.Hour)
	model.SetCreatedAt(newTime)
	if model.GetCreatedAt() != newTime {
		t.Errorf("SetCreatedAt() failed, got %v, expected %v", model.GetCreatedAt(), newTime)
	}

	// Test GetUpdatedAt/SetUpdatedAt
	if model.GetUpdatedAt() != model.UpdatedAt {
		t.Errorf("GetUpdatedAt() returned %v, expected %v", model.GetUpdatedAt(), model.UpdatedAt)
	}
	model.SetUpdatedAt(newTime)
	if model.GetUpdatedAt() != newTime {
		t.Errorf("SetUpdatedAt() failed, got %v, expected %v", model.GetUpdatedAt(), newTime)
	}

	// Test GetCreatedBy/SetCreatedBy
	model.SetCreatedBy("user123")
	if model.GetCreatedBy() != "user123" {
		t.Errorf("GetCreatedBy() failed, got %s, expected user123", model.GetCreatedBy())
	}

	// Test GetUpdatedBy/SetUpdatedBy
	model.SetUpdatedBy("user456")
	if model.GetUpdatedBy() != "user456" {
		t.Errorf("GetUpdatedBy() failed, got %s, expected user456", model.GetUpdatedBy())
	}

	// Test GetDeletedAt/SetDeletedAt
	if model.GetDeletedAt() != nil {
		t.Error("GetDeletedAt() should return nil for non-deleted model")
	}
	deletedTime := time.Now()
	model.SetDeletedAt(&deletedTime)
	if model.GetDeletedAt() == nil || *model.GetDeletedAt() != deletedTime {
		t.Errorf("SetDeletedAt() failed, got %v, expected %v", model.GetDeletedAt(), deletedTime)
	}

	// Test GetDeletedBy/SetDeletedBy
	if model.GetDeletedBy() != nil {
		t.Error("GetDeletedBy() should return nil for non-deleted model")
	}
	deletedBy := "user789"
	model.SetDeletedBy(&deletedBy)
	if model.GetDeletedBy() == nil || *model.GetDeletedBy() != deletedBy {
		t.Errorf("SetDeletedBy() failed, got %v, expected %s", model.GetDeletedBy(), deletedBy)
	}

	// Test IsDeleted
	if !model.IsDeleted() {
		t.Error("IsDeleted() should return true for deleted model")
	}
}

func TestBaseModelHooks(t *testing.T) {
	model := NewBaseModel("TEST", hash.Medium)
	originalCreatedAt := model.CreatedAt
	originalUpdatedAt := model.UpdatedAt

	// Test BeforeCreate
	time.Sleep(time.Millisecond) // Ensure time difference
	if err := model.BeforeCreate(); err != nil {
		t.Errorf("BeforeCreate() returned error: %v", err)
	}

	if model.CreatedAt.Equal(originalCreatedAt) {
		t.Error("BeforeCreate() should update CreatedAt")
	}

	if model.UpdatedAt.Equal(originalUpdatedAt) {
		t.Error("BeforeCreate() should update UpdatedAt")
	}

	// Test BeforeUpdate
	time.Sleep(time.Millisecond) // Ensure time difference
	beforeUpdateTime := model.UpdatedAt
	if err := model.BeforeUpdate(); err != nil {
		t.Errorf("BeforeUpdate() returned error: %v", err)
	}

	if model.UpdatedAt.Equal(beforeUpdateTime) {
		t.Error("BeforeUpdate() should update UpdatedAt")
	}

	// Test BeforeDelete
	if err := model.BeforeDelete(); err != nil {
		t.Errorf("BeforeDelete() returned error: %v", err)
	}

	// Test BeforeSoftDelete
	if err := model.BeforeSoftDelete(); err != nil {
		t.Errorf("BeforeSoftDelete() returned error: %v", err)
	}

	if !model.IsDeleted() {
		t.Error("BeforeSoftDelete() should mark model as deleted")
	}
}

func TestNewUser(t *testing.T) {
	name := "John Doe"
	email := "john@example.com"

	user := NewUser(name, email)

	if user == nil {
		t.Fatal("NewUser returned nil")
	}

	if user.Name != name {
		t.Errorf("Expected name %s, got %s", name, user.Name)
	}

	if user.Email != email {
		t.Errorf("Expected email %s, got %s", email, user.Email)
	}

	if user.GetTableIdentifier() != "USER" {
		t.Errorf("Expected table identifier USER, got %s", user.GetTableIdentifier())
	}

	if user.GetTableSize() != hash.Medium {
		t.Errorf("Expected table size Medium, got %s", user.GetTableSize())
	}

	// Verify ID starts with USER
	if len(user.ID) < 4 || user.ID[:4] != "USER" {
		t.Errorf("Expected ID to start with USER, got %s", user.ID)
	}
}

func TestUserBeforeCreate(t *testing.T) {
	for _, tt := range UserBeforeCreateTests {
		t.Run(tt.name, func(t *testing.T) {
			user := NewUser("", tt.email)
			if tt.name == "Valid user" {
				user.Name = "Test User"
			}

			err := user.BeforeCreate()

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestBaseRepository(t *testing.T) {
	repo := NewBaseRepository[*BaseModel]()
	ctx := context.Background()

	// Test Create
	model := NewBaseModel("TEST", hash.Medium)
	err := repo.Create(ctx, model)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Test GetByID
	retrieved, err := repo.GetByID(ctx, model.GetID())
	if err != nil {
		t.Fatalf("GetByID() failed: %v", err)
	}
	if retrieved.GetID() != model.GetID() {
		t.Errorf("GetByID() returned wrong model, expected %s, got %s", model.GetID(), retrieved.GetID())
	}

	// Test Exists
	exists, err := repo.Exists(ctx, model.GetID())
	if err != nil {
		t.Fatalf("Exists() failed: %v", err)
	}
	if !exists {
		t.Error("Exists() should return true for existing model")
	}

	// Test Count
	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count() failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Count() returned %d, expected 1", count)
	}

	// Test List
	models, err := repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if len(models) != 1 {
		t.Errorf("List() returned %d models, expected 1", len(models))
	}

	// Test Update
	model.SetUpdatedAt(time.Now())
	err = repo.Update(ctx, model)
	if err != nil {
		t.Fatalf("Update() failed: %v", err)
	}

	// Test Delete
	err = repo.Delete(ctx, model.GetID())
	if err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	// Verify deletion
	exists, err = repo.Exists(ctx, model.GetID())
	if err != nil {
		t.Fatalf("Exists() failed: %v", err)
	}
	if exists {
		t.Error("Exists() should return false for deleted model")
	}
}

func TestSoftDeleteOperations(t *testing.T) {
	repo := NewBaseRepository[*BaseModel]()
	ctx := context.Background()

	// Create a model
	model := NewBaseModel("TEST", hash.Medium)
	err := repo.Create(ctx, model)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Test SoftDelete
	deletedBy := "user123"
	err = repo.SoftDelete(ctx, model.GetID(), deletedBy)
	if err != nil {
		t.Fatalf("SoftDelete() failed: %v", err)
	}

	// Verify soft deletion
	exists, err := repo.Exists(ctx, model.GetID())
	if err != nil {
		t.Fatalf("Exists() failed: %v", err)
	}
	if exists {
		t.Error("Exists() should return false for soft deleted model")
	}

	// Test ExistsWithDeleted
	exists, err = repo.ExistsWithDeleted(ctx, model.GetID())
	if err != nil {
		t.Fatalf("ExistsWithDeleted() failed: %v", err)
	}
	if !exists {
		t.Error("ExistsWithDeleted() should return true for soft deleted model")
	}

	// Test Count vs CountWithDeleted
	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count() failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Count() returned %d, expected 0", count)
	}

	countWithDeleted, err := repo.CountWithDeleted(ctx)
	if err != nil {
		t.Fatalf("CountWithDeleted() failed: %v", err)
	}
	if countWithDeleted != 1 {
		t.Errorf("CountWithDeleted() returned %d, expected 1", countWithDeleted)
	}

	// Test Restore
	err = repo.Restore(ctx, model.GetID())
	if err != nil {
		t.Fatalf("Restore() failed: %v", err)
	}

	// Verify restoration
	exists, err = repo.Exists(ctx, model.GetID())
	if err != nil {
		t.Fatalf("Exists() failed: %v", err)
	}
	if !exists {
		t.Error("Exists() should return true for restored model")
	}
}

func TestAuditOperations(t *testing.T) {
	repo := NewBaseRepository[*BaseModel]()
	ctx := context.Background()

	// Create models with different audit fields
	model1 := CreateTestModelWithAudit("TEST", "user1", "user2")
	model2 := CreateTestModelWithAudit("TEST", "user1", "user3")
	model3 := CreateTestModelWithAudit("TEST", "user2", "user4")

	err := repo.Create(ctx, model1)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}
	err = repo.Create(ctx, model2)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}
	err = repo.Create(ctx, model3)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Test GetByCreatedBy
	models, err := repo.GetByCreatedBy(ctx, "user1", 10, 0)
	if err != nil {
		t.Fatalf("GetByCreatedBy() failed: %v", err)
	}
	if len(models) != 2 {
		t.Errorf("GetByCreatedBy() returned %d models, expected 2", len(models))
	}

	// Test GetByUpdatedBy
	models, err = repo.GetByUpdatedBy(ctx, "user3", 10, 0)
	if err != nil {
		t.Fatalf("GetByUpdatedBy() failed: %v", err)
	}
	if len(models) != 1 {
		t.Errorf("GetByUpdatedBy() returned %d models, expected 1", len(models))
	}

	// Test GetByDeletedBy
	err = repo.SoftDelete(ctx, model1.GetID(), "user5")
	if err != nil {
		t.Fatalf("SoftDelete() failed: %v", err)
	}

	models, err = repo.GetByDeletedBy(ctx, "user5", 10, 0)
	if err != nil {
		t.Fatalf("GetByDeletedBy() failed: %v", err)
	}
	if len(models) != 1 {
		t.Errorf("GetByDeletedBy() returned %d models, expected 1", len(models))
	}
}

func TestBulkOperations(t *testing.T) {
	for _, tt := range BulkOperationTests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewBaseRepository[*BaseModel]()
			ctx := context.Background()

			// Test CreateMany
			var models []*BaseModel
			for i := 0; i < tt.count; i++ {
				model := NewBaseModel("TEST", hash.Medium)
				models = append(models, model)
			}

			err := repo.CreateMany(ctx, models)
			if err != nil {
				t.Fatalf("CreateMany() failed: %v", err)
			}

			// Verify creation
			count, err := repo.Count(ctx)
			if err != nil {
				t.Fatalf("Count() failed: %v", err)
			}
			if count != int64(tt.count) {
				t.Errorf("Expected %d models, got %d", tt.count, count)
			}

			// Test UpdateMany
			for _, model := range models {
				model.SetUpdatedBy("bulk-updater")
			}
			err = repo.UpdateMany(ctx, models)
			if err != nil {
				t.Fatalf("UpdateMany() failed: %v", err)
			}

			// Test SoftDeleteMany
			var ids []string
			for _, model := range models {
				ids = append(ids, model.GetID())
			}
			err = repo.SoftDeleteMany(ctx, ids, "bulk-deleter")
			if err != nil {
				t.Fatalf("SoftDeleteMany() failed: %v", err)
			}

			// Verify soft deletion
			count, err = repo.Count(ctx)
			if err != nil {
				t.Fatalf("Count() failed: %v", err)
			}
			if count != 0 {
				t.Errorf("Expected 0 models after soft delete, got %d", count)
			}

			countWithDeleted, err := repo.CountWithDeleted(ctx)
			if err != nil {
				t.Fatalf("CountWithDeleted() failed: %v", err)
			}
			if countWithDeleted != int64(tt.count) {
				t.Errorf("Expected %d models with deleted, got %d", tt.count, countWithDeleted)
			}
		})
	}
}

func TestUserRepository(t *testing.T) {
	repo := NewUserRepository()
	ctx := context.Background()

	// Test Create
	user := NewUser("John Doe", "john@example.com")
	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Test GetByEmail
	retrieved, err := repo.GetByEmail(ctx, "john@example.com")
	if err != nil {
		t.Fatalf("GetByEmail() failed: %v", err)
	}
	if retrieved.Email != "john@example.com" {
		t.Errorf("GetByEmail() returned wrong user, expected %s, got %s", "john@example.com", retrieved.Email)
	}

	// Test email uniqueness
	duplicateUser := NewUser("Jane Doe", "john@example.com")
	err = repo.Create(ctx, duplicateUser)
	if err == nil {
		t.Error("Create() should fail for duplicate email")
	}

	// Test GetByEmail for non-existent email
	_, err = repo.GetByEmail(ctx, "nonexistent@example.com")
	if err == nil {
		t.Error("GetByEmail() should fail for non-existent email")
	}

	// Test soft delete with email uniqueness
	err = repo.SoftDelete(ctx, user.GetID(), "admin")
	if err != nil {
		t.Fatalf("SoftDelete() failed: %v", err)
	}

	// Should now be able to create user with same email
	err = repo.Create(ctx, duplicateUser)
	if err != nil {
		t.Errorf("Create() should succeed after soft delete, got error: %v", err)
	}
}

func TestRepositoryConcurrency(t *testing.T) {
	repo := NewBaseRepository[*BaseModel]()
	ctx := context.Background()
	numGoroutines := 100
	done := make(chan bool, numGoroutines)

	// Test concurrent creation
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			model := NewBaseModel("TEST", hash.Medium)
			err := repo.Create(ctx, model)
			if err != nil {
				t.Errorf("Concurrent Create() failed: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify all models were created
	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count() failed: %v", err)
	}
	if count != int64(numGoroutines) {
		t.Errorf("Expected %d models, got %d", numGoroutines, count)
	}
}

func TestRepositoryPagination(t *testing.T) {
	repo := NewBaseRepository[*BaseModel]()
	ctx := context.Background()

	// Create multiple models
	for i := 0; i < 10; i++ {
		model := NewBaseModel("TEST", hash.Medium)
		err := repo.Create(ctx, model)
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}
	}

	// Test pagination
	models, err := repo.List(ctx, 5, 0) // First 5
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if len(models) != 5 {
		t.Errorf("Expected 5 models, got %d", len(models))
	}

	models, err = repo.List(ctx, 5, 5) // Next 5
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if len(models) != 5 {
		t.Errorf("Expected 5 models, got %d", len(models))
	}

	models, err = repo.List(ctx, 5, 10) // Beyond available
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("Expected 0 models, got %d", len(models))
	}
}

func TestModelInterfaceCompliance(t *testing.T) {
	// Test that BaseModel implements ModelInterface
	var _ ModelInterface = (*BaseModel)(nil)

	// Test that User implements ModelInterface
	var _ ModelInterface = (*User)(nil)

	// Test that BaseRepository implements Repository
	var _ Repository[*BaseModel] = (*BaseRepository[*BaseModel])(nil)

	// Test that UserRepository implements Repository
	var _ Repository[*User] = (*UserRepository)(nil)
}
