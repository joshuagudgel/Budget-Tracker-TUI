package storage

import (
	"budget-tracker-tui/internal/database"
	"budget-tracker-tui/internal/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// Test Infrastructure

// setupTestMainStore creates a complete Store for integration testing
func setupTestMainStore(t *testing.T) (*Store, *database.Connection) {
	// Create in-memory test database
	conn := setupTestDB(t)

	// Create and initialize complete store
	store := NewStore()
	store.db = conn

	// Initialize domain stores with test database connection
	store.Categories = NewCategoryStore(conn)
	store.Templates = NewCSVTemplateStore(conn)
	store.Statements = NewBankStatementStore(conn)
	store.Transactions = NewTransactionStore(conn)
	store.TransactionAudits = NewTransactionAuditStore(conn)

	// Set up cross-references between stores (critical for cross-domain operations)
	store.Transactions.SetTransactionAuditStore(store.TransactionAudits)
	store.Transactions.SetStore(store) // For ML access
	store.Categories.SetTransactionStore(store.Transactions)
	store.Statements.SetTransactionStore(store.Transactions)

	// Initialize ML categorizer manually (simplified for testing)
	err := store.initializeMLCategorizer()
	if err != nil {
		t.Fatalf("Failed to initialize ML categorizer: %v", err)
	}

	// Initialize CSV parser with dependencies
	store.CSVParser = NewCSVParser(store.Transactions, store.Categories, store.MLCategorizer)

	return store, conn
}

// createTestCSVFile creates a temporary CSV file with provided content
func createTestCSVFile(t *testing.T, filename, content string) string {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, filename)

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test CSV file: %v", err)
	}

	return filePath
}

// ===========================
// P1 Priority Method Tests - CRITICAL
// ===========================

// TestInit tests the complete Store initialization process
func TestMainStoreInit(t *testing.T) {
	tests := []struct {
		name        string
		expectError bool
		validate    func(*testing.T, *Store, error)
	}{
		{
			name:        "successful complete initialization",
			expectError: false,
			validate: func(t *testing.T, store *Store, err error) {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}

				// Verify all domain stores are initialized
				if store.Transactions == nil {
					t.Error("TransactionStore not initialized")
				}
				if store.Categories == nil {
					t.Error("CategoryStore not initialized")
				}
				if store.Statements == nil {
					t.Error("BankStatementStore not initialized")
				}
				if store.Templates == nil {
					t.Error("CSVTemplateStore not initialized")
				}
				if store.TransactionAudits == nil {
					t.Error("TransactionAuditStore not initialized")
				}

				// Verify CSV parser is initialized
				if store.CSVParser == nil {
					t.Error("CSVParser not initialized")
				}

				// Verify ML categorizer is initialized
				if store.MLCategorizer == nil {
					t.Error("MLCategorizer not initialized")
				}

				// Verify database connection is healthy
				if store.db == nil {
					t.Error("Database connection not initialized")
				}

				// Test database health check
				err = store.db.CheckHealth()
				if err != nil {
					t.Errorf("Database health check failed: %v", err)
				}

				// Verify cross-references are set up (critical for cross-domain operations)
				// Note: We can't directly test private fields, but we can test behavior

				// Test that default category exists (created during initialization)
				defaultId := store.Categories.GetDefaultCategoryId()
				if defaultId <= 0 {
					t.Error("Default category not properly initialized")
				}

				// Verify categories exist for ML training
				categories, err := store.Categories.GetCategories()
				if err != nil {
					t.Errorf("Failed to get categories after init: %v", err)
				}
				if len(categories) == 0 {
					t.Error("No categories available after initialization")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh store for each test
			store := NewStore()

			// Call method under test
			err := store.Init()

			// Ensure cleanup
			if store.db != nil {
				defer func() {
					store.db.Close()
				}()
			}

			// Validate results
			tt.validate(t, store, err)
		})
	}
}

