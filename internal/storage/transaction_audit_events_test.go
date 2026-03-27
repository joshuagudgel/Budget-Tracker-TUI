package storage

import (
	"strings"
	"testing"
	"time"

	"budget-tracker-tui/internal/database"
	"budget-tracker-tui/internal/types"
)

// setupTestTransactionAuditStore creates a complete Store with test dependencies for transaction audit testing
func setupTestTransactionAuditStore(t *testing.T) (*Store, *database.Connection) {
	t.Helper()

	conn := setupTestDB(t)

	// Create a complete store setup like production
	store := &Store{}
	store.db = conn

	// Initialize domain stores
	store.Categories = NewCategoryStore(conn)
	store.Templates = NewCSVTemplateStore(conn)
	store.Statements = NewBankStatementStore(conn)
	store.Transactions = NewTransactionStore(conn)
	store.TransactionAudits = NewTransactionAuditStore(conn)

	// Set cross-references between stores like production
	store.Transactions.SetTransactionAuditStore(store.TransactionAudits)
	store.Transactions.SetStore(store)
	store.Categories.SetTransactionStore(store.Transactions)

	// Initialize CSV parser with dependencies (no ML for tests)
	store.CSVParser = NewCSVParser(store.Transactions, store.Categories, nil)

	return store, conn
}

// Test fixture helpers for transaction audit events

// createTestTransactionAuditEvent creates a test audit event and returns its ID
func createTestTransactionAuditEvent(t *testing.T, store *Store, transactionId, bankStatementId int64, actionType, source string) int64 {
	t.Helper()

	event := &types.TransactionAuditEvent{
		TransactionId:          transactionId,
		BankStatementId:        bankStatementId,
		ActionType:             actionType,
		Source:                 source,
		DescriptionFingerprint: "test_description",
		CategoryAssigned:       1,
		CategoryConfidence:     0.85,
		PreviousCategory:       2,
		ModificationReason:     stringPtr("category"),
		PreEditSnapshot:        stringPtr(`{"description":"before"}`),
		PostEditSnapshot:       stringPtr(`{"description":"after"}`),
		Timestamp:              time.Now(),
	}

	err := store.TransactionAudits.RecordEvent(event)
	if err != nil {
		t.Fatalf("Failed to create test transaction audit event: %v", err)
	}

	return event.Id
}

// stringPtr returns a pointer to a string (helper for optional fields)
func stringPtr(s string) *string {
	return &s
}

// assertTransactionAuditEventEqual compares two TransactionAuditEvent structs
func assertTransactionAuditEventEqual(t *testing.T, expected, actual types.TransactionAuditEvent, msg string) {
	t.Helper()

	if expected.TransactionId != actual.TransactionId {
		t.Errorf("%s: TransactionId mismatch. Expected: %d, Got: %d", msg, expected.TransactionId, actual.TransactionId)
	}
	if expected.BankStatementId != actual.BankStatementId {
		t.Errorf("%s: BankStatementId mismatch. Expected: %d, Got: %d", msg, expected.BankStatementId, actual.BankStatementId)
	}
	if expected.ActionType != actual.ActionType {
		t.Errorf("%s: ActionType mismatch. Expected: %s, Got: %s", msg, expected.ActionType, actual.ActionType)
	}
	if expected.Source != actual.Source {
		t.Errorf("%s: Source mismatch. Expected: %s, Got: %s", msg, expected.Source, actual.Source)
	}
	if expected.DescriptionFingerprint != actual.DescriptionFingerprint {
		t.Errorf("%s: DescriptionFingerprint mismatch. Expected: %s, Got: %s", msg, expected.DescriptionFingerprint, actual.DescriptionFingerprint)
	}
	if expected.CategoryAssigned != actual.CategoryAssigned {
		t.Errorf("%s: CategoryAssigned mismatch. Expected: %d, Got: %d", msg, expected.CategoryAssigned, actual.CategoryAssigned)
	}
}

