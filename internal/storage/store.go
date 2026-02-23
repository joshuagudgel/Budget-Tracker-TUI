package storage

import (
	"budget-tracker-tui/internal/types"
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
	transactions  []types.Transaction
	nextId        int64
	csvTemplates  CSVTemplateStore
	categories    CategoryStore
	statements    BankStatementStore // New: statement history
}

func (s *Store) Init() error {
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

	err = s.loadTransactions()
	if err != nil {
		return err
	}

	// Migrate existing data to new category system
	return s.MigrateTransactionCategories()
}

// Transactions --------------------

func (s *Store) loadTransactions() error {
	if _, err := os.Stat(s.filename); os.IsNotExist(err) {
		s.transactions = []types.Transaction{}
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

func (s *Store) calculateNextCategoryId() int64 {
	var maxId int64 = 0
	for _, cat := range s.categories.Categories {
		if cat.Id > maxId {
			maxId = cat.Id
		}
	}
	return maxId + 1
}

func (s *Store) SaveTransactions() error {
	data, err := json.MarshalIndent(s.transactions, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filename, data, 0644)
}

func (s *Store) GetTransactions() ([]types.Transaction, error) {
	return s.transactions, nil
}

func (s *Store) SaveTransaction(transaction types.Transaction) error {
	found := false
	for i, t := range s.transactions {
		if t.Id == transaction.Id {
			// Set UpdatedAt for existing transactions and mark as user modified
			transaction.UpdatedAt = time.Now().Format(time.RFC3339)
			transaction.UserModified = true
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
		// Set CreatedAt and UpdatedAt for new transactions
		now := time.Now().Format(time.RFC3339)
		if transaction.CreatedAt == "" {
			transaction.CreatedAt = now
		}
		transaction.UpdatedAt = now
		// Default confidence to 0.0 if not set
		if transaction.Confidence == 0.0 {
			transaction.Confidence = 0.0
		}
		s.transactions = append(s.transactions, transaction)
	}

	return s.SaveTransactions()
}

func (s *Store) DeleteTransaction(id int64) error {
	for i, transaction := range s.transactions {
		if transaction.Id == id {
			s.transactions = append(s.transactions[:i], s.transactions[i+1:]...)
			break
		}
	}
	return s.SaveTransactions()
}

func (s *Store) getTransactionByID(id int64) *types.Transaction {
	for i := range s.transactions {
		if s.transactions[i].Id == id {
			return &s.transactions[i]
		}
	}
	return nil
}

func (s *Store) SplitTransaction(parentId int64, splits []types.Transaction) error {
	// Validate splits add up to parent amount (works for negative amounts)
	parent := s.getTransactionByID(parentId)
	if parent == nil {
		return fmt.Errorf("parent transaction not found")
	}

	if len(splits) != 2 {
		return fmt.Errorf("exactly 2 splits required")
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

	// Modify existing transaction to become first split
	parent.Amount = splits[0].Amount
	parent.Description = splits[0].Description
	parent.CategoryId = splits[0].CategoryId
	parent.IsSplit = true
	parent.UpdatedAt = time.Now().Format(time.RFC3339)
	parent.UserModified = true

	// Create only the second split as a new transaction
	secondSplit := splits[1]
	secondSplit.Id = s.nextId
	secondSplit.Date = parent.Date                       // Ensure same date as original
	secondSplit.TransactionType = parent.TransactionType // Ensure same type
	now := time.Now().Format(time.RFC3339)
	secondSplit.CreatedAt = now
	secondSplit.UpdatedAt = now
	secondSplit.Confidence = 0.0
	s.nextId++

	s.transactions = append(s.transactions, secondSplit)

	return s.SaveTransactions()
}

// CSV Template Access Methods --------------------
func (s *Store) GetCSVTemplates() []types.CSVTemplate {
	return s.csvTemplates.Templates
}

func (s *Store) GetDefaultTemplate() string {
	return s.csvTemplates.Default
}

// Import Transactions --------------------

func (s *Store) ValidateAndImportCSV(filePath, templateName string) *types.ImportResult {
	result := &types.ImportResult{}

	// Store the import path for later use
	s.importName = filePath

	template := s.GetTemplateByName(templateName)
	if template == nil {
		result.Message = fmt.Sprintf("Template '%s' not found", templateName)
		return result
	}

	// Parse transactions to check for overlaps
	transactions, err := s.parseCSVTransactions(filePath, template)
	if err != nil {
		result.Message = fmt.Sprintf("Parse error: %v", err)
		return result
	}

	if len(transactions) == 0 {
		result.Message = "No valid transactions found in CSV file"
		return result
	}

	// Extract period and detect overlaps
	result.PeriodStart, result.PeriodEnd = s.ExtractPeriodFromTransactions(transactions)
	result.OverlappingStmts = s.DetectOverlap(result.PeriodStart, result.PeriodEnd)
	result.Filename = filepath.Base(filePath)

	if len(result.OverlappingStmts) > 0 {
		result.OverlapDetected = true
		result.Message = fmt.Sprintf("Import period (%s to %s) overlaps with %d existing statements",
			result.PeriodStart, result.PeriodEnd, len(result.OverlappingStmts))
		return result
	}

	// No overlaps, proceed with import
	err = s.ImportTransactionsFromCSV(templateName)
	if err != nil {
		result.Message = fmt.Sprintf("Import failed: %v", err)
		return result
	}

	result.Success = true
	result.ImportedCount = len(transactions)
	result.Message = fmt.Sprintf("Successfully imported %d transactions from %s", len(transactions), result.Filename)
	return result
}

func (s *Store) ImportCSVWithOverride(templateName string) *types.ImportResult {
	result := &types.ImportResult{}

	template := s.GetTemplateByName(templateName)
	if template == nil {
		result.Message = "Template not found"
		return result
	}

	// Parse transactions
	transactions, err := s.parseCSVTransactions(s.importName, template)
	if err != nil {
		result.Message = fmt.Sprintf("Parse error: %v", err)
		return result
	}

	if len(transactions) == 0 {
		result.Message = "No valid transactions found"
		return result
	}

	// Add transactions with timestamps
	now := time.Now().Format(time.RFC3339)
	for i := range transactions {
		transactions[i].Id = s.nextId
		s.nextId++
		transactions[i].CreatedAt = now
		transactions[i].UpdatedAt = now
		transactions[i].Confidence = 0.0
		if transactions[i].TransactionType == "" {
			transactions[i].TransactionType = "expense"
		}
	}

	s.transactions = append(s.transactions, transactions...)

	// Record statement with override status
	result.PeriodStart, result.PeriodEnd = s.ExtractPeriodFromTransactions(transactions)
	filename := filepath.Base(s.importName)
	err = s.RecordBankStatement(filename, result.PeriodStart, result.PeriodEnd, templateName, len(transactions), "override")
	if err != nil {
		log.Printf("Failed to record statement: %v", err)
	}

	err = s.SaveTransactions()
	if err != nil {
		result.Message = fmt.Sprintf("Save failed: %v", err)
		return result
	}

	result.Success = true
	result.ImportedCount = len(transactions)
	result.Filename = filename
	result.Message = fmt.Sprintf("Override import successful: %d transactions from %s", len(transactions), filename)
	return result
}

func (s *Store) parseCSVTransactions(filePath string, template *types.CSVTemplate) ([]types.Transaction, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty CSV file")
	}

	var transactions []types.Transaction
	startLine := 0
	if template.HasHeader {
		startLine = 1
	}

	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		fields := s.ParseCSVLine(line)
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

		transaction, err := s.ParseTransactionFromTemplate(fields, template)
		if err != nil {
			continue // Skip invalid transactions
		}

		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

// CSV Template Business Logic --------------------

type TemplateResult struct {
	Success bool
	Message string
}

func (s *Store) CreateCSVTemplate(template types.CSVTemplate) *TemplateResult {
	result := &TemplateResult{}

	// Validate template name
	if strings.TrimSpace(template.Name) == "" {
		result.Message = "Template name cannot be empty"
		return result
	}

	// Check for duplicates
	for _, existing := range s.csvTemplates.Templates {
		if existing.Name == template.Name {
			result.Message = "Template name already exists"
			return result
		}
	}

	// Validate column indices
	if template.DateColumn < 0 || template.AmountColumn < 0 || template.DescColumn < 0 {
		result.Message = "Column indices must be non-negative"
		return result
	}

	// Add template and set as default
	s.csvTemplates.Templates = append(s.csvTemplates.Templates, template)
	s.csvTemplates.Default = template.Name

	err := s.SaveCSVTemplates()
	if err != nil {
		result.Message = fmt.Sprintf("Failed to save template: %v", err)
		return result
	}

	result.Success = true
	result.Message = fmt.Sprintf("Template '%s' created and set as default", template.Name)
	return result
}

func (s *Store) SetDefaultTemplate(templateName string) *TemplateResult {
	result := &TemplateResult{}

	// Verify template exists
	if s.GetTemplateByName(templateName) == nil {
		result.Message = "Template not found"
		return result
	}

	s.csvTemplates.Default = templateName
	err := s.SaveCSVTemplates()
	if err != nil {
		result.Message = fmt.Sprintf("Failed to save: %v", err)
		return result
	}

	result.Success = true
	result.Message = fmt.Sprintf("Default template set to '%s'", templateName)
	return result
}

func (s *Store) ImportTransactionsFromCSV(templateName string) error {
	template := s.GetTemplateByName(templateName)
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
	var importedTransactions []types.Transaction
	startLine := 0
	if template.HasHeader {
		startLine = 1
	}

	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		fields := s.ParseCSVLine(line)
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

		transaction, err := s.ParseTransactionFromTemplate(fields, template)
		if err != nil {
			continue
		}

		transaction.Id = s.nextId
		s.nextId++
		transaction.TransactionType = "expense"
		// Set timestamps for imported transactions
		now := time.Now().Format(time.RFC3339)
		transaction.CreatedAt = now
		transaction.UpdatedAt = now
		transaction.Confidence = 0.0
		importedTransactions = append(importedTransactions, transaction)
	}

	if len(importedTransactions) == 0 {
		return fmt.Errorf("no valid transactions found in CSV")
	}

	// Extract period from transactions
	periodStart, periodEnd := s.ExtractPeriodFromTransactions(importedTransactions)

	// Check for overlaps
	overlaps := s.DetectOverlap(periodStart, periodEnd)
	if len(overlaps) > 0 {
		// Return special error for overlap detection
		return fmt.Errorf("OVERLAP_DETECTED")
	}

	// Add transactions and record statement
	s.transactions = append(s.transactions, importedTransactions...)

	// Record successful import
	filename := filepath.Base(s.importName)
	err = s.RecordBankStatement(filename, periodStart, periodEnd, templateName, len(importedTransactions), "completed")
	if err != nil {
		// Log error but don't fail the import
		log.Printf("Failed to record statement: %v", err)
	}

	return s.SaveTransactions()
}

func (s *Store) ExtractPeriodFromTransactions(transactions []types.Transaction) (start, end string) {
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

func (s *Store) DetectOverlap(periodStart, periodEnd string) []types.BankStatement {
	var overlaps []types.BankStatement

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

func (s *Store) RecordBankStatement(filename, periodStart, periodEnd, templateUsed string, txCount int, status string) error {
	statement := types.BankStatement{
		Id:           s.statements.NextId,
		Filename:     filename,
		ImportDate:   time.Now().Format(time.RFC3339), // RFC3339 timestamp
		PeriodStart:  periodStart,
		PeriodEnd:    periodEnd,
		TemplateUsed: templateUsed,
		TxCount:      txCount,
		Status:       status,
	}

	s.statements.Statements = append(s.statements.Statements, statement)
	s.statements.NextId++

	return s.SaveBankStatements()
}

func (s *Store) ParseTransactionFromTemplate(fields []string, template *types.CSVTemplate) (types.Transaction, error) {
	var transaction types.Transaction
	var err error

	// Extract date from specified column
	transaction.Date = strings.Trim(fields[template.DateColumn], "\"")

	// Extract description from specified column
	desc := strings.Trim(fields[template.DescColumn], "\"")
	transaction.Description = desc
	transaction.RawDescription = desc // Store original description

	// Extract amount from specified column
	amountStr := strings.Trim(fields[template.AmountColumn], "\"")
	transaction.Amount, err = s.ParseAmount(amountStr)
	if err != nil {
		return transaction, fmt.Errorf("invalid amount: %v", err)
	}

	// Use default category from CategoryStore
	transaction.CategoryId = s.categories.DefaultId

	return transaction, nil
}

func (s *Store) ParseCSVLine(line string) []string {
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

func (s *Store) ParseAmount(amountStr string) (float64, error) {
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

type CategoryStore struct {
	Categories []types.Category `json:"categories"`
	DefaultId  int64            `json:"defaultId"`
	NextId     int64            `json:"nextId"`
}

type CategoryResult struct {
	Success bool
	Message string
}

// Category Access Methods --------------------
func (s *Store) GetDefaultCategoryId() int64 {
	return s.categories.DefaultId
}

func (s *Store) CreateCategory(displayName string) *CategoryResult {
	result := &CategoryResult{}

	// Validate inputs
	if strings.TrimSpace(displayName) == "" {
		result.Message = "Display name cannot be empty"
		return result
	}

	// Check for duplicates by display name
	for _, cat := range s.categories.Categories {
		if strings.EqualFold(cat.DisplayName, strings.TrimSpace(displayName)) {
			result.Message = "Category display name already exists"
			return result
		}
	}

	err := s.AddCategory(displayName)
	if err != nil {
		result.Message = fmt.Sprintf("Failed to add category: %v", err)
		return result
	}

	result.Success = true
	result.Message = fmt.Sprintf("Category '%s' created successfully", displayName)
	return result
}

func (s *Store) SetDefaultCategory(categoryId int64) *CategoryResult {
	result := &CategoryResult{}

	// Verify category exists
	found := false
	for _, cat := range s.categories.Categories {
		if cat.Id == categoryId {
			found = true
			break
		}
	}

	if !found {
		result.Message = "Category not found"
		return result
	}

	s.categories.DefaultId = categoryId
	err := s.SaveCategories()
	if err != nil {
		result.Message = fmt.Sprintf("Failed to save: %v", err)
		return result
	}

	categoryName := s.GetCategoryDisplayName(categoryId)
	result.Success = true
	result.Message = fmt.Sprintf("Default category set to '%s'", categoryName)
	return result
}

func (s *Store) loadCategories() error {
	if _, err := os.Stat(s.categoryName); os.IsNotExist(err) {
		// Create default categories
		now := time.Now().Format(time.RFC3339)
		s.categories = CategoryStore{
			Categories: []types.Category{
				{Id: 1, DisplayName: "Food & Dining", IsActive: true, CreatedAt: now, UpdatedAt: now},
				{Id: 2, DisplayName: "Transportation", IsActive: true, CreatedAt: now, UpdatedAt: now},
				{Id: 3, DisplayName: "Entertainment", IsActive: true, CreatedAt: now, UpdatedAt: now},
				{Id: 4, DisplayName: "Utilities", IsActive: true, CreatedAt: now, UpdatedAt: now},
				{Id: 5, DisplayName: "Unsorted", IsActive: true, CreatedAt: now, UpdatedAt: now},
				{Id: 6, DisplayName: "Sorted", IsActive: true, CreatedAt: now, UpdatedAt: now},
			},
			DefaultId: 5, // Unsorted
			NextId:    7,
		}
		return s.SaveCategories()
	}

	data, err := os.ReadFile(s.categoryName)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &s.categories)
	if err != nil {
		return err
	}

	// Initialize NextId if not set or calculate from existing categories
	if s.categories.NextId == 0 {
		s.categories.NextId = s.calculateNextCategoryId()
	}

	return nil
}

func (s *Store) SaveCategories() error {
	data, err := json.MarshalIndent(s.categories, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.categoryName, data, 0644)
}

func (s *Store) GetCategories() ([]types.Category, error) {
	return s.categories.Categories, nil
}

func (s *Store) AddCategory(displayName string) error {
	// Check for duplicates by display name
	for _, cat := range s.categories.Categories {
		if strings.EqualFold(cat.DisplayName, strings.TrimSpace(displayName)) {
			return fmt.Errorf("category '%s' already exists", displayName)
		}
	}

	now := time.Now().Format(time.RFC3339)
	category := types.Category{
		Id:          s.categories.NextId,
		DisplayName: strings.TrimSpace(displayName),
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.categories.Categories = append(s.categories.Categories, category)
	s.categories.NextId++
	return s.SaveCategories()
}

// GetCategoryDisplayName returns the display name for a category ID, or empty string if not found
func (s *Store) GetCategoryDisplayName(categoryId int64) string {
	for _, category := range s.categories.Categories {
		if category.Id == categoryId {
			return category.DisplayName
		}
	}
	return ""
}

// GetCategoryById returns a category by its ID, or nil if not found
func (s *Store) GetCategoryById(categoryId int64) *types.Category {
	for _, category := range s.categories.Categories {
		if category.Id == categoryId {
			return &category
		}
	}
	return nil
}

// GetCategoryByDisplayName returns a category by its display name (case insensitive), or nil if not found
func (s *Store) GetCategoryByDisplayName(displayName string) *types.Category {
	trimmed := strings.TrimSpace(displayName)
	for _, category := range s.categories.Categories {
		if strings.EqualFold(category.DisplayName, trimmed) {
			return &category
		}
	}
	return nil
}

// MigrateTransactionCategories migrates old string-based categories to category IDs
// This function should be called during application startup to handle legacy data
func (s *Store) MigrateTransactionCategories() error {
	needsMigration := false

	// For now, set default category ID for transactions that have CategoryId = 0
	for i := range s.transactions {
		if s.transactions[i].CategoryId == 0 {
			s.transactions[i].CategoryId = s.categories.DefaultId
			needsMigration = true
		}
	}

	// Handle migration of legacy CategoryStore format
	// If DefaultId is 0 but we have categories, set it to first category or Unsorted (id 5)
	if s.categories.DefaultId == 0 && len(s.categories.Categories) > 0 {
		// Try to find Unsorted category (our default)
		found := false
		for _, cat := range s.categories.Categories {
			if cat.DisplayName == "Unsorted" {
				s.categories.DefaultId = cat.Id
				found = true
				break
			}
		}
		// If no Unsorted category, use the first one
		if !found {
			s.categories.DefaultId = s.categories.Categories[0].Id
		}
		if err := s.SaveCategories(); err != nil {
			return err
		}
	}

	if needsMigration {
		return s.SaveTransactions()
	}

	return nil
}

// Phase 2: Enhanced Category CRUD Operations --------------------

// CreateCategoryFull creates a new category with full category object support
func (s *Store) CreateCategoryFull(category *types.Category) error {
	// Validate the category using built-in validation
	existingCategories, _ := s.GetCategories()
	result := category.Validate(existingCategories)
	if !result.IsValid {
		// Return the first validation error
		return fmt.Errorf("%s", result.Errors[0].Message)
	}

	// Check for duplicates by display name
	for _, cat := range s.categories.Categories {
		if strings.EqualFold(cat.DisplayName, strings.TrimSpace(category.DisplayName)) {
			return fmt.Errorf("category '%s' already exists", category.DisplayName)
		}
	}

	// Set metadata
	now := time.Now().Format(time.RFC3339)
	category.Id = s.categories.NextId
	category.CreatedAt = now
	category.UpdatedAt = now
	category.IsActive = true

	// Add to store
	s.categories.Categories = append(s.categories.Categories, *category)
	s.categories.NextId++

	return s.SaveCategories()
}

// UpdateCategory updates an existing category
func (s *Store) UpdateCategory(category *types.Category) error {
	// Validate the category using built-in validation
	existingCategories, _ := s.GetCategories()
	result := category.Validate(existingCategories)
	if !result.IsValid {
		// Return the first validation error
		return fmt.Errorf("%s", result.Errors[0].Message)
	}

	// Find the category to update
	categoryIndex := -1
	for i, cat := range s.categories.Categories {
		if cat.Id == category.Id {
			categoryIndex = i
			break
		}
	}

	if categoryIndex == -1 {
		return fmt.Errorf("category with ID %d not found", category.Id)
	}

	// Check for duplicate display name (excluding current category)
	for i, cat := range s.categories.Categories {
		if i != categoryIndex && strings.EqualFold(cat.DisplayName, strings.TrimSpace(category.DisplayName)) {
			return fmt.Errorf("category '%s' already exists", category.DisplayName)
		}
	}

	// Preserve original creation time and update timestamp
	existingCategory := s.categories.Categories[categoryIndex]
	category.CreatedAt = existingCategory.CreatedAt
	category.UpdatedAt = time.Now().Format(time.RFC3339)

	// Update the category
	s.categories.Categories[categoryIndex] = *category

	return s.SaveCategories()
}

// DeleteCategory deletes a category with safety checks
func (s *Store) DeleteCategory(categoryId int64) error {
	// Find the category to delete
	categoryIndex := -1
	for i, cat := range s.categories.Categories {
		if cat.Id == categoryId {
			categoryIndex = i
			break
		}
	}

	if categoryIndex == -1 {
		return fmt.Errorf("category with ID %d not found", categoryId)
	}

	// Check if category is in use by transactions
	for _, tx := range s.transactions {
		if tx.CategoryId == categoryId {
			return fmt.Errorf("cannot delete category: it is being used by transactions")
		}
	}

	// Check if category has subcategories
	for _, cat := range s.categories.Categories {
		if cat.ParentId != nil && *cat.ParentId == categoryId {
			return fmt.Errorf("cannot delete category: it has subcategories. Delete or reassign subcategories first")
		}
	}

	// Check if this is the default category
	if s.categories.DefaultId == categoryId {
		return fmt.Errorf("cannot delete the default category. Set a different default category first")
	}

	// Remove the category
	s.categories.Categories = append(
		s.categories.Categories[:categoryIndex],
		s.categories.Categories[categoryIndex+1:]...,
	)

	return s.SaveCategories()
}

// GetCategoryHierarchy returns categories sorted by parent-child relationship
func (s *Store) GetCategoryHierarchy() []types.Category {
	var result []types.Category

	// Helper function to recursively add children
	var addChildren func(parentId *int64, level int)
	addChildren = func(parentId *int64, level int) {
		for _, cat := range s.categories.Categories {
			// Check if this category belongs at this level
			if (parentId == nil && cat.ParentId == nil) ||
				(parentId != nil && cat.ParentId != nil && *cat.ParentId == *parentId) {
				result = append(result, cat)
				// Recursively add children of this category
				addChildren(&cat.Id, level+1)
			}
		}
	}

	// Start with top-level categories (no parent)
	addChildren(nil, 0)
	return result
}

// getNextCategoryId returns the next available category ID (alias for existing method)
func (s *Store) GetNextCategoryId() int64 {
	return s.calculateNextCategoryId()
}

// ValidateCategoryForDeletion validates if a category can be safely deleted
func (s *Store) ValidateCategoryForDeletion(categoryId int64) error {
	// Find the category
	var targetCategory *types.Category
	for _, cat := range s.categories.Categories {
		if cat.Id == categoryId {
			targetCategory = &cat
			break
		}
	}

	if targetCategory == nil {
		return fmt.Errorf("category with ID %d not found", categoryId)
	}

	// Check if category is in use by transactions
	transactionCount := 0
	for _, tx := range s.transactions {
		if tx.CategoryId == categoryId {
			transactionCount++
		}
	}

	if transactionCount > 0 {
		return fmt.Errorf("cannot delete category '%s': it is being used by %d transaction(s)",
			targetCategory.DisplayName, transactionCount)
	}

	// Check if category has subcategories
	subcategoryCount := 0
	var subcategoryNames []string
	for _, cat := range s.categories.Categories {
		if cat.ParentId != nil && *cat.ParentId == categoryId {
			subcategoryCount++
			subcategoryNames = append(subcategoryNames, cat.DisplayName)
		}
	}

	if subcategoryCount > 0 {
		return fmt.Errorf("cannot delete category '%s': it has %d subcategorie(s) (%s). Delete or reassign subcategories first",
			targetCategory.DisplayName, subcategoryCount, strings.Join(subcategoryNames, ", "))
	}

	// Check if this is the default category
	if s.categories.DefaultId == categoryId {
		return fmt.Errorf("cannot delete category '%s': it is the default category. Set a different default category first",
			targetCategory.DisplayName)
	}

	return nil
}

// GetCategoriesForParentSelection returns categories suitable for parent selection
// (excludes the category itself and its descendants to prevent circular references)
func (s *Store) GetCategoriesForParentSelection(excludeCategoryId int64) []types.Category {
	var result []types.Category

	// Get all descendants of the excluded category
	excludeIds := make(map[int64]bool)
	excludeIds[excludeCategoryId] = true

	// Helper to find all descendants
	var findDescendants func(parentId int64)
	findDescendants = func(parentId int64) {
		for _, cat := range s.categories.Categories {
			if cat.ParentId != nil && *cat.ParentId == parentId {
				if !excludeIds[cat.Id] {
					excludeIds[cat.Id] = true
					findDescendants(cat.Id) // Recursively find deeper descendants
				}
			}
		}
	}

	findDescendants(excludeCategoryId)

	// Return categories not in the exclude list
	for _, cat := range s.categories.Categories {
		if !excludeIds[cat.Id] {
			result = append(result, cat)
		}
	}

	return result
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

type RestoreResult struct {
	Success          bool
	Message          string
	TransactionCount int
}

func (s *Store) RestoreTransactionsFromBackup() *RestoreResult {
	result := &RestoreResult{}

	// Read backup file
	data, err := os.ReadFile(s.backupName)
	if err != nil {
		result.Message = fmt.Sprintf("Cannot read backup file: %v", err)
		return result
	}

	var backup BackupFile
	err = json.Unmarshal(data, &backup)
	if err != nil {
		result.Message = fmt.Sprintf("Invalid backup format: %v", err)
		return result
	}

	if len(backup.Transactions) == 0 {
		result.Message = "Backup file contains no transactions"
		return result
	}

	// Convert backup transactions to current format
	var newTransactions []types.Transaction
	now := time.Now().Format(time.RFC3339)

	for i, backupTx := range backup.Transactions {
		// Try to find category by display name, fallback to default
		categoryId := s.categories.DefaultId
		if category := s.GetCategoryByDisplayName(backupTx.Category); category != nil {
			categoryId = category.Id
		}

		transaction := types.Transaction{
			Id:              int64(i + 1),
			Amount:          backupTx.Amount,
			Description:     backupTx.Description,
			RawDescription:  backupTx.Description,
			Date:            backupTx.Date,
			CategoryId:      categoryId,
			TransactionType: backupTx.TransactionType,
			CreatedAt:       now,
			UpdatedAt:       now,
			Confidence:      0.0,
		}

		// Set payment method if available (legacy field)
		if backupTx.PaymentMethod != "" {
			// Could map to a category or store in description
			transaction.Description = fmt.Sprintf("%s (%s)", transaction.Description, backupTx.PaymentMethod)
		}

		newTransactions = append(newTransactions, transaction)
	}

	// Replace current transactions
	s.transactions = newTransactions
	s.nextId = int64(len(newTransactions) + 1)

	err = s.SaveTransactions()
	if err != nil {
		result.Message = fmt.Sprintf("Failed to save restored transactions: %v", err)
		return result
	}

	result.Success = true
	result.TransactionCount = len(newTransactions)
	result.Message = fmt.Sprintf("Successfully restored %d transactions from backup", len(newTransactions))
	return result
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
	var newTransactions []types.Transaction
	currentId := int64(1)
	for _, backupTx := range backup.Transactions {
		now := time.Now().Format(time.RFC3339)
		// Try to find category by display name, fallback to default
		categoryId := s.categories.DefaultId
		if category := s.GetCategoryByDisplayName(backupTx.Category); category != nil {
			categoryId = category.Id
		}

		transaction := types.Transaction{
			Id:              currentId,
			Amount:          backupTx.Amount,
			Description:     backupTx.Description,
			RawDescription:  backupTx.Description,
			Date:            backupTx.Date,
			CategoryId:      categoryId,
			TransactionType: backupTx.TransactionType,
			CreatedAt:       now,
			UpdatedAt:       now,
			Confidence:      0.0,
		}
		newTransactions = append(newTransactions, transaction)
		currentId++
	}

	// replace current transactions with new
	s.transactions = newTransactions

	// save transactions
	return s.SaveTransactions()
}

// CSV Template ---------------------

type CSVTemplateStore struct {
	Templates []types.CSVTemplate `json:"templates"`
	Default   string              `json:"default"`
}

func (s *Store) loadCSVProfiles() error {
	if _, err := os.Stat(s.profileName); os.IsNotExist(err) {
		// Create default templates
		s.csvTemplates = CSVTemplateStore{
			Templates: []types.CSVTemplate{
				{Name: "Bank1", DateColumn: 0, AmountColumn: 1, DescColumn: 4, HasHeader: false},
				{Name: "Bank2", DateColumn: 0, AmountColumn: 5, DescColumn: 2, HasHeader: true},
			},
			Default: "Bank1",
		}
		return s.SaveCSVTemplates()
	}

	data, err := os.ReadFile(s.profileName)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.csvTemplates)
}

func (s *Store) SaveCSVTemplates() error {
	data, err := json.MarshalIndent(s.csvTemplates, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.profileName, data, 0644)
}

func (s *Store) GetTemplateByName(name string) *types.CSVTemplate {
	for _, template := range s.csvTemplates.Templates {
		if template.Name == name {
			return &template
		}
	}
	return nil
}

// Bank Statement History Business Logic ---------------------
type BankStatementStore struct {
	Statements []types.BankStatement `json:"statements"`
	NextId     int64                 `json:"nextId"`
}

func (s *Store) GetStatementHistory() []types.BankStatement {
	return s.statements.Statements
}

func (s *Store) GetStatementByIndex(index int) (*types.BankStatement, error) {
	if index < 0 || index >= len(s.statements.Statements) {
		return nil, fmt.Errorf("statement index %d out of range", index)
	}
	return &s.statements.Statements[index], nil
}

func (s *Store) GetStatementDetails(index int) (types.BankStatement, bool) {
	if index < 0 || index >= len(s.statements.Statements) {
		return types.BankStatement{}, false
	}
	return s.statements.Statements[index], true
}

func (s *Store) loadBankStatements() error {
	if _, err := os.Stat(s.statementName); os.IsNotExist(err) {
		s.statements = BankStatementStore{
			Statements: []types.BankStatement{},
			NextId:     1,
		}
		return s.SaveBankStatements()
	}

	data, err := os.ReadFile(s.statementName)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.statements)
}

func (s *Store) SaveBankStatements() error {
	data, err := json.MarshalIndent(s.statements, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.statementName, data, 0644)
}

// Directory Navigation Business Logic --------------------

type DirectoryResult struct {
	Entries     []string
	CurrentPath string
	Success     bool
	Message     string
}

func (s *Store) LoadDirectoryEntries(currentDir string) *DirectoryResult {
	result := &DirectoryResult{CurrentPath: currentDir}

	entries, err := os.ReadDir(currentDir)
	if err != nil {
		result.Message = fmt.Sprintf("Cannot read directory: %v", err)
		return result
	}

	// Add parent directory option if not at root
	if currentDir != filepath.Dir(currentDir) {
		result.Entries = append(result.Entries, "..")
	}

	// Add directories first
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			result.Entries = append(result.Entries, entry.Name())
		}
	}

	// Add CSV files
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".csv") {
			result.Entries = append(result.Entries, entry.Name())
		}
	}

	result.Success = true
	return result
}

func (s *Store) LoadDirectoryEntriesWithFallback(currentDir string) *DirectoryResult {
	result := s.LoadDirectoryEntries(currentDir)
	if !result.Success {
		// Fallback to home directory on error
		if homeDir, err := os.UserHomeDir(); err == nil {
			result = s.LoadDirectoryEntries(homeDir)
			if result.Success {
				result.Message = "Directory access failed, showing home directory"
			}
		}
	}
	return result
}

// Initialize default categories if none exist
func (s *Store) ensureDefaultCategories() error {
	if len(s.categories.Categories) == 0 {
		defaultCategories := []string{
			"Uncategorized",
			"Groceries",
			"Utilities",
			"Entertainment",
			"Transportation",
		}

		for _, displayName := range defaultCategories {
			err := s.AddCategory(displayName)
			if err != nil {
				return err
			}
		}

		// Set default category to first category (Uncategorized)
		if s.categories.DefaultId == 0 && len(s.categories.Categories) > 0 {
			s.categories.DefaultId = s.categories.Categories[0].Id
			return s.SaveCategories()
		}
	}
	return nil
}
