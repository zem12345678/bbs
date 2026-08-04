package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	accountDomain "reaction-service/internal/domain/account"
	domain "reaction-service/internal/domain/reaction"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	rdb *redis.Client
}

var addLikeScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[3]) == 1 then
  return {redis.call('SCARD', KEYS[1]), 0, 1}
end
local changed = redis.call('SADD', KEYS[1], ARGV[1])
local count = redis.call('SCARD', KEYS[1])
redis.call('ZADD', KEYS[2], count, ARGV[2])
return {count, changed, 0}
`)

var addFavoriteScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[2]) == 1 then
  return {redis.call('SCARD', KEYS[1]), 0, 1}
end
local changed = redis.call('SADD', KEYS[1], ARGV[1])
local count = redis.call('SCARD', KEYS[1])
return {count, changed, 0}
`)

var purgeLikeScript = redis.NewScript(`
local changed = redis.call('SREM', KEYS[1], ARGV[1])
local count = redis.call('SCARD', KEYS[1])
if count == 0 then
  redis.call('ZREM', KEYS[2], ARGV[2])
else
  redis.call('ZADD', KEYS[2], count, ARGV[2])
end
return {count, changed}
`)

func NewRedisStore(rdb *redis.Client) *RedisStore {
	return &RedisStore{rdb: rdb}
}

func likeKey(ref domain.EntityRef) string {
	return fmt.Sprintf("bbs:reaction:%s:%d:likes", ref.Type, ref.ID)
}

func favoriteKey(ref domain.EntityRef) string {
	return fmt.Sprintf("bbs:reaction:%s:%d:favorites", ref.Type, ref.ID)
}

func hotKey(entityType domain.EntityType) string {
	return fmt.Sprintf("bbs:reaction:%s:hot", entityType)
}

func erasedUserKey(userID int64) string {
	return fmt.Sprintf("bbs:reaction:erased-user:%d", userID)
}

func (s *RedisStore) Like(ctx context.Context, ref domain.EntityRef, userID int64) (int64, bool, error) {
	values, err := addLikeScript.Run(ctx, s.rdb,
		[]string{likeKey(ref), hotKey(ref.Type), erasedUserKey(userID)}, userID, ref.ID).Slice()
	return parseGuardedMutation(values, err)
}

func (s *RedisStore) Unlike(ctx context.Context, ref domain.EntityRef, userID int64) (int64, bool, error) {
	removed, err := s.rdb.SRem(ctx, likeKey(ref), userID).Result()
	if err != nil {
		return 0, false, err
	}
	count, err := s.rdb.SCard(ctx, likeKey(ref)).Result()
	if err != nil {
		return 0, false, err
	}
	if count == 0 {
		_ = s.rdb.ZRem(ctx, hotKey(ref.Type), ref.ID).Err()
	} else {
		_ = s.rdb.ZAdd(ctx, hotKey(ref.Type), redis.Z{Score: float64(count), Member: ref.ID}).Err()
	}
	return count, removed > 0, nil
}

func (s *RedisStore) Favorite(ctx context.Context, ref domain.EntityRef, userID int64) (int64, bool, error) {
	values, err := addFavoriteScript.Run(ctx, s.rdb,
		[]string{favoriteKey(ref), erasedUserKey(userID)}, userID).Slice()
	return parseGuardedMutation(values, err)
}

func (s *RedisStore) Unfavorite(ctx context.Context, ref domain.EntityRef, userID int64) (int64, bool, error) {
	removed, err := s.rdb.SRem(ctx, favoriteKey(ref), userID).Result()
	if err != nil {
		return 0, false, err
	}
	count, err := s.rdb.SCard(ctx, favoriteKey(ref)).Result()
	return count, removed > 0, err
}

func (s *RedisStore) LikeCount(ctx context.Context, ref domain.EntityRef) (int64, error) {
	return s.rdb.SCard(ctx, likeKey(ref)).Result()
}

func (s *RedisStore) FavoriteCount(ctx context.Context, ref domain.EntityRef) (int64, error) {
	return s.rdb.SCard(ctx, favoriteKey(ref)).Result()
}

func (s *RedisStore) HotIDs(ctx context.Context, entityType domain.EntityType, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 20
	}
	members, err := s.rdb.ZRevRange(ctx, hotKey(entityType), 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(members))
	for _, member := range members {
		var id int64
		if _, err := fmt.Sscan(member, &id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (s *RedisStore) TombstoneAccount(ctx context.Context, userID, deletionJobID int64, policyVersion int32) error {
	if s == nil || s.rdb == nil || userID <= 0 || deletionJobID <= 0 || policyVersion <= 0 {
		return accountDomain.ErrInvalidErasure
	}
	value := fmt.Sprintf("%d:%d", deletionJobID, policyVersion)
	return s.rdb.Set(ctx, erasedUserKey(userID), value, 0).Err()
}

func (s *RedisStore) PurgeAccount(ctx context.Context, userID int64) error {
	if s == nil || s.rdb == nil || userID <= 0 {
		return accountDomain.ErrInvalidErasure
	}
	if err := s.purgeAccountPattern(ctx, "bbs:reaction:*:*:likes", userID, true); err != nil {
		return err
	}
	return s.purgeAccountPattern(ctx, "bbs:reaction:*:*:favorites", userID, false)
}

func (s *RedisStore) purgeAccountPattern(ctx context.Context, pattern string, userID int64, likes bool) error {
	var cursor uint64
	for {
		keys, next, err := s.rdb.Scan(ctx, cursor, pattern, 250).Result()
		if err != nil {
			return err
		}
		for _, key := range keys {
			if likes {
				ref, ok := reactionRefFromKey(key, ":likes")
				if !ok {
					continue
				}
				if _, err := purgeLikeScript.Run(ctx, s.rdb, []string{key, hotKey(ref.Type)}, userID, ref.ID).Result(); err != nil {
					return err
				}
				continue
			}
			if err := s.rdb.SRem(ctx, key, userID).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

func reactionRefFromKey(key, suffix string) (domain.EntityRef, bool) {
	const prefix = "bbs:reaction:"
	if !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, suffix) {
		return domain.EntityRef{}, false
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(key, prefix), suffix), ":")
	if len(parts) != 2 {
		return domain.EntityRef{}, false
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	ref := domain.EntityRef{Type: domain.EntityType(parts[0]), ID: id}
	return ref, err == nil && ref.Validate() == nil
}

func parseGuardedMutation(values []any, err error) (int64, bool, error) {
	if err != nil {
		return 0, false, err
	}
	if len(values) != 3 {
		return 0, false, fmt.Errorf("unexpected Redis reaction mutation result")
	}
	count, err := redisInt64(values[0])
	if err != nil {
		return 0, false, err
	}
	changed, err := redisInt64(values[1])
	if err != nil {
		return 0, false, err
	}
	erased, err := redisInt64(values[2])
	if err != nil {
		return 0, false, err
	}
	if erased != 0 {
		return count, false, accountDomain.ErrUserErased
	}
	return count, changed != 0, nil
}

func redisInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return strconv.ParseInt(fmt.Sprint(value), 10, 64)
	}
}

var _ accountDomain.ErasureCache = (*RedisStore)(nil)
