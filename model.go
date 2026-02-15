package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	menuView             uint = iota
	listView                  = 1
	titleView                 = 2
	bodyView                  = 3
	editView                  = 4
	backupView                = 5
	filePickerView            = 6
	csvTemplateView           = 7
	createTemplateView        = 8
	categoryView              = 9
	createCategoryView        = 10
	bulkEditView              = 11
	bankStatementView         = 12
	statementHistoryView      = 13
	statementOverlapView      = 14
)

const (
	editAmount uint = iota
	editDescription
	editDate
	editType
	editCategory
	editSplit
)

const (
	createTemplateName uint = iota
	createTemplateDate
	createTemplateAmount
	createTemplateDesc
	createTemplateHeader
)

const (
	splitAmount1Field uint = iota
	splitDesc1Field
	splitCategory1Field
	splitAmount2Field
	splitDesc2Field
	splitCategory2Field
)

const (
	createCategoryName uint = iota
	createCategoryDisplayName
	bulkEditAmount uint = iota
	bulkEditDescription
	bulkEditDate
	bulkEditCategory
	bulkEditType
)

type model struct {
	state           uint
	store           *Store
	transactions    []Transaction
	currTransaction Transaction
	listIndex       int
	windowHeight    int
	editField       uint
	backupMessage   string
	importMessage   string
	editAmountStr   string
	// file explorer
	currentDir   string
	dirEntries   []string
	fileIndex    int
	selectedFile string
	// CSV template creation
	templateIndex    int
	selectedTemplate string
	newTemplate      CSVTemplate
	createField      uint
	createMessage    string
	// category management
	categoryIndex       int
	selectedCategory    string
	newCategory         Category
	createCategoryField uint
	categoryMessage     string

	// Split editing state flags (matching main edit pattern)
	isSplitEditingAmount1   bool
	isSplitEditingAmount2   bool
	isSplitEditingDesc1     bool
	isSplitEditingDesc2     bool
	isSplitEditingCategory1 bool
	isSplitEditingCategory2 bool

	// Split selection modes (for dropdown categories)
	isSplitSelectingCategory1 bool
	isSplitSelectingCategory2 bool
	splitCat1SelectIndex      int
	splitCat2SelectIndex      int

	// Temporary editing values
	splitEditingAmount1   string
	splitEditingAmount2   string
	splitEditingDesc1     string
	splitEditingDesc2     string
	splitEditingCategory1 string
	splitEditingCategory2 string

	// Split transaction fields
	isSplitMode    bool
	splitAmount1   string
	splitAmount2   string
	splitDesc1     string
	splitDesc2     string
	splitCategory1 string
	splitCategory2 string
	splitField     uint
	splitMessage   string

	// Multi-select / bulk edit mode
	isMultiSelectMode        bool
	selectedTxIds            map[int64]bool
	bulkEditField            uint
	bulkEditValue            string
	isBulkSelectingCategory  bool
	isBulkSelectingType      bool
	bulkCategorySelectIndex  int
	bulkTypeSelectIndex      int
	bulkCategoryValue        string
	bulkTypeValue            string
	bulkAmountValue          string
	bulkDescriptionValue     string
	bulkDateValue            string
	isBulkEditingAmount      bool
	isBulkEditingDescription bool
	isBulkEditingDate        bool
	// Bulk edit placeholder states
	bulkAmountIsPlaceholder      bool
	bulkDescriptionIsPlaceholder bool
	bulkDateIsPlaceholder        bool
	bulkCategoryIsPlaceholder    bool
	bulkTypeIsPlaceholder        bool

	// Selection mode fields
	isSelectingCategory bool
	isSelectingType     bool
	categorySelectIndex int
	typeSelectIndex     int
	availableTypes      []string

	// Field editing state
	isEditingAmount      bool
	isEditingDescription bool
	isEditingDate        bool
	editingAmountStr     string
	editingDescStr       string
	editingDateStr       string

	// Bank statement fields
	statementIndex   int
	overlappingStmts []BankStatement
	pendingStatement BankStatement
	statementMessage string
}

func NewModel(store *Store) model {
	transactions, err := store.GetTransactions()
	if err != nil {
		log.Fatalf("unable to get notes: %v", err)
	}
	return model{
		state:          menuView,
		store:          store,
		transactions:   transactions,
		listIndex:      0,
		availableTypes: []string{"income", "expense", "transfer"},
		//currTransaction: transactions[0],
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if file, err := tea.LogToFile("debug.log", "debug"); err == nil {
		defer file.Close()

		// Safe debug logging with bounds checking
		currTxId := int64(-1)
		if len(m.transactions) > 0 && m.listIndex >= 0 && m.listIndex < len(m.transactions) {
			currTxId = m.transactions[m.listIndex].Id
		}

		log.Printf("State: %d, ListIndex: %d, Transactions: %d, CurrTx: %d",
			m.state, m.listIndex, len(m.transactions), currTxId)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch m.state {
		case menuView:
			return m.handleMenuView(key)
		case listView:
			return m.handleListView(key)
		case editView:
			return m.handleEditView(key)
		case backupView:
			return m.handleBackupView(key)
		case filePickerView:
			return m.handleFilePickerView(key)
		case csvTemplateView:
			return m.handleCSVTemplateView(key)
		case createTemplateView:
			return m.handleCreateTemplateView(key)
		case categoryView:
			return m.handleCategoryView(key)
		case createCategoryView:
			return m.handleCreateCategoryView(key)
		case bulkEditView:
			return m.handleBulkEditView(key)
		case bankStatementView:
			return m.handleBankStatementView(key)
		case statementHistoryView:
			return m.handleStatementHistoryView(key)
		case statementOverlapView:
			return m.handleStatementOverlapView(key)
		}
	case tea.WindowSizeMsg:
		m.windowHeight = msg.Height
		return m, nil
	}
	return m, nil
}

func (m model) handleMenuView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "t":
		m.state = listView
	case "r":
		m.state = backupView
		m.backupMessage = ""
	case "i":
		m.state = bankStatementView
		m.statementMessage = ""
	case "c":
		m.state = categoryView
		m.categoryMessage = ""
		m.categoryIndex = 0
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

// List View

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
				m.state = bulkEditView
			}
			return m, nil
		}
		// Single edit mode (existing logic)
		if len(m.transactions) > 0 {
			m.currTransaction = m.transactions[m.listIndex]
			m.editField = editAmount
			m.editAmountStr = ""
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
		if !m.isMultiSelectMode {
			// Existing single delete logic
			m.store.DeleteTransaction(m.transactions[m.listIndex].Id)
			m.transactions, _ = m.store.GetTransactions()
			// Bounds checking for list index
			if m.listIndex >= len(m.transactions) && len(m.transactions) > 0 {
				m.listIndex = len(m.transactions) - 1
			}
			if len(m.transactions) == 0 {
				m.listIndex = 0
			}
		}
	case "esc":
		if m.isMultiSelectMode {
			return m.exitMultiSelectMode()
		}
		m.state = menuView
	}
	return m, nil
}

