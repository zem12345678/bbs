package app

import (
	"context"
	"fmt"
	"time"

	articlecommand "content-service/internal/application/article/command"
	articlequery "content-service/internal/application/article/query"
	categorycommand "content-service/internal/application/category/command"
	categoryquery "content-service/internal/application/category/query"
	outboxapp "content-service/internal/application/outbox"
	topiccommand "content-service/internal/application/topic/command"
	topicquery "content-service/internal/application/topic/query"
	commentclient "content-service/internal/clients/comment"
	creditclient "content-service/internal/clients/credit"
	mallclient "content-service/internal/clients/mall"
	articleDomain "content-service/internal/domain/article"
	categoryDomain "content-service/internal/domain/category"
	outboxDomain "content-service/internal/domain/outbox"
	topicDomain "content-service/internal/domain/topic"
	"content-service/internal/infrastructure/cache"
	"content-service/internal/infrastructure/messaging"
	"content-service/internal/infrastructure/persistence"
	iocgrpc "content-service/internal/ioc/grpc"
	"content-service/pkg/logger"
	"content-service/pkg/snowflake"

	"github.com/google/uuid"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ContentOutboxRunner struct {
	ctx                 context.Context
	cancel              context.CancelFunc
	qaDispatcher        *topiccommand.QAAcceptanceOutboxDispatcher
	lifecycleDispatcher *outboxapp.LifecycleDispatcher
	interval            time.Duration
	publisher           interface{ Close() error }
	log                 logger.Logger
}

func ProvideZapLogger(l logger.Logger) *zap.Logger { return l.GetZapLogger() }

func ProvideArticleRepository(db *gorm.DB) *persistence.Repo {
	return persistence.NewRepo(db)
}

func ProvideTopicRepository(db *gorm.DB) *persistence.TopicRepo {
	return persistence.NewTopicRepo(db)
}

func ProvideContentLifecycleOutboxRepository(db *gorm.DB) *persistence.ContentLifecycleOutboxRepo {
	return persistence.NewContentLifecycleOutboxRepo(db)
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
	var err error
	workerID, err = snowflake.ResolveWorkerID(
		workerID,
		v.GetInt64("snowflake.workerIdRangeStart"),
		v.GetInt64("snowflake.workerIdRangeSize"),
		v.GetString("snowflake.instanceName"),
	)
	if err != nil {
		return nil, err
	}
	return snowflake.NewNode(workerID)
}

func ProvideEventPublisher(writer *kafka.Writer, log logger.Logger) *messaging.KafkaEventPublisher {
	return messaging.NewKafkaEventPublisher(writer, log)
}

func ProvideLifecycleOutboxDispatcher(repo *persistence.ContentLifecycleOutboxRepo, publisher *messaging.KafkaEventPublisher, v *viper.Viper) *outboxapp.LifecycleDispatcher {
	return outboxapp.NewLifecycleDispatcher(repo, publisher, outboxapp.LifecycleDispatcherOptions{
		Owner:         contentOutboxOwner(v.GetString("outbox.owner")),
		BatchSize:     v.GetInt("outbox.batchSize"),
		LeaseDuration: v.GetDuration("outbox.leaseDuration"),
		RetryDelay:    v.GetDuration("outbox.retryDelay"),
	})
}

func ProvideContentOutboxRunner(repo *persistence.TopicRepo, lifecycleDispatcher *outboxapp.LifecycleDispatcher, publisher *messaging.KafkaEventPublisher, v *viper.Viper, log logger.Logger) *ContentOutboxRunner {
	ctx, cancel := context.WithCancel(context.Background())
	interval := v.GetDuration("outbox.interval")
	if interval <= 0 {
		interval = time.Second
	}
	return &ContentOutboxRunner{
		ctx:    ctx,
		cancel: cancel,
		qaDispatcher: topiccommand.NewQAAcceptanceOutboxDispatcher(repo, publisher, topiccommand.QAAcceptanceOutboxDispatcherOptions{
			Owner:         contentOutboxOwner(v.GetString("outbox.owner")),
			BatchSize:     v.GetInt("outbox.batchSize"),
			LeaseDuration: v.GetDuration("outbox.leaseDuration"),
			RetryDelay:    v.GetDuration("outbox.retryDelay"),
		}),
		lifecycleDispatcher: lifecycleDispatcher,
		interval:            interval,
		publisher:           publisher,
		log:                 log,
	}
}

func contentOutboxOwner(value string) string {
	return StringDefault(value, "content-service") + ":" + uuid.NewString()
}

func (r *ContentOutboxRunner) Start() error {
	if r.qaDispatcher != nil || r.lifecycleDispatcher != nil {
		go r.run()
	}
	return nil
}

func (r *ContentOutboxRunner) Stop() error {
	r.cancel()
	if r.publisher != nil {
		if err := r.publisher.Close(); err != nil {
			return fmt.Errorf("close content outbox publisher: %w", err)
		}
	}
	return nil
}

func (r *ContentOutboxRunner) run() {
	for {
		if r.qaDispatcher != nil {
			if _, err := r.qaDispatcher.DispatchOnce(r.ctx); err != nil && r.log != nil {
				r.log.Warn("dispatch QA acceptance outbox failed", logger.Error(err))
			}
		}
		if r.lifecycleDispatcher != nil {
			if _, err := r.lifecycleDispatcher.DispatchOnce(r.ctx); err != nil && r.log != nil {
				r.log.Warn("dispatch content lifecycle outbox failed", logger.Error(err))
			}
		}
		timer := time.NewTimer(r.interval)
		select {
		case <-r.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func ProvideCommentReader(grpcClient *iocgrpc.Client, v *viper.Viper) (topiccommand.CommentReader, error) {
	return commentclient.NewClient(grpcClient, v)
}

func ProvideMembershipEntitlementReader(grpcClient *iocgrpc.Client, v *viper.Viper) (topiccommand.MembershipEntitlementReader, error) {
	return mallclient.NewClient(grpcClient, v)
}

func ProvideBountyCreditReader(grpcClient *iocgrpc.Client, v *viper.Viper) (topiccommand.BountyCreditReader, error) {
	return creditclient.NewClient(grpcClient, v)
}

func ProvideArticleCommandService(
	repo articleDomain.Repository,
	articleCache *cache.ArticleCache,
	idgen articlecommand.IDGenerator,
	publisher messaging.EventPublisher,
	lifecycleOutbox *outboxapp.LifecycleDispatcher,
	log logger.Logger,
) *articlecommand.Service {
	return articlecommand.NewService(repo, articleCache, idgen, publisher, log, lifecycleOutbox)
}

func ProvideArticleQueryService(
	repo articleDomain.Repository,
	articleCache *cache.ArticleCache,
	publisher messaging.EventPublisher,
	log logger.Logger,
) *articlequery.Service {
	return articlequery.NewService(repo, articleCache, publisher, log)
}

func ProvideTopicCommandService(
	repo topicDomain.Repository,
	idgen topiccommand.IDGenerator,
	publisher messaging.EventPublisher,
	commentReader topiccommand.CommentReader,
	log logger.Logger,
	membershipEntitlements topiccommand.MembershipEntitlementReader,
	bountyCredits topiccommand.BountyCreditReader,
	lifecycleOutbox *outboxapp.LifecycleDispatcher,
) *topiccommand.Service {
	return topiccommand.NewService(repo, idgen, publisher, commentReader, log, membershipEntitlements, bountyCredits, lifecycleOutbox)
}

func ProvideTopicQueryService(repo topicDomain.Repository, publisher messaging.EventPublisher, log logger.Logger) *topicquery.Service {
	return topicquery.NewService(repo, publisher, log)
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
	ProvideContentLifecycleOutboxRepository,
	ProvideCategoryRepository,
	ProvideArticleCache,
	ProvideSnowflakeNode,
	ProvideEventPublisher,
	ProvideLifecycleOutboxDispatcher,
	ProvideContentOutboxRunner,
	ProvideCommentReader,
	ProvideMembershipEntitlementReader,
	ProvideBountyCreditReader,
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
var _ outboxDomain.LifecycleRepository = (*persistence.ContentLifecycleOutboxRepo)(nil)
var _ outboxDomain.LifecyclePublisher = (*messaging.KafkaEventPublisher)(nil)
var _ articlecommand.IDGenerator = (*snowflake.Node)(nil)
var _ topiccommand.IDGenerator = (*snowflake.Node)(nil)
var _ categorycommand.IDGenerator = (*snowflake.Node)(nil)
