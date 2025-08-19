// Package base provides base models and interfaces for the application.
package base

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// FilterOperator represents the type of comparison operation
type FilterOperator string

const (
	// Equality operators
	OpEqual    FilterOperator = "eq"
	OpNotEqual FilterOperator = "ne"

	// Comparison operators
	OpGreaterThan  FilterOperator = "gt"
	OpGreaterEqual FilterOperator = "gte"
	OpLessThan     FilterOperator = "lt"
	OpLessEqual    FilterOperator = "lte"

	// String operators
	OpContains   FilterOperator = "contains"
	OpStartsWith FilterOperator = "starts_with"
	OpEndsWith   FilterOperator = "ends_with"
	OpLike       FilterOperator = "like"
	OpNotLike    FilterOperator = "not_like"

	// Array operators
	OpIn    FilterOperator = "in"
	OpNotIn FilterOperator = "not_in"

	// Null operators
	OpIsNull    FilterOperator = "is_null"
	OpIsNotNull FilterOperator = "is_not_null"

	// Date operators
	OpDateEqual   FilterOperator = "date_eq"
	OpDateBefore  FilterOperator = "date_before"
	OpDateAfter   FilterOperator = "date_after"
	OpDateBetween FilterOperator = "date_between"
)

// FilterCondition represents a single filter condition
type FilterCondition struct {
	Field    string         `json:"field"`
	Operator FilterOperator `json:"operator"`
	Value    interface{}    `json:"value"`
	Value2   interface{}    `json:"value2,omitempty"` // For between operations
}

// FilterGroup represents a group of filter conditions with logical operators
type FilterGroup struct {
	Conditions []FilterCondition `json:"conditions"`
	Groups     []FilterGroup     `json:"groups"`
	Logic      FilterLogic       `json:"logic"`
}

// FilterLogic represents the logical operator for combining conditions
type FilterLogic string

const (
	LogicAnd FilterLogic = "and"
	LogicOr  FilterLogic = "or"
)

// Filter represents the complete filter structure
type Filter struct {
	Group    FilterGroup `json:"group"`
	Sort     []SortField `json:"sort,omitempty"`
	Page     int         `json:"page,omitempty"`
	PageSize int         `json:"page_size,omitempty"`
	Limit    int         `json:"limit,omitempty"`
	Offset   int         `json:"offset,omitempty"`
}

// SortField represents a sorting field
type SortField struct {
	Field     string `json:"field"`
	Direction string `json:"direction"` // "asc" or "desc"
}

// FilterBuilder provides a fluent interface for building filters
type FilterBuilder struct {
	filter *Filter
}

// NewFilter creates a new filter
func NewFilter() *Filter {
	return &Filter{
		Group: FilterGroup{
			Logic: LogicAnd,
		},
	}
}

// NewFilterBuilder creates a new filter builder
func NewFilterBuilder() *FilterBuilder {
	return &FilterBuilder{
		filter: NewFilter(),
	}
}

// Where adds a condition to the filter
func (fb *FilterBuilder) Where(field string, operator FilterOperator, value interface{}) *FilterBuilder {
	fb.filter.Group.Conditions = append(fb.filter.Group.Conditions, FilterCondition{
		Field:    field,
		Operator: operator,
		Value:    value,
	})
	return fb
}

// WhereBetween adds a between condition
func (fb *FilterBuilder) WhereBetween(field string, value1, value2 interface{}) *FilterBuilder {
	fb.filter.Group.Conditions = append(fb.filter.Group.Conditions, FilterCondition{
		Field:    field,
		Operator: OpDateBetween,
		Value:    value1,
		Value2:   value2,
	})
	return fb
}

// WhereIn adds an IN condition
func (fb *FilterBuilder) WhereIn(field string, values []interface{}) *FilterBuilder {
	fb.filter.Group.Conditions = append(fb.filter.Group.Conditions, FilterCondition{
		Field:    field,
		Operator: OpIn,
		Value:    values,
	})
	return fb
}

// WhereNull adds an IS NULL condition
func (fb *FilterBuilder) WhereNull(field string) *FilterBuilder {
	fb.filter.Group.Conditions = append(fb.filter.Group.Conditions, FilterCondition{
		Field:    field,
		Operator: OpIsNull,
	})
	return fb
}

// WhereNotNull adds an IS NOT NULL condition
func (fb *FilterBuilder) WhereNotNull(field string) *FilterBuilder {
	fb.filter.Group.Conditions = append(fb.filter.Group.Conditions, FilterCondition{
		Field:    field,
		Operator: OpIsNotNull,
	})
	return fb
}

