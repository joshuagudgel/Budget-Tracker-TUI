package types

// AnalyticsSummary represents aggregated transaction data for reporting
type AnalyticsSummary struct {
	DateRange         string
	TotalIncome       float64
	TotalExpenses     float64
	NetAmount         float64
	TransactionCount  int
	CategoryBreakdown []CategorySpending
}

// CategorySpending represents spending breakdown by category
type CategorySpending struct {
	CategoryName     string
	Amount           float64
	Percentage       float64
	TransactionCount int
}
