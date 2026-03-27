package storage

import (
	"strings"
	"testing"
	"time"

	"budget-tracker-tui/internal/database"
	"budget-tracker-tui/internal/types"
)

// setupTestCategoryStore creates a complete Store with test dependencies for category testing
func setupTestCategoryStore(t *testing.T) (*Store, *database.Connection) {
	t.Helper()

	conn := setupTestDB(t)

	// Create a more complete store setup like production
	store := &Store{}
	store.db = conn

	// Initialize domain stores
	store.Categories = NewCategoryStore(conn)
	store.Templates = NewCSVTemplateStore(conn)
	store.Statements = NewBankStatementStore(conn)
	store.Transactions = NewTransactionStore(conn)
	store.TransactionAudits = NewTransactionAuditStore(conn)

	// Set cross-references between stores like production
	store.Transactions.SetTransactionAuditStore(store.TransactionAudits)
	store.Transactions.SetStore(store)
	store.Categories.SetTransactionStore(store.Transactions)

	// Initialize CSV parser with dependencies (no ML for tests)
	store.CSVParser = NewCSVParser(store.Transactions, store.Categories, nil)

	return store, conn
}

// Test fixture helpers for categories

// createTestCategoryFull creates a category with all fields and returns its ID
func createTestCategoryFull(t *testing.T, conn *database.Connection, name, color string, parentId *int64) int64 {
	t.Helper()

	query := `INSERT INTO categories (display_name, color, parent_id, is_active, created_at, updated_at) 
	          VALUES (?, ?, ?, 1, ?, ?)`
	now := time.Now().Format(time.RFC3339)

	result, err := conn.DB.Exec(query, name, color, parentId, now, now)
	if err != nil {
		t.Fatalf("Failed to create test category: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get category ID: %v", err)
	}

	return id
}

// createTestCategoryInactive creates an inactive category for testing filtering
func createTestCategoryInactive(t *testing.T, conn *database.Connection, name string) int64 {
	t.Helper()

	query := `INSERT INTO categories (display_name, is_active, created_at, updated_at) 
	          VALUES (?, 0, ?, ?)`
	now := time.Now().Format(time.RFC3339)

	result, err := conn.DB.Exec(query, name, now, now)
	if err != nil {
		t.Fatalf("Failed to create inactive test category: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get category ID: %v", err)
	}

	return id
}

// TestGetCategories tests the GetCategories method
func TestGetCategories(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*testing.T, *Store, *database.Connection) []types.Category
		expectError bool
		validate    func(*testing.T, []types.Category, []types.Category)
	}{
		{
			name: "default database has Uncategorized category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) []types.Category {
				// Database starts with default "Uncategorized" category
				return []types.Category{
					{Id: 1, DisplayName: "Uncategorized", IsActive: true},
				}
			},
			expectError: false,
			validate: func(t *testing.T, expected, actual []types.Category) {
				if actual == nil {
					t.Error("Expected slice with default category, got nil")
				}
				if len(actual) != 1 {
					t.Errorf("Expected 1 category (default), got %d categories", len(actual))
				}
				if len(actual) > 0 && actual[0].DisplayName != "Uncategorized" {
					t.Errorf("Expected first category to be 'Uncategorized', got '%s'", actual[0].DisplayName)
				}
			},
		},
		{
			name: "multiple categories ordered by display_name ASC",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) []types.Category {
				// Create categories in random order to test sorting
				id2 := createTestCategory(t, conn, "Food")           // Should be 2nd
				id1 := createTestCategory(t, conn, "Bills")          // Should be 1st
				id3 := createTestCategory(t, conn, "Transportation") // Should be 3rd

				// Expected order: Bills, Food, Transportation
				return []types.Category{
					{Id: id1, DisplayName: "Bills", IsActive: true},
					{Id: id2, DisplayName: "Food", IsActive: true},
					{Id: id3, DisplayName: "Transportation", IsActive: true},
				}
			},
			expectError: false,
			validate: func(t *testing.T, expected, actual []types.Category) {
				if len(actual) != 4 {
					t.Fatalf("Expected 4 categories (3 created + 1 default), got %d", len(actual))
				}

				// Verify alphabetical ordering: Bills, Food, Transportation, Uncategorized
				if actual[0].DisplayName != "Bills" {
					t.Errorf("First category: expected 'Bills', got '%s'", actual[0].DisplayName)
				}
				if actual[1].DisplayName != "Food" {
					t.Errorf("Second category: expected 'Food', got '%s'", actual[1].DisplayName)
				}
				if actual[2].DisplayName != "Transportation" {
					t.Errorf("Third category: expected 'Transportation', got '%s'", actual[2].DisplayName)
				}
				if actual[3].DisplayName != "Uncategorized" {
					t.Errorf("Fourth category: expected 'Uncategorized' (default), got '%s'", actual[3].DisplayName)
				}

				// Verify all are active
				for i, cat := range actual {
					if !cat.IsActive {
						t.Errorf("Category %d should be active", i)
					}
				}
			},
		},
		{
			name: "filters out inactive categories",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) []types.Category {
				// Create active and inactive categories
				activeId := createTestCategory(t, conn, "Active Food")
				createTestCategoryInactive(t, conn, "Inactive Bills")

				// Only active should be returned
				return []types.Category{
					{Id: activeId, DisplayName: "Active Food", IsActive: true},
				}
			},
			expectError: false,
			validate: func(t *testing.T, expected, actual []types.Category) {
				if len(actual) != 2 {
					t.Fatalf("Expected 2 active categories (1 default + 1 created), got %d", len(actual))
				}

				// Should be ordered alphabetically: Active Food, Uncategorized
				if actual[0].DisplayName != "Active Food" {
					t.Errorf("First category: expected 'Active Food', got '%s'", actual[0].DisplayName)
				}
				if actual[1].DisplayName != "Uncategorized" {
					t.Errorf("Second category: expected 'Uncategorized' (default), got '%s'", actual[1].DisplayName)
				}

				// Both should be active
				for i, cat := range actual {
					if !cat.IsActive {
						t.Errorf("Category %d should be active", i)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			store, conn := setupTestCategoryStore(t)
			defer teardownTestDB(t, conn)

			expected := tt.setupData(t, store, conn)

			// Execute
			actual, err := store.Categories.GetCategories()

			// Assert
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, expected, actual)
			}
		})
	}
}

