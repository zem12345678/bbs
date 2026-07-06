package main

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
  admin: file-admin:9114
  user: file-user:9102
  content: file-content:9103
  comment: file-comment:9104
  reaction: file-reaction:9105
  search: file-search:9106
  feed: file-feed:9113
  credit: file-credit:9107
  notification: file-notification:9108
`)
	t.Setenv("BBS_GATEWAY_SERVICE_HTTP_PORT", "18080")
	t.Setenv("BBS_GATEWAY_AUTH_JWT_SECRET", "env-jwt")
	t.Setenv("BBS_GATEWAY_LOG_LEVEL", "debug")
	t.Setenv("BBS_GATEWAY_LOG_STDOUT", "true")
	t.Setenv("BBS_GATEWAY_TRACE_ENV", "prod")
	t.Setenv("BBS_GATEWAY_CORS_ALLOWED_ORIGINS", "https://bbs.example.com, https://admin.example.com")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_ADMIN", "env-admin:9114")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_NOTIFICATION", "env-notification:9108")

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
	if cfg.Auth.JWTSecret != "env-jwt" {
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
	if cfg.Upstreams.Admin != "env-admin:9114" {
		t.Fatalf("admin upstream = %q", cfg.Upstreams.Admin)
	}
	if cfg.Upstreams.Notification != "env-notification:9108" {
		t.Fatalf("notification upstream = %q", cfg.Upstreams.Notification)
	}
	if cfg.Upstreams.User != "file-user:9102" {
		t.Fatalf("user upstream = %q", cfg.Upstreams.User)
	}
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	path := writeGatewayConfigFile(t, `{}`)

	v, cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Service.Name != "api-gateway" {
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
	if cfg.Upstreams.Admin == "" || cfg.Upstreams.User == "" || cfg.Upstreams.Notification == "" {
		t.Fatalf("expected default upstreams, got %#v", cfg.Upstreams)
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
