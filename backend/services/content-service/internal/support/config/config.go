package config

import (
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
