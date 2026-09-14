package encryption

import (
	"bytes"
	"testing"
)

func TestRoundTripAndAAD(t *testing.T) {
	c, _ := New(bytes.Repeat([]byte{1}, 32))
	v, err := c.Encrypt([]byte("secret"), []byte("user:1"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := c.Decrypt(v, []byte("user:1"))
	if err != nil || string(plain) != "secret" {
		t.Fatalf("plain=%q err=%v", plain, err)
	}
	if _, err = c.Decrypt(v, []byte("user:2")); err == nil {
		t.Fatal("AAD incorreto deveria falhar")
	}
}
