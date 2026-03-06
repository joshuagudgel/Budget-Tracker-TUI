package ui

import (
	"budget-tracker-tui/internal/storage"
	"budget-tracker-tui/internal/validation"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"budget-tracker-tui/internal/types"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/table"
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

	// Template creation editing state
	isEditingTemplateName      bool
	isEditingTemplatePostDate  bool
	isEditingTemplateAmount    bool
	isEditingTemplateDesc      bool
	isEditingTemplateCategory  bool
	editingTemplateNameStr     string
	editingTemplatePostDateStr string
	editingTemplateAmountStr   string
	editingTemplateDescStr     string
	editingTemplateCategoryStr string

	// Template validation state
	templateFieldErrors            map[string]string
	templateValidationErrors       bool
	templateValidationNotification string

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

	// Phase 3: Enhanced Category Management State
	categories           []types.Category                        // All available categories
	selectedCategoryIdx  int                                     // Currently selected category in list
	editingCategory      types.Category                          // Category being edited/created
	categoryActiveField  int                                     // Currently active field index
	categoryEditingField bool                                    // Whether currently editing a field
	categoryFieldErrors  map[string]string                       // Field-specific validation errors
	categoryFields       []string                                // Field names for category editing
	categoryFieldValues  map[int]string                          // Current field values indexed by field constant
	categoryEditingStr   string                                  // Current editing text for active field
	validator            *validation.TransactionValidator        // Transaction validator instance
	categoryValidator    *validation.CategoryManagementValidator // Category validator instance

	// Category selection and hierarchy
	selectedParentId  *int64           // Selected parent category ID
	selectedParentIdx int              // Index for parent category selection
	isSelectingParent bool             // Whether in parent selection mode
	availableParents  []types.Category // Categories available for parent selection

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

	// Current import document details (for overlap warnings)
	currentImportFilename    string
	currentImportPeriodStart string
	currentImportPeriodEnd   string

	// Enhanced bank statement management
	bankStatementListIndex   int
	selectedBankStatementId  int64
	bankStatementListMessage string
	isInBankStatementActions bool
	bankStatementActionIndex int

	// Statement transaction management
	filteredTransactions []types.Transaction
	currentStatementId   int64
	statementTxMessage   string
	filteredListIndex    int
	previousState        uint // Track where to return after edit/bulk operations

	// Messages
	backupMessage string
	importMessage string

	// Validation errors
	validationErrors []types.ValidationError

	// Undo import functionality
	undoStatementId   int64
	undoStatementName string
	undoTxCount       int
	undoMessage       string

	// Validation state
	fieldErrors            map[string]string
	hasValidationErrors    bool
	validationNotification string

	// Analytics state
	analyticsTable          table.Model
	analyticsStartDate      time.Time
	analyticsEndDate        time.Time
	analyticsSummary        *types.AnalyticsSummary
	categorySpending        []types.CategorySpending
	analyticsMessage        string
	isEditingStartDate      bool
	isEditingEndDate        bool
	editingStartDateStr     string
	editingEndDateStr       string
	analyticsDateField      int // 0 for start date, 1 for end date
}

// sortTransactionsByDate sorts transactions by date in descending order (newest first)
func (m *model) sortTransactionsByDate() {
	sort.Slice(m.transactions, func(i, j int) bool {
		// Compare dates directly since they're now time.Time
		dateI := m.transactions[i].Date
		dateJ := m.transactions[j].Date

		// Handle zero times (treat as very old dates for sorting)
		if dateI.IsZero() && dateJ.IsZero() {
			return false // Equal
		}
		if dateI.IsZero() {
			return false // i is older
		}
		if dateJ.IsZero() {
			return true // j is older
		}

		// Return true if i's date is after j's date (descending order)
		return dateI.After(dateJ)
	})
}

