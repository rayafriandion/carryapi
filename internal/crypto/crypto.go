package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

type Cipher struct {
	gcm cipher.AEAD
}

func New(masterKey []byte) (*Cipher, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("master key must be 32 bytes (AES-256)")
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("aes new: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm new: %w", err)
	}
	return &Cipher{gcm: gcm}, nil
}

func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	blob := c.gcm.Seal(nonce, nonce, plaintext, nil)
	return blob, nil
}

func (c *Cipher) Decrypt(blob []byte) ([]byte, error) {
	ns := c.gcm.NonceSize()
	if len(blob) < ns {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ciphertext := blob[:ns], blob[ns:]
	plain, err := c.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open: %w", err)
	}
	return plain, nil
}