func (m model) handleMultiSelectToggle() (tea.Model, tea.Cmd) {
	if file, err := tea.LogToFile("debug.log", "debug"); err == nil {
		defer file.Close()
		log.Printf("handleMultiSelectToggle called: current mode=%v", m.isMultiSelectMode)
	}

	if !m.isMultiSelectMode {
		// Enter multi-select mode
		m.isMultiSelectMode = true
		m.selectedTxIds = make(map[int64]bool)
		if file, err := tea.LogToFile("debug.log", "debug"); err == nil {
			defer file.Close()
			log.Printf("Entered multi-select mode")
		}
	} else {
		// Exit multi-select mode
		if file, err := tea.LogToFile("debug.log", "debug"); err == nil {
			defer file.Close()
			log.Printf("Exiting multi-select mode")
		}
		return m.exitMultiSelectMode()
	}
	return m, nil
}

func (m model) exitMultiSelectMode() (tea.Model, tea.Cmd) {
	m.isMultiSelectMode = false
	m.selectedTxIds = make(map[int64]bool)
	return m, nil
}

func (m model) handleToggleSelection() (tea.Model, tea.Cmd) {
	if len(m.transactions) > 0 && m.listIndex < len(m.transactions) {
		txId := m.transactions[m.listIndex].Id
		if m.selectedTxIds[txId] {
			delete(m.selectedTxIds, txId)
		} else {
			m.selectedTxIds[txId] = true
		}
	}
	return m, nil
}

// Edit Transaction View

func (m model) handleEditView(key string) (tea.Model, tea.Cmd) {
	// Handle active editing states
	if m.isEditingAmount {
		return m.handleAmountEditing(key)
	}
	if m.isEditingDescription {
		return m.handleDescriptionEditing(key)
	}
	if m.isEditingDate {
		return m.handleDateEditing(key)
	}
	if m.isSelectingCategory {
		return m.handleCategorySelection(key)
	}
	if m.isSelectingType {
		return m.handleTypeSelection(key)
	}

	if m.isSplitMode {
		return m.handleSplitFieldEditing(key)
	}

	switch key {
	case "esc":
		if m.isSplitMode {
			return m.exitSplitMode()
		}
		m.state = listView
	case "enter":
		return m.handleFieldActivation()
	case "backspace":
		// Enter editing mode for text fields only
		return m.handleBackspaceActivation()
	case "s":
		if !m.isSplitMode {
			return m.enterSplitMode()
		} else {
			return m.exitSplitMode()
		}
	case "ctrl+s": // Save entire transaction
		return m.handleSaveTransaction()
	case "down", "tab":
		if m.isSplitMode {
			return m.handleSplitFieldNavigation(1)
		}
		return m.handleFieldNavigation(1)
	case "up":
		if m.isSplitMode {
			return m.handleSplitFieldNavigation(-1)
		}
		return m.handleFieldNavigation(-1)
	}
	return m, nil
}

// Edit Transaction Helpers

func (m model) handleFieldNavigation(direction int) (tea.Model, tea.Cmd) {
	if direction > 0 && m.editField < editCategory {
		m.editField++
	} else if direction < 0 && m.editField > editAmount {
		m.editField--
	}

	// Initialize amount string when entering amount field
	if m.editField == editAmount && m.editAmountStr == "" {
		if m.currTransaction.Amount != 0 {
			m.editAmountStr = fmt.Sprintf("%.2f", m.currTransaction.Amount)
		}
	}
	return m, nil
}

func (m model) handleBackspaceActivation() (tea.Model, tea.Cmd) {
	// Only activate text input fields with backspace
	switch m.editField {
	case editAmount:
		return m.enterAmountEditingWithBackspace()
	case editDescription:
		return m.enterDescriptionEditingWithBackspace()
	case editDate:
		return m.enterDateEditingWithBackspace()
	case editType, editCategory:
		// For dropdown fields, backspace doesn't activate - only enter does
		return m, nil
	}
	return m, nil
}

