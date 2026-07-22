package server

import (
	"context"

	creditapp "credit-service/internal/app"
	interfacesgrpc "credit-service/internal/interfaces/grpc"
	iocapplication "credit-service/internal/ioc/application"
	"credit-service/internal/ioc/config"
	datasource "credit-service/internal/ioc/db/postgres"
	iocgrpc "credit-service/internal/ioc/grpc"
	iockafka "credit-service/internal/ioc/kafka"
	ioclogger "credit-service/internal/ioc/logger"
	iocredis "credit-service/internal/ioc/redis"
	ioctrace "credit-service/internal/ioc/trace"
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

	ctx := context.Background()
	dbOptions, err := datasource.NewOptions(v, log)
	if err != nil {
		return nil, err
	}
	pool, err := creditapp.ProvidePostgresPool(ctx, dbOptions)
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
	leaderboardCache := creditapp.ProvideLeaderboardCache(redisClient)
	repo, err := creditapp.ProvideRepository(ctx, pool, leaderboardCache)
	if err != nil {
		return nil, err
	}
	service := creditapp.ProvideCreditService(repo)
	projector := creditapp.ProvideProjector(service)
	kafkaOptions, err := iockafka.NewConsumerOptions(v, log)
	if err != nil {
		return nil, err
	}
	runner, err := creditapp.ProvideConsumerRunner(v, kafkaOptions, projector, log)
	if err != nil {
		return nil, err
	}
	handler := interfacesgrpc.NewHandler(service)
	initServers := interfacesgrpc.NewInitServers(handler)

	grpcOptions, err := iocgrpc.NewServerOptions(v, log)
	if err != nil {
		return nil, err
	}
	transportServer, err := iocgrpc.NewServer(grpcOptions, log, initServers, tracer)
	if err != nil {
		return nil, err
	}

	appOptions, err := creditapp.NewOptions(v, zapLogger)
	if err != nil {
		return nil, err
	}
	return creditapp.NewApp(appOptions, zapLogger, transportServer, runner)
}
