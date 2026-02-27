package storage

import (
	"budget-tracker-tui/internal/types"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"
)

// TransactionStore handles all transaction-related operations
type TransactionStore struct {
	filename     string
	backupName   string
	transactions []types.Transaction
	nextId       int64
	sharedUtils  SharedUtilsInterface
}

// NewTransactionStore creates a new TransactionStore instance
func NewTransactionStore(transactionFile, backupFile string, sharedUtils SharedUtilsInterface) *TransactionStore {
	return &TransactionStore{
		filename:     transactionFile,
		backupName:   backupFile,
		transactions: []types.Transaction{},
		nextId:       1,
		sharedUtils:  sharedUtils,
	}
}

// LoadTransactions loads transactions from the JSON file
func (ts *TransactionStore) LoadTransactions() error {
	if _, err := os.Stat(ts.filename); os.IsNotExist(err) {
		ts.transactions = []types.Transaction{}
		ts.nextId = 1
		return nil
	}

	data, err := os.ReadFile(ts.filename)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &ts.transactions)
	if err != nil {
		return err
	}
	ts.nextId = ts.CalculateNextId()
	return nil
}

// SaveTransactions saves transactions to the JSON file
func (ts *TransactionStore) SaveTransactions() error {
	data, err := json.MarshalIndent(ts.transactions, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ts.filename, data, 0644)
}

// CalculateNextId calculates the next available ID
func (ts *TransactionStore) CalculateNextId() int64 {
	var maxId int64 = 0
	for _, tx := range ts.transactions {
		if tx.Id > maxId {
			maxId = tx.Id
		}
	}
	return maxId + 1
}

// GetTransactions returns all transactions
func (ts *TransactionStore) GetTransactions() ([]types.Transaction, error) {
	return ts.transactions, nil
}

// GetTransactionsByStatement returns all transactions for a specific bank statement
func (ts *TransactionStore) GetTransactionsByStatement(statementId int64) ([]types.Transaction, error) {
	var filteredTransactions []types.Transaction

	statementIdStr := fmt.Sprintf("%d", statementId)
	for _, tx := range ts.transactions {
		if tx.StatementId == statementIdStr {
			filteredTransactions = append(filteredTransactions, tx)
		}
	}

	return filteredTransactions, nil
}

// GetTransactionByID returns a transaction by ID
func (ts *TransactionStore) GetTransactionByID(id int64) *types.Transaction {
	for i := range ts.transactions {
		if ts.transactions[i].Id == id {
			return &ts.transactions[i]
		}
	}
	return nil
}

// SaveTransaction saves or updates a transaction
func (ts *TransactionStore) SaveTransaction(transaction types.Transaction) error {
	found := false
	for i, t := range ts.transactions {
		if t.Id == transaction.Id {
			// Set UpdatedAt for existing transactions and mark as user modified
			transaction.UpdatedAt = time.Now().Format(time.RFC3339)
			transaction.UserModified = true
			ts.transactions[i] = transaction
			found = true
			break
		}
	}

	if !found {
		if transaction.Id == 0 {
			transaction.Id = ts.nextId
			ts.nextId++
		}
		// Set CreatedAt and UpdatedAt for new transactions
		now := time.Now().Format(time.RFC3339)
		if transaction.CreatedAt == "" {
			transaction.CreatedAt = now
		}
		transaction.UpdatedAt = now
		// Default confidence to 0.0 if not set
		if transaction.Confidence == 0.0 {
			transaction.Confidence = 0.0
		}
		ts.transactions = append(ts.transactions, transaction)
	}

	return ts.SaveTransactions()
}

// DeleteTransaction removes a transaction by ID
func (ts *TransactionStore) DeleteTransaction(id int64) error {
	for i, transaction := range ts.transactions {
		if transaction.Id == id {
			ts.transactions = append(ts.transactions[:i], ts.transactions[i+1:]...)
			break
		}
	}
	return ts.SaveTransactions()
}

// SplitTransaction splits a parent transaction into two transactions
func (ts *TransactionStore) SplitTransaction(parentId int64, splits []types.Transaction) error {
	// Validate splits add up to parent amount (works for negative amounts)
	parent := ts.GetTransactionByID(parentId)
	if parent == nil {
		return fmt.Errorf("parent transaction not found")
	}

	if len(splits) != 2 {
		return fmt.Errorf("exactly 2 splits required")
	}

	var totalSplit float64
	for _, split := range splits {
		totalSplit += split.Amount
	}

	// Use epsilon comparison for floating point
	if math.Abs(totalSplit-parent.Amount) > 0.01 {
		return fmt.Errorf("split amounts (%.2f) don't match parent (%.2f)",
			totalSplit, parent.Amount)
	}

	// Modify existing transaction to become first split
	parent.Amount = splits[0].Amount
	parent.Description = splits[0].Description
	parent.CategoryId = splits[0].CategoryId
	parent.IsSplit = true
	parent.UpdatedAt = time.Now().Format(time.RFC3339)
	parent.UserModified = true

	// Create only the second split as a new transaction
	secondSplit := splits[1]
	secondSplit.Id = ts.nextId
	secondSplit.Date = parent.Date                       // Ensure same date as original
	secondSplit.TransactionType = parent.TransactionType // Ensure same type
	secondSplit.StatementId = parent.StatementId         // Inherit statement ID from parent
	now := time.Now().Format(time.RFC3339)
	secondSplit.CreatedAt = now
	secondSplit.UpdatedAt = now
	secondSplit.Confidence = 0.0
	ts.nextId++

	ts.transactions = append(ts.transactions, secondSplit)

	return ts.SaveTransactions()
}

