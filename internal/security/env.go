package security

import "os"

const EncryptionKeyEnv = "VI_SQL_SECRET_KEY"

// GetEnvKey returns the encryption key from VI_SQL_SECRET_KEY, or "" if unset.
func GetEnvKey() string {
	return os.Getenv(EncryptionKeyEnv)
}
