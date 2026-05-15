-- +goose Up
CREATE TABLE cp_tickets (
    id              BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id      INTEGER      NOT NULL,
    author_username VARCHAR(23)  NOT NULL,
    category        VARCHAR(64)  NOT NULL,
    subject         VARCHAR(150) NOT NULL,
    status          TEXT         NOT NULL DEFAULT 'open'
                    CHECK (status IN ('open','resolved','closed')),
    last_actor      TEXT         NOT NULL DEFAULT 'player'
                    CHECK (last_actor IN ('player','staff')),
    message_count   INTEGER      NOT NULL DEFAULT 1,
    last_activity   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    closed_by       INTEGER      NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cp_tickets_account_lastact ON cp_tickets (account_id, last_activity DESC);
CREATE INDEX idx_cp_tickets_category_status ON cp_tickets (category, status);

CREATE INDEX idx_cp_tickets_open_no_response ON cp_tickets (last_activity DESC)
    WHERE status = 'open' AND message_count = 1;
CREATE INDEX idx_cp_tickets_open_active ON cp_tickets (last_activity DESC)
    WHERE status = 'open' AND message_count > 1;
CREATE INDEX idx_cp_tickets_terminal ON cp_tickets (last_activity DESC)
    WHERE status IN ('resolved','closed');

CREATE TABLE cp_ticket_messages (
    id          BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ticket_id   BIGINT      NOT NULL REFERENCES cp_tickets(id) ON DELETE CASCADE,
    author_id   INTEGER     NOT NULL,
    author_role TEXT        NOT NULL CHECK (author_role IN ('player','staff')),
    visibility  TEXT        NOT NULL DEFAULT 'public'
                CHECK (visibility IN ('public','internal','system')),
    body        TEXT        NOT NULL,
    event       JSONB       NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_cp_ticket_messages_ticket_created ON cp_ticket_messages (ticket_id, created_at);

CREATE TABLE cp_ticket_views (
    account_id  INTEGER     NOT NULL,
    ticket_id   BIGINT      NOT NULL REFERENCES cp_tickets(id) ON DELETE CASCADE,
    last_viewed TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, ticket_id)
);

-- +goose Down
DROP TABLE IF EXISTS cp_ticket_views;
DROP TABLE IF EXISTS cp_ticket_messages;
DROP TABLE IF EXISTS cp_tickets;
