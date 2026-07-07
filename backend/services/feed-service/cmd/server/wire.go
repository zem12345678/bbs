package server

import (
	feedapp "feed-service/internal/app"
	interfacesgrpc "feed-service/internal/interfaces/grpc"
	iocapplication "feed-service/internal/ioc/application"
	"feed-service/internal/ioc/config"
	iocgrpc "feed-service/internal/ioc/grpc"
	iockafka "feed-service/internal/ioc/kafka"
	ioclogger "feed-service/internal/ioc/logger"
	iocredis "feed-service/internal/ioc/redis"
	ioctrace "feed-service/internal/ioc/trace"
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

	redisOptions, err := iocredis.NewOptions(v, log)
	if err != nil {
		return nil, err
	}
	redisClient, err := iocredis.New(redisOptions)
	if err != nil {
		return nil, err
	}

	repo := feedapp.ProvideFeedRepository(redisClient)
	queryService := feedapp.ProvideQueryService(repo)
	kafkaOptions, err := iockafka.NewConsumerOptions(v, log)
	if err != nil {
		return nil, err
	}
	runner, err := feedapp.ProvideConsumerRunner(v, kafkaOptions, repo, log)
	if err != nil {
		return nil, err
	}
	handler := interfacesgrpc.NewHandler(queryService)
	initServers := interfacesgrpc.NewInitServers(handler)

	grpcOptions, err := iocgrpc.NewServerOptions(v, log)
	if err != nil {
		return nil, err
	}
	transportServer, err := iocgrpc.NewServer(grpcOptions, log, initServers, tracer)
	if err != nil {
		return nil, err
	}

	appOptions, err := feedapp.NewOptions(v, zapLogger)
	if err != nil {
		return nil, err
	}
	return feedapp.NewApp(appOptions, zapLogger, transportServer, runner)
}
