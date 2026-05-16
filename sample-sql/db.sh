#!/usr/bin/env bash

set -e

# Postgres config
PG_CONTAINER="tui-postgres"
PG_USER="postgres"
PG_PASSWORD="postgres"
PG_DB="tui_sample_db"
PG_PORT="5432"
PG_SQL="sample.postgres.sql"

# Postgres SSL config (separate container, port 5433)
PG_SSL_CONTAINER="tui-postgres-ssl"
PG_SSL_PORT="5433"
PG_SSL_VOLUME="tui-pg-ssl-certs"

# MySQL config
MY_CONTAINER="tui-mysql"
MY_USER="root"
MY_PASSWORD="mysql"
MY_DB="tui_sample_db"
MY_PORT="3306"
MY_SQL="sample.mysql.sql"

# MariaDB config
MARIA_CONTAINER="tui-mariadb"
MARIA_USER="root"
MARIA_PASSWORD="mariadb"
MARIA_DB="tui_sample_db"
MARIA_PORT="3307"
MARIA_SQL="sample.mariadb.sql"

usage() {
  echo "Usage: $0 <command>"
  echo ""
  echo "Commands:"
  echo "  postgres up       Start PostgreSQL container and load sample data"
  echo "  postgres up-ssl   Start PostgreSQL container with SSL (port $PG_SSL_PORT) and load sample data"
  echo "  postgres stop     Stop PostgreSQL container"
  echo "  postgres stop-ssl Stop PostgreSQL SSL container"
  echo "  postgres rm       Remove PostgreSQL container"
  echo "  postgres rm-ssl   Remove PostgreSQL SSL container and cert volume"
  echo "  postgres url      Print PostgreSQL connection strings"
  echo "  mysql up          Start MySQL container and load sample data"
  echo "  mysql stop        Stop MySQL container"
  echo "  mysql rm          Remove MySQL container"
  echo "  mysql url         Print MySQL connection strings"
  echo "  mariadb up        Start MariaDB container and load sample data"
  echo "  mariadb stop      Stop MariaDB container"
  echo "  mariadb rm        Remove MariaDB container"
  echo "  mariadb url       Print MariaDB connection strings"
  echo "  stop-all          Stop all containers"
  echo "  rm-all            Remove all containers"
  echo "  clean             Stop and remove all containers"
  exit 1
}

_pg_load_data() {
  local container=$1
  echo "Copying SQL file..."
  docker cp $PG_SQL $container:/sample.sql
  echo "Executing SQL..."
  docker exec -i $container psql -U $PG_USER -d $PG_DB -f /sample.sql
  echo "Verifying tables..."
  docker exec -it $container psql -U $PG_USER -d $PG_DB -c "\dt"
}

postgres_up() {
  if [ ! -f "$PG_SQL" ]; then
    echo "Error: $PG_SQL not found"
    exit 1
  fi

  echo "Removing old container (if exists)..."
  docker rm -f $PG_CONTAINER 2>/dev/null || true

  echo "Starting PostgreSQL..."
  docker run -d \
    --name $PG_CONTAINER \
    -e POSTGRES_USER=$PG_USER \
    -e POSTGRES_PASSWORD=$PG_PASSWORD \
    -e POSTGRES_DB=$PG_DB \
    -p $PG_PORT:5432 \
    postgres:16

  echo "Waiting for Postgres..."
  until docker exec $PG_CONTAINER pg_isready -U $PG_USER > /dev/null 2>&1; do
    sleep 2
  done

  _pg_load_data $PG_CONTAINER

  echo
  echo "Postgres is ready on port $PG_PORT"
  postgres_url
}

