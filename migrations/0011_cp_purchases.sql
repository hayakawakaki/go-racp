-- +goose Up
CREATE TABLE cp_purchases (
    id BIGSERIAL PRIMARY KEY,
    account_id INTEGER NOT NULL,
    package_key TEXT NOT NULL,
    provider TEXT NOT NULL,
    provider_ref TEXT,
    provider_payment_id TEXT,
    amount BIGINT NOT NULL,
    currency TEXT NOT NULL,
    cash_points INTEGER NOT NULL,
    status SMALLINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    disputed_at TIMESTAMPTZ,
    refunded_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    CONSTRAINT cp_purchases_status_valid CHECK (status IN (1, 2, 3, 4, 5)),
    CONSTRAINT cp_purchases_amount_pos CHECK (amount > 0),
    CONSTRAINT cp_purchases_points_pos CHECK (cash_points > 0)
);

CREATE UNIQUE INDEX idx_cp_purchases_provider_ref ON cp_purchases (provider, provider_ref) WHERE provider_ref IS NOT NULL;
CREATE INDEX idx_cp_purchases_account ON cp_purchases (account_id);
CREATE INDEX idx_cp_purchases_payment ON cp_purchases (provider, provider_payment_id) WHERE provider_payment_id IS NOT NULL;
CREATE INDEX idx_cp_purchases_status ON cp_purchases (status);
CREATE INDEX idx_cp_purchases_completed_revenue ON cp_purchases (completed_at) WHERE status = 2;
CREATE INDEX idx_cp_purchases_created_at ON cp_purchases (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS cp_purchases;
