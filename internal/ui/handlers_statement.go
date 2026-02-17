package ui

import (
	"fmt"
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
		m.state = statementHistoryView
		m.statementIndex = 0
	case "esc":
		m.state = menuView
	}
	return m, nil
}

func (m model) handleStatementHistoryView(key string) (tea.Model, tea.Cmd) {
	statements := m.store.GetStatementHistory()

	switch key {
	case "up":
		if m.statementIndex > 0 {
			m.statementIndex--
		}
	case "down":
		if len(statements) > 0 && m.statementIndex < len(statements)-1 {
			m.statementIndex++
		}
	case "enter":
		// Use store method with proper error handling
		if stmt, err := m.store.GetStatementByIndex(m.statementIndex); err == nil {
			m.statementMessage = fmt.Sprintf("Statement Details: %s | Period: %s to %s | %d transactions | Template: %s | Status: %s | Import Date: %s",
				stmt.Filename, stmt.PeriodStart, stmt.PeriodEnd, stmt.TxCount, stmt.TemplateUsed, stmt.Status, stmt.ImportDate)
		} else {
			m.statementMessage = "Error: " + err.Error()
		}
		m.state = bankStatementView
	case "esc":
		m.state = bankStatementView
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
