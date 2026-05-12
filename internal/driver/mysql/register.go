package mysql

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

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
			return NewDao(client), &Formatter{}, nil
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
			{Kind: database.FieldDropDown, Label: "SSL", Default: "disable", Options: []string{"disable", "enable"}},
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
			// Map sslMode back to the dropdown value.
			if conn.SSLMode != "" && conn.SSLMode != "disable" {
				m["SSL"] = "enable"
			} else {
				m["SSL"] = "disable"
			}
			return m
		},
		BuildConfig: buildMySQLConfig,
	})
}

func buildMySQLConfig(fields map[string]string, editConn *config.SQLConfig) (*config.SQLConfig, error) {
	name := fields["Name"]

	timeout := 5
	if t := fields["Timeout"]; t != "" {
		parsed, err := strconv.Atoi(t)
		if err != nil {
			return nil, fmt.Errorf("timeout must be a number")
		}
		timeout = parsed
	}

	trimmedDSN := strings.TrimSpace(fields["DSN"])
	dsnUnchanged := editConn != nil && trimmedDSN == editConn.DSN

	if !dsnUnchanged && trimmedDSN != "" {
		if strings.HasPrefix(trimmedDSN, "$") {
			if name == "" {
				name = trimmedDSN
			}
			return &config.SQLConfig{
				Driver:  "mysql",
				Name:    name,
				DSN:     trimmedDSN,
				Timeout: timeout,
			}, nil
		}
		// Parse as URL to extract individual fields.
		u, err := url.Parse(trimmedDSN)
		if err != nil || u.Host == "" {
			return nil, fmt.Errorf("could not parse host from DSN — expected mysql://user:pass@host:3306/db")
		}
		portStr := u.Port()
		if portStr == "" {
			portStr = "3306"
		}
		intPort, _ := strconv.Atoi(portStr)
		password, _ := u.User.Password()
		if name == "" {
			name = u.Hostname() + ":" + portStr
		}
		return &config.SQLConfig{
			Driver:   "mysql",
			Name:     name,
			Host:     u.Hostname(),
			Port:     intPort,
			Username: u.User.Username(),
			Password: password,
			Database: strings.TrimPrefix(u.Path, "/"),
			Timeout:  timeout,
		}, nil
	}

	host := fields["Host"]
	portStr := fields["Port"]
	intPort, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("port must be a number")
	}
	password := fields["Password"]
	if password == "" && editConn != nil && util.IsEncrypted(editConn.Password) && editConn.IsPasswordReadable() {
		password = editConn.Password
	}
	sslMode := "disable"
	if fields["SSL"] == "enable" {
		sslMode = "skip-verify"
	}
	if name == "" {
		name = host + ":" + portStr
	}
	return &config.SQLConfig{
		Driver:   "mysql",
		Name:     name,
		Host:     host,
		Port:     intPort,
		Username: fields["Username"],
		Password: password,
		Database: fields["Database"],
		SSLMode:  sslMode,
		Timeout:  timeout,
	}, nil
}
