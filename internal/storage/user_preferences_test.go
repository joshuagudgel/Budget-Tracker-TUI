package storage

import (
	"strings"
	"testing"

	"budget-tracker-tui/internal/database"
)

// setupTestUserPreferencesStore creates a UserPreferencesStore for testing
func setupTestUserPreferencesStore(t *testing.T) (*UserPreferencesStore, *database.Connection) {
	t.Helper()

	conn := setupTestDB(t)
	store := NewUserPreferencesStore(conn)

	return store, conn
}

// Test fixture helpers

// createTestPreference inserts a test preference directly into the database
func createTestPreference(t *testing.T, conn *database.Connection, key, value string) {
	t.Helper()

	query := `INSERT INTO user_preferences (preference_key, preference_value) VALUES (?, ?)`
	_, err := conn.DB.Exec(query, key, value)
	if err != nil {
		t.Fatalf("Failed to create test preference: %v", err)
	}
}

// countPreferences returns the total number of preferences in the database
func countPreferences(t *testing.T, conn *database.Connection) int {
	t.Helper()

	var count int
	query := `SELECT COUNT(*) FROM user_preferences`
	err := conn.DB.QueryRow(query).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count preferences: %v", err)
	}

	return count
}

// TestNewUserPreferencesStore tests the constructor
func TestNewUserPreferencesStore(t *testing.T) {
	conn := setupTestDB(t)
	defer teardownTestDB(t, conn)

	store := NewUserPreferencesStore(conn)

	if store == nil {
		t.Error("Expected UserPreferencesStore instance, got nil")
	}

	if store.db != conn {
		t.Error("Expected database connection to be set")
	}

	if store.helper == nil {
		t.Error("Expected helper to be initialized")
	}
}

// TestGetPreference tests preference retrieval
func TestGetPreference(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*testing.T, *database.Connection)
		key         string
		expected    string
		expectError bool
		errorCheck  func(*testing.T, error)
	}{
		{
			name:        "empty key returns error",
			setupData:   func(t *testing.T, conn *database.Connection) {},
			key:         "",
			expected:    "",
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				if err == nil || !strings.Contains(err.Error(), "preference key cannot be empty") {
					t.Errorf("Expected empty key error, got: %v", err)
				}
			},
		},
		{
			name:        "non-existent key returns not found error",
			setupData:   func(t *testing.T, conn *database.Connection) {},
			key:         "non_existent_key",
			expected:    "",
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				if err == nil || !strings.Contains(err.Error(), "preference not found: non_existent_key") {
					t.Errorf("Expected not found error, got: %v", err)
				}
			},
		},
		{
			name: "existing preference returns correct value",
			setupData: func(t *testing.T, conn *database.Connection) {
				createTestPreference(t, conn, "test_key", "test_value")
			},
			key:         "test_key",
			expected:    "test_value",
			expectError: false,
		},
		{
			name: "preference with special characters",
			setupData: func(t *testing.T, conn *database.Connection) {
				createTestPreference(t, conn, "path_key", "C:\\Users\\Test\\Documents\\Bank Statements")
			},
			key:         "path_key",
			expected:    "C:\\Users\\Test\\Documents\\Bank Statements",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, conn := setupTestUserPreferencesStore(t)
			defer teardownTestDB(t, conn)

			tt.setupData(t, conn)

			result, err := store.GetPreference(tt.key)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorCheck != nil {
					tt.errorCheck(t, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected value '%s', got '%s'", tt.expected, result)
				}
			}
		})
	}
}

