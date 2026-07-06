package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

const encryptedSecretPrefix = "enc:v1:"

type SecretCipher struct {
	aead cipher.AEAD
}

func NewSecretCipher(secret string) (*SecretCipher, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, errors.New("secret encryption key is required")
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &SecretCipher{aead: aead}, nil
}

func (c *SecretCipher) Encrypt(plaintext string) (string, error) {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" || c == nil || c.aead == nil || c.IsEncrypted(plaintext) {
		return plaintext, nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return encryptedSecretPrefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (c *SecretCipher) Decrypt(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || c == nil || c.aead == nil || !c.IsEncrypted(value) {
		return value, nil
	}
	raw := strings.TrimPrefix(value, encryptedSecretPrefix)
	sealed, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", err
	}
	nonceSize := c.aead.NonceSize()
	if len(sealed) <= nonceSize {
		return "", errors.New("encrypted secret payload is too short")
	}
	nonce := sealed[:nonceSize]
	ciphertext := sealed[nonceSize:]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func (c *SecretCipher) IsEncrypted(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), encryptedSecretPrefix)
}
