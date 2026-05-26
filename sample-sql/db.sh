#!/usr/bin/env bash

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Directory containing seed CSVs (go run ./sample-sql/seed generates these).
SEED_DIR="${SEED_DIR:-/tmp/vi-sql-seed}"

# ============================================================
# Driver registration framework
# ============================================================
# Each Docker-backed driver registers with:
#   drv <name> <container> <port> <sql_file>
#
# Then defines four hook functions:
#   <name>_docker_run <container>     — docker run ...
#   <name>_wait       <container>     — block until DB is ready
#   <name>_load       <container> <sql_file> — copy + execute SQL
#   <name>_url                        — print connection string(s)
#
# Optional:
#   <name>_setup      <container>     — runs after wait, before seed copy
#                                       (e.g. CREATE DATABASE)
#
# SQLite is non-Docker and handled separately at the bottom.
# ============================================================

DOCKER_DRIVERS=()

drv() {
  DOCKER_DRIVERS+=("$1")
  printf -v "_DRV_${1}_CTR"  '%s' "$2"
  printf -v "_DRV_${1}_PORT" '%s' "$3"
  printf -v "_DRV_${1}_SQL"  '%s' "$4"
}

_drv() { local v="_DRV_${1}_${2}"; printf '%s' "${!v}"; }

# ============================================================
# Shared seed-data helpers
# ============================================================

_ensure_seed() {
  if [ ! -d "$SEED_DIR" ] || [ -z "$(ls "$SEED_DIR"/*.csv 2>/dev/null)" ]; then
    echo "Seed data not found in $SEED_DIR — generating..."
    go run "$SCRIPT_DIR/seed"
  fi
}

_copy_seed() {
  _ensure_seed
  echo "Copying seed data from $SEED_DIR..."
  # Run as root so the directory can be created regardless of the container's
  # default user (e.g. SQL Server runs as the non-root mssql user).
  docker exec -u 0 "$1" mkdir -p /data
  docker cp "${SEED_DIR}/." "$1:/data"
}

# ============================================================
# Shared SQL loaders (called from <name>_load hooks)
# ============================================================

_load_psql() {
  local ctr=$1 sql=$2 user=$3 db=$4
  echo "Copying SQL file..."
  docker cp "$sql" "$ctr:/sample.sql"
  echo "Executing SQL..."
  docker exec -i "$ctr" psql -U "$user" -d "$db" -f /sample.sql
}

_load_mysql_cli() {
  local ctr=$1 sql=$2 cli=$3 user=$4 pass=$5 db=$6
  echo "Copying SQL file..."
  docker cp "$sql" "$ctr:/sample.sql"
  echo "Executing SQL..."
  docker exec -i "$ctr" "$cli" --local-infile=1 -u "$user" -p"$pass" "$db" -e "source /sample.sql"
  echo "Verifying tables..."
  docker exec -it "$ctr" "$cli" -u "$user" -p"$pass" "$db" -e "SHOW TABLES;"
}

# Find sqlcmd inside the SQL Server container regardless of mssql-tools version.
# The glob must expand inside the container, so we use bash -c.
_sqlcmd() {
  local ctr=$1; shift
  local bin; bin=$(docker exec "$ctr" bash -c 'find /opt/mssql-tools* -name sqlcmd 2>/dev/null | sort | tail -1')
  docker exec "$ctr" "$bin" "$@"
}

# ============================================================
# Generic up / stop / rm
# ============================================================

_generic_up() {
  local name=$1 ctr sql
  ctr=$(_drv "$name" CTR); sql=$(_drv "$name" SQL)

  [ -f "$sql" ] || { echo "Error: $sql not found"; exit 1; }

  echo "Removing old $name container..."
  docker rm -f -v "$ctr" 2>/dev/null || true

  "${name}_docker_run" "$ctr"
  "${name}_wait"       "$ctr"

  # Optional post-start setup (e.g. CREATE DATABASE).
  if declare -f "${name}_setup" > /dev/null 2>&1; then
    "${name}_setup" "$ctr"
  fi

  _copy_seed "$ctr"
  "${name}_load" "$ctr" "$sql"

  echo
  "${name}_url"
}

_generic_stop() {
  local ctr; ctr=$(_drv "$1" CTR)
  echo "Stopping $1..."
  docker stop "$ctr" 2>/dev/null || echo "Container not running"
}

_generic_rm() {
  local ctr; ctr=$(_drv "$1" CTR)
  echo "Removing $1..."
  docker rm -f -v "$ctr" 2>/dev/null || echo "Container not found"
}

_dispatch() {
  local name=$1 cmd=$2
  case "$cmd" in
    up)   _generic_up   "$name" ;;
    stop) _generic_stop "$name" ;;
    rm)   _generic_rm   "$name" ;;
    url)  "${name}_url" ;;
    *)    usage ;;
  esac
}

