package server

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type config struct {
	Service struct {
		Name     string
		GRPCPort int
	}
	GRPC struct {
		Server struct {
			Port        int
			ServiceName string
			EtcdAddr    []string
		}
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
	Trace struct {
		Env string
	}
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
		cfg.Service.Name = "bbs-user-service"
	}
	if cfg.Service.GRPCPort == 0 {
		cfg.Service.GRPCPort = cfg.GRPC.Server.Port
	}
	if cfg.Service.GRPCPort == 0 {
		cfg.Service.GRPCPort = 9102
	}
	if cfg.GRPC.Server.Port == 0 {
		cfg.GRPC.Server.Port = cfg.Service.GRPCPort
	}
	if cfg.GRPC.Server.ServiceName == "" {
		cfg.GRPC.Server.ServiceName = cfg.Service.Name
	}
	if len(cfg.GRPC.Server.EtcdAddr) == 0 {
		cfg.GRPC.Server.EtcdAddr = []string{"127.0.0.1:2379"}
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
	if isProductionEnvironment(cfg.Trace.Env) && cfg.JWT.Secret == "bbs-local-dev-secret" {
		return nil, fmt.Errorf("jwt.secret must be set to a non-default value in production")
	}
	if cfg.JWT.TTL <= 0 {
		cfg.JWT.TTL = 7 * 24 * time.Hour
	}
	if cfg.Password.MinLength <= 0 {
		cfg.Password.MinLength = 8
	}
	return &cfg, nil
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_USER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "service.name", "BBS_USER_SERVICE_NAME")
	bindEnv(v, "service.grpcPort", "BBS_USER_SERVICE_GRPC_PORT")
	bindEnv(v, "grpc.server.port", "BBS_USER_GRPC_SERVER_PORT")
	bindEnv(v, "grpc.server.serviceName", "BBS_USER_GRPC_SERVER_SERVICE_NAME")
	bindEnv(v, "grpc.server.etcdAddr", "BBS_USER_GRPC_SERVER_ETCD_ADDR")
	bindEnv(v, "postgres.dsn", "BBS_USER_POSTGRES_DSN")
	bindEnv(v, "postgres.debug", "BBS_USER_POSTGRES_DEBUG")
	bindEnv(v, "kafka.brokers", "BBS_USER_KAFKA_BROKERS")
	bindEnv(v, "kafka.topic", "BBS_USER_KAFKA_TOPIC")
	bindEnv(v, "snowflake.workerId", "BBS_USER_SNOWFLAKE_WORKER_ID")
	bindEnv(v, "jwt.secret", "BBS_USER_JWT_SECRET")
	bindEnv(v, "jwt.ttl", "BBS_USER_JWT_TTL")
	bindEnv(v, "password.minLength", "BBS_USER_PASSWORD_MIN_LENGTH")
	bindEnv(v, "trace.env", "BBS_USER_TRACE_ENV")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func applyEnvOverrides(v *viper.Viper) {
	if value := strings.TrimSpace(os.Getenv("BBS_USER_KAFKA_BROKERS")); value != "" {
		v.Set("kafka.brokers", splitCommaSeparated(value))
	}
	if value := strings.TrimSpace(os.Getenv("BBS_USER_GRPC_SERVER_ETCD_ADDR")); value != "" {
		v.Set("grpc.server.etcdAddr", splitCommaSeparated(value))
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

func isProductionEnvironment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}
