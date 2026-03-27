package storage

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"budget-tracker-tui/internal/database"
	"budget-tracker-tui/internal/types"
)

// setupTestBankStatementStore creates a complete Store with test dependencies for bank statement testing
func setupTestBankStatementStore(t *testing.T) (*Store, *database.Connection) {
	t.Helper()

	conn := setupTestDB(t)

	// Create a complete store setup like production
	store := &Store{}
	store.db = conn

	// Initialize domain stores
	store.Categories = NewCategoryStore(conn)
	store.Templates = NewCSVTemplateStore(conn)
	store.Statements = NewBankStatementStore(conn)
	store.Transactions = NewTransactionStore(conn)
	store.TransactionAudits = NewTransactionAuditStore(conn)

	// Set cross-references between stores like production
	store.Transactions.SetTransactionAuditStore(store.TransactionAudits)
	store.Transactions.SetStore(store)
	store.Statements.SetTransactionStore(store.Transactions)

	// Initialize CSV parser with dependencies (no ML for tests)
	store.CSVParser = NewCSVParser(store.Transactions, store.Categories, nil)

	return store, conn
}

// Test fixture helpers for bank statements

// createTestBankStatementWithStatus creates a bank statement with specific status and returns its ID
func createTestBankStatementWithStatus(t *testing.T, conn *database.Connection, filename, status string) int64 {
	t.Helper()

	// Create a CSV template first (required by foreign key)
	templateId := createTestCSVTemplate(t, conn, "template_for_"+filename)

	query := `INSERT INTO bank_statements (filename, import_date, period_start, period_end, 
	                                      template_used, tx_count, status, processing_time, 
	                                      created_at, updated_at) 
	          VALUES (?, ?, ?, ?, ?, 0, ?, 100, ?, ?)`

	now := time.Now()
	nowStr := now.Format(time.RFC3339)
	dateStr := now.Format("2006-01-02")

	result, err := conn.DB.Exec(query, filename, nowStr, dateStr, dateStr, templateId, status, nowStr, nowStr)
	if err != nil {
		t.Fatalf("Failed to create test bank statement: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get statement ID: %v", err)
	}

	return id
}

// createTestBankStatementWithPeriod creates a bank statement with specific period dates and template
func createTestBankStatementWithPeriod(t *testing.T, conn *database.Connection, filename, periodStart, periodEnd string, templateId int64) int64 {
	t.Helper()

	query := `INSERT INTO bank_statements (filename, import_date, period_start, period_end, 
	                                      template_used, tx_count, status, processing_time, 
	                                      created_at, updated_at) 
	          VALUES (?, ?, ?, ?, ?, 0, 'completed', 100, ?, ?)`

	now := time.Now()
	nowStr := now.Format(time.RFC3339)

	result, err := conn.DB.Exec(query, filename, nowStr, periodStart, periodEnd, templateId, nowStr, nowStr)
	if err != nil {
		t.Fatalf("Failed to create test bank statement with period: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get statement ID: %v", err)
	}

	return id
}

// assertBankStatementEqual compares two bank statements for equality (ignoring timestamps)
func assertBankStatementEqual(t *testing.T, expected, actual types.BankStatement) {
	t.Helper()

	if actual.Id != expected.Id {
		t.Errorf("ID: expected %d, got %d", expected.Id, actual.Id)
	}
	if actual.Filename != expected.Filename {
		t.Errorf("Filename: expected %s, got %s", expected.Filename, actual.Filename)
	}
	if actual.Status != expected.Status {
		t.Errorf("Status: expected %s, got %s", expected.Status, actual.Status)
	}
	if actual.TxCount != expected.TxCount {
		t.Errorf("TxCount: expected %d, got %d", expected.TxCount, actual.TxCount)
	}
	if actual.TemplateUsed != expected.TemplateUsed {
		t.Errorf("TemplateUsed: expected %d, got %d", expected.TemplateUsed, actual.TxCount)
	}
}

// ===========================
// P1 Priority Method Tests
// ===========================

