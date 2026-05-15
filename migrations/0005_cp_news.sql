-- +goose Up
CREATE TABLE cp_news (
    id          BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title       VARCHAR(200) NOT NULL,
    body        TEXT         NOT NULL,
    category    VARCHAR(64)  NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cp_news_created          ON cp_news (created_at DESC);
CREATE INDEX idx_cp_news_category_created ON cp_news (category, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS cp_news;
