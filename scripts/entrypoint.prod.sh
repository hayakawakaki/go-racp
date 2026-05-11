#!/bin/sh
set -e
GOOSE=./goose ./scripts/migrate.sh
exec ./main
