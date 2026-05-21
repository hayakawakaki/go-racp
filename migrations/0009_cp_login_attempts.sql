-- +goose Up
CREATE TABLE cp_login_attempts (
    id           BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username     VARCHAR(64)  NOT NULL,
    account_id   BIGINT       NULL,
    ip           INET         NOT NULL,
    user_agent   TEXT         NOT NULL DEFAULT '',
    success      BOOLEAN      NOT NULL,
    attempted_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cp_login_attempts_username_lc_time
    ON cp_login_attempts (LOWER(username), attempted_at);

-- +goose Down
DROP TABLE IF EXISTS cp_login_attempts;
