package server

import (
	"context"
	"time"

	gatewayapp "api-gateway/internal/app"
	"api-gateway/internal/clients"
	httpiface "api-gateway/internal/interfaces/http"
	iocapplication "api-gateway/internal/ioc/application"
	"api-gateway/internal/ioc/config"
	iocgrpc "api-gateway/internal/ioc/grpc"
	iochttp "api-gateway/internal/ioc/http"
	ioclogger "api-gateway/internal/ioc/logger"
	iocredis "api-gateway/internal/ioc/redis"
	ioctrace "api-gateway/internal/ioc/trace"
	realtimechat "api-gateway/internal/realtime/chat"
	"api-gateway/internal/storage"
	"api-gateway/pkg/ratelimt"
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
	cleanup := true
	defer func() {
		if cleanup {
			_ = bbsClients.Close()
		}
	}()
	redisOptions, err := iocredis.NewOptions(v, log)
	if err != nil {
		return nil, err
	}
	redisClient, err := iocredis.New(redisOptions)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cleanup {
			_ = redisClient.Close()
		}
	}()
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 3*time.Second)
	pingErr := redisClient.Ping(pingCtx).Err()
	pingCancel()
	if pingErr != nil {
		return nil, pingErr
	}
	chatJoinLimit := ratelimit.NewRedisSlidingWindowLimiter(
		redisClient,
		runtimeCfg.Chat.RateLimit.JoinInterval,
		runtimeCfg.Chat.RateLimit.JoinRate,
	)
	chatSendLimit := ratelimit.NewRedisSlidingWindowLimiter(
		redisClient,
		runtimeCfg.Chat.RateLimit.SendInterval,
		runtimeCfg.Chat.RateLimit.SendRate,
	)
	chatRealtime := realtimechat.NewService(redisClient, bbsClients.Chat, realtimechat.Options{
		TicketTTL: 45 * time.Second, AllowedOrigins: v.GetStringSlice("cors.allowedOrigins"), Logger: zapLogger,
		SendLimiter: chatSendLimit,
	})

	attachmentStore, err := storage.NewMinIO(v)
	if err != nil {
		return nil, err
	}
	handler := httpiface.NewHandlerWithRealtimeAndRateLimits(
		bbsClients, runtimeCfg.Auth.TokenHeader, runtimeCfg.Auth.TokenPrefix,
		runtimeCfg.Auth.JWTSecret, attachmentStore, chatRealtime, chatJoinLimit, chatSendLimit,
	)
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
	runtime := gatewayapp.NewRuntime(chatRealtime, bbsClients)
	application, err := gatewayapp.NewApp(appOptions, zapLogger, httpServer, runtime)
	if err != nil {
		return nil, err
	}
	cleanup = false
	return application, nil
}
