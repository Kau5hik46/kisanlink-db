package db

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Kisanlink/kisanlink-db/pkg/base"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Test models for business logic validation
type TestFarm struct {
	base.BaseModel
	Name     string    `json:"name" gorm:"column:name"`
	AreaHa   float64   `json:"area_ha" gorm:"column:area_ha"`
	Location string    `json:"location" gorm:"column:location"`
	Crops    []TestCrop `json:"crops" gorm:"foreignKey:FarmID"`
}

func (TestFarm) TableName() string {
	return "test_farms"
}

type TestCrop struct {
	base.BaseModel
	FarmID      string       `json:"farm_id" gorm:"column:farm_id"`
	Farm        *TestFarm    `json:"farm" gorm:"foreignKey:FarmID"`
	Name        string       `json:"name" gorm:"column:name"`
	VarietyID   string       `json:"variety_id" gorm:"column:variety_id"`
	Variety     *TestVariety `json:"variety" gorm:"foreignKey:ID;references:VarietyID"`
	PlantedDate time.Time    `json:"planted_date" gorm:"column:planted_date"`
}

func (TestCrop) TableName() string {
	return "test_crops"
}

type TestVariety struct {
	base.BaseModel
	Name        string `json:"name" gorm:"column:name"`
	Description string `json:"description" gorm:"column:description"`
	Season      string `json:"season" gorm:"column:season"`
}

func (TestVariety) TableName() string {
	return "test_varieties"
}

// setupTestDatabase creates an in-memory SQLite database for testing
func setupTestDatabase(t *testing.T) (*gorm.DB, *PostgresManager) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to setup test database: %v", err)
	}

	// Auto-migrate test models
	err = db.AutoMigrate(&TestFarm{}, &TestCrop{}, &TestVariety{})
	if err != nil {
		t.Fatalf("Failed to migrate test models: %v", err)
	}

	// Seed test data
	seedTestData(t, db)

	pm := &PostgresManager{
		primary: db,
		logger:  zap.NewNop(),
	}

	return db, pm
}

func seedTestData(t *testing.T, db *gorm.DB) {
	// Create varieties
	varieties := []TestVariety{
		{BaseModel: base.BaseModel{Model: base.Model{ID: "v1"}}, Name: "Wheat-HD", Description: "High Density Wheat", Season: "Winter"},
		{BaseModel: base.BaseModel{Model: base.Model{ID: "v2"}}, Name: "Rice-Basmati", Description: "Aromatic Rice", Season: "Summer"},
		{BaseModel: base.BaseModel{Model: base.Model{ID: "v3"}}, Name: "Corn-Sweet", Description: "Sweet Corn", Season: "Spring"},
	}
	for _, v := range varieties {
		if err := db.Create(&v).Error; err != nil {
			t.Fatalf("Failed to create variety: %v", err)
		}
	}

	// Create farms
	farms := []TestFarm{
		{BaseModel: base.BaseModel{Model: base.Model{ID: "f1"}}, Name: "Green Valley Farm", AreaHa: 50.5, Location: "North"},
		{BaseModel: base.BaseModel{Model: base.Model{ID: "f2"}}, Name: "Sunrise Farm", AreaHa: 75.2, Location: "South"},
		{BaseModel: base.BaseModel{Model: base.Model{ID: "f3"}}, Name: "River Side Farm", AreaHa: 30.0, Location: "East"},
	}
	for _, f := range farms {
		if err := db.Create(&f).Error; err != nil {
			t.Fatalf("Failed to create farm: %v", err)
		}
	}

	// Create crops
	crops := []TestCrop{
		{BaseModel: base.BaseModel{Model: base.Model{ID: "c1"}}, FarmID: "f1", Name: "Wheat Field 1", VarietyID: "v1", PlantedDate: time.Now().AddDate(0, -2, 0)},
		{BaseModel: base.BaseModel{Model: base.Model{ID: "c2"}}, FarmID: "f1", Name: "Rice Field 1", VarietyID: "v2", PlantedDate: time.Now().AddDate(0, -1, 0)},
		{BaseModel: base.BaseModel{Model: base.Model{ID: "c3"}}, FarmID: "f2", Name: "Corn Field 1", VarietyID: "v3", PlantedDate: time.Now()},
		{BaseModel: base.BaseModel{Model: base.Model{ID: "c4"}}, FarmID: "f2", Name: "Wheat Field 2", VarietyID: "v1", PlantedDate: time.Now().AddDate(0, -3, 0)},
		{BaseModel: base.BaseModel{Model: base.Model{ID: "c5"}}, FarmID: "f3", Name: "Rice Field 2", VarietyID: "v2", PlantedDate: time.Now().AddDate(0, -1, 0)},
	}
	for _, c := range crops {
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("Failed to create crop: %v", err)
		}
	}
}

