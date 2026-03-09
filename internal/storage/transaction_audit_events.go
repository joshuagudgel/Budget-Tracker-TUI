package storage

import (
	"database/sql"
	"fmt"
	"time"

	"budget-tracker-tui/internal/database"
	"budget-tracker-tui/internal/types"
)

// TransactionAuditStore handles transaction audit event storage operations
type TransactionAuditStore struct {
	db     *database.Connection
	helper *database.SQLHelper
}

// NewTransactionAuditStore creates a new transaction audit store instance
func NewTransactionAuditStore(db *database.Connection) *TransactionAuditStore {
	return &TransactionAuditStore{
		db:     db,
		helper: database.NewSQLHelper(db),
	}
}

// RecordEvent creates a new transaction audit event
func (tas *TransactionAuditStore) RecordEvent(event *types.TransactionAuditEvent) error {
	// Validate the event
	if result := event.Validate(); !result.IsValid {
		return fmt.Errorf("validation failed: %v", result.Errors)
	}

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	query := `
		INSERT INTO transaction_audit_events (
			transaction_id, bank_statement_id, timestamp, action_type, source,
			description_fingerprint, merchant_extracted, amount_range, category_assigned,
			category_confidence, alternative_categories, modification_reason,
			pre_edit_snapshot, post_edit_snapshot, edit_latency, processing_time_ms, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	args := []interface{}{
		event.TransactionId,
		event.BankStatementId,
		event.Timestamp.Format(time.RFC3339),
		event.ActionType,
		event.Source,
		event.DescriptionFingerprint,
		sql.NullString{String: event.MerchantExtracted, Valid: event.MerchantExtracted != ""},
		sql.NullString{String: event.AmountRange, Valid: event.AmountRange != ""},
		event.CategoryAssigned,
		sql.NullFloat64{Float64: event.CategoryConfidence, Valid: event.CategoryConfidence > 0},
		sql.NullString{String: event.AlternativeCategories, Valid: event.AlternativeCategories != ""},
		getNullString(event.ModificationReason),
		getNullString(event.PreEditSnapshot),
		getNullString(event.PostEditSnapshot),
		sql.NullInt64{Int64: int64(event.EditLatency), Valid: event.EditLatency > 0},
		sql.NullInt64{Int64: int64(event.ProcessingTimeMs), Valid: event.ProcessingTimeMs > 0},
		time.Now().Format(time.RFC3339),
	}

	id, err := tas.helper.ExecReturnID(query, args...)
	if err != nil {
		return fmt.Errorf("failed to insert transaction audit event: %v", err)
	}

	event.Id = id
	return nil
}

// GetEventsByTransaction retrieves all audit events for a specific transaction
func (tas *TransactionAuditStore) GetEventsByTransaction(transactionId int64) ([]types.TransactionAuditEvent, error) {
	query := `
		SELECT id, transaction_id, bank_statement_id, timestamp, action_type, source,
			   description_fingerprint, merchant_extracted, amount_range, category_assigned,
			   category_confidence, alternative_categories, modification_reason,
			   pre_edit_snapshot, post_edit_snapshot, edit_latency, processing_time_ms, created_at
		FROM transaction_audit_events 
		WHERE transaction_id = ?
		ORDER BY timestamp DESC`

	rows, err := tas.helper.QueryRows(query, transactionId)
	if err != nil {
		return nil, fmt.Errorf("failed to query transaction audit events: %v", err)
	}
	defer rows.Close()

	return tas.scanTransactionAuditEvents(rows)
}

// GetEventsByStatement retrieves all audit events for transactions in a bank statement
func (tas *TransactionAuditStore) GetEventsByStatement(bankStatementId int64) ([]types.TransactionAuditEvent, error) {
	query := `
		SELECT id, transaction_id, bank_statement_id, timestamp, action_type, source,
			   description_fingerprint, merchant_extracted, amount_range, category_assigned,
			   category_confidence, alternative_categories, modification_reason,
			   pre_edit_snapshot, post_edit_snapshot, edit_latency, processing_time_ms, created_at
		FROM transaction_audit_events 
		WHERE bank_statement_id = ?
		ORDER BY timestamp DESC`

	rows, err := tas.helper.QueryRows(query, bankStatementId)
	if err != nil {
		return nil, fmt.Errorf("failed to query transaction audit events by statement: %v", err)
	}
	defer rows.Close()

	return tas.scanTransactionAuditEvents(rows)
}

// GetEventsByTimeRange retrieves audit events within a time range
func (tas *TransactionAuditStore) GetEventsByTimeRange(startTime, endTime time.Time) ([]types.TransactionAuditEvent, error) {
	query := `
		SELECT id, transaction_id, bank_statement_id, timestamp, action_type, source,
			   description_fingerprint, merchant_extracted, amount_range, category_assigned,
			   category_confidence, alternative_categories, modification_reason,
			   pre_edit_snapshot, post_edit_snapshot, edit_latency, processing_time_ms, created_at
		FROM transaction_audit_events 
		WHERE timestamp BETWEEN ? AND ?
		ORDER BY timestamp DESC`

	startStr := startTime.Format(time.RFC3339)
	endStr := endTime.Format(time.RFC3339)

	rows, err := tas.helper.QueryRows(query, startStr, endStr)
	if err != nil {
		return nil, fmt.Errorf("failed to query transaction audit events by time range: %v", err)
	}
	defer rows.Close()

	return tas.scanTransactionAuditEvents(rows)
}

// GetEventsByActionType retrieves audit events by action type
func (tas *TransactionAuditStore) GetEventsByActionType(actionType string) ([]types.TransactionAuditEvent, error) {
	query := `
		SELECT id, transaction_id, bank_statement_id, timestamp, action_type, source,
			   description_fingerprint, merchant_extracted, amount_range, category_assigned,
			   category_confidence, alternative_categories, modification_reason,
			   pre_edit_snapshot, post_edit_snapshot, edit_latency, processing_time_ms, created_at
		FROM transaction_audit_events 
		WHERE action_type = ?
		ORDER BY timestamp DESC`

	rows, err := tas.helper.QueryRows(query, actionType)
	if err != nil {
		return nil, fmt.Errorf("failed to query transaction audit events by action type: %v", err)
	}
	defer rows.Close()

	return tas.scanTransactionAuditEvents(rows)
}

// GetRecentEvents retrieves the most recent transaction audit events
func (tas *TransactionAuditStore) GetRecentEvents(limit int) ([]types.TransactionAuditEvent, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT id, transaction_id, bank_statement_id, timestamp, action_type, source,
			   description_fingerprint, merchant_extracted, amount_range, category_assigned,
			   category_confidence, alternative_categories, modification_reason,
			   pre_edit_snapshot, post_edit_snapshot, edit_latency, processing_time_ms, created_at
		FROM transaction_audit_events 
		ORDER BY timestamp DESC 
		LIMIT ?`

	rows, err := tas.helper.QueryRows(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent transaction audit events: %v", err)
	}
	defer rows.Close()

	return tas.scanTransactionAuditEvents(rows)
}

