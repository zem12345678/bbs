package app

import (
	"context"
	"fmt"

	feedquery "feed-service/internal/application/feed/query"
	domain "feed-service/internal/domain/feed"
	"feed-service/internal/infrastructure/messaging"
	"feed-service/internal/infrastructure/persistence"
	iockafka "feed-service/internal/ioc/kafka"
	"feed-service/pkg/logger"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type eventConsumer interface {
	Start(context.Context) error
	Close() error
}

type ConsumerRunner struct {
	ctx       context.Context
	cancel    context.CancelFunc
	consumers []eventConsumer
	log       logger.Logger
}

func ProvideZapLogger(l logger.Logger) *zap.Logger { return l.GetZapLogger() }

func ProvideFeedRepository(rdb *redis.Client) *persistence.RedisRepository {
	return persistence.NewRedisRepository(rdb)
}

func ProvideQueryService(repo domain.Repository) *feedquery.Service {
	return feedquery.NewService(repo)
}

func ProvideConsumerRunner(v *viper.Viper, kafkaOptions *iockafka.ConsumerOptions, repo *persistence.RedisRepository, log logger.Logger) (*ConsumerRunner, error) {
	articleReader, err := iockafka.NewConsumer(kafkaOptions.WithTopic(
		StringDefault(v.GetString("kafka.articleTopic"), "article.events"),
		StringDefault(v.GetString("kafka.articleGroupId"), "bbs-feed-article-projector"),
	))
	if err != nil {
		return nil, err
	}
	commentReader, err := iockafka.NewConsumer(kafkaOptions.WithTopic(
		StringDefault(v.GetString("kafka.commentTopic"), "comment.events"),
		StringDefault(v.GetString("kafka.commentGroupId"), "bbs-feed-comment-projector"),
	))
	if err != nil {
		_ = articleReader.Close()
		return nil, err
	}
	reactionReader, err := iockafka.NewConsumer(kafkaOptions.WithTopic(
		StringDefault(v.GetString("kafka.reactionTopic"), "reaction.events"),
		StringDefault(v.GetString("kafka.reactionGroupId"), "bbs-feed-reaction-projector"),
	))
	if err != nil {
		_ = articleReader.Close()
		_ = commentReader.Close()
		return nil, err
	}
	consumers := []eventConsumer{
		messaging.NewArticleConsumer(articleReader, repo, log),
		messaging.NewCommentConsumer(commentReader, repo, log),
		messaging.NewReactionConsumer(reactionReader, repo, log),
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &ConsumerRunner{ctx: ctx, cancel: cancel, consumers: consumers, log: log}, nil
}

func (r *ConsumerRunner) Start() error {
	for _, consumer := range r.consumers {
		consumer := consumer
		go func() {
			if err := consumer.Start(r.ctx); err != nil {
				r.log.Error("event consumer stopped", logger.Error(err))
			}
		}()
	}
	return nil
}

func (r *ConsumerRunner) Stop() error {
	r.cancel()
	var firstErr error
	for _, consumer := range r.consumers {
		if err := consumer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return fmt.Errorf("close feed consumer: %w", firstErr)
	}
	return nil
}

var BusinessProviderSet = wire.NewSet(
	ProvideZapLogger,
	ProvideFeedRepository,
	ProvideQueryService,
	ProvideConsumerRunner,
)

var _ domain.Repository = (*persistence.RedisRepository)(nil)
