# gaussdbconn

Package gaussdbconn is a low-level GaussDB database driver. It is primarily intended to serve as the foundation for 
higher level libraries such as https://github.com/HuaweiCloudDeveloper/gaussdb-go. Applications should handle normal 
queries with a higher level library and only use gaussdbconn directly when required for low-level access to GaussDB 
functionality.

## Example Usage

```go
gaussdbConn, err := gaussdbconn.Connect(context.Background(), os.Getenv("DATABASE_URL"))
if err != nil {
	log.Fatalln("gaussdbconn failed to connect:", err)
}
defer gaussdbConn.Close(context.Background())

result := gaussdbConn.ExecParams(context.Background(), "SELECT email FROM users WHERE id=$1", [][]byte{[]byte("123")}, nil, nil, nil)
for result.NextRow() {
	fmt.Println("User 123 has email:", string(result.Values()[0]))
}
_, err = result.Close()
if err != nil {
	log.Fatalln("failed reading result:", err)
}
```

## Testing

See CONTRIBUTING.md for setup instructions.
