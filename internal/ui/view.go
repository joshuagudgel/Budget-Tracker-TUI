package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"budget-tracker-tui/internal/types"

	"github.com/charmbracelet/lipgloss"
)

var (
	appNameStyle = lipgloss.NewStyle().Background(lipgloss.Color("99")).Padding(0, 1)

	faintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Faint(true)

	enumeratorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).MarginRight(3)

	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))

	// Edit form styles
	formLabelStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).Width(15)
	formFieldStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(30)
	activeFieldStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("99")).Padding(0, 1).Width(30)

	// Selection mode indicators (add these)
	selectingFieldStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("208")).Padding(0, 1).Width(30) // Orange border for selecting
	errorFieldStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("9")).Padding(0, 1).Width(30)   // Red border for validation errors

	// Notification styles
	notificationStyle = lipgloss.NewStyle().Background(lipgloss.Color("196")).Foreground(lipgloss.Color("15")).Padding(0, 1).MarginBottom(1)
	warningStyle      = lipgloss.NewStyle().Background(lipgloss.Color("214")).Foreground(lipgloss.Color("0")).Padding(0, 1).MarginBottom(1)
	successStyle      = lipgloss.NewStyle().Background(lipgloss.Color("46")).Foreground(lipgloss.Color("0")).Padding(0, 1).MarginBottom(1)

	// Phase 4: Enhanced Category Management Styles
	categoryHeaderStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33")).Background(lipgloss.Color("240")).Padding(0, 1)
	categorySelectedStyle  = lipgloss.NewStyle().Background(lipgloss.Color("99")).Foreground(lipgloss.Color("15")).Padding(0, 1)
	categoryParentStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Faint(true)
	categoryColorBadge     = lipgloss.NewStyle().Bold(true).Padding(0, 1).MarginRight(1)
	categoryIdStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(4)
	categoryHierarchyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).MarginRight(1)
	formSectionStyle       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(1).MarginBottom(1)
	helpTextStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true).MarginTop(1)
	statusIconStyle        = lipgloss.NewStyle().Bold(true).MarginRight(1)
)

// formatDateForDisplay formats a date string to MM/DD/YYYY for display
// Handles both RFC3339 timestamps (YYYY-MM-DDTHH:MM:SSZ) and ISO 8601 dates (YYYY-MM-DD)
func formatDateForDisplay(dateStr string) string {
	if dateStr == "" {
		return ""
	}

	// First try to parse as RFC3339 timestamp (YYYY-MM-DDTHH:MM:SSZ)
	if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return t.Format("01/02/2006")
	}

	// Then try to parse as ISO 8601 date format (YYYY-MM-DD)
	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		return t.Format("01/02/2006")
	}

	// If parsing fails, return original string
	return dateStr
}

// formatTimestampForDisplay formats an RFC3339 timestamp string to MM/DD/YYYY for display
func formatTimestampForDisplay(timestampStr string) string {
	if timestampStr == "" {
		return ""
	}

	// Try to parse the RFC3339 timestamp format
	if t, err := time.Parse(time.RFC3339, timestampStr); err == nil {
		return t.Format("01/02/2006")
	}

	// If parsing fails, return original string
	return timestampStr
}

