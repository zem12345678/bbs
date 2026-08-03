package server

import (
	userapp "user-service/internal/app"
	interfacesgrpc "user-service/internal/interfaces/grpc"
	iocapplication "user-service/internal/ioc/application"
	iocconfig "user-service/internal/ioc/config"
	datasource "user-service/internal/ioc/db/postgres"
	iocgrpc "user-service/internal/ioc/grpc"
	iockafka "user-service/internal/ioc/kafka"
	ioclogger "user-service/internal/ioc/logger"
	iocredis "user-service/internal/ioc/redis"
	ioctrace "user-service/internal/ioc/trace"
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
	zapLogger := userapp.ProvideZapLogger(log)

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

	kafkaOptions, err := iockafka.NewProducerOptions(v, log)
	if err != nil {
		return nil, err
	}
	kafkaWriter, err := iockafka.NewProducer(kafkaOptions)
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

	repo := userapp.ProvideRepository(db)
	idgen, err := userapp.ProvideIDGenerator(v)
	if err != nil {
		return nil, err
	}
	publisher := userapp.ProvideEventPublisher(kafkaWriter, log)
	grpcClientOptions, err := iocgrpc.NewClientOptions(v, log, tracer)
	if err != nil {
		return nil, err
	}
	grpcClient, err := iocgrpc.NewClient(grpcClientOptions)
	if err != nil {
		return nil, err
	}
	themeEntitlements, err := userapp.ProvideProfileThemeEntitlementReader(grpcClient, v)
	if err != nil {
		return nil, err
	}
	securityEmails, err := userapp.ProvideSecurityEmailSender(v)
	if err != nil {
		return nil, err
	}
	credentialVersions := userapp.ProvideCredentialVersionCache(redisClient)
	mfaManager, err := userapp.ProvideMFAManager(v)
	if err != nil {
		return nil, err
	}
	commandService := userapp.ProvideCommandService(repo, idgen, publisher, log, v, themeEntitlements, securityEmails, credentialVersions, mfaManager)
	queryService := userapp.ProvideQueryService(repo, themeEntitlements)
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

	appOptions, err := userapp.NewOptions(v, zapLogger)
	if err != nil {
		return nil, err
	}
	return userapp.NewApp(appOptions, zapLogger, transportServer)
}
