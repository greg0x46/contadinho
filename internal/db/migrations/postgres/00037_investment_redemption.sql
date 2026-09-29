-- +goose Up

ALTER TABLE investment_operations ADD COLUMN principal_amount TEXT NOT NULL DEFAULT '0';
ALTER TABLE investment_operations ADD COLUMN income_amount TEXT NOT NULL DEFAULT '0';
ALTER TABLE investment_operations DROP CONSTRAINT investment_operations_kind_check;
ALTER TABLE investment_operations ADD CONSTRAINT investment_operations_kind_check CHECK (kind IN (
    'initial_balance', 'deposit', 'withdrawal', 'redemption', 'buy', 'sell', 'income', 'fee', 'tax', 'valuation', 'transfer_out', 'transfer_in'
));

-- +goose Down

-- This constraint deliberately refuses rollback while redemptions exist.
ALTER TABLE investment_operations DROP CONSTRAINT investment_operations_kind_check;
ALTER TABLE investment_operations ADD CONSTRAINT investment_operations_kind_check CHECK (kind IN (
    'initial_balance', 'deposit', 'withdrawal', 'buy', 'sell', 'income', 'fee', 'tax', 'valuation', 'transfer_out', 'transfer_in'
));
ALTER TABLE investment_operations DROP COLUMN principal_amount;
ALTER TABLE investment_operations DROP COLUMN income_amount;
