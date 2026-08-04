package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	accountapp "chat-service/internal/application/account"
	chatapp "chat-service/internal/application/chat"
	domain "chat-service/internal/domain/chat"
	"chat-service/internal/infrastructure/messaging"
	"chat-service/internal/infrastructure/persistence"
	datasource "chat-service/internal/ioc/db/postgres"
	iockafka "chat-service/internal/ioc/kafka"
	"chat-service/pkg/logger"
	"chat-service/pkg/snowflake"

	"github.com/google/uuid"
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type RuntimeRunner struct {
	ctx       context.Context
	cancel    context.CancelFunc
	outbox    *chatapp.OutboxDispatcher
	realtime  *messaging.RealtimeDispatcher
	publisher *messaging.KafkaOutboxPublisher
	redis     *redis.Client
	pool      *pgxpool.Pool
	logger    *zap.Logger
	startOnce sync.Once
	stopOnce  sync.Once
	waitGroup sync.WaitGroup
	stopErr   error
}

func ProvideZapLogger(l logger.Logger) *zap.Logger { return l.GetZapLogger() }

func ProvidePostgresPool(ctx context.Context, options *datasource.Options) (*pgxpool.Pool, error) {
	return datasource.NewPool(ctx, options)
}

func ProvideRepository(pool *pgxpool.Pool) *persistence.PostgresRepository {
	return persistence.NewPostgresRepository(pool)
}

func ProvideSnowflakeGenerator(v *viper.Viper) (*snowflake.Generator, error) {
	workerID, err := snowflake.ResolveWorkerID(
		v.GetInt64("snowflake.workerId"),
		v.GetInt64("snowflake.workerIdRangeStart"),
		v.GetInt64("snowflake.workerIdRangeSize"),
		v.GetString("snowflake.instanceName"),
	)
	if err != nil {
		return nil, err
	}
	return snowflake.New(workerID)
}

func ProvideChatService(repo domain.Repository, ids chatapp.IDGenerator) *chatapp.Service {
	return chatapp.NewService(repo, ids)
}

func ProvideAccountErasureService(repo domain.AccountErasureRepository) *accountapp.Service {
	return accountapp.NewService(repo)
}

func ProvideOutboxPublisher(writer *kafka.Writer) *messaging.KafkaOutboxPublisher {
	return messaging.NewKafkaOutboxPublisher(writer)
}

func ProvideOutboxDispatcher(
	repo domain.OutboxRepository,
	publisher domain.OutboxPublisher,
	v *viper.Viper,
	logger *zap.Logger,
) *chatapp.OutboxDispatcher {
	return chatapp.NewOutboxDispatcher(repo, publisher, chatapp.OutboxDispatcherOptions{
		Owner:          StringDefault(v.GetString("outbox.owner"), "bbs-chat-service") + ":" + uuid.NewString(),
		BatchSize:      v.GetInt("outbox.batchSize"),
		LeaseDuration:  v.GetDuration("outbox.leaseDuration"),
		Interval:       v.GetDuration("outbox.interval"),
		RetryDelay:     v.GetDuration("outbox.retryDelay"),
		PublishTimeout: v.GetDuration("outbox.publishTimeout"),
		Logger:         logger,
	})
}

func ProvideRealtimeDispatcher(
	v *viper.Viper,
	consumerOptions *iockafka.ConsumerOptions,
	redisClient *redis.Client,
	logger *zap.Logger,
) (*messaging.RealtimeDispatcher, error) {
	reader, err := iockafka.NewConsumer(consumerOptions.WithTopic(
		StringDefault(v.GetString("kafka.topic"), "chat.events"),
		StringDefault(v.GetString("kafka.realtimeGroupId"), "bbs-chat-realtime"),
	))
	if err != nil {
		return nil, err
	}
	return messaging.NewRealtimeDispatcher(
		reader,
		messaging.NewRedisRealtimePublisher(redisClient),
		v.GetDuration("outbox.retryDelay"),
		logger,
	), nil
}

func ProvideRuntimeRunner(
	outbox *chatapp.OutboxDispatcher,
	realtime *messaging.RealtimeDispatcher,
	publisher *messaging.KafkaOutboxPublisher,
	redisClient *redis.Client,
	pool *pgxpool.Pool,
	logger *zap.Logger,
) *RuntimeRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &RuntimeRunner{
		ctx: ctx, cancel: cancel, outbox: outbox, realtime: realtime,
		publisher: publisher, redis: redisClient, pool: pool, logger: logger,
	}
}

func (r *RuntimeRunner) Start() error {
	r.startOnce.Do(func() {
		if r.outbox != nil {
			r.waitGroup.Add(1)
			go func() {
				defer r.waitGroup.Done()
				r.outbox.Run(r.ctx)
			}()
		}
		if r.realtime != nil {
			r.waitGroup.Add(1)
			go func() {
				defer r.waitGroup.Done()
				if err := r.realtime.Run(r.ctx); err != nil && r.ctx.Err() == nil {
					r.logger.Error("chat realtime dispatcher stopped", zap.Error(err))
				}
			}()
		}
	})
	return nil
}

func (r *RuntimeRunner) Stop() error {
	r.stopOnce.Do(func() {
		r.cancel()
		var closeErrors []error
		if r.realtime != nil {
			if err := r.realtime.Close(); err != nil {
				closeErrors = append(closeErrors, fmt.Errorf("close chat realtime consumer: %w", err))
			}
		}
		r.waitGroup.Wait()
		if r.publisher != nil {
			if err := r.publisher.Close(); err != nil {
				closeErrors = append(closeErrors, fmt.Errorf("close chat outbox publisher: %w", err))
			}
		}
		if r.redis != nil {
			if err := r.redis.Close(); err != nil {
				closeErrors = append(closeErrors, fmt.Errorf("close chat redis: %w", err))
			}
		}
		if r.pool != nil {
			r.pool.Close()
		}
		r.stopErr = errors.Join(closeErrors...)
	})
	return r.stopErr
}

var BusinessProviderSet = wire.NewSet(
	ProvideZapLogger,
	ProvidePostgresPool,
	ProvideRepository,
	ProvideSnowflakeGenerator,
	ProvideChatService,
	ProvideAccountErasureService,
	ProvideOutboxPublisher,
	ProvideOutboxDispatcher,
	ProvideRealtimeDispatcher,
	ProvideRuntimeRunner,
)

var _ domain.Repository = (*persistence.PostgresRepository)(nil)
var _ domain.AccountErasureRepository = (*persistence.PostgresRepository)(nil)
var _ domain.OutboxRepository = (*persistence.PostgresRepository)(nil)
var _ chatapp.IDGenerator = (*snowflake.Generator)(nil)
