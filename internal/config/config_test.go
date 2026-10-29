package config

import (
	"testing"
)

func TestResolvePassword(t *testing.T) {
	tests := []struct {
		name    string
		conn    SQLConfig
		want    string
		wantErr bool
	}{
		{
			name: "password command output is used",
			conn: SQLConfig{PasswordCommand: "echo secretpass"},
			want: "secretpass",
		},
		{
			name: "command output is trimmed of surrounding whitespace",
			conn: SQLConfig{PasswordCommand: "printf '  spaced  \\n'"},
			want: "spaced",
		},
		{
			name:    "failed password command errors",
			conn:    SQLConfig{PasswordCommand: "false"},
			wantErr: true,
		},
		{
			name: "plaintext password passes through",
			conn: SQLConfig{Password: "plain"},
			want: "plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.conn.ResolvePassword()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolvePassword() err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ResolvePassword() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidatePasswordCommand(t *testing.T) {
	tests := []struct {
		name    string
		conn    SQLConfig
		wantErr bool
	}{
		{
			name: "field mode with command is allowed",
			conn: SQLConfig{Host: "localhost", PasswordCommand: "echo x"},
		},
		{
			name: "field mode with masked display DSN is allowed",
			conn: SQLConfig{Host: "localhost", DSN: "postgres://a:***@h/db", PasswordCommand: "echo x"},
		},
		{
			name: "raw DSN without command is allowed",
			conn: SQLConfig{DSN: "postgres://a@h/db"},
		},
		{
			name:    "raw DSN with command is rejected",
			conn:    SQLConfig{DSN: "postgres://a@h/db", PasswordCommand: "echo x"},
			wantErr: true,
		},
		{
			name:    "plain password with command is rejected",
			conn:    SQLConfig{Host: "localhost", Password: "plain", PasswordCommand: "echo x"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.conn.ValidatePasswordCommand(); (err != nil) != tt.wantErr {
				t.Errorf("ValidatePasswordCommand() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsPasswordReadable_PasswordCommand(t *testing.T) {
	conn := SQLConfig{PasswordCommand: "echo secret"}
	if !conn.IsPasswordReadable() {
		t.Error("IsPasswordReadable should return true when PasswordCommand is set")
	}
}
