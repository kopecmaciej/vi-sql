package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

type EncryptionError struct {
	Operation string
	Err       error
}

func (e *EncryptionError) Error() string {
	return fmt.Sprintf("encryption error during %s: %v", e.Operation, e.Err)
}

func (e *EncryptionError) Unwrap() error {
	return e.Err
}

const (
	KeyLength = 32
	EncPrefix = "enc:"
)

func IsEncrypted(s string) bool {
	return strings.HasPrefix(s, EncPrefix)
}

// EncryptPasswordWithMethod encrypts password and embeds the security method enc:<method>:<ciphertext>.
func EncryptPasswordWithMethod(password, hexKey, method string) (ciphertext string, err error) {
	enc, err := EncryptPassword(password, hexKey)
	if err != nil {
		return "", err
	}
	return EncPrefix + method + ":" + strings.TrimPrefix(enc, EncPrefix), nil
}

// DecryptPasswordWithMethod decrypts an enc:<method>:<hex-ciphertext> value
func DecryptPasswordWithMethod(encryptedHex, hexKey string) (plaintext string, method string, err error) {
	rest, ok := strings.CutPrefix(encryptedHex, EncPrefix)
	if !ok {
		return "", "", &EncryptionError{Operation: "tag parsing", Err: fmt.Errorf("not a tagged value")}
	}
	method, hex, ok := strings.Cut(rest, ":")
	if !ok {
		return "", "", &EncryptionError{Operation: "tag parsing", Err: fmt.Errorf("invalid tagged format")}
	}
	plaintext, err = DecryptPassword(EncPrefix+hex, hexKey)
	return plaintext, method, err
}

// ParseMethodTag returns the method embedded in a tagged password value, or "" if untagged.
func ParseMethodTag(s string) string {
	rest, ok := strings.CutPrefix(s, EncPrefix)
	if !ok {
		return ""
	}
	method, _, ok := strings.Cut(rest, ":")
	if !ok {
		return ""
	}
	return method
}

func GenerateEncryptionKey() (string, error) {
	key := make([]byte, KeyLength)
	_, err := rand.Read(key)
	if err != nil {
		return "", &EncryptionError{Operation: "key generation", Err: err}
	}
	return hex.EncodeToString(key), nil
}

func PrintEncryptionKeyInstructions() {
	key, err := GenerateEncryptionKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate encryption key: %v\n", err)
		return
	}

	fmt.Println("Encryption key successfully generated for vi-sql:")
	fmt.Println(key)
	fmt.Println("\nSet it as an environment variable to use the 'env' security method:")
	fmt.Println("  export VI_SQL_SECRET_KEY=" + key)
}

func EncryptPassword(password string, hexKey string) (string, error) {
	if password == "" {
		return "", nil
	}
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", &EncryptionError{Operation: "key decoding", Err: err}
	}
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", &EncryptionError{Operation: "cipher creation", Err: err}
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", &EncryptionError{Operation: "GCM creation", Err: err}
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", &EncryptionError{Operation: "nonce generation", Err: err}
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(password), nil)
	return EncPrefix + hex.EncodeToString(ciphertext), nil
}

func DecryptPassword(encryptedHex string, hexKey string) (string, error) {
	if encryptedHex == "" {
		return "", nil
	}
	encryptedHex = strings.TrimPrefix(encryptedHex, EncPrefix)
	ciphertext, err := hex.DecodeString(encryptedHex)
	if err != nil {
		return "", &EncryptionError{Operation: "decode encrypted password", Err: err}
	}
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", &EncryptionError{Operation: "key decoding", Err: err}
	}
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", &EncryptionError{Operation: "cipher creation", Err: err}
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", &EncryptionError{Operation: "GCM creation", Err: err}
	}
	if len(ciphertext) < gcm.NonceSize() {
		return "", &EncryptionError{Operation: "ciphertext validation", Err: fmt.Errorf("ciphertext too short")}
	}
	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", &EncryptionError{Operation: "password decryption", Err: err}
	}
	return string(plaintext), nil
}
