# Finance Wrapped TUI - AI Instructions

## Core Architecture

**Framework**: Bubble Tea TUI with state machine pattern
**Persistence**: SQLite located in `~/.finance-wrapped/`
**Styling**: Lipgloss with consistent field patterns

## Key Patterns

### State Machine

```go
const (
	menuView             uint = iota
	listView                  = 1
	titleView                 = 2
	bodyView                  = 3
	editView                  = 4
	backupView                = 5
	filePickerView            = 6
	csvTemplateView           = 7
	createTemplateView        = 8
	categoryView              = 9
	createCategoryView        = 10
	bulkEditView              = 11
	bankStatementView         = 12
	statementOverlapView      = 13
	categoryListView          = 14
	categoryEditView          = 15
	categoryCreateView        = 16
	undoConfirmView           = 17
	bankStatementListView     = 18  // Enhanced bank statement management
	bankStatementManageView   = 19  // Individual statement actions
)

// Nested switch in Update()
switch m.state {
case editView:
    switch key {
    case "enter": // Activate field
    case "backspace": // Activate field for editing
    case "ctrl+s": // Save
    }
}
```

### Bank Statement Workflow

**Hybrid Menu Approach**:

- **Quick Import**: Menu → 'i' → Direct import workflow (legacy)
- **Management**: Menu → 'b' → Bank Statement List → Actions
- **Alt Management**: Import View → 'h' → Bank Statement List → Actions

**Enhanced Management Features**:

- List all imported statements with status indicators
- Quick actions: 'u' for undo, 'd' for details, 'i' for new import
- Individual statement management with detailed actions
- Comprehensive undo system with confirmation dialogs
- All undo operations return to management interface

### Field Editing (Two-Phase)

1. **Navigation**: Up/Down moves between fields
2. **Editing**: Enter/Backspace activates field editing, Enter deactivates field editing, Esc cancels field editing

### Data Structures

**SQLite-focused core types** prepared for database storage with time.Time fields:

```go
type Transaction struct {
    Id              int64     `db:"id"`
    ParentId        *int64    `db:"parent_id"`
    Amount          float64   `db:"amount"`
    Description     string    `db:"description"`
    RawDescription  string    `db:"raw_description"`
    Date            time.Time `db:"date"`            // time.Time for type safety
    CategoryId      int64     `db:"category_id"`
    AutoCategory    string    `db:"auto_category"`
    TransactionType string    `db:"transaction_type"`
    IsSplit         bool      `db:"is_split"`
    IsRecurring     bool      `db:"is_recurring"`
    StatementId     string    `db:"statement_id"`
    Confidence      float64   `db:"confidence"`      // ML prediction confidence
    UserModified    bool      `db:"user_modified"`   // Track manual changes
    CreatedAt       time.Time `db:"created_at"`      // Timestamp for learning
    UpdatedAt       time.Time `db:"updated_at"`      // Track modifications
}
```

```go
// TransactionEditState manages UI editing state separate from business model
type TransactionEditState struct {
    Original         *Transaction          // Original being edited
    AmountInput      string               // User input: "$123.45"
    DescriptionInput string               // User input text
    DateInput        string               // User input: "03/15/2024" or "03-15-2024"
    CategoryId       int64                // Selected category
    TransactionType  string               // Selected type
    FieldErrors      map[string]string    // Field-specific validation errors
    IsValid          bool                 // Overall validation state
    ValidationMsg    string               // Summary validation message
}
```

```go
type Category struct {
	Id          int64     `db:"id"`
	DisplayName string    `db:"display_name"`
	ParentId    *int64    `db:"parent_id"`
	Color       string    `db:"color"`
	IsActive    bool      `db:"is_active"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}
