package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Helper methods for snapshot operations

// createCurrentDatabaseBackup creates a backup of the current database
func (m model) createCurrentDatabaseBackup() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	backupDir := filepath.Join(homeDir, ".finance-wrapped", "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("backup_before_restore_%s.db", timestamp))

	err = m.store.Snapshots.CreateSnapshotFile(backupPath)
	if err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	return backupPath, nil
}

// copyFile copies a file from src to dst
func (m model) copyFile(src, dst string) error {
	// Read source file
	sourceData, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	// Write to destination
	err = os.WriteFile(dst, sourceData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	return nil
}

// restoreFromSnapshotFile performs actual database restore by copying snapshot file
func (m model) restoreFromSnapshotFile(snapshotPath string) error {
	// Validate the snapshot file exists and is readable
	if _, err := os.Stat(snapshotPath); err != nil {
		return fmt.Errorf("snapshot file not accessible: %w", err)
	}

	// Get current database path
	currentDBPath := m.store.GetDatabasePath()
	if currentDBPath == "" {
		return fmt.Errorf("could not determine current database path")
	}

	// Close current database connection
	err := m.store.Close()
	if err != nil {
		return fmt.Errorf("failed to close current database: %w", err)
	}

	// Copy snapshot file over current database
	err = m.copyFile(snapshotPath, currentDBPath)
	if err != nil {
		// Try to reinitialize store even if copy failed
		m.store.Init()
		return fmt.Errorf("failed to copy snapshot file: %w", err)
	}

	// Reinitialize database connection with restored data
	err = m.store.Init()
	if err != nil {
		return fmt.Errorf("failed to reinitialize database after restore: %w", err)
	}

	return nil
}
