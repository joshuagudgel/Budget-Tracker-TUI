package storage

import (
	"budget-tracker-tui/internal/database"
	"budget-tracker-tui/internal/types"
	"database/sql"
	"fmt"
	"strconv"
	"time"
)

// TransactionStore handles all transaction-related operations using SQLite
type TransactionStore struct {
	db     *database.Connection
	helper *database.SQLHelper
}

// NewTransactionStore creates a new TransactionStore instance
func NewTransactionStore(db *database.Connection) *TransactionStore {
	return &TransactionStore{
		db:     db,
		helper: database.NewSQLHelper(db),
	}
}

// CalculateNextId calculates the next available ID using SQLite's auto-increment
func (ts *TransactionStore) CalculateNextId() int64 {
	maxID, err := ts.helper.GetMaxID("transactions", "id")
	if err != nil {
		return 1 // Default to 1 if error or no records
	}
	return maxID + 1
}

// GetTransactions returns all transactions from the database
func (ts *TransactionStore) GetTransactions() ([]types.Transaction, error) {
	query := `
		SELECT id, parent_id, amount, description, raw_description, date, 
		       category_id, auto_category, transaction_type, is_split, 
		       is_recurring, statement_id, confidence, user_modified, 
		       created_at, updated_at 
		FROM transactions 
		ORDER BY date DESC, id DESC
	`

	rows, err := ts.helper.QueryRows(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var transactions []types.Transaction
	for rows.Next() {
		tx, err := ts.scanTransaction(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, tx)
	}

	return transactions, rows.Err()
}

// GetTransactionsByStatement returns all transactions for a specific bank statement
func (ts *TransactionStore) GetTransactionsByStatement(statementId int64) ([]types.Transaction, error) {
	query := `
		SELECT id, parent_id, amount, description, raw_description, date, 
		       category_id, auto_category, transaction_type, is_split, 
		       is_recurring, statement_id, confidence, user_modified, 
		       created_at, updated_at 
		FROM transactions 
		WHERE statement_id = ? 
		ORDER BY date DESC, id DESC
	`

	rows, err := ts.helper.QueryRows(query, statementId)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions by statement: %w", err)
	}
	defer rows.Close()

	var transactions []types.Transaction
	for rows.Next() {
		tx, err := ts.scanTransaction(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, tx)
	}

	return transactions, rows.Err()
}

// scanTransaction scans a database row into a Transaction struct
func (ts *TransactionStore) scanTransaction(rows *sql.Rows) (types.Transaction, error) {
	var tx types.Transaction
	var parentID sql.NullInt64
	var statementID sql.NullInt64
	var rawDescription sql.NullString
	var autoCategory sql.NullString
	var confidence sql.NullFloat64

	err := rows.Scan(
		&tx.Id, &parentID, &tx.Amount, &tx.Description, &rawDescription,
		&tx.Date, &tx.CategoryId, &autoCategory, &tx.TransactionType,
		&tx.IsSplit, &tx.IsRecurring, &statementID, &confidence,
		&tx.UserModified, &tx.CreatedAt, &tx.UpdatedAt,
	)

	if err != nil {
		return tx, err
	}

	// Handle nullable fields
	if parentID.Valid {
		tx.ParentId = &parentID.Int64
	}
	if statementID.Valid {
		tx.StatementId = fmt.Sprintf("%d", statementID.Int64)
	}
	if rawDescription.Valid {
		tx.RawDescription = rawDescription.String
	}
	if autoCategory.Valid {
		tx.AutoCategory = autoCategory.String
	}
	if confidence.Valid {
		tx.Confidence = confidence.Float64
	}

	return tx, nil
}

// GetTransactionByID returns a transaction by ID
func (ts *TransactionStore) GetTransactionByID(id int64) *types.Transaction {
	query := `
		SELECT id, parent_id, amount, description, raw_description, date, 
		       category_id, auto_category, transaction_type, is_split, 
		       is_recurring, statement_id, confidence, user_modified, 
		       created_at, updated_at 
		FROM transactions 
		WHERE id = ?
	`

	row := ts.helper.QuerySingleRow(query, id)
	tx, err := ts.scanTransactionRow(row)
	if err != nil {
		return nil // Transaction not found or error
	}

	return &tx
}

