-- Finance Tracker SQLite Schema

PRAGMA foreign_keys = ON;

-- Transaction Audit Events for tracking all interactions with transactions
CREATE TABLE transaction_audit_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    transaction_id INTEGER NOT NULL,
    bank_statement_id INTEGER NOT NULL,
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    action_type TEXT NOT NULL,
    source TEXT NOT NULL,
    description_fingerprint TEXT NOT NULL,
    category_assigned INTEGER NOT NULL,
    category_confidence DECIMAL(3,2), -- wait to implement
    previous_category INTEGER NOT NULL,
    modification_reason TEXT, -- "description", "transaction type", "category"
    pre_edit_snapshot TEXT, -- json transaction state
    post_edit_snapshot TEXT, -- json transaction state
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (transaction_id) REFERENCES transactions(id) ON DELETE CASCADE,
    FOREIGN KEY (bank_statement_id) REFERENCES bank_statements(id) ON DELETE CASCADE,
    FOREIGN KEY (category_assigned) REFERENCES categories(id) ON DELETE RESTRICT,
    FOREIGN KEY (previous_category) REFERENCES categories(id) ON DELETE RESTRICT,
    CHECK (action_type IN ('edit', 'import', 'split')),
    CHECK (source IN ('user', 'import', 'auto')),
    CHECK (modification_reason IS NULL OR modification_reason IN ('description', 'transaction type', 'category')),
    CHECK (category_confidence IS NULL OR (category_confidence >= 0.0 AND category_confidence <= 1.0)),
    CHECK (length(action_type) > 0),
    CHECK (length(source) > 0)
); 

-- Categories table with hierarchical support
CREATE TABLE categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    display_name TEXT NOT NULL UNIQUE,
    parent_id INTEGER,
    color TEXT,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_id) REFERENCES categories(id) ON DELETE SET NULL,
    CHECK (length(display_name) > 0),
    CHECK (is_active IN (0, 1))
);

-- CSV Templates for bank statement imports
CREATE TABLE csv_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    post_date_column INTEGER NOT NULL,
    amount_column INTEGER NOT NULL,
    desc_column INTEGER NOT NULL,
    category_column INTEGER,
    has_header BOOLEAN NOT NULL DEFAULT 0,
    date_format TEXT DEFAULT '2006-01-02',
    delimiter TEXT NOT NULL DEFAULT ',',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (length(name) > 0),
    CHECK (post_date_column >= 0),
    CHECK (amount_column >= 0),
    CHECK (desc_column >= 0),
    CHECK (category_column IS NULL OR category_column >= 0),
    CHECK (has_header IN (0, 1)),
    CHECK (length(delimiter) > 0)
);

-- Bank statements tracking imports
CREATE TABLE bank_statements (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    filename TEXT NOT NULL,
    import_date DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    period_start DATE,
    period_end DATE,
    template_used INTEGER NOT NULL,
    tx_count INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'completed',
    processing_time INTEGER, -- milliseconds
    error_log TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (template_used) REFERENCES csv_templates(id) ON DELETE RESTRICT,
    CHECK (length(filename) > 0),
    CHECK (tx_count >= 0),
    CHECK (status IN ('completed', 'failed', 'override', 'undone', 'importing')),
    CHECK (processing_time IS NULL OR processing_time >= 0)
);

-- Transactions table with comprehensive tracking
CREATE TABLE transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_id INTEGER,
    amount DECIMAL(15,2) NOT NULL,
    description TEXT NOT NULL,
    raw_description TEXT,
    date DATE NOT NULL,
    category_id INTEGER NOT NULL,
    transaction_type TEXT DEFAULT 'expense',
    is_split BOOLEAN NOT NULL DEFAULT 0,
    statement_id INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_id) REFERENCES transactions(id) ON DELETE CASCADE,
    FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE RESTRICT,
    FOREIGN KEY (statement_id) REFERENCES bank_statements(id) ON DELETE SET NULL,
    CHECK (amount != 0),
    CHECK (length(description) > 0),
    CHECK (transaction_type IN ('expense', 'income', 'transfer')),
    CHECK (is_split IN (0, 1))
);

-- Indexes for performance optimization
-- Transaction lookups by date range
CREATE INDEX idx_transactions_date ON transactions(date);
-- Transaction categorization queries  
CREATE INDEX idx_transactions_category ON transactions(category_id);
-- Statement-based queries for bulk operations
CREATE INDEX idx_transactions_statement ON transactions(statement_id);
-- Split transaction relationships
CREATE INDEX idx_transactions_parent ON transactions(parent_id);
-- Category hierarchy queries
CREATE INDEX idx_categories_parent ON categories(parent_id);
-- Active category filtering
CREATE INDEX idx_categories_active ON categories(is_active);
-- Transaction audit event lookups by transaction
CREATE INDEX idx_transaction_audit_events_transaction ON transaction_audit_events(transaction_id);
-- Transaction audit event lookups by bank statement
CREATE INDEX idx_transaction_audit_events_statement ON transaction_audit_events(bank_statement_id);
-- Transaction audit event chronological queries
CREATE INDEX idx_transaction_audit_events_timestamp ON transaction_audit_events(timestamp);
-- Transaction audit event filtering by action type
CREATE INDEX idx_transaction_audit_events_action ON transaction_audit_events(action_type);

-- Triggers for automatic updated_at maintenance
CREATE TRIGGER update_categories_updated_at
    AFTER UPDATE ON categories
    FOR EACH ROW
    WHEN NEW.updated_at = OLD.updated_at
BEGIN
    UPDATE categories SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER update_transactions_updated_at
    AFTER UPDATE ON transactions
    FOR EACH ROW
    WHEN NEW.updated_at = OLD.updated_at
BEGIN
    UPDATE transactions SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER update_csv_templates_updated_at
    AFTER UPDATE ON csv_templates
    FOR EACH ROW
    WHEN NEW.updated_at = OLD.updated_at
BEGIN
    UPDATE csv_templates SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER update_bank_statements_updated_at
    AFTER UPDATE ON bank_statements
    FOR EACH ROW
    WHEN NEW.updated_at = OLD.updated_at
BEGIN
    UPDATE bank_statements SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Insert default "Uncategorized" category
INSERT INTO categories (id, display_name, is_active, created_at, updated_at)
VALUES (1, 'Uncategorized', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

-- Schema version tracking
CREATE TABLE schema_version (
    version INTEGER NOT NULL,
    applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO schema_version (version) VALUES (1);