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
	// Only handle DisplayName field now
	return m.enterCategoryDisplayNameEditingWithBackspace()
}

func (m model) handleCategoryFieldActivation() (tea.Model, tea.Cmd) {
	// Only handle DisplayName field now
	return m.enterCategoryDisplayNameEditing()
}

// Only DisplayName editing functions are now needed
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
	// Validate display name is not empty
	if m.newCategory.DisplayName == "" {
		m.categoryMessage = "Error: Display name cannot be empty"
		return m, nil
	}

	// Add new category (now only needs DisplayName)
	err := m.store.AddCategory(m.newCategory.DisplayName)
	if err != nil {
		m.categoryMessage = "Error: " + err.Error()
		return m, nil
	}

	// Return to category view
	m.state = categoryView
	m.categoryMessage = "Category added successfully"
	return m, nil
}
