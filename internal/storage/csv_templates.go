package storage

import (
	"budget-tracker-tui/internal/database"
	"budget-tracker-tui/internal/types"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// CSVTemplateStore handles all CSV template-related operations using SQLite
type CSVTemplateStore struct {
	db              *database.Connection
	helper          *database.SQLHelper
	defaultTemplate string
}

// NewCSVTemplateStore creates a new CSVTemplateStore instance
func NewCSVTemplateStore(db *database.Connection) *CSVTemplateStore {
	store := &CSVTemplateStore{
		db:              db,
		helper:          database.NewSQLHelper(db),
		defaultTemplate: "Bank1",
	}

	// Ensure default templates exist
	store.ensureDefaultTemplates()

	return store
}

// ensureDefaultTemplates creates default CSV templates if none exist
func (cts *CSVTemplateStore) ensureDefaultTemplates() {
	count, err := cts.helper.CountBy("csv_templates", "")
	if err != nil || count > 0 {
		return // Templates already exist or error occurred
	}

	// Create default templates
	defaultTemplates := []types.CSVTemplate{
		{Name: "Bank1", PostDateColumn: 0, AmountColumn: 1, DescColumn: 4, HasHeader: false},
		{Name: "Bank2", PostDateColumn: 0, AmountColumn: 5, DescColumn: 2, HasHeader: true},
	}

	for _, template := range defaultTemplates {
		cts.SaveCSVTemplate(template) // Ignore errors during initialization
	}
}

// GetCSVTemplates returns all CSV templates from the database
func (cts *CSVTemplateStore) GetCSVTemplates() ([]types.CSVTemplate, error) {
	query := `
		SELECT id, name, post_date_column, amount_column, desc_column, category_column,
		       has_header, date_format, delimiter, created_at, updated_at
		FROM csv_templates
		ORDER BY name
	`

	rows, err := cts.helper.QueryRows(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query CSV templates: %w", err)
	}
	defer rows.Close()

	var templates []types.CSVTemplate
	for rows.Next() {
		template, err := cts.scanCSVTemplate(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan CSV template: %w", err)
		}
		templates = append(templates, template)
	}

	return templates, rows.Err()
}

// scanCSVTemplate scans a database row into a CSVTemplate struct
func (cts *CSVTemplateStore) scanCSVTemplate(rows *sql.Rows) (types.CSVTemplate, error) {
	var template types.CSVTemplate
	var categoryColumn sql.NullInt64
	var dateFormat, delimiter sql.NullString
	var createdAtStr, updatedAtStr string

	err := rows.Scan(
		&template.Id, &template.Name, &template.PostDateColumn, &template.AmountColumn,
		&template.DescColumn, &categoryColumn, &template.HasHeader,
		&dateFormat, &delimiter, &createdAtStr, &updatedAtStr,
	)

	if err != nil {
		return template, err
	}

	// Parse time fields from database
	template.CreatedAt, err = cts.helper.ParseTimeFromDB(createdAtStr)
	if err != nil {
		return template, fmt.Errorf("failed to parse created_at: %w", err)
	}
	template.UpdatedAt, err = cts.helper.ParseTimeFromDB(updatedAtStr)
	if err != nil {
		return template, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	// Handle nullable fields
	if categoryColumn.Valid {
		categoryInt := int(categoryColumn.Int64)
		template.CategoryColumn = &categoryInt
	}
	if dateFormat.Valid {
		template.DateFormat = dateFormat.String
	}
	if delimiter.Valid {
		template.Delimiter = delimiter.String
	}

	return template, nil
}

// GetDefaultTemplate returns the name of the default template
func (cts *CSVTemplateStore) GetDefaultTemplate() string {
	return cts.defaultTemplate
}

// SetDefaultTemplate sets the default template
func (cts *CSVTemplateStore) SetDefaultTemplate(templateName string) *TemplateResult {
	result := &TemplateResult{}

	// Verify template exists
	if cts.GetTemplateByName(templateName) == nil {
		result.Message = "Template not found"
		return result
	}

	cts.defaultTemplate = templateName
	result.Success = true
	result.Message = fmt.Sprintf("Default template set to '%s'", templateName)
	return result
}

// GetTemplateByName returns a CSV template by name
func (cts *CSVTemplateStore) GetTemplateByName(name string) *types.CSVTemplate {
	query := `
		SELECT id, name, post_date_column, amount_column, desc_column, category_column,
		       has_header, date_format, delimiter, created_at, updated_at
		FROM csv_templates
		WHERE name = ?
	`

	row := cts.helper.QuerySingleRow(query, name)
	template, err := cts.scanCSVTemplateRow(row)
	if err != nil {
		return nil // Template not found or error
	}

	return &template
}

// scanCSVTemplateRow scans a single database row into a CSVTemplate struct
func (cts *CSVTemplateStore) scanCSVTemplateRow(row *sql.Row) (types.CSVTemplate, error) {
	var template types.CSVTemplate
	var categoryColumn sql.NullInt64
	var dateFormat, delimiter sql.NullString
	var createdAtStr, updatedAtStr string

	err := row.Scan(
		&template.Id, &template.Name, &template.PostDateColumn, &template.AmountColumn,
		&template.DescColumn, &categoryColumn, &template.HasHeader, &dateFormat,
		&delimiter, &createdAtStr, &updatedAtStr,
	)

	if err != nil {
		return template, err
	}

	// Parse time fields from database
	template.CreatedAt, err = cts.helper.ParseTimeFromDB(createdAtStr)
	if err != nil {
		return template, fmt.Errorf("failed to parse created_at: %w", err)
	}
	template.UpdatedAt, err = cts.helper.ParseTimeFromDB(updatedAtStr)
	if err != nil {
		return template, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	// Handle nullable fields
	if categoryColumn.Valid {
		categoryInt := int(categoryColumn.Int64)
		template.CategoryColumn = &categoryInt
	}
	if dateFormat.Valid {
		template.DateFormat = dateFormat.String
	}
	if delimiter.Valid {
		template.Delimiter = delimiter.String
	}

	return template, nil
}

// GetTemplateById returns a CSV template by its ID
func (cts *CSVTemplateStore) GetTemplateById(id int64) *types.CSVTemplate {
	query := `
		SELECT id, name, post_date_column, amount_column, desc_column, category_column,
		       has_header, date_format, delimiter, created_at, updated_at
		FROM csv_templates
		WHERE id = ?
	`

	row := cts.helper.QuerySingleRow(query, id)
	template, err := cts.scanCSVTemplateRow(row)
	if err != nil {
		return nil // Template not found or error
	}

	return &template
}

// GetTemplateNameById returns the name of a template by its ID, or empty string if not found
func (cts *CSVTemplateStore) GetTemplateNameById(id int64) string {
	template := cts.GetTemplateById(id)
	if template != nil {
		return template.Name
	}
	return ""
}

// CreateCSVTemplate creates a new CSV template
func (cts *CSVTemplateStore) CreateCSVTemplate(template types.CSVTemplate) *TemplateResult {
	result := &TemplateResult{}

	// Validate template name
	if strings.TrimSpace(template.Name) == "" {
		result.Message = "Template name cannot be empty"
		return result
	}

	// Set default delimiter if empty (matches database schema default)
	if template.Delimiter == "" {
		template.Delimiter = ","
	}

	// Check for duplicates
	existing := cts.GetTemplateByName(template.Name)
	if existing != nil {
		result.Message = "Template name already exists"
		return result
	}

	// Validate column indices
	if template.PostDateColumn < 0 || template.AmountColumn < 0 || template.DescColumn < 0 {
		result.Message = "Column indices must be non-negative"
		return result
	}

	err := cts.SaveCSVTemplate(template)
	if err != nil {
		result.Message = fmt.Sprintf("Failed to save template: %v", err)
		return result
	}

	// Set as default
	cts.defaultTemplate = template.Name

	result.Success = true
	result.Message = fmt.Sprintf("Template '%s' created and set as default", template.Name)
	return result
}

// SaveCSVTemplate saves or updates a CSV template
func (cts *CSVTemplateStore) SaveCSVTemplate(template types.CSVTemplate) error {
	now := time.Now()

	if template.Id == 0 {
		// Insert new template
		return cts.insertTemplate(template, now)
	} else {
		// Update existing template
		return cts.updateTemplate(template, now)
	}
}

// insertTemplate inserts a new CSV template
func (cts *CSVTemplateStore) insertTemplate(template types.CSVTemplate, now time.Time) error {
	query := `
		INSERT INTO csv_templates (
			name, post_date_column, amount_column, desc_column, category_column,
			has_header, date_format, delimiter, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// Handle nullable fields
	var categoryColumn interface{}
	if template.CategoryColumn != nil {
		categoryColumn = *template.CategoryColumn
	}

	var dateFormat interface{}
	if template.DateFormat != "" {
		dateFormat = template.DateFormat
	}

	// Delimiter is NOT NULL in schema, so use default if empty
	delimiter := template.Delimiter
	if delimiter == "" {
		delimiter = "," // Use schema default
	}

	// Set creation timestamp if not provided
	createdAt := template.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}

	// Format times for database storage
	createdAtStr := createdAt.Format(time.RFC3339)
	updatedAtStr := now.Format(time.RFC3339)

	_, err := cts.helper.ExecReturnID(query,
		template.Name, template.PostDateColumn, template.AmountColumn,
		template.DescColumn, categoryColumn, template.HasHeader,
		dateFormat, delimiter, createdAtStr, updatedAtStr,
	)

	if err != nil {
		return fmt.Errorf("failed to insert CSV template: %w", err)
	}

	// Update the template ID
	return nil
}

// updateTemplate updates an existing CSV template
func (cts *CSVTemplateStore) updateTemplate(template types.CSVTemplate, now time.Time) error {
	query := `
		UPDATE csv_templates SET 
			name = ?, post_date_column = ?, amount_column = ?, desc_column = ?, category_column = ?, 
			has_header = ?, date_format = ?, delimiter = ?, 
			updated_at = ?
		WHERE id = ?
	`

	// Handle nullable fields
	var categoryColumn interface{}
	if template.CategoryColumn != nil {
		categoryColumn = *template.CategoryColumn
	}

	var dateFormat interface{}
	if template.DateFormat != "" {
		dateFormat = template.DateFormat
	}

	// Delimiter is NOT NULL in schema, so use default if empty
	delimiter := template.Delimiter
	if delimiter == "" {
		delimiter = "," // Use schema default
	}

	_, err := cts.helper.ExecReturnRowsAffected(query,
		template.Name, template.PostDateColumn, template.AmountColumn,
		template.DescColumn, categoryColumn, template.HasHeader,
		dateFormat, delimiter, now, template.Id,
	)

	if err != nil {
		return fmt.Errorf("failed to update CSV template: %w", err)
	}

	return nil
}

// DeleteCSVTemplate deletes a CSV template by ID
func (cts *CSVTemplateStore) DeleteCSVTemplate(id int64) *TemplateResult {
	// Check if template is being used by any bank statements
	query := `SELECT COUNT(*) FROM bank_statements WHERE template_used = ?`
	row := cts.helper.QuerySingleRow(query, id)

	var count int
	err := row.Scan(&count)
	if err != nil {
		return &TemplateResult{
			Success: false,
			Message: fmt.Sprintf("Failed to check template usage: %v", err),
		}
	}

	if count > 0 {
		return &TemplateResult{
			Success: false,
			Message: fmt.Sprintf("Cannot delete template: it is used by %d bank statement(s)", count),
		}
	}

	// Delete the template
	deleteQuery := `DELETE FROM csv_templates WHERE id = ?`
	rowsAffected, err := cts.helper.ExecReturnRowsAffected(deleteQuery, id)
	if err != nil {
		return &TemplateResult{
			Success: false,
			Message: fmt.Sprintf("Failed to delete template: %v", err),
		}
	}

	if rowsAffected == 0 {
		return &TemplateResult{
			Success: false,
			Message: "Template not found",
		}
	}

	return &TemplateResult{
		Success: true,
		Message: "Template deleted successfully",
	}
}
