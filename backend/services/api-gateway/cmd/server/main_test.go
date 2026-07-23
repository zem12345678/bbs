package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigAppliesEnvironmentOverrides(t *testing.T) {
	path := writeGatewayConfigFile(t, `
service:
  name: file-gateway
  httpPort: 8080
log:
  filename: logs/file.log
  level: info
  stdout: false
trace:
  grpcEndpoint: 127.0.0.1:4317
  serviceName: file-gateway
  version: file
  env: file
auth:
  tokenHeader: Authorization
  tokenPrefix: Bearer
  jwtSecret: file-jwt
cors:
  allowedOrigins:
    - http://file.local
upstreams:
  admin: file-admin-service
  user: file-user-service
  content: file-content-service
  comment: file-comment-service
  reaction: file-reaction-service
  search: file-search-service
  feed: file-feed-service
  credit: file-credit-service
  notification: file-notification-service
  chat: file-chat-service
`)
	t.Setenv("BBS_GATEWAY_SERVICE_HTTP_PORT", "18080")
	t.Setenv("BBS_GATEWAY_AUTH_JWT_SECRET", "env-jwt-secret-with-at-least-32-chars")
	t.Setenv("BBS_GATEWAY_LOG_LEVEL", "debug")
	t.Setenv("BBS_GATEWAY_LOG_STDOUT", "true")
	t.Setenv("BBS_GATEWAY_TRACE_ENV", "prod")
	t.Setenv("BBS_GATEWAY_CORS_ALLOWED_ORIGINS", "https://bbs.example.com, https://admin.example.com")
	t.Setenv("BBS_GATEWAY_GRPC_CLIENT_ETCD_ADDR", "etcd-a:2379, etcd-b:2379")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_ADMIN", "env-admin-service")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_NOTIFICATION", "env-notification-service")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_CHAT", "env-chat-service")

	v, cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Service.HTTPPort != 18080 {
		t.Fatalf("http port = %d, want 18080", cfg.Service.HTTPPort)
	}
	if v.GetInt("service.httpPort") != 18080 {
		t.Fatalf("viper http port = %d, want 18080", v.GetInt("service.httpPort"))
	}
	if cfg.Auth.JWTSecret != "env-jwt-secret-with-at-least-32-chars" {
		t.Fatalf("jwt secret = %q", cfg.Auth.JWTSecret)
	}
	if v.GetString("log.level") != "debug" {
		t.Fatalf("log level = %q", v.GetString("log.level"))
	}
	if !v.GetBool("log.stdout") {
		t.Fatalf("log stdout should be true")
	}
	if v.GetString("trace.env") != "prod" {
		t.Fatalf("trace env = %q", v.GetString("trace.env"))
	}
	origins := v.GetStringSlice("cors.allowedOrigins")
	if len(origins) != 2 || origins[0] != "https://bbs.example.com" || origins[1] != "https://admin.example.com" {
		t.Fatalf("cors origins = %#v", origins)
	}
	etcdEndpoints := v.GetStringSlice("grpc.client.etcdAddr")
	if len(etcdEndpoints) != 2 || etcdEndpoints[0] != "etcd-a:2379" || etcdEndpoints[1] != "etcd-b:2379" {
		t.Fatalf("grpc client etcd endpoints = %#v", etcdEndpoints)
	}
	if cfg.Upstreams.Admin != "env-admin-service" {
		t.Fatalf("admin upstream = %q", cfg.Upstreams.Admin)
	}
	if cfg.Upstreams.Notification != "env-notification-service" {
		t.Fatalf("notification upstream = %q", cfg.Upstreams.Notification)
	}
	if cfg.Upstreams.Chat != "env-chat-service" {
		t.Fatalf("chat upstream = %q", cfg.Upstreams.Chat)
	}
	if cfg.Upstreams.User != "file-user-service" {
		t.Fatalf("user upstream = %q", cfg.Upstreams.User)
	}
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	path := writeGatewayConfigFile(t, `{}`)

	v, cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Service.Name != "bbs-api-gateway" {
		t.Fatalf("service name = %q", cfg.Service.Name)
	}
	if cfg.Service.HTTPPort != 8080 || v.GetInt("service.httpPort") != 8080 {
		t.Fatalf("http port cfg=%d viper=%d, want 8080", cfg.Service.HTTPPort, v.GetInt("service.httpPort"))
	}
	if cfg.Auth.TokenHeader != "Authorization" || cfg.Auth.TokenPrefix != "Bearer" {
		t.Fatalf("token header/prefix = %q/%q", cfg.Auth.TokenHeader, cfg.Auth.TokenPrefix)
	}
	if cfg.Auth.JWTSecret == "" {
		t.Fatalf("expected default jwt secret")
	}
	if cfg.Upstreams.Admin == "" || cfg.Upstreams.User == "" || cfg.Upstreams.Notification == "" || cfg.Upstreams.Chat == "" {
		t.Fatalf("expected default upstreams, got %#v", cfg.Upstreams)
	}
}

func TestLoadConfigRejectsDefaultJWTSecretInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, "trace:\n  env: production\n")

	_, _, err := loadConfig(path)
	if err == nil {
		t.Fatal("expected production config with default JWT secret to fail")
	}
}

func TestLoadConfigRejectsShortJWTSecretInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, `
trace:
  env: production
auth:
  jwtSecret: too-short
cors:
  allowedOrigins:
    - https://bbs.example.com
`)

	_, _, err := loadConfig(path)
	if err == nil {
		t.Fatal("expected production config with short JWT secret to fail")
	}
}

func TestLoadConfigRejectsMissingCORSOriginsInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, `
trace:
  env: production
auth:
  jwtSecret: production-jwt-secret-with-at-least-32-chars
`)

	_, _, err := loadConfig(path)
	if err == nil {
		t.Fatal("expected production config without CORS origins to fail")
	}
}

func TestLoadConfigRejectsWildcardCORSOriginInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, `
trace:
  env: production
auth:
  jwtSecret: production-jwt-secret-with-at-least-32-chars
cors:
  allowedOrigins:
    - "*"
`)

	_, _, err := loadConfig(path)
	if err == nil {
		t.Fatal("expected production config with wildcard CORS origin to fail")
	}
}

func TestLoadConfigRejectsLocalCORSOriginInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, `
trace:
  env: production
auth:
  jwtSecret: production-jwt-secret-with-at-least-32-chars
cors:
  allowedOrigins:
    - http://127.0.0.1:8850
`)

	_, _, err := loadConfig(path)
	if err == nil {
		t.Fatal("expected production config with local CORS origin to fail")
	}
}

func writeGatewayConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