// TestGetStatementHistory tests the GetStatementHistory method
func TestGetStatementHistory(t *testing.T) {
	tests := []struct {
		name      string
		setupData func(*testing.T, *Store, *database.Connection) []int64
		validate  func(*testing.T, []types.BankStatement, []int64)
	}{
		{
			name: "Empty history returns empty slice",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) []int64 {
				return []int64{} // No statements created
			},
			validate: func(t *testing.T, statements []types.BankStatement, expectedIds []int64) {
				if len(statements) != 0 {
					t.Errorf("Expected empty slice, got %d statements", len(statements))
				}
			},
		},
		{
			name: "Single statement returned correctly",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) []int64 {
				id := createTestBankStatementWithStatus(t, conn, "test_statement.csv", "completed")
				return []int64{id}
			},
			validate: func(t *testing.T, statements []types.BankStatement, expectedIds []int64) {
				if len(statements) != 1 {
					t.Fatalf("Expected 1 statement, got %d", len(statements))
				}
				if statements[0].Id != expectedIds[0] {
					t.Errorf("Expected ID %d, got %d", expectedIds[0], statements[0].Id)
				}
				if statements[0].Filename != "test_statement.csv" {
					t.Errorf("Expected filename 'test_statement.csv', got %s", statements[0].Filename)
				}
				if statements[0].Status != "completed" {
					t.Errorf("Expected status 'completed', got %s", statements[0].Status)
				}
			},
		},
		{
			name: "Multiple statements ordered by import date descending",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) []int64 {
				// Create statements with explicit dates to ensure order
				templateId := createTestCSVTemplate(t, conn, "template1")

				// Create older statement first
				stmt1Query := `INSERT INTO bank_statements (filename, import_date, period_start, period_end, 
				                                           template_used, tx_count, status, processing_time, 
				                                           created_at, updated_at) 
				              VALUES (?, ?, ?, ?, ?, 0, 'completed', 100, ?, ?)`

				older := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
				dateStr := time.Now().Format("2006-01-02")

				result1, err := conn.DB.Exec(stmt1Query, "older_statement.csv", older, dateStr, dateStr, templateId, older, older)
				if err != nil {
					t.Fatalf("Failed to create older statement: %v", err)
				}
				id1, _ := result1.LastInsertId()

				// Create newer statement
				newer := time.Now().Format(time.RFC3339)
				result2, err := conn.DB.Exec(stmt1Query, "newer_statement.csv", newer, dateStr, dateStr, templateId, newer, newer)
				if err != nil {
					t.Fatalf("Failed to create newer statement: %v", err)
				}
				id2, _ := result2.LastInsertId()

				return []int64{id1, id2} // Return [older_id, newer_id]
			},
			validate: func(t *testing.T, statements []types.BankStatement, expectedIds []int64) {
				if len(statements) != 2 {
					t.Fatalf("Expected 2 statements, got %d", len(statements))
				}
				// Should be ordered by import_date DESC (newer first)
				if statements[0].Filename != "newer_statement.csv" {
					t.Errorf("Expected newer statement first, got %s", statements[0].Filename)
				}
				if statements[1].Filename != "older_statement.csv" {
					t.Errorf("Expected older statement second, got %s", statements[1].Filename)
				}
			},
		},
		{
			name: "Statements with different statuses all returned",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) []int64 {
				completed := createTestBankStatementWithStatus(t, conn, "completed.csv", "completed")
				failed := createTestBankStatementWithStatus(t, conn, "failed.csv", "failed")
				undone := createTestBankStatementWithStatus(t, conn, "undone.csv", "undone")
				return []int64{completed, failed, undone}
			},
			validate: func(t *testing.T, statements []types.BankStatement, expectedIds []int64) {
				if len(statements) != 3 {
					t.Fatalf("Expected 3 statements, got %d", len(statements))
				}
				// Verify all statuses are present
				statusesSeen := make(map[string]bool)
				for _, stmt := range statements {
					statusesSeen[stmt.Status] = true
				}
				expectedStatuses := []string{"completed", "failed", "undone"}
				for _, status := range expectedStatuses {
					if !statusesSeen[status] {
						t.Errorf("Expected to see status %s in results", status)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, conn := setupTestBankStatementStore(t)
			defer teardownTestDB(t, conn)

			expectedIds := tt.setupData(t, store, conn)
			statements := store.Statements.GetStatementHistory()
			tt.validate(t, statements, expectedIds)
		})
	}
}

// TestRecordBankStatement tests the RecordBankStatement method
func TestRecordBankStatement(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*testing.T, *Store, *database.Connection) int64 // Returns templateId
		filename    string
		periodStart time.Time
		periodEnd   time.Time
		txCount     int
		status      string
		validate    func(*testing.T, *Store, *database.Connection, int64, int64) // statementId, templateId
		expectError bool
	}{
		{
			name: "Successfully record statement with valid data",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestCSVTemplate(t, conn, "valid_template")
			},
			filename:    "test_statement.csv",
			periodStart: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			periodEnd:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
			txCount:     10,
			status:      "completed",
			validate: func(t *testing.T, store *Store, conn *database.Connection, statementId, templateId int64) {
				// Retrieve the created statement
				stmt, err := store.Statements.GetStatementById(statementId)
				if err != nil {
					t.Fatalf("Error retrieving statement: %v", err)
				}
				if stmt == nil {
					t.Fatal("Failed to retrieve created statement")
				}

				if stmt.Filename != "test_statement.csv" {
					t.Errorf("Expected filename 'test_statement.csv', got %s", stmt.Filename)
				}
				if stmt.TxCount != 10 {
					t.Errorf("Expected tx_count 10, got %d", stmt.TxCount)
				}
				if stmt.Status != "completed" {
					t.Errorf("Expected status 'completed', got %s", stmt.Status)
				}
				if stmt.TemplateUsed != templateId {
					t.Errorf("Expected template_used %d, got %d", templateId, stmt.TemplateUsed)
				}

				// Check period dates are stored correctly
				expectedStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
				expectedEnd := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
				if !stmt.PeriodStart.Equal(expectedStart) {
					t.Errorf("Expected period_start %v, got %v", expectedStart, stmt.PeriodStart)
				}
				if !stmt.PeriodEnd.Equal(expectedEnd) {
					t.Errorf("Expected period_end %v, got %v", expectedEnd, stmt.PeriodEnd)
				}
			},
			expectError: false,
		},
		{
			name: "Record statement with zero time periods (nullable fields)",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestCSVTemplate(t, conn, "nullable_template")
			},
			filename:    "nullable_periods.csv",
			periodStart: time.Time{}, // Zero time
			periodEnd:   time.Time{}, // Zero time
			txCount:     5,
			status:      "importing",
			validate: func(t *testing.T, store *Store, conn *database.Connection, statementId, templateId int64) {
				stmt, err := store.Statements.GetStatementById(statementId)
				if err != nil {
					t.Fatalf("Error retrieving statement: %v", err)
				}
				if stmt == nil {
					t.Fatal("Failed to retrieve created statement")
				}

				// Zero times should be handled gracefully
				if stmt.TxCount != 5 {
					t.Errorf("Expected tx_count 5, got %d", stmt.TxCount)
				}
				if stmt.Status != "importing" {
					t.Errorf("Expected status 'importing', got %s", stmt.Status)
				}
			},
			expectError: false,
		},
		{
			name: "Record statement returns valid ID",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestCSVTemplate(t, conn, "id_test_template")
			},
			filename:    "id_test.csv",
			periodStart: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			periodEnd:   time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC),
			txCount:     0,
			status:      "failed",
			validate: func(t *testing.T, store *Store, conn *database.Connection, statementId, templateId int64) {
				if statementId <= 0 {
					t.Errorf("Expected positive statement ID, got %d", statementId)
				}

				// Verify ID is unique by checking retrieval
				stmt, err := store.Statements.GetStatementById(statementId)
				if err != nil {
					t.Fatalf("Error retrieving statement: %v", err)
				}
				if stmt == nil {
					t.Fatal("Created statement should be retrievable by returned ID")
				}
				if stmt.Id != statementId {
					t.Errorf("Statement ID mismatch: expected %d, got %d", statementId, stmt.Id)
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, conn := setupTestBankStatementStore(t)
			defer teardownTestDB(t, conn)

			templateId := tt.setupData(t, store, conn)

			periodStartStr := ""
			periodEndStr := ""
			if !tt.periodStart.IsZero() {
				periodStartStr = tt.periodStart.Format("2006-01-02")
			}
			if !tt.periodEnd.IsZero() {
				periodEndStr = tt.periodEnd.Format("2006-01-02")
			}

			statementId, err := store.Statements.RecordBankStatement(
				tt.filename, periodStartStr, periodEndStr, templateId, tt.txCount, tt.status,
			)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				tt.validate(t, store, conn, statementId, templateId)
			}
		})
	}
}

