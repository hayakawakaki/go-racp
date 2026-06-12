-- +goose Up
CREATE TABLE cp_currency_hold (
    id         BIGSERIAL   NOT NULL,
    account_id INTEGER     NOT NULL,
    zeny       BIGINT      NOT NULL DEFAULT 0,
    cashpoint  INTEGER     NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id),
    CONSTRAINT cp_currency_hold_nonneg CHECK (zeny >= 0 AND cashpoint >= 0)
);

CREATE INDEX idx_cp_currency_hold_account ON cp_currency_hold (account_id);

-- +goose Down
DROP TABLE IF EXISTS cp_currency_hold;