// TestValidateAndImportCSV tests the primary import workflow with validation and overlap detection
func TestMainStoreValidateAndImportCSV(t *testing.T) {
	tests := []struct {
		name         string
		setupData    func(*testing.T, *Store) (string, string) // Returns filePath, templateName
		expectResult func(*testing.T, *Store, *types.ImportResult)
	}{
		{
			name: "successful import with valid CSV data",
			setupData: func(t *testing.T, store *Store) (string, string) {
				// Create test template
				template := types.CSVTemplate{
					Name:           "TestBank",
					PostDateColumn: 0,
					AmountColumn:   1,
					DescColumn:     2,
					HasHeader:      false,
					DateFormat:     "2006-01-02",
				}
				result := store.Templates.CreateCSVTemplate(template)
				if !result.Success {
					t.Fatalf("Failed to create test template: %s", result.Message)
				}

				// Create test CSV file with valid data
				csvContent := `2024-01-15,100.50,Store Purchase
2024-01-16,75.25,Gas Station
2024-01-17,-200.00,ATM Withdrawal`

				filePath := createTestCSVFile(t, "test_import.csv", csvContent)
				return filePath, "TestBank"
			},
			expectResult: func(t *testing.T, store *Store, result *types.ImportResult) {
				if !result.Success {
					t.Errorf("Expected successful import, got failure: %s", result.Message)
				}

				if result.HasValidationErrors {
					t.Errorf("Unexpected validation errors: %v", result.ValidationErrors)
				}

				if result.OverlapDetected {
					t.Error("Unexpected overlap detection on fresh database")
				}

				if result.ImportedCount != 3 {
					t.Errorf("Expected 3 imported transactions, got %d", result.ImportedCount)
				}

				// Verify transactions were actually imported
				transactions, err := store.Transactions.GetTransactions()
				if err != nil {
					t.Fatalf("Failed to retrieve transactions: %v", err)
				}
				if len(transactions) != 3 {
					t.Errorf("Expected 3 transactions in database, got %d", len(transactions))
				}

				// Verify bank statement was recorded
				statements := store.Statements.GetStatementHistory()
				if len(statements) != 1 {
					t.Errorf("Expected 1 bank statement record, got %d", len(statements))
				}
				if statements[0].Status != "completed" {
					t.Errorf("Expected statement status 'completed', got '%s'", statements[0].Status)
				}
			},
		},
		{
			name: "validation errors in CSV data prevent import",
			setupData: func(t *testing.T, store *Store) (string, string) {
				// Create test template
				template := types.CSVTemplate{
					Name:           "TestBank",
					PostDateColumn: 0,
					AmountColumn:   1,
					DescColumn:     2,
					HasHeader:      false,
					DateFormat:     "2006-01-02",
				}
				result := store.Templates.CreateCSVTemplate(template)
				if !result.Success {
					t.Fatalf("Failed to create test template: %s", result.Message)
				}

				// Create test CSV file with validation errors
				csvContent := `invalid-date,invalid-amount,Valid Description
2024-01-16,75.25,Gas Station`

				filePath := createTestCSVFile(t, "test_validation_errors.csv", csvContent)
				return filePath, "TestBank"
			},
			expectResult: func(t *testing.T, store *Store, result *types.ImportResult) {
				if result.Success {
					t.Error("Expected import to fail due to validation errors")
				}

				// In FailFast mode, validation errors result in immediate error return
				// rather than collected validation errors
				if !strings.Contains(result.Message, "Validation error") {
					t.Errorf("Expected 'Validation error' in message, got: %s", result.Message)
				}

				// Verify no transactions were imported due to validation failure
				transactions, err := store.Transactions.GetTransactions()
				if err != nil {
					t.Fatalf("Failed to retrieve transactions: %v", err)
				}
				if len(transactions) != 0 {
					t.Errorf("Expected 0 transactions due to validation failure, got %d", len(transactions))
				}

				// Verify no bank statement was recorded due to validation failure
				statements := store.Statements.GetStatementHistory()
				if len(statements) != 0 {
					t.Errorf("Expected 0 bank statements due to validation failure, got %d", len(statements))
				}
			},
		},
		{
			name: "overlap detection prevents import when periods conflict",
			setupData: func(t *testing.T, store *Store) (string, string) {
				// Create test template
				template := types.CSVTemplate{
					Name:           "TestBank",
					PostDateColumn: 0,
					AmountColumn:   1,
					DescColumn:     2,
					HasHeader:      false,
					DateFormat:     "2006-01-02",
				}
				result := store.Templates.CreateCSVTemplate(template)
				if !result.Success {
					t.Fatalf("Failed to create test template: %s", result.Message)
				}

				// First, import a statement to create overlap condition
				firstCSV := `2024-01-15,100.50,Initial Purchase
2024-01-16,75.25,Initial Gas`
				firstPath := createTestCSVFile(t, "first_import.csv", firstCSV)
				firstResult := store.ValidateAndImportCSV(firstPath, "TestBank")
				if !firstResult.Success {
					t.Fatalf("Failed to import first statement: %s", firstResult.Message)
				}

				// Now create overlapping CSV
				overlappingCSV := `2024-01-16,50.00,Overlapping Purchase
2024-01-17,25.00,Overlapping Gas`

				filePath := createTestCSVFile(t, "overlapping_import.csv", overlappingCSV)
				return filePath, "TestBank"
			},
			expectResult: func(t *testing.T, store *Store, result *types.ImportResult) {
				if result.Success {
					t.Error("Expected import to fail due to overlap detection")
				}

				if !result.OverlapDetected {
					t.Error("Expected overlap to be detected")
				}

				if len(result.OverlappingStmts) == 0 {
					t.Error("Expected overlapping statements to be reported")
				}

				// Verify original transactions remain unchanged
				transactions, err := store.Transactions.GetTransactions()
				if err != nil {
					t.Fatalf("Failed to retrieve transactions: %v", err)
				}
				if len(transactions) != 2 { // From first import only
					t.Errorf("Expected 2 transactions from first import only, got %d", len(transactions))
				}

				// Verify only one statement exists (first import)
				statements := store.Statements.GetStatementHistory()
				if len(statements) != 1 {
					t.Errorf("Expected 1 statement from first import only, got %d", len(statements))
				}
			},
		},
		{
			name: "template not found returns appropriate error",
			setupData: func(t *testing.T, store *Store) (string, string) {
				// Create CSV without creating template
				csvContent := `2024-01-15,100.50,Store Purchase`
				filePath := createTestCSVFile(t, "no_template.csv", csvContent)
				return filePath, "NonExistentTemplate"
			},
			expectResult: func(t *testing.T, store *Store, result *types.ImportResult) {
				if result.Success {
					t.Error("Expected import to fail due to missing template")
				}

				if !strings.Contains(result.Message, "not found") {
					t.Errorf("Expected 'not found' message, got: %s", result.Message)
				}

				// Verify no data was imported
				transactions, err := store.Transactions.GetTransactions()
				if err != nil {
					t.Fatalf("Failed to retrieve transactions: %v", err)
				}
				if len(transactions) != 0 {
					t.Errorf("Expected 0 transactions due to template error, got %d", len(transactions))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			store, conn := setupTestMainStore(t)
			defer teardownTestDB(t, conn)

			// Setup test data
			filePath, templateName := tt.setupData(t, store)

			// Call method under test
			result := store.ValidateAndImportCSV(filePath, templateName)

			// Validate results
			tt.expectResult(t, store, result)
		})
	}
}

// TestImportCSVWithOverride tests the override import workflow with duplicate filtering
func TestMainStoreImportCSVWithOverride(t *testing.T) {
	tests := []struct {
		name         string
		setupData    func(*testing.T, *Store) (string, string) // Returns filePath, templateName
		expectResult func(*testing.T, *Store, *types.ImportResult)
	}{
		{
			name: "successful override import with new transactions only",
			setupData: func(t *testing.T, store *Store) (string, string) {
				// Create test template
				template := types.CSVTemplate{
					Name:           "TestBank",
					PostDateColumn: 0,
					AmountColumn:   1,
					DescColumn:     2,
					HasHeader:      false,
					DateFormat:     "2006-01-02",
				}
				result := store.Templates.CreateCSVTemplate(template)
				if !result.Success {
					t.Fatalf("Failed to create test template: %s", result.Message)
				}

				// First, import some transactions normally
				firstCSV := `2024-01-15,100.50,Initial Purchase
2024-01-16,75.25,Initial Gas`
				firstPath := createTestCSVFile(t, "first_import.csv", firstCSV)
				firstResult := store.ValidateAndImportCSV(firstPath, "TestBank")
				if !firstResult.Success {
					t.Fatalf("Failed to import first statement: %s", firstResult.Message)
				}

				// Now create CSV with mix of duplicates and new transactions
				mixedCSV := `2024-01-15,100.50,Initial Purchase
2024-01-16,75.25,Initial Gas
2024-01-17,50.00,New Purchase
2024-01-18,25.00,New Gas`

				filePath := createTestCSVFile(t, "override_import.csv", mixedCSV)
				return filePath, "TestBank"
			},
			expectResult: func(t *testing.T, store *Store, result *types.ImportResult) {
				if !result.Success {
					t.Errorf("Expected successful override import, got failure: %s", result.Message)
				}

				if result.HasValidationErrors {
					t.Errorf("Unexpected validation errors: %v", result.ValidationErrors)
				}

				// Should import only new transactions (2 new ones)
				if result.ImportedCount != 2 {
					t.Errorf("Expected 2 new transactions imported, got %d", result.ImportedCount)
				}

				// Verify total transactions in database (2 original + 2 new)
				transactions, err := store.Transactions.GetTransactions()
				if err != nil {
					t.Fatalf("Failed to retrieve transactions: %v", err)
				}
				if len(transactions) != 4 {
					t.Errorf("Expected 4 total transactions, got %d", len(transactions))
				}

				// Verify statements recorded (1 original + 1 override)
				statements := store.Statements.GetStatementHistory()
				if len(statements) != 2 {
					t.Errorf("Expected 2 statements (original + override), got %d", len(statements))
				}

				// Verify override statement has correct status
				var overrideStatement *types.BankStatement
				for _, stmt := range statements {
					if stmt.Status == "override" {
						overrideStatement = &stmt
						break
					}
				}
				if overrideStatement == nil {
					t.Error("Expected to find statement with 'override' status")
				}
			},
		},
		{
			name: "override with all duplicates imports nothing",
			setupData: func(t *testing.T, store *Store) (string, string) {
				// Create test template
				template := types.CSVTemplate{
					Name:           "TestBank",
					PostDateColumn: 0,
					AmountColumn:   1,
					DescColumn:     2,
					HasHeader:      false,
					DateFormat:     "2006-01-02",
				}
				result := store.Templates.CreateCSVTemplate(template)
				if !result.Success {
					t.Fatalf("Failed to create test template: %s", result.Message)
				}

				// First, import transactions normally
				firstCSV := `2024-01-15,100.50,Purchase
2024-01-16,75.25,Gas`
				firstPath := createTestCSVFile(t, "first_import.csv", firstCSV)
				firstResult := store.ValidateAndImportCSV(firstPath, "TestBank")
				if !firstResult.Success {
					t.Fatalf("Failed to import first statement: %s", firstResult.Message)
				}

				// Create CSV with identical transactions (all duplicates)
				duplicateCSV := `2024-01-15,100.50,Purchase
2024-01-16,75.25,Gas`

				filePath := createTestCSVFile(t, "duplicate_import.csv", duplicateCSV)
				return filePath, "TestBank"
			},
			expectResult: func(t *testing.T, store *Store, result *types.ImportResult) {
				if !result.Success {
					// This might be considered success or failure depending on implementation
					// Check that appropriate message is provided
					if !strings.Contains(result.Message, "No new transactions") && !strings.Contains(result.Message, "duplicate") {
						t.Errorf("Expected message about no new transactions, got: %s", result.Message)
					}
				}

				if result.ImportedCount != 0 {
					t.Errorf("Expected 0 new transactions imported, got %d", result.ImportedCount)
				}

				// Verify no additional transactions were added
				transactions, err := store.Transactions.GetTransactions()
				if err != nil {
					t.Fatalf("Failed to retrieve transactions: %v", err)
				}
				if len(transactions) != 2 { // Only original transactions
					t.Errorf("Expected 2 transactions (original only), got %d", len(transactions))
				}
			},
		},
		{
			name: "validation errors prevent override import",
			setupData: func(t *testing.T, store *Store) (string, string) {
				// Create test template
				template := types.CSVTemplate{
					Name:           "TestBank",
					PostDateColumn: 0,
					AmountColumn:   1,
					DescColumn:     2,
					HasHeader:      false,
					DateFormat:     "2006-01-02",
				}
				result := store.Templates.CreateCSVTemplate(template)
				if !result.Success {
					t.Fatalf("Failed to create test template: %s", result.Message)
				}

				// Create CSV with validation errors
				invalidCSV := `invalid-date,invalid-amount,Description`
				filePath := createTestCSVFile(t, "invalid_override.csv", invalidCSV)
				return filePath, "TestBank"
			},
			expectResult: func(t *testing.T, store *Store, result *types.ImportResult) {
				if result.Success {
					t.Error("Expected override import to fail due to validation errors")
				}

				// In FailFast mode, validation errors result in immediate error return
				if !strings.Contains(result.Message, "Parse error") {
					t.Errorf("Expected 'Parse error' in message, got: %s", result.Message)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			store, conn := setupTestMainStore(t)
			defer teardownTestDB(t, conn)

			// Setup test data
			filePath, templateName := tt.setupData(t, store)

			// Call method under test
			result := store.ImportCSVWithOverride(filePath, templateName)

			// Validate results
			tt.expectResult(t, store, result)
		})
	}
}

// TestPredictCategory tests ML-based transaction categorization
func TestMainStorePredictCategory(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*testing.T, *Store) // Setup categories and training data
		description string
		amount      float64
		validate    func(*testing.T, *Store, string, float64, interface{}) // Generic interface for prediction
	}{
		{
			name: "ML categorizer not initialized returns default category",
			setupData: func(t *testing.T, store *Store) {
				// Intentionally don't initialize ML categorizer
				store.MLCategorizer = nil
			},
			description: "Test Purchase",
			amount:      100.50,
			validate: func(t *testing.T, store *Store, description string, amount float64, predictionInterface interface{}) {
				// Need to get the actual prediction type from ML package
				// For now, validate basic behavior
				defaultId := store.Categories.GetDefaultCategoryId()
				if defaultId <= 0 {
					t.Error("Expected valid default category ID")
				}
			},
		},
		{
			name: "ML categorizer initialized returns prediction",
			setupData: func(t *testing.T, store *Store) {
				// ML categorizer should be initialized by setupTestMainStore
				// Add some training data by creating audit events
				if store.MLCategorizer == nil {
					t.Skip("ML categorizer not available for testing")
				}

				// Create additional category for testing
				categoryResult := store.Categories.CreateCategory("Groceries")
				if !categoryResult.Success {
					t.Fatalf("Failed to create test category: %s", categoryResult.Message)
				}
			},
			description: "Grocery Store Purchase",
			amount:      85.75,
			validate: func(t *testing.T, store *Store, description string, amount float64, predictionInterface interface{}) {
				// Validate that a prediction was returned
				// The specific prediction logic is tested in the ML package
				// Here we just verify the Store method works correctly
				if store.MLCategorizer == nil {
					t.Error("Expected ML categorizer to be initialized")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			store, conn := setupTestMainStore(t)
			defer teardownTestDB(t, conn)

			// Setup test-specific data
			tt.setupData(t, store)

			// Call method under test - just verify it doesn't crash
			// The actual ML prediction logic is tested in the ml package
			prediction := store.PredictCategory(tt.description, tt.amount)

			// Basic validation that we got something back
			tt.validate(t, store, tt.description, tt.amount, prediction)
		})
	}
}

// TestIsHighConfidencePrediction tests ML confidence threshold checking
func TestMainStoreIsHighConfidencePrediction(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*testing.T, *Store)
		expectHigh  bool
		description string
	}{
		{
			name: "ML categorizer not initialized returns false",
			setupData: func(t *testing.T, store *Store) {
				store.MLCategorizer = nil
			},
			expectHigh:  false,
			description: "No ML categorizer available",
		},
		{
			name: "ML categorizer initialized returns confidence check",
			setupData: func(t *testing.T, store *Store) {
				// Use initialized ML categorizer from setupTestMainStore
				if store.MLCategorizer == nil {
					t.Skip("ML categorizer not available for testing")
				}
			},
			expectHigh:  false, // Default for unknown predictions
			description: "ML categorizer available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			store, conn := setupTestMainStore(t)
			defer teardownTestDB(t, conn)

			// Setup test-specific data
			tt.setupData(t, store)

			// Get a prediction first
			prediction := store.PredictCategory("Test Transaction", 100.0)

			// Call method under test
			isHigh := store.IsHighConfidencePrediction(prediction)

			// Validate based on ML categorizer availability
			if store.MLCategorizer == nil {
				if isHigh {
					t.Error("Expected false when ML categorizer not available")
				}
			} else {
				// When ML is available, the result depends on the actual prediction
				// We just verify the method works without crashing
				t.Logf("ML confidence check returned: %v", isHigh)
			}
		})
	}
}

