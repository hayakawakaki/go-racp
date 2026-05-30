-- +goose Up
DROP TABLE IF EXISTS cp_store_orders;
DROP TABLE IF EXISTS cp_store_items;

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

-- +goose Down
DROP TABLE IF EXISTS cp_purchases;

CREATE TABLE cp_store_items (
    id                BIGSERIAL   NOT NULL,
    cashpoint_value   INTEGER     NOT NULL,
    bonus_cashpoint   INTEGER     NOT NULL DEFAULT 0,
    price_minor       BIGINT      NOT NULL,
    currency_code     VARCHAR(3)  NOT NULL,
    per_account_limit INTEGER,
    sale_starts_at    TIMESTAMPTZ,
    sale_ends_at      TIMESTAMPTZ,
    enabled           BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id),
    CONSTRAINT cp_store_items_value_pos CHECK (cashpoint_value > 0),
    CONSTRAINT cp_store_items_bonus_nonneg CHECK (bonus_cashpoint >= 0),
    CONSTRAINT cp_store_items_price_pos CHECK (price_minor > 0)
);

CREATE TABLE cp_store_orders (
    id                BIGSERIAL   NOT NULL,
    account_id        INTEGER     NOT NULL,
    store_item_id     BIGINT      NOT NULL REFERENCES cp_store_items(id),
    amount            INTEGER     NOT NULL,
    unit_cashpoint    INTEGER     NOT NULL,
    total_cashpoint   BIGINT      NOT NULL,
    unit_price_minor  BIGINT      NOT NULL,
    total_price_minor BIGINT      NOT NULL,
    currency_code     VARCHAR(3)  NOT NULL,
    payment_status    SMALLINT    NOT NULL DEFAULT 1,
    payment_ref       TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at           TIMESTAMPTZ,
    PRIMARY KEY (id),
    CONSTRAINT cp_store_orders_amount_pos CHECK (amount > 0)
);

CREATE INDEX idx_cp_store_orders_paid_cap ON cp_store_orders (account_id, store_item_id) WHERE payment_status = 2;
