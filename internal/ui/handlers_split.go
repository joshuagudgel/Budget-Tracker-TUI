package ui

import (
	"budget-tracker-tui/internal/types"
	"fmt"
	"math"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Split Transaction Mode Functions

// enterSplitMode initializes split mode with default values
func (m model) enterSplitMode() (tea.Model, tea.Cmd) {
	m.isSplitMode = true
	m.splitField = splitAmount1Field
	m.splitMessage = ""

	// Pre-populate with half amounts (requirement E)
	halfAmount := m.currTransaction.Amount / 2
	m.splitAmount1 = fmt.Sprintf("%.2f", halfAmount)
	m.splitAmount2 = fmt.Sprintf("%.2f", halfAmount)

	// Add default description values with part tags
	m.splitDesc1 = m.currTransaction.Description + " (part 1)"
	m.splitDesc2 = m.currTransaction.Description + " (part 2)"

	// Pre-populate categories with current transaction's category
	categoryDisplayName := m.store.Categories.GetCategoryDisplayName(m.currTransaction.CategoryId)
	m.splitCategory1 = categoryDisplayName
	m.splitCategory2 = categoryDisplayName

	return m, nil
}

// exitSplitMode clears all split mode state and returns to list view
func (m model) exitSplitMode() (tea.Model, tea.Cmd) {
	m.isSplitMode = false
	m.splitField = 0
	m.splitMessage = ""
	m.splitAmount1 = ""
	m.splitAmount2 = ""
	m.splitDesc1 = ""
	m.splitDesc2 = ""
	m.splitCategory1 = ""
	m.splitCategory2 = ""
	return m, nil
}

// handleSplitFieldNavigation moves between split fields with up/down navigation
func (m model) handleSplitFieldNavigation(direction int) (tea.Model, tea.Cmd) {
	if direction > 0 && m.splitField < splitCategory2Field {
		m.splitField++
	} else if direction < 0 && m.splitField > splitAmount1Field {
		m.splitField--
	}
	return m, nil
}

// handleSplitFieldEditing is the main handler for split field editing and navigation
func (m model) handleSplitFieldEditing(key string) (tea.Model, tea.Cmd) {
	// Handle active editing states FIRST (like main edit view)
	if m.isSplitEditingAmount1 || m.isSplitEditingAmount2 {
		return m.handleSplitAmountEditing(key)
	}
	if m.isSplitEditingDesc1 || m.isSplitEditingDesc2 {
		return m.handleSplitDescEditing(key)
	}
	if m.isSplitSelectingCategory1 || m.isSplitSelectingCategory2 {
		return m.handleSplitCategorySelection(key)
	}

	// Field navigation (when not editing)
	switch key {
	case "up":
		return m.handleSplitFieldNavigation(-1)
	case "down", "tab":
		return m.handleSplitFieldNavigation(1)
	case "enter":
		return m.handleSplitFieldActivation()
	case "backspace":
		// Enter editing mode for text fields with backspace removal
		return m.handleSplitBackspaceActivation()
	case "ctrl+s":
		return m.handleSaveSplit()
	case "esc":
		return m.exitSplitMode()
	}
	return m, nil
}

// handleSplitFieldActivation activates the current field for editing
func (m model) handleSplitFieldActivation() (tea.Model, tea.Cmd) {
	switch m.splitField {
	case splitAmount1Field:
		return m.enterSplitAmount1Editing()
	case splitAmount2Field:
		return m.enterSplitAmount2Editing()
	case splitDesc1Field:
		return m.enterSplitDesc1Editing()
	case splitDesc2Field:
		return m.enterSplitDesc2Editing()
	case splitCategory1Field:
		return m.enterSplitCategory1Selection()
	case splitCategory2Field:
		return m.enterSplitCategory2Selection()
	}
	return m, nil
}

// handleSplitBackspaceActivation activates field editing with immediate backspace for text fields
func (m model) handleSplitBackspaceActivation() (tea.Model, tea.Cmd) {
	switch m.splitField {
	case splitAmount1Field:
		return m.enterSplitAmount1EditingWithBackspace()
	case splitAmount2Field:
		return m.enterSplitAmount2EditingWithBackspace()
	case splitDesc1Field:
		return m.enterSplitDesc1EditingWithBackspace()
	case splitDesc2Field:
		return m.enterSplitDesc2EditingWithBackspace()
	case splitCategory1Field, splitCategory2Field:
		// For dropdown fields, backspace doesn't activate - only enter does
		return m, nil
	}
	return m, nil
}

// Description Editing Functions

// enterSplitDesc1Editing enters editing mode for first split description
func (m model) enterSplitDesc1Editing() (tea.Model, tea.Cmd) {
	m.isSplitEditingDesc1 = true
	m.splitEditingDesc1 = m.splitDesc1
	return m, nil
}

// enterSplitDesc2Editing enters editing mode for second split description
func (m model) enterSplitDesc2Editing() (tea.Model, tea.Cmd) {
	m.isSplitEditingDesc2 = true
	m.splitEditingDesc2 = m.splitDesc2
	return m, nil
}

// enterSplitDesc1EditingWithBackspace enters editing mode for first description with immediate backspace
func (m model) enterSplitDesc1EditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isSplitEditingDesc1 = true
	m.splitEditingDesc1 = m.splitDesc1
	// Apply backspace immediately
	if len(m.splitEditingDesc1) > 0 {
		m.splitEditingDesc1 = m.splitEditingDesc1[:len(m.splitEditingDesc1)-1]
	}
	return m, nil
}