```

```go
type BankStatement struct {
    Id             int64     `db:"id"`
    Filename       string    `db:"filename"`
    ImportDate     time.Time `db:"import_date"`
    PeriodStart    time.Time `db:"period_start"`
    PeriodEnd      time.Time `db:"period_end"`
    TemplateUsed   int64     `db:"template_used"`
    TxCount        int       `db:"tx_count"`
    Status         string    `db:"status"` // "completed", "failed", "override", "undone", "importing"
    ProcessingTime int64     `db:"processing_time"` // milliseconds
    ErrorLog       string    `db:"error_log"`       // Import issues
    CreatedAt      time.Time `db:"created_at"`
    UpdatedAt      time.Time `db:"updated_at"`
}
```

```go
type CSVTemplate struct {
    Id             int64     `db:"id"`
    Name           string    `db:"name"`
    PostDateColumn int       `db:"post_date_column"`    // Renamed from DateColumn for clarity
    AmountColumn   int       `db:"amount_column"`
    DescColumn     int       `db:"desc_column"`
    CategoryColumn *int      `db:"category_column"`     // Optional category column index
    HasHeader      bool      `db:"has_header"`
    DateFormat     string    `db:"date_format"`         // Format hint: "01/02/2006" also accepts "01-02-2006"
    MerchantColumn *int      `db:"merchant_column"`     // Optional merchant extraction
    Delimiter      string    `db:"delimiter"`           // Default ","
    CreatedAt      time.Time `db:"created_at"`
    UpdatedAt      time.Time `db:"updated_at"`
}
```

```go
type TransactionAuditEvent struct {
    Id                     int64     `db:"id"`
    TransactionId          int64     `db:"transaction_id"`
    BankStatementId        int64     `db:"bank_statement_id"`
    Timestamp              time.Time `db:"timestamp"`
    ActionType             string    `db:"action_type"`               // "edit", "import", "split"
    Source                 string    `db:"source"`                    // "user", "import", "auto"
    DescriptionFingerprint string    `db:"description_fingerprint"`
    CategoryAssigned       int64     `db:"category_assigned"`
    CategoryConfidence     float64   `db:"category_confidence"`       // ML prediction confidence (0.0-1.0)
    PreviousCategory       int64     `db:"previous_category"`         // Category before change
    ModificationReason     *string   `db:"modification_reason"`       // "description", "transaction type", "category"
    PreEditSnapshot        *string   `db:"pre_edit_snapshot"`         // JSON transaction state before edit
    PostEditSnapshot       *string   `db:"post_edit_snapshot"`        // JSON transaction state after edit
    CreatedAt              time.Time `db:"created_at"`
}
```

**TransactionAuditEvent** tracks all significant interactions with transactions for audit trails and potential ML training data:

- **Action Types**: Limited to "edit" (user modifications), "import" (CSV imports), "split" (transaction splitting)
- **Source Tracking**: Distinguishes between "user" (manual edits), "import" (CSV operations), "auto" (system actions)
- **Category History**: `PreviousCategory` field enables tracking category changes over time
- **Change Detection**: `ModificationReason` identifies what field was changed ("description", "transaction type", "category")
- **State Snapshots**: JSON snapshots capture complete transaction state before/after edits for detailed audit trails
- **Confidence Scoring**: `CategoryConfidence` stores ML prediction confidence scores (0.0-1.0)
- **Foreign Key Constraints**: Links to transactions, bank statements, and categories with proper cascade/restrict rules

**Usage Patterns**:

- Created automatically on transaction edits (captures old→new category changes)
- Generated during transaction splitting operations
- **Created during CSV imports (tracks ML predictions and confidence scores)**
- Provides audit trail for user actions and system changes

### Validation System

**Location**: `internal/validation/` package with types in `internal/types/types.go`

#### Core Types

```go
type ValidationError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}

type ValidationResult struct {
    IsValid bool               `json:"isValid"`
    Errors  []ValidationError `json:"errors"`
}

// Methods: AddError(), HasError(), GetError()
```

#### Transaction Validation Methods

```go
// Built into Transaction struct
func (t *Transaction) Validate(availableCategories []string) ValidationResult
func (t *Transaction) ValidateField(field string, availableCategories []string) error

// Using TransactionValidator
validator := validation.NewTransactionValidator()
result := validator.ValidateTransaction(tx, categories)
bulkResults := validator.ValidateBulkEdit(transactions, categories)
```

#### Validation Rules

- **Amount**: Non-zero, max 2 decimal places, supports currency parsing (`$123.45`)
- **Date**: Flexible input formats (mm-dd-yyyy, mm/dd/yyyy, yyyy-mm-dd, etc.), normalized to ISO 8601 (YYYY-MM-DD) for storage
- **Description**: 1-255 characters, non-empty after trim
- **Category**: Must exist in provided categories list (case-insensitive)

#### Individual Validators

```go
validation.AmountValidator{}     // .Validate(), .ParseAmount()
validation.DateValidator{}       // .Validate(), .ParseDate()
validation.DescriptionValidator{} // .Validate()
validation.CategoryValidator{}   // .Validate(), .GetSuggestions()
```

#### Integration Patterns

- **Real-time**: Use `ValidateField()` during field editing
- **Form submission**: Use `Validate()` before saving
- **Bulk operations**: Use `ValidateBulkEdit()` for multi-select
- **Error display**: Use `GetUserFriendlyMessage()` for TUI messages
- **Category autocomplete**: Use `GetSuggestions()` for dropdown filtering

#### UI Integration

```go
// Model validation state
fieldErrors           map[string]string
hasValidationErrors   bool
validationNotification string
validator             *validation.TransactionValidator

