# Preload and Select Support for FilterBuilder and Filter

## Status
Design Phase - Awaiting Architecture Review

## Problem Statement

Currently, the FilterBuilder and Filter structs do not support:
1. **GORM's Preload()** functionality for eager loading relationships - causing N+1 query problems
2. **GORM's Select()** functionality for loading only specific fields - causing unnecessary data transfer

This forces developers to either:
- Make N+1 queries when accessing relationships
- Load entire models when only a few fields are needed
- Manually bypass the filter system and use raw GORM queries
- Load relationships in the service layer after fetching data

This creates performance issues and inconsistent data access patterns across the codebase.

## Current Limitation

```go
// Current approach - no preload or select support
filter := base.NewFilterBuilder().
    Where("crop_id", base.OpEqual, cropID).
    Where("is_active", base.OpEqual, true).
    Build()

cropStages, err := repo.Find(ctx, filter)
// cropStages[0].Stage is nil - not loaded (N+1 problem)
// All fields are loaded even if only ID and Name are needed
```

## Current Architecture Analysis

### Filter Structure (pkg/base/filters.go)
- `Filter` struct: Contains Group, Sort, Page, PageSize, Limit, Offset
- `FilterBuilder`: Provides fluent API for building filters
- `FilterEvaluator`: Evaluates filters in-memory

### Database Layer (pkg/db/postgres.go)
- `PostgresManager.List()`: Applies filters, sorting, and pagination at lines 518-559
- Uses `applyFilterGroup()` to recursively apply filter conditions
- No preload or select support currently exists

## Proposed Solution

### 1. Extend Filter Struct

Add preload and select support to the `Filter` struct in `pkg/base/filters.go`:

```go
// Filter represents the complete filter structure
type Filter struct {
    Group    FilterGroup `json:"group"`
    Sort     []SortField `json:"sort,omitempty"`
    Page     int         `json:"page,omitempty"`
    PageSize int         `json:"page_size,omitempty"`
    Limit    int         `json:"limit,omitempty"`
    Offset   int         `json:"offset,omitempty"`
    Preloads []Preload   `json:"preloads,omitempty"`  // NEW
    Selects  []string    `json:"selects,omitempty"`   // NEW
}

// Preload represents a relationship to eager load
type Preload struct {
    Relation   string        `json:"relation"`   // e.g., "Stage", "Crop.Variety"
    Conditions []interface{} `json:"conditions,omitempty"` // Optional preload conditions
}
```

### 2. Add Preload and Select Methods to FilterBuilder

Add fluent API methods to `FilterBuilder` in `pkg/base/filters.go`:

```go
// Preload adds a relationship to preload
func (fb *FilterBuilder) Preload(relation string, conditions ...interface{}) *FilterBuilder {
    if fb.filter.Preloads == nil {
        fb.filter.Preloads = []Preload{}
    }
    fb.filter.Preloads = append(fb.filter.Preloads, Preload{
        Relation:   relation,
        Conditions: conditions,
    })
    return fb
}

// Select adds fields to select (load only specific fields)
func (fb *FilterBuilder) Select(fields ...string) *FilterBuilder {
    fb.filter.Selects = append(fb.filter.Selects, fields...)
    return fb
}
```

### 3. Update PostgresManager.List() to Apply Preloads and Selects

Modify `pkg/db/postgres.go` List method starting at line 518:

```go
// List retrieves records from PostgreSQL with filter support including pagination, sorting, preloads, and selects
func (pm *PostgresManager) List(ctx context.Context, filter *base.Filter, model interface{}) error {
    db, err := pm.GetDB(ctx, true)
    if err != nil {
        return fmt.Errorf("failed to get database connection: %w", err)
    }

    // Set the model/table context for GORM
    query := db.WithContext(ctx).Model(model)

    // Apply field selection if provided (before other operations for performance)
    if filter != nil && len(filter.Selects) > 0 {
        query = query.Select(filter.Selects)
    }

    // Apply filter conditions if provided
    if filter != nil {
        pm.applyFilterGroup(query, &filter.Group)
    }

    // Apply preloads if provided
    if filter != nil && len(filter.Preloads) > 0 {
        for _, preload := range filter.Preloads {
            if len(preload.Conditions) > 0 {
                query = query.Preload(preload.Relation, preload.Conditions...)
            } else {
                query = query.Preload(preload.Relation)
            }
        }
    }

    // Apply sorting if provided
    if filter != nil && len(filter.Sort) > 0 {
        for _, sort := range filter.Sort {
            direction := "ASC"
            if sort.Direction == "desc" || sort.Direction == "DESC" {
                direction = "DESC"
            }
            query = query.Order(fmt.Sprintf("%s %s", sort.Field, direction))
        }
    }

    // Apply pagination if provided
    if filter != nil {
        if filter.Limit > 0 {
            query = query.Limit(filter.Limit)
        }
        if filter.Offset > 0 {
            query = query.Offset(filter.Offset)
        }
        // Alternative pagination using Page/PageSize
        if filter.Page > 0 && filter.PageSize > 0 {
            offset := (filter.Page - 1) * filter.PageSize
            query = query.Limit(filter.PageSize).Offset(offset)
        }
    }

    return query.Find(model).Error
}
```

