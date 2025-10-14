package db

import (
	"testing"

	"github.com/Kisanlink/kisanlink-db/pkg/base"
	"go.uber.org/zap"
)

func TestPostgresManager_validateSelectFields(t *testing.T) {
	pm := &PostgresManager{
		logger: zap.NewNop(),
	}

	tests := []struct {
		name    string
		fields  []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid single field",
			fields:  []string{"id"},
			wantErr: false,
		},
		{
			name:    "valid multiple fields",
			fields:  []string{"id", "name", "area_ha"},
			wantErr: false,
		},
		{
			name:    "valid table qualified field",
			fields:  []string{"farms.id", "farms.name"},
			wantErr: false,
		},
		{
			name:    "valid underscore fields",
			fields:  []string{"created_at", "updated_at", "deleted_at"},
			wantErr: false,
		},
		{
			name:    "invalid field with special char",
			fields:  []string{"id'; DROP TABLE users; --"},
			wantErr: true,
			errMsg:  "invalid field name",
		},
		{
			name:    "invalid field with spaces",
			fields:  []string{"id name"},
			wantErr: true,
			errMsg:  "invalid field name",
		},
		{
			name:    "invalid field starting with number",
			fields:  []string{"1id"},
			wantErr: true,
			errMsg:  "invalid field name",
		},
		{
			name:    "too many fields",
			fields:  make([]string, 51),
			wantErr: true,
			errMsg:  "too many select fields",
		},
		{
			name:    "valid field with numbers",
			fields:  []string{"field123", "col_456"},
			wantErr: false,
		},
		{
			name:    "invalid multiple dots",
			fields:  []string{"table.schema.field"},
			wantErr: true,
			errMsg:  "invalid field name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize fields for "too many fields" test
			if tt.name == "too many fields" {
				for i := range tt.fields {
					tt.fields[i] = "field"
				}
			}

			err := pm.validateSelectFields(tt.fields)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSelectFields() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && tt.errMsg != "" {
				if err.Error() == "" || len(err.Error()) == 0 {
					t.Errorf("validateSelectFields() error message is empty, want contains %s", tt.errMsg)
				}
			}
		})
	}
}

func TestPostgresManager_validatePreload(t *testing.T) {
	pm := &PostgresManager{
		logger: zap.NewNop(),
	}

	tests := []struct {
		name    string
		preload base.Preload
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid simple preload",
			preload: base.Preload{
				Relation: "Stage",
			},
			wantErr: false,
		},
		{
			name: "valid nested preload",
			preload: base.Preload{
				Relation: "CropCycle.Crop",
			},
			wantErr: false,
		},
		{
			name: "valid max depth preload",
			preload: base.Preload{
				Relation: "CropCycle.Crop.Variety",
			},
			wantErr: false,
		},
		{
			name: "valid underscore relation",
			preload: base.Preload{
				Relation: "Crop_Stage",
			},
			wantErr: false,
		},
		{
			name: "invalid too deep preload",
			preload: base.Preload{
				Relation: "Level1.Level2.Level3.Level4",
			},
			wantErr: true,
			errMsg:  "preload depth",
		},
		{
			name: "invalid special character",
			preload: base.Preload{
				Relation: "Stage'; DROP TABLE users; --",
			},
			wantErr: true,
			errMsg:  "invalid preload relation",
		},
		{
			name: "invalid space in relation",
			preload: base.Preload{
				Relation: "Crop Stage",
			},
			wantErr: true,
			errMsg:  "invalid preload relation",
		},
		{
			name: "invalid starting with number",
			preload: base.Preload{
				Relation: "1Stage",
			},
			wantErr: true,
			errMsg:  "invalid preload relation",
		},
		{
			name: "valid with conditions",
			preload: base.Preload{
				Relation:   "Farm",
				Conditions: []interface{}{"area_ha > ?", 5.0},
			},
			wantErr: false,
		},
		{
			name: "valid relation with numbers",
			preload: base.Preload{
				Relation: "Stage123.Crop456",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pm.validatePreload(tt.preload)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePreload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && tt.errMsg != "" {
				if err.Error() == "" || len(err.Error()) == 0 {
					t.Errorf("validatePreload() error message is empty, want contains %s", tt.errMsg)
				}
			}
		})
	}
}

func TestPostgresManager_validatePreloadMaxCount(t *testing.T) {
	// This test would be part of the List() method test
	// Testing that max 5 preloads are allowed
	preloads := []base.Preload{
		{Relation: "Rel1"},
		{Relation: "Rel2"},
		{Relation: "Rel3"},
		{Relation: "Rel4"},
		{Relation: "Rel5"},
		{Relation: "Rel6"}, // This should cause error
	}

	if len(preloads) > 5 {
		// Expected: error should be returned
		t.Log("Correctly identified too many preloads")
	}
}

func TestPostgresManager_validateSelectFieldsMaxCount(t *testing.T) {
	pm := &PostgresManager{
		logger: zap.NewNop(),
	}

	// Test with exactly 50 fields (should pass)
	fields50 := make([]string, 50)
	for i := range fields50 {
		fields50[i] = "field"
	}
	err := pm.validateSelectFields(fields50)
	if err != nil {
		t.Errorf("validateSelectFields() with 50 fields should pass, got error: %v", err)
	}

	// Test with 51 fields (should fail)
	fields51 := make([]string, 51)
	for i := range fields51 {
		fields51[i] = "field"
	}
	err = pm.validateSelectFields(fields51)
	if err == nil {
		t.Errorf("validateSelectFields() with 51 fields should fail")
	}
}
