package types

// This file serves as a coordinator, making all types available from their organized submodules
// for backward compatibility during the transition period.

// Re-export all types by importing from organized files:
// - Domain objects (Transaction, Category, BankStatement, CSVTemplate) are in domain.go
// - Operation results (ValidationError, ValidationResult, ImportResult) are in results.go
// - Analytics types (AnalyticsSummary, CategorySpending) are in analytics.go
// - Audit types (TransactionAuditEvent, constants) are in audit.go
// - Date utilities are in types_dates.go
// - CSV parsing types are in csv_results.go

// All types are automatically available because they're in the same package.
