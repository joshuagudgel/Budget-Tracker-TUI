package ui

import (
	"fmt"
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
	case "s":
		// Save snapshot - prompt for name
		m.state = snapshotNameInputView
		m.snapshotName = ""
		m.isEditingSnapshotName = true
		m.editingSnapshotNameStr = ""
		m.snapshotMessage = ""
	case "l":
		// Load snapshot - open file picker
		m.state = snapshotLoadPickerView
		m.snapshotMessage = ""
		m.snapshotFileIndex = 0
		return m.loadSnapshotDirectory()
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

		// Check for validation errors first
		if result.HasValidationErrors {
			m.validationErrors = result.ValidationErrors
			m.statementMessage = result.Message
			m.state = validationErrorView
			return m, nil
		}

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
			// Clear any existing bank statement list message for fresh display
			m.bankStatementListMessage = ""
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

// Validation Error View Handler

func (m model) handleValidationErrorView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		// Clear validation errors and return to file picker
		m.validationErrors = nil
		m.statementMessage = ""
		m.state = filePickerView
	}
	return m, nil
}

// Snapshot Name Input View Handler
func (m model) handleSnapshotNameInputView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		// Cancel and return to backup view
		m.state = backupView
		m.snapshotMessage = ""
	case "enter":
		// Validate snapshot name and proceed to file picker
		if m.isEditingSnapshotName {
			// Save editing state and deactivate editing
			m.snapshotName = m.editingSnapshotNameStr
			m.isEditingSnapshotName = false
		}

		if strings.TrimSpace(m.snapshotName) == "" {
			m.snapshotMessage = "Please enter a snapshot name"
			return m, nil
		}

		// Proceed to save picker
		m.state = snapshotSavePickerView
		m.snapshotMessage = ""
		m.snapshotFileIndex = 0
		return m.loadSnapshotDirectory()
	case "backspace":
		if m.isEditingSnapshotName && len(m.editingSnapshotNameStr) > 0 {
			m.editingSnapshotNameStr = m.editingSnapshotNameStr[:len(m.editingSnapshotNameStr)-1]
		}
	default:
		if m.isEditingSnapshotName {
			m.editingSnapshotNameStr += key
		}
	}
	return m, nil
}

// Snapshot Save Picker View Handler
func (m model) handleSnapshotSavePickerView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		// Return to name input
		m.state = snapshotNameInputView
	case "s":
		// Save to current directory
		return m.handleSnapshotSaveToCurrentDirectory()
	case "up":
		if m.snapshotFileIndex > 0 {
			m.snapshotFileIndex--
		}
	case "down":
		if len(m.snapshotDirectoryEntries) > 0 && m.snapshotFileIndex < len(m.snapshotDirectoryEntries)-1 {
			m.snapshotFileIndex++
		}
	case "enter":
		return m.handleSnapshotSaveSelection()
	}
	return m, nil
}

// Snapshot Load Picker View Handler
func (m model) handleSnapshotLoadPickerView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		// Return to backup view
		m.state = backupView
		m.snapshotMessage = ""
	case "up":
		if m.snapshotFileIndex > 0 {
			m.snapshotFileIndex--
		}
	case "down":
		if len(m.snapshotDirectoryEntries) > 0 && m.snapshotFileIndex < len(m.snapshotDirectoryEntries)-1 {
			m.snapshotFileIndex++
		}
	case "enter":
		return m.handleSnapshotLoadSelection()
	}
	return m, nil
}

// Load snapshot directory helper
func (m model) loadSnapshotDirectory() (tea.Model, tea.Cmd) {
	if m.currentSnapshotDir == "" {
		// Start with user home directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			m.currentSnapshotDir = "."
		} else {
			m.currentSnapshotDir = homeDir
		}
	}

	result := m.store.LoadSnapshotDirectoryForPicker(m.currentSnapshotDir)
	if !result.Success {
		m.snapshotMessage = result.Message
	} else {
		m.snapshotDirectoryEntries = result.Entries
	}
	return m, nil
}