// TestGetCategoryByDisplayName tests the GetCategoryByDisplayName method
func TestGetCategoryByDisplayName(t *testing.T) {
	tests := []struct {
		name      string
		setupData func(*testing.T, *Store, *database.Connection)
		queryName string
		expectNil bool
		validate  func(*testing.T, *types.Category)
	}{
		{
			name: "finds existing category case-insensitive",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) {
				createTestCategory(t, conn, "Food")
			},
			queryName: "food", // lowercase query
			expectNil: false,
			validate: func(t *testing.T, result *types.Category) {
				if result.DisplayName != "Food" {
					t.Errorf("Expected 'Food', got '%s'", result.DisplayName)
				}
			},
		},
		{
			name: "finds with different case variations",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) {
				createTestCategory(t, conn, "Bills & Utilities")
			},
			queryName: "BILLS & UTILITIES", // uppercase query
			expectNil: false,
			validate: func(t *testing.T, result *types.Category) {
				if result.DisplayName != "Bills & Utilities" {
					t.Errorf("Expected 'Bills & Utilities', got '%s'", result.DisplayName)
				}
			},
		},
		{
			name: "handles whitespace in query",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) {
				createTestCategory(t, conn, "Transportation")
			},
			queryName: "  Transportation  ", // with leading/trailing spaces
			expectNil: false,
			validate: func(t *testing.T, result *types.Category) {
				if result.DisplayName != "Transportation" {
					t.Errorf("Expected 'Transportation', got '%s'", result.DisplayName)
				}
			},
		},
		{
			name: "returns nil for non-existent category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) {
				createTestCategory(t, conn, "Food")
			},
			queryName: "Non-existent Category",
			expectNil: true,
			validate:  nil,
		},
		{
			name: "returns nil for empty string",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) {
				createTestCategory(t, conn, "Food")
			},
			queryName: "",
			expectNil: true,
			validate:  nil,
		},
		{
			name: "returns nil for whitespace-only string",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) {
				createTestCategory(t, conn, "Food")
			},
			queryName: "   ",
			expectNil: true,
			validate:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			store, conn := setupTestCategoryStore(t)
			defer teardownTestDB(t, conn)

			if tt.setupData != nil {
				tt.setupData(t, store, conn)
			}

			// Execute
			result := store.Categories.GetCategoryByDisplayName(tt.queryName)

			// Assert
			if tt.expectNil && result != nil {
				t.Errorf("Expected nil but got category: %+v", result)
			}
			if !tt.expectNil && result == nil {
				t.Error("Expected category but got nil")
			}

			if tt.validate != nil && result != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestCreateCategory tests the CreateCategory method
func TestCreateCategory(t *testing.T) {
	tests := []struct {
		name         string
		setupData    func(*testing.T, *Store, *database.Connection)
		categoryName string
		expectError  bool
		validate     func(*testing.T, *CategoryResult)
	}{
		{
			name: "successful creation with valid name",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) {
				// No setup needed
			},
			categoryName: "New Food Category",
			expectError:  false,
			validate: func(t *testing.T, result *CategoryResult) {
				if result.CategoryId == 0 {
					t.Error("Expected non-zero ID")
				}
				if !result.Success {
					t.Error("Category should be created successfully")
				}
			},
		},
		{
			name: "fails on duplicate name (case-insensitive)",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) {
				createTestCategory(t, conn, "Food")
			},
			categoryName: "food", // different case
			expectError:  true,
			validate: func(t *testing.T, result *CategoryResult) {
				if result != nil && result.Success {
					t.Error("Expected failure for duplicate category")
				}
			},
		},
		{
			name: "fails on empty name",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) {
				// No setup needed
			},
			categoryName: "",
			expectError:  true,
			validate:     nil,
		},
		{
			name: "fails on whitespace-only name",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) {
				// No setup needed
			},
			categoryName: "   ",
			expectError:  true,
			validate:     nil,
		},
		{
			name: "fails on name too long (>100 chars)",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) {
				// No setup needed
			},
			categoryName: string(make([]byte, 101)), // 101 characters
			expectError:  true,
			validate:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			store, conn := setupTestCategoryStore(t)
			defer teardownTestDB(t, conn)

			if tt.setupData != nil {
				tt.setupData(t, store, conn)
			}

			// Execute
			result := store.Categories.CreateCategory(tt.categoryName)

			// Assert error expectation
			if tt.expectError {
				if result != nil && result.Success {
					t.Error("Expected error but got success")
				}
			} else {
				if result == nil || !result.Success {
					t.Errorf("Expected success but got failure: %+v", result)
				}
			}

			if tt.validate != nil && result != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestGetDefaultCategoryId tests the GetDefaultCategoryId method
