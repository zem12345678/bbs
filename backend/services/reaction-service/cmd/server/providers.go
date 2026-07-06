package server

import (
	"context"

	"reaction-service/internal/application/reaction/command"
	"reaction-service/internal/application/reaction/query"
	domain "reaction-service/internal/domain/reaction"
	"reaction-service/internal/infrastructure/messaging"
	"reaction-service/internal/infrastructure/store"
	reactiongrpc "reaction-service/internal/interfaces/grpc"
	"reaction-service/internal/support/config"
	"reaction-service/pkg/logger"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func provideLogger() logger.Logger {
	return logger.NewNopLogger()
}

func provideRedisClient(ctx context.Context, cfg *config.Config) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, DB: cfg.Redis.DB, Password: cfg.Redis.Password})
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, err
	}
	return rdb, nil
}

func provideDB(cfg *config.Config) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{})
}

func provideReportRepository(ctx context.Context, db *gorm.DB) (*store.PostgresReportRepository, error) {
	repo := store.NewPostgresReportRepository(db)
	return repo, repo.EnsureSchema(ctx)
}

func provideLikeRepository(ctx context.Context, db *gorm.DB) (*store.PostgresLikeRepository, error) {
	repo := store.NewPostgresLikeRepository(db)
	return repo, repo.EnsureSchema(ctx)
}

func provideFavoriteRepository(ctx context.Context, db *gorm.DB) (*store.PostgresFavoriteRepository, error) {
	repo := store.NewPostgresFavoriteRepository(db)
	return repo, repo.EnsureSchema(ctx)
}

type CacheWarmup struct{}

func provideCacheWarmup(ctx context.Context, cfg *config.Config, db *gorm.DB, rdb *redis.Client, _ *store.PostgresLikeRepository, _ *store.PostgresFavoriteRepository) (*CacheWarmup, error) {
	if !cfg.Reaction.RebuildCacheOnStart {
		return &CacheWarmup{}, nil
	}
	rebuilder := store.NewReactionCacheRebuilder(db, rdb)
	if _, err := rebuilder.Rebuild(ctx); err != nil {
		return nil, err
	}
	return &CacheWarmup{}, nil
}

func provideReactionStore(rdb *redis.Client) *store.RedisStore {
	return store.NewRedisStore(rdb)
}

func provideEventPublisher(cfg *config.Config, log logger.Logger) messaging.EventPublisher {
	return messaging.NewKafkaEventPublisher(cfg.Kafka.Brokers, cfg.Kafka.Topic, log)
}

func provideCommandService(
	reactionStore domain.Store,
	reports domain.ReportRepository,
	likes domain.LikeRepository,
	favorites domain.FavoriteRepository,
	publisher messaging.EventPublisher,
	log logger.Logger,
) *command.Service {
	return command.NewService(reactionStore, reports, likes, favorites, publisher, log)
}

func provideQueryService(reactionStore domain.Store, reports domain.ReportRepository, likes domain.LikeRepository, favorites domain.FavoriteRepository) *query.Service {
	return query.NewService(reactionStore, reports, likes, favorites)
}

func provideHandler(cmd *command.Service, qry *query.Service) *reactiongrpc.Handler {
	return reactiongrpc.NewHandler(cmd, qry)
}

var _ domain.Store = (*store.RedisStore)(nil)
var _ domain.ReportRepository = (*store.PostgresReportRepository)(nil)
var _ domain.LikeRepository = (*store.PostgresLikeRepository)(nil)
var _ domain.FavoriteRepository = (*store.PostgresFavoriteRepository)(nil)