// scanTransactionAuditEvents is a helper function to scan audit events from rows
func (tas *TransactionAuditStore) scanTransactionAuditEvents(rows *sql.Rows) ([]types.TransactionAuditEvent, error) {
	var events []types.TransactionAuditEvent

	for rows.Next() {
		var event types.TransactionAuditEvent
		var timestampStr, createdAtStr string
		var merchantExtracted, amountRange, alternativeCategories sql.NullString
		var modificationReason, preEditSnapshot, postEditSnapshot sql.NullString
		var categoryConfidence sql.NullFloat64
		var editLatency, processingTimeMs sql.NullInt64

		err := rows.Scan(
			&event.Id,
			&event.TransactionId,
			&event.BankStatementId,
			&timestampStr,
			&event.ActionType,
			&event.Source,
			&event.DescriptionFingerprint,
			&merchantExtracted,
			&amountRange,
			&event.CategoryAssigned,
			&categoryConfidence,
			&alternativeCategories,
			&modificationReason,
			&preEditSnapshot,
			&postEditSnapshot,
			&editLatency,
			&processingTimeMs,
			&createdAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction audit event: %v", err)
		}

		// Parse timestamps
		if event.Timestamp, err = tas.helper.ParseTimeFromDB(timestampStr); err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %v", err)
		}
		if event.CreatedAt, err = tas.helper.ParseTimeFromDB(createdAtStr); err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %v", err)
		}

		// Handle nullable fields
		event.MerchantExtracted = merchantExtracted.String
		event.AmountRange = amountRange.String
		event.CategoryConfidence = categoryConfidence.Float64
		event.AlternativeCategories = alternativeCategories.String
		event.ModificationReason = nullStringToPointer(modificationReason)
		event.PreEditSnapshot = nullStringToPointer(preEditSnapshot)
		event.PostEditSnapshot = nullStringToPointer(postEditSnapshot)
		event.EditLatency = int(editLatency.Int64)
		event.ProcessingTimeMs = int(processingTimeMs.Int64)

		events = append(events, event)
	}

	return events, nil
}

// Helper functions for nullable fields
func getNullString(strPtr *string) sql.NullString {
	if strPtr == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *strPtr, Valid: true}
}

func nullStringToPointer(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}
