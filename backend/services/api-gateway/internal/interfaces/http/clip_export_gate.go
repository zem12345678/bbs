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
	errExportRateLimited = errors.New("export rate limit exceeded")
	errExportInProgress  = errors.New("export already in progress")

	// Keep the existing names for the focused Clip tests and callers.
	errClipExportRateLimited = errExportRateLimited
	errClipExportInProgress  = errExportInProgress
)

const (
	clipExportKeyPrefix      = "rate:exports:clips:user:"
	antennaExportKeyPrefix   = "rate:exports:antennas:user:"
	blockingExportKeyPrefix  = "rate:exports:blocking:user:"
	followingExportKeyPrefix = "rate:exports:following:user:"
	muteExportKeyPrefix      = "rate:exports:muting:user:"
	userListExportKeyPrefix  = "rate:exports:user-lists:user:"
)

type ExportGate interface {
	Begin(context.Context, int64) (ExportPermit, error)
}

type ExportPermit interface {
	Commit(context.Context) error
	Release(context.Context) error
}

type ClipExportGate = ExportGate
type ClipExportPermit = ExportPermit

type redisExportGate struct {
	redis     redis.Cmdable
	keyPrefix string
	interval  time.Duration
	lockTTL   time.Duration
}

func NewRedisClipExportGate(client redis.Cmdable, interval, lockTTL time.Duration) ClipExportGate {
	return newRedisExportGate(client, clipExportKeyPrefix, interval, lockTTL)
}

func NewRedisAntennaExportGate(client redis.Cmdable, interval, lockTTL time.Duration) ExportGate {
	return newRedisExportGate(client, antennaExportKeyPrefix, interval, lockTTL)
}

func NewRedisBlockingExportGate(client redis.Cmdable, interval, lockTTL time.Duration) ExportGate {
	return newRedisExportGate(client, blockingExportKeyPrefix, interval, lockTTL)
}

func NewRedisFollowingExportGate(client redis.Cmdable, interval, lockTTL time.Duration) ExportGate {
	return newRedisExportGate(client, followingExportKeyPrefix, interval, lockTTL)
}

func NewRedisMuteExportGate(client redis.Cmdable, interval, lockTTL time.Duration) ExportGate {
	return newRedisExportGate(client, muteExportKeyPrefix, interval, lockTTL)
}

func NewRedisUserListExportGate(client redis.Cmdable, interval, lockTTL time.Duration) ExportGate {
	return newRedisExportGate(client, userListExportKeyPrefix, interval, lockTTL)
}

func newRedisExportGate(client redis.Cmdable, keyPrefix string, interval, lockTTL time.Duration) ExportGate {
	return &redisExportGate{redis: client, keyPrefix: keyPrefix, interval: interval, lockTTL: lockTTL}
}

func (g *redisExportGate) Begin(ctx context.Context, userID int64) (ExportPermit, error) {
	if g == nil || g.redis == nil || userID <= 0 || g.interval <= 0 || g.lockTTL <= 0 {
		return nil, errors.New("export gate unavailable")
	}
	token, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	key := g.keyPrefix + strconv.FormatInt(userID, 10)
	result, err := g.redis.Eval(ctx, exportBeginScript, []string{key + ":completed", key + ":lock"}, token.String(), g.lockTTL.Milliseconds()).Int()
	if err != nil {
		return nil, err
	}
	switch result {
	case 1:
		return nil, errExportRateLimited
	case 2:
		return nil, errExportInProgress
	case 0:
		return &redisExportPermit{redis: g.redis, completedKey: key + ":completed", lockKey: key + ":lock", token: token.String(), interval: g.interval}, nil
	default:
		return nil, fmt.Errorf("unexpected export gate result: %d", result)
	}
}

type redisExportPermit struct {
	redis        redis.Cmdable
	completedKey string
	lockKey      string
	token        string
	interval     time.Duration
}

func (p *redisExportPermit) Commit(ctx context.Context) error {
	result, err := p.redis.Eval(ctx, exportCommitScript, []string{p.completedKey, p.lockKey}, p.token, p.interval.Milliseconds()).Int()
	if err != nil {
		return err
	}
	if result != 1 {
		return errors.New("export permit expired")
	}
	return nil
}

func (p *redisExportPermit) Release(ctx context.Context) error {
	_, err := p.redis.Eval(ctx, exportReleaseScript, []string{p.lockKey}, p.token).Result()
	return err
}

const exportBeginScript = `
if redis.call('EXISTS', KEYS[1]) == 1 then
  return 1
end
if redis.call('SET', KEYS[2], ARGV[1], 'NX', 'PX', ARGV[2]) then
  return 0
end
return 2
`

const exportCommitScript = `
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

const exportReleaseScript = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`
