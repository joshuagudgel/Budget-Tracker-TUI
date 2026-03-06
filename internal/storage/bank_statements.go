package storage

import (
	"budget-tracker-tui/internal/database"
	"budget-tracker-tui/internal/types"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BankStatementStore handles all bank statement-related operations using SQLite
type BankStatementStore struct {
	db     *database.Connection
	helper *database.SQLHelper
}

// NewBankStatementStore creates a new BankStatementStore instance
func NewBankStatementStore(db *database.Connection) *BankStatementStore {
	return &BankStatementStore{
		db:     db,
		helper: database.NewSQLHelper(db),
	}
}

// NextId calculates the next available ID for bank statements
func (bs *BankStatementStore) NextId() int64 {
	maxID, err := bs.helper.GetMaxID("bank_statements", "id")
	if err != nil {
		return 1 // Default to 1 if error or no records
	}
	return maxID + 1
}

// scanBankStatement scans a database row into a BankStatement struct
func (bs *BankStatementStore) scanBankStatement(rows *sql.Rows) (types.BankStatement, error) {
	var stmt types.BankStatement
	var periodStart, periodEnd sql.NullString
	var processingTime sql.NullInt64
	var errorLog sql.NullString
	var importDateStr, createdAtStr, updatedAtStr string

	err := rows.Scan(
		&stmt.Id, &stmt.Filename, &importDateStr, &periodStart, &periodEnd,
		&stmt.TemplateUsed, &stmt.TxCount, &stmt.Status, &processingTime,
		&errorLog, &createdAtStr, &updatedAtStr,
	)

	if err != nil {
		return stmt, err
	}

	// Parse time fields from database
	stmt.ImportDate, err = bs.helper.ParseTimeFromDB(importDateStr)
	if err != nil {
		return stmt, fmt.Errorf("failed to parse import_date: %w", err)
	}
	stmt.CreatedAt, err = bs.helper.ParseTimeFromDB(createdAtStr)
	if err != nil {
		return stmt, fmt.Errorf("failed to parse created_at: %w", err)
	}
	stmt.UpdatedAt, err = bs.helper.ParseTimeFromDB(updatedAtStr)
	if err != nil {
		return stmt, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	// Handle nullable fields
	if periodStart.Valid {
		parsedStart, err := bs.helper.ParseDateFromDB(periodStart.String)
		if err != nil {
			return stmt, fmt.Errorf("failed to parse period start: %w", err)
		}
		stmt.PeriodStart = parsedStart
	}
	if periodEnd.Valid {
		parsedEnd, err := bs.helper.ParseDateFromDB(periodEnd.String)
		if err != nil {
			return stmt, fmt.Errorf("failed to parse period end: %w", err)
		}
		stmt.PeriodEnd = parsedEnd
	}
	if processingTime.Valid {
		stmt.ProcessingTime = processingTime.Int64
	}
	if errorLog.Valid {
		stmt.ErrorLog = errorLog.String
	}

	return stmt, nil
}

// GetStatementHistory returns all bank statements ordered by import date
func (bs *BankStatementStore) GetStatementHistory() []types.BankStatement {
	query := `
		SELECT id, filename, import_date, period_start, period_end,
		       template_used, tx_count, status, processing_time, error_log,
		       created_at, updated_at
		FROM bank_statements
		ORDER BY import_date DESC
	`

	rows, err := bs.helper.QueryRows(query)
	if err != nil {
		return []types.BankStatement{} // Return empty slice on error
	}
	defer rows.Close()

	var statements []types.BankStatement
	for rows.Next() {
		stmt, err := bs.scanBankStatement(rows)
		if err != nil {
			continue // Skip malformed rows
		}
		statements = append(statements, stmt)
	}

	return statements
}

// scanBankStatementRow scans a single database row into a BankStatement struct
func (bs *BankStatementStore) scanBankStatementRow(row *sql.Row) (types.BankStatement, error) {
	var stmt types.BankStatement
	var periodStart, periodEnd sql.NullString
	var processingTime sql.NullInt64
	var errorLog sql.NullString
	var importDateStr, createdAtStr, updatedAtStr string

	err := row.Scan(
		&stmt.Id, &stmt.Filename, &importDateStr, &periodStart, &periodEnd,
		&stmt.TemplateUsed, &stmt.TxCount, &stmt.Status, &processingTime,
		&errorLog, &createdAtStr, &updatedAtStr,
	)

	if err != nil {
		return stmt, err
	}

	// Parse time fields from database
	stmt.ImportDate, err = bs.helper.ParseTimeFromDB(importDateStr)
	if err != nil {
		return stmt, fmt.Errorf("failed to parse import_date: %w", err)
	}
	stmt.CreatedAt, err = bs.helper.ParseTimeFromDB(createdAtStr)
	if err != nil {
		return stmt, fmt.Errorf("failed to parse created_at: %w", err)
	}
	stmt.UpdatedAt, err = bs.helper.ParseTimeFromDB(updatedAtStr)
	if err != nil {
		return stmt, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	// Handle nullable fields
	if periodStart.Valid {
		parsedStart, err := bs.helper.ParseDateFromDB(periodStart.String)
		if err != nil {
			return stmt, fmt.Errorf("failed to parse period start: %w", err)
		}
		stmt.PeriodStart = parsedStart
	}
	if periodEnd.Valid {
		parsedEnd, err := bs.helper.ParseDateFromDB(periodEnd.String)
		if err != nil {
			return stmt, fmt.Errorf("failed to parse period end: %w", err)
		}
		stmt.PeriodEnd = parsedEnd
	}
	if processingTime.Valid {
		stmt.ProcessingTime = processingTime.Int64
	}
	if errorLog.Valid {
		stmt.ErrorLog = errorLog.String
	}

	return stmt, nil
}

// GetStatementByIndex returns a statement by its index in the history list
func (bs *BankStatementStore) GetStatementByIndex(index int) (*types.BankStatement, error) {
	statements := bs.GetStatementHistory()
	if index < 0 || index >= len(statements) {
		return nil, fmt.Errorf("statement index %d out of range", index)
	}
	return &statements[index], nil
}

// GetStatementDetails returns a statement and existence boolean by index
func (bs *BankStatementStore) GetStatementDetails(index int) (types.BankStatement, bool) {
	statement, err := bs.GetStatementByIndex(index)
	if err != nil {
		return types.BankStatement{}, false
	}
	return *statement, true
}

// GetStatementById retrieves a bank statement by its ID
func (bs *BankStatementStore) GetStatementById(id int64) (*types.BankStatement, error) {
	query := `
		SELECT id, filename, import_date, period_start, period_end,
		       template_used, tx_count, status, processing_time, error_log,
		       created_at, updated_at
		FROM bank_statements
		WHERE id = ?
	`

	row := bs.helper.QuerySingleRow(query, id)
	stmt, err := bs.scanBankStatementRow(row)
	if err != nil {
		return nil, fmt.Errorf("statement not found: %w", err)
	}

	return &stmt, nil
}

// RecordBankStatement records a new bank statement import and returns the actual assigned ID
func (bs *BankStatementStore) RecordBankStatement(filename, periodStart, periodEnd string, templateId int64, txCount int, status string) (int64, error) {
	now := time.Now().Format(time.RFC3339)

	query := `
		INSERT INTO bank_statements (
			filename, import_date, period_start, period_end,
			template_used, tx_count, status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var periodStartVal, periodEndVal interface{}
	if periodStart != "" {
		periodStartVal = periodStart
	}
	if periodEnd != "" {
		periodEndVal = periodEnd
	}

	return bs.helper.ExecReturnID(query,
		filename, now, periodStartVal, periodEndVal,
		templateId, txCount, status, now, now,
	)
}

// MarkStatementUndone marks a statement as undone
func (bs *BankStatementStore) MarkStatementUndone(statementId int64) error {
	query := "UPDATE bank_statements SET status = 'undone', updated_at = ? WHERE id = ?"
	now := time.Now().Format(time.RFC3339)

	rowsAffected, err := bs.helper.ExecReturnRowsAffected(query, now, statementId)
	if err != nil {
		return fmt.Errorf("failed to mark statement as undone: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("statement not found")
	}

	return nil
}

// CanUndoImport checks if a statement can be undone
func (bs *BankStatementStore) CanUndoImport(statementId int64) bool {
	exists, err := bs.helper.ExistsBy("bank_statements", "id = ? AND status IN ('completed', 'override')", statementId)
	return err == nil && exists
}

// MarkStatementFailed marks a statement as failed with error message
func (bs *BankStatementStore) MarkStatementFailed(statementId int64, errorMsg string) error {
	query := "UPDATE bank_statements SET status = 'failed', error_log = ?, updated_at = ? WHERE id = ?"
	now := time.Now().Format(time.RFC3339)

	rowsAffected, err := bs.helper.ExecReturnRowsAffected(query, errorMsg, now, statementId)
	if err != nil {
		return fmt.Errorf("failed to mark statement as failed: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("statement not found")
	}

	return nil
}

// MarkStatementCompleted marks a statement as completed
func (bs *BankStatementStore) MarkStatementCompleted(statementId int64) error {
	query := "UPDATE bank_statements SET status = 'completed', updated_at = ? WHERE id = ?"
	now := time.Now().Format(time.RFC3339)

	rowsAffected, err := bs.helper.ExecReturnRowsAffected(query, now, statementId)
	if err != nil {
		return fmt.Errorf("failed to mark statement as completed: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("statement not found")
	}

	return nil
}

// DetectOverlap checks for period overlaps with existing completed statements using the same template
func (bs *BankStatementStore) DetectOverlap(periodStart, periodEnd string, templateId int64) []types.BankStatement {
	var overlaps []types.BankStatement

	query := `
		SELECT id, filename, import_date, period_start, period_end,
		       template_used, tx_count, status, processing_time, error_log,
		       created_at, updated_at
		FROM bank_statements
		WHERE status = 'completed' AND period_start IS NOT NULL AND period_end IS NOT NULL
		  AND template_used = ?
		  AND ? <= period_end AND ? >= period_start
	`

	rows, err := bs.helper.QueryRows(query, templateId, periodStart, periodEnd)
	if err != nil {
		return overlaps // Return empty slice on error
	}
	defer rows.Close()

	for rows.Next() {
		stmt, err := bs.scanBankStatement(rows)
		if err != nil {
			continue // Skip malformed rows
		}
		overlaps = append(overlaps, stmt)
	}

	return overlaps
}

// GetStatementSummary returns a formatted summary for display
func (bs *BankStatementStore) GetStatementSummary(stmt types.BankStatement) string {
	return fmt.Sprintf("%s | %s - %s | %d txns | %s",
		stmt.Filename, stmt.PeriodStart, stmt.PeriodEnd, stmt.TxCount, stmt.Status)
}

// GetStatementsByStatus returns statements filtered by status
func (bs *BankStatementStore) GetStatementsByStatus(status string) []types.BankStatement {
	query := `
		SELECT id, filename, import_date, period_start, period_end,
		       template_used, tx_count, status, processing_time, error_log,
		       created_at, updated_at
		FROM bank_statements
		WHERE status = ?
		ORDER BY import_date DESC
	`

	rows, err := bs.helper.QueryRows(query, status)
	if err != nil {
		return []types.BankStatement{} // Return empty slice on error
	}
	defer rows.Close()

	var statements []types.BankStatement
	for rows.Next() {
		stmt, err := bs.scanBankStatement(rows)
		if err != nil {
			continue // Skip malformed rows
		}
		statements = append(statements, stmt)
	}

	return statements
}

// GetUndoableStatements returns statements that can be undone
func (bs *BankStatementStore) GetUndoableStatements() []types.BankStatement {
	return bs.GetStatementsByStatus("completed")
}

// DeleteStatement permanently removes a bank statement from the database
func (bs *BankStatementStore) DeleteStatement(id int64) error {
	rowsAffected, err := bs.helper.DeleteBy("bank_statements", "id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete statement: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("statement not found")
	}

	return nil
}

// ExtractPeriodFromTransactions extracts start and end dates from transactions
// Returns ISO 8601 formatted date strings (YYYY-MM-DD) for database storage
func (bs *BankStatementStore) ExtractPeriodFromTransactions(transactions []types.Transaction) (start, end string) {
	if len(transactions) == 0 {
		return "", ""
	}

	startTime := transactions[0].Date
	endTime := transactions[0].Date
	for _, tx := range transactions {
		if tx.Date.Before(startTime) {
			startTime = tx.Date
		}
		if tx.Date.After(endTime) {
			endTime = tx.Date
		}
	}

	// Format as ISO 8601 strings for database storage
	start = bs.helper.FormatDateForDB(startTime)
	end = bs.helper.FormatDateForDB(endTime)

	return start, end
}

// Directory Navigation Business Logic
type DirectoryResult struct {
	Entries     []string
	CurrentPath string
	Success     bool
	Message     string
}

// LoadDirectoryEntries loads directory entries for file picker
func (bs *BankStatementStore) LoadDirectoryEntries(currentDir string) *DirectoryResult {
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

// LoadDirectoryEntriesWithFallback loads directory entries with home directory fallback
func (bs *BankStatementStore) LoadDirectoryEntriesWithFallback(currentDir string) *DirectoryResult {
	result := bs.LoadDirectoryEntries(currentDir)
	if !result.Success {
		// Fallback to home directory on error
		if homeDir, err := os.UserHomeDir(); err == nil {
			result = bs.LoadDirectoryEntries(homeDir)
			if result.Success {
				result.Message = "Directory access failed, showing home directory"
			}
		}
	}
	return result
}

// CleanupOrphanedImportingStatements finds and removes bank statements stuck in "importing" status
// These can occur when imports fail after statement creation but before completion
func (bs *BankStatementStore) CleanupOrphanedImportingStatements() (int, error) {
	query := "DELETE FROM bank_statements WHERE status = 'importing'"
	rowsAffected, err := bs.helper.ExecReturnRowsAffected(query)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup orphaned statements: %v", err)
	}
	return int(rowsAffected), nil
}

// GetOrphanedImportingStatements returns bank statements stuck in "importing" status
func (bs *BankStatementStore) GetOrphanedImportingStatements() ([]types.BankStatement, error) {
	query := `
		SELECT id, filename, import_date, period_start, period_end,
		       template_used, tx_count, status, processing_time, error_log,
		       created_at, updated_at
		FROM bank_statements 
		WHERE status = 'importing'
		ORDER BY import_date DESC
	`

	rows, err := bs.helper.QueryRows(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query orphaned statements: %v", err)
	}
	defer rows.Close()

	var statements []types.BankStatement
	for rows.Next() {
		stmt, err := bs.scanBankStatement(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan statement row: %v", err)
		}
		statements = append(statements, stmt)
	}

	return statements, nil
}
