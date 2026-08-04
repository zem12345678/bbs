package persistence

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"

	domain "credit-service/internal/domain/credit"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	creditLeaderboardZSetKey        = "bbs:credit:leaderboard"
	creditLeaderboardRevisionKey    = "bbs:credit:leaderboard:revision"
	creditLeaderboardReadyMember    = "__leaderboard_ready__"
	creditLeaderboardMemberPrefix   = "u:"
	creditLeaderboardWriteBatchSize = 1000
	maxExactLeaderboardScore        = int64(1<<53 - 1)
)

var errLeaderboardScoreOutOfRange = errors.New("credit leaderboard score exceeds redis zset exact integer range")

const leaderboardApplyUpdatesScript = `
local cachedRevision = tonumber(redis.call('GET', KEYS[2]))
if not cachedRevision or cachedRevision ~= tonumber(ARGV[2]) then
  return 0
end
if not redis.call('ZSCORE', KEYS[1], ARGV[1]) then
  return 0
end
for index = 4, #ARGV, 2 do
  local member = ARGV[index]
  local total = tonumber(ARGV[index + 1])
  if total > 0 then
    redis.call('ZADD', KEYS[1], total, member)
  else
    redis.call('ZREM', KEYS[1], member)
  end
end
redis.call('SET', KEYS[2], ARGV[3])
return 1
`

const leaderboardReplaceScript = `
local cachedRevision = tonumber(redis.call('GET', KEYS[2]))
if cachedRevision and cachedRevision > tonumber(ARGV[1]) then
  return 0
end
redis.call('RENAME', KEYS[3], KEYS[1])
redis.call('SET', KEYS[2], ARGV[1])
return 1
`

// RedisLeaderboardCache stores an immutable, revisioned snapshot of positive credit balances.
// PostgreSQL validates the revision before this cache is used.
type RedisLeaderboardCache struct {
	rdb *redis.Client
}

func NewRedisLeaderboardCache(rdb *redis.Client) *RedisLeaderboardCache {
	return &RedisLeaderboardCache{rdb: rdb}
}

func (c *RedisLeaderboardCache) List(ctx context.Context, limit int32) ([]domain.LeaderboardEntry, int64, bool, error) {
	if c == nil || c.rdb == nil || limit <= 0 {
		return nil, 0, false, nil
	}
	revision, err := c.rdb.Get(ctx, creditLeaderboardRevisionKey).Result()
	if errors.Is(err, redis.Nil) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	parsedRevision, err := strconv.ParseInt(revision, 10, 64)
	if err != nil || parsedRevision < 0 {
		return nil, 0, false, nil
	}
	if _, err := c.rdb.ZScore(ctx, creditLeaderboardZSetKey, creditLeaderboardReadyMember).Result(); errors.Is(err, redis.Nil) {
		return nil, 0, false, nil
	} else if err != nil {
		return nil, 0, false, err
	}
	rows, err := c.rdb.ZRevRangeWithScores(ctx, creditLeaderboardZSetKey, 0, int64(limit)-1).Result()
	if err != nil {
		return nil, 0, false, err
	}
	items := make([]domain.LeaderboardEntry, 0, len(rows))
	for _, row := range rows {
		member := fmt.Sprint(row.Member)
		if member == creditLeaderboardReadyMember {
			continue
		}
		userID, ok := parseLeaderboardMember(member)
		if !ok || row.Score <= 0 || row.Score > float64(maxExactLeaderboardScore) || math.Trunc(row.Score) != row.Score {
			return nil, 0, false, nil
		}
		items = append(items, domain.LeaderboardEntry{
			UserID: userID,
			Total:  int64(row.Score),
			Rank:   int32(len(items) + 1),
		})
	}
	return items, parsedRevision, true, nil
}

