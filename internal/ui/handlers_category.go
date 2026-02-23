package ui

import (
	"budget-tracker-tui/internal/types"

	tea "github.com/charmbracelet/bubbletea"
)

// Category management handlers
func (m model) handleCategoryView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up":
		if m.categoryIndex > 0 {
			m.categoryIndex--
		}
	case "down":
		categories, _ := m.store.GetCategories()
		if m.categoryIndex < len(categories)-1 {
			m.categoryIndex++
		}
	case "c":
		m.state = createCategoryView
		m.createCategoryField = createCategoryName
		m.newCategory = types.Category{}
		m.categoryMessage = ""
	case "esc":
		m.state = menuView
	}
	return m, nil
}

func (m model) handleCreateCategoryView(key string) (tea.Model, tea.Cmd) {
	// Handle active editing states
	if m.isEditingCategoryName {
		return m.handleCategoryNameEditing(key)
	}
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
	case "down", "tab":
		return m.handleCreateCategoryFieldNavigation(1)
	case "up":
		return m.handleCreateCategoryFieldNavigation(-1)
	}
	return m, nil
}

// Category field navigation
func (m model) handleCreateCategoryFieldNavigation(direction int) (tea.Model, tea.Cmd) {
	if direction > 0 && m.createCategoryField < createCategoryDisplayName {
		m.createCategoryField++
	} else if direction < 0 && m.createCategoryField > createCategoryName {
		m.createCategoryField--
	}
	return m, nil
}

func (m model) handleCategoryBackspaceActivation() (tea.Model, tea.Cmd) {
	switch m.createCategoryField {
	case createCategoryName:
		return m.enterCategoryNameEditingWithBackspace()
	case createCategoryDisplayName:
		return m.enterCategoryDisplayNameEditingWithBackspace()
	}
	return m, nil
}

func (m model) handleCategoryFieldActivation() (tea.Model, tea.Cmd) {
	switch m.createCategoryField {
	case createCategoryName:
		return m.enterCategoryNameEditing()
	case createCategoryDisplayName:
		return m.enterCategoryDisplayNameEditing()
	}
	return m, nil
}

// Category Name editing functions
func (m model) enterCategoryNameEditing() (tea.Model, tea.Cmd) {
	m.isEditingCategoryName = true
	m.editingCategoryNameStr = m.newCategory.Name
	return m, nil
}

func (m model) enterCategoryNameEditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isEditingCategoryName = true
	m.editingCategoryNameStr = m.newCategory.Name
	if len(m.editingCategoryNameStr) > 0 {
		m.editingCategoryNameStr = m.editingCategoryNameStr[:len(m.editingCategoryNameStr)-1]
	}
	return m, nil
}

func (m model) handleCategoryNameEditing(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "esc":
		m.isEditingCategoryName = false
		if key == "enter" {
			m.newCategory.Name = m.editingCategoryNameStr
		}
		return m, nil
	case "backspace":
		if len(m.editingCategoryNameStr) > 0 {
			m.editingCategoryNameStr = m.editingCategoryNameStr[:len(m.editingCategoryNameStr)-1]
		}
		return m, nil
	default:
		if len(key) == 1 {
			m.editingCategoryNameStr += key
		}
		return m, nil
	}
}

// Category Display Name editing functions
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
	// Validate category name is not empty
	if m.newCategory.Name == "" {
		m.categoryMessage = "Error: Category name cannot be empty"
		return m, nil
	}

	// Validate display name is not empty
	if m.newCategory.DisplayName == "" {
		m.categoryMessage = "Error: Display name cannot be empty"
		return m, nil
	}

	// Add new category
	err := m.store.AddCategory(m.newCategory.Name, m.newCategory.DisplayName)
	if err != nil {
		m.categoryMessage = "Error: " + err.Error()
		return m, nil
	}

	// Return to category view
	m.state = categoryView
	m.categoryMessage = "Category added successfully"
	return m, nil
}
