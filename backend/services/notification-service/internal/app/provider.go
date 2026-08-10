package app

import (
	"context"
	"fmt"
	"strings"

	notificationservice "notification-service/internal/application/notification"
	domain "notification-service/internal/domain/notification"
	"notification-service/internal/infrastructure/messaging"
	"notification-service/internal/infrastructure/persistence"
	"notification-service/internal/infrastructure/webpush"
	datasource "notification-service/internal/ioc/db/postgres"
	iockafka "notification-service/internal/ioc/kafka"
	"notification-service/pkg/logger"

	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
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

func ProvidePostgresPool(ctx context.Context, options *datasource.Options) (*pgxpool.Pool, error) {
	return datasource.NewPool(ctx, options)
}

func ProvideRepository(ctx context.Context, pool *pgxpool.Pool) (*persistence.PostgresRepository, error) {
	// A server process only consumes an already-migrated schema.
	return persistence.NewPostgresRepository(pool), nil
}

func ProvideWebPushConfig(v *viper.Viper) domain.WebPushConfig {
	return domain.WebPushConfig{
		Enabled:    v.GetBool("webPush.enabled"),
		Subject:    strings.TrimSpace(v.GetString("webPush.subject")),
		PublicKey:  strings.TrimSpace(v.GetString("webPush.publicKey")),
		PrivateKey: strings.TrimSpace(v.GetString("webPush.privateKey")),
	}
}

func ProvideNotificationService(repo domain.Repository, webPushConfig domain.WebPushConfig) *notificationservice.Service {
	return notificationservice.NewService(repo, webPushConfig)
}

func ProvideProjector(service *notificationservice.Service) *messaging.Projector {
	return messaging.NewProjector(service)
}

func ProvideWebPushDispatcher(repo *persistence.PostgresRepository, config domain.WebPushConfig, log logger.Logger) *webpush.Dispatcher {
	if !config.Enabled {
		return nil
	}
	return webpush.NewDispatcher(repo, webpush.NewSender(config), log)
}

func ProvideConsumerRunner(v *viper.Viper, kafkaOptions *iockafka.ConsumerOptions, projector *messaging.Projector, log logger.Logger) (*ConsumerRunner, error) {
	articleReader, err := iockafka.NewConsumer(kafkaOptions.WithTopic(
		StringDefault(v.GetString("kafka.articleTopic"), "article.events"),
		StringDefault(v.GetString("kafka.articleGroupId"), "bbs-notification-article-consumer"),
	))
	if err != nil {
		return nil, err
	}
	userReader, err := iockafka.NewConsumer(kafkaOptions.WithTopic(
		StringDefault(v.GetString("kafka.userTopic"), "user.events"),
		StringDefault(v.GetString("kafka.userGroupId"), "bbs-notification-user-consumer"),
	))
	if err != nil {
		_ = articleReader.Close()
		return nil, err
	}
	commentReader, err := iockafka.NewConsumer(kafkaOptions.WithTopic(
		StringDefault(v.GetString("kafka.commentTopic"), "comment.events"),
		StringDefault(v.GetString("kafka.commentGroupId"), "bbs-notification-comment-consumer"),
	))
	if err != nil {
		_ = articleReader.Close()
		_ = userReader.Close()
		return nil, err
	}
	reactionReader, err := iockafka.NewConsumer(kafkaOptions.WithTopic(
		StringDefault(v.GetString("kafka.reactionTopic"), "reaction.events"),
		StringDefault(v.GetString("kafka.reactionGroupId"), "bbs-notification-reaction-consumer"),
	))
	if err != nil {
		_ = articleReader.Close()
		_ = userReader.Close()
		_ = commentReader.Close()
		return nil, err
	}
	mallReader, err := iockafka.NewConsumer(kafkaOptions.WithTopic(
		StringDefault(v.GetString("kafka.mallTopic"), "mall.events"),
		StringDefault(v.GetString("kafka.mallGroupId"), "bbs-notification-mall-consumer"),
	))
	if err != nil {
		_ = articleReader.Close()
		_ = userReader.Close()
		_ = commentReader.Close()
		_ = reactionReader.Close()
		return nil, err
	}
	consumers := []eventConsumer{
		messaging.NewConsumer(articleReader, "article", projector.HandleArticle, log),
		messaging.NewConsumer(userReader, "user", projector.HandleUser, log),
		messaging.NewConsumer(commentReader, "comment", projector.HandleComment, log),
		messaging.NewConsumer(reactionReader, "reaction", projector.HandleReaction, log),
		messaging.NewConsumer(mallReader, "mall", projector.HandleMall, log),
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
		return fmt.Errorf("close notification consumer: %w", firstErr)
	}
	return nil
}

var BusinessProviderSet = wire.NewSet(
	ProvideZapLogger,
	ProvidePostgresPool,
	ProvideRepository,
	ProvideWebPushConfig,
	ProvideNotificationService,
	ProvideProjector,
	ProvideWebPushDispatcher,
	ProvideConsumerRunner,
)

var _ domain.Repository = (*persistence.PostgresRepository)(nil)