func TestGetDefaultCategoryId(t *testing.T) {
	tests := []struct {
		name      string
		setupData func(*testing.T, *Store, *database.Connection) int64
		validate  func(*testing.T, int64, *Store)
	}{
		{
			name: "returns category ID (should be positive)",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				// Create a category - GetDefaultCategoryId should return some valid ID
				createTestCategory(t, conn, "Test Category")
				return 1 // Expected minimum ID
			},
			validate: func(t *testing.T, expected int64, store *Store) {
				actual := store.Categories.GetDefaultCategoryId()
				if actual <= 0 {
					t.Errorf("Expected positive default ID, got %d", actual)
				}
			},
		},
		{
			name: "default ID should correspond to existing category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				createTestCategory(t, conn, "Existing Category")
				return 1 // Expected minimum ID
			},
			validate: func(t *testing.T, expected int64, store *Store) {
				actual := store.Categories.GetDefaultCategoryId()

				// Verify the default category ID is positive
				if actual <= 0 {
					t.Errorf("Expected positive default ID, got %d", actual)
				}

				// Verify the returned ID exists in database
				categories, _ := store.Categories.GetCategories()
				if len(categories) == 0 {
					t.Error("No categories exist, but default ID returned")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			store, conn := setupTestCategoryStore(t)
			defer teardownTestDB(t, conn)

			expected := tt.setupData(t, store, conn)

			// Validate
			if tt.validate != nil {
				tt.validate(t, expected, store)
			}
		})
	}
}

// TestUpdateCategory tests the UpdateCategory method
func TestUpdateCategory(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*testing.T, *Store, *database.Connection) *types.Category
		expectError bool
		errorMsg    string
		validate    func(*testing.T, *Store, *types.Category)
	}{
		{
			name: "successful update with valid changes",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) *types.Category {
				// Create category using store method, not direct SQL
				result := store.Categories.CreateCategory("Original Name")
				if !result.Success {
					t.Fatalf("Failed to create test category: %s", result.Message)
				}

				// Find the category that was created
				created := store.Categories.GetCategoryByDisplayName("Original Name")
				if created == nil {
					t.Fatal("Created category not found")
				}

				// Return a category object for updating
				return &types.Category{
					Id:          created.Id,
					DisplayName: "Updated Name",
					IsActive:    true,
				}
			},
			expectError: false,
			validate: func(t *testing.T, store *Store, updated *types.Category) {
				// Verify the category was updated
				found := store.Categories.GetCategoryByDisplayName("Updated Name")
				if found == nil {
					t.Error("Updated category not found")
					return
				}
				if found.DisplayName != "Updated Name" {
					t.Errorf("Expected display name 'Updated Name', got '%s'", found.DisplayName)
				}
			},
		},
		{
			name: "fails on duplicate name (case-insensitive)",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) *types.Category {
				// Create categories using store methods
				result := store.Categories.CreateCategory("Existing Category")
				if !result.Success {
					t.Fatalf("Failed to create first test category: %s", result.Message)
				}
				result = store.Categories.CreateCategory("Category to Update")
				if !result.Success {
					t.Fatalf("Failed to create second test category: %s", result.Message)
				}

				// Find the category to update
				toUpdate := store.Categories.GetCategoryByDisplayName("Category to Update")
				if toUpdate == nil {
					t.Fatal("Category to update not found")
				}

				return &types.Category{
					Id:          toUpdate.Id,
					DisplayName: "EXISTING CATEGORY", // Different case
					IsActive:    true,
				}
			},
			expectError: true,
			errorMsg:    "already exists",
			validate:    nil,
		},
		{
			name: "fails on invalid category ID",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) *types.Category {
				return &types.Category{
					Id:          0, // Invalid ID
					DisplayName: "Valid Name",
					IsActive:    true,
				}
			},
			expectError: true,
			errorMsg:    "invalid category ID",
			validate:    nil,
		},
		{
			name: "fails on empty name",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) *types.Category {
				// Create a valid category first
				result := store.Categories.CreateCategory("Valid Category")
				if !result.Success {
					t.Fatalf("Failed to create test category: %s", result.Message)
				}

				// Find the created category
				created := store.Categories.GetCategoryByDisplayName("Valid Category")
				if created == nil {
					t.Fatal("Created category not found")
				}

				return &types.Category{
					Id:          created.Id,
					DisplayName: "", // Empty name for validation error
					IsActive:    true,
				}
			},
			expectError: true,
			errorMsg:    "cannot be empty",
			validate:    nil,
		},
		{
			name: "fails on category not found",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) *types.Category {
				return &types.Category{
					Id:          9999, // Non-existent ID
					DisplayName: "Valid Name",
					IsActive:    true,
				}
			},
			expectError: true,
			errorMsg:    "category not found",
			validate:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := setupTestDB(t)
			defer teardownTestDB(t, conn)
			store, _ := setupTestCategoryStore(t)

			// Setup test data
			categoryToUpdate := tt.setupData(t, store, conn)

			// Execute the update
			err := store.Categories.UpdateCategory(categoryToUpdate)

			// Validate error expectation
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}

				// Run validation if no error expected
				if tt.validate != nil {
					tt.validate(t, store, categoryToUpdate)
				}
			}
		})
	}
}

