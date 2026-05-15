-- +goose Up
CREATE TABLE cp_account_record (
    account_id   INTEGER     NOT NULL,
    change_type  SMALLINT    NOT NULL,
    changed_at   TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (account_id, change_type)
);

-- +goose Down
DROP TABLE IF EXISTS cp_account_record;
