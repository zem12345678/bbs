package server

import (
	contentapp "content-service/internal/app"
	interfacesgrpc "content-service/internal/interfaces/grpc"
	iocapplication "content-service/internal/ioc/application"
	"content-service/internal/ioc/config"
	datasource "content-service/internal/ioc/db/postgres"
	iocgrpc "content-service/internal/ioc/grpc"
	iockafka "content-service/internal/ioc/kafka"
	ioclogger "content-service/internal/ioc/logger"
	iocredis "content-service/internal/ioc/redis"
	ioctrace "content-service/internal/ioc/trace"
)

func CreateApp(configFile string) (*iocapplication.Application, error) {
	v, err := config.New(configFile)
	if err != nil {
		return nil, err
	}

	logOptions, err := ioclogger.NewOptions(v)
	if err != nil {
		return nil, err
	}
	log, err := ioclogger.New(logOptions)
	if err != nil {
		return nil, err
	}
	zapLogger := log.GetZapLogger()

	traceOptions, err := ioctrace.NewOptions(v, zapLogger)
	if err != nil {
		return nil, err
	}
	tracer, err := ioctrace.New(traceOptions)
	if err != nil {
		return nil, err
	}

	dbOptions, err := datasource.NewOptions(v, log)
	if err != nil {
		return nil, err
	}
	db, err := datasource.New(dbOptions)
	if err != nil {
		return nil, err
	}

	redisOptions, err := iocredis.NewOptions(v, log)
	if err != nil {
		return nil, err
	}
	redisClient, err := iocredis.New(redisOptions)
	if err != nil {
		return nil, err
	}

	kafkaOptions, err := iockafka.NewProducerOptions(v, log)
	if err != nil {
		return nil, err
	}
	kafkaWriter, err := iockafka.NewProducer(kafkaOptions)
	if err != nil {
		return nil, err
	}

	articleRepo := contentapp.ProvideArticleRepository(db)
	topicRepo := contentapp.ProvideTopicRepository(db)
	lifecycleOutboxRepo := contentapp.ProvideContentLifecycleOutboxRepository(db)
	categoryRepo := contentapp.ProvideCategoryRepository(db)
	articleCache := contentapp.ProvideArticleCache(v, redisClient)
	node, err := contentapp.ProvideSnowflakeNode(v)
	if err != nil {
		return nil, err
	}
	publisher := contentapp.ProvideEventPublisher(kafkaWriter, log)
	lifecycleOutboxDispatcher := contentapp.ProvideLifecycleOutboxDispatcher(lifecycleOutboxRepo, publisher, v)
	contentOutboxRunner := contentapp.ProvideContentOutboxRunner(topicRepo, lifecycleOutboxDispatcher, publisher, v, log)
	articleCmd := contentapp.ProvideArticleCommandService(articleRepo, articleCache, node, publisher, lifecycleOutboxDispatcher, log)
	articleQry := contentapp.ProvideArticleQueryService(articleRepo, articleCache, publisher, log)

	grpcClientOptions, err := iocgrpc.NewClientOptions(v, log, tracer)
	if err != nil {
		return nil, err
	}
	grpcClient, err := iocgrpc.NewClient(grpcClientOptions)
	if err != nil {
		return nil, err
	}
	commentReader, err := contentapp.ProvideCommentReader(grpcClient, v)
	if err != nil {
		return nil, err
	}
	membershipEntitlements, err := contentapp.ProvideMembershipEntitlementReader(grpcClient, v)
	if err != nil {
		return nil, err
	}
	bountyCredits, err := contentapp.ProvideBountyCreditReader(grpcClient, v)
	if err != nil {
		return nil, err
	}
	topicCmd := contentapp.ProvideTopicCommandService(topicRepo, node, publisher, commentReader, log, membershipEntitlements, bountyCredits, lifecycleOutboxDispatcher)
	topicQry := contentapp.ProvideTopicQueryService(topicRepo, publisher, log)
	categoryCmd := contentapp.ProvideCategoryCommandService(categoryRepo, node)
	categoryQry := contentapp.ProvideCategoryQueryService(categoryRepo)
	handler := interfacesgrpc.NewHandler(articleCmd, articleQry, topicCmd, topicQry, categoryCmd, categoryQry)
	initServers := interfacesgrpc.NewInitServers(handler)

	grpcOptions, err := iocgrpc.NewServerOptions(v, log)
	if err != nil {
		return nil, err
	}
	transportServer, err := iocgrpc.NewServer(grpcOptions, log, initServers, tracer)
	if err != nil {
		return nil, err
	}

	appOptions, err := contentapp.NewOptions(v, zapLogger)
	if err != nil {
		return nil, err
	}
	return contentapp.NewApp(appOptions, zapLogger, transportServer, contentOutboxRunner)
}
