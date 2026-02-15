package main

import tea "github.com/charmbracelet/bubbletea"

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
	case "enter":
		// Set selected category as default
		categories, _ := m.store.GetCategories()
		if len(categories) > 0 && m.categoryIndex < len(categories) {
			selectedCategory := categories[m.categoryIndex]
			m.store.categories.Default = selectedCategory.Name
			err := m.store.saveCategories()
			if err != nil {
				m.categoryMessage = "Error setting default: " + err.Error()
			} else {
				m.categoryMessage = "Default category updated"
			}
		}
	case "c":
		m.state = createCategoryView
		m.createCategoryField = createCategoryName
		m.newCategory = Category{}
		m.categoryMessage = ""
	case "esc":
		m.state = menuView
	}
	return m, nil
}

func (m model) handleCreateCategoryView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.state = categoryView
	case "enter":
		return m.handleSaveCategory()
	case "up":
		if m.createCategoryField > createCategoryName {
			m.createCategoryField--
		}
	case "down", "tab":
		if m.createCategoryField < createCategoryDisplayName {
			m.createCategoryField++
		}
	case "backspace":
		return m.handleCreateCategoryBackspace()
	default:
		return m.handleCreateCategoryInput(key)
	}
	return m, nil
}

func (m model) handleCreateCategoryInput(key string) (tea.Model, tea.Cmd) {
	switch m.createCategoryField {
	case createCategoryName:
		m.newCategory.Name += key
	case createCategoryDisplayName:
		m.newCategory.DisplayName += key
	}
	return m, nil
}

func (m model) handleCreateCategoryBackspace() (tea.Model, tea.Cmd) {
	switch m.createCategoryField {
	case createCategoryName:
		if len(m.newCategory.Name) > 0 {
			m.newCategory.Name = m.newCategory.Name[:len(m.newCategory.Name)-1]
		}
	case createCategoryDisplayName:
		if len(m.newCategory.DisplayName) > 0 {
			m.newCategory.DisplayName = m.newCategory.DisplayName[:len(m.newCategory.DisplayName)-1]
		}
	}
	return m, nil
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
