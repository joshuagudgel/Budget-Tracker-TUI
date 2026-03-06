package validation

import (
	"budget-tracker-tui/internal/types"
	"fmt"
	"time"
)

// Example demonstrates how to use the validation system
func Example() {
	// Available categories for this example
	categories := []types.Category{
		{Id: 1, DisplayName: "Food"},
		{Id: 2, DisplayName: "Transportation"},
		{Id: 3, DisplayName: "Entertainment"},
		{Id: 4, DisplayName: "Utilities"},
		{Id: 5, DisplayName: "Shopping"},
	}

	// Create a transaction validator
	validator := NewTransactionValidator()

	// Example 1: Valid transaction
	validDate, _ := time.Parse("01-02-2006", "02-22-2024")
	validTx := &types.Transaction{
		Amount:      45.99,
		Description: "Lunch at downtown cafe",
		Date:        validDate,
		CategoryId:  1, // Food
	}

	fmt.Println("=== Validating Valid Transaction ===")
	result := validator.ValidateTransaction(validTx, categories)
	if result.IsValid {
		fmt.Println("✓ Transaction is valid!")
	} else {
		fmt.Println("✗ Transaction has errors:")
		for _, err := range result.Errors {
			fmt.Printf("  - %s: %s\n", err.Field, err.Message)
		}
	}

	// Example 2: Invalid transaction
	invalidDate, _ := time.Parse("2006-01-02", "2024-02-22")
	invalidTx := &types.Transaction{
		Amount:      0,           // Error: zero amount
		Description: "",          // Error: empty description
		Date:        invalidDate, // This date is valid, other fields will cause errors
		CategoryId:  99,          // Error: category ID doesn't exist
	}

	fmt.Println("\n=== Validating Invalid Transaction ===")
	result = validator.ValidateTransaction(invalidTx, categories)
	if result.IsValid {
		fmt.Println("✓ Transaction is valid!")
	} else {
		fmt.Println("✗ Transaction has errors:")
		for _, err := range result.Errors {
			fmt.Printf("  - %s: %s\n", err.Field, err.Message)
		}
	}

	// Example 3: Field-by-field validation
	// fmt.Println(\"\\n=== Field-by-field Validation ===\")
	testDate, _ := time.Parse("01-02-2006", "12-31-2024")
	testTx := &types.Transaction{
		Amount:      123.456, // Too many decimals
		Description: "Valid description",
		Date:        testDate,
		CategoryId:  1, // Food
	}

	if err := validator.ValidateField(testTx, "amount", categories); err != nil {
		fmt.Printf("Amount error: %s\n", err)
	}

	if err := validator.ValidateField(testTx, "description", categories); err != nil {
		fmt.Printf("Description error: %s\n", err)
	} else {
		fmt.Println("Description is valid")
	}

	// Example 4: Using individual validators
	fmt.Println("\n=== Individual Validator Usage ===")

	amountValidator := AmountValidator{}
	if err := amountValidator.Validate(99.99); err != nil {
		fmt.Printf("Amount validation failed: %s\n", err)
	} else {
		fmt.Println("Amount $99.99 is valid")
	}

	dateValidator := DateValidator{}
	if err := dateValidator.Validate("02-22-24"); err != nil {
		fmt.Printf("Date validation failed: %s\n", err)
	} else {
		fmt.Println("Date 02-22-24 is valid")
	}

	// Example 5: Category suggestions
	fmt.Println("\n=== Category Suggestions ===")
	categoryValidator := CategoryValidator{}
	suggestions := categoryValidator.GetSuggestions("foo", categories)
	if len(suggestions) > 0 {
		fmt.Print("Suggestions for 'foo': ")
		for _, cat := range suggestions {
			fmt.Printf("%s ", cat.DisplayName)
		}
		fmt.Println()
	} else {
		fmt.Println("No suggestions found for 'foo'")
	}

	suggestions = categoryValidator.GetSuggestions("trans", categories)
	if len(suggestions) > 0 {
		fmt.Print("Suggestions for 'trans': ")
		for _, cat := range suggestions {
			fmt.Printf("%s ", cat.DisplayName)
		}
		fmt.Println()
	}

	// Example 6: Parsing and validating amounts
	fmt.Println("\n=== Amount Parsing ===")
	testAmounts := []string{"$123.45", "99", "50.999", "abc", ""}

	for _, amountStr := range testAmounts {
		if amount, err := amountValidator.ParseAmount(amountStr); err != nil {
			fmt.Printf("'%s' -> Error: %s\n", amountStr, err)
		} else {
			fmt.Printf("'%s' -> %.2f (valid)\n", amountStr, amount)
		}
	}
}

