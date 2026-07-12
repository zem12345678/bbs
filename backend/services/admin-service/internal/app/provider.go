package app

import (
	"context"
	"strings"
	"time"

	adminapp "admin/internal/application/admin"
	"admin/internal/clients/upstream"
	adminauth "admin/internal/infrastructure/auth"
	"admin/internal/infrastructure/authz"
	"admin/internal/infrastructure/persistence"
	iocgrpc "admin/internal/ioc/grpc"
	"admin/pkg/logger"

	"github.com/google/wire"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func ProvideZapLogger(l logger.Logger) *zap.Logger { return l.GetZapLogger() }

func ProvideRepository(ctx context.Context, db *gorm.DB, v *viper.Viper) (*persistence.Repository, error) {
	repo := persistence.NewRepository(db)
	if err := repo.EnsureSchema(ctx); err != nil {
		return nil, err
	}
	if err := repo.SeedDefaults(ctx, BootstrapAdminPrefixes(v), StringDefault(v.GetString("auth.defaultAdminPassword"), "Admin123!")); err != nil {
		return nil, err
	}
	return repo, nil
}

func ProvideAuthorizer(ctx context.Context, repo *persistence.Repository, v *viper.Viper) (*authz.Authorizer, error) {
	return authz.NewAuthorizer(ctx, repo, BootstrapAdminPrefixes(v))
}

func ProvideUpstreams(client *iocgrpc.Client, v *viper.Viper) (*upstream.Clients, error) {
	return upstream.New(client, upstream.Options{
		User:     ServiceNameDefault(v.GetString("upstreams.user"), "bbs-user-service"),
		Reaction: ServiceNameDefault(v.GetString("upstreams.reaction"), "bbs-reaction-service"),
		Content:  ServiceNameDefault(v.GetString("upstreams.content"), "bbs-content-service"),
		Comment:  ServiceNameDefault(v.GetString("upstreams.comment"), "bbs-comment-service"),
	})
}

func ProvideTokenManager(v *viper.Viper) (*adminauth.TokenManager, error) {
	ttl, err := DurationDefault(v, "auth.jwtTtl", "168h")
	if err != nil {
		return nil, err
	}
	return adminauth.NewTokenManager(StringDefault(v.GetString("auth.jwtSecret"), "bbs-admin-local-dev-secret"), ttl), nil
}

func ProvideSecretCipher(v *viper.Viper) (*adminauth.SecretCipher, error) {
	secret := strings.TrimSpace(v.GetString("auth.secretEncryptionKey"))
	if secret == "" {
		secret = strings.TrimSpace(v.GetString("auth.jwtSecret"))
	}
	return adminauth.NewSecretCipher(StringDefault(secret, "bbs-admin-local-dev-secret"))
}

func ProvideAdminService(authorizer *authz.Authorizer, repo *persistence.Repository, passwords *adminauth.PasswordManager, tokens *adminauth.TokenManager, secrets *adminauth.SecretCipher, clients *upstream.Clients) *adminapp.Service {
	service := adminapp.NewService(authorizer, repo, repo, repo, repo, passwords, passwords, tokens, clients, clients, clients, clients)
	service.SetSettingSecretCipher(secrets)
	return service
}

func BootstrapAdminPrefixes(v *viper.Viper) []string {
	values := v.GetStringSlice("rbac.bootstrapAdminPrefixes")
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return []string{"admin"}
	}
	return out
}

func StringDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func ServiceNameDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	switch value {
	case "user-service":
		return "bbs-user-service"
	case "reaction-service":
		return "bbs-reaction-service"
	case "content-service":
		return "bbs-content-service"
	case "comment-service":
		return "bbs-comment-service"
	default:
		return value
	}
}

func DurationDefault(v *viper.Viper, key string, fallback string) (time.Duration, error) {
	raw := strings.TrimSpace(v.GetString(key))
	if raw != "" {
		return time.ParseDuration(raw)
	}
	if value := v.GetDuration(key); value > 0 {
		return value, nil
	}
	return time.ParseDuration(fallback)
}

var BusinessProviderSet = wire.NewSet(
	ProvideZapLogger,
	ProvideRepository,
	ProvideAuthorizer,
	ProvideUpstreams,
	adminauth.NewPasswordManager,
	ProvideTokenManager,
	ProvideSecretCipher,
	ProvideAdminService,
)
