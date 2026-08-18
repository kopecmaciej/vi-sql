# Description

This is a sample todo list implemented using gaussdbgo as the connector to a
GaussDB data store.

# Usage

Create a GaussDB database and run structure.sql into it to create the
necessary data schema.

Example:

    createdb todo
    psql todo < structure.sql

Build todo:

    go build

## Connection configuration

The database connection is configured via DATABASE_URL and standard GaussDB environment variables (GAUSSDB_HOST, GAUSSDB_USER, etc.)

You can either export them then run todo:

    export GAUSSDB_DATABASE=todo
    ./todo list

Or you can prefix the todo execution with the environment variables:

    GAUSSDB_DATABASE=todo ./todo list

## Add a todo item

    ./todo add 'Learn go'

## List tasks

    ./todo list

## Update a task

    ./todo update 1 'Learn more go'

## Delete a task

    ./todo remove 1

# Example Setup and Execution

    jack@hk-47~/dev/go/src/github.com/HuaweiCloudDeveloper/gaussdb-go/examples/todo$ createdb todo
    jack@hk-47~/dev/go/src/github.com/HuaweiCloudDeveloper/gaussdb-go/examples/todo$ psql todo < structure.sql
    Expanded display is used automatically.
    Timing is on.
    CREATE TABLE
    Time: 6.363 ms
    jack@hk-47~/dev/go/src/github.com/HuaweiCloudDeveloper/gaussdb-go/examples/todo$ go build
    jack@hk-47~/dev/go/src/github.com/HuaweiCloudDeveloper/gaussdb-go/examples/todo$ export GAUSSDB_DATABASE=todo
    jack@hk-47~/dev/go/src/github.com/HuaweiCloudDeveloper/gaussdb-go/examples/todo$ ./todo list
    jack@hk-47~/dev/go/src/github.com/HuaweiCloudDeveloper/gaussdb-go/examples/todo$ ./todo add 'Learn Go'
    jack@hk-47~/dev/go/src/github.com/HuaweiCloudDeveloper/gaussdb-go/examples/todo$ ./todo list
    1. Learn Go
    jack@hk-47~/dev/go/src/github.com/HuaweiCloudDeveloper/gaussdb-go/examples/todo$ ./todo update 1 'Learn more Go'
    jack@hk-47~/dev/go/src/github.com/HuaweiCloudDeveloper/gaussdb-go/examples/todo$ ./todo list
    1. Learn more Go
    jack@hk-47~/dev/go/src/github.com/HuaweiCloudDeveloper/gaussdb-go/examples/todo$ ./todo remove 1
    jack@hk-47~/dev/go/src/github.com/HuaweiCloudDeveloper/gaussdb-go/examples/todo$ ./todo list