// TestCanUndoImport tests the CanUndoImport method
func TestCanUndoImport(t *testing.T) {
	tests := []struct {
		name          string
		setupData     func(*testing.T, *Store, *database.Connection) int64 // Returns statement ID
		expectCanUndo bool
	}{
		{
			name: "Can undo completed statement",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestBankStatementWithStatus(t, conn, "completed.csv", "completed")
			},
			expectCanUndo: true,
		},
		{
			name: "Can undo override statement",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestBankStatementWithStatus(t, conn, "override.csv", "override")
			},
			expectCanUndo: true,
		},
		{
			name: "Cannot undo failed statement",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestBankStatementWithStatus(t, conn, "failed.csv", "failed")
			},
			expectCanUndo: false,
		},
		{
			name: "Cannot undo undone statement",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestBankStatementWithStatus(t, conn, "undone.csv", "undone")
			},
			expectCanUndo: false,
		},
		{
			name: "Cannot undo importing statement",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestBankStatementWithStatus(t, conn, "importing.csv", "importing")
			},
			expectCanUndo: false,
		},
		{
			name: "Cannot undo non-existent statement",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return 99999 // Non-existent ID
			},
			expectCanUndo: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, conn := setupTestBankStatementStore(t)
			defer teardownTestDB(t, conn)

			statementId := tt.setupData(t, store, conn)
			canUndo := store.Statements.CanUndoImport(statementId)

			if canUndo != tt.expectCanUndo {
				t.Errorf("Expected CanUndoImport to return %t, got %t", tt.expectCanUndo, canUndo)
			}
		})
	}
}

