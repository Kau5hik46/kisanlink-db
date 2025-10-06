// Package base provides base interfaces and structures for API operations.
package base

import (
	"fmt"
	"time"
)

// RequestInterface defines the base interface that all API requests should implement.
type RequestInterface interface {
	// Validate validates the request data
	Validate() error
	// GetUserID returns the user ID making the request (if authenticated)
	GetUserID() string
	// SetUserID sets the user ID making the request
	SetUserID(userID string)
}

// BaseRequest provides a default implementation of RequestInterface.
// @Description Base request structure with user ID (hidden from JSON)
type BaseRequest struct {
	UserID string `json:"-" example:"USER123456789"` // Hidden from JSON serialization
}

// Validate provides a default validation implementation.
func (r *BaseRequest) Validate() error {
	// Default implementation - can be overridden in specific requests
	return nil
}

// GetUserID returns the user ID making the request.
func (r *BaseRequest) GetUserID() string {
	return r.UserID
}

// SetUserID sets the user ID making the request.
func (r *BaseRequest) SetUserID(userID string) {
	r.UserID = userID
}

// ResponseInterface defines the base interface for all API responses.
type ResponseInterface interface {
	// IsSuccess returns whether the response indicates success
	IsSuccess() bool
	// GetMessage returns the response message
	GetMessage() string
	// GetData returns the response data
	GetData() interface{}
	// GetError returns the response error (if any)
	GetError() ErrorInterface
}

