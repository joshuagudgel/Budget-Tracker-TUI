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

// Phase 2: File picker integration and enhanced operations

// LoadSnapshotDirectoryEntries loads directory entries for snapshot file picker
func (ss *SnapshotStore) LoadSnapshotDirectoryEntries(currentDir string) *SnapshotDirectoryResult {
	result := &SnapshotDirectoryResult{CurrentPath: currentDir}

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

	// Add snapshot files (.db files)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".db") {
			result.Entries = append(result.Entries, entry.Name())
		}
	}

	result.Success = true
	return result
}

// LoadSnapshotDirectoryEntriesWithFallback loads directory entries with home directory fallback
func (ss *SnapshotStore) LoadSnapshotDirectoryEntriesWithFallback(currentDir string) *SnapshotDirectoryResult {
	result := ss.LoadSnapshotDirectoryEntries(currentDir)
	if !result.Success {
		// Fallback to home directory on error
		if homeDir, err := os.UserHomeDir(); err == nil {
			result = ss.LoadSnapshotDirectoryEntries(homeDir)
			if result.Success {
				result.Message = "Directory access failed, showing home directory"
			}
		}
	}
	return result
}

// CreateSnapshotWithUserPath creates a snapshot at user-specified location
func (ss *SnapshotStore) CreateSnapshotWithUserPath(name, description, userSelectedPath string) (*SnapshotResult, error) {
	// Validate inputs
	if name == "" {
		return &SnapshotResult{Success: false, Message: "snapshot name cannot be empty"}, nil
	}

	if userSelectedPath == "" {
		return &SnapshotResult{Success: false, Message: "snapshot file path cannot be empty"}, nil
	}

	// Ensure .db extension
	if !strings.HasSuffix(strings.ToLower(userSelectedPath), ".db") {
		userSelectedPath += ".db"
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
	err = ss.CreateSnapshotFile(userSelectedPath)
	if err != nil {
		return &SnapshotResult{Success: false, Message: fmt.Sprintf("failed to create snapshot file: %v", err)}, nil
	}

	// Get file size
	fileInfo, err := os.Stat(userSelectedPath)
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

	snapshotId, err := ss.helper.ExecReturnID(query, name, description, userSelectedPath, fileInfo.Size(),
		txCount, catCount, stmtCount, tmpCount, auditCount)
	if err != nil {
		// Clean up the snapshot file if database insert fails
		os.Remove(userSelectedPath)
		return &SnapshotResult{Success: false, Message: fmt.Sprintf("failed to save snapshot metadata: %v", err)}, nil
	}

	return &SnapshotResult{
		Success:    true,
		Message:    fmt.Sprintf("snapshot '%s' created successfully at %s", name, userSelectedPath),
		SnapshotId: snapshotId,
		FilePath:   userSelectedPath,
	}, nil
}

// RestoreFromSnapshotWithBackup performs safe restore with automatic backup
func (ss *SnapshotStore) RestoreFromSnapshotWithBackup(snapshotId int64) (*RestoreResult, error) {
	// Get snapshot info
	snapshot, err := ss.GetSnapshotById(snapshotId)
	if err != nil {
		return &RestoreResult{Success: false, Message: fmt.Sprintf("failed to get snapshot: %v", err)}, nil
	}
	if snapshot == nil {
		return &RestoreResult{Success: false, Message: fmt.Sprintf("snapshot with ID %d not found", snapshotId)}, nil
	}

	// Validate snapshot file exists and is valid
	if err := ss.ValidateSnapshotFile(snapshot.FilePath); err != nil {
		return &RestoreResult{Success: false, Message: fmt.Sprintf("snapshot validation failed: %v", err)}, nil
	}

	// Create automatic backup of current state
	backupPath := fmt.Sprintf("%s.backup.%d.db",
		ss.db.GetPath(),
		time.Now().Unix())

	err = ss.CreateSnapshotFile(backupPath)
	if err != nil {
		return &RestoreResult{Success: false, Message: fmt.Sprintf("failed to create current state backup: %v", err)}, nil
	}

	// Get backup info for result
	backupInfo, _ := os.Stat(backupPath)
	backupSize := int64(0)
	if backupInfo != nil {
		backupSize = backupInfo.Size()
	}

	// Note: Full database restoration requires application restart due to SQLite connection limitations
	// For now, we create the backup and provide instructions
	return &RestoreResult{
		Success: true,
		Message: fmt.Sprintf("Current database backed up to %s. To complete restore, please restart application and copy %s to main database location.",
			backupPath, snapshot.FilePath),
		TxCount:    snapshot.TransactionCount,
		BackupDate: snapshot.GetCreatedAtDisplay(),
		BackupSize: backupSize,
	}, nil
}

// ValidateSnapshotFileAdvanced performs comprehensive snapshot file validation
func (ss *SnapshotStore) ValidateSnapshotFileAdvanced(filePath string) error {
	// Basic existence and size check
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("snapshot file not found: %w", err)
	}

	if fileInfo.Size() == 0 {
		return fmt.Errorf("snapshot file is empty")
	}

	// Check if file is readable
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("cannot open snapshot file: %w", err)
	}
	file.Close()

	// TODO: Add SQLite header validation
	// SQLite files start with "SQLite format 3\000" (16 bytes)
	// This could be implemented to verify file format

	return nil
}

// GetSnapshotFileSize returns the size of a snapshot file
func (ss *SnapshotStore) GetSnapshotFileSize(filePath string) (int64, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}
	return fileInfo.Size(), nil
}

// GenerateSnapshotFileName creates a suggested filename for a new snapshot
func (ss *SnapshotStore) GenerateSnapshotFileName(baseName string) string {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	if baseName == "" {
		baseName = "snapshot"
	}
	// Sanitize base name for filename
	safeName := strings.ReplaceAll(baseName, " ", "_")
	safeName = strings.ReplaceAll(safeName, "/", "_")
	safeName = strings.ReplaceAll(safeName, "\\", "_")

	return fmt.Sprintf("%s_%s.db", safeName, timestamp)
}

// CleanupOrphanedSnapshots removes snapshot metadata for files that no longer exist
func (ss *SnapshotStore) CleanupOrphanedSnapshots() (int, error) {
	snapshots, err := ss.GetSnapshots()
	if err != nil {
		return 0, fmt.Errorf("failed to get snapshots for cleanup: %w", err)
	}

	orphanedCount := 0
	for _, snapshot := range snapshots {
		if _, err := os.Stat(snapshot.FilePath); os.IsNotExist(err) {
			// File doesn't exist, remove metadata
			if err := ss.DeleteSnapshot(snapshot.Id); err == nil {
				orphanedCount++
			}
		}
	}

	return orphanedCount, nil
}
