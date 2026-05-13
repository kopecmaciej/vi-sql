package security

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/argon2"
)

// Argon2idParams holds the KDF tuning parameters persisted in config.
type Argon2idParams struct {
	Memory      uint32 `yaml:"memory"`
	Iterations  uint32 `yaml:"iterations"`
	Parallelism uint8  `yaml:"parallelism"`
}

func DefaultArgon2idParams() Argon2idParams {
	return Argon2idParams{
		Memory:      64 * 1024, // 64 MiB
		Iterations:  3,
		Parallelism: 4,
	}
}

// GenerateSalt returns a new random 16-byte salt as a hex string.
func GenerateSalt() (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}
	return hex.EncodeToString(salt), nil
}

// DeriveKey derives a 32-byte AES key from passphrase+salt and returns it as hex.
// This call blocks for the full Argon2id duration (typically 100-300ms with
// default params) — never invoke it on the tview main goroutine. Use
// DeriveKeyAsync from UI code.
func DeriveKey(passphrase string, saltHex string, params Argon2idParams) (string, error) {
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return "", fmt.Errorf("decoding salt: %w", err)
	}
	key := argon2.IDKey([]byte(passphrase), salt, params.Iterations, params.Memory, params.Parallelism, 32)
	return hex.EncodeToString(key), nil
}

type DeriveResult struct {
	Key string
	Err error
}

// DeriveKeyAsync runs DeriveKey on a background goroutine and returns a
// single-shot channel with the result. Callers can select on the channel
// alongside UI events without blocking the tview main loop.
func DeriveKeyAsync(passphrase string, saltHex string, params Argon2idParams) <-chan DeriveResult {
	out := make(chan DeriveResult, 1)
	go func() {
		key, err := DeriveKey(passphrase, saltHex, params)
		out <- DeriveResult{Key: key, Err: err}
		close(out)
	}()
	return out
}
