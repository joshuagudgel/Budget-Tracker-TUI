package ui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"budget-tracker-tui/internal/types"
)

// TransactionEditState manages UI editing state for transactions
type TransactionEditState struct {
	// Original transaction being edited
	Original *types.Transaction

	// UI input fields (what user types)
	AmountInput      string
	DescriptionInput string
	DateInput        string // MM/DD/YYYY or MM-DD-YYYY format
	CategoryId       int64
	TransactionType  string

	// Validation state
	FieldErrors   map[string]string
	IsValid       bool
	ValidationMsg string
}

// NewTransactionEditState creates a new edit state from an existing transaction
func NewTransactionEditState(tx *types.Transaction) *TransactionEditState {
	if tx == nil {
		return &TransactionEditState{
			Original:    nil,
			FieldErrors: make(map[string]string),
			IsValid:     true,
		}
	}

	return &TransactionEditState{
		Original:         tx,
		AmountInput:      fmt.Sprintf("%.2f", tx.Amount),
		DescriptionInput: tx.Description,
		DateInput:        tx.GetDateForDisplay(),
		CategoryId:       tx.CategoryId,
		TransactionType:  tx.TransactionType,
		FieldErrors:      make(map[string]string),
		IsValid:          true,
	}
}

// ToTransaction converts edit state back to a Transaction
func (es *TransactionEditState) ToTransaction() (*types.Transaction, error) {
	var tx types.Transaction

	// Copy original fields if editing existing transaction
	if es.Original != nil {
		tx = *es.Original
	}

	// Parse amount
	var err error
	if es.AmountInput != "" {
		tx.Amount, err = parseAmount(es.AmountInput)
		if err != nil {
			return nil, fmt.Errorf("amount: %w", err)
		}
	}

	// Set description
	tx.Description = strings.TrimSpace(es.DescriptionInput)

	// Parse date
	if es.DateInput != "" {
		tx.Date, err = types.TryParseMultipleDateFormats(es.DateInput)
		if err != nil {
			return nil, fmt.Errorf("date: %w", err)
		}
	}

	// Set category and type
	tx.CategoryId = es.CategoryId
	tx.TransactionType = es.TransactionType

	// Update timestamps
	now := time.Now()
	if es.Original == nil {
		tx.CreatedAt = now
	}
	tx.UpdatedAt = now

	return &tx, nil
}

// Helper method for amount parsing
func parseAmount(amountStr string) (float64, error) {
	trimmed := strings.TrimSpace(amountStr)
	if trimmed == "" {
		return 0, fmt.Errorf("amount cannot be empty")
	}

	// Remove currency symbols and commas
	cleaned := regexp.MustCompile(`[\$,]`).ReplaceAllString(trimmed, "")

	amount, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount format")
	}

	return amount, nil
}
