package app

import (
	"context"
	"fmt"
	"strings"

	notificationservice "notification-service/internal/application/notification"
	userclient "notification-service/internal/clients/user"
	domain "notification-service/internal/domain/notification"
	"notification-service/internal/infrastructure/messaging"
	"notification-service/internal/infrastructure/persistence"
	"notification-service/internal/infrastructure/webhook"
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
	user      *userclient.Client
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

func ProvideWebhookConfig(v *viper.Viper) domain.WebhookConfig {
	return domain.WebhookConfig{
		Enabled:               v.GetBool("webhook.enabled"),
		ServerURL:             strings.TrimRight(strings.TrimSpace(v.GetString("webhook.serverURL")), "/"),
		AllowPrivateEndpoints: v.GetBool("webhook.allowPrivateEndpoints"),
	}
}

func ProvideNotificationService(repo domain.Repository, webPushConfig domain.WebPushConfig, webhookConfig domain.WebhookConfig) *notificationservice.Service {
	return notificationservice.NewService(repo, webPushConfig).SetWebhookConfig(webhookConfig)
}

func ProvideProjector(service *notificationservice.Service, user *userclient.Client) *messaging.Projector {
	return messaging.NewProjector(service, user)
}

func ProvideWebPushDispatcher(repo *persistence.PostgresRepository, config domain.WebPushConfig, log logger.Logger) *webpush.Dispatcher {
	if !config.Enabled {
		return nil
	}
	return webpush.NewDispatcher(repo, webpush.NewSender(config), log)
}

func ProvideWebhookDispatcher(repo *persistence.PostgresRepository, config domain.WebhookConfig, log logger.Logger) *webhook.Dispatcher {
	if !config.Enabled {
		return nil
	}
	return webhook.NewDispatcher(repo, webhook.NewSender(config), log)
}

func ProvideConsumerRunner(v *viper.Viper, kafkaOptions *iockafka.ConsumerOptions, projector *messaging.Projector, user *userclient.Client, log logger.Logger) (*ConsumerRunner, error) {
	articleReader, err := iockafka.NewConsumer(kafkaOptions.WithTopic(
		StringDefault(v.GetString("kafka.articleTopic"), "article.events"),
		StringDefault(v.GetString("kafka.articleGroupId"), "bbs-notification-article-consumer"),
	))
	if err != nil {
		_ = user.Close()
		return nil, err
	}
	userReader, err := iockafka.NewConsumer(kafkaOptions.WithTopic(
		StringDefault(v.GetString("kafka.userTopic"), "user.events"),
		StringDefault(v.GetString("kafka.userGroupId"), "bbs-notification-user-consumer"),
	))
	if err != nil {
		_ = articleReader.Close()
		_ = user.Close()
		return nil, err
	}
	commentReader, err := iockafka.NewConsumer(kafkaOptions.WithTopic(
		StringDefault(v.GetString("kafka.commentTopic"), "comment.events"),
		StringDefault(v.GetString("kafka.commentGroupId"), "bbs-notification-comment-consumer"),
	))
	if err != nil {
		_ = articleReader.Close()
		_ = userReader.Close()
		_ = user.Close()
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
		_ = user.Close()
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
		_ = user.Close()
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
	return &ConsumerRunner{ctx: ctx, cancel: cancel, consumers: consumers, user: user, log: log}, nil
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
	if r.user != nil {
		if err := r.user.Close(); err != nil && firstErr == nil {
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
	ProvideWebhookConfig,
	ProvideNotificationService,
	userclient.NewClient,
	ProvideProjector,
	ProvideWebPushDispatcher,
	ProvideWebhookDispatcher,
	ProvideConsumerRunner,
)

var _ domain.Repository = (*persistence.PostgresRepository)(nil)
