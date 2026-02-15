package main

import (
	"log"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

// Transaction edit view handler
func (m model) handleEditView(key string) (tea.Model, tea.Cmd) {
	// Handle active editing states
	if m.isEditingAmount {
		return m.handleAmountEditing(key)
	}
	if m.isEditingDescription {
		return m.handleDescriptionEditing(key)
	}
	if m.isEditingDate {
		return m.handleDateEditing(key)
	}
	if m.isSelectingCategory {
		return m.handleCategorySelection(key)
	}
	if m.isSelectingType {
		return m.handleTypeSelection(key)
	}

	if m.isSplitMode {
		return m.handleSplitFieldEditing(key)
	}

	switch key {
	case "esc":
		if m.isSplitMode {
			return m.exitSplitMode()
		}
		m.state = listView
	case "enter":
		return m.handleFieldActivation()
	case "backspace":
		// Enter editing mode for text fields only
		return m.handleBackspaceActivation()
	case "s":
		if !m.isSplitMode {
			return m.enterSplitMode()
		} else {
			return m.exitSplitMode()
		}
	case "ctrl+s": // Save entire transaction
		return m.handleSaveTransaction()
	case "down", "tab":
		if m.isSplitMode {
			return m.handleSplitFieldNavigation(1)
		}
		return m.handleFieldNavigation(1)
	case "up":
		if m.isSplitMode {
			return m.handleSplitFieldNavigation(-1)
		}
		return m.handleFieldNavigation(-1)
	}
	return m, nil
}

// Save transaction
func (m model) handleSaveTransaction() (tea.Model, tea.Cmd) {
	// Validate and save amount from edit string with proper formatting
	if m.editField == editAmount && m.editAmountStr != "" {
		if amount, err := strconv.ParseFloat(m.editAmountStr, 64); err == nil {
			m.currTransaction.Amount = amount
		}
	}
	m.editAmountStr = ""
	err := m.store.SaveTransaction(m.currTransaction)
	if err != nil {
		log.Printf("Error saving transaction: %v", err)
	} else {
		m.transactions, _ = m.store.GetTransactions()
	}
	m.state = listView
	return m, nil
}
