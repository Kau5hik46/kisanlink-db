package base

import (
	"context"
	"testing"
	"time"

	"github.com/Kisanlink/kisanlink-db/pkg/core/hash"
)

func TestFilterBuilder(t *testing.T) {
	t.Run("Basic filter building", func(t *testing.T) {
		filter := NewFilterBuilder().
			Where("name", OpEqual, "John").
			Where("email", OpContains, "example").
			Sort("created_at", "desc").
			Page(1, 10).
			Build()

		if len(filter.Group.Conditions) != 2 {
			t.Errorf("Expected 2 conditions, got %d", len(filter.Group.Conditions))
		}

		if len(filter.Sort) != 1 {
			t.Errorf("Expected 1 sort field, got %d", len(filter.Sort))
		}

		if filter.Page != 1 || filter.PageSize != 10 {
			t.Errorf("Expected page 1, size 10, got page %d, size %d", filter.Page, filter.PageSize)
		}
	})

	t.Run("Complex filter with groups", func(t *testing.T) {
		filter := NewFilterBuilder().
			Where("is_deleted", OpEqual, false).
			Or(
				FilterCondition{Field: "created_by", Operator: OpEqual, Value: "admin123"},
				FilterCondition{Field: "updated_by", Operator: OpEqual, Value: "admin123"},
			).
			And(
				FilterCondition{Field: "name", Operator: OpStartsWith, Value: "John"},
			).
			Build()

		if len(filter.Group.Conditions) != 1 {
			t.Errorf("Expected 1 condition in main group, got %d", len(filter.Group.Conditions))
		}

		if len(filter.Group.Groups) != 2 {
			t.Errorf("Expected 2 sub-groups, got %d", len(filter.Group.Groups))
		}
	})

	t.Run("Filter with special operators", func(t *testing.T) {
		filter := NewFilterBuilder().
			WhereIn("status", []interface{}{"active", "pending"}).
			WhereNull("deleted_at").
			WhereBetween("created_at", time.Now().AddDate(0, -1, 0), time.Now()).
			Build()

		if len(filter.Group.Conditions) != 3 {
			t.Errorf("Expected 3 conditions, got %d", len(filter.Group.Conditions))
		}
	})
}