func (m model) View() string {
	s := appNameStyle.Render("Finances Wrapped") + "\n\n"

	// Add multi-select indicator
	if m.isMultiSelectMode {
		s += headerStyle.Render(fmt.Sprintf("MULTI-SELECT MODE (%d selected)", len(m.selectedTxIds))) + "\n"
	}

	switch m.state {
	case menuView:
		s += headerStyle.Render("Manage Transactions ('t')") + "\n"
		s += headerStyle.Render("Import Bank Statement ('i')") + "\n"
		s += headerStyle.Render("Manage Bank Statements ('b')") + "\n"
		s += headerStyle.Render("Manage Categories ('c')") + "\n"
		s += headerStyle.Render("Analytics ('a')") + "\n"
		s += headerStyle.Render("Settings ('r')") + "\n"
		s += headerStyle.Render("Quit ('q')") + "\n"
	case listView:
		// view transactions in one large list
		// headers stay along top with aligned columns

		// Show deletion confirmation if pending
		if m.pendingDeleteTx {
			desc := m.deleteTransactionDesc
			if len(desc) > 30 {
				desc = desc[:27] + "..."
			}
			s += warningStyle.Render(fmt.Sprintf("Delete transaction: %s ($%s)? (y/n/Esc)", desc, m.deleteTransactionAmount)) + "\n\n"
		}

		s += fmt.Sprintf("%-12s | %-40s | %12s | %-20s | %-15s\n",
			headerStyle.Render("Date"),
			headerStyle.Render("Description"),
			headerStyle.Render("Amount"),
			headerStyle.Render("Category"),
			headerStyle.Render("Type")) + "\n"

		headerLines := 4 // App title + debug + empty line + header line
		availableHeight := m.windowHeight - headerLines - 2

		if availableHeight <= 0 {
			availableHeight = 10 // Fallback minimum
		}

		// Calculate scroll offset
		startIndex := 0
		if len(m.transactions) > availableHeight {
			startIndex = m.listIndex - availableHeight/2
			if startIndex < 0 {
				startIndex = 0
			}
			if startIndex > len(m.transactions)-availableHeight {
				startIndex = len(m.transactions) - availableHeight
			}
		}

		endIndex := startIndex + availableHeight
		if endIndex > len(m.transactions) {
			endIndex = len(m.transactions)
		}

		// Render visible transactions with selection indicators
		for i := startIndex; i < endIndex; i++ {
			t := m.transactions[i]
			prefix := " "

			if m.isMultiSelectMode {
				if m.selectedTxIds[t.Id] {
					prefix = "✓"
				} else {
					prefix = " "
				}

				if i == m.listIndex {
					prefix += ">"
				} else {
					prefix += " "
				}
			} else if i == m.listIndex {
				prefix = ">"
			}

			// Format transaction data with aligned columns
			description := t.Description
			if len(description) > 40 {
				description = description[:37] + "..."
			}

			categoryName := m.getCategoryDisplayName(t.CategoryId)
			if len(categoryName) > 20 {
				categoryName = categoryName[:17] + "..."
			}

			transactionType := t.TransactionType
			if len(transactionType) > 15 {
				transactionType = transactionType[:12] + "..."
			}

			s += enumeratorStyle.Render(prefix) + fmt.Sprintf("%-12s | %-40s | %12.2f | %-20s | %-15s\n",
				formatDateForDisplay(t.Date.Format("2006-01-02")),
				description,
				t.Amount,
				categoryName,
				transactionType)
		}

		// Fill remaining space if needed
		for i := endIndex - startIndex; i < availableHeight; i++ {
			s += "\n"
		}

		if len(m.transactions) == 0 {
			s += faintStyle.Render("Import a bank statement to view transactions.")
		} else {
			scrollInfo := ""
			if len(m.transactions) > availableHeight {
				scrollInfo = fmt.Sprintf(" (%d/%d)", m.listIndex+1, len(m.transactions))
			}

			// Updated help text based on mode
			if m.isMultiSelectMode {
				s += faintStyle.Render("Enter: Toggle Selection | e: Edit Selected | m: Exit Multi-Select | Esc: Menu" + scrollInfo)
			} else {
				s += faintStyle.Render("Up/Down: Navigate | e: Edit | m: Multi-Select | d: Delete | Esc: Menu" + scrollInfo)
			}
		}
	case editView:
		s += headerStyle.Render("Edit Transaction") + "\n\n"

		if m.isSplitMode {
			s += m.renderSplitView()
		} else {
			s += m.renderNormalEditView()
		}

	case categoryView:
		s += headerStyle.Render("Category Management") + "\n\n"

		categories, _ := m.store.Categories.GetCategories()
		if len(categories) == 0 {
			s += faintStyle.Render("No categories found.") + "\n\n"
		} else {
			// Display available categories
			for i, category := range categories {
				prefix := "  "
				if i == m.categoryIndex {
					prefix = "> "
				}

				// Show category details - now just display name since Name field removed
				categoryDetails := fmt.Sprintf("%d - %s", category.Id, category.DisplayName)
				s += enumeratorStyle.Render(prefix) + categoryDetails + "\n"
			}
		}

		s += "\n" + faintStyle.Render("Up/Down: Navigate | c: Create Category | Esc: Return to menu")

	case categoryListView:
		s += m.renderCategoryListView()

	case categoryEditView:
		s += m.renderCategoryEditView()

	case categoryCreateView:
		s += m.renderCategoryCreateView()

	case createCategoryView:
		s += headerStyle.Render("Create Category") + "\n\n"

		if m.categoryMessage != "" {
			s += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.categoryMessage) + "\n\n"
		}

		// Display Name field (now the only field needed)
		displayValue := m.newCategory.DisplayName
		if m.isEditingCategoryDisplayName {
			displayValue = m.editingCategoryDisplayNameStr
		}
		displayStyle := m.getCategoryFieldStyle(createCategoryDisplayName, m.isEditingCategoryDisplayName)
		s += formLabelStyle.Render("Category Name:") + "\n" + displayStyle.Render(displayValue) + "\n\n"

		s += faintStyle.Render("Up/Down: Navigate | Enter/Backspace: Edit | Ctrl+S: Save | Esc: Cancel")
	case backupView:
		s += headerStyle.Render("Snapshot Options:") + "\n\n"

		s += faintStyle.Render("s: Save Snapshot") + "\n"
		s += faintStyle.Render("l: Load Snapshot") + "\n\n"

		if m.snapshotMessage != "" {
			if strings.Contains(m.snapshotMessage, "successfully") {
				s += successStyle.Render(m.snapshotMessage) + "\n\n"
			} else {
				s += warningStyle.Render(m.snapshotMessage) + "\n\n"
			}
		}

		s += faintStyle.Render("Esc: Return to menu") + "\n\n"
	case filePickerView:
		s += headerStyle.Render("Select CSV File") + "\n\n"
		s += faintStyle.Render("Current Directory: "+m.currentDir) + "\n\n"

		if len(m.dirEntries) == 0 {
			s += faintStyle.Render("No directories or CSV files found in this location.") + "\n\n"
		} else {
			// Display directory entries
			for i, entry := range m.dirEntries {
				prefix := "  "
				if i == m.fileIndex {
					prefix = "> "
				}

				// Style directories differently
				fullPath := filepath.Join(m.currentDir, entry)
				if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
					s += enumeratorStyle.Render(prefix) + headerStyle.Render(entry+"/") + "\n"
				} else {
					s += enumeratorStyle.Render(prefix) + entry + "\n"
				}
			}
		}

		s += "\n" + faintStyle.Render("Up/Down: Navigate | Enter: Select | Esc: Cancel")
	case csvTemplateView:
		s += headerStyle.Render("Select CSV Template") + "\n\n"

		templates, _ := m.store.Templates.GetCSVTemplates()
		currentDefaultTemplate := m.store.Templates.GetDefaultTemplate()
		if len(templates) == 0 {
			s += faintStyle.Render("No CSV templates found.") + "\n\n"
		} else {
			// Display available templates
			for i, template := range templates {
				prefix := "  "
				if i == m.templateIndex {
					prefix = "> "
				}

				// Show current default
				suffix := ""
				if template.Name == currentDefaultTemplate {
					suffix = " (current)"
				}

				// Show template details
				templateDetails := fmt.Sprintf("%s - Date:%d, Amount:%d, Desc:%d, Header:%v%s",
					template.Name, template.PostDateColumn, template.AmountColumn, template.DescColumn, template.HasHeader, suffix)

				s += enumeratorStyle.Render(prefix) + templateDetails + "\n"
			}
		}

		s += "\n" + faintStyle.Render("Up/Down: Navigate | Enter: Select | d: Delete | c: Create Template | Esc: Cancel")
	case createTemplateView:
		s += headerStyle.Render("Create CSV Template") + "\n\n"

		if m.createMessage != "" {
			s += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.createMessage) + "\n\n"
		}

		// Validation notification
		if m.templateValidationNotification != "" {
			s += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Background(lipgloss.Color("0")).
				Padding(0, 1).Render(m.templateValidationNotification) + "\n\n"
		}

		// Template Name field
		nameStyle := m.getTemplateFieldStyle("name", m.createField == templateName, m.isEditingTemplateName)
		nameValue := m.newTemplate.Name
		if m.isEditingTemplateName {
			nameValue = m.editingTemplateNameStr
		}
		s += formLabelStyle.Render("Template Name:") + "\n" + nameStyle.Render(nameValue) + "\n"
		s += m.renderTemplateFieldError("name")

		// Post Date Column field
		postDateStyle := m.getTemplateFieldStyle("postdate", m.createField == templatePostDate, m.isEditingTemplatePostDate)
		postDateValue := fmt.Sprintf("%d", m.newTemplate.PostDateColumn)
		if m.isEditingTemplatePostDate {
			postDateValue = m.editingTemplatePostDateStr
		}
		s += formLabelStyle.Render("Post Date Column Index:") + "\n" + postDateStyle.Render(postDateValue) + "\n"
		s += m.renderTemplateFieldError("postdate")

		// Amount Column field
		amountStyle := m.getTemplateFieldStyle("amount", m.createField == templateAmount, m.isEditingTemplateAmount)
		amountValue := fmt.Sprintf("%d", m.newTemplate.AmountColumn)
		if m.isEditingTemplateAmount {
			amountValue = m.editingTemplateAmountStr
		}
		s += formLabelStyle.Render("Amount Column Index:") + "\n" + amountStyle.Render(amountValue) + "\n"
		s += m.renderTemplateFieldError("amount")

		// Description Column field
		descStyle := m.getTemplateFieldStyle("description", m.createField == templateDesc, m.isEditingTemplateDesc)
		descValue := fmt.Sprintf("%d", m.newTemplate.DescColumn)
		if m.isEditingTemplateDesc {
			descValue = m.editingTemplateDescStr
		}
		s += formLabelStyle.Render("Description Column Index:") + "\n" + descStyle.Render(descValue) + "\n"
		s += m.renderTemplateFieldError("description")

		// Category Column field (optional)
		categoryStyle := m.getTemplateFieldStyle("category", m.createField == templateCategory, m.isEditingTemplateCategory)
		categoryValue := "Not specified"
		if m.isEditingTemplateCategory {
			if m.editingTemplateCategoryStr != "" {
				categoryValue = m.editingTemplateCategoryStr
			} else {
				categoryValue = ""
			}
		} else if m.newTemplate.CategoryColumn != nil {
			categoryValue = fmt.Sprintf("%d", *m.newTemplate.CategoryColumn)
		}
		s += formLabelStyle.Render("Category Column Index (optional):") + "\n" + categoryStyle.Render(categoryValue) + "\n"
		s += m.renderTemplateFieldError("category")

		// Has Header field
		headerStyle := m.getTemplateFieldStyle("header", m.createField == templateHeader, false)
		headerValue := "No"
		if m.newTemplate.HasHeader {
			headerValue = "Yes"
		}
		s += formLabelStyle.Render("Has Header:") + "\n" + headerStyle.Render(headerValue) + "\n\n"

		s += faintStyle.Render("Up/Down: Navigate | Enter/Backspace: Edit | Ctrl+S: Save | Esc: Cancel")
	case bulkEditView:
		s += headerStyle.Render(fmt.Sprintf("Bulk Edit %d Transactions", len(m.selectedTxIds))) + "\n\n"

		// Add validation notification
		s += m.renderValidationNotification()

		// Amount field
		s += m.renderBulkEditField("Amount:", m.bulkAmountValue, m.bulkAmountIsPlaceholder,
			"Enter new amount", bulkEditAmount, m.isBulkEditingAmount) + "\n"

		// Description field
		s += m.renderBulkEditField("Description:", m.bulkDescriptionValue, m.bulkDescriptionIsPlaceholder,
			"Enter new description", bulkEditDescription, m.isBulkEditingDescription) + "\n"

		// Date field
		s += m.renderBulkEditField("Date:", m.bulkDateValue, m.bulkDateIsPlaceholder,
			"Enter new date", bulkEditDate, m.isBulkEditingDate) + "\n"

		// Category dropdown (existing logic)
		s += m.renderBulkCategoryField() + "\n"

		// Type dropdown (existing logic)
		s += m.renderBulkTypeField() + "\n"

		applyInstruction := "Ctrl+S: Apply Changes"
		if m.hasValidationErrors {
			applyInstruction = faintStyle.Render("Ctrl+S: Apply (fix errors first)")
		}

		s += faintStyle.Render("Up/Down: Navigate | Enter/Backspace: Edit | " + applyInstruction + " | Esc: Cancel")
	case bankStatementView:
		s += headerStyle.Render("Bank Statement Import") + "\n\n"

		statements := m.store.Statements.GetStatementHistory()

		// Current configuration status
		currentTemplate := m.store.Templates.GetDefaultTemplate()
		if currentTemplate != "" {
			s += formLabelStyle.Render("CSV Template:") + " " + headerStyle.Render(currentTemplate) + " ✓\n"
		} else {
			s += formLabelStyle.Render("CSV Template:") + " " + faintStyle.Render("Not configured") + " ⚠\n"
		}

		if m.selectedFile != "" {
			s += formLabelStyle.Render("Selected File:") + " " + headerStyle.Render(filepath.Base(m.selectedFile)) + " ✓\n\n"
		} else {
			s += formLabelStyle.Render("Selected File:") + " " + faintStyle.Render("None selected") + "\n\n"
		}

		// Recent statement history (last 3)
		if len(statements) > 0 {
			s += headerStyle.Render("Recent Imports:") + "\n"
			startIdx := len(statements) - 3
			if startIdx < 0 {
				startIdx = 0
			}

			for i := startIdx; i < len(statements); i++ {
				stmt := statements[i]
				statusIcon := "✓"
				switch stmt.Status {
				case "failed":
					statusIcon = "✗"
				case "override":
					statusIcon = "⚠"
				}

				s += faintStyle.Render(fmt.Sprintf("  %s %s (%s to %s) - %d txs",
					statusIcon, stmt.Filename, stmt.PeriodStart, stmt.PeriodEnd, stmt.TxCount)) + "\n"
			}
			s += "\n"
		}

		if m.statementMessage != "" {
			if strings.Contains(m.statementMessage, "Error") {
				s += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.statementMessage) + "\n\n"
			} else {
				s += lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(m.statementMessage) + "\n\n"
			}
		}

		s += faintStyle.Render("t: Select Template | f: Choose File | h: Manage Statements | Esc: Menu")

	case undoConfirmView:
		return m.renderUndoConfirmView()

	case statementOverlapView:
		s += headerStyle.Render("Import Overlap Warning") + "\n\n"

		// Display current document details
		s += lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render(
			fmt.Sprintf("📄 Current Document: %s", m.currentImportFilename)) + "\n"
		s += faintStyle.Render(fmt.Sprintf("   Period: %s to %s",
			m.currentImportPeriodStart, m.currentImportPeriodEnd)) + "\n\n"

		// Display overlap warning
		s += lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render(
			"⚠ This document overlaps with existing imports:") + "\n\n"

		for _, stmt := range m.overlappingStmts {
			s += faintStyle.Render(fmt.Sprintf("  • %s (%s to %s) - %d transactions",
				stmt.Filename, stmt.PeriodStart, stmt.PeriodEnd, stmt.TxCount)) + "\n"
		}

		s += "\n" + faintStyle.Render("This may create duplicate transactions in your data.") + "\n\n"

		s += faintStyle.Render("y: Import Anyway | n: Cancel Import | Esc: Cancel")

	case validationErrorView:
		s += headerStyle.Render("CSV Validation Errors") + "\n\n"

		// Display error summary
		errorCount := len(m.validationErrors)
		s += notificationStyle.Render(fmt.Sprintf(" ✗ Found %d formatting error(s) in CSV file ", errorCount)) + "\n\n"
		s += faintStyle.Render("Please fix these errors and try importing again.") + "\n\n"

		// Display errors (first 5)
		displayCount := errorCount
		if displayCount > 5 {
			displayCount = 5
		}

		s += lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Errors (showing first %d):", displayCount)) + "\n\n"

		for i := 0; i < displayCount; i++ {
			err := m.validationErrors[i]
			lineInfo := ""
			if err.LineNumber > 0 {
				lineInfo = fmt.Sprintf("Line %d: ", err.LineNumber)
			}

			s += lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("  • ") +
				lipgloss.NewStyle().Bold(true).Render(lineInfo) +
				lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render("["+err.Field+"]") +
				" - " + err.Message + "\n"
		}

		if errorCount > 5 {
			s += "\n" + faintStyle.Render(fmt.Sprintf("... and %d more error(s)", errorCount-5)) + "\n"
		}

		s += "\n" + faintStyle.Render("Esc: Go Back")

	case bankStatementListView:
		return m.renderBankStatementListView()

	case bankStatementManageView:
		return m.renderBankStatementManageView()

	case statementTransactionListView:
		return m.renderStatementTransactionListView()

	case analyticsView:
		return m.renderAnalyticsView()
	case snapshotNameInputView:
		return m.renderSnapshotNameInputView()
	case snapshotSavePickerView:
		return m.renderSnapshotSavePickerView()
	case snapshotLoadPickerView:
		return m.renderSnapshotLoadPickerView()
	}

	return s
}

