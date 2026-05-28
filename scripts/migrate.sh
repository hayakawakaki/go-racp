#!/bin/sh
set -e
: "${DB_CP_URL:?DB_CP_URL is required}"
GOOSE="${GOOSE:-go tool goose}"
$GOOSE -dir migrations postgres "$DB_CP_URL" up
if [ -n "$DB_CP_TEST_URL" ]; then
    $GOOSE -dir migrations postgres "$DB_CP_TEST_URL" up
fi

if [ "$MODE" = "development" ] && [ -f docker/postgres/seed.sql ] && command -v psql >/dev/null 2>&1; then
    echo "applying postgres dev seed (idempotent)..."
    psql -v ON_ERROR_STOP=1 "$DB_CP_URL" -f docker/postgres/seed.sql
fi
