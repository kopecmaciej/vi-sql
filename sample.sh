#!/usr/bin/env bash

set -e

CONTAINER="tui-postgres"
DB_USER="postgres"
DB_PASSWORD="postgres"
DB_NAME="tui_sample_db"
PORT="5432"
SQL_FILE="sample.sql"

if [ ! -f "$SQL_FILE" ]; then
  echo "Error: $SQL_FILE not found"
  exit 1
fi

echo "Removing old container (if exists)..."
docker rm -f $CONTAINER 2>/dev/null || true

echo "Starting PostgreSQL..."
docker run -d \
  --name $CONTAINER \
  -e POSTGRES_USER=$DB_USER \
  -e POSTGRES_PASSWORD=$DB_PASSWORD \
  -e POSTGRES_DB=$DB_NAME \
  -p $PORT:5432 \
  postgres:16

echo "Waiting for Postgres..."
until docker exec $CONTAINER pg_isready -U $DB_USER > /dev/null 2>&1; do
  sleep 2
done

echo "Copying SQL file..."
docker cp $SQL_FILE $CONTAINER:/sample.sql

echo "Executing SQL..."
docker exec -i $CONTAINER psql -U $DB_USER -d $DB_NAME -f /sample.sql

echo "Verifying tables..."
docker exec -it $CONTAINER psql -U $DB_USER -d $DB_NAME -c "\dt"

echo
echo "Postgres is ready on port $PORT"
echo "Connection string:"
echo "postgres://$DB_USER:$DB_PASSWORD@localhost:$PORT/$DB_NAME?sslmode=disable"
