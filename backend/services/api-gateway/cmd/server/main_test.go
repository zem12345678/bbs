package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"api-gateway/internal/clients"
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
	t.Setenv("BBS_GATEWAY_AUTH_RATE_LIMIT_REGISTER_INTERVAL", "2h")
	t.Setenv("BBS_GATEWAY_AUTH_RATE_LIMIT_REGISTER_RATE", "4")
	t.Setenv("BBS_GATEWAY_AUTH_RATE_LIMIT_LOGIN_INTERVAL", "2m")
	t.Setenv("BBS_GATEWAY_AUTH_RATE_LIMIT_LOGIN_RATE", "8")
	t.Setenv("BBS_GATEWAY_AUTH_RATE_LIMIT_PASSWORD_RESET_INTERVAL", "20m")
	t.Setenv("BBS_GATEWAY_AUTH_RATE_LIMIT_PASSWORD_RESET_RATE", "3")
	t.Setenv("BBS_GATEWAY_AUTH_RATE_LIMIT_PASSWORD_RESET_CONFIRM_INTERVAL", "10m")
	t.Setenv("BBS_GATEWAY_AUTH_RATE_LIMIT_PASSWORD_RESET_CONFIRM_RATE", "2")
	t.Setenv("BBS_GATEWAY_AUTH_RATE_LIMIT_EMAIL_VERIFICATION_INTERVAL", "30m")
	t.Setenv("BBS_GATEWAY_AUTH_RATE_LIMIT_EMAIL_VERIFICATION_RATE", "2")
	t.Setenv("BBS_GATEWAY_AUTH_RATE_LIMIT_ADMIN_LOGIN_INTERVAL", "45m")
	t.Setenv("BBS_GATEWAY_AUTH_RATE_LIMIT_ADMIN_LOGIN_RATE", "6")
	t.Setenv("BBS_GATEWAY_SEARCH_RATE_LIMIT_CONTENT_INTERVAL", "7m")
	t.Setenv("BBS_GATEWAY_SEARCH_RATE_LIMIT_CONTENT_RATE", "42")
	t.Setenv("BBS_GATEWAY_SEARCH_RATE_LIMIT_USER_INTERVAL", "3m")
	t.Setenv("BBS_GATEWAY_SEARCH_RATE_LIMIT_USER_RATE", "6")
	t.Setenv("BBS_GATEWAY_LOG_LEVEL", "debug")
	t.Setenv("BBS_GATEWAY_LOG_STDOUT", "true")
	t.Setenv("BBS_GATEWAY_TRACE_ENV", "prod")
	t.Setenv("BBS_GATEWAY_CORS_ALLOWED_ORIGINS", "https://bbs.example.com, https://admin.example.com")
	t.Setenv("BBS_GATEWAY_HTTP_HOST", "0.0.0.0")
	t.Setenv("BBS_GATEWAY_HTTP_PUBLIC_BASE_URL", "https://bbs.example.com/")
	t.Setenv("BBS_GATEWAY_HTTP_TRUSTED_PROXIES", "10.0.0.0/8, 127.0.0.1")
	t.Setenv("BBS_GATEWAY_GRPC_CLIENT_ETCD_ADDR", "etcd-a:2379, etcd-b:2379")
	t.Setenv("BBS_GATEWAY_GRPC_CLIENT_SECURE", "true")
	t.Setenv("BBS_GATEWAY_GRPC_CLIENT_TLS_CA_FILE", "/mnt/gateway-tls/ca.crt")
	t.Setenv("BBS_GATEWAY_GRPC_CLIENT_TLS_CERT_FILE", "/mnt/gateway-tls/tls.crt")
	t.Setenv("BBS_GATEWAY_GRPC_CLIENT_TLS_KEY_FILE", "/mnt/gateway-tls/tls.key")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_ADMIN", "env-admin-service")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_ADMIN_INTERNAL_AUTH_TOKEN", "env-admin-internal-token-with-at-least-32-bytes")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_ADMIN_INTERNAL_AUTH_SECURE", "true")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_NOTIFICATION", "env-notification-service")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_NOTIFICATION_INTERNAL_AUTH_TOKEN", "env-notification-internal-token-with-at-least-32-bytes")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_CHAT", "env-chat-service")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_CHAT_INTERNAL_AUTH_TOKEN", "env-chat-internal-token-with-at-least-32-bytes")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_CHAT_INTERNAL_AUTH_SECURE", "true")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_COMMENT_INTERNAL_AUTH_TOKEN", "env-comment-internal-token-with-at-least-32-bytes")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_CONTENT_INTERNAL_AUTH_TOKEN", "env-content-internal-token-with-at-least-32-bytes")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_USER_INTERNAL_AUTH_TOKEN", "env-user-internal-token-with-at-least-32-bytes")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_USER_INTERNAL_AUTH_SECURE", "false")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_MALL_INTERNAL_AUTH_TOKEN", "env-mall-internal-token-with-at-least-32-bytes")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_MALL_INTERNAL_AUTH_SECURE", "false")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_CREDIT_INTERNAL_AUTH_TOKEN", "env-credit-internal-token-with-at-least-32-bytes")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_CREDIT_INTERNAL_AUTH_SECURE", "false")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_FILE_INTERNAL_AUTH_TOKEN", "env-file-internal-token-with-at-least-32-bytes")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_FILE_INTERNAL_AUTH_SECURE", "false")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_FEED_INTERNAL_AUTH_TOKEN", "env-feed-internal-token-with-at-least-32-bytes")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_REACTION_INTERNAL_AUTH_TOKEN", "env-reaction-internal-token-with-at-least-32-bytes")
	t.Setenv("BBS_GATEWAY_UPSTREAMS_SEARCH_INTERNAL_AUTH_TOKEN", "env-search-internal-token-with-at-least-32-bytes")
	t.Setenv("BBS_GATEWAY_CHAT_RATE_LIMIT_CREATE_ROOM_INTERVAL", "8m")
	t.Setenv("BBS_GATEWAY_CHAT_RATE_LIMIT_CREATE_ROOM_RATE", "4")
	t.Setenv("BBS_GATEWAY_CHAT_RATE_LIMIT_TICKET_INTERVAL", "4m")
	t.Setenv("BBS_GATEWAY_CHAT_RATE_LIMIT_TICKET_RATE", "9")
	t.Setenv("BBS_GATEWAY_CHAT_RATE_LIMIT_SUBSCRIBE_INTERVAL", "5m")
	t.Setenv("BBS_GATEWAY_CHAT_RATE_LIMIT_SUBSCRIBE_RATE", "11")
	t.Setenv("BBS_GATEWAY_CHAT_WEBSOCKET_MAX_CONNECTIONS_PER_USER", "7")
	t.Setenv("BBS_GATEWAY_CHAT_WEBSOCKET_MAX_CONNECTIONS_PER_IP", "70")
	t.Setenv("BBS_GATEWAY_CHAT_RATE_LIMIT_JOIN_INTERVAL", "2m")
	t.Setenv("BBS_GATEWAY_CHAT_RATE_LIMIT_JOIN_RATE", "12")
	t.Setenv("BBS_GATEWAY_CHAT_RATE_LIMIT_SEND_INTERVAL", "3s")
	t.Setenv("BBS_GATEWAY_CHAT_RATE_LIMIT_SEND_RATE", "7")
	t.Setenv("BBS_GATEWAY_CHAT_RATE_LIMIT_READ_INTERVAL", "6m")
	t.Setenv("BBS_GATEWAY_CHAT_RATE_LIMIT_READ_RATE", "17")

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
	if cfg.Auth.RateLimit.RegisterInterval != 2*time.Hour || cfg.Auth.RateLimit.RegisterRate != 4 {
		t.Fatalf("register rate limit = %s/%d", cfg.Auth.RateLimit.RegisterInterval, cfg.Auth.RateLimit.RegisterRate)
	}
	if cfg.Auth.RateLimit.LoginInterval != 2*time.Minute || cfg.Auth.RateLimit.LoginRate != 8 {
		t.Fatalf("login rate limit = %s/%d", cfg.Auth.RateLimit.LoginInterval, cfg.Auth.RateLimit.LoginRate)
	}
	if cfg.Auth.RateLimit.PasswordResetInterval != 20*time.Minute || cfg.Auth.RateLimit.PasswordResetRate != 3 {
		t.Fatalf("password reset rate limit = %s/%d", cfg.Auth.RateLimit.PasswordResetInterval, cfg.Auth.RateLimit.PasswordResetRate)
	}
	if cfg.Auth.RateLimit.PasswordResetConfirmInterval != 10*time.Minute || cfg.Auth.RateLimit.PasswordResetConfirmRate != 2 {
		t.Fatalf("password reset confirm rate limit = %s/%d", cfg.Auth.RateLimit.PasswordResetConfirmInterval, cfg.Auth.RateLimit.PasswordResetConfirmRate)
	}
	if cfg.Auth.RateLimit.EmailVerificationInterval != 30*time.Minute || cfg.Auth.RateLimit.EmailVerificationRate != 2 {
		t.Fatalf("email verification rate limit = %s/%d", cfg.Auth.RateLimit.EmailVerificationInterval, cfg.Auth.RateLimit.EmailVerificationRate)
	}
	if cfg.Auth.RateLimit.AdminLoginInterval != 45*time.Minute || cfg.Auth.RateLimit.AdminLoginRate != 6 {
		t.Fatalf("admin login rate limit = %s/%d", cfg.Auth.RateLimit.AdminLoginInterval, cfg.Auth.RateLimit.AdminLoginRate)
	}
	if cfg.Search.RateLimit.ContentInterval != 7*time.Minute || cfg.Search.RateLimit.ContentRate != 42 {
		t.Fatalf("content search rate limit = %s/%d", cfg.Search.RateLimit.ContentInterval, cfg.Search.RateLimit.ContentRate)
	}
	if cfg.Search.RateLimit.UserInterval != 3*time.Minute || cfg.Search.RateLimit.UserRate != 6 {
		t.Fatalf("user search rate limit = %s/%d", cfg.Search.RateLimit.UserInterval, cfg.Search.RateLimit.UserRate)
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
	if v.GetString("http.host") != "0.0.0.0" {
		t.Fatalf("http host = %q", v.GetString("http.host"))
	}
	if cfg.PublicBaseURL != "https://bbs.example.com" || v.GetString("http.publicBaseURL") != "https://bbs.example.com" {
		t.Fatalf("public base URL cfg=%q viper=%q", cfg.PublicBaseURL, v.GetString("http.publicBaseURL"))
	}
	origins := v.GetStringSlice("cors.allowedOrigins")
	if len(origins) != 2 || origins[0] != "https://bbs.example.com" || origins[1] != "https://admin.example.com" {
		t.Fatalf("cors origins = %#v", origins)
	}
	trustedProxies := v.GetStringSlice("http.trustedProxies")
	if len(trustedProxies) != 2 || trustedProxies[0] != "10.0.0.0/8" || trustedProxies[1] != "127.0.0.1" {
		t.Fatalf("trusted proxies = %#v", trustedProxies)
	}
	etcdEndpoints := v.GetStringSlice("grpc.client.etcdAddr")
	if len(etcdEndpoints) != 2 || etcdEndpoints[0] != "etcd-a:2379" || etcdEndpoints[1] != "etcd-b:2379" {
		t.Fatalf("grpc client etcd endpoints = %#v", etcdEndpoints)
	}
	if cfg.Upstreams.Admin != "env-admin-service" {
		t.Fatalf("admin upstream = %q", cfg.Upstreams.Admin)
	}
	if cfg.Upstreams.AdminInternalAuthToken != "env-admin-internal-token-with-at-least-32-bytes" {
		t.Fatalf("admin internal auth token = %q", cfg.Upstreams.AdminInternalAuthToken)
	}
	if !cfg.Upstreams.AdminInternalAuthSecure {
		t.Fatal("admin internal auth transport should be secure")
	}
	if cfg.Upstreams.Notification != "env-notification-service" {
		t.Fatalf("notification upstream = %q", cfg.Upstreams.Notification)
	}
	if cfg.Upstreams.NotificationInternalAuthToken != "env-notification-internal-token-with-at-least-32-bytes" {
		t.Fatalf("notification internal auth token = %q", cfg.Upstreams.NotificationInternalAuthToken)
	}
	if cfg.Upstreams.Chat != "env-chat-service" {
		t.Fatalf("chat upstream = %q", cfg.Upstreams.Chat)
	}
	if cfg.Upstreams.ChatInternalAuthToken != "env-chat-internal-token-with-at-least-32-bytes" {
		t.Fatalf("chat internal auth token = %q", cfg.Upstreams.ChatInternalAuthToken)
	}
	if !cfg.Upstreams.ChatInternalAuthSecure {
		t.Fatal("chat internal auth transport should be secure")
	}
	if cfg.Upstreams.CommentInternalAuthToken != "env-comment-internal-token-with-at-least-32-bytes" {
		t.Fatalf("comment internal auth token = %q", cfg.Upstreams.CommentInternalAuthToken)
	}
	if cfg.Upstreams.ContentInternalAuthToken != "env-content-internal-token-with-at-least-32-bytes" {
		t.Fatalf("content internal auth token = %q", cfg.Upstreams.ContentInternalAuthToken)
	}
	if cfg.Upstreams.UserInternalAuthSecure || cfg.Upstreams.MallInternalAuthSecure || cfg.Upstreams.CreditInternalAuthSecure || cfg.Upstreams.FileInternalAuthSecure {
		t.Fatalf("plaintext upstream overrides were not preserved: %#v", cfg.Upstreams)
	}
	if cfg.Upstreams.UserInternalAuthToken != "env-user-internal-token-with-at-least-32-bytes" {
		t.Fatalf("user internal auth token = %q", cfg.Upstreams.UserInternalAuthToken)
	}
	if cfg.Upstreams.MallInternalAuthToken != "env-mall-internal-token-with-at-least-32-bytes" {
		t.Fatalf("mall internal auth token = %q", cfg.Upstreams.MallInternalAuthToken)
	}
	if cfg.Upstreams.CreditInternalAuthToken != "env-credit-internal-token-with-at-least-32-bytes" {
		t.Fatalf("credit internal auth token = %q", cfg.Upstreams.CreditInternalAuthToken)
	}
	if cfg.Upstreams.FileInternalAuthToken != "env-file-internal-token-with-at-least-32-bytes" {
		t.Fatalf("file internal auth token = %q", cfg.Upstreams.FileInternalAuthToken)
	}
	if cfg.Upstreams.FeedInternalAuthToken != "env-feed-internal-token-with-at-least-32-bytes" {
		t.Fatalf("feed internal auth token = %q", cfg.Upstreams.FeedInternalAuthToken)
	}
	if cfg.Upstreams.ReactionInternalAuthToken != "env-reaction-internal-token-with-at-least-32-bytes" {
		t.Fatalf("reaction internal auth token = %q", cfg.Upstreams.ReactionInternalAuthToken)
	}
	if cfg.Upstreams.SearchInternalAuthToken != "env-search-internal-token-with-at-least-32-bytes" {
		t.Fatalf("search internal auth token = %q", cfg.Upstreams.SearchInternalAuthToken)
	}
	if cfg.Upstreams.User != "file-user-service" {
		t.Fatalf("user upstream = %q", cfg.Upstreams.User)
	}
	if cfg.Chat.RateLimit.CreateRoomInterval != 8*time.Minute || cfg.Chat.RateLimit.CreateRoomRate != 4 {
		t.Fatalf("chat create room rate limit = %s/%d", cfg.Chat.RateLimit.CreateRoomInterval, cfg.Chat.RateLimit.CreateRoomRate)
	}
	if cfg.Chat.RateLimit.TicketInterval != 4*time.Minute || cfg.Chat.RateLimit.TicketRate != 9 {
		t.Fatalf("chat ticket rate limit = %s/%d", cfg.Chat.RateLimit.TicketInterval, cfg.Chat.RateLimit.TicketRate)
	}
	if cfg.Chat.RateLimit.SubscribeInterval != 5*time.Minute || cfg.Chat.RateLimit.SubscribeRate != 11 {
		t.Fatalf("chat subscribe rate limit = %s/%d", cfg.Chat.RateLimit.SubscribeInterval, cfg.Chat.RateLimit.SubscribeRate)
	}
	if cfg.Chat.RateLimit.JoinInterval != 2*time.Minute || cfg.Chat.RateLimit.JoinRate != 12 {
		t.Fatalf("chat join rate limit = %s/%d", cfg.Chat.RateLimit.JoinInterval, cfg.Chat.RateLimit.JoinRate)
	}
	if cfg.Chat.RateLimit.SendInterval != 3*time.Second || cfg.Chat.RateLimit.SendRate != 7 {
		t.Fatalf("chat send rate limit = %s/%d", cfg.Chat.RateLimit.SendInterval, cfg.Chat.RateLimit.SendRate)
	}
	if cfg.Chat.RateLimit.ReadInterval != 6*time.Minute || cfg.Chat.RateLimit.ReadRate != 17 {
		t.Fatalf("chat read rate limit = %s/%d", cfg.Chat.RateLimit.ReadInterval, cfg.Chat.RateLimit.ReadRate)
	}
	if cfg.Chat.WebSocket.MaxConnectionsPerUser != 7 || cfg.Chat.WebSocket.MaxConnectionsPerIP != 70 {
		t.Fatalf("chat websocket limits = %d/%d", cfg.Chat.WebSocket.MaxConnectionsPerUser, cfg.Chat.WebSocket.MaxConnectionsPerIP)
	}
}

