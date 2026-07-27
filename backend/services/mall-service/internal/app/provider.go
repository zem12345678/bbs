package app

import (
	"context"
	"fmt"

	mallapp "mall-service/internal/application/mall"
	creditclient "mall-service/internal/clients/credit"
	domain "mall-service/internal/domain/mall"
	"mall-service/internal/infrastructure/messaging"
	"mall-service/internal/infrastructure/persistence"
	datasource "mall-service/internal/ioc/db/postgres"
	iocgrpc "mall-service/internal/ioc/grpc"
	"mall-service/pkg/logger"

	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type OutboxRunner struct {
	ctx        context.Context
	cancel     context.CancelFunc
	dispatcher *mallapp.OutboxDispatcher
	publisher  interface{ Close() error }
}

type ExpiredOrderRunner struct {
	ctx    context.Context
	cancel context.CancelFunc
	closer *mallapp.ExpiredOrderCloser
}

func ProvideZapLogger(l logger.Logger) *zap.Logger { return l.GetZapLogger() }

func ProvidePostgresPool(ctx context.Context, options *datasource.Options) (*pgxpool.Pool, error) {
	return datasource.NewPool(ctx, options)
}

func ProvideRepository(ctx context.Context, pool *pgxpool.Pool) (*persistence.PostgresRepository, error) {
	// Schema changes run through cmd/migrate before the deployment rolls out.
	return persistence.NewPostgresRepository(pool), nil
}

func ProvideCreditCharger(grpcClient *iocgrpc.Client, v *viper.Viper) (mallapp.CreditCharger, error) {
	return creditclient.NewClient(grpcClient, v)
}

func ProvideMallService(repo domain.Repository, charger mallapp.CreditCharger, v *viper.Viper) *mallapp.Service {
	return mallapp.NewService(repo, charger, v.GetDuration("order.expireAfter"))
}

func ProvideOutboxPublisher(writer *kafka.Writer, v *viper.Viper) domain.OutboxPublisher {
	topic := StringDefault(v.GetString("kafka.mallTopic"), StringDefault(v.GetString("kafka.topic"), "mall.events"))
	return messaging.NewKafkaOutboxPublisher(writer, mallOutboxTopics(topic))
}

func mallOutboxTopics(topic string) map[string]string {
	return map[string]string{
		mallapp.OrderPaidEventType:          topic,
		mallapp.OrderShippedEventType:       topic,
		mallapp.OrderCompletedEventType:     topic,
		mallapp.RefundApprovedEventType:     topic,
		mallapp.RefundRejectedEventType:     topic,
		mallapp.ReviewPublishedEventType:    topic,
		mallapp.ReviewHiddenEventType:       topic,
		mallapp.EntitlementRevokedEventType: topic,
	}
}

func ProvideOutboxRunner(repo domain.Repository, publisher domain.OutboxPublisher, v *viper.Viper, log logger.Logger) *OutboxRunner {
	ctx, cancel := context.WithCancel(context.Background())
	dispatcher := mallapp.NewOutboxDispatcher(repo, publisher, mallapp.OutboxDispatcherOptions{
		Owner:       StringDefault(v.GetString("outbox.owner"), "mall-service"),
		BatchSize:   v.GetInt("outbox.batchSize"),
		MaxAttempts: v.GetInt("outbox.maxAttempts"),
		Interval:    v.GetDuration("outbox.interval"),
		RetryDelay:  v.GetDuration("outbox.retryDelay"),
		Log:         log.With(logger.String("component", "outbox_dispatcher")),
	})
	runner := &OutboxRunner{ctx: ctx, cancel: cancel, dispatcher: dispatcher}
	if closer, ok := publisher.(interface{ Close() error }); ok {
		runner.publisher = closer
	}
	return runner
}

func ProvideExpiredOrderRunner(service *mallapp.Service, v *viper.Viper, log logger.Logger) *ExpiredOrderRunner {
	ctx, cancel := context.WithCancel(context.Background())
	closer := mallapp.NewExpiredOrderCloser(service, mallapp.ExpiredOrderCloserOptions{
		ExpireAfter:        v.GetDuration("order.expireAfter"),
		RecoverPayingAfter: v.GetDuration("order.paymentRecoveryAfter"),
		Interval:           v.GetDuration("order.expireScanInterval"),
		Limit:              v.GetInt("order.expireScanBatchSize"),
		Log:                log.With(logger.String("component", "expired_order_closer")),
	})
	return &ExpiredOrderRunner{ctx: ctx, cancel: cancel, closer: closer}
}

func (r *OutboxRunner) Start() error {
	if r.dispatcher != nil {
		r.dispatcher.Start(r.ctx)
	}
	return nil
}

func (r *OutboxRunner) Stop() error {
	r.cancel()
	if r.publisher != nil {
		if err := r.publisher.Close(); err != nil {
			return fmt.Errorf("close mall outbox publisher: %w", err)
		}
	}
	return nil
}

func (r *ExpiredOrderRunner) Start() error {
	if r.closer != nil {
		r.closer.Start(r.ctx)
	}
	return nil
}

func (r *ExpiredOrderRunner) Stop() error {
	r.cancel()
	return nil
}

var BusinessProviderSet = wire.NewSet(
	ProvideZapLogger,
	ProvidePostgresPool,
	ProvideRepository,
	ProvideCreditCharger,
	ProvideMallService,
	ProvideOutboxPublisher,
	ProvideOutboxRunner,
	ProvideExpiredOrderRunner,
)

var _ domain.Repository = (*persistence.PostgresRepository)(nil)
