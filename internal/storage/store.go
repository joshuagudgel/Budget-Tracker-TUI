package storage

import (
	"budget-tracker-tui/internal/types"
	"fmt"
	"os"
	"path/filepath"
)

// Store is the main store that integrates all domain stores
type Store struct {
	// Public domain stores - directly accessible by UI layer
	Transactions *TransactionStore
	Categories   *CategoryStore
	Statements   *BankStatementStore
	Templates    *CSVTemplateStore

	// Private file paths for shared operations
	transactionFile string
	categoryFile    string
	statementFile   string
	templateFile    string
	backupFile      string
}

// NewStore creates a new Store with all domain stores
func NewStore() *Store {
	return &Store{}
}

// Init initializes the store and all domain stores
func (s *Store) Init() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	appDir := filepath.Join(homeDir, ".finance-wrapped")
	os.MkdirAll(appDir, 0755)

	// Set up file paths
	s.transactionFile = filepath.Join(appDir, "transactions.json")
	s.categoryFile = filepath.Join(appDir, "categories.json")
	s.statementFile = filepath.Join(appDir, "bank-statements.json")
	s.templateFile = filepath.Join(appDir, "csv-templates.json")
	s.backupFile = filepath.Join(appDir, "backup.json")

	// Initialize domain stores
	s.Categories = NewCategoryStore(s.categoryFile)
	s.Templates = NewCSVTemplateStore(s.templateFile)
	s.Statements = NewBankStatementStore(s.statementFile)
	s.Transactions = NewTransactionStore(s.transactionFile, s.backupFile, s)

	// Load all stores
	err = s.Categories.LoadCategories()
	if err != nil {
		return err
	}

	err = s.Templates.LoadCSVTemplates()
	if err != nil {
		return err
	}

	err = s.Statements.LoadBankStatements()
	if err != nil {
		return err
	}

	err = s.Transactions.LoadTransactions()
	if err != nil {
		return err
	}

	// Migrate existing data to new category system
	return s.MigrateTransactionCategories()
}

// GetFilePaths returns the file paths for shared operations
func (s *Store) GetFilePaths() (transactions, categories, statements, templates string) {
	return s.transactionFile, s.categoryFile, s.statementFile, s.templateFile
}

// SharedUtilsInterface implementation for TransactionStore
func (s *Store) ParseCSVLine(line string, delimiter string) []string {
	return s.Templates.ParseCSVLine(line, delimiter)
}

func (s *Store) ParseAmount(amountStr string) (float64, error) {
	return s.Templates.ParseAmount(amountStr)
}

