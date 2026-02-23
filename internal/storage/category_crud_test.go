package storage_test

import (
	"budget-tracker-tui/internal/storage"
	"budget-tracker-tui/internal/types"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"
)

func createTestStore(t *testing.T) *storage.Store {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "category_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Clean up temp dir after test
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	store := &storage.Store{}

	// Initialize store with test directory
	// We need to simulate the Init() method but with our temp directory
	// Since Init() uses home directory, we'll create a minimal test setup

	return store
}

// Generate unique category name for tests
func uniqueCategoryName(prefix string) string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), rand.Intn(10000))
}

func TestCreateCategoryFull(t *testing.T) {
	store := createTestStore(t)

	// Initialize basic category store for testing
	err := store.Init()
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}

	tests := []struct {
		name          string
		category      *types.Category
		expectError   bool
		errorContains string
	}{
		{
			name: "Valid category creation",
			category: &types.Category{
				DisplayName: uniqueCategoryName("TestFood"),
				Color:       "#FF0000",
				ParentId:    nil,
			},
			expectError: false,
		},
		{
			name: "Category with parent",
			category: &types.Category{
				DisplayName: uniqueCategoryName("FastFood"),
				Color:       "#00FF00",
				ParentId:    &[]int64{1}[0], // Assuming Food category exists with ID 1
			},
			expectError: false,
		},
		{
			name: "Empty display name",
			category: &types.Category{
				DisplayName: "",
				Color:       "#FF0000",
			},
			expectError:   true,
			errorContains: "empty",
		},
		{
			name: "Invalid color format",
			category: &types.Category{
				DisplayName: uniqueCategoryName("ValidName"),
				Color:       "invalid-color",
			},
			expectError:   true,
			errorContains: "color",
		},
		{
			name: "Duplicate category name",
			category: &types.Category{
				DisplayName: "Food & Dining", // This should already exist from default categories
			},
			expectError:   true,
			errorContains: "already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.CreateCategoryFull(tt.category)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !containsIgnoreCase(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s' but got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				} else {
					// Verify category was created with proper metadata
					categories, _ := store.GetCategories()
					found := false
					for _, cat := range categories {
						if cat.DisplayName == tt.category.DisplayName {
							found = true
							if cat.Id == 0 {
								t.Errorf("Created category should have valid ID")
							}
							if cat.CreatedAt == "" {
								t.Errorf("Created category should have CreatedAt timestamp")
							}
							if cat.UpdatedAt == "" {
								t.Errorf("Created category should have UpdatedAt timestamp")
							}
							if !cat.IsActive {
								t.Errorf("Created category should be active")
							}
							break
						}
					}
					if !found {
						t.Errorf("Category was not found after creation")
					}
				}
			}
		})
	}
}

func TestUpdateCategory(t *testing.T) {
	store := createTestStore(t)
	err := store.Init()
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}

	// Create a test category first
	testCategoryName := uniqueCategoryName("OriginalName")
	testCategory := &types.Category{
		DisplayName: testCategoryName,
		Color:       "#FF0000",
	}
	err = store.CreateCategoryFull(testCategory)
	if err != nil {
		t.Fatalf("Failed to create test category: %v", err)
	}

	// Get the created category to get its ID
	categories, _ := store.GetCategories()
	var createdCategory *types.Category
	for _, cat := range categories {
		if cat.DisplayName == testCategoryName {
			createdCategory = &cat
			break
		}
	}

	if createdCategory == nil {
		t.Fatalf("Could not find created test category")
	}

	tests := []struct {
		name           string
		updateCategory *types.Category
		expectError    bool
		errorContains  string
	}{
		{
			name: "Valid category update",
			updateCategory: &types.Category{
				Id:          createdCategory.Id,
				DisplayName: uniqueCategoryName("UpdatedName"),
				Color:       "#00FF00",
			},
			expectError: false,
		},
		{
			name: "Update non-existent category",
			updateCategory: &types.Category{
				Id:          99999,
				DisplayName: uniqueCategoryName("NonExistent"),
				Color:       "#FF0000",
			},
			expectError:   true,
			errorContains: "not found",
		},
		{
			name: "Update with empty name",
			updateCategory: &types.Category{
				Id:          createdCategory.Id,
				DisplayName: "",
				Color:       "#FF0000",
			},
			expectError:   true,
			errorContains: "empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.UpdateCategory(tt.updateCategory)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !containsIgnoreCase(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s' but got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				} else {
					// Verify category was updated
					updatedCategory := store.GetCategoryById(tt.updateCategory.Id)
					if updatedCategory == nil {
						t.Errorf("Updated category not found")
					} else if updatedCategory.DisplayName != tt.updateCategory.DisplayName {
						t.Errorf("Category name not updated. Expected '%s', got '%s'",
							tt.updateCategory.DisplayName, updatedCategory.DisplayName)
					}
				}
			}
		})
	}
}

