package mfa

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/hotp"
)

func TestManagerEncryptsAndDecryptsSecrets(t *testing.T) {
	manager, err := New("test-mfa-encryption-key-that-is-long-enough", "Test Community")
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	first, err := manager.EncryptSecret("JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatalf("encrypt secret: %v", err)
	}
	second, err := manager.EncryptSecret("JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatalf("encrypt secret again: %v", err)
	}
	if first == second {
		t.Fatal("ciphertexts match; want a fresh nonce per encryption")
	}
	if strings.Contains(first, "JBSWY3DPEHPK3PXP") {
		t.Fatal("ciphertext contains plaintext secret")
	}
	plaintext, err := manager.DecryptSecret(first)
	if err != nil {
		t.Fatalf("decrypt secret: %v", err)
	}
	if plaintext != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("decrypted secret = %q", plaintext)
	}

	other, err := New("different-test-mfa-encryption-key-value", "Test Community")
	if err != nil {
		t.Fatalf("new second manager: %v", err)
	}
	if _, err := other.DecryptSecret(first); err == nil {
		t.Fatal("decrypt with another key succeeded")
	}
}

func TestManagerGeneratesAndVerifiesTOTPEnrollment(t *testing.T) {
	manager, err := New("test-mfa-encryption-key-that-is-long-enough", "Test Community")
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	enrollment, err := manager.NewTOTP("alice")
	if err != nil {
		t.Fatalf("new TOTP enrollment: %v", err)
	}
	if enrollment.Secret == "" || !strings.HasPrefix(enrollment.URL, "otpauth://totp/") {
		t.Fatalf("invalid enrollment: %+v", enrollment)
	}
	if !strings.HasPrefix(enrollment.QRDataURL, "data:image/png;base64,") {
		t.Fatalf("QR data URL = %q", enrollment.QRDataURL)
	}
	if enrollment.Issuer != "Test Community" || enrollment.Account != "alice" {
		t.Fatalf("enrollment identity = %q/%q", enrollment.Issuer, enrollment.Account)
	}

	at := time.Unix(1_800_000_000, 0).UTC()
	step := at.Unix() / totpPeriodSeconds
	code, err := hotp.GenerateCodeCustom(enrollment.Secret, uint64(step), hotp.ValidateOpts{
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatalf("generate test TOTP: %v", err)
	}
	gotStep, valid := manager.VerifyTOTP(enrollment.Secret, code, at)
	if !valid || gotStep != step {
		t.Fatalf("VerifyTOTP = (%d, %v), want (%d, true)", gotStep, valid, step)
	}
	if _, valid := manager.VerifyTOTP(enrollment.Secret, code, at.Add(2*time.Minute)); valid {
		t.Fatal("code outside the configured skew window was accepted")
	}
	if _, valid := manager.VerifyTOTP(enrollment.Secret, "12345x", at); valid {
		t.Fatal("non-numeric TOTP was accepted")
	}
}

func TestManagerRecoveryCodesAndChallengesAreHashed(t *testing.T) {
	manager, err := New("test-mfa-encryption-key-that-is-long-enough", "Test Community")
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	codes, hashes, err := manager.NewRecoveryCodes(10)
	if err != nil {
		t.Fatalf("new recovery codes: %v", err)
	}
	if len(codes) != 10 || len(hashes) != 10 {
		t.Fatalf("recovery code counts = %d/%d", len(codes), len(hashes))
	}
	seen := map[string]struct{}{}
	for i, code := range codes {
		if code == hashes[i] || len(hashes[i]) != 64 {
			t.Fatalf("recovery code %d was not SHA-256 hashed", i)
		}
		if manager.HashRecoveryCode(strings.ToLower(strings.ReplaceAll(code, "-", " "))) != hashes[i] {
			t.Fatalf("recovery code %d normalization changed its hash", i)
		}
		if _, exists := seen[hashes[i]]; exists {
			t.Fatalf("duplicate recovery code hash at %d", i)
		}
		seen[hashes[i]] = struct{}{}
	}

	challenge, hash, err := manager.NewChallenge()
	if err != nil {
		t.Fatalf("new challenge: %v", err)
	}
	if challenge == "" || hash == challenge || len(hash) != 64 {
		t.Fatalf("challenge/hash are not distinct secure values: %q/%q", challenge, hash)
	}
	if manager.HashChallenge(challenge) != hash {
		t.Fatal("challenge hash is not reproducible")
	}
}