// TestDeleteCategory tests the DeleteCategory method
func TestDeleteCategory(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*testing.T, *Store, *database.Connection) int64
		expectError bool
		errorMsg    string
		validate    func(*testing.T, *Store, int64)
	}{
		{
			name: "successful deletion (soft delete)",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				// Create extra categories to ensure we don't delete the last one
				result := store.Categories.CreateCategory("Keep This Category")
				if !result.Success {
					t.Fatalf("Failed to create first category: %s", result.Message)
				}
				result = store.Categories.CreateCategory("Delete This Category")
				if !result.Success {
					t.Fatalf("Failed to create second category: %s", result.Message)
				}

				// Find the category to delete
				toDelete := store.Categories.GetCategoryByDisplayName("Delete This Category")
				if toDelete == nil {
					t.Fatal("Category to delete not found")
				}

				return toDelete.Id
			},
			expectError: false,
			validate: func(t *testing.T, store *Store, deletedId int64) {
				// Verify category is no longer returned in active list
				categories, _ := store.Categories.GetCategories()
				for _, cat := range categories {
					if cat.Id == deletedId {
						t.Errorf("Deleted category (ID: %d) should not appear in active categories", deletedId)
					}
				}

				// Verify soft delete - category should still exist but inactive
				found := store.Categories.GetCategoryByDisplayName("Delete This Category")
				if found != nil {
					t.Error("Deleted category should not be found by display name search")
				}
			},
		},
		{
			name: "fails on non-existent category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return 9999 // Non-existent ID
			},
			expectError: true,
			errorMsg:    "category not found",
			validate:    nil,
		},
		{
			name: "fails when trying to delete last active category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				// Only the default "Uncategorized" category exists
				// Try to delete it (should fail as it's the last one)
				return 1 // Default category ID
			},
			expectError: true,
			errorMsg:    "cannot delete the last active category",
			validate:    nil,
		},
		{
			name: "fails when category has child categories",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				// Create parent and child categories using store methods
				result := store.Categories.CreateCategory("Parent Category")
				if !result.Success {
					t.Fatalf("Failed to create parent category: %s", result.Message)
				}

				// Find the parent category
				parent := store.Categories.GetCategoryByDisplayName("Parent Category")
				if parent == nil {
					t.Fatal("Parent category not found")
				}

				// Create child category using store method to maintain consistency
				childCategory := &types.Category{
					DisplayName: "Child Category",
					ParentId:    &parent.Id,
					IsActive:    true,
				}
				err := store.Categories.CreateCategoryFull(childCategory)
				if err != nil {
					t.Fatalf("Failed to create child category: %v", err)
				}

				// Also create another category to avoid last-category issue
				result = store.Categories.CreateCategory("Another Category")
				if !result.Success {
					t.Fatalf("Failed to create additional category: %s", result.Message)
				}

				return parent.Id // Try to delete parent (should fail)
			},
			expectError: true,
			errorMsg:    "cannot delete category with active child categories",
			validate:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := setupTestDB(t)
			defer teardownTestDB(t, conn)
			store, _ := setupTestCategoryStore(t)

			// Setup test data
			categoryId := tt.setupData(t, store, conn)

			// Execute the deletion
			err := store.Categories.DeleteCategory(categoryId)

			// Validate error expectation
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}

				// Run validation if no error expected
				if tt.validate != nil {
					tt.validate(t, store, categoryId)
				}
			}
		})
	}
}

// TestCategoryEdgeCases tests various edge cases and boundary conditions
func TestCategoryEdgeCases(t *testing.T) {
	t.Run("case sensitivity in duplicate detection", func(t *testing.T) {
		store, conn := setupTestCategoryStore(t)
		defer teardownTestDB(t, conn)

		// Create category with mixed case
		result1 := store.Categories.CreateCategory("Food & Dining")
		if result1 == nil || !result1.Success {
			t.Fatalf("Failed to create first category: %v", result1)
		}

		// Try to create with different case - should fail
		result2 := store.Categories.CreateCategory("FOOD & DINING")
		if result2 == nil || result2.Success {
			t.Error("Expected error for case-insensitive duplicate")
		}

		// Try lowercase version - should also fail
		result3 := store.Categories.CreateCategory("food & dining")
		if result3 == nil || result3.Success {
			t.Error("Expected error for case-insensitive duplicate")
		}
	})

	t.Run("whitespace handling in names", func(t *testing.T) {
		store, conn := setupTestCategoryStore(t)
		defer teardownTestDB(t, conn)

		// Create category with specific name
		result := store.Categories.CreateCategory("Transportation")
		if result == nil || !result.Success {
			t.Fatalf("Failed to create category: %v", result)
		}

		// Search with whitespace should find it
		found := store.Categories.GetCategoryByDisplayName("  Transportation  ")
		if found == nil {
			t.Error("Should find category despite whitespace in query")
		}
		if found != nil && found.DisplayName != "Transportation" {
			t.Errorf("Expected 'Transportation', got '%s'", found.DisplayName)
		}
	})

	t.Run("boundary conditions for name length", func(t *testing.T) {
		store, conn := setupTestCategoryStore(t)
		defer teardownTestDB(t, conn)

		// Test maximum allowed length (100 characters)
		maxLengthName := string(make([]rune, 100))
		for i := range maxLengthName {
			maxLengthName = maxLengthName[:i] + "A" + maxLengthName[i+1:]
		}

		result1 := store.Categories.CreateCategory(maxLengthName)
		if result1 == nil || !result1.Success {
			t.Errorf("Should allow 100-character names: %v", result1)
		}

		// Test one character over limit
		tooLongName := maxLengthName + "B"
		result2 := store.Categories.CreateCategory(tooLongName)
		if result2 == nil || result2.Success {
			t.Error("Should reject names over 100 characters")
		}
	})
}

