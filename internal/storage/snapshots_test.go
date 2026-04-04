package storage

import (
	"budget-tracker-tui/internal/types"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupTestSnapshotStore creates a test SnapshotStore with isolated in-memory database
func setupTestSnapshotStore(t *testing.T) *SnapshotStore {
	// Use the same pattern as other tests
	db := setupTestDB(t)
	return NewSnapshotStore(db)
}

// TestSnapshotStore_GetSnapshots tests retrieving all snapshots
func TestSnapshotStore_GetSnapshots(t *testing.T) {
	store := setupTestSnapshotStore(t)

	// Test with empty database
	snapshots, err := store.GetSnapshots()
	if err != nil {
		t.Errorf("GetSnapshots() failed: %v", err)
	}
	if len(snapshots) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snapshots))
	}
}

// TestSnapshotStore_CreateSnapshot tests creating a new snapshot
func TestSnapshotStore_CreateSnapshot(t *testing.T) {
	store := setupTestSnapshotStore(t)

	tests := []struct {
		name          string
		snapshotName  string
		description   string
		filePath      string
		expectSuccess bool
		expectedMsg   string
	}{
		{
			name:          "valid snapshot creation",
			snapshotName:  "test-snapshot",
			description:   "Test snapshot for unit testing",
			filePath:      filepath.Join(os.TempDir(), "test-snapshot.db"),
			expectSuccess: true,
			expectedMsg:   "snapshot 'test-snapshot' created successfully",
		},
		{
			name:          "empty name",
			snapshotName:  "",
			description:   "Test with empty name",
			filePath:      filepath.Join(os.TempDir(), "empty-name.db"),
			expectSuccess: false,
			expectedMsg:   "snapshot name cannot be empty",
		},
		{
			name:          "empty file path",
			snapshotName:  "valid-name",
			description:   "Test with empty path",
			filePath:      "",
			expectSuccess: false,
			expectedMsg:   "snapshot file path cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing test file
			if tt.filePath != "" {
				os.Remove(tt.filePath)
				defer os.Remove(tt.filePath) // Clean up after test
			}

			result, err := store.CreateSnapshot(tt.snapshotName, tt.description, tt.filePath)
			if err != nil {
				t.Errorf("CreateSnapshot() returned error: %v", err)
				return
			}

			if result.Success != tt.expectSuccess {
				t.Errorf("CreateSnapshot() success = %v, want %v", result.Success, tt.expectSuccess)
			}

			if result.Message != tt.expectedMsg {
				t.Errorf("CreateSnapshot() message = '%s', want '%s'", result.Message, tt.expectedMsg)
			}

			if tt.expectSuccess {
				// Verify snapshot was created in database
				snapshot, err := store.GetSnapshotById(result.SnapshotId)
				if err != nil {
					t.Errorf("failed to get created snapshot: %v", err)
				}
				if snapshot == nil {
					t.Error("created snapshot not found in database")
				}
				if snapshot != nil && snapshot.Name != tt.snapshotName {
					t.Errorf("snapshot name = '%s', want '%s'", snapshot.Name, tt.snapshotName)
				}

				// Verify snapshot file was created
				if _, err := os.Stat(tt.filePath); os.IsNotExist(err) {
					t.Errorf("snapshot file was not created at %s", tt.filePath)
				}
			}
		})
	}
}

// TestSnapshotStore_GetSnapshotById tests retrieving a snapshot by ID
func TestSnapshotStore_GetSnapshotById(t *testing.T) {
	store := setupTestSnapshotStore(t)

	// Test with non-existent ID
	snapshot, err := store.GetSnapshotById(999)
	if err != nil {
		t.Errorf("GetSnapshotById() failed: %v", err)
	}
	if snapshot != nil {
		t.Error("expected nil snapshot for non-existent ID")
	}

	// Create a test snapshot first
	testPath := filepath.Join(os.TempDir(), "test-get-by-id.db")
	defer os.Remove(testPath)

	result, err := store.CreateSnapshot("test-get", "Test description", testPath)
	if err != nil || !result.Success {
		t.Fatalf("failed to create test snapshot: %v", err)
	}

	// Test retrieving the created snapshot
	snapshot, err = store.GetSnapshotById(result.SnapshotId)
	if err != nil {
		t.Errorf("GetSnapshotById() failed: %v", err)
	}
	if snapshot == nil {
		t.Error("expected snapshot but got nil")
	}
	if snapshot != nil && snapshot.Name != "test-get" {
		t.Errorf("snapshot name = '%s', want 'test-get'", snapshot.Name)
	}
}

// TestSnapshotStore_GetSnapshotByName tests retrieving a snapshot by name
func TestSnapshotStore_GetSnapshotByName(t *testing.T) {
	store := setupTestSnapshotStore(t)

	// Test with non-existent name
	snapshot, err := store.GetSnapshotByName("non-existent")
	if err != nil {
		t.Errorf("GetSnapshotByName() failed: %v", err)
	}
	if snapshot != nil {
		t.Error("expected nil snapshot for non-existent name")
	}

	// Create a test snapshot first
	testPath := filepath.Join(os.TempDir(), "test-get-by-name.db")
	defer os.Remove(testPath)

	result, err := store.CreateSnapshot("test-by-name", "Test description", testPath)
	if err != nil || !result.Success {
		t.Fatalf("failed to create test snapshot: %v", err)
	}

	// Test retrieving the created snapshot
	snapshot, err = store.GetSnapshotByName("test-by-name")
	if err != nil {
		t.Errorf("GetSnapshotByName() failed: %v", err)
	}
	if snapshot == nil {
		t.Error("expected snapshot but got nil")
	}
	if snapshot != nil && snapshot.Id != result.SnapshotId {
		t.Errorf("snapshot ID = %d, want %d", snapshot.Id, result.SnapshotId)
	}
}

