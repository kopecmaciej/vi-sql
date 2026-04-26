package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validHexKey = "6368616e676520746869732070617373776f726420746f206120736563726574"

func TestGenerateEncryptionKey(t *testing.T) {
	key, err := GenerateEncryptionKey()
	require.NoError(t, err)
	assert.Len(t, key, 64)

	key2, err := GenerateEncryptionKey()
	require.NoError(t, err)
	assert.NotEqual(t, key, key2)
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	tests := []struct {
		password string
	}{
		{"secret"},
		{"p@$$w0rd!"},
		{"unicode: 日本語"},
		{"with spaces and\nnewlines"},
	}

	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			encrypted, err := EncryptPassword(tt.password, validHexKey)
			require.NoError(t, err)
			assert.NotEmpty(t, encrypted)
			assert.NotEqual(t, tt.password, encrypted)

			decrypted, err := DecryptPassword(encrypted, validHexKey)
			require.NoError(t, err)
			assert.Equal(t, tt.password, decrypted)
		})
	}
}

func TestEncryptEmptyPasswordReturnsEmpty(t *testing.T) {
	encrypted, err := EncryptPassword("", validHexKey)
	require.NoError(t, err)
	assert.Empty(t, encrypted)
}

func TestDecryptEmptyReturnsEmpty(t *testing.T) {
	decrypted, err := DecryptPassword("", validHexKey)
	require.NoError(t, err)
	assert.Empty(t, decrypted)
}

func TestEncryptProducesDifferentOutputEachCall(t *testing.T) {
	enc1, err := EncryptPassword("secret", validHexKey)
	require.NoError(t, err)
	enc2, err := EncryptPassword("secret", validHexKey)
	require.NoError(t, err)
	assert.NotEqual(t, enc1, enc2)
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	encrypted, err := EncryptPassword("secret", validHexKey)
	require.NoError(t, err)

	wrongKey := "0000000000000000000000000000000000000000000000000000000000000000"
	_, err = DecryptPassword(encrypted, wrongKey)
	assert.Error(t, err)
}

func TestDecryptTamperedCiphertextFails(t *testing.T) {
	encrypted, err := EncryptPassword("secret", validHexKey)
	require.NoError(t, err)

	tampered := encrypted[:len(encrypted)-4] + "ffff"
	_, err = DecryptPassword(tampered, validHexKey)
	assert.Error(t, err)
}

func TestEncryptInvalidHexKeyFails(t *testing.T) {
	_, err := EncryptPassword("secret", "not-hex")
	assert.Error(t, err)
}

func TestDecryptInvalidHexKeyFails(t *testing.T) {
	_, err := DecryptPassword("aabbcc", "not-hex")
	assert.Error(t, err)
}

func TestDecryptTooShortCiphertextFails(t *testing.T) {
	_, err := DecryptPassword("aabb", validHexKey)
	assert.Error(t, err)
}

func TestEncryptedValueHasPrefix(t *testing.T) {
	enc, err := EncryptPassword("secret", validHexKey)
	require.NoError(t, err)
	assert.True(t, IsEncrypted(enc))
}

func TestIsEncrypted(t *testing.T) {
	assert.True(t, IsEncrypted("enc:deadbeef"))
	assert.False(t, IsEncrypted("plaintext"))
	assert.False(t, IsEncrypted(""))
	assert.False(t, IsEncrypted("enc")) // no colon
	assert.True(t, IsEncrypted("enc:")) // prefix only — invalid ciphertext but still detected
}
