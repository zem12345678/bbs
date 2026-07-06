//go:build wireinject

package server

import (
	"context"

	adminauth "admin/internal/infrastructure/auth"
	"admin/internal/infrastructure/persistence"
	admingrpc "admin/internal/interfaces/grpc"
	"admin/internal/interfaces/grpc/pb/adminpb"
	"admin/internal/support/config"

	"github.com/google/wire"
)

func InitializeServerApp(ctx context.Context, configPath string) (*App, error) {
	wire.Build(
		config.New,
		persistence.NewDB,
		provideRepository,
		provideAuthorizer,
		provideUpstreams,
		adminauth.NewPasswordManager,
		provideTokenManager,
		provideSecretCipher,
		provideAdminService,
		provideHandler,
		wire.Bind(new(adminpb.AdminServiceServer), new(*admingrpc.Handler)),
		NewGRPCServer,
		NewApp,
	)
	return nil, nil
}
