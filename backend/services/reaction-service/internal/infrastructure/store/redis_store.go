package store

import (
	"context"
	"fmt"

	domain "reaction-service/internal/domain/reaction"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	rdb *redis.Client
}

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

func (s *RedisStore) Like(ctx context.Context, ref domain.EntityRef, userID int64) (int64, bool, error) {
	added, err := s.rdb.SAdd(ctx, likeKey(ref), userID).Result()
	if err != nil {
		return 0, false, err
	}
	count, err := s.rdb.SCard(ctx, likeKey(ref)).Result()
	if err != nil {
		return 0, false, err
	}
	if err := s.rdb.ZAdd(ctx, hotKey(ref.Type), redis.Z{Score: float64(count), Member: ref.ID}).Err(); err != nil {
		return count, added > 0, err
	}
	return count, added > 0, nil
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
	added, err := s.rdb.SAdd(ctx, favoriteKey(ref), userID).Result()
	if err != nil {
		return 0, false, err
	}
	count, err := s.rdb.SCard(ctx, favoriteKey(ref)).Result()
	return count, added > 0, err
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