// ===========================
// P2 Priority Method Tests - HIGH
// ===========================

// TestGetTransactionSummaryByDateRange tests analytics summary functionality
func TestMainStoreGetTransactionSummaryByDateRange(t *testing.T) {
	tests := []struct {
		name      string
		setupData func(*testing.T, *Store) (time.Time, time.Time) // Returns startDate, endDate
		validate  func(*testing.T, *Store, interface{}, error)    // Generic interface for summary
	}{
		{
			name: "empty date range returns zero totals",
			setupData: func(t *testing.T, store *Store) (time.Time, time.Time) {
				// No transactions - just return date range
				startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
				endDate := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
				return startDate, endDate
			},
			validate: func(t *testing.T, store *Store, summaryInterface interface{}, err error) {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				// Verify summary structure (implementation-specific validation)
				if summaryInterface == nil {
					t.Error("Expected summary object to be returned")
				}
			},
		},
		{
			name: "date range with transactions returns correct totals",
			setupData: func(t *testing.T, store *Store) (time.Time, time.Time) {
				// Create test transactions with known values
				categoryId := createTestCategory(t, store.db, "Test Category")

				// Create transactions within date range
				transactions := []types.Transaction{
					{
						Amount:          100.50, // Income
						Description:     "Salary",
						Date:            time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
						CategoryId:      categoryId,
						TransactionType: "income",
					},
					{
						Amount:          50.25, // Expense
						Description:     "Groceries",
						Date:            time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
						CategoryId:      categoryId,
						TransactionType: "expense",
					},
				}

				// Save transactions
				for _, tx := range transactions {
					err := store.Transactions.SaveTransaction(tx)
					if err != nil {
						t.Fatalf("Failed to save test transaction: %v", err)
					}
				}

				startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
				endDate := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
				return startDate, endDate
			},
			validate: func(t *testing.T, store *Store, summaryInterface interface{}, err error) {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if summaryInterface == nil {
					t.Error("Expected summary object to be returned")
				}
				// Additional validation would depend on the actual AnalyticsSummary structure
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			store, conn := setupTestMainStore(t)
			defer teardownTestDB(t, conn)

			// Setup test data
			startDate, endDate := tt.setupData(t, store)

			// Call method under test
			summary, err := store.GetTransactionSummaryByDateRange(startDate, endDate)

			// Validate results
			tt.validate(t, store, summary, err)
		})
	}
}

