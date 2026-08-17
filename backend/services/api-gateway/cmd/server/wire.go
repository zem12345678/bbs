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
	"api-gateway/internal/popularity"
	realtimechat "api-gateway/internal/realtime/chat"
	"api-gateway/internal/storage"
	"api-gateway/pkg/ratelimit"
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
	chatTicketLimit := ratelimit.NewRedisSlidingWindowLimiter(
		redisClient,
		runtimeCfg.Chat.RateLimit.TicketInterval,
		runtimeCfg.Chat.RateLimit.TicketRate,
	)
	chatCreateRoomLimit := ratelimit.NewRedisSlidingWindowLimiter(
		redisClient,
		runtimeCfg.Chat.RateLimit.CreateRoomInterval,
		runtimeCfg.Chat.RateLimit.CreateRoomRate,
	)
	chatSubscribeLimit := ratelimit.NewRedisSlidingWindowLimiter(
		redisClient,
		runtimeCfg.Chat.RateLimit.SubscribeInterval,
		runtimeCfg.Chat.RateLimit.SubscribeRate,
	)
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
	chatReadLimit := ratelimit.NewRedisSlidingWindowLimiter(
		redisClient,
		runtimeCfg.Chat.RateLimit.ReadInterval,
		runtimeCfg.Chat.RateLimit.ReadRate,
	)
	authRateLimits := httpiface.AuthRateLimits{
		Register: ratelimit.NewRedisSlidingWindowLimiter(
			redisClient, runtimeCfg.Auth.RateLimit.RegisterInterval, runtimeCfg.Auth.RateLimit.RegisterRate,
		),
		Login: ratelimit.NewRedisSlidingWindowLimiter(
			redisClient, runtimeCfg.Auth.RateLimit.LoginInterval, runtimeCfg.Auth.RateLimit.LoginRate,
		),
		PasswordReset: ratelimit.NewRedisSlidingWindowLimiter(
			redisClient, runtimeCfg.Auth.RateLimit.PasswordResetInterval, runtimeCfg.Auth.RateLimit.PasswordResetRate,
		),
		PasswordResetConfirm: ratelimit.NewRedisSlidingWindowLimiter(
			redisClient, runtimeCfg.Auth.RateLimit.PasswordResetConfirmInterval, runtimeCfg.Auth.RateLimit.PasswordResetConfirmRate,
		),
		EmailVerification: ratelimit.NewRedisSlidingWindowLimiter(
			redisClient, runtimeCfg.Auth.RateLimit.EmailVerificationInterval, runtimeCfg.Auth.RateLimit.EmailVerificationRate,
		),
		AdminLogin: ratelimit.NewRedisSlidingWindowLimiter(
			redisClient, runtimeCfg.Auth.RateLimit.AdminLoginInterval, runtimeCfg.Auth.RateLimit.AdminLoginRate,
		),
	}
	searchRateLimits := httpiface.SearchRateLimits{
		Content: ratelimit.NewRedisSlidingWindowLimiter(
			redisClient, runtimeCfg.Search.RateLimit.ContentInterval, runtimeCfg.Search.RateLimit.ContentRate,
		),
		User: ratelimit.NewRedisSlidingWindowLimiter(
			redisClient, runtimeCfg.Search.RateLimit.UserInterval, runtimeCfg.Search.RateLimit.UserRate,
		),
	}
	fileUploadLimit := ratelimit.NewRedisSlidingWindowLimiter(
		redisClient, runtimeCfg.Files.RateLimit.UploadInterval, runtimeCfg.Files.RateLimit.UploadRate,
	)
	antennaExportGate := httpiface.NewRedisAntennaExportGate(redisClient, runtimeCfg.Exports.RateLimit.AntennaInterval, 15*time.Minute)
	blockingExportGate := httpiface.NewRedisBlockingExportGate(redisClient, runtimeCfg.Exports.RateLimit.BlockingInterval, 15*time.Minute)
	clipExportGate := httpiface.NewRedisClipExportGate(redisClient, runtimeCfg.Exports.RateLimit.ClipInterval, 15*time.Minute)
	muteExportGate := httpiface.NewRedisMuteExportGate(redisClient, runtimeCfg.Exports.RateLimit.MuteInterval, 15*time.Minute)
	popularityStore := popularity.NewStore(redisClient)
	chatRealtime := realtimechat.NewService(redisClient, bbsClients.Chat, realtimechat.Options{
		TicketTTL: 45 * time.Second, AllowedOrigins: v.GetStringSlice("cors.allowedOrigins"), Logger: zapLogger,
		SubscribeLimiter:      chatSubscribeLimit,
		SendLimiter:           chatSendLimit,
		ReadLimiter:           chatReadLimit,
		MaxConnectionsPerUser: runtimeCfg.Chat.WebSocket.MaxConnectionsPerUser,
		MaxConnectionsPerIP:   runtimeCfg.Chat.WebSocket.MaxConnectionsPerIP,
		Popularity:            popularityStore,
	})
	tokenRevocations := httpiface.NewRedisTokenRevocationStore(redisClient)
	credentialVersions := httpiface.NewRedisCredentialVersionStore(
		redisClient,
		httpiface.NewUserCredentialVersionAuthority(bbsClients.UserCredentialVersion),
	)

	attachmentStore, err := storage.NewMinIO(v)
	if err != nil {
		return nil, err
	}
	storageCtx, storageCancel := context.WithTimeout(context.Background(), 10*time.Second)
	err = attachmentStore.EnsureReady(storageCtx)
	storageCancel()
	if err != nil {
		return nil, err
	}
	objectCleanup := storage.NewObjectCleanupQueue(attachmentStore, redisClient, zapLogger)
	handler := httpiface.NewHandlerWithRealtimeAndRateLimitsAndTokenSecurityStores(
		bbsClients, runtimeCfg.Auth.TokenHeader, runtimeCfg.Auth.TokenPrefix,
		runtimeCfg.Auth.JWTSecret, attachmentStore, chatRealtime, chatJoinLimit, chatSendLimit, tokenRevocations, credentialVersions,
	)
	handler.SetPublicBaseURL(runtimeCfg.PublicBaseURL)
	handler.SetChatTicketLimit(chatTicketLimit)
	handler.SetChatTicketRetryAfter(runtimeCfg.Chat.RateLimit.TicketInterval)
	handler.SetChatCreateRoomLimit(chatCreateRoomLimit)
	handler.SetChatReadLimit(chatReadLimit)
	handler.SetAuthRateLimits(authRateLimits)
	handler.SetSearchRateLimits(searchRateLimits)
	handler.SetFileUploadLimit(fileUploadLimit)
	handler.SetAntennaExportGate(antennaExportGate)
	handler.SetBlockingExportGate(blockingExportGate)
	handler.SetClipExportGate(clipExportGate)
	handler.SetMuteExportGate(muteExportGate)
	handler.SetUploadedObjectCleaner(objectCleanup)
	handler.SetPopularityStore(popularityStore)
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
	application, err := gatewayapp.NewApp(appOptions, zapLogger, httpServer, runtime, objectCleanup)
	if err != nil {
		return nil, err
	}
	cleanup = false
	return application, nil
}
