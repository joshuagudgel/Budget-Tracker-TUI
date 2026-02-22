package validation

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"budget-tracker-tui/internal/types"
)

// AmountValidator provides validation for amount fields
type AmountValidator struct{}

// Validate validates an amount value
func (av AmountValidator) Validate(amount float64) error {
	// Check for zero amount
	if amount == 0 {
		return fmt.Errorf("amount cannot be zero")
	}

	// Check for max 2 decimal places
	rounded := math.Round(amount*100) / 100
	if math.Abs(amount-rounded) > 0.001 {
		return fmt.Errorf("amount cannot have more than 2 decimal places")
	}

	return nil
}

// ParseAmount parses and validates an amount string
func (av AmountValidator) ParseAmount(amountStr string) (float64, error) {
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

	return amount, av.Validate(amount)
}

// DateValidator provides validation for date fields
type DateValidator struct{}

// Validate validates a date string in mm-dd-yyyy, mm/dd/yyyy, or mm-dd-yy format
func (dv DateValidator) Validate(dateStr string) error {
	if strings.TrimSpace(dateStr) == "" {
		return fmt.Errorf("date cannot be empty")
	}

	// Try parsing mm-dd-yyyy format
	if _, err := time.Parse("01-02-2006", dateStr); err == nil {
		return nil
	}

	// Try parsing mm/dd/yyyy format
	if _, err := time.Parse("01/02/2006", dateStr); err == nil {
		return nil
	}

	// Try parsing mm-dd-yy format
	if _, err := time.Parse("01-02-06", dateStr); err == nil {
		return nil
	}

	return fmt.Errorf("date must be in mm-dd-yyyy, mm/dd/yyyy, or mm-dd-yy format")
}

// ParseDate parses a date string and returns a time.Time
func (dv DateValidator) ParseDate(dateStr string) (time.Time, error) {
	if err := dv.Validate(dateStr); err != nil {
		return time.Time{}, err
	}

	// Try parsing mm-dd-yyyy format first
	if t, err := time.Parse("01-02-2006", dateStr); err == nil {
		return t, nil
	}

	// Try parsing mm/dd/yyyy format
	if t, err := time.Parse("01/02/2006", dateStr); err == nil {
		return t, nil
	}

	// Try parsing mm-dd-yy format
	if t, err := time.Parse("01-02-06", dateStr); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse date")
}

// DescriptionValidator provides validation for description fields
type DescriptionValidator struct{}

// Validate validates a description string
func (dv DescriptionValidator) Validate(description string) error {
	trimmed := strings.TrimSpace(description)

	if trimmed == "" {
		return fmt.Errorf("description cannot be empty")
	}

	if len(trimmed) > 255 {
		return fmt.Errorf("description cannot exceed 255 characters (current: %d)", len(trimmed))
	}

	return nil
}

// CategoryValidator provides validation for category fields
type CategoryValidator struct{}

// Validate validates a category against available categories
func (cv CategoryValidator) Validate(category string, availableCategories []string) error {
	trimmedCategory := strings.TrimSpace(category)

	if trimmedCategory == "" {
		return fmt.Errorf("category cannot be empty")
	}

	// Check if category exists in available categories
	for _, availableCategory := range availableCategories {
		if strings.EqualFold(availableCategory, trimmedCategory) {
			return nil
		}
	}

	return fmt.Errorf("category '%s' is not available", trimmedCategory)
}

// GetSuggestions returns category suggestions for a partial match
func (cv CategoryValidator) GetSuggestions(partial string, availableCategories []string) []string {
	var suggestions []string
	lowerPartial := strings.ToLower(strings.TrimSpace(partial))

	for _, category := range availableCategories {
		if strings.Contains(strings.ToLower(category), lowerPartial) {
			suggestions = append(suggestions, category)
		}
	}

	return suggestions
}

// TransactionValidator provides comprehensive transaction validation
type TransactionValidator struct {
	Amount      AmountValidator
	Date        DateValidator
	Description DescriptionValidator
	Category    CategoryValidator
}

// NewTransactionValidator creates a new transaction validator
func NewTransactionValidator() *TransactionValidator {
	return &TransactionValidator{
		Amount:      AmountValidator{},
		Date:        DateValidator{},
		Description: DescriptionValidator{},
		Category:    CategoryValidator{},
	}
}

// ValidateTransaction validates an entire transaction
func (tv *TransactionValidator) ValidateTransaction(transaction *types.Transaction, availableCategories []string) types.ValidationResult {
	result := types.ValidationResult{IsValid: true}

	// Validate Amount
	if err := tv.Amount.Validate(transaction.Amount); err != nil {
		result.AddError("amount", err.Error())
	}

	// Validate Date
	if err := tv.Date.Validate(transaction.Date); err != nil {
		result.AddError("date", err.Error())
	}

	// Validate Description
	if err := tv.Description.Validate(transaction.Description); err != nil {
		result.AddError("description", err.Error())
	}

	// Validate Category
	if err := tv.Category.Validate(transaction.Category, availableCategories); err != nil {
		result.AddError("category", err.Error())
	}

	return result
}

// ValidateField validates a single field of a transaction
func (tv *TransactionValidator) ValidateField(transaction *types.Transaction, field string, availableCategories []string) error {
	switch strings.ToLower(field) {
	case "amount":
		return tv.Amount.Validate(transaction.Amount)
	case "date":
		return tv.Date.Validate(transaction.Date)
	case "description":
		return tv.Description.Validate(transaction.Description)
	case "category":
		return tv.Category.Validate(transaction.Category, availableCategories)
	default:
		return fmt.Errorf("unknown field: %s", field)
	}
}

// ValidateBulkEdit validates multiple transactions for bulk editing
func (tv *TransactionValidator) ValidateBulkEdit(transactions []*types.Transaction, availableCategories []string) map[int64]types.ValidationResult {
	results := make(map[int64]types.ValidationResult)

	for _, transaction := range transactions {
		results[transaction.Id] = tv.ValidateTransaction(transaction, availableCategories)
	}

	return results
}
