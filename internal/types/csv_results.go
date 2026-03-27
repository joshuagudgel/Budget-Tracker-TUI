package types

// ParseMode defines how CSV parsing should handle errors
type ParseMode int

const (
	FailFast    ParseMode = iota // Stop on first error
	SkipInvalid                  // Continue parsing, collect errors for later review
)

// CSVParseResult contains the results of CSV parsing operation
type CSVParseResult struct {
	SuccessfulTransactions []Transaction `json:"successfulTransactions"`
	FailedRows             []RowError    `json:"failedRows"`
	DuplicateRows          []Transaction `json:"duplicateRows"` // For override scenarios
	Summary                ImportSummary `json:"summary"`
	CanProceedPartially    bool          `json:"canProceedPartially"`
}

// RowError represents a parsing error for a specific CSV row
type RowError struct {
	LineNumber int    `json:"lineNumber"`
	RawRow     string `json:"rawRow"`
	ErrorType  string `json:"errorType"` // "validation", "parsing", "duplicate"
	Message    string `json:"message"`
	Field      string `json:"field"` // Which field caused the error
}

// ImportSummary provides statistics about the CSV parsing operation
type ImportSummary struct {
	TotalRows     int     `json:"totalRows"`
	Successful    int     `json:"successful"`
	Failed        int     `json:"failed"`
	Duplicates    int     `json:"duplicates"`
	FailureRate   float64 `json:"failureRate"`
	ProcessedRows int     `json:"processedRows"` // Total non-empty rows processed
}

// NewImportSummary creates an ImportSummary from parse results
func NewImportSummary(totalRows, successful, failed, duplicates int) ImportSummary {
	processedRows := successful + failed + duplicates
	failureRate := 0.0
	if processedRows > 0 {
		failureRate = float64(failed) / float64(processedRows)
	}

	return ImportSummary{
		TotalRows:     totalRows,
		Successful:    successful,
		Failed:        failed,
		Duplicates:    duplicates,
		FailureRate:   failureRate,
		ProcessedRows: processedRows,
	}
}

// HasErrors returns true if any parsing errors occurred
func (r *CSVParseResult) HasErrors() bool {
	return len(r.FailedRows) > 0
}

// HasDuplicates returns true if any duplicate transactions were found
func (r *CSVParseResult) HasDuplicates() bool {
	return len(r.DuplicateRows) > 0
}

// GetErrorsByType returns all errors of a specific type
func (r *CSVParseResult) GetErrorsByType(errorType string) []RowError {
	var errors []RowError
	for _, err := range r.FailedRows {
		if err.ErrorType == errorType {
			errors = append(errors, err)
		}
	}
	return errors
}
