-- +goose Up
CREATE TABLE cp_api_keys (
    id            BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key_hash      BYTEA        NOT NULL CHECK (octet_length(key_hash) = 32),
    name          VARCHAR(100) NOT NULL,
    rate_tier     VARCHAR(64)  NOT NULL,
    last_used_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    revoked_at    TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_cp_api_keys_key_hash ON cp_api_keys (key_hash);

-- +goose Down
DROP TABLE IF EXISTS cp_api_keys;