// TestDetectOverlap tests the DetectOverlap method
func TestDetectOverlap(t *testing.T) {
	tests := []struct {
		name          string
		setupData     func(*testing.T, *Store, *database.Connection) int64 // Returns template ID
		periodStart   time.Time
		periodEnd     time.Time
		expectOverlap bool
		overlapCount  int
	}{
		{
			name: "No overlap with empty database",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestCSVTemplate(t, conn, "no_overlap_template")
			},
			periodStart:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			periodEnd:     time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
			expectOverlap: false,
			overlapCount:  0,
		},
		{
			name: "No overlap with non-overlapping periods",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				templateId := createTestCSVTemplate(t, conn, "template1")
				// Create existing statement for Jan 2024 using the same template
				createTestBankStatementWithPeriod(t, conn, "jan2024.csv", "2024-01-01", "2024-01-31", templateId)
				return templateId
			},
			periodStart:   time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), // Feb 2024
			periodEnd:     time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC),
			expectOverlap: false,
			overlapCount:  0,
		},
		{
			name: "Overlap detected - exact period match",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				templateId := createTestCSVTemplate(t, conn, "template2")
				// Create existing statement for Jan 2024 using the same template
				createTestBankStatementWithPeriod(t, conn, "jan2024.csv", "2024-01-01", "2024-01-31", templateId)
				return templateId
			},
			periodStart:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), // Same period
			periodEnd:     time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
			expectOverlap: true,
			overlapCount:  1,
		},
		{
			name: "Overlap detected - partial overlap (start)",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				templateId := createTestCSVTemplate(t, conn, "template3")
				// Create existing statement for Jan 15-31 using the same template
				createTestBankStatementWithPeriod(t, conn, "partial.csv", "2024-01-15", "2024-01-31", templateId)
				return templateId
			},
			periodStart:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), // Jan 1-20 overlaps Jan 15-31
			periodEnd:     time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
			expectOverlap: true,
			overlapCount:  1,
		},
		{
			name: "Overlap detected - partial overlap (end)",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				templateId := createTestCSVTemplate(t, conn, "template4")
				// Create existing statement for Jan 1-15 using the same template
				createTestBankStatementWithPeriod(t, conn, "partial2.csv", "2024-01-01", "2024-01-15", templateId)
				return templateId
			},
			periodStart:   time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), // Jan 10-25 overlaps Jan 1-15
			periodEnd:     time.Date(2024, 1, 25, 0, 0, 0, 0, time.UTC),
			expectOverlap: true,
			overlapCount:  1,
		},
		{
			name: "Overlap detected - multiple overlapping statements",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				templateId := createTestCSVTemplate(t, conn, "template5")
				// Create multiple overlapping statements using the same template
				createTestBankStatementWithPeriod(t, conn, "overlap1.csv", "2024-01-01", "2024-01-15", templateId)
				createTestBankStatementWithPeriod(t, conn, "overlap2.csv", "2024-01-10", "2024-01-25", templateId)
				return templateId
			},
			periodStart:   time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC), // Overlaps with both
			periodEnd:     time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
			expectOverlap: true,
			overlapCount:  2,
		},
		{
			name: "No overlap - adjacent periods (boundary test)",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				templateId := createTestCSVTemplate(t, conn, "template6")
				// Create statement ending Jan 31 using the same template
				createTestBankStatementWithPeriod(t, conn, "jan.csv", "2024-01-01", "2024-01-31", templateId)
				return templateId
			},
			periodStart:   time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), // Starts Feb 1 (no overlap)
			periodEnd:     time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC),
			expectOverlap: false,
			overlapCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, conn := setupTestBankStatementStore(t)
			defer teardownTestDB(t, conn)

			templateId := tt.setupData(t, store, conn)
			periodStartStr := tt.periodStart.Format("2006-01-02")
			periodEndStr := tt.periodEnd.Format("2006-01-02")
			overlaps := store.Statements.DetectOverlap(periodStartStr, periodEndStr, templateId)

			hasOverlap := len(overlaps) > 0
			if hasOverlap != tt.expectOverlap {
				t.Errorf("Expected overlap %t, got %t (overlaps: %v)", tt.expectOverlap, hasOverlap, overlaps)
			}

			if len(overlaps) != tt.overlapCount {
				t.Errorf("Expected %d overlaps, got %d", tt.overlapCount, len(overlaps))
			}
		})
	}
}

// TestMarkStatementCompleted tests the MarkStatementCompleted method
func TestMarkStatementCompleted(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*testing.T, *Store, *database.Connection) int64 // Returns statement ID
		expectError    bool
		validateStatus func(*testing.T, *Store, *database.Connection, int64) // Validate final status
	}{
		{
			name: "Successfully mark importing statement as completed",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestBankStatementWithStatus(t, conn, "importing.csv", "importing")
			},
			expectError: false,
			validateStatus: func(t *testing.T, store *Store, conn *database.Connection, statementId int64) {
				stmt, err := store.Statements.GetStatementById(statementId)
				if err != nil {
					t.Fatalf("Error retrieving statement: %v", err)
				}
				if stmt == nil {
					t.Fatal("Statement should exist after marking completed")
				}
				if stmt.Status != "completed" {
					t.Errorf("Expected status 'completed', got %s", stmt.Status)
				}
			},
		},
		{
			name: "Successfully mark failed statement as completed",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestBankStatementWithStatus(t, conn, "failed.csv", "failed")
			},
			expectError: false,
			validateStatus: func(t *testing.T, store *Store, conn *database.Connection, statementId int64) {
				stmt, err := store.Statements.GetStatementById(statementId)
				if err != nil {
					t.Fatalf("Error retrieving statement: %v", err)
				}
				if stmt != nil && stmt.Status != "completed" {
					t.Errorf("Expected status 'completed', got %s", stmt.Status)
				}
			},
		},
		{
			name: "Error when marking non-existent statement",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return 99999 // Non-existent ID
			},
			expectError: true,
			validateStatus: func(t *testing.T, store *Store, conn *database.Connection, statementId int64) {
				// Should not exist
				stmt, _ := store.Statements.GetStatementById(statementId)
				if stmt != nil {
					t.Error("Non-existent statement should remain non-existent")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, conn := setupTestBankStatementStore(t)
			defer teardownTestDB(t, conn)

			statementId := tt.setupData(t, store, conn)
			err := store.Statements.MarkStatementCompleted(statementId)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			tt.validateStatus(t, store, conn, statementId)
		})
	}
}

