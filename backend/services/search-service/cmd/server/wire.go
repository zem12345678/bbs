package server

import (
	"context"

	searchapp "search-service/internal/app"
	interfacesgrpc "search-service/internal/interfaces/grpc"
	iocapplication "search-service/internal/ioc/application"
	iocconfig "search-service/internal/ioc/config"
	ioces "search-service/internal/ioc/es"
	iocgrpc "search-service/internal/ioc/grpc"
	iockafka "search-service/internal/ioc/kafka"
	ioclogger "search-service/internal/ioc/logger"
	ioctrace "search-service/internal/ioc/trace"
)

func CreateApp(configFile string) (*iocapplication.Application, error) {
	v, err := iocconfig.New(configFile)
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
	zapLogger := searchapp.ProvideZapLogger(log)

	traceOptions, err := ioctrace.NewOptions(v, zapLogger)
	if err != nil {
		return nil, err
	}
	tracer, err := ioctrace.New(traceOptions)
	if err != nil {
		return nil, err
	}

	esOptions, err := ioces.NewOptions(v, zapLogger)
	if err != nil {
		return nil, err
	}
	esClient, err := ioces.New(esOptions)
	if err != nil {
		return nil, err
	}

	repo := searchapp.ProvideSearchRepository(esClient, esOptions)
	commandService := searchapp.ProvideCommandService(repo)
	queryService := searchapp.ProvideQueryService(repo)
	kafkaOptions, err := iockafka.NewConsumerOptions(v, log)
	if err != nil {
		return nil, err
	}
	consumerRunner, err := searchapp.ProvideEventConsumerRunner(v, kafkaOptions, repo, log)
	if err != nil {
		return nil, err
	}
	consumerRunner.Start(context.Background())

	handler := interfacesgrpc.NewHandler(commandService, queryService)
	initServers := interfacesgrpc.NewInitServers(handler)

	grpcOptions, err := iocgrpc.NewServerOptions(v, log)
	if err != nil {
		return nil, err
	}
	transportServer, err := iocgrpc.NewServer(grpcOptions, log, initServers, tracer)
	if err != nil {
		return nil, err
	}

	appOptions, err := searchapp.NewOptions(v, zapLogger)
	if err != nil {
		return nil, err
	}
	return searchapp.NewApp(appOptions, zapLogger, transportServer)
}