func (c *RedisLeaderboardCache) Replace(ctx context.Context, revision int64, entries []domain.LeaderboardEntry) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	if revision < 0 {
		return errors.New("credit leaderboard revision must not be negative")
	}
	for _, entry := range entries {
		if entry.UserID <= 0 || entry.Total <= 0 || entry.Total > maxExactLeaderboardScore {
			return errLeaderboardScoreOutOfRange
		}
	}
	temporaryKey := creditLeaderboardZSetKey + ":rebuild:" + uuid.NewString()
	if err := c.rdb.ZAdd(ctx, temporaryKey, redis.Z{
		Score:  -1,
		Member: creditLeaderboardReadyMember,
	}).Err(); err != nil {
		return err
	}
	for start := 0; start < len(entries); start += creditLeaderboardWriteBatchSize {
		end := start + creditLeaderboardWriteBatchSize
		if end > len(entries) {
			end = len(entries)
		}
		pipe := c.rdb.Pipeline()
		for _, entry := range entries[start:end] {
			pipe.ZAdd(ctx, temporaryKey, redis.Z{
				Score:  float64(entry.Total),
				Member: leaderboardMember(entry.UserID),
			})
		}
		if _, err := pipe.Exec(ctx); err != nil {
			_ = c.rdb.Del(ctx, temporaryKey).Err()
			return err
		}
	}
	replaced, err := c.rdb.Eval(ctx, leaderboardReplaceScript, []string{
		creditLeaderboardZSetKey,
		creditLeaderboardRevisionKey,
		temporaryKey,
	}, revision).Result()
	if err != nil || fmt.Sprint(replaced) != "1" {
		_ = c.rdb.Del(ctx, temporaryKey).Err()
	}
	return err
}

// Apply updates a ready cache only when its revision matches the expected prior revision.
// A missed or out-of-order update leaves the cache stale so PostgreSQL can rebuild it.
func (c *RedisLeaderboardCache) Apply(ctx context.Context, expectedRevision, revision int64, entries []domain.LeaderboardEntry) (bool, error) {
	if c == nil || c.rdb == nil || len(entries) == 0 {
		return false, nil
	}
	if expectedRevision < 0 || revision <= expectedRevision {
		return false, errors.New("invalid credit leaderboard revision update")
	}
	args := make([]interface{}, 0, 3+len(entries)*2)
	args = append(args, creditLeaderboardReadyMember, expectedRevision, revision)
	for _, entry := range entries {
		if entry.UserID <= 0 || entry.Total < 0 || entry.Total > maxExactLeaderboardScore {
			return false, errLeaderboardScoreOutOfRange
		}
		args = append(args, leaderboardMember(entry.UserID), entry.Total)
	}
	updated, err := c.rdb.Eval(ctx, leaderboardApplyUpdatesScript, []string{
		creditLeaderboardZSetKey,
		creditLeaderboardRevisionKey,
	}, args...).Result()
	if err != nil {
		return false, err
	}
	return fmt.Sprint(updated) == "1", nil
}

func (c *RedisLeaderboardCache) Remove(ctx context.Context, userIDs ...int64) error {
	if c == nil || c.rdb == nil || len(userIDs) == 0 {
		return nil
	}
	members := make([]any, 0, len(userIDs))
	for _, userID := range userIDs {
		if userID > 0 {
			members = append(members, leaderboardMember(userID))
		}
	}
	if len(members) == 0 {
		return nil
	}
	return c.rdb.ZRem(ctx, creditLeaderboardZSetKey, members...).Err()
}

func leaderboardMember(userID int64) string {
	return creditLeaderboardMemberPrefix + fmt.Sprintf("%019d", userID)
}

func parseLeaderboardMember(member string) (int64, bool) {
	if len(member) != len(creditLeaderboardMemberPrefix)+19 || member[:len(creditLeaderboardMemberPrefix)] != creditLeaderboardMemberPrefix {
		return 0, false
	}
	userID, err := strconv.ParseInt(member[len(creditLeaderboardMemberPrefix):], 10, 64)
	if err != nil || userID <= 0 {
		return 0, false
	}
	return userID, true
}