postgres_up_ssl() {
  if [ ! -f "$PG_SQL" ]; then
    echo "Error: $PG_SQL not found"
    exit 1
  fi

  echo "Removing old SSL container and cert volume (if exist)..."
  docker rm -f $PG_SSL_CONTAINER 2>/dev/null || true
  docker volume rm $PG_SSL_VOLUME 2>/dev/null || true

  # Generate certs inside docker so they are owned by UID 999 (postgres user).
  # PostgreSQL refuses a key file readable by other users, so ownership matters.
  echo "Generating SSL certificates..."
  docker run --rm \
    -v $PG_SSL_VOLUME:/certs \
    --entrypoint bash \
    postgres:16 -c "
      openssl req -new -x509 -days 365 -nodes \
        -out /certs/server.crt -keyout /certs/server.key \
        -subj '/CN=localhost' 2>/dev/null
      chown 999:999 /certs/server.crt /certs/server.key
      chmod 600 /certs/server.key
    "

  echo "Starting PostgreSQL with SSL..."
  docker run -d \
    --name $PG_SSL_CONTAINER \
    -e POSTGRES_USER=$PG_USER \
    -e POSTGRES_PASSWORD=$PG_PASSWORD \
    -e POSTGRES_DB=$PG_DB \
    -p $PG_SSL_PORT:5432 \
    -v $PG_SSL_VOLUME:/var/lib/postgresql/certs:ro \
    postgres:16 \
    -c ssl=on \
    -c ssl_cert_file=/var/lib/postgresql/certs/server.crt \
    -c ssl_key_file=/var/lib/postgresql/certs/server.key

  echo "Waiting for Postgres (SSL)..."
  until docker exec $PG_SSL_CONTAINER pg_isready -U $PG_USER > /dev/null 2>&1; do
    sleep 2
  done

  _pg_load_data $PG_SSL_CONTAINER

  echo
  echo "PostgreSQL SSL is ready on port $PG_SSL_PORT"
  postgres_url
}

postgres_stop() {
  echo "Stopping PostgreSQL..."
  docker stop $PG_CONTAINER 2>/dev/null || echo "Container not running"
}

postgres_stop_ssl() {
  echo "Stopping PostgreSQL SSL..."
  docker stop $PG_SSL_CONTAINER 2>/dev/null || echo "Container not running"
}

postgres_rm() {
  echo "Removing PostgreSQL container..."
  docker rm -f $PG_CONTAINER 2>/dev/null || echo "Container not found"
}

postgres_rm_ssl() {
  echo "Removing PostgreSQL SSL container and cert volume..."
  docker rm -f $PG_SSL_CONTAINER 2>/dev/null || echo "Container not found"
  docker volume rm $PG_SSL_VOLUME 2>/dev/null || echo "Volume not found"
}

postgres_url() {
  echo "Plain:       postgres://$PG_USER:$PG_PASSWORD@localhost:$PG_PORT/$PG_DB?sslmode=disable"
  echo "SSL require: postgres://$PG_USER:$PG_PASSWORD@localhost:$PG_SSL_PORT/$PG_DB?sslmode=require"
  echo "SSL verify:  postgres://$PG_USER:$PG_PASSWORD@localhost:$PG_SSL_PORT/$PG_DB?sslmode=verify-ca"
}

mysql_up() {
  if [ ! -f "$MY_SQL" ]; then
    echo "Error: $MY_SQL not found"
    exit 1
  fi

  echo "Removing old container (if exists)..."
  docker rm -f $MY_CONTAINER 2>/dev/null || true

  # MySQL 8 auto-generates SSL certs on first start, so tls=skip-verify works
  # against this container without any extra configuration.
  echo "Starting MySQL..."
  docker run -d \
    --name $MY_CONTAINER \
    -e MYSQL_ROOT_PASSWORD=$MY_PASSWORD \
    -e MYSQL_DATABASE=$MY_DB \
    -p $MY_PORT:3306 \
    mysql:8

  echo "Waiting for MySQL..."
  until docker exec $MY_CONTAINER mysqladmin ping -u $MY_USER -p$MY_PASSWORD --silent 2>/dev/null; do
    sleep 2
  done

  echo "Copying SQL file..."
  docker cp $MY_SQL $MY_CONTAINER:/sample.sql

  echo "Executing SQL..."
  docker exec -i $MY_CONTAINER mysql -u $MY_USER -p$MY_PASSWORD $MY_DB -e "source /sample.sql"

  echo "Verifying tables..."
  docker exec -it $MY_CONTAINER mysql -u $MY_USER -p$MY_PASSWORD $MY_DB -e "SHOW TABLES;"

  echo
  echo "MySQL is ready on port $MY_PORT"
  mysql_url
}

mysql_stop() {
  echo "Stopping MySQL..."
  docker stop $MY_CONTAINER 2>/dev/null || echo "Container not running"
}

