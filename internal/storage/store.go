package storage

import (
	"budget-tracker-tui/internal/database"
	"budget-tracker-tui/internal/types"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"
)

// Store is the main store that integrates all domain stores using SQLite
type Store struct {
	// Public domain stores - directly accessible by UI layer
	Transactions *TransactionStore
	Categories   *CategoryStore
	Statements   *BankStatementStore
	Templates    *CSVTemplateStore

	// Private database connection
	db *database.Connection
}

// NewStore creates a new Store with all domain stores
func NewStore() *Store {
	return &Store{}
}

// Init initializes the store and all domain stores with SQLite database
func (s *Store) Init() error {
	// Initialize SQLite database connection
	db, err := database.NewConnection()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	s.db = db

	// Initialize domain stores with database connection
	s.Categories = NewCategoryStore(db)
	s.Templates = NewCSVTemplateStore(db)
	s.Statements = NewBankStatementStore(db)
	s.Transactions = NewTransactionStore(db)

	// Set cross-references between stores
	s.Templates.SetTransactionStore(s.Transactions)
	s.Templates.SetCategoryStore(s.Categories)

	// No need to load stores explicitly with SQLite - data is always persisted
	// Database health check to ensure everything is working
	err = s.db.CheckHealth()
	if err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	// Migrate existing data to new category system if needed
	return s.MigrateTransactionCategories()
}

// Close closes the database connection
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// GetDatabasePath returns the database file path
func (s *Store) GetDatabasePath() string {
	if s.db != nil {
		return s.db.GetPath()
	}
	return ""
}

// GetDatabaseStats returns database statistics
func (s *Store) GetDatabaseStats() (map[string]interface{}, error) {
	if s.db != nil {
		return s.db.GetStats()
	}
	return nil, fmt.Errorf("database not initialized")
}

// SharedUtilsInterface implementation for backward compatibility
func (s *Store) ParseCSVLine(line string, delimiter string) []string {
	return s.Templates.ParseCSVLine(line, delimiter)
}

func (s *Store) ParseAmount(amountStr string) (float64, error) {
	return s.Templates.ParseAmount(amountStr)
}

