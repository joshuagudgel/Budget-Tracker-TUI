package storage

import (
	"budget-tracker-tui/internal/database"
	"budget-tracker-tui/internal/types"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SnapshotStore handles all snapshot-related operations using SQLite
type SnapshotStore struct {
	db     *database.Connection
	helper *database.SQLHelper
}

// NewSnapshotStore creates a new SnapshotStore instance
func NewSnapshotStore(db *database.Connection) *SnapshotStore {
	return &SnapshotStore{
		db:     db,
		helper: database.NewSQLHelper(db),
	}
}

// GetSnapshots returns all snapshots ordered by creation date (newest first)
func (ss *SnapshotStore) GetSnapshots() ([]types.Snapshot, error) {
	query := `
		SELECT id, name, description, file_path, file_size, 
			   transaction_count, category_count, statement_count, 
			   template_count, audit_event_count, created_at, updated_at
		FROM snapshots
		ORDER BY created_at DESC
	`

	rows, err := ss.helper.QueryRows(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []types.Snapshot
	for rows.Next() {
		var snapshot types.Snapshot
		err := rows.Scan(&snapshot.Id, &snapshot.Name, &snapshot.Description,
			&snapshot.FilePath, &snapshot.FileSize,
			&snapshot.TransactionCount, &snapshot.CategoryCount,
			&snapshot.StatementCount, &snapshot.TemplateCount,
			&snapshot.AuditEventCount, &snapshot.CreatedAt, &snapshot.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan snapshot: %w", err)
		}
		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

// GetSnapshotById returns a snapshot by its ID
func (ss *SnapshotStore) GetSnapshotById(id int64) (*types.Snapshot, error) {
	query := `
		SELECT id, name, description, file_path, file_size, 
			   transaction_count, category_count, statement_count, 
			   template_count, audit_event_count, created_at, updated_at
		FROM snapshots
		WHERE id = ?
	`

	row := ss.helper.QuerySingleRow(query, id)
	var snapshot types.Snapshot
	err := row.Scan(&snapshot.Id, &snapshot.Name, &snapshot.Description,
		&snapshot.FilePath, &snapshot.FileSize,
		&snapshot.TransactionCount, &snapshot.CategoryCount,
		&snapshot.StatementCount, &snapshot.TemplateCount,
		&snapshot.AuditEventCount, &snapshot.CreatedAt, &snapshot.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot by ID %d: %w", id, err)
	}

	return &snapshot, nil
}

// GetSnapshotByName returns a snapshot by its name
func (ss *SnapshotStore) GetSnapshotByName(name string) (*types.Snapshot, error) {
	query := `
		SELECT id, name, description, file_path, file_size, 
			   transaction_count, category_count, statement_count, 
			   template_count, audit_event_count, created_at, updated_at
		FROM snapshots
		WHERE name = ?
	`

	row := ss.helper.QuerySingleRow(query, name)
	var snapshot types.Snapshot
	err := row.Scan(&snapshot.Id, &snapshot.Name, &snapshot.Description,
		&snapshot.FilePath, &snapshot.FileSize,
		&snapshot.TransactionCount, &snapshot.CategoryCount,
		&snapshot.StatementCount, &snapshot.TemplateCount,
		&snapshot.AuditEventCount, &snapshot.CreatedAt, &snapshot.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot by name %s: %w", name, err)
	}

	return &snapshot, nil
}

// CreateSnapshot creates a new snapshot with the given name, description, and file path
func (ss *SnapshotStore) CreateSnapshot(name, description, filePath string) (*SnapshotResult, error) {
	// Validate inputs
	if name == "" {
		return &SnapshotResult{Success: false, Message: "snapshot name cannot be empty"}, nil
	}

	if filePath == "" {
		return &SnapshotResult{Success: false, Message: "snapshot file path cannot be empty"}, nil
	}

	// Check if name already exists
	existing, err := ss.GetSnapshotByName(name)
	if err != nil {
		return &SnapshotResult{Success: false, Message: fmt.Sprintf("failed to check existing snapshot: %v", err)}, nil
	}
	if existing != nil {
		return &SnapshotResult{Success: false, Message: fmt.Sprintf("snapshot with name '%s' already exists", name)}, nil
	}

	// Create the snapshot file first
	err = ss.CreateSnapshotFile(filePath)
	if err != nil {
		return &SnapshotResult{Success: false, Message: fmt.Sprintf("failed to create snapshot file: %v", err)}, nil
	}

	// Get file size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return &SnapshotResult{Success: false, Message: fmt.Sprintf("failed to get file info: %v", err)}, nil
	}

	// Calculate data counts
	txCount, catCount, stmtCount, tmpCount, auditCount, err := ss.CalculateSnapshotCounts()
	if err != nil {
		return &SnapshotResult{Success: false, Message: fmt.Sprintf("failed to calculate data counts: %v", err)}, nil
	}

	// Insert snapshot metadata
	query := `
		INSERT INTO snapshots (name, description, file_path, file_size, 
							  transaction_count, category_count, statement_count, 
							  template_count, audit_event_count) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	snapshotId, err := ss.helper.ExecReturnID(query, name, description, filePath, fileInfo.Size(),
		txCount, catCount, stmtCount, tmpCount, auditCount)
	if err != nil {
		// Clean up the snapshot file if database insert fails
		os.Remove(filePath)
		return &SnapshotResult{Success: false, Message: fmt.Sprintf("failed to save snapshot metadata: %v", err)}, nil
	}

	return &SnapshotResult{
		Success:    true,
		Message:    fmt.Sprintf("snapshot '%s' created successfully", name),
		SnapshotId: snapshotId,
		FilePath:   filePath,
	}, nil
}

// DeleteSnapshot removes a snapshot and its associated file
func (ss *SnapshotStore) DeleteSnapshot(id int64) error {
	// Get snapshot to find file path
	snapshot, err := ss.GetSnapshotById(id)
	if err != nil {
		return fmt.Errorf("failed to get snapshot: %w", err)
	}
	if snapshot == nil {
		return fmt.Errorf("snapshot with ID %d not found", id)
	}

	// Delete from database first (safer order)
	query := "DELETE FROM snapshots WHERE id = ?"
	rowsAffected, err := ss.helper.ExecReturnRowsAffected(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete snapshot from database: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no snapshot found with ID %d", id)
	}

	// Delete the file (ignore errors if file doesn't exist)
	if _, err := os.Stat(snapshot.FilePath); err == nil {
		os.Remove(snapshot.FilePath)
	}

	return nil
}

// CreateSnapshotFile creates a complete database snapshot using SQLite backup API
func (ss *SnapshotStore) CreateSnapshotFile(filePath string) error {
	// Ensure the directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// Remove existing file if it exists
	if _, err := os.Stat(filePath); err == nil {
		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("failed to remove existing snapshot file: %w", err)
		}
	}

	// Use SQLite backup API for atomic snapshot creation
	err := ss.db.ExecuteInTransaction(func(tx *sql.Tx) error {
		// The backup operation needs to be performed on the main connection
		// We'll use a direct backup command
		backupSQL := fmt.Sprintf("VACUUM INTO '%s'", filePath)
		_, err := ss.db.DB.Exec(backupSQL)
		return err
	})

	if err != nil {
		// Clean up partial file on error
		os.Remove(filePath)
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	return nil
}

// RestoreFromSnapshot restores the database from a snapshot
func (ss *SnapshotStore) RestoreFromSnapshot(snapshotId int64) (*RestoreResult, error) {
	// Get snapshot info
	snapshot, err := ss.GetSnapshotById(snapshotId)
	if err != nil {
		return &RestoreResult{Success: false, Message: fmt.Sprintf("failed to get snapshot: %v", err)}, nil
	}
	if snapshot == nil {
		return &RestoreResult{Success: false, Message: fmt.Sprintf("snapshot with ID %d not found", snapshotId)}, nil
	}

	// Validate snapshot file exists
	if err := ss.ValidateSnapshotFile(snapshot.FilePath); err != nil {
		return &RestoreResult{Success: false, Message: fmt.Sprintf("snapshot validation failed: %v", err)}, nil
	}

	// Create a backup of current state first (auto-backup)
	backupPath := fmt.Sprintf("%s.backup.%d.db",
		ss.db.GetPath(),
		time.Now().Unix())

	err = ss.CreateSnapshotFile(backupPath)
	if err != nil {
		return &RestoreResult{Success: false, Message: fmt.Sprintf("failed to create current state backup: %v", err)}, nil
	}

	// Restore from snapshot (this would require database reconnection)
	// For now, return a placeholder - full implementation would require
	// closing current connection and copying snapshot to main database location
	return &RestoreResult{
		Success:    false,
		Message:    "restore functionality not yet implemented - requires database reconnection",
		BackupDate: snapshot.GetCreatedAtDisplay(),
		BackupSize: snapshot.FileSize,
	}, nil
}

// ValidateSnapshotFile checks if a snapshot file is valid
func (ss *SnapshotStore) ValidateSnapshotFile(filePath string) error {
	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("snapshot file not found: %w", err)
	}

	// Check file size
	if fileInfo.Size() == 0 {
		return fmt.Errorf("snapshot file is empty")
	}

	// TODO: Add SQLite file format validation
	// Could open the file and check SQLite header magic bytes

	return nil
}

// UpdateSnapshotMetadata updates an existing snapshot's metadata
func (ss *SnapshotStore) UpdateSnapshotMetadata(snapshot *types.Snapshot) error {
	query := `
		UPDATE snapshots 
		SET name = ?, description = ?, file_size = ?,
			transaction_count = ?, category_count = ?, statement_count = ?, 
			template_count = ?, audit_event_count = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := ss.helper.ExecReturnRowsAffected(query, snapshot.Name, snapshot.Description, snapshot.FileSize,
		snapshot.TransactionCount, snapshot.CategoryCount, snapshot.StatementCount,
		snapshot.TemplateCount, snapshot.AuditEventCount, snapshot.Id)

	if err != nil {
		return fmt.Errorf("failed to update snapshot metadata: %w", err)
	}

	return nil
}

// CalculateSnapshotCounts returns current counts of all major data types
func (ss *SnapshotStore) CalculateSnapshotCounts() (transactionCount, categoryCount, statementCount, templateCount, auditEventCount int, err error) {
	// Count transactions
	txCount, err := ss.helper.CountBy("transactions", "")
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("failed to count transactions: %w", err)
	}
	transactionCount = int(txCount)

	// Count categories
	catCount, err := ss.helper.CountBy("categories", "")
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("failed to count categories: %w", err)
	}
	categoryCount = int(catCount)

	// Count bank statements
	stmtCount, err := ss.helper.CountBy("bank_statements", "")
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("failed to count bank statements: %w", err)
	}
	statementCount = int(stmtCount)

	// Count CSV templates
	tmpCount, err := ss.helper.CountBy("csv_templates", "")
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("failed to count CSV templates: %w", err)
	}
	templateCount = int(tmpCount)

	// Count audit events
	auditCount, err := ss.helper.CountBy("transaction_audit_events", "")
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("failed to count audit events: %w", err)
	}
	auditEventCount = int(auditCount)

	return transactionCount, categoryCount, statementCount, templateCount, auditEventCount, nil
}
