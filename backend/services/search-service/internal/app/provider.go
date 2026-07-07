package app

import (
	"context"
	"strings"

	"search-service/internal/application/search/command"
	"search-service/internal/application/search/query"
	searches "search-service/internal/infrastructure/elasticsearch"
	"search-service/internal/infrastructure/messaging"
	ioces "search-service/internal/ioc/es"
	iockafka "search-service/internal/ioc/kafka"
	"search-service/pkg/logger"

	elastic "github.com/elastic/go-elasticsearch/v9"
	"github.com/google/wire"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type EventConsumer interface {
	Start(ctx context.Context) error
	Close() error
}

type EventConsumerRunner struct {
	consumers []EventConsumer
	log       logger.Logger
}

func ProvideZapLogger(l logger.Logger) *zap.Logger { return l.GetZapLogger() }

func ProvideSearchRepository(esClient *elastic.Client, esOptions *ioces.Options) *searches.ArticleRepository {
	return searches.NewArticleRepository(esClient, esOptions.Indices.Articles, esOptions.Indices.Topics)
}

func ProvideCommandService(repo *searches.ArticleRepository) *command.Service {
	return command.NewService(repo)
}

func ProvideQueryService(repo *searches.ArticleRepository) *query.Service {
	return query.NewService(repo)
}

func ProvideEventConsumerRunner(v *viper.Viper, kafkaOptions *iockafka.ConsumerOptions, repo *searches.ArticleRepository, log logger.Logger) (*EventConsumerRunner, error) {
	articleReader, err := iockafka.NewConsumer(kafkaOptions.WithTopic(
		StringDefault(v.GetString("kafka.articleTopic"), "article.events"),
		StringDefault(v.GetString("kafka.articleGroupId"), StringDefault(v.GetString("kafka.groupId"), "bbs-search-indexer")),
	))
	if err != nil {
		return nil, err
	}
	commentReader, err := iockafka.NewConsumer(kafkaOptions.WithTopic(
		StringDefault(v.GetString("kafka.commentTopic"), "comment.events"),
		StringDefault(v.GetString("kafka.commentGroupId"), "bbs-search-comment-counter"),
	))
	if err != nil {
		_ = articleReader.Close()
		return nil, err
	}
	reactionReader, err := iockafka.NewConsumer(kafkaOptions.WithTopic(
		StringDefault(v.GetString("kafka.reactionTopic"), "reaction.events"),
		StringDefault(v.GetString("kafka.reactionGroupId"), "bbs-search-reaction-counter"),
	))
	if err != nil {
		_ = articleReader.Close()
		_ = commentReader.Close()
		return nil, err
	}
	articleConsumer := messaging.NewArticleConsumer(articleReader, repo, log)
	commentConsumer := messaging.NewCommentConsumer(commentReader, repo, log)
	reactionConsumer := messaging.NewReactionConsumer(reactionReader, repo, log)
	return &EventConsumerRunner{
		consumers: []EventConsumer{articleConsumer, commentConsumer, reactionConsumer},
		log:       log,
	}, nil
}

func (r *EventConsumerRunner) Start(ctx context.Context) {
	for _, consumer := range r.consumers {
		consumer := consumer
		go func() {
			if err := consumer.Start(ctx); err != nil && r.log != nil {
				r.log.Warn("event consumer stopped", logger.Error(err))
			}
		}()
	}
}

func StringDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func StringSliceDefault(values []string, fallback []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

var BusinessProviderSet = wire.NewSet(
	ProvideZapLogger,
	ProvideSearchRepository,
	ProvideCommandService,
	ProvideQueryService,
	ProvideEventConsumerRunner,
)
