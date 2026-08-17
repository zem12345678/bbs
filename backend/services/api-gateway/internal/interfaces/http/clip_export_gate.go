package http

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	errClipExportRateLimited = errors.New("clip export rate limit exceeded")
	errClipExportInProgress  = errors.New("clip export already in progress")
)

const clipExportKeyPrefix = "rate:exports:clips:user:"

type ClipExportGate interface {
	Begin(context.Context, int64) (ClipExportPermit, error)
}

type ClipExportPermit interface {
	Commit(context.Context) error
	Release(context.Context) error
}

type redisClipExportGate struct {
	redis    redis.Cmdable
	interval time.Duration
	lockTTL  time.Duration
}

func NewRedisClipExportGate(client redis.Cmdable, interval, lockTTL time.Duration) ClipExportGate {
	return &redisClipExportGate{redis: client, interval: interval, lockTTL: lockTTL}
}

func (g *redisClipExportGate) Begin(ctx context.Context, userID int64) (ClipExportPermit, error) {
	if g == nil || g.redis == nil || userID <= 0 || g.interval <= 0 || g.lockTTL <= 0 {
		return nil, errors.New("clip export gate unavailable")
	}
	token, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	key := clipExportKeyPrefix + strconv.FormatInt(userID, 10)
	result, err := g.redis.Eval(ctx, clipExportBeginScript, []string{key + ":completed", key + ":lock"}, token.String(), g.lockTTL.Milliseconds()).Int()
	if err != nil {
		return nil, err
	}
	switch result {
	case 1:
		return nil, errClipExportRateLimited
	case 2:
		return nil, errClipExportInProgress
	case 0:
		return &redisClipExportPermit{redis: g.redis, completedKey: key + ":completed", lockKey: key + ":lock", token: token.String(), interval: g.interval}, nil
	default:
		return nil, fmt.Errorf("unexpected clip export gate result: %d", result)
	}
}

type redisClipExportPermit struct {
	redis        redis.Cmdable
	completedKey string
	lockKey      string
	token        string
	interval     time.Duration
}

func (p *redisClipExportPermit) Commit(ctx context.Context) error {
	result, err := p.redis.Eval(ctx, clipExportCommitScript, []string{p.completedKey, p.lockKey}, p.token, p.interval.Milliseconds()).Int()
	if err != nil {
		return err
	}
	if result != 1 {
		return errors.New("clip export permit expired")
	}
	return nil
}

func (p *redisClipExportPermit) Release(ctx context.Context) error {
	_, err := p.redis.Eval(ctx, clipExportReleaseScript, []string{p.lockKey}, p.token).Result()
	return err
}

const clipExportBeginScript = `
if redis.call('EXISTS', KEYS[1]) == 1 then
  return 1
end
if redis.call('SET', KEYS[2], ARGV[1], 'NX', 'PX', ARGV[2]) then
  return 0
end
return 2
`

const clipExportCommitScript = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return 1
end
if redis.call('GET', KEYS[2]) ~= ARGV[1] then
  return 0
end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
redis.call('DEL', KEYS[2])
return 1
`

const clipExportReleaseScript = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`