// scanTransactionRow scans a single database row into a Transaction struct
func (ts *TransactionStore) scanTransactionRow(row *sql.Row) (types.Transaction, error) {
	var tx types.Transaction
	var parentID sql.NullInt64
	var statementID sql.NullInt64
	var rawDescription sql.NullString
	var autoCategory sql.NullString
	var confidence sql.NullFloat64

	err := row.Scan(
		&tx.Id, &parentID, &tx.Amount, &tx.Description, &rawDescription,
		&tx.Date, &tx.CategoryId, &autoCategory, &tx.TransactionType,
		&tx.IsSplit, &tx.IsRecurring, &statementID, &confidence,
		&tx.UserModified, &tx.CreatedAt, &tx.UpdatedAt,
	)

	if err != nil {
		return tx, err
	}

	// Handle nullable fields
	if parentID.Valid {
		tx.ParentId = &parentID.Int64
	}
	if statementID.Valid {
		tx.StatementId = fmt.Sprintf("%d", statementID.Int64)
	}
	if rawDescription.Valid {
		tx.RawDescription = rawDescription.String
	}
	if autoCategory.Valid {
		tx.AutoCategory = autoCategory.String
	}
	if confidence.Valid {
		tx.Confidence = confidence.Float64
	}

	return tx, nil
}

// SaveTransaction saves or updates a transaction in the database
func (ts *TransactionStore) SaveTransaction(transaction types.Transaction) error {
	now := time.Now().Format(time.RFC3339)

	if transaction.Id == 0 {
		// Insert new transaction
		return ts.insertTransaction(transaction, now)
	} else {
		// Update existing transaction
		return ts.updateTransaction(transaction, now)
	}
}

