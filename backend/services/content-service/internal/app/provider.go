package app

import (
	"time"

	articlecommand "content-service/internal/application/article/command"
	articlequery "content-service/internal/application/article/query"
	categorycommand "content-service/internal/application/category/command"
	categoryquery "content-service/internal/application/category/query"
	topiccommand "content-service/internal/application/topic/command"
	topicquery "content-service/internal/application/topic/query"
	articleDomain "content-service/internal/domain/article"
	categoryDomain "content-service/internal/domain/category"
	topicDomain "content-service/internal/domain/topic"
	"content-service/internal/infrastructure/cache"
	"content-service/internal/infrastructure/messaging"
	"content-service/internal/infrastructure/persistence"
	"content-service/pkg/logger"
	"content-service/pkg/snowflake"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func ProvideZapLogger(l logger.Logger) *zap.Logger { return l.GetZapLogger() }

func ProvideArticleRepository(db *gorm.DB) *persistence.Repo {
	return persistence.NewRepo(db)
}

func ProvideTopicRepository(db *gorm.DB) *persistence.TopicRepo {
	return persistence.NewTopicRepo(db)
}

func ProvideCategoryRepository(db *gorm.DB) *persistence.CategoryRepo {
	return persistence.NewCategoryRepo(db)
}

func ProvideArticleCache(v *viper.Viper, rdb *redis.Client) *cache.ArticleCache {
	ttl := v.GetDuration("cache.ttl")
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return cache.NewArticleCache(rdb, ttl)
}

func ProvideSnowflakeNode(v *viper.Viper) (*snowflake.Node, error) {
	workerID := v.GetInt64("snowflake.workerId")
	if workerID == 0 {
		workerID = 3
	}
	return snowflake.NewNode(workerID)
}

func ProvideEventPublisher(writer *kafka.Writer, log logger.Logger) messaging.EventPublisher {
	return messaging.NewKafkaEventPublisher(writer, log)
}

func ProvideArticleCommandService(
	repo articleDomain.Repository,
	articleCache *cache.ArticleCache,
	idgen articlecommand.IDGenerator,
	publisher messaging.EventPublisher,
	log logger.Logger,
) *articlecommand.Service {
	return articlecommand.NewService(repo, articleCache, idgen, publisher, log)
}

func ProvideArticleQueryService(repo articleDomain.Repository, articleCache *cache.ArticleCache) *articlequery.Service {
	return articlequery.NewService(repo, articleCache)
}

func ProvideTopicCommandService(
	repo topicDomain.Repository,
	idgen topiccommand.IDGenerator,
	publisher messaging.EventPublisher,
	log logger.Logger,
) *topiccommand.Service {
	return topiccommand.NewService(repo, idgen, publisher, log)
}

func ProvideTopicQueryService(repo topicDomain.Repository) *topicquery.Service {
	return topicquery.NewService(repo)
}

func ProvideCategoryCommandService(repo categoryDomain.Repository, idgen categorycommand.IDGenerator) *categorycommand.Service {
	return categorycommand.NewService(repo, idgen)
}

func ProvideCategoryQueryService(repo categoryDomain.Repository) *categoryquery.Service {
	return categoryquery.NewService(repo)
}

var BusinessProviderSet = wire.NewSet(
	ProvideZapLogger,
	ProvideArticleRepository,
	ProvideTopicRepository,
	ProvideCategoryRepository,
	ProvideArticleCache,
	ProvideSnowflakeNode,
	ProvideEventPublisher,
	ProvideArticleCommandService,
	ProvideArticleQueryService,
	ProvideTopicCommandService,
	ProvideTopicQueryService,
	ProvideCategoryCommandService,
	ProvideCategoryQueryService,
)

var _ articleDomain.Repository = (*persistence.Repo)(nil)
var _ topicDomain.Repository = (*persistence.TopicRepo)(nil)
var _ categoryDomain.Repository = (*persistence.CategoryRepo)(nil)
var _ articlecommand.IDGenerator = (*snowflake.Node)(nil)
var _ topiccommand.IDGenerator = (*snowflake.Node)(nil)
var _ categorycommand.IDGenerator = (*snowflake.Node)(nil)
