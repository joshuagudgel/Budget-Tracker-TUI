package storage

import (
	"budget-tracker-tui/internal/database"
	"budget-tracker-tui/internal/types"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// CategoryStore handles all category-related operations using SQLite
type CategoryStore struct {
	db        *database.Connection
	helper    *database.SQLHelper
	defaultId int64
}

// NewCategoryStore creates a new CategoryStore instance
func NewCategoryStore(db *database.Connection) *CategoryStore {
	store := &CategoryStore{
		db:        db,
		helper:    database.NewSQLHelper(db),
		defaultId: 1, // Default to "Uncategorized" category
	}

	// Ensure default categories exist
	store.ensureDefaultCategories()

	return store
}

// ensureDefaultCategories creates default categories if they don't exist
func (cs *CategoryStore) ensureDefaultCategories() {
	// Check if categories exist
	count, err := cs.helper.CountBy("categories", "")
	if err != nil || count > 0 {
		return // Categories already exist or error occurred
	}

	// Create default categories (note: ID 1 "Uncategorized" is created by schema)
	defaultCategories := []types.Category{
		{DisplayName: "Food & Dining", IsActive: true},
		{DisplayName: "Transportation", IsActive: true},
		{DisplayName: "Entertainment", IsActive: true},
		{DisplayName: "Utilities", IsActive: true},
		{DisplayName: "Shopping", IsActive: true},
		{DisplayName: "Healthcare", IsActive: true},
	}

	for _, category := range defaultCategories {
		cs.CreateCategoryFull(&category) // Ignore errors during initialization
	}
}

// CalculateNextCategoryId calculates the next available category ID using SQLite
func (cs *CategoryStore) CalculateNextCategoryId() int64 {
	maxID, err := cs.helper.GetMaxID("categories", "id")
	if err != nil {
		return 1 // Default to 1 if error or no records
	}
	return maxID + 1
}

// GetCategories returns all categories from the database
func (cs *CategoryStore) GetCategories() ([]types.Category, error) {
	query := `
		SELECT id, display_name, parent_id, color, is_active, created_at, updated_at 
		FROM categories 
		WHERE is_active = 1 
		ORDER BY display_name
	`

	rows, err := cs.helper.QueryRows(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer rows.Close()

	var categories []types.Category
	for rows.Next() {
		category, err := cs.scanCategory(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, category)
	}

	return categories, rows.Err()
}

// GetDefaultCategoryId returns the default category ID
func (cs *CategoryStore) GetDefaultCategoryId() int64 {
	return cs.defaultId
}

// SetDefaultCategoryId sets the default category ID
func (cs *CategoryStore) SetDefaultCategoryId(categoryId int64) error {
	// Verify category exists
	exists, err := cs.helper.ExistsBy("categories", "id = ? AND is_active = 1", categoryId)
	if err != nil {
		return fmt.Errorf("failed to check category existence: %w", err)
	}

	if !exists {
		return fmt.Errorf("category not found or inactive")
	}

	cs.defaultId = categoryId
	return nil
}

// CategoryExists checks if a category with the given ID exists and is active
func (cs *CategoryStore) CategoryExists(categoryId int64) (bool, error) {
	return cs.helper.ExistsBy("categories", "id = ? AND is_active = 1", categoryId)
}

// GetCategoryDisplayName returns the display name for a category ID, or empty string if not found
func (cs *CategoryStore) GetCategoryDisplayName(categoryId int64) string {
	query := "SELECT display_name FROM categories WHERE id = ? AND is_active = 1"

	var displayName string
	err := cs.helper.QuerySingleRow(query, categoryId).Scan(&displayName)
	if err != nil {
		return "" // Category not found or error
	}

	return displayName
}

// scanCategory scans a database row into a Category struct
func (cs *CategoryStore) scanCategory(rows *sql.Rows) (types.Category, error) {
	var category types.Category
	var parentID sql.NullInt64
	var color sql.NullString
	var createdAtStr, updatedAtStr string

	err := rows.Scan(
		&category.Id, &category.DisplayName, &parentID, &color,
		&category.IsActive, &createdAtStr, &updatedAtStr,
	)

	if err != nil {
		return category, err
	}

	// Parse time fields from database
	category.CreatedAt, err = cs.helper.ParseTimeFromDB(createdAtStr)
	if err != nil {
		return category, fmt.Errorf("failed to parse created_at: %w", err)
	}
	category.UpdatedAt, err = cs.helper.ParseTimeFromDB(updatedAtStr)
	if err != nil {
		return category, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	// Handle nullable fields
	if parentID.Valid {
		category.ParentId = &parentID.Int64
	}
	if color.Valid {
		category.Color = color.String
	}

	return category, nil
}

// GetCategoryById returns a category by its ID, or nil if not found
func (cs *CategoryStore) GetCategoryById(categoryId int64) *types.Category {
	query := `
		SELECT id, display_name, parent_id, color, is_active, created_at, updated_at 
		FROM categories 
		WHERE id = ? AND is_active = 1
	`

	row := cs.helper.QuerySingleRow(query, categoryId)
	category, err := cs.scanCategoryRow(row)
	if err != nil {
		return nil // Category not found or error
	}

	return &category
}

// scanCategoryRow scans a single database row into a Category struct
func (cs *CategoryStore) scanCategoryRow(row *sql.Row) (types.Category, error) {
	var category types.Category
	var parentID sql.NullInt64
	var color sql.NullString
	var createdAtStr, updatedAtStr string

	err := row.Scan(
		&category.Id, &category.DisplayName, &parentID, &color,
		&category.IsActive, &createdAtStr, &updatedAtStr,
	)

	if err != nil {
		return category, err
	}

	// Parse time fields from database
	category.CreatedAt, err = cs.helper.ParseTimeFromDB(createdAtStr)
	if err != nil {
		return category, fmt.Errorf("failed to parse created_at: %w", err)
	}
	category.UpdatedAt, err = cs.helper.ParseTimeFromDB(updatedAtStr)
	if err != nil {
		return category, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	// Handle nullable fields
	if parentID.Valid {
		category.ParentId = &parentID.Int64
	}
	if color.Valid {
		category.Color = color.String
	}

	return category, nil
}

// GetCategoryByDisplayName returns a category by its display name (case insensitive), or nil if not found
func (cs *CategoryStore) GetCategoryByDisplayName(displayName string) *types.Category {
	trimmed := strings.TrimSpace(displayName)
	if trimmed == "" {
		return nil
	}

	query := `
		SELECT id, display_name, parent_id, color, is_active, created_at, updated_at 
		FROM categories 
		WHERE LOWER(display_name) = LOWER(?) AND is_active = 1
	`

	row := cs.helper.QuerySingleRow(query, trimmed)
	category, err := cs.scanCategoryRow(row)
	if err != nil {
		return nil // Category not found or error
	}

	return &category
}

// ResolveOrCreateCategory resolves a category by name or creates it if it doesn't exist
// Returns the category ID
func (cs *CategoryStore) ResolveOrCreateCategory(categoryText string) int64 {
	if categoryText == "" {
		return cs.defaultId
	}

	// Clean up category text
	cleanText := strings.TrimSpace(categoryText)
	if cleanText == "" {
		return cs.defaultId
	}

	// Try exact match first
	existingCategory := cs.GetCategoryByDisplayName(cleanText)
	if existingCategory != nil {
		return existingCategory.Id
	}

	// Create new category for unmatched text
	result := cs.CreateCategory(cleanText)
	if result.Success {
		return result.CategoryId
	}

	// Fallback to default category if creation failed
	return cs.defaultId
}

// CreateCategory creates a new category with display name only (legacy method)
func (cs *CategoryStore) CreateCategory(displayName string) *CategoryResult {
	result := &CategoryResult{}

	// Validate inputs
	if strings.TrimSpace(displayName) == "" {
		result.Message = "Display name cannot be empty"
		return result
	}

	// Check for duplicates by display name (case insensitive)
	existing := cs.GetCategoryByDisplayName(strings.TrimSpace(displayName))
	if existing != nil {
		result.Message = "Category display name already exists"
		return result
	}

	// Create new category
	category := &types.Category{
		DisplayName: strings.TrimSpace(displayName),
		IsActive:    true,
	}

	err := cs.CreateCategoryFull(category)
	if err != nil {
		result.Message = fmt.Sprintf("Failed to create category: %v", err)
		return result
	}

	result.Success = true
	result.CategoryId = category.Id
	result.Message = fmt.Sprintf("Category '%s' created successfully", displayName)
	return result
}

// CreateCategoryFull creates a new category with full category object support
func (cs *CategoryStore) CreateCategoryFull(category *types.Category) error {
	// Validate the category using built-in validation
	existingCategories, _ := cs.GetCategories()
	result := category.Validate(existingCategories)
	if !result.IsValid {
		// Return the first validation error
		return fmt.Errorf("%s", result.Errors[0].Message)
	}

	// Check for duplicates by display name (case insensitive)
	existing := cs.GetCategoryByDisplayName(category.DisplayName)
	if existing != nil {
		return fmt.Errorf("category '%s' already exists", category.DisplayName)
	}

	now := time.Now()

	query := `
		INSERT INTO categories (display_name, parent_id, color, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	// Handle nullable fields
	var parentID interface{}
	if category.ParentId != nil {
		parentID = *category.ParentId
	}

	var color interface{}
	if category.Color != "" {
		color = category.Color
	}

	// Set creation timestamp if not provided
	createdAt := category.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}

	// Format times for database storage
	createdAtStr := createdAt.Format(time.RFC3339)
	updatedAtStr := now.Format(time.RFC3339)

	id, err := cs.helper.ExecReturnID(query,
		strings.TrimSpace(category.DisplayName), parentID, color,
		true, createdAtStr, updatedAtStr,
	)

	if err != nil {
		return fmt.Errorf("failed to create category: %w", err)
	}

	// Update the category ID
	category.Id = id
	category.IsActive = true
	category.CreatedAt = createdAt
	category.UpdatedAt = now

	// Audit: Category creation events are no longer recorded

	return nil
}

// UpdateCategory updates an existing category
func (cs *CategoryStore) UpdateCategory(category *types.Category) error {
	// Validation check before update
	if category.Id <= 0 {
		return fmt.Errorf("invalid category ID")
	}

	// Validate the category using built-in validation
	existingCategories, _ := cs.GetCategories()
	result := category.Validate(existingCategories)
	if !result.IsValid {
		// Return the first validation error
		return fmt.Errorf("%s", result.Errors[0].Message)
	}

	// Check for display name conflicts (excluding self)
	existing := cs.GetCategoryByDisplayName(category.DisplayName)
	if existing != nil && existing.Id != category.Id {
		return fmt.Errorf("category '%s' already exists", category.DisplayName)
	}

	now := time.Now()
	query := `
		UPDATE categories SET 
			display_name = ?, parent_id = ?, color = ?, is_active = ?, updated_at = ?
		WHERE id = ?
	`

	// Handle nullable fields
	var parentID interface{}
	if category.ParentId != nil {
		parentID = *category.ParentId
	}

	var color interface{}
	if category.Color != "" {
		color = category.Color
	}

	rowsAffected, err := cs.helper.ExecReturnRowsAffected(query,
		strings.TrimSpace(category.DisplayName), parentID, color,
		category.IsActive, now, category.Id,
	)

	if err != nil {
		return fmt.Errorf("failed to update category: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("category not found")
	}

	// Update the category timestamps
	category.UpdatedAt = now

	// Audit: Category updates are no longer recorded

	return nil
}

// DeleteCategory safely deletes a category (sets as inactive)
func (cs *CategoryStore) DeleteCategory(categoryId int64) error {
	// First validate that deletion is safe
	err := cs.ValidateCategoryForDeletion(categoryId)
	if err != nil {
		return err
	}

	now := time.Now()
	query := "UPDATE categories SET is_active = 0, updated_at = ? WHERE id = ?"

	rowsAffected, err := cs.helper.ExecReturnRowsAffected(query, now, categoryId)
	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("category not found")
	}

	// Check if this was the default category and reset if needed
	if cs.defaultId == categoryId {
		// Find first active category to set as new default
		categories, err := cs.GetCategories()
		if err == nil && len(categories) > 0 {
			cs.defaultId = categories[0].Id
		} else {
			cs.defaultId = 1 // Fallback to "Uncategorized"
		}
	}

	// Audit: Category deletion (soft delete) events are no longer recorded

	return nil
}

// ValidateCategoryForDeletion checks if a category can be safely deleted
func (cs *CategoryStore) ValidateCategoryForDeletion(categoryId int64) error {
	// Check if category exists and is active
	exists, err := cs.helper.ExistsBy("categories", "id = ? AND is_active = 1", categoryId)
	if err != nil {
		return fmt.Errorf("failed to check category existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("category not found or already inactive")
	}

	// Check for child categories
	hasChildren, err := cs.helper.ExistsBy("categories", "parent_id = ? AND is_active = 1", categoryId)
	if err != nil {
		return fmt.Errorf("failed to check for child categories: %w", err)
	}
	if hasChildren {
		return fmt.Errorf("cannot delete category with active child categories")
	}

	// Count active categories to ensure at least one remains
	activeCount, err := cs.helper.CountBy("categories", "is_active = 1")
	if err != nil {
		return fmt.Errorf("failed to count active categories: %w", err)
	}
	if activeCount <= 1 {
		return fmt.Errorf("cannot delete the last active category")
	}

	// Note: We don't check for transaction usage here as that would require
	// coordination with TransactionStore. The UI or main Store should handle
	// that validation at a higher level.

	return nil
}

// GetCategoryHierarchy returns categories sorted by parent-child relationship
func (cs *CategoryStore) GetCategoryHierarchy() []types.Category {
	// Get all active categories first
	categories, err := cs.GetCategories()
	if err != nil {
		return []types.Category{}
	}

	var result []types.Category

	// Helper function to recursively add children
	var addChildren func(parentId *int64, level int)
	addChildren = func(parentId *int64, level int) {
		for _, cat := range categories {
			// Check if this category belongs at this level
			if (parentId == nil && cat.ParentId == nil) ||
				(parentId != nil && cat.ParentId != nil && *cat.ParentId == *parentId) {
				result = append(result, cat)
				// Recursively add children of this category
				addChildren(&cat.Id, level+1)
			}
		}
	}

	// Start with top-level categories (no parent)
	addChildren(nil, 0)
	return result
}

// GetCategoriesForParentSelection returns categories suitable for parent selection
// (excludes the category itself and its descendants to prevent circular references)
func (cs *CategoryStore) GetCategoriesForParentSelection(excludeCategoryId int64) []types.Category {
	categories, err := cs.GetCategories()
	if err != nil {
		return []types.Category{}
	}

	var result []types.Category
	excluded := make(map[int64]bool)

	// Helper function to mark category and all descendants as excluded
	var markExcluded func(categoryId int64)
	markExcluded = func(categoryId int64) {
		excluded[categoryId] = true
		for _, cat := range categories {
			if cat.ParentId != nil && *cat.ParentId == categoryId {
				markExcluded(cat.Id)
			}
		}
	}

	// Mark the category and all its descendants as excluded
	markExcluded(excludeCategoryId)

	// Add all non-excluded categories to result
	for _, cat := range categories {
		if !excluded[cat.Id] {
			result = append(result, cat)
		}
	}

	return result
}