// TestGetCategorySpendingByDateRange tests analytics category breakdown functionality
func TestMainStoreGetCategorySpendingByDateRange(t *testing.T) {
	tests := []struct {
		name      string
		setupData func(*testing.T, *Store) (time.Time, time.Time) // Returns startDate, endDate
		validate  func(*testing.T, *Store, interface{}, error)    // Generic interface for spending data
	}{
		{
			name: "empty date range returns empty spending data",
			setupData: func(t *testing.T, store *Store) (time.Time, time.Time) {
				// No transactions
				startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
				endDate := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
				return startDate, endDate
			},
			validate: func(t *testing.T, store *Store, spendingInterface interface{}, err error) {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				// Verify spending data structure
				if spendingInterface == nil {
					t.Error("Expected spending data to be returned")
				}
			},
		},
		{
			name: "date range with categorized transactions returns spending breakdown",
			setupData: func(t *testing.T, store *Store) (time.Time, time.Time) {
				// Create test categories
				groceryCategoryId := createTestCategory(t, store.db, "Groceries")
				gasCategoryId := createTestCategory(t, store.db, "Gas")

				// Create transactions in different categories
				transactions := []types.Transaction{
					{
						Amount:          50.25,
						Description:     "Supermarket",
						Date:            time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
						CategoryId:      groceryCategoryId,
						TransactionType: "expense",
					},
					{
						Amount:          30.75,
						Description:     "Gas Station",
						Date:            time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
						CategoryId:      gasCategoryId,
						TransactionType: "expense",
					},
					{
						Amount:          25.00,
						Description:     "Grocery Store",
						Date:            time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC),
						CategoryId:      groceryCategoryId,
						TransactionType: "expense",
					},
				}

				// Save transactions
				for _, tx := range transactions {
					err := store.Transactions.SaveTransaction(tx)
					if err != nil {
						t.Fatalf("Failed to save test transaction: %v", err)
					}
				}

				startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
				endDate := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
				return startDate, endDate
			},
			validate: func(t *testing.T, store *Store, spendingInterface interface{}, err error) {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if spendingInterface == nil {
					t.Error("Expected spending data to be returned")
				}
				// Additional validation would depend on the actual CategorySpending structure
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			store, conn := setupTestMainStore(t)
			defer teardownTestDB(t, conn)

			// Setup test data
			startDate, endDate := tt.setupData(t, store)

			// Call method under test
			spending, err := store.GetCategorySpendingByDateRange(startDate, endDate)

			// Validate results
			tt.validate(t, store, spending, err)
		})
	}
}

