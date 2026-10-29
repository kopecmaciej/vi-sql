package database

import (
	"fmt"
	"strconv"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

// ParseTimeoutField parses a connection-form Timeout field as seconds; empty input returns fallback.
func ParseTimeoutField(s string, fallback int) (int, error) {
	if s == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("timeout must be a number")
	}
	return n, nil
}

func ParsePortField(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("port must be a number")
	}
	return n, nil
}

// PreserveEncryptedPassword keeps the existing encrypted password when the user
// leaves the password field blank during edit. Plaintext fields and add-mode
// pass through unchanged.
func PreserveEncryptedPassword(fieldPassword string, editConn *config.SQLConfig) string {
	if fieldPassword != "" || editConn == nil {
		return fieldPassword
	}
	if util.IsEncrypted(editConn.Password) && editConn.IsPasswordReadable() {
		return editConn.Password
	}
	return fieldPassword
}

// ResolvePasswordFields errors if both the password and passwordCommand form
// fields are set. In edit mode an empty password field preserves an existing
// encrypted password when no command is provided.
func ResolvePasswordFields(fieldPassword, fieldCmd string, editConn *config.SQLConfig) (string, string, error) {
	if fieldPassword != "" && fieldCmd != "" {
		return "", "", fmt.Errorf("password and password command are mutually exclusive — fill only one")
	}
	if fieldCmd != "" {
		return "", fieldCmd, nil
	}
	return PreserveEncryptedPassword(fieldPassword, editConn), "", nil
}
