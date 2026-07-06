package admin

import (
	"testing"

	domain "admin/internal/domain/admin"

	"golang.org/x/crypto/bcrypt"
)

func TestProtectSensitiveSettingHashesWebmasterPassword(t *testing.T) {
	command, err := protectSensitiveSetting(domain.UpsertSettingCommand{
		Key:       "site.webmaster.password",
		Value:     "webmaster123",
		ValueType: "password",
	})
	if err != nil {
		t.Fatalf("protect setting: %v", err)
	}
	if command.Value == "webmaster123" || !isBcryptHash(command.Value) {
		t.Fatalf("expected bcrypt hash, got %q", command.Value)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(command.Value), []byte("webmaster123")); err != nil {
		t.Fatalf("hash does not match password: %v", err)
	}
}

func TestProtectSensitiveSettingKeepsExistingHash(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("webmaster123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	command, err := protectSensitiveSetting(domain.UpsertSettingCommand{
		Key:       "site.webmaster.password",
		Value:     string(hash),
		ValueType: "password",
	})
	if err != nil {
		t.Fatalf("protect setting: %v", err)
	}
	if command.Value != string(hash) {
		t.Fatalf("expected existing hash to be preserved")
	}
}
