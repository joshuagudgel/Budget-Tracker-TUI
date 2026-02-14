package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
)

func (m model) View() string {
	s := appNameStyle.Render("Finances Wrapped") + "\n\n"

	// Add multi-select indicator
	if m.isMultiSelectMode {
		s += headerStyle.Render(fmt.Sprintf("MULTI-SELECT MODE (%d selected)", len(m.selectedTxIds))) + "\n"
	}

	switch m.state {
	case menuView:
		s += headerStyle.Render("Manage Transactions ('t')") + "\n"
		s += headerStyle.Render("Import Bank Statements ('i')") + "\n"
		s += headerStyle.Render("Manage Categories ('c')") + "\n"
		s += headerStyle.Render("Settings ('r')") + "\n"
		s += headerStyle.Render("Quit ('q')") + "\n"
	case listView:
		// view transactions in one large list
		// headers stay along top
		s += headerStyle.Render("Date") + " | " +
			headerStyle.Render("Description") + " | " +
			headerStyle.Render("Amount") + " | " +
			headerStyle.Render("Category") + " | " +
			headerStyle.Render("Type") + "\n\n"

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

			s += enumeratorStyle.Render(prefix) + t.Date + " | " +
				t.Description + " | " +
				fmt.Sprintf("%.2f", t.Amount) + " | " +
				t.Category + " | " +
				t.TransactionType + "\n"
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

		// Show current default category
		currentDefault := m.store.categories.Default
		s += faintStyle.Render(fmt.Sprintf("Current Default: %s", currentDefault)) + "\n\n"

		if len(m.store.categories.Categories) == 0 {
			s += faintStyle.Render("No categories found.") + "\n\n"
		} else {
			// Display available categories
			for i, category := range m.store.categories.Categories {
				prefix := "  "
				if i == m.categoryIndex {
					prefix = "> "
				}

				// Show default indicator
				suffix := ""
				if category.Name == m.store.categories.Default {
					suffix = " (default)"
				}

				// Show category details
				categoryDetails := fmt.Sprintf("%s - %s%s", category.Name, category.DisplayName, suffix)
				s += enumeratorStyle.Render(prefix) + categoryDetails + "\n"
			}
		}

		s += "\n" + faintStyle.Render("Up/Down: Navigate | Enter: Set Default | c: Create Category | Esc: Return to menu")

	case createCategoryView:
		s += headerStyle.Render("Create Category") + "\n\n"

		if m.categoryMessage != "" {
			s += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.categoryMessage) + "\n\n"
		}

		// Category Name field
		nameStyle := formFieldStyle
		if m.createCategoryField == createCategoryName {
			nameStyle = activeFieldStyle
		}
		s += formLabelStyle.Render("Name:") + "\n" + nameStyle.Render(m.newCategory.Name) + "\n\n"

		// Display Name field
		displayStyle := formFieldStyle
		if m.createCategoryField == createCategoryDisplayName {
			displayStyle = activeFieldStyle
		}
		s += formLabelStyle.Render("Display Name:") + "\n" + displayStyle.Render(m.newCategory.DisplayName) + "\n\n"

		s += faintStyle.Render("Up/Down: Navigate fields | Enter: Save | Esc: Cancel")
	case backupView:
		s += headerStyle.Render("Backup Options:") + "\n\n"

		s += faintStyle.Render("r: Restore from backup | Esc: Return to menu") + "\n\n"
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

		if len(m.store.csvTemplates.Templates) == 0 {
			s += faintStyle.Render("No CSV templates found.") + "\n\n"
		} else {
			// Display available templates
			for i, template := range m.store.csvTemplates.Templates {
				prefix := "  "
				if i == m.templateIndex {
					prefix = "> "
				}

				// Show current default
				suffix := ""
				if template.Name == m.store.csvTemplates.Default {
					suffix = " (current)"
				}

				// Show template details
				templateDetails := fmt.Sprintf("%s - Date:%d, Amount:%d, Desc:%d, Header:%v%s",
					template.Name, template.DateColumn, template.AmountColumn, template.DescColumn, template.HasHeader, suffix)

				s += enumeratorStyle.Render(prefix) + templateDetails + "\n"
			}
		}

		s += "\n" + faintStyle.Render("Up/Down: Navigate | Enter: Select | c: Create Template | Esc: Cancel")
	case createTemplateView:
		s += headerStyle.Render("Create CSV Template") + "\n\n"

		if m.createMessage != "" {
			s += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.createMessage) + "\n\n"
		}

		// Template Name field
		nameStyle := formFieldStyle
		if m.createField == createTemplateName {
			nameStyle = activeFieldStyle
		}
		s += formLabelStyle.Render("Template Name:") + nameStyle.Render(m.newTemplate.Name) + "\n\n"

		// Date Column field
		dateStyle := formFieldStyle
		if m.createField == createTemplateDate {
			dateStyle = activeFieldStyle
		}
		s += formLabelStyle.Render("Date Column:") + dateStyle.Render(fmt.Sprintf("%d", m.newTemplate.DateColumn)) + "\n\n"

		// Amount Column field
		amountStyle := formFieldStyle
		if m.createField == createTemplateAmount {
			amountStyle = activeFieldStyle
		}
		s += formLabelStyle.Render("Amount Column:") + amountStyle.Render(fmt.Sprintf("%d", m.newTemplate.AmountColumn)) + "\n\n"

		// Description Column field
		descStyle := formFieldStyle
		if m.createField == createTemplateDesc {
			descStyle = activeFieldStyle
		}
		s += formLabelStyle.Render("Desc Column:") + descStyle.Render(fmt.Sprintf("%d", m.newTemplate.DescColumn)) + "\n\n"

		// Has Header field
		headerStyle := formFieldStyle
		if m.createField == createTemplateHeader {
			headerStyle = activeFieldStyle
		}
		headerValue := "No"
		if m.newTemplate.HasHeader {
			headerValue = "Yes"
		}
		s += formLabelStyle.Render("Has Header:") + headerStyle.Render(headerValue) + " (y/n)\n\n"

		s += faintStyle.Render("Up/Down: Navigate fields | Enter: Save | Esc: Cancel")
	case bulkEditView:
		s += headerStyle.Render(fmt.Sprintf("Bulk Edit %d Transactions", len(m.selectedTxIds))) + "\n\n"

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

		s += faintStyle.Render("Up/Down: Navigate | Enter/Backspace: Edit | Ctrl+S: Apply Changes | Esc: Cancel")
	case bankStatementView:
		s += headerStyle.Render("Bank Statement Import") + "\n\n"

		// Current configuration status
		currentTemplate := m.store.csvTemplates.Default
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
		if len(m.store.statements.Statements) > 0 {
			s += headerStyle.Render("Recent Imports:") + "\n"
			startIdx := len(m.store.statements.Statements) - 3
			if startIdx < 0 {
				startIdx = 0
			}

			for i := startIdx; i < len(m.store.statements.Statements); i++ {
				stmt := m.store.statements.Statements[i]
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

		s += faintStyle.Render("t: Select Template | f: Choose File | h: View History | Esc: Menu")

	case statementHistoryView:
		s += headerStyle.Render("Bank Statement History") + "\n\n"

		if len(m.store.statements.Statements) == 0 {
			s += faintStyle.Render("No import history found.") + "\n\n"
		} else {
			for i, stmt := range m.store.statements.Statements {
				prefix := "  "
				if i == m.statementIndex {
					prefix = "> "
				}

				statusIcon := "✓"
				statusColor := "10" // Green
				switch stmt.Status {
				case "failed":
					statusIcon = "✗"
					statusColor = "9" // Red
				case "override":
					statusIcon = "⚠"
					statusColor = "208" // Orange
				}

				statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor))

				s += enumeratorStyle.Render(prefix) +
					statusStyle.Render(statusIcon) + " " +
					headerStyle.Render(stmt.Filename) + " " +
					faintStyle.Render(fmt.Sprintf("(%s to %s) - %d txs using %s",
						stmt.PeriodStart, stmt.PeriodEnd, stmt.TxCount, stmt.TemplateUsed)) + "\n"
			}
		}

		s += "\n" + faintStyle.Render("Up/Down: Navigate | Enter: View Details | Esc: Back")

	case statementOverlapView:
		s += headerStyle.Render("Import Overlap Warning") + "\n\n"

		filename := filepath.Base(m.selectedFile)
		s += lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render(
			fmt.Sprintf("⚠ The file '%s' contains transactions that overlap with existing imports:", filename)) + "\n\n"

		for _, stmt := range m.overlappingStmts {
			s += faintStyle.Render(fmt.Sprintf("  • %s (%s to %s) - %d transactions",
				stmt.Filename, stmt.PeriodStart, stmt.PeriodEnd, stmt.TxCount)) + "\n"
		}

		s += "\n" + faintStyle.Render("This may create duplicate transactions in your data.") + "\n\n"

		s += faintStyle.Render("y: Import Anyway | n: Cancel Import | Esc: Cancel")
	}

	return s
}

