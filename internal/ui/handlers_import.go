package ui

import (
	"os"
	"path/filepath"
	"strings"

	"budget-tracker-tui/internal/types"

	tea "github.com/charmbracelet/bubbletea"
)

// Backup View

func (m model) handleBackupView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.state = menuView
	case "r":
		result := m.store.RestoreTransactionsFromBackup()
		if result.Success {
			m.transactions, _ = m.store.GetTransactions()
			m.backupMessage = result.Message
			m.listIndex = 0
		} else {
			m.backupMessage = result.Message
		}
	}
	return m, nil
}

// File Picker View
func (m model) handleFilePickerView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.state = bankStatementView
	case "up":
		if m.fileIndex > 0 {
			m.fileIndex--
		}
	case "down":
		if len(m.dirEntries) > 0 && m.fileIndex < len(m.dirEntries)-1 {
			m.fileIndex++
		}
	case "enter":
		return m.handleFileSelection()
	}
	return m, nil
}

func (m model) handleFileSelection() (tea.Model, tea.Cmd) {
	if len(m.dirEntries) == 0 || m.fileIndex >= len(m.dirEntries) {
		return m, nil
	}

	selected := m.dirEntries[m.fileIndex]
	fullPath := filepath.Join(m.currentDir, selected)

	// Handle directory navigation
	if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
		if selected == ".." {
			m.currentDir = filepath.Dir(m.currentDir)
		} else {
			m.currentDir = fullPath
		}
		m.fileIndex = 0

		result := m.store.LoadDirectoryEntries(m.currentDir)
		if !result.Success {
			m.statementMessage = result.Message
		} else {
			m.dirEntries = result.Entries
		}
		return m, nil
	}

	// Handle CSV file selection
	if strings.HasSuffix(strings.ToLower(selected), ".csv") {
		templateToUse := m.store.GetDefaultTemplate()
		if templateToUse == "" {
			templates := m.store.GetCSVTemplates()
			if len(templates) > 0 {
				templateToUse = templates[0].Name
			}
		}

		result := m.store.ValidateAndImportCSV(fullPath, templateToUse)
		if result.OverlapDetected {
			m.overlappingStmts = result.OverlappingStmts
			m.selectedTemplate = templateToUse
			m.selectedFile = fullPath
			m.state = statementOverlapView
			return m, nil
		}

		if result.Success {
			m.transactions, _ = m.store.GetTransactions()
		}
		m.statementMessage = result.Message
		m.state = bankStatementView
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

func (m model) getSelectedTemplate() string {
	templateToUse := m.store.GetDefaultTemplate()
	if templateToUse == "" && len(m.store.GetCSVTemplates()) > 0 {
		templateToUse = "unsorted"
	}
	return templateToUse
}

// CSV Template View

func (m model) handleCSVTemplateView(key string) (tea.Model, tea.Cmd) {
	templates := m.store.GetCSVTemplates()

	switch key {
	case "esc":
		m.state = bankStatementView
	case "up":
		if m.templateIndex > 0 {
			m.templateIndex--
		}
	case "down":
		if len(templates) > 0 && m.templateIndex < len(templates)-1 {
			m.templateIndex++
		}
	case "enter":
		if len(templates) > 0 && m.templateIndex < len(templates) {
			selectedTemplate := templates[m.templateIndex]

			result := m.store.SetDefaultTemplate(selectedTemplate.Name)
			m.importMessage = result.Message
			m.state = bankStatementView
		}
	case "c":
		m.newTemplate = types.CSVTemplate{}
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
		// Use store's business logic instead of UI logic
		result := m.store.CreateCSVTemplate(m.newTemplate)
		if result.Success {
			m.createMessage = ""
			m.state = csvTemplateView
		} else {
			m.createMessage = result.Message
		}
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
			digit := int(key[0] - '0')
			m.newTemplate.DateColumn = m.newTemplate.DateColumn*10 + digit
		}
	case createTemplateAmount:
		if key >= "0" && key <= "9" {
			digit := int(key[0] - '0')
			m.newTemplate.AmountColumn = m.newTemplate.AmountColumn*10 + digit
		}
	case createTemplateDesc:
		if key >= "0" && key <= "9" {
			digit := int(key[0] - '0')
			m.newTemplate.DescColumn = m.newTemplate.DescColumn*10 + digit
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
