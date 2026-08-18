package gaussdbconn

import (
	"crypto/hmac"
	"crypto/sha1"
	"fmt"
	"strings"

	"crypto/sha256"

	"github.com/tjfoc/gmsm/sm3"
	"golang.org/x/crypto/pbkdf2"
)

func charToByte(c byte) byte {
	return byte(strings.Index("0123456789ABCDEF", string(c)))
}

func hexStringToBytes(hexString string) []byte {

	if hexString == "" {
		return []byte("")
	}

	upperString := strings.ToUpper(hexString)
	bytes_len := len(upperString) / 2
	array := make([]byte, bytes_len)

	for i := 0; i < bytes_len; i++ {
		pos := i * 2
		array[i] = byte(charToByte(upperString[pos])<<4 | charToByte(upperString[pos+1]))
	}
	return array
}

func generateKFromPBKDF2(password string, random64code string, serverIteration int) []byte {
	random32code := hexStringToBytes(random64code)
	pwdEn := pbkdf2.Key([]byte(password), random32code, serverIteration, 32, sha1.New)
	return pwdEn
}

func bytesToHexString(src []byte) string {
	s := ""
	for i := 0; i < len(src); i++ {
		v := src[i] & 0xFF
		hv := fmt.Sprintf("%x", v)
		if len(hv) < 2 {
			s += hv
			s += "0"
		} else {
			s += hv
		}
	}
	return s
}

func getKeyFromHmac(key []byte, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func getSha256(message []byte) []byte {
	hash := sha256.New()
	hash.Write(message)

	return hash.Sum(nil)
}

func getSm3(message []byte) []byte {
	hash := sm3.New()
	hash.Write(message)

	return hash.Sum(nil)
}

func XorBetweenPassword(password1 []byte, password2 []byte, length int) []byte {
	array := make([]byte, length)
	for i := 0; i < length; i++ {
		array[i] = (password1[i] ^ password2[i])
	}
	return array
}

func bytesToHex(bytes []byte) []byte {
	lookup :=
		[16]byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f'}
	result := make([]byte, len(bytes)*2)
	pos := 0
	for i := 0; i < len(bytes); i++ {
		c := int(bytes[i] & 0xFF)
		j := c >> 4
		result[pos] = lookup[j]
		pos++
		j = c & 0xF
		result[pos] = lookup[j]
		pos++
	}
	return result

}

func RFC5802Algorithm(password string, random64code string, token string, serverSignature string, serverIteration int, method string) []byte {
	k := generateKFromPBKDF2(password, random64code, serverIteration)
	serverKey := getKeyFromHmac(k, []byte("Sever Key"))
	clientKey := getKeyFromHmac(k, []byte("Client Key"))
	var storedKey []byte

	if strings.EqualFold(method, "sha256") {
		storedKey = getSha256(clientKey)
	} else if strings.EqualFold(method, "sm3") {
		storedKey = getSm3(clientKey)
	}
	tokenByte := hexStringToBytes(token)
	clientSignature := getKeyFromHmac(serverKey, tokenByte)
	if serverSignature != "" && serverSignature != bytesToHexString(clientSignature) {
		return []byte("")
	}
	hmacResult := getKeyFromHmac(storedKey, tokenByte)
	h := XorBetweenPassword(hmacResult, clientKey, len(clientKey))
	result := bytesToHex(h)
	return result

}
