package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	menuView           uint = iota
	listView                = 1
	titleView               = 2
	bodyView                = 3
	editView                = 4
	backupView              = 5
	importView              = 6
	filePickerView          = 7
	csvTemplateView         = 8
	createTemplateView      = 9
	categoryView            = 10
	createCategoryView      = 11
	bulkEditView            = 12
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
)

const (
	bulkEditCategory uint = iota
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

	// Multi-select mode
	isMultiSelectMode       bool
	selectedTxIds           map[int64]bool
	bulkEditField           uint
	bulkEditValue           string
	isBulkSelectingCategory bool
	isBulkSelectingType     bool
	bulkCategorySelectIndex int
	bulkTypeSelectIndex     int
	bulkCategoryValue       string
	bulkTypeValue           string

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
		case importView:
			return m.handleImportView(key)
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
		m.state = importView
		m.importMessage = ""
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
		if len(m.transactions) > 0 {
			m.currTransaction = m.transactions[m.listIndex]
			m.editField = editAmount
			m.editAmountStr = ""
			m.state = editView
		}
	case "m":
		// Toggle multi-select mode
		if file, err := tea.LogToFile("debug.log", "debug"); err == nil {
			defer file.Close()
			log.Printf("M key pressed! Current mode: %v", m.isMultiSelectMode)
		}
		return m.handleMultiSelectToggle()
	case "b":
		if m.isMultiSelectMode && len(m.selectedTxIds) > 0 {
			// Enter bulk edit mode
			m.bulkEditField = bulkEditCategory
			m.bulkEditValue = ""
			m.state = bulkEditView
		}
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
			// ...existing bounds checking...
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
		// Save amount and exit editing mode
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
	// Validate and save amount from edit string
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

// Import View

func (m model) handleImportView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.state = menuView
	case "i":
		// Open file picker
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "."
		}
		m.currentDir = homeDir
		m.fileIndex = 0
		m.selectedFile = ""

		// Load directory entries with error handling
		err = m.loadDirectoryEntries()
		if err != nil {
			m.importMessage = fmt.Sprintf("Error opening directory: %v", err)
			return m, nil
		}

		// Debug: Check if we found any entries
		if len(m.dirEntries) == 0 {
			m.importMessage = "No directories or CSV files found"
		}

		m.state = filePickerView
	case "p":
		m.state = csvTemplateView
	}
	return m, nil
}

// File Picker View

func (m model) handleFilePickerView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.state = importView
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
					// Go up one directory
					m.currentDir = filepath.Dir(m.currentDir)
				} else {
					// Enter directory
					m.currentDir = fullPath
				}
				m.fileIndex = 0
				m.loadDirectoryEntries()
			} else if strings.HasSuffix(strings.ToLower(selected), ".csv") {
				// CSV file selected - set import path and import
				m.store.importName = fullPath

				currentCount := len(m.transactions)
				// Use the selected template instead of hardcoded templateName
				templateToUse := m.store.csvTemplates.Default
				if templateToUse == "" && len(m.store.csvTemplates.Templates) > 0 {
					templateToUse = m.store.csvTemplates.Templates[0].Name
				}

				err := m.store.ImportTransactionsFromCSV(templateToUse)
				if err != nil {
					m.importMessage = fmt.Sprintf("Error: %v", err)
				} else {
					m.transactions, _ = m.store.GetTransactions()
					imported := len(m.transactions) - currentCount
					m.importMessage = fmt.Sprintf("Successfully imported %d transactions from %s using template %s",
						imported, filepath.Base(selected), templateToUse)
				}
				m.state = importView
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
		m.state = importView
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

			m.state = importView
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

	// Initialize split data with defaults - preserve sign
	halfAmount := m.currTransaction.Amount / 2
	m.splitAmount1 = fmt.Sprintf("%.2f", halfAmount)
	m.splitAmount2 = fmt.Sprintf("%.2f", halfAmount)
	m.splitDesc1 = m.currTransaction.Description
	m.splitDesc2 = m.currTransaction.Description
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

func (m model) handleSplitBackspace() (tea.Model, tea.Cmd) {
	switch m.splitField {
	case splitAmount1Field:
		if len(m.splitAmount1) > 0 {
			m.splitAmount1 = m.splitAmount1[:len(m.splitAmount1)-1]
		}
	case splitAmount2Field:
		if len(m.splitAmount2) > 0 {
			m.splitAmount2 = m.splitAmount2[:len(m.splitAmount2)-1]
		}
	case splitDesc1Field:
		if len(m.splitDesc1) > 0 {
			m.splitDesc1 = m.splitDesc1[:len(m.splitDesc1)-1]
		}
	case splitDesc2Field:
		if len(m.splitDesc2) > 0 {
			m.splitDesc2 = m.splitDesc2[:len(m.splitDesc2)-1]
		}
	case splitCategory1Field:
		if len(m.splitCategory1) > 0 {
			m.splitCategory1 = m.splitCategory1[:len(m.splitCategory1)-1]
		}
	case splitCategory2Field:
		if len(m.splitCategory2) > 0 {
			m.splitCategory2 = m.splitCategory2[:len(m.splitCategory2)-1]
		}
	}
	return m, nil
}

// Bulk Edit View --------------------
func (m model) handleBulkEditView(key string) (tea.Model, tea.Cmd) {
	// Handle active selection states first
	if m.isBulkSelectingCategory {
		return m.handleBulkCategorySelection(key)
	}
	if m.isBulkSelectingType {
		return m.handleBulkTypeSelection(key)
	}

	switch key {
	case "esc":
		m.state = listView
	case "down", "tab":
		if m.bulkEditField < bulkEditType {
			m.bulkEditField++
		}
	case "up":
		if m.bulkEditField > bulkEditCategory {
			m.bulkEditField--
		}
	case "enter":
		switch m.bulkEditField {
		case bulkEditCategory:
			m.isBulkSelectingCategory = true
			m.bulkCategorySelectIndex = 0
		case bulkEditType:
			m.isBulkSelectingType = true
			m.bulkTypeSelectIndex = 0
		default:
			return m.handleSaveBulkEdit()
		}
	case "ctrl+s": // Add save functionality
		return m.handleSaveBulkEdit()
	}
	return m, nil
}

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
		}
		m.isBulkSelectingCategory = false
	}
	return m, nil
}

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
		m.isBulkSelectingType = false
	}
	return m, nil
}

func (m model) handleSaveBulkEdit() (tea.Model, tea.Cmd) {
	// Update all selected transactions
	for i := range m.transactions {
		if m.selectedTxIds[m.transactions[i].Id] {
			// Apply category if one was selected
			if strings.TrimSpace(m.bulkCategoryValue) != "" {
				m.transactions[i].Category = m.bulkCategoryValue
			}

			// Apply type if one was selected
			if strings.TrimSpace(m.bulkTypeValue) != "" {
				m.transactions[i].TransactionType = m.bulkTypeValue
			}

			// Save each transaction
			m.store.SaveTransaction(m.transactions[i])
		}
	}

	// Refresh and exit
	m.transactions, _ = m.store.GetTransactions()
	m.state = listView
	return m.exitMultiSelectMode()
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
