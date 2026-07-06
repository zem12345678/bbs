package config

import (
	"admin/internal/infrastructure/upstream"

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
	}
	Upstreams upstream.Options
}

func New(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
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