## Usage Examples

### Simple Preload

```go
filter := base.NewFilterBuilder().
    Where("crop_id", base.OpEqual, cropID).
    Preload("Stage").
    Preload("Crop").
    Build()

cropStages, err := repo.Find(ctx, filter)
// cropStages[0].Stage is now loaded
// cropStages[0].Crop is now loaded
```

### Conditional Preload

```go
filter := base.NewFilterBuilder().
    Where("farmer_id", base.OpEqual, farmerID).
    Preload("Farm", "area_ha > ?", 5.0).  // Only load farms > 5 hectares
    Build()
```

### Nested Preload

```go
filter := base.NewFilterBuilder().
    Where("status", base.OpEqual, "ACTIVE").
    Preload("CropCycle.Crop").
    Preload("CropCycle.Farm").
    Build()
```

### Select Specific Fields

```go
filter := base.NewFilterBuilder().
    Where("is_active", base.OpEqual, true).
    Select("id", "name", "area_ha").  // Only load these fields
    Build()

farms, err := repo.Find(ctx, filter)
// Only id, name, and area_ha are loaded (not description, created_at, etc.)
```

### Combined Preload and Select

```go
filter := base.NewFilterBuilder().
    Where("crop_id", base.OpEqual, cropID).
    Select("id", "name", "stage_id", "sequence_number").
    Preload("Stage").  // Preload the Stage relationship
    Build()

cropStages, err := repo.Find(ctx, filter)
// Only specified fields are loaded from crop_stages table
// Full Stage object is preloaded
```

### Performance Optimization Example

```go
// Before: Load all fields for 1000 farms (heavy)
filter := base.NewFilterBuilder().
    Where("district", base.OpEqual, "Pune").
    Limit(1000, 0).
    Build()

// After: Load only required fields (much lighter)
filter := base.NewFilterBuilder().
    Where("district", base.OpEqual, "Pune").
    Select("id", "name", "location").  // ~90% reduction in data transfer
    Limit(1000, 0).
    Build()
```

## Benefits

### Preload Benefits
1. **Performance**: Eliminates N+1 queries by loading relationships in a single query
2. **Consistency**: Maintains use of the filter system without bypassing to raw GORM
3. **Developer Experience**: Fluent API matches GORM's preload patterns
4. **Flexibility**: Supports simple, conditional, and nested preloads

### Select Benefits
1. **Network Efficiency**: Reduces data transfer by loading only required fields
2. **Memory Optimization**: Lower memory footprint especially with large result sets
3. **Database Performance**: Reduces I/O by fetching fewer columns
4. **API Optimization**: Faster JSON serialization with fewer fields

### Combined Benefits
1. **Backward Compatibility**: Existing code continues to work without changes
2. **Unified API**: Single consistent interface for all query optimizations
3. **Composability**: Features work together seamlessly

## Implementation Considerations

### JSON Serialization
- The Preload struct supports JSON serialization for API-based filters
- Conditions containing function references or complex types may not serialize properly
- Selects array serializes cleanly as JSON string array

### In-Memory Fallback
- The in-memory implementation in `BaseFilterableRepository` cannot support preloads or selects
- When using in-memory storage, these features will be silently ignored
- Consider adding debug logging to inform developers

### Error Handling
- Invalid relation names will cause GORM errors during query execution
- Invalid field names in Select will cause GORM errors
- Preload conditions with invalid syntax will fail at query time
- Clear error messages should guide developers to fix issues

### Performance Considerations
- **Preload**: Can impact memory with large relationships; consider pagination
- **Select**: Always improves performance when loading subset of fields
- **Order Matters**: Select is applied first for optimal performance
- **GORM Behavior**: Select affects only the main model, not preloaded associations

### Select with Preload Interaction
- When Select is used, it only affects the main model fields
- Preloaded associations load all their fields by default
- To select fields in preloaded associations, use nested preload queries (future enhancement)

