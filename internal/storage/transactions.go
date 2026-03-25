package storage

import (
	"budget-tracker-tui/internal/database"
	"budget-tracker-tui/internal/types"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"
)

// TransactionStore handles all transaction-related operations using SQLite
type TransactionStore struct {
	db                *database.Connection
	helper            *database.SQLHelper
	transactionAudits *TransactionAuditStore
	store             *Store // Reference to main store for ML access
	debugLogger       *log.Logger
}

// NewTransactionStore creates a new TransactionStore instance
func NewTransactionStore(db *database.Connection) *TransactionStore {
	// Create debug logger that writes to debug.log
	debugFile, err := os.OpenFile("debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	var debugLogger *log.Logger
	if err != nil {
		// Fallback to stdout if file can't be opened
		debugLogger = log.New(os.Stdout, "[DEBUG] ", log.LstdFlags)
	} else {
		debugLogger = log.New(debugFile, "", log.LstdFlags)
		// Log initialization message
		debugLogger.Printf("[INIT] TransactionStore debug logging initialized")
	}

	return &TransactionStore{
		db:          db,
		helper:      database.NewSQLHelper(db),
		debugLogger: debugLogger,
	}
}

// SetTransactionAuditStore sets the transaction audit store reference (called after all stores are initialized)
func (ts *TransactionStore) SetTransactionAuditStore(tas *TransactionAuditStore) {
	ts.transactionAudits = tas
}

// SetStore sets the main store reference for ML access (called after all stores are initialized)
func (ts *TransactionStore) SetStore(s *Store) {
	ts.store = s
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
		       category_id, transaction_type, is_split, 
		       statement_id, created_at, updated_at 
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
		       category_id, transaction_type, is_split, 
		       statement_id, created_at, updated_at 
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
	var dateStr, createdAtStr, updatedAtStr string

	err := rows.Scan(
		&tx.Id, &parentID, &tx.Amount, &tx.Description, &rawDescription,
		&dateStr, &tx.CategoryId, &tx.TransactionType,
		&tx.IsSplit, &statementID, &createdAtStr, &updatedAtStr,
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
		tx.StatementId = statementID.Int64
	}
	if rawDescription.Valid {
		tx.RawDescription = rawDescription.String
	}

	return tx, nil
}

// GetTransactionByID returns a transaction by ID
func (ts *TransactionStore) GetTransactionByID(id int64) *types.Transaction {
	query := `
		SELECT id, parent_id, amount, description, raw_description, date, 
		       category_id, transaction_type, is_split, 
		       statement_id, created_at, updated_at 
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
	var dateStr, createdAtStr, updatedAtStr string

	err := row.Scan(
		&tx.Id, &parentID, &tx.Amount, &tx.Description, &rawDescription,
		&dateStr, &tx.CategoryId, &tx.TransactionType,
		&tx.IsSplit, &statementID, &createdAtStr, &updatedAtStr,
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

	// Handle nullable fields
	if parentID.Valid {
		tx.ParentId = &parentID.Int64
	}
	if statementID.Valid {
		tx.StatementId = statementID.Int64
	}
	if rawDescription.Valid {
		tx.RawDescription = rawDescription.String
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
			category_id, transaction_type, is_split, 
			statement_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// Convert nullable fields
	var parentID interface{}
	if transaction.ParentId != nil {
		parentID = *transaction.ParentId
	}

	var statementID interface{}
	if transaction.StatementId != 0 {
		statementID = transaction.StatementId
	}

	var rawDescription interface{}
	if transaction.RawDescription != "" {
		rawDescription = transaction.RawDescription
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
		dateStr, transaction.CategoryId, transaction.TransactionType,
		transaction.IsSplit, statementID, createdAtStr, updatedAtStr,
	)

	if err != nil {
		return fmt.Errorf("failed to insert transaction: %w", err)
	}

	// Update the transaction with the generated ID
	transaction.Id = id

	// No audit event creation for new transactions

	return nil
}

// updateTransaction updates an existing transaction in the database
func (ts *TransactionStore) updateTransaction(transaction types.Transaction, now time.Time) error {
	// Get old transaction for logging if needed
	oldTransaction := ts.GetTransactionByID(transaction.Id)
	if oldTransaction == nil {
		return fmt.Errorf("transaction not found")
	}

	query := `
		UPDATE transactions SET 
			parent_id = ?, amount = ?, description = ?, raw_description = ?, 
			date = ?, category_id = ?, transaction_type = ?, 
			is_split = ?, statement_id = ?, updated_at = ?
		WHERE id = ?
	`

	// Convert nullable fields
	var parentID interface{}
	if transaction.ParentId != nil {
		parentID = *transaction.ParentId
	}

	var statementID interface{}
	if transaction.StatementId != 0 {
		statementID = transaction.StatementId
	}

	var rawDescription interface{}
	if transaction.RawDescription != "" {
		rawDescription = transaction.RawDescription
	}

	_, err := ts.helper.ExecReturnRowsAffected(query,
		parentID, transaction.Amount, transaction.Description, rawDescription,
		transaction.Date, transaction.CategoryId, transaction.TransactionType,
		transaction.IsSplit, statementID, now, transaction.Id,
	)

	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	// Log audit events for field changes
	ts.logTransactionFieldChanges(oldTransaction, &transaction)

	return nil
}

// logTransactionFieldChanges creates a transaction audit event for edits
func (ts *TransactionStore) logTransactionFieldChanges(oldTx, newTx *types.Transaction) {
	// Determine what was modified
	var modificationReason *string

	if oldTx.Description != newTx.Description {
		reason := types.ModReasonDescription
		modificationReason = &reason
	} else if oldTx.TransactionType != newTx.TransactionType {
		reason := types.ModReasonTransactionType
		modificationReason = &reason
	} else if oldTx.CategoryId != newTx.CategoryId {
		reason := types.ModReasonCategory
		modificationReason = &reason
	}

	// Get bank statement ID
	bankStatementId := newTx.StatementId

	// Create pre and post snapshots (simplified JSON-like format for now)
	preSnapshot := fmt.Sprintf("{\"amount\":%.2f,\"description\":\"%s\",\"category\":%d,\"type\":\"%s\"}",
		oldTx.Amount, oldTx.Description, oldTx.CategoryId, oldTx.TransactionType)
	postSnapshot := fmt.Sprintf("{\"amount\":%.2f,\"description\":\"%s\",\"category\":%d,\"type\":\"%s\"}",
		newTx.Amount, newTx.Description, newTx.CategoryId, newTx.TransactionType)

	auditEvent := &types.TransactionAuditEvent{
		TransactionId:          newTx.Id,
		BankStatementId:        bankStatementId,
		Timestamp:              time.Now(),
		ActionType:             types.ActionTypeEdit,
		Source:                 types.SourceUser,
		DescriptionFingerprint: newTx.Description,
		CategoryAssigned:       newTx.CategoryId,
		CategoryConfidence:     1.0,
		PreviousCategory:       oldTx.CategoryId,
		ModificationReason:     modificationReason,
		PreEditSnapshot:        &preSnapshot,
		PostEditSnapshot:       &postSnapshot,
	}

	ts.transactionAudits.RecordEvent(auditEvent)
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

	// Note: For now we're not recording delete events in TransactionAuditEvent
	// since deleted transactions won't have valid references
	// This could be enhanced in the future if needed

	return nil
}

// SplitTransaction splits a updates current transaction into new values and creates a split transaction linked to itself
func (ts *TransactionStore) SplitTransaction(parentId int64, splits []types.Transaction) error {
	// Get original transaction for audit logging
	var originalTransaction *types.Transaction
	if ts.transactionAudits != nil {
		originalTransaction = ts.GetTransactionByID(parentId)
	}

	var secondSplitId int64 // Capture ID of newly created second split

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
				updated_at = ?
			WHERE id = ?
		`
		_, err := tx.Exec(updateQuery,
			splits[0].Amount, splits[0].Description, splits[0].CategoryId,
			true, now, parentId,
		)
		if err != nil {
			return fmt.Errorf("failed to update parent transaction: %w", err)
		}

		// Create second split as new transaction
		insertQuery := `
			INSERT INTO transactions (
				amount, description, date, category_id, transaction_type, 
				statement_id, is_split, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

		var statementID interface{}
		if parent.StatementId != 0 {
			statementID = parent.StatementId
		}

		result, err := tx.Exec(insertQuery,
			splits[1].Amount, splits[1].Description, parent.Date,
			splits[1].CategoryId, parent.TransactionType, statementID,
			false, now, now,
		)
		if err != nil {
			return fmt.Errorf("failed to insert second split: %w", err)
		}

		// Capture the ID of the newly created second split transaction
		secondSplitId, err = result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get second split ID: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Log audit event for split transaction after successful database transaction
	if ts.transactionAudits != nil && originalTransaction != nil {
		// Get bank statement ID
		bankStatementId := originalTransaction.StatementId

		// Create split audit event for parent (first split)
		parentAuditEvent := &types.TransactionAuditEvent{
			TransactionId:          parentId,
			BankStatementId:        bankStatementId,
			Timestamp:              time.Now(),
			ActionType:             types.ActionTypeSplit,
			Source:                 types.SourceUser,
			DescriptionFingerprint: splits[0].Description,
			CategoryAssigned:       splits[0].CategoryId,
			CategoryConfidence:     1.0,
			PreviousCategory:       originalTransaction.CategoryId,
		}

		ts.transactionAudits.RecordEvent(parentAuditEvent)

		// Create split audit event for second split (newly created transaction)
		if secondSplitId > 0 {
			secondSplitAuditEvent := &types.TransactionAuditEvent{
				TransactionId:          secondSplitId,
				BankStatementId:        bankStatementId,
				Timestamp:              time.Now(),
				ActionType:             types.ActionTypeSplit,
				Source:                 types.SourceUser,
				DescriptionFingerprint: splits[1].Description,
				CategoryAssigned:       splits[1].CategoryId,
				CategoryConfidence:     1.0,
				PreviousCategory:       originalTransaction.CategoryId,
			}

			ts.transactionAudits.RecordEvent(secondSplitAuditEvent)
		}
	}

	return nil
}

// ImportTransactionsFromCSV imports a batch of transactions from CSV parsing
func (ts *TransactionStore) ImportTransactionsFromCSV(transactions []types.Transaction, statementId int64) error {
	if len(transactions) == 0 {
		return nil
	}

	// Prepare bulk insert data
	var records [][]interface{}
	now := time.Now()

	var statementID interface{}
	if statementId != 0 {
		statementID = statementId
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

		record := []interface{}{
			parentID, tx.Amount, tx.Description, rawDescription, dateStr,
			tx.CategoryId, transactionType, tx.IsSplit,
			statementID, createdAtStr, updatedAtStr,
		}
		records = append(records, record)
	}

	// Bulk insert using transaction
	fields := []string{
		"parent_id", "amount", "description", "raw_description", "date",
		"category_id", "transaction_type", "is_split",
		"statement_id", "created_at", "updated_at",
	}

	err := ts.helper.BulkInsert("transactions", fields, records)
	if err != nil {
		return err
	}

	// Create audit events for imported transactions with ML prediction tracking
	if ts.transactionAudits != nil {
		ts.debugLogger.Printf("[DEBUG] Creating audit events for %d imported transactions", len(transactions))
		err = ts.createImportAuditEvents(transactions, statementId)
		if err != nil {
			// Log error but don't fail the import - audit is supplementary
			ts.debugLogger.Printf("[Warning] Failed to create import audit events: %v", err)
		}
	} else {
		ts.debugLogger.Printf("[WARNING] TransactionAudits store is nil - audit events will not be created")
	}

	return nil
}

// FindDuplicateTransactions finds existing transactions that match date, amount, and description
func (ts *TransactionStore) FindDuplicateTransactions(date string, amount float64, description string) ([]types.Transaction, error) {
	query := `
		SELECT id, parent_id, amount, description, raw_description, date, 
		       category_id, transaction_type, is_split, 
		       statement_id, created_at, updated_at 
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

// createImportAuditEvents creates audit events for imported transactions with ML prediction tracking
func (ts *TransactionStore) createImportAuditEvents(transactions []types.Transaction, statementId int64) error {
	ts.debugLogger.Printf("[DEBUG] createImportAuditEvents called with %d transactions, statementId=%d", len(transactions), statementId)

	// Use statement ID directly
	bankStatementId := statementId
	ts.debugLogger.Printf("[DEBUG] Parsed bankStatementId: %d", bankStatementId)

	// Query for the actual inserted transactions to get their database IDs
	// We'll match by statement_id, description, amount, and date to identify each transaction
	for _, tx := range transactions {
		// Find the actual inserted transaction by its unique attributes
		query := `
			SELECT id FROM transactions 
			WHERE statement_id = ? AND description = ? AND amount = ? AND date = ?
			ORDER BY id DESC LIMIT 1
		`

		var actualTxId int64
		dateStr := tx.Date.Format("2006-01-02")
		err := ts.helper.QuerySingleRow(query, bankStatementId, tx.Description, tx.Amount, dateStr).Scan(&actualTxId)
		if err != nil {
			ts.debugLogger.Printf("[Warning] Could not find inserted transaction ID for %s: %v", tx.Description, err)
			continue
		}

		// Determine if this was ML auto-categorized
		var source string = types.SourceImport // Default to import source
		var confidenceScore float64 = 0.0

		if ts.store != nil && ts.store.MLCategorizer != nil {
			prediction := ts.store.PredictCategory(tx.Description, tx.Amount)
			confidenceScore = prediction.Confidence
			ts.debugLogger.Printf("[DEBUG] ML Prediction for '%s': CategoryId=%d, Confidence=%.2f, Assigned=%d", tx.Description, prediction.CategoryId, prediction.Confidence, tx.CategoryId)

			// Validate confidence score is in expected range
			if confidenceScore < 0.0 || confidenceScore > 1.0 {
				ts.debugLogger.Printf("[WARNING] ML confidence score out of range (0.0-1.0): %.2f for '%s'", confidenceScore, tx.Description)
				confidenceScore = 0.0 // Reset to safe value
			}

			// If ML made a high confidence prediction and it matches the assigned category, it was auto-categorized
			if ts.store.IsHighConfidencePrediction(prediction) && prediction.CategoryId == tx.CategoryId {
				source = types.SourceAuto
				ts.debugLogger.Printf("[DEBUG] Detected ML auto-categorization for '%s'", tx.Description)
			}
		}

		// Create appropriate audit event based on source
		var postSnapshot *string
		var modReason *string
		var preSnapshot *string

		// Only create audit events for ML auto-categorized transactions
		if source == types.SourceAuto {
			// For auto-categorization, include the required fields
			postSnapshotStr := fmt.Sprintf(`{"id":%d,"amount":%.2f,"description":"%s","category":%d,"type":"%s","date":"%s"}`,
				actualTxId, tx.Amount, tx.Description, tx.CategoryId, tx.TransactionType, tx.Date.Format("2006-01-02"))
			postSnapshot = &postSnapshotStr

			modReasonStr := types.ModReasonCategory
			modReason = &modReasonStr

			preSnapshotStr := ""
			preSnapshot = &preSnapshotStr

			auditEvent := &types.TransactionAuditEvent{
				TransactionId:          actualTxId, // Use the actual database ID
				BankStatementId:        bankStatementId,
				Timestamp:              time.Now(),
				ActionType:             types.ActionTypeImport,
				Source:                 source, // 'auto' for ML categorization
				DescriptionFingerprint: tx.Description,
				CategoryAssigned:       tx.CategoryId,
				CategoryConfidence:     confidenceScore, // Use actual ML confidence score
				PreviousCategory:       tx.CategoryId,   // Same as assigned for new imports (no previous state)
				ModificationReason:     modReason,       // Set for auto-categorization
				PreEditSnapshot:        preSnapshot,     // Set for auto-categorization
				PostEditSnapshot:       postSnapshot,    // Set for auto-categorization
			}

			// Record the audit event
			ts.debugLogger.Printf("[DEBUG] Creating audit event: source=%s, confidence=%.2f, categoryId=%d, previousCategory=%d",
				source, confidenceScore, tx.CategoryId, tx.CategoryId)

			// Validate foreign key references before creating audit event
			ts.debugLogger.Printf("[DEBUG] Validating audit event references - TxId:%d, StatementId:%d, CategoryId:%d",
				actualTxId, bankStatementId, tx.CategoryId)

			// Check if transaction exists
			if txExists := ts.GetTransactionByID(actualTxId); txExists == nil {
				ts.debugLogger.Printf("[ERROR] Transaction ID %d does not exist - cannot create audit event", actualTxId)
				continue
			}

			// Check if bank statement exists (if StatementId > 0)
			if bankStatementId > 0 {
				checkStmtQuery := "SELECT COUNT(*) FROM bank_statements WHERE id = ?"
				var stmtCount int
				err := ts.helper.QuerySingleRow(checkStmtQuery, bankStatementId).Scan(&stmtCount)
				if err != nil || stmtCount == 0 {
					ts.debugLogger.Printf("[ERROR] Bank statement ID %d does not exist - cannot create audit event", bankStatementId)
					continue
				}
			}

			// Check if category exists
			checkCatQuery := "SELECT COUNT(*) FROM categories WHERE id = ?"
			var catCount int
			err = ts.helper.QuerySingleRow(checkCatQuery, tx.CategoryId).Scan(&catCount)
			if err != nil || catCount == 0 {
				ts.debugLogger.Printf("[ERROR] Category ID %d does not exist - cannot create audit event", tx.CategoryId)
				continue
			}

			ts.debugLogger.Printf("[DEBUG] All foreign key references validated successfully")

			err = ts.transactionAudits.RecordEvent(auditEvent)
			if err != nil {
				// Log individual failures but continue with other events
				ts.debugLogger.Printf("[ERROR] Failed to create audit event for transaction %s: %v", tx.Description, err)
				ts.debugLogger.Printf("[ERROR] Audit event details - TxId:%d, StatementId:%d, CategoryId:%d, Source:%s",
					actualTxId, bankStatementId, tx.CategoryId, source)
			} else {
				ts.debugLogger.Printf("[ML] Created audit event for auto-categorized transaction: %s → Category %d", tx.Description, tx.CategoryId)
			}
		} else {
			// Regular import transactions don't need audit events
			ts.debugLogger.Printf("[DEBUG] Skipping audit event for regular import transaction: %s", tx.Description)
		}
	}

	return nil
}

// End of TransactionStore
