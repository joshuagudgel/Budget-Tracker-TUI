package database

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

// Connection represents a database connection with management utilities
type Connection struct {
	DB   *sql.DB
	path string
}

// NewConnection creates and initializes a new SQLite database connection
// The database file will be created in ~/.finance-wrapped/finance.db
func NewConnection() (*Connection, error) {
	// Get user home directory for cross-platform compatibility
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Create data directory
	dataDir := filepath.Join(homeDir, ".finance-wrapped")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Database file path
	dbPath := filepath.Join(dataDir, "finance.db")

	// Check if database exists to determine if we need to initialize schema
	_, err = os.Stat(dbPath)
	isNewDatabase := os.IsNotExist(err)

	// Open SQLite connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure SQLite connection
	if err := configureConnection(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure database: %w", err)
	}

	conn := &Connection{
		DB:   db,
		path: dbPath,
	}

	// Initialize schema for new databases
	if isNewDatabase {
		if err := conn.InitializeSchema(); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to initialize database schema: %w", err)
		}
	}

	return conn, nil
}

// configureConnection sets up SQLite connection parameters
func configureConnection(db *sql.DB) error {
	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Set journal mode for better performance and ACID compliance
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return fmt.Errorf("failed to set journal mode: %w", err)
	}

	// Set busy timeout for concurrent access
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		return fmt.Errorf("failed to set busy timeout: %w", err)
	}

	// Optimize SQLite settings for performance
	if _, err := db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		return fmt.Errorf("failed to set synchronous mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA cache_size = 10000"); err != nil {
		return fmt.Errorf("failed to set cache size: %w", err)
	}

	return nil
}

// InitializeSchema creates all tables, indexes, and triggers from schema.sql
func (c *Connection) InitializeSchema() error {
	// Read embedded schema file
	schemaSQL, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	// Execute schema creation within a transaction
	tx, err := c.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute the complete schema
	if _, err := tx.Exec(string(schemaSQL)); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	// Commit schema creation
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit schema: %w", err)
	}

	return nil
}

// GetSchemaVersion returns the current schema version
func (c *Connection) GetSchemaVersion() (int, error) {
	var version int
	err := c.DB.QueryRow("SELECT version FROM schema_version ORDER BY applied_at DESC LIMIT 1").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get schema version: %w", err)
	}
	return version, nil
}

// Close closes the database connection
func (c *Connection) Close() error {
	if c.DB == nil {
		return nil
	}
	return c.DB.Close()
}

// GetPath returns the database file path
func (c *Connection) GetPath() string {
	return c.path
}

// BeginTransaction starts a new database transaction
func (c *Connection) BeginTransaction() (*sql.Tx, error) {
	return c.DB.Begin()
}

// ExecuteInTransaction executes a function within a database transaction
// If the function returns an error, the transaction is rolled back
func (c *Connection) ExecuteInTransaction(fn func(*sql.Tx) error) error {
	tx, err := c.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// CheckHealth verifies database connection and basic functionality
func (c *Connection) CheckHealth() error {
	// Ping database
	if err := c.DB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Verify foreign keys are enabled
	var fkEnabled int
	err := c.DB.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled)
	if err != nil {
		return fmt.Errorf("failed to check foreign key status: %w", err)
	}
	if fkEnabled != 1 {
		return fmt.Errorf("foreign keys are not enabled")
	}

	// Verify schema version exists
	_, err = c.GetSchemaVersion()
	if err != nil {
		return fmt.Errorf("schema version check failed: %w", err)
	}

	return nil
}

// GetStats returns basic database statistics
func (c *Connection) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get file size
	if fileInfo, err := os.Stat(c.path); err == nil {
		stats["file_size_bytes"] = fileInfo.Size()
	}

	// Count tables and records
	tables := []struct {
		name  string
		table string
	}{
		{"transactions", "transactions"},
		{"categories", "categories"},
		{"bank_statements", "bank_statements"},
		{"csv_templates", "csv_templates"},
	}

	for _, t := range tables {
		var count int
		err := c.DB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", t.table)).Scan(&count)
		if err != nil {
			return nil, fmt.Errorf("failed to count %s: %w", t.table, err)
		}
		stats[t.name+"_count"] = count
	}

	// Get schema version
	if version, err := c.GetSchemaVersion(); err == nil {
		stats["schema_version"] = version
	}

	return stats, nil
}