func (m model) renderBulkCategoryOptions() string {
	var s string
	s += faintStyle.Render("Categories:") + "\n"

	categories, _ := m.store.Categories.GetCategories()
	maxVisible := 5

	startIdx := 0
	if len(categories) > maxVisible {
		startIdx = m.bulkCategorySelectIndex - maxVisible/2
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx > len(categories)-maxVisible {
			startIdx = len(categories) - maxVisible
		}
	}

	endIdx := startIdx + maxVisible
	if endIdx > len(categories) {
		endIdx = len(categories)
	}

	for i := startIdx; i < endIdx; i++ {
		cat := categories[i]
		prefix := "  "
		if i == m.bulkCategorySelectIndex {
			prefix = "> "
			s += enumeratorStyle.Render(prefix) + headerStyle.Render(cat.DisplayName) + "\n"
		} else {
			s += faintStyle.Render(prefix+cat.DisplayName) + "\n"
		}
	}

	if len(categories) > maxVisible {
		s += faintStyle.Render(fmt.Sprintf("   (%d/%d categories)", m.bulkCategorySelectIndex+1, len(categories))) + "\n"
	}

	return s
}

// Bulk edit

func (m model) renderBulkEditField(label, value string, isPlaceholder bool, placeholder string, fieldType uint, isEditing bool) string {
	// Map fieldType to validation field name
	fieldName := ""
	switch fieldType {
	case bulkEditAmount:
		fieldName = "amount"
	case bulkEditDescription:
		fieldName = "description"
	case bulkEditDate:
		fieldName = "date"
	}

	style := m.getFieldStyle(fieldName, m.bulkEditField == fieldType, isEditing)

	displayValue := value
	if isPlaceholder {
		displayValue = placeholder
		style = style.Faint(true)
	}

	result := formLabelStyle.Render(label) + "\n" + style.Render(displayValue)

	// Add field-specific error message if any
	if fieldName != "" {
		if err, hasErr := m.fieldErrors[fieldName]; hasErr {
			result += "\n" + faintStyle.Render("  ⚠ "+err)
		}
	}

	return result
}

func (m model) renderBulkCategoryField() string {
	var s string

	// Category dropdown field
	categoryStyle := m.getFieldStyle("category", m.bulkEditField == bulkEditCategory, m.isBulkSelectingCategory)

	categoryValue := m.bulkCategoryValue
	if m.bulkEditField == bulkEditCategory && m.isBulkSelectingCategory {
		categoryValue = "▼ Select Category"
	}
	if m.bulkCategoryIsPlaceholder || categoryValue == "" {
		categoryValue = "Select category to change"
		categoryStyle = categoryStyle.Faint(true)
	} else if !(m.bulkEditField == bulkEditCategory && m.isBulkSelectingCategory) {
		// Add dropdown indicator when not actively selecting
		categoryValue += " ▼"
	}

	s += formLabelStyle.Render("Category:") + "\n" + categoryStyle.Render(categoryValue)

	// Add field-specific error message if any
	if err, hasErr := m.fieldErrors["category"]; hasErr {
		s += "\n" + faintStyle.Render("  ⚠ "+err)
	}

	// Show category dropdown when selecting
	if m.isBulkSelectingCategory {
		s += "\n" + m.renderBulkCategoryOptions()
	}

	return s
}

func (m model) renderBulkTypeField() string {
	var s string

	// Type dropdown field
	typeStyle := formFieldStyle
	if m.bulkEditField == bulkEditType {
		if m.isBulkSelectingType {
			typeStyle = selectingFieldStyle
		} else {
			typeStyle = activeFieldStyle
		}
	}

	typeValue := m.bulkTypeValue
	if m.bulkEditField == bulkEditType && m.isBulkSelectingType {
		typeValue = "▼ Select Type"
	}
	if m.bulkTypeIsPlaceholder || typeValue == "" {
		typeValue = "Select type to change"
		typeStyle = typeStyle.Faint(true)
	} else if !(m.bulkEditField == bulkEditType && m.isBulkSelectingType) {
		// Add dropdown indicator when not actively selecting
		typeValue += " ▼"
	}

	s += formLabelStyle.Render("Type:") + "\n" + typeStyle.Render(typeValue)

	// Show type dropdown when selecting
	if m.isBulkSelectingType {
		s += "\n" + m.renderBulkTypeOptions()
	}

	return s
}

func (m model) renderBulkTypeOptions() string {
	var s string
	s += faintStyle.Render("Types:") + "\n"

	for i, transactionType := range m.availableTypes {
		prefix := "  "
		if i == m.bulkTypeSelectIndex {
			prefix = "> "
			s += enumeratorStyle.Render(prefix) + headerStyle.Render(transactionType) + "\n"
		} else {
			s += faintStyle.Render(prefix+transactionType) + "\n"
		}
	}
	return s
}

