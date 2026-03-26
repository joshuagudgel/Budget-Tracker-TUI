package storage

import (
	"time"

	"budget-tracker-tui/internal/ml"
	"budget-tracker-tui/internal/types"
)

// TransactionStoreInterface defines the contract for transaction operations
type TransactionStoreInterface interface {
	// CRUD Operations
	GetTransactions() ([]types.Transaction, error)
	GetTransactionsByStatement(statementId int64) ([]types.Transaction, error)
	GetTransactionByID(id int64) *types.Transaction
	SaveTransaction(transaction types.Transaction) error
	DeleteTransaction(id int64) error
	FindDuplicateTransactions(date string, amount float64, description string) ([]types.Transaction, error)

	// Bulk Operations
	ImportTransactionsFromCSV(transactions []types.Transaction, statementId int64) error

	// Split Operations
	SplitTransaction(parentId int64, splits []types.Transaction) error
}

// CategoryStoreInterface defines the contract for category operations
type CategoryStoreInterface interface {
	// CRUD Operations
	GetCategories() ([]types.Category, error)
	GetCategoryByDisplayName(displayName string) *types.Category
	GetCategoryDisplayName(categoryId int64) string
	CreateCategory(displayName string) *CategoryResult
	CreateCategoryFull(category *types.Category) error
	UpdateCategory(category *types.Category) error
	DeleteCategory(categoryId int64) error

	// Validation
	ValidateCategoryForDeletion(categoryId int64) error
	CategoryExists(categoryId int64) (bool, error)

	// Default handling
	GetDefaultCategoryId() int64
	SetDefaultCategoryId(categoryId int64) error

	// Utilities
	CalculateNextCategoryId() int64
}

// BankStatementStoreInterface defines the contract for bank statement operations
type BankStatementStoreInterface interface {
	// CRUD Operations
	GetStatementHistory() []types.BankStatement
	GetStatementByIndex(index int) (*types.BankStatement, error)
	GetStatementById(id int64) (*types.BankStatement, error)
	GetStatementDetails(index int) (types.BankStatement, bool)
	DeleteStatement(id int64) error

	// Import Operations
	ValidateAndImportCSV(filePath, templateName string) *types.ImportResult
	ImportCSVWithOverride(templateName string) *types.ImportResult
	DetectOverlap(periodStart, periodEnd string, templateId int64) []types.BankStatement
	ExtractPeriodFromTransactions(transactions []types.Transaction) (start, end string)

	// Undo Operations
	CanUndoImport(statementId int64) bool
	UndoImport(statementId int64) (int, error)
	MarkStatementFailed(statementId int64, errorMsg string) error
	MarkStatementCompleted(statementId int64) error

	// Template Integration
	GetTemplateNameById(templateId string) string
}

// CSVTemplateStoreInterface defines the contract for CSV template operations
type CSVTemplateStoreInterface interface {
	// CRUD Operations
	GetCSVTemplates() []types.CSVTemplate
	GetTemplateByName(name string) *types.CSVTemplate
	CreateCSVTemplate(template types.CSVTemplate) *TemplateResult
	DeleteCSVTemplate(id int64) *TemplateResult

	// Default handling
	GetDefaultTemplate() string
	SetDefaultTemplate(templateName string) *TemplateResult

	// Parsing Operations
	ParseCSVLine(line string, delimiter string) []string
	ParseTransactionFromTemplate(fields []string, template *types.CSVTemplate, lineNum int) (*types.Transaction, error)
	ParseCSVTransactions(filePath string, template *types.CSVTemplate) ([]types.Transaction, error)
	ParseCSVTransactionsWithDuplicateFilter(filePath string, template *types.CSVTemplate, defaultCategoryId int64) ([]types.Transaction, []types.Transaction, error)

	// Utilities
	ParseAmount(amountStr string) (float64, error)
}

// TransactionAuditStoreInterface defines the contract for transaction audit event operations
type TransactionAuditStoreInterface interface {
	// Core Operations
	RecordEvent(event *types.TransactionAuditEvent) error

	// Query Operations
	GetEventsByTransaction(transactionId int64) ([]types.TransactionAuditEvent, error)
	GetEventsByStatement(bankStatementId int64) ([]types.TransactionAuditEvent, error)
	GetEventsByTimeRange(startTime, endTime time.Time) ([]types.TransactionAuditEvent, error)
	GetEventsByActionType(actionType string) ([]types.TransactionAuditEvent, error)
	GetRecentEvents(limit int) ([]types.TransactionAuditEvent, error)

	// ML Training Data Operations
	GetCategoryEditEvents() ([]types.TransactionAuditEvent, error)
	GetImportEvents() ([]types.TransactionAuditEvent, error)
}

// MLCategorizerInterface defines the contract for ML-based transaction categorization
type MLCategorizerInterface interface {
	// Training Operations
	Train(auditEvents []types.TransactionAuditEvent, categories []types.Category) error
	GetStats() map[string]interface{}

	// Prediction Operations
	PredictCategory(description string, amount float64) ml.CategoryPrediction
	IsHighConfidence(prediction ml.CategoryPrediction) bool
}

// SharedUtilsInterface defines the contract for shared utilities
type SharedUtilsInterface interface {
	// Cross-domain Operations
	MigrateTransactionCategories() error

	// Shared parsing utilities
	ParseCSVLine(line string, delimiter string) []string
	ParseAmount(amountStr string) (float64, error)
}

// Result types for operations
type RestoreResult struct {
	Success    bool
	Message    string
	TxCount    int
	BackupDate string
	BackupSize int64
}

type CategoryResult struct {
	Success    bool
	Message    string
	CategoryId int64
}

type TemplateResult struct {
	Success bool
	Message string
}
