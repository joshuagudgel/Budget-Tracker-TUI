package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Backup View

func (m model) handleBackupView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.state = menuView
	case "r":
		err := m.store.RestoreFromBackup()
		if err != nil {
			m.backupMessage = fmt.Sprintf("Error: %v", err)
		} else {
			m.transactions, _ = m.store.GetTransactions()
			m.backupMessage = "Successfully restored from backup"
			m.listIndex = 0
		}
	}
	return m, nil
}

// File Picker View

func (m model) handleFilePickerView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		// Return to appropriate view based on context
		if m.state == filePickerView {
			// Check which view we came from
			m.state = bankStatementView
		}
	case "up":
		if m.fileIndex > 0 {
			m.fileIndex--
		}
	case "down":
		if len(m.dirEntries) > 0 && m.fileIndex < len(m.dirEntries)-1 {
			m.fileIndex++
		}
	case "enter":
		if len(m.dirEntries) > 0 && m.fileIndex < len(m.dirEntries) {
			selected := m.dirEntries[m.fileIndex]
			fullPath := filepath.Join(m.currentDir, selected)

			// Check if it's a directory
			if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
				if selected == ".." {
					m.currentDir = filepath.Dir(m.currentDir)
				} else {
					m.currentDir = fullPath
				}
				m.fileIndex = 0
				err = m.loadDirectoryEntries()
				if err != nil {
					m.statementMessage = fmt.Sprintf("Error opening directory: %v", err)
					return m, nil
				}
			} else if strings.HasSuffix(strings.ToLower(selected), ".csv") {
				// CSV file selected - set import path and check for overlaps
				m.store.importName = fullPath
				m.selectedFile = fullPath

				templateToUse := m.store.csvTemplates.Default
				if templateToUse == "" && len(m.store.csvTemplates.Templates) > 0 {
					templateToUse = m.store.csvTemplates.Templates[0].Name
				}

				// Attempt import to check for overlaps
				err := m.store.ImportTransactionsFromCSV(templateToUse)
				if err != nil {
					if strings.Contains(err.Error(), "OVERLAP_DETECTED") {
						// Extract period and detect overlaps
						template := m.store.getTemplateByName(templateToUse)
						if template != nil {
							// Quick parse to get period for overlap detection
							data, readErr := os.ReadFile(m.store.importName)
							if readErr == nil {
								lines := strings.Split(string(data), "\n")
								var tempTransactions []Transaction
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
									if len(fields) <= template.DateColumn || len(fields) <= template.AmountColumn || len(fields) <= template.DescColumn {
										continue
									}
									if transaction, parseErr := m.store.parseTransactionFromTemplate(fields, template); parseErr == nil {
										tempTransactions = append(tempTransactions, transaction)
									}
								}

								if len(tempTransactions) > 0 {
									periodStart, periodEnd := m.store.extractPeriodFromTransactions(tempTransactions)
									m.overlappingStmts = m.store.detectOverlap(periodStart, periodEnd)
									m.state = statementOverlapView
									return m, nil
								}
							}
						}
						m.statementMessage = "Overlap detected but unable to show details"
					} else {
						m.statementMessage = fmt.Sprintf("Error: %v", err)
					}
				} else {
					m.transactions, _ = m.store.GetTransactions()
					//imported := len(m.transactions) - len(m.transactions)
					m.statementMessage = fmt.Sprintf("Successfully imported transactions from %s using template %s",
						filepath.Base(selected), templateToUse)
				}
				m.state = bankStatementView
			}
		}
	}
	return m, nil
}

func (m *model) loadDirectoryEntries() error {
	// Initialize currentDir to user's home directory if not set
	if m.currentDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		m.currentDir = homeDir
	}

	entries, err := os.ReadDir(m.currentDir)
	if err != nil {
		return err
	}

	m.dirEntries = []string{}

	// Add parent directory option if not at root
	if m.currentDir != filepath.Dir(m.currentDir) {
		m.dirEntries = append(m.dirEntries, "..")
	}

	// Add directories first
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			m.dirEntries = append(m.dirEntries, entry.Name())
		}
	}

	// Add CSV files
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".csv") {
			m.dirEntries = append(m.dirEntries, entry.Name())
		}
	}

	return nil
}

// CSV Template View