func TestLoadConfigDerivesLocalPublicBaseURLFromGatewayPortOverride(t *testing.T) {
	path := writeGatewayConfigFile(t, `
service:
  httpPort: 18080
trace:
  env: local
http:
  publicBaseURL: http://127.0.0.1:18080
`)
	t.Setenv("BBS_GATEWAY_SERVICE_HTTP_PORT", "28080")
	t.Setenv("BBS_GATEWAY_HTTP_PUBLIC_BASE_URL", "")

	v, cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.PublicBaseURL != "http://127.0.0.1:28080" {
		t.Fatalf("public base URL = %q, want isolated Gateway port", cfg.PublicBaseURL)
	}
	if v.GetString("http.publicBaseURL") != "http://127.0.0.1:28080" {
		t.Fatalf("viper public base URL = %q", v.GetString("http.publicBaseURL"))
	}
}

func TestLoadConfigPreservesExplicitLocalPublicBaseURL(t *testing.T) {
	path := writeGatewayConfigFile(t, `
service:
  httpPort: 18080
trace:
  env: local
http:
  publicBaseURL: http://127.0.0.1:29080
`)
	t.Setenv("BBS_GATEWAY_SERVICE_HTTP_PORT", "28080")

	_, cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.PublicBaseURL != "http://127.0.0.1:29080" {
		t.Fatalf("public base URL = %q, want explicit local URL", cfg.PublicBaseURL)
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
	if cfg.Auth.RateLimit.RegisterInterval != time.Hour || cfg.Auth.RateLimit.RegisterRate != 5 {
		t.Fatalf("register defaults = %s/%d", cfg.Auth.RateLimit.RegisterInterval, cfg.Auth.RateLimit.RegisterRate)
	}
	if cfg.Auth.RateLimit.LoginInterval != time.Minute || cfg.Auth.RateLimit.LoginRate != 10 {
		t.Fatalf("login defaults = %s/%d", cfg.Auth.RateLimit.LoginInterval, cfg.Auth.RateLimit.LoginRate)
	}
	if cfg.Auth.RateLimit.PasswordResetInterval != 15*time.Minute || cfg.Auth.RateLimit.PasswordResetRate != 5 {
		t.Fatalf("password reset defaults = %s/%d", cfg.Auth.RateLimit.PasswordResetInterval, cfg.Auth.RateLimit.PasswordResetRate)
	}
	if cfg.Auth.RateLimit.PasswordResetConfirmInterval != 15*time.Minute || cfg.Auth.RateLimit.PasswordResetConfirmRate != 5 {
		t.Fatalf("password reset confirm defaults = %s/%d", cfg.Auth.RateLimit.PasswordResetConfirmInterval, cfg.Auth.RateLimit.PasswordResetConfirmRate)
	}
	if cfg.Auth.RateLimit.EmailVerificationInterval != 15*time.Minute || cfg.Auth.RateLimit.EmailVerificationRate != 5 {
		t.Fatalf("email verification defaults = %s/%d", cfg.Auth.RateLimit.EmailVerificationInterval, cfg.Auth.RateLimit.EmailVerificationRate)
	}
	if cfg.Auth.RateLimit.AdminLoginInterval != 15*time.Minute || cfg.Auth.RateLimit.AdminLoginRate != 5 {
		t.Fatalf("admin login defaults = %s/%d", cfg.Auth.RateLimit.AdminLoginInterval, cfg.Auth.RateLimit.AdminLoginRate)
	}
	if cfg.Search.RateLimit.ContentInterval != time.Minute || cfg.Search.RateLimit.ContentRate != 60 {
		t.Fatalf("content search defaults = %s/%d", cfg.Search.RateLimit.ContentInterval, cfg.Search.RateLimit.ContentRate)
	}
	if cfg.Search.RateLimit.UserInterval != time.Minute || cfg.Search.RateLimit.UserRate != 10 {
		t.Fatalf("user search defaults = %s/%d", cfg.Search.RateLimit.UserInterval, cfg.Search.RateLimit.UserRate)
	}
	if cfg.Upstreams.Admin == "" || cfg.Upstreams.User == "" || cfg.Upstreams.Notification == "" || cfg.Upstreams.Chat == "" {
		t.Fatalf("expected default upstreams, got %#v", cfg.Upstreams)
	}
	if cfg.Upstreams.ChatInternalAuthToken != clients.LocalDevChatInternalAuthToken {
		t.Fatalf("chat internal auth token = %q", cfg.Upstreams.ChatInternalAuthToken)
	}
	if cfg.Upstreams.AdminInternalAuthToken != clients.LocalDevAdminInternalAuthToken {
		t.Fatalf("admin internal auth token = %q", cfg.Upstreams.AdminInternalAuthToken)
	}
	if cfg.Upstreams.UserInternalAuthToken != clients.LocalDevUserInternalAuthToken {
		t.Fatalf("user internal auth token = %q", cfg.Upstreams.UserInternalAuthToken)
	}
	if cfg.Upstreams.MallInternalAuthToken != clients.LocalDevMallInternalAuthToken {
		t.Fatalf("mall internal auth token = %q", cfg.Upstreams.MallInternalAuthToken)
	}
	if cfg.Upstreams.CreditInternalAuthToken != clients.LocalDevCreditInternalAuthToken {
		t.Fatalf("credit internal auth token = %q", cfg.Upstreams.CreditInternalAuthToken)
	}
	if cfg.Upstreams.FeedInternalAuthToken != clients.LocalDevFeedInternalAuthToken {
		t.Fatalf("feed internal auth token = %q", cfg.Upstreams.FeedInternalAuthToken)
	}
	if cfg.Upstreams.NotificationInternalAuthToken != clients.LocalDevNotificationInternalAuthToken {
		t.Fatalf("notification internal auth token = %q", cfg.Upstreams.NotificationInternalAuthToken)
	}
	if cfg.Upstreams.ReactionInternalAuthToken != clients.LocalDevReactionInternalAuthToken {
		t.Fatalf("reaction internal auth token = %q", cfg.Upstreams.ReactionInternalAuthToken)
	}
	if cfg.Upstreams.SearchInternalAuthToken != clients.LocalDevSearchInternalAuthToken {
		t.Fatalf("search internal auth token = %q", cfg.Upstreams.SearchInternalAuthToken)
	}
	if cfg.Upstreams.CommentInternalAuthToken != clients.LocalDevCommentInternalAuthToken {
		t.Fatalf("comment internal auth token = %q", cfg.Upstreams.CommentInternalAuthToken)
	}
	if cfg.Upstreams.ContentInternalAuthToken != clients.LocalDevContentInternalAuthToken {
		t.Fatalf("content internal auth token = %q", cfg.Upstreams.ContentInternalAuthToken)
	}
	if cfg.Upstreams.AdminInternalAuthSecure || cfg.Upstreams.ChatInternalAuthSecure {
		t.Fatalf("mTLS upstream defaults should be disabled, got %#v", cfg.Upstreams)
	}
	if cfg.Chat.RateLimit.CreateRoomInterval != time.Minute || cfg.Chat.RateLimit.CreateRoomRate != 5 {
		t.Fatalf("chat create room defaults = %s/%d", cfg.Chat.RateLimit.CreateRoomInterval, cfg.Chat.RateLimit.CreateRoomRate)
	}
	if cfg.Chat.RateLimit.TicketInterval != time.Minute || cfg.Chat.RateLimit.TicketRate != 10 {
		t.Fatalf("chat ticket defaults = %s/%d", cfg.Chat.RateLimit.TicketInterval, cfg.Chat.RateLimit.TicketRate)
	}
	if cfg.Chat.RateLimit.SubscribeInterval != time.Minute || cfg.Chat.RateLimit.SubscribeRate != 10 {
		t.Fatalf("chat subscribe defaults = %s/%d", cfg.Chat.RateLimit.SubscribeInterval, cfg.Chat.RateLimit.SubscribeRate)
	}
	if cfg.Chat.RateLimit.JoinInterval != time.Minute || cfg.Chat.RateLimit.JoinRate != 10 {
		t.Fatalf("chat join defaults = %s/%d", cfg.Chat.RateLimit.JoinInterval, cfg.Chat.RateLimit.JoinRate)
	}
	if cfg.Chat.RateLimit.SendInterval != time.Second || cfg.Chat.RateLimit.SendRate != 5 {
		t.Fatalf("chat send defaults = %s/%d", cfg.Chat.RateLimit.SendInterval, cfg.Chat.RateLimit.SendRate)
	}
	if cfg.Chat.RateLimit.ReadInterval != time.Minute || cfg.Chat.RateLimit.ReadRate != 60 {
		t.Fatalf("chat read defaults = %s/%d", cfg.Chat.RateLimit.ReadInterval, cfg.Chat.RateLimit.ReadRate)
	}
	if cfg.Chat.WebSocket.MaxConnectionsPerUser != 5 || cfg.Chat.WebSocket.MaxConnectionsPerIP != 50 {
		t.Fatalf("chat websocket defaults = %d/%d", cfg.Chat.WebSocket.MaxConnectionsPerUser, cfg.Chat.WebSocket.MaxConnectionsPerIP)
	}
}

func TestLoadConfigRejectsDefaultJWTSecretInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, "trace:\n  env: production\n")

	_, _, err := loadConfig(path)
	if err == nil {
		t.Fatal("expected production config with default JWT secret to fail")
	}
}