// NewModel creates a new model instance
func NewModel(store *storage.Store) model {
	transactions, err := store.Transactions.GetTransactions()
	if err != nil {
		log.Fatalf("unable to get transactions: %v", err)
	}
	m := model{
		state:               menuView,
		store:               store,
		transactions:        transactions,
		listIndex:           0,
		availableTypes:      []string{"income", "expense", "transfer"},
		selectedTxIds:       make(map[int64]bool),
		fieldErrors:         make(map[string]string),
		templateFieldErrors: make(map[string]string),
		validator:           validation.NewTransactionValidator(),
		previousState:       listView, // Default to listView for backward compatibility
	}
	// Sort transactions by date (newest first)
	m.sortTransactionsByDate()
	return m
}

// getCategoryDisplayName returns the display name for a category ID, or empty string if not found
func (m model) getCategoryDisplayName(categoryId int64) string {
	return m.store.Categories.GetCategoryDisplayName(categoryId)
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
		case categoryListView:
			return m.handleCategoryListView(key)
		case categoryEditView:
			return m.handleCategoryEditView(key)
		case categoryCreateView:
			return m.handleCategoryCreateView(key)
		case createCategoryView:
			return m.handleCreateCategoryView(key)
		case bulkEditView:
			return m.handleBulkEditView(key)
		case bankStatementView:
			return m.handleBankStatementView(key)
		case statementOverlapView:
			return m.handleStatementOverlapView(key)
		case validationErrorView:
			return m.handleValidationErrorView(key)
		case undoConfirmView:
			return m.handleUndoConfirmView(key)
		case bankStatementListView:
			return m.handleBankStatementListView(key)
		case bankStatementManageView:
			return m.handleBankStatementManageView(key)
		case statementTransactionListView:
			return m.handleStatementTransactionListView(key)
		case analyticsView:
			return m.handleAnalyticsView(key)
		}
	case tea.WindowSizeMsg:
		m.windowHeight = msg.Height
		return m, nil
	}
	return m, nil
}

