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

CREATE INDEX idx_cp_deposit_processed_recent ON cp_deposit_processed (processed_at DESC, deposit_id DESC);

CREATE INDEX idx_cp_deposit_processed_account ON cp_deposit_processed (account_id, processed_at DESC, deposit_id DESC);

CREATE TABLE cp_withdraw_requests (
    id           BIGSERIAL   NOT NULL,
    account_id   INTEGER     NOT NULL,
    zeny         BIGINT      NOT NULL DEFAULT 0,
    cashpoint    INTEGER     NOT NULL DEFAULT 0,
    status       SMALLINT    NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at      TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    PRIMARY KEY (id),
    CONSTRAINT cp_withdraw_requests_amounts CHECK (zeny >= 0 AND cashpoint >= 0)
);

CREATE INDEX idx_cp_withdraw_requests_pending ON cp_withdraw_requests (id) WHERE status = 1;

CREATE INDEX idx_cp_withdraw_requests_account ON cp_withdraw_requests (account_id, id DESC);

CREATE INDEX idx_cp_withdraw_requests_sent ON cp_withdraw_requests (sent_at) WHERE status = 2;

-- +goose Down
DROP TABLE IF EXISTS cp_withdraw_requests;
DROP TABLE IF EXISTS cp_deposit_processed;
DROP TABLE IF EXISTS cp_currency;
