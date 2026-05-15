-- +goose Up
CREATE TABLE cp_sessions (
    token_hash    BYTEA       NOT NULL PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    user_id       INTEGER     NOT NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    last_seen_at  TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_cp_sessions_user    ON cp_sessions (user_id);
CREATE INDEX idx_cp_sessions_expires ON cp_sessions (expires_at);

-- +goose Down
DROP TABLE IF EXISTS cp_sessions;
