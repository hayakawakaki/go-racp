#!/bin/sh
set -e
./scripts/migrate.sh
exec air -c .air.toml