// insertTransaction inserts a new transaction into the database
func (ts *TransactionStore) insertTransaction(transaction types.Transaction, now string) error {
	query := `
		INSERT INTO transactions (
			parent_id, amount, description, raw_description, date, 
			category_id, auto_category, transaction_type, is_split, 
			is_recurring, statement_id, confidence, user_modified, 
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// Convert nullable fields
	var parentID interface{}
	if transaction.ParentId != nil {
		parentID = *transaction.ParentId
	}

	var statementID interface{}
	if transaction.StatementId != "" {
		if id, err := strconv.ParseInt(transaction.StatementId, 10, 64); err == nil {
			statementID = id
		}
	}

	var rawDescription interface{}
	if transaction.RawDescription != "" {
		rawDescription = transaction.RawDescription
	}

	var autoCategory interface{}
	if transaction.AutoCategory != "" {
		autoCategory = transaction.AutoCategory
	}

	var confidence interface{}
	if transaction.Confidence > 0 {
		confidence = transaction.Confidence
	}

	// Set creation timestamp if not provided
	createdAt := transaction.CreatedAt
	if createdAt == "" {
		createdAt = now
	}

	id, err := ts.helper.ExecReturnID(query,
		parentID, transaction.Amount, transaction.Description, rawDescription,
		transaction.Date, transaction.CategoryId, autoCategory, transaction.TransactionType,
		transaction.IsSplit, transaction.IsRecurring, statementID, confidence,
		transaction.UserModified, createdAt, now,
	)

	if err != nil {
		return fmt.Errorf("failed to insert transaction: %w", err)
	}

	// Update the transaction ID
	transaction.Id = id
	return nil
}

// updateTransaction updates an existing transaction in the database
func (ts *TransactionStore) updateTransaction(transaction types.Transaction, now string) error {
	query := `
		UPDATE transactions SET 
			parent_id = ?, amount = ?, description = ?, raw_description = ?, 
			date = ?, category_id = ?, auto_category = ?, transaction_type = ?, 
			is_split = ?, is_recurring = ?, statement_id = ?, confidence = ?, 
			user_modified = ?, updated_at = ?
		WHERE id = ?
	`

	// Convert nullable fields
	var parentID interface{}
	if transaction.ParentId != nil {
		parentID = *transaction.ParentId
	}

	var statementID interface{}
	if transaction.StatementId != "" {
		if id, err := strconv.ParseInt(transaction.StatementId, 10, 64); err == nil {
			statementID = id
		}
	}

	var rawDescription interface{}
	if transaction.RawDescription != "" {
		rawDescription = transaction.RawDescription
	}

	var autoCategory interface{}
	if transaction.AutoCategory != "" {
		autoCategory = transaction.AutoCategory
	}

	var confidence interface{}
	if transaction.Confidence > 0 {
		confidence = transaction.Confidence
	}

	// Mark as user modified when updating
	transaction.UserModified = true

	_, err := ts.helper.ExecReturnRowsAffected(query,
		parentID, transaction.Amount, transaction.Description, rawDescription,
		transaction.Date, transaction.CategoryId, autoCategory, transaction.TransactionType,
		transaction.IsSplit, transaction.IsRecurring, statementID, confidence,
		transaction.UserModified, now, transaction.Id,
	)

	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	return nil
}

// DeleteTransaction removes a transaction by ID
func (ts *TransactionStore) DeleteTransaction(id int64) error {
	query := "DELETE FROM transactions WHERE id = ?"

	rowsAffected, err := ts.helper.ExecReturnRowsAffected(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete transaction: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("transaction with ID %d not found", id)
	}

	return nil
}

// SplitTransaction splits a parent transaction into two transactions using database transaction
func (ts *TransactionStore) SplitTransaction(parentId int64, splits []types.Transaction) error {
	return ts.db.ExecuteInTransaction(func(tx *sql.Tx) error {
		// Validate splits add up to parent amount
		parent := ts.GetTransactionByID(parentId)
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
		const epsilon = 0.01
		if totalSplit-parent.Amount > epsilon || parent.Amount-totalSplit > epsilon {
			return fmt.Errorf("split amounts (%.2f) don't match parent (%.2f)", totalSplit, parent.Amount)
		}

		now := time.Now().Format(time.RFC3339)

		// Update existing transaction to become first split
		updateQuery := `
			UPDATE transactions SET 
				amount = ?, description = ?, category_id = ?, is_split = ?, 
				user_modified = ?, updated_at = ?
			WHERE id = ?
		`
		_, err := tx.Exec(updateQuery,
			splits[0].Amount, splits[0].Description, splits[0].CategoryId,
			true, true, now, parentId,
		)
		if err != nil {
			return fmt.Errorf("failed to update parent transaction: %w", err)
		}

		// Create second split as new transaction
		insertQuery := `
			INSERT INTO transactions (
				amount, description, date, category_id, transaction_type, 
				statement_id, is_split, user_modified, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`

		var statementID interface{}
		if parent.StatementId != "" {
			if id, parseErr := strconv.ParseInt(parent.StatementId, 10, 64); parseErr == nil {
				statementID = id
			}
		}

		_, err = tx.Exec(insertQuery,
			splits[1].Amount, splits[1].Description, parent.Date,
			splits[1].CategoryId, parent.TransactionType, statementID,
			false, true, now, now,
		)
		if err != nil {
			return fmt.Errorf("failed to insert second split: %w", err)
		}

		return nil
	})
}

// ImportTransactionsFromCSV imports a batch of transactions from CSV parsing
func (ts *TransactionStore) ImportTransactionsFromCSV(transactions []types.Transaction, statementId string) error {
	if len(transactions) == 0 {
		return nil
	}

	// Prepare bulk insert data
	var records [][]interface{}
	now := time.Now().Format(time.RFC3339)

	var statementID interface{}
	if statementId != "" {
		if id, err := strconv.ParseInt(statementId, 10, 64); err == nil {
			statementID = id
		}
	}

	for _, tx := range transactions {
		// Set default transaction type if not set
		transactionType := tx.TransactionType
		if transactionType == "" {
			transactionType = "expense"
		}

		// Set timestamps for imported transactions
		createdAt := tx.CreatedAt
		if createdAt == "" {
			createdAt = now
		}

		// Handle nullable fields
		var parentID interface{}
		if tx.ParentId != nil {
			parentID = *tx.ParentId
		}

		var rawDescription interface{}
		if tx.RawDescription != "" {
			rawDescription = tx.RawDescription
		}

		var autoCategory interface{}
		if tx.AutoCategory != "" {
			autoCategory = tx.AutoCategory
		}

		var confidence interface{}
		if tx.Confidence > 0 {
			confidence = tx.Confidence
		}

		record := []interface{}{
			parentID, tx.Amount, tx.Description, rawDescription, tx.Date,
			tx.CategoryId, autoCategory, transactionType, tx.IsSplit,
			tx.IsRecurring, statementID, confidence, tx.UserModified,
			createdAt, now,
		}
		records = append(records, record)
	}

	// Bulk insert using transaction
	fields := []string{
		"parent_id", "amount", "description", "raw_description", "date",
		"category_id", "auto_category", "transaction_type", "is_split",
		"is_recurring", "statement_id", "confidence", "user_modified",
		"created_at", "updated_at",
	}

	return ts.helper.BulkInsert("transactions", fields, records)
}

// CreateBackup creates a backup of current transactions in a simplified format
func (ts *TransactionStore) CreateBackup() error {
	// This functionality would need to be coordinated with a CategoryStore
	// for category display names. For now, return an error suggesting
	// backup should be handled at the main Store level.
	return fmt.Errorf("backup functionality moved to main Store level for cross-domain coordination")
}

// RestoreFromBackup restores transactions from a backup file
func (ts *TransactionStore) RestoreFromBackup() (*RestoreResult, error) {
	// This functionality would need to be coordinated with a CategoryStore
	// for category name resolution. For now, return an error suggesting
	// restore should be handled at the main Store level.
	return nil, fmt.Errorf("restore functionality moved to main Store level for cross-domain coordination")
}

// Backup-related types for TransactionStore (kept for interface compatibility)
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
