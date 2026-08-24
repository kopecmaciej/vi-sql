package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestUpdateConnectionPreservesIDAndLastUsed(t *testing.T) {
	tmp := t.TempDir()
	cfg := &Config{
		ConfigPath: filepath.Join(tmp, "config.yaml"),
		Connections: []SQLConfig{
			{ID: "conn-1", Name: "gaussdb", Driver: "gaussdb", Host: "h", Port: 8000, Username: "u", Database: "d", SSLMode: "disable"},
		},
	}
	cfg.Connections[0].LastUsed = time.Date(2026, 8, 18, 14, 50, 0, 0, time.UTC)

	updated := &SQLConfig{
		Driver:        "gaussdb",
		Name:          "gaussdb",
		Host:          "new-host",
		Port:          8000,
		Username:      "u",
		Password:      "secret",
		Database:      "d",
		SSLMode:       "disable",
		DriverOptions: map[string]string{"target_session_attrs": "primary"},
	}

	if err := cfg.UpdateConnection("gaussdb", updated); err != nil {
		t.Fatal(err)
	}

	conn, err := cfg.GetConnectionByName("gaussdb")
	if err != nil {
		t.Fatal(err)
	}
	if conn.ID != "conn-1" {
		t.Errorf("ID not preserved: got %q, want conn-1", conn.ID)
	}
	if conn.LastUsed.IsZero() {
		t.Error("LastUsed should be preserved across a form edit")
	}
	if got := conn.LastUsed.Format("2006-01-02 15:04"); got != "2026-08-18 14:50" {
		t.Errorf("LastUsed changed: got %s", got)
	}
	if conn.Host != "new-host" {
		t.Errorf("edit itself not applied: host = %s", conn.Host)
	}
	if conn.Password != "secret" {
		t.Errorf("plain password not stored: %s", conn.Password)
	}
}
