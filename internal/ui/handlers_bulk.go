package ui

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Multi-select and bulk edit handlers

// handleMultiSelectToggle toggles multi-select mode for bulk operations
func (m model) handleMultiSelectToggle() (tea.Model, tea.Cmd) {
	if file, err := tea.LogToFile("debug.log", "debug"); err == nil {
		defer file.Close()
		log.Printf("handleMultiSelectToggle called: current mode=%v", m.isMultiSelectMode)
	}

	if !m.isMultiSelectMode {
		// Enter multi-select mode
		m.isMultiSelectMode = true
		m.selectedTxIds = make(map[int64]bool)
		if file, err := tea.LogToFile("debug.log", "debug"); err == nil {
			defer file.Close()
			log.Printf("Entered multi-select mode")
		}
	} else {
		// Exit multi-select mode
		if file, err := tea.LogToFile("debug.log", "debug"); err == nil {
			defer file.Close()
			log.Printf("Exiting multi-select mode")
		}
		return m.exitMultiSelectMode()
	}
	return m, nil
}

// exitMultiSelectMode clears multi-select state and returns to normal mode
func (m model) exitMultiSelectMode() (tea.Model, tea.Cmd) {
	m.isMultiSelectMode = false
	m.selectedTxIds = make(map[int64]bool)
	return m, nil
}

// handleToggleSelection toggles selection state for current transaction
func (m model) handleToggleSelection() (tea.Model, tea.Cmd) {
	if len(m.transactions) > 0 && m.listIndex < len(m.transactions) {
		txId := m.transactions[m.listIndex].Id
		if m.selectedTxIds[txId] {
			delete(m.selectedTxIds, txId)
		} else {
			m.selectedTxIds[txId] = true
		}
	}
	return m, nil
}

// handleBulkEditView handles key events in bulk edit view
func (m model) handleBulkEditView(key string) (tea.Model, tea.Cmd) {
	// Handle active editing states
	if m.isBulkEditingAmount || m.isBulkEditingDescription || m.isBulkEditingDate {
		return m.handleBulkTextEditing(key)
	}
	if m.isBulkSelectingCategory || m.isBulkSelectingType {
		return m.handleBulkDropdownSelection(key)
	}

	switch key {
	case "esc":
		m.state = listView
	case "down", "tab":
		if m.bulkEditField < bulkEditType {
			m.bulkEditField++
		}
	case "up":
		if m.bulkEditField > bulkEditAmount {
			m.bulkEditField--
		}
	case "enter", "backspace":
		return m.handleBulkFieldActivation(key)
	case "ctrl+s":
		return m.handleSaveBulkEdit()
	}
	return m, nil
}

// handleBulkFieldActivation activates field editing based on current field
func (m model) handleBulkFieldActivation(key string) (tea.Model, tea.Cmd) {
	switch m.bulkEditField {
	case bulkEditAmount:
		return m.enterBulkAmountEditing(key == "backspace")
	case bulkEditDescription:
		return m.enterBulkDescriptionEditing(key == "backspace")
	case bulkEditDate:
		return m.enterBulkDateEditing(key == "backspace")
	case bulkEditCategory:
		m.isBulkSelectingCategory = true
		m.bulkCategorySelectIndex = 0
	case bulkEditType:
		m.isBulkSelectingType = true
		m.bulkTypeSelectIndex = 0
	}
	return m, nil
}

// handleBulkCategorySelection handles category selection dropdown navigation
func (m model) handleBulkCategorySelection(key string) (tea.Model, tea.Cmd) {
	categories, _ := m.store.GetCategories()

	switch key {
	case "esc":
		m.isBulkSelectingCategory = false
	case "up":
		if m.bulkCategorySelectIndex > 0 {
			m.bulkCategorySelectIndex--
		}
	case "down":
		if m.bulkCategorySelectIndex < len(categories)-1 {
			m.bulkCategorySelectIndex++
		}
	case "enter":
		if len(categories) > 0 {
			selectedCategory := categories[m.bulkCategorySelectIndex]
			m.bulkCategoryValue = selectedCategory.Name
			m.bulkCategoryIsPlaceholder = false // Clear placeholder state
			// Validate bulk edit data on category selection
			m.validateBulkEditData()
		}
		m.isBulkSelectingCategory = false
	}
	return m, nil
}

