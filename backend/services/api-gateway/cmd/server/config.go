package server

import (
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
	cfg.Service.Name = firstNonEmpty(v.GetString("service.name"), "api-gateway")
	cfg.Service.HTTPPort = v.GetInt("service.httpPort")
	if cfg.Service.HTTPPort == 0 {
		cfg.Service.HTTPPort = 8080
	}
	cfg.Auth.TokenHeader = firstNonEmpty(v.GetString("auth.tokenHeader"), "Authorization")
	cfg.Auth.TokenPrefix = firstNonEmpty(v.GetString("auth.tokenPrefix"), "Bearer")
	cfg.Auth.JWTSecret = firstNonEmpty(v.GetString("auth.jwtSecret"), "bbs-local-dev-secret")
	cfg.Upstreams = clients.NewOptions(v)

	v.Set("service.name", cfg.Service.Name)
	v.Set("service.httpPort", cfg.Service.HTTPPort)
	v.Set("auth.tokenHeader", cfg.Auth.TokenHeader)
	v.Set("auth.tokenPrefix", cfg.Auth.TokenPrefix)
	v.Set("auth.jwtSecret", cfg.Auth.JWTSecret)
	return &cfg, nil
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
	bindEnv(v, "upstreams.notification", "BBS_GATEWAY_UPSTREAMS_NOTIFICATION")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func applyEnvOverrides(v *viper.Viper) {
	if value := strings.TrimSpace(os.Getenv("BBS_GATEWAY_CORS_ALLOWED_ORIGINS")); value != "" {
		v.Set("cors.allowedOrigins", splitCommaSeparated(value))
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
