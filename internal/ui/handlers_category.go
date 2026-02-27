package ui

import (
	"budget-tracker-tui/internal/types"

	tea "github.com/charmbracelet/bubbletea"
)

// Phase 3: Enhanced Category Management Handlers

// handleCategoryListView handles the main category list view
func (m model) handleCategoryListView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up":
		m.navigateCategoryUp()
	case "down":
		m.navigateCategoryDown()
	case "n":
		// Create new category
		m.state = categoryCreateView
		m.editingCategory = types.Category{}
		m.initCategoryFields()
		m.categoryActiveField = 0
		m.categoryEditingField = false
		m.categoryMessage = ""
		return m, m.loadCategories()
	case "e":
		// Edit selected category
		if m.selectedCategoryIdx >= 0 && m.selectedCategoryIdx < len(m.categories) {
			m.state = categoryEditView
			m.editingCategory = m.categories[m.selectedCategoryIdx]
			m.initCategoryFields()
			m.categoryActiveField = 0
			m.categoryEditingField = false
			m.categoryMessage = ""
		}
		return m, nil
	case "d":
		// Delete selected category
		return m, m.deleteCategoryWithValidation()
	case "q", "esc":
		m.state = menuView
	}
	return m, nil
}

// handleCategoryEditView handles category editing (both create and edit)
func (m model) handleCategoryEditView(key string) (tea.Model, tea.Cmd) {
	// Handle parent selection mode
	if m.isSelectingParent {
		return m, m.handleParentCategorySelection(key)
	}

	// Handle field editing mode
	if m.categoryEditingField {
		return m, m.handleCategoryFieldEdit(key)
	}

	// Handle field navigation and activation
	switch key {
	case "up":
		m.handleCategoryFieldNavigation(-1)
	case "down":
		m.handleCategoryFieldNavigation(1)
	case "enter":
		return m.activateCategoryFieldEditing()
	case "backspace":
		return m.activateCategoryFieldEditingWithBackspace()
	case "ctrl+s":
		return m.saveCategoryAndReturn()
	case "esc":
		m.state = categoryListView
		return m, m.loadCategories()
	}
	return m, nil
}

// handleCategoryCreateView handles category creation
func (m model) handleCategoryCreateView(key string) (tea.Model, tea.Cmd) {
	// Delegate to edit view handler since they use the same logic
	return m.handleCategoryEditView(key)
}

// Legacy category handlers (maintaining backwards compatibility)

// handleCategoryView handles the legacy category view
func (m model) handleCategoryView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up":
		if m.categoryIndex > 0 {
			m.categoryIndex--
		}
	case "down":
		categories, _ := m.store.Categories.GetCategories()
		if m.categoryIndex < len(categories)-1 {
			m.categoryIndex++
		}
	case "c":
		// Redirect to new category creation view
		m.state = categoryCreateView
		m.editingCategory = types.Category{}
		m.initCategoryFields()
		m.categoryActiveField = 0
		m.categoryEditingField = false
		m.categoryMessage = ""
		return m, m.loadCategories()
	case "esc":
		m.state = menuView
	}
	return m, nil
}

// handleCreateCategoryView handles legacy category creation
func (m model) handleCreateCategoryView(key string) (tea.Model, tea.Cmd) {
	// Handle active editing states
	if m.isEditingCategoryDisplayName {
		return m.handleCategoryDisplayNameEditing(key)
	}

	switch key {
	case "esc":
		m.state = categoryView
	case "enter":
		return m.handleCategoryFieldActivation()
	case "backspace":
		return m.handleCategoryBackspaceActivation()
	case "ctrl+s":
		return m.handleSaveCategory()
	}
	return m, nil
}

// Legacy field editing methods (for backwards compatibility)

func (m model) handleCategoryFieldActivation() (tea.Model, tea.Cmd) {
	// Only handle DisplayName field now
	return m.enterCategoryDisplayNameEditing()
}

func (m model) handleCategoryBackspaceActivation() (tea.Model, tea.Cmd) {
	// Only handle DisplayName field now
	return m.enterCategoryDisplayNameEditingWithBackspace()
}

func (m model) enterCategoryDisplayNameEditing() (tea.Model, tea.Cmd) {
	m.isEditingCategoryDisplayName = true
	m.editingCategoryDisplayNameStr = m.newCategory.DisplayName
	return m, nil
}

func (m model) enterCategoryDisplayNameEditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isEditingCategoryDisplayName = true
	m.editingCategoryDisplayNameStr = m.newCategory.DisplayName
	if len(m.editingCategoryDisplayNameStr) > 0 {
		m.editingCategoryDisplayNameStr = m.editingCategoryDisplayNameStr[:len(m.editingCategoryDisplayNameStr)-1]
	}
	return m, nil
}

func (m model) handleCategoryDisplayNameEditing(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "esc":
		m.isEditingCategoryDisplayName = false
		if key == "enter" {
			m.newCategory.DisplayName = m.editingCategoryDisplayNameStr
		}
		return m, nil
	case "backspace":
		if len(m.editingCategoryDisplayNameStr) > 0 {
			m.editingCategoryDisplayNameStr = m.editingCategoryDisplayNameStr[:len(m.editingCategoryDisplayNameStr)-1]
		}
		return m, nil
	default:
		if len(key) == 1 {
			m.editingCategoryDisplayNameStr += key
		}
		return m, nil
	}
}

func (m model) handleSaveCategory() (tea.Model, tea.Cmd) {
	// Validate display name is not empty
	if m.newCategory.DisplayName == "" {
		m.categoryMessage = "Error: Display name cannot be empty"
		return m, nil
	}

	// Add new category (now only needs DisplayName)
	result := m.store.Categories.CreateCategory(m.newCategory.DisplayName)
	if !result.Success {
		m.categoryMessage = "Error: " + result.Message
		return m, nil
	}

	// Return to category view
	m.state = categoryView
	m.categoryMessage = "Category added successfully"
	return m, nil
}
