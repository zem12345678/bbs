package article

import (
	"context"
	"time"
)

type TagStats struct {
	Name  string
	Count int64
}

type Repository interface {
	Create(ctx context.Context, a *Article) error
	Update(ctx context.Context, a *Article) error
	FindBySlug(ctx context.Context, slug string) (*Article, error)
	FindByID(ctx context.Context, id int64) (*Article, error)
	List(ctx context.Context, status Status, tag string, authorID int64, limit, offset int) ([]*Article, error)
	ListTags(ctx context.Context, status Status, query string, limit int) ([]TagStats, error)
	UpdateStatus(ctx context.Context, id int64, status Status, publishedAt *time.Time) error
	FeedByTime(ctx context.Context, limit, offset int) ([]*Article, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*Article, error)
}

type Cache interface {
	Get(ctx context.Context, slug string) (*Article, bool)
	Set(ctx context.Context, a *Article)
	Del(ctx context.Context, slug string)
}
