-- +goose Up

CREATE TEMP TABLE redemption_saved_links AS SELECT * FROM investment_reconciliations;
DROP TABLE investment_reconciliations;
CREATE TEMP TABLE redemption_saved_operations AS SELECT * FROM investment_operations;
DROP TABLE investment_operations;
CREATE TABLE investment_operations (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES investment_accounts (id) ON DELETE RESTRICT,
    position_id TEXT REFERENCES investment_positions (id) ON DELETE RESTRICT,
    kind TEXT NOT NULL CHECK (kind IN (
        'initial_balance', 'deposit', 'withdrawal', 'redemption', 'buy', 'sell', 'income', 'fee', 'tax', 'valuation',
        'transfer_out', 'transfer_in'
    )),
    occurred_on TEXT NOT NULL,
    amount TEXT NOT NULL,
    principal_amount TEXT NOT NULL DEFAULT '0',
    income_amount TEXT NOT NULL DEFAULT '0',
    quantity TEXT,
    unit_price TEXT,
    fees TEXT NOT NULL DEFAULT '0',
    taxes TEXT NOT NULL DEFAULT '0',
    notes TEXT,
    source TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'synced')),
    transfer_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX ix_investment_operations_account_date ON investment_operations (account_id, occurred_on, created_at, id);
CREATE INDEX ix_investment_operations_position_date ON investment_operations (position_id, occurred_on, created_at, id);
CREATE INDEX ix_investment_operations_transfer_id ON investment_operations (transfer_id);

INSERT INTO investment_operations (id, account_id, position_id, kind, occurred_on, amount, quantity, unit_price, fees, taxes, notes, source, transfer_id, created_at, updated_at) SELECT id, account_id, position_id, kind, occurred_on, amount, quantity, unit_price, fees, taxes, notes, source, transfer_id, created_at, updated_at FROM redemption_saved_operations;
DROP TABLE redemption_saved_operations;
CREATE TABLE investment_reconciliations (
    id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL REFERENCES investment_operations (id) ON DELETE CASCADE,
    financial_transaction_id TEXT REFERENCES financial_transactions (id) ON DELETE CASCADE,
    financial_investment_transaction_id TEXT REFERENCES financial_investment_transactions (id) ON DELETE CASCADE,
    amount TEXT NOT NULL,
    created_at TEXT NOT NULL,
    CHECK (financial_transaction_id IS NOT NULL OR financial_investment_transaction_id IS NOT NULL)
);
CREATE INDEX ix_investment_reconciliations_operation_id ON investment_reconciliations (operation_id);
CREATE UNIQUE INDEX uq_investment_reconciliation_operation_financial_transaction
    ON investment_reconciliations (operation_id, financial_transaction_id)
    WHERE financial_transaction_id IS NOT NULL;
CREATE UNIQUE INDEX uq_investment_reconciliation_operation_financial_investment_transaction
    ON investment_reconciliations (operation_id, financial_investment_transaction_id)
    WHERE financial_investment_transaction_id IS NOT NULL;
INSERT INTO investment_reconciliations SELECT * FROM redemption_saved_links;
DROP TABLE redemption_saved_links;

-- +goose Down

-- Refuse rollback while detailed redemptions exist; never discard financial history.
CREATE TEMP TABLE redemption_saved_links AS SELECT * FROM investment_reconciliations;
DROP TABLE investment_reconciliations;
CREATE TEMP TABLE redemption_saved_operations AS SELECT * FROM investment_operations;
DROP TABLE investment_operations;
CREATE TABLE investment_operations (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES investment_accounts (id) ON DELETE RESTRICT,
    position_id TEXT REFERENCES investment_positions (id) ON DELETE RESTRICT,
    kind TEXT NOT NULL CHECK (kind IN (
        'initial_balance', 'deposit', 'withdrawal', 'buy', 'sell', 'income', 'fee', 'tax', 'valuation',
        'transfer_out', 'transfer_in'
    )),
    occurred_on TEXT NOT NULL,
    amount TEXT NOT NULL,
    quantity TEXT,
    unit_price TEXT,
    fees TEXT NOT NULL DEFAULT '0',
    taxes TEXT NOT NULL DEFAULT '0',
    notes TEXT,
    source TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'synced')),
    transfer_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX ix_investment_operations_account_date ON investment_operations (account_id, occurred_on, created_at, id);
CREATE INDEX ix_investment_operations_position_date ON investment_operations (position_id, occurred_on, created_at, id);
CREATE INDEX ix_investment_operations_transfer_id ON investment_operations (transfer_id);

INSERT INTO investment_operations (id, account_id, position_id, kind, occurred_on, amount, quantity, unit_price, fees, taxes, notes, source, transfer_id, created_at, updated_at) SELECT id, account_id, position_id, kind, occurred_on, amount, quantity, unit_price, fees, taxes, notes, source, transfer_id, created_at, updated_at FROM redemption_saved_operations;
DROP TABLE redemption_saved_operations;
CREATE TABLE investment_reconciliations (
    id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL REFERENCES investment_operations (id) ON DELETE CASCADE,
    financial_transaction_id TEXT REFERENCES financial_transactions (id) ON DELETE CASCADE,
    financial_investment_transaction_id TEXT REFERENCES financial_investment_transactions (id) ON DELETE CASCADE,
    amount TEXT NOT NULL,
    created_at TEXT NOT NULL,
    CHECK (financial_transaction_id IS NOT NULL OR financial_investment_transaction_id IS NOT NULL)
);
CREATE INDEX ix_investment_reconciliations_operation_id ON investment_reconciliations (operation_id);
CREATE UNIQUE INDEX uq_investment_reconciliation_operation_financial_transaction
    ON investment_reconciliations (operation_id, financial_transaction_id)
    WHERE financial_transaction_id IS NOT NULL;
CREATE UNIQUE INDEX uq_investment_reconciliation_operation_financial_investment_transaction
    ON investment_reconciliations (operation_id, financial_investment_transaction_id)
    WHERE financial_investment_transaction_id IS NOT NULL;
INSERT INTO investment_reconciliations SELECT * FROM redemption_saved_links;
DROP TABLE redemption_saved_links;
