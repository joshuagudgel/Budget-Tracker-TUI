package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Store struct {
	filename      string
	backupName    string
	importName    string
	profileName   string
	categoryName  string
	statementName string // New: bank-statements.json path
	transactions  []Transaction
	nextId        int64
	csvTemplates  CSVTemplateStore
	categories    CategoryStore
	statements    BankStatementStore // New: statement history
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
	s.profileName = filepath.Join(appDir, "csv-templates.json")
	s.categoryName = filepath.Join(appDir, "categories.json")
	s.statementName = filepath.Join(appDir, "bank-statements.json")

	s.loadCSVProfiles()
	s.loadCategories()
	s.loadBankStatements()

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

func (s *Store) ImportTransactionsFromCSV(templateName string) error {
	template := s.getTemplateByName(templateName)
	if template == nil {
		return fmt.Errorf("template '%s' not found", templateName)
	}

	data, err := os.ReadFile(s.importName)
	if err != nil {
		return fmt.Errorf("failed to read CSV file: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return fmt.Errorf("empty CSV file")
	}

	// Parse transactions first to get period
	var importedTransactions []Transaction
	startLine := 0
	if template.HasHeader {
		startLine = 1
	}

	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		fields := s.parseCSVLine(line)
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

		transaction, err := s.parseTransactionFromTemplate(fields, template)
		if err != nil {
			continue
		}

		transaction.Id = s.nextId
		s.nextId++
		transaction.TransactionType = "expense"
		importedTransactions = append(importedTransactions, transaction)
	}

	if len(importedTransactions) == 0 {
		return fmt.Errorf("no valid transactions found in CSV")
	}

	// Extract period from transactions
	periodStart, periodEnd := s.extractPeriodFromTransactions(importedTransactions)

	// Check for overlaps
	overlaps := s.detectOverlap(periodStart, periodEnd)
	if len(overlaps) > 0 {
		// Return special error for overlap detection
		return fmt.Errorf("OVERLAP_DETECTED")
	}

	// Add transactions and record statement
	s.transactions = append(s.transactions, importedTransactions...)

	// Record successful import
	filename := filepath.Base(s.importName)
	err = s.recordBankStatement(filename, periodStart, periodEnd, templateName, len(importedTransactions), "completed")
	if err != nil {
		// Log error but don't fail the import
		log.Printf("Failed to record statement: %v", err)
	}

	return s.saveTransactions()
}

func (s *Store) extractPeriodFromTransactions(transactions []Transaction) (start, end string) {
	if len(transactions) == 0 {
		return "", ""
	}

	start = transactions[0].Date
	end = transactions[0].Date

	for _, tx := range transactions {
		if tx.Date < start {
			start = tx.Date
		}
		if tx.Date > end {
			end = tx.Date
		}
	}

	return start, end
}

func (s *Store) detectOverlap(periodStart, periodEnd string) []BankStatement {
	var overlaps []BankStatement

	for _, stmt := range s.statements.Statements {
		if stmt.Status != "completed" {
			continue
		}

		// Check for date range overlap
		if periodStart <= stmt.PeriodEnd && periodEnd >= stmt.PeriodStart {
			overlaps = append(overlaps, stmt)
		}
	}

	return overlaps
}

func (s *Store) recordBankStatement(filename, periodStart, periodEnd, templateUsed string, txCount int, status string) error {
	statement := BankStatement{
		Id:           s.statements.NextId,
		Filename:     filename,
		ImportDate:   fmt.Sprintf("%d", time.Now().Unix()), // Simple timestamp
		PeriodStart:  periodStart,
		PeriodEnd:    periodEnd,
		TemplateUsed: templateUsed,
		TxCount:      txCount,
		Status:       status,
	}

	s.statements.Statements = append(s.statements.Statements, statement)
	s.statements.NextId++

	return s.saveBankStatements()
}

func (s *Store) parseTransactionFromTemplate(fields []string, template *CSVTemplate) (Transaction, error) {
	var transaction Transaction
	var err error

	// Extract date from specified column
	transaction.Date = strings.Trim(fields[template.DateColumn], "\"")

	// Extract description from specified column
	transaction.Description = strings.Trim(fields[template.DescColumn], "\"")

	// Extract amount from specified column
	amountStr := strings.Trim(fields[template.AmountColumn], "\"")
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

// CSV Template ---------------------
type CSVTemplate struct {
	Name         string `json:"name"`
	DateColumn   int    `json:"dateColumn"`
	AmountColumn int    `json:"amountColumn"`
	DescColumn   int    `json:"descColumn"`
	HasHeader    bool   `json:"hasHeader"`
}

type CSVTemplateStore struct {
	Templates []CSVTemplate `json:"templates"`
	Default   string        `json:"default"`
}

func (s *Store) loadCSVProfiles() error {
	if _, err := os.Stat(s.profileName); os.IsNotExist(err) {
		// Create default templates
		s.csvTemplates = CSVTemplateStore{
			Templates: []CSVTemplate{
				{Name: "Bank1", DateColumn: 0, AmountColumn: 1, DescColumn: 4, HasHeader: false},
				{Name: "Bank2", DateColumn: 0, AmountColumn: 5, DescColumn: 2, HasHeader: true},
			},
			Default: "Bank1",
		}
		return s.saveCSVTemplates()
	}

	data, err := os.ReadFile(s.profileName)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.csvTemplates)
}

func (s *Store) saveCSVTemplates() error {
	data, err := json.MarshalIndent(s.csvTemplates, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.profileName, data, 0644)
}

func (s *Store) getTemplateByName(name string) *CSVTemplate {
	for _, template := range s.csvTemplates.Templates {
		if template.Name == name {
			return &template
		}
	}
	return nil
}

// Bank Statement History ---------------------
type BankStatement struct {
	Id           int64  `json:"id"`
	Filename     string `json:"filename"`
	ImportDate   string `json:"importDate"`
	PeriodStart  string `json:"periodStart"`
	PeriodEnd    string `json:"periodEnd"`
	TemplateUsed string `json:"templateUsed"`
	TxCount      int    `json:"txCount"`
	Status       string `json:"status"` // "completed", "failed", "override"
}

type BankStatementStore struct {
	Statements []BankStatement `json:"statements"`
	NextId     int64           `json:"nextId"`
}

func (s *Store) loadBankStatements() error {
	if _, err := os.Stat(s.statementName); os.IsNotExist(err) {
		s.statements = BankStatementStore{
			Statements: []BankStatement{},
			NextId:     1,
		}
		return s.saveBankStatements()
	}

	data, err := os.ReadFile(s.statementName)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.statements)
}

func (s *Store) saveBankStatements() error {
	data, err := json.MarshalIndent(s.statements, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.statementName, data, 0644)
}