func (m model) renderSplitView() string {
	var s string
	// Show amount with proper sign formatting
	amountDisplay := fmt.Sprintf("$%.2f", m.currTransaction.Amount)
	if m.currTransaction.Amount < 0 {
		amountDisplay = fmt.Sprintf("-$%.2f", -m.currTransaction.Amount)
	}

	s += headerStyle.Render(fmt.Sprintf("Split Transaction: %s", amountDisplay)) + "\n\n"

	// Add validation notification
	s += m.renderValidationNotification()

	if m.splitMessage != "" {
		if strings.Contains(m.splitMessage, "Error") {
			s += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.splitMessage) + "\n\n"
		} else {
			s += lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(m.splitMessage) + "\n\n"
		}
	}

	// Split 1 fields
	s += headerStyle.Render("Split 1:") + "\n"
	s += m.renderSplitField("Amount:", m.splitAmount1, splitAmount1Field) + "\n"
	s += m.renderSplitField("Description:", m.splitDesc1, splitDesc1Field) + "\n"
	s += m.renderSplitField("Category:", m.splitCategory1, splitCategory1Field) + "\n\n"

	// Split 2 fields
	s += headerStyle.Render("Split 2:") + "\n"
	s += m.renderSplitField("Amount:", m.splitAmount2, splitAmount2Field) + "\n"
	s += m.renderSplitField("Description:", m.splitDesc2, splitDesc2Field) + "\n"
	s += m.renderSplitField("Category:", m.splitCategory2, splitCategory2Field) + "\n\n"

	// Show remaining amount with proper formatting
	total1, _ := strconv.ParseFloat(m.splitAmount1, 64)
	total2, _ := strconv.ParseFloat(m.splitAmount2, 64)
	remaining := m.currTransaction.Amount - total1 - total2

	remainingDisplay := fmt.Sprintf("Remaining: $%.2f", remaining)
	if remaining < 0 {
		remainingDisplay = fmt.Sprintf("Remaining: -$%.2f", -remaining)
	} else if remaining == 0 {
		remainingDisplay = faintStyle.Render("✓ Balanced")
	}
	s += faintStyle.Render(remainingDisplay) + "\n\n"

	saveInstruction := "Ctrl+S: Save Split"
	if m.hasValidationErrors {
		saveInstruction = faintStyle.Render("Ctrl+S: Save (fix errors first)")
	}

	s += faintStyle.Render("Up/Down: Navigate | Enter: Edit Field | " + saveInstruction + " | Esc: Cancel Split")
	return s
}

func (m model) renderSplitField(label, value string, fieldType uint) string {
	// Map split field types to validation field names
	var validationFieldName string
	switch fieldType {
	case splitAmount1Field:
		validationFieldName = "splitAmount1"
	case splitAmount2Field:
		validationFieldName = "splitAmount2"
	case splitDesc1Field:
		validationFieldName = "splitDesc1"
	case splitDesc2Field:
		validationFieldName = "splitDesc2"
	case splitCategory1Field:
		validationFieldName = "splitCategory1"
	case splitCategory2Field:
		validationFieldName = "splitCategory2"
	}

	isEditing := false

	// Check if this field is being edited and get current editing value
	var displayValue string
	switch fieldType {
	case splitAmount1Field:
		isEditing = m.isSplitEditingAmount1
		if isEditing && m.splitEditingAmount1 != "" {
			displayValue = m.splitEditingAmount1
		} else {
			displayValue = value
		}
	case splitAmount2Field:
		isEditing = m.isSplitEditingAmount2
		if isEditing && m.splitEditingAmount2 != "" {
			displayValue = m.splitEditingAmount2
		} else {
			displayValue = value
		}
	case splitDesc1Field:
		isEditing = m.isSplitEditingDesc1
		if isEditing && m.splitEditingDesc1 != "" {
			displayValue = m.splitEditingDesc1
		} else {
			displayValue = value
		}
	case splitDesc2Field:
		isEditing = m.isSplitEditingDesc2
		if isEditing && m.splitEditingDesc2 != "" {
			displayValue = m.splitEditingDesc2
		} else {
			displayValue = value
		}
	case splitCategory1Field:
		isEditing = m.isSplitSelectingCategory1
		displayValue = value // value is already the display name for split categories
	case splitCategory2Field:
		isEditing = m.isSplitSelectingCategory2
		displayValue = value // value is already the display name for split categories
	default:
		displayValue = value
	}

	// Use validation-aware styling
	style := m.getFieldStyle(validationFieldName, m.splitField == fieldType, isEditing)

	// Category specific display logic
	if fieldType == splitCategory1Field && m.isSplitSelectingCategory1 {
		displayValue = "▼ Select Category"
	} else if fieldType == splitCategory2Field && m.isSplitSelectingCategory2 {
		displayValue = "▼ Select Category"
	} else if fieldType == splitCategory1Field || fieldType == splitCategory2Field {
		// Add dropdown indicator to category fields when not selecting
		displayValue += " ▼"
	}

	result := formLabelStyle.Render(label) + "\n" + style.Render(displayValue)

	// Add field-specific error message if any
	if validationFieldName != "" {
		if err, hasErr := m.fieldErrors[validationFieldName]; hasErr {
			result += "\n" + faintStyle.Render("  ⚠ "+err)
		}
	}

	// Show category dropdown when selecting split categories
	if fieldType == splitCategory1Field && m.isSplitSelectingCategory1 {
		result += "\n" + m.renderSplitCategoryOptions(1)
	} else if fieldType == splitCategory2Field && m.isSplitSelectingCategory2 {
		result += "\n" + m.renderSplitCategoryOptions(2)
	}

	return result
}

func (m model) renderSplitCategoryOptions(splitNumber int) string {
	var s string
	s += faintStyle.Render("Categories:") + "\n"

	categories, _ := m.store.Categories.GetCategories()
	maxVisible := 5

	var currentIndex int
	if splitNumber == 1 {
		currentIndex = m.splitCat1SelectIndex
	} else {
		currentIndex = m.splitCat2SelectIndex
	}

	startIdx := 0
	if len(categories) > maxVisible {
		startIdx = currentIndex - maxVisible/2
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx > len(categories)-maxVisible {
			startIdx = len(categories) - maxVisible
		}
	}

	endIdx := startIdx + maxVisible
	if endIdx > len(categories) {
		endIdx = len(categories)
	}

	for i := startIdx; i < endIdx; i++ {
		cat := categories[i]
		prefix := "  "
		if i == currentIndex {
			prefix = "> "
			s += enumeratorStyle.Render(prefix) + headerStyle.Render(cat.DisplayName) + "\n"
		} else {
			s += faintStyle.Render(prefix+cat.DisplayName) + "\n"
		}
	}

	if len(categories) > maxVisible {
		s += faintStyle.Render(fmt.Sprintf("   (%d/%d categories)", currentIndex+1, len(categories))) + "\n"
	}

	return s
}

func (m model) renderNormalEditView() string {
	var s string

	// Add validation notification at the top
	s += m.renderValidationNotification()

	// Amount field
	amountStyle := m.getFieldStyle("amount", m.editField == editAmount, m.isEditingAmount)

	amountValue := fmt.Sprintf("%.2f", m.currTransaction.Amount)
	if m.isEditingAmount && m.editingAmountStr != "" {
		amountValue = m.editingAmountStr
	}

	s += formLabelStyle.Render("Amount:") + "\n" + amountStyle.Render(amountValue) + "\n"

	// Add field-specific error message if any
	if err, hasErr := m.fieldErrors["amount"]; hasErr {
		s += faintStyle.Render("  ⚠ "+err) + "\n"
	}
	s += "\n"

	// Description field
	descStyle := m.getFieldStyle("description", m.editField == editDescription, m.isEditingDescription)

	descValue := m.currTransaction.Description
	if m.isEditingDescription && m.editingDescStr != "" {
		descValue = m.editingDescStr
	}

	s += formLabelStyle.Render("Description:") + "\n" + descStyle.Render(descValue) + "\n"

	// Add field-specific error message if any
	if err, hasErr := m.fieldErrors["description"]; hasErr {
		s += faintStyle.Render("  ⚠ "+err) + "\n"
	}
	s += "\n"

	// Date field
	dateStyle := m.getFieldStyle("date", m.editField == editDate, m.isEditingDate)

	dateValue := formatDateForDisplay(m.currTransaction.Date.Format("2006-01-02"))
	if m.isEditingDate && m.editingDateStr != "" {
		dateValue = m.editingDateStr
	}

	s += formLabelStyle.Render("Date:") + "\n" + dateStyle.Render(dateValue) + "\n"

	// Add field-specific error message if any
	if err, hasErr := m.fieldErrors["date"]; hasErr {
		s += faintStyle.Render("  ⚠ "+err) + "\n"
	}
	s += "\n"

	// Transaction Type field with selection
	typeStyle := formFieldStyle
	if m.editField == editType {
		if m.isSelectingType {
			typeStyle = selectingFieldStyle
		} else {
			typeStyle = activeFieldStyle
		}
	}

	typeValue := m.currTransaction.TransactionType
	if m.editField == editType && m.isSelectingType {
		typeValue = "▼ Select Type"
	} else {
		typeValue += " ▼"
	}

	s += formLabelStyle.Render("Type:") + "\n" + typeStyle.Render(typeValue) + "\n"

	// Show type options when selecting
	if m.editField == editType && m.isSelectingType {
		s += m.renderTypeOptions() + "\n"
	}

	s += "\n"

	// Category field with selection - Fix the dropdown display logic
	categoryStyle := m.getFieldStyle("category", m.editField == editCategory, m.isSelectingCategory)

	categoryValue := m.getCategoryDisplayName(m.currTransaction.CategoryId)
	if m.editField == editCategory && m.isSelectingCategory {
		categoryValue = "▼ Select Category"
	} else {
		categoryValue += " ▼"
	}

	s += formLabelStyle.Render("Category:") + "\n" + categoryStyle.Render(categoryValue) + "\n"

	// Add field-specific error message if any
	if err, hasErr := m.fieldErrors["category"]; hasErr {
		s += faintStyle.Render("  ⚠ "+err) + "\n"
	}

	// Show category options when selecting - Fix this condition
	if m.editField == editCategory && m.isSelectingCategory {
		s += m.renderCategoryOptions() + "\n"
	}

	saveInstruction := "Ctrl+S: Save Transaction"
	if m.hasValidationErrors {
		saveInstruction = faintStyle.Render("Ctrl+S: Save (fix errors first)")
	}

	s += "\n" + faintStyle.Render("Up/Down: Navigate | Enter: Edit Field | "+saveInstruction+" | s: Split | Esc: Cancel")
	return s
}

