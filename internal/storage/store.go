package storage

import (
	"budget-tracker-tui/internal/database"
	"budget-tracker-tui/internal/ml"
	"budget-tracker-tui/internal/types"
	"fmt"
	"path/filepath"
	"time"
)

// Store is the main store that integrates all domain stores using SQLite
type Store struct {
	// Public domain stores - directly accessible by UI layer
	Transactions      *TransactionStore
	Categories        *CategoryStore
	Statements        *BankStatementStore
	Templates         *CSVTemplateStore
	TransactionAudits *TransactionAuditStore
	Snapshots         *SnapshotStore
	UserPreferences   *UserPreferencesStore

	// CSV parsing service
	CSVParser *CSVParser

	// ML categorization service
	MLCategorizer *ml.EmbeddingsCategorizer

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
	s.TransactionAudits = NewTransactionAuditStore(db)
	s.Snapshots = NewSnapshotStore(db)
	s.UserPreferences = NewUserPreferencesStore(db)

	// Set cross-references between stores
	s.Transactions.SetTransactionAuditStore(s.TransactionAudits)
	s.Transactions.SetStore(s)                       // Add store reference for ML access
	s.Categories.SetTransactionStore(s.Transactions) // For cross-domain category validation
	s.Statements.SetTransactionStore(s.Transactions) // For cross-domain undo operations

	// Initialize ML categorization service
	err = s.initializeMLCategorizer()
	if err != nil {
		return fmt.Errorf("failed to initialize ML categorizer: %w", err)
	}

	// Initialize CSV parser with dependencies
	s.CSVParser = NewCSVParser(s.Transactions, s.Categories, s.MLCategorizer)

	// No need to load stores explicitly with SQLite - data is always persisted
	// Database health check to ensure everything is working
	err = s.db.CheckHealth()
	if err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	return nil
}