// TestMarkStatementUndone tests the MarkStatementUndone method
func TestMarkStatementUndone(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*testing.T, *Store, *database.Connection) int64
		expectError    bool
		validateStatus func(*testing.T, *Store, *database.Connection, int64)
	}{
		{
			name: "Successfully mark completed statement as undone",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestBankStatementWithStatus(t, conn, "completed.csv", "completed")
			},
			expectError: false,
			validateStatus: func(t *testing.T, store *Store, conn *database.Connection, statementId int64) {
				stmt, err := store.Statements.GetStatementById(statementId)
				if err != nil {
					t.Fatalf("Error retrieving statement: %v", err)
				}
				if stmt == nil {
					t.Fatal("Statement should exist after marking undone")
				}
				if stmt.Status != "undone" {
					t.Errorf("Expected status 'undone', got %s", stmt.Status)
				}
			},
		},
		{
			name: "Successfully mark override statement as undone",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestBankStatementWithStatus(t, conn, "override.csv", "override")
			},
			expectError: false,
			validateStatus: func(t *testing.T, store *Store, conn *database.Connection, statementId int64) {
				stmt, err := store.Statements.GetStatementById(statementId)
				if err != nil {
					t.Fatalf("Error retrieving statement: %v", err)
				}
				if stmt != nil && stmt.Status != "undone" {
					t.Errorf("Expected status 'undone', got %s", stmt.Status)
				}
			},
		},
		{
			name: "Error when marking non-existent statement as undone",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return 99999 // Non-existent ID
			},
			expectError: true,
			validateStatus: func(t *testing.T, store *Store, conn *database.Connection, statementId int64) {
				stmt, _ := store.Statements.GetStatementById(statementId)
				if stmt != nil {
					t.Error("Non-existent statement should remain non-existent")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, conn := setupTestBankStatementStore(t)
			defer teardownTestDB(t, conn)

			statementId := tt.setupData(t, store, conn)
			err := store.Statements.MarkStatementUndone(statementId)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			tt.validateStatus(t, store, conn, statementId)
		})
	}
}

// ===========================
// P2 Priority Method Tests
// ===========================

// TestGetStatementById tests the GetStatementById method
func TestGetStatementById(t *testing.T) {
	tests := []struct {
		name      string
		setupData func(*testing.T, *Store, *database.Connection) int64 // Returns statement ID or test ID
		testId    int64                                                // ID to query for (may be different from setupData return for edge cases)
		expectNil bool
	}{
		{
			name: "Successfully retrieve existing statement",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestBankStatementWithStatus(t, conn, "existing.csv", "completed")
			},
			testId:    0, // Will be set to setupData return value
			expectNil: false,
		},
		{
			name: "Return nil for non-existent statement",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				// Create a statement but return a different ID to test
				createTestBankStatementWithStatus(t, conn, "real.csv", "completed")
				return 99999 // Non-existent ID to test
			},
			testId:    99999,
			expectNil: true,
		},
		{
			name: "Successfully retrieve statement with different statuses",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestBankStatementWithStatus(t, conn, "failed_stmt.csv", "failed")
			},
			testId:    0, // Will be set to setupData return value
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, conn := setupTestBankStatementStore(t)
			defer teardownTestDB(t, conn)

			statementId := tt.setupData(t, store, conn)
			testId := tt.testId
			if testId == 0 {
				testId = statementId // Use the actual created ID
			}

			stmt, err := store.Statements.GetStatementById(testId)

			if tt.expectNil {
				if err == nil && stmt != nil {
					t.Error("Expected nil statement but got one")
				}
			} else {
				if err != nil {
					t.Fatalf("Error retrieving statement: %v", err)
				}
				if stmt == nil {
					t.Fatal("Expected statement but got nil")
				}
			}

			if !tt.expectNil {
				if stmt.Id != testId {
					t.Errorf("Expected ID %d, got %d", testId, stmt.Id)
				}
			}
		})
	}
}

// TestMarkStatementFailed tests the MarkStatementFailed method
func TestMarkStatementFailed(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*testing.T, *Store, *database.Connection) int64
		errorMsg    string
		expectError bool
		validate    func(*testing.T, *Store, *database.Connection, int64, string)
	}{
		{
			name: "Successfully mark importing statement as failed with error message",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestBankStatementWithStatus(t, conn, "importing.csv", "importing")
			},
			errorMsg:    "CSV parsing failed: invalid format",
			expectError: false,
			validate: func(t *testing.T, store *Store, conn *database.Connection, statementId int64, errorMsg string) {
				stmt, err := store.Statements.GetStatementById(statementId)
				if err != nil {
					t.Fatalf("Error retrieving statement: %v", err)
				}
				if stmt == nil {
					t.Fatal("Statement should exist after marking failed")
				}
				if stmt.Status != "failed" {
					t.Errorf("Expected status 'failed', got %s", stmt.Status)
				}
				if stmt.ErrorLog != errorMsg {
					t.Errorf("Expected error log '%s', got '%s'", errorMsg, stmt.ErrorLog)
				}
			},
		},
		{
			name: "Successfully mark completed statement as failed",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestBankStatementWithStatus(t, conn, "completed.csv", "completed")
			},
			errorMsg:    "Post-processing validation failed",
			expectError: false,
			validate: func(t *testing.T, store *Store, conn *database.Connection, statementId int64, errorMsg string) {
				stmt, err := store.Statements.GetStatementById(statementId)
				if err != nil {
					t.Fatalf("Error retrieving statement: %v", err)
				}
				if stmt != nil {
					if stmt.Status != "failed" {
						t.Errorf("Expected status 'failed', got %s", stmt.Status)
					}
					if stmt.ErrorLog != errorMsg {
						t.Errorf("Expected error log '%s', got '%s'", errorMsg, stmt.ErrorLog)
					}
				}
			},
		},
		{
			name: "Error when marking non-existent statement as failed",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return 99999 // Non-existent ID
			},
			errorMsg:    "Some error message",
			expectError: true,
			validate: func(t *testing.T, store *Store, conn *database.Connection, statementId int64, errorMsg string) {
				// Should not exist
				stmt, _ := store.Statements.GetStatementById(statementId)
				if stmt != nil {
					t.Error("Non-existent statement should remain non-existent")
				}
			},
		},
		{
			name: "Mark statement failed with empty error message",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestBankStatementWithStatus(t, conn, "empty_error.csv", "importing")
			},
			errorMsg:    "",
			expectError: false,
			validate: func(t *testing.T, store *Store, conn *database.Connection, statementId int64, errorMsg string) {
				stmt, err := store.Statements.GetStatementById(statementId)
				if err != nil {
					t.Fatalf("Error retrieving statement: %v", err)
				}
				if stmt == nil {
					t.Fatal("Statement should exist after marking failed")
				}
				if stmt.Status != "failed" {
					t.Errorf("Expected status 'failed', got %s", stmt.Status)
				}
				// Empty error message should be stored as is
				if stmt.ErrorLog != "" {
					t.Errorf("Expected empty error log, got '%s'", stmt.ErrorLog)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, conn := setupTestBankStatementStore(t)
			defer teardownTestDB(t, conn)

			statementId := tt.setupData(t, store, conn)
			err := store.Statements.MarkStatementFailed(statementId, tt.errorMsg)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			tt.validate(t, store, conn, statementId, tt.errorMsg)
		})
	}
}

