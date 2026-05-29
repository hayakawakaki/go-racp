-- +goose Up
CREATE TABLE cp_currency (
    account_id   INTEGER     NOT NULL,
    zeny         BIGINT      NOT NULL DEFAULT 0,
    cashpoint    INTEGER     NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    PRIMARY KEY (account_id),
    CONSTRAINT cp_currency_zeny_nonneg CHECK (zeny >= 0),
    CONSTRAINT cp_currency_cashpoint_nonneg CHECK (cashpoint >= 0)
);

CREATE TABLE cp_deposit_processed (
    deposit_id   BIGINT      NOT NULL,
    account_id   INTEGER     NOT NULL,
    zeny         BIGINT      NOT NULL,
    cashpoint    INTEGER     NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (deposit_id)
);

CREATE TABLE cp_withdraw_requests (
    id         BIGSERIAL   NOT NULL,
    account_id INTEGER     NOT NULL,
    zeny       BIGINT      NOT NULL DEFAULT 0,
    cashpoint  INTEGER     NOT NULL DEFAULT 0,
    status     SMALLINT    NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at    TIMESTAMPTZ,
    PRIMARY KEY (id),
    CONSTRAINT cp_withdraw_requests_amounts CHECK (zeny >= 0 AND cashpoint >= 0)
);

CREATE INDEX idx_cp_withdraw_requests_pending ON cp_withdraw_requests (id) WHERE status = 1;

CREATE INDEX idx_cp_withdraw_requests_account ON cp_withdraw_requests (account_id, id DESC);

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

-- +goose Down
DROP TABLE IF EXISTS cp_store_orders;
DROP TABLE IF EXISTS cp_store_items;
DROP TABLE IF EXISTS cp_withdraw_requests;
DROP TABLE IF EXISTS cp_deposit_processed;
DROP TABLE IF EXISTS cp_currency;
