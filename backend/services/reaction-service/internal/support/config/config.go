package config

import "github.com/spf13/viper"

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
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
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
	if c.Kafka.Topic == "" {
		c.Kafka.Topic = "reaction.events"
	}
}
