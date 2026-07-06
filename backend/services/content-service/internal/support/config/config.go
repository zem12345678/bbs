package config

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Service struct {
		Name     string `mapstructure:"name"`
		GRPCPort int    `mapstructure:"grpcPort"`
	} `mapstructure:"service"`
	Postgres struct {
		DSN   string `mapstructure:"dsn"`
		Debug bool   `mapstructure:"debug"`
	} `mapstructure:"postgres"`
	Redis struct {
		Addr     string `mapstructure:"addr"`
		DB       int    `mapstructure:"db"`
		Password string `mapstructure:"password"`
	} `mapstructure:"redis"`
	Kafka struct {
		Brokers []string `mapstructure:"brokers"`
		Topic   string   `mapstructure:"topic"`
	} `mapstructure:"kafka"`
	Cache struct {
		TTL time.Duration `mapstructure:"ttl"`
	} `mapstructure:"cache"`
	Snowflake struct {
		WorkerID int64 `mapstructure:"workerId"`
	} `mapstructure:"snowflake"`
}

func New(path string) (*Config, error) {
	v := viper.New()
	configureEnv(v)
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	applyEnvOverrides(v)
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	cfg.setDefaults()
	return &cfg, nil
}

func (c *Config) setDefaults() {
	if c.Service.Name == "" {
		c.Service.Name = "content-service"
	}
	if c.Service.GRPCPort == 0 {
		c.Service.GRPCPort = 9103
	}
	if c.Redis.Addr == "" {
		c.Redis.Addr = "127.0.0.1:6379"
	}
	if c.Cache.TTL <= 0 {
		c.Cache.TTL = 5 * time.Minute
	}
	if c.Snowflake.WorkerID == 0 {
		c.Snowflake.WorkerID = 3
	}
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_CONTENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "service.name", "BBS_CONTENT_SERVICE_NAME")
	bindEnv(v, "service.grpcPort", "BBS_CONTENT_SERVICE_GRPC_PORT")
	bindEnv(v, "postgres.dsn", "BBS_CONTENT_POSTGRES_DSN")
	bindEnv(v, "postgres.debug", "BBS_CONTENT_POSTGRES_DEBUG")
	bindEnv(v, "redis.addr", "BBS_CONTENT_REDIS_ADDR")
	bindEnv(v, "redis.db", "BBS_CONTENT_REDIS_DB")
	bindEnv(v, "redis.password", "BBS_CONTENT_REDIS_PASSWORD")
	bindEnv(v, "kafka.brokers", "BBS_CONTENT_KAFKA_BROKERS")
	bindEnv(v, "kafka.topic", "BBS_CONTENT_KAFKA_TOPIC")
	bindEnv(v, "cache.ttl", "BBS_CONTENT_CACHE_TTL")
	bindEnv(v, "snowflake.workerId", "BBS_CONTENT_SNOWFLAKE_WORKER_ID")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func applyEnvOverrides(v *viper.Viper) {
	if value := strings.TrimSpace(os.Getenv("BBS_CONTENT_KAFKA_BROKERS")); value != "" {
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