func (m model) enterAmountEditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isEditingAmount = true
	// Start with current value and immediately apply backspace
	m.editingAmountStr = fmt.Sprintf("%.2f", m.currTransaction.Amount)
	if len(m.editingAmountStr) > 0 {
		m.editingAmountStr = m.editingAmountStr[:len(m.editingAmountStr)-1]
	}
	return m, nil
}

func (m model) enterDescriptionEditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isEditingDescription = true
	// Start with current value and immediately apply backspace
	m.editingDescStr = m.currTransaction.Description
	if len(m.editingDescStr) > 0 {
		m.editingDescStr = m.editingDescStr[:len(m.editingDescStr)-1]
	}
	return m, nil
}

func (m model) enterDateEditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isEditingDate = true
	// Start with current value and immediately apply backspace
	m.editingDateStr = m.currTransaction.Date
	if len(m.editingDateStr) > 0 {
		m.editingDateStr = m.editingDateStr[:len(m.editingDateStr)-1]
	}
	return m, nil
}

func (m model) handleFieldActivation() (tea.Model, tea.Cmd) {
	switch m.editField {
	case editAmount:
		return m.enterAmountEditing()
	case editDescription:
		return m.enterDescriptionEditing()
	case editDate:
		return m.enterDateEditing()
	case editType:
		return m.enterTypeSelection()
	case editCategory:
		return m.enterCategorySelection()
	}
	return m, nil
}

func (m model) enterAmountEditing() (tea.Model, tea.Cmd) {
	m.isEditingAmount = true
	m.editingAmountStr = fmt.Sprintf("%.2f", m.currTransaction.Amount)
	return m, nil
}

func (m model) enterDescriptionEditing() (tea.Model, tea.Cmd) {
	m.isEditingDescription = true
	m.editingDescStr = m.currTransaction.Description
	return m, nil
}

func (m model) enterDateEditing() (tea.Model, tea.Cmd) {
	m.isEditingDate = true
	m.editingDateStr = m.currTransaction.Date
	return m, nil
}

func (m model) handleAmountEditing(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		// Save amount with proper formatting and exit editing mode
		if amount, err := strconv.ParseFloat(m.editingAmountStr, 64); err == nil {
			m.currTransaction.Amount = amount
		}
		m.isEditingAmount = false
		m.editingAmountStr = ""
	case "backspace":
		if len(m.editingAmountStr) > 0 {
			m.editingAmountStr = m.editingAmountStr[:len(m.editingAmountStr)-1]
		}
	default:
		if len(key) == 1 {
			return m.handleAmountInput(key)
		}
	}
	return m, nil
}

func (m model) handleAmountInput(key string) (tea.Model, tea.Cmd) {
	// Handle negative sign
	if key == "-" {
		if len(m.editingAmountStr) == 0 {
			m.editingAmountStr = "-"
		}
		return m, nil
	}

	// Only allow digits and decimal point
	if (key >= "0" && key <= "9") || key == "." {
		// Don't allow multiple decimal points
		if key == "." && strings.Contains(m.editingAmountStr, ".") {
			return m, nil
		}

		newStr := m.editingAmountStr + key

		// Validate decimal places (max 2)
		dotIndex := strings.LastIndex(newStr, ".")
		if dotIndex != -1 && len(newStr)-dotIndex-1 > 2 {
			return m, nil
		}

		// Validate it's a valid number format
		if _, err := strconv.ParseFloat(newStr, 64); err == nil || newStr == "." || newStr == "-." {
			m.editingAmountStr = newStr
		}
	}
	return m, nil
}

func (m model) handleDescriptionEditing(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		// Save description and exit editing mode
		m.currTransaction.Description = strings.TrimSpace(m.editingDescStr)
		m.isEditingDescription = false
		m.editingDescStr = ""
	case "backspace":
		if len(m.editingDescStr) > 0 {
			m.editingDescStr = m.editingDescStr[:len(m.editingDescStr)-1]
		}
	default:
		if len(key) == 1 {
			m.editingDescStr += key
		}
	}
	return m, nil
}

func (m model) handleDateEditing(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		// Save date and exit editing mode
		m.currTransaction.Date = strings.TrimSpace(m.editingDateStr)
		m.isEditingDate = false
		m.editingDateStr = ""
	case "backspace":
		if len(m.editingDateStr) > 0 {
			m.editingDateStr = m.editingDateStr[:len(m.editingDateStr)-1]
		}
	default:
		if len(key) == 1 {
			m.editingDateStr += key
		}
	}
	return m, nil
}

func (m model) handleSaveTransaction() (tea.Model, tea.Cmd) {
	// Validate and save amount from edit string with proper formatting
	if m.editField == editAmount && m.editAmountStr != "" {
		if amount, err := strconv.ParseFloat(m.editAmountStr, 64); err == nil {
			m.currTransaction.Amount = amount
		}
	}
	m.editAmountStr = ""
	err := m.store.SaveTransaction(m.currTransaction)
	if err != nil {
		log.Printf("Error saving transaction: %v", err)
	} else {
		m.transactions, _ = m.store.GetTransactions()
	}
	m.state = listView
	return m, nil
}

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

// Update the existing handleFilePickerView method
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