// TestNextId tests the NextId method
func TestNextId(t *testing.T) {
	tests := []struct {
		name      string
		setupData func(*testing.T, *Store, *database.Connection) []int64         // Returns created statement IDs
		validate  func(*testing.T, *Store, *database.Connection, int64, []int64) // nextId, createdIds
	}{
		{
			name: "NextId returns 1 for empty database",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) []int64 {
				return []int64{} // No statements created
			},
			validate: func(t *testing.T, store *Store, conn *database.Connection, nextId int64, createdIds []int64) {
				if nextId != 1 {
					t.Errorf("Expected NextId to return 1 for empty database, got %d", nextId)
				}
			},
		},
		{
			name: "NextId returns max ID + 1 with existing statements",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) []int64 {
				id1 := createTestBankStatementWithStatus(t, conn, "stmt1.csv", "completed")
				id2 := createTestBankStatementWithStatus(t, conn, "stmt2.csv", "completed")
				id3 := createTestBankStatementWithStatus(t, conn, "stmt3.csv", "failed")
				return []int64{id1, id2, id3}
			},
			validate: func(t *testing.T, store *Store, conn *database.Connection, nextId int64, createdIds []int64) {
				// Find the maximum ID from created statements
				maxId := int64(0)
				for _, id := range createdIds {
					if id > maxId {
						maxId = id
					}
				}
				expectedNextId := maxId + 1
				if nextId != expectedNextId {
					t.Errorf("Expected NextId to return %d (max %d + 1), got %d", expectedNextId, maxId, nextId)
				}
			},
		},
		{
			name: "NextId handles single statement correctly",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) []int64 {
				id := createTestBankStatementWithStatus(t, conn, "single.csv", "undone")
				return []int64{id}
			},
			validate: func(t *testing.T, store *Store, conn *database.Connection, nextId int64, createdIds []int64) {
				expectedNextId := createdIds[0] + 1
				if nextId != expectedNextId {
					t.Errorf("Expected NextId to return %d, got %d", expectedNextId, nextId)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, conn := setupTestBankStatementStore(t)
			defer teardownTestDB(t, conn)

			createdIds := tt.setupData(t, store, conn)
			nextId := store.Statements.NextId()
			tt.validate(t, store, conn, nextId, createdIds)
		})
	}
}

// TestDeleteStatement tests the DeleteStatement method
func TestDeleteStatement(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*testing.T, *Store, *database.Connection) int64 // Returns statement ID
		expectError bool
		validate    func(*testing.T, *Store, *database.Connection, int64)
	}{
		{
			name: "Successfully delete existing statement",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestBankStatementWithStatus(t, conn, "to_delete.csv", "failed")
			},
			expectError: false,
			validate: func(t *testing.T, store *Store, conn *database.Connection, statementId int64) {
				// Statement should no longer exist
				stmt, _ := store.Statements.GetStatementById(statementId)
				if stmt != nil {
					t.Error("Statement should be deleted and not retrievable")
				}

				// Verify it's not in the history either
				history := store.Statements.GetStatementHistory()
				for _, histStmt := range history {
					if histStmt.Id == statementId {
						t.Error("Deleted statement should not appear in history")
					}
				}
			},
		},
		{
			name: "Error when deleting non-existent statement",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return 99999 // Non-existent ID
			},
			expectError: true,
			validate: func(t *testing.T, store *Store, conn *database.Connection, statementId int64) {
				// Nothing should change since statement didn't exist
			},
		},
		{
			name: "Delete statement with different status values",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) int64 {
				return createTestBankStatementWithStatus(t, conn, "undone_delete.csv", "undone")
			},
			expectError: false,
			validate: func(t *testing.T, store *Store, conn *database.Connection, statementId int64) {
				stmt, _ := store.Statements.GetStatementById(statementId)
				if stmt != nil {
					t.Error("Statement should be deleted regardless of status")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, conn := setupTestBankStatementStore(t)
			defer teardownTestDB(t, conn)

			statementId := tt.setupData(t, store, conn)
			err := store.Statements.DeleteStatement(statementId)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			tt.validate(t, store, conn, statementId)
		})
	}
}

