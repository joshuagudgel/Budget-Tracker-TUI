package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Transaction struct {
	Id              int64   `json:"id"`
	Amount          float64 `json:"amount"`
	Description     string  `json:"description"`
	Date            string  `json:"date"`
	Category        string  `json:"category"`
	TransactionType string  `json:"transactionType"`
}

type Store struct {
	filename     string
	backupName   string
	importName   string
	transactions []Transaction
	nextId       int64
}

func (s *Store) Init() error {
	var err error
	homeDir, err := os.UserHomeDir()

	if err != nil {
		return err
	}

	appDir := filepath.Join(homeDir, ".budget-app")
	os.MkdirAll(appDir, 0755)

	s.filename = filepath.Join(appDir, "transaction-data.json")
	s.backupName = filepath.Join(appDir, "backup.json")
	s.importName = filepath.Join(appDir, "import.csv")

	fmt.Printf("Transactions will be saved to: %s\n", s.filename)

	return s.loadTransactions()
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

func (s *Store) ImportTransactionsFromCSV() error {
	data, err := os.ReadFile(s.importName)
	if err != nil {
		return fmt.Errorf("failed to read CSV file: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return fmt.Errorf("empty CSV file")
	}

	// Detect format by analyzing first line
	format := s.detectCSVFormat(lines[0])

	var startLine int
	switch format {
	case "bank2":
		startLine = 1 // Skip header row
	case "bank1":
		startLine = 0 // No headers
	default:
		return fmt.Errorf("unsupported CSV format")
	}

	var importedTransactions []Transaction
	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		fields := s.parseCSVLine(line)
		if len(fields) < 5 {
			continue // Skip malformed lines
		}

		transaction, err := s.parseTransactionFromFields(fields, format)
		if err != nil {
			//fmt.Printf("Skipping invalid line %d: %v\n", i+1, err)
			continue
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

	//fmt.Printf("Imported %d transactions from CSV\n", len(importedTransactions))
	return s.saveTransactions()
}

func (s *Store) detectCSVFormat(firstLine string) string {
	// Check for bank2 format by looking for header keywords
	lowerLine := strings.ToLower(firstLine)
	if strings.Contains(lowerLine, "date") &&
		strings.Contains(lowerLine, "description") &&
		strings.Contains(lowerLine, "amount") {
		return "bank2"
	}

	// Assume bank1 format (no headers)
	return "bank1"
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

func (s *Store) parseTransactionFromFields(fields []string, format string) (Transaction, error) {
	var transaction Transaction
	var err error

	switch format {
	case "bank2":
		// Date*=0, Post Date=1, Description*=2, Category=3, Type=4, Amount*=5, Memo=6
		if len(fields) < 6 {
			return transaction, fmt.Errorf("insufficient fields for bank2 format")
		}

		transaction.Date = strings.Trim(fields[0], "\"")
		transaction.Description = strings.Trim(fields[2], "\"")
		if len(fields) > 3 {
			transaction.Category = strings.Trim(fields[3], "\"")
		}

		amountStr := strings.Trim(fields[5], "\"")
		transaction.Amount, err = s.parseAmount(amountStr)

	case "bank1":
		// Date*=0, Amount*=1, something=2, something=3, Description*=4
		if len(fields) < 5 {
			return transaction, fmt.Errorf("insufficient fields for bank1 format")
		}

		transaction.Date = strings.Trim(fields[0], "\"")
		transaction.Description = strings.Trim(fields[4], "\"")
		transaction.Category = "" // Not available in bank1 format

		amountStr := strings.Trim(fields[1], "\"")
		transaction.Amount, err = s.parseAmount(amountStr)
	}

	if err != nil {
		return transaction, fmt.Errorf("invalid amount: %v", err)
	}

	return transaction, nil
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