func (m model) renderCategoryOptions() string {
	var s string
	s += faintStyle.Render("Categories:") + "\n"

	// Calculate available height for scrolling
	maxVisible := 5
	categories, _ := m.store.Categories.GetCategories()

	// Scroll logic for large category lists
	startIdx := 0
	if len(categories) > maxVisible {
		startIdx = m.categorySelectIndex - maxVisible/2
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx > len(categories)-maxVisible {
			startIdx = len(categories) - maxVisible
		}
	}

	endIdx := startIdx + maxVisible
	if endIdx > len(categories) {
		endIdx = len(categories)
	}

	for i := startIdx; i < endIdx; i++ {
		cat := categories[i]
		prefix := "  "
		if i == m.categorySelectIndex {
			prefix = "> "
			s += enumeratorStyle.Render(prefix) + headerStyle.Render(cat.DisplayName) + "\n"
		} else {
			s += faintStyle.Render(prefix+cat.DisplayName) + "\n"
		}
	}

	// Show scroll indicator if needed
	if len(categories) > maxVisible {
		s += faintStyle.Render(fmt.Sprintf("   (%d/%d categories)", m.categorySelectIndex+1, len(categories))) + "\n"
	}

	return s
}

func (m model) renderTypeOptions() string {
	var s string
	s += faintStyle.Render("Types:") + "\n"

	for i, transactionType := range m.availableTypes {
		prefix := "  "
		if i == m.typeSelectIndex {
			prefix = "> "
			s += enumeratorStyle.Render(prefix) + headerStyle.Render(transactionType) + "\n"
		} else {
			s += faintStyle.Render(prefix+transactionType) + "\n"
		}
	}
	return s
}

// renderValidationNotification renders validation errors if any exist
func (m model) renderValidationNotification() string {
	if m.validationNotification == "" {
		return ""
	}

	return warningStyle.Render(m.validationNotification) + "\n"
}

// getFieldStyle returns the appropriate style for a field based on its state (errors shown in notification area only)
func (m model) getFieldStyle(fieldName string, isActive bool, isEditing bool) lipgloss.Style {
	// Check editing state first
	if isEditing {
		return selectingFieldStyle
	}

	// Then check active state
	if isActive {
		return activeFieldStyle
	}

	// Default state
	return formFieldStyle
}

// getCategoryFieldStyle returns the appropriate style for category creation fields
func (m model) getCategoryFieldStyle(fieldType uint, isEditing bool) lipgloss.Style {
	isActive := m.createCategoryField == fieldType
	return m.getFieldStyle("category", isActive, isEditing)
}

// Phase 3: New Category View Rendering Methods

// renderCategoryListView renders the main category list view with enhanced styling
func (m model) renderCategoryListView() string {
	s := categoryHeaderStyle.Render("📂 Category Management") + "\n\n"

	// Show status notification with appropriate styling
	if m.categoryMessage != "" {
		s += m.renderCategoryNotification(m.categoryMessage) + "\n\n"
	}

	if len(m.categories) == 0 {
		emptyMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true).
			Align(lipgloss.Center).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(2, 4).
			Render("📝 No categories found\n\nPress 'n' to create your first category")
		s += emptyMsg + "\n\n"
	} else {
		// Category count and stats
		topLevelCount := 0
		childCount := 0
		for _, cat := range m.categories {
			if cat.ParentId == nil {
				topLevelCount++
			} else {
				childCount++
			}
		}
		statsText := fmt.Sprintf("📊 %d categories (%d top-level, %d subcategories)",
			len(m.categories), topLevelCount, childCount)
		s += faintStyle.Render(statsText) + "\n\n"

		// Categories list with hierarchical styling
		hierarchicalCategories := m.getHierarchicalCategoryList()
		for _, categoryItem := range hierarchicalCategories {
			isSelected := m.categories[m.selectedCategoryIdx].Id == categoryItem.category.Id
			s += m.renderHierarchicalCategoryItem(categoryItem, isSelected) + "\n"
		}
	}

	// Enhanced help text with icons
	helpText := "⌨️  Navigation: " + lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Render("↑↓") + " Navigate | " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("n") + " New | " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("e") + " Edit | " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("d") + " Delete | " +
		lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("Esc") + " Menu"
	s += "\n" + helpTextStyle.Render(helpText)
	return s
}

// renderCategoryEditView renders the category editing form with enhanced styling
func (m model) renderCategoryEditView() string {
	title := "✏️  Edit Category"
	if m.editingCategory.Id == 0 {
		title = "➕ Create New Category"
	}

	s := categoryHeaderStyle.Render(title) + "\n\n"

	// Show validation notification if present
	if m.categoryMessage != "" {
		s += m.renderCategoryNotification(m.categoryMessage) + "\n\n"
	}

	// Parent selection mode
	if m.isSelectingParent {
		return s + m.renderParentCategorySelection()
	}

	// Form sections with enhanced styling
	s += m.renderCategoryFormSection("Basic Information", []categoryFormField{
		{
			label:    "Display Name",
			value:    m.categoryFieldValues[categoryFieldDisplayName],
			field:    categoryFieldDisplayName,
			required: true,
			help:     "Unique name for this category",
		},
		{
			label:    "Color Code",
			value:    m.categoryFieldValues[categoryFieldColor],
			field:    categoryFieldColor,
			required: false,
			help:     "Hex color code (e.g., #FF5733) for visual identification",
		},
	})

	s += m.renderCategoryFormSection("Hierarchy", []categoryFormField{
		{
			label:    "Parent Category",
			value:    m.getCategoryParentDisplay(),
			field:    categoryFieldParent,
			required: false,
			help:     "Select a parent to create a subcategory",
		},
	})

	// Instructions with enhanced styling
	instructions := m.renderCategoryFormInstructions()
	s += "\n" + helpTextStyle.Render(instructions)

	return s
}

// renderCategoryCreateView delegates to edit view since they're identical
func (m model) renderCategoryCreateView() string {
	return m.renderCategoryEditView()
}

// renderParentCategorySelection renders the parent selection interface with enhanced styling
func (m model) renderParentCategorySelection() string {
	s := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("99")).
		Background(lipgloss.Color("240")).
		Padding(0, 1).
		Render("🔍 Select Parent Category") + "\n\n"

	// Selection instructions
	s += faintStyle.Render("Choose a parent category or select 'None' for top-level category") + "\n\n"

	// "None" option with enhanced styling
	noneStyle := lipgloss.NewStyle()
	if m.selectedParentIdx == -1 {
		noneStyle = categorySelectedStyle.Width(50)
		s += noneStyle.Render("▶ 🏠 None (Top Level Category)") + "\n"
	} else {
		s += "   🏠 None (Top Level Category)\n"
	}

	// Available parent categories
	availableParents := m.getAvailableParentCategories()
	if len(availableParents) == 0 {
		s += "\n" + faintStyle.Render("💡 No other categories available as parents")
	} else {
		s += "\n"
		for i, category := range availableParents {
			var line string

			// Color indicator if available
			colorIndicator := ""
			if category.Color != "" && strings.HasPrefix(category.Color, "#") && len(category.Color) == 7 {
				colorIndicator = categoryColorBadge.
					Background(lipgloss.Color(category.Color[1:])).
					Foreground(lipgloss.Color("15")).
					Render("●") + " "
			}

			// Format the option
			if i == m.selectedParentIdx {
				line = categorySelectedStyle.Width(50).Render(fmt.Sprintf("▶ 📁 %s%s", colorIndicator, category.DisplayName))
			} else {
				line = fmt.Sprintf("   📁 %s%s", colorIndicator, category.DisplayName)
			}
			s += line + "\n"
		}
	}

	// Enhanced instructions
	s += "\n" + helpTextStyle.Render(
		"⌨️  "+lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Render("↑↓")+" Navigate • "+
			lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("Enter")+" Select • "+
			lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Esc")+" Cancel")

	return s
}

// getCategoryEditFieldStyle returns the appropriate style for category editing fields
func (m model) getCategoryEditFieldStyle(field int) lipgloss.Style {
	isActive := m.categoryActiveField == field
	isEditing := m.categoryEditingField && isActive

	// Convert field constant to field name
	fieldName := ""
	switch field {
	case categoryFieldDisplayName:
		fieldName = "displayName"
	case categoryFieldColor:
		fieldName = "color"
	case categoryFieldParent:
		fieldName = "parentId"
	}

	// Check for validation errors first
	if _, hasError := m.categoryFieldErrors[fieldName]; hasError {
		return errorFieldStyle
	}

	// Then check editing state
	if isEditing {
		return selectingFieldStyle
	}

	// Then check active state
	if isActive {
		return activeFieldStyle
	}

	// Default state
	return formFieldStyle
}

