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
	_, err := c.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Rename(ctx, temporaryKey, creditLeaderboardZSetKey)
		pipe.Set(ctx, creditLeaderboardRevisionKey, revision, 0)
		return nil
	})
	if err != nil {
		_ = c.rdb.Del(ctx, temporaryKey).Err()
	}
	return err
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
