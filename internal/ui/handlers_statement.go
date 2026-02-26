package ui

import (
	"budget-tracker-tui/internal/types"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// Bank statement view handlers
func (m model) handleBankStatementView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "t":
		m.state = csvTemplateView
		m.templateIndex = 0
	case "f", "i":
		// Initialize directory if needed
		if m.currentDir == "" {
			if homeDir, err := os.UserHomeDir(); err == nil {
				m.currentDir = homeDir
			}
		}

		result := m.store.LoadDirectoryEntriesWithFallback(m.currentDir)
		if !result.Success {
			m.statementMessage = result.Message
		} else {
			m.dirEntries = result.Entries
			m.currentDir = result.CurrentPath // Use returned path in case of fallback
			m.state = filePickerView
			m.fileIndex = 0
		}
	case "h":
		m.state = bankStatementListView
		m.bankStatementListIndex = 0
		m.bankStatementListMessage = ""
		m.isInBankStatementActions = false
	case "esc":
		m.state = menuView
	}
	return m, nil
}

func (m model) handleStatementOverlapView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y":
		// Use current template stored from file selection
		result := m.store.ImportCSVWithOverride(m.selectedTemplate)
		if result.Success {
			m.transactions, _ = m.store.GetTransactions()
		}
		m.statementMessage = result.Message
		m.state = bankStatementView
	case "n", "esc":
		m.state = bankStatementView
	}
	return m, nil
}

// handleUndoConfirmView handles the undo confirmation view
func (m model) handleUndoConfirmView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y":
		m.executeUndo()
	case "n", "esc":
		m.state = bankStatementListView
	}
	return m, nil
}

// Enhanced Bank Statement Management Handlers

// handleBankStatementListView handles the new bank statement list management view
func (m model) handleBankStatementListView(key string) (tea.Model, tea.Cmd) {
	statements := m.store.GetStatementHistory()

	switch key {
	case "up":
		if m.bankStatementListIndex > 0 {
			m.bankStatementListIndex--
		}
	case "down":
		if len(statements) > 0 && m.bankStatementListIndex < len(statements)-1 {
			m.bankStatementListIndex++
		}
	case "enter":
		// Enter action mode for the selected statement
		if len(statements) > 0 && m.bankStatementListIndex >= 0 && m.bankStatementListIndex < len(statements) {
			m.selectedBankStatementId = statements[m.bankStatementListIndex].Id
			m.isInBankStatementActions = true
			m.bankStatementActionIndex = 0
			m.state = bankStatementManageView
		}
	case "i":
		// Quick import shortcut
		m.state = bankStatementView
		m.statementMessage = ""
	case "u":
		// Quick undo shortcut
		if len(statements) > 0 && m.bankStatementListIndex >= 0 && m.bankStatementListIndex < len(statements) {
			stmt := statements[m.bankStatementListIndex]
			if m.store.CanUndoImport(stmt.Id) {
				m.initUndoConfirmationById(stmt.Id)
			} else {
				m.bankStatementListMessage = "Cannot undo this import - invalid status or already undone"
			}
		}
	case "d":
		// Show details shortcut
		if len(statements) > 0 && m.bankStatementListIndex >= 0 && m.bankStatementListIndex < len(statements) {
			stmt := statements[m.bankStatementListIndex]
			m.bankStatementListMessage = m.store.GetStatementSummary(stmt)
			if stmt.ErrorLog != "" {
				m.bankStatementListMessage += " | Error: " + stmt.ErrorLog
			}
		}
	case "esc":
		m.state = menuView
	}
	return m, nil
}

// handleBankStatementManageView handles individual statement action selection
func (m model) handleBankStatementManageView(key string) (tea.Model, tea.Cmd) {
	stmt, err := m.store.GetStatementById(m.selectedBankStatementId)
	if err != nil {
		m.bankStatementListMessage = "Error: " + err.Error()
		m.state = bankStatementListView
		return m, nil
	}

	actions := m.getAvailableActions(*stmt)

	switch key {
	case "up":
		if m.bankStatementActionIndex > 0 {
			m.bankStatementActionIndex--
		}
	case "down":
		if m.bankStatementActionIndex < len(actions)-1 {
			m.bankStatementActionIndex++
		}
	case "enter":
		return m.executeStatementAction(*stmt, actions[m.bankStatementActionIndex])
	case "esc":
		m.isInBankStatementActions = false
		m.state = bankStatementListView
	}
	return m, nil
}

// Helper methods for bank statement management

// getAvailableActions returns available actions for a statement
func (m model) getAvailableActions(stmt types.BankStatement) []string {
	actions := []string{"View Details"}

	// Add manage transactions option for completed statements
	if stmt.Status == "completed" {
		actions = append(actions, "Manage Transactions")
	}

	if m.store.CanUndoImport(stmt.Id) {
		actions = append(actions, "Undo Import")
	}

	// Add reimport option for failed statements
	if stmt.Status == "failed" {
		actions = append(actions, "Retry Import")
	}

	return actions
}

// executeStatementAction executes the selected action on a statement
func (m model) executeStatementAction(stmt types.BankStatement, action string) (tea.Model, tea.Cmd) {
	switch action {
	case "View Details":
		m.bankStatementListMessage = m.store.GetStatementSummary(stmt)
		if stmt.ErrorLog != "" {
			m.bankStatementListMessage += " | Error: " + stmt.ErrorLog
		}
		m.state = bankStatementListView
	case "Manage Transactions":
		// Load filtered transactions for this statement
		filteredTransactions, err := m.store.GetTransactionsByStatement(stmt.Id)
		if err != nil {
			m.bankStatementListMessage = "Error loading transactions: " + err.Error()
			m.state = bankStatementListView
			return m, nil
		}
		m.filteredTransactions = filteredTransactions
		m.currentStatementId = stmt.Id
		m.filteredListIndex = 0
		m.statementTxMessage = ""
		m.state = statementTransactionListView
	case "Undo Import":
		m.initUndoConfirmationById(stmt.Id)
	case "Retry Import":
		// TODO: Implement retry import functionality
		m.bankStatementListMessage = "Retry import not yet implemented"
		m.state = bankStatementListView
	}
	return m, nil
}
