package hash

import (
	"strings"
	"testing"
)

func TestNewIDGenerator(t *testing.T) {
	generator := NewIDGenerator()
	if generator == nil {
		t.Fatal("NewIDGenerator() returned nil")
	}
	if generator.counters == nil {
		t.Fatal("counters map was not initialized")
	}
}

func TestGenerateRandomID(t *testing.T) {
	for _, tt := range GenerateRandomIDTests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := GenerateRandomID(tt.tableIdentifier, tt.size)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(id) != tt.expectedLength {
				t.Errorf("Expected ID length %d, got %d", tt.expectedLength, len(id))
			}

			if !strings.HasPrefix(id, tt.expectedPrefix) {
				t.Errorf("Expected ID to start with %s, got %s", tt.expectedPrefix, id)
			}

			// Verify numeric part is actually numeric
			numericPart := id[4:]
			for _, char := range numericPart {
				if char < '0' || char > '9' {
					t.Errorf("Numeric part contains non-digit character: %c", char)
				}
			}
		})
	}
}

func TestGenerateRandomIDWithPattern(t *testing.T) {
	for _, tt := range GenerateRandomIDWithPatternTests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := GenerateRandomIDWithPattern(tt.tableIdentifier, tt.size, tt.pattern)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(id) != tt.expectedLength {
				t.Errorf("Expected ID length %d, got %d", tt.expectedLength, len(id))
			}

			if !strings.HasPrefix(id, tt.expectedPrefix) {
				t.Errorf("Expected ID to start with %s, got %s", tt.expectedPrefix, id)
			}
		})
	}
}

func TestIncrementalIDGeneration(t *testing.T) {
	generator := NewIDGenerator()
	tableID := "TEST"

	// Generate multiple IDs and verify they are incremental
	ids := make([]string, 5)
	for i := 0; i < 5; i++ {
		id, err := generator.GenerateID(tableID, Medium, "incremental")
		if err != nil {
			t.Fatalf("Failed to generate ID: %v", err)
		}
		ids[i] = id
	}

	// Verify IDs are unique
	idSet := make(map[string]bool)
	for _, id := range ids {
		if idSet[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		idSet[id] = true
	}

	// Verify numeric parts are incremental
	for i := 1; i < len(ids); i++ {
		prevNum := extractNumericPart(ids[i-1])
		currNum := extractNumericPart(ids[i])
		if currNum <= prevNum {
			t.Errorf("IDs are not incremental: %s -> %s", ids[i-1], ids[i])
		}
	}
}

func TestTimestampIDGeneration(t *testing.T) {
	generator := NewIDGenerator()
	tableID := "TEST"

	// Generate multiple IDs rapidly
	ids := make([]string, 10)
	for i := 0; i < 10; i++ {
		id, err := generator.GenerateID(tableID, Medium, "timestamp")
		if err != nil {
			t.Fatalf("Failed to generate ID: %v", err)
		}
		ids[i] = id
	}

	// Verify IDs are unique
	idSet := make(map[string]bool)
	for _, id := range ids {
		if idSet[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		idSet[id] = true
	}
}

func TestHashIDGeneration(t *testing.T) {
	generator := NewIDGenerator()
	tableID := "TEST"

	// Generate multiple IDs
	ids := make([]string, 10)
	for i := 0; i < 10; i++ {
		id, err := generator.GenerateID(tableID, Medium, "hash")
		if err != nil {
			t.Fatalf("Failed to generate ID: %v", err)
		}
		ids[i] = id
	}

	// Verify IDs are unique
	idSet := make(map[string]bool)
	for _, id := range ids {
		if idSet[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		idSet[id] = true
	}
}

func TestConcurrentIDGeneration(t *testing.T) {
	generator := NewIDGenerator()
	tableID := "TEST"
	numGoroutines := 100
	ids := make(chan string, numGoroutines)

	// Generate IDs concurrently
	for i := 0; i < numGoroutines; i++ {
		go func() {
			id, err := generator.GenerateID(tableID, Medium, "incremental")
			if err != nil {
				t.Errorf("Failed to generate ID: %v", err)
				return
			}
			ids <- id
		}()
	}

	// Collect all IDs
	idSet := make(map[string]bool)
	for i := 0; i < numGoroutines; i++ {
		id := <-ids
		if idSet[id] {
			t.Errorf("Duplicate ID generated in concurrent test: %s", id)
		}
		idSet[id] = true
	}

	if len(idSet) != numGoroutines {
		t.Errorf("Expected %d unique IDs, got %d", numGoroutines, len(idSet))
	}
}

func TestDifferentTableIdentifiers(t *testing.T) {
	generator := NewIDGenerator()

	// Test different table identifiers
	tables := []string{"USER", "ORDR", "PROD", "CATE"}

	for _, tableID := range tables {
		id, err := generator.GenerateID(tableID, Medium, "incremental")
		if err != nil {
			t.Fatalf("Failed to generate ID for table %s: %v", tableID, err)
		}

		if !strings.HasPrefix(id, tableID) {
			t.Errorf("Expected ID to start with %s, got %s", tableID, id)
		}
	}
}

func TestTableSizeFormats(t *testing.T) {
	generator := NewIDGenerator()
	tableID := "TEST"

	for _, tt := range TableSizeFormatTests {
		t.Run(string(tt.size), func(t *testing.T) {
			id, err := generator.GenerateID(tableID, tt.size, "incremental")
			if err != nil {
				t.Fatalf("Failed to generate ID: %v", err)
			}

			if len(id) != tt.expectedLength {
				t.Errorf("Expected length %d for size %s, got %d", tt.expectedLength, tt.size, len(id))
			}
		})
	}
}

func TestGetModelTypeFromHash(t *testing.T) {
	for _, tt := range GetModelTypeFromHashTests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetModelTypeFromHash(tt.hashID)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
			t.Logf("Successfully extracted model type %s from hash ID %s", result, tt.hashID)
		})
	}
}

