package storage

import (
	"budget-tracker-tui/internal/types"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// CategoryStore handles all category-related operations
type CategoryStore struct {
	filename   string
	Categories []types.Category `json:"categories"`
	DefaultId  int64            `json:"defaultId"`
	NextId     int64            `json:"nextId"`
}

// NewCategoryStore creates a new CategoryStore instance
func NewCategoryStore(categoryFile string) *CategoryStore {
	return &CategoryStore{
		filename:   categoryFile,
		Categories: []types.Category{},
		DefaultId:  0,
		NextId:     1,
	}
}

// LoadCategories loads categories from the JSON file
func (cs *CategoryStore) LoadCategories() error {
	if _, err := os.Stat(cs.filename); os.IsNotExist(err) {
		// Create default categories
		now := time.Now().Format(time.RFC3339)
		cs.Categories = []types.Category{
			{Id: 1, DisplayName: "Food & Dining", IsActive: true, CreatedAt: now, UpdatedAt: now},
			{Id: 2, DisplayName: "Transportation", IsActive: true, CreatedAt: now, UpdatedAt: now},
			{Id: 3, DisplayName: "Entertainment", IsActive: true, CreatedAt: now, UpdatedAt: now},
			{Id: 4, DisplayName: "Utilities", IsActive: true, CreatedAt: now, UpdatedAt: now},
			{Id: 5, DisplayName: "Unsorted", IsActive: true, CreatedAt: now, UpdatedAt: now},
			{Id: 6, DisplayName: "Sorted", IsActive: true, CreatedAt: now, UpdatedAt: now},
		}
		cs.DefaultId = 5 // Unsorted
		cs.NextId = 7
		return cs.SaveCategories()
	}

	data, err := os.ReadFile(cs.filename)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, cs)
	if err != nil {
		return err
	}

	// Initialize NextId if not set or calculate from existing categories
	if cs.NextId == 0 {
		cs.NextId = cs.CalculateNextCategoryId()
	}

	return nil
}

// SaveCategories saves categories to the JSON file
func (cs *CategoryStore) SaveCategories() error {
	data, err := json.MarshalIndent(cs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cs.filename, data, 0644)
}

// CalculateNextCategoryId calculates the next available category ID
func (cs *CategoryStore) CalculateNextCategoryId() int64 {
	var maxId int64 = 0
	for _, cat := range cs.Categories {
		if cat.Id > maxId {
			maxId = cat.Id
		}
	}
	return maxId + 1
}

// GetCategories returns all categories
func (cs *CategoryStore) GetCategories() ([]types.Category, error) {
	return cs.Categories, nil
}

// GetDefaultCategoryId returns the default category ID
func (cs *CategoryStore) GetDefaultCategoryId() int64 {
	return cs.DefaultId
}