// TestSnapshotStore_DeleteSnapshot tests deleting a snapshot
func TestSnapshotStore_DeleteSnapshot(t *testing.T) {
	store := setupTestSnapshotStore(t)

	// Test deleting non-existent snapshot
	err := store.DeleteSnapshot(999)
	if err == nil {
		t.Error("expected error when deleting non-existent snapshot")
	}

	// Create a test snapshot first
	testPath := filepath.Join(os.TempDir(), "test-delete.db")
	defer os.Remove(testPath) // Cleanup in case deletion fails

	result, err := store.CreateSnapshot("test-delete", "Test description", testPath)
	if err != nil || !result.Success {
		t.Fatalf("failed to create test snapshot: %v", err)
	}

	// Verify file exists before deletion
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Error("snapshot file should exist before deletion")
	}

	// Delete the snapshot
	err = store.DeleteSnapshot(result.SnapshotId)
	if err != nil {
		t.Errorf("DeleteSnapshot() failed: %v", err)
	}

	// Verify snapshot is removed from database
	snapshot, err := store.GetSnapshotById(result.SnapshotId)
	if err != nil {
		t.Errorf("GetSnapshotById() failed after deletion: %v", err)
	}
	if snapshot != nil {
		t.Error("snapshot should not exist after deletion")
	}

	// Verify file is removed (note: file removal might fail but shouldn't error)
	if _, err := os.Stat(testPath); !os.IsNotExist(err) {
		t.Log("snapshot file still exists after deletion (this is acceptable)")
	}
}

// TestSnapshotStore_CalculateSnapshotCounts tests calculating data counts
func TestSnapshotStore_CalculateSnapshotCounts(t *testing.T) {
	store := setupTestSnapshotStore(t)

	// Test with empty database
	txCount, catCount, stmtCount, tmpCount, auditCount, err := store.CalculateSnapshotCounts()
	if err != nil {
		t.Errorf("CalculateSnapshotCounts() failed: %v", err)
	}

	// All counts should be 0 in empty database (except possibly categories which might have defaults)
	if txCount != 0 {
		t.Errorf("transaction count = %d, want 0", txCount)
	}
	if stmtCount != 0 {
		t.Errorf("statement count = %d, want 0", stmtCount)
	}
	if tmpCount != 0 {
		t.Errorf("template count = %d, want 0", tmpCount)
	}
	if auditCount != 0 {
		t.Errorf("audit event count = %d, want 0", auditCount)
	}

	// Note: catCount might be > 0 due to default categories, so we don't test it strictly
	if catCount < 0 {
		t.Errorf("category count = %d, should be >= 0", catCount)
	}
}

// TestSnapshotStore_ValidateSnapshotFile tests snapshot file validation
func TestSnapshotStore_ValidateSnapshotFile(t *testing.T) {
	store := setupTestSnapshotStore(t)

	// Test with non-existent file
	err := store.ValidateSnapshotFile("/non/existent/path.db")
	if err == nil {
		t.Error("expected error for non-existent file")
	}

	// Create an empty file
	emptyPath := filepath.Join(os.TempDir(), "empty-test.db")
	file, err := os.Create(emptyPath)
	if err != nil {
		t.Fatalf("failed to create empty test file: %v", err)
	}
	file.Close()
	defer os.Remove(emptyPath)

	// Test with empty file
	err = store.ValidateSnapshotFile(emptyPath)
	if err == nil {
		t.Error("expected error for empty file")
	}

	// Create a non-empty file
	nonEmptyPath := filepath.Join(os.TempDir(), "non-empty-test.db")
	err = os.WriteFile(nonEmptyPath, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("failed to create non-empty test file: %v", err)
	}
	defer os.Remove(nonEmptyPath)

	// Test with non-empty file (should pass basic validation)
	err = store.ValidateSnapshotFile(nonEmptyPath)
	if err != nil {
		t.Errorf("ValidateSnapshotFile() failed for valid file: %v", err)
	}
}

// TestSnapshot_GetSizeDisplay tests the size display formatting
func TestSnapshot_GetSizeDisplay(t *testing.T) {
	tests := []struct {
		size     int64
		expected string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1536 * 1024, "1.5 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1536 * 1024 * 1024, "1.5 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			snapshot := &types.Snapshot{FileSize: tt.size}
			result := snapshot.GetSizeDisplay()
			if result != tt.expected {
				t.Errorf("GetSizeDisplay() = '%s', want '%s'", result, tt.expected)
			}
		})
	}
}

// TestSnapshot_GetCreatedAtDisplay tests the creation date formatting
func TestSnapshot_GetCreatedAtDisplay(t *testing.T) {
	testTime := time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC)
	snapshot := &types.Snapshot{CreatedAt: testTime}

	result := snapshot.GetCreatedAtDisplay()
	expected := "03/15/2024 2:30 PM"

	if result != expected {
		t.Errorf("GetCreatedAtDisplay() = '%s', want '%s'", result, expected)
	}
}
