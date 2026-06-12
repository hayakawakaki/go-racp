-- +goose Up
CREATE SEQUENCE cp_market_ref_seq;

CREATE TABLE cp_listing (
    id                 BIGINT      NOT NULL DEFAULT nextval('cp_market_ref_seq'),
    seller_account_id  INTEGER     NOT NULL,
    kind               SMALLINT    NOT NULL,
    status             SMALLINT    NOT NULL DEFAULT 1,
    give_item          BOOLEAN     NOT NULL DEFAULT FALSE,
    give_nameid        INTEGER     NOT NULL DEFAULT 0,
    give_refine        SMALLINT    NOT NULL DEFAULT 0,
    give_grade         SMALLINT    NOT NULL DEFAULT 0,
    give_card0         INTEGER     NOT NULL DEFAULT 0,
    give_card1         INTEGER     NOT NULL DEFAULT 0,
    give_card2         INTEGER     NOT NULL DEFAULT 0,
    give_card3         INTEGER     NOT NULL DEFAULT 0,
    give_unit_amount   INTEGER     NOT NULL DEFAULT 0,
    give_zeny          BIGINT      NOT NULL DEFAULT 0,
    give_cashpoint     INTEGER     NOT NULL DEFAULT 0,
    give_hold_id       BIGINT,
    want_nameid        INTEGER     NOT NULL DEFAULT 0,
    want_unit_amount   INTEGER     NOT NULL DEFAULT 0,
    want_zeny          BIGINT      NOT NULL DEFAULT 0,
    want_cashpoint     INTEGER     NOT NULL DEFAULT 0,
    total_quantity     INTEGER     NOT NULL DEFAULT 1,
    remaining_quantity INTEGER     NOT NULL DEFAULT 1,
    stackable          BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at         TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT cp_listing_remaining CHECK (remaining_quantity >= 0 AND remaining_quantity <= total_quantity)
);

CREATE INDEX idx_cp_listing_browse ON cp_listing (status, kind, created_at DESC);

CREATE INDEX idx_cp_listing_seller ON cp_listing (seller_account_id, created_at DESC);

CREATE INDEX idx_cp_listing_expiry ON cp_listing (expires_at) WHERE status = 1;

CREATE INDEX idx_cp_listing_want_nameid ON cp_listing (want_nameid) WHERE status = 1;

CREATE INDEX idx_cp_listing_give_nameid ON cp_listing (give_nameid) WHERE status = 1;

CREATE TABLE cp_settlement (
    id                   BIGSERIAL   NOT NULL,
    listing_id           BIGINT      NOT NULL,
    escrow_ref           BIGINT      NOT NULL,
    recipient_account_id INTEGER     NOT NULL,
    deliver_amount       INTEGER     NOT NULL DEFAULT 0,
    whole                BOOLEAN     NOT NULL DEFAULT TRUE,
    status               SMALLINT    NOT NULL DEFAULT 1,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id)
);

CREATE INDEX idx_cp_settlement_pending ON cp_settlement (id) WHERE status = 1;

-- +goose Down
DROP TABLE IF EXISTS cp_settlement;
DROP TABLE IF EXISTS cp_listing;
DROP SEQUENCE IF EXISTS cp_market_ref_seq;
