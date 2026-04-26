package security

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"golang.org/x/crypto/argon2"
	"golang.org/x/term"
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
func DeriveKey(passphrase string, saltHex string, params Argon2idParams) (string, error) {
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return "", fmt.Errorf("decoding salt: %w", err)
	}
	key := argon2.IDKey([]byte(passphrase), salt, params.Iterations, params.Memory, params.Parallelism, 32)
	return hex.EncodeToString(key), nil
}

// PromptPassphrase reads a passphrase from the terminal without echo.
// When setup is true it prompts twice and requires them to match.
func PromptPassphrase(setup bool) (string, error) {
	fd := int(os.Stdin.Fd())

	if setup {
		fmt.Fprint(os.Stderr, "Set master password: ")
		first, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("reading passphrase: %w", err)
		}
		if len(first) == 0 {
			return "", fmt.Errorf("passphrase must not be empty")
		}

		fmt.Fprint(os.Stderr, "Confirm master password: ")
		second, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("reading passphrase confirmation: %w", err)
		}
		if string(first) != string(second) {
			return "", fmt.Errorf("passphrases do not match")
		}
		return string(first), nil
	}

	fmt.Fprint(os.Stderr, "Master password: ")
	pass, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading passphrase: %w", err)
	}
	return string(pass), nil
}
