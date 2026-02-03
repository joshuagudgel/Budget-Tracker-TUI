package main

import (
	"fmt"
	"os"
	"path/filepath"
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
)

func (m model) View() string {
	s := appNameStyle.Render("Budget Tracker") + "\n\n"
	s += faintStyle.Render(fmt.Sprintf("DEBUG: State=%d, Index=%d, Count=%d", m.state, m.listIndex, len(m.transactions))) + "\n\n"

	switch m.state {
	case menuView:
		s += headerStyle.Render("Transactions ('t')") + "\n"
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
			headerStyle.Render("Expense/Transfer") + headerStyle.Render("\n\n")

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

		// Render visible transactions
		for i := startIndex; i < endIndex; i++ {
			t := m.transactions[i]
			prefix := " "
			if i == m.listIndex {
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
			s += faintStyle.Render("Up/Down: Navigate transactions | e: Edit | d: Delete | Esc: Return to menu" + scrollInfo)
		}
	case editView:
		s += headerStyle.Render("Edit Transaction") + "\n\n"

		// Amount field
		amountStyle := formFieldStyle
		if m.editField == editAmount {
			amountStyle = activeFieldStyle
		}
		displayAmount := fmt.Sprintf("%.2f", m.currTransaction.Amount)
		if m.editField == editAmount && m.editAmountStr != "" {
			displayAmount = m.editAmountStr
		}
		s += formLabelStyle.Render("Amount:") + amountStyle.Render(displayAmount) + "\n\n"

		// Description field
		descStyle := formFieldStyle
		if m.editField == editDescription {
			descStyle = activeFieldStyle
		}
		s += formLabelStyle.Render("Description:") + descStyle.Render(m.currTransaction.Description) + "\n\n"

		// Date field
		dateStyle := formFieldStyle
		if m.editField == editDate {
			dateStyle = activeFieldStyle
		}
		s += formLabelStyle.Render("Date:") + dateStyle.Render(m.currTransaction.Date) + "\n\n"

		// Transaction Type field
		typeStyle := formFieldStyle
		if m.editField == editType {
			typeStyle = activeFieldStyle
		}
		s += formLabelStyle.Render("Type:") + typeStyle.Render(m.currTransaction.TransactionType) + "\n\n"

		// Category field
		categoryStyle := formFieldStyle
		if m.editField == editCategory {
			categoryStyle = activeFieldStyle
		}
		s += formLabelStyle.Render("Category:") + categoryStyle.Render(m.currTransaction.Category) + "\n\n"

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
	}

	return s
}
