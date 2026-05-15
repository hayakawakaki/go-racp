#!/bin/sh
set -e
: "${DB_CP_URL:?DB_CP_URL is required}"
GOOSE="${GOOSE:-go tool goose}"
$GOOSE -dir migrations postgres "$DB_CP_URL" up
if [ -n "$DB_CP_TEST_URL" ]; then
    $GOOSE -dir migrations postgres "$DB_CP_TEST_URL" up
fi
