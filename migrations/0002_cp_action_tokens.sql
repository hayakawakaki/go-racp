-- +goose Up
CREATE TABLE cp_action_tokens (
    token_hash    BINARY(32)           NOT NULL PRIMARY KEY,
    account_id    INT(11) UNSIGNED     NOT NULL,
    action        TINYINT(3) UNSIGNED  NOT NULL,
    expires_at    DATETIME             NOT NULL,
    consumed_at   DATETIME             NULL,
    created_at    DATETIME             NOT NULL,
    payload       VARBINARY(255)       NULL,
    KEY idx_cp_action_tokens_account (account_id),
    KEY idx_cp_action_tokens_expires (expires_at)
) ENGINE=InnoDB;

-- +goose Down
DROP TABLE IF EXISTS cp_action_tokens;