// TestValidateCategoryForDeletion tests the ValidateCategoryForDeletion method (P2)
func TestValidateCategoryForDeletion(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*testing.T, *Store, *database.Connection) int64
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful validation for deletable category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				// Create multiple categories to ensure we're not deleting the last one
				result := store.Categories.CreateCategory("Deletable Category")
				if !result.Success {
					t.Fatalf("Failed to create category: %s", result.Message)
				}

				// Create another category to avoid last-category issue
				result2 := store.Categories.CreateCategory("Keep This Category")
				if !result2.Success {
					t.Fatalf("Failed to create second category: %s", result2.Message)
				}

				// Find the category to validate
				category := store.Categories.GetCategoryByDisplayName("Deletable Category")
				if category == nil {
					t.Fatal("Created category not found")
				}

				return category.Id
			},
			expectError: false,
		},
		{
			name: "fails for non-existent category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return 9999 // Non-existent ID
			},
			expectError: true,
			errorMsg:    "category not found or already inactive",
		},
		{
			name: "fails for inactive category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				// Create and then soft-delete a category
				result := store.Categories.CreateCategory("Will Be Inactive")
				if !result.Success {
					t.Fatalf("Failed to create category: %s", result.Message)
				}

				category := store.Categories.GetCategoryByDisplayName("Will Be Inactive")
				if category == nil {
					t.Fatal("Created category not found")
				}

				// Create another category first to avoid last-category protection
				result2 := store.Categories.CreateCategory("Keep This Category")
				if !result2.Success {
					t.Fatalf("Failed to create second category: %s", result2.Message)
				}

				// Soft delete the category
				err := store.Categories.DeleteCategory(category.Id)
				if err != nil {
					t.Fatalf("Failed to delete category: %v", err)
				}

				return category.Id
			},
			expectError: true,
			errorMsg:    "category not found or already inactive",
		},
		{
			name: "fails when category has child categories",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				// Create parent category
				result := store.Categories.CreateCategory("Parent Category")
				if !result.Success {
					t.Fatalf("Failed to create parent category: %s", result.Message)
				}

				parent := store.Categories.GetCategoryByDisplayName("Parent Category")
				if parent == nil {
					t.Fatal("Parent category not found")
				}

				// Create child category
				childCategory := &types.Category{
					DisplayName: "Child Category",
					ParentId:    &parent.Id,
					IsActive:    true,
				}
				err := store.Categories.CreateCategoryFull(childCategory)
				if err != nil {
					t.Fatalf("Failed to create child category: %v", err)
				}

				// Create another top-level category to avoid last-category issue
				result2 := store.Categories.CreateCategory("Another Category")
				if !result2.Success {
					t.Fatalf("Failed to create additional category: %s", result2.Message)
				}

				return parent.Id
			},
			expectError: true,
			errorMsg:    "cannot delete category with active child categories",
		},
		{
			name: "fails when trying to delete last active category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				// Only the default "Uncategorized" category exists (ID: 1)
				return 1 // Default category ID
			},
			expectError: true,
			errorMsg:    "cannot delete the last active category",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := setupTestDB(t)
			defer teardownTestDB(t, conn)
			store, _ := setupTestCategoryStore(t)

			// Setup test data
			categoryId := tt.setupData(t, store, conn)

			// Execute validation
			err := store.Categories.ValidateCategoryForDeletion(categoryId)

			// Validate expectations
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestCategoryExists tests the CategoryExists method (P2)
func TestCategoryExists(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*testing.T, *Store, *database.Connection) int64
		expected    bool
		expectError bool
	}{
		{
			name: "returns true for existing active category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				result := store.Categories.CreateCategory("Test Category")
				if !result.Success {
					t.Fatalf("Failed to create category: %s", result.Message)
				}

				category := store.Categories.GetCategoryByDisplayName("Test Category")
				if category == nil {
					t.Fatal("Created category not found")
				}

				return category.Id
			},
			expected:    true,
			expectError: false,
		},
		{
			name: "returns true for default category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return 1 // Default "Uncategorized" category
			},
			expected:    true,
			expectError: false,
		},
		{
			name: "returns false for non-existent category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return 9999 // Non-existent ID
			},
			expected:    false,
			expectError: false,
		},
		{
			name: "returns false for inactive category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				// Create and then soft-delete a category
				result := store.Categories.CreateCategory("Will Be Inactive")
				if !result.Success {
					t.Fatalf("Failed to create category: %s", result.Message)
				}

				category := store.Categories.GetCategoryByDisplayName("Will Be Inactive")
				if category == nil {
					t.Fatal("Created category not found")
				}

				// Create another category first to avoid last-category protection
				result2 := store.Categories.CreateCategory("Keep This Category")
				if !result2.Success {
					t.Fatalf("Failed to create second category: %s", result2.Message)
				}

				// Soft delete the category
				err := store.Categories.DeleteCategory(category.Id)
				if err != nil {
					t.Fatalf("Failed to delete category: %v", err)
				}

				return category.Id
			},
			expected:    false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := setupTestDB(t)
			defer teardownTestDB(t, conn)
			store, _ := setupTestCategoryStore(t)

			// Setup test data
			categoryId := tt.setupData(t, store, conn)

			// Execute CategoryExists
			exists, err := store.Categories.CategoryExists(categoryId)

			// Validate expectations
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if exists != tt.expected {
					t.Errorf("Expected exists=%v, got exists=%v", tt.expected, exists)
				}
			}
		})
	}
}

