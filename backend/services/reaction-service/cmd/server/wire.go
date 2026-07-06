//go:build wireinject

package server

import (
	"context"

	domain "reaction-service/internal/domain/reaction"
	"reaction-service/internal/infrastructure/store"
	"reaction-service/internal/support/config"

	"github.com/google/wire"
)

func InitializeServerApp(ctx context.Context, configPath string) (*App, error) {
	wire.Build(
		config.New,
		provideDB,
		provideRedisClient,
		provideReactionStore,
		provideReportRepository,
		provideLikeRepository,
		provideFavoriteRepository,
		provideCacheWarmup,
		provideLogger,
		provideEventPublisher,
		provideCommandService,
		provideQueryService,
		provideHandler,
		NewGRPCServer,
		NewApp,
		wire.Bind(new(domain.Store), new(*store.RedisStore)),
		wire.Bind(new(domain.ReportRepository), new(*store.PostgresReportRepository)),
		wire.Bind(new(domain.LikeRepository), new(*store.PostgresLikeRepository)),
		wire.Bind(new(domain.FavoriteRepository), new(*store.PostgresFavoriteRepository)),
	)
	return nil, nil
}
