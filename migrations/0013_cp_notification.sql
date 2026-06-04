-- +goose Up
CREATE TABLE cp_notification (
    id         BIGSERIAL   NOT NULL,
    account_id INTEGER     NOT NULL,
    category   TEXT        NOT NULL,
    title      TEXT        NOT NULL,
    body       TEXT        NOT NULL DEFAULT '',
    link       TEXT        NOT NULL DEFAULT '',
    read_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id)
);

CREATE INDEX idx_cp_notification_account ON cp_notification (account_id, created_at DESC, id DESC);

CREATE INDEX idx_cp_notification_unread ON cp_notification (account_id) WHERE read_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS cp_notification;
