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
	CategoryId      int64   `json:"categoryId"`
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
func (t *Transaction) Validate(availableCategories []Category) ValidationResult {
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

	// Validate CategoryId
	if err := t.validateCategoryId(availableCategories); err != nil {
		result.AddError("categoryId", err.Error())
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

// validateCategoryId validates the categoryId field
func (t *Transaction) validateCategoryId(availableCategories []Category) error {
	if t.CategoryId == 0 {
		return fmt.Errorf("category must be selected")
	}

	// Check if category ID exists in available categories
	for _, category := range availableCategories {
		if category.Id == t.CategoryId {
			return nil
		}
	}

	return fmt.Errorf("category ID %d is not available", t.CategoryId)
}

// ValidateField validates a single field and returns any error
func (t *Transaction) ValidateField(field string, availableCategories []Category) error {
	switch strings.ToLower(field) {
	case "amount":
		return t.validateAmount()
	case "date":
		return t.validateDate()
	case "description":
		return t.validateDescription()
	case "categoryid", "category":
		return t.validateCategoryId(availableCategories)
	default:
		return fmt.Errorf("unknown field: %s", field)
	}
}

// Category validation methods

// Validate validates the category and returns a ValidationResult
func (c *Category) Validate(availableCategories []Category) ValidationResult {
	result := ValidationResult{IsValid: true}

	// Validate DisplayName
	if err := c.validateDisplayName(); err != nil {
		result.AddError("displayName", err.Error())
	}

	// Validate Color (optional field)
	if err := c.validateColor(); err != nil {
		result.AddError("color", err.Error())
	}

	// Validate ParentId
	if err := c.validateParentId(availableCategories); err != nil {
		result.AddError("parentId", err.Error())
	}

	return result
}

// validateDisplayName validates the category name
func (c *Category) validateDisplayName() error {
	trimmed := strings.TrimSpace(c.DisplayName)
	if trimmed == "" {
		return fmt.Errorf("category name cannot be empty")
	}
	if len(c.DisplayName) > 100 {
		return fmt.Errorf("category name cannot exceed 100 characters")
	}
	return nil
}

// validateColor validates the color field (hex format)
func (c *Category) validateColor() error {
	if c.Color == "" {
		return nil // Color is optional
	}

	if len(c.Color) != 7 || c.Color[0] != '#' {
		return fmt.Errorf("color must be in format #RRGGBB")
	}

	for i := 1; i < 7; i++ {
		ch := c.Color[i]
		if !((ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'F') || (ch >= 'a' && ch <= 'f')) {
			return fmt.Errorf("color must be a valid hex code")
		}
	}
	return nil
}

// validateParentId validates the parent category relationship
func (c *Category) validateParentId(availableCategories []Category) error {
	if c.ParentId == nil {
		return nil // Top-level category is valid
	}

	// Check if parent exists
	parentExists := false
	for _, cat := range availableCategories {
		if cat.Id == *c.ParentId {
			parentExists = true
			break
		}
	}

	if !parentExists {
		return fmt.Errorf("selected parent category does not exist")
	}

	// Prevent circular reference
	if *c.ParentId == c.Id {
		return fmt.Errorf("category cannot be its own parent")
	}

	return nil
}

// ValidateField validates a single field and returns any error
func (c *Category) ValidateField(field string, availableCategories []Category) error {
	switch strings.ToLower(field) {
	case "displayname", "name":
		return c.validateDisplayName()
	case "color":
		return c.validateColor()
	case "parentid", "parent":
		return c.validateParentId(availableCategories)
	default:
		return fmt.Errorf("unknown field: %s", field)
	}
}

// isValidHexColor is a helper function for hex color validation
func isValidHexColor(color string) bool {
	if len(color) != 7 || color[0] != '#' {
		return false
	}
	for i := 1; i < 7; i++ {
		c := color[i]
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
