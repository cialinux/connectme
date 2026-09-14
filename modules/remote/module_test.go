package remote

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestSignedDataWireFormat(t *testing.T) {
	key := []byte("0123456789abcdef")
	value := map[string]any{"username": "test", "expires": 1234567890}
	encoded, err := signedData(key, value)
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := aes.NewCipher(key)
	cipher.NewCBCDecrypter(block, make([]byte, 16)).CryptBlocks(data, data)
	padding := int(data[len(data)-1])
	if padding < 1 || padding > 16 {
		t.Fatal("invalid padding")
	}
	for _, b := range data[len(data)-padding:] {
		if int(b) != padding {
			t.Fatal("invalid PKCS7")
		}
	}
	plain := data[:len(data)-padding]
	expected, _ := json.Marshal(value)
	mac := hmac.New(sha256.New, key)
	mac.Write(expected)
	if !hmac.Equal(plain[:32], mac.Sum(nil)) {
		t.Fatal("signature mismatch")
	}
	if string(plain[32:]) != string(expected) {
		t.Fatal("payload mismatch")
	}
}
func TestSignedDataRejectsBadKey(t *testing.T) {
	if _, err := signedData([]byte("bad"), map[string]string{}); err == nil {
		t.Fatal("invalid key accepted")
	}
}

func TestSessionIsolation(t *testing.T) {
	if sessionKey("owner-a", "one") == sessionKey("owner-a", "two") {
		t.Fatal("launches collided")
	}
	if sessionKey("owner-a", "one") == sessionKey("owner-b", "one") {
		t.Fatal("owner isolation failed")
	}
}
