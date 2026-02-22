package validation

import "errors"

// Common validation errors
var (
	// Amount validation errors
	ErrAmountZero            = errors.New("amount cannot be zero")
	ErrAmountTooManyDecimals = errors.New("amount cannot have more than 2 decimal places")
	ErrAmountInvalid         = errors.New("invalid amount format")
	ErrAmountTooLarge        = errors.New("amount exceeds maximum allowed value")
	ErrAmountTooSmall        = errors.New("amount is below minimum allowed value")

	// Date validation errors
	ErrDateEmpty         = errors.New("date cannot be empty")
	ErrInvalidDateFormat = errors.New("date must be in mm-dd-yyyy or mm-dd-yy format")
	ErrDateInFuture      = errors.New("date cannot be in the future")
	ErrDateTooOld        = errors.New("date is too old to be valid")

	// Description validation errors
	ErrDescriptionEmpty   = errors.New("description cannot be empty")
	ErrDescriptionTooLong = errors.New("description exceeds maximum length")

	// Category validation errors
	ErrCategoryEmpty    = errors.New("category cannot be empty")
	ErrCategoryNotFound = errors.New("category is not available")
	ErrCategoryInvalid  = errors.New("category contains invalid characters")

	// General validation errors
	ErrFieldRequired    = errors.New("field is required")
	ErrUnknownField     = errors.New("unknown field")
	ErrValidationFailed = errors.New("validation failed")
)

// ErrorMessages provides user-friendly error messages for validation errors
var ErrorMessages = map[error]string{
	ErrAmountZero:            "Please enter a non-zero amount",
	ErrAmountTooManyDecimals: "Amount can have at most 2 decimal places",
	ErrAmountInvalid:         "Please enter a valid numeric amount",
	ErrAmountTooLarge:        "Amount is too large",
	ErrAmountTooSmall:        "Amount is too small",

	ErrDateEmpty:         "Please enter a date",
	ErrInvalidDateFormat: "Date must be in MM-DD-YYYY format (e.g., 12-31-2023)",
	ErrDateInFuture:      "Date cannot be in the future",
	ErrDateTooOld:        "Date is too old to be valid",

	ErrDescriptionEmpty:   "Please enter a description",
	ErrDescriptionTooLong: "Description is too long (max 255 characters)",

	ErrCategoryEmpty:    "Please select a category",
	ErrCategoryNotFound: "Selected category is not available",
	ErrCategoryInvalid:  "Category name contains invalid characters",

	ErrFieldRequired:    "This field is required",
	ErrUnknownField:     "Unknown field",
	ErrValidationFailed: "Validation failed",
}

// GetUserFriendlyMessage returns a user-friendly version of an error message
func GetUserFriendlyMessage(err error) string {
	if message, exists := ErrorMessages[err]; exists {
		return message
	}
	return err.Error()
}

// ValidationErrorType represents different types of validation errors
type ValidationErrorType int

const (
	ErrorTypeRequired ValidationErrorType = iota
	ErrorTypeFormat
	ErrorTypeRange
	ErrorTypeExistence
	ErrorTypeConstraint
)

// DetailedValidationError provides more context about validation errors
type DetailedValidationError struct {
	Field        string
	Type         ValidationErrorType
	Message      string
	Suggestion   string
	CurrentValue interface{}
}

// Error implements the error interface
func (dve DetailedValidationError) Error() string {
	return dve.Message
}

// NewDetailedError creates a new detailed validation error
func NewDetailedError(field string, errorType ValidationErrorType, message, suggestion string, currentValue interface{}) DetailedValidationError {
	return DetailedValidationError{
		Field:        field,
		Type:         errorType,
		Message:      message,
		Suggestion:   suggestion,
		CurrentValue: currentValue,
	}
}
