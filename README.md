# Finances-Wrapped-TUI

A CLI/TUI application made to be lightweight and run on Windows, Linux and MacOS systems.

Upload bank statements, organize transactions and generate spending summaries through a terminal user interface.

## Features

### Core Transaction Management

- **Transaction CRUD**: Create, read, update, and delete transactions with intuitive navigation
- **Multi-select Operations**: Bulk edit multiple transactions using 'm' to toggle selection and 'e' to edit
- **Split Transactions**: Divide transactions into multiple entries while preserving the original
- **Real-time Editing Validation**: Editing field validation with immediate feedback

### Bank Statement Import

- **CSV Import**: Template-based import system for various bank statement formats
- **Overlap Detection**: Automatically detect and prevent duplicate transaction imports
- **Import Templates**: Create and manage custom CSV parsing templates for different banks
- **Bank Statement Management**: Enhanced interface for managing imported statements with undo functionality

### Category Management

- **User-defined Categories**: Create and organize custom transaction categories
- **Category Hierarchy**: Support for parent-child category relationships
- **Auto-categorization**: Intelligent category suggestions based on transaction descriptions
- **Category Validation**: Ensure data integrity with category existence validation

### Analytics

- **Spending Analysis Dashboard**: Comprehensive spending insights accessible via 'a' from main menu
- **Date Range Selection**: Flexible date filtering with default to previous month for monthly workflows
- **Summary Overview**: Total income, expenses, net amount, and transaction count for selected period
- **Category Breakdown**: Detailed spending by category with amounts, percentages, and transaction counts
- **Dynamic Display**: All categories with transactions shown in responsive table layout
- **Positive Values**: Expense amounts displayed as positive values for clearer financial insights

### Data Management

- **Backup & Restore**: Save and restore transaction data to/from saved states
- **Undo Operations**: Complete undo functionality for imports and bulk operations
- **Data Persistence**: SQLite-based storage in the user's home directory
- **Error Handling**: Graceful error handling with user-friendly messages

### User Interface

- **Bubble Tea TUI**: Modern terminal interface with state machine architecture
- **Lipgloss Styling**: Consistent and attractive styling throughout the application
- **Field Editing**: Two-phase editing system (navigation → activation → editing)
- **Quick Actions**: Keyboard shortcuts for common operations ('i' for import, 'b' for statements, 's' for split)

### Navigation & Controls

- **Arrow Keys**: Navigate between fields and menu items
- **Enter/Backspace**: Activate field editing or modify values
- **Ctrl+S**: Save changes in edit modes
- **Esc**: Return to previous view or cancel operations
- **Quick Keys**: Single-letter shortcuts for major functions

# Project Data

Stored within the home directory under ./tui-sweet/finances-wrapped. Import CSV files from any location with the import tool after creating a CSV profile. Choose a location to save backups if you'd like to save snapshot of your app-data.

# Build Dev

go build .

# Run Dev

go run .

# Build Dist

./build.bat
