package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"user-service/internal/application/user/command"
	"user-service/internal/application/user/query"
	"user-service/internal/infrastructure/messaging"
	"user-service/internal/infrastructure/persistence"
	usergrpc "user-service/internal/interfaces/grpc"
	"user-service/internal/interfaces/grpc/pb/userpb"
	"user-service/pkg/logger"
	"user-service/pkg/snowflake"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type config struct {
	Service struct {
		Name     string
		GRPCPort int
	}
	Postgres struct {
		DSN   string
		Debug bool
	}
	Kafka struct {
		Brokers []string
		Topic   string
	}
	Snowflake struct {
		WorkerID int64
	}
	JWT struct {
		Secret string
		TTL    time.Duration
	}
	Password struct {
		MinLength int
	}
}

func main() {
	command, args := parseCommand(os.Args[1:])
	configPath := parseConfigPath(command, args)

	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if command == "migrate" {
		if err := runMigrations(cfg.Postgres.DSN); err != nil {
			log.Fatalf("migrate: %v", err)
		}
		log.Printf("user-service migrations applied")
		return
	}

	runServer(cfg)
}

func runServer(cfg *config) {
	zlog := logger.NewNopLogger()
	db, err := gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	if cfg.Postgres.Debug {
		db = db.Debug()
	}
	node, err := snowflake.NewNode(cfg.Snowflake.WorkerID)
	if err != nil {
		log.Fatalf("create snowflake node: %v", err)
	}
	repo := persistence.NewRepo(db)
	publisher := messaging.NewKafkaEventPublisher(cfg.Kafka.Brokers, cfg.Kafka.Topic, zlog)
	defer func() { _ = publisher.Close() }()

	cmd := command.NewService(repo, node, publisher, zlog, cfg.JWT.Secret, cfg.JWT.TTL, cfg.Password.MinLength)
	qry := query.NewService(repo)
	handler := usergrpc.NewHandler(cmd, qry)

	server := grpc.NewServer()
	userpb.RegisterUserServiceServer(server, handler)
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

func parseCommand(args []string) (string, []string) {
	if len(args) == 0 {
		return "server", args
	}
	switch args[0] {
	case "server", "migrate":
		return args[0], args[1:]
	default:
		return "server", args
	}
}

func parseConfigPath(command string, args []string) string {
	fs := flag.NewFlagSet("user-service "+command, flag.ExitOnError)
	configPath := fs.String("config", "configs/config.yaml", "config file path")
	fs.StringVar(configPath, "c", "configs/config.yaml", "config file path")
	_ = fs.Parse(args)
	return *configPath
}

func runMigrations(dsn string) error {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join("migrations", "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if err := db.Exec(string(data)).Error; err != nil {
			return err
		}
		log.Printf("applied migration %s", file)
	}
	return nil
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
		cfg.Service.Name = "user-service"
	}
	if cfg.Service.GRPCPort == 0 {
		cfg.Service.GRPCPort = 9102
	}
	if len(cfg.Kafka.Brokers) == 0 {
		cfg.Kafka.Brokers = []string{"127.0.0.1:9092"}
	}
	if cfg.Kafka.Topic == "" {
		cfg.Kafka.Topic = "user.events"
	}
	if cfg.Snowflake.WorkerID == 0 {
		cfg.Snowflake.WorkerID = 2
	}
	if cfg.JWT.Secret == "" {
		cfg.JWT.Secret = "bbs-local-dev-secret"
	}
	if cfg.JWT.TTL <= 0 {
		cfg.JWT.TTL = 7 * 24 * time.Hour
	}
	if cfg.Password.MinLength <= 0 {
		cfg.Password.MinLength = 8
	}
	return &cfg, nil
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
