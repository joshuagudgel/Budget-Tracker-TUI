package storage

import (
	"budget-tracker-tui/internal/ml"
	"budget-tracker-tui/internal/types"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// CSVParser handles CSV parsing operations with support for partial completion
type CSVParser struct {
	transactionStore *TransactionStore
	categoryStore    *CategoryStore
	mlCategorizer    *ml.EmbeddingsCategorizer
}

// NewCSVParser creates a new CSV parser with required dependencies
func NewCSVParser(transactionStore *TransactionStore, categoryStore *CategoryStore, mlCategorizer *ml.EmbeddingsCategorizer) *CSVParser {
	return &CSVParser{
		transactionStore: transactionStore,
		categoryStore:    categoryStore,
		mlCategorizer:    mlCategorizer,
	}
}

// ParseCSV parses a CSV file based on the specified template and mode
func (cp *CSVParser) ParseCSV(filePath string, template *types.CSVTemplate, mode types.ParseMode) (*types.CSVParseResult, error) {
	// Validate dependencies
	if cp.categoryStore == nil {
		return nil, fmt.Errorf("category store is required for CSV parsing")
	}

	// Read CSV file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty CSV file")
	}

	// Initialize result
	result := &types.CSVParseResult{
		SuccessfulTransactions: make([]types.Transaction, 0),
		FailedRows:             make([]types.RowError, 0),
		DuplicateRows:          make([]types.Transaction, 0),
		CanProceedPartially:    mode == types.SkipInvalid,
	}

	// Calculate starting line
	startLine := 0
	if template.HasHeader {
		startLine = 1
	}

	// Determine delimiter
	delimiter := ","
	if template.Delimiter != "" {
		delimiter = template.Delimiter
	}

	// Get default category ID
	defaultCategoryId := cp.categoryStore.GetDefaultCategoryId()

	// Parse each line
	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue // Skip empty lines
		}

		// Parse CSV line into fields
		fields := cp.ParseCSVLine(line, delimiter)

		// Parse transaction from fields
		transaction, err := cp.parseTransactionFromTemplate(fields, template, i+1, defaultCategoryId)
		if err != nil {
			// Handle error based on mode
			if mode == types.FailFast {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}

			// SkipInvalid mode - collect error and continue
			rowError := types.RowError{
				LineNumber: i + 1,
				RawRow:     line,
				ErrorType:  cp.categorizeError(err),
				Message:    err.Error(),
				Field:      cp.extractFieldFromError(err),
			}
			result.FailedRows = append(result.FailedRows, rowError)
			continue
		}

		// Add successful transaction
		result.SuccessfulTransactions = append(result.SuccessfulTransactions, *transaction)
	}

	// Calculate summary
	totalRows := len(lines)
	if template.HasHeader && totalRows > 0 {
		totalRows-- // Don't count header
	}

	result.Summary = types.NewImportSummary(
		totalRows,
		len(result.SuccessfulTransactions),
		len(result.FailedRows),
		len(result.DuplicateRows),
	)

	return result, nil
}

// ParseWithDuplicateDetection parses CSV and separates new from duplicate transactions
func (cp *CSVParser) ParseWithDuplicateDetection(filePath string, template *types.CSVTemplate) (*types.CSVParseResult, error) {
	// First parse all transactions (fail-fast mode for validation)
	result, err := cp.ParseCSV(filePath, template, types.FailFast)
	if err != nil {
		return nil, err
	}

	// If no transaction store, return all as successful
	if cp.transactionStore == nil {
		return result, nil
	}

	// Filter duplicates
	newTransactions := make([]types.Transaction, 0)
	duplicateTransactions := make([]types.Transaction, 0)

	// Check each transaction for duplicates
	for _, tx := range result.SuccessfulTransactions {
		isDuplicate := cp.checkForDuplicate(tx, result.SuccessfulTransactions, newTransactions)
		if isDuplicate {
			duplicateTransactions = append(duplicateTransactions, tx)
		} else {
			newTransactions = append(newTransactions, tx)
		}
	}

	// Update result with duplicate information
	result.SuccessfulTransactions = newTransactions
	result.DuplicateRows = duplicateTransactions

	// Recalculate summary
	result.Summary = types.NewImportSummary(
		result.Summary.TotalRows,
		len(result.SuccessfulTransactions),
		len(result.FailedRows),
		len(result.DuplicateRows),
	)

	return result, nil
}

// parseTransactionFromTemplate creates a transaction from CSV fields using a template
func (cp *CSVParser) parseTransactionFromTemplate(fields []string, template *types.CSVTemplate, lineNum int, defaultCategoryId int64) (*types.Transaction, error) {
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
	if template.CategoryColumn != nil && *template.CategoryColumn > maxColumn {
		maxColumn = *template.CategoryColumn
	}

	if len(fields) <= maxColumn {
		return nil, fmt.Errorf("Insufficient columns (%d), need at least %d", len(fields), maxColumn+1)
	}

	// Extract and parse date
	rawDate := strings.Trim(fields[template.PostDateColumn], "\"")
	normalizedDate, err := types.NormalizeDateToISO8601(rawDate, template.DateFormat)
	if err != nil {
		return nil, fmt.Errorf("invalid date '%s': %w", rawDate, err)
	}

	txDate, err := time.Parse("2006-01-02", normalizedDate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse normalized date '%s': %w", normalizedDate, err)
	}
	transaction.Date = txDate

	// Extract description
	desc := strings.Trim(fields[template.DescColumn], "\"")
	if strings.TrimSpace(desc) == "" {
		return nil, fmt.Errorf("empty description not allowed")
	}
	transaction.Description = desc
	transaction.RawDescription = desc

	// Extract and parse amount
	amountStr := strings.Trim(fields[template.AmountColumn], "\"")
	transaction.Amount, err = cp.ParseAmount(amountStr)
	if err != nil {
		return nil, fmt.Errorf("invalid amount '%s': %w", amountStr, err)
	}

	// Handle category assignment with ML prediction integration
	categoryId := cp.assignCategory(desc, transaction.Amount, template, fields, defaultCategoryId)
	transaction.CategoryId = categoryId

	return &transaction, nil
}