// TestRecordEvent tests the RecordEvent method
func TestRecordEvent(t *testing.T) {
	store, conn := setupTestTransactionAuditStore(t)
	defer teardownTestDB(t, conn)

	// Create test data
	categoryId := createTestCategory(t, conn, "Test Category")
	statementId := createTestBankStatement(t, conn, "test.csv")

	// Create and save a test transaction
	tx := createTestTransaction(100.00, "Test Transaction", categoryId)
	tx.StatementId = statementId
	err := store.Transactions.SaveTransaction(tx)
	if err != nil {
		t.Fatalf("Failed to save test transaction: %v", err)
	}

	// Get the saved transaction to obtain the ID
	savedTxs, err := store.Transactions.GetTransactionsByStatement(statementId)
	if err != nil {
		t.Fatalf("Failed to retrieve saved transaction: %v", err)
	}
	if len(savedTxs) == 0 {
		t.Fatal("No transactions found after saving")
	}
	transactionId := savedTxs[0].Id

	tests := []struct {
		name        string
		event       *types.TransactionAuditEvent
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid edit event",
			event: &types.TransactionAuditEvent{
				TransactionId:          transactionId,
				BankStatementId:        statementId,
				ActionType:             types.ActionTypeEdit,
				Source:                 types.SourceUser,
				DescriptionFingerprint: "edit_test_description",
				CategoryAssigned:       categoryId,
				CategoryConfidence:     0.95,
				PreviousCategory:       1, // Use default "Uncategorized" category
				ModificationReason:     stringPtr(types.ModReasonCategory),
				PreEditSnapshot:        stringPtr(`{"category":"old"}`),
				PostEditSnapshot:       stringPtr(`{"category":"new"}`),
			},
			expectError: false,
		},
		{
			name: "Valid import event",
			event: &types.TransactionAuditEvent{
				TransactionId:          transactionId,
				BankStatementId:        statementId,
				ActionType:             types.ActionTypeImport,
				Source:                 types.SourceImport,
				DescriptionFingerprint: "import_test_description",
				CategoryAssigned:       categoryId,
				CategoryConfidence:     0.75,
				PreviousCategory:       1, // Use default "Uncategorized" category
			},
			expectError: false,
		},
		{
			name: "Valid split event",
			event: &types.TransactionAuditEvent{
				TransactionId:          transactionId,
				BankStatementId:        statementId,
				ActionType:             types.ActionTypeSplit,
				Source:                 types.SourceUser,
				DescriptionFingerprint: "split_test_description",
				CategoryAssigned:       categoryId,
				PreviousCategory:       1, // Use default "Uncategorized" category
				ModificationReason:     stringPtr(types.ModReasonCategory),
			},
			expectError: false,
		},
		{
			name: "Invalid action type",
			event: &types.TransactionAuditEvent{
				TransactionId:          transactionId,
				BankStatementId:        statementId,
				ActionType:             "invalid_action",
				Source:                 types.SourceUser,
				DescriptionFingerprint: "invalid_test",
				CategoryAssigned:       categoryId,
			},
			expectError: true,
			errorMsg:    "validation failed",
		},
		{
			name: "Missing required fields",
			event: &types.TransactionAuditEvent{
				TransactionId:   transactionId,
				BankStatementId: statementId,
				// Missing ActionType, Source, DescriptionFingerprint
				CategoryAssigned: categoryId,
			},
			expectError: true,
			errorMsg:    "validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set timestamp if not provided
			if tt.event.Timestamp.IsZero() {
				tt.event.Timestamp = time.Now()
			}

			err := store.TransactionAudits.RecordEvent(tt.event)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				} else {
					// Verify the event was created with an ID
					if tt.event.Id == 0 {
						t.Errorf("Expected event ID to be set after creation")
					}

					// Verify timestamp was set if not provided
					if tt.event.Timestamp.IsZero() {
						t.Errorf("Expected timestamp to be set automatically")
					}
				}
			}
		})
	}
}

