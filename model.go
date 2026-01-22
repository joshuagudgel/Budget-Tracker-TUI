package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	menuView   uint = iota
	listView        = 1
	titleView       = 2
	bodyView        = 3
	editView        = 4
	backupView      = 5
	importView      = 6
)

const (
	editAmount uint = iota
	editDescription
	editDate
	editType
	editCategory
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
}

func NewModel(store *Store) model {
	transactions, err := store.GetTransactions()
	if err != nil {
		log.Fatalf("unable to get notes: %v", err)
	}
	return model{
		state:        menuView,
		store:        store,
		transactions: transactions,
		listIndex:    0,
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
	case "d":
		if file, err := tea.LogToFile("debug.log", "debug"); err == nil {
			defer file.Close()
			log.Printf("Delete attempt: listIndex=%d, transactionCount=%d", m.listIndex, len(m.transactions))
		}

		if m.listIndex >= len(m.transactions) || m.listIndex < 0 {
			log.Printf("Attempting to delete out of bounds")
		}
		m.store.DeleteTransaction(m.transactions[m.listIndex].Id)
		m.transactions, _ = m.store.GetTransactions()

		if m.listIndex >= len(m.transactions) && len(m.transactions) > 0 {
			m.listIndex = len(m.transactions) - 1
		} else if len(m.transactions) == 0 {
			m.listIndex = 0
		}

		if file, err := tea.LogToFile("debug.log", "debug"); err == nil {
			defer file.Close()
			log.Printf("Delete completed: newListIndex=%d, newTransactionCount=%d", m.listIndex, len(m.transactions))
		}
	case "esc":
		m.state = menuView
	}
	return m, nil
}

// Edit Transaction View

func (m model) handleEditView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.editAmountStr = ""
		m.state = listView
	case "down", "tab":
		return m.handleFieldNavigation(1)
	case "up":
		return m.handleFieldNavigation(-1)
	case "enter":
		return m.handleSaveTransaction()
	case "backspace":
		return m.handleBackspace()
	default:
		if len(key) == 1 {
			return m.handleTextInput(key)
		}
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

func (m model) handleBackspace() (tea.Model, tea.Cmd) {
	switch m.editField {
	case editAmount:
		// Initialize if empty
		if m.editAmountStr == "" {
			if m.currTransaction.Amount == 0 {
				m.editAmountStr = ""
			} else {
				m.editAmountStr = fmt.Sprintf("%.2f", m.currTransaction.Amount)
			}
		}
		// Remove last character
		if len(m.editAmountStr) > 0 {
			m.editAmountStr = m.editAmountStr[:len(m.editAmountStr)-1]
		}
		amountStr := fmt.Sprintf("%g", m.currTransaction.Amount)
		if len(amountStr) > 1 {
			amountStr = amountStr[:len(amountStr)-1]
			if newAmount, err := strconv.ParseFloat(amountStr, 64); err == nil {
				m.currTransaction.Amount = newAmount
			} else {
				m.currTransaction.Amount = 0 // Reset to 0 if invalid
			}
		} else {
			m.currTransaction.Amount = 0 // Reset to 0 if only one character left
		}
	case editDescription:
		if len(m.currTransaction.Description) > 0 {
			m.currTransaction.Description = m.currTransaction.Description[:len(m.currTransaction.Description)-1]
		}
	case editDate:
		if len(m.currTransaction.Date) > 0 {
			m.currTransaction.Date = m.currTransaction.Date[:len(m.currTransaction.Date)-1]
		}
	case editType:
		if len(m.currTransaction.TransactionType) > 0 {
			m.currTransaction.TransactionType = m.currTransaction.TransactionType[:len(m.currTransaction.TransactionType)-1]
		}
	case editCategory:
		if len(m.currTransaction.Category) > 0 {
			m.currTransaction.Category = m.currTransaction.Category[:len(m.currTransaction.Category)-1]
		}
	}
	return m, nil
}

func (m model) handleTextInput(key string) (tea.Model, tea.Cmd) {
	switch m.editField {
	case editAmount:
		return m.handleAmountInput(key)
	case editDescription:
		m.currTransaction.Description += key
	case editDate:
		m.currTransaction.Date += key
	case editType:
		m.currTransaction.TransactionType += key
	case editCategory:
		m.currTransaction.Category += key
	}
	return m, nil
}

func (m model) handleAmountInput(key string) (tea.Model, tea.Cmd) {
	// initialize if empty
	if m.editAmountStr == "" {
		if m.currTransaction.Amount != 0 {
			m.editAmountStr = fmt.Sprintf("%.2f", m.currTransaction.Amount)
		}
	}

	// Only allow digits and decimal point
	if (key >= "0" && key <= "9") || key == "." {
		newStr := m.editAmountStr + key

		// Validate decimal places (max 2)
		dotIndex := strings.LastIndex(newStr, ".")
		if dotIndex != -1 && len(newStr)-dotIndex-1 > 2 {
			return m, nil
		}

		// Validate it's a valid number format
		if _, err := strconv.ParseFloat(newStr, 64); err == nil || newStr == "." {
			m.editAmountStr = newStr
		}
	}
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
		currentCount := len(m.transactions)
		err := m.store.ImportTransactionsFromCSV()
		if err != nil {
			m.importMessage = fmt.Sprintf("Error: %v", err)
		} else {
			m.transactions, _ = m.store.GetTransactions()
			imported := len(m.transactions) - currentCount
			m.importMessage = fmt.Sprintf("Successfully imported %d transactions", imported)
		}
	}
	return m, nil
}
