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

// Template Field Navigation

func (m model) handleTemplateFieldNavigation(direction int) (tea.Model, tea.Cmd) {
	if direction > 0 && m.createField < templateHeader {
		m.createField++
	} else if direction < 0 && m.createField > templateName {
		m.createField--
	}

	// Initialize editing strings when entering fields
	m.initTemplateEditingStrings()

	return m, nil
}

func (m *model) initTemplateEditingStrings() {
	if m.editingTemplateNameStr == "" {
		m.editingTemplateNameStr = m.newTemplate.Name
	}
	if m.editingTemplatePostDateStr == "" {
		m.editingTemplatePostDateStr = strconv.Itoa(m.newTemplate.PostDateColumn)
	}
	if m.editingTemplateAmountStr == "" {
		m.editingTemplateAmountStr = strconv.Itoa(m.newTemplate.AmountColumn)
	}
	if m.editingTemplateDescStr == "" {
		m.editingTemplateDescStr = strconv.Itoa(m.newTemplate.DescColumn)
	}
	if m.editingTemplateCategoryStr == "" && m.newTemplate.CategoryColumn != nil {
		m.editingTemplateCategoryStr = strconv.Itoa(*m.newTemplate.CategoryColumn)
	}
}

func (m model) handleTemplateBackspaceActivation() (tea.Model, tea.Cmd) {
	switch m.createField {
	case templateName:
		return m.enterTemplateNameEditingWithBackspace()
	case templatePostDate:
		return m.enterTemplatePostDateEditingWithBackspace()
	case templateAmount:
		return m.enterTemplateAmountEditingWithBackspace()
	case templateDesc:
		return m.enterTemplateDescEditingWithBackspace()
	case templateCategory:
		return m.enterTemplateCategoryEditingWithBackspace()
	}
	return m, nil
}

func (m model) handleTemplateFieldActivation() (tea.Model, tea.Cmd) {
	switch m.createField {
	case templateName:
		return m.enterTemplateNameEditing()
	case templatePostDate:
		return m.enterTemplatePostDateEditing()
	case templateAmount:
		return m.enterTemplateAmountEditing()
	case templateDesc:
		return m.enterTemplateDescEditing()
	case templateCategory:
		return m.enterTemplateCategoryEditing()
	case templateHeader:
		return m.enterTemplateHeaderMode()
	}
	return m, nil
}

// Template Name Editing

func (m model) enterTemplateNameEditing() (tea.Model, tea.Cmd) {
	m.isEditingTemplateName = true
	if m.editingTemplateNameStr == "" {
		m.editingTemplateNameStr = m.newTemplate.Name
	}
	return m, nil
}

func (m model) enterTemplateNameEditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isEditingTemplateName = true
	if m.editingTemplateNameStr == "" {
		m.editingTemplateNameStr = m.newTemplate.Name
	}
	if len(m.editingTemplateNameStr) > 0 {
		m.editingTemplateNameStr = m.editingTemplateNameStr[:len(m.editingTemplateNameStr)-1]
	}
	return m, nil
}

func (m model) handleTemplateNameInput(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "esc":
		if key == "enter" {
			m.newTemplate.Name = m.editingTemplateNameStr
		}
		m.validateTemplateField("name")
		m.isEditingTemplateName = false
	case "backspace":
		if len(m.editingTemplateNameStr) > 0 {
			m.editingTemplateNameStr = m.editingTemplateNameStr[:len(m.editingTemplateNameStr)-1]
		}
	default:
		if len(key) == 1 {
			m.editingTemplateNameStr += key
		}
	}
	return m, nil
}

// Template Column Editing (Post Date, Amount, Description, Category)

func (m model) enterTemplatePostDateEditing() (tea.Model, tea.Cmd) {
	m.isEditingTemplatePostDate = true
	if m.editingTemplatePostDateStr == "" {
		m.editingTemplatePostDateStr = strconv.Itoa(m.newTemplate.PostDateColumn)
	}
	return m, nil
}

func (m model) enterTemplatePostDateEditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isEditingTemplatePostDate = true
	if m.editingTemplatePostDateStr == "" {
		m.editingTemplatePostDateStr = strconv.Itoa(m.newTemplate.PostDateColumn)
	}
	if len(m.editingTemplatePostDateStr) > 0 {
		m.editingTemplatePostDateStr = m.editingTemplatePostDateStr[:len(m.editingTemplatePostDateStr)-1]
	}
	return m, nil
}

func (m model) handleTemplatePostDateInput(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "esc":
		if key == "enter" {
			if value, err := strconv.Atoi(m.editingTemplatePostDateStr); err == nil && value >= 0 {
				m.newTemplate.PostDateColumn = value
			}
		}
		m.validateTemplateField("postdate")
		m.isEditingTemplatePostDate = false
	case "backspace":
		if len(m.editingTemplatePostDateStr) > 0 {
			m.editingTemplatePostDateStr = m.editingTemplatePostDateStr[:len(m.editingTemplatePostDateStr)-1]
		}
	default:
		if len(key) == 1 && key >= "0" && key <= "9" {
			m.editingTemplatePostDateStr += key
		}
	}
	return m, nil
}

