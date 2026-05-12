-- +goose Up
CREATE TABLE cp_account_record (
    account_id   INT(11) UNSIGNED     NOT NULL,
    change_type  TINYINT(3) UNSIGNED  NOT NULL,
    changed_at   DATETIME             NOT NULL,
    PRIMARY KEY (account_id, change_type)
) ENGINE=InnoDB;

-- +goose Down
DROP TABLE IF EXISTS cp_account_record;