// BaseResponse represents a standard API response structure.
type BaseResponse struct {
	Success   bool           `json:"success"`
	Message   string         `json:"message"`
	Data      interface{}    `json:"data,omitempty"`
	Error     ErrorInterface `json:"error,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	RequestID string         `json:"request_id,omitempty"`
}

// IsSuccess returns whether the response indicates success.
func (r *BaseResponse) IsSuccess() bool {
	return r.Success
}

// GetMessage returns the response message.
func (r *BaseResponse) GetMessage() string {
	return r.Message
}

// GetData returns the response data.
func (r *BaseResponse) GetData() interface{} {
	return r.Data
}

// GetError returns the response error.
func (r *BaseResponse) GetError() ErrorInterface {
	return r.Error
}

// NewSuccessResponse creates a new successful response.
func NewSuccessResponse(message string, data interface{}) *BaseResponse {
	return &BaseResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Error:     nil,
		Timestamp: time.Now(),
	}
}

// NewErrorResponse creates a new error response.
func NewErrorResponse(message string, err ErrorInterface) *BaseResponse {
	return &BaseResponse{
		Success:   false,
		Message:   message,
		Data:      nil,
		Error:     err,
		Timestamp: time.Now(),
	}
}

// PaginatedResponse represents a paginated API response.
type PaginatedResponse struct {
	BaseResponse
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// NewPaginatedResponse creates a new paginated response.
func NewPaginatedResponse(message string, data interface{}, pagination *PaginationInfo) *PaginatedResponse {
	return &PaginatedResponse{
		BaseResponse: BaseResponse{
			Success:   true,
			Message:   message,
			Data:      data,
			Error:     nil,
			Timestamp: time.Now(),
		},
		Pagination: pagination,
	}
}

// PaginationInfo represents pagination metadata.
type PaginationInfo struct {
	Page       int  `json:"page"`
	PerPage    int  `json:"per_page"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// NewPaginationInfo creates pagination info with calculated fields.
func NewPaginationInfo(page, perPage, total int) *PaginationInfo {
	// Safety check to prevent division by zero
	if perPage <= 0 {
		perPage = 1
	}
	if page <= 0 {
		page = 1
	}

	totalPages := (total + perPage - 1) / perPage // Ceiling division
	if totalPages == 0 {
		totalPages = 1
	}

	return &PaginationInfo{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

// ErrorInterface defines the base interface for API errors.
type ErrorInterface interface {
	// Error returns the error message (implements Go's error interface)
	Error() string
	// GetCode returns the error code
	GetCode() string
	// GetMessage returns the error message
	GetMessage() string
	// GetDetails returns additional error details
	GetDetails() string
	// GetStatusCode returns the HTTP status code
	GetStatusCode() int
	// IsRetryable returns whether the error is retryable
	IsRetryable() bool
}

// BaseError provides a default implementation of ErrorInterface.
type BaseError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Details    string `json:"details,omitempty"`
	StatusCode int    `json:"-"` // HTTP status code, not included in JSON
	Retryable  bool   `json:"-"` // Whether error is retryable, not included in JSON
}

// Error implements the Go error interface.
func (e *BaseError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// GetCode returns the error code.
func (e *BaseError) GetCode() string {
	return e.Code
}

// GetMessage returns the error message.
func (e *BaseError) GetMessage() string {
	return e.Message
}

// GetDetails returns additional error details.
func (e *BaseError) GetDetails() string {
	return e.Details
}

// GetStatusCode returns the HTTP status code.
func (e *BaseError) GetStatusCode() int {
	return e.StatusCode
}

// IsRetryable returns whether the error is retryable.
func (e *BaseError) IsRetryable() bool {
	return e.Retryable
}

// Common error constructors

// NewValidationError creates a validation error.
func NewValidationError(message string, details string) *BaseError {
	return &BaseError{
		Code:       "VALIDATION_ERROR",
		Message:    message,
		Details:    details,
		StatusCode: 400,
		Retryable:  false,
	}
}

// NewNotFoundError creates a not found error.
func NewNotFoundError(resource string, id string) *BaseError {
	return &BaseError{
		Code:       "NOT_FOUND",
		Message:    fmt.Sprintf("%s not found", resource),
		Details:    fmt.Sprintf("Resource with ID '%s' does not exist", id),
		StatusCode: 404,
		Retryable:  false,
	}
}

// NewUnauthorizedError creates an unauthorized error.
func NewUnauthorizedError(message string) *BaseError {
	return &BaseError{
		Code:       "UNAUTHORIZED",
		Message:    message,
		Details:    "",
		StatusCode: 401,
		Retryable:  false,
	}
}

// NewForbiddenError creates a forbidden error.
func NewForbiddenError(message string) *BaseError {
	return &BaseError{
		Code:       "FORBIDDEN",
		Message:    message,
		Details:    "",
		StatusCode: 403,
		Retryable:  false,
	}
}

// NewConflictError creates a conflict error.
func NewConflictError(resource string, details string) *BaseError {
	return &BaseError{
		Code:       "CONFLICT",
		Message:    fmt.Sprintf("%s already exists", resource),
		Details:    details,
		StatusCode: 409,
		Retryable:  false,
	}
}

// NewInternalServerError creates an internal server error.
func NewInternalServerError(message string, details string) *BaseError {
	return &BaseError{
		Code:       "INTERNAL_SERVER_ERROR",
		Message:    message,
		Details:    details,
		StatusCode: 500,
		Retryable:  true,
	}
}

// NewServiceUnavailableError creates a service unavailable error.
func NewServiceUnavailableError(message string) *BaseError {
	return &BaseError{
		Code:       "SERVICE_UNAVAILABLE",
		Message:    message,
		Details:    "",
		StatusCode: 503,
		Retryable:  true,
	}
}

// NewTooManyRequestsError creates a rate limit error.
func NewTooManyRequestsError(message string) *BaseError {
	return &BaseError{
		Code:       "TOO_MANY_REQUESTS",
		Message:    message,
		Details:    "",
		StatusCode: 429,
		Retryable:  true,
	}
}

// CreateRequest represents a base structure for creation requests.
// CreateRequest represents a base structure for creation requests.
// @Description Base structure for creation requests
type CreateRequest struct {
	BaseRequest
}

// UpdateRequest represents a base structure for update requests.
// UpdateRequest represents a base structure for update requests.
// @Description Base structure for update requests
type UpdateRequest struct {
	BaseRequest
	ID string `json:"id" validate:"required" example:"USER123456789"`
}

// Validate validates the update request.
func (r *UpdateRequest) Validate() error {
	if r.ID == "" {
		return NewValidationError("ID is required", "Update operations require a valid ID")
	}
	return r.BaseRequest.Validate()
}

// DeleteRequest represents a base structure for delete requests.
// DeleteRequest represents a base structure for delete requests.
// @Description Base structure for delete requests
type DeleteRequest struct {
	BaseRequest
	ID string `json:"id" validate:"required" example:"USER123456789"`
}

// Validate validates the delete request.
func (r *DeleteRequest) Validate() error {
	if r.ID == "" {
		return NewValidationError("ID is required", "Delete operations require a valid ID")
	}
	return r.BaseRequest.Validate()
}

// ListRequest represents a base structure for list requests with pagination.
// ListRequest represents a base structure for list requests with pagination.
// @Description Base structure for list requests with pagination
type ListRequest struct {
	BaseRequest
	Page    int `json:"page" validate:"min=1" example:"1"`
	PerPage int `json:"per_page" validate:"min=1,max=100" example:"20"`
}

// Validate validates the list request.
func (r *ListRequest) Validate() error {
	if r.Page < 1 {
		r.Page = 1 // Default to page 1
	}
	if r.PerPage < 1 || r.PerPage > 100 {
		r.PerPage = 20 // Default to 20 items per page
	}
	return r.BaseRequest.Validate()
}

// GetByIDRequest represents a base structure for get by ID requests.
// GetByIDRequest represents a base structure for get by ID requests.
// @Description Base structure for get by ID requests
type GetByIDRequest struct {
	BaseRequest
	ID string `json:"id" validate:"required" example:"USER123456789"`
}

// Validate validates the get by ID request.
func (r *GetByIDRequest) Validate() error {
	if r.ID == "" {
		return NewValidationError("ID is required", "Get operations require a valid ID")
	}
	return r.BaseRequest.Validate()
}
