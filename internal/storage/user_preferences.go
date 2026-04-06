package storage

import (
	"budget-tracker-tui/internal/database"
	"database/sql"
	"fmt"
)

// UserPreferencesStore handles all user preference operations using SQLite
type UserPreferencesStore struct {
	db     *database.Connection
	helper *database.SQLHelper
}

// NewUserPreferencesStore creates a new UserPreferencesStore instance
func NewUserPreferencesStore(db *database.Connection) *UserPreferencesStore {
	return &UserPreferencesStore{
		db:     db,
		helper: database.NewSQLHelper(db),
	}
}

// GetPreference retrieves a preference value by key
func (ups *UserPreferencesStore) GetPreference(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("preference key cannot be empty")
	}

	var value string
	query := `SELECT preference_value FROM user_preferences WHERE preference_key = ?`

	err := ups.db.DB.QueryRow(query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("preference not found: %s", key)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get preference %s: %v", key, err)
	}

	return value, nil
}

// SetPreference sets or updates a preference value
func (ups *UserPreferencesStore) SetPreference(key, value string) error {
	if key == "" {
		return fmt.Errorf("preference key cannot be empty")
	}
	if value == "" {
		return fmt.Errorf("preference value cannot be empty")
	}

	// Use UPSERT (INSERT OR REPLACE) to handle both insert and update
	query := `INSERT OR REPLACE INTO user_preferences (preference_key, preference_value, updated_at) 
			  VALUES (?, ?, CURRENT_TIMESTAMP)`

	_, err := ups.db.DB.Exec(query, key, value)
	if err != nil {
		return fmt.Errorf("failed to set preference %s: %v", key, err)
	}

	return nil
}

// DeletePreference removes a preference by key
func (ups *UserPreferencesStore) DeletePreference(key string) error {
	if key == "" {
		return fmt.Errorf("preference key cannot be empty")
	}

	query := `DELETE FROM user_preferences WHERE preference_key = ?`

	result, err := ups.db.DB.Exec(query, key)
	if err != nil {
		return fmt.Errorf("failed to delete preference %s: %v", key, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check deletion result for preference %s: %v", key, err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("preference not found: %s", key)
	}

	return nil
}

// HasPreference checks if a preference exists
func (ups *UserPreferencesStore) HasPreference(key string) bool {
	if key == "" {
		return false
	}

	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM user_preferences WHERE preference_key = ?)`

	err := ups.db.DB.QueryRow(query, key).Scan(&exists)
	if err != nil {
		return false
	}

	return exists
}

// GetPreferenceWithDefault retrieves a preference value or returns default if not found
func (ups *UserPreferencesStore) GetPreferenceWithDefault(key, defaultValue string) string {
	value, err := ups.GetPreference(key)
	if err != nil {
		return defaultValue
	}
	return value
}