func TestLoadConfigRejectsInvalidPublicBaseURL(t *testing.T) {
	path := writeGatewayConfigFile(t, "http:\n  publicBaseURL: javascript:alert(1)\n")

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "http.publicBaseURL") {
		t.Fatalf("load config error = %v, want public base URL error", err)
	}
}

func TestLoadConfigRequiresPublicBaseURLInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, strings.Replace(validProductionGatewayConfig, "http:\n  publicBaseURL: https://bbs.example.com\n", "", 1))

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "http.publicBaseURL") {
		t.Fatalf("load config error = %v, want public base URL error", err)
	}
}

func TestLoadConfigRejectsDefaultChatInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, `
trace:
  env: production
auth:
  jwtSecret: production-jwt-secret-with-at-least-32-chars
cors:
  allowedOrigins:
    - https://bbs.example.com
upstreams:
  adminInternalAuthToken: production-admin-internal-token-with-32-bytes
  userInternalAuthToken: production-user-internal-token-with-32-bytes
`)

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "chatInternalAuthToken") {
		t.Fatalf("load config error = %v, want default chat internal auth token error", err)
	}
}

func TestLoadConfigRejectsDefaultAdminInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, `
trace:
  env: production
auth:
  jwtSecret: production-jwt-secret-with-at-least-32-chars
cors:
  allowedOrigins:
    - https://bbs.example.com
upstreams:
  chatInternalAuthToken: production-chat-internal-token-with-32-bytes
  userInternalAuthToken: production-user-internal-token-with-32-bytes
  mallInternalAuthToken: production-mall-internal-token-with-32-bytes
`)

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "adminInternalAuthToken") {
		t.Fatalf("load config error = %v, want default admin internal auth token error", err)
	}
}

func TestLoadConfigRejectsDefaultUserInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, `
trace:
  env: production
auth:
  jwtSecret: production-jwt-secret-with-at-least-32-chars
cors:
  allowedOrigins:
    - https://bbs.example.com
upstreams:
  adminInternalAuthToken: production-admin-internal-token-with-32-bytes
  chatInternalAuthToken: production-chat-internal-token-with-32-bytes
`)

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "userInternalAuthToken") {
		t.Fatalf("load config error = %v, want default user internal auth token error", err)
	}
}

func TestLoadConfigAcceptsConfiguredChatInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, validProductionGatewayConfig)

	_, cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Upstreams.ChatInternalAuthToken != "production-chat-internal-token-with-32-bytes" {
		t.Fatalf("chat internal auth token = %q", cfg.Upstreams.ChatInternalAuthToken)
	}
}

func TestLoadConfigAcceptsConfiguredUserInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, validProductionGatewayConfig)

	_, cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Upstreams.UserInternalAuthToken != "production-user-internal-token-with-32-bytes" {
		t.Fatalf("user internal auth token = %q", cfg.Upstreams.UserInternalAuthToken)
	}
}

func TestLoadConfigRejectsShortChatInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, `
trace:
  env: production
auth:
  jwtSecret: production-jwt-secret-with-at-least-32-chars
cors:
  allowedOrigins:
    - https://bbs.example.com
upstreams:
  adminInternalAuthToken: production-admin-internal-token-with-32-bytes
  chatInternalAuthToken: too-short
  userInternalAuthToken: production-user-internal-token-with-32-bytes
`)

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "chatInternalAuthToken") {
		t.Fatalf("load config error = %v, want short chat internal auth token error", err)
	}
}

func TestLoadConfigRejectsShortUserInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, `
trace:
  env: production
auth:
  jwtSecret: production-jwt-secret-with-at-least-32-chars
cors:
  allowedOrigins:
    - https://bbs.example.com
upstreams:
  adminInternalAuthToken: production-admin-internal-token-with-32-bytes
  chatInternalAuthToken: production-chat-internal-token-with-32-bytes
  userInternalAuthToken: too-short
`)

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "userInternalAuthToken") {
		t.Fatalf("load config error = %v, want short user internal auth token error", err)
	}
}

func TestLoadConfigRejectsDefaultMallInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, `
trace:
  env: production
auth:
  jwtSecret: production-jwt-secret-with-at-least-32-chars
cors:
  allowedOrigins:
    - https://bbs.example.com
upstreams:
  adminInternalAuthToken: production-admin-internal-token-with-32-bytes
  chatInternalAuthToken: production-chat-internal-token-with-32-bytes
  userInternalAuthToken: production-user-internal-token-with-32-bytes
`)

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "mallInternalAuthToken") {
		t.Fatalf("load config error = %v, want default mall internal auth token error", err)
	}
}

func TestLoadConfigRejectsShortMallInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, `
trace:
  env: production
auth:
  jwtSecret: production-jwt-secret-with-at-least-32-chars
cors:
  allowedOrigins:
    - https://bbs.example.com
upstreams:
  adminInternalAuthToken: production-admin-internal-token-with-32-bytes
  chatInternalAuthToken: production-chat-internal-token-with-32-bytes
  userInternalAuthToken: production-user-internal-token-with-32-bytes
  mallInternalAuthToken: too-short
`)

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "mallInternalAuthToken") {
		t.Fatalf("load config error = %v, want short mall internal auth token error", err)
	}
}