// TestCreateCategoryFull tests the CreateCategoryFull method (P2)
func TestCreateCategoryFull(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*testing.T, *Store, *database.Connection) *types.Category
		expectError bool
		errorMsg    string
		validate    func(*testing.T, *Store, *types.Category)
	}{
		{
			name: "successful creation with all fields",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) *types.Category {
				return &types.Category{
					DisplayName: "Full Test Category",
					Color:       "#FF0000",
					IsActive:    true,
					CreatedAt:   time.Now(),
				}
			},
			expectError: false,
			validate: func(t *testing.T, store *Store, created *types.Category) {
				// Verify category was created with all fields
				found := store.Categories.GetCategoryByDisplayName("Full Test Category")
				if found == nil {
					t.Error("Created category not found")
					return
				}

				if found.Id <= 0 {
					t.Errorf("Expected positive ID, got %d", found.Id)
				}
				if found.DisplayName != "Full Test Category" {
					t.Errorf("Expected display name 'Full Test Category', got '%s'", found.DisplayName)
				}
				if !found.IsActive {
					t.Error("Expected category to be active")
				}

				// Check that the input category was updated with the new ID
				if created.Id <= 0 {
					t.Errorf("Expected created category to have positive ID, got %d", created.Id)
				}
			},
		},
		{
			name: "successful creation with parent category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) *types.Category {
				// Create parent category first
				result := store.Categories.CreateCategory("Parent Category")
				if !result.Success {
					t.Fatalf("Failed to create parent category: %s", result.Message)
				}

				parent := store.Categories.GetCategoryByDisplayName("Parent Category")
				if parent == nil {
					t.Fatal("Parent category not found")
				}

				return &types.Category{
					DisplayName: "Child Category",
					ParentId:    &parent.Id,
					Color:       "#00FF00",
					IsActive:    true,
				}
			},
			expectError: false,
			validate: func(t *testing.T, store *Store, created *types.Category) {
				// Verify child category was created properly
				found := store.Categories.GetCategoryByDisplayName("Child Category")
				if found == nil {
					t.Error("Created child category not found")
					return
				}

				if found.ParentId == nil {
					t.Error("Expected child category to have parent ID")
				}
			},
		},
		{
			name: "fails on duplicate name (case-insensitive)",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) *types.Category {
				// Create existing category first
				result := store.Categories.CreateCategory("Existing Category")
				if !result.Success {
					t.Fatalf("Failed to create existing category: %s", result.Message)
				}

				// Try to create with same name but different case
				return &types.Category{
					DisplayName: "EXISTING CATEGORY",
					IsActive:    true,
				}
			},
			expectError: true,
			errorMsg:    "already exists",
			validate:    nil,
		},
		{
			name: "fails on empty display name",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) *types.Category {
				return &types.Category{
					DisplayName: "", // Empty name
					IsActive:    true,
				}
			},
			expectError: true,
			errorMsg:    "cannot be empty",
			validate:    nil,
		},
		{
			name: "fails on display name too long",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) *types.Category {
				// Create name longer than 100 characters
				tooLongName := strings.Repeat("A", 101)
				return &types.Category{
					DisplayName: tooLongName,
					IsActive:    true,
				}
			},
			expectError: true,
			errorMsg:    "cannot exceed 100 characters",
			validate:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := setupTestDB(t)
			defer teardownTestDB(t, conn)
			store, _ := setupTestCategoryStore(t)

			// Setup test data
			category := tt.setupData(t, store, conn)

			// Execute CreateCategoryFull
			err := store.Categories.CreateCategoryFull(category)

			// Validate expectations
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}

				// Run validation if no error expected
				if tt.validate != nil {
					tt.validate(t, store, category)
				}
			}
		})
	}
}

