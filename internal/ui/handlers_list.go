package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// List view handler
func (m model) handleListView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up":
		if len(m.transactions) > 0 && m.listIndex > 0 {
			m.listIndex--
		}
	case "down":
		if len(m.transactions) > 0 && m.listIndex < len(m.transactions)-1 {
			m.listIndex++
		}
	case "e":
		if m.isMultiSelectMode {
			// Edit all selected records in bulk edit mode
			if len(m.selectedTxIds) > 0 {
				m.bulkEditField = bulkEditAmount
				m.resetBulkEditValues()
				m.previousState = listView // Track where to return
				m.state = bulkEditView
			}
			return m, nil
		}
		// Single edit mode (existing logic)
		if len(m.transactions) > 0 {
			m.currTransaction = m.transactions[m.listIndex]
			m.editField = editAmount
			m.editAmountStr = ""
			m.previousState = listView // Track where to return
			m.state = editView
		}
	case "m":
		return m.handleMultiSelectToggle()
	case "enter":
		if m.isMultiSelectMode && len(m.transactions) > 0 {
			// Toggle selection for current transaction
			return m.handleToggleSelection()
		}
	case "d":
		if !m.isMultiSelectMode && !m.pendingDeleteTx {
			// Setup deletion confirmation
			tx := m.transactions[m.listIndex]
			m.pendingDeleteTx = true
			m.deleteTransactionId = tx.Id
			m.deleteTransactionDesc = tx.Description
			m.deleteTransactionAmount = fmt.Sprintf("%.2f", tx.Amount)
		}
	case "y":
		if m.pendingDeleteTx {
			// Confirm deletion
			m.store.Transactions.DeleteTransaction(m.deleteTransactionId)
			m.transactions, _ = m.store.Transactions.GetTransactions()
			m.sortTransactionsByDate()
			// Bounds checking for list index
			if m.listIndex >= len(m.transactions) && len(m.transactions) > 0 {
				m.listIndex = len(m.transactions) - 1
			}
			if len(m.transactions) == 0 {
				m.listIndex = 0
			}
			// Clear confirmation state
			m.pendingDeleteTx = false
			m.deleteTransactionId = 0
			m.deleteTransactionDesc = ""
			m.deleteTransactionAmount = ""
		}
	case "n":
		if m.pendingDeleteTx {
			// Cancel deletion
			m.pendingDeleteTx = false
			m.deleteTransactionId = 0
			m.deleteTransactionDesc = ""
			m.deleteTransactionAmount = ""
		}
	case "esc":
		if m.pendingDeleteTx {
			// Cancel deletion
			m.pendingDeleteTx = false
			m.deleteTransactionId = 0
			m.deleteTransactionDesc = ""
			m.deleteTransactionAmount = ""
			return m, nil
		}
		if m.isMultiSelectMode {
			return m.exitMultiSelectMode()
		}
		m.state = menuView
	}
	return m, nil
}