// Validation methods in model
validateCurrentTransaction()    // Real-time validation for edit view
validateBulkEditData()         // Bulk edit validation
validateSplitTransaction()     // Split transaction validation
buildValidationNotification()   // User-friendly error messages
clearFieldError(field string)  // Clear specific field errors

// View helper methods
renderValidationNotification() string                    // Error notification component
getFieldStyle(field, isActive, isEditing) lipgloss.Style // Validation-aware styling
```

### Styling Convention

```go
formFieldStyle      // Default field border
activeFieldStyle    // Blue border (selected)
selectingFieldStyle // Orange border (editing)

successStyle       // Green background for success
```

### UI Validation Integration

**Field State Priority**: Editing > Active > Default
**Error Display**: Notification area only + inline messages below fields
**Save Prevention**: Disabled save when `hasValidationErrors = true`
**Views**: Integrated in EditView, BulkEditView, and SplitView

### Date Handling Standards

**Storage Convention**: All transaction and statement dates use **ISO 8601 format (YYYY-MM-DD)** in SQLite database. Timestamps use **RFC3339 format** for precision and timezone information.

**CSV Input Flexibility**: CSV imports accept multiple date format variations and automatically convert to ISO 8601:

- MM/DD/YYYY and MM-DD-YYYY (US formats)
- DD/MM/YYYY and DD-MM-YYYY (European formats)
- YYYY-MM-DD and YYYY/MM/DD (ISO formats)
- M/D/YYYY or D/M/YYYY (shortened numerical)

**Key Feature**: Template format acts as hint - if template specifies MM/DD/YYYY, import also accepts MM-DD-YYYY (slash/dash variants).

**Import Flow Integration**:

```go
// 1. ValidateCSVData: Pre-validation with flexible formats
//    - accepts multiple date format variations
//    - rejects entire import if ANY date unparseable
//    - returns line numbers for debugging

// 2. parseTransactionFromTemplate: Date normalization
//    - calls types.NormalizeDateToISO8601(csvDate, template.DateFormat)
//    - stores normalized dates as YYYY-MM-DD

