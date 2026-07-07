package cache

import (
	"context"

	"reaction-service/internal/infrastructure/store"
	"reaction-service/internal/ioc/config"
	datasource "reaction-service/internal/ioc/db/postgres"
	ioclogger "reaction-service/internal/ioc/logger"
	iocredis "reaction-service/internal/ioc/redis"

	"github.com/redis/go-redis/v9"
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

func CreateRebuilder(configFile string) (*Rebuilder, error) {
	v, err := config.New(configFile)
	if err != nil {
		return nil, err
	}
	logOptions, err := ioclogger.NewOptions(v)
	if err != nil {
		return nil, err
	}
	log, err := ioclogger.New(logOptions)
	if err != nil {
		return nil, err
	}
	dbOptions, err := datasource.NewOptions(v, log)
	if err != nil {
		return nil, err
	}
	db, err := datasource.New(dbOptions)
	if err != nil {
		return nil, err
	}
	redisOptions, err := iocredis.NewOptions(v, log)
	if err != nil {
		return nil, err
	}
	rdb, err := iocredis.New(redisOptions)
	if err != nil {
		return nil, err
	}
	return NewRebuilder(db, rdb), nil
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
