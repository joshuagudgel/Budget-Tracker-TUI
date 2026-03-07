-- Finance Tracker SQLite Schema

PRAGMA foreign_keys = ON;

-- Audit Events for tracking all interactions with entities
CREATE TABLE audit_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    entity_type TEXT NOT NULL,
    entity_id INTEGER NOT NULL, 
    event_type TEXT NOT NULL,
    field_name TEXT,
    old_value TEXT,
    new_value TEXT,
    source TEXT NOT NULL,
    context TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (entity_type IN ('Transaction', 'Category', 'BankStatement')),
    CHECK (event_type IN ('CREATE', 'UPDATE', 'DELETE', 'SPLIT', 'IMPORT')),
    CHECK (source IN ('user', 'auto', 'import')),
    CHECK (length(entity_type) > 0),
    CHECK (length(event_type) > 0),
    CHECK (length(source) > 0)
);

-- Audit Events for future ML integration 

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
    merchant_column INTEGER,
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
    CHECK (merchant_column IS NULL OR merchant_column >= 0),
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
    auto_category TEXT,
    transaction_type TEXT DEFAULT 'expense',
    is_split BOOLEAN NOT NULL DEFAULT 0,
    is_recurring BOOLEAN NOT NULL DEFAULT 0,
    statement_id INTEGER,
    confidence DECIMAL(3,2), -- ML prediction confidence 0.00-1.00
    user_modified BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_id) REFERENCES transactions(id) ON DELETE CASCADE,
    FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE RESTRICT,
    FOREIGN KEY (statement_id) REFERENCES bank_statements(id) ON DELETE SET NULL,
    CHECK (amount != 0),
    CHECK (length(description) > 0),
    CHECK (transaction_type IN ('expense', 'income', 'transfer')),
    CHECK (is_split IN (0, 1)),
    CHECK (is_recurring IN (0, 1)),
    CHECK (confidence IS NULL OR (confidence >= 0.0 AND confidence <= 1.0)),
    CHECK (user_modified IN (0, 1))
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
-- Audit event lookups by entity
CREATE INDEX idx_audit_events_entity ON audit_events(entity_type, entity_id);
-- Audit event chronological queries
CREATE INDEX idx_audit_events_timestamp ON audit_events(timestamp);
-- Audit event filtering by type
CREATE INDEX idx_audit_events_type ON audit_events(event_type);

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

CREATE TRIGGER update_audit_events_updated_at
    AFTER UPDATE ON audit_events
    FOR EACH ROW
    WHEN NEW.created_at = OLD.created_at
BEGIN
    UPDATE audit_events SET created_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
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