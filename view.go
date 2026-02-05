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
	s := appNameStyle.Render("Budget Tracker") + "\n\n"

	// Add multi-select indicator
	if m.isMultiSelectMode {
		s += headerStyle.Render(fmt.Sprintf("MULTI-SELECT MODE (%d selected)", len(m.selectedTxIds))) + "\n"
	}

	s += faintStyle.Render(fmt.Sprintf("DEBUG: State=%d, Index=%d, Count=%d", m.state, m.listIndex, len(m.transactions))) + "\n\n"

	switch m.state {
	case menuView:
		s += headerStyle.Render("Transactions ('t')") + "\n"
		s += headerStyle.Render("Categories ('c')") + "\n"
		s += headerStyle.Render("Restore ('r')") + "\n"
		s += headerStyle.Render("Import ('i')") + "\n"
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
			s += faintStyle.Render("No transactions found.")
		} else {
			scrollInfo := ""
			if len(m.transactions) > availableHeight {
				scrollInfo = fmt.Sprintf(" (%d/%d)", m.listIndex+1, len(m.transactions))
			}

			// Updated help text based on mode
			if m.isMultiSelectMode {
				s += faintStyle.Render("Enter: Toggle Selection | b: Bulk Edit | m: Exit Multi-Select | Esc: Menu" + scrollInfo)
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

		if m.categoryMessage != "" {
			if strings.Contains(m.categoryMessage, "Error") {
				s += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.categoryMessage) + "\n\n"
			} else {
				s += lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(m.categoryMessage) + "\n\n"
			}
		}

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
		s += formLabelStyle.Render("Name:") + nameStyle.Render(m.newCategory.Name) + "\n\n"

		// Display Name field
		displayStyle := formFieldStyle
		if m.createCategoryField == createCategoryDisplayName {
			displayStyle = activeFieldStyle
		}
		s += formLabelStyle.Render("Display Name:") + displayStyle.Render(m.newCategory.DisplayName) + "\n\n"

		s += faintStyle.Render("Up/Down: Navigate fields | Enter: Save | Esc: Cancel")
	case backupView:
		s += headerStyle.Render("Backup Options:") + "\n\n"

		s += faintStyle.Render("r: Restore from backup | Esc: Return to menu") + "\n\n"
	case importView:
		s += headerStyle.Render("Import Options:") + "\n\n"

		// Show current selected profile
		currentProfile := m.store.csvProfiles.Default
		if currentProfile == "" && len(m.store.csvProfiles.Profiles) > 0 {
			currentProfile = m.store.csvProfiles.Profiles[0].Name
		}
		s += faintStyle.Render(fmt.Sprintf("Current CSV Profile: %s", currentProfile)) + "\n\n"

		if m.importMessage != "" {
			if strings.Contains(m.importMessage, "Error") {
				s += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.importMessage) + "\n\n"
			} else {
				s += lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(m.importMessage) + "\n\n"
			}
		}

		s += faintStyle.Render("i: Import CSV file | p: Select CSV Profile | Esc: Return to menu") + "\n\n"
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
	case csvProfileView:
		s += headerStyle.Render("Select CSV Profile") + "\n\n"

		if len(m.store.csvProfiles.Profiles) == 0 {
			s += faintStyle.Render("No CSV profiles found.") + "\n\n"
		} else {
			// Display available profiles
			for i, profile := range m.store.csvProfiles.Profiles {
				prefix := "  "
				if i == m.profileIndex {
					prefix = "> "
				}

				// Show current default
				suffix := ""
				if profile.Name == m.store.csvProfiles.Default {
					suffix = " (current)"
				}

				// Show profile details
				profileDetails := fmt.Sprintf("%s - Date:%d, Amount:%d, Desc:%d, Header:%v%s",
					profile.Name, profile.DateColumn, profile.AmountColumn, profile.DescColumn, profile.HasHeader, suffix)

				s += enumeratorStyle.Render(prefix) + profileDetails + "\n"
			}
		}

		s += "\n" + faintStyle.Render("Up/Down: Navigate | Enter: Select | c: Create Profile | Esc: Cancel")
	case createProfileView:
		s += headerStyle.Render("Create CSV Profile") + "\n\n"

		if m.createMessage != "" {
			s += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.createMessage) + "\n\n"
		}

		// Profile Name field
		nameStyle := formFieldStyle
		if m.createField == createProfileName {
			nameStyle = activeFieldStyle
		}
		s += formLabelStyle.Render("Profile Name:") + nameStyle.Render(m.newProfile.Name) + "\n\n"

		// Date Column field
		dateStyle := formFieldStyle
		if m.createField == createProfileDate {
			dateStyle = activeFieldStyle
		}
		s += formLabelStyle.Render("Date Column:") + dateStyle.Render(fmt.Sprintf("%d", m.newProfile.DateColumn)) + "\n\n"

		// Amount Column field
		amountStyle := formFieldStyle
		if m.createField == createProfileAmount {
			amountStyle = activeFieldStyle
		}
		s += formLabelStyle.Render("Amount Column:") + amountStyle.Render(fmt.Sprintf("%d", m.newProfile.AmountColumn)) + "\n\n"

		// Description Column field
		descStyle := formFieldStyle
		if m.createField == createProfileDesc {
			descStyle = activeFieldStyle
		}
		s += formLabelStyle.Render("Desc Column:") + descStyle.Render(fmt.Sprintf("%d", m.newProfile.DescColumn)) + "\n\n"

		// Has Header field
		headerStyle := formFieldStyle
		if m.createField == createProfileHeader {
			headerStyle = activeFieldStyle
		}
		headerValue := "No"
		if m.newProfile.HasHeader {
			headerValue = "Yes"
		}
		s += formLabelStyle.Render("Has Header:") + headerStyle.Render(headerValue) + " (y/n)\n\n"

		s += faintStyle.Render("Up/Down: Navigate fields | Enter: Save | Esc: Cancel")
	case bulkEditView:
		s += headerStyle.Render(fmt.Sprintf("Bulk Edit %d Transactions", len(m.selectedTxIds))) + "\n\n"

		// Category field
		categoryStyle := formFieldStyle
		if m.bulkEditField == bulkEditCategory {
			categoryStyle = activeFieldStyle
		}
		s += formLabelStyle.Render("Category:") + "\n" + categoryStyle.Render(m.bulkEditValue) + "\n\n"

		// Type field
		typeStyle := formFieldStyle
		if m.bulkEditField == bulkEditType {
			typeStyle = activeFieldStyle
		}
		s += formLabelStyle.Render("Type:") + "\n" + typeStyle.Render(m.bulkEditValue) + "\n\n"

		s += faintStyle.Render("Up/Down: Navigate fields | Enter: Apply to all selected | Esc: Cancel")
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

	s += faintStyle.Render("Up/Down: Navigate | Enter: Save Split | s: Exit Split Mode | Esc: Cancel")
	return s
}

func (m model) renderSplitField(label, value string, fieldType uint) string {
	style := formFieldStyle
	if m.splitField == fieldType {
		style = activeFieldStyle
	}
	return formLabelStyle.Render(label) + style.Render(value)
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
	if m.isEditingAmount {
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
	if m.isEditingDescription {
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
	if m.isEditingDate {
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
	if m.isSelectingType {
		typeValue = "▼ Select Type"
	}

	s += formLabelStyle.Render("Type:") + "\n" + typeStyle.Render(typeValue) + "\n"

	// Show type options when selecting
	if m.isSelectingType {
		s += m.renderTypeOptions() + "\n"
	}

	s += "\n"

	// Category field with selection
	categoryStyle := formFieldStyle
	if m.editField == editCategory {
		if m.isSelectingCategory {
			categoryStyle = selectingFieldStyle
		} else {
			categoryStyle = activeFieldStyle
		}
	}

	categoryValue := m.currTransaction.Category
	if m.isSelectingCategory {
		categoryValue = "▼ Select Category"
	}

	s += formLabelStyle.Render("Category:") + "\n" + categoryStyle.Render(categoryValue) + "\n"

	// Show category options when selecting
	if m.isSelectingCategory {
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