// MigrateTransactionCategories migrates transactions to use proper category IDs
// This function ensures data integrity during the SQLite migration
func (s *Store) MigrateTransactionCategories() error {
	transactions, err := s.Transactions.GetTransactions()
	if err != nil {
		return err
	}

	needsMigration := false
	defaultCategoryId := s.Categories.GetDefaultCategoryId()

	// Set default category ID for transactions that have CategoryId = 0
	for i := range transactions {
		if transactions[i].CategoryId == 0 {
			transactions[i].CategoryId = defaultCategoryId
			needsMigration = true
		}
	}

	if needsMigration {
		// Update transactions with the migrated data using SQLite transaction
		err = s.db.ExecuteInTransaction(func(tx *sql.Tx) error {
			for _, transaction := range transactions {
				if transaction.CategoryId == defaultCategoryId {
					updateQuery := "UPDATE transactions SET category_id = ? WHERE id = ?"
					_, err := tx.Exec(updateQuery, defaultCategoryId, transaction.Id)
					if err != nil {
						return fmt.Errorf("failed to migrate transaction %d: %w", transaction.Id, err)
					}
				}
			}
			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to migrate transaction categories: %w", err)
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

	// Validate CSV data before importing
	validationErrors, err := s.Templates.ValidateCSVData(filePath, template, s.Categories.GetDefaultCategoryId())
	if err != nil {
		result.Message = fmt.Sprintf("Validation error: %v", err)
		return result
	}

	if len(validationErrors) > 0 {
		result.HasValidationErrors = true
		result.ValidationErrors = validationErrors
		result.Success = false
		result.Message = fmt.Sprintf("Found %d formatting error(s) in CSV file", len(validationErrors))
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
	result.OverlappingStmts = s.Statements.DetectOverlap(result.PeriodStart, result.PeriodEnd, template.Id)
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

// ImportCSVWithOverride imports CSV with duplicate filtering (only new transactions)
func (s *Store) ImportCSVWithOverride(filePath, templateName string) *types.ImportResult {
	result := &types.ImportResult{}

	template := s.Templates.GetTemplateByName(templateName)
	if template == nil {
		result.Message = "Template not found"
		return result
	}

	// Validate CSV data before importing
	validationErrors, err := s.Templates.ValidateCSVData(filePath, template, s.Categories.GetDefaultCategoryId())
	if err != nil {
		result.Message = fmt.Sprintf("Validation error: %v", err)
		return result
	}

	if len(validationErrors) > 0 {
		result.HasValidationErrors = true
		result.ValidationErrors = validationErrors
		result.Success = false
		result.Message = fmt.Sprintf("Found %d formatting error(s) in CSV file", len(validationErrors))
		return result
	}

	// Parse transactions and filter duplicates
	newTransactions, duplicateTransactions, err := s.Templates.ParseCSVTransactionsWithDuplicateFilter(filePath, template, s.Categories.GetDefaultCategoryId())
	if err != nil {
		result.Message = fmt.Sprintf("Parse error: %v", err)
		return result
	}

	if len(newTransactions) == 0 {
		if len(duplicateTransactions) > 0 {
			result.Message = fmt.Sprintf("No new transactions found. %d duplicate transactions were filtered out.", len(duplicateTransactions))
		} else {
			result.Message = "No valid transactions found"
		}
		return result
	}

	// Record statement with override status first to get statement ID
	result.PeriodStart, result.PeriodEnd = s.Statements.ExtractPeriodFromTransactions(newTransactions)
	filename := filepath.Base(filePath)

	actualStatementId, err := s.Statements.RecordBankStatement(filename, result.PeriodStart, result.PeriodEnd, template.Id, len(newTransactions), "override")
	if err != nil {
		result.Message = fmt.Sprintf("Failed to record statement: %v", err)
		return result
	}

	// Import only new transactions with actual statement ID
	err = s.Transactions.ImportTransactionsFromCSV(newTransactions, fmt.Sprintf("%d", actualStatementId))
	if err != nil {
		result.Message = fmt.Sprintf("Save failed: %v", err)
		return result
	}

	result.Success = true
	result.ImportedCount = len(newTransactions)
	result.Filename = filename
	if len(duplicateTransactions) > 0 {
		result.Message = fmt.Sprintf("Override import successful: %d new transactions from %s. %d duplicates filtered out.", len(newTransactions), filename, len(duplicateTransactions))
	} else {
		result.Message = fmt.Sprintf("Override import successful: %d new transactions from %s", len(newTransactions), filename)
	}
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

	// Validate that default category exists before importing
	defaultCategoryId := s.Categories.GetDefaultCategoryId()
	if defaultCategoryId <= 0 {
		return fmt.Errorf("no default category configured")
	}

	// Verify the category exists in the database
	exists, err := s.Categories.CategoryExists(defaultCategoryId)
	if err != nil {
		return fmt.Errorf("failed to validate default category: %v", err)
	}
	if !exists {
		return fmt.Errorf("default category (ID: %d) not found in database. Please create categories first.", defaultCategoryId)
	}

	// Extract period from transactions
	periodStart, periodEnd := s.Statements.ExtractPeriodFromTransactions(transactions)

	// Check for overlaps with same template
	overlaps := s.Statements.DetectOverlap(periodStart, periodEnd, template.Id)
	if len(overlaps) > 0 {
		// Return special error for overlap detection
		return fmt.Errorf("OVERLAP_DETECTED")
	}

	// Create statement record first and get actual assigned ID
	filename := filepath.Base(filePath)

	// Create statement record first with "importing" status to satisfy foreign key
	actualStatementId, err := s.Statements.RecordBankStatement(filename, periodStart, periodEnd, template.Id, len(transactions), "importing")
	if err != nil {
		return fmt.Errorf("failed to create statement record: %v", err)
	}

	// Now import transactions with actual statement_id reference
	err = s.Transactions.ImportTransactionsFromCSV(transactions, fmt.Sprintf("%d", actualStatementId))
	if err != nil {
		// If transaction import fails, mark statement as failed using actual ID
		s.Statements.MarkStatementFailed(actualStatementId, fmt.Sprintf("Transaction import failed: %v", err))
		return fmt.Errorf("failed to import transactions: %v", err)
	}

	// Update statement status to completed after successful import
	err = s.Statements.MarkStatementCompleted(actualStatementId)
	if err != nil {
		return fmt.Errorf("failed to mark statement as completed: %v", err)
	}

	return nil
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

	// Batch delete transactions with prepared statement for efficiency
	if removedCount > 0 {
		err := s.db.ExecuteInTransaction(func(tx *sql.Tx) error {
			deleteQuery := "DELETE FROM transactions WHERE statement_id = ?"
			_, err := tx.Exec(deleteQuery, statementId)
			return err
		})
		if err != nil {
			return 0, fmt.Errorf("failed to remove transactions for statement %d: %w", statementId, err)
		}
	}

	// Update statement status to indicate it was undone
	err = s.Statements.MarkStatementUndone(statementId)
	if err != nil {
		return removedCount, err
	}

	return removedCount, nil
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

// Analytics methods for spending analysis

// GetTransactionSummaryByDateRange returns income/expense totals for a date range
func (s *Store) GetTransactionSummaryByDateRange(startDate, endDate time.Time) (*types.AnalyticsSummary, error) {
	query := "SELECT COALESCE(SUM(CASE WHEN transaction_type = 'income' THEN amount ELSE 0 END), 0) as total_income, " +
		"COALESCE(SUM(CASE WHEN transaction_type = 'expense' THEN ABS(amount) ELSE 0 END), 0) as total_expense, " +
		"COUNT(*) as transaction_count FROM transactions WHERE date >= ? AND date <= ?"

	startStr := startDate.Format("2006-01-02")
	endStr := endDate.Format("2006-01-02")

	helper := database.NewSQLHelper(s.db)
	row := helper.QuerySingleRow(query, startStr, endStr)

	var summary types.AnalyticsSummary
	err := row.Scan(&summary.TotalIncome, &summary.TotalExpenses, &summary.TransactionCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction summary: %w", err)
	}

	summary.NetAmount = summary.TotalIncome - summary.TotalExpenses
	summary.DateRange = fmt.Sprintf("%s to %s", startStr, endStr)

	return &summary, nil
}

// GetCategorySpendingByDateRange returns spending breakdown by category for a date range
func (s *Store) GetCategorySpendingByDateRange(startDate, endDate time.Time) ([]types.CategorySpending, error) {
	helper := database.NewSQLHelper(s.db)
	startStr := startDate.Format("2006-01-02")
	endStr := endDate.Format("2006-01-02")

	// Main query - get expenses with positive amounts
	query := "SELECT c.display_name, COALESCE(SUM(ABS(t.amount)), 0) as total_amount, COUNT(t.id) as transaction_count " +
		"FROM categories c INNER JOIN transactions t ON c.id = t.category_id " +
		"AND t.date >= ? AND t.date <= ? AND t.transaction_type = 'expense' " +
		"WHERE c.is_active = true GROUP BY c.id, c.display_name ORDER BY total_amount DESC"

	rows, err := helper.QueryRows(query, startStr, endStr)
	if err != nil {
		return nil, fmt.Errorf("failed to query category spending: %w", err)
	}
	defer rows.Close()

	var categorySpending []types.CategorySpending
	var totalExpenses float64

	// First pass: collect data and calculate total
	for rows.Next() {
		var spending types.CategorySpending
		err := rows.Scan(&spending.CategoryName, &spending.Amount, &spending.TransactionCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category spending: %w", err)
		}
		categorySpending = append(categorySpending, spending)
		totalExpenses += spending.Amount
	}

	// Second pass: calculate percentages
	for i := range categorySpending {
		if totalExpenses > 0 {
			categorySpending[i].Percentage = (categorySpending[i].Amount / totalExpenses) * 100
		}
	}

	return categorySpending, rows.Err()
}
