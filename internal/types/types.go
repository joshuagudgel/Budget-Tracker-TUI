package types

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type Transaction struct {
	Id              int64   `json:"id"`
	ParentId        *int64  `json:"parentId,omitempty"`
	Amount          float64 `json:"amount"`
	Description     string  `json:"description"`
	RawDescription  string  `json:"rawDescription"`
	Date            string  `json:"date"`
	Category        string  `json:"category"`
	AutoCategory    string  `json:"autoCategory"`
	TransactionType string  `json:"transactionType"`
	IsSplit         bool    `json:"isSplit"`
	IsRecurring     bool    `json:"isRecurring"`
	StatementId     string  `json:"statementId"`
	Confidence      float64 `json:"confidence,omitempty"`
	UserModified    bool    `json:"userModified,omitempty"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

type Category struct {
	Id          int64  `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	ParentId    *int64 `json:"parentId,omitempty"`
	Color       string `json:"color,omitempty"`
	IsActive    bool   `json:"isActive"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type BankStatement struct {
	Id             int64  `json:"id"`
	Filename       string `json:"filename"`
	ImportDate     string `json:"importDate"`
	PeriodStart    string `json:"periodStart"`
	PeriodEnd      string `json:"periodEnd"`
	TemplateUsed   string `json:"templateUsed"`
	TxCount        int    `json:"txCount"`
	Status         string `json:"status"`
	ProcessingTime int64  `json:"processingTime,omitempty"`
	ErrorLog       string `json:"errorLog,omitempty"`
}

type CSVTemplate struct {
	Name           string `json:"name"`
	DateColumn     int    `json:"dateColumn"`
	AmountColumn   int    `json:"amountColumn"`
	DescColumn     int    `json:"descColumn"`
	HasHeader      bool   `json:"hasHeader"`
	DateFormat     string `json:"dateFormat,omitempty"`
	MerchantColumn *int   `json:"merchantColumn,omitempty"`
	Delimiter      string `json:"delimiter,omitempty"`
}

type ImportResult struct {
	Success          bool
	ImportedCount    int
	OverlapDetected  bool
	OverlappingStmts []BankStatement
	PeriodStart      string
	PeriodEnd        string
	Message          string
	Filename         string
}

// ValidationError represents a single validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationResult represents the result of validation
type ValidationResult struct {
	IsValid bool              `json:"isValid"`
	Errors  []ValidationError `json:"errors"`
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

// Validate validates the transaction and returns a ValidationResult
func (t *Transaction) Validate(availableCategories []string) ValidationResult {
	result := ValidationResult{IsValid: true}

	// Validate Amount
	if err := t.validateAmount(); err != nil {
		result.AddError("amount", err.Error())
	}

	// Validate Date
	if err := t.validateDate(); err != nil {
		result.AddError("date", err.Error())
	}

	// Validate Description
	if err := t.validateDescription(); err != nil {
		result.AddError("description", err.Error())
	}

	// Validate Category
	if err := t.validateCategory(availableCategories); err != nil {
		result.AddError("category", err.Error())
	}

	return result
}

// validateAmount validates the amount field
func (t *Transaction) validateAmount() error {
	// Check for zero amount
	if t.Amount == 0 {
		return fmt.Errorf("amount cannot be zero")
	}

	// Check for valid float64 (this is implicitly handled by the type system)
	// Check for max 2 decimal places
	rounded := math.Round(t.Amount*100) / 100
	if math.Abs(t.Amount-rounded) > 0.001 {
		return fmt.Errorf("amount cannot have more than 2 decimal places")
	}

	return nil
}

// validateDate validates the date field
func (t *Transaction) validateDate() error {
	if strings.TrimSpace(t.Date) == "" {
		return fmt.Errorf("date cannot be empty")
	}

	// Try parsing mm-dd-yyyy format
	if _, err := time.Parse("01-02-2006", t.Date); err == nil {
		return nil
	}

	// Try parsing mm-dd-yy format
	if _, err := time.Parse("01-02-06", t.Date); err == nil {
		return nil
	}

	return fmt.Errorf("date must be in mm-dd-yyyy or mm-dd-yy format")
}

// validateDescription validates the description field
func (t *Transaction) validateDescription() error {
	trimmed := strings.TrimSpace(t.Description)

	if trimmed == "" {
		return fmt.Errorf("description cannot be empty")
	}

	if len(trimmed) > 255 {
		return fmt.Errorf("description cannot exceed 255 characters")
	}

	return nil
}

// validateCategory validates the category field
func (t *Transaction) validateCategory(availableCategories []string) error {
	trimmedCategory := strings.TrimSpace(t.Category)

	if trimmedCategory == "" {
		return fmt.Errorf("category cannot be empty")
	}

	// Check if category exists in available categories
	for _, category := range availableCategories {
		if strings.EqualFold(category, trimmedCategory) {
			return nil
		}
	}

	return fmt.Errorf("category '%s' is not available", trimmedCategory)
}

// ValidateField validates a single field and returns any error
func (t *Transaction) ValidateField(field string, availableCategories []string) error {
	switch strings.ToLower(field) {
	case "amount":
		return t.validateAmount()
	case "date":
		return t.validateDate()
	case "description":
		return t.validateDescription()
	case "category":
		return t.validateCategory(availableCategories)
	default:
		return fmt.Errorf("unknown field: %s", field)
	}
}
