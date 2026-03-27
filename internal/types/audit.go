package types

import (
	"fmt"
	"time"
)

// TransactionAuditEvent tracks all interactions with transactions for ML and audit purposes
type TransactionAuditEvent struct {
	Id                     int64     `db:"id"`
	TransactionId          int64     `db:"transaction_id"`
	BankStatementId        int64     `db:"bank_statement_id"`
	Timestamp              time.Time `db:"timestamp"`
	ActionType             string    `db:"action_type"` // "edit", "import", "split"
	Source                 string    `db:"source"`      // "user", "import", "auto"
	DescriptionFingerprint string    `db:"description_fingerprint"`
	CategoryAssigned       int64     `db:"category_assigned"`
	CategoryConfidence     float64   `db:"category_confidence"` // ML prediction confidence (0.0-1.0)
	PreviousCategory       int64     `db:"previous_category"`
	ModificationReason     *string   `db:"modification_reason"` // "description", "transaction type", "category"
	PreEditSnapshot        *string   `db:"pre_edit_snapshot"`   // json transaction state
	PostEditSnapshot       *string   `db:"post_edit_snapshot"`  // json transaction state
	CreatedAt              time.Time `db:"created_at"`
}

// TransactionAuditEvent constants
const (
	// Action Types
	ActionTypeEdit   = "edit"
	ActionTypeImport = "import"
	ActionTypeSplit  = "split"

	// Source Types
	SourceUser   = "user"
	SourceAuto   = "auto"
	SourceImport = "import"

	// Modification Reasons
	ModReasonDescription     = "description"
	ModReasonTransactionType = "transaction type"
	ModReasonCategory        = "category"
)

// Legacy validation method for TransactionAuditEvent (should eventually be moved to validation package)

// Validate validates the transaction audit event and returns a ValidationResult
func (tae *TransactionAuditEvent) Validate() ValidationResult {
	result := ValidationResult{IsValid: true}

	// Validate ActionType
	if err := tae.validateActionType(); err != nil {
		result.AddError("actionType", err.Error())
	}

	// Validate Source
	if err := tae.validateSource(); err != nil {
		result.AddError("source", err.Error())
	}

	// Validate CategoryConfidence
	if err := tae.validateCategoryConfidence(); err != nil {
		result.AddError("categoryConfidence", err.Error())
	}

	return result
}

// validateActionType validates the action type field
func (tae *TransactionAuditEvent) validateActionType() error {
	if tae.ActionType == "" {
		return fmt.Errorf("action type cannot be empty")
	}

	validTypes := []string{ActionTypeEdit, ActionTypeImport, ActionTypeSplit}
	for _, validType := range validTypes {
		if tae.ActionType == validType {
			return nil
		}
	}

	return fmt.Errorf("invalid action type: %s", tae.ActionType)
}

// validateSource validates the source field
func (tae *TransactionAuditEvent) validateSource() error {
	if tae.Source == "" {
		return fmt.Errorf("source cannot be empty")
	}

	validSources := []string{SourceUser, SourceAuto, SourceImport}
	for _, validSource := range validSources {
		if tae.Source == validSource {
			return nil
		}
	}

	return fmt.Errorf("invalid source: %s", tae.Source)
}

// validateCategoryConfidence validates the category confidence field
func (tae *TransactionAuditEvent) validateCategoryConfidence() error {
	if tae.CategoryConfidence < 0.0 || tae.CategoryConfidence > 1.0 {
		return fmt.Errorf("category confidence must be between 0.0 and 1.0, got: %f", tae.CategoryConfidence)
	}
	return nil
}
