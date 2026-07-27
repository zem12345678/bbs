package app

import (
	"context"
	"errors"
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
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func ProvideZapLogger(l logger.Logger) *zap.Logger { return l.GetZapLogger() }

func ProvideRepository(ctx context.Context, db *gorm.DB, v *viper.Viper) (*persistence.Repository, error) {
	// Schema creation and bootstrap data are deliberately handled by the
	// explicit migrate command. A regular server instance must be safe to run
	// with a DML-only database role and alongside other replicas.
	return persistence.NewRepository(db), nil
}

func ProvideAuthorizer(ctx context.Context, repo *persistence.Repository, v *viper.Viper) (*authz.Authorizer, error) {
	return authz.NewAuthorizer(ctx, repo)
}

func ProvideUpstreams(client *iocgrpc.Client, v *viper.Viper) (*upstream.Clients, error) {
	return upstream.New(client, upstream.Options{
		User:                          ServiceNameDefault(v.GetString("upstreams.user"), "bbs-user-service"),
		UserInternalAuthToken:         StringDefault(v.GetString("upstreams.userInternalAuthToken"), "bbs-local-user-internal-token"),
		Reaction:                      ServiceNameDefault(v.GetString("upstreams.reaction"), "bbs-reaction-service"),
		ReactionInternalAuthToken:     StringDefault(v.GetString("upstreams.reactionInternalAuthToken"), "bbs-local-reaction-internal-token"),
		Content:                       ServiceNameDefault(v.GetString("upstreams.content"), "bbs-content-service"),
		ContentInternalAuthToken:      StringDefault(v.GetString("upstreams.contentInternalAuthToken"), "bbs-local-content-internal-token"),
		Comment:                       ServiceNameDefault(v.GetString("upstreams.comment"), "bbs-comment-service"),
		CommentInternalAuthToken:      StringDefault(v.GetString("upstreams.commentInternalAuthToken"), "bbs-local-comment-internal-token"),
		Notification:                  ServiceNameDefault(v.GetString("upstreams.notification"), "bbs-notification-service"),
		NotificationInternalAuthToken: StringDefault(v.GetString("upstreams.notificationInternalAuthToken"), "bbs-local-notification-internal-token"),
		Search:                        ServiceNameDefault(v.GetString("upstreams.search"), "bbs-search-service"),
		SearchInternalAuthToken:       StringDefault(v.GetString("upstreams.searchInternalAuthToken"), "bbs-local-search-internal-token"),
	})
}

func ProvideSearchRebuildGateway(clients *upstream.Clients, redisClient *redis.Client) *upstream.SearchRebuilder {
	return upstream.NewRedisSearchRebuilder(clients, redisClient)
}

func ProvideTokenManager(v *viper.Viper) (*adminauth.TokenManager, error) {
	accessTTL, err := DurationDefault(v, "auth.jwtTtl", "168h")
	if err != nil {
		return nil, err
	}
	refreshTTL, err := DurationDefault(v, "auth.refreshTtl", "720h")
	if err != nil {
		return nil, err
	}
	if accessTTL <= 0 || refreshTTL <= accessTTL {
		return nil, errors.New("auth.refreshTtl must be greater than auth.jwtTtl and both must be positive")
	}
	secret := strings.TrimSpace(v.GetString("auth.jwtSecret"))
	if secret == "" {
		return nil, errors.New("auth.jwtSecret is required")
	}
	return adminauth.NewTokenManager(secret, accessTTL, refreshTTL), nil
}

func ProvideSecretCipher(v *viper.Viper) (*adminauth.SecretCipher, error) {
	secret := strings.TrimSpace(v.GetString("auth.secretEncryptionKey"))
	if secret == "" {
		return nil, errors.New("auth.secretEncryptionKey is required")
	}
	return adminauth.NewSecretCipher(secret)
}

func ProvideAdminService(authorizer *authz.Authorizer, repo *persistence.Repository, passwords *adminauth.PasswordManager, tokens *adminauth.TokenManager, secrets *adminauth.SecretCipher, clients *upstream.Clients) *adminapp.Service {
	service := adminapp.NewService(authorizer, repo, repo, repo, repo, passwords, passwords, tokens, clients, clients, clients, clients)
	service.SetSettingSecretCipher(secrets)
	service.SetSystemNotificationGateway(clients)
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
	case "notification-service":
		return "bbs-notification-service"
	case "search-service":
		return "bbs-search-service"
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
	ProvideSearchRebuildGateway,
	adminauth.NewPasswordManager,
	ProvideTokenManager,
	ProvideSecretCipher,
	ProvideAdminService,
)