// TestUndoImport tests the UndoImport method (cross-domain operation)
func TestUndoImport(t *testing.T) {
	tests := []struct {
		name         string
		setupData    func(*testing.T, *Store, *database.Connection) (int64, int) // Returns statement ID, expected tx count
		expectError  bool
		errorMsg     string
		validateUndo func(*testing.T, *Store, *database.Connection, int64, int, int) // statementId, expectedTxCount, actualRemovedCount
	}{
		{
			name: "Successfully undo import with transactions",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) (int64, int) {
				categoryId := createTestCategory(t, conn, "Test Category")
				statementId := createTestBankStatementWithStatus(t, conn, "undo_test.csv", "completed")

				// Create 3 transactions associated with this statement
				for i := 0; i < 3; i++ {
					tx := createTestTransaction(100.00*float64(i+1), fmt.Sprintf("Transaction %d", i+1), categoryId)
					tx.StatementId = statementId
					if err := store.Transactions.SaveTransaction(tx); err != nil {
						t.Fatalf("Failed to save test transaction: %v", err)
					}
				}
				return statementId, 3
			},
			expectError: false,
			validateUndo: func(t *testing.T, store *Store, conn *database.Connection, statementId int64, expectedTxCount, actualRemovedCount int) {
				// Check that the correct number of transactions were removed
				if actualRemovedCount != expectedTxCount {
					t.Errorf("Expected %d transactions to be removed, got %d", expectedTxCount, actualRemovedCount)
				}

				// Verify transactions are actually deleted
				remainingTx, err := store.Transactions.GetTransactionsByStatement(statementId)
				if err != nil {
					t.Fatalf("Error checking remaining transactions: %v", err)
				}
				if len(remainingTx) != 0 {
					t.Errorf("Expected 0 remaining transactions, got %d", len(remainingTx))
				}

				// Verify statement status was updated to "undone"
				stmt, err := store.Statements.GetStatementById(statementId)
				if err != nil {
					t.Fatalf("Error retrieving statement: %v", err)
				}
				if stmt == nil {
					t.Fatal("Statement should still exist after undo")
				}
				if stmt.Status != "undone" {
					t.Errorf("Expected statement status 'undone', got %s", stmt.Status)
				}
			},
		},
		{
			name: "Successfully undo import with no transactions",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) (int64, int) {
				statementId := createTestBankStatementWithStatus(t, conn, "empty_undo.csv", "completed")
				return statementId, 0
			},
			expectError: false,
			validateUndo: func(t *testing.T, store *Store, conn *database.Connection, statementId int64, expectedTxCount, actualRemovedCount int) {
				if actualRemovedCount != 0 {
					t.Errorf("Expected 0 transactions to be removed, got %d", actualRemovedCount)
				}

				// Statement should still be marked as undone
				stmt, err := store.Statements.GetStatementById(statementId)
				if err != nil {
					t.Fatalf("Error retrieving statement: %v", err)
				}
				if stmt != nil && stmt.Status != "undone" {
					t.Errorf("Expected statement status 'undone', got %s", stmt.Status)
				}
			},
		},
		{
			name: "Error when transaction store not initialized",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) (int64, int) {
				statementId := createTestBankStatementWithStatus(t, conn, "no_tx_store.csv", "completed")

				// Simulate uninitialized transaction store by setting it to nil
				store.Statements.transactions = nil
				return statementId, 0
			},
			expectError: true,
			errorMsg:    "transaction store not initialized",
			validateUndo: func(t *testing.T, store *Store, conn *database.Connection, statementId int64, expectedTxCount, actualRemovedCount int) {
				// Should have no effect when error occurs
				stmt, err := store.Statements.GetStatementById(statementId)
				if err != nil {
					t.Fatalf("Error retrieving statement: %v", err)
				}
				if stmt != nil && stmt.Status != "completed" {
					t.Errorf("Statement status should remain unchanged on error, got %s", stmt.Status)
				}
			},
		},
		{
			name: "Undo import with mixed statement IDs (partial removal)",
			setupData: func(t *testing.T, store *Store, conn *database.Connection) (int64, int) {
				categoryId := createTestCategory(t, conn, "Mixed Category")
				targetStatementId := createTestBankStatementWithStatus(t, conn, "target_undo.csv", "completed")
				otherStatementId := createTestBankStatementWithStatus(t, conn, "other_undo.csv", "completed")

				// Create 2 transactions for target statement and 1 for other statement
				for i := 0; i < 2; i++ {
					tx := createTestTransaction(100.00, fmt.Sprintf("Target Transaction %d", i+1), categoryId)
					tx.StatementId = targetStatementId
					if err := store.Transactions.SaveTransaction(tx); err != nil {
						t.Fatalf("Failed to save target transaction: %v", err)
					}
				}

				otherTx := createTestTransaction(200.00, "Other Transaction", categoryId)
				otherTx.StatementId = otherStatementId
				if err := store.Transactions.SaveTransaction(otherTx); err != nil {
					t.Fatalf("Failed to save other transaction: %v", err)
				}

				return targetStatementId, 2
			},
			expectError: false,
			validateUndo: func(t *testing.T, store *Store, conn *database.Connection, statementId int64, expectedTxCount, actualRemovedCount int) {
				if actualRemovedCount != expectedTxCount {
					t.Errorf("Expected %d transactions to be removed, got %d", expectedTxCount, actualRemovedCount)
				}

				// Verify only transactions for target statement were removed
				targetTx, err := store.Transactions.GetTransactionsByStatement(statementId)
				if err != nil {
					t.Fatalf("Error checking target transactions: %v", err)
				}
				if len(targetTx) != 0 {
					t.Errorf("Expected 0 target statement transactions, got %d", len(targetTx))
				}

				// Verify transactions from other statement are still there
				allTx, err := store.Transactions.GetTransactions()
				if err != nil {
					t.Fatalf("Error getting all transactions: %v", err)
				}
				if len(allTx) != 1 {
					t.Errorf("Expected 1 remaining transaction from other statement, got %d", len(allTx))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, conn := setupTestBankStatementStore(t)
			defer teardownTestDB(t, conn)

			statementId, expectedTxCount := tt.setupData(t, store, conn)
			removedCount, err := store.Statements.UndoImport(statementId)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if tt.validateUndo != nil {
				tt.validateUndo(t, store, conn, statementId, expectedTxCount, removedCount)
			}
		})
	}
}

