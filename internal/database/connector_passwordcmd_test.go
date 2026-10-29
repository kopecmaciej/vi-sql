package database_test

import (
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/database"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/cockroachdb"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/gaussdb"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/mariadb"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/mysql"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/oracle"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/postgres"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/sqlserver"
	"github.com/stretchr/testify/require"
)

func hasFormField(spec []database.FieldSpec, label string) bool {
	for _, f := range spec {
		if f.Label == label {
			return true
		}
	}
	return false
}

// driverDSNs holds a DSN URL per password-supporting driver, in that driver's
// own scheme, in two forms: one carrying an inline password and one without.
// Used to exercise each driver's DSN parsing branch.
var driverDSNs = map[string]struct {
	withPassword string
	passwordless string
}{
	"postgres":    {"postgres://alice:secret@localhost:5432/mydb", "postgres://alice@localhost:5432/mydb"},
	"cockroachdb": {"postgresql://alice:secret@localhost:26257/mydb", "postgresql://alice@localhost:26257/mydb"},
	"gaussdb":     {"gaussdb://alice:secret@localhost:8000/mydb", "gaussdb://alice@localhost:8000/mydb"},
	"mariadb":     {"mariadb://alice:secret@localhost:3306/mydb", "mariadb://alice@localhost:3306/mydb"},
	"mysql":       {"mysql://alice:secret@localhost:3306/mydb", "mysql://alice@localhost:3306/mydb"},
	"oracle":      {"oracle://alice:secret@localhost:1521/orcl", "oracle://alice@localhost:1521/orcl"},
	"sqlserver":   {"sqlserver://alice:secret@localhost:1433?database=mydb", "sqlserver://alice@localhost:1433?database=mydb"},
}

// TestPasswordCommandValidationAcrossDrivers runs every connector that offers a
// password command through the save path (BuildConfig then
// ValidatePasswordCommand), checking each accepts and rejects the same cases. If
// a future driver behaves differently, it fails here.
func TestPasswordCommandValidationAcrossDrivers(t *testing.T) {
	buildAndValidate := func(def database.ConnectorDef, fields map[string]string) error {
		cfg, err := def.BuildConfig(fields, nil)
		if err != nil {
			return err
		}
		return cfg.ValidatePasswordCommand()
	}

	tested := 0
	for _, name := range database.ListConnectors() {
		def, ok := database.GetConnector(name)
		require.True(t, ok)
		if !hasFormField(def.FormSpec, "Password command") {
			continue
		}
		tested++

		t.Run(name, func(t *testing.T) {
			dsns, ok := driverDSNs[name]
			require.Truef(t, ok, "no driverDSNs fixture for driver %q — add one", name)

			// Field mode: a command on its own is accepted.
			require.NoError(t, buildAndValidate(def, map[string]string{
				"Host":             "localhost",
				"Port":             "5432",
				"Username":         "alice",
				"Database":         "mydb",
				"Password command": "echo x",
			}), "a password command on its own should be accepted")

			// Field mode: a password and a command together are rejected.
			require.Error(t, buildAndValidate(def, map[string]string{
				"Host":             "localhost",
				"Port":             "5432",
				"Username":         "alice",
				"Database":         "mydb",
				"Password":         "secret",
				"Password command": "echo x",
			}), "a password and a password command together should be rejected")

			// DSN mode: an env-var DSN plus a command is rejected (raw DSN).
			require.Error(t, buildAndValidate(def, map[string]string{
				"DSN":              "$MY_DSN",
				"Password command": "echo x",
			}), "an env-var DSN plus a command should be rejected")

			// DSN mode: a DSN carrying an inline password plus a command is rejected.
			require.Error(t, buildAndValidate(def, map[string]string{
				"DSN":              dsns.withPassword,
				"Password command": "echo x",
			}), "a DSN with an inline password plus a command should be rejected")

			// DSN mode: a passwordless URL parses into Host, leaving the command
			// free to supply the password — accepted.
			require.NoError(t, buildAndValidate(def, map[string]string{
				"DSN":              dsns.passwordless,
				"Password command": "echo x",
			}), "a passwordless DSN URL plus a command should be accepted")
		})
	}

	require.GreaterOrEqual(t, tested, 7,
		"expected every password-supporting driver to be exercised — is a driver import or the field label missing?")
}
