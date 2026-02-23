package ui

import (
	"budget-tracker-tui/internal/storage"
	"budget-tracker-tui/internal/validation"
	"fmt"
	"log"

	"budget-tracker-tui/internal/types"

	tea "github.com/charmbracelet/bubbletea"
)

// Main application model
type model struct {
	// Core state
	state           uint
	store           *storage.Store
	transactions    []types.Transaction
	currTransaction types.Transaction
	listIndex       int
	windowHeight    int

	// Edit transaction fields
	editField     uint
	editAmountStr string

	// Field editing state
	isEditingAmount      bool
	isEditingDescription bool
	isEditingDate        bool
	editingAmountStr     string
	editingDescStr       string
	editingDateStr       string

	// Selection mode fields
	isSelectingCategory bool
	isSelectingType     bool
	categorySelectIndex int
	typeSelectIndex     int
	availableTypes      []string

	// File explorer
	currentDir   string
	dirEntries   []string
	fileIndex    int
	selectedFile string

	// CSV template management
	templateIndex    int
	selectedTemplate string
	newTemplate      types.CSVTemplate
	createField      uint
	createMessage    string

	// Category management
	categoryIndex       int
	selectedCategory    string
	newCategory         types.Category
	createCategoryField uint
	categoryMessage     string

	// Category creation editing state
	isEditingCategoryName         bool
	isEditingCategoryDisplayName  bool
	editingCategoryNameStr        string
	editingCategoryDisplayNameStr string

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

	// Split editing state flags
	isSplitEditingAmount1   bool
	isSplitEditingAmount2   bool
	isSplitEditingDesc1     bool
	isSplitEditingDesc2     bool
	isSplitEditingCategory1 bool
	isSplitEditingCategory2 bool

	// Split selection modes
	isSplitSelectingCategory1 bool
	isSplitSelectingCategory2 bool
	splitCat1SelectIndex      int
	splitCat2SelectIndex      int

	// Temporary editing values for splits
	splitEditingAmount1   string
	splitEditingAmount2   string
	splitEditingDesc1     string
	splitEditingDesc2     string
	splitEditingCategory1 string
	splitEditingCategory2 string

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

	// Bank statement fields
	statementIndex   int
	overlappingStmts []types.BankStatement
	pendingStatement types.BankStatement
	statementMessage string

	// Messages
	backupMessage string
	importMessage string

	// Validation state
	fieldErrors            map[string]string
	hasValidationErrors    bool
	validationNotification string
	validator              *validation.TransactionValidator
}

// NewModel creates a new model instance
func NewModel(store *storage.Store) model {
	transactions, err := store.GetTransactions()
	if err != nil {
		log.Fatalf("unable to get transactions: %v", err)
	}
	return model{
		state:          menuView,
		store:          store,
		transactions:   transactions,
		listIndex:      0,
		availableTypes: []string{"income", "expense", "transfer"},
		selectedTxIds:  make(map[int64]bool),
		fieldErrors:    make(map[string]string),
		validator:      validation.NewTransactionValidator(),
	}
}

// getCategoryDisplayName returns the display name for a category name, or the name itself if not found
func (m model) getCategoryDisplayName(categoryName string) string {
	categories, err := m.store.GetCategories()
	if err != nil {
		return categoryName // Fallback to category name
	}

	for _, category := range categories {
		if category.Name == categoryName {
			if category.DisplayName != "" {
				return category.DisplayName
			}
			return category.Name // Fallback if DisplayName is empty
		}
	}

	return categoryName // Category not found, return original name
}

// Init initializes the model
func (m model) Init() tea.Cmd {
	return nil
}

// Update handles all state transitions and user input
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

// validateCurrentTransaction validates the current transaction and updates field errors
func (m *model) validateCurrentTransaction() {
	categories, err := m.store.GetCategories()
	if err != nil {
		return
	}

	categoryNames := make([]string, len(categories))
	for i, category := range categories {
		categoryNames[i] = category.Name
	}

	result := m.validator.ValidateTransaction(&m.currTransaction, categoryNames)

	// Clear existing errors
	m.fieldErrors = make(map[string]string)
	m.hasValidationErrors = false
	m.validationNotification = ""

	// Set new errors if any
	if !result.IsValid {
		m.hasValidationErrors = true
		for _, err := range result.Errors {
			m.fieldErrors[err.Field] = err.Message
		}
		m.buildValidationNotification()
	}
}

