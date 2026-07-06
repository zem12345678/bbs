package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	app "notification-service/internal/application/notification"
	"notification-service/internal/infrastructure/messaging"
	"notification-service/internal/infrastructure/persistence"
	notificationgrpc "notification-service/internal/interfaces/grpc"
	"notification-service/internal/interfaces/grpc/pb/notificationpb"

	"github.com/jackc/pgx/v5/pgxpool"
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
	Postgres struct {
		DSN string
	}
	Kafka struct {
		Brokers         []string
		UserTopic       string
		ArticleTopic    string
		CommentTopic    string
		ReactionTopic   string
		UserGroupID     string
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

	pool, err := pgxpool.New(context.Background(), cfg.Postgres.DSN)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("ping postgres: %v", err)
	}

	repo := persistence.NewPostgresRepository(pool)
	if err := repo.EnsureSchema(context.Background()); err != nil {
		log.Fatalf("ensure schema: %v", err)
	}
	service := app.NewService(repo)
	projector := messaging.NewProjector(service)

	consumerCtx, stopConsumers := context.WithCancel(context.Background())
	consumers := []eventConsumer{
		messaging.NewConsumer(messaging.ConsumerOptions{Brokers: cfg.Kafka.Brokers, Topic: cfg.Kafka.ArticleTopic, GroupID: cfg.Kafka.ArticleGroupID, Name: "article"}, projector.HandleArticle),
		messaging.NewConsumer(messaging.ConsumerOptions{Brokers: cfg.Kafka.Brokers, Topic: cfg.Kafka.UserTopic, GroupID: cfg.Kafka.UserGroupID, Name: "user"}, projector.HandleUser),
		messaging.NewConsumer(messaging.ConsumerOptions{Brokers: cfg.Kafka.Brokers, Topic: cfg.Kafka.CommentTopic, GroupID: cfg.Kafka.CommentGroupID, Name: "comment"}, projector.HandleComment),
		messaging.NewConsumer(messaging.ConsumerOptions{Brokers: cfg.Kafka.Brokers, Topic: cfg.Kafka.ReactionTopic, GroupID: cfg.Kafka.ReactionGroupID, Name: "reaction"}, projector.HandleReaction),
	}
	startConsumers(consumerCtx, consumers)

	server := grpc.NewServer()
	notificationpb.RegisterNotificationServiceServer(server, notificationgrpc.NewHandler(service))
	grpc_health_v1.RegisterHealthServer(server, health.NewServer())

	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(cfg.Service.GRPCPort))
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
		cfg.Service.Name = "notification-service"
	}
	if cfg.Service.GRPCPort == 0 {
		cfg.Service.GRPCPort = 9108
	}
	if cfg.Postgres.DSN == "" {
		cfg.Postgres.DSN = "postgres://bbs_notification_app:local_notification_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_notification"
	}
	if len(cfg.Kafka.Brokers) == 0 {
		cfg.Kafka.Brokers = []string{"127.0.0.1:9092"}
	}
	if cfg.Kafka.UserTopic == "" {
		cfg.Kafka.UserTopic = "user.events"
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
	if cfg.Kafka.UserGroupID == "" {
		cfg.Kafka.UserGroupID = "bbs-notification-user-consumer"
	}
	if cfg.Kafka.ArticleGroupID == "" {
		cfg.Kafka.ArticleGroupID = "bbs-notification-article-consumer"
	}
	if cfg.Kafka.CommentGroupID == "" {
		cfg.Kafka.CommentGroupID = "bbs-notification-comment-consumer"
	}
	if cfg.Kafka.ReactionGroupID == "" {
		cfg.Kafka.ReactionGroupID = "bbs-notification-reaction-consumer"
	}
	return &cfg, nil
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_NOTIFICATION")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "service.name", "BBS_NOTIFICATION_SERVICE_NAME")
	bindEnv(v, "service.grpcPort", "BBS_NOTIFICATION_SERVICE_GRPC_PORT")
	bindEnv(v, "postgres.dsn", "BBS_NOTIFICATION_POSTGRES_DSN")
	bindEnv(v, "kafka.brokers", "BBS_NOTIFICATION_KAFKA_BROKERS")
	bindEnv(v, "kafka.userTopic", "BBS_NOTIFICATION_KAFKA_USER_TOPIC")
	bindEnv(v, "kafka.articleTopic", "BBS_NOTIFICATION_KAFKA_ARTICLE_TOPIC")
	bindEnv(v, "kafka.commentTopic", "BBS_NOTIFICATION_KAFKA_COMMENT_TOPIC")
	bindEnv(v, "kafka.reactionTopic", "BBS_NOTIFICATION_KAFKA_REACTION_TOPIC")
	bindEnv(v, "kafka.userGroupId", "BBS_NOTIFICATION_KAFKA_USER_GROUP_ID")
	bindEnv(v, "kafka.articleGroupId", "BBS_NOTIFICATION_KAFKA_ARTICLE_GROUP_ID")
	bindEnv(v, "kafka.commentGroupId", "BBS_NOTIFICATION_KAFKA_COMMENT_GROUP_ID")
	bindEnv(v, "kafka.reactionGroupId", "BBS_NOTIFICATION_KAFKA_REACTION_GROUP_ID")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func applyEnvOverrides(v *viper.Viper) {
	if value := strings.TrimSpace(os.Getenv("BBS_NOTIFICATION_KAFKA_BROKERS")); value != "" {
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
