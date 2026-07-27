package app

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestProvideUpstreamsSuppliesLocalInternalAuthDefaults(t *testing.T) {
	_, err := ProvideUpstreams(nil, viper.New())
	if err == nil || !strings.Contains(err.Error(), "user grpc client required") {
		t.Fatalf("ProvideUpstreams() error = %v, want client validation after token defaults", err)
	}
}

func TestServiceNameDefaultNormalizesLegacyNames(t *testing.T) {
	tests := map[string]string{
		"user-service":         "bbs-user-service",
		"reaction-service":     "bbs-reaction-service",
		"content-service":      "bbs-content-service",
		"comment-service":      "bbs-comment-service",
		"notification-service": "bbs-notification-service",
		"search-service":       "bbs-search-service",
	}

	for input, want := range tests {
		if got := ServiceNameDefault(input, "fallback"); got != want {
			t.Fatalf("ServiceNameDefault(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSensitiveProvidersRejectMissingSecrets(t *testing.T) {
	v := viper.New()
	if _, err := ProvideTokenManager(v); err == nil {
		t.Fatal("ProvideTokenManager() error = nil, want missing jwt secret error")
	}
	if _, err := ProvideSecretCipher(v); err == nil {
		t.Fatal("ProvideSecretCipher() error = nil, want missing encryption key error")
	}
}

func TestProvideTokenManagerRequiresRefreshTTLAfterAccessTTL(t *testing.T) {
	v := viper.New()
	v.Set("auth.jwtSecret", "test-secret")
	v.Set("auth.jwtTtl", "1h")
	v.Set("auth.refreshTtl", "1h")
	if _, err := ProvideTokenManager(v); err == nil {
		t.Fatal("ProvideTokenManager() error = nil, want invalid refresh TTL error")
	}

	v.Set("auth.refreshTtl", "24h")
	if _, err := ProvideTokenManager(v); err != nil {
		t.Fatalf("ProvideTokenManager() error = %v", err)
	}
}
