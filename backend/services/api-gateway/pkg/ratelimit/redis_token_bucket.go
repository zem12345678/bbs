package ratelimit

import (
	"context"
	_ "embed"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed token_bucket.lua
var luaTokenBucket string

type RedisTokenBucketLimiter struct {
	cmd            redis.Cmdable
	capacity       int
	refillInterval time.Duration
}

func NewRedisTokenBucketLimiter(cmd redis.Cmdable, capacity int, refillInterval time.Duration) Limiter {
	return &RedisTokenBucketLimiter{cmd: cmd, capacity: capacity, refillInterval: refillInterval}
}

func (r *RedisTokenBucketLimiter) Limit(ctx context.Context, key string) (bool, error) {
	return r.cmd.Eval(
		ctx,
		luaTokenBucket,
		[]string{key},
		r.capacity,
		r.refillInterval.Milliseconds(),
		time.Now().UnixMilli(),
	).Bool()
}
