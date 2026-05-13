package security

import (
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/zalando/go-keyring"
)

const (
	DefaultService = "vi-sql"
	DefaultAccount = "encryption-key"
)

func IsKeyringAvailable() bool {
	_, err := keyring.Get(DefaultService, DefaultAccount+"_probe")
	// ErrNotFound - backend is reachable but has no entry.
	return err == nil || err == keyring.ErrNotFound
}

func GetOrCreateKey(service, account string) (string, error) {
	if service == "" {
		service = DefaultService
	}
	if account == "" {
		account = DefaultAccount
	}

	key, err := keyring.Get(service, account)
	if err == nil {
		return key, nil
	}
	if err != keyring.ErrNotFound {
		return "", err
	}

	key, err = util.GenerateEncryptionKey()
	if err != nil {
		return "", err
	}
	if err := keyring.Set(service, account, key); err != nil {
		return "", err
	}
	return key, nil
}

// DeleteKey removes the stored key from the keyring.
func DeleteKey(service, account string) error {
	if service == "" {
		service = DefaultService
	}
	if account == "" {
		account = DefaultAccount
	}
	return keyring.Delete(service, account)
}