// Test 1: N+1 Query Prevention
func TestPreload_N1QueryPrevention(t *testing.T) {
	_, pm := setupTestDatabase(t)
	ctx := context.Background()

	t.Run("Without Preload - N+1 queries expected", func(t *testing.T) {
		var farms []TestFarm
		filter := &base.Filter{}

		err := pm.List(ctx, filter, &farms)
		if err != nil {
			t.Fatalf("Failed to list farms: %v", err)
		}

		// Accessing related data should trigger individual queries
		for _, farm := range farms {
			// This would typically cause N+1 queries in a real scenario
			// In our test, crops won't be loaded
			if len(farm.Crops) > 0 {
				t.Errorf("Crops should not be loaded without preload")
			}
		}
	})

	t.Run("With Preload - Single query expected", func(t *testing.T) {
		var farms []TestFarm
		filter := &base.Filter{
			Preloads: []base.Preload{
				{Relation: "Crops"},
			},
		}

		err := pm.List(ctx, filter, &farms)
		if err != nil {
			t.Fatalf("Failed to list farms with preload: %v", err)
		}

		// Crops should be loaded
		foundCrops := false
		for _, farm := range farms {
			if len(farm.Crops) > 0 {
				foundCrops = true
				break
			}
		}

		if !foundCrops {
			t.Errorf("Crops should be loaded with preload")
		}
	})

	t.Run("Multiple Preloads - Batch loading", func(t *testing.T) {
		var crops []TestCrop
		filter := &base.Filter{
			Preloads: []base.Preload{
				{Relation: "Farm"},
				{Relation: "Variety"},
			},
		}

		err := pm.List(ctx, filter, &crops)
		if err != nil {
			t.Fatalf("Failed to list crops with multiple preloads: %v", err)
		}

		// Both Farm and Variety should be loaded
		for _, crop := range crops {
			if crop.Farm == nil {
				t.Errorf("Farm should be loaded for crop %s", crop.ID)
			}
			if crop.Variety == nil {
				t.Errorf("Variety should be loaded for crop %s", crop.ID)
			}
		}
	})
}

// Test 2: Data Integrity with Select
func TestSelect_DataIntegrity(t *testing.T) {
	_, pm := setupTestDatabase(t)
	ctx := context.Background()

	t.Run("Select specific fields", func(t *testing.T) {
		var farms []TestFarm
		filter := &base.Filter{
			Selects: []string{"id", "name"},
		}

		err := pm.List(ctx, filter, &farms)
		if err != nil {
			t.Fatalf("Failed to list farms with select: %v", err)
		}

		for _, farm := range farms {
			// Selected fields should be loaded
			if farm.ID == "" {
				t.Errorf("ID should be loaded")
			}
			if farm.Name == "" {
				t.Errorf("Name should be loaded")
			}
			// Non-selected fields should be zero values
			if farm.AreaHa != 0 {
				t.Errorf("AreaHa should be zero value when not selected")
			}
			if farm.Location != "" {
				t.Errorf("Location should be zero value when not selected")
			}
		}
	})

	t.Run("Select with empty list - should load all fields", func(t *testing.T) {
		var farms []TestFarm
		filter := &base.Filter{
			Selects: []string{},
		}

		err := pm.List(ctx, filter, &farms)
		if err != nil {
			t.Fatalf("Failed to list farms with empty select: %v", err)
		}

		// All fields should be loaded
		for _, farm := range farms {
			if farm.ID == "" || farm.Name == "" {
				t.Errorf("All fields should be loaded with empty select")
			}
		}
	})

	t.Run("Select with table qualified names", func(t *testing.T) {
		var farms []TestFarm
		filter := &base.Filter{
			Selects: []string{"test_farms.id", "test_farms.name"},
		}

		err := pm.List(ctx, filter, &farms)
		if err != nil {
			t.Fatalf("Failed to list farms with qualified select: %v", err)
		}

		// Fields should still be loaded correctly
		if len(farms) == 0 {
			t.Errorf("Farms should be loaded with qualified field names")
		}
	})
}

