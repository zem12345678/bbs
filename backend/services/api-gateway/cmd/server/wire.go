package server

import (
	gatewayapp "api-gateway/internal/app"
	"api-gateway/internal/clients"
	httpiface "api-gateway/internal/interfaces/http"
	iocapplication "api-gateway/internal/ioc/application"
	"api-gateway/internal/ioc/config"
	iocgrpc "api-gateway/internal/ioc/grpc"
	iochttp "api-gateway/internal/ioc/http"
	ioclogger "api-gateway/internal/ioc/logger"
	ioctrace "api-gateway/internal/ioc/trace"
)

func CreateApp(configFile string) (*iocapplication.Application, error) {
	v, err := config.New(configFile)
	if err != nil {
		return nil, err
	}
	runtimeCfg, err := loadRuntimeConfig(v)
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

	grpcClientOptions, err := iocgrpc.NewClientOptions(v, log, tracer)
	if err != nil {
		return nil, err
	}
	grpcClient, err := iocgrpc.NewClient(grpcClientOptions)
	if err != nil {
		return nil, err
	}
	bbsClients, err := clients.New(grpcClient, runtimeCfg.Upstreams)
	if err != nil {
		return nil, err
	}

	handler := httpiface.NewHandler(bbsClients, runtimeCfg.Auth.TokenHeader, runtimeCfg.Auth.TokenPrefix, runtimeCfg.Auth.JWTSecret)
	initControllers := httpiface.NewInitControllers(handler)

	httpOptions, err := iochttp.NewOptions(v, zapLogger)
	if err != nil {
		return nil, err
	}
	router := iochttp.NewRouter(httpOptions, zapLogger, initControllers, tracer)
	httpServer, err := iochttp.New(httpOptions, zapLogger, router)
	if err != nil {
		return nil, err
	}

	appOptions, err := gatewayapp.NewOptions(v, zapLogger)
	if err != nil {
		return nil, err
	}
	return gatewayapp.NewApp(appOptions, zapLogger, httpServer)
}
