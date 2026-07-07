package server

import (
	"os"
	"strings"

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
		cfg.Service.GRPCPort = cfg.GRPC.Server.Port
	}
	if cfg.Service.GRPCPort == 0 {
		cfg.Service.GRPCPort = 9104
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
	bindEnv(v, "grpc.server.port", "BBS_COMMENT_GRPC_SERVER_PORT")
	bindEnv(v, "grpc.server.serviceName", "BBS_COMMENT_GRPC_SERVER_SERVICE_NAME")
	bindEnv(v, "grpc.server.etcdAddr", "BBS_COMMENT_GRPC_SERVER_ETCD_ADDR")
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
	if value := strings.TrimSpace(os.Getenv("BBS_COMMENT_GRPC_SERVER_ETCD_ADDR")); value != "" {
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
