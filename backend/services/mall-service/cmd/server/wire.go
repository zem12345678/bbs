package server

import (
	"context"

	mallapp "mall-service/internal/app"
	interfacesgrpc "mall-service/internal/interfaces/grpc"
	iocapplication "mall-service/internal/ioc/application"
	"mall-service/internal/ioc/config"
	datasource "mall-service/internal/ioc/db/postgres"
	iocgrpc "mall-service/internal/ioc/grpc"
	iockafka "mall-service/internal/ioc/kafka"
	ioclogger "mall-service/internal/ioc/logger"
	ioctrace "mall-service/internal/ioc/trace"
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
	pool, err := mallapp.ProvidePostgresPool(ctx, dbOptions)
	if err != nil {
		return nil, err
	}
	repo, err := mallapp.ProvideRepository(ctx, pool)
	if err != nil {
		return nil, err
	}

	grpcClientOptions, err := iocgrpc.NewClientOptions(v, log, tracer)
	if err != nil {
		return nil, err
	}
	grpcClient, err := iocgrpc.NewClient(grpcClientOptions)
	if err != nil {
		return nil, err
	}
	charger, err := mallapp.ProvideCreditCharger(grpcClient, v)
	if err != nil {
		return nil, err
	}
	service := mallapp.ProvideMallService(repo, charger, v)

	kafkaOptions, err := iockafka.NewProducerOptions(v, log)
	if err != nil {
		return nil, err
	}
	writer, err := iockafka.NewProducer(kafkaOptions)
	if err != nil {
		return nil, err
	}
	publisher := mallapp.ProvideOutboxPublisher(writer, v)
	runner := mallapp.ProvideOutboxRunner(repo, publisher, v, log)
	expiredOrderRunner := mallapp.ProvideExpiredOrderRunner(service, v)
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

	appOptions, err := mallapp.NewOptions(v, zapLogger)
	if err != nil {
		return nil, err
	}
	return mallapp.NewApp(appOptions, zapLogger, transportServer, runner, expiredOrderRunner)
}
