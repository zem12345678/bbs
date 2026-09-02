package app

import (
	"context"

	accountcommand "reaction-service/internal/application/account"
	"reaction-service/internal/application/reaction/command"
	"reaction-service/internal/application/reaction/query"
	accountDomain "reaction-service/internal/domain/account"
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
	// DDL is performed by cmd/migrate before a service rollout.
	return store.NewPostgresReportRepository(db), nil
}

func ProvideLikeRepository(ctx context.Context, db *gorm.DB) (*store.PostgresLikeRepository, error) {
	return store.NewPostgresLikeRepository(db), nil
}

func ProvideReactionRepository(ctx context.Context, db *gorm.DB) (*store.PostgresReactionRepository, error) {
	return store.NewPostgresReactionRepository(db), nil
}

func ProvideFavoriteRepository(ctx context.Context, db *gorm.DB) (*store.PostgresFavoriteRepository, error) {
	return store.NewPostgresFavoriteRepository(db), nil
}

func ProvidePinRepository(ctx context.Context, db *gorm.DB) (*store.PostgresPinRepository, error) {
	return store.NewPostgresPinRepository(db), nil
}

func ProvideCollectionRepository(ctx context.Context, db *gorm.DB) (*store.PostgresCollectionRepository, error) {
	return store.NewPostgresCollectionRepository(db), nil
}

func ProvideAccountErasureRepository(ctx context.Context, db *gorm.DB) (*store.AccountErasureRepository, error) {
	return store.NewAccountErasureRepository(db), nil
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
	pins domain.PinRepository,
	collections domain.CollectionRepository,
	publisher messaging.EventPublisher,
	log logger.Logger,
	reactions domain.ReactionRepository,
) *command.Service {
	return command.NewService(reactionStore, reports, likes, favorites, pins, collections, publisher, log, reactions)
}

func ProvideQueryService(reactionStore domain.Store, reports domain.ReportRepository, likes domain.LikeRepository, favorites domain.FavoriteRepository, pins domain.PinRepository, collections domain.CollectionRepository, reactions domain.ReactionRepository) *query.Service {
	return query.NewService(reactionStore, reports, likes, favorites, pins, collections, reactions)
}

func ProvideAccountErasureService(repo accountDomain.ErasureRepository, cache accountDomain.ErasureCache) *accountcommand.Service {
	return accountcommand.NewService(repo, cache)
}

var BusinessProviderSet = wire.NewSet(
	ProvideZapLogger,
	ProvideReactionStore,
	ProvideReportRepository,
	ProvideLikeRepository,
	ProvideReactionRepository,
	ProvideFavoriteRepository,
	ProvidePinRepository,
	ProvideCollectionRepository,
	ProvideAccountErasureRepository,
	ProvideCacheWarmup,
	ProvideEventPublisher,
	ProvideCommandService,
	ProvideQueryService,
	ProvideAccountErasureService,
)

var _ domain.Store = (*store.RedisStore)(nil)
var _ domain.ReportRepository = (*store.PostgresReportRepository)(nil)
var _ domain.LikeRepository = (*store.PostgresLikeRepository)(nil)
var _ domain.ReactionRepository = (*store.PostgresReactionRepository)(nil)
var _ domain.FavoriteRepository = (*store.PostgresFavoriteRepository)(nil)
var _ domain.PinRepository = (*store.PostgresPinRepository)(nil)
var _ domain.CollectionRepository = (*store.PostgresCollectionRepository)(nil)
var _ accountDomain.ErasureRepository = (*store.AccountErasureRepository)(nil)
var _ accountDomain.ErasureCache = (*store.RedisStore)(nil)