// Category View --------------------
func (m model) handleCategoryView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.state = menuView
	case "up":
		if m.categoryIndex > 0 {
			m.categoryIndex--
		}
	case "down":
		if len(m.store.categories.Categories) > 0 && m.categoryIndex < len(m.store.categories.Categories)-1 {
			m.categoryIndex++
		}
	case "enter":
		if len(m.store.categories.Categories) > 0 && m.categoryIndex < len(m.store.categories.Categories) {
			selectedCategory := m.store.categories.Categories[m.categoryIndex]
			m.selectedCategory = selectedCategory.Name

			// Update the store's default category
			m.store.categories.Default = selectedCategory.Name
			err := m.store.saveCategories()
			if err != nil {
				m.categoryMessage = fmt.Sprintf("Error saving category selection: %v", err)
			} else {
				m.categoryMessage = fmt.Sprintf("Selected default category: %s", selectedCategory.DisplayName)
			}
		}
	case "c":
		m.newCategory = Category{}
		m.createCategoryField = createCategoryName
		m.categoryMessage = ""
		m.state = createCategoryView
	}
	return m, nil
}

// Create Category View
func (m model) handleCreateCategoryView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.state = categoryView
	case "down", "tab":
		if m.createCategoryField < createCategoryDisplayName {
			m.createCategoryField++
		}
	case "up":
		if m.createCategoryField > createCategoryName {
			m.createCategoryField--
		}
	case "enter":
		return m.handleSaveCategory()
	case "backspace":
		return m.handleCreateCategoryBackspace()
	default:
		if len(key) == 1 {
			return m.handleCreateCategoryInput(key)
		}
	}
	return m, nil
}

func (m model) handleCreateCategoryInput(key string) (tea.Model, tea.Cmd) {
	switch m.createCategoryField {
	case createCategoryName:
		m.newCategory.Name += key
	case createCategoryDisplayName:
		m.newCategory.DisplayName += key
	}
	return m, nil
}

func (m model) handleCreateCategoryBackspace() (tea.Model, tea.Cmd) {
	switch m.createCategoryField {
	case createCategoryName:
		if len(m.newCategory.Name) > 0 {
			m.newCategory.Name = m.newCategory.Name[:len(m.newCategory.Name)-1]
		}
	case createCategoryDisplayName:
		if len(m.newCategory.DisplayName) > 0 {
			m.newCategory.DisplayName = m.newCategory.DisplayName[:len(m.newCategory.DisplayName)-1]
		}
	}
	return m, nil
}

func (m model) handleSaveCategory() (tea.Model, tea.Cmd) {
	// Validate category name is not empty
	if strings.TrimSpace(m.newCategory.Name) == "" {
		m.categoryMessage = "Category name cannot be empty"
		return m, nil
	}

	// Validate display name is not empty
	if strings.TrimSpace(m.newCategory.DisplayName) == "" {
		m.categoryMessage = "Display name cannot be empty"
		return m, nil
	}

	// Add new category
	err := m.store.AddCategory(m.newCategory.Name, m.newCategory.DisplayName)
	if err != nil {
		m.categoryMessage = fmt.Sprintf("Error: %v", err)
		return m, nil
	}

	// Return to category view
	m.categoryMessage = fmt.Sprintf("Created category: %s", m.newCategory.DisplayName)
	m.state = categoryView
	return m, nil
}

// Split Transaction Mode --------------------
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

	m.splitCategory1 = m.currTransaction.Category
	m.splitCategory2 = m.currTransaction.Category

	return m, nil
}

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

func (m model) handleSplitFieldNavigation(direction int) (tea.Model, tea.Cmd) {
	if direction > 0 && m.splitField < splitCategory2Field {
		m.splitField++
	} else if direction < 0 && m.splitField > splitAmount1Field {
		m.splitField--
	}
	return m, nil
}

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

func (m model) enterSplitDesc1Editing() (tea.Model, tea.Cmd) {
	m.isSplitEditingDesc1 = true
	m.splitEditingDesc1 = m.splitDesc1
	return m, nil
}

func (m model) enterSplitDesc2Editing() (tea.Model, tea.Cmd) {
	m.isSplitEditingDesc2 = true
	m.splitEditingDesc2 = m.splitDesc2
	return m, nil
}

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

func (m model) handleSplitDescInput(key string) (tea.Model, tea.Cmd) {
	if m.isSplitEditingDesc1 {
		m.splitEditingDesc1 += key
	} else if m.isSplitEditingDesc2 {
		m.splitEditingDesc2 += key
	}
	return m, nil
}

func (m model) handleSplitDescBackspace() (tea.Model, tea.Cmd) {
	if m.isSplitEditingDesc1 && len(m.splitEditingDesc1) > 0 {
		m.splitEditingDesc1 = m.splitEditingDesc1[:len(m.splitEditingDesc1)-1]
	} else if m.isSplitEditingDesc2 && len(m.splitEditingDesc2) > 0 {
		m.splitEditingDesc2 = m.splitEditingDesc2[:len(m.splitEditingDesc2)-1]
	}
	return m, nil
}

func (m model) exitSplitDescEditing() (tea.Model, tea.Cmd) {
	if m.isSplitEditingDesc1 {
		m.splitDesc1 = m.splitEditingDesc1
		m.isSplitEditingDesc1 = false
		m.splitEditingDesc1 = ""
	}
	if m.isSplitEditingDesc2 {
		m.splitDesc2 = m.splitEditingDesc2
		m.isSplitEditingDesc2 = false
		m.splitEditingDesc2 = ""
	}
	return m, nil
}

func (m model) handleSplitAmountBackspace() (tea.Model, tea.Cmd) {
	if m.isSplitEditingAmount1 && len(m.splitEditingAmount1) > 0 {
		m.splitEditingAmount1 = m.splitEditingAmount1[:len(m.splitEditingAmount1)-1]
	} else if m.isSplitEditingAmount2 && len(m.splitEditingAmount2) > 0 {
		m.splitEditingAmount2 = m.splitEditingAmount2[:len(m.splitEditingAmount2)-1]
	}
	return m, nil
}

