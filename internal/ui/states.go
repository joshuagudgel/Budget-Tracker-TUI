package ui

// Application state constants
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
	statementHistoryView      = 13
	statementOverlapView      = 14
	categoryListView          = 15
	categoryEditView          = 16
	categoryCreateView        = 17
	undoConfirmView           = 18
)

// Edit field constants
const (
	editAmount uint = iota
	editDescription
	editDate
	editType
	editCategory
	editSplit
)

// Template creation field constants
const (
	createTemplateName uint = iota
	createTemplateDate
	createTemplateAmount
	createTemplateDesc
	createTemplateHeader
)

// Split transaction field constants
const (
	splitAmount1Field uint = iota
	splitDesc1Field
	splitCategory1Field
	splitAmount2Field
	splitDesc2Field
	splitCategory2Field
)

// Category and bulk edit field constants
const (
	createCategoryName uint = iota
	createCategoryDisplayName
	bulkEditAmount uint = iota
	bulkEditDescription
	bulkEditDate
	bulkEditCategory
	bulkEditType
)

// Phase 3: Category field constants (using int to match model field types)
const (
	categoryFieldDisplayName int = iota
	categoryFieldColor
	categoryFieldParent
)