// validateBulkEditData validates bulk edit values and updates field errors
func (m *model) validateBulkEditData() {
	categories, err := m.store.GetCategories()
	if err != nil {
		return
	}

	categoryNames := make([]string, len(categories))
	for i, category := range categories {
		categoryNames[i] = category.Name
	}

	// Create temporary transaction with bulk edit values
	tempTx := types.Transaction{
		Amount:      0, // Will be parsed from string
		Description: m.bulkDescriptionValue,
		Date:        m.bulkDateValue,
		Category:    m.bulkCategoryValue,
	}

	// Parse amount if not placeholder
	if !m.bulkAmountIsPlaceholder && m.bulkAmountValue != "" {
		if amount, err := m.validator.Amount.ParseAmount(m.bulkAmountValue); err == nil {
			tempTx.Amount = amount
		}
	}

	// Only validate fields that are not placeholders
	m.fieldErrors = make(map[string]string)
	m.hasValidationErrors = false
	m.validationNotification = ""

	if !m.bulkAmountIsPlaceholder {
		if err := m.validator.ValidateField(&tempTx, "amount", categoryNames); err != nil {
			m.fieldErrors["amount"] = err.Error()
			m.hasValidationErrors = true
		}
	}

	if !m.bulkDescriptionIsPlaceholder {
		if err := m.validator.ValidateField(&tempTx, "description", categoryNames); err != nil {
			m.fieldErrors["description"] = err.Error()
			m.hasValidationErrors = true
		}
	}

	if !m.bulkDateIsPlaceholder {
		if err := m.validator.ValidateField(&tempTx, "date", categoryNames); err != nil {
			m.fieldErrors["date"] = err.Error()
			m.hasValidationErrors = true
		}
	}

	if !m.bulkCategoryIsPlaceholder {
		if err := m.validator.ValidateField(&tempTx, "category", categoryNames); err != nil {
			m.fieldErrors["category"] = err.Error()
			m.hasValidationErrors = true
		}
	}

	if m.hasValidationErrors {
		m.buildValidationNotification()
	}
}

// validateSplitTransaction validates split transaction fields and updates field errors
func (m *model) validateSplitTransaction() {
	categories, err := m.store.GetCategories()
	if err != nil {
		return
	}

	categoryNames := make([]string, len(categories))
	for i, category := range categories {
		categoryNames[i] = category.Name
	}

	// Clear existing errors
	m.fieldErrors = make(map[string]string)
	m.hasValidationErrors = false
	m.validationNotification = ""

	// Validate split amount 1
	if amount1, err := m.validator.Amount.ParseAmount(m.splitAmount1); err != nil {
		m.fieldErrors["splitAmount1"] = err.Error()
		m.hasValidationErrors = true
	} else {
		tempTx1 := types.Transaction{Amount: amount1}
		if err := m.validator.ValidateField(&tempTx1, "amount", categoryNames); err != nil {
			m.fieldErrors["splitAmount1"] = err.Error()
			m.hasValidationErrors = true
		}
	}

	// Validate split amount 2
	if amount2, err := m.validator.Amount.ParseAmount(m.splitAmount2); err != nil {
		m.fieldErrors["splitAmount2"] = err.Error()
		m.hasValidationErrors = true
	} else {
		tempTx2 := types.Transaction{Amount: amount2}
		if err := m.validator.ValidateField(&tempTx2, "amount", categoryNames); err != nil {
			m.fieldErrors["splitAmount2"] = err.Error()
			m.hasValidationErrors = true
		}
	}

	// Validate descriptions
	tempTx := types.Transaction{Description: m.splitDesc1}
	if err := m.validator.ValidateField(&tempTx, "description", categoryNames); err != nil {
		m.fieldErrors["splitDesc1"] = err.Error()
		m.hasValidationErrors = true
	}

	tempTx.Description = m.splitDesc2
	if err := m.validator.ValidateField(&tempTx, "description", categoryNames); err != nil {
		m.fieldErrors["splitDesc2"] = err.Error()
		m.hasValidationErrors = true
	}

	// Validate categories
	tempTx.Category = m.splitCategory1
	if err := m.validator.ValidateField(&tempTx, "category", categoryNames); err != nil {
		m.fieldErrors["splitCategory1"] = err.Error()
		m.hasValidationErrors = true
	}

	tempTx.Category = m.splitCategory2
	if err := m.validator.ValidateField(&tempTx, "category", categoryNames); err != nil {
		m.fieldErrors["splitCategory2"] = err.Error()
		m.hasValidationErrors = true
	}

	if m.hasValidationErrors {
		m.buildValidationNotification()
	}
}

// buildValidationNotification builds a user-friendly validation notification message
func (m *model) buildValidationNotification() {
	if len(m.fieldErrors) == 0 {
		m.validationNotification = ""
		return
	}

	if len(m.fieldErrors) == 1 {
		for field, message := range m.fieldErrors {
			m.validationNotification = "⚠ " + m.getFieldDisplayName(field) + ": " + message
			return
		}
	}

	// Multiple errors
	m.validationNotification = fmt.Sprintf("⚠ %d fields need correction", len(m.fieldErrors))
}

// getFieldDisplayName returns user-friendly field names
func (m *model) getFieldDisplayName(field string) string {
	displayNames := map[string]string{
		"amount":         "Amount",
		"description":    "Description",
		"date":           "Date",
		"category":       "Category",
		"splitAmount1":   "Split Amount 1",
		"splitAmount2":   "Split Amount 2",
		"splitDesc1":     "Split Description 1",
		"splitDesc2":     "Split Description 2",
		"splitCategory1": "Split Category 1",
		"splitCategory2": "Split Category 2",
	}

	if displayName, exists := displayNames[field]; exists {
		return displayName
	}
	return field
}

// clearFieldError clears error for a specific field
func (m *model) clearFieldError(field string) {
	delete(m.fieldErrors, field)
	m.hasValidationErrors = len(m.fieldErrors) > 0
	if m.hasValidationErrors {
		m.buildValidationNotification()
	} else {
		m.validationNotification = ""
	}
}