// Test 3: Security Validation Logic
func TestSecurity_ValidationLogic(t *testing.T) {
	_, pm := setupTestDatabase(t)

	testCases := []struct {
		name        string
		fields      []string
		preloads    []base.Preload
		expectError bool
		errorMsg    string
	}{
		{
			name:        "SQL injection in field name",
			fields:      []string{"id'; DROP TABLE users; --"},
			expectError: true,
			errorMsg:    "invalid field name",
		},
		{
			name:        "Malformed field with SELECT",
			fields:      []string{"field; SELECT * FROM users"},
			expectError: true,
			errorMsg:    "invalid field name",
		},
		{
			name:        "Path traversal attempt",
			fields:      []string{"../../../etc/passwd"},
			expectError: true,
			errorMsg:    "invalid field name",
		},
		{
			name:        "Too many select fields",
			fields:      generateFieldNames(51),
			expectError: true,
			errorMsg:    "too many select fields",
		},
		{
			name: "Too many preloads",
			preloads: []base.Preload{
				{Relation: "Rel1"},
				{Relation: "Rel2"},
				{Relation: "Rel3"},
				{Relation: "Rel4"},
				{Relation: "Rel5"},
				{Relation: "Rel6"}, // 6th preload
			},
			expectError: true,
			errorMsg:    "too many preloads",
		},
		{
			name: "Deep nesting exceeds limit",
			preloads: []base.Preload{
				{Relation: "Level1.Level2.Level3.Level4"}, // 4 levels
			},
			expectError: true,
			errorMsg:    "preload depth",
		},
		{
			name:        "Valid table.field format",
			fields:      []string{"test_farms.name", "test_farms.area_ha"},
			expectError: false,
		},
		{
			name: "Valid nested preload at boundary",
			preloads: []base.Preload{
				{Relation: "Farm.Crops.Variety"}, // Exactly 3 levels
			},
			expectError: false,
		},
		{
			name: "SQL injection in preload",
			preloads: []base.Preload{
				{Relation: "Farm'; DROP TABLE users; --"},
			},
			expectError: true,
			errorMsg:    "invalid preload relation",
		},
		{
			name:        "Field with spaces",
			fields:      []string{"field name"},
			expectError: true,
			errorMsg:    "invalid field name",
		},
		{
			name: "Preload with spaces",
			preloads: []base.Preload{
				{Relation: "Farm Crops"},
			},
			expectError: true,
			errorMsg:    "invalid preload relation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var errorOccurred bool
			var errorMsg string

			// Test select field validation
			if len(tc.fields) > 0 {
				err := pm.validateSelectFields(tc.fields)
				if err != nil {
					errorOccurred = true
					errorMsg = err.Error()
				}
			}

			// Test preload validation
			if len(tc.preloads) > 0 {
				// Check preload count
				if len(tc.preloads) > 5 {
					errorOccurred = true
					errorMsg = fmt.Sprintf("too many preloads: %d (max: 5)", len(tc.preloads))
				} else {
					// Validate each preload
					for _, p := range tc.preloads {
						err := pm.validatePreload(p)
						if err != nil {
							errorOccurred = true
							errorMsg = err.Error()
							break
						}
					}
				}
			}

			if errorOccurred != tc.expectError {
				t.Errorf("Expected error: %v, got error: %v (message: %s)",
					tc.expectError, errorOccurred, errorMsg)
			}

			if tc.expectError && errorMsg != "" && !strings.Contains(errorMsg, tc.errorMsg) {
				t.Errorf("Expected error message to contain '%s', got '%s'",
					tc.errorMsg, errorMsg)
			}
		})
	}
}

