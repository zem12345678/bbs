package redis

import (
	"chat-service/pkg/logger"
	"chat-service/pkg/redis/metrics"
	"chat-service/pkg/redis/tracing"
	"context"
	"errors"
	"time"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

// / ClientType 定义redis client 结构体
type RedisDB struct {
	*redis.Client
}

// Client  redis连接类型
var RedisClient RedisDB

// Options redis option
type Options struct {
	URL         string `mapstructure:"url"`
	Addr        string `mapstructure:"addr"`
	DBNum       int    `mapstructure:"dbNum"`
	DB          int    `mapstructure:"db"`
	MaxIdle     int    `mapstructure:"maxIdle"`
	MaxActive   int    `mapstructure:"maxActive"`
	IdleTimeout int    `mapstructure:"idleTimeout"`
	Timeout     int    `mapstructure:"timeout"`
	Network     string `mapstructure:"network"`
	Password    string `mapstructure:"password"`
}

// NewOptions for redis
func NewOptions(v *viper.Viper, l logger.Logger) (*Options, error) {
	var (
		err error
		o   = new(Options)
	)
	if err = v.UnmarshalKey("redis", o); err != nil {
		return nil, errors.New("unmarshal redis option error")
	}
	o.URL = v.GetString("redis.url")
	o.Addr = v.GetString("redis.addr")
	o.DBNum = v.GetInt("redis.dbNum")
	o.DB = v.GetInt("redis.db")
	o.MaxIdle = v.GetInt("redis.maxIdle")
	o.MaxActive = v.GetInt("redis.maxActive")
	o.IdleTimeout = v.GetInt("redis.idleTimeout")
	o.Timeout = v.GetInt("redis.timeout")
	o.Network = v.GetString("redis.network")
	o.Password = v.GetString("redis.password")
	if o.URL == "" {
		o.URL = o.Addr
	}
	l.Info("load redis options success", logger.String("addr", o.URL), logger.Int("db", o.DBNum))
	return o, err
}

// New redis pool conn
func New(o *Options) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Network:      o.Network,
		Addr:         o.URL,
		Password:     o.Password,
		DB:           o.DBNum,
		MaxIdleConns: o.MaxIdle,
		PoolSize:     o.MaxActive,
		DialTimeout:  time.Duration(o.IdleTimeout) * time.Second,
		ReadTimeout:  time.Duration(o.Timeout) * time.Second,
		WriteTimeout: time.Duration(o.Timeout) * time.Second,
	})
	rdb = tracing.WithTracing(rdb)
	rdb = metrics.WithMetrics(rdb)
	//if err := redisotel.InstrumentTracing(rdb); err != nil {
	//	return nil, err
	//}
	//if err := redisotel.InstrumentMetrics(rdb); err != nil {
	//	return nil, err
	//}
	return rdb, nil
}

func Ping(ctx context.Context, client *redis.Client) error {
	return client.Ping(ctx).Err()
}

var ProviderSet = wire.NewSet(New, NewOptions)
