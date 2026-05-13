package mysql

import (
	"context"
	"fmt"
	"strings"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

func init() {
	database.RegisterConnector("mysql", database.ConnectorDef{
		Factory: func(cfg *config.SQLConfig) (database.Driver, database.ValueFormatter, error) {
			client := NewClient(cfg)
			if err := client.Connect(context.Background()); err != nil {
				return nil, nil, err
			}
			return NewDao(client), database.DefaultFormatter{}, nil
		},
		FormSpec: []database.FieldSpec{
			{Kind: database.FieldTextArea, Label: "DSN", Clipboard: true, Rows: 3},
			{Kind: database.FieldLabel, Label: "Example", Default: "user:pass@tcp(host:3306)/db or $ENV_VAR"},
			{Kind: database.FieldLabel, Label: " ", Default: "----------------------------------------------"},
			{Kind: database.FieldInput, Label: "Host", Clipboard: true},
			{Kind: database.FieldInput, Label: "Port", Default: "3306", Clipboard: true},
			{Kind: database.FieldInput, Label: "Username", Clipboard: true},
			{Kind: database.FieldPassword, Label: "Password", Clipboard: true},
			{Kind: database.FieldInput, Label: "Database", Clipboard: true},
			// TLS values map to go-sql-driver/mysql tls config tokens.
			{Kind: database.FieldDropDown, Label: "TLS", Default: "false", Options: []string{"false", "skip-verify", "preferred", "true"}},
			{Kind: database.FieldInput, Label: "Timeout", Default: "5"},
		},
		PreFill: func(conn *config.SQLConfig) map[string]string {
			m := map[string]string{
				"DSN":      conn.DSN,
				"Host":     conn.Host,
				"Username": conn.Username,
				"Database": conn.Database,
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
			if conn.SSLMode != "" {
				m["TLS"] = conn.SSLMode
			}
			return m
		},
		BuildConfig: buildMySQLConfig,
	})
}

func buildMySQLConfig(fields map[string]string, editConn *config.SQLConfig) (*config.SQLConfig, error) {
	name := fields["Name"]

	timeout, err := database.ParseTimeoutField(fields["Timeout"], 5)
	if err != nil {
		return nil, err
	}

	trimmedDSN := strings.TrimSpace(fields["DSN"])
	dsnUnchanged := editConn != nil && trimmedDSN == editConn.DSN

	var host, username, password, dbName, sslMode, portStr string
	var fromURL bool

	if !dsnUnchanged && trimmedDSN != "" {
		if strings.HasPrefix(trimmedDSN, "$") {
			if name == "" {
				name = trimmedDSN
			}
			return &config.SQLConfig{Driver: "mysql", Name: name, DSN: trimmedDSN, Timeout: timeout}, nil
		}
		if strings.HasPrefix(trimmedDSN, "mysql://") {
			parsed, err := util.ParseMySQLDSN(trimmedDSN)
			if err != nil || parsed.Host == "" {
				return nil, fmt.Errorf("could not parse host from DSN — check format: mysql://user:pass@host:3306/db")
			}
			host, portStr, username, password, dbName, sslMode = parsed.Host, parsed.Port, parsed.Username, parsed.Password, parsed.Database, parsed.SSLMode
			fromURL = true
		} else {
			parsed, err := mysqldrv.ParseDSN(trimmedDSN)
			if err != nil {
				return nil, fmt.Errorf("could not parse DSN — expected user:pass@tcp(host:3306)/db or mysql://user:pass@host:3306/db: %w", err)
			}
			host, portStr, _ = strings.Cut(parsed.Addr, ":")
			if portStr == "" {
				portStr = "3306"
			}
			username, password, dbName, sslMode = parsed.User, parsed.Passwd, parsed.DBName, parsed.TLSConfig
		}
		if name == "" {
			name = host + ":" + portStr
		}
	} else {
		host, portStr = fields["Host"], fields["Port"]
		username, dbName, sslMode = fields["Username"], fields["Database"], fields["TLS"]
		password = database.PreserveEncryptedPassword(fields["Password"], editConn)
		if name == "" {
			name = host + ":" + portStr
		}
	}

	port, err := database.ParsePortField(portStr)
	if err != nil {
		return nil, err
	}
	cfg := &config.SQLConfig{
		Driver:   "mysql",
		Name:     name,
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		Database: dbName,
		SSLMode:  sslMode,
		Timeout:  timeout,
	}
	if fromURL {
		// Store safe DSN so the edit form can pre-fill the URL field, matching Postgres behaviour.
		cfg.DSN = cfg.GetSafeDSN()
	}
	return cfg, nil
}