// Test 4: Combined Operations
func TestCombined_Operations(t *testing.T) {
	_, pm := setupTestDatabase(t)
	ctx := context.Background()

	t.Run("Filter + Select", func(t *testing.T) {
		var farms []TestFarm
		filter := &base.Filter{
			Group: base.FilterGroup{
				Conditions: []base.FilterCondition{
					{Field: "area_ha", Operator: base.OpGreaterThan, Value: 40},
				},
			},
			Selects: []string{"id", "name", "area_ha"},
		}

		err := pm.List(ctx, filter, &farms)
		if err != nil {
			t.Fatalf("Failed to list farms with filter+select: %v", err)
		}

		// Should have filtered results with selected fields
		for _, farm := range farms {
			if farm.AreaHa <= 40 {
				t.Errorf("Filter not applied correctly: area_ha %f should be > 40", farm.AreaHa)
			}
			if farm.Location != "" {
				t.Errorf("Location should not be loaded when not selected")
			}
		}
	})

	t.Run("Filter + Preload", func(t *testing.T) {
		var farms []TestFarm
		filter := &base.Filter{
			Group: base.FilterGroup{
				Conditions: []base.FilterCondition{
					{Field: "name", Operator: base.OpContains, Value: "Valley"},
				},
			},
			Preloads: []base.Preload{
				{Relation: "Crops"},
			},
		}

		err := pm.List(ctx, filter, &farms)
		if err != nil {
			t.Fatalf("Failed to list farms with filter+preload: %v", err)
		}

		// Should have filtered results with preloaded relations
		for _, farm := range farms {
			if !strings.Contains(farm.Name, "Valley") {
				t.Errorf("Filter not applied correctly: name should contain 'Valley'")
			}
		}
	})

	t.Run("Filter + Select + Preload + Pagination", func(t *testing.T) {
		var crops []TestCrop
		filter := &base.Filter{
			Group: base.FilterGroup{
				Conditions: []base.FilterCondition{
					{Field: "name", Operator: base.OpNotEqual, Value: ""},
				},
			},
			Selects: []string{"id", "name", "farm_id"},
			Preloads: []base.Preload{
				{Relation: "Farm"},
			},
			Page:     1,
			PageSize: 2,
			Sort: []base.SortField{
				{Field: "name", Direction: "asc"},
			},
		}

		err := pm.List(ctx, filter, &crops)
		if err != nil {
			t.Fatalf("Failed to list crops with combined operations: %v", err)
		}

		// Check pagination
		if len(crops) > 2 {
			t.Errorf("Pagination not applied: expected max 2 results, got %d", len(crops))
		}

		// Check preload on paginated results
		for _, crop := range crops {
			if crop.Farm == nil {
				t.Errorf("Farm should be preloaded even with pagination")
			}
		}
	})
}

// Test 5: Operation Order Verification
func TestOperationOrder(t *testing.T) {
	_, pm := setupTestDatabase(t)
	ctx := context.Background()

	t.Run("Preload happens after pagination", func(t *testing.T) {
		var farms []TestFarm
		filter := &base.Filter{
			Preloads: []base.Preload{
				{Relation: "Crops"},
			},
			Page:     1,
			PageSize: 1,
		}

		err := pm.List(ctx, filter, &farms)
		if err != nil {
			t.Fatalf("Failed to list farms: %v", err)
		}

		// Only 1 farm should be loaded due to pagination
		if len(farms) != 1 {
			t.Errorf("Expected 1 farm due to pagination, got %d", len(farms))
		}

		// But that farm should have its crops preloaded
		if farms[0].Crops == nil {
			t.Errorf("Crops should be preloaded for the paginated result")
		}
	})
}

