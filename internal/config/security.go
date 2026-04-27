package config

import (
	"fmt"
	"os"
	"strings"

	sec "github.com/kopecmaciej/vi-sql/internal/security"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
)

var EncryptionKey = ""

const (
	SecurityMethodKeyring = "keyring"
	SecurityMethodMaster  = "master"
	SecurityMethodEnv     = "env"
	SecurityMethodOff     = "off"
)

type SecurityConfig struct {
	Method           string `yaml:"method"`
	KeyringService   string `yaml:"keyringService,omitempty"`
	KeyringAccount   string `yaml:"keyringAccount,omitempty"`
	MasterSalt       string `yaml:"masterSalt,omitempty"`
	MasterMemory     uint32 `yaml:"masterMemory,omitempty"`
	MasterIter       uint32 `yaml:"masterIterations,omitempty"`
	MasterParallel   uint8  `yaml:"masterParallelism,omitempty"`
	MasterWrappedKey string `yaml:"masterWrappedKey,omitempty"`
}

func (c *Config) LoadEncryptionKey() error {
	if c.EncryptionKeyPath != nil {
		key, err := os.ReadFile(*c.EncryptionKeyPath)
		if err != nil {
			return fmt.Errorf("failed to load encryption key: %s", err)
		}
		EncryptionKey = strings.TrimSpace(string(key))
		return nil
	}

	method := c.Security.Method
	if method == "" {
		method = SecurityMethodKeyring
	}

	switch method {
	case SecurityMethodKeyring:
		if envKey := sec.GetEnvKey(); envKey != "" {
			log.Warn().Msgf("%s is set but Security.Method is %q — ignoring env var", sec.EncryptionKeyEnv, method)
		}
		return c.loadKeyringKey()
	case SecurityMethodMaster:
		if envKey := sec.GetEnvKey(); envKey != "" {
			log.Warn().Msgf("%s is set but Security.Method is %q — ignoring env var", sec.EncryptionKeyEnv, method)
		}
		return c.loadMasterKey()
	case SecurityMethodEnv:
		envKey := sec.GetEnvKey()
		if envKey == "" {
			log.Warn().Msgf("Security method is %q but %s is not set — passwords will not be decrypted", method, sec.EncryptionKeyEnv)
			return nil
		}
		EncryptionKey = envKey
		return nil
	case SecurityMethodOff:
	}
	return nil
}

func (c *Config) loadKeyringKey() error {
	key, err := sec.EnsureKey(c.Security.KeyringService, c.Security.KeyringAccount)
	if err != nil {
		return fmt.Errorf("keyring unavailable: %w", err)
	}
	EncryptionKey = key
	return nil
}

// MasterParams returns the Argon2id parameters from the security config,
// substituting defaults when the config has none yet (fresh setup).
func (c *Config) MasterParams() sec.Argon2idParams {
	p := sec.Argon2idParams{
		Memory:      c.Security.MasterMemory,
		Iterations:  c.Security.MasterIter,
		Parallelism: c.Security.MasterParallel,
	}
	if p.Memory == 0 {
		p = sec.DefaultArgon2idParams()
	}
	return p
}

// IsMasterConfigured reports whether the master password setup has run.
func (c *Config) IsMasterConfigured() bool {
	return c.Security.MasterWrappedKey != ""
}

// ApplyMasterSetup completes a fresh master-password setup using a KEK that
// the caller has already derived (typically via DeriveKeyAsync). It generates
// a new data key, wraps it with the KEK, persists the security config, and
// loads the data key as the active EncryptionKey.
func (c *Config) ApplyMasterSetup(kek string, salt string, params sec.Argon2idParams) error {
	dataKey, err := util.GenerateEncryptionKey()
	if err != nil {
		return err
	}
	wrapped, err := util.EncryptPassword(dataKey, kek)
	if err != nil {
		return fmt.Errorf("wrapping data key: %w", err)
	}
	c.Security.MasterSalt = salt
	c.Security.MasterMemory = params.Memory
	c.Security.MasterIter = params.Iterations
	c.Security.MasterParallel = params.Parallelism
	c.Security.MasterWrappedKey = wrapped
	if err := c.UpdateConfig(); err != nil {
		return fmt.Errorf("persisting master password config: %w", err)
	}
	EncryptionKey = dataKey
	return nil
}

// ApplyMasterUnlock unwraps the stored data key with a KEK that the caller
// has already derived from the user-typed passphrase. On success the data
// key is loaded as EncryptionKey.
func (c *Config) ApplyMasterUnlock(kek string) error {
	dataKey, err := util.DecryptPassword(c.Security.MasterWrappedKey, kek)
	if err != nil {
		return err
	}
	EncryptionKey = dataKey
	return nil
}

// ApplyMasterChange re-wraps the existing data key with a new KEK and salt,
// without touching the data key itself. Existing connection ciphertexts stay
// valid because the data key is unchanged. The caller must verify the old
// passphrase first via ApplyMasterUnlock.
func (c *Config) ApplyMasterChange(newKEK string, newSalt string, params sec.Argon2idParams) error {
	if EncryptionKey == "" {
		return fmt.Errorf("data key not loaded — unlock first")
	}
	wrapped, err := util.EncryptPassword(EncryptionKey, newKEK)
	if err != nil {
		return fmt.Errorf("re-wrapping data key: %w", err)
	}
	c.Security.MasterSalt = newSalt
	c.Security.MasterMemory = params.Memory
	c.Security.MasterIter = params.Iterations
	c.Security.MasterParallel = params.Parallelism
	c.Security.MasterWrappedKey = wrapped
	return c.UpdateConfig()
}

// ApplyMasterReset clears the wrapped key and salt, and zeros the password
// field on every connection that has an encrypted password (the ciphertext
// is unrecoverable without the old data key, so the user must re-enter).
// Host/user/database fields are preserved.
func (c *Config) ApplyMasterReset() error {
	c.Security.MasterSalt = ""
	c.Security.MasterMemory = 0
	c.Security.MasterIter = 0
	c.Security.MasterParallel = 0
	c.Security.MasterWrappedKey = ""
	for i := range c.Connections {
		if util.IsEncrypted(c.Connections[i].Password) {
			c.Connections[i].Password = ""
		}
	}
	EncryptionKey = ""
	return c.UpdateConfig()
}

func (c *Config) loadMasterKey() error {
	params := c.MasterParams()

	if !c.IsMasterConfigured() {
		pass, err := sec.PromptPassphrase(true)
		if err != nil {
			return fmt.Errorf("master password: %w", err)
		}
		salt, err := sec.GenerateSalt()
		if err != nil {
			return err
		}
		kek, err := sec.DeriveKey(pass, salt, params)
		if err != nil {
			return err
		}
		return c.ApplyMasterSetup(kek, salt, params)
	}

	for attempt := 1; attempt <= 3; attempt++ {
		pass, err := sec.PromptPassphrase(false)
		if err != nil {
			return fmt.Errorf("master password: %w", err)
		}
		kek, err := sec.DeriveKey(pass, c.Security.MasterSalt, params)
		if err != nil {
			return err
		}
		if err := c.ApplyMasterUnlock(kek); err == nil {
			return nil
		}
		if attempt < 3 {
			fmt.Fprintln(os.Stderr, "Wrong master password, please try again.")
		}
	}
	return fmt.Errorf("master password: too many failed attempts")
}
