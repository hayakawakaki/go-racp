#!/bin/sh
set -e
GOOSE="${GOOSE:-go tool goose}"
exec $GOOSE -dir migrations mysql "$DB_MAIN_URL" up