func TestDeleteCategory(t *testing.T) {
	store := createTestStore(t)
	err := store.Init()
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}

	// Create test categories with unique names
	leafCategoryName := uniqueCategoryName("LeafCategory")
	leafCategory := &types.Category{
		DisplayName: leafCategoryName,
		Color:       "#FF0000",
	}
	err = store.CreateCategoryFull(leafCategory)
	if err != nil {
		t.Fatalf("Failed to create leaf category: %v", err)
	}

	parentCategoryName := uniqueCategoryName("ParentCategory")
	parentCategory := &types.Category{
		DisplayName: parentCategoryName,
		Color:       "#00FF00",
	}
	err = store.CreateCategoryFull(parentCategory)
	if err != nil {
		t.Fatalf("Failed to create parent category: %v", err)
	}

	// Get created categories
	categories, _ := store.GetCategories()
	var leafCat, parentCat *types.Category
	for _, cat := range categories {
		if cat.DisplayName == leafCategoryName {
			leafCat = &cat
		} else if cat.DisplayName == parentCategoryName {
			parentCat = &cat
		}
	}

	// Create child category
	childCategoryName := uniqueCategoryName("ChildCategory")
	childCategory := &types.Category{
		DisplayName: childCategoryName,
		Color:       "#0000FF",
		ParentId:    &parentCat.Id,
	}
	err = store.CreateCategoryFull(childCategory)
	if err != nil {
		t.Fatalf("Failed to create child category: %v", err)
	}

	tests := []struct {
		name          string
		categoryId    int64
		expectError   bool
		errorContains string
	}{
		{
			name:        "Delete leaf category (no children, no transactions)",
			categoryId:  leafCat.Id,
			expectError: false,
		},
		{
			name:          "Delete category with children",
			categoryId:    parentCat.Id,
			expectError:   true,
			errorContains: "subcategories",
		},
		{
			name:          "Delete non-existent category",
			categoryId:    99999,
			expectError:   true,
			errorContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.DeleteCategory(tt.categoryId)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !containsIgnoreCase(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s' but got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				} else {
					// Verify category was deleted
					deletedCategory := store.GetCategoryById(tt.categoryId)
					if deletedCategory != nil {
						t.Errorf("Category should have been deleted but still exists")
					}
				}
			}
		})
	}
}

func TestGetCategoryHierarchy(t *testing.T) {
	store := createTestStore(t)
	err := store.Init()
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}

	// Create hierarchical categories with unique names
	// Level 1: Main Food Category
	foodCategoryName := uniqueCategoryName("MainFoodCategory")
	foodCategory := &types.Category{
		DisplayName: foodCategoryName,
		Color:       "#FF0000",
	}
	err = store.CreateCategoryFull(foodCategory)
	if err != nil {
		t.Fatalf("Failed to create food category: %v", err)
	}

	// Get the created food category ID
	categories, _ := store.GetCategories()
	var foodId int64
	for _, cat := range categories {
		if cat.DisplayName == foodCategoryName {
			foodId = cat.Id
			break
		}
	}

	// Level 2: Quick Meals (child of Main Food Category)
	subFoodCategoryName := uniqueCategoryName("QuickMeals")
	subFoodCategory := &types.Category{
		DisplayName: subFoodCategoryName,
		Color:       "#00FF00",
		ParentId:    &foodId,
	}
	err = store.CreateCategoryFull(subFoodCategory)
	if err != nil {
		t.Fatalf("Failed to create sub food category: %v", err)
	}

	// Get hierarchy
	hierarchy := store.GetCategoryHierarchy()

	// Verify structure
	if len(hierarchy) == 0 {
		t.Errorf("Expected hierarchy to contain categories")
	}

	// Verify Main Food Category comes before Quick Meals in hierarchy
	foodIndex, subFoodIndex := -1, -1
	for i, cat := range hierarchy {
		if cat.DisplayName == foodCategoryName {
			foodIndex = i
		} else if cat.DisplayName == subFoodCategoryName {
			subFoodIndex = i
		}
	}

	if foodIndex == -1 {
		t.Errorf("Main Food Category not found in hierarchy")
	}
	if subFoodIndex == -1 {
		t.Errorf("Quick Meals category not found in hierarchy")
	}
	if foodIndex >= subFoodIndex {
		t.Errorf("Parent category should come before child in hierarchy")
	}

	t.Logf("Hierarchy order: %s at index %d, %s at index %d", foodCategoryName, foodIndex, subFoodCategoryName, subFoodIndex)
}