// handleBulkTypeSelection handles transaction type selection dropdown navigation
func (m model) handleBulkTypeSelection(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.isBulkSelectingType = false
	case "up":
		if m.bulkTypeSelectIndex > 0 {
			m.bulkTypeSelectIndex--
		}
	case "down":
		if m.bulkTypeSelectIndex < len(m.availableTypes)-1 {
			m.bulkTypeSelectIndex++
		}
	case "enter":
		selectedType := m.availableTypes[m.bulkTypeSelectIndex]
		m.bulkTypeValue = selectedType
		m.bulkTypeIsPlaceholder = false // Clear placeholder state
		// Note: Transaction type doesn't require validation as it's from predefined list
		m.isBulkSelectingType = false
	}
	return m, nil
}

// handleSaveBulkEdit saves bulk edit changes to all selected transactions
func (m model) handleSaveBulkEdit() (tea.Model, tea.Cmd) {
	// Validate bulk edit data first
	m.validateBulkEditData()

	// Block save if validation errors exist
	if m.hasValidationErrors {
		return m, nil
	}

	// Validate individual transactions that will be modified
	var brokenTransactions []int64
	for i := range m.transactions {
		if m.selectedTxIds[m.transactions[i].Id] {
			tempTx := m.transactions[i] // Copy current transaction

			// Apply changes to temp transaction for validation
			if !m.bulkAmountIsPlaceholder && strings.TrimSpace(m.bulkAmountValue) != "" {
				if amount, err := strconv.ParseFloat(m.bulkAmountValue, 64); err == nil {
					tempTx.Amount = amount
				}
			}
			if !m.bulkDescriptionIsPlaceholder && strings.TrimSpace(m.bulkDescriptionValue) != "" {
				tempTx.Description = m.bulkDescriptionValue
			}
			if !m.bulkDateIsPlaceholder && strings.TrimSpace(m.bulkDateValue) != "" {
				tempTx.Date = m.bulkDateValue
			}
			if !m.bulkCategoryIsPlaceholder && strings.TrimSpace(m.bulkCategoryValue) != "" {
				tempTx.Category = m.bulkCategoryValue
			}
			if !m.bulkTypeIsPlaceholder && strings.TrimSpace(m.bulkTypeValue) != "" {
				tempTx.TransactionType = m.bulkTypeValue
			}

			// Validate the modified transaction
			categories, _ := m.store.GetCategories()
			categoryNames := make([]string, len(categories))
			for j, category := range categories {
				categoryNames[j] = category.Name
			}

			result := m.validator.ValidateTransaction(&tempTx, categoryNames)
			if !result.IsValid {
				brokenTransactions = append(brokenTransactions, tempTx.Id)
			}
		}
	}

	// If any transactions would be invalid, prevent save and show error
	if len(brokenTransactions) > 0 {
		m.validationNotification = fmt.Sprintf("⚠ %d transactions would become invalid", len(brokenTransactions))
		m.hasValidationErrors = true
		return m, nil
	}

	// All validations passed, proceed with save
	for i := range m.transactions {
		if m.selectedTxIds[m.transactions[i].Id] {
			// Apply amount if modified
			if !m.bulkAmountIsPlaceholder && strings.TrimSpace(m.bulkAmountValue) != "" {
				if amount, err := strconv.ParseFloat(m.bulkAmountValue, 64); err == nil {
					m.transactions[i].Amount = amount
				}
			}

			// Apply description if modified
			if !m.bulkDescriptionIsPlaceholder && strings.TrimSpace(m.bulkDescriptionValue) != "" {
				m.transactions[i].Description = m.bulkDescriptionValue
			}

			// Apply date if modified
			if !m.bulkDateIsPlaceholder && strings.TrimSpace(m.bulkDateValue) != "" {
				m.transactions[i].Date = m.bulkDateValue
			}

			// Apply category if modified
			if !m.bulkCategoryIsPlaceholder && strings.TrimSpace(m.bulkCategoryValue) != "" {
				m.transactions[i].Category = m.bulkCategoryValue
			}

			// Apply type if modified
			if !m.bulkTypeIsPlaceholder && strings.TrimSpace(m.bulkTypeValue) != "" {
				m.transactions[i].TransactionType = m.bulkTypeValue
			}

			m.store.SaveTransaction(m.transactions[i])
		}
	}

	m.transactions, _ = m.store.GetTransactions()
	m.state = listView
	return m.exitMultiSelectMode()
}

