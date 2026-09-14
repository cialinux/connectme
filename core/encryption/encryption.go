package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

type Cipher struct{ aead cipher.AEAD }

func New(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, errors.New("AES-256 requer chave de 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead}, nil
}
func (c *Cipher) Encrypt(plain, aad []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return append(nonce, c.aead.Seal(nil, nonce, plain, aad)...), nil
}
func (c *Cipher) Decrypt(value, aad []byte) ([]byte, error) {
	n := c.aead.NonceSize()
	if len(value) < n {
		return nil, errors.New("ciphertext inválido")
	}
	out, err := c.aead.Open(nil, value[:n], value[n:], aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return out, nil
}