func (m model) enterSplitCategory1Selection() (tea.Model, tea.Cmd) {
	m.isSplitSelectingCategory1 = true
	m.splitCat1SelectIndex = 0

	// Find current category in list
	for i, cat := range m.store.categories.Categories {
		if cat.Name == m.splitCategory1 {
			m.splitCat1SelectIndex = i
			break
		}
	}
	return m, nil
}

func (m model) enterSplitCategory2Selection() (tea.Model, tea.Cmd) {
	m.isSplitSelectingCategory2 = true
	m.splitCat2SelectIndex = 0

	// Find current category in list
	for i, cat := range m.store.categories.Categories {
		if cat.Name == m.splitCategory2 {
			m.splitCat2SelectIndex = i
			break
		}
	}
	return m, nil
}

func (m model) handleSplitCategorySelection(key string) (tea.Model, tea.Cmd) {
	categories := m.store.categories.Categories

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
				m.splitCategory1 = selectedCategory.Name
				m.isSplitSelectingCategory1 = false
			} else {
				m.splitCategory2 = selectedCategory.Name
				m.isSplitSelectingCategory2 = false
			}
		}
	case "esc":
		// Exit category selection without saving changes
		m.isSplitSelectingCategory1 = false
		m.isSplitSelectingCategory2 = false
	}
	return m, nil
}

func (m model) enterSplitAmount1Editing() (tea.Model, tea.Cmd) {
	m.isSplitEditingAmount1 = true
	m.splitEditingAmount1 = m.splitAmount1
	return m, nil
}

func (m model) enterSplitAmount2Editing() (tea.Model, tea.Cmd) {
	m.isSplitEditingAmount2 = true
	m.splitEditingAmount2 = m.splitAmount2
	return m, nil
}

func (m model) enterSplitAmount1EditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isSplitEditingAmount1 = true
	m.splitEditingAmount1 = m.splitAmount1
	// Apply backspace immediately
	if len(m.splitEditingAmount1) > 0 {
		m.splitEditingAmount1 = m.splitEditingAmount1[:len(m.splitEditingAmount1)-1]
	}
	return m, nil
}

func (m model) enterSplitAmount2EditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isSplitEditingAmount2 = true
	m.splitEditingAmount2 = m.splitAmount2
	// Apply backspace immediately
	if len(m.splitEditingAmount2) > 0 {
		m.splitEditingAmount2 = m.splitEditingAmount2[:len(m.splitEditingAmount2)-1]
	}
	return m, nil
}

func (m model) enterSplitDesc1EditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isSplitEditingDesc1 = true
	m.splitEditingDesc1 = m.splitDesc1
	// Apply backspace immediately
	if len(m.splitEditingDesc1) > 0 {
		m.splitEditingDesc1 = m.splitEditingDesc1[:len(m.splitEditingDesc1)-1]
	}
	return m, nil
}

func (m model) enterSplitDesc2EditingWithBackspace() (tea.Model, tea.Cmd) {
	m.isSplitEditingDesc2 = true
	m.splitEditingDesc2 = m.splitDesc2
	// Apply backspace immediately
	if len(m.splitEditingDesc2) > 0 {
		m.splitEditingDesc2 = m.splitEditingDesc2[:len(m.splitEditingDesc2)-1]
	}
	return m, nil
}

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

func (m model) exitSplitAmountEditing() (tea.Model, tea.Cmd) {
	if m.isSplitEditingAmount1 {
		m.splitAmount1 = m.splitEditingAmount1
		m.isSplitEditingAmount1 = false
		m.splitEditingAmount1 = ""
	}
	if m.isSplitEditingAmount2 {
		m.splitAmount2 = m.splitEditingAmount2
		m.isSplitEditingAmount2 = false
		m.splitEditingAmount2 = ""
	}
	return m, nil
}

func (m model) handleSaveSplit() (tea.Model, tea.Cmd) {
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
	split1 := Transaction{
		Amount:          amount1,
		Description:     m.splitDesc1,
		Date:            m.currTransaction.Date,
		Category:        m.splitCategory1,
		TransactionType: m.currTransaction.TransactionType,
	}

	split2 := Transaction{
		Amount:          amount2,
		Description:     m.splitDesc2,
		Date:            m.currTransaction.Date,
		Category:        m.splitCategory2,
		TransactionType: m.currTransaction.TransactionType,
	}

	// Save split using store method
	err := m.store.SplitTransaction(m.currTransaction.Id, []Transaction{split1, split2})
	if err != nil {
		m.splitMessage = fmt.Sprintf("Error saving split: %v", err)
		return m, nil
	}

	// Refresh transactions and exit
	m.transactions, _ = m.store.GetTransactions()
	m.state = listView
	return m.exitSplitMode()
}

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

// Bulk Edit View --------------------
func (m model) handleBulkEditView(key string) (tea.Model, tea.Cmd) {
	// Handle active editing states
	if m.isBulkEditingAmount || m.isBulkEditingDescription || m.isBulkEditingDate {
		return m.handleBulkTextEditing(key)
	}
	if m.isBulkSelectingCategory || m.isBulkSelectingType {
		return m.handleBulkDropdownSelection(key)
	}

	switch key {
	case "esc":
		m.state = listView
	case "down", "tab":
		if m.bulkEditField < bulkEditType {
			m.bulkEditField++
		}
	case "up":
		if m.bulkEditField > bulkEditAmount {
			m.bulkEditField--
		}
	case "enter", "backspace":
		return m.handleBulkFieldActivation(key)
	case "ctrl+s":
		return m.handleSaveBulkEdit()
	default:
		if len(key) == 1 {
			return m.handleBulkImmediateInput(key)
		}
	}
	return m, nil
}

