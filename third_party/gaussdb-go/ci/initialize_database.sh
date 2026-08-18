#!/usr/bin/env bash
set -eux

max_retries=30
retry_count=0
interval_check_time=10
container_name="${CONTAINER_NAME}"
password="${OPENGAUSS_PASSWORD}"
port=${PORT}
sql_file=${SQL_FILE}
mounted_dir=${MOUNT_DIR_IN_CONTAINER}


if [ -t 0 ]; then
  TTY_FLAG="-t"
else
  TTY_FLAG=""
fi

# verify_sql_user_connection: verify that the username and password created using the SQL file can connect to the database.
function verify_sql_created_user_connection() {
  docker exec -i ${TTY_FLAG} "${container_name}" bash -c "su - omm -c 'gsql -U gaussdbgo_md5 -p ${port} -W${password} -d gaussdbgo_test -c \"select version(); show server_version;\"'"
}

sleep ${interval_check_time}
docker ps -a

while [ $retry_count -lt $max_retries ]; do
    # if not running, quit
    if [ "$(docker inspect -f '{{.State.Status}}' "${container_name}")" != "running" ]; then
        echo "Container '${container_name}' is not running."
        break
    fi

    set +e
    output=$(docker exec -i "${container_name}" bash -c "su - omm -c 'gsql -U omm -c \"select 1;\"'" 2>&1)
    status=$?
    set -e
    # when debug wo need to see what happened
    echo "${output}"

    if [ $status -eq 0 ]; then
      # replace password
      sudo sed -i "s/'{{OPENGAUSS_PASSWORD}}'/'${password}'/" "${mounted_dir}/${sql_file}"
      docker exec -i ${TTY_FLAG} "${container_name}" bash -c "su - omm -c 'gsql -U omm -c \"CREATE DATABASE gaussdbgo_test DBCOMPATIBILITY 'pg';\" -f ${mounted_dir}/${sql_file}'"
      echo "Database initialization completed."
      verify_sql_created_user_connection
      exit 0
    fi

    echo "Waiting for database to be ready... (attempt $((retry_count + 1))/$max_retries)"
    sleep ${interval_check_time}
    ((++retry_count))
done

echo "Database initialization failed."
docker logs "${container_name}"
exit 1