package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

type AESGCM struct {
	aead cipher.AEAD
}

func NewAESGCMFromEncodedKey(encoded string) (*AESGCM, error) {
	key, err := decodeKey(strings.TrimSpace(encoded))
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must decode to 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm cipher: %w", err)
	}
	return &AESGCM{aead: aead}, nil
}

func (b *AESGCM) EncryptString(plaintext string) ([]byte, []byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := b.aead.Seal(nil, nonce, []byte(plaintext), nil)
	return ciphertext, nonce, nil
}

func (b *AESGCM) DecryptString(ciphertext, nonce []byte) (string, error) {
	if len(nonce) != b.aead.NonceSize() {
		return "", fmt.Errorf("invalid nonce size %d", len(nonce))
	}
	plaintext, err := b.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plaintext), nil
}

func decodeKey(encoded string) ([]byte, error) {
	if encoded == "" {
		return nil, errors.New("encryption key is required")
	}
	if len(encoded) == 64 {
		if key, err := hex.DecodeString(encoded); err == nil {
			return key, nil
		}
	}
	if key, err := base64.StdEncoding.DecodeString(encoded); err == nil {
		return key, nil
	}
	if key, err := base64.RawStdEncoding.DecodeString(encoded); err == nil {
		return key, nil
	}
	if key, err := base64.RawURLEncoding.DecodeString(encoded); err == nil {
		return key, nil
	}
	return nil, errors.New("encryption key must be base64 or hex encoded")
}
