# gaussdb-go - GaussDB Driver and Toolkit

gaussdb-go is a pure Go driver and toolkit for GaussDB.

The gaussdb-go driver is a low-level, high performance interface that exposes GaussDB-specific features such as `COPY`. It also includes an adapter for the standard `database/sql` interface.

The toolkit component is a related set of packages that implement GaussDB functionality such as parsing the wire protocol
and type mapping between GaussDB and Go. These underlying packages can be used to implement alternative drivers,
proxies, load balancers, logical replication clients, etc.

## Example Usage

```go
package main

import (
	"context"
	"fmt"
	"os"

	gassdbgo "github.com/HuaweiCloudDeveloper/gaussdb-go"
)

func main() {
	// urlExample := "gaussdb://username:password@localhost:5432/database_name"
	conn, err := gassdbgo.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	var name string
	var weight int64
	err = conn.QueryRow(context.Background(), "select name, weight from widgets where id=$1", 42).Scan(&name, &weight)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(name, weight)
}
```

## Features

* Support for approximately 70 different GaussDB types
* Automatic statement preparation and caching
* Batch queries
* Single-round trip query mode
* Full TLS connection control
* Binary format support for custom types (allows for much quicker encoding/decoding)
* `COPY` protocol support for faster bulk data loads
* Tracing and logging support
* Connection pool with after-connect hook for arbitrary connection setup
* Conversion of GaussDB arrays to Go slice mappings for integers, floats, and strings
* `hstore` support
* `json` and `jsonb` support
* Maps `inet` and `cidr` GaussDB types to `netip.Addr` and `netip.Prefix`
* Large object support
* NULL mapping to pointer to pointer
* Supports `database/sql.Scanner` and `database/sql/driver.Valuer` interfaces for custom types
* Notice response handling
* Simulated nested transactions with savepoints

## Choosing Between the gaussdb-go and database/sql Interfaces

The gaussdb-go interface is faster. Many GaussDB specific features such as `COPY` are not available
through the `database/sql` interface.

The gaussdb-go interface is recommended when:

1. The application only targets GaussDB.
2. No other libraries that require `database/sql` are in use.

It is also possible to use the `database/sql` interface and convert a connection to the lower-level gaussdb-go interface as needed.

## Testing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for setup instructions.

## Architecture

TODO: add architecture diagram

## Supported Go and GaussDB Versions

gaussdb-go supports the same versions of Go and GaussDB that are supported by their respective teams. For [Go](https://golang.org/doc/devel/release.html#policy) that is the two most recent major releases and for [GaussDB](https://doc.hcs.huawei.com/db/en-us/index.html) the major releases in the last 5 years. This means gausdb-go supports Go 1.22 and higher and GaussDB 2.x and higher. 

## Version Policy

gaussdb-go follows semantic versioning for the documented public API on stable releases.

