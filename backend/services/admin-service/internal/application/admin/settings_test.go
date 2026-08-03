package admin

import (
	"strings"
	"testing"

	domain "admin/internal/domain/admin"

	"golang.org/x/crypto/bcrypt"
)

type fakeSettingCipher struct{}

func (fakeSettingCipher) Encrypt(value string) (string, error) {
	if strings.HasPrefix(value, "enc:") {
		return value, nil
	}
	return "enc:" + value, nil
}

func (fakeSettingCipher) Decrypt(value string) (string, error) {
	return strings.TrimPrefix(value, "enc:"), nil
}

func (fakeSettingCipher) IsEncrypted(value string) bool {
	return strings.HasPrefix(value, "enc:")
}

func TestProtectSensitiveSettingHashesWebmasterPassword(t *testing.T) {
	command, err := protectSensitiveSetting(domain.UpsertSettingCommand{
		Key:   "site.webmaster.password",
		Value: "webmaster123",
	}, fakeSettingCipher{})
	if err != nil {
		t.Fatalf("protect setting: %v", err)
	}
	if command.Value == "webmaster123" || !isBcryptHash(command.Value) {
		t.Fatalf("expected bcrypt hash, got %q", command.Value)
	}
	if command.ValueType != "password" {
		t.Fatalf("expected password value type, got %q", command.ValueType)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(command.Value), []byte("webmaster123")); err != nil {
		t.Fatalf("hash does not match password: %v", err)
	}
}

func TestAuthSettingWhitelistIncludesEmailVerificationRequirement(t *testing.T) {
	if !isAuthSettingKeyAllowed("auth.email_verification.required", false) {
		t.Fatalf("expected auth email verification requirement setting to be exposed")
	}
}

func TestAuthSettingWhitelistIncludesRegistrationMode(t *testing.T) {
	if !isAuthSettingKeyAllowed("auth.register.mode", false) {
		t.Fatalf("expected auth registration mode setting to be exposed")
	}
}

func TestFilterPublicSettingsOnlyReturnsWhitelistedNonSensitiveSettings(t *testing.T) {
	result := filterPublicSettings(domain.SettingList{Items: []domain.Setting{
		{Key: "site_name", Value: "BBS 社区"},
		{Key: "site_logo_url", Value: "https://cdn.example.com/logo.png"},
		{Key: "site_navigation", Value: `[{"key":"plaza","label":"发现"}]`},
		{Key: "site_announcements", Value: `[{"id":"maintenance","title":"维护公告","text":"今晚维护"}]`},
		{Key: "seo_keywords", Value: "bbs, community"},
		{Key: "auth.github.client_secret", Value: "github-secret", ValueType: "password"},
		{Key: "site.webmaster.password", Value: "webmaster-secret", ValueType: "password"},
		{Key: "email_from_name", Value: "Internal sender"},
	}})

	if result.Total != 5 || len(result.Items) != 5 {
		t.Fatalf("expected exactly the five public settings, got %+v", result.Items)
	}
	for _, item := range result.Items {
		if !isPublicSettingKeyAllowed(item.Key) {
			t.Fatalf("unexpected public setting %q", item.Key)
		}
	}
	if isPublicSettingKeyAllowed("auth.github.client_secret") || isPublicSettingKeyAllowed("site.webmaster.password") {
		t.Fatal("sensitive settings must never be public")
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
	}, fakeSettingCipher{})
	if err != nil {
		t.Fatalf("protect setting: %v", err)
	}
	if command.Value != string(hash) {
		t.Fatalf("expected existing hash to be preserved")
	}
}

func TestProtectSensitiveSettingPreservesEmptyPasswordValue(t *testing.T) {
	command, err := protectSensitiveSetting(domain.UpsertSettingCommand{
		Key:       "auth.github.client_secret",
		Value:     "",
		ValueType: "password",
	}, fakeSettingCipher{})
	if err != nil {
		t.Fatalf("protect setting: %v", err)
	}
	if !command.PreserveValue {
		t.Fatalf("expected empty password update to preserve existing value")
	}
}

func TestProtectSensitiveSettingPreservesMaskedPasswordValue(t *testing.T) {
	command, err := protectSensitiveSetting(domain.UpsertSettingCommand{
		Key:       "auth.google.client_secret",
		Value:     maskedSettingValue,
		ValueType: "password",
	}, fakeSettingCipher{})
	if err != nil {
		t.Fatalf("protect setting: %v", err)
	}
	if !command.PreserveValue {
		t.Fatalf("expected masked password update to preserve existing value")
	}
}

func TestProtectSensitiveSettingClearsPasswordValue(t *testing.T) {
	command, err := protectSensitiveSetting(domain.UpsertSettingCommand{
		Key:        "auth.github.client_secret",
		Value:      maskedSettingValue,
		ValueType:  "password",
		ClearValue: true,
	}, fakeSettingCipher{})
	if err != nil {
		t.Fatalf("protect setting: %v", err)
	}
	if command.PreserveValue {
		t.Fatalf("expected explicit clear to update existing value")
	}
	if command.Value != "" {
		t.Fatalf("expected explicit clear to store empty value, got %q", command.Value)
	}
	if command.ValueType != "password" {
		t.Fatalf("expected password value type, got %q", command.ValueType)
	}
}

func TestMaskSensitiveSettingsHidesPasswordValues(t *testing.T) {
	result := maskSensitiveSettings(domain.SettingList{Items: []domain.Setting{
		{Key: "site_name", Value: "BBS", ValueType: "string"},
		{Key: "auth.github.client_secret", Value: "github-secret", ValueType: "password"},
		{Key: "site.webmaster.password", Value: "$2a$10$012345678901234567890123456789012345678901234567890123", ValueType: "password"},
	}})
	if result.Items[0].Value != "BBS" {
		t.Fatalf("expected normal setting value to remain visible")
	}
	if result.Items[1].Value != maskedSettingValue {
		t.Fatalf("expected oauth secret to be masked, got %q", result.Items[1].Value)
	}
	if result.Items[2].Value != maskedSettingValue {
		t.Fatalf("expected webmaster password to be masked, got %q", result.Items[2].Value)
	}
}

func TestProtectSensitiveSettingEncryptsOAuthSecret(t *testing.T) {
	command, err := protectSensitiveSetting(domain.UpsertSettingCommand{
		Key:   "auth.github.client_secret",
		Value: "github-secret",
	}, fakeSettingCipher{})
	if err != nil {
		t.Fatalf("protect setting: %v", err)
	}
	if command.Value != "enc:github-secret" {
		t.Fatalf("expected encrypted oauth secret, got %q", command.Value)
	}
	if command.ValueType != "password" {
		t.Fatalf("expected password value type, got %q", command.ValueType)
	}
}

func TestDecryptSensitiveSettingReturnsPlainSecret(t *testing.T) {
	setting, err := decryptSensitiveSetting(domain.Setting{
		Key:       "auth.github.client_secret",
		Value:     "enc:github-secret",
		ValueType: "password",
	}, fakeSettingCipher{})
	if err != nil {
		t.Fatalf("decrypt setting: %v", err)
	}
	if setting.Value != "github-secret" {
		t.Fatalf("expected decrypted secret, got %q", setting.Value)
	}
}
