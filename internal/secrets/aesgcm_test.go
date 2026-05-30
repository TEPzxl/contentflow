package secrets

import (
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestNewAESGCMFromEncodedKeyAcceptsBase64AndEncryptsRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	encoded := base64.StdEncoding.EncodeToString(key)

	box, err := NewAESGCMFromEncodedKey(encoded)
	if err != nil {
		t.Fatalf("NewAESGCMFromEncodedKey() error = %v", err)
	}

	ciphertext, nonce, err := box.EncryptString("sk-test")
	if err != nil {
		t.Fatalf("EncryptString() error = %v", err)
	}
	if string(ciphertext) == "sk-test" {
		t.Fatal("ciphertext stores plaintext")
	}

	plaintext, err := box.DecryptString(ciphertext, nonce)
	if err != nil {
		t.Fatalf("DecryptString() error = %v", err)
	}
	if plaintext != "sk-test" {
		t.Fatalf("plaintext = %q, want sk-test", plaintext)
	}
}

func TestNewAESGCMFromEncodedKeyAcceptsHex(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(255 - i)
	}

	box, err := NewAESGCMFromEncodedKey(hex.EncodeToString(key))
	if err != nil {
		t.Fatalf("NewAESGCMFromEncodedKey() error = %v", err)
	}
	if box == nil {
		t.Fatal("box is nil")
	}
}

func TestNewAESGCMFromEncodedKeyRejectsInvalidLength(t *testing.T) {
	_, err := NewAESGCMFromEncodedKey(base64.StdEncoding.EncodeToString([]byte("short")))
	if err == nil {
		t.Fatal("NewAESGCMFromEncodedKey() error is nil")
	}
}
