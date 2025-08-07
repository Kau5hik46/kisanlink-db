// Package base provides base models and interfaces for the application.
package base

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Kisanlink/kisanlink-db/pkg/core/hash"
	"gorm.io/gorm"
)

// Model defines the base struct that all models should embed
type Model struct {
	ID        string     `json:"id" gorm:"type:varchar(255);primary_key"`
	CreatedAt time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"not null"`
	CreatedBy string     `json:"created_by" gorm:"type:varchar(255)"`
	UpdatedBy string     `json:"updated_by" gorm:"type:varchar(255)"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`
	DeletedBy *string    `json:"deleted_by,omitempty" gorm:"type:varchar(255)"`
}

// ModelInterface defines the interface that all models should implement.
type ModelInterface interface {
	GetID() string
	SetID(string)
	GetCreatedAt() time.Time
	SetCreatedAt(time.Time)
	GetUpdatedAt() time.Time
	SetUpdatedAt(time.Time)
	GetCreatedBy() string
	SetCreatedBy(string)
	GetUpdatedBy() string
	SetUpdatedBy(string)
	GetDeletedAt() *time.Time
	SetDeletedAt(*time.Time)
	GetDeletedBy() *string
	SetDeletedBy(*string)
	IsDeleted() bool
	GetTableIdentifier() string
	GetTableSize() hash.TableSize
	BeforeCreate() error
	BeforeUpdate() error
	BeforeDelete() error
	BeforeSoftDelete() error
}

// BaseModel provides a default implementation of ModelInterface
type BaseModel struct {
	Model
	tableIdentifier string
	tableSize       hash.TableSize
}

// NewBaseModel creates a new BaseModel with initialized fields
func NewBaseModel(tableIdentifier string, tableSize hash.TableSize) *BaseModel {
	now := time.Now()

	// Generate ID using the provided table identifier
	id, err := hash.GenerateRandomID(tableIdentifier, tableSize)
	if err != nil {
		// If hash generation fails, create a simple timestamp-based ID
		id = fmt.Sprintf("%s%d", tableIdentifier, now.UnixNano())
	}

	return &BaseModel{
		Model: Model{
			ID:        id,
			CreatedAt: now,
			UpdatedAt: now,
		},
		tableIdentifier: tableIdentifier,
		tableSize:       tableSize,
	}
}

// GetID returns the model's ID
func (b *BaseModel) GetID() string {
	return b.ID
}

// SetID sets the model's ID
func (b *BaseModel) SetID(id string) {
	b.ID = id
}

// GetCreatedAt returns the creation timestamp
func (b *BaseModel) GetCreatedAt() time.Time {
	return b.CreatedAt
}

// SetCreatedAt sets the creation timestamp
func (b *BaseModel) SetCreatedAt(t time.Time) {
	b.CreatedAt = t
}

// GetUpdatedAt returns the last update timestamp
func (b *BaseModel) GetUpdatedAt() time.Time {
	return b.UpdatedAt
}

// SetUpdatedAt sets the last update timestamp
func (b *BaseModel) SetUpdatedAt(t time.Time) {
	b.UpdatedAt = t
}

// GetCreatedBy returns the user who created the record
func (b *BaseModel) GetCreatedBy() string {
	return b.CreatedBy
}

// SetCreatedBy sets the user who created the record
func (b *BaseModel) SetCreatedBy(userID string) {
	b.CreatedBy = userID
}

// GetUpdatedBy returns the user who last updated the record
func (b *BaseModel) GetUpdatedBy() string {
	return b.UpdatedBy
}

// SetUpdatedBy sets the user who last updated the record
func (b *BaseModel) SetUpdatedBy(userID string) {
	b.UpdatedBy = userID
}

// GetDeletedAt returns the deletion timestamp
func (b *BaseModel) GetDeletedAt() *time.Time {
	return b.DeletedAt
}

// SetDeletedAt sets the deletion timestamp
func (b *BaseModel) SetDeletedAt(t *time.Time) {
	b.DeletedAt = t
}

// GetDeletedBy returns the user who deleted the record
func (b *BaseModel) GetDeletedBy() *string {
	return b.DeletedBy
}

// SetDeletedBy sets the user who deleted the record
func (b *BaseModel) SetDeletedBy(userID *string) {
	b.DeletedBy = userID
}

// IsDeleted checks if the record is soft deleted
func (b *BaseModel) IsDeleted() bool {
	return b.DeletedAt != nil
}

// GetTableIdentifier returns the table identifier for this model
func (b *BaseModel) GetTableIdentifier() string {
	return b.tableIdentifier
}

// GetTableSize returns the table size for this model
func (b *BaseModel) GetTableSize() hash.TableSize {
	return b.tableSize
}

// BeforeCreate is called before creating a new record
func (b *BaseModel) BeforeCreate() error {
	now := time.Now()
	b.CreatedAt = now
	b.UpdatedAt = now
	return nil
}

// BeforeUpdate is called before updating an existing record
func (b *BaseModel) BeforeUpdate() error {
	b.UpdatedAt = time.Now()
	return nil
}

// BeforeDelete is called before hard deleting a record
func (b *BaseModel) BeforeDelete() error {
	// Default implementation does nothing
	return nil
}

// BeforeSoftDelete is called before soft deleting a record
func (b *BaseModel) BeforeSoftDelete() error {
	now := time.Now()
	b.DeletedAt = &now
	return nil
}

// GORM Hooks - These are for GORM compatibility
// BeforeCreateGORM is called by GORM before creating a new record
func (b *BaseModel) BeforeCreateGORM(tx *gorm.DB) error {
	return b.BeforeCreate()
}

// BeforeUpdateGORM is called by GORM before updating an existing record
func (b *BaseModel) BeforeUpdateGORM(tx *gorm.DB) error {
	return b.BeforeUpdate()
}

// BeforeDeleteGORM is called by GORM before hard deleting a record
func (b *BaseModel) BeforeDeleteGORM(tx *gorm.DB) error {
	return b.BeforeDelete()
}

// Repository defines the generic repository interface for CRUD operations
type Repository[T ModelInterface] interface {
	// Basic CRUD operations
	Create(ctx context.Context, model T) error
	GetByID(ctx context.Context, id string) (T, error)
	Update(ctx context.Context, model T) error
	Delete(ctx context.Context, id string) error
	SoftDelete(ctx context.Context, id string, deletedBy string) error
	Restore(ctx context.Context, id string) error

	// Query operations
	List(ctx context.Context, limit, offset int) ([]T, error)
	ListWithDeleted(ctx context.Context, limit, offset int) ([]T, error)
	Count(ctx context.Context) (int64, error)
	CountWithDeleted(ctx context.Context) (int64, error)
	Exists(ctx context.Context, id string) (bool, error)
	ExistsWithDeleted(ctx context.Context, id string) (bool, error)

	// Audit operations
	GetByCreatedBy(ctx context.Context, createdBy string, limit, offset int) ([]T, error)
	GetByUpdatedBy(ctx context.Context, updatedBy string, limit, offset int) ([]T, error)
	GetByDeletedBy(ctx context.Context, deletedBy string, limit, offset int) ([]T, error)

	// Bulk operations
	CreateMany(ctx context.Context, models []T) error
	UpdateMany(ctx context.Context, models []T) error
	DeleteMany(ctx context.Context, ids []string) error
	SoftDeleteMany(ctx context.Context, ids []string, deletedBy string) error
}

// BaseRepository provides a default implementation of Repository interface
type BaseRepository[T ModelInterface] struct {
	models map[string]T
	mu     sync.RWMutex
}

// NewBaseRepository creates a new base repository
func NewBaseRepository[T ModelInterface]() *BaseRepository[T] {
	return &BaseRepository[T]{
		models: make(map[string]T),
	}
}

// Create implements Repository.Create
func (r *BaseRepository[T]) Create(ctx context.Context, model T) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := model.BeforeCreate(); err != nil {
		return fmt.Errorf("before create hook failed: %w", err)
	}

	r.models[model.GetID()] = model
	return nil
}

// GetByID implements Repository.GetByID
func (r *BaseRepository[T]) GetByID(ctx context.Context, id string) (T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, exists := r.models[id]
	if !exists {
		var zero T
		return zero, fmt.Errorf("model with id %s not found", id)
	}

	// Check if soft deleted
	if model.IsDeleted() {
		var zero T
		return zero, fmt.Errorf("model with id %s is deleted", id)
	}

	return model, nil
}

// Update implements Repository.Update
func (r *BaseRepository[T]) Update(ctx context.Context, model T) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := model.BeforeUpdate(); err != nil {
		return fmt.Errorf("before update hook failed: %w", err)
	}

	r.models[model.GetID()] = model
	return nil
}

// Delete implements Repository.Delete (hard delete)
func (r *BaseRepository[T]) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	model, exists := r.models[id]
	if !exists {
		return fmt.Errorf("model with id %s not found", id)
	}

	if err := model.BeforeDelete(); err != nil {
		return fmt.Errorf("before delete hook failed: %w", err)
	}

	delete(r.models, id)
	return nil
}

// SoftDelete implements Repository.SoftDelete
func (r *BaseRepository[T]) SoftDelete(ctx context.Context, id string, deletedBy string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	model, exists := r.models[id]
	if !exists {
		return fmt.Errorf("model with id %s not found", id)
	}

	if err := model.BeforeSoftDelete(); err != nil {
		return fmt.Errorf("before soft delete hook failed: %w", err)
	}

	model.SetDeletedBy(&deletedBy)
	r.models[id] = model
	return nil
}

// Restore implements Repository.Restore
func (r *BaseRepository[T]) Restore(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	model, exists := r.models[id]
	if !exists {
		return fmt.Errorf("model with id %s not found", id)
	}

	model.SetDeletedAt(nil)
	model.SetDeletedBy(nil)
	r.models[id] = model
	return nil
}

// List implements Repository.List (excludes deleted records)
func (r *BaseRepository[T]) List(ctx context.Context, limit, offset int) ([]T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var models []T
	count := 0
	for _, model := range r.models {
		if !model.IsDeleted() {
			if count >= offset {
				models = append(models, model)
				if len(models) >= limit {
					break
				}
			}
			count++
		}
	}
	return models, nil
}

// ListWithDeleted implements Repository.ListWithDeleted (includes deleted records)
func (r *BaseRepository[T]) ListWithDeleted(ctx context.Context, limit, offset int) ([]T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var models []T
	count := 0
	for _, model := range r.models {
		if count >= offset {
			models = append(models, model)
			if len(models) >= limit {
				break
			}
		}
		count++
	}
	return models, nil
}

// Count implements Repository.Count (excludes deleted records)
func (r *BaseRepository[T]) Count(ctx context.Context) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int64
	for _, model := range r.models {
		if !model.IsDeleted() {
			count++
		}
	}
	return count, nil
}

// CountWithDeleted implements Repository.CountWithDeleted (includes deleted records)
func (r *BaseRepository[T]) CountWithDeleted(ctx context.Context) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return int64(len(r.models)), nil
}

// Exists implements Repository.Exists (excludes deleted records)
func (r *BaseRepository[T]) Exists(ctx context.Context, id string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, exists := r.models[id]
	if !exists {
		return false, nil
	}
	return !model.IsDeleted(), nil
}

// ExistsWithDeleted implements Repository.ExistsWithDeleted (includes deleted records)
func (r *BaseRepository[T]) ExistsWithDeleted(ctx context.Context, id string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.models[id]
	return exists, nil
}

// GetByCreatedBy implements Repository.GetByCreatedBy
func (r *BaseRepository[T]) GetByCreatedBy(ctx context.Context, createdBy string, limit, offset int) ([]T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var models []T
	count := 0
	for _, model := range r.models {
		if !model.IsDeleted() && model.GetCreatedBy() == createdBy {
			if count >= offset {
				models = append(models, model)
				if len(models) >= limit {
					break
				}
			}
			count++
		}
	}
	return models, nil
}

// GetByUpdatedBy implements Repository.GetByUpdatedBy
func (r *BaseRepository[T]) GetByUpdatedBy(ctx context.Context, updatedBy string, limit, offset int) ([]T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var models []T
	count := 0
	for _, model := range r.models {
		if !model.IsDeleted() && model.GetUpdatedBy() == updatedBy {
			if count >= offset {
				models = append(models, model)
				if len(models) >= limit {
					break
				}
			}
			count++
		}
	}
	return models, nil
}

// GetByDeletedBy implements Repository.GetByDeletedBy
func (r *BaseRepository[T]) GetByDeletedBy(ctx context.Context, deletedBy string, limit, offset int) ([]T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var models []T
	count := 0
	for _, model := range r.models {
		if model.IsDeleted() && model.GetDeletedBy() != nil && *model.GetDeletedBy() == deletedBy {
			if count >= offset {
				models = append(models, model)
				if len(models) >= limit {
					break
				}
			}
			count++
		}
	}
	return models, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CreateMany implements Repository.CreateMany with concurrent processing
func (r *BaseRepository[T]) CreateMany(ctx context.Context, models []T) error {
	if len(models) == 0 {
		return nil
	}

	// Use worker pool pattern for concurrent creation
	const maxWorkers = 10
	workerCount := min(maxWorkers, len(models))

	// Create channels for coordination
	jobs := make(chan T, len(models))
	results := make(chan error, len(models))

	// Start workers
	for i := 0; i < workerCount; i++ {
		go func() {
			for model := range jobs {
				if err := model.BeforeCreate(); err != nil {
					results <- fmt.Errorf("before create hook failed for model %s: %w", model.GetID(), err)
					continue
				}
				results <- nil
			}
		}()
	}

	// Send jobs
	for _, model := range models {
		jobs <- model
	}
	close(jobs)

	// Collect results and store models
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := 0; i < len(models); i++ {
		if err := <-results; err != nil {
			return err
		}
		// Store the model after successful validation
		r.models[models[i].GetID()] = models[i]
	}

	return nil
}

// UpdateMany implements Repository.UpdateMany with concurrent processing
func (r *BaseRepository[T]) UpdateMany(ctx context.Context, models []T) error {
	if len(models) == 0 {
		return nil
	}

	// Use worker pool pattern for concurrent updates
	const maxWorkers = 10
	workerCount := min(maxWorkers, len(models))

	// Create channels for coordination
	jobs := make(chan T, len(models))
	results := make(chan error, len(models))

	// Start workers
	for i := 0; i < workerCount; i++ {
		go func() {
			for model := range jobs {
				if err := model.BeforeUpdate(); err != nil {
					results <- fmt.Errorf("before update hook failed for model %s: %w", model.GetID(), err)
					continue
				}
				results <- nil
			}
		}()
	}

	// Send jobs
	for _, model := range models {
		jobs <- model
	}
	close(jobs)

	// Collect results and store models
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := 0; i < len(models); i++ {
		if err := <-results; err != nil {
			return err
		}
		// Store the model after successful validation
		r.models[models[i].GetID()] = models[i]
	}

	return nil
}

// DeleteMany implements Repository.DeleteMany with concurrent processing
func (r *BaseRepository[T]) DeleteMany(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// Use worker pool pattern for concurrent deletions
	const maxWorkers = 10
	workerCount := min(maxWorkers, len(ids))

	// Create channels for coordination
	jobs := make(chan string, len(ids))
	results := make(chan error, len(ids))

	// Start workers
	for i := 0; i < workerCount; i++ {
		go func() {
			for range jobs {
				// Note: We can't do the actual deletion in goroutines due to mutex requirements
				// This is just for validation
				results <- nil
			}
		}()
	}

	// Send jobs
	for _, id := range ids {
		jobs <- id
	}
	close(jobs)

	// Collect validation results
	for i := 0; i < len(ids); i++ {
		if err := <-results; err != nil {
			return err
		}
	}

	// Perform actual deletions with mutex protection
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, id := range ids {
		model, exists := r.models[id]
		if !exists {
			return fmt.Errorf("model with id %s not found", id)
		}

		if err := model.BeforeDelete(); err != nil {
			return fmt.Errorf("before delete hook failed for model %s: %w", id, err)
		}

		delete(r.models, id)
	}

	return nil
}

// SoftDeleteMany implements Repository.SoftDeleteMany with concurrent processing
func (r *BaseRepository[T]) SoftDeleteMany(ctx context.Context, ids []string, deletedBy string) error {
	if len(ids) == 0 {
		return nil
	}

	// Use worker pool pattern for concurrent soft deletions
	const maxWorkers = 10
	workerCount := min(maxWorkers, len(ids))

	// Create channels for coordination
	jobs := make(chan string, len(ids))
	results := make(chan error, len(ids))

	// Start workers
	for i := 0; i < workerCount; i++ {
		go func() {
			for range jobs {
				// Note: We can't do the actual soft deletion in goroutines due to mutex requirements
				// This is just for validation
				results <- nil
			}
		}()
	}

	// Send jobs
	for _, id := range ids {
		jobs <- id
	}
	close(jobs)

	// Collect validation results
	for i := 0; i < len(ids); i++ {
		if err := <-results; err != nil {
			return err
		}
	}

	// Perform actual soft deletions with mutex protection
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, id := range ids {
		model, exists := r.models[id]
		if !exists {
			return fmt.Errorf("model with id %s not found", id)
		}

		if err := model.BeforeSoftDelete(); err != nil {
			return fmt.Errorf("before soft delete hook failed for model %s: %w", id, err)
		}

		model.SetDeletedBy(&deletedBy)
		r.models[id] = model
	}

	return nil
}

// User represents a user in the system
type User struct {
	*BaseModel
	Name  string `json:"name" gorm:"not null"`
	Email string `json:"email" gorm:"unique;not null"`
}

// UserCreate represents the data needed to create a new user
type UserCreate struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// NewUser creates a new User with initialized fields
func NewUser(name, email string) *User {
	baseModel := NewBaseModel("USER", hash.Medium)
	return &User{
		BaseModel: baseModel,
		Name:      name,
		Email:     email,
	}
}

// BeforeCreate overrides BaseModel.BeforeCreate for User-specific logic
func (u *User) BeforeCreate() error {
	// Call parent implementation
	if err := u.BaseModel.BeforeCreate(); err != nil {
		return err
	}

	// Add user-specific validation
	if u.Name == "" {
		return fmt.Errorf("user name cannot be empty")
	}
	if u.Email == "" {
		return fmt.Errorf("user email cannot be empty")
	}

	return nil
}

// BeforeCreateGORM is called by GORM before creating a new record
func (u *User) BeforeCreateGORM(tx *gorm.DB) error {
	return u.BeforeCreate()
}

// BeforeUpdateGORM is called by GORM before updating an existing record
func (u *User) BeforeUpdateGORM(tx *gorm.DB) error {
	return u.BeforeUpdate()
}

// BeforeDeleteGORM is called by GORM before hard deleting a record
func (u *User) BeforeDeleteGORM(tx *gorm.DB) error {
	return u.BeforeDelete()
}

// UserRepository extends BaseFilterableRepository with User-specific methods
type UserRepository struct {
	*BaseFilterableRepository[*User]
}

// NewUserRepository creates a new user repository
func NewUserRepository() *UserRepository {
	return &UserRepository{
		BaseFilterableRepository: NewBaseFilterableRepository[*User](),
	}
}

// GetByEmail retrieves a user by email
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, user := range r.models {
		if !user.IsDeleted() && user.Email == email {
			return user, nil
		}
	}
	return nil, fmt.Errorf("user with email %s not found", email)
}

// Create overrides BaseRepository.Create to add email uniqueness check
func (r *UserRepository) Create(ctx context.Context, user *User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check email uniqueness
	for _, existingUser := range r.models {
		if !existingUser.IsDeleted() && existingUser.Email == user.Email {
			return fmt.Errorf("user with email %s already exists", user.Email)
		}
	}

	if err := user.BeforeCreate(); err != nil {
		return fmt.Errorf("before create hook failed: %w", err)
	}

	r.models[user.GetID()] = user
	return nil
}
