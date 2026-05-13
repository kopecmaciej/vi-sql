//go:build integration

package testutil

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
)

// SeedBindMount returns a testcontainers option that bind-mounts the seed CSV
// directory on the host to /data inside the container. The bind is attached at
// container creation time, so it is visible to init scripts that run during the
// container's first startup (before any PostStarts or PostReadies hooks).
func SeedBindMount() testcontainers.CustomizeRequestOption {
	dir := EnsureSeed()
	return addBind(dir + ":/data:ro")
}

// MySQLLocalInfileMount returns a testcontainers option that bind-mounts a
// MySQL config snippet enabling local-infile for both the server ([mysqld])
// and the client ([mysql]). MySQL's docker-entrypoint init scripts run the
// mysql CLI without --local-infile, so the config file is the only reliable
// way to enable it before init scripts execute.
func MySQLLocalInfileMount() testcontainers.CustomizeRequestOption {
	dir := EnsureSeed()
	cnfPath := filepath.Join(dir, "local-infile.cnf")
	const cnf = "[mysql]\nlocal-infile=1\n[mysqld]\nlocal_infile=1\n"
	if err := os.WriteFile(cnfPath, []byte(cnf), 0o644); err != nil {
		panic("testutil: write mysql local-infile config: " + err.Error())
	}
	return addBind(cnfPath + ":/etc/mysql/conf.d/local-infile.cnf:ro")
}

// addBind appends a single bind-mount string to the container's HostConfig,
// composing with any modifier already set on the request.
// testcontainers.WithHostConfigModifier replaces req.HostConfigModifier, so
// multiple calls to WithHostConfigModifier would clobber earlier ones. This
// helper chains instead of replacing.
func addBind(bind string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		prev := req.HostConfigModifier
		req.HostConfigModifier = func(hc *container.HostConfig) {
			if prev != nil {
				prev(hc)
			}
			hc.Binds = append(hc.Binds, bind)
		}
		return nil
	}
}