// resetBulkEditValues resets all bulk edit values to placeholders
func (m *model) resetBulkEditValues() {
	m.bulkAmountValue = ""
	m.bulkDescriptionValue = ""
	m.bulkDateValue = ""
	m.bulkCategoryValue = ""
	m.bulkTypeValue = ""

	m.bulkAmountIsPlaceholder = true
	m.bulkDescriptionIsPlaceholder = true
	m.bulkDateIsPlaceholder = true
	m.bulkCategoryIsPlaceholder = true
	m.bulkTypeIsPlaceholder = true
}

// handleBulkTextEditing handles text input during bulk editing
func (m model) handleBulkTextEditing(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		// Exit editing mode and save current value
		return m.exitBulkTextEditing()
	case "esc":
		// Cancel editing and reset to placeholder state
		return m.cancelBulkTextEditing()
	case "backspace":
		return m.handleBulkTextBackspace()
	default:
		if len(key) == 1 {
			return m.handleBulkTextInput(key)
		}
	}
	return m, nil
}

// exitBulkTextEditing exits all bulk text editing modes
func (m model) exitBulkTextEditing() (tea.Model, tea.Cmd) {
	// Check if fields are empty and reset to placeholder state if needed
	switch {
	case m.isBulkEditingAmount:
		if strings.TrimSpace(m.bulkAmountValue) == "" {
			m.bulkAmountValue = ""
			m.bulkAmountIsPlaceholder = true
		}
		m.isBulkEditingAmount = false
		// Validate bulk edit data on field commit
		m.validateBulkEditData()
	case m.isBulkEditingDescription:
		if strings.TrimSpace(m.bulkDescriptionValue) == "" {
			m.bulkDescriptionValue = ""
			m.bulkDescriptionIsPlaceholder = true
		}
		m.isBulkEditingDescription = false
		// Validate bulk edit data on field commit
		m.validateBulkEditData()
	case m.isBulkEditingDate:
		if strings.TrimSpace(m.bulkDateValue) == "" {
			m.bulkDateValue = ""
			m.bulkDateIsPlaceholder = true
		}
		m.isBulkEditingDate = false
		// Validate bulk edit data on field commit
		m.validateBulkEditData()
	default:
		// Exit all text editing modes (fallback)
		m.isBulkEditingAmount = false
		m.isBulkEditingDescription = false
		m.isBulkEditingDate = false
	}
	return m, nil
}

// cancelBulkTextEditing cancels text editing and resets current field to placeholder state
func (m model) cancelBulkTextEditing() (tea.Model, tea.Cmd) {
	switch {
	case m.isBulkEditingAmount:
		m.bulkAmountValue = ""
		m.bulkAmountIsPlaceholder = true
		m.isBulkEditingAmount = false
	case m.isBulkEditingDescription:
		m.bulkDescriptionValue = ""
		m.bulkDescriptionIsPlaceholder = true
		m.isBulkEditingDescription = false
	case m.isBulkEditingDate:
		m.bulkDateValue = ""
		m.bulkDateIsPlaceholder = true
		m.isBulkEditingDate = false
	}
	return m, nil
}

// handleBulkTextBackspace handles backspace in bulk text editing
func (m model) handleBulkTextBackspace() (tea.Model, tea.Cmd) {
	switch {
	case m.isBulkEditingAmount:
		if len(m.bulkAmountValue) > 0 {
			m.bulkAmountValue = m.bulkAmountValue[:len(m.bulkAmountValue)-1]
			m.bulkAmountIsPlaceholder = len(m.bulkAmountValue) == 0
		}
	case m.isBulkEditingDescription:
		if len(m.bulkDescriptionValue) > 0 {
			m.bulkDescriptionValue = m.bulkDescriptionValue[:len(m.bulkDescriptionValue)-1]
			m.bulkDescriptionIsPlaceholder = len(m.bulkDescriptionValue) == 0
		}
	case m.isBulkEditingDate:
		if len(m.bulkDateValue) > 0 {
			m.bulkDateValue = m.bulkDateValue[:len(m.bulkDateValue)-1]
			m.bulkDateIsPlaceholder = len(m.bulkDateValue) == 0
		}
	}
	return m, nil
}