func (m model) handleBulkFieldActivation(key string) (tea.Model, tea.Cmd) {
	switch m.bulkEditField {
	case bulkEditAmount:
		return m.enterBulkAmountEditing(key == "backspace")
	case bulkEditDescription:
		return m.enterBulkDescriptionEditing(key == "backspace")
	case bulkEditDate:
		return m.enterBulkDateEditing(key == "backspace")
	case bulkEditCategory:
		m.isBulkSelectingCategory = true
		m.bulkCategorySelectIndex = 0
	case bulkEditType:
		m.isBulkSelectingType = true
		m.bulkTypeSelectIndex = 0
	}
	return m, nil
}

func (m model) handleBulkImmediateInput(key string) (tea.Model, tea.Cmd) {
	switch m.bulkEditField {
	case bulkEditAmount:
		if m.isValidAmountChar(key) {
			return m.enterBulkAmountEditing(false)
		}
	case bulkEditDescription, bulkEditDate:
		return m.enterBulkTextEditingWithChar(key)
	}
	return m, nil
}

// Update the existing handleBulkCategorySelection to properly set placeholder state
func (m model) handleBulkCategorySelection(key string) (tea.Model, tea.Cmd) {
	categories := m.store.categories.Categories

	switch key {
	case "esc":
		m.isBulkSelectingCategory = false
	case "up":
		if m.bulkCategorySelectIndex > 0 {
			m.bulkCategorySelectIndex--
		}
	case "down":
		if m.bulkCategorySelectIndex < len(categories)-1 {
			m.bulkCategorySelectIndex++
		}
	case "enter":
		if len(categories) > 0 {
			selectedCategory := categories[m.bulkCategorySelectIndex]
			m.bulkCategoryValue = selectedCategory.Name
			m.bulkCategoryIsPlaceholder = false // Clear placeholder state
		}
		m.isBulkSelectingCategory = false
	}
	return m, nil
}

// Update the existing handleBulkTypeSelection to properly set placeholder state
func (m model) handleBulkTypeSelection(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.isBulkSelectingType = false
	case "up":
		if m.bulkTypeSelectIndex > 0 {
			m.bulkTypeSelectIndex--
		}
	case "down":
		if m.bulkTypeSelectIndex < len(m.availableTypes)-1 {
			m.bulkTypeSelectIndex++
		}
	case "enter":
		selectedType := m.availableTypes[m.bulkTypeSelectIndex]
		m.bulkTypeValue = selectedType
		m.bulkTypeIsPlaceholder = false // Clear placeholder state
		m.isBulkSelectingType = false
	}
	return m, nil
}

func (m model) handleSaveBulkEdit() (tea.Model, tea.Cmd) {
	// Update all selected transactions
	for i := range m.transactions {
		if m.selectedTxIds[m.transactions[i].Id] {
			// Apply amount if modified
			if !m.bulkAmountIsPlaceholder && strings.TrimSpace(m.bulkAmountValue) != "" {
				if amount, err := strconv.ParseFloat(m.bulkAmountValue, 64); err == nil {
					m.transactions[i].Amount = amount
				}
			}

			// Apply description if modified
			if !m.bulkDescriptionIsPlaceholder && strings.TrimSpace(m.bulkDescriptionValue) != "" {
				m.transactions[i].Description = m.bulkDescriptionValue
			}

			// Apply date if modified
			if !m.bulkDateIsPlaceholder && strings.TrimSpace(m.bulkDateValue) != "" {
				m.transactions[i].Date = m.bulkDateValue
			}

			// Apply category if modified
			if !m.bulkCategoryIsPlaceholder && strings.TrimSpace(m.bulkCategoryValue) != "" {
				m.transactions[i].Category = m.bulkCategoryValue
			}

			// Apply type if modified
			if !m.bulkTypeIsPlaceholder && strings.TrimSpace(m.bulkTypeValue) != "" {
				m.transactions[i].TransactionType = m.bulkTypeValue
			}

			m.store.SaveTransaction(m.transactions[i])
		}
	}

	m.transactions, _ = m.store.GetTransactions()
	m.state = listView
	return m.exitMultiSelectMode()
}

func (m *model) resetBulkEditValues() {
	m.bulkAmountValue = ""
	m.bulkDescriptionValue = ""
	m.bulkDateValue = ""
	m.bulkCategoryValue = ""
	m.bulkTypeValue = ""

	m.bulkAmountIsPlaceholder = true
	m.bulkDescriptionIsPlaceholder = true
	m.bulkDateIsPlaceholder = true
	m.bulkCategoryIsPlaceholder = true
	m.bulkTypeIsPlaceholder = true
}

func (m model) enterCategorySelection() (tea.Model, tea.Cmd) {
	m.isSelectingCategory = true
	m.categorySelectIndex = 0

	// Find current category in list for initial position
	for i, cat := range m.store.categories.Categories {
		if cat.Name == m.currTransaction.Category {
			m.categorySelectIndex = i
			break
		}
	}
	return m, nil
}

func (m model) handleCategorySelection(key string) (tea.Model, tea.Cmd) {
	categories := m.store.categories.Categories

	switch key {
	case "up":
		if m.categorySelectIndex > 0 {
			m.categorySelectIndex--
		}
	case "down":
		if m.categorySelectIndex < len(categories)-1 {
			m.categorySelectIndex++
		}
	case "enter":
		if len(categories) > 0 {
			m.currTransaction.Category = categories[m.categorySelectIndex].Name
		}
		m.isSelectingCategory = false
	case "esc":
		m.isSelectingCategory = false
	}
	return m, nil
}

