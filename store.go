package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Store struct {
	filename     string
	backupName   string
	importName   string
	profileName  string
	categoryName string
	transactions []Transaction
	nextId       int64
	csvProfiles  CSVProfileStore
	categories   CategoryStore
}

func (s *Store) Init() error {
	var err error
	homeDir, err := os.UserHomeDir()

	if err != nil {
		return err
	}

	appDir := filepath.Join(homeDir, ".finance-wrapped")
	os.MkdirAll(appDir, 0755)

	s.filename = filepath.Join(appDir, "transactions.json")
	s.backupName = filepath.Join(appDir, "backup.json")
	s.profileName = filepath.Join(appDir, "csv-profiles.json")
	s.categoryName = filepath.Join(appDir, "categories.json")

	s.loadCSVProfiles()
	s.loadCategories()

	return s.loadTransactions()
}

// Transactions --------------------

type Transaction struct {
	Id              int64   `json:"id"`
	ParentId        *int64  `json:"parentId,omitempty"`
	Amount          float64 `json:"amount"`
	Description     string  `json:"description"`
	Date            string  `json:"date"`
	Category        string  `json:"category"`
	TransactionType string  `json:"transactionType"`
	IsSplit         bool    `json:"isSplit"`
}

func (s *Store) loadTransactions() error {
	if _, err := os.Stat(s.filename); os.IsNotExist(err) {
		s.transactions = []Transaction{}
		s.nextId = 1
		return nil
	}

	data, err := os.ReadFile(s.filename)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &s.transactions)
	if err != nil {
		return err
	}
	s.nextId = s.calculateNextId()
	return nil
}

func (s *Store) calculateNextId() int64 {
	var maxId int64 = 0
	for _, tx := range s.transactions {
		if tx.Id > maxId {
			maxId = tx.Id
		}
	}
	return maxId + 1
}

func (s *Store) saveTransactions() error {
	data, err := json.MarshalIndent(s.transactions, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filename, data, 0644)
}

func (s *Store) GetTransactions() ([]Transaction, error) {
	return s.transactions, nil
}

func (s *Store) SaveTransaction(transaction Transaction) error {
	found := false
	for i, t := range s.transactions {
		if t.Id == transaction.Id {
			s.transactions[i] = transaction
			found = true
			break
		}
	}

	if !found {
		if transaction.Id == 0 {
			transaction.Id = s.nextId
			s.nextId++
		}
		s.transactions = append(s.transactions, transaction)
	}

	return s.saveTransactions()
}

func (s *Store) DeleteTransaction(id int64) error {
	for i, transaction := range s.transactions {
		if transaction.Id == id {
			s.transactions = append(s.transactions[:i], s.transactions[i+1:]...)
			break
		}
	}
	return s.saveTransactions()
}

func (s *Store) getTransactionByID(id int64) *Transaction {
	for i := range s.transactions {
		if s.transactions[i].Id == id {
			return &s.transactions[i]
		}
	}
	return nil
}

func (s *Store) SplitTransaction(parentId int64, splits []Transaction) error {
	// Validate splits add up to parent amount (works for negative amounts)
	parent := s.getTransactionByID(parentId)
	if parent == nil {
		return fmt.Errorf("parent transaction not found")
	}

	var totalSplit float64
	for _, split := range splits {
		totalSplit += split.Amount
	}

	// Use epsilon comparison for floating point
	if math.Abs(totalSplit-parent.Amount) > 0.01 {
		return fmt.Errorf("split amounts (%.2f) don't match parent (%.2f)",
			totalSplit, parent.Amount)
	}

	// Mark parent as split
	parent.IsSplit = true

	// Add split transactions
	for _, split := range splits {
		split.Id = s.nextId
		split.ParentId = &parentId
		s.nextId++
		s.transactions = append(s.transactions, split)
	}

	return s.saveTransactions()
}

// Import Transactions --------------------

