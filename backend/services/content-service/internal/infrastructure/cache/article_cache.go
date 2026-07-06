package cache

import (
	"context"
	"encoding/json"
	"time"

	domain "content-service/internal/domain/article"

	"github.com/redis/go-redis/v9"
)

type ArticleCache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewArticleCache(rdb *redis.Client, ttl time.Duration) *ArticleCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &ArticleCache{rdb: rdb, ttl: ttl}
}

func articleSlugKey(slug string) string {
	return "bbs:content:article:slug:" + slug
}

func (c *ArticleCache) Get(ctx context.Context, slug string) (*domain.Article, bool) {
	b, err := c.rdb.Get(ctx, articleSlugKey(slug)).Bytes()
	if err != nil {
		return nil, false
	}
	var a domain.Article
	if json.Unmarshal(b, &a) != nil {
		return nil, false
	}
	return &a, true
}

func (c *ArticleCache) Set(ctx context.Context, a *domain.Article) {
	b, err := json.Marshal(a)
	if err != nil {
		return
	}
	_ = c.rdb.Set(ctx, articleSlugKey(a.Slug), b, c.ttl).Err()
}

func (c *ArticleCache) Del(ctx context.Context, slug string) {
	_ = c.rdb.Del(ctx, articleSlugKey(slug)).Err()
}
