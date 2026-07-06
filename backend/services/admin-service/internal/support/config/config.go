package config

import (
	"admin/internal/infrastructure/upstream"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Service struct {
		Name     string
		GRPCPort int
	}
	Postgres struct {
		DSN   string
		Debug bool
	}
	RBAC struct {
		BootstrapAdminPrefixes []string
	}
	Auth struct {
		JWTSecret            string `mapstructure:"jwtSecret"`
		JWTTTL               string `mapstructure:"jwtTtl"`
		DefaultAdminPassword string `mapstructure:"defaultAdminPassword"`
		SecretEncryptionKey  string `mapstructure:"secretEncryptionKey"`
	}
	Upstreams upstream.Options
}

func New(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	configureEnv(v)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	if cfg.Service.Name == "" {
		cfg.Service.Name = "admin-service"
	}
	if cfg.Service.GRPCPort == 0 {
		cfg.Service.GRPCPort = 9114
	}
	if cfg.Postgres.DSN == "" {
		cfg.Postgres.DSN = "postgres://bbs_admin_app:local_admin_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_admin"
	}
	if len(cfg.RBAC.BootstrapAdminPrefixes) == 0 {
		cfg.RBAC.BootstrapAdminPrefixes = []string{"admin"}
	}
	if cfg.Auth.JWTSecret == "" {
		cfg.Auth.JWTSecret = "bbs-admin-local-dev-secret"
	}
	if cfg.Auth.JWTTTL == "" {
		cfg.Auth.JWTTTL = "168h"
	}
	if cfg.Auth.DefaultAdminPassword == "" {
		cfg.Auth.DefaultAdminPassword = "Admin123!"
	}
	if cfg.Auth.SecretEncryptionKey == "" {
		cfg.Auth.SecretEncryptionKey = cfg.Auth.JWTSecret
	}
	if cfg.Upstreams.User == "" {
		cfg.Upstreams.User = "127.0.0.1:9102"
	}
	if cfg.Upstreams.Reaction == "" {
		cfg.Upstreams.Reaction = "127.0.0.1:9105"
	}
	if cfg.Upstreams.Content == "" {
		cfg.Upstreams.Content = "127.0.0.1:9103"
	}
	if cfg.Upstreams.Comment == "" {
		cfg.Upstreams.Comment = "127.0.0.1:9104"
	}
	return &cfg, nil
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_ADMIN")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "service.name", "BBS_ADMIN_SERVICE_NAME")
	bindEnv(v, "service.grpcPort", "BBS_ADMIN_SERVICE_GRPC_PORT")
	bindEnv(v, "postgres.dsn", "BBS_ADMIN_POSTGRES_DSN")
	bindEnv(v, "postgres.debug", "BBS_ADMIN_POSTGRES_DEBUG")
	bindEnv(v, "auth.jwtSecret", "BBS_ADMIN_AUTH_JWT_SECRET")
	bindEnv(v, "auth.jwtTtl", "BBS_ADMIN_AUTH_JWT_TTL")
	bindEnv(v, "auth.defaultAdminPassword", "BBS_ADMIN_AUTH_DEFAULT_ADMIN_PASSWORD")
	bindEnv(v, "auth.secretEncryptionKey", "BBS_ADMIN_AUTH_SECRET_ENCRYPTION_KEY")
	bindEnv(v, "upstreams.user", "BBS_ADMIN_UPSTREAMS_USER")
	bindEnv(v, "upstreams.reaction", "BBS_ADMIN_UPSTREAMS_REACTION")
	bindEnv(v, "upstreams.content", "BBS_ADMIN_UPSTREAMS_CONTENT")
	bindEnv(v, "upstreams.comment", "BBS_ADMIN_UPSTREAMS_COMMENT")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}
