package app

import (
	"context"
	"fmt"
	"time"

	articlecommand "content-service/internal/application/article/command"
	articlequery "content-service/internal/application/article/query"
	categorycommand "content-service/internal/application/category/command"
	categoryquery "content-service/internal/application/category/query"
	topiccommand "content-service/internal/application/topic/command"
	topicquery "content-service/internal/application/topic/query"
	commentclient "content-service/internal/clients/comment"
	creditclient "content-service/internal/clients/credit"
	mallclient "content-service/internal/clients/mall"
	articleDomain "content-service/internal/domain/article"
	categoryDomain "content-service/internal/domain/category"
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

type QAAcceptanceOutboxRunner struct {
	ctx        context.Context
	cancel     context.CancelFunc
	dispatcher *topiccommand.QAAcceptanceOutboxDispatcher
	interval   time.Duration
	publisher  interface{ Close() error }
	log        logger.Logger
}

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

func ProvideEventPublisher(writer *kafka.Writer, log logger.Logger) *messaging.KafkaEventPublisher {
	return messaging.NewKafkaEventPublisher(writer, log)
}

func ProvideQAAcceptanceOutboxRunner(repo *persistence.TopicRepo, publisher *messaging.KafkaEventPublisher, v *viper.Viper, log logger.Logger) *QAAcceptanceOutboxRunner {
	ctx, cancel := context.WithCancel(context.Background())
	interval := v.GetDuration("outbox.interval")
	if interval <= 0 {
		interval = time.Second
	}
	return &QAAcceptanceOutboxRunner{
		ctx:    ctx,
		cancel: cancel,
		dispatcher: topiccommand.NewQAAcceptanceOutboxDispatcher(repo, publisher, topiccommand.QAAcceptanceOutboxDispatcherOptions{
			Owner:         qaAcceptanceOutboxOwner(v.GetString("outbox.owner")),
			BatchSize:     v.GetInt("outbox.batchSize"),
			LeaseDuration: v.GetDuration("outbox.leaseDuration"),
			RetryDelay:    v.GetDuration("outbox.retryDelay"),
		}),
		interval:  interval,
		publisher: publisher,
		log:       log,
	}
}

func qaAcceptanceOutboxOwner(value string) string {
	return StringDefault(value, "content-service") + ":" + uuid.NewString()
}

func (r *QAAcceptanceOutboxRunner) Start() error {
	if r.dispatcher != nil {
		go r.run()
	}
	return nil
}

func (r *QAAcceptanceOutboxRunner) Stop() error {
	r.cancel()
	if r.publisher != nil {
		if err := r.publisher.Close(); err != nil {
			return fmt.Errorf("close QA acceptance outbox publisher: %w", err)
		}
	}
	return nil
}

func (r *QAAcceptanceOutboxRunner) run() {
	for {
		if _, err := r.dispatcher.DispatchOnce(r.ctx); err != nil && r.log != nil {
			r.log.Warn("dispatch QA acceptance outbox failed", logger.Error(err))
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
	log logger.Logger,
) *articlecommand.Service {
	return articlecommand.NewService(repo, articleCache, idgen, publisher, log)
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
) *topiccommand.Service {
	return topiccommand.NewService(repo, idgen, publisher, commentReader, log, membershipEntitlements, bountyCredits)
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
	ProvideCategoryRepository,
	ProvideArticleCache,
	ProvideSnowflakeNode,
	ProvideEventPublisher,
	ProvideQAAcceptanceOutboxRunner,
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
var _ articlecommand.IDGenerator = (*snowflake.Node)(nil)
var _ topiccommand.IDGenerator = (*snowflake.Node)(nil)
var _ categorycommand.IDGenerator = (*snowflake.Node)(nil)