// Phase 4: Enhanced Category Rendering Helper Methods

// renderCategoryNotification renders category status messages with appropriate styling
func (m model) renderCategoryNotification(message string) string {
	if message == "" {
		return ""
	}

	// Determine notification type based on message content
	if strings.Contains(message, "successfully") || strings.Contains(message, "created") || strings.Contains(message, "updated") {
		return statusIconStyle.Foreground(lipgloss.Color("46")).Render("✅") +
			successStyle.Render(message)
	} else if strings.Contains(message, "Error") || strings.Contains(message, "error") || strings.Contains(message, "Cannot") {
		return statusIconStyle.Foreground(lipgloss.Color("9")).Render("❌") +
			notificationStyle.Render(message)
	} else if strings.Contains(message, "validation") || strings.Contains(message, "fix") {
		return statusIconStyle.Foreground(lipgloss.Color("214")).Render("⚠️") +
			warningStyle.Render(message)
	} else {
		return statusIconStyle.Foreground(lipgloss.Color("33")).Render("ℹ️") +
			lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render(message)
	}
}

// renderCategoryItem renders a single category item with enhanced styling
func (m model) renderCategoryItem(category types.Category, index int, isSelected bool) string {
	var parts []string

	// Hierarchy indicator
	hierarchyIcon := ""
	indent := ""
	if category.ParentId != nil {
		hierarchyIcon = categoryHierarchyStyle.Render("├─")
		indent = "   "
	} else {
		hierarchyIcon = categoryHierarchyStyle.Render("📁")
	}

	// Selection indicator
	selector := "   "
	if isSelected {
		selector = categorySelectedStyle.Render(" ▶ ")
	}

	// Category ID (right-aligned in fixed width)
	idDisplay := categoryIdStyle.Render(fmt.Sprintf("%3d", category.Id))

	// Color badge if color is set
	colorBadge := ""
	if category.Color != "" {
		// Parse hex color for display
		if strings.HasPrefix(category.Color, "#") && len(category.Color) == 7 {
			colorBadge = categoryColorBadge.
				Background(lipgloss.Color(category.Color[1:])).
				Foreground(lipgloss.Color("15")).
				Render("●")
		} else {
			colorBadge = categoryColorBadge.
				Foreground(lipgloss.Color("244")).
				Render(fmt.Sprintf("[%s]", category.Color))
		}
	}

	// Category name with appropriate styling
	nameStyle := lipgloss.NewStyle()
	if isSelected {
		nameStyle = nameStyle.Bold(true).Foreground(lipgloss.Color("15"))
	} else if category.ParentId != nil {
		nameStyle = categoryParentStyle
	}

	categoryName := nameStyle.Render(category.DisplayName)

	// Combine all parts
	parts = append(parts, indent+hierarchyIcon, selector, idDisplay, colorBadge, categoryName)

	line := strings.Join(parts, " ")

	// Apply selection styling to entire line if selected
	if isSelected {
		return categorySelectedStyle.Width(60).Render(line)
	}

	return line
}

// renderHierarchicalCategoryItem renders a category item with proper hierarchical indentation
func (m model) renderHierarchicalCategoryItem(item hierarchicalCategoryItem, isSelected bool) string {
	var parts []string
	category := item.category

	// Calculate indentation based on level
	baseIndent := strings.Repeat("  ", item.level)
	hierarchyIcon := ""

	if item.level == 0 {
		// Top-level category
		hierarchyIcon = categoryHierarchyStyle.Render("📁")
	} else {
		// Subcategory - use different icons based on level
		if item.level == 1 {
			hierarchyIcon = categoryHierarchyStyle.Render("├─")
		} else {
			// Deeper nesting
			hierarchyIcon = categoryHierarchyStyle.Render("  └─")
		}
	}

	// Selection indicator
	selector := "   "
	if isSelected {
		selector = categorySelectedStyle.Render(" ▶ ")
	}

	// Category ID (right-aligned in fixed width)
	idDisplay := categoryIdStyle.Render(fmt.Sprintf("%3d", category.Id))

	// Color badge if color is set
	colorBadge := ""
	if category.Color != "" {
		// Parse hex color for display
		if strings.HasPrefix(category.Color, "#") && len(category.Color) == 7 {
			colorBadge = categoryColorBadge.
				Background(lipgloss.Color(category.Color[1:])).
				Foreground(lipgloss.Color("15")).
				Render("●")
		} else {
			colorBadge = categoryColorBadge.
				Foreground(lipgloss.Color("244")).
				Render(fmt.Sprintf("[%s]", category.Color))
		}
	}

	// Category name with appropriate styling
	nameStyle := lipgloss.NewStyle()
	if isSelected {
		nameStyle = nameStyle.Bold(true).Foreground(lipgloss.Color("15"))
	} else if item.level > 0 {
		nameStyle = categoryParentStyle
	}

	categoryName := nameStyle.Render(category.DisplayName)

	// Combine all parts with proper indentation
	parts = append(parts, baseIndent+hierarchyIcon, selector, idDisplay, colorBadge, categoryName)

	line := strings.Join(parts, " ")

	// Apply selection styling to entire line if selected
	if isSelected {
		return categorySelectedStyle.Width(60 + len(baseIndent)).Render(line)
	}

	return line
}

// categoryFormField represents a form field for category editing
type categoryFormField struct {
	label    string
	value    string
	field    int
	required bool
	help     string
}

// renderCategoryFormSection renders a form section with enhanced styling
func (m model) renderCategoryFormSection(sectionTitle string, fields []categoryFormField) string {
	s := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("33")).
		MarginTop(1).
		Render("📋 "+sectionTitle) + "\n\n"

	for _, fieldDef := range fields {
		s += m.renderCategoryFormField(fieldDef) + "\n"
	}

	return s
}

// renderCategoryFormField renders a single form field with enhanced styling
func (m model) renderCategoryFormField(fieldDef categoryFormField) string {
	var s string

	// Field label with required indicator
	label := fieldDef.label
	if fieldDef.required {
		label += " *"
	}
	labelStyle := formLabelStyle
	if m.categoryActiveField == fieldDef.field {
		labelStyle = labelStyle.Foreground(lipgloss.Color("99")).Bold(true)
	}
	s += labelStyle.Render(label+":") + "\n"

	// Field value
	value := fieldDef.value
	if m.categoryEditingField && m.categoryActiveField == fieldDef.field {
		value = m.categoryEditingStr
	}

	// Special handling for parent field
	if fieldDef.field == categoryFieldParent && m.categoryEditingField && m.categoryActiveField == categoryFieldParent {
		value = "🔍 [Selecting Parent...]"
	}

	// Field styling
	fieldStyle := m.getCategoryEditFieldStyle(fieldDef.field)

	// Color preview for color field
	if fieldDef.field == categoryFieldColor && value != "" && strings.HasPrefix(value, "#") && len(value) == 7 {
		colorPreview := categoryColorBadge.
			Background(lipgloss.Color(value[1:])).
			Foreground(lipgloss.Color("15")).
			Render("●")
		s += fieldStyle.Render(value) + " " + colorPreview + "\n"
	} else {
		s += fieldStyle.Render(value) + "\n"
	}

	// Field error
	fieldName := m.getFieldNameFromConstant(fieldDef.field)
	if err, exists := m.categoryFieldErrors[fieldName]; exists {
		s += statusIconStyle.Foreground(lipgloss.Color("9")).Render("❌") +
			lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(" "+err) + "\n"
	}

	// Field help text (only show when field is active)
	if m.categoryActiveField == fieldDef.field && fieldDef.help != "" && !m.categoryEditingField {
		s += lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true).
			MarginLeft(2).
			Render("💡 "+fieldDef.help) + "\n"
	}

	s += "\n"
	return s
}

// getCategoryParentDisplay gets the display text for parent field
func (m model) getCategoryParentDisplay() string {
	if m.selectedParentId != nil {
		if parent := m.findCategoryById(*m.selectedParentId); parent != nil {
			return "📁 " + parent.DisplayName
		}
	}
	return "🏠 None (Top Level)"
}

// renderCategoryFormInstructions renders context-sensitive instructions
func (m model) renderCategoryFormInstructions() string {
	if m.categoryEditingField {
		return "⌨️  Type to edit • " +
			lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("Enter") + " Confirm • " +
			lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Esc") + " Cancel"
	} else {
		return "⌨️  Navigation: " +
			lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Render("↑↓") + " Fields • " +
			lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("Enter/Backspace") + " Edit • " +
			lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("Ctrl+S") + " Save • " +
			lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Esc") + " Cancel"
	}
}

// getFieldNameFromConstant converts field constant to string name
func (m model) getFieldNameFromConstant(field int) string {
	switch field {
	case categoryFieldDisplayName:
		return "displayName"
	case categoryFieldColor:
		return "color"
	case categoryFieldParent:
		return "parentId"
	default:
		return ""
	}
}

