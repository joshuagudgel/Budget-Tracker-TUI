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

	err := rows.Scan(
		&stmt.Id, &stmt.Filename, &stmt.ImportDate, &periodStart, &periodEnd,
		&stmt.TemplateUsed, &stmt.TxCount, &stmt.Status, &processingTime,
		&errorLog, &stmt.CreatedAt, &stmt.UpdatedAt,
	)

	if err != nil {
		return stmt, err
	}

	// Handle nullable fields
	if periodStart.Valid {
		stmt.PeriodStart = periodStart.String
	}
	if periodEnd.Valid {
		stmt.PeriodEnd = periodEnd.String
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

	err := row.Scan(
		&stmt.Id, &stmt.Filename, &stmt.ImportDate, &periodStart, &periodEnd,
		&stmt.TemplateUsed, &stmt.TxCount, &stmt.Status, &processingTime,
		&errorLog, &stmt.CreatedAt, &stmt.UpdatedAt,
	)

	if err != nil {
		return stmt, err
	}

	// Handle nullable fields
	if periodStart.Valid {
		stmt.PeriodStart = periodStart.String
	}
	if periodEnd.Valid {
		stmt.PeriodEnd = periodEnd.String
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

// RecordBankStatement records a new bank statement import
func (bs *BankStatementStore) RecordBankStatement(filename, periodStart, periodEnd string, templateId int64, txCount int, status string) error {
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

	_, err := bs.helper.ExecReturnID(query,
		filename, now, periodStartVal, periodEndVal,
		templateId, txCount, status, now, now,
	)

	return err
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

// DetectOverlap checks for period overlaps with existing completed statements
func (bs *BankStatementStore) DetectOverlap(periodStart, periodEnd string) []types.BankStatement {
	var overlaps []types.BankStatement

	query := `
		SELECT id, filename, import_date, period_start, period_end,
		       template_used, tx_count, status, processing_time, error_log,
		       created_at, updated_at
		FROM bank_statements
		WHERE status = 'completed' AND period_start IS NOT NULL AND period_end IS NOT NULL
		  AND ? <= period_end AND ? >= period_start
	`

	rows, err := bs.helper.QueryRows(query, periodStart, periodEnd)
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

// ExtractPeriodFromTransactions extracts start and end dates from transactions
func (bs *BankStatementStore) ExtractPeriodFromTransactions(transactions []types.Transaction) (start, end string) {
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
