//go:build wireinject

package server

import (
	"context"

	articlecommand "content-service/internal/application/article/command"
	topiccommand "content-service/internal/application/topic/command"
	articleDomain "content-service/internal/domain/article"
	categoryDomain "content-service/internal/domain/category"
	topicDomain "content-service/internal/domain/topic"
	"content-service/internal/infrastructure/persistence"
	"content-service/internal/support/config"
	"content-service/pkg/snowflake"

	"github.com/google/wire"
)

func InitializeServerApp(ctx context.Context, configPath string) (*App, error) {
	wire.Build(
		config.New,
		persistence.OpenDB,
		persistence.NewRepo,
		persistence.NewTopicRepo,
		persistence.NewCategoryRepo,
		provideLogger,
		provideRedisClient,
		provideArticleCache,
		provideSnowflakeNode,
		provideEventPublisher,
		provideArticleCommandService,
		provideArticleQueryService,
		provideTopicCommandService,
		provideTopicQueryService,
		provideCategoryQueryService,
		provideHandler,
		NewGRPCServer,
		NewApp,
		wire.Bind(new(articleDomain.Repository), new(*persistence.Repo)),
		wire.Bind(new(topicDomain.Repository), new(*persistence.TopicRepo)),
		wire.Bind(new(categoryDomain.Repository), new(*persistence.CategoryRepo)),
		wire.Bind(new(articlecommand.IDGenerator), new(*snowflake.Node)),
		wire.Bind(new(topiccommand.IDGenerator), new(*snowflake.Node)),
	)
	return nil, nil
}
