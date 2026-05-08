#!/bin/bash
set -e
shopt -s nullglob

for f in /migrations/*.sql; do
    echo "Running CP migration: $f"
    mariadb -uroot -p"${MARIADB_ROOT_PASSWORD}" "${MARIADB_DATABASE}" < "$f"
done
