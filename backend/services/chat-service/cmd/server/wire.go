package server

import (
	"context"
	"time"

	chatapp "chat-service/internal/app"
	interfacesgrpc "chat-service/internal/interfaces/grpc"
	iocapplication "chat-service/internal/ioc/application"
	"chat-service/internal/ioc/config"
	datasource "chat-service/internal/ioc/db/postgres"
	iocgrpc "chat-service/internal/ioc/grpc"
	iockafka "chat-service/internal/ioc/kafka"
	ioclogger "chat-service/internal/ioc/logger"
	iocredis "chat-service/internal/ioc/redis"
	ioctrace "chat-service/internal/ioc/trace"
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
	pool, err := chatapp.ProvidePostgresPool(ctx, dbOptions)
	if err != nil {
		return nil, err
	}
	repository := chatapp.ProvideRepository(pool)
	ids, err := chatapp.ProvideSnowflakeGenerator(v)
	if err != nil {
		pool.Close()
		return nil, err
	}
	service := chatapp.ProvideChatService(repository, ids)
	erasureService := chatapp.ProvideAccountErasureService(repository)

	redisOptions, err := iocredis.NewOptions(v, log)
	if err != nil {
		pool.Close()
		return nil, err
	}
	redisClient, err := iocredis.New(redisOptions)
	if err != nil {
		pool.Close()
		return nil, err
	}
	pingCtx, pingCancel := context.WithTimeout(ctx, 3*time.Second)
	err = iocredis.Ping(pingCtx, redisClient)
	pingCancel()
	if err != nil {
		_ = redisClient.Close()
		pool.Close()
		return nil, err
	}

	producerOptions, err := iockafka.NewProducerOptions(v, log)
	if err != nil {
		_ = redisClient.Close()
		pool.Close()
		return nil, err
	}
	consumerOptions, err := iockafka.NewConsumerOptions(v, log)
	if err != nil {
		_ = redisClient.Close()
		pool.Close()
		return nil, err
	}
	if err := producerOptions.VerifyTopic(ctx); err != nil {
		_ = redisClient.Close()
		pool.Close()
		return nil, err
	}
	realtimeTopic := chatapp.StringDefault(v.GetString("kafka.topic"), "chat.events")
	if err := consumerOptions.VerifyTopic(ctx, realtimeTopic); err != nil {
		_ = redisClient.Close()
		pool.Close()
		return nil, err
	}
	writer, err := iockafka.NewProducer(producerOptions)
	if err != nil {
		_ = redisClient.Close()
		pool.Close()
		return nil, err
	}
	publisher := chatapp.ProvideOutboxPublisher(writer)
	outbox := chatapp.ProvideOutboxDispatcher(repository, publisher, v, zapLogger)
	realtime, err := chatapp.ProvideRealtimeDispatcher(v, consumerOptions, redisClient, zapLogger)
	if err != nil {
		_ = publisher.Close()
		_ = redisClient.Close()
		pool.Close()
		return nil, err
	}
	runner := chatapp.ProvideRuntimeRunner(outbox, realtime, publisher, redisClient, pool, zapLogger)
	cleanup := true
	defer func() {
		if cleanup {
			_ = runner.Stop()
		}
	}()

	handler := interfacesgrpc.NewHandler(service, erasureService)
	initServers := interfacesgrpc.NewInitServers(handler)
	grpcOptions, err := iocgrpc.NewServerOptions(v, log)
	if err != nil {
		return nil, err
	}
	grpcRateLimiter := iocgrpc.NewRateLimiter(v, redisClient)
	transportServer, err := iocgrpc.NewServer(grpcOptions, log, initServers, tracer, grpcRateLimiter)
	if err != nil {
		return nil, err
	}
	appOptions, err := chatapp.NewOptions(v, zapLogger)
	if err != nil {
		return nil, err
	}
	application, err := chatapp.NewApp(appOptions, zapLogger, transportServer, runner)
	if err != nil {
		return nil, err
	}
	cleanup = false
	return application, nil
}