# ============================================================
# PostgreSQL
# ============================================================
PG_USER=postgres; PG_PASS=postgres; PG_DB=tui_sample_db; PG_PORT=5432
PG_SSL_CTR=tui-postgres-ssl; PG_SSL_PORT=5433; PG_SSL_VOL=tui-pg-ssl-certs

drv postgres tui-postgres $PG_PORT "$SCRIPT_DIR/sample.postgres.sql"

postgres_docker_run() {
  docker run -d --name "$1" \
    -e POSTGRES_USER=$PG_USER -e POSTGRES_PASSWORD=$PG_PASS -e POSTGRES_DB=$PG_DB \
    -p $PG_PORT:5432 \
    postgres:16
}

postgres_wait() {
  echo "Waiting for PostgreSQL..."
  until docker exec "$1" pg_isready -U $PG_USER > /dev/null 2>&1; do sleep 2; done
}

postgres_load() { _load_psql "$1" "$2" $PG_USER $PG_DB; }

postgres_url() {
  echo "Plain:       postgres://$PG_USER:$PG_PASS@localhost:$PG_PORT/$PG_DB?sslmode=disable"
  echo "SSL require: postgres://$PG_USER:$PG_PASS@localhost:$PG_SSL_PORT/$PG_DB?sslmode=require"
  echo "SSL verify:  postgres://$PG_USER:$PG_PASS@localhost:$PG_SSL_PORT/$PG_DB?sslmode=verify-ca"
}

# SSL variant — separate container on port $PG_SSL_PORT.
postgres_up_ssl() {
  [ -f "$(_drv postgres SQL)" ] || { echo "Error: SQL file not found"; exit 1; }

  echo "Removing old SSL container and cert volume..."
  docker rm -f -v $PG_SSL_CTR 2>/dev/null || true
  docker volume rm $PG_SSL_VOL 2>/dev/null || true

  # Generate certs inside Docker so they are owned by UID 999 (postgres user).
  # PostgreSQL refuses a key file readable by other users, so ownership matters.
  echo "Generating SSL certificates..."
  docker run --rm -v $PG_SSL_VOL:/certs --entrypoint bash postgres:16 -c "
    openssl req -new -x509 -days 365 -nodes \
      -out /certs/server.crt -keyout /certs/server.key \
      -subj '/CN=localhost' 2>/dev/null
    chown 999:999 /certs/server.crt /certs/server.key
    chmod 600 /certs/server.key"

  echo "Starting PostgreSQL with SSL..."
  docker run -d --name $PG_SSL_CTR \
    -e POSTGRES_USER=$PG_USER -e POSTGRES_PASSWORD=$PG_PASS -e POSTGRES_DB=$PG_DB \
    -p $PG_SSL_PORT:5432 \
    -v $PG_SSL_VOL:/var/lib/postgresql/certs:ro \
    postgres:16 \
    -c ssl=on \
    -c ssl_cert_file=/var/lib/postgresql/certs/server.crt \
    -c ssl_key_file=/var/lib/postgresql/certs/server.key

  echo "Waiting for PostgreSQL (SSL)..."
  until docker exec $PG_SSL_CTR pg_isready -U $PG_USER > /dev/null 2>&1; do sleep 2; done

  _copy_seed $PG_SSL_CTR
  _load_psql $PG_SSL_CTR "$(_drv postgres SQL)" $PG_USER $PG_DB

  echo
  postgres_url
}

postgres_stop_ssl() { docker stop $PG_SSL_CTR 2>/dev/null || echo "Not running"; }
postgres_rm_ssl()   {
  docker rm -f -v $PG_SSL_CTR 2>/dev/null || true
  docker volume rm $PG_SSL_VOL 2>/dev/null || echo "Volume not found"
}

# ============================================================
# MySQL
# ============================================================
MY_USER=root; MY_PASS=mysql; MY_DB=tui_sample_db; MY_PORT=3306

drv mysql tui-mysql $MY_PORT "$SCRIPT_DIR/sample.mysql.sql"

mysql_docker_run() {
  # MySQL 8 auto-generates SSL certs on first start, so tls=skip-verify works
  # against this container without any extra configuration.
  docker run -d --name "$1" \
    -e MYSQL_ROOT_PASSWORD=$MY_PASS -e MYSQL_DATABASE=$MY_DB \
    -p $MY_PORT:3306 \
    mysql:8
}

mysql_wait() {
  echo "Waiting for MySQL..."
  until docker exec "$1" mysqladmin ping -u $MY_USER -p$MY_PASS --silent 2>/dev/null; do sleep 2; done
}

