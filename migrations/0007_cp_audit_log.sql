-- +goose Up
CREATE TABLE cp_audit_log (
    id             BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_user_id  INTEGER     NOT NULL,
    target_user_id INTEGER     NOT NULL,
    action         VARCHAR(32) NOT NULL,
    reason         TEXT        NOT NULL DEFAULT '',
    before_value   TEXT        NOT NULL DEFAULT '',
    after_value    TEXT        NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cp_audit_log_target ON cp_audit_log (target_user_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS cp_audit_log;
