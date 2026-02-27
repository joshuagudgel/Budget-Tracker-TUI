package types

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type Transaction struct {
	Id              int64
	ParentId        *int64
	Amount          float64
	Description     string
	RawDescription  string
	Date            string
	CategoryId      int64
	AutoCategory    string
	TransactionType string
	IsSplit         bool
	IsRecurring     bool
	StatementId     string
	Confidence      float64
	UserModified    bool
	CreatedAt       string
	UpdatedAt       string
}

type Category struct {
	Id          int64
	DisplayName string
	ParentId    *int64
	Color       string
	IsActive    bool
	CreatedAt   string
	UpdatedAt   string
}

type BankStatement struct {
	Id             int64
	Filename       string
	ImportDate     string
	PeriodStart    string
	PeriodEnd      string
	TemplateUsed   int64
	TxCount        int
	Status         string
	ProcessingTime int64
	ErrorLog       string
	CreatedAt      string
	UpdatedAt      string
}

type CSVTemplate struct {
	Id             int64
	Name           string
	DateColumn     int
	AmountColumn   int
	DescColumn     int
	HasHeader      bool
	DateFormat     string
	MerchantColumn *int
	Delimiter      string
	CreatedAt      string
	UpdatedAt      string
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
	Field   string
	Message string
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

// CSVTemplate validation methods

// Validate validates the CSV template and returns a ValidationResult
func (ct *CSVTemplate) Validate() ValidationResult {
	result := ValidationResult{IsValid: true}

	// Validate Name
	if err := ct.validateName(); err != nil {
		result.AddError("name", err.Error())
	}

	// Validate columns
	if err := ct.validateColumns(); err != nil {
		result.AddError("columns", err.Error())
	}

	// Validate DateFormat
	if err := ct.validateDateFormat(); err != nil {
		result.AddError("dateFormat", err.Error())
	}

	// Validate Delimiter
	if err := ct.validateDelimiter(); err != nil {
		result.AddError("delimiter", err.Error())
	}

	return result
}

// validateName validates the template name
func (ct *CSVTemplate) validateName() error {
	trimmed := strings.TrimSpace(ct.Name)
	if trimmed == "" {
		return fmt.Errorf("template name cannot be empty")
	}
	if len(ct.Name) > 100 {
		return fmt.Errorf("template name cannot exceed 100 characters")
	}
	return nil
}

// validateColumns ensures columns are properly configured
func (ct *CSVTemplate) validateColumns() error {
	// Check for negative column indices
	if ct.DateColumn < 0 {
		return fmt.Errorf("date column index cannot be negative")
	}
	if ct.AmountColumn < 0 {
		return fmt.Errorf("amount column index cannot be negative")
	}
	if ct.DescColumn < 0 {
		return fmt.Errorf("description column index cannot be negative")
	}
	if ct.MerchantColumn != nil && *ct.MerchantColumn < 0 {
		return fmt.Errorf("merchant column index cannot be negative")
	}

	// Check for duplicate column assignments
	columns := []int{ct.DateColumn, ct.AmountColumn, ct.DescColumn}
	if ct.MerchantColumn != nil {
		columns = append(columns, *ct.MerchantColumn)
	}

	for i := 0; i < len(columns); i++ {
		for j := i + 1; j < len(columns); j++ {
			if columns[i] == columns[j] {
				return fmt.Errorf("columns cannot have duplicate indices")
			}
		}
	}

	return nil
}

// validateDateFormat validates the date format string
func (ct *CSVTemplate) validateDateFormat() error {
	if ct.DateFormat == "" {
		return nil // Optional field
	}

	// Test common date formats
	validFormats := []string{
		"2006-01-02", "01/02/2006", "01-02-2006", "2006/01/02",
		"02/01/2006", "02-01-2006", "2/1/2006", "1/2/2006",
	}

	for _, format := range validFormats {
		if ct.DateFormat == format {
			return nil
		}
	}

	return fmt.Errorf("unsupported date format: %s", ct.DateFormat)
}

// validateDelimiter validates the CSV delimiter
func (ct *CSVTemplate) validateDelimiter() error {
	if ct.Delimiter == "" {
		return nil // Default to comma
	}

	if len(ct.Delimiter) != 1 {
		return fmt.Errorf("delimiter must be a single character")
	}

	// Common valid delimiters
	validDelimiters := ",;|\t"
	for _, valid := range validDelimiters {
		if ct.Delimiter == string(valid) {
			return nil
		}
	}

	return fmt.Errorf("unsupported delimiter: %s", ct.Delimiter)
}

// ValidateField validates a single field and returns any error
func (ct *CSVTemplate) ValidateField(field string) error {
	switch strings.ToLower(field) {
	case "name":
		return ct.validateName()
	case "columns":
		return ct.validateColumns()
	case "dateformat":
		return ct.validateDateFormat()
	case "delimiter":
		return ct.validateDelimiter()
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