// enterSplitDesc2EditingWithBackspace enters editing mode for second description with immediate backspace
func (m model) enterSplitDesc2EditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isSplitEditingDesc2 = true
	m.splitEditingDesc2 = m.splitDesc2
	// Apply backspace immediately
	if len(m.splitEditingDesc2) > 0 {
		m.splitEditingDesc2 = m.splitEditingDesc2[:len(m.splitEditingDesc2)-1]
	}
	return m, nil
}

// handleSplitDescEditing handles key input during description editing
func (m model) handleSplitDescEditing(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		return m.exitSplitDescEditing()
	case "backspace":
		return m.handleSplitDescBackspace()
	default:
		if len(key) == 1 {
			return m.handleSplitDescInput(key)
		}
	}
	return m, nil
}

// handleSplitDescInput adds character input to the active description field
func (m model) handleSplitDescInput(key string) (tea.Model, tea.Cmd) {
	if m.isSplitEditingDesc1 {
		m.splitEditingDesc1 += key
	} else if m.isSplitEditingDesc2 {
		m.splitEditingDesc2 += key
	}
	return m, nil
}

// handleSplitDescBackspace removes the last character from the active description field
func (m model) handleSplitDescBackspace() (tea.Model, tea.Cmd) {
	if m.isSplitEditingDesc1 && len(m.splitEditingDesc1) > 0 {
		m.splitEditingDesc1 = m.splitEditingDesc1[:len(m.splitEditingDesc1)-1]
	} else if m.isSplitEditingDesc2 && len(m.splitEditingDesc2) > 0 {
		m.splitEditingDesc2 = m.splitEditingDesc2[:len(m.splitEditingDesc2)-1]
	}
	return m, nil
}

// exitSplitDescEditing saves description changes and exits editing mode
func (m model) exitSplitDescEditing() (tea.Model, tea.Cmd) {
	if m.isSplitEditingDesc1 {
		m.splitDesc1 = m.splitEditingDesc1
		m.isSplitEditingDesc1 = false
		m.splitEditingDesc1 = ""
		// Validate split on description field commit
		m.validateSplitTransaction()
	}
	if m.isSplitEditingDesc2 {
		m.splitDesc2 = m.splitEditingDesc2
		m.isSplitEditingDesc2 = false
		m.splitEditingDesc2 = ""
		// Validate split on description field commit
		m.validateSplitTransaction()
	}
	return m, nil
}

// Amount Editing Functions

// enterSplitAmount1Editing enters editing mode for first split amount
func (m model) enterSplitAmount1Editing() (tea.Model, tea.Cmd) {
	m.isSplitEditingAmount1 = true
	m.splitEditingAmount1 = m.splitAmount1
	return m, nil
}

// enterSplitAmount2Editing enters editing mode for second split amount
func (m model) enterSplitAmount2Editing() (tea.Model, tea.Cmd) {
	m.isSplitEditingAmount2 = true
	m.splitEditingAmount2 = m.splitAmount2
	return m, nil
}

// enterSplitAmount1EditingWithBackspace enters editing mode for first amount with immediate backspace
func (m model) enterSplitAmount1EditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isSplitEditingAmount1 = true
	m.splitEditingAmount1 = m.splitAmount1
	// Apply backspace immediately
	if len(m.splitEditingAmount1) > 0 {
		m.splitEditingAmount1 = m.splitEditingAmount1[:len(m.splitEditingAmount1)-1]
	}
	return m, nil
}

