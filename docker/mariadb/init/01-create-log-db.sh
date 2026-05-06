#!/bin/bash
set -e

mariadb -uroot -p"${MARIADB_ROOT_PASSWORD}" <<-EOSQL
    CREATE DATABASE IF NOT EXISTS \`log\`;
    GRANT ALL PRIVILEGES ON \`log\`.* TO '${MARIADB_USER}'@'%';
    FLUSH PRIVILEGES;
EOSQL
