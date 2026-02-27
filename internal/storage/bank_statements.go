package storage

import (
	"budget-tracker-tui/internal/types"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BankStatementStore handles all bank statement-related operations
type BankStatementStore struct {
	filename   string
	Statements []types.BankStatement `json:"statements"`
	NextId     int64                 `json:"nextId"`
}

// NewBankStatementStore creates a new BankStatementStore instance
func NewBankStatementStore(statementFile string) *BankStatementStore {
	return &BankStatementStore{
		filename:   statementFile,
		Statements: []types.BankStatement{},
		NextId:     1,
	}
}

// LoadBankStatements loads bank statements from the JSON file
func (bs *BankStatementStore) LoadBankStatements() error {
	if _, err := os.Stat(bs.filename); os.IsNotExist(err) {
		bs.Statements = []types.BankStatement{}
		bs.NextId = 1
		return bs.SaveBankStatements()
	}

	data, err := os.ReadFile(bs.filename)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, bs)
}

// SaveBankStatements saves bank statements to the JSON file
func (bs *BankStatementStore) SaveBankStatements() error {
	data, err := json.MarshalIndent(bs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(bs.filename, data, 0644)
}

// GetStatementHistory returns all bank statements
func (bs *BankStatementStore) GetStatementHistory() []types.BankStatement {
	return bs.Statements
}

// GetStatementByIndex returns a bank statement by index
func (bs *BankStatementStore) GetStatementByIndex(index int) (*types.BankStatement, error) {
	if index < 0 || index >= len(bs.Statements) {
		return nil, fmt.Errorf("statement index %d out of range", index)
	}
	return &bs.Statements[index], nil
}

// GetStatementDetails returns a bank statement by index with bool indicator
func (bs *BankStatementStore) GetStatementDetails(index int) (types.BankStatement, bool) {
	if index < 0 || index >= len(bs.Statements) {
		return types.BankStatement{}, false
	}
	return bs.Statements[index], true
}

// GetStatementById retrieves a bank statement by its ID
func (bs *BankStatementStore) GetStatementById(id int64) (*types.BankStatement, error) {
	for i, stmt := range bs.Statements {
		if stmt.Id == id {
			return &bs.Statements[i], nil
		}
	}
	return nil, fmt.Errorf("statement with ID %d not found", id)
}

// RecordBankStatement records a new bank statement
func (bs *BankStatementStore) RecordBankStatement(filename, periodStart, periodEnd string, templateId int64, txCount int, status string) error {
	statement := types.BankStatement{
		Id:           bs.NextId,
		Filename:     filename,
		ImportDate:   time.Now().Format(time.RFC3339), // RFC3339 timestamp
		PeriodStart:  periodStart,
		PeriodEnd:    periodEnd,
		TemplateUsed: templateId,
		TxCount:      txCount,
		Status:       status,
	}

	bs.Statements = append(bs.Statements, statement)
	bs.NextId++

	return bs.SaveBankStatements()
}

// MarkStatementUndone marks a statement as undone
func (bs *BankStatementStore) MarkStatementUndone(statementId int64) error {
	for i, stmt := range bs.Statements {
		if stmt.Id == statementId {
			bs.Statements[i].Status = "undone"
			return bs.SaveBankStatements()
		}
	}
	return fmt.Errorf("statement with ID %d not found", statementId)
}

// CanUndoImport checks if a statement can be undone
func (bs *BankStatementStore) CanUndoImport(statementId int64) bool {
	for _, stmt := range bs.Statements {
		if stmt.Id == statementId {
			return stmt.Status == "completed" || stmt.Status == "override"
		}
	}
	return false
}

// DetectOverlap checks for period overlaps with existing completed statements
func (bs *BankStatementStore) DetectOverlap(periodStart, periodEnd string) []types.BankStatement {
	var overlaps []types.BankStatement

	for _, stmt := range bs.Statements {
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

// GetStatementSummary returns a formatted summary for display
func (bs *BankStatementStore) GetStatementSummary(stmt types.BankStatement) string {
	return fmt.Sprintf("%s | %s - %s | %d txns | %s",
		stmt.Filename, stmt.PeriodStart, stmt.PeriodEnd, stmt.TxCount, stmt.Status)
}

// GetStatementsByStatus returns statements filtered by status
func (bs *BankStatementStore) GetStatementsByStatus(status string) []types.BankStatement {
	var filtered []types.BankStatement
	for _, stmt := range bs.Statements {
		if stmt.Status == status {
			filtered = append(filtered, stmt)
		}
	}
	return filtered
}

// GetUndoableStatements returns statements that can be undone
func (bs *BankStatementStore) GetUndoableStatements() []types.BankStatement {
	var undoable []types.BankStatement
	for _, stmt := range bs.Statements {
		if bs.CanUndoImport(stmt.Id) {
			undoable = append(undoable, stmt)
		}
	}
	return undoable
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
