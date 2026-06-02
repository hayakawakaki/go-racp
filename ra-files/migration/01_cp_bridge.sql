CREATE TABLE IF NOT EXISTS cp_deposit (
    id         BIGINT NOT NULL AUTO_INCREMENT,
    account_id INT    NOT NULL,
    zeny       BIGINT NOT NULL DEFAULT 0,
    points     INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS cp_withdraw (
    id           BIGINT NOT NULL,
    account_id   INT    NOT NULL,
    zeny         BIGINT NOT NULL DEFAULT 0,
    points       INT    NOT NULL DEFAULT 0,
    delivered_at BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (id),
    KEY idx_cp_withdraw_delivered (delivered_at)
);
