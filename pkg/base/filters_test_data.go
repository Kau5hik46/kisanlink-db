package base

import (
	"time"

	"github.com/Kisanlink/kisanlink-db/pkg/core/hash"
)

// Test data for filter operators
var FilterOperatorTests = []struct {
	name     string
	operator FilterOperator
	value    interface{}
	expected bool
}{
	{
		name:     "Equal operator",
		operator: OpEqual,
		value:    "test",
		expected: true,
	},
	{
		name:     "Not equal operator",
		operator: OpNotEqual,
		value:    "test",
		expected: false,
	},
	{
		name:     "Greater than operator",
		operator: OpGreaterThan,
		value:    10,
		expected: true,
	},
	{
		name:     "Less than operator",
		operator: OpLessThan,
		value:    5,
		expected: true,
	},
	{
		name:     "Contains operator",
		operator: OpContains,
		value:    "test",
		expected: true,
	},
	{
		name:     "Starts with operator",
		operator: OpStartsWith,
		value:    "test",
		expected: true,
	},
	{
		name:     "Ends with operator",
		operator: OpEndsWith,
		value:    "test",
		expected: true,
	},
}

// Test data for filter conditions
var FilterConditionTests = []struct {
	name        string
	condition   FilterCondition
	shouldMatch bool
}{
	{
		name: "ID equals",
		condition: FilterCondition{
			Field:    "id",
			Operator: OpEqual,
			Value:    "USER12345678",
		},
		shouldMatch: true,
	},
	{
		name: "Created by equals",
		condition: FilterCondition{
			Field:    "created_by",
			Operator: OpEqual,
			Value:    "admin123",
		},
		shouldMatch: true,
	},
	{
		name: "Email contains",
		condition: FilterCondition{
			Field:    "email",
			Operator: OpContains,
			Value:    "example",
		},
		shouldMatch: true,
	},
	{
		name: "Name starts with",
		condition: FilterCondition{
			Field:    "name",
			Operator: OpStartsWith,
			Value:    "John",
		},
		shouldMatch: true,
	},
	{
		name: "Is deleted false",
		condition: FilterCondition{
			Field:    "is_deleted",
			Operator: OpEqual,
			Value:    false,
		},
		shouldMatch: true,
	},
}

// Test data for complex filters
var ComplexFilterTests = []struct {
	name          string
	filter        *Filter
	expectedCount int
}{
	{
		name: "Simple AND filter",
		filter: &Filter{
			Group: FilterGroup{
				Logic: LogicAnd,
				Conditions: []FilterCondition{
					{Field: "created_by", Operator: OpEqual, Value: "admin123"},
					{Field: "is_deleted", Operator: OpEqual, Value: false},
				},
			},
		},
		expectedCount: 1,
	},
	{
		name: "Simple OR filter",
		filter: &Filter{
			Group: FilterGroup{
				Logic: LogicOr,
				Conditions: []FilterCondition{
					{Field: "created_by", Operator: OpEqual, Value: "admin123"},
					{Field: "created_by", Operator: OpEqual, Value: "user456"},
				},
			},
		},
		expectedCount: 2,
	},
	{
		name: "Nested filter groups",
		filter: &Filter{
			Group: FilterGroup{
				Logic: LogicAnd,
				Conditions: []FilterCondition{
					{Field: "is_deleted", Operator: OpEqual, Value: false},
				},
				Groups: []FilterGroup{
					{
						Logic: LogicOr,
						Conditions: []FilterCondition{
							{Field: "created_by", Operator: OpEqual, Value: "admin123"},
							{Field: "updated_by", Operator: OpEqual, Value: "admin123"},
						},
					},
				},
			},
		},
		expectedCount: 1,
	},
}

// Test data for pagination
var PaginationTests = []struct {
	name          string
	page          int
	pageSize      int
	limit         int
	offset        int
	expectedCount int
}{
	{
		name:          "Page 1, size 5",
		page:          1,
		pageSize:      5,
		expectedCount: 5,
	},
	{
		name:          "Page 2, size 3",
		page:          2,
		pageSize:      3,
		expectedCount: 3,
	},
	{
		name:          "Limit 10, offset 5",
		limit:         10,
		offset:        5,
		expectedCount: 10,
	},
	{
		name:          "Limit 5, offset 15",
		limit:         5,
		offset:        15,
		expectedCount: 5,
	},
}

// Test data for sorting
var SortingTests = []struct {
	name          string
	sortFields    []SortField
	expectedOrder []string
}{
	{
		name: "Sort by created_at ascending",
		sortFields: []SortField{
			{Field: "created_at", Direction: "asc"},
		},
		expectedOrder: []string{"oldest", "newest"},
	},
	{
		name: "Sort by name descending",
		sortFields: []SortField{
			{Field: "name", Direction: "desc"},
		},
		expectedOrder: []string{"Zebra", "John"},
	},
	{
		name: "Sort by multiple fields",
		sortFields: []SortField{
			{Field: "created_by", Direction: "asc"},
			{Field: "created_at", Direction: "desc"},
		},
		expectedOrder: []string{"admin123", "user456"},
	},
}