func (s *Store) ImportTransactionsFromCSV(profileName string) error {
	profile := s.getProfileByName(profileName)
	if profile == nil {
		return fmt.Errorf("profile '%s' not found", profileName)
	}

	data, err := os.ReadFile(s.importName)
	if err != nil {
		return fmt.Errorf("failed to read CSV file: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return fmt.Errorf("empty CSV file")
	}

	// Use profile.HasHeader to determine start line
	var startLine int
	if profile.HasHeader {
		startLine = 1 // Skip header row
	} else {
		startLine = 0 // No headers
	}

	var importedTransactions []Transaction
	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		fields := s.parseCSVLine(line)

		// Check if we have enough columns based on profile requirements
		maxColumn := profile.DateColumn
		if profile.AmountColumn > maxColumn {
			maxColumn = profile.AmountColumn
		}
		if profile.DescColumn > maxColumn {
			maxColumn = profile.DescColumn
		}

		if len(fields) <= maxColumn {
			continue // Skip lines with insufficient columns
		}

		transaction, err := s.parseTransactionFromProfile(fields, profile)
		if err != nil {
			continue // Skip invalid lines
		}

		// Auto-assign ID using existing pattern
		transaction.Id = s.nextId
		s.nextId++
		transaction.TransactionType = "expense" // Default as specified

		importedTransactions = append(importedTransactions, transaction)
	}

	if len(importedTransactions) == 0 {
		return fmt.Errorf("no valid transactions found in CSV")
	}

	// Add to existing transactions
	s.transactions = append(s.transactions, importedTransactions...)

	return s.saveTransactions()
}

func (s *Store) parseTransactionFromProfile(fields []string, profile *CSVProfile) (Transaction, error) {
	var transaction Transaction
	var err error

	// Extract date from specified column
	transaction.Date = strings.Trim(fields[profile.DateColumn], "\"")

	// Extract description from specified column
	transaction.Description = strings.Trim(fields[profile.DescColumn], "\"")

	// Extract amount from specified column
	amountStr := strings.Trim(fields[profile.AmountColumn], "\"")
	transaction.Amount, err = s.parseAmount(amountStr)
	if err != nil {
		return transaction, fmt.Errorf("invalid amount: %v", err)
	}

	// Use default category from CategoryStore
	transaction.Category = s.categories.Default

	return transaction, nil
}

func (s *Store) parseCSVLine(line string) []string {
	var fields []string
	var current strings.Builder
	inQuotes := false

	for _, char := range line {
		if char == '"' {
			inQuotes = !inQuotes
		} else if char == ',' && !inQuotes {
			fields = append(fields, strings.TrimSpace(current.String()))
			current.Reset()
		} else {
			current.WriteRune(char)
		}
	}

	// Add final field
	fields = append(fields, strings.TrimSpace(current.String()))
	return fields
}

func (s *Store) parseAmount(amountStr string) (float64, error) {
	// Clean amount string: remove currency symbols, parentheses, extra spaces
	cleaned := strings.TrimSpace(amountStr)
	cleaned = strings.ReplaceAll(cleaned, "$", "")
	cleaned = strings.ReplaceAll(cleaned, ",", "")

	// Handle negative amounts in parentheses (e.g., "(50.00)")
	if strings.HasPrefix(cleaned, "(") && strings.HasSuffix(cleaned, ")") {
		cleaned = "-" + strings.Trim(cleaned, "()")
	}

	return strconv.ParseFloat(cleaned, 64)
}

// Category --------------------
type Category struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type CategoryStore struct {
	Categories []Category `json:"categories"`
	Default    string     `json:"default"`
}