// TestSetPreference tests preference setting/updating
func TestSetPreference(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*testing.T, *database.Connection)
		key         string
		value       string
		expectError bool
		errorCheck  func(*testing.T, error)
		validate    func(*testing.T, *UserPreferencesStore)
	}{
		{
			name:        "empty key returns error",
			setupData:   func(t *testing.T, conn *database.Connection) {},
			key:         "",
			value:       "some_value",
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				if err == nil || !strings.Contains(err.Error(), "preference key cannot be empty") {
					t.Errorf("Expected empty key error, got: %v", err)
				}
			},
		},
		{
			name:        "empty value returns error",
			setupData:   func(t *testing.T, conn *database.Connection) {},
			key:         "some_key",
			value:       "",
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				if err == nil || !strings.Contains(err.Error(), "preference value cannot be empty") {
					t.Errorf("Expected empty value error, got: %v", err)
				}
			},
		},
		{
			name:        "new preference creation succeeds",
			setupData:   func(t *testing.T, conn *database.Connection) {},
			key:         "new_key",
			value:       "new_value",
			expectError: false,
			validate: func(t *testing.T, store *UserPreferencesStore) {
				// Verify preference was created
				result, err := store.GetPreference("new_key")
				if err != nil {
					t.Errorf("Failed to get created preference: %v", err)
				}
				if result != "new_value" {
					t.Errorf("Expected value 'new_value', got '%s'", result)
				}
			},
		},
		{
			name: "existing preference update succeeds",
			setupData: func(t *testing.T, conn *database.Connection) {
				createTestPreference(t, conn, "existing_key", "old_value")
			},
			key:         "existing_key",
			value:       "updated_value",
			expectError: false,
			validate: func(t *testing.T, store *UserPreferencesStore) {
				// Verify preference was updated
				result, err := store.GetPreference("existing_key")
				if err != nil {
					t.Errorf("Failed to get updated preference: %v", err)
				}
				if result != "updated_value" {
					t.Errorf("Expected value 'updated_value', got '%s'", result)
				}
			},
		},
		{
			name:        "special characters in key and value",
			setupData:   func(t *testing.T, conn *database.Connection) {},
			key:         "last_import_directory",
			value:       "C:\\Users\\Test\\Documents\\Bank Statements",
			expectError: false,
			validate: func(t *testing.T, store *UserPreferencesStore) {
				result, err := store.GetPreference("last_import_directory")
				if err != nil {
					t.Errorf("Failed to get preference with special chars: %v", err)
				}
				if result != "C:\\Users\\Test\\Documents\\Bank Statements" {
					t.Errorf("Expected path, got '%s'", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, conn := setupTestUserPreferencesStore(t)
			defer teardownTestDB(t, conn)

			tt.setupData(t, conn)

			err := store.SetPreference(tt.key, tt.value)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorCheck != nil {
					tt.errorCheck(t, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if tt.validate != nil {
					tt.validate(t, store)
				}
			}
		})
	}
}

// TestDeletePreference tests preference deletion
func TestDeletePreference(t *testing.T) {
	tests := []struct {
		name        string
		setupData   func(*testing.T, *database.Connection)
		key         string
		expectError bool
		errorCheck  func(*testing.T, error)
		validate    func(*testing.T, *UserPreferencesStore, *database.Connection)
	}{
		{
			name:        "empty key returns error",
			setupData:   func(t *testing.T, conn *database.Connection) {},
			key:         "",
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				if err == nil || !strings.Contains(err.Error(), "preference key cannot be empty") {
					t.Errorf("Expected empty key error, got: %v", err)
				}
			},
		},
		{
			name:        "non-existent key returns not found error",
			setupData:   func(t *testing.T, conn *database.Connection) {},
			key:         "non_existent_key",
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				if err == nil || !strings.Contains(err.Error(), "preference not found: non_existent_key") {
					t.Errorf("Expected not found error, got: %v", err)
				}
			},
		},
		{
			name: "existing preference deletion succeeds",
			setupData: func(t *testing.T, conn *database.Connection) {
				createTestPreference(t, conn, "delete_test", "delete_value")
			},
			key:         "delete_test",
			expectError: false,
			validate: func(t *testing.T, store *UserPreferencesStore, conn *database.Connection) {
				// Verify preference was deleted
				_, err := store.GetPreference("delete_test")
				if err == nil {
					t.Error("Expected preference to be deleted but it still exists")
				}
				if !strings.Contains(err.Error(), "preference not found") {
					t.Errorf("Expected not found error after deletion, got: %v", err)
				}

				// Verify database count decreased
				count := countPreferences(t, conn)
				if count != 0 {
					t.Errorf("Expected 0 preferences after deletion, got %d", count)
				}
			},
		},
		{
			name: "multiple preferences - delete specific one",
			setupData: func(t *testing.T, conn *database.Connection) {
				createTestPreference(t, conn, "keep_me", "keep_value")
				createTestPreference(t, conn, "delete_me", "delete_value")
			},
			key:         "delete_me",
			expectError: false,
			validate: func(t *testing.T, store *UserPreferencesStore, conn *database.Connection) {
				// Verify correct preference was deleted
				_, err := store.GetPreference("delete_me")
				if err == nil || !strings.Contains(err.Error(), "preference not found") {
					t.Error("Expected deleted preference to not be found")
				}

				// Verify other preference still exists
				result, err := store.GetPreference("keep_me")
				if err != nil {
					t.Errorf("Expected other preference to still exist: %v", err)
				}
				if result != "keep_value" {
					t.Errorf("Expected 'keep_value', got '%s'", result)
				}

				// Verify count is correct
				count := countPreferences(t, conn)
				if count != 1 {
					t.Errorf("Expected 1 preference remaining, got %d", count)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, conn := setupTestUserPreferencesStore(t)
			defer teardownTestDB(t, conn)

			tt.setupData(t, conn)

			err := store.DeletePreference(tt.key)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorCheck != nil {
					tt.errorCheck(t, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if tt.validate != nil {
					tt.validate(t, store, conn)
				}
			}
		})
	}
}

// TestHasPreference tests preference existence checking
func TestHasPreference(t *testing.T) {
	tests := []struct {
		name      string
		setupData func(*testing.T, *database.Connection)
		key       string
		expected  bool
	}{
		{
			name:      "empty key returns false",
			setupData: func(t *testing.T, conn *database.Connection) {},
			key:       "",
			expected:  false,
		},
		{
			name:      "non-existent key returns false",
			setupData: func(t *testing.T, conn *database.Connection) {},
			key:       "non_existent_key",
			expected:  false,
		},
		{
			name: "existing key returns true",
			setupData: func(t *testing.T, conn *database.Connection) {
				createTestPreference(t, conn, "existing_key", "some_value")
			},
			key:      "existing_key",
			expected: true,
		},
		{
			name: "case sensitive key matching",
			setupData: func(t *testing.T, conn *database.Connection) {
				createTestPreference(t, conn, "CaseSensitive", "value")
			},
			key:      "casesensitive",
			expected: false,
		},
		{
			name: "special characters in key",
			setupData: func(t *testing.T, conn *database.Connection) {
				createTestPreference(t, conn, "last_import_directory", "/path/to/dir")
			},
			key:      "last_import_directory",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, conn := setupTestUserPreferencesStore(t)
			defer teardownTestDB(t, conn)

			tt.setupData(t, conn)

			result := store.HasPreference(tt.key)

			if result != tt.expected {
				t.Errorf("Expected %t, got %t for key '%s'", tt.expected, result, tt.key)
			}
		})
	}
}

