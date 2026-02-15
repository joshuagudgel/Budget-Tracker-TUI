package main

import (
	"log"

	tea "github.com/charmbracelet/bubbletea"
)

// Main application model
type model struct {
	// Core state
	state           uint
	store           *Store
	transactions    []Transaction
	currTransaction Transaction
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
	newTemplate      CSVTemplate
	createField      uint
	createMessage    string

	// Category management
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
	overlappingStmts []BankStatement
	pendingStatement BankStatement
	statementMessage string

	// Messages
	backupMessage string
	importMessage string
}

// NewModel creates a new model instance
func NewModel(store *Store) model {
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
	}
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
