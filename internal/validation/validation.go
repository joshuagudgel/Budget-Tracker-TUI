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

// Validate validates a date string in mm-dd-yyyy, mm/dd/yyyy, or yyyy-mm-dd format
func (dv DateValidator) Validate(dateStr string) error {
	if strings.TrimSpace(dateStr) == "" {
		return fmt.Errorf("date cannot be empty")
	}

	// Try parsing ISO 8601 format (storage format)
	if _, err := time.Parse("2006-01-02", dateStr); err == nil {
		return nil
	}

	// Try parsing mm-dd-yyyy format
	if _, err := time.Parse("01-02-2006", dateStr); err == nil {
		return nil
	}

	// Try parsing mm/dd/yyyy format
	if _, err := time.Parse("01/02/2006", dateStr); err == nil {
		return nil
	}

	return fmt.Errorf("date must be in mm-dd-yyyy, mm/dd/yyyy, or yyyy-mm-dd format")
}

// ValidateTime validates a time.Time value for business rules
func (dv DateValidator) ValidateTime(date time.Time) error {
	if date.IsZero() {
		return fmt.Errorf("date cannot be empty")
	}

	// Add business rule validation
	if date.After(time.Now().AddDate(1, 0, 0)) {
		return fmt.Errorf("date cannot be more than 1 year in the future")
	}

	return nil
}

