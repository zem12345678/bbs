package server

import (
	"context"
	"time"

	app "admin/internal/application/admin"
	adminauth "admin/internal/infrastructure/auth"
	"admin/internal/infrastructure/authz"
	"admin/internal/infrastructure/persistence"
	"admin/internal/infrastructure/upstream"
	admingrpc "admin/internal/interfaces/grpc"
	"admin/internal/support/config"
)

func provideRepository(ctx context.Context, db *persistence.DB, cfg *config.Config) (*persistence.Repository, error) {
	repo := persistence.NewRepository(db.Gorm())
	if err := repo.EnsureSchema(ctx); err != nil {
		return nil, err
	}
	if err := repo.SeedDefaults(ctx, cfg.RBAC.BootstrapAdminPrefixes, cfg.Auth.DefaultAdminPassword); err != nil {
		return nil, err
	}
	return repo, nil
}

func provideAuthorizer(ctx context.Context, repo *persistence.Repository, cfg *config.Config) (*authz.Authorizer, error) {
	return authz.NewAuthorizer(ctx, repo, cfg.RBAC.BootstrapAdminPrefixes)
}

func provideUpstreams(cfg *config.Config) (*upstream.Clients, error) {
	return upstream.New(cfg.Upstreams)
}

func provideTokenManager(cfg *config.Config) (*adminauth.TokenManager, error) {
	ttl, err := time.ParseDuration(cfg.Auth.JWTTTL)
	if err != nil {
		return nil, err
	}
	return adminauth.NewTokenManager(cfg.Auth.JWTSecret, ttl), nil
}

func provideAdminService(authorizer *authz.Authorizer, repo *persistence.Repository, passwords *adminauth.PasswordManager, tokens *adminauth.TokenManager, clients *upstream.Clients) *app.Service {
	return app.NewService(authorizer, repo, repo, repo, repo, passwords, passwords, tokens, clients, clients, clients, clients)
}

func provideHandler(service *app.Service) *admingrpc.Handler {
	return admingrpc.NewHandler(service)
}
