package mfa

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image/png"
	"strings"
	"time"

	domain "user-service/internal/domain/user"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/hotp"
	"github.com/pquerna/otp/totp"
)

const (
	totpPeriodSeconds = int64(30)
	totpSecretSize    = uint(20)
)

type Manager struct {
	aead   cipher.AEAD
	issuer string
}

func New(encryptionKey string, issuer string) (*Manager, error) {
	encryptionKey = strings.TrimSpace(encryptionKey)
	if encryptionKey == "" {
		return nil, fmt.Errorf("mfa encryption key required")
	}
	key := sha256.Sum256([]byte(encryptionKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create mfa cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create mfa gcm: %w", err)
	}
	issuer = strings.TrimSpace(issuer)
	if issuer == "" {
		issuer = "BBS Community"
	}
	return &Manager{aead: aead, issuer: issuer}, nil
}

func (m *Manager) NewTOTP(account string) (domain.TOTPEnrollment, error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return domain.TOTPEnrollment{}, fmt.Errorf("mfa account required")
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      m.issuer,
		AccountName: account,
		Period:      uint(totpPeriodSeconds),
		SecretSize:  totpSecretSize,
		Secret:      nil,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return domain.TOTPEnrollment{}, fmt.Errorf("generate totp secret: %w", err)
	}
	image, err := key.Image(220, 220)
	if err != nil {
		return domain.TOTPEnrollment{}, fmt.Errorf("render totp qr: %w", err)
	}
	var qr bytes.Buffer
	if err := png.Encode(&qr, image); err != nil {
		return domain.TOTPEnrollment{}, fmt.Errorf("encode totp qr: %w", err)
	}
	return domain.TOTPEnrollment{
		Secret:    key.Secret(),
		URL:       key.URL(),
		QRDataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(qr.Bytes()),
		Issuer:    m.issuer,
		Account:   account,
	}, nil
}

func (m *Manager) EncryptSecret(secret string) (string, error) {
	if m == nil || m.aead == nil {
		return "", fmt.Errorf("mfa cipher unavailable")
	}
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", fmt.Errorf("mfa secret required")
	}
	nonce := make([]byte, m.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate mfa nonce: %w", err)
	}
	ciphertext := m.aead.Seal(nil, nonce, []byte(secret), nil)
	value := append(nonce, ciphertext...)
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (m *Manager) DecryptSecret(value string) (string, error) {
	if m == nil || m.aead == nil {
		return "", fmt.Errorf("mfa cipher unavailable")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(raw) <= m.aead.NonceSize() {
		return "", fmt.Errorf("invalid mfa ciphertext")
	}
	plaintext, err := m.aead.Open(nil, raw[:m.aead.NonceSize()], raw[m.aead.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt mfa secret: %w", err)
	}
	return string(plaintext), nil
}

func (m *Manager) VerifyTOTP(secret string, code string, at time.Time) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return 0, false
	}
	for _, character := range code {
		if character < '0' || character > '9' {
			return 0, false
		}
	}
	current := at.Unix() / totpPeriodSeconds
	for offset := int64(-1); offset <= 1; offset++ {
		step := current + offset
		if step < 0 {
			continue
		}
		valid, err := hotp.ValidateCustom(code, uint64(step), strings.TrimSpace(secret), hotp.ValidateOpts{
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		})
		if err == nil && valid {
			return step, true
		}
	}
	return 0, false
}

func (m *Manager) NewRecoveryCodes(count int) ([]string, []string, error) {
	if count <= 0 {
		return nil, nil, fmt.Errorf("recovery code count must be positive")
	}
	codes := make([]string, 0, count)
	hashes := make([]string, 0, count)
	seen := make(map[string]struct{}, count)
	for len(codes) < count {
		raw := make([]byte, 16)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, fmt.Errorf("generate recovery code: %w", err)
		}
		encoded := strings.ToUpper(hex.EncodeToString(raw))
		code := strings.Join([]string{encoded[0:8], encoded[8:16], encoded[16:24], encoded[24:32]}, "-")
		hash := m.HashRecoveryCode(code)
		if _, duplicate := seen[hash]; duplicate {
			continue
		}
		seen[hash] = struct{}{}
		codes = append(codes, code)
		hashes = append(hashes, hash)
	}
	return codes, hashes, nil
}

func (m *Manager) NewChallenge() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate mfa challenge: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, m.HashChallenge(token), nil
}

func (m *Manager) HashChallenge(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func (m *Manager) HashRecoveryCode(code string) string {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	normalized = strings.NewReplacer("-", "", " ", "").Replace(normalized)
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