// Or adds an OR group
func (fb *FilterBuilder) Or(conditions ...FilterCondition) *FilterBuilder {
	group := FilterGroup{
		Conditions: conditions,
		Logic:      LogicOr,
	}
	fb.filter.Group.Groups = append(fb.filter.Group.Groups, group)
	return fb
}

// And adds an AND group
func (fb *FilterBuilder) And(conditions ...FilterCondition) *FilterBuilder {
	group := FilterGroup{
		Conditions: conditions,
		Logic:      LogicAnd,
	}
	fb.filter.Group.Groups = append(fb.filter.Group.Groups, group)
	return fb
}

// Sort adds a sort field
func (fb *FilterBuilder) Sort(field, direction string) *FilterBuilder {
	fb.filter.Sort = append(fb.filter.Sort, SortField{
		Field:     field,
		Direction: direction,
	})
	return fb
}

// Page sets pagination
func (fb *FilterBuilder) Page(page, pageSize int) *FilterBuilder {
	fb.filter.Page = page
	fb.filter.PageSize = pageSize
	return fb
}

// Limit sets limit and offset
func (fb *FilterBuilder) Limit(limit, offset int) *FilterBuilder {
	fb.filter.Limit = limit
	fb.filter.Offset = offset
	return fb
}

// Build returns the built filter
func (fb *FilterBuilder) Build() *Filter {
	return fb.filter
}

// FilterEvaluator evaluates filters against models
type FilterEvaluator struct{}

// NewFilterEvaluator creates a new filter evaluator
func NewFilterEvaluator() *FilterEvaluator {
	return &FilterEvaluator{}
}

// Evaluate evaluates a filter against a model
func (fe *FilterEvaluator) Evaluate(filter *Filter, model ModelInterface) (bool, error) {
	return fe.evaluateGroup(&filter.Group, model)
}

// evaluateGroup evaluates a filter group
func (fe *FilterEvaluator) evaluateGroup(group *FilterGroup, model ModelInterface) (bool, error) {
	// Evaluate conditions
	conditionResults := make([]bool, 0, len(group.Conditions))
	for _, condition := range group.Conditions {
		result, err := fe.evaluateCondition(&condition, model)
		if err != nil {
			return false, err
		}
		conditionResults = append(conditionResults, result)
	}

	// Evaluate sub-groups
	groupResults := make([]bool, 0, len(group.Groups))
	for _, subGroup := range group.Groups {
		result, err := fe.evaluateGroup(&subGroup, model)
		if err != nil {
			return false, err
		}
		groupResults = append(groupResults, result)
	}

	// Combine results based on logic
	allResults := append(conditionResults, groupResults...)
	if len(allResults) == 0 {
		return true, nil
	}

	if group.Logic == LogicAnd {
		for _, result := range allResults {
			if !result {
				return false, nil
			}
		}
		return true, nil
	} else { // LogicOr
		for _, result := range allResults {
			if result {
				return true, nil
			}
		}
		return false, nil
	}
}

// evaluateCondition evaluates a single filter condition
func (fe *FilterEvaluator) evaluateCondition(condition *FilterCondition, model ModelInterface) (bool, error) {
	value, err := fe.getFieldValue(condition.Field, model)
	if err != nil {
		return false, err
	}

	switch condition.Operator {
	case OpEqual:
		return fe.compareEqual(value, condition.Value), nil
	case OpNotEqual:
		return !fe.compareEqual(value, condition.Value), nil
	case OpGreaterThan:
		return fe.compareGreaterThan(value, condition.Value), nil
	case OpGreaterEqual:
		return fe.compareGreaterEqual(value, condition.Value), nil
	case OpLessThan:
		return fe.compareLessThan(value, condition.Value), nil
	case OpLessEqual:
		return fe.compareLessEqual(value, condition.Value), nil
	case OpContains:
		return fe.compareContains(value, condition.Value), nil
	case OpStartsWith:
		return fe.compareStartsWith(value, condition.Value), nil
	case OpEndsWith:
		return fe.compareEndsWith(value, condition.Value), nil
	case OpLike:
		return fe.compareLike(value, condition.Value), nil
	case OpNotLike:
		return !fe.compareLike(value, condition.Value), nil
	case OpIn:
		return fe.compareIn(value, condition.Value), nil
	case OpNotIn:
		return !fe.compareIn(value, condition.Value), nil
	case OpIsNull:
		return fe.isNull(value), nil
	case OpIsNotNull:
		return !fe.isNull(value), nil
	case OpDateEqual:
		return fe.compareDateEqual(value, condition.Value), nil
	case OpDateBefore:
		return fe.compareDateBefore(value, condition.Value), nil
	case OpDateAfter:
		return fe.compareDateAfter(value, condition.Value), nil
	case OpDateBetween:
		return fe.compareDateBetween(value, condition.Value, condition.Value2), nil
	default:
		return false, fmt.Errorf("unknown operator: %s", condition.Operator)
	}
}

