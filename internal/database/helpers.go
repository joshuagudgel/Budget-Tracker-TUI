package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SQLHelper provides common database operations and utilities
type SQLHelper struct {
	conn *Connection
}

// NewSQLHelper creates a new SQL helper with the given connection
func NewSQLHelper(conn *Connection) *SQLHelper {
	return &SQLHelper{conn: conn}
}

// ScanRowToMap scans a database row into a map[string]interface{}
// Useful for flexible data retrieval and JSON serialization
func (h *SQLHelper) ScanRowToMap(rows *sql.Rows) (map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Create slice of interface{} pointers for scanning
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	// Scan the row
	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	// Convert to map
	result := make(map[string]interface{})
	for i, col := range columns {
		result[col] = h.convertSQLValue(values[i])
	}

	return result, nil
}

// convertSQLValue converts SQL values to appropriate Go types
func (h *SQLHelper) convertSQLValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case []byte:
		return string(val)
	case time.Time:
		return val.Format(time.RFC3339)
	case int64:
		return val
	case float64:
		return val
	case bool:
		return val
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}

// BuildInsertSQL generates an INSERT SQL statement with placeholders
func (h *SQLHelper) BuildInsertSQL(table string, fields []string) string {
	placeholders := make([]string, len(fields))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(fields, ", "),
		strings.Join(placeholders, ", "),
	)
}

// BuildUpdateSQL generates an UPDATE SQL statement with placeholders
// ID field is automatically added to WHERE clause
func (h *SQLHelper) BuildUpdateSQL(table string, fields []string, idField string) string {
	setParts := make([]string, len(fields))
	for i, field := range fields {
		setParts[i] = field + " = ?"
	}

	return fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = ?",
		table,
		strings.Join(setParts, ", "),
		idField,
	)
}

// BuildSelectSQL generates a SELECT SQL statement with optional WHERE conditions
func (h *SQLHelper) BuildSelectSQL(table string, fields []string, whereClause string) string {
	selectFields := "*"
	if len(fields) > 0 {
		selectFields = strings.Join(fields, ", ")
	}

	query := fmt.Sprintf("SELECT %s FROM %s", selectFields, table)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}

	return query
}

// ExecReturnID executes an INSERT statement and returns the last inserted ID
func (h *SQLHelper) ExecReturnID(query string, args ...interface{}) (int64, error) {
	result, err := h.conn.DB.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// ExecReturnRowsAffected executes a statement and returns the number of affected rows
func (h *SQLHelper) ExecReturnRowsAffected(query string, args ...interface{}) (int64, error) {
	result, err := h.conn.DB.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// QuerySingleRow executes a query expected to return exactly one row
func (h *SQLHelper) QuerySingleRow(query string, args ...interface{}) *sql.Row {
	return h.conn.DB.QueryRow(query, args...)
}

// QueryRows executes a query that returns multiple rows
func (h *SQLHelper) QueryRows(query string, args ...interface{}) (*sql.Rows, error) {
	return h.conn.DB.Query(query, args...)
}

// ExistsBy checks if a record exists with the given WHERE condition
func (h *SQLHelper) ExistsBy(table, whereClause string, args ...interface{}) (bool, error) {
	query := fmt.Sprintf("SELECT 1 FROM %s WHERE %s LIMIT 1", table, whereClause)
	var exists int
	err := h.conn.DB.QueryRow(query, args...).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CountBy returns the count of records matching the WHERE condition
func (h *SQLHelper) CountBy(table, whereClause string, args ...interface{}) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}

	var count int64
	err := h.conn.DB.QueryRow(query, args...).Scan(&count)
	return count, err
}

// GetMaxID returns the maximum ID value from the specified table and column
func (h *SQLHelper) GetMaxID(table, idColumn string) (int64, error) {
	query := fmt.Sprintf("SELECT COALESCE(MAX(%s), 0) FROM %s", idColumn, table)
	var maxID int64
	err := h.conn.DB.QueryRow(query).Scan(&maxID)
	return maxID, err
}

// DeleteBy deletes records matching the WHERE condition
func (h *SQLHelper) DeleteBy(table, whereClause string, args ...interface{}) (int64, error) {
	query := fmt.Sprintf("DELETE FROM %s WHERE %s", table, whereClause)
	return h.ExecReturnRowsAffected(query, args...)
}

// BulkInsert performs efficient bulk insertion using a transaction
func (h *SQLHelper) BulkInsert(table string, fields []string, records [][]interface{}) error {
	if len(records) == 0 {
		return nil
	}

	insertSQL := h.BuildInsertSQL(table, fields)

	return h.conn.ExecuteInTransaction(func(tx *sql.Tx) error {
		stmt, err := tx.Prepare(insertSQL)
		if err != nil {
			return fmt.Errorf("failed to prepare bulk insert: %w", err)
		}
		defer stmt.Close()

		for _, record := range records {
			if _, err := stmt.Exec(record...); err != nil {
				return fmt.Errorf("failed to execute bulk insert row: %w", err)
			}
		}

		return nil
	})
}

// QueryToMap executes a query and returns all results as a slice of maps
func (h *SQLHelper) QueryToMap(query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := h.QueryRows(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		rowMap, err := h.ScanRowToMap(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, rowMap)
	}

	return results, rows.Err()
}

// FormatTimeForDB formats a time.Time for database storage
func (h *SQLHelper) FormatTimeForDB(t time.Time) string {
	return t.Format(time.RFC3339)
}

// ParseTimeFromDB parses a database time string back to time.Time
func (h *SQLHelper) ParseTimeFromDB(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// FormatDateForDB formats a time.Time as date-only for database storage
func (h *SQLHelper) FormatDateForDB(t time.Time) string {
	return t.Format("2006-01-02")
}

// ParseDateFromDB parses a database date string back to time.Time
func (h *SQLHelper) ParseDateFromDB(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}
