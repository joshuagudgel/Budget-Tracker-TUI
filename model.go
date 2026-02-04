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
	csvProfileView          = 8
	createProfileView       = 9
	categoryView            = 10
	createCategoryView      = 11
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
	createProfileName uint = iota
	createProfileDate
	createProfileAmount
	createProfileDesc
	createProfileHeader
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
	// csvProfile creation
	profileIndex    int
	selectedProfile string
	newProfile      CSVProfile
	createField     uint
	createMessage   string
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
		case filePickerView:
			return m.handleFilePickerView(key)
		case csvProfileView:
			return m.handleCSVProfileView(key)
		case createProfileView:
			return m.handleCreateProfileView(key)
		case categoryView:
			return m.handleCategoryView(key)
		case createCategoryView:
			return m.handleCreateCategoryView(key)
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
		if m.isSplitMode {
			// Exit split mode and clear split data
			return m.exitSplitMode()
		}
		m.editAmountStr = ""
		m.state = listView
	case "s":
		if !m.isSplitMode {
			return m.enterSplitMode()
		} else {
			return m.exitSplitMode()
		}
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
	case "enter":
		if m.isSplitMode {
			return m.handleSaveSplit()
		}
		return m.handleSaveTransaction()
	case "backspace":
		if m.isSplitMode {
			return m.handleSplitBackspace()
		}
		return m.handleBackspace()
	default:
		if len(key) == 1 {
			if m.isSplitMode {
				return m.handleSplitInput(key)
			}
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
		m.state = csvProfileView
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
				// Use the selected profile instead of hardcoded profileName
				profileToUse := m.store.csvProfiles.Default
				if profileToUse == "" && len(m.store.csvProfiles.Profiles) > 0 {
					profileToUse = m.store.csvProfiles.Profiles[0].Name
				}

				err := m.store.ImportTransactionsFromCSV(profileToUse)
				if err != nil {
					m.importMessage = fmt.Sprintf("Error: %v", err)
				} else {
					m.transactions, _ = m.store.GetTransactions()
					imported := len(m.transactions) - currentCount
					m.importMessage = fmt.Sprintf("Successfully imported %d transactions from %s using profile %s",
						imported, filepath.Base(selected), profileToUse)
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

// CSV Profile View

func (m model) handleCSVProfileView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.state = importView
	case "up":
		if m.profileIndex > 0 {
			m.profileIndex--
		}
	case "down":
		if len(m.store.csvProfiles.Profiles) > 0 && m.profileIndex < len(m.store.csvProfiles.Profiles)-1 {
			m.profileIndex++
		}
	case "enter":
		if len(m.store.csvProfiles.Profiles) > 0 && m.profileIndex < len(m.store.csvProfiles.Profiles) {
			selectedProfile := m.store.csvProfiles.Profiles[m.profileIndex]
			m.selectedProfile = selectedProfile.Name

			// Update the store's default profile
			m.store.csvProfiles.Default = selectedProfile.Name
			err := m.store.saveCSVProfiles()
			if err != nil {
				m.importMessage = fmt.Sprintf("Error saving profile selection: %v", err)
			} else {
				m.importMessage = fmt.Sprintf("Selected CSV profile: %s", selectedProfile.Name)
			}

			m.state = importView
		}
	case "c":
		m.newProfile = CSVProfile{}
		m.createField = createProfileName
		m.createMessage = ""
		m.state = createProfileView
	}
	return m, nil
}

// Create Profile View
func (m model) handleCreateProfileView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.state = csvProfileView
	case "down", "tab":
		if m.createField < createProfileHeader {
			m.createField++
		}
	case "up":
		if m.createField > createProfileName {
			m.createField--
		}
	case "enter":
		return m.handleSaveProfile()
	case "backspace":
		return m.handleCreateProfileBackspace()
	default:
		if len(key) == 1 {
			return m.handleCreateProfileInput(key)
		}
	}
	return m, nil
}

func (m model) handleCreateProfileInput(key string) (tea.Model, tea.Cmd) {
	switch m.createField {
	case createProfileName:
		m.newProfile.Name += key
	case createProfileDate:
		if key >= "0" && key <= "9" {
			if digit, err := strconv.Atoi(key); err == nil {
				m.newProfile.DateColumn = m.newProfile.DateColumn*10 + digit
			}
		}
	case createProfileAmount:
		if key >= "0" && key <= "9" {
			if digit, err := strconv.Atoi(key); err == nil {
				m.newProfile.AmountColumn = m.newProfile.AmountColumn*10 + digit
			}
		}
	case createProfileDesc:
		if key >= "0" && key <= "9" {
			if digit, err := strconv.Atoi(key); err == nil {
				m.newProfile.DescColumn = m.newProfile.DescColumn*10 + digit
			}
		}
	case createProfileHeader:
		if key == "y" || key == "Y" {
			m.newProfile.HasHeader = true
		} else if key == "n" || key == "N" {
			m.newProfile.HasHeader = false
		}
	}
	return m, nil
}

