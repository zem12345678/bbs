package server

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"api-gateway/internal/clients"

	"github.com/spf13/viper"
)

type runtimeConfig struct {
	Service struct {
		Name     string
		HTTPPort int
	}
	Auth struct {
		TokenHeader string
		TokenPrefix string
		JWTSecret   string
	}
	Upstreams clients.Options
}

const (
	localDevJWTSecret            = "bbs-local-dev-secret"
	minProductionJWTSecretLength = 32
)

func loadConfig(path string) (*viper.Viper, *runtimeConfig, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, nil, err
	}
	cfg, err := loadRuntimeConfig(v)
	if err != nil {
		return nil, nil, err
	}
	return v, cfg, nil
}

func loadRuntimeConfig(v *viper.Viper) (*runtimeConfig, error) {
	configureEnv(v)
	applyEnvOverrides(v)

	var cfg runtimeConfig
	cfg.Service.Name = firstNonEmpty(v.GetString("service.name"), "bbs-api-gateway")
	cfg.Service.HTTPPort = v.GetInt("service.httpPort")
	if cfg.Service.HTTPPort == 0 {
		cfg.Service.HTTPPort = 8080
	}
	cfg.Auth.TokenHeader = firstNonEmpty(v.GetString("auth.tokenHeader"), "Authorization")
	cfg.Auth.TokenPrefix = firstNonEmpty(v.GetString("auth.tokenPrefix"), "Bearer")
	cfg.Auth.JWTSecret = firstNonEmpty(v.GetString("auth.jwtSecret"), localDevJWTSecret)
	if isProductionEnvironment(v.GetString("trace.env")) {
		if err := validateProductionSecurityConfig(cfg.Auth.JWTSecret, v.GetStringSlice("cors.allowedOrigins")); err != nil {
			return nil, err
		}
	}
	cfg.Upstreams = clients.NewOptions(v)

	v.Set("service.name", cfg.Service.Name)
	v.Set("service.httpPort", cfg.Service.HTTPPort)
	v.Set("auth.tokenHeader", cfg.Auth.TokenHeader)
	v.Set("auth.tokenPrefix", cfg.Auth.TokenPrefix)
	v.Set("auth.jwtSecret", cfg.Auth.JWTSecret)
	return &cfg, nil
}

func validateProductionSecurityConfig(jwtSecret string, corsAllowedOrigins []string) error {
	secret := strings.TrimSpace(jwtSecret)
	if secret == "" || secret == localDevJWTSecret {
		return fmt.Errorf("auth.jwtSecret must be set to a non-default value in production")
	}
	if len(secret) < minProductionJWTSecretLength {
		return fmt.Errorf("auth.jwtSecret must be at least %d characters in production", minProductionJWTSecretLength)
	}
	if len(corsAllowedOrigins) == 0 {
		return fmt.Errorf("cors.allowedOrigins must be explicitly set in production")
	}
	for _, origin := range corsAllowedOrigins {
		if err := validateProductionCORSOrigin(origin); err != nil {
			return err
		}
	}
	return nil
}

func validateProductionCORSOrigin(origin string) error {
	trimmed := strings.TrimSpace(origin)
	if trimmed == "" {
		return fmt.Errorf("cors.allowedOrigins contains an empty origin in production")
	}
	if trimmed == "*" {
		return fmt.Errorf("cors.allowedOrigins must not contain wildcard origins in production")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("cors.allowedOrigins contains invalid origin %q in production", origin)
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return fmt.Errorf("cors.allowedOrigins must use https in production: %q", origin)
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || host == "0.0.0.0" || host == "::1" || strings.HasPrefix(host, "127.") {
		return fmt.Errorf("cors.allowedOrigins must not contain local origins in production: %q", origin)
	}
	return nil
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_GATEWAY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "app.name", "BBS_GATEWAY_APP_NAME")
	bindEnv(v, "service.name", "BBS_GATEWAY_SERVICE_NAME")
	bindEnv(v, "service.httpPort", "BBS_GATEWAY_SERVICE_HTTP_PORT")
	bindEnv(v, "auth.tokenHeader", "BBS_GATEWAY_AUTH_TOKEN_HEADER")
	bindEnv(v, "auth.tokenPrefix", "BBS_GATEWAY_AUTH_TOKEN_PREFIX")
	bindEnv(v, "auth.jwtSecret", "BBS_GATEWAY_AUTH_JWT_SECRET")
	bindEnv(v, "log.filename", "BBS_GATEWAY_LOG_FILENAME")
	bindEnv(v, "log.level", "BBS_GATEWAY_LOG_LEVEL")
	bindEnv(v, "log.stdout", "BBS_GATEWAY_LOG_STDOUT")
	bindEnv(v, "trace.grpcEndpoint", "BBS_GATEWAY_TRACE_GRPC_ENDPOINT")
	bindEnv(v, "trace.serviceName", "BBS_GATEWAY_TRACE_SERVICE_NAME")
	bindEnv(v, "trace.version", "BBS_GATEWAY_TRACE_VERSION")
	bindEnv(v, "trace.env", "BBS_GATEWAY_TRACE_ENV")
	bindEnv(v, "cors.allowedOrigins", "BBS_GATEWAY_CORS_ALLOWED_ORIGINS")
	bindEnv(v, "upstreams.admin", "BBS_GATEWAY_UPSTREAMS_ADMIN")
	bindEnv(v, "upstreams.user", "BBS_GATEWAY_UPSTREAMS_USER")
	bindEnv(v, "upstreams.content", "BBS_GATEWAY_UPSTREAMS_CONTENT")
	bindEnv(v, "upstreams.comment", "BBS_GATEWAY_UPSTREAMS_COMMENT")
	bindEnv(v, "upstreams.reaction", "BBS_GATEWAY_UPSTREAMS_REACTION")
	bindEnv(v, "upstreams.search", "BBS_GATEWAY_UPSTREAMS_SEARCH")
	bindEnv(v, "upstreams.feed", "BBS_GATEWAY_UPSTREAMS_FEED")
	bindEnv(v, "upstreams.credit", "BBS_GATEWAY_UPSTREAMS_CREDIT")
	bindEnv(v, "upstreams.mall", "BBS_GATEWAY_UPSTREAMS_MALL")
	bindEnv(v, "upstreams.notification", "BBS_GATEWAY_UPSTREAMS_NOTIFICATION")
	bindEnv(v, "upstreams.file", "BBS_GATEWAY_UPSTREAMS_FILE")
	bindEnv(v, "storage.endpoint", "BBS_GATEWAY_STORAGE_ENDPOINT")
	bindEnv(v, "storage.bucket", "BBS_GATEWAY_STORAGE_BUCKET")
	bindEnv(v, "storage.accessKey", "BBS_GATEWAY_STORAGE_ACCESS_KEY")
	bindEnv(v, "storage.secretKey", "BBS_GATEWAY_STORAGE_SECRET_KEY")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func applyEnvOverrides(v *viper.Viper) {
	if value := strings.TrimSpace(os.Getenv("BBS_GATEWAY_CORS_ALLOWED_ORIGINS")); value != "" {
		v.Set("cors.allowedOrigins", splitCommaSeparated(value))
	}
	if value := strings.TrimSpace(os.Getenv("BBS_GATEWAY_GRPC_CLIENT_ETCD_ADDR")); value != "" {
		v.Set("grpc.client.etcdAddr", splitCommaSeparated(value))
	}
}

func splitCommaSeparated(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func isProductionEnvironment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}
