package auth

import "testing"

func TestSecretCipherEncryptDecrypt(t *testing.T) {
	cipher, err := NewSecretCipher("local-test-secret")
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	encrypted, err := cipher.Encrypt("github-secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if encrypted == "github-secret" || !cipher.IsEncrypted(encrypted) {
		t.Fatalf("expected encrypted value with prefix, got %q", encrypted)
	}
	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != "github-secret" {
		t.Fatalf("expected decrypted secret, got %q", decrypted)
	}
}

func TestSecretCipherKeepsPlainValuesReadable(t *testing.T) {
	cipher, err := NewSecretCipher("local-test-secret")
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	decrypted, err := cipher.Decrypt("legacy-secret")
	if err != nil {
		t.Fatalf("decrypt legacy: %v", err)
	}
	if decrypted != "legacy-secret" {
		t.Fatalf("expected legacy plaintext to pass through, got %q", decrypted)
	}
}

func TestNewSecretCipherRequiresKey(t *testing.T) {
	if _, err := NewSecretCipher(" "); err == nil {
		t.Fatalf("expected empty key to fail")
	}
}