func (m model) handleCreateProfileBackspace() (tea.Model, tea.Cmd) {
	switch m.createField {
	case createProfileName:
		if len(m.newProfile.Name) > 0 {
			m.newProfile.Name = m.newProfile.Name[:len(m.newProfile.Name)-1]
		}
	case createProfileDate:
		m.newProfile.DateColumn = m.newProfile.DateColumn / 10
	case createProfileAmount:
		m.newProfile.AmountColumn = m.newProfile.AmountColumn / 10
	case createProfileDesc:
		m.newProfile.DescColumn = m.newProfile.DescColumn / 10
	case createProfileHeader:
		m.newProfile.HasHeader = false
	}
	return m, nil
}

func (m model) handleSaveProfile() (tea.Model, tea.Cmd) {
	// Validate profile name is not empty
	if strings.TrimSpace(m.newProfile.Name) == "" {
		m.createMessage = "Profile name cannot be empty"
		return m, nil
	}

	// Check for duplicate names
	for _, profile := range m.store.csvProfiles.Profiles {
		if profile.Name == m.newProfile.Name {
			m.createMessage = "Profile name already exists"
			return m, nil
		}
	}

	// Add new profile and set as default
	m.store.csvProfiles.Profiles = append(m.store.csvProfiles.Profiles, m.newProfile)
	m.store.csvProfiles.Default = m.newProfile.Name

	// Save profiles
	err := m.store.saveCSVProfiles()
	if err != nil {
		m.createMessage = fmt.Sprintf("Error saving profile: %v", err)
		return m, nil
	}

	// Return to CSV profile view
	m.state = csvProfileView
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

func (m model) handleSplitInput(key string) (tea.Model, tea.Cmd) {
	switch m.splitField {
	case splitAmount1Field:
		m.splitAmount1 = m.updateAmountField(m.splitAmount1, key)
	case splitAmount2Field:
		m.splitAmount2 = m.updateAmountField(m.splitAmount2, key)
	case splitDesc1Field:
		m.splitDesc1 += key
	case splitDesc2Field:
		m.splitDesc2 += key
	case splitCategory1Field:
		m.splitCategory1 += key
	case splitCategory2Field:
		m.splitCategory2 += key
	}
	return m, nil
}

func (m model) updateAmountField(currentValue, key string) string {
	// Handle negative sign
	if key == "-" {
		if len(currentValue) == 0 {
			return "-"
		}
		return currentValue
	}

	// Handle digits and decimal point
	if (key >= "0" && key <= "9") || key == "." {
		// Don't allow multiple decimal points
		if key == "." && strings.Contains(currentValue, ".") {
			return currentValue
		}

		newStr := currentValue + key

		// Validate decimal places (max 2)
		dotIndex := strings.LastIndex(newStr, ".")
		if dotIndex != -1 && len(newStr)-dotIndex-1 > 2 {
			return currentValue
		}

		return newStr
	}
	return currentValue
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

func (m model) handleSaveSplit() (tea.Model, tea.Cmd) {
	// Validate amounts
	amount1, err1 := strconv.ParseFloat(m.splitAmount1, 64)
	amount2, err2 := strconv.ParseFloat(m.splitAmount2, 64)

	if err1 != nil || err2 != nil {
		m.splitMessage = "Error: Invalid amount format"
		return m, nil
	}

	// Allow negative amounts for expenses - just check they're not zero
	if amount1 == 0 || amount2 == 0 {
		m.splitMessage = "Error: Amounts cannot be zero"
		return m, nil
	}

	// Validate splits add up to original amount (works for both positive and negative)
	if amount1+amount2 != m.currTransaction.Amount {
		m.splitMessage = fmt.Sprintf("Error: Split amounts (%.2f + %.2f = %.2f) must equal original amount (%.2f)",
			amount1, amount2, amount1+amount2, m.currTransaction.Amount)
		return m, nil
	}

	// Validate descriptions
	if strings.TrimSpace(m.splitDesc1) == "" || strings.TrimSpace(m.splitDesc2) == "" {
		m.splitMessage = "Error: Descriptions cannot be empty"
		return m, nil
	}

	// Create split transactions
	split1 := Transaction{
		Amount:          amount1,
		Description:     strings.TrimSpace(m.splitDesc1),
		Date:            m.currTransaction.Date,
		Category:        strings.TrimSpace(m.splitCategory1),
		TransactionType: m.currTransaction.TransactionType,
	}

	split2 := Transaction{
		Amount:          amount2,
		Description:     strings.TrimSpace(m.splitDesc2),
		Date:            m.currTransaction.Date,
		Category:        strings.TrimSpace(m.splitCategory2),
		TransactionType: m.currTransaction.TransactionType,
	}

	// Use store's SplitTransaction method
	err := m.store.SplitTransaction(m.currTransaction.Id, []Transaction{split1, split2})
	if err != nil {
		m.splitMessage = fmt.Sprintf("Error saving split: %v", err)
		return m, nil
	}

	// Refresh transactions and return to list view
	m.transactions, _ = m.store.GetTransactions()
	m.state = listView
	return m.exitSplitMode()
}
