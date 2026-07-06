package store

import (
	"context"
	"fmt"

	domain "reaction-service/internal/domain/reaction"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type ReactionCacheRebuilder struct {
	db  *gorm.DB
	rdb *redis.Client
}

type ReactionCacheRebuildStats struct {
	DeletedKeys     int64
	LikesLoaded     int64
	FavoritesLoaded int64
	HotEntries      int64
}

func NewReactionCacheRebuilder(db *gorm.DB, rdb *redis.Client) *ReactionCacheRebuilder {
	return &ReactionCacheRebuilder{db: db, rdb: rdb}
}

func (r *ReactionCacheRebuilder) Rebuild(ctx context.Context) (ReactionCacheRebuildStats, error) {
	var stats ReactionCacheRebuildStats
	deleted, err := r.deletePattern(ctx, "bbs:reaction:*:*:likes")
	if err != nil {
		return stats, err
	}
	stats.DeletedKeys += deleted
	deleted, err = r.deletePattern(ctx, "bbs:reaction:*:*:favorites")
	if err != nil {
		return stats, err
	}
	stats.DeletedKeys += deleted
	deleted, err = r.deletePattern(ctx, "bbs:reaction:*:hot")
	if err != nil {
		return stats, err
	}
	stats.DeletedKeys += deleted
	likeStats, err := r.rebuildLikes(ctx)
	if err != nil {
		return stats, err
	}
	stats.LikesLoaded = likeStats.LikesLoaded
	stats.HotEntries = likeStats.HotEntries
	favoritesLoaded, err := r.rebuildFavorites(ctx)
	if err != nil {
		return stats, err
	}
	stats.FavoritesLoaded = favoritesLoaded
	return stats, nil
}

func (r *ReactionCacheRebuilder) deletePattern(ctx context.Context, pattern string) (int64, error) {
	var cursor uint64
	var deleted int64
	for {
		keys, next, err := r.rdb.Scan(ctx, cursor, pattern, 500).Result()
		if err != nil {
			return deleted, fmt.Errorf("scan redis keys %s: %w", pattern, err)
		}
		if len(keys) > 0 {
			n, err := r.rdb.Del(ctx, keys...).Result()
			if err != nil {
				return deleted, fmt.Errorf("delete redis keys %s: %w", pattern, err)
			}
			deleted += n
		}
		cursor = next
		if cursor == 0 {
			return deleted, nil
		}
	}
}

func (r *ReactionCacheRebuilder) rebuildLikes(ctx context.Context) (ReactionCacheRebuildStats, error) {
	var stats ReactionCacheRebuildStats
	var rows []likePO
	hotCounts := map[domain.EntityType]map[int64]int64{}
	err := r.db.WithContext(ctx).
		Where("status = ?", likeStatusActive).
		Order("id ASC").
		FindInBatches(&rows, 1000, func(tx *gorm.DB, batch int) error {
			pipe := r.rdb.Pipeline()
			for _, row := range rows {
				ref := domain.EntityRef{Type: domain.EntityType(row.EntityType), ID: row.EntityID}
				if err := ref.Validate(); err != nil {
					continue
				}
				pipe.SAdd(ctx, likeKey(ref), row.UserID)
				stats.LikesLoaded++
				if hotCounts[ref.Type] == nil {
					hotCounts[ref.Type] = map[int64]int64{}
				}
				hotCounts[ref.Type][ref.ID]++
			}
			_, err := pipe.Exec(ctx)
			return err
		}).Error
	if err != nil {
		return stats, fmt.Errorf("rebuild like cache: %w", err)
	}
	for entityType, counts := range hotCounts {
		if len(counts) == 0 {
			continue
		}
		pipe := r.rdb.Pipeline()
		for entityID, count := range counts {
			pipe.ZAdd(ctx, hotKey(entityType), redis.Z{Score: float64(count), Member: entityID})
			stats.HotEntries++
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return stats, fmt.Errorf("rebuild hot cache: %w", err)
		}
	}
	return stats, nil
}

func (r *ReactionCacheRebuilder) rebuildFavorites(ctx context.Context) (int64, error) {
	var loaded int64
	var rows []favoritePO
	err := r.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("id ASC").
		FindInBatches(&rows, 1000, func(tx *gorm.DB, batch int) error {
			pipe := r.rdb.Pipeline()
			for _, row := range rows {
				ref := domain.EntityRef{Type: domain.EntityType(row.EntityType), ID: row.EntityID}
				if err := ref.Validate(); err != nil {
					continue
				}
				pipe.SAdd(ctx, favoriteKey(ref), row.UserID)
				loaded++
			}
			_, err := pipe.Exec(ctx)
			return err
		}).Error
	if err != nil {
		return loaded, fmt.Errorf("rebuild favorite cache: %w", err)
	}
	return loaded, nil
}

func (r *ReactionCacheRebuilder) Verify(ctx context.Context) error {
	if err := r.verifyLikeSets(ctx); err != nil {
		return err
	}
	if err := r.verifyFavoriteSets(ctx); err != nil {
		return err
	}
	return r.verifyHotSets(ctx)
}

func (r *ReactionCacheRebuilder) verifyLikeSets(ctx context.Context) error {
	var rows []struct {
		EntityType string
		EntityID   int64
		Count      int64
	}
	err := r.db.WithContext(ctx).Model(&likePO{}).
		Select("entity_type, entity_id, COUNT(*) AS count").
		Where("status = ?", likeStatusActive).
		Group("entity_type, entity_id").
		Scan(&rows).Error
	if err != nil {
		return err
	}
	for _, row := range rows {
		ref := domain.EntityRef{Type: domain.EntityType(row.EntityType), ID: row.EntityID}
		if err := ref.Validate(); err != nil {
			continue
		}
		got, err := r.rdb.SCard(ctx, likeKey(ref)).Result()
		if err != nil {
			return err
		}
		if got != row.Count {
			return fmt.Errorf("like cache mismatch %s/%d: pg=%d redis=%d", ref.Type, ref.ID, row.Count, got)
		}
	}
	return nil
}

func (r *ReactionCacheRebuilder) verifyFavoriteSets(ctx context.Context) error {
	var rows []struct {
		EntityType string
		EntityID   int64
		Count      int64
	}
	err := r.db.WithContext(ctx).Model(&favoritePO{}).
		Select("entity_type, entity_id, COUNT(*) AS count").
		Where("deleted_at IS NULL").
		Group("entity_type, entity_id").
		Scan(&rows).Error
	if err != nil {
		return err
	}
	for _, row := range rows {
		ref := domain.EntityRef{Type: domain.EntityType(row.EntityType), ID: row.EntityID}
		if err := ref.Validate(); err != nil {
			continue
		}
		got, err := r.rdb.SCard(ctx, favoriteKey(ref)).Result()
		if err != nil {
			return err
		}
		if got != row.Count {
			return fmt.Errorf("favorite cache mismatch %s/%d: pg=%d redis=%d", ref.Type, ref.ID, row.Count, got)
		}
	}
	return nil
}

func (r *ReactionCacheRebuilder) verifyHotSets(ctx context.Context) error {
	var rows []struct {
		EntityType string
		EntityID   int64
		Count      int64
	}
	err := r.db.WithContext(ctx).Model(&likePO{}).
		Select("entity_type, entity_id, COUNT(*) AS count").
		Where("status = ?", likeStatusActive).
		Group("entity_type, entity_id").
		Scan(&rows).Error
	if err != nil {
		return err
	}
	for _, row := range rows {
		entityType := domain.EntityType(row.EntityType)
		if !entityType.Valid() {
			continue
		}
		got, err := r.rdb.ZScore(ctx, hotKey(entityType), fmt.Sprint(row.EntityID)).Result()
		if err != nil {
			return err
		}
		if int64(got) != row.Count {
			return fmt.Errorf("hot cache mismatch %s/%d: pg=%d redis=%d", entityType, row.EntityID, row.Count, int64(got))
		}
	}
	return nil
}