// Test data for date filters
var DateFilterTests = []struct {
	name        string
	condition   FilterCondition
	shouldMatch bool
}{
	{
		name: "Date equal",
		condition: FilterCondition{
			Field:    "created_at",
			Operator: OpDateEqual,
			Value:    time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		shouldMatch: true,
	},
	{
		name: "Date before",
		condition: FilterCondition{
			Field:    "created_at",
			Operator: OpDateBefore,
			Value:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		shouldMatch: true,
	},
	{
		name: "Date after",
		condition: FilterCondition{
			Field:    "created_at",
			Operator: OpDateAfter,
			Value:    time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		shouldMatch: true,
	},
	{
		name: "Date between",
		condition: FilterCondition{
			Field:    "created_at",
			Operator: OpDateBetween,
			Value:    time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
			Value2:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		shouldMatch: true,
	},
}

// Test data for null filters
var NullFilterTests = []struct {
	name        string
	condition   FilterCondition
	shouldMatch bool
}{
	{
		name: "Is null",
		condition: FilterCondition{
			Field:    "deleted_at",
			Operator: OpIsNull,
		},
		shouldMatch: true,
	},
	{
		name: "Is not null",
		condition: FilterCondition{
			Field:    "created_at",
			Operator: OpIsNotNull,
		},
		shouldMatch: true,
	},
}

// Test data for IN filters
var InFilterTests = []struct {
	name        string
	condition   FilterCondition
	shouldMatch bool
}{
	{
		name: "In array",
		condition: FilterCondition{
			Field:    "created_by",
			Operator: OpIn,
			Value:    []interface{}{"admin123", "user456", "user789"},
		},
		shouldMatch: true,
	},
	{
		name: "Not in array",
		condition: FilterCondition{
			Field:    "created_by",
			Operator: OpNotIn,
			Value:    []interface{}{"user999", "user888"},
		},
		shouldMatch: true,
	},
}

// Helper function to create test models for filtering
func CreateTestModelsForFiltering() []*BaseModel {
	models := []*BaseModel{}

	// Create models with different characteristics
	model1 := NewBaseModel("TEST", hash.Medium)
	model1.SetCreatedBy("admin123")
	model1.SetUpdatedBy("admin123")
	models = append(models, model1)

	model2 := NewBaseModel("TEST", hash.Medium)
	model2.SetCreatedBy("user456")
	model2.SetUpdatedBy("user456")
	models = append(models, model2)

	model3 := NewBaseModel("TEST", hash.Medium)
	model3.SetCreatedBy("user789")
	model3.SetUpdatedBy("admin123")
	models = append(models, model3)

	// Create a soft deleted model
	model4 := NewBaseModel("TEST", hash.Medium)
	model4.SetCreatedBy("admin123")
	model4.SetDeletedAt(&time.Time{})
	model4.SetDeletedBy(&[]string{"admin123"}[0])
	models = append(models, model4)

	return models
}

// Helper function to create test users for filtering
func CreateTestUsersForFiltering() []*User {
	users := []*User{}

	user1 := NewUser("John Doe", "john@example.com")
	user1.SetCreatedBy("admin123")
	user1.SetUpdatedBy("admin123")
	users = append(users, user1)

	user2 := NewUser("Jane Smith", "jane@example.com")
	user2.SetCreatedBy("user456")
	user2.SetUpdatedBy("user456")
	users = append(users, user2)

	user3 := NewUser("Bob Johnson", "bob@example.com")
	user3.SetCreatedBy("user789")
	user3.SetUpdatedBy("admin123")
	users = append(users, user3)

	// Create a soft deleted user
	user4 := NewUser("Deleted User", "deleted@example.com")
	user4.SetCreatedBy("admin123")
	user4.SetDeletedAt(&time.Time{})
	user4.SetDeletedBy(&[]string{"admin123"}[0])
	users = append(users, user4)

	return users
}

// Helper function to create filter builder with common conditions
func CreateFilterBuilder() *FilterBuilder {
	return NewFilterBuilder().
		Where("is_deleted", OpEqual, false).
		Sort("created_at", "desc")
}

// Helper function to validate filter results
func ValidateFilterResults(results []ModelInterface, expectedCount int) bool {
	return len(results) == expectedCount
}

// Helper function to check if model matches filter condition
func ModelMatchesCondition(model ModelInterface, condition FilterCondition) bool {
	evaluator := NewFilterEvaluator()
	filter := &Filter{
		Group: FilterGroup{
			Logic:      LogicAnd,
			Conditions: []FilterCondition{condition},
		},
	}

	matches, err := evaluator.Evaluate(filter, model)
	if err != nil {
		return false
	}
	return matches
}
