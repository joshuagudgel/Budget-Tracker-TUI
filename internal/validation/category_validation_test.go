package validation_test

import (
	"budget-tracker-tui/internal/types"
	"budget-tracker-tui/internal/validation"
	"testing"
)

func TestCategoryValidationDisplayName(t *testing.T) {
	validator := validation.NewCategoryManagementValidator()

	tests := []struct {
		name        string
		displayName string
		expectError bool
		errorMsg    string
	}{
		{"Valid name", "Food & Dining", false, ""},
		{"Empty name", "", true, "category name cannot be empty"},
		{"Whitespace only", "   ", true, "category name cannot be empty"},
		{"Max length name", string(make([]byte, 100)), false, ""},
		{"Too long name", string(make([]byte, 101)), true, "category name cannot exceed 100 characters"},
		{"Single character", "A", false, ""},
		{"Special characters", "Bills & Utilities", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateDisplayName(tt.displayName)
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

func TestCategoryValidationColor(t *testing.T) {
	validator := validation.NewCategoryManagementValidator()

	tests := []struct {
		name        string
		color       string
		expectError bool
	}{
		{"Valid hex color uppercase", "#FF0000", false},
		{"Valid hex color lowercase", "#ff0000", false},
		{"Valid hex color mixed", "#Ff0000", false},
		{"Empty color (optional)", "", false},
		{"Invalid format - no hash", "FF0000", true},
		{"Invalid format - too short", "#FF00", true},
		{"Invalid format - too long", "#FF00000", true},
		{"Invalid characters", "#GGGGGG", true},
		{"Invalid characters - special", "#FF00!0", true},
		{"Valid color with numbers", "#123456", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateColor(tt.color)
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

func TestCategoryValidationParent(t *testing.T) {
	validator := validation.NewCategoryManagementValidator()

	categories := []types.Category{
		{Id: 1, DisplayName: "Parent1", ParentId: nil},
		{Id: 2, DisplayName: "Parent2", ParentId: nil},
		{Id: 3, DisplayName: "Child1", ParentId: &[]int64{1}[0]},
		{Id: 4, DisplayName: "Grandchild", ParentId: &[]int64{3}[0]},
	}

	tests := []struct {
		name        string
		parentId    *int64
		currentId   int64
		expectError bool
		description string
	}{
		{"No parent (top-level)", nil, 5, false, "Top-level category should be valid"},
		{"Valid parent", &[]int64{1}[0], 5, false, "Valid parent reference"},
		{"Self-reference", &[]int64{1}[0], 1, true, "Category cannot be its own parent"},
		{"Non-existent parent", &[]int64{99}[0], 5, true, "Parent does not exist"},
		{"Circular reference", &[]int64{4}[0], 1, true, "Would create circular reference"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateParent(tt.parentId, categories, tt.currentId)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none for: %s", tt.description)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v for: %s", err, tt.description)
				}
			}
		})
	}
}

func TestCategoryCRUDValidation(t *testing.T) {
	validator := validation.NewCategoryManagementValidator()

	existingCategories := []types.Category{
		{Id: 1, DisplayName: "Existing1", ParentId: nil},
		{Id: 2, DisplayName: "Existing2", ParentId: nil},
	}

	validCategory := &types.Category{
		Id:          3,
		DisplayName: "New Category",
		Color:       "#FF0000",
		ParentId:    &[]int64{1}[0], // Parent to existing category 1
	}

	invalidCategory := &types.Category{
		Id:          4,
		DisplayName: "",              // Invalid: empty name
		Color:       "invalid-color", // Invalid: wrong format
		ParentId:    &[]int64{99}[0], // Invalid: non-existent parent
	}

	t.Run("Valid category", func(t *testing.T) {
		result := validator.ValidateCategory(validCategory, existingCategories)
		if !result.IsValid {
			t.Errorf("Expected valid category but got errors: %v", result.Errors)
		}
	})

	t.Run("Invalid category", func(t *testing.T) {
		result := validator.ValidateCategory(invalidCategory, existingCategories)
		if result.IsValid {
			t.Errorf("Expected invalid category but validation passed")
		}

		expectedErrors := 3 // displayName, color, parentId
		if len(result.Errors) != expectedErrors {
			t.Errorf("Expected %d errors but got %d", expectedErrors, len(result.Errors))
		}

		// Check specific errors
		if !result.HasError("displayName") {
			t.Errorf("Expected displayName error")
		}
		if !result.HasError("color") {
			t.Errorf("Expected color error")
		}
		if !result.HasError("parentId") {
			t.Errorf("Expected parentId error")
		}
	})
}

func TestCategoryFieldValidation(t *testing.T) {
	validator := validation.NewCategoryManagementValidator()

	categories := []types.Category{
		{Id: 1, DisplayName: "Parent", ParentId: nil},
	}

	category := &types.Category{
		Id:          2,
		DisplayName: "Test Category",
		Color:       "#FF0000",
		ParentId:    &[]int64{1}[0],
	}

	tests := []struct {
		name           string
		field          string
		expectError    bool
		modifyCategory func(*types.Category)
	}{
		{
			"Valid displayName field",
			"displayName",
			false,
			func(c *types.Category) { c.DisplayName = "Valid Name" },
		},
		{
			"Invalid displayName field",
			"displayName",
			true,
			func(c *types.Category) { c.DisplayName = "" },
		},
		{
			"Valid color field",
			"color",
			false,
			func(c *types.Category) { c.Color = "#00FF00" },
		},
		{
			"Invalid color field",
			"color",
			true,
			func(c *types.Category) { c.Color = "invalid" },
		},
		{
			"Valid parent field",
			"parentId",
			false,
			func(c *types.Category) { c.ParentId = &[]int64{1}[0] },
		},
		{
			"Invalid parent field",
			"parentId",
			true,
			func(c *types.Category) { c.ParentId = &[]int64{99}[0] },
		},
		{
			"Unknown field",
			"unknownField",
			true,
			func(c *types.Category) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCategory := *category // Copy category
			tt.modifyCategory(&testCategory)

			err := validator.ValidateCategoryField(&testCategory, tt.field, categories)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for field '%s' but got none", tt.field)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for field '%s' but got: %v", tt.field, err)
				}
			}
		})
	}
}

