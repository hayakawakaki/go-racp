-- +goose Up
CREATE TABLE cp_metric_peak (
    metric      VARCHAR(32) NOT NULL,
    period      VARCHAR(16) NOT NULL,
    window_key  DATE        NOT NULL,
    value       INTEGER     NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (metric, period, window_key)
);

CREATE INDEX idx_cp_metric_peak_lookup ON cp_metric_peak (metric, period, window_key DESC);

CREATE TABLE cp_metric_general_snapshot (
    captured_at      TIMESTAMPTZ NOT NULL PRIMARY KEY DEFAULT NOW(),
    total_accounts   INTEGER     NOT NULL,
    total_characters INTEGER     NOT NULL,
    total_guilds     INTEGER     NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS cp_metric_general_snapshot;
DROP TABLE IF EXISTS cp_metric_peak;