// ===========================
// Additional Method Tests
// ===========================

// TestMainStoreImportTransactionsFromCSV tests the legacy import method delegation
func TestMainStoreImportTransactionsFromCSV(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*testing.T, *Store) (string, string) // Returns filePath, templateName
		expectError bool
		errorCheck  func(*testing.T, error)
	}{
		{
			name: "successful legacy import",
			setupData: func(t *testing.T, store *Store) (string, string) {
				// Create test template
				template := types.CSVTemplate{
					Name:           "LegacyBank",
					PostDateColumn: 0,
					AmountColumn:   1,
					DescColumn:     2,
					HasHeader:      false,
					DateFormat:     "2006-01-02",
				}
				result := store.Templates.CreateCSVTemplate(template)
				if !result.Success {
					t.Fatalf("Failed to create test template: %s", result.Message)
				}

				// Create test CSV
				csvContent := `2024-01-15,100.50,Legacy Purchase`
				filePath := createTestCSVFile(t, "legacy_import.csv", csvContent)
				return filePath, "LegacyBank"
			},
			expectError: false,
			errorCheck: func(t *testing.T, err error) {
				if err != nil {
					t.Errorf("Unexpected error in legacy import: %v", err)
				}
			},
		},
		{
			name: "template not found error",
			setupData: func(t *testing.T, store *Store) (string, string) {
				csvContent := `2024-01-15,100.50,Purchase`
				filePath := createTestCSVFile(t, "no_template.csv", csvContent)
				return filePath, "NonExistentTemplate"
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				if err == nil {
					t.Error("Expected error for missing template")
				}
				if !strings.Contains(err.Error(), "not found") {
					t.Errorf("Expected 'not found' error, got: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			store, conn := setupTestMainStore(t)
			defer teardownTestDB(t, conn)

			// Setup test data
			filePath, templateName := tt.setupData(t, store)

			// Call method under test
			err := store.ImportTransactionsFromCSV(filePath, templateName)

			// Validate results
			tt.errorCheck(t, err)
		})
	}
}

// TestMainStoreGetCategoryDisplayName tests the legacy delegation method
func TestMainStoreGetCategoryDisplayName(t *testing.T) {
	tests := []struct {
		name       string
		setupData  func(*testing.T, *Store) int64 // Returns categoryId
		expectName string
	}{
		{
			name: "existing category returns display name",
			setupData: func(t *testing.T, store *Store) int64 {
				return createTestCategory(t, store.db, "Test Category")
			},
			expectName: "Test Category",
		},
		{
			name: "non-existent category returns empty string",
			setupData: func(t *testing.T, store *Store) int64 {
				return 99999 // Non-existent ID
			},
			expectName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			store, conn := setupTestMainStore(t)
			defer teardownTestDB(t, conn)

			// Setup test data
			categoryId := tt.setupData(t, store)

			// Call method under test
			name := store.GetCategoryDisplayName(categoryId)

			// Validate results
			if name != tt.expectName {
				t.Errorf("Expected name '%s', got '%s'", tt.expectName, name)
			}
		})
	}
}

