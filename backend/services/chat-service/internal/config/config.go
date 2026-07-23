package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/spf13/viper"
)

type Config struct {
	Service struct {
		Name     string `mapstructure:"name"`
		GRPCPort int    `mapstructure:"grpcPort"`
	} `mapstructure:"service"`
	App struct {
		Name string `mapstructure:"name"`
	} `mapstructure:"app"`
	Nacos NacosConfig `mapstructure:"nacos"`
	Log   struct {
		Level string `mapstructure:"level"`
	} `mapstructure:"log"`
	Postgres struct {
		DSN          string `mapstructure:"dsn"`
		MaxOpenConns int32  `mapstructure:"max_open_conns"`
	} `mapstructure:"postgres"`
	Snowflake struct {
		WorkerID int64 `mapstructure:"workerId"`
	} `mapstructure:"snowflake"`
	GRPC struct {
		Server struct {
			Host          string   `mapstructure:"host"`
			Port          int      `mapstructure:"port"`
			AdvertiseHost string   `mapstructure:"advertiseHost"`
			EtcdAddr      []string `mapstructure:"etcdAddr"`
			ServiceName   string   `mapstructure:"serviceName"`
		} `mapstructure:"server"`
	} `mapstructure:"grpc"`
	Kafka struct {
		Brokers         []string `mapstructure:"brokers"`
		Topic           string   `mapstructure:"topic"`
		RealtimeGroupID string   `mapstructure:"realtimeGroupId"`
	} `mapstructure:"kafka"`
	Redis struct {
		Addr     string `mapstructure:"addr"`
		Password string `mapstructure:"password"`
		DB       int    `mapstructure:"db"`
	} `mapstructure:"redis"`
}

type NacosConfig struct {
	Addr        string `mapstructure:"addr"`
	Port        uint64 `mapstructure:"port"`
	NamespaceID string `mapstructure:"namespaceId"`
	DataID      string `mapstructure:"dataId"`
	GroupID     string `mapstructure:"groupId"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read chat config: %w", err)
	}

	var nacosConfig NacosConfig
	if err := v.UnmarshalKey("nacos", &nacosConfig); err != nil {
		return nil, fmt.Errorf("decode nacos config: %w", err)
	}
	if !skipNacos() {
		if err := mergeNacos(v, nacosConfig); err != nil {
			return nil, err
		}
	}
	if err := applyEnvironment(v); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("decode chat config: %w", err)
	}
	applyDefaults(&cfg)
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func mergeNacos(v *viper.Viper, cfg NacosConfig) error {
	if strings.TrimSpace(cfg.Addr) == "" || cfg.Port == 0 || strings.TrimSpace(cfg.DataID) == "" {
		return errors.New("complete nacos configuration is required")
	}
	client, err := clients.CreateConfigClient(map[string]any{
		"serverConfigs": []constant.ServerConfig{{IpAddr: cfg.Addr, Port: cfg.Port}},
		"clientConfig": constant.ClientConfig{
			NamespaceId:         cfg.NamespaceID,
			TimeoutMs:           5000,
			NotLoadCacheAtStart: true,
			LogDir:              "tmp/nacos/log",
			CacheDir:            "tmp/nacos/cache",
			LogLevel:            "info",
		},
	})
	if err != nil {
		return fmt.Errorf("create nacos config client: %w", err)
	}
	group := strings.TrimSpace(cfg.GroupID)
	if group == "" {
		group = "DEFAULT_GROUP"
	}
	content, err := client.GetConfig(vo.ConfigParam{DataId: cfg.DataID, Group: group})
	if err != nil {
		return fmt.Errorf("load chat config from nacos: %w", err)
	}
	if strings.TrimSpace(content) == "" {
		return errors.New("chat nacos configuration is empty")
	}
	if err := v.MergeConfig(bytes.NewBufferString(content)); err != nil {
		return fmt.Errorf("merge chat nacos config: %w", err)
	}
	return nil
}

func applyEnvironment(v *viper.Viper) error {
	stringOverrides := map[string]string{
		"BBS_CHAT_POSTGRES_DSN":        "postgres.dsn",
		"BBS_CHAT_GRPC_HOST":           "grpc.server.host",
		"BBS_CHAT_GRPC_ADVERTISE_HOST": "grpc.server.advertiseHost",
		"BBS_CHAT_GRPC_SERVICE_NAME":   "grpc.server.serviceName",
		"BBS_CHAT_REDIS_ADDR":          "redis.addr",
		"BBS_CHAT_REDIS_PASSWORD":      "redis.password",
		"BBS_CHAT_KAFKA_TOPIC":         "kafka.topic",
	}
	for environment, key := range stringOverrides {
		if value := strings.TrimSpace(os.Getenv(environment)); value != "" {
			v.Set(key, value)
		}
	}
	listOverrides := map[string]string{
		"BBS_CHAT_ETCD_ADDR":     "grpc.server.etcdAddr",
		"BBS_CHAT_KAFKA_BROKERS": "kafka.brokers",
	}
	for environment, key := range listOverrides {
		if value := splitCSV(os.Getenv(environment)); len(value) > 0 {
			v.Set(key, value)
		}
	}
	integerOverrides := map[string]string{
		"BBS_CHAT_GRPC_PORT":               "grpc.server.port",
		"BBS_CHAT_SNOWFLAKE_WORKER_ID":     "snowflake.workerId",
		"BBS_CHAT_POSTGRES_MAX_OPEN_CONNS": "postgres.max_open_conns",
		"BBS_CHAT_REDIS_DB":                "redis.db",
	}
	for environment, key := range integerOverrides {
		value := strings.TrimSpace(os.Getenv(environment))
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("parse %s: %w", environment, err)
		}
		v.Set(key, parsed)
	}
	return nil
}

func applyDefaults(cfg *Config) {
	if cfg.Service.Name == "" {
		cfg.Service.Name = "bbs-chat-service"
	}
	if cfg.App.Name == "" {
		cfg.App.Name = cfg.Service.Name
	}
	if cfg.GRPC.Server.ServiceName == "" {
		cfg.GRPC.Server.ServiceName = cfg.Service.Name
	}
	if cfg.GRPC.Server.Port == 0 {
		cfg.GRPC.Server.Port = cfg.Service.GRPCPort
	}
	if cfg.GRPC.Server.Port == 0 {
		cfg.GRPC.Server.Port = 9116
	}
	if cfg.GRPC.Server.Host == "" {
		cfg.GRPC.Server.Host = "0.0.0.0"
	}
	if cfg.Postgres.MaxOpenConns <= 0 {
		cfg.Postgres.MaxOpenConns = 8
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Kafka.Topic == "" {
		cfg.Kafka.Topic = "chat.events"
	}
	if cfg.Kafka.RealtimeGroupID == "" {
		cfg.Kafka.RealtimeGroupID = "bbs-chat-realtime"
	}
}

func validate(cfg *Config) error {
	if strings.TrimSpace(cfg.Postgres.DSN) == "" {
		return errors.New("chat postgres dsn is required")
	}
	if len(cfg.GRPC.Server.EtcdAddr) == 0 {
		return errors.New("chat etcd addresses are required")
	}
	if cfg.Snowflake.WorkerID < 0 || cfg.Snowflake.WorkerID > 1023 {
		return errors.New("chat snowflake worker id must be between 0 and 1023")
	}
	return nil
}

func skipNacos() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("BBS_CHAT_SKIP_NACOS"))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}
