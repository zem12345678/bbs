package server

import (
	"context"

	articlecommand "content-service/internal/application/article/command"
	articlequery "content-service/internal/application/article/query"
	categoryquery "content-service/internal/application/category/query"
	topiccommand "content-service/internal/application/topic/command"
	topicquery "content-service/internal/application/topic/query"
	articleDomain "content-service/internal/domain/article"
	categoryDomain "content-service/internal/domain/category"
	topicDomain "content-service/internal/domain/topic"
	"content-service/internal/infrastructure/cache"
	"content-service/internal/infrastructure/messaging"
	"content-service/internal/infrastructure/persistence"
	contentgrpc "content-service/internal/interfaces/grpc"
	"content-service/internal/support/config"
	"content-service/pkg/logger"
	"content-service/pkg/snowflake"

	"github.com/redis/go-redis/v9"
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

func provideArticleCache(cfg *config.Config, rdb *redis.Client) *cache.ArticleCache {
	return cache.NewArticleCache(rdb, cfg.Cache.TTL)
}

func provideSnowflakeNode(cfg *config.Config) (*snowflake.Node, error) {
	return snowflake.NewNode(cfg.Snowflake.WorkerID)
}

func provideEventPublisher(cfg *config.Config, log logger.Logger) messaging.EventPublisher {
	return messaging.NewKafkaEventPublisher(cfg.Kafka.Brokers, cfg.Kafka.Topic, log)
}

func provideArticleCommandService(
	repo articleDomain.Repository,
	articleCache *cache.ArticleCache,
	idgen articlecommand.IDGenerator,
	publisher messaging.EventPublisher,
	log logger.Logger,
) *articlecommand.Service {
	return articlecommand.NewService(repo, articleCache, idgen, publisher, log)
}

func provideArticleQueryService(repo articleDomain.Repository, articleCache *cache.ArticleCache) *articlequery.Service {
	return articlequery.NewService(repo, articleCache)
}

func provideTopicCommandService(
	repo topicDomain.Repository,
	idgen topiccommand.IDGenerator,
	publisher messaging.EventPublisher,
	log logger.Logger,
) *topiccommand.Service {
	return topiccommand.NewService(repo, idgen, publisher, log)
}

func provideTopicQueryService(repo topicDomain.Repository) *topicquery.Service {
	return topicquery.NewService(repo)
}

func provideCategoryQueryService(repo categoryDomain.Repository) *categoryquery.Service {
	return categoryquery.NewService(repo)
}

func provideHandler(
	articleCmd *articlecommand.Service,
	articleQry *articlequery.Service,
	topicCmd *topiccommand.Service,
	topicQry *topicquery.Service,
	categoryQry *categoryquery.Service,
) *contentgrpc.Handler {
	return contentgrpc.NewHandler(articleCmd, articleQry, topicCmd, topicQry, categoryQry)
}

var _ articleDomain.Repository = (*persistence.Repo)(nil)
var _ topicDomain.Repository = (*persistence.TopicRepo)(nil)
var _ categoryDomain.Repository = (*persistence.CategoryRepo)(nil)
