package server

import (
	"context"

	adminapp "admin/internal/app"
	adminauth "admin/internal/infrastructure/auth"
	"admin/internal/infrastructure/authz"
	"admin/internal/infrastructure/persistence"
	interfacesgrpc "admin/internal/interfaces/grpc"
	iocapplication "admin/internal/ioc/application"
	"admin/internal/ioc/config"
	datasource "admin/internal/ioc/db/postgres"
	iocgrpc "admin/internal/ioc/grpc"
	ioclogger "admin/internal/ioc/logger"
	iocredis "admin/internal/ioc/redis"
	ioctrace "admin/internal/ioc/trace"
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
	_ = redisClient

	grpcClientOptions, err := iocgrpc.NewClientOptions(v, log, tracer)
	if err != nil {
		return nil, err
	}
	grpcClient, err := iocgrpc.NewClient(grpcClientOptions)
	if err != nil {
		return nil, err
	}

	repo := persistence.NewRepository(db)
	authorizer, err := authz.NewAuthorizer(context.Background(), repo, adminapp.BootstrapAdminPrefixes(v))
	if err != nil {
		return nil, err
	}
	passwords := adminauth.NewPasswordManager()
	tokens, err := adminapp.ProvideTokenManager(v)
	if err != nil {
		return nil, err
	}
	secrets, err := adminapp.ProvideSecretCipher(v)
	if err != nil {
		return nil, err
	}
	upstreams, err := adminapp.ProvideUpstreams(grpcClient, v)
	if err != nil {
		return nil, err
	}
	service := adminapp.ProvideAdminService(authorizer, repo, passwords, tokens, secrets, upstreams)
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

	appOptions, err := adminapp.NewOptions(v, zapLogger)
	if err != nil {
		return nil, err
	}
	return adminapp.NewApp(appOptions, zapLogger, transportServer)
}