func TestCategoryDeletionValidation(t *testing.T) {
	validator := validation.NewCategoryManagementValidator()

	categories := []types.Category{
		{Id: 1, DisplayName: "Parent", ParentId: nil},
		{Id: 2, DisplayName: "Child", ParentId: &[]int64{1}[0]},
		{Id: 3, DisplayName: "Unused", ParentId: nil},
	}

	transactions := []types.Transaction{
		{Id: 1, CategoryId: 1, Description: "Transaction using Parent category"},
		{Id: 2, CategoryId: 2, Description: "Transaction using Child category"},
	}

	tests := []struct {
		name        string
		categoryId  int64
		expectError bool
		description string
	}{
		{"Delete category with transactions", 1, true, "Category 1 is used by transactions"},
		{"Delete category with subcategories", 1, true, "Category 1 has subcategories"},
		{"Delete unused leaf category", 3, false, "Category 3 is not used and has no children"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateForDeletion(tt.categoryId, transactions, categories)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for %s but got none", tt.description)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for %s but got: %v", tt.description, err)
				}
			}
		})
	}
}

func TestCategoryNameSuggestions(t *testing.T) {
	validator := validation.NewCategoryManagementValidator()

	existingCategories := []types.Category{
		{Id: 1, DisplayName: "Food & Dining"},
		{Id: 2, DisplayName: "Transportation"},
	}

	tests := []struct {
		name             string
		partial          string
		expectedCount    int
		shouldNotContain []string
	}{
		{"Food suggestions", "food", 0, []string{"Food & Dining"}},        // Already exists
		{"Transport suggestions", "trans", 0, []string{"Transportation"}}, // Already exists
		{"Entertainment suggestions", "enter", 1, []string{}},
		{"Utility suggestions", "util", 1, []string{}},
		{"No matches", "xyz", 0, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := validator.GetCategoryNameSuggestions(tt.partial, existingCategories)

			if len(suggestions) != tt.expectedCount {
				t.Errorf("Expected %d suggestions but got %d", tt.expectedCount, len(suggestions))
			}

			// Check that existing categories are not suggested
			for _, suggestion := range suggestions {
				for _, notContain := range tt.shouldNotContain {
					if suggestion == notContain {
						t.Errorf("Suggestion should not contain existing category '%s'", notContain)
					}
				}
			}
		})
	}
}

func TestCategoryColorSuggestions(t *testing.T) {
	validator := validation.NewCategoryManagementValidator()

	suggestions := validator.GetColorSuggestions()

	if len(suggestions) == 0 {
		t.Errorf("Expected color suggestions but got none")
	}

	// Verify all suggestions are valid hex colors
	for _, color := range suggestions {
		err := validator.ValidateColor(color)
		if err != nil {
			t.Errorf("Invalid color suggestion '%s': %v", color, err)
		}
	}
}

func TestCategoryStructValidation(t *testing.T) {
	// Test the validation methods added to the Category struct itself
	categories := []types.Category{
		{Id: 1, DisplayName: "Parent", ParentId: nil},
	}

	validCategory := types.Category{
		Id:          2,
		DisplayName: "Valid Category",
		Color:       "#FF0000",
		ParentId:    &[]int64{1}[0],
	}

	invalidCategory := types.Category{
		Id:          3,
		DisplayName: "",
		Color:       "invalid",
		ParentId:    &[]int64{99}[0],
	}

	t.Run("Valid category struct validation", func(t *testing.T) {
		result := validCategory.Validate(categories)
		if !result.IsValid {
			t.Errorf("Expected valid category but got errors: %v", result.Errors)
		}
	})

	t.Run("Invalid category struct validation", func(t *testing.T) {
		result := invalidCategory.Validate(categories)
		if result.IsValid {
			t.Errorf("Expected invalid category but validation passed")
		}

		if !result.HasError("displayName") {
			t.Errorf("Expected displayName error")
		}
		if !result.HasError("color") {
			t.Errorf("Expected color error")
		}
		if !result.HasError("parentId") {
			t.Errorf("Expected parentId error")
		}
	})

	t.Run("Category field validation", func(t *testing.T) {
		// Test valid field
		err := validCategory.ValidateField("displayName", categories)
		if err != nil {
			t.Errorf("Expected no error for valid displayName field but got: %v", err)
		}

		// Test invalid field
		err = invalidCategory.ValidateField("displayName", categories)
		if err == nil {
			t.Errorf("Expected error for invalid displayName field but got none")
		}

		// Test unknown field
		err = validCategory.ValidateField("unknownField", categories)
		if err == nil {
			t.Errorf("Expected error for unknown field but got none")
		}
	})
}
