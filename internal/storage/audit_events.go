package storage

import (
	"database/sql"
	"fmt"
	"time"

	"budget-tracker-tui/internal/database"
	"budget-tracker-tui/internal/types"
)

// AuditStore handles audit event storage operations
type AuditStore struct {
	db     *database.Connection
	helper *database.SQLHelper
}

// NewAuditStore creates a new audit store instance
func NewAuditStore(db *database.Connection) *AuditStore {
	return &AuditStore{
		db:     db,
		helper: database.NewSQLHelper(db),
	}
}

// RecordEvent creates a new audit event
func (as *AuditStore) RecordEvent(event *types.AuditEvent) error {
	// Validate the event
	if result := event.Validate(); !result.IsValid {
		return fmt.Errorf("validation failed: %v", result.Errors)
	}

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	query := `
		INSERT INTO audit_events (
			timestamp, entity_type, entity_id, event_type, 
			field_name, old_value, new_value, source, context, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	args := []interface{}{
		event.Timestamp.Format(time.RFC3339),
		event.EntityType,
		event.EntityId,
		event.EventType,
		event.FieldName,
		event.OldValue,
		event.NewValue,
		event.Source,
		event.Context,
		time.Now().Format(time.RFC3339),
	}

	id, err := as.helper.ExecReturnID(query, args...)
	if err != nil {
		return fmt.Errorf("failed to insert audit event: %v", err)
	}

	event.Id = id
	return nil
}

// RecordFieldChange creates an audit event for a specific field change
func (as *AuditStore) RecordFieldChange(entityType string, entityId int64, eventType string,
	fieldName string, oldValue interface{}, newValue interface{}, source string, context string) error {

	event := &types.AuditEvent{
		EntityType: entityType,
		EntityId:   entityId,
		EventType:  eventType,
		FieldName:  fieldName,
		OldValue:   fmt.Sprintf("%v", oldValue),
		NewValue:   fmt.Sprintf("%v", newValue),
		Source:     source,
		Context:    context,
		Timestamp:  time.Now(),
	}

	return as.RecordEvent(event)
}

// RecordMultipleFieldChanges creates multiple audit events in a single transaction
func (as *AuditStore) RecordMultipleFieldChanges(changes []FieldChange) error {
	return as.db.ExecuteInTransaction(func(tx *sql.Tx) error {
		for _, change := range changes {
			query := `
				INSERT INTO audit_events (
					timestamp, entity_type, entity_id, event_type, 
					field_name, old_value, new_value, source, context, created_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

			args := []interface{}{
				time.Now().Format(time.RFC3339),
				change.EntityType,
				change.EntityId,
				change.EventType,
				change.FieldName,
				change.OldValue,
				change.NewValue,
				change.Source,
				change.Context,
				time.Now().Format(time.RFC3339),
			}

			_, err := tx.Exec(query, args...)
			if err != nil {
				return fmt.Errorf("failed to insert audit event for field %s: %v", change.FieldName, err)
			}
		}
		return nil
	})
}

// GetEventsByEntity retrieves all audit events for a specific entity
func (as *AuditStore) GetEventsByEntity(entityType string, entityId int64) ([]types.AuditEvent, error) {
	query := `
		SELECT id, timestamp, entity_type, entity_id, event_type, 
			   field_name, old_value, new_value, source, context, created_at
		FROM audit_events 
		WHERE entity_type = ? AND entity_id = ?
		ORDER BY timestamp DESC`

	rows, err := as.helper.QueryRows(query, entityType, entityId)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit events: %v", err)
	}
	defer rows.Close()

	var events []types.AuditEvent
	for rows.Next() {
		var event types.AuditEvent
		var timestampStr, createdAtStr string
		var fieldName, oldValue, newValue, context sql.NullString

		err := rows.Scan(
			&event.Id,
			&timestampStr,
			&event.EntityType,
			&event.EntityId,
			&event.EventType,
			&fieldName,
			&oldValue,
			&newValue,
			&event.Source,
			&context,
			&createdAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit event: %v", err)
		}

		// Parse timestamps
		if event.Timestamp, err = as.helper.ParseTimeFromDB(timestampStr); err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %v", err)
		}
		if event.CreatedAt, err = as.helper.ParseTimeFromDB(createdAtStr); err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %v", err)
		}

		// Handle nullable fields
		event.FieldName = fieldName.String
		event.OldValue = oldValue.String
		event.NewValue = newValue.String
		event.Context = context.String

		events = append(events, event)
	}

	return events, nil
}

