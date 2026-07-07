package app

import (
	"context"

	"reaction-service/internal/application/reaction/command"
	"reaction-service/internal/application/reaction/query"
	domain "reaction-service/internal/domain/reaction"
	"reaction-service/internal/infrastructure/messaging"
	"reaction-service/internal/infrastructure/store"
	"reaction-service/pkg/logger"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type CacheWarmup struct{}

func ProvideZapLogger(l logger.Logger) *zap.Logger { return l.GetZapLogger() }

func ProvideReactionStore(rdb *redis.Client) *store.RedisStore {
	return store.NewRedisStore(rdb)
}

func ProvideReportRepository(ctx context.Context, db *gorm.DB) (*store.PostgresReportRepository, error) {
	repo := store.NewPostgresReportRepository(db)
	return repo, repo.EnsureSchema(ctx)
}

func ProvideLikeRepository(ctx context.Context, db *gorm.DB) (*store.PostgresLikeRepository, error) {
	repo := store.NewPostgresLikeRepository(db)
	return repo, repo.EnsureSchema(ctx)
}

func ProvideFavoriteRepository(ctx context.Context, db *gorm.DB) (*store.PostgresFavoriteRepository, error) {
	repo := store.NewPostgresFavoriteRepository(db)
	return repo, repo.EnsureSchema(ctx)
}

func ProvideCacheWarmup(ctx context.Context, v *viper.Viper, db *gorm.DB, rdb *redis.Client, _ *store.PostgresLikeRepository, _ *store.PostgresFavoriteRepository) (*CacheWarmup, error) {
	if !v.GetBool("reaction.rebuildCacheOnStart") {
		return &CacheWarmup{}, nil
	}
	rebuilder := store.NewReactionCacheRebuilder(db, rdb)
	if _, err := rebuilder.Rebuild(ctx); err != nil {
		return nil, err
	}
	return &CacheWarmup{}, nil
}

func ProvideEventPublisher(writer *kafka.Writer, log logger.Logger) messaging.EventPublisher {
	return messaging.NewKafkaEventPublisher(writer, log)
}

func ProvideCommandService(
	reactionStore domain.Store,
	reports domain.ReportRepository,
	likes domain.LikeRepository,
	favorites domain.FavoriteRepository,
	publisher messaging.EventPublisher,
	log logger.Logger,
) *command.Service {
	return command.NewService(reactionStore, reports, likes, favorites, publisher, log)
}

func ProvideQueryService(reactionStore domain.Store, reports domain.ReportRepository, likes domain.LikeRepository, favorites domain.FavoriteRepository) *query.Service {
	return query.NewService(reactionStore, reports, likes, favorites)
}

var BusinessProviderSet = wire.NewSet(
	ProvideZapLogger,
	ProvideReactionStore,
	ProvideReportRepository,
	ProvideLikeRepository,
	ProvideFavoriteRepository,
	ProvideCacheWarmup,
	ProvideEventPublisher,
	ProvideCommandService,
	ProvideQueryService,
)

var _ domain.Store = (*store.RedisStore)(nil)
var _ domain.ReportRepository = (*store.PostgresReportRepository)(nil)
var _ domain.LikeRepository = (*store.PostgresLikeRepository)(nil)
var _ domain.FavoriteRepository = (*store.PostgresFavoriteRepository)(nil)
