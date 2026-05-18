package cockroachdb

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	pgdriver "github.com/kopecmaciej/vi-sql/internal/driver/postgres"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

func init() {
	database.RegisterConnector("cockroachdb", database.ConnectorDef{
		Quoter: util.ANSIQuoter,
		Factory: func(cfg *config.SQLConfig) (database.Driver, database.ValueFormatter, error) {
			client := pgdriver.NewClient(cfg)
			if err := client.Connect(); err != nil {
				return nil, nil, err
			}
			if err := client.Ping(); err != nil {
				return nil, nil, err
			}
			return NewDao(client), database.DefaultFormatter{}, nil
		},
		FormSpec: []database.FieldSpec{
			{Kind: database.FieldTextArea, Label: "DSN", Default: "postgresql://", Clipboard: true, Rows: 3},
			{Kind: database.FieldLabel, Label: "Example", Default: "postgresql://user:password@host:26257/db?sslmode=require"},
			{Kind: database.FieldLabel, Label: " ", Default: "----------------------------------------------"},
			{Kind: database.FieldInput, Label: "Host", Clipboard: true},
			{Kind: database.FieldInput, Label: "Port", Default: "26257", Clipboard: true},
			{Kind: database.FieldInput, Label: "Username", Clipboard: true},
			{Kind: database.FieldPassword, Label: "Password", Clipboard: true},
			{Kind: database.FieldInput, Label: "Database", Clipboard: true},
			{Kind: database.FieldDropDown, Label: "SSL Mode", Default: "disable", Options: []string{"disable", "require", "verify-ca", "verify-full", "prefer", "allow"}},
			{Kind: database.FieldInput, Label: "Timeout", Default: "5"},
		},
		PreFill: func(conn *config.SQLConfig) map[string]string {
			m := map[string]string{
				"DSN":      conn.DSN,
				"Host":     conn.Host,
				"Username": conn.Username,
				"Database": conn.Database,
				"SSL Mode": conn.SSLMode,
			}
			if conn.Port > 0 {
				m["Port"] = fmt.Sprintf("%d", conn.Port)
			}
			if conn.Timeout > 0 {
				m["Timeout"] = fmt.Sprintf("%d", conn.Timeout)
			}
			if !util.IsEncrypted(conn.Password) {
				m["Password"] = conn.Password
			}
			return m
		},
		BuildConfig: buildCockroachDBConfig,
	})
}

func buildCockroachDBConfig(fields map[string]string, editConn *config.SQLConfig) (*config.SQLConfig, error) {
	name := fields["Name"]

	timeout, err := database.ParseTimeoutField(fields["Timeout"], 5)
	if err != nil {
		return nil, err
	}

	trimmedDSN := strings.TrimSpace(fields["DSN"])
	dsnUnchanged := editConn != nil && trimmedDSN == editConn.DSN

	// cockroachdb:// is an alias for postgresql:// — same wire protocol.
	if rest, ok := strings.CutPrefix(trimmedDSN, "cockroachdb://"); ok {
		trimmedDSN = "postgresql://" + rest
	}

	if !dsnUnchanged && trimmedDSN != "postgresql://" && trimmedDSN != "postgres://" && trimmedDSN != "" {
		if name == "" {
			name = trimmedDSN
		}
		cfg := &config.SQLConfig{
			Driver:  "cockroachdb",
			Name:    name,
			DSN:     trimmedDSN,
			Timeout: timeout,
		}
		if strings.HasPrefix(trimmedDSN, "$") {
			return cfg, nil
		}
		if !strings.HasPrefix(trimmedDSN, "postgres://") && !strings.HasPrefix(trimmedDSN, "postgresql://") {
			return nil, fmt.Errorf("DSN must start with postgresql://, postgres://, or cockroachdb://")
		}
		parsed, err := util.ParsePostgresDSN(trimmedDSN)
		if err != nil || parsed.Host == "" {
			return nil, fmt.Errorf("could not parse host from DSN — check format: postgresql://user:pass@host:26257/db")
		}
		parsedPort, _ := strconv.Atoi(parsed.Port)
		cfg.Host = parsed.Host
		cfg.Port = parsedPort
		cfg.Username = parsed.Username
		cfg.Password = parsed.Password
		cfg.Database = parsed.Database
		cfg.SSLMode = parsed.SSLMode
		cfg.DSN = cfg.GetSafeDSN()
		return cfg, nil
	}

	host := fields["Host"]
	portStr := fields["Port"]
	port, err := database.ParsePortField(portStr)
	if err != nil {
		return nil, err
	}
	password := database.PreserveEncryptedPassword(fields["Password"], editConn)
	if name == "" {
		name = host + ":" + portStr
	}
	return &config.SQLConfig{
		Driver:   "cockroachdb",
		Name:     name,
		Host:     host,
		Port:     port,
		Username: fields["Username"],
		Password: password,
		Database: fields["Database"],
		SSLMode:  fields["SSL Mode"],
		Timeout:  timeout,
	}, nil
}
