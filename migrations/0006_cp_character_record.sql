-- +goose Up
CREATE TABLE cp_character_record (
    char_id      INTEGER     NOT NULL,
    change_type  SMALLINT    NOT NULL,
    changed_at   TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (char_id, change_type)
);

-- +goose Down
DROP TABLE IF EXISTS cp_character_record;
