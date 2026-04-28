package util

import (
	"strings"
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
	assert.True(t, IsEncrypted("enc:keyring:deadbeef"))
	assert.False(t, IsEncrypted("plaintext"))
	assert.False(t, IsEncrypted(""))
	assert.False(t, IsEncrypted("enc"))
}

func TestTaggedEncryptDecryptRoundtrip(t *testing.T) {
	methods := []string{"keyring", "master", "env"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			tagged, err := EncryptPasswordWithMethod("secret", validHexKey, method)
			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(tagged, "enc:"+method+":"))

			plaintext, got, err := DecryptPasswordWithMethod(tagged, validHexKey)
			require.NoError(t, err)
			assert.Equal(t, "secret", plaintext)
			assert.Equal(t, method, got)
		})
	}
}

func TestTaggedDecryptWrongKeyFails(t *testing.T) {
	tagged, err := EncryptPasswordWithMethod("secret", validHexKey, "master")
	require.NoError(t, err)

	wrongKey := "0000000000000000000000000000000000000000000000000000000000000000"
	_, _, err = DecryptPasswordWithMethod(tagged, wrongKey)
	assert.Error(t, err)
}

func TestTaggedDecryptInvalidFormatFails(t *testing.T) {
	_, _, err := DecryptPasswordWithMethod("enc:nocolon", validHexKey)
	assert.Error(t, err)

	_, _, err = DecryptPasswordWithMethod("plaintext", validHexKey)
	assert.Error(t, err)
}

func TestParseMethodTag(t *testing.T) {
	assert.Equal(t, "keyring", ParseMethodTag("enc:keyring:deadbeef"))
	assert.Equal(t, "master", ParseMethodTag("enc:master:deadbeef"))
	assert.Equal(t, "", ParseMethodTag("enc:deadbeef"))
	assert.Equal(t, "", ParseMethodTag("plaintext"))
	assert.Equal(t, "", ParseMethodTag(""))
}