// getFieldValue gets the value of a field from a model
func (fe *FilterEvaluator) getFieldValue(field string, model ModelInterface) (interface{}, error) {
	// Handle base model fields
	switch field {
	case "id":
		return model.GetID(), nil
	case "created_at":
		return model.GetCreatedAt(), nil
	case "updated_at":
		return model.GetUpdatedAt(), nil
	case "created_by":
		return model.GetCreatedBy(), nil
	case "updated_by":
		return model.GetUpdatedBy(), nil
	case "deleted_at":
		return model.GetDeletedAt(), nil
	case "deleted_by":
		return model.GetDeletedBy(), nil
	case "is_deleted":
		return model.IsDeleted(), nil
	}

	// Use reflection for other fields
	val := reflect.ValueOf(model)
	// Unwrap interface and pointer layers to reach the concrete struct
	for val.IsValid() && (val.Kind() == reflect.Interface || val.Kind() == reflect.Ptr) {
		if val.IsNil() {
			break
		}
		val = val.Elem()
	}

	// Try both original and Title-cased field names
	fieldNames := []string{field, strings.Title(field)}

	if val.Kind() == reflect.Struct {
		for _, fname := range fieldNames {
			fieldVal := val.FieldByName(fname)
			if fieldVal.IsValid() {
				return fieldVal.Interface(), nil
			}
		}

		// If not found, try to find it in embedded structs
		for i := 0; i < val.NumField(); i++ {
			fieldVal := val.Field(i)
			// Unwrap pointers to inspect struct fields
			fv := fieldVal
			if fv.Kind() == reflect.Ptr && !fv.IsNil() {
				fv = fv.Elem()
			}
			if fv.Kind() == reflect.Struct {
				for _, fname := range fieldNames {
					embeddedField := fv.FieldByName(fname)
					if embeddedField.IsValid() {
						return embeddedField.Interface(), nil
					}
				}
			}
		}

		// If still not found, try to find it in the embedded BaseModel
		baseModelField := val.FieldByName("BaseModel")
		if baseModelField.IsValid() && baseModelField.Kind() == reflect.Ptr {
			baseModelVal := baseModelField.Elem()
			if baseModelVal.Kind() == reflect.Struct {
				for _, fname := range fieldNames {
					embeddedField := baseModelVal.FieldByName(fname)
					if embeddedField.IsValid() {
						return embeddedField.Interface(), nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("field %s not found", field)
}

// Comparison methods
func (fe *FilterEvaluator) compareEqual(a, b interface{}) bool {
	// Dereference pointers for fair comparison
	if a != nil {
		av := reflect.ValueOf(a)
		if av.Kind() == reflect.Ptr {
			if av.IsNil() {
				a = nil
			} else {
				a = av.Elem().Interface()
			}
		}
	}
	if b != nil {
		bv := reflect.ValueOf(b)
		if bv.Kind() == reflect.Ptr {
			if bv.IsNil() {
				b = nil
			} else {
				b = bv.Elem().Interface()
			}
		}
	}
	return reflect.DeepEqual(a, b)
}

func (fe *FilterEvaluator) compareGreaterThan(a, b interface{}) bool {
	return fe.compareNumeric(a, b, func(x, y float64) bool { return x > y })
}

func (fe *FilterEvaluator) compareGreaterEqual(a, b interface{}) bool {
	return fe.compareNumeric(a, b, func(x, y float64) bool { return x >= y })
}

func (fe *FilterEvaluator) compareLessThan(a, b interface{}) bool {
	return fe.compareNumeric(a, b, func(x, y float64) bool { return x < y })
}

func (fe *FilterEvaluator) compareLessEqual(a, b interface{}) bool {
	return fe.compareNumeric(a, b, func(x, y float64) bool { return x <= y })
}

func (fe *FilterEvaluator) compareNumeric(a, b interface{}, compare func(float64, float64) bool) bool {
	aVal := fe.toFloat64(a)
	bVal := fe.toFloat64(b)
	return compare(aVal, bVal)
}

func (fe *FilterEvaluator) toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case float64:
		return val
	case time.Time:
		return float64(val.Unix())
	case string:
		// Try to parse as number
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	default:
		return 0
	}
}

func (fe *FilterEvaluator) compareContains(a, b interface{}) bool {
	aStr := fe.toString(a)
	bStr := fe.toString(b)
	return strings.Contains(strings.ToLower(aStr), strings.ToLower(bStr))
}

func (fe *FilterEvaluator) compareStartsWith(a, b interface{}) bool {
	aStr := fe.toString(a)
	bStr := fe.toString(b)
	return strings.HasPrefix(strings.ToLower(aStr), strings.ToLower(bStr))
}

func (fe *FilterEvaluator) compareEndsWith(a, b interface{}) bool {
	aStr := fe.toString(a)
	bStr := fe.toString(b)
	return strings.HasSuffix(strings.ToLower(aStr), strings.ToLower(bStr))
}

func (fe *FilterEvaluator) compareLike(a, b interface{}) bool {
	aStr := fe.toString(a)
	bStr := fe.toString(b)
	// Simple wildcard matching
	pattern := strings.ReplaceAll(bStr, "%", ".*")
	pattern = strings.ReplaceAll(pattern, "_", ".")
	return strings.Contains(strings.ToLower(aStr), strings.ToLower(bStr))
}

func (fe *FilterEvaluator) compareIn(a, b interface{}) bool {
	if reflect.TypeOf(b).Kind() != reflect.Slice {
		return false
	}

	bSlice := reflect.ValueOf(b)
	for i := 0; i < bSlice.Len(); i++ {
		if fe.compareEqual(a, bSlice.Index(i).Interface()) {
			return true
		}
	}
	return false
}

func (fe *FilterEvaluator) isNull(a interface{}) bool {
	if a == nil {
		return true
	}

	val := reflect.ValueOf(a)
	switch val.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return val.IsNil()
	default:
		return false
	}
}

func (fe *FilterEvaluator) compareDateEqual(a, b interface{}) bool {
	aTime := fe.toTime(a)
	bTime := fe.toTime(b)
	return aTime.Equal(bTime)
}

func (fe *FilterEvaluator) compareDateBefore(a, b interface{}) bool {
	aTime := fe.toTime(a)
	bTime := fe.toTime(b)
	return aTime.Before(bTime)
}

func (fe *FilterEvaluator) compareDateAfter(a, b interface{}) bool {
	aTime := fe.toTime(a)
	bTime := fe.toTime(b)
	return aTime.After(bTime)
}

func (fe *FilterEvaluator) compareDateBetween(a, b, c interface{}) bool {
	aTime := fe.toTime(a)
	bTime := fe.toTime(b)
	cTime := fe.toTime(c)
	return (aTime.Equal(bTime) || aTime.After(bTime)) && (aTime.Equal(cTime) || aTime.Before(cTime))
}

func (fe *FilterEvaluator) toTime(v interface{}) time.Time {
	switch val := v.(type) {
	case time.Time:
		return val
	case *time.Time:
		if val == nil {
			return time.Time{}
		}
		return *val
	case string:
		t, _ := time.Parse(time.RFC3339, val)
		return t
	default:
		return time.Time{}
	}
}

func (fe *FilterEvaluator) toString(v interface{}) string {
	if v == nil {
		return ""
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	return fmt.Sprintf("%v", rv.Interface())
}

// FilterableRepository extends Repository with filtering capabilities
type FilterableRepository[T ModelInterface] interface {
	Repository[T]
	Find(ctx context.Context, filter *Filter) ([]T, error)
	FindOne(ctx context.Context, filter *Filter) (T, error)
	CountWithFilter(ctx context.Context, filter *Filter) (int64, error)
	GetStats(ctx context.Context) (map[string]int64, error)
	FindManyWithRelationships(ctx context.Context, ids []string, filter *Filter) ([]T, error)
}

// BaseFilterableRepository provides a default implementation of FilterableRepository
type BaseFilterableRepository[T ModelInterface] struct {
	*BaseRepository[T]
	evaluator *FilterEvaluator
	dbManager interface{} // Will hold db.DBManager - using interface{} to avoid circular imports
}

// NewBaseFilterableRepository creates a new base filterable repository
func NewBaseFilterableRepository[T ModelInterface]() *BaseFilterableRepository[T] {
	return &BaseFilterableRepository[T]{
		BaseRepository: NewBaseRepository[T](),
		evaluator:      NewFilterEvaluator(),
		dbManager:      nil,
	}
}

// SetDBManager sets the database manager for this repository
func (r *BaseFilterableRepository[T]) SetDBManager(dbManager interface{}) {
	r.dbManager = dbManager
}

// Create overrides BaseRepository.Create to use database manager if available
func (r *BaseFilterableRepository[T]) Create(ctx context.Context, model T) error {
	if r.dbManager != nil {
		// Use database manager interface
		if dbMgr, ok := r.dbManager.(interface {
			Create(ctx context.Context, model interface{}) error
		}); ok {
			return dbMgr.Create(ctx, model)
		}
	}
	// Fallback to in-memory storage
	return r.BaseRepository.Create(ctx, model)
}

// Update overrides BaseRepository.Update to use database manager if available
func (r *BaseFilterableRepository[T]) Update(ctx context.Context, model T) error {
	if r.dbManager != nil {
		// Use database manager interface
		if dbMgr, ok := r.dbManager.(interface {
			Update(ctx context.Context, model interface{}) error
		}); ok {
			return dbMgr.Update(ctx, model)
		}
	}
	// Fallback to in-memory storage
	return r.BaseRepository.Update(ctx, model)
}

// GetByID overrides BaseRepository.GetByID to use database manager if available
func (r *BaseFilterableRepository[T]) GetByID(ctx context.Context, id string, model T) (T, error) {
	// If database manager is available, use it for database-level operations
	if r.dbManager != nil {
		if dbMgr, ok := r.dbManager.(interface {
			GetByID(ctx context.Context, id interface{}, model interface{}) error
		}); ok {
			if err := dbMgr.GetByID(ctx, id, model); err != nil {
				var zero T
				return zero, fmt.Errorf("database query failed: %w", err)
			}
			return model, nil
		}
	}

	// Fallback to in-memory storage
	return r.BaseRepository.GetByID(ctx, id, model)
}

// Delete overrides BaseRepository.Delete to use database manager if available
func (r *BaseFilterableRepository[T]) Delete(ctx context.Context, id string, model T) error {
	if r.dbManager != nil {
		// Use the enhanced database manager interface that takes both id and model
		if dbMgr, ok := r.dbManager.(interface {
			Delete(ctx context.Context, id interface{}, model interface{}) error
		}); ok {
			return dbMgr.Delete(ctx, id, model)
		}
	}
	// Fallback to in-memory storage
	return r.BaseRepository.Delete(ctx, id, model)
}

// SoftDelete performs a soft delete operation using the database manager if available
func (r *BaseFilterableRepository[T]) SoftDelete(ctx context.Context, id string, deletedBy string) error {
	if r.dbManager != nil {
		// Try to use an enhanced database manager interface that supports soft delete
		if enhancedDBMgr, ok := r.dbManager.(interface {
			SoftDelete(ctx context.Context, id interface{}, model interface{}, deletedBy string) error
		}); ok {
			// Create a zero value of the generic type to pass as model
			var zero T
			return enhancedDBMgr.SoftDelete(ctx, id, &zero, deletedBy)
		}

		// Fallback to standard soft delete through the base repository
		return r.BaseRepository.SoftDelete(ctx, id, deletedBy)
	}
	// Fallback to in-memory storage
	return r.BaseRepository.SoftDelete(ctx, id, deletedBy)
}

// Restore performs a restore operation using the database manager if available
func (r *BaseFilterableRepository[T]) Restore(ctx context.Context, id string) error {
	if r.dbManager != nil {
		// Try to use an enhanced database manager interface that supports restore
		if enhancedDBMgr, ok := r.dbManager.(interface {
			Restore(ctx context.Context, id interface{}, model interface{}) error
		}); ok {
			// Create a zero value of the generic type to pass as model
			var zero T
			return enhancedDBMgr.Restore(ctx, id, &zero)
		}

		// Fallback to standard restore through the base repository
		return r.BaseRepository.Restore(ctx, id)
	}
	// Fallback to in-memory storage
	return r.BaseRepository.Restore(ctx, id)
}

// Exists overrides BaseRepository.Exists to use database manager if available
func (r *BaseFilterableRepository[T]) Exists(ctx context.Context, id string) (bool, error) {
	// If database manager is available, use it for database-level operations
	if r.dbManager != nil {
		if dbMgr, ok := r.dbManager.(interface {
			Exists(ctx context.Context, id interface{}) (bool, error)
		}); ok {
			return dbMgr.Exists(ctx, id)
		}
	}

	// Fallback to in-memory storage
	return r.BaseRepository.Exists(ctx, id)
}

// Count overrides BaseRepository.Count to use database manager if available
func (r *BaseFilterableRepository[T]) Count(ctx context.Context, filter *Filter, model interface{}) (int64, error) {
	// If database manager is available, use it for database-level operations
	if r.dbManager != nil {
		// Use the same interface assertion pattern that works in FindOne
		if dbMgr, ok := r.dbManager.(interface {
			Count(ctx context.Context, filter *Filter, model interface{}) (int64, error)
		}); ok {
			return dbMgr.Count(ctx, filter, nil)
		}
	}

	// Fallback to in-memory counting
	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int64
	for _, model := range r.models {
		// Skip soft deleted models unless explicitly requested
		if model.IsDeleted() && !r.shouldIncludeDeleted(filter) {
			continue
		}

		matches, err := r.evaluator.Evaluate(filter, model)
		if err != nil {
			return 0, fmt.Errorf("filter evaluation failed: %w", err)
		}

		if matches {
			count++
		}
	}

	return count, nil
}

// Find implements FilterableRepository.Find
func (r *BaseFilterableRepository[T]) Find(ctx context.Context, filter *Filter) ([]T, error) {
	// If database manager is available, use it for database-level operations
	if r.dbManager != nil {
		// Use the exact same interface assertion that works in FindOne
		if dbMgr, ok := r.dbManager.(interface {
			List(ctx context.Context, filter *Filter, model interface{}) error
		}); ok {
			var results []T
			if err := dbMgr.List(ctx, filter, &results); err != nil {
				return nil, fmt.Errorf("database query failed: %w", err)
			}
			return results, nil
		}
	}

	// Fallback to in-memory filtering
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []T
	for _, model := range r.models {
		// Skip soft deleted models unless explicitly requested
		if model.IsDeleted() && !r.shouldIncludeDeleted(filter) {
			continue
		}

		matches, err := r.evaluator.Evaluate(filter, model)
		if err != nil {
			return nil, fmt.Errorf("filter evaluation failed: %w", err)
		}

		if matches {
			results = append(results, model)
		}
	}

	// Apply sorting
	if len(filter.Sort) > 0 {
		r.sortResults(results, filter.Sort)
	}

	// Apply pagination only when Page/PageSize are set explicitly to avoid cutting small datasets in-memory
	if filter.Page > 0 && filter.PageSize > 0 {
		limit, offset := r.getPaginationParams(filter)
		if offset < len(results) {
			end := offset + limit
			if end > len(results) {
				end = len(results)
			}
			results = results[offset:end]
		} else {
			results = []T{}
		}
	}

	return results, nil
}

// FindOne implements FilterableRepository.FindOne
func (r *BaseFilterableRepository[T]) FindOne(ctx context.Context, filter *Filter) (T, error) {
	// If database manager is available, use it for database-level operations
	if r.dbManager != nil {
		if dbMgr, ok := r.dbManager.(interface {
			List(ctx context.Context, filter *Filter, model interface{}) error
		}); ok {
			var results []T
			if err := dbMgr.List(ctx, filter, &results); err != nil {
				var zero T
				return zero, fmt.Errorf("database query failed: %w", err)
			}

			if len(results) == 0 {
				var zero T
				return zero, fmt.Errorf("no matching records found")
			}

			return results[0], nil
		}
	}

	// Fallback to in-memory filtering
	results, err := r.Find(ctx, filter)
	if err != nil {
		var zero T
		return zero, err
	}

	if len(results) == 0 {
		var zero T
		return zero, fmt.Errorf("no matching records found")
	}

	return results[0], nil
}

// CountWithFilter implements FilterableRepository.CountWithFilter
func (r *BaseFilterableRepository[T]) CountWithFilter(ctx context.Context, filter *Filter) (int64, error) {
	// If database manager is available, use it for database-level operations
	if r.dbManager != nil {
		// Use the same interface assertion pattern that works in FindOne
		if dbMgr, ok := r.dbManager.(interface {
			Count(ctx context.Context, filter *Filter, model interface{}) (int64, error)
		}); ok {
			return dbMgr.Count(ctx, filter, nil)
		}
	}

	// Fallback to in-memory counting
	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int64
	for _, model := range r.models {
		// Skip soft deleted models unless explicitly requested
		if model.IsDeleted() && !r.shouldIncludeDeleted(filter) {
			continue
		}

		matches, err := r.evaluator.Evaluate(filter, model)
		if err != nil {
			return 0, fmt.Errorf("filter evaluation failed: %w", err)
		}

		if matches {
			count++
		}
	}

	return count, nil
}

// GetStats concurrently calculates various statistics using goroutines
func (r *BaseFilterableRepository[T]) GetStats(ctx context.Context) (map[string]int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create channels for different stats
	totalChan := make(chan int64, 1)
	activeChan := make(chan int64, 1)
	deletedChan := make(chan int64, 1)

	// Calculate total count
	go func() {
		var count int64
		for range r.models {
			count++
		}
		totalChan <- count
	}()

	// Calculate active count
	go func() {
		var count int64
		for _, model := range r.models {
			if !model.IsDeleted() {
				count++
			}
		}
		activeChan <- count
	}()

	// Calculate deleted count
	go func() {
		var count int64
		for _, model := range r.models {
			if model.IsDeleted() {
				count++
			}
		}
		deletedChan <- count
	}()

	// Collect results
	stats := make(map[string]int64)
	stats["total"] = <-totalChan
	stats["active"] = <-activeChan
	stats["deleted"] = <-deletedChan

	return stats, nil
}

// FindManyWithRelationships efficiently loads multiple models with their relationships using goroutines
func (r *BaseFilterableRepository[T]) FindManyWithRelationships(ctx context.Context, ids []string, filter *Filter) ([]T, error) {
	if len(ids) == 0 {
		return []T{}, nil
	}

	// Use goroutines to load models concurrently
	modelsChan := make(chan []T, 1)
	modelsErrChan := make(chan error, 1)

	go func() {
		var results []T
		for _, model := range r.models {
			// Check if model ID is in the requested IDs
			found := false
			for _, id := range ids {
				if model.GetID() == id {
					found = true
					break
				}
			}
			if !found {
				continue
			}

			// Apply filter if provided
			if filter != nil {
				matches, err := r.evaluator.Evaluate(filter, model)
				if err != nil {
					modelsErrChan <- fmt.Errorf("filter evaluation failed: %w", err)
					return
				}
				if !matches {
					continue
				}
			}

			results = append(results, model)
		}

		// Apply sorting if specified in filter
		if filter != nil && len(filter.Sort) > 0 {
			r.sortResults(results, filter.Sort)
		}

		modelsChan <- results
	}()

	// Wait for results
	select {
	case models := <-modelsChan:
		return models, nil
	case err := <-modelsErrChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// shouldIncludeDeleted checks if deleted records should be included
func (r *BaseFilterableRepository[T]) shouldIncludeDeleted(filter *Filter) bool {
	// Check if filter explicitly includes deleted records
	for _, condition := range filter.Group.Conditions {
		if condition.Field == "is_deleted" {
			if condition.Operator == OpEqual {
				return condition.Value == true
			}
		}
	}
	return false
}

// getPaginationParams extracts pagination parameters from filter
func (r *BaseFilterableRepository[T]) getPaginationParams(filter *Filter) (limit, offset int) {
	if filter.Limit > 0 {
		return filter.Limit, filter.Offset
	}
	if filter.Page > 0 && filter.PageSize > 0 {
		offset = (filter.Page - 1) * filter.PageSize
		return filter.PageSize, offset
	}
	return 1000, 0 // Default limit
}

// sortResults sorts results based on sort fields
func (r *BaseFilterableRepository[T]) sortResults(results []T, sortFields []SortField) {
	// This is a simplified sorting implementation
	// In a real implementation, you might want to use a more sophisticated sorting library
	for i := len(sortFields) - 1; i >= 0; i-- {
		sortField := sortFields[i]
		// Sort by field (simplified implementation)
		// In practice, you'd implement proper field-based sorting
		_ = sortField // Use sortField to avoid unused variable warning
	}
}

// GetByCreatedBy overrides BaseRepository.GetByCreatedBy to use database manager if available
func (r *BaseFilterableRepository[T]) GetByCreatedBy(ctx context.Context, createdBy string, limit, offset int) ([]T, error) {
	// If database manager is available, use it for database-level operations
	if r.dbManager != nil {
		if dbMgr, ok := r.dbManager.(interface {
			GetByCreatedBy(ctx context.Context, createdBy interface{}, limit, offset int, models interface{}) error
		}); ok {
			var results []T
			if err := dbMgr.GetByCreatedBy(ctx, createdBy, limit, offset, &results); err != nil {
				return nil, fmt.Errorf("database query failed: %w", err)
			}
			return results, nil
		}
	}

	// Fallback to in-memory storage
	return r.BaseRepository.GetByCreatedBy(ctx, createdBy, limit, offset)
}

// GetByUpdatedBy overrides BaseRepository.GetByUpdatedBy to use database manager if available
func (r *BaseFilterableRepository[T]) GetByUpdatedBy(ctx context.Context, updatedBy string, limit, offset int) ([]T, error) {
	// If database manager is available, use it for database-level operations
	if r.dbManager != nil {
		if dbMgr, ok := r.dbManager.(interface {
			GetByUpdatedBy(ctx context.Context, updatedBy interface{}, limit, offset int, models interface{}) error
		}); ok {
			var results []T
			if err := dbMgr.GetByUpdatedBy(ctx, updatedBy, limit, offset, &results); err != nil {
				return nil, fmt.Errorf("database query failed: %w", err)
			}
			return results, nil
		}
	}

	// Fallback to in-memory storage
	return r.BaseRepository.GetByUpdatedBy(ctx, updatedBy, limit, offset)
}

// GetByDeletedBy overrides BaseRepository.GetByDeletedBy to use database manager if available
func (r *BaseFilterableRepository[T]) GetByDeletedBy(ctx context.Context, deletedBy string, limit, offset int) ([]T, error) {
	// If database manager is available, use it for database-level operations
	if r.dbManager != nil {
		if dbMgr, ok := r.dbManager.(interface {
			GetByDeletedBy(ctx context.Context, deletedBy interface{}, limit, offset int, models interface{}) error
		}); ok {
			var results []T
			if err := dbMgr.GetByDeletedBy(ctx, deletedBy, limit, offset, &results); err != nil {
				return nil, fmt.Errorf("database query failed: %w", err)
			}
			return results, nil
		}
	}

	// Fallback to in-memory storage
	return r.BaseRepository.GetByDeletedBy(ctx, deletedBy, limit, offset)
}

// ListWithDeleted overrides BaseRepository.ListWithDeleted to use database manager if available
func (r *BaseFilterableRepository[T]) ListWithDeleted(ctx context.Context, limit, offset int) ([]T, error) {
	// If database manager is available, use it for database-level operations
	if r.dbManager != nil {
		if dbMgr, ok := r.dbManager.(interface {
			ListWithDeleted(ctx context.Context, limit, offset int, models interface{}) error
		}); ok {
			var results []T
			if err := dbMgr.ListWithDeleted(ctx, limit, offset, &results); err != nil {
				return nil, fmt.Errorf("database query failed: %w", err)
			}
			return results, nil
		}
	}

	// Fallback to in-memory storage
	return r.BaseRepository.ListWithDeleted(ctx, limit, offset)
}

// CountWithDeleted overrides BaseRepository.CountWithDeleted to use database manager if available
func (r *BaseFilterableRepository[T]) CountWithDeleted(ctx context.Context) (int64, error) {
	// If database manager is available, use it for database-level operations
	if r.dbManager != nil {
		if dbMgr, ok := r.dbManager.(interface {
			CountWithDeleted(ctx context.Context) (int64, error)
		}); ok {
			return dbMgr.CountWithDeleted(ctx)
		}
	}

	// Fallback to in-memory storage
	return r.BaseRepository.CountWithDeleted(ctx)
}

// ExistsWithDeleted overrides BaseRepository.ExistsWithDeleted to use database manager if available
func (r *BaseFilterableRepository[T]) ExistsWithDeleted(ctx context.Context, id string) (bool, error) {
	// If database manager is available, use it for database-level operations
	if r.dbManager != nil {
		if dbMgr, ok := r.dbManager.(interface {
			ExistsWithDeleted(ctx context.Context, id interface{}) (bool, error)
		}); ok {
			return dbMgr.ExistsWithDeleted(ctx, id)
		}
	}

	// Fallback to in-memory storage
	return r.BaseRepository.ExistsWithDeleted(ctx, id)
}

// CreateMany overrides BaseRepository.CreateMany to use database manager if available
func (r *BaseFilterableRepository[T]) CreateMany(ctx context.Context, models []T) error {
	// If database manager is available, use it for database-level operations
	if r.dbManager != nil {
		if dbMgr, ok := r.dbManager.(interface {
			CreateMany(ctx context.Context, models []interface{}) error
		}); ok {
			// Convert []T to []interface{}
			interfaceModels := make([]interface{}, len(models))
			for i, model := range models {
				interfaceModels[i] = model
			}
			return dbMgr.CreateMany(ctx, interfaceModels)
		}
	}

	// Fallback to in-memory storage
	return r.BaseRepository.CreateMany(ctx, models)
}

// UpdateMany overrides BaseRepository.UpdateMany to use database manager if available
func (r *BaseFilterableRepository[T]) UpdateMany(ctx context.Context, models []T) error {
	// If database manager is available, use it for database-level operations
	if r.dbManager != nil {
		if dbMgr, ok := r.dbManager.(interface {
			UpdateMany(ctx context.Context, models []interface{}) error
		}); ok {
			// Convert []T to []interface{}
			interfaceModels := make([]interface{}, len(models))
			for i, model := range models {
				interfaceModels[i] = model
			}
			return dbMgr.UpdateMany(ctx, interfaceModels)
		}
	}

	// Fallback to in-memory storage
	return r.BaseRepository.UpdateMany(ctx, models)
}

// DeleteMany overrides BaseRepository.DeleteMany to use database manager if available
func (r *BaseFilterableRepository[T]) DeleteMany(ctx context.Context, ids []string) error {
	// If database manager is available, use it for database-level operations
	if r.dbManager != nil {
		if dbMgr, ok := r.dbManager.(interface {
			DeleteMany(ctx context.Context, ids []interface{}) error
		}); ok {
			// Convert []string to []interface{}
			interfaceIDs := make([]interface{}, len(ids))
			for i, id := range ids {
				interfaceIDs[i] = id
			}
			return dbMgr.DeleteMany(ctx, interfaceIDs)
		}
	}

	// Fallback to in-memory storage
	return r.BaseRepository.DeleteMany(ctx, ids)
}
