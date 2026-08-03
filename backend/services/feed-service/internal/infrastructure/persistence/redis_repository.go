package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	domain "feed-service/internal/domain/feed"

	"github.com/redis/go-redis/v9"
)

const (
	articleKeyPrefix              = "bbs:feed:article:"
	latestKey                     = "bbs:feed:latest"
	hotKey                        = "bbs:feed:hot"
	activeKey                     = "bbs:feed:active"
	authorLatestKeyPrefix         = "bbs:feed:latest:author:"
	authorLatestBackfillCursorKey = "bbs:feed:latest:authors:v1:cursor"
	authorLatestBackfillReadyKey  = "bbs:feed:latest:authors:v1:ready"
	authorLatestBackfillBatchSize = 250
)

type RedisRepository struct {
	rdb *redis.Client
}

func NewRedisRepository(rdb *redis.Client) *RedisRepository {
	return &RedisRepository{rdb: rdb}
}

func (r *RedisRepository) UpsertArticle(ctx context.Context, item domain.Item) error {
	item.EntityType = "article"
	return r.upsert(ctx, item)
}

func (r *RedisRepository) UpsertTopic(ctx context.Context, item domain.Item) error {
	item.EntityType = "topic"
	return r.upsert(ctx, item)
}

func (r *RedisRepository) upsert(ctx context.Context, item domain.Item) error {
	if item.ID <= 0 {
		return nil
	}
	if item.EntityType == "" {
		item.EntityType = "article"
	}
	existing, _ := r.get(ctx, item.ID)
	if item.LikeCount == 0 {
		item.LikeCount = existing.LikeCount
	}
	if item.FavoriteCount == 0 {
		item.FavoriteCount = existing.FavoriteCount
	}
	if item.CommentCount == 0 {
		item.CommentCount = existing.CommentCount
	}
	if item.ViewCount == 0 {
		item.ViewCount = existing.ViewCount
	}
	if item.CategoryID == 0 {
		item.CategoryID = existing.CategoryID
	}
	if item.PublishedAt == 0 {
		item.PublishedAt = item.UpdatedAt
	}
	if item.UpdatedAt == 0 {
		item.UpdatedAt = item.PublishedAt
	}
	item.HotScore = hotScore(item)
	tags, err := json.Marshal(item.Tags)
	if err != nil {
		return err
	}
	key := articleKey(item.ID)
	values := map[string]any{
		"entity_type":    item.EntityType,
		"id":             item.ID,
		"slug":           item.Slug,
		"title":          item.Title,
		"summary":        item.Summary,
		"body":           item.Body,
		"cover_url":      item.CoverURL,
		"tags":           string(tags),
		"author_id":      item.AuthorID,
		"status":         item.Status,
		"created_at":     item.CreatedAt,
		"updated_at":     item.UpdatedAt,
		"published_at":   item.PublishedAt,
		"like_count":     item.LikeCount,
		"favorite_count": item.FavoriteCount,
		"comment_count":  item.CommentCount,
		"hot_score":      item.HotScore,
		"view_count":     item.ViewCount,
		"category_id":    item.CategoryID,
	}
	pipe := r.rdb.TxPipeline()
	pipe.HSet(ctx, key, values)
	pipe.ZAdd(ctx, latestKey, redis.Z{Score: float64(item.PublishedAt), Member: item.ID})
	pipe.ZAdd(ctx, hotKey, redis.Z{Score: item.HotScore, Member: item.ID})
	pipe.ZAdd(ctx, activeKey, redis.Z{Score: activeScore(item), Member: item.ID})
	if existing.AuthorID > 0 && existing.AuthorID != item.AuthorID {
		pipe.ZRem(ctx, authorLatestKey(existing.AuthorID), item.ID)
	}
	if item.AuthorID > 0 {
		pipe.ZAdd(ctx, authorLatestKey(item.AuthorID), redis.Z{Score: float64(item.PublishedAt), Member: item.ID})
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (r *RedisRepository) RemoveArticle(ctx context.Context, id int64) error {
	return r.remove(ctx, id)
}

func (r *RedisRepository) RemoveTopic(ctx context.Context, id int64) error {
	return r.remove(ctx, id)
}

func (r *RedisRepository) remove(ctx context.Context, id int64) error {
	existing, _ := r.get(ctx, id)
	pipe := r.rdb.TxPipeline()
	pipe.Del(ctx, articleKey(id))
	pipe.ZRem(ctx, latestKey, id)
	pipe.ZRem(ctx, hotKey, id)
	pipe.ZRem(ctx, activeKey, id)
	if existing.AuthorID > 0 {
		pipe.ZRem(ctx, authorLatestKey(existing.AuthorID), id)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisRepository) SetLikeCount(ctx context.Context, id int64, count int64) error {
	return r.updateCount(ctx, id, "like_count", count)
}

func (r *RedisRepository) SetFavoriteCount(ctx context.Context, id int64, count int64) error {
	return r.updateCount(ctx, id, "favorite_count", count)
}

func (r *RedisRepository) SetViewCount(ctx context.Context, id int64, count int64) error {
	if count < 0 {
		count = 0
	}
	key := articleKey(id)
	if err := r.rdb.HSetNX(ctx, key, "id", id).Err(); err != nil {
		return err
	}
	return r.rdb.HSet(ctx, key, "view_count", count).Err()
}

func (r *RedisRepository) IncrementCommentCount(ctx context.Context, id int64, delta int64, activityAt int64) error {
	key := articleKey(id)
	if err := r.rdb.HSetNX(ctx, key, "id", id).Err(); err != nil {
		return err
	}
	count, err := r.rdb.HIncrBy(ctx, key, "comment_count", delta).Result()
	if err != nil {
		return err
	}
	if count < 0 {
		count = 0
		if err := r.rdb.HSet(ctx, key, "comment_count", 0).Err(); err != nil {
			return err
		}
	}
	if hasArticle, err := r.rdb.HExists(ctx, key, "title").Result(); err != nil || !hasArticle {
		return err
	}
	return r.refreshScores(ctx, id, activityAt)
}

func (r *RedisRepository) ListLatest(ctx context.Context, limit, offset int, authorIDs []int64) ([]domain.Item, error) {
	if len(authorIDs) == 0 {
		return r.list(ctx, latestKey, limit, offset)
	}
	authorIDs = uniquePositiveIDs(authorIDs)
	if len(authorIDs) == 0 {
		return []domain.Item{}, nil
	}
	ready, err := r.backfillAuthorLatestIndexes(ctx)
	if err == nil && ready {
		return r.listLatestByAuthorIndexes(ctx, authorIDs, limit, offset)
	}
	return r.listLatestByGlobalFilter(ctx, authorIDs, limit, offset)
}

func (r *RedisRepository) ListHot(ctx context.Context, limit, offset int) ([]domain.Item, error) {
	return r.list(ctx, hotKey, limit, offset)
}

func (r *RedisRepository) ListActive(ctx context.Context, limit, offset int) ([]domain.Item, error) {
	items, err := r.list(ctx, activeKey, limit, offset)
	if err != nil || len(items) > 0 || offset > 0 {
		return items, err
	}
	return r.list(ctx, latestKey, limit, offset)
}

func (r *RedisRepository) updateCount(ctx context.Context, id int64, field string, count int64) error {
	key := articleKey(id)
	if err := r.rdb.HSetNX(ctx, key, "id", id).Err(); err != nil {
		return err
	}
	if err := r.rdb.HSet(ctx, key, field, count).Err(); err != nil {
		return err
	}
	if hasArticle, err := r.rdb.HExists(ctx, key, "title").Result(); err != nil || !hasArticle {
		return err
	}
	return r.refreshScores(ctx, id, 0)
}

func (r *RedisRepository) refreshScores(ctx context.Context, id int64, activityAt int64) error {
	item, err := r.get(ctx, id)
	if err != nil {
		return err
	}
	if activityAt > item.UpdatedAt {
		item.UpdatedAt = activityAt
	}
	if item.PublishedAt == 0 {
		item.PublishedAt = item.UpdatedAt
	}
	if item.UpdatedAt == 0 {
		item.UpdatedAt = item.PublishedAt
	}
	item.HotScore = hotScore(item)
	pipe := r.rdb.TxPipeline()
	pipe.HSet(ctx, articleKey(id), "updated_at", item.UpdatedAt, "published_at", item.PublishedAt, "hot_score", item.HotScore)
	pipe.ZAdd(ctx, hotKey, redis.Z{Score: item.HotScore, Member: id})
	pipe.ZAdd(ctx, activeKey, redis.Z{Score: activeScore(item), Member: id})
	_, err = pipe.Exec(ctx)
	return err
}

func (r *RedisRepository) list(ctx context.Context, key string, limit, offset int) ([]domain.Item, error) {
	if limit <= 0 {
		limit = 20
	}
	stop := int64(offset + limit - 1)
	ids, err := r.rdb.ZRevRange(ctx, key, int64(offset), stop).Result()
	if err != nil {
		return nil, err
	}
	items := make([]domain.Item, 0, len(ids))
	for _, rawID := range ids {
		id, err := strconv.ParseInt(rawID, 10, 64)
		if err != nil {
			continue
		}
		item, err := r.get(ctx, id)
		if err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *RedisRepository) listLatestByAuthorIndexes(ctx context.Context, authorIDs []int64, limit, offset int) ([]domain.Item, error) {
	count := offset + limit
	pipe := r.rdb.Pipeline()
	commands := make([]*redis.ZSliceCmd, 0, len(authorIDs))
	for _, authorID := range authorIDs {
		commands = append(commands, pipe.ZRevRangeWithScores(ctx, authorLatestKey(authorID), 0, int64(count-1)))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}

	byID := make(map[string]redis.Z)
	for _, command := range commands {
		for _, candidate := range command.Val() {
			id := fmt.Sprint(candidate.Member)
			if current, ok := byID[id]; !ok || candidate.Score > current.Score {
				candidate.Member = id
				byID[id] = candidate
			}
		}
	}
	candidates := make([]redis.Z, 0, len(byID))
	for _, candidate := range byID {
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Member.(string) > candidates[j].Member.(string)
		}
		return candidates[i].Score > candidates[j].Score
	})
	if offset >= len(candidates) {
		return []domain.Item{}, nil
	}
	end := offset + limit
	if end > len(candidates) {
		end = len(candidates)
	}
	items := make([]domain.Item, 0, end-offset)
	for _, candidate := range candidates[offset:end] {
		id, err := strconv.ParseInt(candidate.Member.(string), 10, 64)
		if err != nil {
			continue
		}
		item, err := r.get(ctx, id)
		if err == nil {
			items = append(items, item)
		}
	}
	return items, nil
}

func (r *RedisRepository) listLatestByGlobalFilter(ctx context.Context, authorIDs []int64, limit, offset int) ([]domain.Item, error) {
	authors := make(map[int64]struct{}, len(authorIDs))
	for _, authorID := range authorIDs {
		authors[authorID] = struct{}{}
	}
	items := make([]domain.Item, 0, limit)
	const batchSize = int64(200)
	var start int64
	for len(items) < limit {
		ids, err := r.rdb.ZRevRange(ctx, latestKey, start, start+batchSize-1).Result()
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			break
		}
		for _, rawID := range ids {
			id, err := strconv.ParseInt(rawID, 10, 64)
			if err != nil {
				continue
			}
			item, err := r.get(ctx, id)
			if err != nil {
				continue
			}
			if _, ok := authors[item.AuthorID]; !ok {
				continue
			}
			if offset > 0 {
				offset--
				continue
			}
			items = append(items, item)
			if len(items) == limit {
				break
			}
		}
		start += int64(len(ids))
		if len(ids) < int(batchSize) {
			break
		}
	}
	return items, nil
}

func (r *RedisRepository) backfillAuthorLatestIndexes(ctx context.Context) (bool, error) {
	ready, err := r.rdb.Exists(ctx, authorLatestBackfillReadyKey).Result()
	if err != nil || ready > 0 {
		return ready > 0, err
	}
	cursor := uint64(0)
	rawCursor, err := r.rdb.Get(ctx, authorLatestBackfillCursorKey).Result()
	if err != nil && err != redis.Nil {
		return false, err
	}
	if err == nil {
		cursor, _ = strconv.ParseUint(rawCursor, 10, 64)
	}
	values, nextCursor, err := r.rdb.ZScan(ctx, latestKey, cursor, "", authorLatestBackfillBatchSize).Result()
	if err != nil {
		return false, err
	}

	pipe := r.rdb.TxPipeline()
	for index := 0; index+1 < len(values); index += 2 {
		id, parseErr := strconv.ParseInt(values[index], 10, 64)
		score, scoreErr := strconv.ParseFloat(values[index+1], 64)
		if parseErr != nil || scoreErr != nil {
			continue
		}
		item, getErr := r.get(ctx, id)
		if getErr == nil && item.AuthorID > 0 {
			pipe.ZAdd(ctx, authorLatestKey(item.AuthorID), redis.Z{Score: score, Member: id})
		}
	}
	if nextCursor == 0 {
		pipe.Del(ctx, authorLatestBackfillCursorKey)
		pipe.Set(ctx, authorLatestBackfillReadyKey, "1", 0)
	} else {
		pipe.Set(ctx, authorLatestBackfillCursorKey, strconv.FormatUint(nextCursor, 10), 0)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}
	return nextCursor == 0, nil
}

func (r *RedisRepository) get(ctx context.Context, id int64) (domain.Item, error) {
	values, err := r.rdb.HGetAll(ctx, articleKey(id)).Result()
	if err != nil {
		return domain.Item{}, err
	}
	if len(values) == 0 {
		return domain.Item{}, fmt.Errorf("feed item %d not found", id)
	}
	var tags []string
	_ = json.Unmarshal([]byte(values["tags"]), &tags)
	entityType := values["entity_type"]
	if entityType == "" {
		entityType = "article"
	}
	return domain.Item{
		EntityType:    entityType,
		ID:            int64Field(values, "id"),
		Slug:          values["slug"],
		Title:         values["title"],
		Summary:       values["summary"],
		Body:          values["body"],
		CoverURL:      values["cover_url"],
		Tags:          tags,
		AuthorID:      int64Field(values, "author_id"),
		Status:        int32(int64Field(values, "status")),
		CreatedAt:     int64Field(values, "created_at"),
		UpdatedAt:     int64Field(values, "updated_at"),
		PublishedAt:   int64Field(values, "published_at"),
		LikeCount:     int64Field(values, "like_count"),
		FavoriteCount: int64Field(values, "favorite_count"),
		CommentCount:  int64Field(values, "comment_count"),
		HotScore:      float64Field(values, "hot_score"),
		ViewCount:     int64Field(values, "view_count"),
		CategoryID:    int64Field(values, "category_id"),
	}, nil
}

func articleKey(id int64) string {
	return articleKeyPrefix + strconv.FormatInt(id, 10)
}

func authorLatestKey(authorID int64) string {
	return authorLatestKeyPrefix + strconv.FormatInt(authorID, 10)
}

func uniquePositiveIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func hotScore(item domain.Item) float64 {
	base := float64(item.PublishedAt)
	return base + float64(item.LikeCount*3000+item.FavoriteCount*5000+item.CommentCount*2000)
}

func activeScore(item domain.Item) float64 {
	base := item.UpdatedAt
	if base == 0 {
		base = item.PublishedAt
	}
	return float64(base) + float64(item.CommentCount*4000+item.LikeCount*1000+item.FavoriteCount*1000)
}

func int64Field(values map[string]string, key string) int64 {
	value, _ := strconv.ParseInt(values[key], 10, 64)
	return value
}

func float64Field(values map[string]string, key string) float64 {
	value, _ := strconv.ParseFloat(values[key], 64)
	return value
}