func (m model) enterTypeSelection() (tea.Model, tea.Cmd) {
	m.isSelectingType = true
	m.typeSelectIndex = 0

	// Find current type in list
	for i, t := range m.availableTypes {
		if t == m.currTransaction.TransactionType {
			m.typeSelectIndex = i
			break
		}
	}
	return m, nil
}

func (m model) handleTypeSelection(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up":
		if m.typeSelectIndex > 0 {
			m.typeSelectIndex--
		}
	case "down":
		if m.typeSelectIndex < len(m.availableTypes)-1 {
			m.typeSelectIndex++
		}
	case "enter":
		m.currTransaction.TransactionType = m.availableTypes[m.typeSelectIndex]
		m.isSelectingType = false
	case "esc":
		m.isSelectingType = false
	}
	return m, nil
}

// Bank Statement View
func (m model) handleBankStatementView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.state = menuView
	case "t":
		m.state = csvTemplateView
	case "f":
		// Open file picker
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "."
		}
		m.currentDir = homeDir
		m.fileIndex = 0
		m.selectedFile = ""

		err = m.loadDirectoryEntries()
		if err != nil {
			m.statementMessage = fmt.Sprintf("Error opening directory: %v", err)
			return m, nil
		}

		if len(m.dirEntries) == 0 {
			m.statementMessage = "No directories or CSV files found"
		}

		m.state = filePickerView
	case "h":
		m.statementIndex = 0
		m.state = statementHistoryView
	}
	return m, nil
}

// Statement History View
func (m model) handleStatementHistoryView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.state = bankStatementView
	case "up":
		if m.statementIndex > 0 {
			m.statementIndex--
		}
	case "down":
		if len(m.store.statements.Statements) > 0 && m.statementIndex < len(m.store.statements.Statements)-1 {
			m.statementIndex++
		}
	case "enter":
		if len(m.store.statements.Statements) > 0 && m.statementIndex < len(m.store.statements.Statements) {
			selectedStatement := m.store.statements.Statements[m.statementIndex]
			m.statementMessage = fmt.Sprintf("Statement: %s | Period: %s to %s | Transactions: %d | Template: %s | Status: %s",
				selectedStatement.Filename, selectedStatement.PeriodStart, selectedStatement.PeriodEnd,
				selectedStatement.TxCount, selectedStatement.TemplateUsed, selectedStatement.Status)
			m.state = bankStatementView
		}
	}
	return m, nil
}

// Statement Overlap View
func (m model) handleStatementOverlapView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "n":
		m.state = bankStatementView
		m.overlappingStmts = nil
	case "y":
		// Import anyway with override status
		return m.handleOverrideImport()
	}
	return m, nil
}

func (m model) handleOverrideImport() (tea.Model, tea.Cmd) {
	// Get the template to use
	templateToUse := m.store.csvTemplates.Default
	if templateToUse == "" && len(m.store.csvTemplates.Templates) > 0 {
		templateToUse = m.store.csvTemplates.Templates[0].Name
	}

	// Parse and import transactions with override
	template := m.store.getTemplateByName(templateToUse)
	if template == nil {
		m.statementMessage = fmt.Sprintf("Error: template '%s' not found", templateToUse)
		m.state = bankStatementView
		return m, nil
	}

	data, err := os.ReadFile(m.store.importName)
	if err != nil {
		m.statementMessage = fmt.Sprintf("Error reading file: %v", err)
		m.state = bankStatementView
		return m, nil
	}

	lines := strings.Split(string(data), "\n")
	var importedTransactions []Transaction
	startLine := 0
	if template.HasHeader {
		startLine = 1
	}

	currentCount := len(m.transactions)

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
		// Set timestamps for imported transactions (matching main import function)
		now := time.Now().Format(time.RFC3339)
		transaction.CreatedAt = now
		transaction.UpdatedAt = now
		transaction.Confidence = 0.0
		importedTransactions = append(importedTransactions, transaction)
	}

	if len(importedTransactions) == 0 {
		m.statementMessage = "No valid transactions found in CSV"
		m.state = bankStatementView
		return m, nil
	}

	// Add transactions to store
	m.store.transactions = append(m.store.transactions, importedTransactions...)

	// Record import with override status
	periodStart, periodEnd := m.store.extractPeriodFromTransactions(importedTransactions)
	filename := filepath.Base(m.store.importName)
	err = m.store.recordBankStatement(filename, periodStart, periodEnd, templateToUse, len(importedTransactions), "override")
	if err != nil {
		log.Printf("Failed to record statement: %v", err)
	}

	// Save and refresh
	err = m.store.saveTransactions()
	if err != nil {
		m.statementMessage = fmt.Sprintf("Error saving transactions: %v", err)
	} else {
		m.transactions, _ = m.store.GetTransactions()
		imported := len(m.transactions) - currentCount
		m.statementMessage = fmt.Sprintf("Successfully imported %d transactions (with overlap override) from %s using template %s",
			imported, filepath.Base(m.selectedFile), templateToUse)
	}

	m.overlappingStmts = nil
	m.state = bankStatementView
	return m, nil
}

// bulk editing

func (m model) handleBulkTextEditing(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		// Exit editing mode and save current value
		return m.exitBulkTextEditing()
	case "backspace":
		return m.handleBulkTextBackspace()
	default:
		if len(key) == 1 {
			return m.handleBulkTextInput(key)
		}
	}
	return m, nil
}