func TestLoadConfigAcceptsConfiguredMallInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, validProductionGatewayConfig)

	_, cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Upstreams.MallInternalAuthToken != "production-mall-internal-token-with-32-bytes" {
		t.Fatalf("mall internal auth token = %q", cfg.Upstreams.MallInternalAuthToken)
	}
}

func TestLoadConfigRejectsDefaultCreditInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, `
trace:
  env: production
auth:
  jwtSecret: production-jwt-secret-with-at-least-32-chars
cors:
  allowedOrigins:
    - https://bbs.example.com
upstreams:
  adminInternalAuthToken: production-admin-internal-token-with-32-bytes
  chatInternalAuthToken: production-chat-internal-token-with-32-bytes
  userInternalAuthToken: production-user-internal-token-with-32-bytes
  mallInternalAuthToken: production-mall-internal-token-with-32-bytes
`)

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "creditInternalAuthToken") {
		t.Fatalf("load config error = %v, want default credit internal auth token error", err)
	}
}

func TestLoadConfigRejectsShortCreditInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, `
trace:
  env: production
auth:
  jwtSecret: production-jwt-secret-with-at-least-32-chars
cors:
  allowedOrigins:
    - https://bbs.example.com
upstreams:
  adminInternalAuthToken: production-admin-internal-token-with-32-bytes
  chatInternalAuthToken: production-chat-internal-token-with-32-bytes
  userInternalAuthToken: production-user-internal-token-with-32-bytes
  mallInternalAuthToken: production-mall-internal-token-with-32-bytes
  creditInternalAuthToken: too-short
`)

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "creditInternalAuthToken") {
		t.Fatalf("load config error = %v, want short credit internal auth token error", err)
	}
}

func TestLoadConfigAcceptsConfiguredCreditInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, validProductionGatewayConfig)

	_, cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Upstreams.CreditInternalAuthToken != "production-credit-internal-token-with-32-bytes" {
		t.Fatalf("credit internal auth token = %q", cfg.Upstreams.CreditInternalAuthToken)
	}
}

func TestLoadConfigRejectsDefaultFileInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, `
trace:
  env: production
http:
  publicBaseURL: https://bbs.example.com
auth:
  jwtSecret: production-jwt-secret-with-at-least-32-chars
cors:
  allowedOrigins:
    - https://bbs.example.com
upstreams:
  adminInternalAuthToken: production-admin-internal-token-with-32-bytes
  chatInternalAuthToken: production-chat-internal-token-with-32-bytes
  userInternalAuthToken: production-user-internal-token-with-32-bytes
  mallInternalAuthToken: production-mall-internal-token-with-32-bytes
  creditInternalAuthToken: production-credit-internal-token-with-32-bytes
`)

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "fileInternalAuthToken") {
		t.Fatalf("load config error = %v, want default file internal auth token error", err)
	}
}

func TestLoadConfigRejectsShortFileInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, strings.Replace(validProductionGatewayConfig, "fileInternalAuthToken: production-file-internal-token-with-32-bytes", "fileInternalAuthToken: too-short", 1))

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "fileInternalAuthToken") {
		t.Fatalf("load config error = %v, want short file internal auth token error", err)
	}
}

func TestLoadConfigAcceptsConfiguredFileInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, validProductionGatewayConfig)

	_, cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Upstreams.FileInternalAuthToken != "production-file-internal-token-with-32-bytes" {
		t.Fatalf("file internal auth token = %q", cfg.Upstreams.FileInternalAuthToken)
	}
}

func TestLoadConfigRejectsDefaultFeedInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, strings.Replace(validProductionGatewayConfig, "  feedInternalAuthToken: production-feed-internal-token-with-32-bytes\n", "", 1))

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "feedInternalAuthToken") {
		t.Fatalf("load config error = %v, want default feed internal auth token error", err)
	}
}

func TestLoadConfigRejectsShortFeedInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, strings.Replace(validProductionGatewayConfig, "feedInternalAuthToken: production-feed-internal-token-with-32-bytes", "feedInternalAuthToken: too-short", 1))

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "feedInternalAuthToken") {
		t.Fatalf("load config error = %v, want short feed internal auth token error", err)
	}
}

func TestLoadConfigAcceptsConfiguredFeedInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, validProductionGatewayConfig)

	_, cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Upstreams.FeedInternalAuthToken != "production-feed-internal-token-with-32-bytes" {
		t.Fatalf("feed internal auth token = %q", cfg.Upstreams.FeedInternalAuthToken)
	}
}

func TestLoadConfigRejectsDefaultNotificationInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, strings.Replace(validProductionGatewayConfig, "  notificationInternalAuthToken: production-notification-internal-token-with-32-bytes\n", "", 1))

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "notificationInternalAuthToken") {
		t.Fatalf("load config error = %v, want default notification internal auth token error", err)
	}
}

func TestLoadConfigRejectsShortNotificationInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, strings.Replace(validProductionGatewayConfig, "notificationInternalAuthToken: production-notification-internal-token-with-32-bytes", "notificationInternalAuthToken: too-short", 1))

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "notificationInternalAuthToken") {
		t.Fatalf("load config error = %v, want short notification internal auth token error", err)
	}
}

func TestLoadConfigAcceptsConfiguredNotificationInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, validProductionGatewayConfig)

	_, cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Upstreams.NotificationInternalAuthToken != "production-notification-internal-token-with-32-bytes" {
		t.Fatalf("notification internal auth token = %q", cfg.Upstreams.NotificationInternalAuthToken)
	}
}

func TestLoadConfigRejectsDefaultReactionInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, strings.Replace(validProductionGatewayConfig, "  reactionInternalAuthToken: production-reaction-internal-token-with-32-bytes\n", "", 1))

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "reactionInternalAuthToken") {
		t.Fatalf("load config error = %v, want default reaction internal auth token error", err)
	}
}

func TestLoadConfigRejectsShortReactionInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, strings.Replace(validProductionGatewayConfig, "reactionInternalAuthToken: production-reaction-internal-token-with-32-bytes", "reactionInternalAuthToken: too-short", 1))

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "reactionInternalAuthToken") {
		t.Fatalf("load config error = %v, want short reaction internal auth token error", err)
	}
}

func TestLoadConfigAcceptsConfiguredReactionInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, validProductionGatewayConfig)

	_, cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Upstreams.ReactionInternalAuthToken != "production-reaction-internal-token-with-32-bytes" {
		t.Fatalf("reaction internal auth token = %q", cfg.Upstreams.ReactionInternalAuthToken)
	}
}

func TestLoadConfigRejectsDefaultSearchInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, strings.Replace(validProductionGatewayConfig, "  searchInternalAuthToken: production-search-internal-token-with-32-bytes\n", "", 1))

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "searchInternalAuthToken") {
		t.Fatalf("load config error = %v, want default search internal auth token error", err)
	}
}

func TestLoadConfigRejectsShortSearchInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, strings.Replace(validProductionGatewayConfig, "searchInternalAuthToken: production-search-internal-token-with-32-bytes", "searchInternalAuthToken: too-short", 1))

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "searchInternalAuthToken") {
		t.Fatalf("load config error = %v, want short search internal auth token error", err)
	}
}

func TestLoadConfigAcceptsConfiguredSearchInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, validProductionGatewayConfig)

	_, cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Upstreams.SearchInternalAuthToken != "production-search-internal-token-with-32-bytes" {
		t.Fatalf("search internal auth token = %q", cfg.Upstreams.SearchInternalAuthToken)
	}
}

func TestLoadConfigRejectsDefaultCommentInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, strings.Replace(validProductionGatewayConfig, "  commentInternalAuthToken: production-comment-internal-token-with-32-bytes\n", "", 1))

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "commentInternalAuthToken") {
		t.Fatalf("load config error = %v, want default comment internal auth token error", err)
	}
}

func TestLoadConfigRejectsShortCommentInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, strings.Replace(validProductionGatewayConfig, "commentInternalAuthToken: production-comment-internal-token-with-32-bytes", "commentInternalAuthToken: too-short", 1))

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "commentInternalAuthToken") {
		t.Fatalf("load config error = %v, want short comment internal auth token error", err)
	}
}