func (m model) handleCSVTemplateView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.state = bankStatementView
	case "up":
		if m.templateIndex > 0 {
			m.templateIndex--
		}
	case "down":
		if len(m.store.csvTemplates.Templates) > 0 && m.templateIndex < len(m.store.csvTemplates.Templates)-1 {
			m.templateIndex++
		}
	case "enter":
		if len(m.store.csvTemplates.Templates) > 0 && m.templateIndex < len(m.store.csvTemplates.Templates) {
			selectedTemplate := m.store.csvTemplates.Templates[m.templateIndex]
			m.selectedTemplate = selectedTemplate.Name

			// Update the store's default template
			m.store.csvTemplates.Default = selectedTemplate.Name
			err := m.store.saveCSVTemplates()
			if err != nil {
				m.importMessage = fmt.Sprintf("Error saving template selection: %v", err)
			} else {
				m.importMessage = fmt.Sprintf("Selected CSV template: %s", selectedTemplate.Name)
			}

			m.state = bankStatementView
		}
	case "c":
		m.newTemplate = CSVTemplate{}
		m.createField = createTemplateName
		m.createMessage = ""
		m.state = createTemplateView
	}
	return m, nil
}

// Create CSV Template View

func (m model) handleCreateTemplateView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.state = csvTemplateView
	case "down", "tab":
		if m.createField < createTemplateHeader {
			m.createField++
		}
	case "up":
		if m.createField > createTemplateName {
			m.createField--
		}
	case "enter":
		return m.handleSaveTemplate()
	case "backspace":
		return m.handleCreateTemplateBackspace()
	default:
		if len(key) == 1 {
			return m.handleCreateTemplateInput(key)
		}
	}
	return m, nil
}

func (m model) handleCreateTemplateInput(key string) (tea.Model, tea.Cmd) {
	switch m.createField {
	case createTemplateName:
		m.newTemplate.Name += key
	case createTemplateDate:
		if key >= "0" && key <= "9" {
			if digit, err := strconv.Atoi(key); err == nil {
				m.newTemplate.DateColumn = m.newTemplate.DateColumn*10 + digit
			}
		}
	case createTemplateAmount:
		if key >= "0" && key <= "9" {
			if digit, err := strconv.Atoi(key); err == nil {
				m.newTemplate.AmountColumn = m.newTemplate.AmountColumn*10 + digit
			}
		}
	case createTemplateDesc:
		if key >= "0" && key <= "9" {
			if digit, err := strconv.Atoi(key); err == nil {
				m.newTemplate.DescColumn = m.newTemplate.DescColumn*10 + digit
			}
		}
	case createTemplateHeader:
		switch key {
		case "y", "Y":
			m.newTemplate.HasHeader = true
		case "n", "N":
			m.newTemplate.HasHeader = false
		}
	}
	return m, nil
}

func (m model) handleCreateTemplateBackspace() (tea.Model, tea.Cmd) {
	switch m.createField {
	case createTemplateName:
		if len(m.newTemplate.Name) > 0 {
			m.newTemplate.Name = m.newTemplate.Name[:len(m.newTemplate.Name)-1]
		}
	case createTemplateDate:
		m.newTemplate.DateColumn = m.newTemplate.DateColumn / 10
	case createTemplateAmount:
		m.newTemplate.AmountColumn = m.newTemplate.AmountColumn / 10
	case createTemplateDesc:
		m.newTemplate.DescColumn = m.newTemplate.DescColumn / 10
	case createTemplateHeader:
		m.newTemplate.HasHeader = false
	}
	return m, nil
}

func (m model) handleSaveTemplate() (tea.Model, tea.Cmd) {
	// Validate template name is not empty
	if strings.TrimSpace(m.newTemplate.Name) == "" {
		m.createMessage = "Template name cannot be empty"
		return m, nil
	}

	// Check for duplicate names
	for _, template := range m.store.csvTemplates.Templates {
		if template.Name == m.newTemplate.Name {
			m.createMessage = "Template name already exists"
			return m, nil
		}
	}

	// Add new template and set as default
	m.store.csvTemplates.Templates = append(m.store.csvTemplates.Templates, m.newTemplate)
	m.store.csvTemplates.Default = m.newTemplate.Name

	// Save templates
	err := m.store.saveCSVTemplates()
	if err != nil {
		m.createMessage = fmt.Sprintf("Error saving template: %v", err)
		return m, nil
	}

	// Return to CSV template view
	m.state = csvTemplateView
	return m, nil
}
