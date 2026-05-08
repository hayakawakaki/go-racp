CREATE TABLE IF NOT EXISTS cp_sessions (
    token_hash    BINARY(32)        NOT NULL PRIMARY KEY,
    user_id       INT(11) UNSIGNED  NOT NULL,
    expires_at    DATETIME          NOT NULL,
    last_seen_at  DATETIME          NOT NULL,
    created_at    DATETIME          NOT NULL,
    KEY idx_cp_sessions_user    (user_id),
    KEY idx_cp_sessions_expires (expires_at)
) ENGINE=InnoDB;