// Handle snapshot save location selection
func (m model) handleSnapshotSaveSelection() (tea.Model, tea.Cmd) {
	if len(m.snapshotDirectoryEntries) == 0 || m.snapshotFileIndex >= len(m.snapshotDirectoryEntries) {
		return m, nil
	}

	selected := m.snapshotDirectoryEntries[m.snapshotFileIndex]
	fullPath := filepath.Join(m.currentSnapshotDir, selected)

	// Handle directory navigation
	if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
		if selected == ".." {
			m.currentSnapshotDir = filepath.Dir(m.currentSnapshotDir)
		} else {
			m.currentSnapshotDir = fullPath
		}
		m.snapshotFileIndex = 0

		result := m.store.LoadSnapshotDirectoryForPicker(m.currentSnapshotDir)
		if !result.Success {
			m.snapshotMessage = result.Message
		} else {
			m.snapshotDirectoryEntries = result.Entries
		}
		return m, nil
	}

	// Save snapshot to selected directory
	snapshotPath := filepath.Join(m.currentSnapshotDir, m.snapshotName+".db")
	err := m.store.Snapshots.CreateSnapshotFile(snapshotPath)

	if err == nil {
		m.snapshotMessage = fmt.Sprintf("Snapshot saved successfully: %s", snapshotPath)
		m.state = backupView
	} else {
		m.snapshotMessage = fmt.Sprintf("Failed to save snapshot: %s", err.Error())
	}

	return m, nil
}

// Handle saving to current directory
func (m model) handleSnapshotSaveToCurrentDirectory() (tea.Model, tea.Cmd) {
	// Save snapshot to current directory
	snapshotPath := filepath.Join(m.currentSnapshotDir, m.snapshotName+".db")
	err := m.store.Snapshots.CreateSnapshotFile(snapshotPath)

	if err == nil {
		m.snapshotMessage = fmt.Sprintf("Snapshot saved successfully: %s", snapshotPath)
		m.state = backupView
	} else {
		m.snapshotMessage = fmt.Sprintf("Failed to save snapshot: %s", err.Error())
	}

	return m, nil
}

// Handle snapshot load selection
func (m model) handleSnapshotLoadSelection() (tea.Model, tea.Cmd) {
	if len(m.snapshotDirectoryEntries) == 0 || m.snapshotFileIndex >= len(m.snapshotDirectoryEntries) {
		return m, nil
	}

	selected := m.snapshotDirectoryEntries[m.snapshotFileIndex]
	fullPath := filepath.Join(m.currentSnapshotDir, selected)

	// Handle directory navigation
	if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
		if selected == ".." {
			m.currentSnapshotDir = filepath.Dir(m.currentSnapshotDir)
		} else {
			m.currentSnapshotDir = fullPath
		}
		m.snapshotFileIndex = 0

		result := m.store.LoadSnapshotDirectoryForPicker(m.currentSnapshotDir)
		if !result.Success {
			m.snapshotMessage = result.Message
		} else {
			m.snapshotDirectoryEntries = result.Entries
		}
		return m, nil
	}

	// Load snapshot from selected file
	if strings.HasSuffix(strings.ToLower(selected), ".db") {
		// Create backup of current database before restore
		backupPath, err := m.createCurrentDatabaseBackup()
		if err != nil {
			m.snapshotMessage = fmt.Sprintf("Failed to create backup before restore: %s", err.Error())
			return m, nil
		}

		// Simple restore by copying file and reinitializing database
		err = m.restoreFromSnapshotFile(fullPath)

		if err == nil {
			// Successful restore - reload all data
			m.transactions, _ = m.store.Transactions.GetTransactions()
			// Reset UI state
			m.listIndex = 0
			m.snapshotMessage = fmt.Sprintf("Snapshot restored successfully from: %s (backup saved to: %s)", fullPath, backupPath)
			m.state = backupView
		} else {
			m.snapshotMessage = fmt.Sprintf("Failed to restore snapshot: %s", err.Error())
		}
	} else {
		m.snapshotMessage = "Please select a .db file to load"
	}

	return m, nil
}
