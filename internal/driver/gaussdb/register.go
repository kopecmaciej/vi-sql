package gaussdb

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

func init() {
	database.RegisterConnector("gaussdb", database.ConnectorDef{
		Quoter: util.BacktickQuoter,
		Factory: func(cfg *config.SQLConfig) (database.Driver, database.ValueFormatter, error) {
			client := NewClient(cfg)
			if err := client.Connect(); err != nil {
				return nil, nil, err
			}
			if err := client.Ping(); err != nil {
				return nil, nil, err
			}
			return NewDao(client), database.DefaultFormatter{}, nil
		},
		FormSpec: []database.FieldSpec{
			{Kind: database.FieldTextArea, Label: "DSN", Default: "", Rows: 3},
			{Kind: database.FieldLabel, Label: "Example", Default: "gaussdb://user:password@host:8000/db?sslmode=disable"},
			{Kind: database.FieldLabel, Label: " ", Default: "----------------------------------------------"},
			{Kind: database.FieldInput, Label: "Host"},
			{Kind: database.FieldInput, Label: "Port", Default: "8000"},
			{Kind: database.FieldInput, Label: "Username"},
			{Kind: database.FieldPassword, Label: "Password"},
			{Kind: database.FieldInput, Label: "Database"},
			{Kind: database.FieldDropDown, Label: "SSL Mode", Default: "disable", Options: []string{"disable", "require", "verify-ca", "verify-full", "prefer", "allow"}},
			{Kind: database.FieldDropDown, Label: "Target Session", Default: "primary", Options: []string{"primary", "standby", "any"}},
			{Kind: database.FieldInput, Label: "Timeout", Default: "5"},
		},
		PreFill: func(conn *config.SQLConfig) map[string]string {
			m := map[string]string{
				"DSN":            conn.DSN,
				"Host":           conn.Host,
				"Username":       conn.Username,
				"Database":       conn.Database,
				"SSL Mode":       conn.SSLMode,
				"Target Session": conn.GetDriverOption("target_session_attrs"),
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
		BuildConfig: buildGaussDBConfig,
	})
}

func buildGaussDBConfig(fields map[string]string, editConn *config.SQLConfig) (*config.SQLConfig, error) {
	name := fields["Name"]

	timeout, err := database.ParseTimeoutField(fields["Timeout"], 5)
	if err != nil {
		return nil, err
	}

	trimmedDSN := strings.TrimSpace(fields["DSN"])
	dsnUnchanged := editConn != nil && trimmedDSN == editConn.DSN

	if dsnUnchanged && util.IsMultiHostDSN(trimmedDSN) {
		out := *editConn
		out.Name = name
		out.Timeout = timeout
		opts := make(map[string]string, len(editConn.DriverOptions)+2)
		for k, v := range editConn.DriverOptions {
			opts[k] = v
		}
		out.DriverOptions = opts
		applyGaussDBFormOptions(&out, fields)
		return &out, nil
	}

	if !dsnUnchanged && trimmedDSN != "gaussdb://" && trimmedDSN != "" {
		if name == "" {
			name = trimmedDSN
		}
		cfg := &config.SQLConfig{
			Driver:  "gaussdb",
			Name:    name,
			DSN:     trimmedDSN,
			Timeout: timeout,
		}
		if strings.HasPrefix(trimmedDSN, "$") {
			return cfg, nil
		}
		if !strings.HasPrefix(trimmedDSN, "gaussdb://") {
			return nil, fmt.Errorf("DSN must start with gaussdb://")
		}
		if util.IsMultiHostDSN(trimmedDSN) {
			stripped, pw := util.SplitDSNPassword(trimmedDSN)
			cfg.DSN = stripped
			cfg.Password = pw
			if cfg.Name == trimmedDSN {
				cfg.Name = stripped
			}
			return cfg, nil
		}
		parsed, err := util.ParseGaussDBDSN(trimmedDSN)
		if err != nil || parsed.Host == "" {
			return nil, fmt.Errorf("could not parse host from DSN — check format: gaussdb://user:pass@host:8000/db")
		}
		parsedPort, _ := strconv.Atoi(parsed.Port)
		cfg.Host = parsed.Host
		cfg.Port = parsedPort
		cfg.Username = parsed.Username
		cfg.Password = parsed.Password
		cfg.Database = parsed.Database
		cfg.SSLMode = parsed.SSLMode
		if parsed.SSLMode != "" {
			cfg.SetDriverOption("sslmode", parsed.SSLMode)
		}
		if parsed.TargetSessionAttrs != "" {
			cfg.SetDriverOption("target_session_attrs", parsed.TargetSessionAttrs)
		}
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
	cfg := &config.SQLConfig{
		Driver:   "gaussdb",
		Name:     name,
		Host:     host,
		Port:     port,
		Username: fields["Username"],
		Password: password,
		Database: fields["Database"],
		SSLMode:  fields["SSL Mode"],
		Timeout:  timeout,
	}
	applyGaussDBFormOptions(cfg, fields)
	return cfg, nil
}

// applyGaussDBFormOptions packs form settings into DriverOptions keyed by
// their DSN parameter names.
func applyGaussDBFormOptions(cfg *config.SQLConfig, fields map[string]string) {
	if v := fields["SSL Mode"]; v != "" {
		cfg.SetDriverOption("sslmode", v)
	}
	if v := fields["Target Session"]; v != "" {
		cfg.SetDriverOption("target_session_attrs", v)
	}
}
