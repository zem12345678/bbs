package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestSetDefaultsFillsAdminAuthAndUpstreams(t *testing.T) {
	v := viper.New()
	configureEnv(v)

	setDefaults(v)

	assertString(t, v, "auth.jwtSecret", "bbs-admin-local-dev-secret")
	assertString(t, v, "auth.jwtTtl", "168h")
	assertString(t, v, "auth.defaultAdminPassword", "Admin123!")
	assertString(t, v, "auth.secretEncryptionKey", "bbs-admin-local-setting-secret")
	assertString(t, v, "upstreams.user", "bbs-user-service")
	assertString(t, v, "upstreams.reaction", "bbs-reaction-service")
	assertString(t, v, "upstreams.content", "bbs-content-service")
	assertString(t, v, "upstreams.comment", "bbs-comment-service")
}

func TestConfigureEnvBindsAdminAuthAndUpstreams(t *testing.T) {
	t.Setenv("BBS_ADMIN_AUTH_JWT_SECRET", "env-jwt-secret")
	t.Setenv("BBS_ADMIN_AUTH_JWT_TTL", "24h")
	t.Setenv("BBS_ADMIN_AUTH_DEFAULT_ADMIN_PASSWORD", "EnvAdmin123!")
	t.Setenv("BBS_ADMIN_AUTH_SECRET_ENCRYPTION_KEY", "env-setting-secret")
	t.Setenv("BBS_ADMIN_UPSTREAMS_USER", "file-user-service")
	t.Setenv("BBS_ADMIN_UPSTREAMS_REACTION", "file-reaction-service")
	t.Setenv("BBS_ADMIN_UPSTREAMS_CONTENT", "file-content-service")
	t.Setenv("BBS_ADMIN_UPSTREAMS_COMMENT", "file-comment-service")

	v := viper.New()
	configureEnv(v)
	setDefaults(v)

	assertString(t, v, "auth.jwtSecret", "env-jwt-secret")
	assertString(t, v, "auth.jwtTtl", "24h")
	assertString(t, v, "auth.defaultAdminPassword", "EnvAdmin123!")
	assertString(t, v, "auth.secretEncryptionKey", "env-setting-secret")
	assertString(t, v, "upstreams.user", "file-user-service")
	assertString(t, v, "upstreams.reaction", "file-reaction-service")
	assertString(t, v, "upstreams.content", "file-content-service")
	assertString(t, v, "upstreams.comment", "file-comment-service")
}

func assertString(t *testing.T, v *viper.Viper, key string, want string) {
	t.Helper()
	if got := v.GetString(key); got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}
