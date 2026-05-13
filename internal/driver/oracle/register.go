package oracle

import (
	"context"
	"fmt"
	"strings"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

func init() {
	database.RegisterConnector("oracle", database.ConnectorDef{
		Quoter: util.ANSIQuoter,
		Factory: func(cfg *config.SQLConfig) (database.Driver, database.ValueFormatter, error) {
			client := NewClient(cfg)
			if err := client.Connect(context.Background()); err != nil {
				return nil, nil, err
			}
			return NewDao(client), database.DefaultFormatter{}, nil
		},
		FormSpec: []database.FieldSpec{
			{Kind: database.FieldTextArea, Label: "DSN", Rows: 3},
			{Kind: database.FieldLabel, Label: "Example", Default: "oracle://user:password@host:1521/service"},
			{Kind: database.FieldLabel, Label: " ", Default: "----------------------------------------------"},
			{Kind: database.FieldInput, Label: "Host"},
			{Kind: database.FieldInput, Label: "Port", Default: "1521"},
			{Kind: database.FieldInput, Label: "Username"},
			{Kind: database.FieldPassword, Label: "Password"},
			{Kind: database.FieldInput, Label: "Service Name"},
			{Kind: database.FieldInput, Label: "Timeout", Default: "5"},
		},
		PreFill: func(conn *config.SQLConfig) map[string]string {
			m := map[string]string{
				"DSN":          conn.DSN,
				"Host":         conn.Host,
				"Username":     conn.Username,
				"Service Name": conn.Database,
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
		BuildConfig: buildOracleConfig,
	})
}

func buildOracleConfig(fields map[string]string, editConn *config.SQLConfig) (*config.SQLConfig, error) {
	name := fields["Name"]

	timeout, err := database.ParseTimeoutField(fields["Timeout"], 5)
	if err != nil {
		return nil, err
	}

	trimmedDSN := strings.TrimSpace(fields["DSN"])
	dsnUnchanged := editConn != nil && trimmedDSN == editConn.DSN

	if !dsnUnchanged && trimmedDSN != "" {
		if strings.HasPrefix(trimmedDSN, "$") {
			if name == "" {
				name = trimmedDSN
			}
			return &config.SQLConfig{Driver: "oracle", Name: name, DSN: trimmedDSN, Timeout: timeout}, nil
		}
		if !strings.HasPrefix(trimmedDSN, "oracle://") {
			return nil, fmt.Errorf("DSN must start with oracle:// or $ENV_VAR")
		}
		parsed, err := util.ParseOracleDSN(trimmedDSN)
		if err != nil || parsed.Host == "" {
			return nil, fmt.Errorf("could not parse host from DSN — check format: oracle://user:pass@host:1521/service")
		}
		port, _ := database.ParsePortField(parsed.Port)
		if port == 0 {
			port = 1521
		}
		if name == "" {
			name = parsed.Host + ":" + parsed.Port
		}
		cfg := &config.SQLConfig{
			Driver:   "oracle",
			Name:     name,
			Host:     parsed.Host,
			Port:     port,
			Username: parsed.Username,
			Password: parsed.Password,
			Database: parsed.Database,
			Timeout:  timeout,
		}
		cfg.DSN = cfg.GetSafeDSN()
		return cfg, nil
	}

	host := fields["Host"]
	portStr := fields["Port"]
	port, err := database.ParsePortField(portStr)
	if err != nil {
		if portStr == "" {
			port = 1521
		} else {
			return nil, err
		}
	}
	password := database.PreserveEncryptedPassword(fields["Password"], editConn)
	if name == "" {
		name = host + ":" + portStr
	}
	return &config.SQLConfig{
		Driver:   "oracle",
		Name:     name,
		Host:     host,
		Port:     port,
		Username: fields["Username"],
		Password: password,
		Database: fields["Service Name"],
		Timeout:  timeout,
	}, nil
}
