package ui

import (
	"fmt"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// initAnalytics initializes the analytics view with default date range (previous month)
func (m model) initAnalytics() (model, tea.Cmd) {
	// Set default to previous month
	now := time.Now()
	firstOfCurrentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	m.analyticsStartDate = firstOfCurrentMonth.AddDate(0, -1, 0) // First of previous month
	m.analyticsEndDate = firstOfCurrentMonth.Add(-time.Nanosecond) // Last moment of previous month

	// Initialize date editing strings
	m.editingStartDateStr = m.analyticsStartDate.Format("01/02/2006")
	m.editingEndDateStr = m.analyticsEndDate.Format("01/02/2006") 

	// Initialize table
	columns := []table.Column{
		{Title: "Category", Width: 25},
		{Title: "Amount", Width: 15},
		{Title: "Percentage", Width: 12},
		{Title: "Transactions", Width: 12},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(false),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	t.SetStyles(s)
	m.analyticsTable = t

	// Load initial data
	m.loadAnalyticsData()
	
	return m, nil
}

// loadAnalyticsData loads and refreshes analytics data from storage
func (m *model) loadAnalyticsData() {
	// Get summary data
	summary, err := m.store.GetTransactionSummaryByDateRange(m.analyticsStartDate, m.analyticsEndDate)
	if err != nil {
		m.analyticsMessage = fmt.Sprintf("Error loading summary: %v", err)
		return
	}
	m.analyticsSummary = summary

	// Get category spending data
	categorySpending, err := m.store.GetCategorySpendingByDateRange(m.analyticsStartDate, m.analyticsEndDate)
	if err != nil {
		m.analyticsMessage = fmt.Sprintf("Error loading category data: %v", err)
		return
	}
	m.categorySpending = categorySpending

	// Build table rows
	rows := []table.Row{}
	for _, spending := range categorySpending {
		rows = append(rows, table.Row{
			spending.CategoryName,
			fmt.Sprintf("$%.2f", spending.Amount),
			fmt.Sprintf("%.1f%%", spending.Percentage),
			strconv.Itoa(spending.TransactionCount),
		})
	}

	m.analyticsTable.SetRows(rows)
	
	// Add helpful message about category distribution
	if len(categorySpending) == 1 && categorySpending[0].CategoryName == "uncategorized" {
		m.analyticsMessage = "All expenses are uncategorized. Use main menu 't' to edit transactions and assign categories."
	} else {
		m.analyticsMessage = ""
	}
}

// handleAnalyticsView handles user input in the analytics view
func (m model) handleAnalyticsView(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		// Return to main menu
		m.state = menuView
		return m, nil

	case "r":
		// Refresh data with debug info
		m.analyticsMessage = "Refreshing analytics data..."
		m.loadAnalyticsData()
		
		// Add debug info about categories
		categories, err := m.store.Categories.GetCategories()
		if err == nil {
			m.analyticsMessage = fmt.Sprintf("Loaded analytics. Found %d categories total.", len(categories))
		} else {
			m.analyticsMessage = fmt.Sprintf("Refreshed analytics. Categories error: %v", err)
		}
		return m, nil

	case "s":
		// Edit start date
		m.isEditingStartDate = true
		m.isEditingEndDate = false
		m.analyticsDateField = 0
		return m, nil

	case "e":
		// Edit end date
		m.isEditingEndDate = true
		m.isEditingStartDate = false
		m.analyticsDateField = 1
		return m, nil

	case "enter":
		if m.isEditingStartDate {
			// Parse and set start date
			if date, err := m.parseDateInput(m.editingStartDateStr); err == nil {
				m.analyticsStartDate = date
				m.isEditingStartDate = false
			m.loadAnalyticsData()
			return m, nil
			} else {
				m.analyticsMessage = fmt.Sprintf("Invalid start date format: %v", err)
			}
		} else if m.isEditingEndDate {
			// Parse and set end date
			if date, err := m.parseDateInput(m.editingEndDateStr); err == nil {
				m.analyticsEndDate = date
				m.isEditingEndDate = false
			m.loadAnalyticsData()
			return m, nil
			} else {
				m.analyticsMessage = fmt.Sprintf("Invalid end date format: %v", err)
			}
		}
		return m, nil

	case "backspace":
		if m.isEditingStartDate && len(m.editingStartDateStr) > 0 {
			m.editingStartDateStr = m.editingStartDateStr[:len(m.editingStartDateStr)-1]
		} else if m.isEditingEndDate && len(m.editingEndDateStr) > 0 {
			m.editingEndDateStr = m.editingEndDateStr[:len(m.editingEndDateStr)-1]
		}
		return m, nil

	default:
		// Handle character input for date editing
		if m.isEditingStartDate {
			m.editingStartDateStr += key
		} else if m.isEditingEndDate {
			m.editingEndDateStr += key
		} else {
			// Handle table navigation
			m.analyticsTable, _ = m.analyticsTable.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		}
		return m, nil
	}
}

// parseDateInput parses date input in various formats and returns time.Time
func (m *model) parseDateInput(input string) (time.Time, error) {
	// Try multiple date formats
	formats := []string{
		"01/02/2006", // MM/DD/YYYY
		"1/2/2006",   // M/D/YYYY
		"01-02-2006", // MM-DD-YYYY
		"1-2-2006",   // M-D-YYYY
		"2006-01-02", // YYYY-MM-DD
	}

	for _, format := range formats {
		if date, err := time.Parse(format, input); err == nil {
			return date, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date format")
}