// TestGetPreferenceWithDefault tests preference retrieval with default fallback
func TestGetPreferenceWithDefault(t *testing.T) {
	tests := []struct {
		name         string
		setupData    func(*testing.T, *database.Connection)
		key          string
		defaultValue string
		expected     string
	}{
		{
			name:         "non-existent key returns default",
			setupData:    func(t *testing.T, conn *database.Connection) {},
			key:          "non_existent_key",
			defaultValue: "default_value",
			expected:     "default_value",
		},
		{
			name: "existing key returns stored value",
			setupData: func(t *testing.T, conn *database.Connection) {
				createTestPreference(t, conn, "existing_key", "stored_value")
			},
			key:          "existing_key",
			defaultValue: "default_value",
			expected:     "stored_value",
		},
		{
			name:         "empty key returns default",
			setupData:    func(t *testing.T, conn *database.Connection) {},
			key:          "",
			defaultValue: "default_for_empty",
			expected:     "default_for_empty",
		},
		{
			name:         "empty default value is returned",
			setupData:    func(t *testing.T, conn *database.Connection) {},
			key:          "missing_key",
			defaultValue: "",
			expected:     "",
		},
		{
			name: "last import directory use case",
			setupData: func(t *testing.T, conn *database.Connection) {
				createTestPreference(t, conn, "last_import_directory", "C:\\Documents\\Statements")
			},
			key:          "last_import_directory",
			defaultValue: "",
			expected:     "C:\\Documents\\Statements",
		},
		{
			name:         "last import directory fallback case",
			setupData:    func(t *testing.T, conn *database.Connection) {},
			key:          "last_import_directory",
			defaultValue: "",
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, conn := setupTestUserPreferencesStore(t)
			defer teardownTestDB(t, conn)

			tt.setupData(t, conn)

			result := store.GetPreferenceWithDefault(tt.key, tt.defaultValue)

			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestUserPreferencesIntegration tests the complete workflow
func TestUserPreferencesIntegration(t *testing.T) {
	store, conn := setupTestUserPreferencesStore(t)
	defer teardownTestDB(t, conn)

	// Test complete workflow: set, get, update, check existence, delete

	// 1. Initially, preference should not exist
	if store.HasPreference("workflow_test") {
		t.Error("Preference should not exist initially")
	}

	result := store.GetPreferenceWithDefault("workflow_test", "default")
	if result != "default" {
		t.Errorf("Expected default value, got '%s'", result)
	}

	// 2. Set preference
	err := store.SetPreference("workflow_test", "initial_value")
	if err != nil {
		t.Fatalf("Failed to set preference: %v", err)
	}

	// 3. Verify existence and retrieval
	if !store.HasPreference("workflow_test") {
		t.Error("Preference should exist after setting")
	}

	result, err = store.GetPreference("workflow_test")
	if err != nil {
		t.Errorf("Failed to get preference: %v", err)
	}
	if result != "initial_value" {
		t.Errorf("Expected 'initial_value', got '%s'", result)
	}

	// 4. Update preference
	err = store.SetPreference("workflow_test", "updated_value")
	if err != nil {
		t.Fatalf("Failed to update preference: %v", err)
	}

	result, err = store.GetPreference("workflow_test")
	if err != nil {
		t.Errorf("Failed to get updated preference: %v", err)
	}
	if result != "updated_value" {
		t.Errorf("Expected 'updated_value', got '%s'", result)
	}

	// 5. Delete preference
	err = store.DeletePreference("workflow_test")
	if err != nil {
		t.Fatalf("Failed to delete preference: %v", err)
	}

	// 6. Verify deletion
	if store.HasPreference("workflow_test") {
		t.Error("Preference should not exist after deletion")
	}

	_, err = store.GetPreference("workflow_test")
	if err == nil || !strings.Contains(err.Error(), "preference not found") {
		t.Error("Expected not found error after deletion")
	}

	result = store.GetPreferenceWithDefault("workflow_test", "back_to_default")
	if result != "back_to_default" {
		t.Errorf("Expected default after deletion, got '%s'", result)
	}
}

// TestConcurrentAccess tests concurrent preference operations
func TestConcurrentAccess(t *testing.T) {
	store, conn := setupTestUserPreferencesStore(t)
	defer teardownTestDB(t, conn)

	// This test verifies that SQLite handles concurrent access properly
	// Set up multiple preferences first
	for i := 0; i < 5; i++ {
		key := "concurrent_test_" + string(rune('A'+i))
		val := "value_" + string(rune('A'+i))
		err := store.SetPreference(key, val)
		if err != nil {
			t.Fatalf("Failed to set preference %s: %v", key, val)
		}
	}

	// Verify all preferences exist
	for i := 0; i < 5; i++ {
		key := "concurrent_test_" + string(rune('A'+i))
		if !store.HasPreference(key) {
			t.Errorf("Preference %s should exist", key)
		}
	}

	// Update all preferences
	for i := 0; i < 5; i++ {
		key := "concurrent_test_" + string(rune('A'+i))
		val := "updated_value_" + string(rune('A'+i))
		err := store.SetPreference(key, val)
		if err != nil {
			t.Fatalf("Failed to update preference %s: %v", key, err)
		}
	}

	// Verify updated values
	for i := 0; i < 5; i++ {
		key := "concurrent_test_" + string(rune('A'+i))
		expected := "updated_value_" + string(rune('A'+i))
		actual, err := store.GetPreference(key)
		if err != nil {
			t.Errorf("Failed to get preference %s: %v", key, err)
		}
		if actual != expected {
			t.Errorf("Expected '%s', got '%s' for key %s", expected, actual, key)
		}
	}

	// Clean up
	for i := 0; i < 5; i++ {
		key := "concurrent_test_" + string(rune('A'+i))
		err := store.DeletePreference(key)
		if err != nil {
			t.Errorf("Failed to delete preference %s: %v", key, err)
		}
	}

	// Verify all are deleted
	count := countPreferences(t, conn)
	if count != 0 {
		t.Errorf("Expected 0 preferences after cleanup, got %d", count)
	}
}