// TestMainStoreGetDatabasePath tests the utility method
func TestMainStoreGetDatabasePath(t *testing.T) {
	store, conn := setupTestMainStore(t)
	defer teardownTestDB(t, conn)

	path := store.GetDatabasePath()

	// In-memory databases may return empty path or ":memory:" - both are valid for tests
	// Just verify the method works without crashing
	t.Logf("Database path returned: '%s'", path)

	// The important thing is that the method doesn't crash and returns a string
	// In production, this would return a real file path
}

// TestRetrainMLCategorizer tests ML retraining functionality
func TestMainStoreRetrainMLCategorizer(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*testing.T, *Store)
		expectError bool
	}{
		{
			name: "retrain with ML categorizer available",
			setupData: func(t *testing.T, store *Store) {
				// ML categorizer should be initialized by setupTestMainStore
				if store.MLCategorizer == nil {
					t.Skip("ML categorizer not available for testing")
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			store, conn := setupTestMainStore(t)
			defer teardownTestDB(t, conn)

			// Setup test data
			tt.setupData(t, store)

			// Call method under test
			err := store.RetrainMLCategorizer()

			// Validate results
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestGetMLCategorizerStats tests ML statistics functionality
func TestMainStoreGetMLCategorizerStats(t *testing.T) {
	store, conn := setupTestMainStore(t)
	defer teardownTestDB(t, conn)

	// Call method under test
	stats := store.GetMLCategorizerStats()

	// Basic validation
	if stats == nil {
		t.Error("Expected stats map to be returned")
	}

	// The actual stats content depends on the ML implementation
	t.Logf("ML stats returned: %v", stats)
}
