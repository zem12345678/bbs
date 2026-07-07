package redis

import (
	"credit-service/pkg/logger"
	"credit-service/pkg/redis/metrics"
	"credit-service/pkg/redis/tracing"
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
	URL         string // host:port
	DBNum       int
	MaxIdle     int    // 最大空闲连接数
	MaxActive   int    // 最大连接数
	IdleTimeout int    // 空闲连接超时时间
	Timeout     int    // 操作超时时间
	Network     string // tcp or udp
	Password    string
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

	l.Info("load redis options success", logger.Any("redis options", o))
	return o, err
}

// New redis pool conn
func New(o *Options) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         o.URL,
		Password:     o.Password, // no password set
		DB:           o.DBNum,    // use default DB
		MaxIdleConns: o.MaxIdle,
		PoolSize:     o.MaxActive,
		DialTimeout:  time.Duration(o.IdleTimeout) * time.Second,
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

var ProviderSet = wire.NewSet(New, NewOptions)
