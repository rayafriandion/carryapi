package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	c, err := New(key)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	plain := []byte("upstream-secret-api-key")
	blob, err := c.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got, err := c.Decrypt(blob)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("round trip mismatch: got %q, want %q", got, plain)
	}
}

func TestEncryptProducesDifferentCiphertext(t *testing.T) {
	c, _ := New(bytes.Repeat([]byte{1}, 32))
	plain := []byte("same-input")
	a, _ := c.Encrypt(plain)
	b, _ := c.Encrypt(plain)
	if bytes.Equal(a, b) {
		t.Error("two encryptions of same plaintext produced identical ciphertext (nonce not random)")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	c1, _ := New(bytes.Repeat([]byte{1}, 32))
	c2, _ := New(bytes.Repeat([]byte{2}, 32))
	blob, _ := c1.Encrypt([]byte("secret"))
	if _, err := c2.Decrypt(blob); err == nil {
		t.Error("expected error decrypting with wrong key, got nil")
	}
}

func TestNewInvalidKey(t *testing.T) {
	if _, err := New(bytes.Repeat([]byte{1}, 16)); err == nil {
		t.Error("expected error for 16-byte key, got nil")
	}
}
