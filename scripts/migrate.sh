#!/bin/sh
set -e
: "${DB_MAIN_URL:?DB_MAIN_URL is required}"
GOOSE="${GOOSE:-go tool goose}"
exec $GOOSE -dir migrations mysql "$DB_MAIN_URL" up