// handleBulkTextInput handles text input for bulk editing fields
func (m model) handleBulkTextInput(key string) (tea.Model, tea.Cmd) {
	switch {
	case m.isBulkEditingAmount:
		return m.handleBulkAmountInput(key)
	case m.isBulkEditingDescription:
		m.bulkDescriptionValue += key
		m.bulkDescriptionIsPlaceholder = false
	case m.isBulkEditingDate:
		m.bulkDateValue += key
		m.bulkDateIsPlaceholder = false
	}
	return m, nil
}

// handleBulkAmountInput handles amount input with validation
func (m model) handleBulkAmountInput(key string) (tea.Model, tea.Cmd) {
	// Handle negative sign
	if key == "-" {
		if len(m.bulkAmountValue) == 0 {
			m.bulkAmountValue = "-"
			m.bulkAmountIsPlaceholder = false
		}
		return m, nil
	}

	// Only allow digits and decimal point
	if (key >= "0" && key <= "9") || key == "." {
		// Don't allow multiple decimal points
		if key == "." && strings.Contains(m.bulkAmountValue, ".") {
			return m, nil
		}

		newStr := m.bulkAmountValue + key

		// Validate decimal places (max 2)
		dotIndex := strings.LastIndex(newStr, ".")
		if dotIndex != -1 && len(newStr)-dotIndex-1 > 2 {
			return m, nil
		}

		// Validate it's a valid number format
		if _, err := strconv.ParseFloat(newStr, 64); err == nil || newStr == "." || newStr == "-." {
			m.bulkAmountValue = newStr
			m.bulkAmountIsPlaceholder = false
		}
	}
	return m, nil
}

// enterBulkAmountEditing enters bulk amount editing mode
func (m model) enterBulkAmountEditing(withBackspace bool) (tea.Model, tea.Cmd) {
	m.isBulkEditingAmount = true
	m.bulkAmountIsPlaceholder = false

	// Clear placeholder and start fresh
	if m.bulkAmountIsPlaceholder {
		m.bulkAmountValue = ""
	}

	// Apply backspace immediately if activated with backspace
	if withBackspace && len(m.bulkAmountValue) > 0 {
		m.bulkAmountValue = m.bulkAmountValue[:len(m.bulkAmountValue)-1]
	}

	return m, nil
}

// enterBulkDescriptionEditing enters bulk description editing mode
func (m model) enterBulkDescriptionEditing(withBackspace bool) (tea.Model, tea.Cmd) {
	m.isBulkEditingDescription = true
	m.bulkDescriptionIsPlaceholder = false

	// Clear placeholder and start fresh
	if m.bulkDescriptionIsPlaceholder {
		m.bulkDescriptionValue = ""
	}

	// Apply backspace immediately if activated with backspace
	if withBackspace && len(m.bulkDescriptionValue) > 0 {
		m.bulkDescriptionValue = m.bulkDescriptionValue[:len(m.bulkDescriptionValue)-1]
	}

	return m, nil
}

// enterBulkDateEditing enters bulk date editing mode
func (m model) enterBulkDateEditing(withBackspace bool) (tea.Model, tea.Cmd) {
	m.isBulkEditingDate = true
	m.bulkDateIsPlaceholder = false

	// Clear placeholder and start fresh
	if m.bulkDateIsPlaceholder {
		m.bulkDateValue = ""
	}

	// Apply backspace immediately if activated with backspace
	if withBackspace && len(m.bulkDateValue) > 0 {
		m.bulkDateValue = m.bulkDateValue[:len(m.bulkDateValue)-1]
	}

	return m, nil
}

// handleBulkDropdownSelection handles dropdown selection for bulk fields
func (m model) handleBulkDropdownSelection(key string) (tea.Model, tea.Cmd) {
	if m.isBulkSelectingCategory {
		return m.handleBulkCategorySelection(key)
	}
	if m.isBulkSelectingType {
		return m.handleBulkTypeSelection(key)
	}
	return m, nil
}
