package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Bank statement view handlers
func (m model) handleBankStatementView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "t":
		// Select CSV template
		m.state = csvTemplateView
		m.templateIndex = 0
	case "f":
		// Choose CSV file
		if err := m.loadDirectoryEntries(); err != nil {
			m.statementMessage = fmt.Sprintf("Error loading directory: %v", err)
		} else {
			m.state = filePickerView
			m.fileIndex = 0
		}
	case "i":
		// Import CSV file (legacy key, same as 'f')
		if err := m.loadDirectoryEntries(); err != nil {
			m.statementMessage = fmt.Sprintf("Error loading directory: %v", err)
		} else {
			m.state = filePickerView
			m.fileIndex = 0
		}
	case "h":
		// View statement history
		m.state = statementHistoryView
		m.statementIndex = 0
	case "esc":
		m.state = menuView
	}
	return m, nil
}

func (m model) handleStatementHistoryView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up":
		if m.statementIndex > 0 {
			m.statementIndex--
		}
	case "down":
		if len(m.store.statements.Statements) > 0 && m.statementIndex < len(m.store.statements.Statements)-1 {
			m.statementIndex++
		}
	case "enter":
		// Show details of selected statement
		if len(m.store.statements.Statements) > 0 && m.statementIndex < len(m.store.statements.Statements) {
			stmt := m.store.statements.Statements[m.statementIndex]
			m.statementMessage = fmt.Sprintf("Statement Details: %s | Period: %s to %s | %d transactions | Template: %s | Status: %s | Import Date: %s",
				stmt.Filename, stmt.PeriodStart, stmt.PeriodEnd, stmt.TxCount, stmt.TemplateUsed, stmt.Status, stmt.ImportDate)
			m.state = bankStatementView
		}
	case "esc":
		m.state = bankStatementView
	}
	return m, nil
}

func (m model) handleStatementOverlapView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y":
		return m.handleOverrideImport()
	case "n", "esc":
		m.state = bankStatementView
	}
	return m, nil
}

func (m model) handleOverrideImport() (tea.Model, tea.Cmd) {
	// Get the template to use
	template := m.store.getTemplateByName(m.selectedTemplate)
	if template == nil {
		m.statementMessage = "Error: No template found"
		return m, nil
	}

	// Parse and import transactions with override
	data, err := os.ReadFile(m.store.importName)
	if err != nil {
		m.statementMessage = fmt.Sprintf("Error reading file: %v", err)
		return m, nil
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		m.statementMessage = "Error: Empty CSV file"
		return m, nil
	}

	var importedTransactions []Transaction
	startLine := 0
	if template.HasHeader {
		startLine = 1
	}

	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		fields := m.store.parseCSVLine(line)
		maxColumn := template.DateColumn
		if template.AmountColumn > maxColumn {
			maxColumn = template.AmountColumn
		}
		if template.DescColumn > maxColumn {
			maxColumn = template.DescColumn
		}

		if len(fields) <= maxColumn {
			continue
		}

		transaction, err := m.store.parseTransactionFromTemplate(fields, template)
		if err != nil {
			continue
		}

		transaction.Id = m.store.nextId
		m.store.nextId++
		transaction.TransactionType = "expense"
		// Set timestamps for imported transactions
		now := time.Now().Format(time.RFC3339)
		transaction.CreatedAt = now
		transaction.UpdatedAt = now
		transaction.Confidence = 0.0
		importedTransactions = append(importedTransactions, transaction)
	}

	// Add transactions to store
	m.store.transactions = append(m.store.transactions, importedTransactions...)

	// Record import with override status
	filename := filepath.Base(m.store.importName)
	periodStart, periodEnd := m.extractPeriodFromTransactions(importedTransactions)
	err = m.store.recordBankStatement(filename, periodStart, periodEnd, m.selectedTemplate, len(importedTransactions), "override")
	if err != nil {
		log.Printf("Failed to record statement: %v", err)
	}

	// Save and refresh
	err = m.store.saveTransactions()
	if err != nil {
		m.statementMessage = fmt.Sprintf("Error saving: %v", err)
		return m, nil
	}

	// Refresh transactions list and return to main view
	m.transactions, _ = m.store.GetTransactions()
	m.state = bankStatementView
	m.statementMessage = fmt.Sprintf("Override import successful: %d transactions", len(importedTransactions))

	return m, nil
}

// Helper function for period extraction
func (m model) extractPeriodFromTransactions(transactions []Transaction) (start, end string) {
	if len(transactions) == 0 {
		return "", ""
	}

	start = transactions[0].Date
	end = transactions[0].Date

	for _, tx := range transactions {
		if tx.Date < start {
			start = tx.Date
		}
		if tx.Date > end {
			end = tx.Date
		}
	}

	return start, end
}