// 3. ExtractPeriodFromTransactions: Simplified comparison
//    - ISO 8601 dates sort correctly as strings
//    - no parsing needed, just string comparison
```

**Utility Functions**:

- `types.TryParseMultipleDateFormats()` - Parse with multiple format attempts
- `types.NormalizeDateToISO8601()` - Convert any format to YYYY-MM-DD
- `cts.getDateFormatsToTry()` - Generate slash/dash format variants

## Current Features

- ✅ **Transaction CRUD**: List, edit, delete, split transactions
- ✅ **Multi-select**: Bulk edit multiple transactions with 'e'
- ✅ **CSV Import**: Template-based bank statement import with overlap detection
- ✅ **CSV Template Management**: Enhanced template creation, editing, and deletion with validation
  - **Two-Phase Editing**: Navigation mode (blue highlight) + editing mode (orange highlight)
  - **Real-time Validation**: Field validation with inline error messages
  - **Column Index Validation**: Prevents duplicate column assignments
  - **Deletion Safety**: Cannot delete templates used by existing bank statements
  - **Optional Category Column**: Support for category column in CSV imports
- ✅ **Categories**: User-defined categories with default selection
- ✅ **Split Transactions**: Modify original + create second transaction
- ✅ **Validation System**: Comprehensive field validation with real-time feedback
  - **Pre-import CSV Validation**: All-or-nothing validation before import
  - **Flexible Date Acceptance**: Multiple format support with automatic normalization
  - **Line-numbered Error Reporting**: Shows first 5 errors with CSV line numbers
- ✅ **Bank Statement Management**: Enhanced management interface with undo functionality
  - **Hybrid Menu Approach**: Both quick import ('i') and full management ('b')
  - **Management Interface**: List view with navigation, actions, and status indicators
  - **Undo System**: Complete undo functionality with confirmation dialogs
  - **Status Tracking**: Visual indicators for completed/failed/undone imports
- ✅ **ML-Enhanced Categorization**: Intelligent automatic transaction categorization
  - **Text Similarity**: Uses Jaccard similarity for description matching against historical edits
  - **Training from User Behavior**: Learns from manual category corrections in audit events
  - **Confidence-Based Decisions**: High-confidence predictions (>0.7) auto-assign, low-confidence falls back
  - **Audit Trail Integration**: All ML predictions tracked with confidence scores
  - **Self-Improving**: Gets better over time as users make corrections
- ✅ **Comprehensive Test Coverage**: Robust testing infrastructure with 78.4% coverage
  - **100% Critical Workflow Testing**: All primary import and coordination workflows tested
  - **Integration Testing**: Real CSV files, database operations, cross-domain coordination
  - **ML Testing**: Category prediction and confidence validation
  - **Error Scenario Coverage**: Validation failures, parsing errors, edge cases
  - **Atomic Import Validation**: Complete transaction safety and rollback testing

## Implementation Patterns

### Input Validation

**Comprehensive validation system** in `internal/validation/`:

- **Real-time validation**: Field-level validation during editing
- **Form validation**: Complete transaction validation before save
- **Bulk validation**: Multi-transaction validation support
- **Error handling**: User-friendly error messages with suggestions
- **Parsing support**: Amount parsing with currency symbols, date normalization

**Legacy patterns** (deprecated):

- Amount fields: Validate digits, decimal points, max 2 decimal places
- Text fields: Free-form input with Enter to confirm
- Dropdowns: Up/Down navigation, Enter to select

### Error Handling

- **Validation errors**: Real-time field validation with inline error messages
- **User-friendly messages**: Via validation notification system and field-specific errors
- **Save prevention**: Disabled save operations when validation errors exist
- **Graceful fallbacks**: For edge cases in data parsing and validation
- **Legacy messages**: via `splitMessage`, `statementMessage` etc. for non-validation errors

### ML Categorization System

**Location**: `internal/ml/categorizer.go` with integration in `internal/storage/`

#### Core Architecture

```go
type EmbeddingsCategorizer struct {
    trainingExamples       []TrainingExample  // User edit history for learning
    availableCategories    []types.Category   // Available categories
    minConfidenceThreshold float64           // Default 0.7, configurable
}

type CategoryPrediction struct {
    CategoryId   int64   // Predicted category
    Confidence   float64 // 0.0-1.0 confidence score
    ReasonCode   string  // "exact_match", "similarity", "fallback"
    SimilarityTo string  // Description of most similar historical transaction
}
```

#### Training Pipeline

1. **Startup Training**: ML categorizer automatically trains from `TransactionAuditEvent` records where:
   - `ActionType = "edit"` AND `Source = "user"` AND `ModificationReason = "category"`
   - Extracts {description, old_category, new_category} as labeled training data

2. **Learning from Corrections**: User manual category edits create new audit events → ML improves over time

3. **In-Memory Model**: Trains on startup, no persistence needed (lightweight text similarity)

#### Prediction Flow

**CSV Import Integration** (`parseTransactionFromTemplate`):

```go
// 1. Generate ML prediction for transaction description
prediction := store.PredictCategory(description, amount)

// 2. ML-first decision logic
if prediction.Confidence >= 0.7 {
    // Use ML prediction (prioritizes learning)
    categoryId = prediction.CategoryId
} else if csvHasCategory {
    // Fallback to CSV category
    categoryId = ResolveCategoryFromCSV(categoryText)
} else {
    // Final fallback to default
    categoryId = defaultCategoryId
}