func TestIDUniquenessAcrossPatterns(t *testing.T) {
	generator := NewIDGenerator()
	tableID := "TEST"
	patterns := []string{"incremental", "timestamp", "hash"}

	idSet := make(map[string]bool)

	for _, pattern := range patterns {
		id, err := generator.GenerateID(tableID, Medium, pattern)
		if err != nil {
			t.Fatalf("Failed to generate ID with pattern %s: %v", pattern, err)
		}

		if idSet[id] {
			t.Errorf("Duplicate ID generated across patterns: %s", id)
		}
		idSet[id] = true
		t.Logf("Generated unique ID with pattern %s: %s", pattern, id)
	}
	t.Logf("Verified uniqueness across all patterns")
}

func TestSpaceReplacementInTableIdentifier(t *testing.T) {
	generator := NewIDGenerator()

	for _, tt := range SpaceReplacementTests {
		t.Run(tt.input, func(t *testing.T) {
			id, err := generator.GenerateID(tt.input, Medium, "incremental")
			if err != nil {
				t.Fatalf("Failed to generate ID: %v", err)
			}

			if !strings.HasPrefix(id, tt.expected) {
				t.Errorf("Expected ID to start with %s, got %s", tt.expected, id)
			}
			t.Logf("Successfully handled space replacement: %s -> %s", tt.input, id)
		})
	}
}

// Benchmark tests
func BenchmarkGenerateIncrementalID(b *testing.B) {
	generator := NewIDGenerator()
	tableID := "TEST"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id, err := generator.GenerateID(tableID, Medium, "incremental")
		if err != nil {
			b.Fatalf("Failed to generate ID: %v", err)
		}
		b.Logf("Generated incremental ID: %s", id)
	}
}

func BenchmarkGenerateTimestampID(b *testing.B) {
	generator := NewIDGenerator()
	tableID := "TEST"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id, err := generator.GenerateID(tableID, Medium, "timestamp")
		if err != nil {
			b.Fatalf("Failed to generate ID: %v", err)
		}
		b.Logf("Generated timestamp ID: %s", id)
	}
}

func BenchmarkGenerateHashID(b *testing.B) {
	generator := NewIDGenerator()
	tableID := "TEST"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id, err := generator.GenerateID(tableID, Medium, "hash")
		if err != nil {
			b.Fatalf("Failed to generate ID: %v", err)
		}
		b.Logf("Generated hash ID: %s", id)
	}
}