// enterSplitAmount2EditingWithBackspace enters editing mode for second amount with immediate backspace
func (m model) enterSplitAmount2EditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isSplitEditingAmount2 = true
	m.splitEditingAmount2 = m.splitAmount2
	// Apply backspace immediately
	if len(m.splitEditingAmount2) > 0 {
		m.splitEditingAmount2 = m.splitEditingAmount2[:len(m.splitEditingAmount2)-1]
	}
	return m, nil
}

// handleSplitAmountEditing handles key input during amount editing
func (m model) handleSplitAmountEditing(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		return m.exitSplitAmountEditing()
	case "backspace":
		return m.handleSplitAmountBackspace()
	default:
		if len(key) == 1 {
			return m.handleSplitAmountInput(key)
		}
	}
	return m, nil
}

// handleSplitAmountInput validates and adds character input to the active amount field
func (m model) handleSplitAmountInput(key string) (tea.Model, tea.Cmd) {
	// Prevent invalid characters while typing
	if !m.isValidAmountChar(key) {
		return m, nil
	}

	var currentValue *string
	if m.isSplitEditingAmount1 {
		currentValue = &m.splitEditingAmount1
	} else if m.isSplitEditingAmount2 {
		currentValue = &m.splitEditingAmount2
	}

	if currentValue == nil {
		return m, nil
	}

	// Handle negative sign
	if key == "-" {
		if len(*currentValue) == 0 {
			*currentValue = "-"
		}
		return m, nil
	}

	// Handle decimal point (prevent multiple)
	if key == "." && strings.Contains(*currentValue, ".") {
		return m, nil
	}

	newStr := *currentValue + key

	// Validate decimal places (max 2)
	dotIndex := strings.LastIndex(newStr, ".")
	if dotIndex != -1 && len(newStr)-dotIndex-1 > 2 {
		return m, nil
	}

	// Validate it's a valid number format
	if _, err := strconv.ParseFloat(newStr, 64); err == nil || newStr == "." || newStr == "-." {
		*currentValue = newStr
	}

	return m, nil
}

// handleSplitAmountBackspace removes the last character from the active amount field
func (m model) handleSplitAmountBackspace() (tea.Model, tea.Cmd) {
	if m.isSplitEditingAmount1 && len(m.splitEditingAmount1) > 0 {
		m.splitEditingAmount1 = m.splitEditingAmount1[:len(m.splitEditingAmount1)-1]
	} else if m.isSplitEditingAmount2 && len(m.splitEditingAmount2) > 0 {
		m.splitEditingAmount2 = m.splitEditingAmount2[:len(m.splitEditingAmount2)-1]
	}
	return m, nil
}

// exitSplitAmountEditing saves amount changes and exits editing mode
func (m model) exitSplitAmountEditing() (tea.Model, tea.Cmd) {
	if m.isSplitEditingAmount1 {
		m.splitAmount1 = m.splitEditingAmount1
		m.isSplitEditingAmount1 = false
		m.splitEditingAmount1 = ""
		// Validate split on amount field commit
		m.validateSplitTransaction()
	}
	if m.isSplitEditingAmount2 {
		m.splitAmount2 = m.splitEditingAmount2
		m.isSplitEditingAmount2 = false
		m.splitEditingAmount2 = ""
		// Validate split on amount field commit
		m.validateSplitTransaction()
	}
	return m, nil
}

// Category Selection Functions

// enterSplitCategory1Selection enters category selection mode for first split
func (m model) enterSplitCategory1Selection() (tea.Model, tea.Cmd) {
	m.isSplitSelectingCategory1 = true
	m.splitCat1SelectIndex = 0
	categories, _ := m.store.Categories.GetCategories()

	// Find current category in list
	for i, cat := range categories {
		if cat.DisplayName == m.splitCategory1 {
			m.splitCat1SelectIndex = i
			break
		}
	}
	return m, nil
}

// enterSplitCategory2Selection enters category selection mode for second split
func (m model) enterSplitCategory2Selection() (tea.Model, tea.Cmd) {
	m.isSplitSelectingCategory2 = true
	m.splitCat2SelectIndex = 0
	categories, _ := m.store.Categories.GetCategories()

	// Find current category in list
	for i, cat := range categories {
		if cat.DisplayName == m.splitCategory2 {
			m.splitCat2SelectIndex = i
			break
		}
	}
	return m, nil
}

