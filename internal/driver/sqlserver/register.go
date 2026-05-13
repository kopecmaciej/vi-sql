package sqlserver

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

func init() {
	database.RegisterConnector("sqlserver", database.ConnectorDef{
		Quoter: util.BracketQuoter,
		Factory: func(cfg *config.SQLConfig) (database.Driver, database.ValueFormatter, error) {
			client := NewClient(cfg)
			if err := client.Connect(context.Background()); err != nil {
				return nil, nil, err
			}
			return NewDao(client), database.DefaultFormatter{}, nil
		},
		FormSpec: []database.FieldSpec{
			{Kind: database.FieldTextArea, Label: "DSN", Rows: 3},
			{Kind: database.FieldLabel, Label: "Example", Default: "sqlserver://user:password@host:1433?database=db&encrypt=disable"},
			{Kind: database.FieldLabel, Label: " ", Default: "----------------------------------------------"},
			{Kind: database.FieldInput, Label: "Host"},
			{Kind: database.FieldInput, Label: "Port", Default: "1433"},
			{Kind: database.FieldInput, Label: "Username"},
			{Kind: database.FieldPassword, Label: "Password"},
			{Kind: database.FieldInput, Label: "Database"},
			{Kind: database.FieldDropDown, Label: "Encrypt", Default: "disable", Options: []string{"disable", "false", "true"}},
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
				m["Encrypt"] = conn.SSLMode
			}
			return m
		},
		BuildConfig: buildSQLServerConfig,
	})
}

func buildSQLServerConfig(fields map[string]string, editConn *config.SQLConfig) (*config.SQLConfig, error) {
	name := fields["Name"]

	timeout, err := database.ParseTimeoutField(fields["Timeout"], 5)
	if err != nil {
		return nil, err
	}

	trimmedDSN := strings.TrimSpace(fields["DSN"])
	dsnUnchanged := editConn != nil && trimmedDSN == editConn.DSN

	var host, username, password, dbName, encrypt, portStr string
	var fromURL bool

	if !dsnUnchanged && trimmedDSN != "" {
		if strings.HasPrefix(trimmedDSN, "$") {
			if name == "" {
				name = trimmedDSN
			}
			return &config.SQLConfig{Driver: "sqlserver", Name: name, DSN: trimmedDSN, Timeout: timeout}, nil
		}
		if strings.HasPrefix(trimmedDSN, "sqlserver://") {
			parsed, err := util.ParseSQLServerDSN(trimmedDSN)
			if err != nil || parsed.Host == "" {
				return nil, fmt.Errorf("could not parse host from DSN — check format: sqlserver://user:pass@host:1433?database=db")
			}
			host, portStr, username, password, dbName, encrypt =
				parsed.Host, parsed.Port, parsed.Username, parsed.Password, parsed.Database, parsed.SSLMode
			fromURL = true
		} else {
			return nil, fmt.Errorf("DSN must start with sqlserver:// or $ENV_VAR")
		}
		if name == "" {
			name = host + ":" + portStr
		}
	} else {
		host, portStr = fields["Host"], fields["Port"]
		username, dbName, encrypt = fields["Username"], fields["Database"], fields["Encrypt"]
		password = database.PreserveEncryptedPassword(fields["Password"], editConn)
		if name == "" {
			name = host + ":" + portStr
		}
	}

	port, err := database.ParsePortField(portStr)
	if err != nil {
		if portStr == "" {
			port = 1433
		} else {
			return nil, err
		}
	}

	// Normalise port string for empty default case.
	if portStr == "" {
		portStr = strconv.Itoa(port)
	}
	if name == "" {
		name = host + ":" + portStr
	}

	cfg := &config.SQLConfig{
		Driver:   "sqlserver",
		Name:     name,
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		Database: dbName,
		SSLMode:  encrypt,
		Timeout:  timeout,
	}
	if fromURL {
		cfg.DSN = cfg.GetSafeDSN()
	}
	return cfg, nil
}