mysql_load() { _load_mysql_cli "$1" "$2" mysql $MY_USER $MY_PASS $MY_DB; }

mysql_url() {
  echo "Plain:       mysql://$MY_USER:$MY_PASS@localhost:$MY_PORT/$MY_DB"
  echo "TLS skip:    mysql://$MY_USER:$MY_PASS@localhost:$MY_PORT/$MY_DB?tls=skip-verify"
  echo "TLS prefer:  mysql://$MY_USER:$MY_PASS@localhost:$MY_PORT/$MY_DB?tls=preferred"
  echo "TLS require: mysql://$MY_USER:$MY_PASS@localhost:$MY_PORT/$MY_DB?tls=true"
}

# ============================================================
# MariaDB
# ============================================================
MARIA_USER=root; MARIA_PASS=mariadb; MARIA_DB=tui_sample_db; MARIA_PORT=3307

drv mariadb tui-mariadb $MARIA_PORT "$SCRIPT_DIR/sample.mariadb.sql"

mariadb_docker_run() {
  docker run -d --name "$1" \
    -e MARIADB_ROOT_PASSWORD=$MARIA_PASS -e MARIADB_DATABASE=$MARIA_DB \
    -p $MARIA_PORT:3306 \
    mariadb:11
}

mariadb_wait() {
  echo "Waiting for MariaDB..."
  until docker exec "$1" mariadb-admin ping -u $MARIA_USER -p$MARIA_PASS --silent 2>/dev/null; do sleep 2; done
}

mariadb_load() { _load_mysql_cli "$1" "$2" mariadb $MARIA_USER $MARIA_PASS $MARIA_DB; }

mariadb_url() {
  echo "Plain:       mariadb://$MARIA_USER:$MARIA_PASS@localhost:$MARIA_PORT/$MARIA_DB"
  echo "TLS skip:    mariadb://$MARIA_USER:$MARIA_PASS@localhost:$MARIA_PORT/$MARIA_DB?tls=skip-verify"
  echo "TLS prefer:  mariadb://$MARIA_USER:$MARIA_PASS@localhost:$MARIA_PORT/$MARIA_DB?tls=preferred"
}

# ============================================================
# CockroachDB
# ============================================================
CRDB_USER=root; CRDB_DB=tui_sample_db; CRDB_PORT=26257; CRDB_HTTP=8080

drv cockroachdb tui-cockroachdb $CRDB_PORT "$SCRIPT_DIR/sample.cockroach.sql"

cockroachdb_docker_run() {
  docker run -d --name "$1" \
    -p $CRDB_PORT:26257 -p $CRDB_HTTP:8080 \
    cockroachdb/cockroach:latest-v24.3 start-single-node --insecure
}

cockroachdb_wait() {
  echo "Waiting for CockroachDB..."
  until docker exec "$1" cockroach sql --insecure --execute "SELECT 1" > /dev/null 2>&1; do sleep 2; done
}

cockroachdb_setup() {
  echo "Creating database..."
  docker exec "$1" cockroach sql --insecure --execute "CREATE DATABASE IF NOT EXISTS $CRDB_DB;"
}

cockroachdb_load() {
  echo "Copying SQL file..."
  docker cp "$2" "$1:/sample.sql"
  echo "Executing SQL..."
  docker exec "$1" bash -c "cockroach sql --insecure --database=$CRDB_DB --set=errexit=true < /sample.sql"
  echo "Verifying tables..."
  docker exec "$1" cockroach sql --insecure --database=$CRDB_DB --execute "SHOW TABLES;"
}

cockroachdb_url() {
  echo "Plain:    cockroachdb://$CRDB_USER@localhost:$CRDB_PORT/$CRDB_DB?sslmode=disable"
  echo "Postgres: postgresql://$CRDB_USER@localhost:$CRDB_PORT/$CRDB_DB?sslmode=disable"
}

# ============================================================
# SQL Server
# ============================================================
# SA password must meet SQL Server complexity rules (upper + lower + digit + symbol).
MSSQL_USER=sa; MSSQL_PASS='SqlServer1!'; MSSQL_DB=tui_sample_db; MSSQL_PORT=1433

drv sqlserver tui-sqlserver $MSSQL_PORT "$SCRIPT_DIR/sample.mssql.sql"

sqlserver_docker_run() {
  docker run -d --name "$1" \
    -e ACCEPT_EULA=Y \
    -e MSSQL_SA_PASSWORD="$MSSQL_PASS" \
    -p $MSSQL_PORT:1433 \
    mcr.microsoft.com/mssql/server:2022-latest
}

