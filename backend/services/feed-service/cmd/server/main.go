package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"feed-service/internal/application/feed/query"
	"feed-service/internal/infrastructure/messaging"
	"feed-service/internal/infrastructure/persistence"
	feedgrpc "feed-service/internal/interfaces/grpc"
	"feed-service/internal/interfaces/grpc/pb/feedpb"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type config struct {
	Service struct {
		Name     string
		GRPCPort int
	}
	Redis struct {
		Addr     string
		DB       int
		Password string
	}
	Kafka struct {
		Brokers         []string
		ArticleTopic    string
		CommentTopic    string
		ReactionTopic   string
		ArticleGroupID  string
		CommentGroupID  string
		ReactionGroupID string
	}
}

type eventConsumer interface {
	Start(ctx context.Context) error
	Close() error
}

func main() {
	configPath := flag.String("config", "configs/config.yaml", "config file path")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, DB: cfg.Redis.DB, Password: cfg.Redis.Password})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("ping redis: %v", err)
	}
	defer func() { _ = rdb.Close() }()

	repo := persistence.NewRedisRepository(rdb)
	qry := query.NewService(repo)
	handler := feedgrpc.NewHandler(qry)

	consumerCtx, stopConsumers := context.WithCancel(context.Background())
	consumers := []eventConsumer{
		messaging.NewArticleConsumer(messaging.ArticleConsumerOptions{Brokers: cfg.Kafka.Brokers, Topic: cfg.Kafka.ArticleTopic, GroupID: cfg.Kafka.ArticleGroupID}, repo),
		messaging.NewCommentConsumer(messaging.CommentConsumerOptions{Brokers: cfg.Kafka.Brokers, Topic: cfg.Kafka.CommentTopic, GroupID: cfg.Kafka.CommentGroupID}, repo),
		messaging.NewReactionConsumer(messaging.ReactionConsumerOptions{Brokers: cfg.Kafka.Brokers, Topic: cfg.Kafka.ReactionTopic, GroupID: cfg.Kafka.ReactionGroupID}, repo),
	}
	startConsumers(consumerCtx, consumers)

	server := grpc.NewServer()
	feedpb.RegisterFeedServiceServer(server, handler)
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

	waitForShutdown(server, stopConsumers, consumers)
}

func loadConfig(path string) (*config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var cfg config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	if cfg.Service.Name == "" {
		cfg.Service.Name = "feed-service"
	}
	if cfg.Service.GRPCPort == 0 {
		cfg.Service.GRPCPort = 9113
	}
	if cfg.Redis.Addr == "" {
		cfg.Redis.Addr = "127.0.0.1:6379"
	}
	if len(cfg.Kafka.Brokers) == 0 {
		cfg.Kafka.Brokers = []string{"127.0.0.1:9092"}
	}
	if cfg.Kafka.ArticleTopic == "" {
		cfg.Kafka.ArticleTopic = "article.events"
	}
	if cfg.Kafka.CommentTopic == "" {
		cfg.Kafka.CommentTopic = "comment.events"
	}
	if cfg.Kafka.ReactionTopic == "" {
		cfg.Kafka.ReactionTopic = "reaction.events"
	}
	if cfg.Kafka.ArticleGroupID == "" {
		cfg.Kafka.ArticleGroupID = "bbs-feed-article-projector"
	}
	if cfg.Kafka.CommentGroupID == "" {
		cfg.Kafka.CommentGroupID = "bbs-feed-comment-projector"
	}
	if cfg.Kafka.ReactionGroupID == "" {
		cfg.Kafka.ReactionGroupID = "bbs-feed-reaction-projector"
	}
	return &cfg, nil
}

func startConsumers(ctx context.Context, consumers []eventConsumer) {
	for _, consumer := range consumers {
		consumer := consumer
		go func() {
			if err := consumer.Start(ctx); err != nil {
				log.Printf("event consumer stopped: %v", err)
			}
		}()
	}
}

func waitForShutdown(server *grpc.Server, stopConsumers context.CancelFunc, consumers []eventConsumer) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	stopConsumers()
	for _, consumer := range consumers {
		if err := consumer.Close(); err != nil {
			log.Printf("close event consumer: %v", err)
		}
	}
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
