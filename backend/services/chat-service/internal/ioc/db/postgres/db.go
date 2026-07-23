package datasource

import (
	"context"
	"database/sql"
	"time"

	"chat-service/pkg/logger"
	"chat-service/pkg/retry"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const defaultMaxOpenConns = 8

type Options struct {
	Dsn          string `mapstructure:"dsn" toml:"dsn" json:"dsn" yaml:"dsn" env:"POSTGRES_DSN"`
	MaxOpenConns int    `mapstructure:"max_open_conns" toml:"max_open_conns" json:"max_open_conns" yaml:"max_open_conns" env:"POSTGRES_MAX_OPEN_CONNS"`
}

func NewOptions(v *viper.Viper, l logger.Logger) (*Options, error) {
	o := new(Options)
	if err := v.UnmarshalKey("postgres", o); err != nil {
		return nil, errors.Wrap(err, "unmarshal database option error")
	}
	o.Dsn = v.GetString("postgres.dsn")
	o.MaxOpenConns = v.GetInt("postgres.max_open_conns")
	l.Info("load database options success")
	return o, nil
}

func NewPool(ctx context.Context, o *Options) (*pgxpool.Pool, error) {
	if err := WaitForDBSetup(ctx, o.Dsn); err != nil {
		return nil, err
	}
	poolConfig, err := pgxpool.ParseConfig(o.Dsn)
	if err != nil {
		return nil, errors.Wrap(err, "parse postgresql pool config error")
	}
	poolConfig.MaxConns = int32(maxOpenConnections(o))
	poolConfig.MinConns = 0
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

func maxOpenConnections(o *Options) int {
	if o != nil && o.MaxOpenConns > 0 {
		return o.MaxOpenConns
	}
	return defaultMaxOpenConns
}

func WaitForDBSetup(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return errors.Wrap(err, "open postgresql connection")
	}
	defer db.Close()

	strategy, err := retry.NewRetry(retry.Config{
		Type: "exponential",
		ExponentialBackoff: &retry.ExponentialBackoffConfig{
			InitialInterval: time.Second,
			MaxInterval:     10 * time.Second,
			MaxRetries:      10,
		},
	})
	if err != nil {
		return err
	}
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = db.PingContext(pingCtx)
		cancel()
		if err == nil {
			return nil
		}
		delay, ok := strategy.Next()
		if !ok {
			return errors.Wrap(err, "wait for postgresql setup")
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