// validateCurrentTransaction validates the current transaction and updates field errors
func (m *model) validateCurrentTransaction() {
	categories, err := m.store.Categories.GetCategories()
	if err != nil {
		return
	}

	result := m.validator.ValidateTransaction(&m.currTransaction, categories)

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

// initUndoConfirmation prepares the undo confirmation view
func (m *model) initUndoConfirmation(statementIndex int) {
	statements := m.store.Statements.GetStatementHistory()
	if statementIndex >= 0 && statementIndex < len(statements) {
		stmt := statements[statementIndex]
		m.undoStatementId = stmt.Id
		m.undoStatementName = stmt.Filename
		m.undoTxCount = stmt.TxCount
		m.undoMessage = ""
		m.state = undoConfirmView
	}
}

// initUndoConfirmationById prepares undo confirmation using statement ID (for new workflow)
func (m *model) initUndoConfirmationById(statementId int64) {
	stmt, err := m.store.Statements.GetStatementById(statementId)
	if err != nil {
		m.bankStatementListMessage = "Error: " + err.Error()
		return
	}

	if !m.store.Statements.CanUndoImport(statementId) {
		m.bankStatementListMessage = "Cannot undo this import - invalid status or already undone"
		return
	}

	m.undoStatementId = stmt.Id
	m.undoStatementName = stmt.Filename
	m.undoTxCount = stmt.TxCount
	m.undoMessage = ""
	m.state = undoConfirmView
}

// executeUndo performs the actual undo operation
func (m *model) executeUndo() {
	if !m.store.Statements.CanUndoImport(m.undoStatementId) {
		m.undoMessage = "Cannot undo this import - statement not in completed/override status"
		return
	}

	removedCount, err := m.store.UndoImport(m.undoStatementId)
	if err != nil {
		m.undoMessage = fmt.Sprintf("Error during undo: %v", err)
		return
	}

	// Refresh transactions
	m.transactions, _ = m.store.Transactions.GetTransactions()
	// Sort transactions by date (newest first)
	m.sortTransactionsByDate()

	// Set success message for both views
	successMsg := fmt.Sprintf("Successfully undone import of %s - removed %d transactions",
		m.undoStatementName, removedCount)

	m.statementMessage = successMsg
	m.bankStatementListMessage = successMsg

	// Always return to bank statement list view
	m.state = bankStatementListView
	m.isInBankStatementActions = false
}

// validateBulkEditData validates bulk edit values and updates field errors
func (m *model) validateBulkEditData() {
	categories, err := m.store.Categories.GetCategories()
	if err != nil {
		return
	}

	// Create temporary transaction with bulk edit values
	tempTx := types.Transaction{
		Amount:      0, // Will be parsed from string
		Description: m.bulkDescriptionValue,
		Date:        time.Time{}, // Zero time for placeholder
		CategoryId:  0,           // Will be set based on category value
	}

	// Parse date if not placeholder
	if !m.bulkDateIsPlaceholder && m.bulkDateValue != "" {
		if normalizedDate, err := types.NormalizeDateToISO8601(m.bulkDateValue, ""); err == nil {
			if parsedDate, parseErr := time.Parse("2006-01-02", normalizedDate); parseErr == nil {
				tempTx.Date = parsedDate
			}
		}
	}

	// Parse amount if not placeholder
	if !m.bulkAmountIsPlaceholder && m.bulkAmountValue != "" {
		if amount, err := m.validator.Amount.ParseAmount(m.bulkAmountValue); err == nil {
			tempTx.Amount = amount
		}
	}

	// Find category ID from display name if not placeholder
	if !m.bulkCategoryIsPlaceholder && m.bulkCategoryValue != "" {
		if category := m.store.Categories.GetCategoryByDisplayName(m.bulkCategoryValue); category != nil {
			tempTx.CategoryId = category.Id
		}
	}

	// Only validate fields that are not placeholders
	m.fieldErrors = make(map[string]string)
	m.hasValidationErrors = false
	m.validationNotification = ""

	if !m.bulkAmountIsPlaceholder {
		if err := m.validator.ValidateField(&tempTx, "amount", categories); err != nil {
			m.fieldErrors["amount"] = err.Error()
			m.hasValidationErrors = true
		}
	}

	if !m.bulkDescriptionIsPlaceholder {
		if err := m.validator.ValidateField(&tempTx, "description", categories); err != nil {
			m.fieldErrors["description"] = err.Error()
			m.hasValidationErrors = true
		}
	}

	if !m.bulkDateIsPlaceholder {
		if err := m.validator.ValidateField(&tempTx, "date", categories); err != nil {
			m.fieldErrors["date"] = err.Error()
			m.hasValidationErrors = true
		}
	}

	if !m.bulkCategoryIsPlaceholder {
		if err := m.validator.ValidateField(&tempTx, "categoryId", categories); err != nil {
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
	categories, err := m.store.Categories.GetCategories()
	if err != nil {
		return
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
		if err := m.validator.ValidateField(&tempTx1, "amount", categories); err != nil {
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
		if err := m.validator.ValidateField(&tempTx2, "amount", categories); err != nil {
			m.fieldErrors["splitAmount2"] = err.Error()
			m.hasValidationErrors = true
		}
	}

	// Validate descriptions
	tempTx := types.Transaction{Description: m.splitDesc1}
	if err := m.validator.ValidateField(&tempTx, "description", categories); err != nil {
		m.fieldErrors["splitDesc1"] = err.Error()
		m.hasValidationErrors = true
	}

	tempTx.Description = m.splitDesc2
	if err := m.validator.ValidateField(&tempTx, "description", categories); err != nil {
		m.fieldErrors["splitDesc2"] = err.Error()
		m.hasValidationErrors = true
	}

	// Validate categories by finding their IDs from display names
	if category1 := m.store.GetCategoryByDisplayName(m.splitCategory1); category1 != nil {
		tempTx.CategoryId = category1.Id
		if err := m.validator.ValidateField(&tempTx, "categoryId", categories); err != nil {
			m.fieldErrors["splitCategory1"] = err.Error()
			m.hasValidationErrors = true
		}
	} else if m.splitCategory1 != "" {
		m.fieldErrors["splitCategory1"] = "Invalid category"
		m.hasValidationErrors = true
	}

	if category2 := m.store.GetCategoryByDisplayName(m.splitCategory2); category2 != nil {
		tempTx.CategoryId = category2.Id
		if err := m.validator.ValidateField(&tempTx, "categoryId", categories); err != nil {
			m.fieldErrors["splitCategory2"] = err.Error()
			m.hasValidationErrors = true
		}
	} else if m.splitCategory2 != "" {
		m.fieldErrors["splitCategory2"] = "Invalid category"
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

// Phase 3: Category Management Methods

// validateCurrentCategory validates the current category and updates field errors
func (m *model) validateCurrentCategory() {
	if m.categoryValidator == nil {
		m.categoryValidator = validation.NewCategoryManagementValidator()
	}

	m.categoryFieldErrors = make(map[string]string)
	m.hasValidationErrors = false

	// Validate entire category
	result := m.editingCategory.Validate(m.categories)
	if !result.IsValid {
		for _, err := range result.Errors {
			m.categoryFieldErrors[err.Field] = err.Message
			m.hasValidationErrors = true
		}
	}
}

// validateCategoryField validates a single category field
func (m *model) validateCategoryField(field string) {
	if m.categoryValidator == nil {
		m.categoryValidator = validation.NewCategoryManagementValidator()
	}

	err := m.editingCategory.ValidateField(field, m.categories)
	if err != nil {
		m.categoryFieldErrors[field] = err.Error()
		m.hasValidationErrors = true
	} else {
		delete(m.categoryFieldErrors, field)
		// Check if any other fields have errors
		m.hasValidationErrors = len(m.categoryFieldErrors) > 0
	}
}

// clearCategoryFieldError clears error for specific field
func (m *model) clearCategoryFieldError(field string) {
	delete(m.categoryFieldErrors, field)
	m.hasValidationErrors = len(m.categoryFieldErrors) > 0
}

// initCategoryFields initializes category field names and validation state
func (m *model) initCategoryFields() {
	m.categoryFields = []string{"displayName", "color", "parentId"}
	m.categoryFieldErrors = make(map[string]string)
	m.categoryFieldValues = make(map[int]string)
	m.hasValidationErrors = false
	if m.categoryValidator == nil {
		m.categoryValidator = validation.NewCategoryManagementValidator()
	}

	// Initialize field values from current category
	m.categoryFieldValues[categoryFieldDisplayName] = m.editingCategory.DisplayName
	m.categoryFieldValues[categoryFieldColor] = m.editingCategory.Color
	m.categoryFieldValues[categoryFieldParent] = ""
}

// loadCategories loads categories from store
func (m *model) loadCategories() tea.Cmd {
	categories, err := m.store.Categories.GetCategories()
	if err != nil {
		m.categoryMessage = "Error loading categories: " + err.Error()
	} else {
		m.categories = categories
		m.categoryMessage = ""
	}
	return nil
}

// Category Field Navigation and Editing

// handleCategoryFieldNavigation navigates between category fields
func (m *model) handleCategoryFieldNavigation(direction int) {
	if direction > 0 && m.categoryActiveField < len(m.categoryFields)-1 {
		m.categoryActiveField++
	} else if direction < 0 && m.categoryActiveField > 0 {
		m.categoryActiveField--
	}
}

// activateCategoryFieldEditing activates field editing (returns model, cmd)
func (m model) activateCategoryFieldEditing() (tea.Model, tea.Cmd) {
	// Inline the field activation logic
	if m.categoryActiveField >= 0 && m.categoryActiveField < len(m.categoryFields) {
		field := m.categoryFields[m.categoryActiveField]
		if field == "parentId" {
			return m, m.enterParentCategorySelection()
		} else {
			m.categoryEditingField = true
			// Load current field value into editing string
			if value, exists := m.categoryFieldValues[m.categoryActiveField]; exists {
				m.categoryEditingStr = value
			}
		}
	}
	return m, nil
}

// activateCategoryFieldEditingWithBackspace activates field editing with backspace (returns model, cmd)
func (m model) activateCategoryFieldEditingWithBackspace() (tea.Model, tea.Cmd) {
	// Inline the backspace activation logic
	if m.categoryActiveField >= 0 && m.categoryActiveField < len(m.categoryFields) {
		field := m.categoryFields[m.categoryActiveField]
		if field != "parentId" { // Only text fields support backspace activation
			m.categoryEditingField = true
			// Load current field value and remove last character
			if value, exists := m.categoryFieldValues[m.categoryActiveField]; exists {
				m.categoryEditingStr = value
				if len(m.categoryEditingStr) > 0 {
					m.categoryEditingStr = m.categoryEditingStr[:len(m.categoryEditingStr)-1]
				}
			}
		}
	}
	return m, nil
}

// Category Field Input Handling

// handleCategoryFieldEdit handles text input during field editing
func (m *model) handleCategoryFieldEdit(key string) tea.Cmd {
	switch key {
	case "enter", "esc":
		if key == "enter" {
			// Save the editing value to the field values map
			m.categoryFieldValues[m.categoryActiveField] = m.categoryEditingStr
			// Update the actual category struct
			m.updateCategoryFromFieldValues()
		}
		m.categoryEditingField = false
		m.categoryEditingStr = ""
		return nil
	case "backspace":
		if len(m.categoryEditingStr) > 0 {
			m.categoryEditingStr = m.categoryEditingStr[:len(m.categoryEditingStr)-1]
		}
		return nil
	default:
		if len(key) == 1 {
			m.categoryEditingStr += key
		}
		return nil
	}
}

// updateCategoryFromFieldValues updates the editing category from field values
func (m *model) updateCategoryFromFieldValues() {
	if displayName, exists := m.categoryFieldValues[categoryFieldDisplayName]; exists {
		m.editingCategory.DisplayName = displayName
	}
	if color, exists := m.categoryFieldValues[categoryFieldColor]; exists {
		m.editingCategory.Color = color
	}
}

// Parent Category Selection Methods

// enterParentCategorySelection enters parent selection mode
func (m *model) enterParentCategorySelection() tea.Cmd {
	m.isSelectingParent = true
	m.selectedParentIdx = -1 // Start with "None" selected
	m.availableParents = m.getAvailableParentCategories()
	return nil
}

// getAvailableParentCategories returns categories that can be parents
func (m *model) getAvailableParentCategories() []types.Category {
	var available []types.Category
	for _, cat := range m.categories {
		// Can't be parent of itself or if it would create circular reference
		if cat.Id != m.editingCategory.Id && !m.wouldCreateCircularReference(cat.Id) {
			available = append(available, cat)
		}
	}
	return available
}

// wouldCreateCircularReference checks if making catId a parent would create circular reference
func (m *model) wouldCreateCircularReference(catId int64) bool {
	// For now, simple check - can be enhanced with full hierarchy traversal
	return m.editingCategory.ParentId != nil && *m.editingCategory.ParentId == catId
}

// handleParentCategorySelection handles parent selection navigation
func (m *model) handleParentCategorySelection(key string) tea.Cmd {
	switch key {
	case "up":
		if m.selectedParentIdx > -1 {
			m.selectedParentIdx--
		}
	case "down":
		if m.selectedParentIdx < len(m.availableParents)-1 {
			m.selectedParentIdx++
		}
	case "enter":
		m.selectParentCategory()
		m.isSelectingParent = false
		return nil
	case "esc":
		m.isSelectingParent = false
		return nil
	}
	return nil
}

// selectParentCategory selects the current parent category
func (m *model) selectParentCategory() {
	if m.selectedParentIdx == -1 {
		// "None" selected
		m.selectedParentId = nil
		m.editingCategory.ParentId = nil
	} else if m.selectedParentIdx >= 0 && m.selectedParentIdx < len(m.availableParents) {
		// Specific parent selected
		parentId := m.availableParents[m.selectedParentIdx].Id
		m.selectedParentId = &parentId
		m.editingCategory.ParentId = &parentId
	}
}

// findCategoryById finds a category by ID
func (m *model) findCategoryById(id int64) *types.Category {
	for i := range m.categories {
		if m.categories[i].Id == id {
			return &m.categories[i]
		}
	}
	return nil
}

// CRUD Operations

// saveCategoryAndReturn saves the current category and returns to list
func (m *model) saveCategoryAndReturn() (tea.Model, tea.Cmd) {
	// Validate before saving
	m.validateCurrentCategory()
	if m.hasValidationErrors {
		m.categoryMessage = "Please fix validation errors before saving"
		return m, nil
	}

	var err error
	if m.editingCategory.Id == 0 {
		// Create new category
		err = m.store.Categories.CreateCategoryFull(&m.editingCategory)
		if err != nil {
			m.categoryMessage = "Error creating category: " + err.Error()
		} else {
			m.categoryMessage = "Category created successfully"
		}
	} else {
		// Update existing category
		err = m.store.Categories.UpdateCategory(&m.editingCategory)
		if err != nil {
			m.categoryMessage = "Error updating category: " + err.Error()
		} else {
			m.categoryMessage = "Category updated successfully"
		}
	}

	m.state = categoryListView
	return m, m.loadCategories()
}

// deleteCategoryWithValidation deletes category with safety checks
func (m *model) deleteCategoryWithValidation() tea.Cmd {
	if m.selectedCategoryIdx < 0 || m.selectedCategoryIdx >= len(m.categories) {
		m.categoryMessage = "No category selected for deletion"
		return nil
	}

	categoryToDelete := m.categories[m.selectedCategoryIdx]

	// Check if category can be deleted
	err := m.store.ValidateCategoryForDeletion(categoryToDelete.Id)
	if err != nil {
		m.categoryMessage = "Cannot delete category: " + err.Error()
		return nil
	}

	// Perform deletion
	err = m.store.Categories.DeleteCategory(categoryToDelete.Id)
	if err != nil {
		m.categoryMessage = "Error deleting category: " + err.Error()
	} else {
		m.categoryMessage = fmt.Sprintf("Category '%s' deleted successfully", categoryToDelete.DisplayName)
		// Adjust selection if needed
		if m.selectedCategoryIdx >= len(m.categories)-1 {
			m.selectedCategoryIdx = len(m.categories) - 2
			if m.selectedCategoryIdx < 0 {
				m.selectedCategoryIdx = 0
			}
		}
	}

	return m.loadCategories()
}

// Phase 4: Hierarchical Navigation Methods

// hierarchicalCategoryItem represents a category with its nesting level
type hierarchicalCategoryItem struct {
	category types.Category
	level    int // 0 for top-level, 1+ for nested levels
}

// getHierarchicalCategoryList returns categories organized hierarchically
func (m model) getHierarchicalCategoryList() []hierarchicalCategoryItem {
	var hierarchical []hierarchicalCategoryItem

	// First, add all top-level categories
	for _, category := range m.categories {
		if category.ParentId == nil {
			hierarchical = append(hierarchical, hierarchicalCategoryItem{
				category: category,
				level:    0,
			})

			// Then add all children of this category
			children := m.getCategoryChildren(category.Id, 1)
			hierarchical = append(hierarchical, children...)
		}
	}

	return hierarchical
}

// getCategoryChildren recursively gets all children of a category at the specified level
func (m model) getCategoryChildren(parentId int64, level int) []hierarchicalCategoryItem {
	var children []hierarchicalCategoryItem

	for _, category := range m.categories {
		if category.ParentId != nil && *category.ParentId == parentId {
			children = append(children, hierarchicalCategoryItem{
				category: category,
				level:    level,
			})

			// Recursively get children of this category
			grandchildren := m.getCategoryChildren(category.Id, level+1)
			children = append(children, grandchildren...)
		}
	}

	return children
}

// navigateCategoryUp moves selection up in the hierarchical category list
func (m *model) navigateCategoryUp() {
	if len(m.categories) == 0 {
		return
	}

	// Ensure selectedCategoryIdx is within bounds
	if m.selectedCategoryIdx < 0 {
		m.selectedCategoryIdx = 0
		return
	}
	if m.selectedCategoryIdx >= len(m.categories) {
		m.selectedCategoryIdx = len(m.categories) - 1
		return
	}

	hierarchical := m.getHierarchicalCategoryList()
	if len(hierarchical) == 0 {
		return
	}

	currentId := m.categories[m.selectedCategoryIdx].Id
	currentPos := -1

	// Find current position in hierarchical list
	for i, item := range hierarchical {
		if item.category.Id == currentId {
			currentPos = i
			break
		}
	}

	// Move to previous item if possible
	if currentPos > 0 {
		newCategory := hierarchical[currentPos-1].category
		// Find the index in the flat categories array
		for i, cat := range m.categories {
			if cat.Id == newCategory.Id {
				m.selectedCategoryIdx = i
				break
			}
		}
	}
}

// navigateCategoryDown moves selection down in the hierarchical category list
func (m *model) navigateCategoryDown() {
	if len(m.categories) == 0 {
		return
	}

	// Ensure selectedCategoryIdx is within bounds
	if m.selectedCategoryIdx < 0 {
		m.selectedCategoryIdx = 0
		return
	}
	if m.selectedCategoryIdx >= len(m.categories) {
		m.selectedCategoryIdx = len(m.categories) - 1
		return
	}

	hierarchical := m.getHierarchicalCategoryList()
	if len(hierarchical) == 0 {
		return
	}

	currentId := m.categories[m.selectedCategoryIdx].Id
	currentPos := -1

	// Find current position in hierarchical list
	for i, item := range hierarchical {
		if item.category.Id == currentId {
			currentPos = i
			break
		}
	}

	// Move to next item if possible
	if currentPos >= 0 && currentPos < len(hierarchical)-1 {
		newCategory := hierarchical[currentPos+1].category
		// Find the index in the flat categories array
		for i, cat := range m.categories {
			if cat.Id == newCategory.Id {
				m.selectedCategoryIdx = i
				break
			}
		}
	}
}

// handleCategoryFieldBackspace handles backspace during field editing
func (m *model) handleCategoryFieldBackspace() {
	if !m.categoryEditingField {
		return
	}

	field := m.categoryFields[m.categoryActiveField]
	switch field {
	case "displayName":
		if len(m.editingCategory.DisplayName) > 0 {
			m.editingCategory.DisplayName = m.editingCategory.DisplayName[:len(m.editingCategory.DisplayName)-1]
		}
	case "color":
		if len(m.editingCategory.Color) > 0 {
			m.editingCategory.Color = m.editingCategory.Color[:len(m.editingCategory.Color)-1]
		}
	}
}

// appendToCategoryField appends character to current field
func (m *model) appendToCategoryField(field, char string) {
	switch field {
	case "displayName":
		m.editingCategory.DisplayName += char
	case "color":
		m.editingCategory.Color += char
	}
}

// Template validation functions

// validateCurrentTemplate validates the current template and updates field errors
func (m *model) validateCurrentTemplate() {
	// Clear existing errors
	m.templateFieldErrors = make(map[string]string)
	m.templateValidationErrors = false
	m.templateValidationNotification = ""

	// Validate all template fields
	result := m.newTemplate.Validate()
	if !result.IsValid {
		m.templateValidationErrors = true
		for _, err := range result.Errors {
			m.templateFieldErrors[err.Field] = err.Message
		}
	}

	m.buildTemplateValidationNotification()
}

// buildTemplateValidationNotification builds user-friendly template validation notification
func (m *model) buildTemplateValidationNotification() {
	if !m.templateValidationErrors {
		m.templateValidationNotification = ""
		return
	}

	var errors []string
	for field, err := range m.templateFieldErrors {
		errors = append(errors, fmt.Sprintf("%s: %s", field, err))
	}
	m.templateValidationNotification = "Validation errors: " + strings.Join(errors, "; ")
}
