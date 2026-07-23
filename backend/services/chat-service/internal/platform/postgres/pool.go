package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, dsn string, maxOpenConns int32) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse chat postgres dsn: %w", err)
	}
	if maxOpenConns > 0 {
		config.MaxConns = maxOpenConns
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open chat postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping chat postgres: %w", err)
	}
	return pool, nil
}