func (s *Store) loadCategories() error {
	if _, err := os.Stat(s.categoryName); os.IsNotExist(err) {
		// Create default categories
		s.categories = CategoryStore{
			Categories: []Category{
				{Name: "food", DisplayName: "Food & Dining"},
				{Name: "transport", DisplayName: "Transportation"},
				{Name: "entertainment", DisplayName: "Entertainment"},
				{Name: "utilities", DisplayName: "Utilities"},
				{Name: "unsorted", DisplayName: "Unsorted"},
				{Name: "sorted", DisplayName: "Sorted"},
			},
			Default: "unsorted",
		}
		return s.saveCategories()
	}

	data, err := os.ReadFile(s.categoryName)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.categories)
}

func (s *Store) saveCategories() error {
	data, err := json.MarshalIndent(s.categories, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.categoryName, data, 0644)
}

func (s *Store) GetCategories() ([]Category, error) {
	return s.categories.Categories, nil
}

func (s *Store) AddCategory(name, displayName string) error {
	// Check for duplicates
	for _, cat := range s.categories.Categories {
		if cat.Name == name {
			return fmt.Errorf("category '%s' already exists", name)
		}
	}

	category := Category{
		Name:        name,
		DisplayName: displayName,
	}

	s.categories.Categories = append(s.categories.Categories, category)
	return s.saveCategories()
}

// Restore --------------------

type BackupTransaction struct {
	Amount          float64 `json:"amount"`
	Description     string  `json:"description"`
	Date            string  `json:"date"`
	Category        string  `json:"category"`
	PaymentMethod   string  `json:"paymentMethod"`
	TransactionType string  `json:"transactionType"`
}

type BackupFile struct {
	Transactions []BackupTransaction `json:"transactions"`
}

func (s *Store) RestoreFromBackup() error {
	// Read backup file
	data, err := os.ReadFile(s.backupName)
	if err != nil {
		return fmt.Errorf("Filed to read backup file: %v", err)
	}

	var backup BackupFile
	if err := json.Unmarshal(data, &backup); err != nil {
		return fmt.Errorf("Failed to parse backup file: %v", err)
	}

	// Iterate "transactions" list that contain the following
	// amount, description, date, category, paymentMethod, transactionType
	// Convert to Transaction structs with ids
	var newTransactions []Transaction
	currentId := int64(1)
	for _, backupTx := range backup.Transactions {
		transaction := Transaction{
			Id:              currentId,
			Amount:          backupTx.Amount,
			Description:     backupTx.Description,
			Date:            backupTx.Date,
			Category:        backupTx.Category,
			TransactionType: backupTx.TransactionType,
		}
		newTransactions = append(newTransactions, transaction)
		currentId++
	}

	// replace current transactions with new
	s.transactions = newTransactions

	// save transactions
	return s.saveTransactions()
}

// CSV Profile ---------------------
type CSVProfile struct {
	Name         string `json:"name"`
	DateColumn   int    `json:"dateColumn"`
	AmountColumn int    `json:"amountColumn"`
	DescColumn   int    `json:"descColumn"`
	HasHeader    bool   `json:"hasHeader"`
}

type CSVProfileStore struct {
	Profiles []CSVProfile `json:"profiles"`
	Default  string
}

func (s *Store) loadCSVProfiles() error {
	if _, err := os.Stat(s.profileName); os.IsNotExist(err) {
		// Create default profiles
		s.csvProfiles = CSVProfileStore{
			Profiles: []CSVProfile{
				{Name: "Bank1", DateColumn: 0, AmountColumn: 1, DescColumn: 4, HasHeader: false},
				{Name: "Bank2", DateColumn: 0, AmountColumn: 5, DescColumn: 2, HasHeader: true},
			},
			Default: "Bank1",
		}
		return s.saveCSVProfiles()
	}

	data, err := os.ReadFile(s.profileName)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.csvProfiles)
}

func (s *Store) saveCSVProfiles() error {
	data, err := json.MarshalIndent(s.csvProfiles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.profileName, data, 0644)
}

func (s *Store) getProfileByName(name string) *CSVProfile {
	for _, profile := range s.csvProfiles.Profiles {
		if profile.Name == name {
			return &profile
		}
	}
	return nil
}