// renderAnalyticsView renders the spending analysis dashboard
func (m model) renderAnalyticsView() string {
	var s string
	s += headerStyle.Render("📊 Spending Analysis Dashboard") + "\n\n"

	// Show any error messages
	if m.analyticsMessage != "" {
		s += errorFieldStyle.Render(m.analyticsMessage) + "\n\n"
	}

	// Date range inputs
	startDateStyle := formFieldStyle
	endDateStyle := formFieldStyle

	if m.isEditingStartDate {
		startDateStyle = selectingFieldStyle
	}
	if m.isEditingEndDate {
		endDateStyle = selectingFieldStyle
	}

	s += formLabelStyle.Render("Date Range:") + "\n"
	s += "Start Date ('s'): " + "\n" + startDateStyle.Render(m.editingStartDateStr) + "\n"
	s += "End Date   ('e'): " + "\n" + endDateStyle.Render(m.editingEndDateStr) + "\n\n"

	if m.analyticsSummary != nil {
		// Summary section
		s += headerStyle.Render("💰 Summary") + "\n"
		s += fmt.Sprintf("Period: %s\n", m.analyticsSummary.DateRange)
		s += fmt.Sprintf("Total Income:    %s\n",
			successStyle.Render(fmt.Sprintf("$%.2f", m.analyticsSummary.TotalIncome)))
		s += fmt.Sprintf("Total Expenses:  %s\n",
			warningStyle.Render(fmt.Sprintf("$%.2f", m.analyticsSummary.TotalExpenses)))
		s += fmt.Sprintf("Net Amount:      %s\n",
			m.formatNetAmount(m.analyticsSummary.NetAmount))
		s += fmt.Sprintf("Transactions:    %d\n\n", m.analyticsSummary.TransactionCount)

		// Category spending table
		if len(m.categorySpending) > 0 {
			s += headerStyle.Render("📋 Category Breakdown") + "\n"
			s += m.analyticsTable.View() + "\n\n"
		} else {
			s += headerStyle.Render("📋 Category Breakdown") + "\n"
			s += faintStyle.Render("No expense data found for this period") + "\n\n"
		}
	} else {
		s += faintStyle.Render("Loading analytics data...") + "\n\n"
	}

	// Command tips at bottom like other views
	s += faintStyle.Render("s: Start Date | e: End Date | r: Refresh | Esc: Menu")

	return s
}

// formatNetAmount formats net amount with appropriate styling
func (m model) formatNetAmount(amount float64) string {
	if amount >= 0 {
		return successStyle.Render(fmt.Sprintf("$%.2f", amount))
	} else {
		return notificationStyle.Render(fmt.Sprintf("-$%.2f", -amount))
	}
}

// Snapshot view render methods

func (m model) renderSnapshotNameInputView() string {
	var s string
	s += headerStyle.Render("Save Snapshot") + "\n\n"

	s += faintStyle.Render("Enter snapshot name:") + "\n\n"

	// Display input field
	inputValue := m.snapshotName
	if m.isEditingSnapshotName {
		inputValue = m.editingSnapshotNameStr
	}

	fieldStyle := formFieldStyle
	if m.isEditingSnapshotName {
		fieldStyle = selectingFieldStyle
	}

	s += fieldStyle.Render(inputValue) + "\n\n"

	// Display messages
	if m.snapshotMessage != "" {
		s += warningStyle.Render(m.snapshotMessage) + "\n\n"
	}

	s += faintStyle.Render("Enter: Continue | Esc: Cancel") + "\n"

	return s
}

func (m model) renderSnapshotSavePickerView() string {
	var s string
	s += headerStyle.Render("Save Snapshot: "+m.snapshotName) + "\n\n"
	s += faintStyle.Render("Choose location to save snapshot:") + "\n"
	s += faintStyle.Render("Current Directory: "+m.currentSnapshotDir) + "\n\n"

	if len(m.snapshotDirectoryEntries) == 0 {
		s += faintStyle.Render("No directories found in this location.") + "\n\n"
	} else {
		// Display directory entries
		for i, entry := range m.snapshotDirectoryEntries {
			prefix := "  "
			if i == m.snapshotFileIndex {
				prefix = "> "
			}

			// Style directories differently
			fullPath := filepath.Join(m.currentSnapshotDir, entry)
			if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
				s += enumeratorStyle.Render(prefix) + headerStyle.Render(entry+"/") + "\n"
			} else {
				s += enumeratorStyle.Render(prefix) + entry + "\n"
			}
		}
		s += "\n"
	}

	// Display messages
	if m.snapshotMessage != "" {
		if strings.Contains(m.snapshotMessage, "successfully") {
			s += successStyle.Render(m.snapshotMessage) + "\n\n"
		} else {
			s += warningStyle.Render(m.snapshotMessage) + "\n\n"
		}
	}

	s += faintStyle.Render("Up/Down: Navigate | Enter: Select Directory | s: Save Here | Esc: Back") + "\n"
	return s
}

func (m model) renderSnapshotLoadPickerView() string {
	var s string
	s += headerStyle.Render("Load Snapshot") + "\n\n"
	s += faintStyle.Render("Choose snapshot file to load:") + "\n"
	s += faintStyle.Render("Current Directory: "+m.currentSnapshotDir) + "\n\n"

	if len(m.snapshotDirectoryEntries) == 0 {
		s += faintStyle.Render("No directories or .db files found in this location.") + "\n\n"
	} else {
		// Display directory entries
		for i, entry := range m.snapshotDirectoryEntries {
			prefix := "  "
			if i == m.snapshotFileIndex {
				prefix = "> "
			}

			// Style directories and .db files differently
			fullPath := filepath.Join(m.currentSnapshotDir, entry)
			if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
				s += enumeratorStyle.Render(prefix) + headerStyle.Render(entry+"/") + "\n"
			} else if strings.HasSuffix(strings.ToLower(entry), ".db") {
				s += enumeratorStyle.Render(prefix) + successStyle.Render(entry) + "\n"
			} else {
				s += enumeratorStyle.Render(prefix) + faintStyle.Render(entry) + "\n"
			}
		}
		s += "\n"
	}

	// Display messages
	if m.snapshotMessage != "" {
		if strings.Contains(m.snapshotMessage, "successfully") {
			s += successStyle.Render(m.snapshotMessage) + "\n\n"
		} else {
			s += warningStyle.Render(m.snapshotMessage) + "\n\n"
		}
	}

	s += faintStyle.Render("Up/Down: Navigate | Enter: Load File/Enter Directory | Esc: Back") + "\n"
	return s
}

// renderUndoConfirmView renders the undo import confirmation view
func (m model) renderUndoConfirmView() string {
	s := headerStyle.Render("Confirm Undo Import") + "\n\n"

	// Warning message
	s += lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render(
		"⚠ This action will permanently remove all transactions from this import:") + "\n\n"

	// Statement details
	s += formLabelStyle.Render("File:") + " " + headerStyle.Render(m.undoStatementName) + "\n"
	s += formLabelStyle.Render("Transactions:") + " " + headerStyle.Render(fmt.Sprintf("%d", m.undoTxCount)) + "\n\n"

	// Error message if any
	if m.undoMessage != "" {
		s += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.undoMessage) + "\n\n"
	}

	s += faintStyle.Render("This action cannot be undone. Your transaction data will be modified.") + "\n\n"
	s += faintStyle.Render("y: Confirm Undo | n: Cancel | Esc: Cancel")

	return s
}

// Enhanced Bank Statement Management Views

// renderBankStatementListView renders the main bank statement management list
func (m model) renderBankStatementListView() string {
	s := headerStyle.Render("Bank Statement Management") + "\n\n"

	statements := m.store.Statements.GetStatementHistory()

	if len(statements) == 0 {
		s += faintStyle.Render("No bank statements imported yet.\n\n")
		s += faintStyle.Render("Get started by importing your first bank statement.\n")
		s += faintStyle.Render("i: Import Bank Statement | Esc: Menu")
		return s
	}

	// Add summary header
	s += faintStyle.Render(fmt.Sprintf("Total Statements: %d", len(statements))) + "\n\n"

	// Add column headers
	s += headerStyle.Render("Filename") + " | " +
		headerStyle.Render("Period") + " | " +
		headerStyle.Render("Imported") + " | " +
		headerStyle.Render("Txns") + " | " +
		headerStyle.Render("Status") + "\n\n"

	// Show statements with enhanced formatting and status indicators
	for i, stmt := range statements {
		prefix := "  "
		style := lipgloss.NewStyle()

		if i == m.bankStatementListIndex {
			prefix = "▶ "
			style = style.Bold(true)
		}

		// Status indicator with color and symbols
		statusStyle := successStyle
		statusSymbol := "✓"
		statusText := stmt.Status

		switch stmt.Status {
		case "completed":
			statusSymbol = "✓"
			statusStyle = successStyle
		case "failed":
			statusSymbol = "✗"
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		case "undone":
			statusSymbol = "↶"
			statusStyle = faintStyle
			statusText = "undone"
		case "override":
			statusSymbol = "⚠"
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
		}

		// Format each line with consistent spacing
		filename := stmt.Filename
		if len(filename) > 25 {
			filename = filename[:22] + "..."
		}

		dateRange := fmt.Sprintf("%s - %s", formatDateForDisplay(stmt.PeriodStart.Format("2006-01-02")), formatDateForDisplay(stmt.PeriodEnd.Format("2006-01-02")))
		txCount := fmt.Sprintf("%d txns", stmt.TxCount)

		// Format import date compactly
		importDate := formatTimestampForDisplay(stmt.ImportDate.Format(time.RFC3339))

		line := style.Render(prefix +
			fmt.Sprintf("%-28s", filename) + " | " +
			fmt.Sprintf("%-23s", dateRange) + " | " +
			fmt.Sprintf("%-8s", importDate) + " | " +
			fmt.Sprintf("%-8s", txCount) + " | " +
			statusStyle.Render(statusSymbol+" "+statusText))

		s += line + "\n"
	}

	s += "\n"

	// Show current statement details if selected
	if len(statements) > 0 && m.bankStatementListIndex >= 0 && m.bankStatementListIndex < len(statements) {
		stmt := statements[m.bankStatementListIndex]
		s += formLabelStyle.Render("Selected:") + " " + stmt.Filename + "\n"
		if stmt.ErrorLog != "" {
			s += formLabelStyle.Render("Error:") + " " +
				lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(stmt.ErrorLog) + "\n"
		}
		s += "\n"
	}

	if m.bankStatementListMessage != "" {
		s += successStyle.Render(m.bankStatementListMessage) + "\n\n"
	}

	s += faintStyle.Render("Up/Down: Navigate | Enter: Manage | u: Quick Undo | i: Import New | Esc: Menu")
	return s
}