// handleSplitCategorySelection handles navigation and selection within category dropdowns
func (m model) handleSplitCategorySelection(key string) (tea.Model, tea.Cmd) {
	categories, _ := m.store.Categories.GetCategories()

	var currentIndex *int
	var isSelecting1 bool

	if m.isSplitSelectingCategory1 {
		currentIndex = &m.splitCat1SelectIndex
		isSelecting1 = true
	} else if m.isSplitSelectingCategory2 {
		currentIndex = &m.splitCat2SelectIndex
		isSelecting1 = false
	} else {
		return m, nil
	}

	switch key {
	case "up":
		if *currentIndex > 0 {
			(*currentIndex)--
		}
	case "down":
		if *currentIndex < len(categories)-1 {
			(*currentIndex)++
		}
	case "enter":
		if len(categories) > 0 && *currentIndex < len(categories) {
			selectedCategory := categories[*currentIndex]
			if isSelecting1 {
				m.splitCategory1 = selectedCategory.DisplayName
				m.isSplitSelectingCategory1 = false
			} else {
				m.splitCategory2 = selectedCategory.DisplayName
				m.isSplitSelectingCategory2 = false
			}
			// Validate split on category selection commit
			m.validateSplitTransaction()
		}
	case "esc":
		// Exit category selection without saving changes
		m.isSplitSelectingCategory1 = false
		m.isSplitSelectingCategory2 = false
	}
	return m, nil
}

// Split Transaction Saving

// handleSaveSplit validates and saves the split transaction
func (m model) handleSaveSplit() (tea.Model, tea.Cmd) {
	// First validate split transaction fields
	m.validateSplitTransaction()

	// Block save if validation errors exist
	if m.hasValidationErrors {
		return m, nil
	}

	// Parse amounts
	amount1, err1 := strconv.ParseFloat(m.splitAmount1, 64)
	amount2, err2 := strconv.ParseFloat(m.splitAmount2, 64)

	if err1 != nil || err2 != nil {
		m.splitMessage = "Error: Invalid amount format"
		return m, nil
	}

	// Validate amounts add up to original (with epsilon for floating point)
	total := amount1 + amount2
	if math.Abs(total-m.currTransaction.Amount) > 0.01 {
		m.splitMessage = fmt.Sprintf("Error: Split amounts (%.2f) don't match original (%.2f)",
			total, m.currTransaction.Amount)
		return m, nil
	}

	// Create split transactions
	// Find category IDs from display names
	category1Id := int64(0)
	if category1 := m.store.Categories.GetCategoryByDisplayName(m.splitCategory1); category1 != nil {
		category1Id = category1.Id
	}
	category2Id := int64(0)
	if category2 := m.store.Categories.GetCategoryByDisplayName(m.splitCategory2); category2 != nil {
		category2Id = category2.Id
	}

	split1 := types.Transaction{
		Amount:          amount1,
		Description:     m.splitDesc1,
		Date:            m.currTransaction.Date,
		CategoryId:      category1Id,
		TransactionType: m.currTransaction.TransactionType,
	}

	split2 := types.Transaction{
		Amount:          amount2,
		Description:     m.splitDesc2,
		Date:            m.currTransaction.Date,
		CategoryId:      category2Id,
		TransactionType: m.currTransaction.TransactionType,
	}

	// Validate both split transactions before saving
	categories, _ := m.store.Categories.GetCategories()

	result1 := m.validator.ValidateTransaction(&split1, categories)
	result2 := m.validator.ValidateTransaction(&split2, categories)

	if !result1.IsValid || !result2.IsValid {
		m.splitMessage = "Error: Split transactions contain validation errors"
		return m, nil
	}

	// Save split using store method
	err := m.store.Transactions.SplitTransaction(m.currTransaction.Id, []types.Transaction{split1, split2})
	if err != nil {
		m.splitMessage = fmt.Sprintf("Error saving split: %v", err)
		return m, nil
	}

	// Refresh transactions and exit
	m.transactions, _ = m.store.Transactions.GetTransactions()
	m.sortTransactionsByDate()
	m.state = listView
	return m.exitSplitMode()
}

// isValidAmountChar validates characters allowed in amount fields
func (m model) isValidAmountChar(key string) bool {
	return (key >= "0" && key <= "9") || key == "." || key == "-"
}
