package validation

import (
	"regexp"
	"time"
)

// Common validation constants
const (
	MaxDescriptionLength = 255
	MaxCategoryLength    = 100
	MinAmountValue       = 0.01
	MaxAmountValue       = 999999999.99
)

// Common date formats supported by the application
var SupportedDateFormats = []string{
	"01-02-2006", // mm-dd-yyyy
	"01-02-06",   // mm-dd-yy
	"2006-01-02", // yyyy-mm-dd (ISO format, for internal use)
}

// Regular expressions for common validation patterns
var (
	// AmountRegex matches valid amount formats (including currency symbols)
	AmountRegex = regexp.MustCompile(`^\$?-?\d{1,9}(\.\d{1,2})?$`)

	// DateRegex matches date formats mm-dd-yyyy or mm-dd-yy
	DateRegex = regexp.MustCompile(`^(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])-(\d{2}|\d{4})$`)

	// CategoryNameRegex matches valid category names (alphanumeric, spaces, hyphens, underscores)
	CategoryNameRegex = regexp.MustCompile(`^[a-zA-Z0-9\s\-_]+$`)
)

// ValidationConfig holds configuration for validation behavior
type ValidationConfig struct {
	StrictMode          bool     // Whether to apply strict validation rules
	AllowEmptyCategory  bool     // Whether to allow empty categories
	AllowNegativeAmount bool     // Whether to allow negative amounts
	MaxDescriptionLen   int      // Maximum description length
	RequiredFields      []string // List of required fields
}

// DefaultValidationConfig returns the default validation configuration
func DefaultValidationConfig() ValidationConfig {
	return ValidationConfig{
		StrictMode:          false,
		AllowEmptyCategory:  false,
		AllowNegativeAmount: true, // Allow negative amounts for refunds/corrections
		MaxDescriptionLen:   MaxDescriptionLength,
		RequiredFields:      []string{"amount", "date", "description", "category"},
	}
}

// ValidationHelper provides utility functions for common validation tasks
type ValidationHelper struct {
	Config ValidationConfig
}

// NewValidationHelper creates a new validation helper with the provided config
func NewValidationHelper(config ValidationConfig) *ValidationHelper {
	return &ValidationHelper{Config: config}
}

// IsValidDateFormat checks if a date string matches any supported format
func (vh *ValidationHelper) IsValidDateFormat(dateStr string) bool {
	for _, format := range SupportedDateFormats {
		if _, err := time.Parse(format, dateStr); err == nil {
			return true
		}
	}
	return false
}

// NormalizeDate converts various date formats to a consistent format (mm-dd-yyyy)
func (vh *ValidationHelper) NormalizeDate(dateStr string) (string, error) {
	// Try parsing with each supported format
	for _, format := range SupportedDateFormats {
		if t, err := time.Parse(format, dateStr); err == nil {
			// Return in standard mm-dd-yyyy format
			return t.Format("01-02-2006"), nil
		}
	}
	return "", ErrInvalidDateFormat
}

// IsRequiredField checks if a field is marked as required
func (vh *ValidationHelper) IsRequiredField(fieldName string) bool {
	for _, required := range vh.Config.RequiredFields {
		if required == fieldName {
			return true
		}
	}
	return false
}

// GetFieldDisplayName returns a user-friendly display name for a field
func (vh *ValidationHelper) GetFieldDisplayName(fieldName string) string {
	displayNames := map[string]string{
		"amount":      "Amount",
		"date":        "Date",
		"description": "Description",
		"category":    "Category",
		"parentId":    "Parent Transaction",
		"type":        "Transaction Type",
	}

	if displayName, exists := displayNames[fieldName]; exists {
		return displayName
	}
	return fieldName
}