// renderBankStatementManageView renders the individual statement management view
func (m model) renderBankStatementManageView() string {
	stmt, err := m.store.Statements.GetStatementById(m.selectedBankStatementId)
	if err != nil {
		return "Error: Statement not found"
	}

	s := headerStyle.Render("Manage Statement: "+stmt.Filename) + "\n\n"

	// Statement details
	s += formLabelStyle.Render("File:") + " " + stmt.Filename + "\n"
	s += formLabelStyle.Render("Period:") + " " + formatDateForDisplay(stmt.PeriodStart.Format("2006-01-02")) + " to " + formatDateForDisplay(stmt.PeriodEnd.Format("2006-01-02")) + "\n"
	s += formLabelStyle.Render("Transactions:") + " " + fmt.Sprintf("%d", stmt.TxCount) + "\n"
	templateName := m.store.Templates.GetTemplateNameById(stmt.TemplateUsed)
	if templateName == "" {
		templateName = fmt.Sprintf("Template ID: %d", stmt.TemplateUsed)
	}
	s += formLabelStyle.Render("Template:") + " " + templateName + "\n"
	s += formLabelStyle.Render("Import Date:") + " " + formatTimestampForDisplay(stmt.ImportDate.Format(time.RFC3339)) + "\n"

	// Status with color
	statusStyle := successStyle
	switch stmt.Status {
	case "failed":
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	case "undone":
		statusStyle = faintStyle
	case "override":
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	}
	s += formLabelStyle.Render("Status:") + " " + statusStyle.Render(stmt.Status) + "\n"

	if stmt.ErrorLog != "" {
		s += formLabelStyle.Render("Error:") + " " + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(stmt.ErrorLog) + "\n"
	}

	s += "\n" + headerStyle.Render("Available Actions:") + "\n\n"

	// Show available actions
	actions := m.getAvailableActions(*stmt)
	for i, action := range actions {
		prefix := "  "
		if i == m.bankStatementActionIndex {
			prefix = "▶ "
		}
		s += prefix + action + "\n"
	}

	s += "\n" + faintStyle.Render("Up/Down: Navigate | Enter: Execute | Esc: Back")
	return s
}

// renderStatementTransactionListView renders the filtered transaction list for a specific bank statement
func (m model) renderStatementTransactionListView() string {
	stmt, err := m.store.Statements.GetStatementById(m.currentStatementId)
	if err != nil {
		return "Error: Statement not found"
	}

	var s string
	s += headerStyle.Render("Transactions: "+stmt.Filename) + "\n"
	s += faintStyle.Render("Period: "+stmt.PeriodStart.Format("2006-01-02")+" to "+stmt.PeriodEnd.Format("2006-01-02")) + " | " +
		faintStyle.Render(fmt.Sprintf("%d transactions", len(m.filteredTransactions))) + "\n\n"

	// Show deletion confirmation if pending
	if m.pendingDeleteTx {
		desc := m.deleteTransactionDesc
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}
		s += warningStyle.Render(fmt.Sprintf("Delete transaction: %s ($%s)? (y/n/Esc)", desc, m.deleteTransactionAmount)) + "\n\n"
	}

	// Column headers with aligned columns
	s += fmt.Sprintf("%-12s | %-40s | %12s | %-20s | %-15s\n",
		headerStyle.Render("Date"),
		headerStyle.Render("Description"),
		headerStyle.Render("Amount"),
		headerStyle.Render("Category"),
		headerStyle.Render("Type")) + "\n"

	// Display message if exists
	if m.statementTxMessage != "" {
		s += successStyle.Render(m.statementTxMessage) + "\n\n"
	}

	headerLines := 6 // Title + period + headers + messages + padding
	availableHeight := m.windowHeight - headerLines - 2

	if availableHeight <= 0 {
		availableHeight = 10 // Fallback minimum
	}

	// Calculate scroll offset
	startIndex := 0
	if len(m.filteredTransactions) > availableHeight {
		startIndex = m.filteredListIndex - availableHeight/2
		if startIndex < 0 {
			startIndex = 0
		}
		if startIndex > len(m.filteredTransactions)-availableHeight {
			startIndex = len(m.filteredTransactions) - availableHeight
		}
	}

	endIndex := startIndex + availableHeight
	if endIndex > len(m.filteredTransactions) {
		endIndex = len(m.filteredTransactions)
	}

	// Render visible transactions with selection indicators
	for i := startIndex; i < endIndex; i++ {
		t := m.filteredTransactions[i]
		prefix := " "

		if m.isMultiSelectMode {
			if m.selectedTxIds[t.Id] {
				prefix = "✓"
			} else {
				prefix = " "
			}

			if i == m.filteredListIndex {
				prefix += ">"
			} else {
				prefix += " "
			}
		} else if i == m.filteredListIndex {
			prefix = ">"
		}

		// Format transaction data with aligned columns
		description := t.Description
		if len(description) > 40 {
			description = description[:37] + "..."
		}

		categoryName := m.getCategoryDisplayName(t.CategoryId)
		if len(categoryName) > 20 {
			categoryName = categoryName[:17] + "..."
		}

		transactionType := t.TransactionType
		if len(transactionType) > 15 {
			transactionType = transactionType[:12] + "..."
		}

		s += enumeratorStyle.Render(prefix) + fmt.Sprintf("%-12s | %-40s | %12.2f | %-20s | %-15s\n",
			formatDateForDisplay(t.Date.Format("2006-01-02")),
			description,
			t.Amount,
			categoryName,
			transactionType)
	}

	// Fill remaining space if needed
	for i := endIndex - startIndex; i < availableHeight; i++ {
		s += "\n"
	}

	if len(m.filteredTransactions) == 0 {
		s += faintStyle.Render("No transactions found for this statement.")
	} else {
		scrollInfo := ""
		if len(m.filteredTransactions) > availableHeight {
			scrollInfo = fmt.Sprintf(" (%d/%d)", m.filteredListIndex+1, len(m.filteredTransactions))
		}

		// Updated help text based on mode
		if m.isMultiSelectMode {
			s += faintStyle.Render("Enter: Toggle Selection | e: Edit Selected | m: Exit Multi-Select | Esc: Back" + scrollInfo)
		} else {
			s += faintStyle.Render("Up/Down: Navigate | e: Edit | m: Multi-Select | d: Delete | Esc: Back" + scrollInfo)
		}
	}

	return s
}

// Template field styling and error rendering functions

// getTemplateFieldStyle returns the appropriate style for template fields based on state
func (m model) getTemplateFieldStyle(fieldName string, isActive bool, isEditing bool) lipgloss.Style {
	// Check for validation errors and show red border if errors exist
	if m.templateFieldErrors != nil {
		if _, hasError := m.templateFieldErrors[fieldName]; hasError {
			if isEditing {
				return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("1")).Padding(0, 1).Width(30) // Red editing
			}
			if isActive {
				return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("1")).Padding(0, 1).Width(30) // Red active
			}
			return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("1")).Padding(0, 1).Width(30) // Red default
		}
	}

	// No validation errors - use standard styling
	if isEditing {
		return selectingFieldStyle
	}
	if isActive {
		return activeFieldStyle
	}
	return formFieldStyle
}

// renderTemplateFieldError renders inline error messages for template fields
func (m model) renderTemplateFieldError(fieldName string) string {
	if m.templateFieldErrors != nil {
		if errorMsg, hasError := m.templateFieldErrors[fieldName]; hasError {
			return lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("  └─ "+errorMsg) + "\n"
		}
	}
	return ""
}
