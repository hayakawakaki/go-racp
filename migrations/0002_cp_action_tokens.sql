-- +goose Up
CREATE TABLE cp_action_tokens (
    token_hash    BYTEA       NOT NULL PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    account_id    INTEGER     NOT NULL,
    action        SMALLINT    NOT NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    consumed_at   TIMESTAMPTZ NULL,
    created_at    TIMESTAMPTZ NOT NULL,
    payload       BYTEA       NULL
);
CREATE INDEX idx_cp_action_tokens_account ON cp_action_tokens (account_id);
CREATE INDEX idx_cp_action_tokens_expires ON cp_action_tokens (expires_at);

-- +goose Down
DROP TABLE IF EXISTS cp_action_tokens;
