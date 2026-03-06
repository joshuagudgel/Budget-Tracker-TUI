package types

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Transaction struct {
	Id              int64     `db:"id"`
	ParentId        *int64    `db:"parent_id"`
	Amount          float64   `db:"amount"`
	Description     string    `db:"description"`
	RawDescription  string    `db:"raw_description"`
	Date            time.Time `db:"date"`
	CategoryId      int64     `db:"category_id"`
	AutoCategory    string    `db:"auto_category"`
	TransactionType string    `db:"transaction_type"`
	IsSplit         bool      `db:"is_split"`
	IsRecurring     bool      `db:"is_recurring"`
	StatementId     string    `db:"statement_id"`
	Confidence      float64   `db:"confidence"`
	UserModified    bool      `db:"user_modified"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

// Analytics data structures
type AnalyticsSummary struct {
	DateRange         string
	TotalIncome       float64
	TotalExpenses     float64
	NetAmount         float64
	TransactionCount  int
	CategoryBreakdown []CategorySpending
}

type CategorySpending struct {
	CategoryName     string
	Amount           float64
	Percentage       float64
	TransactionCount int
}

// TransactionEditState manages UI editing state for transactions
type TransactionEditState struct {
	// Original transaction being edited
	Original *Transaction

	// UI input fields (what user types)
	AmountInput      string
	DescriptionInput string
	DateInput        string // MM/DD/YYYY or MM-DD-YYYY format
	CategoryId       int64
	TransactionType  string

	// Validation state
	FieldErrors   map[string]string
	IsValid       bool
	ValidationMsg string
}

type Category struct {
	Id          int64     `db:"id"`
	DisplayName string    `db:"display_name"`
	ParentId    *int64    `db:"parent_id"`
	Color       string    `db:"color"`
	IsActive    bool      `db:"is_active"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type BankStatement struct {
	Id             int64     `db:"id"`
	Filename       string    `db:"filename"`
	ImportDate     time.Time `db:"import_date"`
	PeriodStart    time.Time `db:"period_start"`
	PeriodEnd      time.Time `db:"period_end"`
	TemplateUsed   int64     `db:"template_used"`
	TxCount        int       `db:"tx_count"`
	Status         string    `db:"status"`
	ProcessingTime int64     `db:"processing_time"`
	ErrorLog       string    `db:"error_log"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

type CSVTemplate struct {
	Id             int64     `db:"id"`
	Name           string    `db:"name"`
	PostDateColumn int       `db:"post_date_column"`
	AmountColumn   int       `db:"amount_column"`
	DescColumn     int       `db:"desc_column"`
	CategoryColumn *int      `db:"category_column"`
	HasHeader      bool      `db:"has_header"`
	DateFormat     string    `db:"date_format"`
	MerchantColumn *int      `db:"merchant_column"`
	Delimiter      string    `db:"delimiter"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

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
	if t.Date.IsZero() {
		return fmt.Errorf("date cannot be empty")
	}

	// Add business rule validation
	if t.Date.After(time.Now().AddDate(1, 0, 0)) {
		return fmt.Errorf("date cannot be more than 1 year in the future")
	}

	return nil
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

// NewTransactionEditState creates a new edit state from an existing transaction
func NewTransactionEditState(tx *Transaction) *TransactionEditState {
	if tx == nil {
		return &TransactionEditState{
			Original:    nil,
			FieldErrors: make(map[string]string),
			IsValid:     true,
		}
	}

	return &TransactionEditState{
		Original:         tx,
		AmountInput:      fmt.Sprintf("%.2f", tx.Amount),
		DescriptionInput: tx.Description,
		DateInput:        tx.GetDateForDisplay(),
		CategoryId:       tx.CategoryId,
		TransactionType:  tx.TransactionType,
		FieldErrors:      make(map[string]string),
		IsValid:          true,
	}
}

// ToTransaction converts edit state back to a Transaction
func (es *TransactionEditState) ToTransaction() (*Transaction, error) {
	var tx Transaction

	// Copy original fields if editing existing transaction
	if es.Original != nil {
		tx = *es.Original
	}

	// Parse amount
	var err error
	if es.AmountInput != "" {
		tx.Amount, err = parseAmount(es.AmountInput)
		if err != nil {
			return nil, fmt.Errorf("amount: %w", err)
		}
	}

	// Set description
	tx.Description = strings.TrimSpace(es.DescriptionInput)

	// Parse date
	if es.DateInput != "" {
		tx.Date, err = TryParseMultipleDateFormats(es.DateInput)
		if err != nil {
			return nil, fmt.Errorf("date: %w", err)
		}
	}

	// Set category and type
	tx.CategoryId = es.CategoryId
	tx.TransactionType = es.TransactionType

	// Update timestamps
	now := time.Now()
	if es.Original == nil {
		tx.CreatedAt = now
	}
	tx.UpdatedAt = now

	return &tx, nil
}

// Helper method for amount parsing
func parseAmount(amountStr string) (float64, error) {
	trimmed := strings.TrimSpace(amountStr)
	if trimmed == "" {
		return 0, fmt.Errorf("amount cannot be empty")
	}

	// Remove currency symbols and commas
	cleaned := regexp.MustCompile(`[\$,]`).ReplaceAllString(trimmed, "")

	amount, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount format")
	}

	return amount, nil
}

// Display and conversion methods for Transaction
func (t *Transaction) GetDateForDisplay() string {
	return t.Date.Format("01/02/2006") // MM/DD/YYYY for UI display
}

func (t *Transaction) GetDateForStorage() string {
	return t.Date.Format("2006-01-02") // ISO 8601 for database
}

func (t *Transaction) SetDateFromUserInput(dateStr string) error {
	parsed, err := TryParseMultipleDateFormats(dateStr)
	if err != nil {
		return err
	}
	t.Date = parsed
	return nil
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
	if ct.PostDateColumn < 0 {
		return fmt.Errorf("post date column index cannot be negative")
	}
	if ct.AmountColumn < 0 {
		return fmt.Errorf("amount column index cannot be negative")
	}
	if ct.DescColumn < 0 {
		return fmt.Errorf("description column index cannot be negative")
	}
	if ct.CategoryColumn != nil && *ct.CategoryColumn < 0 {
		return fmt.Errorf("category column index cannot be negative")
	}
	if ct.MerchantColumn != nil && *ct.MerchantColumn < 0 {
		return fmt.Errorf("merchant column index cannot be negative")
	}

	// Check for duplicate column assignments
	columns := []int{ct.PostDateColumn, ct.AmountColumn, ct.DescColumn}
	if ct.CategoryColumn != nil {
		columns = append(columns, *ct.CategoryColumn)
	}
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
	case "postdate", "postdatecolumn":
		if ct.PostDateColumn < 0 {
			return fmt.Errorf("post date column index cannot be negative")
		}
	case "amount", "amountcolumn":
		if ct.AmountColumn < 0 {
			return fmt.Errorf("amount column index cannot be negative")
		}
	case "description", "desc", "desccolumn":
		if ct.DescColumn < 0 {
			return fmt.Errorf("description column index cannot be negative")
		}
	case "category", "categorycolumn":
		if ct.CategoryColumn != nil && *ct.CategoryColumn < 0 {
			return fmt.Errorf("category column index cannot be negative")
		}
	case "columns":
		return ct.validateColumns()
	case "dateformat":
		return ct.validateDateFormat()
	case "delimiter":
		return ct.validateDelimiter()
	default:
		return fmt.Errorf("unknown field: %s", field)
	}
	return nil
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

// ValidateDateWithFormat validates a date string against a specific format
// Returns error if the date cannot be parsed using the provided format
func ValidateDateWithFormat(dateStr string, format string) error {
	trimmed := strings.TrimSpace(dateStr)
	if trimmed == "" {
		return fmt.Errorf("date cannot be empty")
	}

	_, err := time.Parse(format, trimmed)
	if err != nil {
		return fmt.Errorf("invalid format. Expected: %s", format)
	}

	return nil
}
