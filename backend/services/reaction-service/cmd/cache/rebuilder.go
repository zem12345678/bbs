package cache

import (
	"context"

	"reaction-service/internal/infrastructure/store"
	"reaction-service/internal/support/config"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Rebuilder struct {
	cache *store.ReactionCacheRebuilder
	db    *gorm.DB
	rdb   *redis.Client
}

func NewRebuilder(db *gorm.DB, rdb *redis.Client) *Rebuilder {
	return &Rebuilder{cache: store.NewReactionCacheRebuilder(db, rdb), db: db, rdb: rdb}
}

func provideDB(cfg *config.Config) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{})
}

func provideRedisClient(ctx context.Context, cfg *config.Config) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, DB: cfg.Redis.DB, Password: cfg.Redis.Password})
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, err
	}
	return rdb, nil
}

func (r *Rebuilder) Rebuild(ctx context.Context) (store.ReactionCacheRebuildStats, error) {
	return r.cache.Rebuild(ctx)
}

func (r *Rebuilder) Verify(ctx context.Context) error {
	return r.cache.Verify(ctx)
}

func (r *Rebuilder) Close() {
	if r.rdb != nil {
		_ = r.rdb.Close()
	}
	if r.db == nil {
		return
	}
	sqlDB, err := r.db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}
