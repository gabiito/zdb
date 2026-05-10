#!/usr/bin/env bash
# Apply schema + seed data to one or all supported databases.
#
# Usage:
#   ./apply.sh sqlite [path]   # default path: /tmp/dev.db
#   ./apply.sh postgres        # requires `docker compose up -d postgres`
#   ./apply.sh mysql           # requires `docker compose up -d mysql`
#   ./apply.sh all             # sqlite + postgres + mysql
#   ./apply.sh up              # docker compose up -d (postgres + mysql)
#   ./apply.sh down            # docker compose down -v (drops volumes)

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCHEMA_DIR="$HERE/schema"
DATA_FILE="$HERE/data/data.sql"

apply_sqlite() {
  local target="${1:-/tmp/dev.db}"
  echo "▸ sqlite → $target"
  rm -f "$target"
  python3 - "$target" "$SCHEMA_DIR/sqlite.sql" "$DATA_FILE" <<'PY'
import sqlite3, sys, pathlib
target, schema, data = sys.argv[1], sys.argv[2], sys.argv[3]
conn = sqlite3.connect(target)
conn.executescript(pathlib.Path(schema).read_text())
conn.executescript(pathlib.Path(data).read_text())
conn.commit()
conn.close()
PY
  echo "  done."
}

apply_postgres() {
  echo "▸ postgres (container: zdb-postgres)"
  docker compose -f "$HERE/docker-compose.yml" exec -T postgres \
    psql -U dev -d school -v ON_ERROR_STOP=1 \
      -c "DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;"
  docker compose -f "$HERE/docker-compose.yml" exec -T postgres \
    psql -U dev -d school -v ON_ERROR_STOP=1 < "$SCHEMA_DIR/postgres.sql"
  docker compose -f "$HERE/docker-compose.yml" exec -T postgres \
    psql -U dev -d school -v ON_ERROR_STOP=1 < "$DATA_FILE"
  echo "  done."
}

apply_mysql() {
  echo "▸ mysql (container: zdb-mysql)"
  docker compose -f "$HERE/docker-compose.yml" exec -T mysql \
    mysql -udev -pdev -e "DROP DATABASE IF EXISTS school; CREATE DATABASE school;"
  docker compose -f "$HERE/docker-compose.yml" exec -T mysql \
    sh -c "mysql -udev -pdev school" < "$SCHEMA_DIR/mysql.sql"
  docker compose -f "$HERE/docker-compose.yml" exec -T mysql \
    sh -c "mysql -udev -pdev school" < "$DATA_FILE"
  echo "  done."
}

case "${1:-}" in
  sqlite)
    apply_sqlite "${2:-}"
    ;;
  postgres|pg)
    apply_postgres
    ;;
  mysql|my)
    apply_mysql
    ;;
  all)
    apply_sqlite
    apply_postgres
    apply_mysql
    ;;
  up)
    docker compose -f "$HERE/docker-compose.yml" up -d
    ;;
  down)
    docker compose -f "$HERE/docker-compose.yml" down -v
    ;;
  *)
    echo "usage: $0 {sqlite [path]|postgres|mysql|all|up|down}" >&2
    exit 1
    ;;
esac