sqlserver_wait() {
  echo "Waiting for SQL Server..."
  # SQL Server 2022 requires encrypted connections with its self-signed cert.
  # -C trusts the server certificate so sqlcmd can connect without a CA bundle.
  until _sqlcmd "$1" -S localhost -U $MSSQL_USER -P "$MSSQL_PASS" -C \
      -Q "SELECT 1" > /dev/null 2>&1; do sleep 2; done
}

sqlserver_setup() {
  echo "Creating database..."
  _sqlcmd "$1" -S localhost -U $MSSQL_USER -P "$MSSQL_PASS" -C \
    -Q "CREATE DATABASE [$MSSQL_DB]"
}

sqlserver_load() {
  echo "Copying SQL file..."
  docker cp "$2" "$1:/sample.sql"
  echo "Executing SQL..."
  # BULK INSERT reads /data/ inside the container, already populated by _copy_seed.
  _sqlcmd "$1" -S localhost -U $MSSQL_USER -P "$MSSQL_PASS" -C \
    -d "$MSSQL_DB" -i /sample.sql
  echo "Verifying tables..."
  _sqlcmd "$1" -S localhost -U $MSSQL_USER -P "$MSSQL_PASS" -C \
    -d "$MSSQL_DB" \
    -Q "SELECT TABLE_SCHEMA, TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE='BASE TABLE' ORDER BY 1,2"
}

sqlserver_url() {
  # The dev container does not force encryption, so encrypt=disable avoids any
  # certificate issues with the self-signed cert.
  echo "Plain: sqlserver://$MSSQL_USER:$MSSQL_PASS@localhost:$MSSQL_PORT?database=$MSSQL_DB&encrypt=disable"
}

# ============================================================
# SQLite (no Docker — handled outside the generic framework)
# ============================================================
SQLITE_DB="$SCRIPT_DIR/sample.db"
SQLITE_SQL="$SCRIPT_DIR/sample.sqlite.sql"

sqlite_up() {
  [ -f "$SQLITE_SQL" ] || { echo "Error: $SQLITE_SQL not found"; exit 1; }
  _ensure_seed
  echo "Removing old database..."
  rm -f "$SQLITE_DB"
  echo "Loading schema and data..."
  # Inline SEED_DIR so .import finds the CSV files at their actual path.
  sed "s|/tmp/vi-sql-seed|${SEED_DIR}|g" "$SQLITE_SQL" | sqlite3 "$SQLITE_DB"
  echo "Verifying tables..."
  sqlite3 "$SQLITE_DB" "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name;"
  echo
  sqlite_url
}

sqlite_rm()  { echo "Removing SQLite database..."; rm -f "$SQLITE_DB" && echo "Removed $SQLITE_DB" || echo "File not found"; }
sqlite_url() { echo "Plain: sqlite://$SQLITE_DB"; }

# ============================================================
# Usage (auto-generated from registered drivers)
# ============================================================
usage() {
  echo "Usage: $0 <database> <command>"
  echo ""
  echo "Docker databases:"
  local name port
  for name in "${DOCKER_DRIVERS[@]}"; do
    port=$(_drv "$name" PORT)
    printf "  %-14s  up | stop | rm | url   (port %s)\n" "$name" "$port"
  done
  echo ""
  echo "  postgres also supports:  up-ssl | stop-ssl | rm-ssl   (SSL on port $PG_SSL_PORT)"
  echo "  sqlite   up | rm | url   (no Docker)"
  echo ""
  echo "Bulk commands:"
  echo "  stop-all   Stop all Docker containers"
  echo "  rm-all     Remove all Docker containers"
  echo "  clean      Stop and remove all"
  exit 1
}

# ============================================================
# Dispatch
# ============================================================
case "${1:-}" in
  postgres)
    case "${2:-}" in
      up-ssl)   postgres_up_ssl ;;
      stop-ssl) postgres_stop_ssl ;;
      rm-ssl)   postgres_rm_ssl ;;
      *)        _dispatch postgres "${2:-}" ;;
    esac ;;

  sqlite)
    case "${2:-}" in
      up)  sqlite_up ;;
      rm)  sqlite_rm ;;
      url) sqlite_url ;;
      *)   usage ;;
    esac ;;

  stop-all) for d in "${DOCKER_DRIVERS[@]}"; do _generic_stop "$d"; done ;;
  rm-all)   for d in "${DOCKER_DRIVERS[@]}"; do _generic_rm   "$d"; done ;;
  clean)    for d in "${DOCKER_DRIVERS[@]}"; do _generic_stop "$d"; _generic_rm "$d"; done ;;

  *)
    for d in "${DOCKER_DRIVERS[@]}"; do
      [[ "${1:-}" == "$d" ]] && { _dispatch "$d" "${2:-}"; exit 0; }
    done
    usage ;;
esac