func (m model) exitBulkTextEditing() (tea.Model, tea.Cmd) {
	// Exit all text editing modes
	m.isBulkEditingAmount = false
	m.isBulkEditingDescription = false
	m.isBulkEditingDate = false
	return m, nil
}

func (m model) handleBulkTextBackspace() (tea.Model, tea.Cmd) {
	switch {
	case m.isBulkEditingAmount:
		if len(m.bulkAmountValue) > 0 {
			m.bulkAmountValue = m.bulkAmountValue[:len(m.bulkAmountValue)-1]
			m.bulkAmountIsPlaceholder = len(m.bulkAmountValue) == 0
		}
	case m.isBulkEditingDescription:
		if len(m.bulkDescriptionValue) > 0 {
			m.bulkDescriptionValue = m.bulkDescriptionValue[:len(m.bulkDescriptionValue)-1]
			m.bulkDescriptionIsPlaceholder = len(m.bulkDescriptionValue) == 0
		}
	case m.isBulkEditingDate:
		if len(m.bulkDateValue) > 0 {
			m.bulkDateValue = m.bulkDateValue[:len(m.bulkDateValue)-1]
			m.bulkDateIsPlaceholder = len(m.bulkDateValue) == 0
		}
	}
	return m, nil
}

func (m model) handleBulkTextInput(key string) (tea.Model, tea.Cmd) {
	switch {
	case m.isBulkEditingAmount:
		return m.handleBulkAmountInput(key)
	case m.isBulkEditingDescription:
		m.bulkDescriptionValue += key
		m.bulkDescriptionIsPlaceholder = false
	case m.isBulkEditingDate:
		m.bulkDateValue += key
		m.bulkDateIsPlaceholder = false
	}
	return m, nil
}

func (m model) handleBulkAmountInput(key string) (tea.Model, tea.Cmd) {
	// Handle negative sign
	if key == "-" {
		if len(m.bulkAmountValue) == 0 {
			m.bulkAmountValue = "-"
			m.bulkAmountIsPlaceholder = false
		}
		return m, nil
	}

	// Only allow digits and decimal point
	if (key >= "0" && key <= "9") || key == "." {
		// Don't allow multiple decimal points
		if key == "." && strings.Contains(m.bulkAmountValue, ".") {
			return m, nil
		}

		newStr := m.bulkAmountValue + key

		// Validate decimal places (max 2)
		dotIndex := strings.LastIndex(newStr, ".")
		if dotIndex != -1 && len(newStr)-dotIndex-1 > 2 {
			return m, nil
		}

		// Validate it's a valid number format
		if _, err := strconv.ParseFloat(newStr, 64); err == nil || newStr == "." || newStr == "-." {
			m.bulkAmountValue = newStr
			m.bulkAmountIsPlaceholder = false
		}
	}
	return m, nil
}

func (m model) enterBulkAmountEditing(withBackspace bool) (tea.Model, tea.Cmd) {
	m.isBulkEditingAmount = true
	m.bulkAmountIsPlaceholder = false

	// Clear placeholder and start fresh
	if m.bulkAmountIsPlaceholder {
		m.bulkAmountValue = ""
	}

	// Apply backspace immediately if activated with backspace
	if withBackspace && len(m.bulkAmountValue) > 0 {
		m.bulkAmountValue = m.bulkAmountValue[:len(m.bulkAmountValue)-1]
	}

	return m, nil
}

func (m model) enterBulkDescriptionEditing(withBackspace bool) (tea.Model, tea.Cmd) {
	m.isBulkEditingDescription = true
	m.bulkDescriptionIsPlaceholder = false

	// Clear placeholder and start fresh
	if m.bulkDescriptionIsPlaceholder {
		m.bulkDescriptionValue = ""
	}

	// Apply backspace immediately if activated with backspace
	if withBackspace && len(m.bulkDescriptionValue) > 0 {
		m.bulkDescriptionValue = m.bulkDescriptionValue[:len(m.bulkDescriptionValue)-1]
	}

	return m, nil
}

func (m model) enterBulkDateEditing(withBackspace bool) (tea.Model, tea.Cmd) {
	m.isBulkEditingDate = true
	m.bulkDateIsPlaceholder = false

	// Clear placeholder and start fresh
	if m.bulkDateIsPlaceholder {
		m.bulkDateValue = ""
	}

	// Apply backspace immediately if activated with backspace
	if withBackspace && len(m.bulkDateValue) > 0 {
		m.bulkDateValue = m.bulkDateValue[:len(m.bulkDateValue)-1]
	}

	return m, nil
}

func (m model) enterBulkTextEditingWithChar(key string) (tea.Model, tea.Cmd) {
	switch m.bulkEditField {
	case bulkEditDescription:
		m.isBulkEditingDescription = true
		m.bulkDescriptionValue = key
		m.bulkDescriptionIsPlaceholder = false
	case bulkEditDate:
		m.isBulkEditingDate = true
		m.bulkDateValue = key
		m.bulkDateIsPlaceholder = false
	case bulkEditAmount:
		if m.isValidAmountChar(key) {
			m.isBulkEditingAmount = true
			m.bulkAmountValue = key
			m.bulkAmountIsPlaceholder = false
		}
	}
	return m, nil
}

func (m model) isValidAmountChar(key string) bool {
	return (key >= "0" && key <= "9") || key == "." || key == "-"
}

func (m model) handleBulkDropdownSelection(key string) (tea.Model, tea.Cmd) {
	if m.isBulkSelectingCategory {
		return m.handleBulkCategorySelection(key)
	}
	if m.isBulkSelectingType {
		return m.handleBulkTypeSelection(key)
	}
	return m, nil
}