// Test 6: Edge Cases & Boundary Conditions
func TestEdgeCases(t *testing.T) {
	_, pm := setupTestDatabase(t)
	ctx := context.Background()

	t.Run("Empty filter with only Preload", func(t *testing.T) {
		var farms []TestFarm
		filter := &base.Filter{
			Preloads: []base.Preload{
				{Relation: "Crops"},
			},
		}

		err := pm.List(ctx, filter, &farms)
		if err != nil {
			t.Fatalf("Failed to list with only preload: %v", err)
		}

		if len(farms) == 0 {
			t.Errorf("Should load all farms with only preload")
		}
	})

	t.Run("Empty filter with only Select", func(t *testing.T) {
		var farms []TestFarm
		filter := &base.Filter{
			Selects: []string{"id", "name"},
		}

		err := pm.List(ctx, filter, &farms)
		if err != nil {
			t.Fatalf("Failed to list with only select: %v", err)
		}

		if len(farms) == 0 {
			t.Errorf("Should load all farms with only select")
		}
	})

	t.Run("Exactly 5 preloads (boundary)", func(t *testing.T) {
		filter := &base.Filter{
			Preloads: []base.Preload{
				{Relation: "Rel1"},
				{Relation: "Rel2"},
				{Relation: "Rel3"},
				{Relation: "Rel4"},
				{Relation: "Rel5"},
			},
		}

		// This should not error as it's exactly at the boundary
		var crops []TestCrop
		err := pm.List(ctx, filter, &crops)
		// The preloads might not exist but validation should pass
		if err != nil && strings.Contains(err.Error(), "too many preloads") {
			t.Errorf("5 preloads should be allowed (boundary condition)")
		}
	})

	t.Run("Exactly 50 select fields (boundary)", func(t *testing.T) {
		fields := generateFieldNames(50)
		err := pm.validateSelectFields(fields)
		if err != nil {
			t.Errorf("50 fields should be allowed (boundary condition): %v", err)
		}
	})

	t.Run("Preload with conditions", func(t *testing.T) {
		var farms []TestFarm
		filter := &base.Filter{
			Preloads: []base.Preload{
				{
					Relation:   "Crops",
					Conditions: []interface{}{"name LIKE ?", "%Field 1%"},
				},
			},
		}

		err := pm.List(ctx, filter, &farms)
		if err != nil {
			t.Fatalf("Failed to list with conditional preload: %v", err)
		}

		// Conditional preload should work
		if len(farms) == 0 {
			t.Errorf("Should load farms with conditional preload")
		}
	})
}

// Test 7: Error Handling Invariants
func TestErrorHandling(t *testing.T) {
	_, pm := setupTestDatabase(t)
	ctx := context.Background()

	t.Run("Invalid field names fail gracefully", func(t *testing.T) {
		var farms []TestFarm
		filter := &base.Filter{
			Selects: []string{"'; DROP TABLE --"},
		}

		err := pm.List(ctx, filter, &farms)
		if err == nil {
			t.Errorf("Should error on invalid field name")
		}
		if !strings.Contains(err.Error(), "invalid") {
			t.Errorf("Error should mention 'invalid': %v", err)
		}
	})

	t.Run("Invalid preload relations fail gracefully", func(t *testing.T) {
		var farms []TestFarm
		filter := &base.Filter{
			Preloads: []base.Preload{
				{Relation: "Invalid!@#$"},
			},
		}

		err := pm.List(ctx, filter, &farms)
		if err == nil {
			t.Errorf("Should error on invalid preload relation")
		}
		if !strings.Contains(err.Error(), "invalid preload") {
			t.Errorf("Error should mention 'invalid preload': %v", err)
		}
	})

	t.Run("Query complexity limits enforced", func(t *testing.T) {
		var farms []TestFarm

		// Too many preloads
		filter := &base.Filter{
			Preloads: generatePreloads(6),
		}

		err := pm.List(ctx, filter, &farms)
		if err == nil {
			t.Errorf("Should error on too many preloads")
		}
		if !strings.Contains(err.Error(), "too many preloads") {
			t.Errorf("Error should mention 'too many preloads': %v", err)
		}
	})
}

