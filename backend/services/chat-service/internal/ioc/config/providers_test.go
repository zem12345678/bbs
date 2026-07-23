package config_test

import (
	"reflect"
	"testing"

	"chat-service/internal/ioc/config"
	datasource "chat-service/internal/ioc/db/postgres"
	iocgrpc "chat-service/internal/ioc/grpc"
	iockafka "chat-service/internal/ioc/kafka"
	ioclogger "chat-service/internal/ioc/logger"
	iocredis "chat-service/internal/ioc/redis"
	ioctrace "chat-service/internal/ioc/trace"
	"chat-service/pkg/logger"

	"go.uber.org/zap"
)

func TestLocalConfigMapsIntoIoCProviderOptions(t *testing.T) {
	t.Setenv("BBS_CHAT_SKIP_NACOS", "true")
	t.Setenv("BBS_CHAT_GRPC_SERVER_PORT", "19116")

	v, err := config.New("../../../configs/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	log := logger.NewNopLogger()

	redisOptions, err := iocredis.NewOptions(v, log)
	if err != nil {
		t.Fatal(err)
	}
	if redisOptions.URL != "127.0.0.1:6379" || redisOptions.Network != "tcp" {
		t.Fatalf("redis options = %#v", redisOptions)
	}

	grpcOptions, err := iocgrpc.NewServerOptions(v, log)
	if err != nil {
		t.Fatal(err)
	}
	if grpcOptions.Port != 19116 || !reflect.DeepEqual(grpcOptions.EtcdAddr, []string{"127.0.0.1:2379"}) {
		t.Fatalf("grpc options = %#v", grpcOptions)
	}

	dbOptions, err := datasource.NewOptions(v, log)
	if err != nil {
		t.Fatal(err)
	}
	if dbOptions.Dsn == "" || dbOptions.MaxOpenConns != 8 {
		t.Fatalf("postgres options = %#v", dbOptions)
	}

	producerOptions, err := iockafka.NewProducerOptions(v, log)
	if err != nil {
		t.Fatal(err)
	}
	consumerOptions, err := iockafka.NewConsumerOptions(v, log)
	if err != nil {
		t.Fatal(err)
	}
	if producerOptions.Topic != "chat.events" || !reflect.DeepEqual(producerOptions.Brokers, []string{"127.0.0.1:9092"}) {
		t.Fatalf("producer options = %#v", producerOptions)
	}
	if consumerOptions.GroupID != "bbs-chat-realtime" || !reflect.DeepEqual(consumerOptions.Topics, []string{"chat.events"}) {
		t.Fatalf("consumer options = %#v", consumerOptions)
	}

	logOptions, err := ioclogger.NewOptions(v)
	if err != nil {
		t.Fatal(err)
	}
	if logOptions.Filename == "" || !logOptions.Stdout {
		t.Fatalf("logger options = %#v", logOptions)
	}
	traceOptions, err := ioctrace.NewOptions(v, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if traceOptions.GrpcEndpoint != "127.0.0.1:4317" || traceOptions.ServiceName != "bbs-chat-service" {
		t.Fatalf("trace options = %#v", traceOptions)
	}
}