func TestLoadConfigAcceptsConfiguredCommentInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, validProductionGatewayConfig)

	_, cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Upstreams.CommentInternalAuthToken != "production-comment-internal-token-with-32-bytes" {
		t.Fatalf("comment internal auth token = %q", cfg.Upstreams.CommentInternalAuthToken)
	}
}

func TestLoadConfigRejectsDefaultContentInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, strings.Replace(validProductionGatewayConfig, "  contentInternalAuthToken: production-content-internal-token-with-32-bytes\n", "", 1))

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "contentInternalAuthToken") {
		t.Fatalf("load config error = %v, want default content internal auth token error", err)
	}
}

func TestLoadConfigRejectsShortContentInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, strings.Replace(validProductionGatewayConfig, "contentInternalAuthToken: production-content-internal-token-with-32-bytes", "contentInternalAuthToken: too-short", 1))

	_, _, err := loadConfig(path)
	if err == nil || !strings.Contains(err.Error(), "contentInternalAuthToken") {
		t.Fatalf("load config error = %v, want short content internal auth token error", err)
	}
}

func TestLoadConfigAcceptsConfiguredContentInternalAuthTokenInProduction(t *testing.T) {
	path := writeGatewayConfigFile(t, validProductionGatewayConfig)

	_, cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Upstreams.ContentInternalAuthToken != "production-content-internal-token-with-32-bytes" {
		t.Fatalf("content internal auth token = %q", cfg.Upstreams.ContentInternalAuthToken)
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
upstreams:
  adminInternalAuthToken: production-admin-internal-token-with-32-bytes
  chatInternalAuthToken: production-chat-internal-token-with-32-bytes
  userInternalAuthToken: production-user-internal-token-with-32-bytes
  mallInternalAuthToken: production-mall-internal-token-with-32-bytes
  creditInternalAuthToken: production-credit-internal-token-with-32-bytes
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
upstreams:
  adminInternalAuthToken: production-admin-internal-token-with-32-bytes
  chatInternalAuthToken: production-chat-internal-token-with-32-bytes
  userInternalAuthToken: production-user-internal-token-with-32-bytes
  mallInternalAuthToken: production-mall-internal-token-with-32-bytes
  creditInternalAuthToken: production-credit-internal-token-with-32-bytes
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
upstreams:
  adminInternalAuthToken: production-admin-internal-token-with-32-bytes
  chatInternalAuthToken: production-chat-internal-token-with-32-bytes
  userInternalAuthToken: production-user-internal-token-with-32-bytes
  mallInternalAuthToken: production-mall-internal-token-with-32-bytes
  creditInternalAuthToken: production-credit-internal-token-with-32-bytes
`)

	_, _, err := loadConfig(path)
	if err == nil {
		t.Fatal("expected production config with local CORS origin to fail")
	}
}

func TestLoadConfigRejectsIncompleteProductionMTLSConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "admin plaintext",
			content: strings.Replace(validProductionGatewayConfig, "adminInternalAuthSecure: true", "adminInternalAuthSecure: false", 1),
			want:    "adminInternalAuthSecure",
		},
		{
			name:    "chat plaintext",
			content: strings.Replace(validProductionGatewayConfig, "chatInternalAuthSecure: true", "chatInternalAuthSecure: false", 1),
			want:    "chatInternalAuthSecure",
		},
		{
			name:    "missing client ca",
			content: strings.Replace(validProductionGatewayConfig, "caFile: /mnt/gateway-tls/ca.crt", "caFile: ''", 1),
			want:    "grpc.client.tls.caFile",
		},
		{
			name:    "legacy global switch would enable plaintext user upstream",
			content: strings.Replace(validProductionGatewayConfig, "  client:\n", "  client:\n    secure: true\n", 1),
			want:    "upstreams.userInternalAuthSecure",
		},
		{
			name:    "shared server name",
			content: strings.Replace(validProductionGatewayConfig, "      keyFile: /mnt/gateway-tls/tls.key", "      keyFile: /mnt/gateway-tls/tls.key\n      serverName: bbs-admin-service", 1),
			want:    "grpc.client.tls.serverName",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := loadConfig(writeGatewayConfigFile(t, tt.content))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("load config error = %v, want %q", err, tt.want)
			}
		})
	}
}

const validProductionGatewayConfig = `
trace:
  env: production
http:
  publicBaseURL: https://bbs.example.com
auth:
  jwtSecret: production-jwt-secret-with-at-least-32-chars
cors:
  allowedOrigins:
    - https://bbs.example.com
grpc:
  client:
    tls:
      caFile: /mnt/gateway-tls/ca.crt
      certFile: /mnt/gateway-tls/tls.crt
      keyFile: /mnt/gateway-tls/tls.key
upstreams:
  adminInternalAuthToken: production-admin-internal-token-with-32-bytes
  adminInternalAuthSecure: true
  chatInternalAuthToken: production-chat-internal-token-with-32-bytes
  chatInternalAuthSecure: true
  userInternalAuthToken: production-user-internal-token-with-32-bytes
  mallInternalAuthToken: production-mall-internal-token-with-32-bytes
  creditInternalAuthToken: production-credit-internal-token-with-32-bytes
  fileInternalAuthToken: production-file-internal-token-with-32-bytes
  feedInternalAuthToken: production-feed-internal-token-with-32-bytes
  notificationInternalAuthToken: production-notification-internal-token-with-32-bytes
  reactionInternalAuthToken: production-reaction-internal-token-with-32-bytes
  searchInternalAuthToken: production-search-internal-token-with-32-bytes
  commentInternalAuthToken: production-comment-internal-token-with-32-bytes
  contentInternalAuthToken: production-content-internal-token-with-32-bytes
`

func writeGatewayConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