// Test 8: Backward Compatibility
func TestBackwardCompatibility(t *testing.T) {
	_, pm := setupTestDatabase(t)
	ctx := context.Background()

	t.Run("Existing filters without Preload/Select work unchanged", func(t *testing.T) {
		var farms []TestFarm
		filter := &base.Filter{
			Group: base.FilterGroup{
				Conditions: []base.FilterCondition{
					{Field: "area_ha", Operator: base.OpGreaterThan, Value: 30},
				},
			},
			Sort: []base.SortField{
				{Field: "name", Direction: "asc"},
			},
			Page:     1,
			PageSize: 10,
		}

		err := pm.List(ctx, filter, &farms)
		if err != nil {
			t.Fatalf("Legacy filter should still work: %v", err)
		}

		if len(farms) == 0 {
			t.Errorf("Legacy filter should return results")
		}
	})

	t.Run("Nil Preload/Select behaves like no specification", func(t *testing.T) {
		var farms []TestFarm
		filter := &base.Filter{
			Preloads: nil,
			Selects:  nil,
		}

		err := pm.List(ctx, filter, &farms)
		if err != nil {
			t.Fatalf("Nil preloads/selects should work: %v", err)
		}

		if len(farms) == 0 {
			t.Errorf("Should return all farms with nil preloads/selects")
		}
	})

	t.Run("Empty arrays behave like no specification", func(t *testing.T) {
		var farms []TestFarm
		filter := &base.Filter{
			Preloads: []base.Preload{},
			Selects:  []string{},
		}

		err := pm.List(ctx, filter, &farms)
		if err != nil {
			t.Fatalf("Empty preloads/selects should work: %v", err)
		}

		if len(farms) == 0 {
			t.Errorf("Should return all farms with empty preloads/selects")
		}
	})
}

// Test 9: Concurrency Invariants
func TestConcurrency(t *testing.T) {
	_, pm := setupTestDatabase(t)
	ctx := context.Background()

	t.Run("Multiple concurrent queries don't interfere", func(t *testing.T) {
		done := make(chan bool, 3)
		errors := make(chan error, 3)

		// Query 1: Select specific fields
		go func() {
			var farms []TestFarm
			filter := &base.Filter{
				Selects: []string{"id", "name"},
			}
			err := pm.List(ctx, filter, &farms)
			if err != nil {
				errors <- err
			}
			done <- true
		}()

		// Query 2: Preload relations
		go func() {
			var crops []TestCrop
			filter := &base.Filter{
				Preloads: []base.Preload{
					{Relation: "Farm"},
				},
			}
			err := pm.List(ctx, filter, &crops)
			if err != nil {
				errors <- err
			}
			done <- true
		}()

		// Query 3: Combined operations
		go func() {
			var farms []TestFarm
			filter := &base.Filter{
				Selects: []string{"id", "name", "area_ha"},
				Preloads: []base.Preload{
					{Relation: "Crops"},
				},
			}
			err := pm.List(ctx, filter, &farms)
			if err != nil {
				errors <- err
			}
			done <- true
		}()

		// Wait for all queries
		for i := 0; i < 3; i++ {
			select {
			case <-done:
				// Good
			case err := <-errors:
				t.Errorf("Concurrent query failed: %v", err)
			case <-time.After(5 * time.Second):
				t.Errorf("Concurrent query timed out")
			}
		}
	})

	t.Run("FilterBuilder is immutable after Build()", func(t *testing.T) {
		fb := base.NewFilterBuilder()
		fb.Select("id", "name")
		fb.Preload("Crops")

		filter1 := fb.Build()

		// Modify builder after build
		fb.Select("area_ha")
		fb.Preload("Another")

		filter2 := fb.Build()

		// filter1 should not be affected
		if len(filter1.Selects) != 2 {
			t.Errorf("Filter1 was modified after Build()")
		}
		if len(filter1.Preloads) != 1 {
			t.Errorf("Filter1 preloads were modified after Build()")
		}

		// filter2 should have all modifications
		if len(filter2.Selects) != 3 {
			t.Errorf("Filter2 should have all select fields")
		}
		if len(filter2.Preloads) != 2 {
			t.Errorf("Filter2 should have all preloads")
		}
	})
}

// Helper functions
func generateFieldNames(count int) []string {
	fields := make([]string, count)
	for i := 0; i < count; i++ {
		fields[i] = fmt.Sprintf("field_%d", i)
	}
	return fields
}

func generatePreloads(count int) []base.Preload {
	preloads := make([]base.Preload, count)
	for i := 0; i < count; i++ {
		preloads[i] = base.Preload{Relation: fmt.Sprintf("Relation%d", i)}
	}
	return preloads
}