// SetDefaultCategoryId sets the default category ID
func (cs *CategoryStore) SetDefaultCategoryId(categoryId int64) error {
	// Verify category exists
	found := false
	for _, cat := range cs.Categories {
		if cat.Id == categoryId {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("category not found")
	}

	cs.DefaultId = categoryId
	return cs.SaveCategories()
}

// GetCategoryDisplayName returns the display name for a category ID, or empty string if not found
func (cs *CategoryStore) GetCategoryDisplayName(categoryId int64) string {
	for _, category := range cs.Categories {
		if category.Id == categoryId {
			return category.DisplayName
		}
	}
	return ""
}

// GetCategoryById returns a category by its ID, or nil if not found
func (cs *CategoryStore) GetCategoryById(categoryId int64) *types.Category {
	for _, category := range cs.Categories {
		if category.Id == categoryId {
			return &category
		}
	}
	return nil
}

// GetCategoryByDisplayName returns a category by its display name (case insensitive), or nil if not found
func (cs *CategoryStore) GetCategoryByDisplayName(displayName string) *types.Category {
	trimmed := strings.TrimSpace(displayName)
	for _, category := range cs.Categories {
		if strings.EqualFold(category.DisplayName, trimmed) {
			return &category
		}
	}
	return nil
}

// CreateCategory creates a new category with display name only (legacy method)
func (cs *CategoryStore) CreateCategory(displayName string) *CategoryResult {
	result := &CategoryResult{}

	// Validate inputs
	if strings.TrimSpace(displayName) == "" {
		result.Message = "Display name cannot be empty"
		return result
	}

	// Check for duplicates by display name
	for _, cat := range cs.Categories {
		if strings.EqualFold(cat.DisplayName, strings.TrimSpace(displayName)) {
			result.Message = "Category display name already exists"
			return result
		}
	}

	err := cs.AddCategory(displayName)
	if err != nil {
		result.Message = fmt.Sprintf("Failed to add category: %v", err)
		return result
	}

	result.Success = true
	result.Message = fmt.Sprintf("Category '%s' created successfully", displayName)
	return result
}

// AddCategory adds a new category with display name only
func (cs *CategoryStore) AddCategory(displayName string) error {
	// Check for duplicates by display name
	for _, cat := range cs.Categories {
		if strings.EqualFold(cat.DisplayName, strings.TrimSpace(displayName)) {
			return fmt.Errorf("category '%s' already exists", displayName)
		}
	}

	now := time.Now().Format(time.RFC3339)
	category := types.Category{
		Id:          cs.NextId,
		DisplayName: strings.TrimSpace(displayName),
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	cs.Categories = append(cs.Categories, category)
	cs.NextId++
	return cs.SaveCategories()
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

	// Check for duplicates by display name
	for _, cat := range cs.Categories {
		if strings.EqualFold(cat.DisplayName, strings.TrimSpace(category.DisplayName)) {
			return fmt.Errorf("category '%s' already exists", category.DisplayName)
		}
	}

	// Set metadata
	now := time.Now().Format(time.RFC3339)
	category.Id = cs.NextId
	category.CreatedAt = now
	category.UpdatedAt = now
	category.IsActive = true

	// Add to store
	cs.Categories = append(cs.Categories, *category)
	cs.NextId++

	return cs.SaveCategories()
}

// UpdateCategory updates an existing category
func (cs *CategoryStore) UpdateCategory(category *types.Category) error {
	// Validate the category using built-in validation
	existingCategories, _ := cs.GetCategories()
	result := category.Validate(existingCategories)
	if !result.IsValid {
		// Return the first validation error
		return fmt.Errorf("%s", result.Errors[0].Message)
	}

	// Find the category to update
	categoryIndex := -1
	for i, cat := range cs.Categories {
		if cat.Id == category.Id {
			categoryIndex = i
			break
		}
	}

	if categoryIndex == -1 {
		return fmt.Errorf("category with ID %d not found", category.Id)
	}

	// Check for duplicate display name (excluding current category)
	for i, cat := range cs.Categories {
		if i != categoryIndex && strings.EqualFold(cat.DisplayName, strings.TrimSpace(category.DisplayName)) {
			return fmt.Errorf("category '%s' already exists", category.DisplayName)
		}
	}

	// Preserve original creation time and update timestamp
	existingCategory := cs.Categories[categoryIndex]
	category.CreatedAt = existingCategory.CreatedAt
	category.UpdatedAt = time.Now().Format(time.RFC3339)

	// Update the category
	cs.Categories[categoryIndex] = *category

	return cs.SaveCategories()
}

// DeleteCategory deletes a category with safety checks
func (cs *CategoryStore) DeleteCategory(categoryId int64) error {
	// Find the category to delete
	categoryIndex := -1
	for i, cat := range cs.Categories {
		if cat.Id == categoryId {
			categoryIndex = i
			break
		}
	}

	if categoryIndex == -1 {
		return fmt.Errorf("category with ID %d not found", categoryId)
	}

	// NOTE: Transaction usage checks need to be done by the caller (main Store)
	// since CategoryStore doesn't have access to transactions

	// Check if category has subcategories
	for _, cat := range cs.Categories {
		if cat.ParentId != nil && *cat.ParentId == categoryId {
			return fmt.Errorf("cannot delete category: it has subcategories. Delete or reassign subcategories first")
		}
	}

	// Check if this is the default category
	if cs.DefaultId == categoryId {
		return fmt.Errorf("cannot delete the default category. Set a different default category first")
	}

	// Remove the category
	cs.Categories = append(
		cs.Categories[:categoryIndex],
		cs.Categories[categoryIndex+1:]...,
	)

	return cs.SaveCategories()
}

// ValidateCategoryForDeletion validates if a category can be safely deleted
// Note: This doesn't check transaction usage - that must be done by the caller
func (cs *CategoryStore) ValidateCategoryForDeletion(categoryId int64) error {
	// Find the category
	var targetCategory *types.Category
	for _, cat := range cs.Categories {
		if cat.Id == categoryId {
			targetCategory = &cat
			break
		}
	}

	if targetCategory == nil {
		return fmt.Errorf("category with ID %d not found", categoryId)
	}

	// NOTE: Transaction usage checks need to be done by caller

	// Check if category has subcategories
	subcategoryCount := 0
	var subcategoryNames []string
	for _, cat := range cs.Categories {
		if cat.ParentId != nil && *cat.ParentId == categoryId {
			subcategoryCount++
			subcategoryNames = append(subcategoryNames, cat.DisplayName)
		}
	}

	if subcategoryCount > 0 {
		return fmt.Errorf("cannot delete category '%s': it has %d subcategorie(s) (%s). Delete or reassign subcategories first",
			targetCategory.DisplayName, subcategoryCount, strings.Join(subcategoryNames, ", "))
	}

	// Check if this is the default category
	if cs.DefaultId == categoryId {
		return fmt.Errorf("cannot delete category '%s': it is the default category. Set a different default category first",
			targetCategory.DisplayName)
	}

	return nil
}

// GetCategoryHierarchy returns categories sorted by parent-child relationship
func (cs *CategoryStore) GetCategoryHierarchy() []types.Category {
	var result []types.Category

	// Helper function to recursively add children
	var addChildren func(parentId *int64, level int)
	addChildren = func(parentId *int64, level int) {
		for _, cat := range cs.Categories {
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
	var result []types.Category

	// Get all descendants of the excluded category
	excludeIds := make(map[int64]bool)
	excludeIds[excludeCategoryId] = true

	// Helper to find all descendants
	var findDescendants func(parentId int64)
	findDescendants = func(parentId int64) {
		for _, cat := range cs.Categories {
			if cat.ParentId != nil && *cat.ParentId == parentId {
				if !excludeIds[cat.Id] {
					excludeIds[cat.Id] = true
					findDescendants(cat.Id) // Recursively find deeper descendants
				}
			}
		}
	}

	findDescendants(excludeCategoryId)

	// Return categories not in the exclude list
	for _, cat := range cs.Categories {
		if !excludeIds[cat.Id] {
			result = append(result, cat)
		}
	}

	return result
}