mysql_rm() {
  echo "Removing MySQL container..."
  docker rm -f $MY_CONTAINER 2>/dev/null || echo "Container not found"
}

mysql_url() {
  echo "Plain:       mysql://$MY_USER:$MY_PASSWORD@localhost:$MY_PORT/$MY_DB"
  echo "TLS skip:    mysql://$MY_USER:$MY_PASSWORD@localhost:$MY_PORT/$MY_DB?tls=skip-verify"
  echo "TLS prefer:  mysql://$MY_USER:$MY_PASSWORD@localhost:$MY_PORT/$MY_DB?tls=preferred"
  echo "TLS require: mysql://$MY_USER:$MY_PASSWORD@localhost:$MY_PORT/$MY_DB?tls=true"
}

mariadb_up() {
  if [ ! -f "$MARIA_SQL" ]; then
    echo "Error: $MARIA_SQL not found"
    exit 1
  fi

  echo "Removing old container (if exists)..."
  docker rm -f $MARIA_CONTAINER 2>/dev/null || true

  echo "Starting MariaDB..."
  docker run -d \
    --name $MARIA_CONTAINER \
    -e MARIADB_ROOT_PASSWORD=$MARIA_PASSWORD \
    -e MARIADB_DATABASE=$MARIA_DB \
    -p $MARIA_PORT:3306 \
    mariadb:11

  echo "Waiting for MariaDB..."
  until docker exec $MARIA_CONTAINER mariadb-admin ping -u $MARIA_USER -p$MARIA_PASSWORD --silent 2>/dev/null; do
    sleep 2
  done

  echo "Copying SQL file..."
  docker cp $MARIA_SQL $MARIA_CONTAINER:/sample.sql

  echo "Executing SQL..."
  docker exec -i $MARIA_CONTAINER mariadb -u $MARIA_USER -p$MARIA_PASSWORD $MARIA_DB -e "source /sample.sql"

  echo "Verifying tables..."
  docker exec -it $MARIA_CONTAINER mariadb -u $MARIA_USER -p$MARIA_PASSWORD $MARIA_DB -e "SHOW TABLES;"

  echo
  echo "MariaDB is ready on port $MARIA_PORT"
  mariadb_url
}

mariadb_stop() {
  echo "Stopping MariaDB..."
  docker stop $MARIA_CONTAINER 2>/dev/null || echo "Container not running"
}

mariadb_rm() {
  echo "Removing MariaDB container..."
  docker rm -f $MARIA_CONTAINER 2>/dev/null || echo "Container not found"
}

mariadb_url() {
  echo "Plain:       mariadb://$MARIA_USER:$MARIA_PASSWORD@localhost:$MARIA_PORT/$MARIA_DB"
  echo "TLS skip:    mariadb://$MARIA_USER:$MARIA_PASSWORD@localhost:$MARIA_PORT/$MARIA_DB?tls=skip-verify"
  echo "TLS prefer:  mariadb://$MARIA_USER:$MARIA_PASSWORD@localhost:$MARIA_PORT/$MARIA_DB?tls=preferred"
}

stop_all() {
  postgres_stop
  postgres_stop_ssl
  mysql_stop
  mariadb_stop
}

rm_all() {
  postgres_rm
  postgres_rm_ssl
  mysql_rm
  mariadb_rm
}

clean() {
  stop_all
  rm_all
}

case "$1" in
  postgres)
    case "$2" in
      up)       postgres_up ;;
      up-ssl)   postgres_up_ssl ;;
      stop)     postgres_stop ;;
      stop-ssl) postgres_stop_ssl ;;
      rm)       postgres_rm ;;
      rm-ssl)   postgres_rm_ssl ;;
      url)      postgres_url ;;
      *)        usage ;;
    esac
    ;;
  mysql)
    case "$2" in
      up)   mysql_up ;;
      stop) mysql_stop ;;
      rm)   mysql_rm ;;
      url)  mysql_url ;;
      *)    usage ;;
    esac
    ;;
  mariadb)
    case "$2" in
      up)   mariadb_up ;;
      stop) mariadb_stop ;;
      rm)   mariadb_rm ;;
      url)  mariadb_url ;;
      *)    usage ;;
    esac
    ;;
  stop-all) stop_all ;;
  rm-all)   rm_all ;;
  clean)    clean ;;
  *)        usage ;;
esac