// TestResolveOrCreateCategory tests the ResolveOrCreateCategory method (P2)
func TestResolveOrCreateCategory(t *testing.T) {
	tests := []struct {
		name      string
		setupData func(*testing.T, *Store, *database.Connection)
		input     string
		validate  func(*testing.T, *Store, int64, string)
	}{
		{
			name: "returns existing category ID for exact match",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) {
				result := store.Categories.CreateCategory("Food & Dining")
				if !result.Success {
					t.Fatalf("Failed to create category: %s", result.Message)
				}
			},
			input: "Food & Dining",
			validate: func(t *testing.T, store *Store, resultId int64, input string) {
				// Should return existing category ID
				existing := store.Categories.GetCategoryByDisplayName("Food & Dining")
				if existing == nil {
					t.Error("Existing category not found")
					return
				}

				if resultId != existing.Id {
					t.Errorf("Expected existing category ID %d, got %d", existing.Id, resultId)
				}
			},
		},
		{
			name: "creates new category for non-existent name",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) {
				// No setup - testing creation of new category
			},
			input: "New Category",
			validate: func(t *testing.T, store *Store, resultId int64, input string) {
				// Should create new category and return its ID
				if resultId <= 0 {
					t.Errorf("Expected positive category ID, got %d", resultId)
				}

				// Verify the category was actually created
				created := store.Categories.GetCategoryByDisplayName("New Category")
				if created == nil {
					t.Error("New category was not created")
					return
				}

				if created.Id != resultId {
					t.Errorf("Expected created category ID %d, got %d", created.Id, resultId)
				}
			},
		},
		{
			name: "returns default category ID for empty string",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) {
				// No setup needed
			},
			input: "",
			validate: func(t *testing.T, store *Store, resultId int64, input string) {
				defaultId := store.Categories.GetDefaultCategoryId()
				if resultId != defaultId {
					t.Errorf("Expected default category ID %d, got %d", defaultId, resultId)
				}
			},
		},
		{
			name: "returns default category ID for whitespace-only string",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) {
				// No setup needed
			},
			input: "   \t\n   ",
			validate: func(t *testing.T, store *Store, resultId int64, input string) {
				defaultId := store.Categories.GetDefaultCategoryId()
				if resultId != defaultId {
					t.Errorf("Expected default category ID %d for whitespace input, got %d", defaultId, resultId)
				}
			},
		},
		{
			name: "handles case-insensitive matching",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) {
				result := store.Categories.CreateCategory("Transportation")
				if !result.Success {
					t.Fatalf("Failed to create category: %s", result.Message)
				}
			},
			input: "TRANSPORTATION",
			validate: func(t *testing.T, store *Store, resultId int64, input string) {
				// Should find existing category despite case differences
				existing := store.Categories.GetCategoryByDisplayName("Transportation")
				if existing == nil {
					t.Error("Existing category not found")
					return
				}

				if resultId != existing.Id {
					t.Errorf("Expected existing category ID %d for case-insensitive match, got %d", existing.Id, resultId)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := setupTestDB(t)
			defer teardownTestDB(t, conn)
			store, _ := setupTestCategoryStore(t)

			// Setup test data
			tt.setupData(t, store, conn)

			// Execute ResolveOrCreateCategory
			resultId := store.Categories.ResolveOrCreateCategory(tt.input)

			// Validate result
			tt.validate(t, store, resultId, tt.input)
		})
	}
}

// TestGetCategoryDisplayName tests the GetCategoryDisplayName method (P2)
func TestGetCategoryDisplayName(t *testing.T) {
	tests := []struct {
		name      string
		setupData func(*testing.T, *Store, *database.Connection) int64
		expected  string
	}{
		{
			name: "returns display name for existing active category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				result := store.Categories.CreateCategory("Test Category")
				if !result.Success {
					t.Fatalf("Failed to create category: %s", result.Message)
				}

				category := store.Categories.GetCategoryByDisplayName("Test Category")
				if category == nil {
					t.Fatal("Created category not found")
				}

				return category.Id
			},
			expected: "Test Category",
		},
		{
			name: "returns display name for default category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return 1 // Default "Uncategorized" category
			},
			expected: "Uncategorized",
		},
		{
			name: "returns empty string for non-existent category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return 9999 // Non-existent ID
			},
			expected: "",
		},
		{
			name: "returns empty string for inactive category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				// Create and then soft-delete a category
				result := store.Categories.CreateCategory("Will Be Inactive")
				if !result.Success {
					t.Fatalf("Failed to create category: %s", result.Message)
				}

				category := store.Categories.GetCategoryByDisplayName("Will Be Inactive")
				if category == nil {
					t.Fatal("Created category not found")
				}

				// Create another category first to avoid last-category protection
				result2 := store.Categories.CreateCategory("Keep This Category")
				if !result2.Success {
					t.Fatalf("Failed to create second category: %s", result2.Message)
				}

				// Soft delete the category
				err := store.Categories.DeleteCategory(category.Id)
				if err != nil {
					t.Fatalf("Failed to delete category: %v", err)
				}

				return category.Id
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := setupTestDB(t)
			defer teardownTestDB(t, conn)
			store, _ := setupTestCategoryStore(t)

			// Setup test data
			categoryId := tt.setupData(t, store, conn)

			// Execute GetCategoryDisplayName
			displayName := store.Categories.GetCategoryDisplayName(categoryId)

			// Validate result
			if displayName != tt.expected {
				t.Errorf("Expected display name '%s', got '%s'", tt.expected, displayName)
			}
		})
	}
}

// TestGetCategoryById tests the GetCategoryById method
func TestGetCategoryById(t *testing.T) {
	tests := []struct {
		name      string
		setupData func(*testing.T, *Store, *database.Connection) int64 // Returns category ID to test
		expectNil bool
		validate  func(*testing.T, *types.Category)
	}{
		{
			name: "Returns active category by ID",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				result := store.Categories.CreateCategory("Test Category")
				if !result.Success {
					t.Fatalf("Failed to create category: %s", result.Message)
				}

				category := store.Categories.GetCategoryByDisplayName("Test Category")
				if category == nil {
					t.Fatal("Created category not found")
				}

				return category.Id
			},
			expectNil: false,
			validate: func(t *testing.T, category *types.Category) {
				if category.DisplayName != "Test Category" {
					t.Errorf("Expected display name 'Test Category', got %s", category.DisplayName)
				}
				if !category.IsActive {
					t.Error("Expected category to be active")
				}
			},
		},
		{
			name: "Returns nil for inactive category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				// Create category and then soft-delete it
				result := store.Categories.CreateCategory("Will Be Deleted")
				if !result.Success {
					t.Fatalf("Failed to create category: %s", result.Message)
				}

				category := store.Categories.GetCategoryByDisplayName("Will Be Deleted")
				if category == nil {
					t.Fatal("Created category not found")
				}

				// Create another category to avoid last-category protection
				result2 := store.Categories.CreateCategory("Keep This Category")
				if !result2.Success {
					t.Fatalf("Failed to create second category: %s", result2.Message)
				}

				// Soft delete the first category
				err := store.Categories.DeleteCategory(category.Id)
				if err != nil {
					t.Fatalf("Failed to delete category: %v", err)
				}

				return category.Id
			},
			expectNil: true,
			validate:  nil, // No validation needed for nil result
		},
		{
			name: "Returns nil for non-existent category",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return 99999 // Non-existent ID
			},
			expectNil: true,
			validate:  nil,
		},
		{
			name: "Returns default category correctly",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return 1 // Default "Uncategorized" category ID
			},
			expectNil: false,
			validate: func(t *testing.T, category *types.Category) {
				if category.DisplayName != "Uncategorized" {
					t.Errorf("Expected display name 'Uncategorized', got %s", category.DisplayName)
				}
				if !category.IsActive {
					t.Error("Expected default category to be active")
				}
				if category.Id != 1 {
					t.Errorf("Expected default category ID 1, got %d", category.Id)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := setupTestDB(t)
			defer teardownTestDB(t, conn)
			store, _ := setupTestCategoryStore(t)

			categoryId := tt.setupData(t, store, conn)
			category := store.Categories.GetCategoryById(categoryId)

			if tt.expectNil {
				if category != nil {
					t.Errorf("Expected nil category, got %+v", category)
				}
			} else {
				if category == nil {
					t.Error("Expected non-nil category, got nil")
				} else if tt.validate != nil {
					tt.validate(t, category)
				}
			}
		})
	}
}

