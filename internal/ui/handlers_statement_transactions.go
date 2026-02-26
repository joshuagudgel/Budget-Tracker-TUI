package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Statement transaction list handler
func (m model) handleStatementTransactionListView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up":
		if len(m.filteredTransactions) > 0 && m.filteredListIndex > 0 {
			m.filteredListIndex--
		}
	case "down":
		if len(m.filteredTransactions) > 0 && m.filteredListIndex < len(m.filteredTransactions)-1 {
			m.filteredListIndex++
		}
	case "e":
		if m.isMultiSelectMode {
			// Edit all selected records in bulk edit mode
			if len(m.selectedTxIds) > 0 {
				m.bulkEditField = bulkEditAmount
				m.resetBulkEditValues()
				m.previousState = statementTransactionListView // Track where to return
				m.state = bulkEditView
			}
			return m, nil
		}
		// Single edit mode
		if len(m.filteredTransactions) > 0 {
			m.currTransaction = m.filteredTransactions[m.filteredListIndex]
			m.editField = editAmount
			m.editAmountStr = ""
			m.previousState = statementTransactionListView // Track where to return
			m.state = editView
		}
	case "m":
		return m.handleMultiSelectToggle()
	case "enter":
		if m.isMultiSelectMode && len(m.filteredTransactions) > 0 {
			// Toggle selection for current transaction in filtered view
			return m.handleFilteredToggleSelection()
		}
	case "d":
		if !m.isMultiSelectMode && len(m.filteredTransactions) > 0 {
			// Delete transaction
			m.store.DeleteTransaction(m.filteredTransactions[m.filteredListIndex].Id)

			// Reload filtered transactions
			filteredTransactions, err := m.store.GetTransactionsByStatement(m.currentStatementId)
			if err != nil {
				m.statementTxMessage = "Error reloading transactions: " + err.Error()
			} else {
				m.filteredTransactions = filteredTransactions
			}

			// Bounds checking for list index
			if m.filteredListIndex >= len(m.filteredTransactions) && len(m.filteredTransactions) > 0 {
				m.filteredListIndex = len(m.filteredTransactions) - 1
			}
			if len(m.filteredTransactions) == 0 {
				m.filteredListIndex = 0
			}
		}
	case "esc":
		if m.isMultiSelectMode {
			return m.exitMultiSelectMode()
		}
		// Return to bank statement manage view instead of menu
		m.state = bankStatementManageView
		m.statementTxMessage = ""
	}
	return m, nil
}

// Helper method to handle multi-select toggle in filtered view
func (m model) handleFilteredToggleSelection() (model, tea.Cmd) {
	if len(m.filteredTransactions) > 0 {
		txId := m.filteredTransactions[m.filteredListIndex].Id
		if m.selectedTxIds[txId] {
			delete(m.selectedTxIds, txId)
		} else {
			m.selectedTxIds[txId] = true
		}
	}
	return m, nil
}