### Testing Strategy
- Unit tests for FilterBuilder Preload and Select methods
- Integration tests with PostgresManager to verify execution
- Business logic tests to validate N+1 query elimination
- Performance benchmarks comparing:
  - With/without preloads
  - With/without selects
  - Combined optimizations
- Memory profiling tests

## Alternative Approaches Considered

### 1. Join-Based Approach
Use GORM's `Joins()` instead of `Preload()`:
- **Pros**: Better performance for simple relationships
- **Cons**: More complex, doesn't handle many-to-many well
- **Decision**: `Preload()` is simpler and more flexible. Consider Joins() as future optimization

### 2. Nested Preload Struct
Support nested preloads with tree structure:
```go
type Preload struct {
    Relation   string
    Conditions []interface{}
    Nested     []Preload
}
```
- **Decision**: Simplified approach - use dot notation (e.g., "CropCycle.Crop") which GORM supports natively

### 3. Select with Map Structure
Use map for field selection with aliases:
```go
Selects  map[string]string  // field -> alias
```
- **Decision**: Keep it simple with string array. Aliases are rare use case.

### 4. Separate SelectFields Method
Have different methods: `Select()`, `SelectFields()`, `SelectColumns()`
- **Decision**: Single `Select()` method matching GORM's API

## Migration Strategy

1. Add Preload struct and Selects field to `Filter` in `pkg/base/filters.go`
2. Add Preload() and Select() methods to `FilterBuilder`
3. Update `pkg/db/postgres.go` List() method to handle preloads and selects
4. Add comprehensive tests
5. Update documentation with examples
6. No breaking changes - fully backward compatible

## Open Questions

1. **Should we validate that preload relations exist on the model?**
   - **Recommendation**: No - let GORM handle validation for flexibility

2. **Should we validate that select fields exist on the model?**
   - **Recommendation**: No - let GORM handle validation, allows for flexibility with custom columns

3. **Should in-memory fallback log a warning when preloads/selects are specified?**
   - **Recommendation**: Yes - add debug log to inform developers

4. **Should we support selecting fields within preloaded associations?**
   - **Recommendation**: Defer to future enhancement - complex to implement

## Dependencies

- No new external dependencies required
- Relies on existing GORM functionality
- GORM version: Current (already supports Preload and Select)

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Performance degradation with large preloads | High | Document best practices; provide examples with pagination |
| Breaking changes to Filter JSON schema | Medium | Use `omitempty` tag; maintain backward compatibility |
| In-memory implementation cannot support features | Low | Document limitation; log warning in debug mode |
| Complex preload conditions may fail | Medium | Provide clear examples; improve error messages |
| Select with missing fields causes GORM errors | Medium | Document expected behavior; let GORM handle validation |
| Developers misuse Select and get incomplete models | Low | Document best practices; provide usage guidelines |

## Success Criteria

1. ✅ FilterBuilder supports Preload() method
2. ✅ FilterBuilder supports Select() method
3. ✅ Filter struct includes Preloads field
4. ✅ Filter struct includes Selects field
5. ✅ PostgresManager.List() applies preloads correctly
6. ✅ PostgresManager.List() applies selects correctly
7. ✅ All tests pass with >90% coverage
8. ✅ Documentation includes usage examples for both features
9. ✅ No performance regression in existing queries
10. ✅ N+1 queries eliminated in test scenarios
11. ✅ Data transfer reduced by >50% in select test scenarios
12. ✅ Preload and Select work together correctly

## Performance Benchmarks (Expected)

| Scenario | Before | After | Improvement |
|----------|--------|-------|-------------|
| Load 100 CropStages with Stage | 101 queries (N+1) | 2 queries | 98% reduction |
| Load 1000 Farms (all fields) | ~500KB transfer | Same | Baseline |
| Load 1000 Farms (3 fields only) | ~500KB transfer | ~50KB transfer | 90% reduction |
| Load CropStages with Stage and Crop | 201 queries | 3 queries | 98.5% reduction |

## Next Steps

1. ✅ Create design document
2. 🔄 Architecture review by sde3-backend-architect agent
3. ⏳ Implementation by sde-backend-engineer agent
4. ⏳ Business logic validation by business-logic-tester agent
5. ⏳ Code review and testing
6. ⏳ Documentation update
7. ⏳ Merge and deploy

## References

- GORM Preload Documentation: https://gorm.io/docs/preload.html
- GORM Select Documentation: https://gorm.io/docs/query.html#Select
- Current Filter Implementation: `pkg/base/filters.go`
- Current PostgresManager: `pkg/db/postgres.go:518-559`