func (m model) renderBulkCategoryOptions() string {
	var s string
	s += faintStyle.Render("Categories:") + "\n"

	categories := m.store.categories.Categories
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
	style := formFieldStyle
	if m.bulkEditField == fieldType {
		if isEditing {
			style = selectingFieldStyle
		} else {
			style = activeFieldStyle
		}
	}

	displayValue := value
	if isPlaceholder {
		displayValue = placeholder
		style = style.Faint(true)
	}

	return formLabelStyle.Render(label) + "\n" + style.Render(displayValue)
}

func (m model) renderBulkCategoryField() string {
	var s string

	// Category dropdown field
	categoryStyle := formFieldStyle
	if m.bulkEditField == bulkEditCategory {
		if m.isBulkSelectingCategory {
			categoryStyle = selectingFieldStyle
		} else {
			categoryStyle = activeFieldStyle
		}
	}

	categoryValue := m.bulkCategoryValue
	if m.bulkEditField == bulkEditCategory && m.isBulkSelectingCategory {
		categoryValue = "▼ Select Category"
	}
	if m.bulkCategoryIsPlaceholder || categoryValue == "" {
		categoryValue = "Select category to change"
		categoryStyle = categoryStyle.Faint(true)
	}

	s += formLabelStyle.Render("Category:") + "\n" + categoryStyle.Render(categoryValue)

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

	s += faintStyle.Render("Up/Down: Navigate | Enter: Edit Field | Ctrl+S: Save Split | Esc: Cancel Split")
	return s
}

