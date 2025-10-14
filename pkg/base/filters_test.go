package base

import (
	"testing"
)

func TestFilterBuilder_Preload(t *testing.T) {
	tests := []struct {
		name       string
		relation   string
		conditions []interface{}
		wantLen    int
	}{
		{
			name:       "simple preload",
			relation:   "Stage",
			conditions: nil,
			wantLen:    1,
		},
		{
			name:       "preload with conditions",
			relation:   "Farm",
			conditions: []interface{}{"area_ha > ?", 5.0},
			wantLen:    1,
		},
		{
			name:       "nested preload",
			relation:   "CropCycle.Crop",
			conditions: nil,
			wantLen:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := NewFilterBuilder()
			if tt.conditions != nil {
				fb = fb.Preload(tt.relation, tt.conditions...)
			} else {
				fb = fb.Preload(tt.relation)
			}
			filter := fb.Build()

			if len(filter.Preloads) != tt.wantLen {
				t.Errorf("Preload() len = %d, want %d", len(filter.Preloads), tt.wantLen)
			}

			if filter.Preloads[0].Relation != tt.relation {
				t.Errorf("Preload() relation = %s, want %s", filter.Preloads[0].Relation, tt.relation)
			}

			if tt.conditions != nil && len(filter.Preloads[0].Conditions) != len(tt.conditions) {
				t.Errorf("Preload() conditions len = %d, want %d", len(filter.Preloads[0].Conditions), len(tt.conditions))
			}
		})
	}
}

func TestFilterBuilder_MultiplePreloads(t *testing.T) {
	fb := NewFilterBuilder().
		Preload("Stage").
		Preload("Crop").
		Preload("Farm")

	filter := fb.Build()

	if len(filter.Preloads) != 3 {
		t.Errorf("Multiple Preload() len = %d, want 3", len(filter.Preloads))
	}

	expectedRelations := []string{"Stage", "Crop", "Farm"}
	for i, preload := range filter.Preloads {
		if preload.Relation != expectedRelations[i] {
			t.Errorf("Preload[%d] relation = %s, want %s", i, preload.Relation, expectedRelations[i])
		}
	}
}

func TestFilterBuilder_Select(t *testing.T) {
	tests := []struct {
		name    string
		fields  []string
		wantLen int
	}{
		{
			name:    "single field",
			fields:  []string{"id"},
			wantLen: 1,
		},
		{
			name:    "multiple fields",
			fields:  []string{"id", "name", "area_ha"},
			wantLen: 3,
		},
		{
			name:    "table qualified field",
			fields:  []string{"farms.id", "farms.name"},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := NewFilterBuilder().Select(tt.fields...)
			filter := fb.Build()

			if len(filter.Selects) != tt.wantLen {
				t.Errorf("Select() len = %d, want %d", len(filter.Selects), tt.wantLen)
			}

			for i, field := range tt.fields {
				if filter.Selects[i] != field {
					t.Errorf("Select()[%d] = %s, want %s", i, filter.Selects[i], field)
				}
			}
		})
	}
}

func TestFilterBuilder_MultipleSelects(t *testing.T) {
	fb := NewFilterBuilder().
		Select("id", "name").
		Select("area_ha")

	filter := fb.Build()

	if len(filter.Selects) != 3 {
		t.Errorf("Multiple Select() len = %d, want 3", len(filter.Selects))
	}

	expectedFields := []string{"id", "name", "area_ha"}
	for i, field := range filter.Selects {
		if field != expectedFields[i] {
			t.Errorf("Select[%d] = %s, want %s", i, field, expectedFields[i])
		}
	}
}

func TestFilterBuilder_CombinedPreloadAndSelect(t *testing.T) {
	fb := NewFilterBuilder().
		Where("crop_id", OpEqual, "123").
		Select("id", "name", "stage_id").
		Preload("Stage").
		Preload("Crop").
		Sort("name", "asc").
		Page(1, 10)

	filter := fb.Build()

	// Verify conditions
	if len(filter.Group.Conditions) != 1 {
		t.Errorf("Conditions len = %d, want 1", len(filter.Group.Conditions))
	}

	// Verify selects
	if len(filter.Selects) != 3 {
		t.Errorf("Selects len = %d, want 3", len(filter.Selects))
	}

	// Verify preloads
	if len(filter.Preloads) != 2 {
		t.Errorf("Preloads len = %d, want 2", len(filter.Preloads))
	}

	// Verify sort
	if len(filter.Sort) != 1 {
		t.Errorf("Sort len = %d, want 1", len(filter.Sort))
	}

	// Verify pagination
	if filter.Page != 1 || filter.PageSize != 10 {
		t.Errorf("Pagination = (%d, %d), want (1, 10)", filter.Page, filter.PageSize)
	}
}

func TestFilterBuilder_BackwardCompatibility(t *testing.T) {
	// Test that existing code without preload/select still works
	fb := NewFilterBuilder().
		Where("is_active", OpEqual, true).
		Sort("created_at", "desc").
		Limit(100, 0)

	filter := fb.Build()

	if len(filter.Group.Conditions) != 1 {
		t.Errorf("Conditions len = %d, want 1", len(filter.Group.Conditions))
	}

	if len(filter.Preloads) != 0 {
		t.Errorf("Preloads len = %d, want 0 (backward compatibility)", len(filter.Preloads))
	}

	if len(filter.Selects) != 0 {
		t.Errorf("Selects len = %d, want 0 (backward compatibility)", len(filter.Selects))
	}
}
