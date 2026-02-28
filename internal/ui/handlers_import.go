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
		result, err := m.store.Transactions.RestoreFromBackup()
		if err != nil {
			m.backupMessage = "Error: " + err.Error()
		} else if result.Success {
			m.transactions, _ = m.store.Transactions.GetTransactions()
			m.sortTransactionsByDate()
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

		result := m.store.Statements.LoadDirectoryEntries(m.currentDir)
		if !result.Success {
			m.statementMessage = result.Message
		} else {
			m.dirEntries = result.Entries
		}
		return m, nil
	}

	// Handle CSV file selection
	if strings.HasSuffix(strings.ToLower(selected), ".csv") {
		templateToUse := m.store.Templates.GetDefaultTemplate()
		if templateToUse == "" {
			templates, _ := m.store.Templates.GetCSVTemplates()
			if len(templates) > 0 {
				templateToUse = templates[0].Name
			}
		}

		result := m.store.ValidateAndImportCSV(fullPath, templateToUse)
		if result.OverlapDetected {
			m.overlappingStmts = result.OverlappingStmts
			m.selectedTemplate = templateToUse
			m.selectedFile = fullPath
			// Store current import details for overlap warning
			m.currentImportFilename = result.Filename
			m.currentImportPeriodStart = result.PeriodStart
			m.currentImportPeriodEnd = result.PeriodEnd
			m.state = statementOverlapView
			return m, nil
		}

		if result.Success {
			m.transactions, _ = m.store.Transactions.GetTransactions()
			m.sortTransactionsByDate()
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
	templateToUse := m.store.Templates.GetDefaultTemplate()
	templates, _ := m.store.Templates.GetCSVTemplates()
	if templateToUse == "" && len(templates) > 0 {
		templateToUse = "unsorted"
	}
	return templateToUse
}

// CSV Template View

func (m model) handleCSVTemplateView(key string) (tea.Model, tea.Cmd) {
	templates, _ := m.store.Templates.GetCSVTemplates()

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

			result := m.store.Templates.SetDefaultTemplate(selectedTemplate.Name)
			m.importMessage = result.Message
			m.state = bankStatementView
		}
	case "d":
		if len(templates) > 0 && m.templateIndex < len(templates) {
			selectedTemplate := templates[m.templateIndex]

			result := m.store.Templates.DeleteCSVTemplate(selectedTemplate.Id)
			m.importMessage = result.Message

			if result.Success {
				// Adjust template index if we deleted the last item
				if m.templateIndex > 0 && m.templateIndex >= len(templates)-1 {
					m.templateIndex--
				}
			}
		}
	case "c":
		m.newTemplate = types.CSVTemplate{}
		m.createField = templateName
		m.createMessage = ""
		m.state = createTemplateView
	}
	return m, nil
}

// Create CSV Template View

func (m model) handleCreateTemplateView(key string) (tea.Model, tea.Cmd) {
	// Handle field-specific editing input first
	if m.isEditingTemplateName {
		return m.handleTemplateNameInput(key)
	}
	if m.isEditingTemplatePostDate {
		return m.handleTemplatePostDateInput(key)
	}
	if m.isEditingTemplateAmount {
		return m.handleTemplateAmountInput(key)
	}
	if m.isEditingTemplateDesc {
		return m.handleTemplateDescInput(key)
	}
	if m.isEditingTemplateCategory {
		return m.handleTemplateCategoryInput(key)
	}

	// Handle navigation and general commands
	switch key {
	case "esc":
		m.state = csvTemplateView
		m.createMessage = ""
	case "down", "tab":
		return m.handleTemplateFieldNavigation(1)
	case "up", "shift+tab":
		return m.handleTemplateFieldNavigation(-1)
	case "enter":
		return m.handleTemplateFieldActivation()
	case "backspace":
		return m.handleTemplateBackspaceActivation()
	case "ctrl+s":
		return m.handleTemplateSave()
	}
	return m, nil
}

func (m model) handleCreateTemplateInput(key string) (tea.Model, tea.Cmd) {
	// This function is deprecated - field editing now handled by two-phase system
	return m, nil
}

func (m model) handleCreateTemplateBackspace() (tea.Model, tea.Cmd) {
	// This function is deprecated - field editing now handled by two-phase system
	return m, nil
}

func (m model) handleTemplateSave() (tea.Model, tea.Cmd) {
	// Validate template before saving
	m.validateCurrentTemplate()
	if m.templateValidationErrors {
		return m, nil // Don't save if there are validation errors
	}

	// Use store's business logic to create template
	result := m.store.Templates.CreateCSVTemplate(m.newTemplate)
	if result.Success {
		m.createMessage = "Template created successfully"
		m.state = csvTemplateView
		// Reset template state
		m.newTemplate = types.CSVTemplate{}
		m.createField = templateName
		m.clearTemplateEditingState()
	} else {
		m.createMessage = result.Message
	}
	return m, nil
}

func (m *model) clearTemplateEditingState() {
	m.isEditingTemplateName = false
	m.isEditingTemplatePostDate = false
	m.isEditingTemplateAmount = false
	m.isEditingTemplateDesc = false
	m.isEditingTemplateCategory = false
	m.editingTemplateNameStr = ""
	m.editingTemplatePostDateStr = ""
	m.editingTemplateAmountStr = ""
	m.editingTemplateDescStr = ""
	m.editingTemplateCategoryStr = ""
	m.templateFieldErrors = make(map[string]string)
	m.templateValidationErrors = false
	m.templateValidationNotification = ""
}
