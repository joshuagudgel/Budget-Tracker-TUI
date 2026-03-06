package storage

import (
	"budget-tracker-tui/internal/database"
	"budget-tracker-tui/internal/types"
	"database/sql"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// CSVTemplateStore handles all CSV template-related operations using SQLite
type CSVTemplateStore struct {
	db               *database.Connection
	helper           *database.SQLHelper
	defaultTemplate  string
	transactionStore *TransactionStore
	categoryStore    *CategoryStore
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

// SetTransactionStore sets the transaction store reference (called after all stores are initialized)
func (cts *CSVTemplateStore) SetTransactionStore(ts *TransactionStore) {
	cts.transactionStore = ts
}

// SetCategoryStore sets the category store reference (called after all stores are initialized)
func (cts *CSVTemplateStore) SetCategoryStore(cs *CategoryStore) {
	cts.categoryStore = cs
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
		SELECT id, name, post_date_column, amount_column, desc_column, category_column, merchant_column,
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
	var categoryColumn, merchantColumn sql.NullInt64
	var dateFormat, delimiter sql.NullString

	err := rows.Scan(
		&template.Id, &template.Name, &template.PostDateColumn, &template.AmountColumn,
		&template.DescColumn, &categoryColumn, &merchantColumn, &template.HasHeader,
		&dateFormat, &delimiter, &template.CreatedAt, &template.UpdatedAt,
	)

	if err != nil {
		return template, err
	}

	// Handle nullable fields
	if categoryColumn.Valid {
		categoryInt := int(categoryColumn.Int64)
		template.CategoryColumn = &categoryInt
	}
	if merchantColumn.Valid {
		merchantInt := int(merchantColumn.Int64)
		template.MerchantColumn = &merchantInt
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
		SELECT id, name, post_date_column, amount_column, desc_column, category_column, merchant_column,
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
	var merchantColumn sql.NullInt64
	var dateFormat, delimiter sql.NullString

	err := row.Scan(
		&template.Id, &template.Name, &template.PostDateColumn, &template.AmountColumn,
		&template.DescColumn, &categoryColumn, &merchantColumn, &template.HasHeader, &dateFormat,
		&delimiter, &template.CreatedAt, &template.UpdatedAt,
	)

	if err != nil {
		return template, err
	}

	// Handle nullable fields
	if categoryColumn.Valid {
		categoryInt := int(categoryColumn.Int64)
		template.CategoryColumn = &categoryInt
	}
	if merchantColumn.Valid {
		merchantInt := int(merchantColumn.Int64)
		template.MerchantColumn = &merchantInt
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
		SELECT id, name, post_date_column, amount_column, desc_column, category_column, merchant_column,
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
	now := time.Now().Format(time.RFC3339)

	if template.Id == 0 {
		// Insert new template
		return cts.insertTemplate(template, now)
	} else {
		// Update existing template
		return cts.updateTemplate(template, now)
	}
}

// insertTemplate inserts a new CSV template
func (cts *CSVTemplateStore) insertTemplate(template types.CSVTemplate, now string) error {
	query := `
		INSERT INTO csv_templates (
			name, post_date_column, amount_column, desc_column, category_column, merchant_column,
			has_header, date_format, delimiter, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// Handle nullable fields
	var categoryColumn interface{}
	if template.CategoryColumn != nil {
		categoryColumn = *template.CategoryColumn
	}

	var merchantColumn interface{}
	if template.MerchantColumn != nil {
		merchantColumn = *template.MerchantColumn
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
	if createdAt == "" {
		createdAt = now
	}

	id, err := cts.helper.ExecReturnID(query,
		template.Name, template.PostDateColumn, template.AmountColumn,
		template.DescColumn, categoryColumn, merchantColumn, template.HasHeader,
		dateFormat, delimiter, createdAt, now,
	)

	if err != nil {
		return fmt.Errorf("failed to insert CSV template: %w", err)
	}

	// Update the template ID
	template.Id = id
	return nil
}

// updateTemplate updates an existing CSV template
func (cts *CSVTemplateStore) updateTemplate(template types.CSVTemplate, now string) error {
	query := `
		UPDATE csv_templates SET 
			name = ?, post_date_column = ?, amount_column = ?, desc_column = ?, category_column = ?, 
			merchant_column = ?, has_header = ?, date_format = ?, delimiter = ?, 
			updated_at = ?
		WHERE id = ?
	`

	// Handle nullable fields
	var categoryColumn interface{}
	if template.CategoryColumn != nil {
		categoryColumn = *template.CategoryColumn
	}

	var merchantColumn interface{}
	if template.MerchantColumn != nil {
		merchantColumn = *template.MerchantColumn
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
		template.DescColumn, categoryColumn, merchantColumn, template.HasHeader,
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

// ParseCSVLine parses a CSV line handling quoted fields
func (cts *CSVTemplateStore) ParseCSVLine(line string, delimiter string) []string {
	if delimiter == "" {
		delimiter = ","
	}

	var fields []string
	var current strings.Builder
	inQuotes := false

	for _, char := range line {
		if char == '"' {
			inQuotes = !inQuotes
		} else if char == rune(delimiter[0]) && !inQuotes {
			fields = append(fields, strings.TrimSpace(current.String()))
			current.Reset()
		} else {
			current.WriteRune(char)
		}
	}

	// Add final field
	fields = append(fields, strings.TrimSpace(current.String()))
	return fields
}

// ParseAmount parses an amount string handling currency symbols and parentheses
func (cts *CSVTemplateStore) ParseAmount(amountStr string) (float64, error) {
	// Clean amount string: remove currency symbols, parentheses, extra spaces
	cleaned := strings.TrimSpace(amountStr)
	cleaned = strings.ReplaceAll(cleaned, "$", "")
	cleaned = strings.ReplaceAll(cleaned, ",", "")

	// Handle negative amounts in parentheses (e.g., "(50.00)")
	if strings.HasPrefix(cleaned, "(") && strings.HasSuffix(cleaned, ")") {
		cleaned = "-" + strings.Trim(cleaned, "()")
	}

	return strconv.ParseFloat(cleaned, 64)
}

// ParseTransactionFromTemplate creates a transaction from CSV fields using a template
func (cts *CSVTemplateStore) ParseTransactionFromTemplate(fields []string, template *types.CSVTemplate, lineNum int, defaultCategoryId int64) (*types.Transaction, error) {
	var transaction types.Transaction
	var err error

	// Validate field count
	maxColumn := template.PostDateColumn
	if template.AmountColumn > maxColumn {
		maxColumn = template.AmountColumn
	}
	if template.DescColumn > maxColumn {
		maxColumn = template.DescColumn
	}
	if template.MerchantColumn != nil && *template.MerchantColumn > maxColumn {
		maxColumn = *template.MerchantColumn
	}
	if template.CategoryColumn != nil && *template.CategoryColumn > maxColumn {
		maxColumn = *template.CategoryColumn
	}

	if len(fields) <= maxColumn {
		return nil, fmt.Errorf("line %d: insufficient columns (%d), need at least %d", lineNum, len(fields), maxColumn+1)
	}

	// Extract date from specified column
	rawDate := strings.Trim(fields[template.PostDateColumn], "\"")
	normalizedDate, err := types.NormalizeDateToISO8601(rawDate, template.DateFormat)
	if err != nil {
		return nil, fmt.Errorf("line %d: invalid date '%s': %v", lineNum, rawDate, err)
	}
	transaction.Date = normalizedDate

	// Extract description from specified column
	desc := strings.Trim(fields[template.DescColumn], "\"")
	transaction.Description = desc
	transaction.RawDescription = desc // Store original description

	// Extract merchant if available
	if template.MerchantColumn != nil {
		merchant := strings.Trim(fields[*template.MerchantColumn], "\"")
		if merchant != "" && merchant != desc {
			// Combine merchant and description
			transaction.Description = fmt.Sprintf("%s - %s", merchant, desc)
		}
	}

	// Extract amount from specified column
	amountStr := strings.Trim(fields[template.AmountColumn], "\"")
	transaction.Amount, err = cts.ParseAmount(amountStr)
	if err != nil {
		return nil, fmt.Errorf("line %d: invalid amount '%s': %v", lineNum, amountStr, err)
	}

	// Handle category assignment - use CSV category if available, otherwise default
	var categoryId int64
	var confidence float64
	var autoCategory string

	if template.CategoryColumn != nil {
		// Extract category from CSV
		categoryText := strings.Trim(fields[*template.CategoryColumn], "\"")
		autoCategory = categoryText // Store original bank category text for ML

		if cts.categoryStore != nil {
			// Use category store to resolve or create category
			categoryId, confidence = cts.categoryStore.ResolveOrCreateCategory(categoryText)
		} else {
			// Fallback to default if category store not available
			categoryId = defaultCategoryId
			confidence = 0.5 // Medium confidence - we have bank category but using default
		}
	} else {
		// No category column, use default
		categoryId = defaultCategoryId
		confidence = 0.0 // Low confidence - pure default assignment
	}

	transaction.CategoryId = categoryId
	transaction.AutoCategory = autoCategory
	transaction.Confidence = confidence

	return &transaction, nil
}

// ParseCSVTransactions parses an entire CSV file using a template
func (cts *CSVTemplateStore) ParseCSVTransactions(filePath string, template *types.CSVTemplate, defaultCategoryId int64) ([]types.Transaction, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty CSV file")
	}

	var transactions []types.Transaction
	startLine := 0
	if template.HasHeader {
		startLine = 1
	}

	delimiter := ","
	if template.Delimiter != "" {
		delimiter = template.Delimiter
	}

	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		fields := cts.ParseCSVLine(line, delimiter)

		transaction, err := cts.ParseTransactionFromTemplate(fields, template, i+1, defaultCategoryId)
		if err != nil {
			continue // Skip invalid transactions
		}

		transactions = append(transactions, *transaction)
	}

	return transactions, nil
}

// ParseCSVTransactionsWithDuplicateFilter parses CSV transactions and separates new from duplicate transactions
func (cts *CSVTemplateStore) ParseCSVTransactionsWithDuplicateFilter(filePath string, template *types.CSVTemplate, defaultCategoryId int64) ([]types.Transaction, []types.Transaction, error) {
	// First parse all transactions
	allTransactions, err := cts.ParseCSVTransactions(filePath, template, defaultCategoryId)
	if err != nil {
		return nil, nil, err
	}

	var newTransactions []types.Transaction
	var duplicateTransactions []types.Transaction

	// If transaction store is not set, return all as new (fallback)
	if cts.transactionStore == nil {
		return allTransactions, duplicateTransactions, nil
	}

	// Check each transaction for duplicates
	for _, tx := range allTransactions {
		// Find existing transactions with same date, amount, and description
		existingTxs, err := cts.transactionStore.FindDuplicateTransactions(tx.Date, tx.Amount, tx.Description)
		if err != nil {
			// If error querying, assume it's new to be safe
			newTransactions = append(newTransactions, tx)
			continue
		}

		// Count how many transactions with these details we're trying to import
		importCount := 0
		for _, importTx := range allTransactions {
			if importTx.Date == tx.Date &&
				math.Abs(importTx.Amount-tx.Amount) < 0.01 &&
				importTx.Description == tx.Description {
				importCount++
			}
		}

		existingCount := len(existingTxs)

		// If we have more transactions to import than already exist, this one is new
		// Count how many of this specific transaction we've already processed as "new"
		previousNewCount := 0
		for _, newTx := range newTransactions {
			if newTx.Date == tx.Date &&
				math.Abs(newTx.Amount-tx.Amount) < 0.01 &&
				newTx.Description == tx.Description {
				previousNewCount++
			}
		}

		// Check if this transaction is truly new
		if existingCount+previousNewCount < importCount {
			newTransactions = append(newTransactions, tx)
		} else {
			duplicateTransactions = append(duplicateTransactions, tx)
		}
	}

	return newTransactions, duplicateTransactions, nil
}

// ValidateCSVData validates all transactions in a CSV file without importing them
// Returns a slice of ValidationError objects, each with a line number
func (cts *CSVTemplateStore) ValidateCSVData(filePath string, template *types.CSVTemplate, defaultCategoryId int64) ([]types.ValidationError, error) {
	// Read CSV file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty CSV file")
	}

	var validationErrors []types.ValidationError
	startLine := 0
	if template.HasHeader {
		startLine = 1
	}

	delimiter := ","
	if template.Delimiter != "" {
		delimiter = template.Delimiter
	}

	// Get date format from template, use default if not specified
	dateFormat := template.DateFormat
	if dateFormat == "" {
		dateFormat = "01/02/2006" // Default MM/DD/YYYY format
	}

	_ = cts.getDateFormatsToTry(dateFormat)

	// Validate each line
	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue // Skip empty lines
		}

		lineNumber := i + 1 // 1-based line numbers for user display
		fields := cts.ParseCSVLine(line, delimiter)

		// Validate field count
		maxColumn := template.PostDateColumn
		if template.AmountColumn > maxColumn {
			maxColumn = template.AmountColumn
		}
		if template.DescColumn > maxColumn {
			maxColumn = template.DescColumn
		}
		if template.MerchantColumn != nil && *template.MerchantColumn > maxColumn {
			maxColumn = *template.MerchantColumn
		}
		if template.CategoryColumn != nil && *template.CategoryColumn > maxColumn {
			maxColumn = *template.CategoryColumn
		}

		if len(fields) <= maxColumn {
			validationErrors = append(validationErrors, types.ValidationError{
				Field:      "CSV Structure",
				Message:    fmt.Sprintf("Insufficient columns (%d), need at least %d", len(fields), maxColumn+1),
				LineNumber: lineNumber,
			})
			continue // Skip to next line
		}

		// Validate Date field using flexible multi-format validation
		dateStr := strings.Trim(fields[template.PostDateColumn], "\"")
		_, err := types.TryParseMultipleDateFormats(dateStr)
		if err != nil {
			validationErrors = append(validationErrors, types.ValidationError{
				Field:      "Date",
				Message:    fmt.Sprintf("Invalid date '%s': must be in MM/DD/YYYY or MM-DD-YYYY format", dateStr),
				LineNumber: lineNumber,
			})
		}

		// Validate Amount field
		amountStr := strings.Trim(fields[template.AmountColumn], "\"")
		amount, err := cts.ParseAmount(amountStr)
		if err != nil {
			validationErrors = append(validationErrors, types.ValidationError{
				Field:      "Amount",
				Message:    fmt.Sprintf("Invalid amount '%s': %v", amountStr, err),
				LineNumber: lineNumber,
			})
		} else {
			// Check for zero amount
			if amount == 0 {
				validationErrors = append(validationErrors, types.ValidationError{
					Field:      "Amount",
					Message:    "Amount cannot be zero",
					LineNumber: lineNumber,
				})
			}
			// Check for max 2 decimal places
			rounded := math.Round(amount*100) / 100
			if math.Abs(amount-rounded) > 0.001 {
				validationErrors = append(validationErrors, types.ValidationError{
					Field:      "Amount",
					Message:    "Amount cannot have more than 2 decimal places",
					LineNumber: lineNumber,
				})
			}
		}

		// Validate Description field
		desc := strings.Trim(fields[template.DescColumn], "\"")
		trimmedDesc := strings.TrimSpace(desc)
		if trimmedDesc == "" {
			validationErrors = append(validationErrors, types.ValidationError{
				Field:      "Description",
				Message:    "Description cannot be empty",
				LineNumber: lineNumber,
			})
		}
		if len(trimmedDesc) > 255 {
			validationErrors = append(validationErrors, types.ValidationError{
				Field:      "Description",
				Message:    "Description cannot exceed 255 characters",
				LineNumber: lineNumber,
			})
		}

		// Validate Category field if present (basic validation only)
		if template.CategoryColumn != nil {
			categoryText := strings.Trim(fields[*template.CategoryColumn], "\"")
			categoryText = strings.TrimSpace(categoryText)

			// Validate category text format (length, characters, etc.)
			if len(categoryText) > 100 {
				validationErrors = append(validationErrors, types.ValidationError{
					Field:      "Category",
					Message:    "Category name cannot exceed 100 characters",
					LineNumber: lineNumber,
				})
			}
			// Note: We don't validate if category exists here - unknown categories
			// will be automatically created during import via ResolveOrCreateCategory
		}
	}

	return validationErrors, nil
}