func TestValidateCategoryForDeletion(t *testing.T) {
	store := createTestStore(t)
	err := store.Init()
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}

	// Get default category ID (should not be deletable due to either being default or having transactions)
	defaultCategoryId := store.GetDefaultCategoryId()

	// Create a fresh category that should be deletable
	deletableCategoryName := uniqueCategoryName("DeletableTestCategory")
	deletableCategory := &types.Category{
		DisplayName: deletableCategoryName,
		Color:       "#FF0000",
	}
	err = store.CreateCategoryFull(deletableCategory)
	if err != nil {
		t.Fatalf("Failed to create deletable category: %v", err)
	}

	// Get the created category
	categories, _ := store.GetCategories()
	var deletableCategoryId int64
	for _, cat := range categories {
		if cat.DisplayName == deletableCategoryName {
			deletableCategoryId = cat.Id
			break
		}
	}

	tests := []struct {
		name          string
		categoryId    int64
		expectError   bool
		errorContains string
	}{
		{
			name:          "Validate deletion of default category (should fail)",
			categoryId:    defaultCategoryId,
			expectError:   true,
			errorContains: "cannot delete", // Could be due to being default or having transactions
		},
		{
			name:        "Validate deletion of safe category",
			categoryId:  deletableCategoryId,
			expectError: false,
		},
		{
			name:          "Validate deletion of non-existent category",
			categoryId:    99999,
			expectError:   true,
			errorContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.ValidateCategoryForDeletion(tt.categoryId)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !containsIgnoreCase(err.Error(), tt.errorContains) {
					t.Logf("Got error: %s", err.Error()) // Log the actual error for debugging
					t.Errorf("Expected error containing '%s' but got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestGetCategoriesForParentSelection(t *testing.T) {
	store := createTestStore(t)
	err := store.Init()
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}

	// Create test categories for parent selection with unique names
	parentCategoryName := uniqueCategoryName("ParentTest")
	parentCategory := &types.Category{
		DisplayName: parentCategoryName,
		Color:       "#FF0000",
	}
	err = store.CreateCategoryFull(parentCategory)
	if err != nil {
		t.Fatalf("Failed to create parent category: %v", err)
	}

	// Get created category
	categories, _ := store.GetCategories()
	var parentId int64
	for _, cat := range categories {
		if cat.DisplayName == parentCategoryName {
			parentId = cat.Id
			break
		}
	}

	// Test parent selection (should exclude the category itself)
	availableParents := store.GetCategoriesForParentSelection(parentId)

	// Verify the category itself is not in the list
	for _, cat := range availableParents {
		if cat.Id == parentId {
			t.Errorf("Category should not be available as its own parent")
		}
	}

	t.Logf("Available parents count: %d", len(availableParents))
}

// TestNextCategoryId verifies the ID generation works correctly
func TestNextCategoryId(t *testing.T) {
	store := createTestStore(t)
	err := store.Init()
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}

	// Get next ID
	nextId := store.GetNextCategoryId()
	if nextId <= 0 {
		t.Errorf("Next category ID should be positive, got %d", nextId)
	}

	// Create a category and verify ID increments
	testCategoryName := uniqueCategoryName("IDTestCategory")
	testCategory := &types.Category{
		DisplayName: testCategoryName,
		Color:       "#FF0000",
	}
	err = store.CreateCategoryFull(testCategory)
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Get next ID again
	newNextId := store.GetNextCategoryId()
	if newNextId <= nextId {
		t.Errorf("Next ID should have incremented. Previous: %d, Current: %d", nextId, newNextId)
	}
}

// Helper function for case-insensitive string contains check
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(substr == "" ||
			findSubstring(strings.ToLower(s), strings.ToLower(substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