// ImportTransactionsFromCSV imports a batch of transactions from CSV parsing
func (ts *TransactionStore) ImportTransactionsFromCSV(transactions []types.Transaction, statementId string) error {
	// Set StatementId on all transactions and assign IDs
	for i := range transactions {
		transactions[i].Id = ts.nextId
		ts.nextId++
		transactions[i].StatementId = statementId

		// Set default transaction type if not set
		if transactions[i].TransactionType == "" {
			transactions[i].TransactionType = "expense"
		}

		// Set timestamps for imported transactions
		now := time.Now().Format(time.RFC3339)
		if transactions[i].CreatedAt == "" {
			transactions[i].CreatedAt = now
		}
		transactions[i].UpdatedAt = now

		// Default confidence to 0.0 if not set
		if transactions[i].Confidence == 0.0 {
			transactions[i].Confidence = 0.0
		}
	}

	// Add transactions to store
	ts.transactions = append(ts.transactions, transactions...)

	return ts.SaveTransactions()
}

// CreateBackup creates a backup of current transactions
func (ts *TransactionStore) CreateBackup() error {
	// Convert transactions to backup format
	var backupTransactions []BackupTransaction
	for _, tx := range ts.transactions {
		backupTx := BackupTransaction{
			Amount:          tx.Amount,
			Description:     tx.Description,
			Date:            tx.Date,
			Category:        "", // Would need category display name from CategoryStore
			TransactionType: tx.TransactionType,
		}
		backupTransactions = append(backupTransactions, backupTx)
	}

	backup := BackupFile{
		Transactions: backupTransactions,
	}

	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ts.backupName, data, 0644)
}

// RestoreFromBackup restores transactions from a backup file
func (ts *TransactionStore) RestoreFromBackup() (*RestoreResult, error) {
	result := &RestoreResult{}

	// Read backup file
	data, err := os.ReadFile(ts.backupName)
	if err != nil {
		result.Message = fmt.Sprintf("Cannot read backup file: %v", err)
		return result, err
	}

	// Get backup file stats for metadata
	fileInfo, _ := os.Stat(ts.backupName)
	if fileInfo != nil {
		result.BackupDate = fileInfo.ModTime().Format("2006-01-02 15:04:05")
		result.BackupSize = fileInfo.Size()
	}

	var backup BackupFile
	err = json.Unmarshal(data, &backup)
	if err != nil {
		result.Message = fmt.Sprintf("Invalid backup format: %v", err)
		return result, err
	}

	if len(backup.Transactions) == 0 {
		result.Message = "Backup file contains no transactions"
		return result, fmt.Errorf("empty backup file")
	}

	// Convert backup transactions to current format
	var newTransactions []types.Transaction
	now := time.Now().Format(time.RFC3339)

	for i, backupTx := range backup.Transactions {
		// Note: CategoryId will need to be resolved by the calling Store
		// using CategoryStore.GetCategoryByDisplayName(backupTx.Category)
		transaction := types.Transaction{
			Id:              int64(i + 1),
			Amount:          backupTx.Amount,
			Description:     backupTx.Description,
			RawDescription:  backupTx.Description,
			Date:            backupTx.Date,
			CategoryId:      0, // Will be set by calling Store
			TransactionType: backupTx.TransactionType,
			CreatedAt:       now,
			UpdatedAt:       now,
			Confidence:      0.0,
		}

		// Set payment method if available (legacy field)
		if backupTx.PaymentMethod != "" {
			// Could map to a category or store in description
			transaction.Description = fmt.Sprintf("%s (%s)", transaction.Description, backupTx.PaymentMethod)
		}

		newTransactions = append(newTransactions, transaction)
	}

	// Replace current transactions
	ts.transactions = newTransactions
	ts.nextId = int64(len(newTransactions) + 1)

	err = ts.SaveTransactions()
	if err != nil {
		result.Message = fmt.Sprintf("Failed to save restored transactions: %v", err)
		return result, err
	}

	result.Success = true
	result.TxCount = len(newTransactions)
	result.Message = fmt.Sprintf("Successfully restored %d transactions from backup", len(newTransactions))
	return result, nil
}

// Backup-related types for TransactionStore
type BackupTransaction struct {
	Amount          float64 `json:"amount"`
	Description     string  `json:"description"`
	Date            string  `json:"date"`
	Category        string  `json:"category"`
	PaymentMethod   string  `json:"paymentMethod"`
	TransactionType string  `json:"transactionType"`
}

type BackupFile struct {
	Transactions []BackupTransaction `json:"transactions"`
}
