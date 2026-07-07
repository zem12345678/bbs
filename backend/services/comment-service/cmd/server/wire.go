package server

import (
	"context"

	commentapp "comment-service/internal/app"
	interfacesgrpc "comment-service/internal/interfaces/grpc"
	iocapplication "comment-service/internal/ioc/application"
	iocconfig "comment-service/internal/ioc/config"
	iocmongo "comment-service/internal/ioc/db/mongo"
	iocgrpc "comment-service/internal/ioc/grpc"
	iockafka "comment-service/internal/ioc/kafka"
	ioclogger "comment-service/internal/ioc/logger"
	ioctrace "comment-service/internal/ioc/trace"
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
	zapLogger := commentapp.ProvideZapLogger(log)

	traceOptions, err := ioctrace.NewOptions(v, zapLogger)
	if err != nil {
		return nil, err
	}
	tracer, err := ioctrace.New(traceOptions)
	if err != nil {
		return nil, err
	}

	mongoOptions, err := iocmongo.NewOptions(v, log)
	if err != nil {
		return nil, err
	}
	mongodb, err := iocmongo.New(mongoOptions)
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

	repo, err := commentapp.ProvideRepository(context.Background(), mongodb)
	if err != nil {
		return nil, err
	}
	idgen, err := commentapp.ProvideIDGenerator(v)
	if err != nil {
		return nil, err
	}
	publisher := commentapp.ProvideEventPublisher(kafkaWriter, log)
	commandService := commentapp.ProvideCommandService(repo, idgen, publisher, log)
	queryService := commentapp.ProvideQueryService(repo)
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

	appOptions, err := commentapp.NewOptions(v, zapLogger)
	if err != nil {
		return nil, err
	}
	return commentapp.NewApp(appOptions, zapLogger, transportServer)
}