// GetEventsByTimeRange retrieves audit events within a time range
func (as *AuditStore) GetEventsByTimeRange(startTime, endTime time.Time) ([]types.AuditEvent, error) {
	query := `
		SELECT id, timestamp, entity_type, entity_id, event_type, 
			   field_name, old_value, new_value, source, context, created_at
		FROM audit_events 
		WHERE timestamp BETWEEN ? AND ?
		ORDER BY timestamp DESC`

	startStr := startTime.Format(time.RFC3339)
	endStr := endTime.Format(time.RFC3339)

	rows, err := as.helper.QueryRows(query, startStr, endStr)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit events by time range: %v", err)
	}
	defer rows.Close()

	var events []types.AuditEvent
	for rows.Next() {
		var event types.AuditEvent
		var timestampStr, createdAtStr string
		var fieldName, oldValue, newValue, context sql.NullString

		err := rows.Scan(
			&event.Id,
			&timestampStr,
			&event.EntityType,
			&event.EntityId,
			&event.EventType,
			&fieldName,
			&oldValue,
			&newValue,
			&event.Source,
			&context,
			&createdAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit event: %v", err)
		}

		// Parse timestamps
		if event.Timestamp, err = as.helper.ParseTimeFromDB(timestampStr); err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %v", err)
		}
		if event.CreatedAt, err = as.helper.ParseTimeFromDB(createdAtStr); err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %v", err)
		}

		// Handle nullable fields
		event.FieldName = fieldName.String
		event.OldValue = oldValue.String
		event.NewValue = newValue.String
		event.Context = context.String

		events = append(events, event)
	}

	return events, nil
}

// GetEventsByEventType retrieves audit events by event type
func (as *AuditStore) GetEventsByEventType(eventType string) ([]types.AuditEvent, error) {
	query := `
		SELECT id, timestamp, entity_type, entity_id, event_type, 
			   field_name, old_value, new_value, source, context, created_at
		FROM audit_events 
		WHERE event_type = ?
		ORDER BY timestamp DESC`

	rows, err := as.helper.QueryRows(query, eventType)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit events by event type: %v", err)
	}
	defer rows.Close()

	var events []types.AuditEvent
	for rows.Next() {
		var event types.AuditEvent
		var timestampStr, createdAtStr string
		var fieldName, oldValue, newValue, context sql.NullString

		err := rows.Scan(
			&event.Id,
			&timestampStr,
			&event.EntityType,
			&event.EntityId,
			&event.EventType,
			&fieldName,
			&oldValue,
			&newValue,
			&event.Source,
			&context,
			&createdAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit event: %v", err)
		}

		// Parse timestamps
		if event.Timestamp, err = as.helper.ParseTimeFromDB(timestampStr); err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %v", err)
		}
		if event.CreatedAt, err = as.helper.ParseTimeFromDB(createdAtStr); err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %v", err)
		}

		// Handle nullable fields
		event.FieldName = fieldName.String
		event.OldValue = oldValue.String
		event.NewValue = newValue.String
		event.Context = context.String

		events = append(events, event)
	}

	return events, nil
}

// FieldChange represents a single field modification for batch operations
type FieldChange struct {
	EntityType string
	EntityId   int64
	EventType  string
	FieldName  string
	OldValue   string
	NewValue   string
	Source     string
	Context    string
}

// GetRecentEvents retrieves the most recent audit events (for testing/debugging)
func (as *AuditStore) GetRecentEvents(limit int) ([]types.AuditEvent, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT id, timestamp, entity_type, entity_id, event_type, 
			   field_name, old_value, new_value, source, context, created_at
		FROM audit_events 
		ORDER BY timestamp DESC 
		LIMIT ?`

	rows, err := as.helper.QueryRows(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent audit events: %v", err)
	}
	defer rows.Close()

	var events []types.AuditEvent
	for rows.Next() {
		var event types.AuditEvent
		var timestampStr, createdAtStr string
		var fieldName, oldValue, newValue, context sql.NullString

		err := rows.Scan(
			&event.Id,
			&timestampStr,
			&event.EntityType,
			&event.EntityId,
			&event.EventType,
			&fieldName,
			&oldValue,
			&newValue,
			&event.Source,
			&context,
			&createdAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit event: %v", err)
		}

		// Parse timestamps
		if event.Timestamp, err = as.helper.ParseTimeFromDB(timestampStr); err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %v", err)
		}
		if event.CreatedAt, err = as.helper.ParseTimeFromDB(createdAtStr); err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %v", err)
		}

		// Handle nullable fields
		event.FieldName = fieldName.String
		event.OldValue = oldValue.String
		event.NewValue = newValue.String
		event.Context = context.String

		events = append(events, event)
	}

	return events, nil
}

// GetEventCount returns total number of audit events (for testing)
func (as *AuditStore) GetEventCount() (int, error) {
	var count int
	err := as.db.DB.QueryRow("SELECT COUNT(*) FROM audit_events").Scan(&count)
	return count, err
}

// PrintRecentEvents prints recent audit events to console (for debugging)
func (as *AuditStore) PrintRecentEvents(limit int) error {
	events, err := as.GetRecentEvents(limit)
	if err != nil {
		return err
	}

	fmt.Printf("\n=== Recent Audit Events (Last %d) ===\n", len(events))
	for _, event := range events {
		fmt.Printf("[%s] %s:%d %s",
			event.Timestamp.Format("15:04:05"),
			event.EntityType,
			event.EntityId,
			event.EventType)

		if event.FieldName != "" {
			fmt.Printf(" %s: %s->%s", event.FieldName, event.OldValue, event.NewValue)
		}

		fmt.Printf(" [%s]\n", event.Source)
	}
	fmt.Printf("=== Total Events: %d ===\n\n", len(events))

	return nil
}