// assignCategory determines the appropriate category using ML-first approach
func (cp *CSVParser) assignCategory(description string, amount float64, template *types.CSVTemplate, fields []string, defaultCategoryId int64) int64 {
	// Step 1: Try ML prediction if available
	if cp.mlCategorizer != nil {
		prediction := cp.mlCategorizer.PredictCategory(description, amount)

		// Use high-confidence ML predictions
		if prediction.Confidence >= 0.7 { // High confidence threshold
			fmt.Printf("[ML] Auto-categorized '%s' → Category %d (confidence: %.2f)\n",
				description, prediction.CategoryId, prediction.Confidence)
			return prediction.CategoryId
		}
	}

	// Step 2: Fallback to CSV category column if available
	if template.CategoryColumn != nil && cp.categoryStore != nil {
		categoryText := strings.Trim(fields[*template.CategoryColumn], "\"")
		if categoryText != "" {
			return cp.categoryStore.ResolveOrCreateCategory(categoryText)
		}
	}

	// Step 3: Final fallback to default category
	return defaultCategoryId
}

// checkForDuplicate determines if a transaction is a duplicate using existing logic
func (cp *CSVParser) checkForDuplicate(tx types.Transaction, allTransactions []types.Transaction, processedNew []types.Transaction) bool {
	// Find existing transactions with same date, amount, and description
	dateStr := tx.Date.Format("2006-01-02")
	existingTxs, err := cp.transactionStore.FindDuplicateTransactions(dateStr, tx.Amount, tx.Description)
	if err != nil {
		// If error querying, assume it's new to be safe
		return false
	}

	// Count how many transactions with these details we're trying to import
	importCount := 0
	for _, importTx := range allTransactions {
		if time.Time.Equal(importTx.Date, tx.Date) &&
			math.Abs(importTx.Amount-tx.Amount) < 0.01 &&
			importTx.Description == tx.Description {
			importCount++
		}
	}

	existingCount := len(existingTxs)

	// Count how many of this specific transaction we've already processed as "new"
	previousNewCount := 0
	for _, newTx := range processedNew {
		if time.Time.Equal(newTx.Date, tx.Date) &&
			math.Abs(newTx.Amount-tx.Amount) < 0.01 &&
			newTx.Description == tx.Description {
			previousNewCount++
		}
	}

	// Check if this transaction is truly new
	return existingCount+previousNewCount >= importCount
}

// ParseCSVLine parses a CSV line into fields using the specified delimiter
func (cp *CSVParser) ParseCSVLine(line, delimiter string) []string {
	var fields []string
	current := ""
	inQuotes := false

	for i := 0; i < len(line); i++ {
		char := line[i]

		if char == '"' {
			if inQuotes && i+1 < len(line) && line[i+1] == '"' {
				// Escaped quote
				current += "\""
				i++ // Skip next quote
			} else {
				// Toggle quote state
				inQuotes = !inQuotes
			}
		} else if char == delimiter[0] && !inQuotes {
			// Field separator
			fields = append(fields, current)
			current = ""
		} else {
			current += string(char)
		}
	}

	// Add final field
	fields = append(fields, current)
	return fields
}

// ParseAmount parses a currency string into a float64
func (cp *CSVParser) ParseAmount(amountStr string) (float64, error) {
	// Remove currency symbols and whitespace
	cleaned := strings.TrimSpace(amountStr)
	cleaned = strings.ReplaceAll(cleaned, "$", "")
	cleaned = strings.ReplaceAll(cleaned, ",", "")

	// Handle negative amounts in parentheses (e.g., "(50.00)")
	if strings.HasPrefix(cleaned, "(") && strings.HasSuffix(cleaned, ")") {
		cleaned = "-" + strings.Trim(cleaned, "()")
	}

	// Parse as float
	amount, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount format: %s", amountStr)
	}

	return amount, nil
}

// categorizeError determines the type of parsing error
func (cp *CSVParser) categorizeError(err error) string {
	errMsg := strings.ToLower(err.Error())

	if strings.Contains(errMsg, "date") {
		return "date_parsing"
	}
	if strings.Contains(errMsg, "amount") {
		return "amount_parsing"
	}
	if strings.Contains(errMsg, "columns") {
		return "column_validation"
	}

	return "general_parsing"
}

// extractFieldFromError attempts to extract which field caused the error
func (cp *CSVParser) extractFieldFromError(err error) string {
	errMsg := strings.ToLower(err.Error())

	if strings.Contains(errMsg, "date") {
		return "Date"
	}
	if strings.Contains(errMsg, "amount") {
		return "Amount"
	}
	if strings.Contains(errMsg, "description") {
		return "Description"
	}

	return "unknown"
}
