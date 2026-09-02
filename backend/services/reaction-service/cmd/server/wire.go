package server

import (
	"context"

	reactionapp "reaction-service/internal/app"
	interfacesgrpc "reaction-service/internal/interfaces/grpc"
	iocapplication "reaction-service/internal/ioc/application"
	"reaction-service/internal/ioc/config"
	datasource "reaction-service/internal/ioc/db/postgres"
	iocgrpc "reaction-service/internal/ioc/grpc"
	iockafka "reaction-service/internal/ioc/kafka"
	ioclogger "reaction-service/internal/ioc/logger"
	iocredis "reaction-service/internal/ioc/redis"
	ioctrace "reaction-service/internal/ioc/trace"
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

	ctx := context.Background()
	reactionStore := reactionapp.ProvideReactionStore(redisClient)
	reports, err := reactionapp.ProvideReportRepository(ctx, db)
	if err != nil {
		return nil, err
	}
	likes, err := reactionapp.ProvideLikeRepository(ctx, db)
	if err != nil {
		return nil, err
	}
	reactions, err := reactionapp.ProvideReactionRepository(ctx, db)
	if err != nil {
		return nil, err
	}
	favorites, err := reactionapp.ProvideFavoriteRepository(ctx, db)
	if err != nil {
		return nil, err
	}
	pins, err := reactionapp.ProvidePinRepository(ctx, db)
	if err != nil {
		return nil, err
	}
	collections, err := reactionapp.ProvideCollectionRepository(ctx, db)
	if err != nil {
		return nil, err
	}
	erasureRepo, err := reactionapp.ProvideAccountErasureRepository(ctx, db)
	if err != nil {
		return nil, err
	}
	if _, err := reactionapp.ProvideCacheWarmup(ctx, v, db, redisClient, likes, favorites); err != nil {
		return nil, err
	}
	publisher := reactionapp.ProvideEventPublisher(kafkaWriter, log)
	commandService := reactionapp.ProvideCommandService(reactionStore, reports, likes, favorites, pins, collections, publisher, log, reactions)
	queryService := reactionapp.ProvideQueryService(reactionStore, reports, likes, favorites, pins, collections, reactions)
	erasureService := reactionapp.ProvideAccountErasureService(erasureRepo, reactionStore)
	handler := interfacesgrpc.NewHandler(commandService, queryService, erasureService)
	initServers := interfacesgrpc.NewInitServers(handler)

	grpcOptions, err := iocgrpc.NewServerOptions(v, log)
	if err != nil {
		return nil, err
	}
	transportServer, err := iocgrpc.NewServer(grpcOptions, log, initServers, tracer)
	if err != nil {
		return nil, err
	}

	appOptions, err := reactionapp.NewOptions(v, zapLogger)
	if err != nil {
		return nil, err
	}
	return reactionapp.NewApp(appOptions, zapLogger, transportServer)
}