// 3. Create audit event with ML confidence
auditEvent.CategoryConfidence = prediction.Confidence
```

#### Similarity Algorithm

- **Text Normalization**: Lowercase, trim whitespace, remove common prefixes ("POS ", "PURCHASE ")
- **Jaccard Similarity**: Word-based set intersection/union for description matching
- **Exact Match Bonus**: High confidence (0.95+) for previously seen descriptions
- **Amount Weighting**: Secondary factor for amount-based similarity (configurable)

#### Integration Points

- **Store Initialization**: `initializeMLCategorizer()` called during `store.Init()`
- **CSV Import**: ML predictions injected in `parseTransactionFromTemplate()`
- **Audit Events**: Import events created in `ImportTransactionsFromCSV()` with confidence scores
- **Retraining**: `store.RetrainMLCategorizer()` available for manual retraining
- **Statistics**: `store.GetMLCategorizerStats()` for debugging and analysis

#### Configuration

- **Confidence Threshold**: Configurable via `minConfidenceThreshold` (default 0.7)
- **Training Data Source**: Only high-quality user corrections (not all transactions)
- **Fallback Behavior**: Graceful degradation when no training data or low confidence
- **Debug Output**: `[ML] Auto-categorized 'description' → Category X (confidence: Y)` messages during import

### File Structure

- `main.go`: Entry point, Bubble Tea setup
- `internal/ui/model.go`: State machine, event handlers, business logic
- `internal/ui/view.go`: Rendering with lipgloss styling
- `internal/storage/store.go`: Main store integrating domain stores, shared utilities, cross-cutting concerns
- `internal/storage/store_test.go`: Main Store comprehensive test suite (coordination layer testing)
- `internal/storage/transactions.go`: Transaction domain store (CRUD, splitting, backup/restore, bulk import)
- `internal/storage/categories.go`: Category domain store (hierarchy, validation, CRUD operations)
- `internal/storage/bank_statements.go`: Bank statement domain store (import tracking, undo functionality, overlap detection)
- `internal/storage/csv_templates.go`: CSV template domain store (template management - CRUD operations, validation, defaults)
- `internal/storage/csv_parser.go`: CSV parsing logic (file processing, transaction creation from template definitions, ML integration, duplicate detection)
- `internal/storage/transaction_audit_events.go`: Audit trail store (ML training data, user behavior tracking)
- `internal/storage/interfaces.go`: Domain store contracts and result types
- `internal/ml/categorizer.go`: ML categorization service (embeddings-based transaction categorization)
- `internal/types/types.go`: Core data structures with validation methods
- `internal/validation/`: Validation system (validators, errors, helpers, examples, tests)
- `internal/ui/*`: Supporting files for managing state

### Storage Architecture

**Domain Store Access Pattern**: UI layer calls domain stores directly through the main Store:

```go
// Access pattern for domain stores
m.store.Transactions.GetTransactions()           // Transaction operations
m.store.Categories.GetCategories()               // Category operations
m.store.Statements.GetStatementHistory()         // Bank statement operations
m.store.Templates.GetCSVTemplates()              // CSV template operations

// Cross-domain operations (moved to domain stores with dependency injection)
m.store.Statements.UndoImport(statementId)       // Cross-domain undo (BankStatementStore)
m.store.Categories.ValidateCategoryForDeletion() // Cross-domain validation (CategoryStore)

// High-level coordinated operations (main Store)
m.store.ValidateAndImportCSV(path, template)     // Primary import workflow - FULLY TESTED
m.store.ImportCSVWithOverride(path, template)    // Override import workflow - FULLY TESTED
m.store.Init()                                   // Store initialization - FULLY TESTED
m.store.GetTransactionSummaryByDateRange()       // Analytics queries - FULLY TESTED
m.store.GetCategorySpendingByDateRange()         // Category analytics - FULLY TESTED

// ML categorization operations (main Store) - FULLY TESTED
m.store.PredictCategory(description, amount)     // ML category prediction
m.store.IsHighConfidencePrediction(prediction)   // Confidence threshold check
m.store.RetrainMLCategorizer()                   // Manual retraining
m.store.GetMLCategorizerStats()                  // ML performance statistics
```

**Domain Responsibilities**:

- **TransactionStore**: CRUD operations, splitting, backup/restore, bulk imports (100% test coverage)
- **CategoryStore**: Category hierarchy, validation, CRUD with safety checks, cross-domain transaction validation (87% test coverage)
- **BankStatementStore**: Import tracking, undo functionality, overlap detection, file picker, cross-domain operations (80% test coverage)
- **CSVTemplateStore**: Template management, CSV parsing, transaction parsing from templates, deletion with safety checks (86% test coverage)
- **CSVParser**: CSV parsing service with ML integration, validation, duplicate detection (coordination service)
- **TransactionAuditStore**: Audit trail tracking, ML training data, user behavior analysis (25% coverage - critical methods tested)
- **ML Categorizer**: Text similarity-based category prediction, training from audit events, confidence scoring
- **Main Store**: Initialization, coordination layer, import workflows, analytics, ML management (100% test coverage - all 11 methods)

### Testing Architecture

**Comprehensive Test Coverage**: 78.4% overall coverage with 100% critical workflow testing

**Main Store Test Suite** (`internal/storage/store_test.go`):

```go
// P1 Critical Method Tests - FULLY TESTED
TestMainStoreInit()                             // Complete Store initialization with all dependencies
TestMainStoreValidateAndImportCSV()             // Primary import workflow with validation and overlap detection
TestMainStoreImportCSVWithOverride()            // Override import workflow with duplicate filtering
TestMainStorePredictCategory()                  // ML category prediction integration
TestMainStoreIsHighConfidencePrediction()       // ML confidence threshold validation

// P2 Analytics & Utility Tests - FULLY TESTED
TestMainStoreGetTransactionSummaryByDateRange() // Analytics summary generation
TestMainStoreGetCategorySpendingByDateRange()   // Category spending breakdown
TestMainStoreImportTransactionsFromCSV()        // Legacy import delegation
TestMainStoreGetCategoryDisplayName()           // Legacy category delegation
TestMainStoreGetDatabasePath()                  // Database path utility
TestMainStoreRetrainMLCategorizer()             // ML retraining functionality
TestMainStoreGetMLCategorizerStats()            // ML statistics reporting
```

**Test Infrastructure Patterns**:

```go
// Complete integration test setup
func setupTestMainStore(t *testing.T) (*Store, *database.Connection) {
    // Creates in-memory database with all domain stores
    // Sets up cross-references and dependency injection
    // Initializes ML categorizer with test data
    // Returns fully functional Store for testing
}

// CSV testing with real files
func createTestCSVFile(t *testing.T, filename, content string) string {
    // Creates temporary CSV files for realistic import testing
}

// Table-driven test patterns
tests := []struct {
    name         string
    setupData    func(*testing.T, *Store) (string, string) // filePath, templateName
    expectResult func(*testing.T, *Store, *types.ImportResult)
}{
    // Multiple scenarios per method with comprehensive validation
}
```

**Critical Test Scenarios Covered**:

- **Import Workflows**: Successful imports, validation errors, overlap detection, template not found
- **Override Logic**: New transaction filtering, duplicate handling, mixed scenarios
- **ML Integration**: Category prediction with/without ML categorizer, confidence thresholds
- **Analytics**: Date range queries, empty result sets, categorized transaction breakdowns
- **Cross-Domain Operations**: Coordination between domain stores, dependency injection verification
- **Error Handling**: FailFast validation mode, parsing errors, database constraints

**Testing Benefits**:

- **100% Critical Workflow Coverage**: All primary user flows validated
- **Realistic Integration**: Tests coordinate between actual domain stores
- **CSV Import Safety**: Comprehensive validation of atomic import processes
- **ML Validation**: Graceful handling when ML components unavailable
- **Error Scenarios**: Comprehensive coverage of failure modes and edge cases

### CSV Template vs Parser Separation

**KEY ARCHITECTURAL PRINCIPLE**: CSV templates and CSV parsing are completely separate concerns:

#### CSVTemplateStore (csv_templates.go) - Template Management ONLY

- Template CRUD operations (Create, Read, Update, Delete)
- Template validation and defaults handling
- Database persistence of template definitions
- **ZERO CSV parsing methods** - templates define structure, don't process files

#### CSVParser (csv_parser.go) - File Processing ONLY

- CSV file reading and line parsing
- Transaction creation from CSV data using templates
- ML-integrated category assignment
- Duplicate detection and validation
- **ZERO template management** - pure stateless file processing service

### Template Creation (`createTemplateView`)

**Two-Phase Field Editing System**: Following the transaction editing pattern

**Field Order & Types**:

1. **Template Name** (text input): Unique identifier for the template
2. **Post Date Column Index** (numeric): 0-based index for transaction date
3. **Amount Column Index** (numeric): 0-based index for transaction amount
4. **Description Column Index** (numeric): 0-based index for transaction description
5. **Category Column Index** (numeric, optional): 0-based index for transaction category
6. **Has Header** (boolean toggle): Whether CSV file has header row

**Navigation**: Up/Down arrows move between fields (blue highlight)
**Editing**: Enter/Backspace activates field editing (orange highlight)
**Validation**: Real-time validation on field exit with inline error messages
**Save**: Ctrl+S saves only when all validation passes

### Template Selection (`csvTemplateView`)

**Available Actions**:

- **Enter**: Set as default template for imports
- **d**: Delete template (with safety checks)
- **c**: Create new template
- **Up/Down**: Navigate template list

**Safety Features**:

- Cannot delete templates used by existing bank statements
- Clear error messages for failed operations
- Index adjustment after successful deletions

### Validation Rules

- **Template Name**: Non-empty, max 100 characters, must be unique
- **Column Indices**: Non-negative integers, no duplicates allowed
- **Category Column**: Optional field, when not provided transactions remain uncategorized
- **Column Uniqueness**: Prevents multiple fields using same column index

### CSV Template vs Parser Separation

**KEY ARCHITECTURAL PRINCIPLE**: CSV templates and CSV parsing are completely separate concerns:

#### CSVTemplateStore (csv_templates.go) - Template Management ONLY

- Template CRUD operations (Create, Read, Update, Delete)
- Template validation and defaults handling
- Database persistence of template definitions
- **ZERO CSV parsing methods** - templates define structure, don't process files

#### CSVParser (csv_parser.go) - File Processing ONLY

- CSV file reading and line parsing
- Transaction creation from CSV data using templates
- ML-integrated category assignment
- Duplicate detection and validation
- **ZERO template management** - pure stateless file processing service

## CSV Import Flow

### Atomic Import Process

**Critical Design**: The import process is **atomic** - either everything succeeds together, or everything fails cleanly without inconsistent state.

#### Standard Import Workflow (`ValidateAndImportCSV`)

```go
// 1. Template Validation
template := s.Templates.GetTemplateByName(templateName)
if template == nil {
    return fmt.Errorf("template '%s' not found", templateName)
}

// 2. Pre-import Validation (NEW: validates all rows before import)
validationErrors, err := s.Templates.ValidateCSVData(filePath, template, defaultCategoryId)
if err != nil {
    return fmt.Errorf("validation error: %v", err)
}
if len(validationErrors) > 0 {
    // Return validation errors with line numbers - no partial import
    return &ImportResult{HasValidationErrors: true, ValidationErrors: validationErrors}
}

// 3. CSV Parsing with Date Normalization
transactions, err := s.Templates.ParseCSVTransactions(filePath, template, defaultCategoryId)
if err != nil {
    return fmt.Errorf("failed to parse CSV: %v", err)
}

// 4. Category Existence Validation
defaultCategoryId := s.Categories.GetDefaultCategoryId()
exists, err := s.Categories.CategoryExists(defaultCategoryId)
if !exists {
    return fmt.Errorf("default category (ID: %d) not found")
}

// 5. Overlap Detection
overlaps := s.Statements.DetectOverlap(periodStart, periodEnd, template.Id)
if len(overlaps) > 0 {
    return fmt.Errorf("OVERLAP_DETECTED")
}

// 6. Atomic Import Process
statementId := s.Statements.NextId()

// Step 6a: Create statement with "importing" status (satisfies foreign key)
err = s.Statements.RecordBankStatement(filename, periodStart, periodEnd,
                                       template.Id, len(transactions), "importing")

// Step 6b: Import transactions (references valid statement_id)
err = s.Transactions.ImportTransactionsFromCSV(transactions, fmt.Sprintf("%d", statementId))
if err != nil {
    // Mark statement as failed on transaction import failure
    s.Statements.MarkStatementFailed(statementId, fmt.Sprintf("Transaction import failed: %v", err))
    return fmt.Errorf("failed to import transactions: %v", err)
}

// Step 6c: Mark statement as completed only after successful transaction import
err = s.Statements.MarkStatementCompleted(statementId)
```

#### Override Import Workflow (`ImportCSVWithOverride`)

Same atomic process but with duplicate filtering:

````go
// Parse and filter duplicates
newTransactions, duplicates, err := s.Templates.ParseCSVTransactionsWithDuplicateFilter(...)

// Same atomic import process for newTransactions only
// Statement status becomes "override" instead of "completed"
```go

### Import Status Management

#### Bank Statement Status Flow

```go
"importing"  → "completed"  // Successful import
"importing"  → "failed"     // Import error occurred
"completed" → "undone"     // User undo operation
"failed"    → (retry)      // Manual retry (future feature)
"undone"    → (delete)     // Permanent removal
````

#### Status Methods

```go
// Status transitions
bs.RecordBankStatement(filename, start, end, templateId, count, "importing")
bs.MarkStatementCompleted(statementId)                    // importing → completed
bs.MarkStatementFailed(statementId, errorMessage)        // importing → failed
bs.MarkStatementUndone(statementId)                       // completed → undone
bs.DeleteStatement(statementId)                           // Permanent deletion

// Status queries
bs.CanUndoImport(statementId)                            // Check if undo allowed
bs.GetStatementsByStatus("failed")                        // Filter by status
```

### Foreign Key Constraint Management

**Critical Fix**: The import process creates the bank statement record **before** importing transactions to satisfy foreign key constraints:

```sql
-- Schema constraint
FOREIGN KEY (statement_id) REFERENCES bank_statements(id)

-- Import order ensures referential integrity:
1. INSERT INTO bank_statements (status='importing') -- Creates referenced record
2. INSERT INTO transactions (statement_id=X)        -- Can reference existing record
3. UPDATE bank_statements SET status='completed'    -- Mark as successful
```

### Error Handling & Recovery

#### Common Error Scenarios

1. **Missing Categories**:

   ```
   Error: "Default category (ID: 1) not found in database. Please create categories first."
   Recovery: Navigate to category management, create categories
   ```

2. **CSV Parsing Errors**:

   ```
   Error: "Failed to parse CSV: line 5: invalid amount '$invalid'"
   Recovery: Fix CSV data or adjust template configuration
   ```

3. **Foreign Key Constraints** (Fixed):

   ```
   Old Error: "FOREIGN KEY constraint failed (787)"
   New Behavior: Proper atomic import with clear error messages
   ```

4. **Template Not Found**:
   ```
   Error: "Template 'MyBank' not found"
   Recovery: Create matching template or select existing template
   ```

#### Error State Management

```go
// Failed import leaves clean state:
statement.Status = "failed"           // Clear status indicator
statement.ErrorLog = "specific error" // Detailed error for debugging
statement.TxCount = 0                 // No transactions imported
// No orphaned transaction records
```

### Validation Pipeline

#### Pre-Import Validation

1. **Template Validation**: Verify template exists and is valid
2. **Category Validation**: Ensure default category exists in database
3. **CSV Structure**: Validate column count and data format
4. **Overlap Detection**: Check for period conflicts with existing imports

#### Transaction-Level Validation

```go
// Per-transaction validation during parsing
transaction.Amount, err = cts.ParseAmount(amountStr)     // Amount parsing
transaction.Date = strings.Trim(fields[template.DateColumn], "\"")  // Date extraction
transaction.CategoryId = defaultCategoryId               // Category assignment

// Invalid transactions are skipped, not failed
if err != nil {
    continue // Skip invalid transactions, continue processing
}
```

### Template Integration

#### CSV Template Structure

```go
type CSVTemplate struct {
    DateColumn   int    // Column index for transaction date
    AmountColumn int    // Column index for amount
    DescColumn   int    // Column index for description
    MerchantColumn *int  // Optional merchant column
    HasHeader    bool   // Skip first row if true
    DateFormat   string // Date parsing format
    Delimiter    string // CSV delimiter (default ",")
}
```

#### Transaction Parsing

```go
// Template-driven field extraction
date := fields[template.DateColumn]
amount := fields[template.AmountColumn]
description := fields[template.DescColumn]

// Optional merchant column handling
if template.MerchantColumn != nil {
    merchant := fields[*template.MerchantColumn]
    if merchant != "" {
        description = fmt.Sprintf("%s - %s", merchant, description)
    }
}
```

## Quick Reference

**Navigation**: Up/Down arrows, Enter/Backspace activation
**Save**: Ctrl+S for transactions and bulk operations  
**Cancel**: Esc returns to previous view
**Multi-select**: 'm' toggles, 'e' edits selected items
**Split**: 's' in edit view, modify original + create second transaction
**Bank Statements**: 'i' quick import, 'b' or 'h' management interface, 'u' quick undo
**CSV Templates**: 'c' create template, 'd' delete template, Enter selects default

Use this foundation for implementing new features while maintaining architectural consistency.