func TestFilterEvaluator(t *testing.T) {
	evaluator := NewFilterEvaluator()

	t.Run("Basic field evaluation", func(t *testing.T) {
		model := NewBaseModel("TEST", hash.Medium)
		model.SetCreatedBy("admin123")
		model.SetUpdatedBy("user456")

		// Test ID field
		filter := &Filter{
			Group: FilterGroup{
				Logic: LogicAnd,
				Conditions: []FilterCondition{
					{Field: "id", Operator: OpEqual, Value: model.GetID()},
				},
			},
		}

		matches, err := evaluator.Evaluate(filter, model)
		if err != nil {
			t.Fatalf("Evaluation failed: %v", err)
		}
		if !matches {
			t.Error("Expected model to match filter")
		}
	})

	t.Run("String operators", func(t *testing.T) {
		user := NewUser("John Doe", "john@example.com")

		tests := []struct {
			name      string
			condition FilterCondition
			expected  bool
		}{
			{
				name: "Contains operator",
				condition: FilterCondition{
					Field:    "email",
					Operator: OpContains,
					Value:    "example",
				},
				expected: true,
			},
			{
				name: "Starts with operator",
				condition: FilterCondition{
					Field:    "name",
					Operator: OpStartsWith,
					Value:    "John",
				},
				expected: true,
			},
			{
				name: "Ends with operator",
				condition: FilterCondition{
					Field:    "email",
					Operator: OpEndsWith,
					Value:    ".com",
				},
				expected: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				filter := &Filter{
					Group: FilterGroup{
						Logic:      LogicAnd,
						Conditions: []FilterCondition{tt.condition},
					},
				}

				matches, err := evaluator.Evaluate(filter, user)
				if err != nil {
					t.Fatalf("Evaluation failed: %v", err)
				}
				if matches != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, matches)
				}
			})
		}
	})

	t.Run("Comparison operators", func(t *testing.T) {
		model := NewBaseModel("TEST", hash.Medium)
		model.SetCreatedAt(time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC))

		tests := []struct {
			name      string
			condition FilterCondition
			expected  bool
		}{
			{
				name: "Greater than",
				condition: FilterCondition{
					Field:    "created_at",
					Operator: OpGreaterThan,
					Value:    time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				expected: true,
			},
			{
				name: "Less than",
				condition: FilterCondition{
					Field:    "created_at",
					Operator: OpLessThan,
					Value:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				expected: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				filter := &Filter{
					Group: FilterGroup{
						Logic:      LogicAnd,
						Conditions: []FilterCondition{tt.condition},
					},
				}

				matches, err := evaluator.Evaluate(filter, model)
				if err != nil {
					t.Fatalf("Evaluation failed: %v", err)
				}
				if matches != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, matches)
				}
			})
		}
	})

	t.Run("Null operators", func(t *testing.T) {
		model := NewBaseModel("TEST", hash.Medium)

		// Test IS NULL
		filter := &Filter{
			Group: FilterGroup{
				Logic: LogicAnd,
				Conditions: []FilterCondition{
					{Field: "deleted_at", Operator: OpIsNull},
				},
			},
		}

		matches, err := evaluator.Evaluate(filter, model)
		if err != nil {
			t.Fatalf("Evaluation failed: %v", err)
		}
		if !matches {
			t.Error("Expected model to match IS NULL filter")
		}

		// Test IS NOT NULL
		filter = &Filter{
			Group: FilterGroup{
				Logic: LogicAnd,
				Conditions: []FilterCondition{
					{Field: "created_at", Operator: OpIsNotNull},
				},
			},
		}

		matches, err = evaluator.Evaluate(filter, model)
		if err != nil {
			t.Fatalf("Evaluation failed: %v", err)
		}
		if !matches {
			t.Error("Expected model to match IS NOT NULL filter")
		}
	})

	t.Run("IN operator", func(t *testing.T) {
		model := NewBaseModel("TEST", hash.Medium)
		model.SetCreatedBy("admin123")

		filter := &Filter{
			Group: FilterGroup{
				Logic: LogicAnd,
				Conditions: []FilterCondition{
					{
						Field:    "created_by",
						Operator: OpIn,
						Value:    []interface{}{"admin123", "user456"},
					},
				},
			},
		}

		matches, err := evaluator.Evaluate(filter, model)
		if err != nil {
			t.Fatalf("Evaluation failed: %v", err)
		}
		if !matches {
			t.Error("Expected model to match IN filter")
		}
	})

	t.Run("Complex logical operations", func(t *testing.T) {
		model := NewBaseModel("TEST", hash.Medium)
		model.SetCreatedBy("admin123")
		model.SetUpdatedBy("user456")

		// Test AND logic
		filter := &Filter{
			Group: FilterGroup{
				Logic: LogicAnd,
				Conditions: []FilterCondition{
					{Field: "created_by", Operator: OpEqual, Value: "admin123"},
					{Field: "updated_by", Operator: OpEqual, Value: "user456"},
				},
			},
		}

		matches, err := evaluator.Evaluate(filter, model)
		if err != nil {
			t.Fatalf("Evaluation failed: %v", err)
		}
		if !matches {
			t.Error("Expected model to match AND filter")
		}

		// Test OR logic
		filter = &Filter{
			Group: FilterGroup{
				Logic: LogicOr,
				Conditions: []FilterCondition{
					{Field: "created_by", Operator: OpEqual, Value: "admin123"},
					{Field: "created_by", Operator: OpEqual, Value: "user999"},
				},
			},
		}

		matches, err = evaluator.Evaluate(filter, model)
		if err != nil {
			t.Fatalf("Evaluation failed: %v", err)
		}
		if !matches {
			t.Error("Expected model to match OR filter")
		}
	})
}

func TestBaseFilterableRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("Basic filtering", func(t *testing.T) {
		repo := NewBaseFilterableRepository[*BaseModel]()

		// Create test models
		models := CreateTestModelsForFiltering()
		for _, model := range models {
			err := repo.Create(ctx, model)
			if err != nil {
				t.Fatalf("Failed to create model: %v", err)
			}
		}

		// Test simple filter
		filter := NewFilterBuilder().
			Where("created_by", OpEqual, "admin123").
			Where("is_deleted", OpEqual, false).
			Build()

		results, err := repo.Find(ctx, filter)
		if err != nil {
			t.Fatalf("Find failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}
	})

	t.Run("Complex filtering with groups", func(t *testing.T) {
		repo := NewBaseFilterableRepository[*BaseModel]()

		// Create test models
		models := CreateTestModelsForFiltering()
		for _, model := range models {
			err := repo.Create(ctx, model)
			if err != nil {
				t.Fatalf("Failed to create model: %v", err)
			}
		}

		// Test complex filter with OR group
		filter := NewFilterBuilder().
			Where("is_deleted", OpEqual, false).
			Or(
				FilterCondition{Field: "created_by", Operator: OpEqual, Value: "admin123"},
				FilterCondition{Field: "created_by", Operator: OpEqual, Value: "user456"},
			).
			Build()

		results, err := repo.Find(ctx, filter)
		if err != nil {
			t.Fatalf("Find failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}
	})

	t.Run("Pagination", func(t *testing.T) {
		repo := NewBaseFilterableRepository[*BaseModel]()

		// Create multiple test models
		for i := 0; i < 10; i++ {
			model := NewBaseModel("TEST", hash.Medium)
			model.SetCreatedBy("user123")
			err := repo.Create(ctx, model)
			if err != nil {
				t.Fatalf("Failed to create model: %v", err)
			}
		}

		// Test pagination
		filter := NewFilterBuilder().
			Where("is_deleted", OpEqual, false).
			Page(1, 5).
			Build()

		results, err := repo.Find(ctx, filter)
		if err != nil {
			t.Fatalf("Find failed: %v", err)
		}

		if len(results) != 5 {
			t.Errorf("Expected 5 results, got %d", len(results))
		}

		// Test second page
		filter = NewFilterBuilder().
			Where("is_deleted", OpEqual, false).
			Page(2, 5).
			Build()

		results, err = repo.Find(ctx, filter)
		if err != nil {
			t.Fatalf("Find failed: %v", err)
		}

		if len(results) != 5 {
			t.Errorf("Expected 5 results on second page, got %d", len(results))
		}
	})

	t.Run("Count with filter", func(t *testing.T) {
		repo := NewBaseFilterableRepository[*BaseModel]()

		// Create test models
		models := CreateTestModelsForFiltering()
		for _, model := range models {
			err := repo.Create(ctx, model)
			if err != nil {
				t.Fatalf("Failed to create model: %v", err)
			}
		}

		// Test count with filter
		filter := NewFilterBuilder().
			Where("created_by", OpEqual, "admin123").
			Where("is_deleted", OpEqual, false).
			Build()

		count, err := repo.CountWithFilter(ctx, filter)
		if err != nil {
			t.Fatalf("CountWithFilter failed: %v", err)
		}

		if count != 1 {
			t.Errorf("Expected count 1, got %d", count)
		}
	})

	t.Run("FindOne", func(t *testing.T) {
		repo := NewBaseFilterableRepository[*BaseModel]()

		// Create test models
		models := CreateTestModelsForFiltering()
		for _, model := range models {
			err := repo.Create(ctx, model)
			if err != nil {
				t.Fatalf("Failed to create model: %v", err)
			}
		}

		// Test FindOne
		filter := NewFilterBuilder().
			Where("created_by", OpEqual, "admin123").
			Where("is_deleted", OpEqual, false).
			Build()

		result, err := repo.FindOne(ctx, filter)
		if err != nil {
			t.Fatalf("FindOne failed: %v", err)
		}

		if result.GetCreatedBy() != "admin123" {
			t.Errorf("Expected created_by to be admin123, got %s", result.GetCreatedBy())
		}

		// Test FindOne with no results
		filter = NewFilterBuilder().
			Where("created_by", OpEqual, "nonexistent").
			Build()

		_, err = repo.FindOne(ctx, filter)
		if err == nil {
			t.Error("Expected error when no results found")
		}
	})

	t.Run("Include deleted records", func(t *testing.T) {
		repo := NewBaseFilterableRepository[*BaseModel]()

		// Create test models including deleted ones
		models := CreateTestModelsForFiltering()
		for _, model := range models {
			err := repo.Create(ctx, model)
			if err != nil {
				t.Fatalf("Failed to create model: %v", err)
			}
		}

		// Test filter that includes deleted records
		filter := NewFilterBuilder().
			Where("is_deleted", OpEqual, true).
			Build()

		results, err := repo.Find(ctx, filter)
		if err != nil {
			t.Fatalf("Find failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 deleted result, got %d", len(results))
		}
	})
}

func TestUserRepositoryWithFilters(t *testing.T) {
	ctx := context.Background()

	t.Run("User-specific filtering", func(t *testing.T) {
		repo := NewUserRepository()

		// Create test users
		users := CreateTestUsersForFiltering()
		for _, user := range users {
			err := repo.Create(ctx, user)
			if err != nil {
				t.Fatalf("Failed to create user: %v", err)
			}
		}

		// Test filter by email
		filter := NewFilterBuilder().
			Where("email", OpContains, "example").
			Where("is_deleted", OpEqual, false).
			Build()

		results, err := repo.Find(ctx, filter)
		if err != nil {
			t.Fatalf("Find failed: %v", err)
		}

		if len(results) != 3 {
			t.Errorf("Expected 3 results, got %d", len(results))
		}

		// Test filter by name
		filter = NewFilterBuilder().
			Where("name", OpStartsWith, "John").
			Where("is_deleted", OpEqual, false).
			Build()

		results, err = repo.Find(ctx, filter)
		if err != nil {
			t.Fatalf("Find failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}

		if results[0].Name != "John Doe" {
			t.Errorf("Expected John Doe, got %s", results[0].Name)
		}
	})

	t.Run("Combined user and base filtering", func(t *testing.T) {
		repo := NewUserRepository()

		// Create test users
		users := CreateTestUsersForFiltering()
		for _, user := range users {
			err := repo.Create(ctx, user)
			if err != nil {
				t.Fatalf("Failed to create user: %v", err)
			}
		}

		// Test complex filter combining user fields and base fields
		filter := NewFilterBuilder().
			Where("is_deleted", OpEqual, false).
			Or(
				FilterCondition{Field: "created_by", Operator: OpEqual, Value: "admin123"},
				FilterCondition{Field: "email", Operator: OpContains, Value: "jane"},
			).
			Build()

		results, err := repo.Find(ctx, filter)
		if err != nil {
			t.Fatalf("Find failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}
	})
}

func TestFilterIntegration(t *testing.T) {
	t.Run("Filter builder integration", func(t *testing.T) {
		// Test that filter builder creates valid filters
		filter := NewFilterBuilder().
			Where("name", OpEqual, "test").
			Where("email", OpContains, "example").
			Sort("created_at", "desc").
			Page(1, 10).
			Build()

		if filter.Group.Logic != LogicAnd {
			t.Errorf("Expected LogicAnd, got %s", filter.Group.Logic)
		}

		if len(filter.Group.Conditions) != 2 {
			t.Errorf("Expected 2 conditions, got %d", len(filter.Group.Conditions))
		}

		if len(filter.Sort) != 1 {
			t.Errorf("Expected 1 sort field, got %d", len(filter.Sort))
		}
	})

	t.Run("Filter evaluator integration", func(t *testing.T) {
		evaluator := NewFilterEvaluator()
		model := NewBaseModel("TEST", hash.Medium)
		model.SetCreatedBy("admin123")

		filter := NewFilterBuilder().
			Where("created_by", OpEqual, "admin123").
			Build()

		matches, err := evaluator.Evaluate(filter, model)
		if err != nil {
			t.Fatalf("Evaluation failed: %v", err)
		}
		if !matches {
			t.Error("Expected model to match filter")
		}
	})

	t.Run("Repository integration", func(t *testing.T) {
		repo := NewBaseFilterableRepository[*BaseModel]()
		ctx := context.Background()

		model := NewBaseModel("TEST", hash.Medium)
		model.SetCreatedBy("admin123")
		err := repo.Create(ctx, model)
		if err != nil {
			t.Fatalf("Failed to create model: %v", err)
		}

		filter := NewFilterBuilder().
			Where("created_by", OpEqual, "admin123").
			Build()

		results, err := repo.Find(ctx, filter)
		if err != nil {
			t.Fatalf("Find failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}
	})
}