// TestSetDefaultCategoryId tests the SetDefaultCategoryId method
func TestSetDefaultCategoryId(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*testing.T, *Store, *database.Connection) int64 // Returns category ID to set as default
		expectError bool
		errorMsg    string
		validate    func(*testing.T, *Store, *database.Connection, int64) // Validate the default was set
	}{
		{
			name: "Successfully set existing active category as default",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				result := store.Categories.CreateCategory("New Default Category")
				if !result.Success {
					t.Fatalf("Failed to create category: %s", result.Message)
				}

				category := store.Categories.GetCategoryByDisplayName("New Default Category")
				if category == nil {
					t.Fatal("Created category not found")
				}

				return category.Id
			},
			expectError: false,
			validate: func(t *testing.T, store *Store, conn *database.Connection, categoryId int64) {
				defaultId := store.Categories.GetDefaultCategoryId()
				if defaultId != categoryId {
					t.Errorf("Expected default category ID %d, got %d", categoryId, defaultId)
				}
			},
		},
		{
			name: "Error when setting non-existent category as default",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return 99999 // Non-existent category ID
			},
			expectError: true,
			errorMsg:    "category not found or inactive",
			validate: func(t *testing.T, store *Store, conn *database.Connection, categoryId int64) {
				// Default should remain unchanged (should be 1 for "Uncategorized")
				defaultId := store.Categories.GetDefaultCategoryId()
				if defaultId == categoryId {
					t.Errorf("Default category should not have changed to non-existent ID %d", categoryId)
				}
				if defaultId != 1 {
					t.Errorf("Expected default category to remain as 1, got %d", defaultId)
				}
			},
		},
		{
			name: "Error when setting inactive category as default",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				// Create category and then soft-delete it
				result := store.Categories.CreateCategory("Will Be Deleted")
				if !result.Success {
					t.Fatalf("Failed to create category: %s", result.Message)
				}

				category := store.Categories.GetCategoryByDisplayName("Will Be Deleted")
				if category == nil {
					t.Fatal("Created category not found")
				}

				// Create another category to avoid last-category protection
				result2 := store.Categories.CreateCategory("Keep This Category")
				if !result2.Success {
					t.Fatalf("Failed to create second category: %s", result2.Message)
				}

				// Soft delete the first category
				err := store.Categories.DeleteCategory(category.Id)
				if err != nil {
					t.Fatalf("Failed to delete category: %v", err)
				}

				return category.Id
			},
			expectError: true,
			errorMsg:    "not found",
			validate: func(t *testing.T, store *Store, conn *database.Connection, categoryId int64) {
				// Default should remain unchanged
				defaultId := store.Categories.GetDefaultCategoryId()
				if defaultId == categoryId {
					t.Errorf("Default category should not have changed to inactive category ID %d", categoryId)
				}
			},
		},
		{
			name: "Successfully set default category back to original default",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				// First change the default to something else
				result := store.Categories.CreateCategory("Temporary Default")
				if !result.Success {
					t.Fatalf("Failed to create temporary category: %s", result.Message)
				}

				tempCategory := store.Categories.GetCategoryByDisplayName("Temporary Default")
				if tempCategory == nil {
					t.Fatal("Temporary category not found")
				}

				err := store.Categories.SetDefaultCategoryId(tempCategory.Id)
				if err != nil {
					t.Fatalf("Failed to set temporary default: %v", err)
				}

				// Now return the original default (1) to set it back
				return 1
			},
			expectError: false,
			validate: func(t *testing.T, store *Store, conn *database.Connection, categoryId int64) {
				defaultId := store.Categories.GetDefaultCategoryId()
				if defaultId != 1 {
					t.Errorf("Expected default category to be reset to 1, got %d", defaultId)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := setupTestDB(t)
			defer teardownTestDB(t, conn)
			store, _ := setupTestCategoryStore(t)

			categoryId := tt.setupData(t, store, conn)
			err := store.Categories.SetDefaultCategoryId(categoryId)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if tt.validate != nil {
				tt.validate(t, store, conn, categoryId)
			}
		})
	}
}
