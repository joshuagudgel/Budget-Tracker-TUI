package types

import (
	"fmt"
	"strings"
	"time"
)

// Transaction represents a financial transaction
type Transaction struct {
	Id              int64     `db:"id"`
	ParentId        *int64    `db:"parent_id"`
	Amount          float64   `db:"amount"`
	Description     string    `db:"description"`
	RawDescription  string    `db:"raw_description"`
	Date            time.Time `db:"date"`
	CategoryId      int64     `db:"category_id"`
	TransactionType string    `db:"transaction_type"`
	IsSplit         bool      `db:"is_split"`
	StatementId     int64     `db:"statement_id"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

// Category represents a transaction category
type Category struct {
	Id          int64     `db:"id"`
	DisplayName string    `db:"display_name"`
	ParentId    *int64    `db:"parent_id"`
	Color       string    `db:"color"`
	IsActive    bool      `db:"is_active"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// BankStatement represents an imported bank statement
type BankStatement struct {
	Id             int64     `db:"id"`
	Filename       string    `db:"filename"`
	ImportDate     time.Time `db:"import_date"`
	PeriodStart    time.Time `db:"period_start"`
	PeriodEnd      time.Time `db:"period_end"`
	TemplateUsed   int64     `db:"template_used"`
	TxCount        int       `db:"tx_count"`
	Status         string    `db:"status"`
	ProcessingTime int64     `db:"processing_time"`
	ErrorLog       string    `db:"error_log"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

// CSVTemplate represents a CSV import template configuration
type CSVTemplate struct {
	Id             int64     `db:"id"`
	Name           string    `db:"name"`
	PostDateColumn int       `db:"post_date_column"`
	AmountColumn   int       `db:"amount_column"`
	DescColumn     int       `db:"desc_column"`
	CategoryColumn *int      `db:"category_column"`
	HasHeader      bool      `db:"has_header"`
	DateFormat     string    `db:"date_format"`
	Delimiter      string    `db:"delimiter"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

// Display and conversion methods for Transaction (pure utility methods)
func (t *Transaction) GetDateForDisplay() string {
	return t.Date.Format("01/02/2006") // MM/DD/YYYY for UI display
}

func (t *Transaction) GetDateForStorage() string {
	return t.Date.Format("2006-01-02") // ISO 8601 for database
}

func (t *Transaction) SetDateFromUserInput(dateStr string) error {
	parsed, err := TryParseMultipleDateFormats(dateStr)
	if err != nil {
		return err
	}
	t.Date = parsed
	return nil
}

// Legacy validation methods for Category (should eventually be moved to validation package)

// Validate validates the category and returns a ValidationResult
func (c *Category) Validate(availableCategories []Category) ValidationResult {
	result := ValidationResult{IsValid: true}

	// Validate DisplayName
	if err := c.validateDisplayName(availableCategories); err != nil {
		result.AddError("displayName", err.Error())
	}

	return result
}

// ValidateField validates a single field and returns any error
func (c *Category) ValidateField(field string, availableCategories []Category) error {
	switch strings.ToLower(field) {
	case "displayname":
		return c.validateDisplayName(availableCategories)
	default:
		return fmt.Errorf("unknown field: %s", field)
	}
}

// validateDisplayName validates the displayName field
func (c *Category) validateDisplayName(availableCategories []Category) error {
	trimmed := strings.TrimSpace(c.DisplayName)
	if trimmed == "" {
		return fmt.Errorf("display name cannot be empty")
	}

	if len(trimmed) > 100 {
		return fmt.Errorf("display name cannot exceed 100 characters")
	}

	// Check for duplicates (excluding self)
	for _, category := range availableCategories {
		if category.Id != c.Id && strings.EqualFold(category.DisplayName, trimmed) {
			return fmt.Errorf("category with name '%s' already exists", trimmed)
		}
	}

	return nil
}

// Legacy validation methods for CSVTemplate (should eventually be moved to validation package)

// Validate validates the CSV template and returns a ValidationResult
func (ct *CSVTemplate) Validate() ValidationResult {
	result := ValidationResult{IsValid: true}

	// Validate Name
	if err := ct.validateName(); err != nil {
		result.AddError("name", err.Error())
	}

	// Validate column indices
	if err := ct.validatePostDateColumn(); err != nil {
		result.AddError("postDateColumn", err.Error())
	}

	if err := ct.validateAmountColumn(); err != nil {
		result.AddError("amountColumn", err.Error())
	}

	if err := ct.validateDescColumn(); err != nil {
		result.AddError("descColumn", err.Error())
	}

	if err := ct.validateCategoryColumn(); err != nil {
		result.AddError("categoryColumn", err.Error())
	}

	// Check for duplicate column indices
	if err := ct.validateUniqueColumns(); err != nil {
		result.AddError("columns", err.Error())
	}

	return result
}

// ValidateField validates a single field and returns any error
func (ct *CSVTemplate) ValidateField(field string) error {
	switch strings.ToLower(field) {
	case "name":
		return ct.validateName()
	case "postdatecolumn":
		return ct.validatePostDateColumn()
	case "amountcolumn":
		return ct.validateAmountColumn()
	case "desccolumn":
		return ct.validateDescColumn()
	case "categorycolumn":
		return ct.validateCategoryColumn()
	default:
		return fmt.Errorf("unknown field: %s", field)
	}
}

// validateName validates the template name
func (ct *CSVTemplate) validateName() error {
	trimmed := strings.TrimSpace(ct.Name)
	if trimmed == "" {
		return fmt.Errorf("template name cannot be empty")
	}
	if len(trimmed) > 100 {
		return fmt.Errorf("template name cannot exceed 100 characters")
	}
	return nil
}

// validatePostDateColumn validates the post date column index
func (ct *CSVTemplate) validatePostDateColumn() error {
	if ct.PostDateColumn < 0 {
		return fmt.Errorf("post date column index cannot be negative")
	}
	return nil
}

// validateAmountColumn validates the amount column index
func (ct *CSVTemplate) validateAmountColumn() error {
	if ct.AmountColumn < 0 {
		return fmt.Errorf("amount column index cannot be negative")
	}
	return nil
}

// validateDescColumn validates the description column index
func (ct *CSVTemplate) validateDescColumn() error {
	if ct.DescColumn < 0 {
		return fmt.Errorf("description column index cannot be negative")
	}
	return nil
}

// validateCategoryColumn validates the category column index (optional)
func (ct *CSVTemplate) validateCategoryColumn() error {
	if ct.CategoryColumn != nil && *ct.CategoryColumn < 0 {
		return fmt.Errorf("category column index cannot be negative")
	}
	return nil
}

// validateUniqueColumns ensures no duplicate column indices
func (ct *CSVTemplate) validateUniqueColumns() error {
	usedColumns := make(map[int]string)

	// Check each required column
	usedColumns[ct.PostDateColumn] = "PostDate"

	if existing, exists := usedColumns[ct.AmountColumn]; exists {
		return fmt.Errorf("amount column %d is already used by %s column", ct.AmountColumn, existing)
	}
	usedColumns[ct.AmountColumn] = "Amount"

	if existing, exists := usedColumns[ct.DescColumn]; exists {
		return fmt.Errorf("description column %d is already used by %s column", ct.DescColumn, existing)
	}
	usedColumns[ct.DescColumn] = "Description"

	// Check optional category column
	if ct.CategoryColumn != nil {
		if existing, exists := usedColumns[*ct.CategoryColumn]; exists {
			return fmt.Errorf("category column %d is already used by %s column", *ct.CategoryColumn, existing)
		}
	}

	return nil
}

// Snapshot represents a database snapshot
type Snapshot struct {
	Id               int64     `db:"id"`
	Name             string    `db:"name"`
	Description      string    `db:"description"`
	FilePath         string    `db:"file_path"`
	FileSize         int64     `db:"file_size"`
	TransactionCount int       `db:"transaction_count"`
	CategoryCount    int       `db:"category_count"`
	StatementCount   int       `db:"statement_count"`
	TemplateCount    int       `db:"template_count"`
	AuditEventCount  int       `db:"audit_event_count"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

// GetSizeDisplay returns human-readable file size
func (s *Snapshot) GetSizeDisplay() string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	size := float64(s.FileSize)
	switch {
	case size >= GB:
		return fmt.Sprintf("%.1f GB", size/GB)
	case size >= MB:
		return fmt.Sprintf("%.1f MB", size/MB)
	case size >= KB:
		return fmt.Sprintf("%.1f KB", size/KB)
	default:
		return fmt.Sprintf("%d B", s.FileSize)
	}
}

// GetCreatedAtDisplay returns formatted creation date
func (s *Snapshot) GetCreatedAtDisplay() string {
	return s.CreatedAt.Format("01/02/2006 3:04 PM")
}