// ParseDate parses a date string and returns a time.Time
func (dv DateValidator) ParseDate(dateStr string) (time.Time, error) {
	if err := dv.Validate(dateStr); err != nil {
		return time.Time{}, err
	}

	// Try parsing ISO 8601 format (storage format) first
	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		return t, nil
	}

	// Try parsing mm-dd-yyyy format
	if t, err := time.Parse("01-02-2006", dateStr); err == nil {
		return t, nil
	}

	// Try parsing mm/dd/yyyy format
	if t, err := time.Parse("01/02/2006", dateStr); err == nil {
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

// CategoryValidator provides validation for category ID fields
type CategoryValidator struct{}

// Validate validates a category ID against available categories
func (cv CategoryValidator) Validate(categoryId int64, availableCategories []types.Category) error {
	if categoryId == 0 {
		return fmt.Errorf("category must be selected")
	}

	// Check if category ID exists in available categories
	for _, category := range availableCategories {
		if category.Id == categoryId {
			return nil
		}
	}

	return fmt.Errorf("category ID %d is not available", categoryId)
}

// GetSuggestions returns category suggestions for a partial display name match
func (cv CategoryValidator) GetSuggestions(partial string, availableCategories []types.Category) []types.Category {
	var suggestions []types.Category
	lowerPartial := strings.ToLower(strings.TrimSpace(partial))

	for _, category := range availableCategories {
		if strings.Contains(strings.ToLower(category.DisplayName), lowerPartial) {
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
func (tv *TransactionValidator) ValidateTransaction(transaction *types.Transaction, availableCategories []types.Category) types.ValidationResult {
	result := types.ValidationResult{IsValid: true}

	// Validate Amount
	if err := tv.Amount.Validate(transaction.Amount); err != nil {
		result.AddError("amount", err.Error())
	}

	// Validate Date
	if err := tv.Date.ValidateTime(transaction.Date); err != nil {
		result.AddError("date", err.Error())
	}

	// Validate Description
	if err := tv.Description.Validate(transaction.Description); err != nil {
		result.AddError("description", err.Error())
	}

	// Validate CategoryId
	if err := tv.Category.Validate(transaction.CategoryId, availableCategories); err != nil {
		result.AddError("categoryId", err.Error())
	}

	return result
}

// ValidateField validates a single field of a transaction
func (tv *TransactionValidator) ValidateField(transaction *types.Transaction, field string, availableCategories []types.Category) error {
	switch strings.ToLower(field) {
	case "amount":
		return tv.Amount.Validate(transaction.Amount)
	case "date":
		return tv.Date.ValidateTime(transaction.Date)
	case "description":
		return tv.Description.Validate(transaction.Description)
	case "categoryid", "category":
		return tv.Category.Validate(transaction.CategoryId, availableCategories)
	default:
		return fmt.Errorf("unknown field: %s", field)
	}
}

// ValidateBulkEdit validates multiple transactions for bulk editing
func (tv *TransactionValidator) ValidateBulkEdit(transactions []*types.Transaction, availableCategories []types.Category) map[int64]types.ValidationResult {
	results := make(map[int64]types.ValidationResult)

	for _, transaction := range transactions {
		results[transaction.Id] = tv.ValidateTransaction(transaction, availableCategories)
	}

	return results
}

// ValidateTransactionStruct validates an entire transaction using struct methods (deprecated, use ValidateTransaction instead)
func ValidateTransactionStruct(transaction *types.Transaction, availableCategories []types.Category) types.ValidationResult {
	validator := NewTransactionValidator()
	return validator.ValidateTransaction(transaction, availableCategories)
}

// ValidateTransactionField validates a single field using struct methods (deprecated, use ValidateField instead)
func ValidateTransactionField(transaction *types.Transaction, field string, availableCategories []types.Category) error {
	validator := NewTransactionValidator()
	return validator.ValidateField(transaction, field, availableCategories)
}

// CategoryManagementValidator provides validation for category CRUD operations
type CategoryManagementValidator struct{}

// NewCategoryManagementValidator creates a new category management validator
func NewCategoryManagementValidator() *CategoryManagementValidator {
	return &CategoryManagementValidator{}
}

// ValidateDisplayName validates a category display name
func (cmv *CategoryManagementValidator) ValidateDisplayName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("category name cannot be empty")
	}
	if len(name) > 100 {
		return fmt.Errorf("category name cannot exceed 100 characters")
	}
	return nil
}

// ValidateColor validates a category color (hex format)
func (cmv *CategoryManagementValidator) ValidateColor(color string) error {
	if color == "" {
		return nil // Color is optional
	}

	if len(color) != 7 || color[0] != '#' {
		return fmt.Errorf("color must be in format #RRGGBB")
	}

	for i := 1; i < 7; i++ {
		c := color[i]
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')) {
			return fmt.Errorf("color must be a valid hex code")
		}
	}
	return nil
}

// ValidateParent validates parent category relationship
func (cmv *CategoryManagementValidator) ValidateParent(parentId *int64, availableCategories []types.Category, currentId int64) error {
	if parentId == nil {
		return nil // Top-level category is valid
	}

	// Check if parent exists
	parentExists := false
	for _, cat := range availableCategories {
		if cat.Id == *parentId {
			parentExists = true
			break
		}
	}

	if !parentExists {
		return fmt.Errorf("selected parent category does not exist")
	}

	// Prevent circular reference
	if *parentId == currentId {
		return fmt.Errorf("category cannot be its own parent")
	}

	// Check for deeper circular references (parent's parent chain)
	if cmv.hasCircularReference(*parentId, currentId, availableCategories) {
		return fmt.Errorf("circular reference detected in parent hierarchy")
	}

	return nil
}

// hasCircularReference checks for circular references in the parent hierarchy
func (cmv *CategoryManagementValidator) hasCircularReference(parentId, targetId int64, categories []types.Category) bool {
	visited := make(map[int64]bool)
	current := parentId

	for current != 0 {
		if visited[current] {
			return true // Circular reference detected
		}
		if current == targetId {
			return true // Would create circular reference
		}

		visited[current] = true

		// Find parent of current category
		found := false
		for _, cat := range categories {
			if cat.Id == current {
				if cat.ParentId != nil {
					current = *cat.ParentId
				} else {
					current = 0 // Top-level category
				}
				found = true
				break
			}
		}

		if !found {
			break // Category not found, break loop
		}
	}

	return false
}

// ValidateCategory validates an entire category for create/update operations
func (cmv *CategoryManagementValidator) ValidateCategory(category *types.Category, availableCategories []types.Category) types.ValidationResult {
	result := types.ValidationResult{IsValid: true}

	// Validate DisplayName
	if err := cmv.ValidateDisplayName(category.DisplayName); err != nil {
		result.AddError("displayName", err.Error())
	}

	// Validate Color
	if err := cmv.ValidateColor(category.Color); err != nil {
		result.AddError("color", err.Error())
	}

	// Validate Parent relationship
	if err := cmv.ValidateParent(category.ParentId, availableCategories, category.Id); err != nil {
		result.AddError("parentId", err.Error())
	}

	return result
}

// ValidateCategoryField validates a single field of a category
func (cmv *CategoryManagementValidator) ValidateCategoryField(category *types.Category, field string, availableCategories []types.Category) error {
	switch strings.ToLower(field) {
	case "displayname", "name":
		return cmv.ValidateDisplayName(category.DisplayName)
	case "color":
		return cmv.ValidateColor(category.Color)
	case "parentid", "parent":
		return cmv.ValidateParent(category.ParentId, availableCategories, category.Id)
	default:
		return fmt.Errorf("unknown field: %s", field)
	}
}

// ValidateForDeletion validates if a category can be safely deleted
func (cmv *CategoryManagementValidator) ValidateForDeletion(categoryId int64, transactions []types.Transaction, categories []types.Category) error {
	// Check if category is in use by transactions
	for _, tx := range transactions {
		if tx.CategoryId == categoryId {
			return fmt.Errorf("cannot delete category: it is being used by transactions")
		}
	}

	// Check if category has subcategories
	for _, cat := range categories {
		if cat.ParentId != nil && *cat.ParentId == categoryId {
			return fmt.Errorf("cannot delete category: it has subcategories. Delete or reassign subcategories first")
		}
	}

	return nil
}

// GetCategoryNameSuggestions returns category name suggestions based on partial input
func (cmv *CategoryManagementValidator) GetCategoryNameSuggestions(partial string, existingCategories []types.Category) []string {
	var suggestions []string
	lowerPartial := strings.ToLower(strings.TrimSpace(partial))

	// Common category names
	commonCategories := []string{
		"Food & Dining", "Transportation", "Entertainment", "Utilities", "Shopping",
		"Healthcare", "Insurance", "Education", "Personal Care", "Home Improvement",
		"Travel", "Subscriptions", "Income", "Investment", "Business",
	}

	for _, common := range commonCategories {
		if strings.Contains(strings.ToLower(common), lowerPartial) {
			// Check if not already exists
			exists := false
			for _, existing := range existingCategories {
				if strings.EqualFold(existing.DisplayName, common) {
					exists = true
					break
				}
			}
			if !exists {
				suggestions = append(suggestions, common)
			}
		}
	}

	return suggestions
}

// GetColorSuggestions returns common color suggestions
func (cmv *CategoryManagementValidator) GetColorSuggestions() []string {
	return []string{
		"#FF6B6B", "#4ECDC4", "#45B7D1", "#96CEB4", "#FECA57",
		"#FF9FF3", "#54A0FF", "#5F27CD", "#00D2D3", "#FF9F43",
		"#FC427B", "#BDC3C7", "#6C5CE7", "#A29BFE", "#FD79A8",
	}
}
