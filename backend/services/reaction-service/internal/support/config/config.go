package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Service struct {
		Name     string `mapstructure:"name"`
		GRPCPort int    `mapstructure:"grpcPort"`
	} `mapstructure:"service"`
	Postgres struct {
		DSN string `mapstructure:"dsn"`
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
	Reaction struct {
		RebuildCacheOnStart bool `mapstructure:"rebuildCacheOnStart"`
	} `mapstructure:"reaction"`
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
		c.Service.Name = "reaction-service"
	}
	if c.Service.GRPCPort == 0 {
		c.Service.GRPCPort = 9105
	}
	if c.Redis.Addr == "" {
		c.Redis.Addr = "127.0.0.1:6379"
	}
	if c.Postgres.DSN == "" {
		c.Postgres.DSN = "postgres://bbs_reaction_app:local_reaction_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_reaction"
	}
	if len(c.Kafka.Brokers) == 0 {
		c.Kafka.Brokers = []string{"127.0.0.1:9092"}
	}
	if c.Kafka.Topic == "" {
		c.Kafka.Topic = "reaction.events"
	}
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_REACTION")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "service.name", "BBS_REACTION_SERVICE_NAME")
	bindEnv(v, "service.grpcPort", "BBS_REACTION_SERVICE_GRPC_PORT")
	bindEnv(v, "postgres.dsn", "BBS_REACTION_POSTGRES_DSN")
	bindEnv(v, "redis.addr", "BBS_REACTION_REDIS_ADDR")
	bindEnv(v, "redis.db", "BBS_REACTION_REDIS_DB")
	bindEnv(v, "redis.password", "BBS_REACTION_REDIS_PASSWORD")
	bindEnv(v, "kafka.brokers", "BBS_REACTION_KAFKA_BROKERS")
	bindEnv(v, "kafka.topic", "BBS_REACTION_KAFKA_TOPIC")
	bindEnv(v, "reaction.rebuildCacheOnStart", "BBS_REACTION_REBUILD_CACHE_ON_START")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func applyEnvOverrides(v *viper.Viper) {
	if value := strings.TrimSpace(os.Getenv("BBS_REACTION_KAFKA_BROKERS")); value != "" {
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
