package storage

import (
	"budget-tracker-tui/internal/types"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CSVTemplateStore handles all CSV template-related operations
type CSVTemplateStore struct {
	filename  string
	Templates []types.CSVTemplate `json:"templates"`
	Default   string              `json:"default"`
	NextId    int64               `json:"nextId"`
}

// NewCSVTemplateStore creates a new CSVTemplateStore instance
func NewCSVTemplateStore(templateFile string) *CSVTemplateStore {
	return &CSVTemplateStore{
		filename:  templateFile,
		Templates: []types.CSVTemplate{},
		Default:   "",
		NextId:    1,
	}
}

// LoadCSVTemplates loads CSV templates from the JSON file
func (cts *CSVTemplateStore) LoadCSVTemplates() error {
	if _, err := os.Stat(cts.filename); os.IsNotExist(err) {
		// Create default templates
		cts.Templates = []types.CSVTemplate{
			{Id: 1, Name: "Bank1", DateColumn: 0, AmountColumn: 1, DescColumn: 4, HasHeader: false},
			{Id: 2, Name: "Bank2", DateColumn: 0, AmountColumn: 5, DescColumn: 2, HasHeader: true},
		}
		cts.Default = "Bank1"
		cts.NextId = 3
		return cts.SaveCSVTemplates()
	}

	data, err := os.ReadFile(cts.filename)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, cts)
	if err != nil {
		return err
	}

	// Initialize NextId if not set or calculate from existing templates
	if cts.NextId == 0 {
		cts.NextId = cts.calculateNextTemplateId()
	}

	return nil
}

// SaveCSVTemplates saves CSV templates to the JSON file
func (cts *CSVTemplateStore) SaveCSVTemplates() error {
	data, err := json.MarshalIndent(cts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cts.filename, data, 0644)
}

// calculateNextTemplateId calculates the next available template ID
func (cts *CSVTemplateStore) calculateNextTemplateId() int64 {
	var maxId int64 = 0
	for _, template := range cts.Templates {
		if template.Id > maxId {
			maxId = template.Id
		}
	}
	return maxId + 1
}

// GetCSVTemplates returns all CSV templates
func (cts *CSVTemplateStore) GetCSVTemplates() []types.CSVTemplate {
	return cts.Templates
}

// GetDefaultTemplate returns the name of the default template
func (cts *CSVTemplateStore) GetDefaultTemplate() string {
	return cts.Default
}

// SetDefaultTemplate sets the default template
func (cts *CSVTemplateStore) SetDefaultTemplate(templateName string) *TemplateResult {
	result := &TemplateResult{}

	// Verify template exists
	if cts.GetTemplateByName(templateName) == nil {
		result.Message = "Template not found"
		return result
	}

	cts.Default = templateName
	err := cts.SaveCSVTemplates()
	if err != nil {
		result.Message = fmt.Sprintf("Failed to save: %v", err)
		return result
	}

	result.Success = true
	result.Message = fmt.Sprintf("Default template set to '%s'", templateName)
	return result
}

// GetTemplateByName returns a CSV template by name
func (cts *CSVTemplateStore) GetTemplateByName(name string) *types.CSVTemplate {
	for _, template := range cts.Templates {
		if template.Name == name {
			return &template
		}
	}
	return nil
}

// GetTemplateById returns a CSV template by its ID
func (cts *CSVTemplateStore) GetTemplateById(id int64) *types.CSVTemplate {
	for _, template := range cts.Templates {
		if template.Id == id {
			return &template
		}
	}
	return nil
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

	// Check for duplicates
	for _, existing := range cts.Templates {
		if existing.Name == template.Name {
			result.Message = "Template name already exists"
			return result
		}
	}

	// Validate column indices
	if template.DateColumn < 0 || template.AmountColumn < 0 || template.DescColumn < 0 {
		result.Message = "Column indices must be non-negative"
		return result
	}

	// Assign unique ID to template
	template.Id = cts.NextId
	cts.NextId++

	// Add template and set as default
	cts.Templates = append(cts.Templates, template)
	cts.Default = template.Name

	err := cts.SaveCSVTemplates()
	if err != nil {
		result.Message = fmt.Sprintf("Failed to save template: %v", err)
		return result
	}

	result.Success = true
	result.Message = fmt.Sprintf("Template '%s' created and set as default", template.Name)
	return result
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
	maxColumn := template.DateColumn
	if template.AmountColumn > maxColumn {
		maxColumn = template.AmountColumn
	}
	if template.DescColumn > maxColumn {
		maxColumn = template.DescColumn
	}
	if template.MerchantColumn != nil && *template.MerchantColumn > maxColumn {
		maxColumn = *template.MerchantColumn
	}

	if len(fields) <= maxColumn {
		return nil, fmt.Errorf("line %d: insufficient columns (%d), need at least %d", lineNum, len(fields), maxColumn+1)
	}

	// Extract date from specified column
	transaction.Date = strings.Trim(fields[template.DateColumn], "\"")

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

	// Use provided default category
	transaction.CategoryId = defaultCategoryId

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
