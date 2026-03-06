package validation_test

import (
	"budget-tracker-tui/internal/types"
	"budget-tracker-tui/internal/validation"
	"testing"
	"time"
)

func TestAmountValidator(t *testing.T) {
	validator := validation.AmountValidator{}

	tests := []struct {
		name        string
		amount      float64
		expectError bool
		errorMsg    string
	}{
		{"Valid positive amount", 123.45, false, ""},
		{"Valid negative amount", -50.99, false, ""},
		{"Zero amount", 0, true, "amount cannot be zero"},
		{"Too many decimals", 123.456, true, "amount cannot have more than 2 decimal places"},
		{"Valid single decimal", 99.9, false, ""},
		{"Valid whole number", 100, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.amount)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error '%s' but got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestDateValidator(t *testing.T) {
	validator := validation.DateValidator{}

	tests := []struct {
		name        string
		date        string
		expectError bool
	}{
		{"Valid mm-dd-yyyy", "12-31-2023", false},
		{"Valid mm/dd/yyyy", "12/31/2023", false},
		{"Invalid mm-dd-yy format", "01-15-24", true},
		{"Invalid format yyyy-mm-dd", "2023-12-31", true},
		{"Empty date", "", true},
		{"Invalid date", "13-32-2023", true},
		{"Invalid separator dot", "12.31.2023", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.date)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestDescriptionValidator(t *testing.T) {
	validator := validation.DescriptionValidator{}

	tests := []struct {
		name        string
		description string
		expectError bool
	}{
		{"Valid description", "Coffee at Starbucks", false},
		{"Empty description", "", true},
		{"Whitespace only", "   ", true},
		{"Max length description", string(make([]byte, 255)), false},
		{"Too long description", string(make([]byte, 256)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.description)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestCategoryValidator(t *testing.T) {
	validator := validation.CategoryValidator{}
	categories := []types.Category{
		{Id: 1, DisplayName: "Food"},
		{Id: 2, DisplayName: "Transportation"},
		{Id: 3, DisplayName: "Entertainment"},
		{Id: 4, DisplayName: "Utilities"},
	}

	tests := []struct {
		name        string
		categoryId  int64
		expectError bool
	}{
		{"Valid category", 1, false},
		{"Another valid category", 2, false},
		{"Invalid category ID", 99, true},
		{"Zero category ID", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.categoryId, categories)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestCategoryValidatorSuggestions(t *testing.T) {
	validator := validation.CategoryValidator{}
	categories := []types.Category{
		{Id: 1, DisplayName: "Food & Dining"},
		{Id: 2, DisplayName: "Transportation"},
		{Id: 3, DisplayName: "Entertainment"},
		{Id: 4, DisplayName: "Utilities"},
	}

	tests := []struct {
		name          string
		partial       string
		expectedCount int
		shouldContain string
	}{
		{"Exact match", "Food", 1, "Food & Dining"},
		{"Partial match", "trans", 1, "Transportation"},
		{"Case insensitive", "FOOD", 1, "Food & Dining"},
		{"No matches", "xyz", 0, ""},
		{"Multiple matches", "t", 3, "Transportation"}, // Transportation, Entertainment, and Utilities
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := validator.GetSuggestions(tt.partial, categories)
			if len(suggestions) != tt.expectedCount {
				t.Errorf("Expected %d suggestions but got %d", tt.expectedCount, len(suggestions))
			}

			if tt.expectedCount > 0 {
				found := false
				for _, suggestion := range suggestions {
					if suggestion.DisplayName == tt.shouldContain {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected suggestions to contain '%s'", tt.shouldContain)
				}
			}
		})
	}
}

func TestTransactionValidation(t *testing.T) {
	validator := validation.NewTransactionValidator()
	categories := []types.Category{
		{Id: 1, DisplayName: "Food"},
		{Id: 2, DisplayName: "Transportation"},
		{Id: 3, DisplayName: "Entertainment"},
	}

	validTransaction := &types.Transaction{
		Amount:      123.45,
		Description: "Coffee purchase",
		Date:        time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
		CategoryId:  1, // Food category
	}

	invalidTransaction := &types.Transaction{
		Amount:      0,           // Invalid: zero amount
		Description: "",          // Invalid: empty description
		Date:        time.Time{}, // Invalid: zero value date
		CategoryId:  99,          // Invalid: category ID doesn't exist
	}

	t.Run("Valid transaction", func(t *testing.T) {
		result := validator.ValidateTransaction(validTransaction, categories)
		if !result.IsValid {
			t.Errorf("Expected valid transaction but got errors: %v", result.Errors)
		}
	})

	t.Run("Invalid transaction", func(t *testing.T) {
		result := validator.ValidateTransaction(invalidTransaction, categories)
		if result.IsValid {
			t.Errorf("Expected invalid transaction but validation passed")
		}

		expectedErrors := 4 // amount, description, date, categoryId
		if len(result.Errors) != expectedErrors {
			t.Errorf("Expected %d errors but got %d", expectedErrors, len(result.Errors))
		}

		// Check specific errors
		if !result.HasError("amount") {
			t.Errorf("Expected amount error")
		}
		if !result.HasError("description") {
			t.Errorf("Expected description error")
		}
		if !result.HasError("date") {
			t.Errorf("Expected date error")
		}
		if !result.HasError("categoryId") {
			t.Errorf("Expected categoryId error")
		}
	})
}

func TestValidationResult(t *testing.T) {
	result := types.ValidationResult{IsValid: true}

	// Test adding errors
	result.AddError("field1", "Error message 1")
	result.AddError("field2", "Error message 2")

	if result.IsValid {
		t.Errorf("Expected IsValid to be false after adding errors")
	}

	if len(result.Errors) != 2 {
		t.Errorf("Expected 2 errors but got %d", len(result.Errors))
	}

	// Test HasError
	if !result.HasError("field1") {
		t.Errorf("Expected HasError to return true for field1")
	}

	if result.HasError("nonexistent") {
		t.Errorf("Expected HasError to return false for nonexistent field")
	}

	// Test GetError
	errorMsg := result.GetError("field1")
	if errorMsg != "Error message 1" {
		t.Errorf("Expected 'Error message 1' but got '%s'", errorMsg)
	}

	emptyMsg := result.GetError("nonexistent")
	if emptyMsg != "" {
		t.Errorf("Expected empty string for nonexistent field but got '%s'", emptyMsg)
	}
}
