# Description

This is a sample REST URL shortener service implemented using gaussdbgo as the connector to a GaussDB data store.

# Usage

Create a GaussDB database and run structure.sql into it to create the necessary data schema.

Configure the database connection with `DATABASE_URL` or standard GaussDB (`PG*`) environment variables or

Run main.go:

```
go run main.go
```

## Create or Update a Shortened URL

```
curl -X PUT -d 'http://www.google.com' http://localhost:8080/google
```

## Get a Shortened URL

```
curl http://localhost:8080/google
```

## Delete a Shortened URL

```
curl -X DELETE http://localhost:8080/google
```