func (m model) enterTemplateAmountEditing() (tea.Model, tea.Cmd) {
	m.isEditingTemplateAmount = true
	if m.editingTemplateAmountStr == "" {
		m.editingTemplateAmountStr = strconv.Itoa(m.newTemplate.AmountColumn)
	}
	return m, nil
}

func (m model) enterTemplateAmountEditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isEditingTemplateAmount = true
	if m.editingTemplateAmountStr == "" {
		m.editingTemplateAmountStr = strconv.Itoa(m.newTemplate.AmountColumn)
	}
	if len(m.editingTemplateAmountStr) > 0 {
		m.editingTemplateAmountStr = m.editingTemplateAmountStr[:len(m.editingTemplateAmountStr)-1]
	}
	return m, nil
}

func (m model) handleTemplateAmountInput(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "esc":
		if key == "enter" {
			if value, err := strconv.Atoi(m.editingTemplateAmountStr); err == nil && value >= 0 {
				m.newTemplate.AmountColumn = value
			}
		}
		m.validateTemplateField("amount")
		m.isEditingTemplateAmount = false
	case "backspace":
		if len(m.editingTemplateAmountStr) > 0 {
			m.editingTemplateAmountStr = m.editingTemplateAmountStr[:len(m.editingTemplateAmountStr)-1]
		}
	default:
		if len(key) == 1 && key >= "0" && key <= "9" {
			m.editingTemplateAmountStr += key
		}
	}
	return m, nil
}

func (m model) enterTemplateDescEditing() (tea.Model, tea.Cmd) {
	m.isEditingTemplateDesc = true
	if m.editingTemplateDescStr == "" {
		m.editingTemplateDescStr = strconv.Itoa(m.newTemplate.DescColumn)
	}
	return m, nil
}

func (m model) enterTemplateDescEditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isEditingTemplateDesc = true
	if m.editingTemplateDescStr == "" {
		m.editingTemplateDescStr = strconv.Itoa(m.newTemplate.DescColumn)
	}
	if len(m.editingTemplateDescStr) > 0 {
		m.editingTemplateDescStr = m.editingTemplateDescStr[:len(m.editingTemplateDescStr)-1]
	}
	return m, nil
}

func (m model) handleTemplateDescInput(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "esc":
		if key == "enter" {
			if value, err := strconv.Atoi(m.editingTemplateDescStr); err == nil && value >= 0 {
				m.newTemplate.DescColumn = value
			}
		}
		m.validateTemplateField("description")
		m.isEditingTemplateDesc = false
	case "backspace":
		if len(m.editingTemplateDescStr) > 0 {
			m.editingTemplateDescStr = m.editingTemplateDescStr[:len(m.editingTemplateDescStr)-1]
		}
	default:
		if len(key) == 1 && key >= "0" && key <= "9" {
			m.editingTemplateDescStr += key
		}
	}
	return m, nil
}

func (m model) enterTemplateCategoryEditing() (tea.Model, tea.Cmd) {
	m.isEditingTemplateCategory = true
	if m.editingTemplateCategoryStr == "" && m.newTemplate.CategoryColumn != nil {
		m.editingTemplateCategoryStr = strconv.Itoa(*m.newTemplate.CategoryColumn)
	}
	return m, nil
}

func (m model) enterTemplateCategoryEditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isEditingTemplateCategory = true
	if m.editingTemplateCategoryStr == "" && m.newTemplate.CategoryColumn != nil {
		m.editingTemplateCategoryStr = strconv.Itoa(*m.newTemplate.CategoryColumn)
	}
	if len(m.editingTemplateCategoryStr) > 0 {
		m.editingTemplateCategoryStr = m.editingTemplateCategoryStr[:len(m.editingTemplateCategoryStr)-1]
	}
	return m, nil
}

func (m model) handleTemplateCategoryInput(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "esc":
		if key == "enter" {
			if m.editingTemplateCategoryStr == "" {
				m.newTemplate.CategoryColumn = nil
			} else if value, err := strconv.Atoi(m.editingTemplateCategoryStr); err == nil && value >= 0 {
				m.newTemplate.CategoryColumn = &value
			}
		}
		m.validateTemplateField("category")
		m.isEditingTemplateCategory = false
	case "backspace":
		if len(m.editingTemplateCategoryStr) > 0 {
			m.editingTemplateCategoryStr = m.editingTemplateCategoryStr[:len(m.editingTemplateCategoryStr)-1]
		}
	default:
		if len(key) == 1 && key >= "0" && key <= "9" {
			m.editingTemplateCategoryStr += key
		}
	}
	return m, nil
}

// Template Header Mode (Yes/No selection)

func (m model) enterTemplateHeaderMode() (tea.Model, tea.Cmd) {
	// Header is a simple boolean toggle, no editing mode needed
	m.newTemplate.HasHeader = !m.newTemplate.HasHeader
	return m, nil
}

// Template Validation

func (m *model) validateTemplateField(field string) {
	if m.templateFieldErrors == nil {
		m.templateFieldErrors = make(map[string]string)
	}

	// Clear previous error for this field
	delete(m.templateFieldErrors, field)

	// Validate the specific field
	if err := m.newTemplate.ValidateField(field); err != nil {
		m.templateFieldErrors[field] = err.Error()
	}

	// Update overall validation status
	m.templateValidationErrors = len(m.templateFieldErrors) > 0
	m.buildTemplateValidationNotification()
}