// ValidationUsageInUI demonstrates how to use validation in a TUI context
func ValidationUsageInUI() {
	fmt.Println("\n=== TUI Integration Example ===")

	// Simulate form data from TUI
	formData := map[string]string{
		"amount":      "45.99",
		"description": "Coffee at Starbucks",
		"date":        "02-22-2024",
		"category":    "Food",
	}

	categories := []types.Category{
		{Id: 1, DisplayName: "Food"},
		{Id: 2, DisplayName: "Transportation"},
		{Id: 3, DisplayName: "Entertainment"},
	}
	validator := NewTransactionValidator()

	// Create transaction from form data
	tx := &types.Transaction{}

	// Parse amount (in real TUI, this would come from text input)
	if amount, err := validator.Amount.ParseAmount(formData["amount"]); err != nil {
		fmt.Printf("Amount parsing error: %s\n", err)
		return
	} else {
		tx.Amount = amount
	}

	tx.Description = formData["description"]
	// Parse date from form data
	if parsedDate, err := time.Parse("01-02-2006", formData["date"]); err != nil {
		fmt.Printf("Date parsing error: %s\n", err)
		return
	} else {
		tx.Date = parsedDate
	}
	// In real app, would find category by display name
	tx.CategoryId = 1 // Food category

	// Validate the transaction
	result := validator.ValidateTransaction(tx, categories)

	if result.IsValid {
		fmt.Println("✓ Form data is valid, ready to save transaction")
	} else {
		fmt.Println("✗ Form has validation errors:")
		for _, err := range result.Errors {
			fmt.Printf("  Field '%s': %s\n", err.Field, GetUserFriendlyMessage(fmt.Errorf("%s", err.Message)))
		}
	}
}

// BulkValidationExample shows how to validate multiple transactions
func BulkValidationExample() {
	fmt.Println("\n=== Bulk Validation Example ===")

	categories := []types.Category{
		{Id: 1, DisplayName: "Food"},
		{Id: 2, DisplayName: "Transportation"},
	}
	validator := NewTransactionValidator()

	transaction1Date, _ := time.Parse("01-02-2006", "02-22-2024")
	transaction2Date := time.Time{} // Zero time for invalid date
	transaction3Date, _ := time.Parse("01-02-2006", "02-21-2024")

	transactions := []*types.Transaction{
		{Id: 1, Amount: 25.50, Description: "Lunch", Date: transaction1Date, CategoryId: 1},    // Food
		{Id: 2, Amount: 0, Description: "", Date: transaction2Date, CategoryId: 99},            // Invalid
		{Id: 3, Amount: 15.99, Description: "Bus fare", Date: transaction3Date, CategoryId: 2}, // Transportation
	}

	results := validator.ValidateBulkEdit(transactions, categories)

	for txId, result := range results {
		if result.IsValid {
			fmt.Printf("Transaction %d: ✓ Valid\n", txId)
		} else {
			fmt.Printf("Transaction %d: ✗ %d errors\n", txId, len(result.Errors))
			for _, err := range result.Errors {
				fmt.Printf("  - %s: %s\n", err.Field, err.Message)
			}
		}
	}
}
