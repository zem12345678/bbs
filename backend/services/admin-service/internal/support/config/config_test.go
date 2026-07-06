package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewAppliesEnvironmentOverrides(t *testing.T) {
	path := writeConfigFile(t, `
service:
  name: file-admin-service
  grpcPort: 9114
postgres:
  dsn: file-postgres-dsn
  debug: false
auth:
  jwtSecret: file-jwt
  jwtTtl: 168h
  defaultAdminPassword: FilePass123!
  secretEncryptionKey: file-secret-key
upstreams:
  user: file-user:9102
  reaction: file-reaction:9105
  content: file-content:9103
  comment: file-comment:9104
`)
	t.Setenv("BBS_ADMIN_SERVICE_GRPC_PORT", "19114")
	t.Setenv("BBS_ADMIN_POSTGRES_DSN", "env-postgres-dsn")
	t.Setenv("BBS_ADMIN_POSTGRES_DEBUG", "true")
	t.Setenv("BBS_ADMIN_AUTH_JWT_SECRET", "env-jwt")
	t.Setenv("BBS_ADMIN_AUTH_JWT_TTL", "24h")
	t.Setenv("BBS_ADMIN_AUTH_DEFAULT_ADMIN_PASSWORD", "EnvPass123!")
	t.Setenv("BBS_ADMIN_AUTH_SECRET_ENCRYPTION_KEY", "env-secret-key")
	t.Setenv("BBS_ADMIN_UPSTREAMS_USER", "env-user:9102")

	cfg, err := New(path)
	if err != nil {
		t.Fatalf("new config: %v", err)
	}
	if cfg.Service.GRPCPort != 19114 {
		t.Fatalf("grpc port = %d, want 19114", cfg.Service.GRPCPort)
	}
	if cfg.Postgres.DSN != "env-postgres-dsn" {
		t.Fatalf("postgres dsn = %q", cfg.Postgres.DSN)
	}
	if !cfg.Postgres.Debug {
		t.Fatalf("postgres debug should be true")
	}
	if cfg.Auth.JWTSecret != "env-jwt" {
		t.Fatalf("jwt secret = %q", cfg.Auth.JWTSecret)
	}
	if cfg.Auth.JWTTTL != "24h" {
		t.Fatalf("jwt ttl = %q", cfg.Auth.JWTTTL)
	}
	if cfg.Auth.DefaultAdminPassword != "EnvPass123!" {
		t.Fatalf("default admin password = %q", cfg.Auth.DefaultAdminPassword)
	}
	if cfg.Auth.SecretEncryptionKey != "env-secret-key" {
		t.Fatalf("secret encryption key = %q", cfg.Auth.SecretEncryptionKey)
	}
	if cfg.Upstreams.User != "env-user:9102" {
		t.Fatalf("upstream user = %q", cfg.Upstreams.User)
	}
	if cfg.Upstreams.Content != "file-content:9103" {
		t.Fatalf("upstream content = %q", cfg.Upstreams.Content)
	}
}

func TestNewDefaultsSecretEncryptionKeyToJWTSecret(t *testing.T) {
	path := writeConfigFile(t, `
auth:
  jwtSecret: file-jwt
`)
	t.Setenv("BBS_ADMIN_AUTH_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("BBS_ADMIN_AUTH_JWT_SECRET", "")

	cfg, err := New(path)
	if err != nil {
		t.Fatalf("new config: %v", err)
	}
	if cfg.Auth.SecretEncryptionKey != "file-jwt" {
		t.Fatalf("secret encryption key = %q, want jwt secret fallback", cfg.Auth.SecretEncryptionKey)
	}
	if cfg.Service.Name != "admin-service" {
		t.Fatalf("service name default = %q", cfg.Service.Name)
	}
	if cfg.Upstreams.User == "" || cfg.Upstreams.Content == "" || cfg.Upstreams.Comment == "" {
		t.Fatalf("expected default upstreams, got %#v", cfg.Upstreams)
	}
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