// TestExtractPeriodFromTransactions tests the ExtractPeriodFromTransactions method
func TestExtractPeriodFromTransactions(t *testing.T) {
	tests := []struct {
		name          string
		setupTxData   func(*testing.T) []types.Transaction
		expectedStart string
		expectedEnd   string
	}{
		{
			name: "Empty transaction list returns empty strings",
			setupTxData: func(t *testing.T) []types.Transaction {
				return []types.Transaction{}
			},
			expectedStart: "",
			expectedEnd:   "",
		},
		{
			name: "Single transaction returns same start and end date",
			setupTxData: func(t *testing.T) []types.Transaction {
				return []types.Transaction{
					{
						Id:     1,
						Amount: 100.00,
						Date:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
					},
				}
			},
			expectedStart: "2024-01-15",
			expectedEnd:   "2024-01-15",
		},
		{
			name: "Multiple transactions in chronological order",
			setupTxData: func(t *testing.T) []types.Transaction {
				return []types.Transaction{
					{
						Id:     1,
						Amount: 100.00,
						Date:   time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
					},
					{
						Id:     2,
						Amount: 200.00,
						Date:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
					},
					{
						Id:     3,
						Amount: 300.00,
						Date:   time.Date(2024, 1, 25, 0, 0, 0, 0, time.UTC),
					},
				}
			},
			expectedStart: "2024-01-05",
			expectedEnd:   "2024-01-25",
		},
		{
			name: "Multiple transactions in random order",
			setupTxData: func(t *testing.T) []types.Transaction {
				return []types.Transaction{
					{
						Id:     1,
						Amount: 100.00,
						Date:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
					},
					{
						Id:     2,
						Amount: 200.00,
						Date:   time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC), // Earlier date
					},
					{
						Id:     3,
						Amount: 300.00,
						Date:   time.Date(2024, 1, 25, 0, 0, 0, 0, time.UTC), // Later date
					},
					{
						Id:     4,
						Amount: 400.00,
						Date:   time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC), // Middle date
					},
				}
			},
			expectedStart: "2024-01-05",
			expectedEnd:   "2024-01-25",
		},
		{
			name: "Transactions spanning multiple months",
			setupTxData: func(t *testing.T) []types.Transaction {
				return []types.Transaction{
					{
						Id:     1,
						Amount: 100.00,
						Date:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
					},
					{
						Id:     2,
						Amount: 200.00,
						Date:   time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC),
					},
					{
						Id:     3,
						Amount: 300.00,
						Date:   time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
					},
				}
			},
			expectedStart: "2024-01-31",
			expectedEnd:   "2024-03-01",
		},
		{
			name: "Transactions with same dates (boundary test)",
			setupTxData: func(t *testing.T) []types.Transaction {
				sameDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
				return []types.Transaction{
					{
						Id:     1,
						Amount: 100.00,
						Date:   sameDate,
					},
					{
						Id:     2,
						Amount: 200.00,
						Date:   sameDate,
					},
					{
						Id:     3,
						Amount: 300.00,
						Date:   sameDate,
					},
				}
			},
			expectedStart: "2024-01-15",
			expectedEnd:   "2024-01-15",
		},
		{
			name: "Transactions across year boundary",
			setupTxData: func(t *testing.T) []types.Transaction {
				return []types.Transaction{
					{
						Id:     1,
						Amount: 100.00,
						Date:   time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
					},
					{
						Id:     2,
						Amount: 200.00,
						Date:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					},
				}
			},
			expectedStart: "2023-12-31",
			expectedEnd:   "2024-01-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, conn := setupTestBankStatementStore(t)
			defer teardownTestDB(t, conn)

			transactions := tt.setupTxData(t)
			start, end := store.Statements.ExtractPeriodFromTransactions(transactions)

			if start != tt.expectedStart {
				t.Errorf("Expected start date %s, got %s", tt.expectedStart, start)
			}
			if end != tt.expectedEnd {
				t.Errorf("Expected end date %s, got %s", tt.expectedEnd, end)
			}
		})
	}
}
