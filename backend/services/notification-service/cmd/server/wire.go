package server

import (
	"context"

	notificationapp "notification-service/internal/app"
	interfacesgrpc "notification-service/internal/interfaces/grpc"
	iocapplication "notification-service/internal/ioc/application"
	"notification-service/internal/ioc/config"
	datasource "notification-service/internal/ioc/db/postgres"
	iocgrpc "notification-service/internal/ioc/grpc"
	iockafka "notification-service/internal/ioc/kafka"
	ioclogger "notification-service/internal/ioc/logger"
	ioctrace "notification-service/internal/ioc/trace"
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
	pool, err := notificationapp.ProvidePostgresPool(ctx, dbOptions)
	if err != nil {
		return nil, err
	}
	repo, err := notificationapp.ProvideRepository(ctx, pool)
	if err != nil {
		return nil, err
	}
	service := notificationapp.ProvideNotificationService(repo)
	projector := notificationapp.ProvideProjector(service)
	kafkaOptions, err := iockafka.NewConsumerOptions(v, log)
	if err != nil {
		return nil, err
	}
	runner, err := notificationapp.ProvideConsumerRunner(v, kafkaOptions, projector, log)
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

	appOptions, err := notificationapp.NewOptions(v, zapLogger)
	if err != nil {
		return nil, err
	}
	return notificationapp.NewApp(appOptions, zapLogger, transportServer, runner)
}
