package types

// ValidationError represents a single validation error
type ValidationError struct {
	Field      string
	Message    string
	LineNumber int // CSV row number (0 = not applicable, >0 = CSV line)
}

// ValidationResult represents the result of validation
type ValidationResult struct {
	IsValid bool
	Errors  []ValidationError
}

// AddError adds a validation error to the result
func (vr *ValidationResult) AddError(field, message string) {
	vr.IsValid = false
	vr.Errors = append(vr.Errors, ValidationError{
		Field:   field,
		Message: message,
	})
}

// HasError checks if there's an error for a specific field
func (vr *ValidationResult) HasError(field string) bool {
	for _, err := range vr.Errors {
		if err.Field == field {
			return true
		}
	}
	return false
}

// GetError returns the first error message for a specific field
func (vr *ValidationResult) GetError(field string) string {
	for _, err := range vr.Errors {
		if err.Field == field {
			return err.Message
		}
	}
	return ""
}

// ImportResult represents the result of a CSV import operation
type ImportResult struct {
	Success             bool
	ImportedCount       int
	OverlapDetected     bool
	OverlappingStmts    []BankStatement
	PeriodStart         string
	PeriodEnd           string
	Message             string
	Filename            string
	HasValidationErrors bool
	ValidationErrors    []ValidationError
}