// initializeMLCategorizer sets up the ML categorization service with training data from audit events
func (s *Store) initializeMLCategorizer() error {
	// Initialize categorizer with default category
	defaultCategoryId := s.Categories.GetDefaultCategoryId()
	s.MLCategorizer = ml.NewEmbeddingsCategorizer(defaultCategoryId)

	// Load categories for ML service
	categories, err := s.Categories.GetCategories()
	if err != nil {
		return fmt.Errorf("failed to load categories: %w", err)
	}

	// Load training data from category edit audit events
	auditEvents, err := s.TransactionAudits.GetCategoryEditEvents()
	if err != nil {
		return fmt.Errorf("failed to load category edit events: %w", err)
	}

	// Train the ML model
	err = s.MLCategorizer.Train(auditEvents, categories)
	if err != nil {
		return fmt.Errorf("failed to train ML categorizer: %w", err)
	}

	// Log training statistics
	stats := s.MLCategorizer.GetStats()
	fmt.Printf("[ML] Categorizer initialized with %v examples from %v categories\n",
		stats["total_examples"], stats["categories_with_examples"])
	fmt.Printf("[ML] Training data loaded: %d audit events found\n", len(auditEvents))

	return nil
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

// High-level operations that coordinate between domain stores

// ValidateAndImportCSV validates and imports CSV with overlap detection
func (s *Store) ValidateAndImportCSV(filePath, templateName string) *types.ImportResult {
	result := &types.ImportResult{}

	template := s.Templates.GetTemplateByName(templateName)
	if template == nil {
		result.Message = fmt.Sprintf("Template '%s' not found", templateName)
		return result
	}

	// Validate CSV data before importing using fail-fast mode
	parseResult, err := s.CSVParser.ParseCSV(filePath, template, types.FailFast)
	if err != nil {
		result.Message = fmt.Sprintf("Validation error: %v", err)
		return result
	}

	if parseResult.HasErrors() {
		result.HasValidationErrors = true
		// Convert CSV parse errors to validation errors format
		for _, rowError := range parseResult.FailedRows {
			validationError := types.ValidationError{
				LineNumber: rowError.LineNumber,
				Field:      rowError.Field,
				Message:    rowError.Message,
			}
			result.ValidationErrors = append(result.ValidationErrors, validationError)
		}
		result.Success = false
		result.Message = fmt.Sprintf("Found %d formatting error(s) in CSV file", len(parseResult.FailedRows))
		return result
	}

	transactions := parseResult.SuccessfulTransactions
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

	// Save the directory for future imports (only on success)
	if saveErr := s.SaveLastImportDirectory(filePath); saveErr != nil {
		// Log error but don't fail the import
		fmt.Printf("[Warning] Failed to save last import directory: %v\n", saveErr)
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

	// Parse CSV with duplicate detection
	parseResult, err := s.CSVParser.ParseWithDuplicateDetection(filePath, template)
	if err != nil {
		result.Message = fmt.Sprintf("Parse error: %v", err)
		return result
	}

	if parseResult.HasErrors() {
		result.HasValidationErrors = true
		// Convert CSV parse errors to validation errors format
		for _, rowError := range parseResult.FailedRows {
			validationError := types.ValidationError{
				LineNumber: rowError.LineNumber,
				Field:      rowError.Field,
				Message:    rowError.Message,
			}
			result.ValidationErrors = append(result.ValidationErrors, validationError)
		}
		result.Success = false
		result.Message = fmt.Sprintf("Found %d formatting error(s) in CSV file", len(parseResult.FailedRows))
		return result
	}

	newTransactions := parseResult.SuccessfulTransactions
	duplicateTransactions := parseResult.DuplicateRows

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
	err = s.Transactions.ImportTransactionsFromCSV(newTransactions, actualStatementId)
	if err != nil {
		result.Message = fmt.Sprintf("Save failed: %v", err)
		return result
	}

	// Save the directory for future imports (only on success)
	if saveErr := s.SaveLastImportDirectory(filePath); saveErr != nil {
		// Log error but don't fail the import
		fmt.Printf("[Warning] Failed to save last import directory: %v\n", saveErr)
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

	// Parse transactions using CSV parser
	parseResult, err := s.CSVParser.ParseCSV(filePath, template, types.FailFast)
	if err != nil {
		return fmt.Errorf("failed to parse CSV: %v", err)
	}

	transactions := parseResult.SuccessfulTransactions
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
	err = s.Transactions.ImportTransactionsFromCSV(transactions, actualStatementId)
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

// Legacy method compatibility - delegate to Categories store
func (s *Store) GetCategoryDisplayName(categoryId int64) string {
	return s.Categories.GetCategoryDisplayName(categoryId)
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

// ML Categorization Methods

// PredictCategory uses ML to predict the category for a transaction description
func (s *Store) PredictCategory(description string, amount float64) ml.CategoryPrediction {
	if s.MLCategorizer == nil {
		// ML not initialized - return default category
		return ml.CategoryPrediction{
			CategoryId:   s.Categories.GetDefaultCategoryId(),
			Confidence:   0.1,
			ReasonCode:   "ml_not_available",
			SimilarityTo: "ML categorizer not initialized",
		}
	}

	return s.MLCategorizer.PredictCategory(description, amount)
}

// IsHighConfidencePrediction checks if an ML prediction is high confidence
func (s *Store) IsHighConfidencePrediction(prediction ml.CategoryPrediction) bool {
	if s.MLCategorizer == nil {
		return false
	}
	return s.MLCategorizer.IsHighConfidence(prediction)
}

// RetrainMLCategorizer retrains the ML categorizer with latest audit events
func (s *Store) RetrainMLCategorizer() error {
	if s.MLCategorizer == nil {
		return fmt.Errorf("ML categorizer not initialized")
	}

	// Load fresh categories
	categories, err := s.Categories.GetCategories()
	if err != nil {
		return fmt.Errorf("failed to load categories: %w", err)
	}

	// Load fresh training data from category edit audit events
	auditEvents, err := s.TransactionAudits.GetCategoryEditEvents()
	if err != nil {
		return fmt.Errorf("failed to load category edit events: %w", err)
	}

	// Retrain the ML model
	err = s.MLCategorizer.Train(auditEvents, categories)
	if err != nil {
		return fmt.Errorf("failed to retrain ML categorizer: %w", err)
	}

	// Log retraining statistics
	stats := s.MLCategorizer.GetStats()
	fmt.Printf("[ML] Categorizer retrained with %v examples from %v categories\n",
		stats["total_examples"], stats["categories_with_examples"])

	return nil
}

// GetMLCategorizerStats returns ML categorizer statistics for debugging/analysis
func (s *Store) GetMLCategorizerStats() map[string]interface{} {
	if s.MLCategorizer == nil {
		return map[string]interface{}{
			"status": "not_initialized",
		}
	}

	stats := s.MLCategorizer.GetStats()
	stats["status"] = "active"
	return stats
}

// Phase 2: Snapshot coordination methods

// CreateSnapshotToUserLocation creates a snapshot at a user-specified location with full coordination
func (s *Store) CreateSnapshotToUserLocation(name, description, userPath string) (*SnapshotResult, error) {
	// Input validation
	if name == "" {
		return &SnapshotResult{Success: false, Message: "Snapshot name is required"}, nil
	}

	if userPath == "" {
		return &SnapshotResult{Success: false, Message: "File path is required"}, nil
	}

	// Use the enhanced snapshot store method
	return s.Snapshots.CreateSnapshotWithUserPath(name, description, userPath)
}

// RestoreSnapshotSafely performs a safe restore with automatic backup
func (s *Store) RestoreSnapshotSafely(snapshotId int64) (*RestoreResult, error) {
	// Validate snapshot exists
	snapshot, err := s.Snapshots.GetSnapshotById(snapshotId)
	if err != nil {
		return &RestoreResult{Success: false, Message: fmt.Sprintf("Failed to validate snapshot: %v", err)}, nil
	}
	if snapshot == nil {
		return &RestoreResult{Success: false, Message: fmt.Sprintf("Snapshot with ID %d not found", snapshotId)}, nil
	}

	// Perform safe restore with backup
	return s.Snapshots.RestoreFromSnapshotWithBackup(snapshotId)
}

// LoadSnapshotDirectoryForPicker loads directory entries for snapshot file picker
func (s *Store) LoadSnapshotDirectoryForPicker(currentDir string) *SnapshotDirectoryResult {
	// Use fallback behavior to handle directory access issues gracefully
	return s.Snapshots.LoadSnapshotDirectoryEntriesWithFallback(currentDir)
}

// GenerateSnapshotFileNameSuggestion creates a suggested filename for snapshots
func (s *Store) GenerateSnapshotFileNameSuggestion(baseName string) string {
	return s.Snapshots.GenerateSnapshotFileName(baseName)
}

// ValidateSnapshotFileForRestore validates a snapshot file for restoration
func (s *Store) ValidateSnapshotFileForRestore(filePath string) error {
	return s.Snapshots.ValidateSnapshotFileAdvanced(filePath)
}

// GetSnapshotStatistics returns comprehensive snapshot statistics
func (s *Store) GetSnapshotStatistics() (map[string]interface{}, error) {
	snapshots, err := s.Snapshots.GetSnapshots()
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshots: %v", err)
	}

	totalSize := int64(0)
	totalTransactions := 0
	for _, snapshot := range snapshots {
		totalSize += snapshot.FileSize
		totalTransactions += snapshot.TransactionCount
	}

	// Get current database counts
	currentTx, currentCat, currentStmt, currentTmp, currentAudit, err := s.Snapshots.CalculateSnapshotCounts()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate current counts: %v", err)
	}

	return map[string]interface{}{
		"total_snapshots":      len(snapshots),
		"total_size_bytes":     totalSize,
		"total_size_display":   formatBytes(totalSize),
		"total_transactions":   totalTransactions,
		"current_transactions": currentTx,
		"current_categories":   currentCat,
		"current_statements":   currentStmt,
		"current_templates":    currentTmp,
		"current_audit_events": currentAudit,
	}, nil
}

// CleanupSnapshotDatabase removes orphaned snapshot metadata
func (s *Store) CleanupSnapshotDatabase() (int, error) {
	return s.Snapshots.CleanupOrphanedSnapshots()
}

// Helper function for human-readable byte formatting
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	size := float64(bytes)
	switch {
	case size >= GB:
		return fmt.Sprintf("%.1f GB", size/GB)
	case size >= MB:
		return fmt.Sprintf("%.1f MB", size/MB)
	case size >= KB:
		return fmt.Sprintf("%.1f KB", size/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// User preferences convenience methods for file picker improvements

// GetLastImportDirectory retrieves the last used dir for importing bank statements
func (s *Store) GetLastImportDirectory() string {
	if s.UserPreferences == nil {
		return ""
	}

	// Get saved directory from preferences
	savedDir := s.UserPreferences.GetPreferenceWithDefault("last_import_directory", "")
	if savedDir == "" {
		return ""
	}

	// Use directory utility to find closest existing directory
	return types.FindClosestExistingDirectory(savedDir)
}

// SaveLastImportDirectory saves the directory from a successful import for future use
func (s *Store) SaveLastImportDirectory(filePath string) error {
	if s.UserPreferences == nil {
		return fmt.Errorf("user preferences not initialized")
	}

	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	// Extract directory from file path
	directory := filepath.Dir(filePath)
	if directory == "" || directory == "." {
		return fmt.Errorf("invalid file path: %s", filePath)
	}

	// Clean the directory path
	directory = filepath.Clean(directory)

	// Validate that the directory exists before saving
	if !types.ValidateDirectoryExists(directory) {
		return fmt.Errorf("directory does not exist: %s", directory)
	}

	// Save to preferences
	return s.UserPreferences.SetPreference("last_import_directory", directory)
}
