package datasource

import (
	"context"
	"database/sql"
	"notification-service/pkg/gorm/metrics"
	"notification-service/pkg/gorm/tracing"
	"notification-service/pkg/logger"
	"notification-service/pkg/retry"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Options database option
type Options struct {
	Dsn          string `toml:"dsn" json:"dsn" yaml:"dsn" env:"POSTGRES_DSN"`
	Debug        bool   `toml:"debug" json:"debug" yaml:"debug" env:"POSTGRES_DEBUG"`
	MaxOpenConns int    `mapstructure:"max_open_conns" toml:"max_open_conns" json:"max_open_conns" yaml:"max_open_conns" env:"POSTGRES_MAX_OPEN_CONNS"`
	l            logger.Logger
}

// NewOptions new database option
func NewOptions(v *viper.Viper, l logger.Logger) (*Options, error) {
	var (
		err error
		o   = new(Options)
	)
	if err = v.UnmarshalKey("postgres", &o); err != nil {
		return nil, errors.Wrap(err, "unmarshal database option error")
	}
	l.Info("load database options success", logger.Any("database options", o))
	o.l = l
	return o, err
}

func New(o *Options) (*gorm.DB, error) {
	WaitForDBSetup(o.Dsn)
	db, err := gorm.Open(postgres.Open(o.Dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
		Logger: glogger.New(gormLoggerFunc(o.l.Warn), glogger.Config{
			// 慢查询阈值，超过这个阈值，才会使用
			// 50ms 100ms
			// SQL 查询必须要求命中索引，最好就是走一次磁盘 IO
			// 一次磁盘 iO 是不到 10ms
			SlowThreshold:             time.Millisecond * 10,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
			ParameterizedQueries:      true,
			LogLevel:                  glogger.Error,
		}),
	})
	if err != nil {
		return db, errors.Wrap(err, "gorm open postgresql connection error")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return db, errors.Wrap(err, "get postgresql connection pool error")
	}
	configureSQLPool(sqlDB, o)
	//db, _ := Psql.DB()
	//err = db.Ping()
	//if err != nil {
	//	return Psql, errors.Wrap(err, "postgresql ping fail")
	//}
	if o.Debug {
		db = db.Debug()
	}

	tracePlugin := tracing.NewGormTracingPlugin()
	metricsPlugin := metrics.NewGormMetricsPlugin()
	err = db.Use(tracePlugin)
	if err != nil {
		panic(err)
	}
	err = db.Use(metricsPlugin)
	if err != nil {
		panic(err)
	}
	return db, nil
}

const defaultMaxOpenConns = 4

func maxOpenConnections(o *Options) int {
	if o.MaxOpenConns > 0 {
		return o.MaxOpenConns
	}
	return defaultMaxOpenConns
}

func configureSQLPool(db *sql.DB, o *Options) {
	db.SetMaxOpenConns(maxOpenConnections(o))
	db.SetMaxIdleConns(1)
}

func NewPool(ctx context.Context, o *Options) (*pgxpool.Pool, error) {
	WaitForDBSetup(o.Dsn)
	poolConfig, err := newPoolConfig(o)
	if err != nil {
		return nil, errors.Wrap(err, "parse postgresql pool config error")
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, errors.Wrap(err, "pgxpool open postgresql connection error")
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.Wrap(err, "postgresql ping fail")
	}
	return pool, nil
}

func newPoolConfig(o *Options) (*pgxpool.Config, error) {
	config, err := pgxpool.ParseConfig(o.Dsn)
	if err != nil {
		return nil, err
	}
	config.MaxConns = int32(maxOpenConnections(o))
	config.MinConns = 0
	return config, nil
}

func WaitForDBSetup(dsn string) {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		panic(err)
	}
	defer sqlDB.Close()
	const maxInterval = 10 * time.Second
	const maxRetries = 10
	strategy, err := retry.NewRetry(retry.Config{
		Type: "exponential",
		ExponentialBackoff: &retry.ExponentialBackoffConfig{
			time.Second, maxInterval, maxRetries,
		},
	})
	if err != nil {
		panic(err)
	}

	const timeout = 5 * time.Second
	for {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		err = sqlDB.PingContext(ctx)
		cancel()
		if err == nil {
			break
		}
		next, ok := strategy.Next()
		if !ok {
			panic("WaitForDBSetup 重试失败......")
		}
		time.Sleep(next)
	}
}

type gormLoggerFunc func(msg string, fields ...logger.Field)

func (g gormLoggerFunc) Printf(msg string, args ...interface{}) {
	g(msg, logger.Field{Key: "args", Value: args})
}
