package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Field navigation helpers

func (m model) handleFieldNavigation(direction int) (tea.Model, tea.Cmd) {
	if direction > 0 && m.editField < editCategory {
		m.editField++
	} else if direction < 0 && m.editField > 0 {
		m.editField--
	}

	// Initialize amount string when entering amount field
	if m.editField == editAmount && m.editAmountStr == "" {
		m.editAmountStr = strconv.FormatFloat(m.currTransaction.Amount, 'f', 2, 64)
	}

	return m, nil
}

func (m model) handleBackspaceActivation() (tea.Model, tea.Cmd) {
	// Only activate text input fields with backspace
	switch m.editField {
	case editAmount:
		return m.enterAmountEditingWithBackspace()
	case editDescription:
		return m.enterDescriptionEditingWithBackspace()
	case editDate:
		return m.enterDateEditingWithBackspace()
	}
	return m, nil
}

func (m model) handleFieldActivation() (tea.Model, tea.Cmd) {
	switch m.editField {
	case editAmount:
		return m.enterAmountEditing()
	case editDescription:
		return m.enterDescriptionEditing()
	case editDate:
		return m.enterDateEditing()
	case editType:
		return m.enterTypeSelection()
	case editCategory:
		return m.enterCategorySelection()
	case editSplit:
		return m.enterSplitMode()
	}
	return m, nil
}

// Amount editing functions

func (m model) enterAmountEditing() (tea.Model, tea.Cmd) {
	m.isEditingAmount = true
	m.editingAmountStr = strconv.FormatFloat(m.currTransaction.Amount, 'f', 2, 64)
	return m, nil
}

func (m model) enterAmountEditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isEditingAmount = true
	// Start with current value and immediately apply backspace
	m.editingAmountStr = strconv.FormatFloat(m.currTransaction.Amount, 'f', 2, 64)
	if len(m.editingAmountStr) > 0 {
		m.editingAmountStr = m.editingAmountStr[:len(m.editingAmountStr)-1]
	}
	return m, nil
}

func (m model) handleAmountEditing(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "esc":
		m.isEditingAmount = false
		if key == "enter" && m.editingAmountStr != "" {
			if amount, err := strconv.ParseFloat(m.editingAmountStr, 64); err == nil {
				m.currTransaction.Amount = amount
				// Validate field on enter (field commit)
				m.validateCurrentTransaction()
			}
		}
		return m, nil
	case "backspace":
		if len(m.editingAmountStr) > 0 {
			m.editingAmountStr = m.editingAmountStr[:len(m.editingAmountStr)-1]
		}
		return m, nil
	default:
		return m.handleAmountInput(key)
	}
}

func (m model) handleAmountInput(key string) (tea.Model, tea.Cmd) {
	// Handle negative sign
	if key == "-" {
		if !strings.HasPrefix(m.editingAmountStr, "-") {
			m.editingAmountStr = "-" + m.editingAmountStr
		}
		return m, nil
	}

	// Only allow digits and decimal point
	if len(key) == 1 {
		char := key[0]
		if char >= '0' && char <= '9' {
			m.editingAmountStr += key
		} else if char == '.' && !strings.Contains(m.editingAmountStr, ".") {
			m.editingAmountStr += key
		}
	}

	// Validate decimal places (max 2)
	if parts := strings.Split(m.editingAmountStr, "."); len(parts) == 2 && len(parts[1]) > 2 {
		m.editingAmountStr = parts[0] + "." + parts[1][:2]
	}

	return m, nil
}

// Description editing functions

func (m model) enterDescriptionEditing() (tea.Model, tea.Cmd) {
	m.isEditingDescription = true
	m.editingDescStr = m.currTransaction.Description
	return m, nil
}

func (m model) enterDescriptionEditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isEditingDescription = true
	// Start with current value and immediately apply backspace
	m.editingDescStr = m.currTransaction.Description
	if len(m.editingDescStr) > 0 {
		m.editingDescStr = m.editingDescStr[:len(m.editingDescStr)-1]
	}
	return m, nil
}

func (m model) handleDescriptionEditing(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "esc":
		m.isEditingDescription = false
		if key == "enter" {
			m.currTransaction.Description = m.editingDescStr
			// Validate field on enter (field commit)
			m.validateCurrentTransaction()
		}
		return m, nil
	case "backspace":
		if len(m.editingDescStr) > 0 {
			m.editingDescStr = m.editingDescStr[:len(m.editingDescStr)-1]
		}
		return m, nil
	default:
		if len(key) == 1 {
			m.editingDescStr += key
		}
		return m, nil
	}
}

// Date editing functions

func (m model) enterDateEditing() (tea.Model, tea.Cmd) {
	m.isEditingDate = true
	m.editingDateStr = m.currTransaction.Date
	return m, nil
}

func (m model) enterDateEditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isEditingDate = true
	// Start with current value and immediately apply backspace
	m.editingDateStr = m.currTransaction.Date
	if len(m.editingDateStr) > 0 {
		m.editingDateStr = m.editingDateStr[:len(m.editingDateStr)-1]
	}
	return m, nil
}

func (m model) handleDateEditing(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "esc":
		m.isEditingDate = false
		if key == "enter" {
			m.currTransaction.Date = m.editingDateStr
			// Validate field on enter (field commit)
			m.validateCurrentTransaction()
		}
		return m, nil
	case "backspace":
		if len(m.editingDateStr) > 0 {
			m.editingDateStr = m.editingDateStr[:len(m.editingDateStr)-1]
		}
		return m, nil
	default:
		if len(key) == 1 {
			m.editingDateStr += key
		}
		return m, nil
	}
}

// Selection helpers for categories and types

func (m model) enterCategorySelection() (tea.Model, tea.Cmd) {
	m.isSelectingCategory = true

	// Find current category in list for initial position
	categories, _ := m.store.Categories.GetCategories()
	for i, cat := range categories {
		if cat.Id == m.currTransaction.CategoryId {
			m.categorySelectIndex = i
			break
		}
	}

	return m, nil
}

func (m model) handleCategorySelection(key string) (tea.Model, tea.Cmd) {
	categories, _ := m.store.Categories.GetCategories()

	switch key {
	case "up":
		if m.categorySelectIndex > 0 {
			m.categorySelectIndex--
		}
	case "down":
		if m.categorySelectIndex < len(categories)-1 {
			m.categorySelectIndex++
		}
	case "enter":
		if len(categories) > 0 {
			m.currTransaction.CategoryId = categories[m.categorySelectIndex].Id
			// Validate field on selection commit
			m.validateCurrentTransaction()
		}
		m.isSelectingCategory = false
	case "esc":
		m.isSelectingCategory = false
	}
	return m, nil
}

func (m model) enterTypeSelection() (tea.Model, tea.Cmd) {
	m.isSelectingType = true

	// Find current type in list
	for i, t := range m.availableTypes {
		if t == m.currTransaction.TransactionType {
			m.typeSelectIndex = i
			break
		}
	}

	return m, nil
}

func (m model) handleTypeSelection(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up":
		if m.typeSelectIndex > 0 {
			m.typeSelectIndex--
		}
	case "down":
		if m.typeSelectIndex < len(m.availableTypes)-1 {
			m.typeSelectIndex++
		}
	case "enter":
		m.currTransaction.TransactionType = m.availableTypes[m.typeSelectIndex]
		// Validate transaction type selection
		m.validateCurrentTransaction()
		m.isSelectingType = false
	case "esc":
		m.isSelectingType = false
	}
	return m, nil
}
