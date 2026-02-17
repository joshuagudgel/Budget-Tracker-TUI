package types

type Transaction struct {
	Id              int64   `json:"id"`
	ParentId        *int64  `json:"parentId,omitempty"`
	Amount          float64 `json:"amount"`
	Description     string  `json:"description"`
	RawDescription  string  `json:"rawDescription"`
	Date            string  `json:"date"`
	Category        string  `json:"category"`
	AutoCategory    string  `json:"autoCategory"`
	TransactionType string  `json:"transactionType"`
	IsSplit         bool    `json:"isSplit"`
	IsRecurring     bool    `json:"isRecurring"`
	StatementId     string  `json:"statementId"`
	Confidence      float64 `json:"confidence,omitempty"`
	UserModified    bool    `json:"userModified,omitempty"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

type Category struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	ParentId    *int64 `json:"parentId,omitempty"`
	IsDefault   bool   `json:"isDefault,omitempty"`
	Color       string `json:"color,omitempty"`
}

type BankStatement struct {
	Id             int64  `json:"id"`
	Filename       string `json:"filename"`
	ImportDate     string `json:"importDate"`
	PeriodStart    string `json:"periodStart"`
	PeriodEnd      string `json:"periodEnd"`
	TemplateUsed   string `json:"templateUsed"`
	TxCount        int    `json:"txCount"`
	Status         string `json:"status"`
	ProcessingTime int64  `json:"processingTime,omitempty"`
	ErrorLog       string `json:"errorLog,omitempty"`
}

type CSVTemplate struct {
	Name           string `json:"name"`
	DateColumn     int    `json:"dateColumn"`
	AmountColumn   int    `json:"amountColumn"`
	DescColumn     int    `json:"descColumn"`
	HasHeader      bool   `json:"hasHeader"`
	DateFormat     string `json:"dateFormat,omitempty"`
	MerchantColumn *int   `json:"merchantColumn,omitempty"`
	Delimiter      string `json:"delimiter,omitempty"`
}

type ImportResult struct {
	Success          bool
	ImportedCount    int
	OverlapDetected  bool
	OverlappingStmts []BankStatement
	PeriodStart      string
	PeriodEnd        string
	Message          string
	Filename         string
}
