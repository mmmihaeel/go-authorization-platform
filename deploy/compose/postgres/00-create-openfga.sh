#!/usr/bin/env sh
set -eu
psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<'SQL'
CREATE DATABASE openfga;
SQL
