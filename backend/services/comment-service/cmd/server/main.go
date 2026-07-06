package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"comment-service/internal/application/comment/command"
	"comment-service/internal/application/comment/query"
	"comment-service/internal/infrastructure/messaging"
	"comment-service/internal/infrastructure/persistence"
	commentgrpc "comment-service/internal/interfaces/grpc"
	"comment-service/internal/interfaces/grpc/pb/commentpb"
	"comment-service/pkg/logger"
	"comment-service/pkg/snowflake"

	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type config struct {
	Service struct {
		Name     string
		GRPCPort int
	}
	Mongo struct {
		URI      string
		Database string
	}
	Kafka struct {
		Brokers []string
		Topic   string
	}
	Snowflake struct {
		WorkerID int64
	}
}

func main() {
	configPath := flag.String("config", "configs/config.yaml", "config file path")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	zlog := logger.NewNopLogger()
	mongoClient, err := connectMongo(cfg.Mongo.URI)
	if err != nil {
		log.Fatalf("connect mongo: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mongoClient.Disconnect(ctx)
	}()

	repo := persistence.NewRepository(mongoClient.Database(cfg.Mongo.Database))
	if err := repo.EnsureIndexes(context.Background()); err != nil {
		log.Fatalf("ensure indexes: %v", err)
	}
	node, err := snowflake.NewNode(cfg.Snowflake.WorkerID)
	if err != nil {
		log.Fatalf("create snowflake node: %v", err)
	}
	publisher := messaging.NewKafkaEventPublisher(cfg.Kafka.Brokers, cfg.Kafka.Topic, zlog)
	defer func() { _ = publisher.Close() }()

	cmd := command.NewService(repo, node, publisher, zlog)
	qry := query.NewService(repo)
	handler := commentgrpc.NewHandler(cmd, qry)

	server := grpc.NewServer()
	commentpb.RegisterCommentServiceServer(server, handler)
	grpc_health_v1.RegisterHealthServer(server, health.NewServer())

	addr := net.JoinHostPort("0.0.0.0", intToString(cfg.Service.GRPCPort))
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s: %v", addr, err)
	}

	go func() {
		log.Printf("%s listening on %s", cfg.Service.Name, addr)
		if err := server.Serve(lis); err != nil {
			log.Printf("grpc server stopped: %v", err)
		}
	}()

	waitForShutdown(server)
}

func connectMongo(uri string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}
	return client, nil
}

func loadConfig(path string) (*config, error) {
	v := viper.New()
	configureEnv(v)
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	applyEnvOverrides(v)
	var cfg config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	if cfg.Service.Name == "" {
		cfg.Service.Name = "comment-service"
	}
	if cfg.Service.GRPCPort == 0 {
		cfg.Service.GRPCPort = 9104
	}
	if cfg.Mongo.URI == "" {
		cfg.Mongo.URI = "mongodb://127.0.0.1:27017"
	}
	if cfg.Mongo.Database == "" {
		cfg.Mongo.Database = "bbs_comment"
	}
	if len(cfg.Kafka.Brokers) == 0 {
		cfg.Kafka.Brokers = []string{"127.0.0.1:9092"}
	}
	if cfg.Kafka.Topic == "" {
		cfg.Kafka.Topic = "comment.events"
	}
	if cfg.Snowflake.WorkerID == 0 {
		cfg.Snowflake.WorkerID = 4
	}
	return &cfg, nil
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_COMMENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "service.name", "BBS_COMMENT_SERVICE_NAME")
	bindEnv(v, "service.grpcPort", "BBS_COMMENT_SERVICE_GRPC_PORT")
	bindEnv(v, "mongo.uri", "BBS_COMMENT_MONGO_URI")
	bindEnv(v, "mongo.database", "BBS_COMMENT_MONGO_DATABASE")
	bindEnv(v, "kafka.brokers", "BBS_COMMENT_KAFKA_BROKERS")
	bindEnv(v, "kafka.topic", "BBS_COMMENT_KAFKA_TOPIC")
	bindEnv(v, "snowflake.workerId", "BBS_COMMENT_SNOWFLAKE_WORKER_ID")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func applyEnvOverrides(v *viper.Viper) {
	if value := strings.TrimSpace(os.Getenv("BBS_COMMENT_KAFKA_BROKERS")); value != "" {
		v.Set("kafka.brokers", splitCommaSeparated(value))
	}
}

func splitCommaSeparated(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func waitForShutdown(server *grpc.Server) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	server.GracefulStop()
}

func intToString(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	n := v
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