func (m model) renderSplitField(label, value string, fieldType uint) string {
	style := formFieldStyle
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
		displayValue = value
	case splitCategory2Field:
		isEditing = m.isSplitSelectingCategory2
		displayValue = value
	default:
		displayValue = value
	}

	if m.splitField == fieldType {
		if isEditing {
			style = selectingFieldStyle // Orange border when editing
		} else {
			style = activeFieldStyle // Highlighted when selected
		}
	}

	// Category specific display logic
	if fieldType == splitCategory1Field && m.isSplitSelectingCategory1 {
		displayValue = "▼ Select Category"
	} else if fieldType == splitCategory2Field && m.isSplitSelectingCategory2 {
		displayValue = "▼ Select Category"
	}

	result := formLabelStyle.Render(label) + style.Render(displayValue)

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

	categories := m.store.categories.Categories
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

	// Amount field
	amountStyle := formFieldStyle
	if m.editField == editAmount {
		if m.isEditingAmount {
			amountStyle = selectingFieldStyle
		} else {
			amountStyle = activeFieldStyle
		}
	}

	amountValue := fmt.Sprintf("%.2f", m.currTransaction.Amount)
	if m.isEditingAmount && m.editingAmountStr != "" {
		amountValue = m.editingAmountStr
	}

	s += formLabelStyle.Render("Amount:") + "\n" + amountStyle.Render(amountValue) + "\n\n"

	// Description field
	descStyle := formFieldStyle
	if m.editField == editDescription {
		if m.isEditingDescription {
			descStyle = selectingFieldStyle
		} else {
			descStyle = activeFieldStyle
		}
	}

	descValue := m.currTransaction.Description
	if m.isEditingDescription && m.editingDescStr != "" {
		descValue = m.editingDescStr
	}

	s += formLabelStyle.Render("Description:") + "\n" + descStyle.Render(descValue) + "\n\n"

	// Date field
	dateStyle := formFieldStyle
	if m.editField == editDate {
		if m.isEditingDate {
			dateStyle = selectingFieldStyle
		} else {
			dateStyle = activeFieldStyle
		}
	}

	dateValue := m.currTransaction.Date
	if m.isEditingDate && m.editingDateStr != "" {
		dateValue = m.editingDateStr
	}

	s += formLabelStyle.Render("Date:") + "\n" + dateStyle.Render(dateValue) + "\n\n"

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
	}

	s += formLabelStyle.Render("Type:") + "\n" + typeStyle.Render(typeValue) + "\n"

	// Show type options when selecting
	if m.editField == editType && m.isSelectingType {
		s += m.renderTypeOptions() + "\n"
	}

	s += "\n"

	// Category field with selection - Fix the dropdown display logic
	categoryStyle := formFieldStyle
	if m.editField == editCategory {
		if m.isSelectingCategory {
			categoryStyle = selectingFieldStyle
		} else {
			categoryStyle = activeFieldStyle
		}
	}

	categoryValue := m.currTransaction.Category
	if m.editField == editCategory && m.isSelectingCategory {
		categoryValue = "▼ Select Category"
	}

	s += formLabelStyle.Render("Category:") + "\n" + categoryStyle.Render(categoryValue) + "\n"

	// Show category options when selecting - Fix this condition
	if m.editField == editCategory && m.isSelectingCategory {
		s += m.renderCategoryOptions() + "\n"
	}

	s += "\n" + faintStyle.Render("Up/Down: Navigate | Enter: Edit Field | Ctrl+S: Save Transaction | s: Split | Esc: Cancel")
	return s
}

func (m model) renderCategoryOptions() string {
	var s string
	s += faintStyle.Render("Categories:") + "\n"

	// Calculate available height for scrolling
	maxVisible := 5
	categories := m.store.categories.Categories

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