// TestGetCategoryEditEvents tests the GetCategoryEditEvents method for ML training data
func TestGetCategoryEditEvents(t *testing.T) {
	store, conn := setupTestTransactionAuditStore(t)
	defer teardownTestDB(t, conn)

	// Create test data
	categoryId := createTestCategory(t, conn, "Test Category")
	statementId := createTestBankStatement(t, conn, "test.csv")

	// Create and save test transactions
	tx1 := createTestTransaction(100.00, "Transaction 1", categoryId)
	tx1.StatementId = statementId
	err := store.Transactions.SaveTransaction(tx1)
	if err != nil {
		t.Fatalf("Failed to save test transaction 1: %v", err)
	}

	tx2 := createTestTransaction(200.00, "Transaction 2", categoryId)
	tx2.StatementId = statementId
	err = store.Transactions.SaveTransaction(tx2)
	if err != nil {
		t.Fatalf("Failed to save test transaction 2: %v", err)
	}

	tx3 := createTestTransaction(300.00, "Transaction 3", categoryId)
	tx3.StatementId = statementId
	err = store.Transactions.SaveTransaction(tx3)
	if err != nil {
		t.Fatalf("Failed to save test transaction 3: %v", err)
	}

	// Get the saved transactions to obtain their IDs
	savedTxs, err := store.Transactions.GetTransactionsByStatement(statementId)
	if err != nil {
		t.Fatalf("Failed to retrieve saved transactions: %v", err)
	}
	if len(savedTxs) < 3 {
		t.Fatalf("Expected at least 3 transactions, got %d", len(savedTxs))
	}

	// Map transactions by description to get stable IDs
	txMap := make(map[string]int64)
	for _, tx := range savedTxs {
		txMap[tx.Description] = tx.Id
	}

	transactionId1 := txMap["Transaction 1"]
	transactionId2 := txMap["Transaction 2"]
	transactionId3 := txMap["Transaction 3"]

	// Create various audit events - only some should be returned by GetCategoryEditEvents
	validEditEvent1 := &types.TransactionAuditEvent{
		TransactionId:          transactionId1,
		BankStatementId:        statementId,
		ActionType:             types.ActionTypeEdit,
		Source:                 types.SourceUser,
		DescriptionFingerprint: "valid_edit_1",
		CategoryAssigned:       categoryId,
		PreviousCategory:       1, // Use default "Uncategorized" category
		ModificationReason:     stringPtr(types.ModReasonCategory),
		Timestamp:              time.Now().Add(-2 * time.Hour),
	}

	validEditEvent2 := &types.TransactionAuditEvent{
		TransactionId:          transactionId2,
		BankStatementId:        statementId,
		ActionType:             types.ActionTypeEdit,
		Source:                 types.SourceUser,
		DescriptionFingerprint: "valid_edit_2",
		CategoryAssigned:       categoryId,
		PreviousCategory:       1, // Use default "Uncategorized" category
		ModificationReason:     stringPtr(types.ModReasonCategory),
		Timestamp:              time.Now().Add(-1 * time.Hour),
	}

	// This should NOT be returned - wrong action type
	importEvent := &types.TransactionAuditEvent{
		TransactionId:          transactionId3,
		BankStatementId:        statementId,
		ActionType:             types.ActionTypeImport,
		Source:                 types.SourceImport,
		DescriptionFingerprint: "import_event",
		CategoryAssigned:       categoryId,
		PreviousCategory:       1, // Use default "Uncategorized" category
	}

	// This should NOT be returned - wrong source
	autoEvent := &types.TransactionAuditEvent{
		TransactionId:          transactionId1,
		BankStatementId:        statementId,
		ActionType:             types.ActionTypeEdit,
		Source:                 types.SourceAuto,
		DescriptionFingerprint: "auto_event",
		CategoryAssigned:       categoryId,
		PreviousCategory:       1, // Use default "Uncategorized" category
		ModificationReason:     stringPtr(types.ModReasonCategory),
	}

	// This should NOT be returned - wrong modification reason
	descriptionEvent := &types.TransactionAuditEvent{
		TransactionId:          transactionId2,
		BankStatementId:        statementId,
		ActionType:             types.ActionTypeEdit,
		Source:                 types.SourceUser,
		DescriptionFingerprint: "description_event",
		CategoryAssigned:       categoryId,
		PreviousCategory:       1, // Use default "Uncategorized" category
		ModificationReason:     stringPtr(types.ModReasonDescription),
	}

	// Record all events
	events := []*types.TransactionAuditEvent{
		validEditEvent1, validEditEvent2, importEvent, autoEvent, descriptionEvent,
	}
	for _, event := range events {
		err := store.TransactionAudits.RecordEvent(event)
		if err != nil {
			t.Fatalf("Failed to record event: %v", err)
		}
	}

	// Test GetCategoryEditEvents
	categoryEditEvents, err := store.TransactionAudits.GetCategoryEditEvents()
	if err != nil {
		t.Fatalf("Failed to get category edit events: %v", err)
	}

	// Should only return the 2 valid edit events
	expectedCount := 2
	if len(categoryEditEvents) != expectedCount {
		t.Errorf("Expected %d category edit events, got %d", expectedCount, len(categoryEditEvents))
	}

	// Verify the events are ordered by timestamp ASC (oldest first)
	if len(categoryEditEvents) >= 2 {
		if categoryEditEvents[0].Timestamp.After(categoryEditEvents[1].Timestamp) {
			t.Errorf("Events should be ordered by timestamp ASC, but first event is after second")
		}
	}

	// Verify the correct events were returned
	found := make(map[int64]bool)
	for _, event := range categoryEditEvents {
		found[event.TransactionId] = true

		// Verify it matches our criteria
		if event.ActionType != types.ActionTypeEdit {
			t.Errorf("Expected ActionType %s, got %s", types.ActionTypeEdit, event.ActionType)
		}
		if event.Source != types.SourceUser {
			t.Errorf("Expected Source %s, got %s", types.SourceUser, event.Source)
		}
		if event.ModificationReason == nil || *event.ModificationReason != types.ModReasonCategory {
			t.Errorf("Expected ModificationReason %s, got %v", types.ModReasonCategory, event.ModificationReason)
		}
	}

	// Verify exactly the expected transactions are included
	if !found[transactionId1] || !found[transactionId2] {
		t.Errorf("Expected events for transactions %d and %d to be returned", transactionId1, transactionId2)
	}
	if found[transactionId3] {
		t.Errorf("Did not expect event for transaction %d to be returned", transactionId3)
	}
}

// TestGetCategoryEditEvents_EmptyResult tests GetCategoryEditEvents when no matching events exist
func TestGetCategoryEditEvents_EmptyResult(t *testing.T) {
	store, conn := setupTestTransactionAuditStore(t)
	defer teardownTestDB(t, conn)

	// No events created - should return empty slice, not error
	events, err := store.TransactionAudits.GetCategoryEditEvents()
	if err != nil {
		t.Errorf("Expected no error for empty result, got: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("Expected 0 events, got %d", len(events))
	}
}