// MigrateTransactionCategories migrates old string-based categories to category IDs
// This function should be called during application startup to handle legacy data
func (s *Store) MigrateTransactionCategories() error {
	transactions, err := s.Transactions.GetTransactions()
	if err != nil {
		return err
	}

	needsMigration := false

	// For now, set default category ID for transactions that have CategoryId = 0
	for i := range transactions {
		if transactions[i].CategoryId == 0 {
			transactions[i].CategoryId = s.Categories.GetDefaultCategoryId()
			needsMigration = true
		}
	}

	// Handle migration of legacy CategoryStore format
	// If DefaultId is 0 but we have categories, set it to first category or Unsorted (id 5)
	if s.Categories.DefaultId == 0 && len(s.Categories.Categories) > 0 {
		// Try to find Unsorted category (our default)
		found := false
		for _, cat := range s.Categories.Categories {
			if cat.DisplayName == "Unsorted" {
				s.Categories.DefaultId = cat.Id
				found = true
				break
			}
		}
		// If no Unsorted category, use the first one
		if !found {
			s.Categories.DefaultId = s.Categories.Categories[0].Id
		}
		if err := s.Categories.SaveCategories(); err != nil {
			return err
		}
	}

	if needsMigration {
		// Update transactions with the migrated data
		for _, tx := range transactions {
			err = s.Transactions.SaveTransaction(tx)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// High-level operations that coordinate between domain stores

// ValidateAndImportCSV validates and imports CSV with overlap detection
func (s *Store) ValidateAndImportCSV(filePath, templateName string) *types.ImportResult {
	result := &types.ImportResult{}

	template := s.Templates.GetTemplateByName(templateName)
	if template == nil {
		result.Message = fmt.Sprintf("Template '%s' not found", templateName)
		return result
	}

	// Parse transactions to check for overlaps
	transactions, err := s.Templates.ParseCSVTransactions(filePath, template, s.Categories.GetDefaultCategoryId())
	if err != nil {
		result.Message = fmt.Sprintf("Parse error: %v", err)
		return result
	}

	if len(transactions) == 0 {
		result.Message = "No valid transactions found in CSV file"
		return result
	}

	// Extract period and detect overlaps
	result.PeriodStart, result.PeriodEnd = s.Statements.ExtractPeriodFromTransactions(transactions)
	result.OverlappingStmts = s.Statements.DetectOverlap(result.PeriodStart, result.PeriodEnd)
	result.Filename = filepath.Base(filePath)

	if len(result.OverlappingStmts) > 0 {
		result.OverlapDetected = true
		result.Message = fmt.Sprintf("Import period (%s to %s) overlaps with %d existing statements",
			result.PeriodStart, result.PeriodEnd, len(result.OverlappingStmts))
		return result
	}

	// No overlaps, proceed with import
	err = s.ImportTransactionsFromCSV(filePath, templateName)
	if err != nil {
		result.Message = fmt.Sprintf("Import failed: %v", err)
		return result
	}

	result.Success = true
	result.ImportedCount = len(transactions)
	result.Message = fmt.Sprintf("Successfully imported %d transactions from %s", len(transactions), result.Filename)
	return result
}

// ImportCSVWithOverride imports CSV with override (ignoring overlaps)
func (s *Store) ImportCSVWithOverride(filePath, templateName string) *types.ImportResult {
	result := &types.ImportResult{}

	template := s.Templates.GetTemplateByName(templateName)
	if template == nil {
		result.Message = "Template not found"
		return result
	}

	// Parse transactions
	transactions, err := s.Templates.ParseCSVTransactions(filePath, template, s.Categories.GetDefaultCategoryId())
	if err != nil {
		result.Message = fmt.Sprintf("Parse error: %v", err)
		return result
	}

	if len(transactions) == 0 {
		result.Message = "No valid transactions found"
		return result
	}

	// Record statement with override status first to get statement ID
	result.PeriodStart, result.PeriodEnd = s.Statements.ExtractPeriodFromTransactions(transactions)
	filename := filepath.Base(filePath)
	statementId := s.Statements.NextId

	err = s.Statements.RecordBankStatement(filename, result.PeriodStart, result.PeriodEnd, template.Id, len(transactions), "override")
	if err != nil {
		result.Message = fmt.Sprintf("Failed to record statement: %v", err)
		return result
	}

	// Import transactions with statement ID
	err = s.Transactions.ImportTransactionsFromCSV(transactions, fmt.Sprintf("%d", statementId))
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

// ImportTransactionsFromCSV imports transactions from CSV file
func (s *Store) ImportTransactionsFromCSV(filePath, templateName string) error {
	template := s.Templates.GetTemplateByName(templateName)
	if template == nil {
		return fmt.Errorf("template '%s' not found", templateName)
	}

	// Parse transactions
	transactions, err := s.Templates.ParseCSVTransactions(filePath, template, s.Categories.GetDefaultCategoryId())
	if err != nil {
		return fmt.Errorf("failed to parse CSV: %v", err)
	}

	if len(transactions) == 0 {
		return fmt.Errorf("no valid transactions found in CSV")
	}

	// Extract period from transactions
	periodStart, periodEnd := s.Statements.ExtractPeriodFromTransactions(transactions)

	// Check for overlaps
	overlaps := s.Statements.DetectOverlap(periodStart, periodEnd)
	if len(overlaps) > 0 {
		// Return special error for overlap detection
		return fmt.Errorf("OVERLAP_DETECTED")
	}

	// Record successful import first to get statement ID
	filename := filepath.Base(filePath)
	statementId := s.Statements.NextId

	err = s.Statements.RecordBankStatement(filename, periodStart, periodEnd, template.Id, len(transactions), "completed")
	if err != nil {
		return fmt.Errorf("failed to record statement: %v", err)
	}

	// Import transactions with statement ID
	return s.Transactions.ImportTransactionsFromCSV(transactions, fmt.Sprintf("%d", statementId))
}

// UndoImport removes all transactions from a specific statement import
func (s *Store) UndoImport(statementId int64) (int, error) {
	transactions, err := s.Transactions.GetTransactions()
	if err != nil {
		return 0, err
	}

	var removedCount int
	var remainingTransactions []types.Transaction

	for _, tx := range transactions {
		if tx.StatementId != fmt.Sprintf("%d", statementId) {
			remainingTransactions = append(remainingTransactions, tx)
		} else {
			removedCount++
		}
	}

	// Replace transactions (this is a bit of a hack, but we need to remove transactions)
	s.Transactions.transactions = remainingTransactions

	// Update statement status to indicate it was undone
	err = s.Statements.MarkStatementUndone(statementId)
	if err != nil {
		return removedCount, err
	}

	err = s.Transactions.SaveTransactions()
	if err != nil {
		return 0, err
	}

	return removedCount, nil
}

// RestoreFromBackup restores transactions from backup with category resolution
func (s *Store) RestoreFromBackup() (*RestoreResult, error) {
	result, err := s.Transactions.RestoreFromBackup()
	if err != nil {
		return result, err
	}

	// Update category IDs for restored transactions
	transactions, _ := s.Transactions.GetTransactions()
	for i, tx := range transactions {
		if tx.CategoryId == 0 {
			// Try to find category by display name from backup, fallback to default
			transactions[i].CategoryId = s.Categories.GetDefaultCategoryId()
		}
	}

	// Save updated transactions
	for _, tx := range transactions {
		err = s.Transactions.SaveTransaction(tx)
		if err != nil {
			return result, err
		}
	}

	return result, nil
}

// ValidateCategoryForDeletion validates if a category can be safely deleted
func (s *Store) ValidateCategoryForDeletion(categoryId int64) error {
	// First check category-specific validations
	err := s.Categories.ValidateCategoryForDeletion(categoryId)
	if err != nil {
		return err
	}

	// Check if category is in use by transactions
	transactions, err := s.Transactions.GetTransactions()
	if err != nil {
		return err
	}

	transactionCount := 0
	for _, tx := range transactions {
		if tx.CategoryId == categoryId {
			transactionCount++
		}
	}

	if transactionCount > 0 {
		categoryName := s.Categories.GetCategoryDisplayName(categoryId)
		return fmt.Errorf("cannot delete category '%s': it is being used by %d transaction(s)",
			categoryName, transactionCount)
	}

	return nil
}

// Legacy method compatibility - delegate to Categories store
func (s *Store) GetCategoryDisplayName(categoryId int64) string {
	return s.Categories.GetCategoryDisplayName(categoryId)
}

func (s *Store) GetCategoryByDisplayName(displayName string) *types.Category {
	return s.Categories.GetCategoryByDisplayName(displayName)
}
