package ui

import tea "github.com/charmbracelet/bubbletea"

// Menu view handler
func (m model) handleMenuView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "t":
		m.state = listView
	case "r":
		m.state = backupView
		m.backupMessage = ""
	case "i":
		m.state = bankStatementView
		m.statementMessage = ""
	case "c":
		m.state = categoryListView
		m.categoryMessage = ""
		m.selectedCategoryIdx = 0
		return m, m.loadCategories()
	case "q":
		return m, tea.Quit
	}
	return m, nil
}
