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
	audits *AuditStore
}

// NewTransactionStore creates a new TransactionStore instance
func NewTransactionStore(db *database.Connection) *TransactionStore {
	return &TransactionStore{
		db:     db,
		helper: database.NewSQLHelper(db),
	}
}

// SetAuditStore sets the audit store reference (called after all stores are initialized)
func (ts *TransactionStore) SetAuditStore(as *AuditStore) {
	ts.audits = as
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
	var dateStr, createdAtStr, updatedAtStr string

	err := rows.Scan(
		&tx.Id, &parentID, &tx.Amount, &tx.Description, &rawDescription,
		&dateStr, &tx.CategoryId, &autoCategory, &tx.TransactionType,
		&tx.IsSplit, &tx.IsRecurring, &statementID, &confidence,
		&tx.UserModified, &createdAtStr, &updatedAtStr,
	)

	if err != nil {
		return tx, err
	}

	// Parse date strings into time.Time with multiple format support
	if tx.Date, err = ts.parseFlexibleDate(dateStr); err != nil {
		return tx, fmt.Errorf("failed to parse transaction date '%s': %w", dateStr, err)
	}
	if tx.CreatedAt, err = ts.helper.ParseTimeFromDB(createdAtStr); err != nil {
		return tx, fmt.Errorf("failed to parse created_at timestamp '%s': %w", createdAtStr, err)
	}
	if tx.UpdatedAt, err = ts.helper.ParseTimeFromDB(updatedAtStr); err != nil {
		return tx, fmt.Errorf("failed to parse updated_at timestamp '%s': %w", updatedAtStr, err)
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
	var dateStr, createdAtStr, updatedAtStr string

	err := row.Scan(
		&tx.Id, &parentID, &tx.Amount, &tx.Description, &rawDescription,
		&dateStr, &tx.CategoryId, &autoCategory, &tx.TransactionType,
		&tx.IsSplit, &tx.IsRecurring, &statementID, &confidence,
		&tx.UserModified, &createdAtStr, &updatedAtStr,
	)

	if err != nil {
		return tx, err
	}

	// Parse time fields from database
	tx.Date, err = ts.parseFlexibleDate(dateStr)
	if err != nil {
		return tx, fmt.Errorf("failed to parse date: %w", err)
	}
	tx.CreatedAt, err = ts.helper.ParseTimeFromDB(createdAtStr)
	if err != nil {
		return tx, fmt.Errorf("failed to parse created_at: %w", err)
	}
	tx.UpdatedAt, err = ts.helper.ParseTimeFromDB(updatedAtStr)
	if err != nil {
		return tx, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	// Parse time fields from database
	tx.Date, err = ts.parseFlexibleDate(dateStr)
	if err != nil {
		return tx, fmt.Errorf("failed to parse date: %w", err)
	}
	tx.CreatedAt, err = ts.helper.ParseTimeFromDB(createdAtStr)
	if err != nil {
		return tx, fmt.Errorf("failed to parse created_at: %w", err)
	}
	tx.UpdatedAt, err = ts.helper.ParseTimeFromDB(updatedAtStr)
	if err != nil {
		return tx, fmt.Errorf("failed to parse updated_at: %w", err)
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
	now := time.Now()

	if transaction.Id == 0 {
		// Insert new transaction
		return ts.insertTransaction(transaction, now)
	} else {
		// Update existing transaction
		return ts.updateTransaction(transaction, now)
	}
}

// insertTransaction inserts a new transaction into the database
func (ts *TransactionStore) insertTransaction(transaction types.Transaction, now time.Time) error {
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
	if createdAt.IsZero() {
		createdAt = now
	}

	// Format times for database storage
	dateStr := transaction.Date.Format("2006-01-02")
	createdAtStr := createdAt.Format(time.RFC3339)
	updatedAtStr := now.Format(time.RFC3339)

	id, err := ts.helper.ExecReturnID(query,
		parentID, transaction.Amount, transaction.Description, rawDescription,
		dateStr, transaction.CategoryId, autoCategory, transaction.TransactionType,
		transaction.IsSplit, transaction.IsRecurring, statementID, confidence,
		transaction.UserModified, createdAtStr, updatedAtStr,
	)

	if err != nil {
		return fmt.Errorf("failed to insert transaction: %w", err)
	}

	// Update the transaction with the generated ID
	transaction.Id = id

	// Log audit event for transaction creation
	if ts.audits != nil {
		source := types.SourceUser
		if transaction.StatementId != "" {
			source = types.SourceImport
		}

		err = ts.audits.RecordFieldChange(
			types.EntityTypeTransaction,
			transaction.Id,
			types.EventTypeCreate,
			"",  // No specific field for CREATE events
			nil, // No old value for CREATE
			"created",
			source,
			"", // Empty context for now
		)
		if err != nil {
			// Log error but don't fail the transaction
			fmt.Printf("Warning: Failed to record audit event for transaction creation: %v\n", err)
		}
	}

	return nil
}

// updateTransaction updates an existing transaction in the database
func (ts *TransactionStore) updateTransaction(transaction types.Transaction, now time.Time) error {
	// Get the old transaction for audit logging
	var oldTransaction *types.Transaction
	if ts.audits != nil {
		oldTransaction = ts.GetTransactionByID(transaction.Id)
	}

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

	// Log audit events for field changes
	if ts.audits != nil && oldTransaction != nil {
		ts.logTransactionFieldChanges(oldTransaction, &transaction)
	}

	return nil
}

// logTransactionFieldChanges compares old and new transaction and logs field-level changes
func (ts *TransactionStore) logTransactionFieldChanges(oldTx, newTx *types.Transaction) {
	var changes []FieldChange

	// Check amount changes
	if oldTx.Amount != newTx.Amount {
		changes = append(changes, FieldChange{
			EntityType: types.EntityTypeTransaction,
			EntityId:   newTx.Id,
			EventType:  types.EventTypeUpdate,
			FieldName:  "amount",
			OldValue:   fmt.Sprintf("%.2f", oldTx.Amount),
			NewValue:   fmt.Sprintf("%.2f", newTx.Amount),
			Source:     types.SourceUser,
		})
	}

	// Check description changes
	if oldTx.Description != newTx.Description {
		changes = append(changes, FieldChange{
			EntityType: types.EntityTypeTransaction,
			EntityId:   newTx.Id,
			EventType:  types.EventTypeUpdate,
			FieldName:  "description",
			OldValue:   oldTx.Description,
			NewValue:   newTx.Description,
			Source:     types.SourceUser,
		})
	}

	// Check date changes
	if !oldTx.Date.Equal(newTx.Date) {
		changes = append(changes, FieldChange{
			EntityType: types.EntityTypeTransaction,
			EntityId:   newTx.Id,
			EventType:  types.EventTypeUpdate,
			FieldName:  "date",
			OldValue:   oldTx.Date.Format("2006-01-02"),
			NewValue:   newTx.Date.Format("2006-01-02"),
			Source:     types.SourceUser,
		})
	}

	// Check category changes
	if oldTx.CategoryId != newTx.CategoryId {
		changes = append(changes, FieldChange{
			EntityType: types.EntityTypeTransaction,
			EntityId:   newTx.Id,
			EventType:  types.EventTypeUpdate,
			FieldName:  "category_id",
			OldValue:   fmt.Sprintf("%d", oldTx.CategoryId),
			NewValue:   fmt.Sprintf("%d", newTx.CategoryId),
			Source:     types.SourceUser,
		})
	}

	// Check transaction type changes
	if oldTx.TransactionType != newTx.TransactionType {
		changes = append(changes, FieldChange{
			EntityType: types.EntityTypeTransaction,
			EntityId:   newTx.Id,
			EventType:  types.EventTypeUpdate,
			FieldName:  "transaction_type",
			OldValue:   oldTx.TransactionType,
			NewValue:   newTx.TransactionType,
			Source:     types.SourceUser,
		})
	}

	// Record all changes if any exist
	if len(changes) > 0 {
		err := ts.audits.RecordMultipleFieldChanges(changes)
		if err != nil {
			fmt.Printf("Warning: Failed to record audit events for transaction update: %v\n", err)
		}
	}
}

// DeleteTransaction removes a transaction by ID
func (ts *TransactionStore) DeleteTransaction(id int64) error {
	// Get transaction details for audit logging before deletion
	var deletedTransaction *types.Transaction
	if ts.audits != nil {
		deletedTransaction = ts.GetTransactionByID(id)
	}

	query := "DELETE FROM transactions WHERE id = ?"

	rowsAffected, err := ts.helper.ExecReturnRowsAffected(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete transaction: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("transaction with ID %d not found", id)
	}

	// Log audit event for transaction deletion
	if ts.audits != nil && deletedTransaction != nil {
		err = ts.audits.RecordFieldChange(
			types.EntityTypeTransaction,
			id,
			types.EventTypeDelete,
			"", // No specific field for DELETE events
			"deleted",
			nil, // No new value for DELETE
			types.SourceUser,
			"", // Empty context for now
		)
		if err != nil {
			// Log error but don't fail the transaction
			fmt.Printf("Warning: Failed to record audit event for transaction deletion: %v\n", err)
		}
	}

	return nil
}

// SplitTransaction splits a parent transaction into two transactions using database transaction
func (ts *TransactionStore) SplitTransaction(parentId int64, splits []types.Transaction) error {
	// Get original transaction for audit logging
	var originalTransaction *types.Transaction
	if ts.audits != nil {
		originalTransaction = ts.GetTransactionByID(parentId)
	}

	err := ts.db.ExecuteInTransaction(func(tx *sql.Tx) error {
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

		now := time.Now()

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
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

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

	if err != nil {
		return err
	}

	// Log audit events for split transaction after successful database transaction
	if ts.audits != nil && originalTransaction != nil {
		// Log SPLIT event for the original transaction (now modified)
		splitErr := ts.audits.RecordFieldChange(
			types.EntityTypeTransaction,
			parentId,
			types.EventTypeSplit,
			"amount",
			fmt.Sprintf("%.2f", originalTransaction.Amount),
			fmt.Sprintf("%.2f", splits[0].Amount),
			types.SourceUser,
			fmt.Sprintf("split_into_2_parts"),
		)
		if splitErr != nil {
			fmt.Printf("Warning: Failed to record audit event for transaction split: %v\n", splitErr)
		}

		// Note: The second split transaction audit will be logged by the next INSERT
		// when we get the new transaction ID, but since we're already outside the transaction,
		// we'd need to query for it. For now, we'll log the split event only.
	}

	return nil
}

// ImportTransactionsFromCSV imports a batch of transactions from CSV parsing
func (ts *TransactionStore) ImportTransactionsFromCSV(transactions []types.Transaction, statementId string) error {
	if len(transactions) == 0 {
		return nil
	}

	// Prepare bulk insert data
	var records [][]interface{}
	now := time.Now()

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
		if createdAt.IsZero() {
			createdAt = now
		}

		// Format times for database storage
		dateStr := tx.Date.Format("2006-01-02")
		createdAtStr := createdAt.Format(time.RFC3339)
		updatedAtStr := now.Format(time.RFC3339)

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
			parentID, tx.Amount, tx.Description, rawDescription, dateStr,
			tx.CategoryId, autoCategory, transactionType, tx.IsSplit,
			tx.IsRecurring, statementID, confidence, tx.UserModified,
			createdAtStr, updatedAtStr,
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

	err := ts.helper.BulkInsert("transactions", fields, records)
	if err != nil {
		return err
	}

	// Log audit events for CSV import
	if ts.audits != nil && len(transactions) > 0 {
		// For bulk imports, we create a summary audit event since we can't easily get individual IDs from BulkInsert
		// Individual transaction audits would require querying back the inserted records
		context := fmt.Sprintf("csv_import_count_%d_statement_%s", len(transactions), statementId)
		auditErr := ts.audits.RecordFieldChange(
			types.EntityTypeTransaction,
			0, // Use 0 for bulk operations since we don't have individual IDs
			types.EventTypeImport,
			"bulk_import",
			"",
			fmt.Sprintf("%d_transactions_imported", len(transactions)),
			types.SourceImport,
			context,
		)
		if auditErr != nil {
			fmt.Printf("Warning: Failed to record audit event for CSV import: %v\n", auditErr)
		}
	}

	return nil
}

// FindDuplicateTransactions finds existing transactions that match date, amount, and description
func (ts *TransactionStore) FindDuplicateTransactions(date string, amount float64, description string) ([]types.Transaction, error) {
	query := `
		SELECT id, parent_id, amount, description, raw_description, date, 
		       category_id, auto_category, transaction_type, is_split, 
		       is_recurring, statement_id, confidence, user_modified, 
		       created_at, updated_at 
		FROM transactions 
		WHERE date = ? AND ABS(amount - ?) < 0.01 AND description = ?
		ORDER BY id
	`

	rows, err := ts.helper.QueryRows(query, date, amount, description)
	if err != nil {
		return nil, fmt.Errorf("failed to query duplicate transactions: %w", err)
	}
	defer rows.Close()

	var duplicates []types.Transaction
	for rows.Next() {
		tx, err := ts.scanTransaction(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		duplicates = append(duplicates, tx)
	}

	return duplicates, rows.Err()
}

// parseFlexibleDate tries multiple date formats to handle legacy data
func (ts *TransactionStore) parseFlexibleDate(dateStr string) (time.Time, error) {
	// Try RFC3339 format first (preferred format)
	if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return t, nil
	}

	// Try ISO 8601 date format (YYYY-MM-DD)
	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		return t, nil
	}

	// Try MM-DD-YYYY format (legacy format)
	if t, err := time.Parse("01-02-2006", dateStr); err == nil {
		return t, nil
	}

	// Try MM/DD/YYYY format
	if t, err := time.Parse("01/02/2006", dateStr); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("unable to parse date '%s' with any known format", dateStr)
}

// End of TransactionStore
